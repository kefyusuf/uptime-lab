use std::{
    sync::{Arc, mpsc},
    thread,
    time::{Duration, Instant},
};

use crate::{ClaimError, ControlPlane, Probe, ProbeOutcome, ShutdownToken, SubmitError, WorkItem};

const MAX_ACTIVE_PROBES: usize = 4;
const NO_WORK_DELAY: Duration = Duration::from_secs(1);
const AMBIGUOUS_CLAIM_PAUSE: Duration = Duration::from_secs(20);
const CLAIM_BACKOFF: [Duration; 3] = [
    Duration::from_millis(250),
    Duration::from_millis(500),
    Duration::from_secs(1),
];
const RESULT_RETRY_BACKOFF: [Duration; 2] =
    [Duration::from_millis(250), Duration::from_millis(500)];
const COMPLETION_POLL: Duration = Duration::from_millis(10);

pub trait Sleeper: Send + Sync {
    fn sleep(&self, duration: Duration, shutdown: &ShutdownToken) -> bool;
}

#[derive(Clone, Copy, Debug, Default)]
pub struct ThreadSleeper;

impl Sleeper for ThreadSleeper {
    fn sleep(&self, duration: Duration, shutdown: &ShutdownToken) -> bool {
        let deadline = Instant::now() + duration;
        while !shutdown.is_cancelled() {
            let Some(remaining) = deadline.checked_duration_since(Instant::now()) else {
                return true;
            };
            if remaining.is_zero() {
                return true;
            }
            thread::sleep(remaining.min(Duration::from_millis(25)));
        }
        false
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WorkerEvent {
    pub event: &'static str,
    pub check_id: Option<String>,
    pub monitor_id: Option<String>,
    pub result_kind: Option<String>,
    pub duration_ms: Option<u64>,
}

pub trait EventSink: Send + Sync {
    fn emit(&self, event: WorkerEvent);
}

#[derive(Clone, Copy, Debug, Default)]
pub struct NoopEventSink;

impl EventSink for NoopEventSink {
    fn emit(&self, _event: WorkerEvent) {}
}

pub struct Worker<C, P, S, E>
where
    C: ControlPlane + Send + Sync,
    P: Probe + Send + Sync,
    S: Sleeper,
    E: EventSink,
{
    control_plane: Arc<C>,
    probe: Arc<P>,
    sleeper: Arc<S>,
    events: Arc<E>,
}

impl<C, P, S, E> Worker<C, P, S, E>
where
    C: ControlPlane + Send + Sync,
    P: Probe + Send + Sync,
    S: Sleeper,
    E: EventSink,
{
    pub const fn new(
        control_plane: Arc<C>,
        probe: Arc<P>,
        sleeper: Arc<S>,
        events: Arc<E>,
    ) -> Self {
        Self {
            control_plane,
            probe,
            sleeper,
            events,
        }
    }

    pub fn run(&self, shutdown: &ShutdownToken) {
        self.events.emit(WorkerEvent {
            event: "worker_started",
            check_id: None,
            monitor_id: None,
            result_kind: None,
            duration_ms: None,
        });

        thread::scope(|scope| {
            let (completed_tx, completed_rx) = mpsc::channel::<()>();
            let mut active = 0_usize;
            let mut claim_backoff_index = 0_usize;

            while !shutdown.is_cancelled() {
                while completed_rx.try_recv().is_ok() {
                    active = active.saturating_sub(1);
                }

                if active >= MAX_ACTIVE_PROBES {
                    match completed_rx.recv_timeout(COMPLETION_POLL) {
                        Ok(()) => active = active.saturating_sub(1),
                        Err(mpsc::RecvTimeoutError::Timeout) => {}
                        Err(mpsc::RecvTimeoutError::Disconnected) => break,
                    }
                    continue;
                }

                match self.control_plane.claim(shutdown) {
                    Ok(Some(work)) => {
                        claim_backoff_index = 0;
                        active += 1;

                        let control_plane = Arc::clone(&self.control_plane);
                        let probe = Arc::clone(&self.probe);
                        let sleeper = Arc::clone(&self.sleeper);
                        let events = Arc::clone(&self.events);
                        let completed_tx = completed_tx.clone();
                        let shutdown = shutdown.clone();

                        events.emit(WorkerEvent {
                            event: "check_claimed",
                            check_id: Some(work.id().to_string()),
                            monitor_id: work.monitor_id().map(str::to_string),
                            result_kind: None,
                            duration_ms: None,
                        });

                        scope.spawn(move || {
                            execute_work(
                                control_plane.as_ref(),
                                probe.as_ref(),
                                sleeper.as_ref(),
                                events.as_ref(),
                                &shutdown,
                                work,
                            );
                            let _ = completed_tx.send(());
                        });
                    }
                    Ok(None) => {
                        claim_backoff_index = 0;
                        self.events.emit(WorkerEvent {
                            event: "claim_no_work",
                            check_id: None,
                            monitor_id: None,
                            result_kind: None,
                            duration_ms: None,
                        });
                        if !self.sleeper.sleep(NO_WORK_DELAY, shutdown) {
                            break;
                        }
                    }
                    Err(ClaimError::AmbiguousTransport | ClaimError::Deadline) => {
                        if shutdown.is_cancelled() {
                            break;
                        }
                        claim_backoff_index = 0;
                        self.events.emit(WorkerEvent {
                            event: "claim_ambiguous_pause",
                            check_id: None,
                            monitor_id: None,
                            result_kind: None,
                            duration_ms: None,
                        });
                        if !self.sleeper.sleep(AMBIGUOUS_CLAIM_PAUSE, shutdown) {
                            break;
                        }
                    }
                    Err(ClaimError::Cancelled) if shutdown.is_cancelled() => break,
                    Err(_) => {
                        if shutdown.is_cancelled() {
                            break;
                        }
                        let delay = CLAIM_BACKOFF[claim_backoff_index.min(CLAIM_BACKOFF.len() - 1)];
                        claim_backoff_index =
                            (claim_backoff_index + 1).min(CLAIM_BACKOFF.len() - 1);
                        self.events.emit(WorkerEvent {
                            event: "claim_backoff",
                            check_id: None,
                            monitor_id: None,
                            result_kind: None,
                            duration_ms: None,
                        });
                        if !self.sleeper.sleep(delay, shutdown) {
                            break;
                        }
                    }
                }
            }

            drop(completed_tx);
            while active > 0 {
                match completed_rx.recv_timeout(COMPLETION_POLL) {
                    Ok(()) => active = active.saturating_sub(1),
                    Err(mpsc::RecvTimeoutError::Timeout) => {}
                    Err(mpsc::RecvTimeoutError::Disconnected) => break,
                }
            }
        });
    }
}

fn execute_work<C, P, S, E>(
    control_plane: &C,
    probe: &P,
    sleeper: &S,
    events: &E,
    shutdown: &ShutdownToken,
    work: WorkItem,
) where
    C: ControlPlane + Send + Sync,
    P: Probe + Send + Sync,
    S: Sleeper,
    E: EventSink,
{
    let started = Instant::now();
    let outcome = match probe.probe(&work) {
        Ok(outcome) => outcome,
        Err(_) => ProbeOutcome::Failure {
            duration_ms: u64::try_from(started.elapsed().as_millis())
                .unwrap_or(u64::MAX)
                .min(20_000),
            category: "internal_error".to_string(),
        },
    };

    let (result_kind, duration_ms) = outcome_fields(&outcome);
    events.emit(WorkerEvent {
        event: "probe_completed",
        check_id: Some(work.id().to_string()),
        monitor_id: work.monitor_id().map(str::to_string),
        result_kind: Some(result_kind.to_string()),
        duration_ms: Some(duration_ms),
    });

    let prepared = match control_plane.prepare_result(work.id(), &outcome) {
        Ok(prepared) => prepared,
        Err(_) => {
            events.emit(WorkerEvent {
                event: "result_prepare_failed",
                check_id: Some(work.id().to_string()),
                monitor_id: work.monitor_id().map(str::to_string),
                result_kind: Some(result_kind.to_string()),
                duration_ms: Some(duration_ms),
            });
            return;
        }
    };

    let mut retry_delays = RESULT_RETRY_BACKOFF.into_iter();
    loop {
        if shutdown.is_cancelled() {
            return;
        }

        match control_plane.submit_prepared(&prepared, shutdown) {
            Ok(()) => {
                events.emit(WorkerEvent {
                    event: "result_delivered",
                    check_id: Some(work.id().to_string()),
                    monitor_id: work.monitor_id().map(str::to_string),
                    result_kind: Some(result_kind.to_string()),
                    duration_ms: Some(duration_ms),
                });
                return;
            }
            Err(SubmitError::Cancelled) if shutdown.is_cancelled() => return,
            Err(SubmitError::Transport | SubmitError::Deadline) => {
                let Some(delay) = retry_delays.next() else {
                    events.emit(WorkerEvent {
                        event: "result_delivery_failed",
                        check_id: Some(work.id().to_string()),
                        monitor_id: work.monitor_id().map(str::to_string),
                        result_kind: Some(result_kind.to_string()),
                        duration_ms: Some(duration_ms),
                    });
                    return;
                };
                events.emit(WorkerEvent {
                    event: "result_retry",
                    check_id: Some(work.id().to_string()),
                    monitor_id: work.monitor_id().map(str::to_string),
                    result_kind: Some(result_kind.to_string()),
                    duration_ms: Some(duration_ms),
                });
                if !sleeper.sleep(delay, shutdown) {
                    return;
                }
            }
            Err(_) => {
                events.emit(WorkerEvent {
                    event: "result_delivery_failed",
                    check_id: Some(work.id().to_string()),
                    monitor_id: work.monitor_id().map(str::to_string),
                    result_kind: Some(result_kind.to_string()),
                    duration_ms: Some(duration_ms),
                });
                return;
            }
        }
    }
}

fn outcome_fields(outcome: &ProbeOutcome) -> (&str, u64) {
    match outcome {
        ProbeOutcome::HttpResponse { duration_ms, .. } => ("http_response", *duration_ms),
        ProbeOutcome::Failure {
            duration_ms,
            category,
        } => (category.as_str(), *duration_ms),
    }
}

use std::{
    collections::VecDeque,
    sync::{
        Arc, Mutex,
        atomic::{AtomicBool, AtomicUsize, Ordering},
    },
    thread,
    time::{Duration, Instant},
};

use checker_core::{
    ClaimError, ControlPlane, EventSink, NoopEventSink, Probe, ProbeOutcome, ShutdownToken,
    Sleeper, SubmitError, ThreadSleeper, WorkItem, Worker, WorkerEvent,
};

#[derive(Clone, Debug, Eq, PartialEq)]
struct Prepared {
    work_id: String,
    outcome: ProbeOutcome,
}

enum ClaimStep {
    Return(Result<Option<WorkItem>, ClaimError>),
    WaitForShutdown,
}

enum SubmitStep {
    Return(Result<(), SubmitError>),
    WaitForShutdown,
}

struct FakeControlPlane {
    claims: Mutex<VecDeque<ClaimStep>>,
    submits: Mutex<VecDeque<SubmitStep>>,
    claim_calls: AtomicUsize,
    active_claims: AtomicUsize,
    max_active_claims: AtomicUsize,
    prepare_calls: AtomicUsize,
    submitted: Mutex<Vec<Prepared>>,
    claim_started: AtomicBool,
    submit_started: AtomicBool,
}

impl FakeControlPlane {
    fn new(claims: Vec<ClaimStep>, submits: Vec<SubmitStep>) -> Self {
        Self {
            claims: Mutex::new(claims.into()),
            submits: Mutex::new(submits.into()),
            claim_calls: AtomicUsize::new(0),
            active_claims: AtomicUsize::new(0),
            max_active_claims: AtomicUsize::new(0),
            prepare_calls: AtomicUsize::new(0),
            submitted: Mutex::new(Vec::new()),
            claim_started: AtomicBool::new(false),
            submit_started: AtomicBool::new(false),
        }
    }

    fn submitted(&self) -> Vec<Prepared> {
        self.submitted.lock().expect("submitted lock").clone()
    }
}

impl ControlPlane for FakeControlPlane {
    type PreparedResult = Prepared;

    fn claim(&self, shutdown: &ShutdownToken) -> Result<Option<WorkItem>, ClaimError> {
        self.claim_calls.fetch_add(1, Ordering::SeqCst);
        let active = self.active_claims.fetch_add(1, Ordering::SeqCst) + 1;
        self.max_active_claims.fetch_max(active, Ordering::SeqCst);
        self.claim_started.store(true, Ordering::SeqCst);

        let step = self
            .claims
            .lock()
            .expect("claims lock")
            .pop_front()
            .unwrap_or(ClaimStep::WaitForShutdown);

        let result = match step {
            ClaimStep::Return(result) => result,
            ClaimStep::WaitForShutdown => {
                while !shutdown.is_cancelled() {
                    thread::yield_now();
                }
                Err(ClaimError::Cancelled)
            }
        };

        self.active_claims.fetch_sub(1, Ordering::SeqCst);
        result
    }

    fn prepare_result(
        &self,
        work_id: &str,
        outcome: &ProbeOutcome,
    ) -> Result<Self::PreparedResult, SubmitError> {
        self.prepare_calls.fetch_add(1, Ordering::SeqCst);
        Ok(Prepared {
            work_id: work_id.to_string(),
            outcome: outcome.clone(),
        })
    }

    fn submit_prepared(
        &self,
        result: &Self::PreparedResult,
        shutdown: &ShutdownToken,
    ) -> Result<(), SubmitError> {
        self.submit_started.store(true, Ordering::SeqCst);
        self.submitted
            .lock()
            .expect("submitted lock")
            .push(result.clone());

        let step = self
            .submits
            .lock()
            .expect("submits lock")
            .pop_front()
            .unwrap_or(SubmitStep::Return(Ok(())));

        match step {
            SubmitStep::Return(result) => result,
            SubmitStep::WaitForShutdown => {
                while !shutdown.is_cancelled() {
                    thread::yield_now();
                }
                Err(SubmitError::Cancelled)
            }
        }
    }
}

struct FakeProbe {
    calls: Mutex<Vec<String>>,
    active: AtomicUsize,
    max_active: AtomicUsize,
    block: bool,
    release: AtomicBool,
}

impl FakeProbe {
    fn immediate() -> Self {
        Self {
            calls: Mutex::new(Vec::new()),
            active: AtomicUsize::new(0),
            max_active: AtomicUsize::new(0),
            block: false,
            release: AtomicBool::new(true),
        }
    }

    fn blocking() -> Self {
        Self {
            calls: Mutex::new(Vec::new()),
            active: AtomicUsize::new(0),
            max_active: AtomicUsize::new(0),
            block: true,
            release: AtomicBool::new(false),
        }
    }

    fn release(&self) {
        self.release.store(true, Ordering::SeqCst);
    }

    fn calls(&self) -> Vec<String> {
        self.calls.lock().expect("probe calls lock").clone()
    }
}

impl Probe for FakeProbe {
    fn probe(&self, work: &WorkItem) -> Result<ProbeOutcome, checker_core::BoxError> {
        self.calls
            .lock()
            .expect("probe calls lock")
            .push(work.id().to_string());

        let active = self.active.fetch_add(1, Ordering::SeqCst) + 1;
        self.max_active.fetch_max(active, Ordering::SeqCst);

        while self.block && !self.release.load(Ordering::SeqCst) {
            thread::yield_now();
        }

        self.active.fetch_sub(1, Ordering::SeqCst);
        Ok(ProbeOutcome::Failure {
            duration_ms: 7,
            category: "timeout".to_string(),
        })
    }
}

#[derive(Default)]
struct RecordingSleeper {
    durations: Mutex<Vec<Duration>>,
    cancel_on_call: Option<usize>,
}

impl RecordingSleeper {
    fn cancelling_on(call: usize) -> Self {
        Self {
            durations: Mutex::new(Vec::new()),
            cancel_on_call: Some(call),
        }
    }

    fn durations(&self) -> Vec<Duration> {
        self.durations.lock().expect("durations lock").clone()
    }
}

impl Sleeper for RecordingSleeper {
    fn sleep(&self, duration: Duration, shutdown: &ShutdownToken) -> bool {
        let calls = {
            let mut durations = self.durations.lock().expect("durations lock");
            durations.push(duration);
            durations.len()
        };
        if self.cancel_on_call == Some(calls) {
            shutdown.cancel();
            return false;
        }
        !shutdown.is_cancelled()
    }
}

#[derive(Default)]
struct RecordingEvents {
    events: Mutex<Vec<WorkerEvent>>,
}

impl RecordingEvents {
    fn events(&self) -> Vec<WorkerEvent> {
        self.events.lock().expect("events lock").clone()
    }
}

impl EventSink for RecordingEvents {
    fn emit(&self, event: WorkerEvent) {
        self.events.lock().expect("events lock").push(event);
    }
}

fn work(index: usize) -> WorkItem {
    WorkItem::new(
        format!("018f22d3-1d6a-7cc0-a37b-{index:012x}"),
        "https://example.com/",
        10_000,
        3,
    )
    .with_monitor_id(format!("018f22d3-1d6a-7cc0-a37c-{index:012x}"))
}

struct CancelOnDrop(ShutdownToken);

impl Drop for CancelOnDrop {
    fn drop(&mut self) {
        self.0.cancel();
    }
}

struct ReleaseProbeOnDrop(Arc<FakeProbe>);

impl Drop for ReleaseProbeOnDrop {
    fn drop(&mut self) {
        self.0.release();
    }
}

fn wait_until(mut condition: impl FnMut() -> bool) {
    let deadline = Instant::now() + Duration::from_secs(5);
    while Instant::now() < deadline {
        if condition() {
            return;
        }
        thread::sleep(Duration::from_millis(1));
    }
    panic!("condition was not reached");
}

#[test]
fn worker_started_is_emitted_before_claim_activity() {
    let control_plane = Arc::new(FakeControlPlane::new(
        vec![ClaimStep::Return(Ok(None))],
        vec![],
    ));
    let sleeper = Arc::new(RecordingSleeper::cancelling_on(1));
    let events = Arc::new(RecordingEvents::default());
    let worker = Worker::new(
        control_plane,
        Arc::new(FakeProbe::immediate()),
        sleeper,
        Arc::clone(&events),
    );
    let shutdown = ShutdownToken::new();

    worker.run(&shutdown);

    let emitted = events.events();
    assert_eq!(
        emitted.first().map(|event| event.event),
        Some("worker_started")
    );
    assert!(emitted.iter().any(|event| event.event == "claim_no_work"));
}

#[test]
fn worker_never_exceeds_four_active_probes_and_claims_serially() {
    let control_plane = Arc::new(FakeControlPlane::new(
        (0..5)
            .map(|index| ClaimStep::Return(Ok(Some(work(index)))))
            .collect(),
        vec![],
    ));
    let probe = Arc::new(FakeProbe::blocking());
    let sleeper = Arc::new(RecordingSleeper::default());
    let events = Arc::new(NoopEventSink);
    let shutdown = ShutdownToken::new();
    let worker = Worker::new(
        Arc::clone(&control_plane),
        Arc::clone(&probe),
        Arc::clone(&sleeper),
        events,
    );

    thread::scope(|scope| {
        let _cancel_on_drop = CancelOnDrop(shutdown.clone());
        let _release_probe_on_drop = ReleaseProbeOnDrop(Arc::clone(&probe));
        let shutdown_for_worker = shutdown.clone();
        scope.spawn(move || worker.run(&shutdown_for_worker));

        wait_until(|| probe.max_active.load(Ordering::SeqCst) == 4);
        assert_eq!(probe.max_active.load(Ordering::SeqCst), 4);
        assert_eq!(control_plane.claim_calls.load(Ordering::SeqCst), 4);
        assert_eq!(control_plane.max_active_claims.load(Ordering::SeqCst), 1);

        shutdown.cancel();
        probe.release();
    });
}

#[test]
fn no_work_waits_exactly_one_second() {
    let control_plane = Arc::new(FakeControlPlane::new(
        vec![ClaimStep::Return(Ok(None))],
        vec![],
    ));
    let sleeper = Arc::new(RecordingSleeper::cancelling_on(1));
    let worker = Worker::new(
        control_plane,
        Arc::new(FakeProbe::immediate()),
        Arc::clone(&sleeper),
        Arc::new(NoopEventSink),
    );
    let shutdown = ShutdownToken::new();

    worker.run(&shutdown);

    assert_eq!(sleeper.durations(), vec![Duration::from_secs(1)]);
}

#[test]
fn ordinary_claim_failures_use_bounded_backoff() {
    let control_plane = Arc::new(FakeControlPlane::new(
        vec![
            ClaimStep::Return(Err(ClaimError::Transport)),
            ClaimStep::Return(Err(ClaimError::Rejected)),
            ClaimStep::Return(Err(ClaimError::InvalidResponse)),
        ],
        vec![],
    ));
    let sleeper = Arc::new(RecordingSleeper::cancelling_on(3));
    let worker = Worker::new(
        control_plane,
        Arc::new(FakeProbe::immediate()),
        Arc::clone(&sleeper),
        Arc::new(NoopEventSink),
    );
    let shutdown = ShutdownToken::new();

    worker.run(&shutdown);

    assert_eq!(
        sleeper.durations(),
        vec![
            Duration::from_millis(250),
            Duration::from_millis(500),
            Duration::from_secs(1),
        ]
    );
}

#[test]
fn ambiguous_or_timed_out_claim_pauses_new_claims_for_twenty_seconds() {
    for error in [ClaimError::AmbiguousTransport, ClaimError::Deadline] {
        let control_plane = Arc::new(FakeControlPlane::new(
            vec![ClaimStep::Return(Err(error))],
            vec![],
        ));
        let sleeper = Arc::new(RecordingSleeper::cancelling_on(1));
        let worker = Worker::new(
            control_plane,
            Arc::new(FakeProbe::immediate()),
            Arc::clone(&sleeper),
            Arc::new(NoopEventSink),
        );
        let shutdown = ShutdownToken::new();

        worker.run(&shutdown);

        assert_eq!(sleeper.durations(), vec![Duration::from_secs(20)]);
    }
}

#[test]
fn shutdown_cancels_inflight_claim_without_recovery_pause() {
    let control_plane = Arc::new(FakeControlPlane::new(
        vec![ClaimStep::WaitForShutdown],
        vec![],
    ));
    let sleeper = Arc::new(RecordingSleeper::default());
    let worker = Worker::new(
        Arc::clone(&control_plane),
        Arc::new(FakeProbe::immediate()),
        Arc::clone(&sleeper),
        Arc::new(NoopEventSink),
    );
    let shutdown = ShutdownToken::new();

    thread::scope(|scope| {
        let _cancel_on_drop = CancelOnDrop(shutdown.clone());
        let shutdown_for_worker = shutdown.clone();
        scope.spawn(move || worker.run(&shutdown_for_worker));

        wait_until(|| control_plane.claim_started.load(Ordering::SeqCst));
        shutdown.cancel();
    });

    assert!(sleeper.durations().is_empty());
}

#[test]
fn result_transport_retry_reuses_payload_and_never_reprobes() {
    let control_plane = Arc::new(FakeControlPlane::new(
        vec![
            ClaimStep::Return(Ok(Some(work(1)))),
            ClaimStep::WaitForShutdown,
        ],
        vec![
            SubmitStep::Return(Err(SubmitError::Transport)),
            SubmitStep::Return(Err(SubmitError::Deadline)),
            SubmitStep::Return(Ok(())),
        ],
    ));
    let probe = Arc::new(FakeProbe::immediate());
    let sleeper = Arc::new(RecordingSleeper::default());
    let worker = Worker::new(
        Arc::clone(&control_plane),
        Arc::clone(&probe),
        Arc::clone(&sleeper),
        Arc::new(NoopEventSink),
    );
    let shutdown = ShutdownToken::new();

    thread::scope(|scope| {
        let _cancel_on_drop = CancelOnDrop(shutdown.clone());
        let shutdown_for_worker = shutdown.clone();
        scope.spawn(move || worker.run(&shutdown_for_worker));

        wait_until(|| control_plane.submitted().len() == 3);
        shutdown.cancel();
    });

    assert_eq!(probe.calls(), vec![work(1).id().to_string()]);
    assert_eq!(control_plane.prepare_calls.load(Ordering::SeqCst), 1);

    let submitted = control_plane.submitted();
    assert_eq!(submitted.len(), 3);
    assert!(submitted.windows(2).all(|pair| pair[0] == pair[1]));

    let durations = sleeper.durations();
    assert!(durations.contains(&Duration::from_millis(250)));
    assert!(durations.contains(&Duration::from_millis(500)));
}

#[test]
fn terminal_result_failure_is_not_retried() {
    let control_plane = Arc::new(FakeControlPlane::new(
        vec![
            ClaimStep::Return(Ok(Some(work(2)))),
            ClaimStep::WaitForShutdown,
        ],
        vec![SubmitStep::Return(Err(SubmitError::Rejected))],
    ));
    let probe = Arc::new(FakeProbe::immediate());
    let worker = Worker::new(
        Arc::clone(&control_plane),
        Arc::clone(&probe),
        Arc::new(RecordingSleeper::default()),
        Arc::new(NoopEventSink),
    );
    let shutdown = ShutdownToken::new();

    thread::scope(|scope| {
        let _cancel_on_drop = CancelOnDrop(shutdown.clone());
        let shutdown_for_worker = shutdown.clone();
        scope.spawn(move || worker.run(&shutdown_for_worker));

        wait_until(|| control_plane.submitted().len() == 1);
        shutdown.cancel();
    });

    assert_eq!(probe.calls().len(), 1);
    assert_eq!(control_plane.submitted().len(), 1);
}

#[test]
fn shutdown_cancels_inflight_result_and_stops_retries() {
    let control_plane = Arc::new(FakeControlPlane::new(
        vec![
            ClaimStep::Return(Ok(Some(work(3)))),
            ClaimStep::WaitForShutdown,
        ],
        vec![SubmitStep::WaitForShutdown],
    ));
    let probe = Arc::new(FakeProbe::immediate());
    let sleeper = Arc::new(RecordingSleeper::default());
    let worker = Worker::new(
        Arc::clone(&control_plane),
        Arc::clone(&probe),
        Arc::clone(&sleeper),
        Arc::new(NoopEventSink),
    );
    let shutdown = ShutdownToken::new();

    thread::scope(|scope| {
        let _cancel_on_drop = CancelOnDrop(shutdown.clone());
        let shutdown_for_worker = shutdown.clone();
        scope.spawn(move || worker.run(&shutdown_for_worker));

        wait_until(|| control_plane.submit_started.load(Ordering::SeqCst));
        shutdown.cancel();
    });

    assert_eq!(probe.calls().len(), 1);
    assert_eq!(control_plane.submitted().len(), 1);
    assert!(sleeper.durations().is_empty());
}

#[test]
fn completed_slot_is_released_for_more_work() {
    let control_plane = Arc::new(FakeControlPlane::new(
        (10..15)
            .map(|index| ClaimStep::Return(Ok(Some(work(index)))))
            .chain([ClaimStep::WaitForShutdown])
            .collect(),
        vec![],
    ));
    let probe = Arc::new(FakeProbe::immediate());
    let worker = Worker::new(
        Arc::clone(&control_plane),
        Arc::clone(&probe),
        Arc::new(ThreadSleeper),
        Arc::new(NoopEventSink),
    );
    let shutdown = ShutdownToken::new();

    thread::scope(|scope| {
        let _cancel_on_drop = CancelOnDrop(shutdown.clone());
        let shutdown_for_worker = shutdown.clone();
        scope.spawn(move || worker.run(&shutdown_for_worker));

        wait_until(|| probe.calls().len() == 5);
        shutdown.cancel();
    });

    assert_eq!(probe.calls().len(), 5);
    assert!(probe.max_active.load(Ordering::SeqCst) <= 4);
}

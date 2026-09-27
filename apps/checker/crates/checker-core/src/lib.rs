#![forbid(unsafe_code)]

use std::{
    error::Error,
    fmt,
    sync::{
        Arc,
        atomic::{AtomicBool, Ordering},
    },
};

/// Shared boxed error type for Checker orchestration ports.
pub type BoxError = Box<dyn Error + Send + Sync + 'static>;

/// Stable role marker used by the composition shell.
pub const CRATE_ROLE: &str = "checker-core";

/// Cooperative process-shutdown signal shared by Checker adapters.
#[derive(Clone, Debug, Default)]
pub struct ShutdownToken {
    cancelled: Arc<AtomicBool>,
}

impl ShutdownToken {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn cancel(&self) {
        self.cancelled.store(true, Ordering::Release);
    }

    pub fn is_cancelled(&self) -> bool {
        self.cancelled.load(Ordering::Acquire)
    }
}

/// Stable semantic claim failures exposed to Checker orchestration.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ClaimError {
    Rejected,
    Transport,
    AmbiguousTransport,
    Deadline,
    Cancelled,
    InvalidResponse,
}

impl fmt::Display for ClaimError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(match self {
            Self::Rejected => "control-plane claim rejected",
            Self::Transport => "control-plane claim transport failure",
            Self::AmbiguousTransport => "ambiguous control-plane claim transport failure",
            Self::Deadline => "control-plane claim deadline exceeded",
            Self::Cancelled => "control-plane claim cancelled",
            Self::InvalidResponse => "invalid control-plane claim response",
        })
    }
}

impl Error for ClaimError {}

/// Stable semantic result-delivery failures exposed to Checker orchestration.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SubmitError {
    Rejected,
    Transport,
    Deadline,
    Cancelled,
    InvalidResponse,
    InvalidResult,
}

impl fmt::Display for SubmitError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(match self {
            Self::Rejected => "control-plane result rejected",
            Self::Transport => "control-plane result transport failure",
            Self::Deadline => "control-plane result deadline exceeded",
            Self::Cancelled => "control-plane result cancelled",
            Self::InvalidResponse => "invalid control-plane result response",
            Self::InvalidResult => "invalid normalized result",
        })
    }
}

impl Error for SubmitError {}

/// Contract-neutral work description owned by the Checker core.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WorkItem {
    id: String,
    monitor_id: Option<String>,
    target: String,
    timeout_ms: u64,
    max_redirects: u8,
}

impl WorkItem {
    /// Creates a work item without transport-specific interpretation.
    ///
    /// Probe-local tests may omit MonitorID; control-plane mapping adds it with
    /// the with_monitor_id builder.
    pub fn new(
        id: impl Into<String>,
        target: impl Into<String>,
        timeout_ms: u64,
        max_redirects: u8,
    ) -> Self {
        Self {
            id: id.into(),
            monitor_id: None,
            target: target.into(),
            timeout_ms,
            max_redirects,
        }
    }

    pub fn with_monitor_id(mut self, monitor_id: impl Into<String>) -> Self {
        self.monitor_id = Some(monitor_id.into());
        self
    }

    pub fn id(&self) -> &str {
        &self.id
    }

    pub fn monitor_id(&self) -> Option<&str> {
        self.monitor_id.as_deref()
    }

    pub fn target(&self) -> &str {
        &self.target
    }

    pub const fn timeout_ms(&self) -> u64 {
        self.timeout_ms
    }

    pub const fn max_redirects(&self) -> u8 {
        self.max_redirects
    }
}

/// Normalized probe outcome independent of HTTP client implementation details.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProbeOutcome {
    HttpResponse { duration_ms: u64, status: u16 },
    Failure { duration_ms: u64, category: String },
}

/// Execution port implemented by the probe adapter.
pub trait Probe {
    fn probe(&self, work: &WorkItem) -> Result<ProbeOutcome, BoxError>;
}

/// Control-plane port. Prepared result payloads remain adapter-owned and opaque
/// to checker-core so retries can reuse the exact same canonical bytes.
pub trait ControlPlane {
    type PreparedResult: Clone + Send + Sync;

    fn claim(&self, shutdown: &ShutdownToken) -> Result<Option<WorkItem>, ClaimError>;

    fn prepare_result(
        &self,
        work_id: &str,
        outcome: &ProbeOutcome,
    ) -> Result<Self::PreparedResult, SubmitError>;

    fn submit_prepared(
        &self,
        result: &Self::PreparedResult,
        shutdown: &ShutdownToken,
    ) -> Result<(), SubmitError>;
}

/// Runs at most one orchestration unit without prescribing retry/runtime loops.
pub fn run_once(
    control_plane: &impl ControlPlane,
    probe: &impl Probe,
    shutdown: &ShutdownToken,
) -> Result<bool, BoxError> {
    let Some(work) = control_plane.claim(shutdown)? else {
        return Ok(false);
    };

    let outcome = probe.probe(&work)?;
    let prepared = control_plane.prepare_result(work.id(), &outcome)?;
    control_plane.submit_prepared(&prepared, shutdown)?;
    Ok(true)
}

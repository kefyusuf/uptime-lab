#![forbid(unsafe_code)]

use std::error::Error;

/// Shared boxed error type for Checker orchestration ports.
pub type BoxError = Box<dyn Error + Send + Sync + 'static>;

/// Stable role marker used by the composition shell.
pub const CRATE_ROLE: &str = "checker-core";

/// Contract-neutral work description owned by the Checker core.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct WorkItem {
    id: String,
    target: String,
    timeout_ms: u64,
    max_redirects: u8,
}

impl WorkItem {
    /// Creates a work item without performing transport-specific interpretation.
    pub fn new(
        id: impl Into<String>,
        target: impl Into<String>,
        timeout_ms: u64,
        max_redirects: u8,
    ) -> Self {
        Self {
            id: id.into(),
            target: target.into(),
            timeout_ms,
            max_redirects,
        }
    }

    pub fn id(&self) -> &str {
        &self.id
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

/// Execution port implemented by a probe adapter in later tasks.
pub trait Probe {
    fn probe(&self, work: &WorkItem) -> Result<ProbeOutcome, BoxError>;
}

/// Control-plane port implemented by the internal API client in later tasks.
pub trait ControlPlane {
    fn claim(&self) -> Result<Option<WorkItem>, BoxError>;
    fn submit(&self, work_id: &str, outcome: &ProbeOutcome) -> Result<(), BoxError>;
}

/// Runs at most one orchestration unit without prescribing transport/runtime loops.
pub fn run_once(control_plane: &impl ControlPlane, probe: &impl Probe) -> Result<bool, BoxError> {
    let Some(work) = control_plane.claim()? else {
        return Ok(false);
    };

    let outcome = probe.probe(&work)?;
    control_plane.submit(work.id(), &outcome)?;
    Ok(true)
}

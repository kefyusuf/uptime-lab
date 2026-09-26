#![forbid(unsafe_code)]

/// Internal control-plane client adapter shell.
///
/// HTTP transport behavior is intentionally deferred to a later task.
#[derive(Clone, Copy, Debug, Default)]
pub struct ControlPlaneClient;

impl ControlPlaneClient {
    pub const fn new() -> Self {
        Self
    }
}

pub const CORE_ROLE: &str = checker_core::CRATE_ROLE;

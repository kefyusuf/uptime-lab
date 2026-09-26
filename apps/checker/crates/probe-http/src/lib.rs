#![forbid(unsafe_code)]

/// HTTP probe adapter shell.
///
/// Network execution is intentionally deferred to a later task.
#[derive(Clone, Copy, Debug, Default)]
pub struct HttpProbe;

impl HttpProbe {
    pub const fn new() -> Self {
        Self
    }
}

pub const CORE_ROLE: &str = checker_core::CRATE_ROLE;

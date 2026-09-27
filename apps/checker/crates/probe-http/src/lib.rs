#![forbid(unsafe_code)]

mod destination;

pub use destination::{
    DestinationError, DestinationPolicy, ResolveError, Resolver, ValidatedDestination,
};

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

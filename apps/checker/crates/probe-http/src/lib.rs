#![forbid(unsafe_code)]

mod destination;
mod probe;

#[cfg(test)]
mod probe_tests;

pub use destination::{
    DestinationError, DestinationPolicy, ResolveError, Resolver, ValidatedDestination,
};
pub use probe::{HttpProbe, ProbeFailureKind, ProbeResult, SystemResolver};

pub const CORE_ROLE: &str = checker_core::CRATE_ROLE;

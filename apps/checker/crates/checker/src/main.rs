#![forbid(unsafe_code)]

use std::{env, error::Error, process, sync::Arc};

use checker::{CONTROL_PLANE_URL_ENV, CheckerConfig, LifecycleEventSink};
use checker_core::{ShutdownToken, ThreadSleeper, Worker};
use control_plane_client::ControlPlaneClient;
use probe_http::HttpProbe;

fn main() {
    if run().is_err() {
        eprintln!("event=checker_startup_failed");
        process::exit(1);
    }
}

fn run() -> Result<(), Box<dyn Error>> {
    let raw_control_plane = env::var(CONTROL_PLANE_URL_ENV).ok();
    let config = CheckerConfig::parse(raw_control_plane.as_deref())?;

    let control_plane = Arc::new(ControlPlaneClient::connect(
        config.control_plane_base_url(),
    )?);
    let probe = Arc::new(HttpProbe::new());
    let sleeper = Arc::new(ThreadSleeper);
    let events = Arc::new(LifecycleEventSink::new());
    let shutdown = ShutdownToken::new();

    let worker = Worker::new(control_plane, probe, sleeper, events);
    worker.run(&shutdown);

    Ok(())
}

use std::{
    io::Read,
    net::TcpListener,
    thread,
    time::{Duration, Instant},
};

use checker_core::{ClaimError, ProbeOutcome, ShutdownToken, SubmitError};
use control_plane_client::ControlPlaneClient;

fn hanging_server() -> (String, thread::JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind loopback server");
    let address = listener.local_addr().expect("local address");
    let handle = thread::spawn(move || {
        let (mut stream, _) = listener.accept().expect("accept request");
        let mut buffer = [0_u8; 1024];
        let _ = stream.read(&mut buffer);
        thread::sleep(Duration::from_secs(2));
    });
    (format!("http://{address}"), handle)
}

#[test]
fn shutdown_cancels_in_flight_claim_promptly() {
    let (base_url, server) = hanging_server();
    let client = ControlPlaneClient::connect(&base_url).expect("construct client");
    let shutdown = ShutdownToken::new();
    let cancel = shutdown.clone();
    let canceller = thread::spawn(move || {
        thread::sleep(Duration::from_millis(100));
        cancel.cancel();
    });

    let started = Instant::now();
    let result = client.claim(&shutdown);
    let elapsed = started.elapsed();

    assert_eq!(result, Err(ClaimError::Cancelled));
    assert!(
        elapsed < Duration::from_secs(1),
        "claim cancellation took {elapsed:?}"
    );

    canceller.join().expect("canceller");
    server.join().expect("server");
}

#[test]
fn shutdown_cancels_in_flight_result_promptly_and_stops_delivery() {
    let (base_url, server) = hanging_server();
    let client = ControlPlaneClient::connect(&base_url).expect("construct client");
    let prepared = client
        .prepare_result(
            "018f22d3-1d6a-7cc0-a37b-46fc3fafd101",
            &ProbeOutcome::Failure {
                duration_ms: 10,
                category: "timeout".to_string(),
            },
        )
        .expect("prepare result");

    let shutdown = ShutdownToken::new();
    let cancel = shutdown.clone();
    let canceller = thread::spawn(move || {
        thread::sleep(Duration::from_millis(100));
        cancel.cancel();
    });

    let started = Instant::now();
    let result = client.submit_prepared(&prepared, &shutdown);
    let elapsed = started.elapsed();

    assert_eq!(result, Err(SubmitError::Cancelled));
    assert!(
        elapsed < Duration::from_secs(1),
        "result cancellation took {elapsed:?}"
    );

    canceller.join().expect("canceller");
    server.join().expect("server");
}

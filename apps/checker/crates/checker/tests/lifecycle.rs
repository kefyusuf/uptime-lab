use std::{
    fs,
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};

use checker::{CheckerConfig, LifecycleEventSink, READY_MARKER_PATH, format_event};
use checker_core::{EventSink, WorkerEvent};

fn temp_marker() -> PathBuf {
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("system clock")
        .as_nanos();
    std::env::temp_dir().join(format!("uptime-lab-checker-ready-{nonce}"))
}

#[test]
fn config_requires_only_control_plane_base_url() {
    let config = CheckerConfig::parse(Some("http://api:8080"))
        .expect("valid control-plane base URL should be accepted");
    assert_eq!(config.control_plane_base_url(), "http://api:8080");

    assert!(CheckerConfig::parse(None).is_err());
    assert!(CheckerConfig::parse(Some("")).is_err());
    assert!(CheckerConfig::parse(Some(" https://api:8080 ")).is_err());
}

#[test]
fn readiness_marker_is_created_only_after_worker_started_event() {
    let marker = temp_marker();
    let sink = LifecycleEventSink::with_ready_path(marker.clone());

    sink.emit(WorkerEvent {
        event: "claim_no_work",
        check_id: None,
        monitor_id: None,
        result_kind: None,
        duration_ms: None,
    });
    assert!(!marker.exists());

    sink.emit(WorkerEvent {
        event: "worker_started",
        check_id: None,
        monitor_id: None,
        result_kind: None,
        duration_ms: None,
    });
    assert!(marker.is_file());

    fs::remove_file(marker).expect("remove readiness marker");
}

#[test]
fn readiness_marker_matches_existing_container_contract() {
    assert_eq!(READY_MARKER_PATH, "/run/uptime-lab/ready");
}

#[test]
fn log_format_contains_only_stable_worker_fields() {
    let line = format_event(&WorkerEvent {
        event: "probe_completed",
        check_id: Some("check-1".to_string()),
        monitor_id: Some("monitor-1".to_string()),
        result_kind: Some("timeout".to_string()),
        duration_ms: Some(123),
    });

    assert!(line.contains("event=probe_completed"));
    assert!(line.contains("check_id=check-1"));
    assert!(line.contains("monitor_id=monitor-1"));
    assert!(line.contains("result_kind=timeout"));
    assert!(line.contains("duration_ms=123"));
    assert!(!line.contains("target"));
    assert!(!line.contains("password"));
}

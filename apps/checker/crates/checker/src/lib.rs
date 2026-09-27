#![forbid(unsafe_code)]

use std::{error::Error, fmt, fs, path::PathBuf};

use checker_core::{EventSink, WorkerEvent};

pub const READY_MARKER_PATH: &str = "/run/uptime-lab/ready";
pub const CONTROL_PLANE_URL_ENV: &str = "UPTIME_LAB_CONTROL_PLANE_URL";

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CheckerConfig {
    control_plane_base_url: String,
}

impl CheckerConfig {
    pub fn parse(raw: Option<&str>) -> Result<Self, ConfigError> {
        let value = raw.ok_or(ConfigError::MissingControlPlaneUrl)?;
        if value.is_empty()
            || value.trim() != value
            || !value.starts_with("http://")
            || value.contains(char::is_whitespace)
        {
            return Err(ConfigError::InvalidControlPlaneUrl);
        }

        Ok(Self {
            control_plane_base_url: value.to_string(),
        })
    }

    pub fn control_plane_base_url(&self) -> &str {
        &self.control_plane_base_url
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ConfigError {
    MissingControlPlaneUrl,
    InvalidControlPlaneUrl,
}

impl fmt::Display for ConfigError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(match self {
            Self::MissingControlPlaneUrl => "control-plane base URL is required",
            Self::InvalidControlPlaneUrl => "control-plane base URL is invalid",
        })
    }
}

impl Error for ConfigError {}

#[derive(Clone, Debug)]
pub struct LifecycleEventSink {
    ready_path: PathBuf,
}

impl LifecycleEventSink {
    pub fn new() -> Self {
        Self::with_ready_path(READY_MARKER_PATH)
    }

    pub fn with_ready_path(path: impl Into<PathBuf>) -> Self {
        Self {
            ready_path: path.into(),
        }
    }

    fn mark_ready(&self) -> std::io::Result<()> {
        if let Some(parent) = self.ready_path.parent() {
            fs::create_dir_all(parent)?;
        }
        fs::write(&self.ready_path, b"ready\n")
    }
}

impl Default for LifecycleEventSink {
    fn default() -> Self {
        Self::new()
    }
}

impl EventSink for LifecycleEventSink {
    fn emit(&self, event: WorkerEvent) {
        if event.event == "worker_started"
            && let Err(_error) = self.mark_ready()
        {
            eprintln!("event=readiness_marker_failed");
        }

        eprintln!("{}", format_event(&event));
    }
}

pub fn format_event(event: &WorkerEvent) -> String {
    let mut fields = vec![format!("event={}", event.event)];

    if let Some(check_id) = event.check_id.as_deref() {
        fields.push(format!("check_id={check_id}"));
    }
    if let Some(monitor_id) = event.monitor_id.as_deref() {
        fields.push(format!("monitor_id={monitor_id}"));
    }
    if let Some(result_kind) = event.result_kind.as_deref() {
        fields.push(format!("result_kind={result_kind}"));
    }
    if let Some(duration_ms) = event.duration_ms {
        fields.push(format!("duration_ms={duration_ms}"));
    }

    fields.join(" ")
}

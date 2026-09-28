#![forbid(unsafe_code)]

mod json;
mod transport;

use std::{collections::BTreeMap, time::Duration};

use checker_core::{ClaimError, ControlPlane, ProbeOutcome, ShutdownToken, SubmitError, WorkItem};

use json::{Value, parse_flat_object};
pub use transport::{
    ClientConfigError, HttpRequest, HttpResponse, StdHttpTransport, Transport, TransportError,
};

const OPERATION_TIMEOUT: Duration = Duration::from_secs(5);
const CLAIM_PATH: &str = "/internal/checks/claim";
const RESULT_PREFIX: &str = "/internal/checks/";
const RESULT_SUFFIX: &str = "/result";
const JSON_CONTENT_TYPE: &str = "application/json";

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PreparedResult {
    path: String,
    body: Vec<u8>,
}

impl PreparedResult {
    pub fn body(&self) -> &[u8] {
        &self.body
    }

    pub fn path(&self) -> &str {
        &self.path
    }
}

pub struct ControlPlaneClient<T = StdHttpTransport> {
    transport: T,
}

impl ControlPlaneClient<StdHttpTransport> {
    pub fn connect(base_url: &str) -> Result<Self, ClientConfigError> {
        Ok(Self {
            transport: StdHttpTransport::connect(base_url)?,
        })
    }
}

impl<T> ControlPlaneClient<T>
where
    T: Transport,
{
    pub const fn from_transport(transport: T) -> Self {
        Self { transport }
    }

    pub fn claim(&self, shutdown: &ShutdownToken) -> Result<Option<WorkItem>, ClaimError> {
        self.claim_inner(shutdown)
    }

    pub fn prepare_result(
        &self,
        work_id: &str,
        outcome: &ProbeOutcome,
    ) -> Result<PreparedResult, SubmitError> {
        prepare_result(work_id, outcome)
    }

    pub fn submit_prepared(
        &self,
        result: &PreparedResult,
        shutdown: &ShutdownToken,
    ) -> Result<(), SubmitError> {
        self.submit_inner(result, shutdown)
    }

    fn claim_inner(&self, shutdown: &ShutdownToken) -> Result<Option<WorkItem>, ClaimError> {
        let request = HttpRequest {
            method: "POST",
            path: CLAIM_PATH.to_string(),
            headers: Vec::new(),
            body: Vec::new(),
        };

        let response = self
            .transport
            .execute(&request, OPERATION_TIMEOUT, shutdown)
            .map_err(map_claim_transport)?;

        match response.status {
            204 if response.body.is_empty() => Ok(None),
            200 => decode_work(response).map(Some),
            400..=599 => Err(ClaimError::Rejected),
            _ => Err(ClaimError::InvalidResponse),
        }
    }

    fn submit_inner(
        &self,
        result: &PreparedResult,
        shutdown: &ShutdownToken,
    ) -> Result<(), SubmitError> {
        let request = HttpRequest {
            method: "PUT",
            path: result.path.clone(),
            headers: vec![("content-type".to_string(), JSON_CONTENT_TYPE.to_string())],
            body: result.body.clone(),
        };

        let response = self
            .transport
            .execute(&request, OPERATION_TIMEOUT, shutdown)
            .map_err(map_submit_transport)?;

        match response.status {
            204 if response.body.is_empty() => Ok(()),
            400..=599 => Err(SubmitError::Rejected),
            _ => Err(SubmitError::InvalidResponse),
        }
    }
}

impl<T> ControlPlane for ControlPlaneClient<T>
where
    T: Transport,
{
    type PreparedResult = PreparedResult;

    fn claim(&self, shutdown: &ShutdownToken) -> Result<Option<WorkItem>, ClaimError> {
        self.claim_inner(shutdown)
    }

    fn prepare_result(
        &self,
        work_id: &str,
        outcome: &ProbeOutcome,
    ) -> Result<Self::PreparedResult, SubmitError> {
        prepare_result(work_id, outcome)
    }

    fn submit_prepared(
        &self,
        result: &Self::PreparedResult,
        shutdown: &ShutdownToken,
    ) -> Result<(), SubmitError> {
        self.submit_inner(result, shutdown)
    }
}

pub const CORE_ROLE: &str = checker_core::CRATE_ROLE;

fn decode_work(response: HttpResponse) -> Result<WorkItem, ClaimError> {
    if !is_json_content_type(&response.headers) {
        return Err(ClaimError::InvalidResponse);
    }

    let mut object = parse_flat_object(&response.body).map_err(|_| ClaimError::InvalidResponse)?;
    if object.len() != 5 {
        return Err(ClaimError::InvalidResponse);
    }

    let check_id = take_string(&mut object, "checkId").ok_or(ClaimError::InvalidResponse)?;
    let monitor_id = take_string(&mut object, "monitorId").ok_or(ClaimError::InvalidResponse)?;
    let target_url = take_string(&mut object, "targetUrl").ok_or(ClaimError::InvalidResponse)?;
    let timeout_ms = take_integer(&mut object, "timeoutMs").ok_or(ClaimError::InvalidResponse)?;
    let max_redirects =
        take_integer(&mut object, "maxRedirects").ok_or(ClaimError::InvalidResponse)?;

    if !object.is_empty()
        || !is_uuid(&check_id)
        || !is_uuid(&monitor_id)
        || timeout_ms != 10_000
        || max_redirects != 3
    {
        return Err(ClaimError::InvalidResponse);
    }

    let max_redirects = u8::try_from(max_redirects).map_err(|_| ClaimError::InvalidResponse)?;
    Ok(WorkItem::new(check_id, target_url, timeout_ms, max_redirects).with_monitor_id(monitor_id))
}

fn prepare_result(work_id: &str, outcome: &ProbeOutcome) -> Result<PreparedResult, SubmitError> {
    if !is_uuid(work_id) {
        return Err(SubmitError::InvalidResult);
    }

    let body = match outcome {
        ProbeOutcome::HttpResponse {
            duration_ms,
            status,
        } => {
            if *duration_ms > 20_000 || !(100..=599).contains(status) {
                return Err(SubmitError::InvalidResult);
            }
            format!(
                "{{\"kind\":\"http_response\",\"durationMs\":{duration_ms},\"httpStatus\":{status}}}"
            )
            .into_bytes()
        }
        ProbeOutcome::Failure {
            duration_ms,
            category,
        } => {
            if *duration_ms > 20_000 || !is_rust_failure_kind(category) {
                return Err(SubmitError::InvalidResult);
            }
            format!("{{\"kind\":\"{category}\",\"durationMs\":{duration_ms}}}").into_bytes()
        }
    };

    Ok(PreparedResult {
        path: format!("{RESULT_PREFIX}{work_id}{RESULT_SUFFIX}"),
        body,
    })
}

fn map_claim_transport(error: TransportError) -> ClaimError {
    match error {
        TransportError::BeforeSend => ClaimError::Transport,
        TransportError::AfterSend => ClaimError::AmbiguousTransport,
        TransportError::Deadline => ClaimError::Deadline,
        TransportError::Cancelled => ClaimError::Cancelled,
        TransportError::Protocol => ClaimError::InvalidResponse,
    }
}

fn map_submit_transport(error: TransportError) -> SubmitError {
    match error {
        TransportError::BeforeSend | TransportError::AfterSend => SubmitError::Transport,
        TransportError::Deadline => SubmitError::Deadline,
        TransportError::Cancelled => SubmitError::Cancelled,
        TransportError::Protocol => SubmitError::InvalidResponse,
    }
}

fn is_json_content_type(headers: &[(String, String)]) -> bool {
    let Some(value) = headers
        .iter()
        .find(|(name, _)| name.eq_ignore_ascii_case("content-type"))
        .map(|(_, value)| value.as_str())
    else {
        return false;
    };

    value
        .split(';')
        .next()
        .is_some_and(|media_type| media_type.trim().eq_ignore_ascii_case(JSON_CONTENT_TYPE))
}

fn take_string(object: &mut BTreeMap<String, Value>, name: &str) -> Option<String> {
    match object.remove(name)? {
        Value::String(value) => Some(value),
        Value::Integer(_) => None,
    }
}

fn take_integer(object: &mut BTreeMap<String, Value>, name: &str) -> Option<u64> {
    match object.remove(name)? {
        Value::Integer(value) => Some(value),
        Value::String(_) => None,
    }
}

fn is_uuid(value: &str) -> bool {
    if value.len() != 36 {
        return false;
    }

    value.bytes().enumerate().all(|(index, byte)| match index {
        8 | 13 | 18 | 23 => byte == b'-',
        _ => byte.is_ascii_hexdigit(),
    })
}

fn is_rust_failure_kind(value: &str) -> bool {
    matches!(
        value,
        "dns_error"
            | "policy_rejected"
            | "timeout"
            | "connect_error"
            | "tls_error"
            | "protocol_error"
            | "internal_error"
    )
}

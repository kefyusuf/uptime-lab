use std::{
    sync::{Arc, Mutex},
    time::Duration,
};

use checker_core::{ClaimError, ProbeOutcome, ShutdownToken, SubmitError};
use control_plane_client::{
    ControlPlaneClient, HttpRequest, HttpResponse, PreparedResult, Transport, TransportError,
};

const CHECK_WORK_FIXTURE: &str =
    include_str!("../../../../../contracts/fixtures/internal/check_work.json");
const HTTP_RESULT_FIXTURE: &str =
    include_str!("../../../../../contracts/fixtures/internal/http_response_result.json");
const FAILURE_RESULT_FIXTURE: &str =
    include_str!("../../../../../contracts/fixtures/internal/failure_result.json");

#[derive(Clone, Debug)]
enum Scripted {
    Response(HttpResponse),
    Error(TransportError),
}

#[derive(Default)]
struct RecordingTransport {
    scripted: Mutex<Vec<Scripted>>,
    requests: Mutex<Vec<(HttpRequest, Duration)>>,
}

impl RecordingTransport {
    fn with(scripted: Vec<Scripted>) -> Self {
        Self {
            scripted: Mutex::new(scripted.into_iter().rev().collect()),
            requests: Mutex::new(Vec::new()),
        }
    }

    fn requests(&self) -> Vec<(HttpRequest, Duration)> {
        self.requests.lock().expect("request lock").clone()
    }
}

impl Transport for RecordingTransport {
    fn execute(
        &self,
        request: &HttpRequest,
        timeout: Duration,
        shutdown: &ShutdownToken,
    ) -> Result<HttpResponse, TransportError> {
        assert!(
            !shutdown.is_cancelled(),
            "test request unexpectedly cancelled"
        );
        self.requests
            .lock()
            .expect("request lock")
            .push((request.clone(), timeout));
        match self
            .scripted
            .lock()
            .expect("script lock")
            .pop()
            .expect("scripted response")
        {
            Scripted::Response(response) => Ok(response),
            Scripted::Error(error) => Err(error),
        }
    }
}

fn response(status: u16, content_type: Option<&str>, body: &str) -> HttpResponse {
    let mut headers = Vec::new();
    if let Some(value) = content_type {
        headers.push(("content-type".to_string(), value.to_string()));
    }
    HttpResponse {
        status,
        headers,
        body: body.as_bytes().to_vec(),
    }
}

#[test]
fn claim_posts_exact_path_without_body_and_decodes_fixture() {
    let transport = Arc::new(RecordingTransport::with(vec![Scripted::Response(
        response(200, Some("application/json"), CHECK_WORK_FIXTURE),
    )]));
    let client = ControlPlaneClient::from_transport(Arc::clone(&transport));
    let shutdown = ShutdownToken::new();

    let work = client
        .claim(&shutdown)
        .expect("claim should succeed")
        .expect("work should exist");

    assert_eq!(work.id(), "018f22d3-1d6a-7cc0-a37b-46fc3fafd101");
    assert_eq!(
        work.monitor_id(),
        Some("018f22d3-1d6a-7cc0-a37b-46fc3fafd001")
    );
    assert_eq!(work.target(), "https://example.com/health?region=eu");
    assert_eq!(work.timeout_ms(), 10_000);
    assert_eq!(work.max_redirects(), 3);

    let requests = transport.requests();
    assert_eq!(requests.len(), 1);
    let (request, timeout) = &requests[0];
    assert_eq!(request.method, "POST");
    assert_eq!(request.path, "/internal/checks/claim");
    assert!(request.body.is_empty());
    assert!(
        !request
            .headers
            .iter()
            .any(|(name, _)| name.eq_ignore_ascii_case("content-type"))
    );
    assert_eq!(*timeout, Duration::from_secs(5));
}

#[test]
fn claim_204_is_no_work_and_success_shape_is_strict() {
    let no_work = Arc::new(RecordingTransport::with(vec![Scripted::Response(
        response(204, None, ""),
    )]));
    let client = ControlPlaneClient::from_transport(no_work);
    assert_eq!(
        client.claim(&ShutdownToken::new()).expect("204 is valid"),
        None
    );

    for body in [
        r#"{"checkId":"018f22d3-1d6a-7cc0-a37b-46fc3fafd101"}"#,
        r#"{"checkId":"018f22d3-1d6a-7cc0-a37b-46fc3fafd101","monitorId":"018f22d3-1d6a-7cc0-a37b-46fc3fafd001","targetUrl":"https://example.com","timeoutMs":10000,"maxRedirects":3,"extra":true}"#,
    ] {
        let transport = Arc::new(RecordingTransport::with(vec![Scripted::Response(
            response(200, Some("application/json"), body),
        )]));
        let client = ControlPlaneClient::from_transport(transport);
        assert_eq!(
            client.claim(&ShutdownToken::new()),
            Err(ClaimError::InvalidResponse)
        );
    }

    let wrong_media = Arc::new(RecordingTransport::with(vec![Scripted::Response(
        response(200, Some("text/plain"), CHECK_WORK_FIXTURE),
    )]));
    let client = ControlPlaneClient::from_transport(wrong_media);
    assert_eq!(
        client.claim(&ShutdownToken::new()),
        Err(ClaimError::InvalidResponse)
    );
}

#[test]
fn claim_maps_definite_rejection_transport_ambiguity_deadline_and_cancellation() {
    for (scripted, expected) in [
        (
            Scripted::Response(response(500, Some("application/problem+json"), "secret")),
            ClaimError::Rejected,
        ),
        (
            Scripted::Error(TransportError::BeforeSend),
            ClaimError::Transport,
        ),
        (
            Scripted::Error(TransportError::AfterSend),
            ClaimError::AmbiguousTransport,
        ),
        (
            Scripted::Error(TransportError::Deadline),
            ClaimError::Deadline,
        ),
        (
            Scripted::Error(TransportError::Cancelled),
            ClaimError::Cancelled,
        ),
        (
            Scripted::Error(TransportError::Protocol),
            ClaimError::InvalidResponse,
        ),
    ] {
        let transport = Arc::new(RecordingTransport::with(vec![scripted]));
        let client = ControlPlaneClient::from_transport(transport);
        assert_eq!(client.claim(&ShutdownToken::new()), Err(expected));
    }
}

#[test]
fn canonical_result_fixtures_prepare_exact_retry_bytes() {
    let transport = Arc::new(RecordingTransport::with(vec![
        Scripted::Response(response(204, None, "")),
        Scripted::Response(response(204, None, "")),
    ]));
    let client = ControlPlaneClient::from_transport(Arc::clone(&transport));
    let shutdown = ShutdownToken::new();

    let prepared = client
        .prepare_result(
            "018f22d3-1d6a-7cc0-a37b-46fc3fafd101",
            &ProbeOutcome::HttpResponse {
                duration_ms: 125,
                status: 204,
            },
        )
        .expect("HTTP result should serialize");
    assert_eq!(prepared.body(), HTTP_RESULT_FIXTURE.trim_end().as_bytes());

    client
        .submit_prepared(&prepared, &shutdown)
        .expect("first submit should succeed");
    client
        .submit_prepared(&prepared, &shutdown)
        .expect("same prepared payload should be reusable");

    let requests = transport.requests();
    assert_eq!(requests.len(), 2);
    for (request, timeout) in &requests {
        assert_eq!(request.method, "PUT");
        assert_eq!(
            request.path,
            "/internal/checks/018f22d3-1d6a-7cc0-a37b-46fc3fafd101/result"
        );
        assert_eq!(request.body, prepared.body());
        assert_eq!(*timeout, Duration::from_secs(5));
        assert_eq!(
            request
                .headers
                .iter()
                .find(|(name, _)| name.eq_ignore_ascii_case("content-type"))
                .map(|(_, value)| value.as_str()),
            Some("application/json")
        );
    }

    let failure = client
        .prepare_result(
            "018f22d3-1d6a-7cc0-a37b-46fc3fafd101",
            &ProbeOutcome::Failure {
                duration_ms: 10_000,
                category: "timeout".to_string(),
            },
        )
        .expect("failure should serialize");
    assert_eq!(failure.body(), FAILURE_RESULT_FIXTURE.trim_end().as_bytes());
}

fn timeout_prepared<T>(client: &ControlPlaneClient<T>) -> PreparedResult
where
    T: Transport,
{
    client
        .prepare_result(
            "018f22d3-1d6a-7cc0-a37b-46fc3fafd101",
            &ProbeOutcome::Failure {
                duration_ms: 10_000,
                category: "timeout".to_string(),
            },
        )
        .expect("timeout result should serialize")
}

#[test]
fn result_http_statuses_and_transport_failures_are_stable() {
    for status in [400_u16, 404, 409, 415, 422, 500] {
        let transport = Arc::new(RecordingTransport::with(vec![Scripted::Response(
            response(status, Some("application/problem+json"), "raw secret"),
        )]));
        let client = ControlPlaneClient::from_transport(transport);
        let prepared = timeout_prepared(&client);
        assert_eq!(
            client.submit_prepared(&prepared, &ShutdownToken::new()),
            Err(SubmitError::Rejected)
        );
    }

    for (error, expected) in [
        (TransportError::BeforeSend, SubmitError::Transport),
        (TransportError::AfterSend, SubmitError::Transport),
        (TransportError::Deadline, SubmitError::Deadline),
        (TransportError::Cancelled, SubmitError::Cancelled),
        (TransportError::Protocol, SubmitError::InvalidResponse),
    ] {
        let transport = Arc::new(RecordingTransport::with(vec![Scripted::Error(error)]));
        let client = ControlPlaneClient::from_transport(transport);
        let prepared = timeout_prepared(&client);
        assert_eq!(
            client.submit_prepared(&prepared, &ShutdownToken::new()),
            Err(expected)
        );
    }
}

#[test]
fn invalid_probe_outcome_is_rejected_before_transport() {
    let transport = Arc::new(RecordingTransport::default());
    let client = ControlPlaneClient::from_transport(Arc::clone(&transport));

    assert_eq!(
        client.prepare_result(
            "018f22d3-1d6a-7cc0-a37b-46fc3fafd101",
            &ProbeOutcome::Failure {
                duration_ms: 1,
                category: "worker_timeout".to_string(),
            },
        ),
        Err(SubmitError::InvalidResult)
    );
    assert_eq!(
        client.prepare_result(
            "not-a-uuid",
            &ProbeOutcome::Failure {
                duration_ms: 1,
                category: "timeout".to_string(),
            },
        ),
        Err(SubmitError::InvalidResult)
    );
    assert!(transport.requests().is_empty());
}

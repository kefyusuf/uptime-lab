use std::{
    collections::{HashMap, VecDeque},
    io::{self, Cursor, Read, Write},
    net::{IpAddr, Ipv4Addr, SocketAddr, TcpListener},
    sync::{Arc, Mutex},
    thread,
    time::Duration,
};

use checker_core::{Probe, ProbeOutcome, WorkItem};
use rustls::{
    ClientConfig, RootCertStore, ServerConfig, ServerConnection, StreamOwned,
    pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer},
};

use crate::{
    DestinationError, DestinationPolicy, ResolveError, Resolver, ValidatedDestination,
    probe::{
        Connector, DirectConnector, HopPolicy, ManagedStream, ProbeEngine, ProbeFailureKind,
        ProbeResult,
    },
};

const CA_CERT_DER: &[u8] = include_bytes!("../tests/fixtures/ca.der");
const SERVER_CERT_DER: &[u8] = include_bytes!("../tests/fixtures/server.der");
const SERVER_KEY_DER: &[u8] = include_bytes!("../tests/fixtures/server-key.der");

#[derive(Default)]
struct FixtureResolver {
    answers: HashMap<String, Vec<IpAddr>>,
    calls: Arc<Mutex<Vec<String>>>,
}

impl FixtureResolver {
    fn with_answer(mut self, host: &str, addresses: Vec<IpAddr>) -> Self {
        self.answers.insert(host.to_string(), addresses);
        self
    }

    fn calls(&self) -> Arc<Mutex<Vec<String>>> {
        Arc::clone(&self.calls)
    }
}

impl Resolver for FixtureResolver {
    fn resolve(&self, host: &str) -> Result<Vec<IpAddr>, ResolveError> {
        self.calls
            .lock()
            .expect("resolver calls")
            .push(host.to_string());
        self.answers.get(host).cloned().ok_or(ResolveError)
    }
}

struct TestPolicy<R> {
    resolver: R,
}

impl<R> TestPolicy<R> {
    const fn new(resolver: R) -> Self {
        Self { resolver }
    }
}

impl<R> HopPolicy for TestPolicy<R>
where
    R: Resolver,
{
    fn validate(&self, target: &str) -> Result<ValidatedDestination, DestinationError> {
        ValidatedDestination::test_validate(target, &self.resolver)
    }

    fn validate_redirect(
        &self,
        current: &ValidatedDestination,
        location: &str,
        redirects_followed: u8,
    ) -> Result<ValidatedDestination, DestinationError> {
        current.test_redirect(location, redirects_followed, &self.resolver)
    }
}

enum Behavior {
    Fail(io::ErrorKind),
    Stream(Vec<u8>, Arc<Mutex<usize>>, Arc<Mutex<Vec<u8>>>),
    Timeout,
}

struct RecordingConnector {
    attempts: Arc<Mutex<Vec<SocketAddr>>>,
    behaviors: Mutex<VecDeque<Behavior>>,
}

impl RecordingConnector {
    fn new(behaviors: Vec<Behavior>) -> Self {
        Self {
            attempts: Arc::new(Mutex::new(Vec::new())),
            behaviors: Mutex::new(behaviors.into()),
        }
    }

    fn attempts(&self) -> Arc<Mutex<Vec<SocketAddr>>> {
        Arc::clone(&self.attempts)
    }
}

impl Connector for RecordingConnector {
    fn connect(
        &self,
        address: SocketAddr,
        _remaining: Duration,
    ) -> io::Result<Box<dyn ManagedStream>> {
        self.attempts.lock().expect("attempts").push(address);
        match self
            .behaviors
            .lock()
            .expect("behaviors")
            .pop_front()
            .expect("behavior")
        {
            Behavior::Fail(kind) => Err(io::Error::new(kind, "fixture connect failure")),
            Behavior::Stream(bytes, reads, writes) => Ok(Box::new(ScriptedStream {
                cursor: Cursor::new(bytes),
                reads,
                writes,
            })),
            Behavior::Timeout => Ok(Box::new(TimeoutStream)),
        }
    }
}

struct ScriptedStream {
    cursor: Cursor<Vec<u8>>,
    reads: Arc<Mutex<usize>>,
    writes: Arc<Mutex<Vec<u8>>>,
}

impl Read for ScriptedStream {
    fn read(&mut self, buffer: &mut [u8]) -> io::Result<usize> {
        let read = self.cursor.read(buffer)?;
        *self.reads.lock().expect("reads") += read;
        Ok(read)
    }
}

impl Write for ScriptedStream {
    fn write(&mut self, buffer: &[u8]) -> io::Result<usize> {
        self.writes
            .lock()
            .expect("writes")
            .extend_from_slice(buffer);
        Ok(buffer.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

impl ManagedStream for ScriptedStream {
    fn set_timeout(&self, _remaining: Duration) -> io::Result<()> {
        Ok(())
    }
}

struct TimeoutStream;

impl Read for TimeoutStream {
    fn read(&mut self, _buffer: &mut [u8]) -> io::Result<usize> {
        Err(io::Error::new(io::ErrorKind::TimedOut, "fixture timeout"))
    }
}

impl Write for TimeoutStream {
    fn write(&mut self, buffer: &[u8]) -> io::Result<usize> {
        Ok(buffer.len())
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

impl ManagedStream for TimeoutStream {
    fn set_timeout(&self, _remaining: Duration) -> io::Result<()> {
        Ok(())
    }
}

fn test_client_config() -> Arc<ClientConfig> {
    let mut roots = RootCertStore::empty();
    roots
        .add(CertificateDer::from(CA_CERT_DER.to_vec()))
        .expect("add test CA");
    Arc::new(
        ClientConfig::builder()
            .with_root_certificates(roots)
            .with_no_client_auth(),
    )
}

fn fixture_engine<P, C>(policy: P, connector: C) -> ProbeEngine<P, C>
where
    P: HopPolicy,
    C: Connector,
{
    ProbeEngine::new(
        policy,
        connector,
        test_client_config(),
        Duration::from_secs(1),
    )
}

fn response(status: u16, headers: &[(&str, &str)], body: &[u8]) -> Vec<u8> {
    let mut value = format!("HTTP/1.1 {status} Fixture\r\n");
    for (name, header_value) in headers {
        value.push_str(name);
        value.push_str(": ");
        value.push_str(header_value);
        value.push_str("\r\n");
    }
    value.push_str("\r\n");
    let mut bytes = value.into_bytes();
    bytes.extend_from_slice(body);
    bytes
}

#[test]
fn serial_fallback_uses_only_validated_addresses_without_reresolution() {
    let resolver = FixtureResolver::default().with_answer(
        "fixture.test",
        vec![
            IpAddr::V4(Ipv4Addr::new(127, 0, 0, 2)),
            IpAddr::V4(Ipv4Addr::LOCALHOST),
        ],
    );
    let calls = resolver.calls();
    let connector = RecordingConnector::new(vec![
        Behavior::Fail(io::ErrorKind::ConnectionRefused),
        Behavior::Stream(
            response(204, &[], b""),
            Arc::new(Mutex::new(0)),
            Arc::new(Mutex::new(Vec::new())),
        ),
    ]);
    let attempts = connector.attempts();
    let engine = fixture_engine(TestPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://fixture.test:18080/"),
        ProbeResult::HttpResponse { status: 204, .. }
    ));
    assert_eq!(calls.lock().expect("calls").as_slice(), ["fixture.test"]);
    assert_eq!(
        attempts.lock().expect("attempts").as_slice(),
        [
            SocketAddr::new(IpAddr::V4(Ipv4Addr::new(127, 0, 0, 2)), 18080),
            SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 18080),
        ]
    );
}

#[test]
fn exhausted_validated_addresses_normalize_to_connect_error() {
    let resolver = FixtureResolver::default().with_answer(
        "fixture.test",
        vec![
            IpAddr::V4(Ipv4Addr::new(127, 0, 0, 2)),
            IpAddr::V4(Ipv4Addr::LOCALHOST),
        ],
    );
    let connector = RecordingConnector::new(vec![
        Behavior::Fail(io::ErrorKind::ConnectionRefused),
        Behavior::Fail(io::ErrorKind::ConnectionRefused),
    ]);
    let attempts = connector.attempts();
    let engine = fixture_engine(TestPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://fixture.test:18080/"),
        ProbeResult::Failure {
            kind: ProbeFailureKind::ConnectError,
            ..
        }
    ));
    assert_eq!(attempts.lock().expect("attempts").len(), 2);
}

#[test]
fn resolver_failure_normalizes_to_dns_error_without_connecting() {
    let resolver = FixtureResolver::default();
    let connector = RecordingConnector::new(vec![]);
    let attempts = connector.attempts();
    let engine = fixture_engine(DestinationPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("https://missing.example/"),
        ProbeResult::Failure {
            kind: ProbeFailureKind::DnsError,
            ..
        }
    ));
    assert!(attempts.lock().expect("attempts").is_empty());
}

#[test]
fn post_connect_protocol_failure_stops_fallback() {
    let resolver = FixtureResolver::default().with_answer(
        "fixture.test",
        vec![
            IpAddr::V4(Ipv4Addr::LOCALHOST),
            IpAddr::V4(Ipv4Addr::new(127, 0, 0, 2)),
        ],
    );
    let connector = RecordingConnector::new(vec![
        Behavior::Stream(
            b"not-http\r\n\r\n".to_vec(),
            Arc::new(Mutex::new(0)),
            Arc::new(Mutex::new(Vec::new())),
        ),
        Behavior::Stream(
            response(200, &[], b""),
            Arc::new(Mutex::new(0)),
            Arc::new(Mutex::new(Vec::new())),
        ),
    ]);
    let attempts = connector.attempts();
    let engine = fixture_engine(TestPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://fixture.test:18080/"),
        ProbeResult::Failure {
            kind: ProbeFailureKind::ProtocolError,
            ..
        }
    ));
    assert_eq!(attempts.lock().expect("attempts").len(), 1);
}

#[test]
fn final_response_uses_get_host_and_consumes_no_body_bytes() {
    let resolver = FixtureResolver::default()
        .with_answer("fixture.test", vec![IpAddr::V4(Ipv4Addr::LOCALHOST)]);
    let raw = response(200, &[("Content-Length", "4")], b"BODY");
    let header_len = raw
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .unwrap()
        + 4;
    let reads = Arc::new(Mutex::new(0));
    let writes = Arc::new(Mutex::new(Vec::new()));
    let connector = RecordingConnector::new(vec![Behavior::Stream(
        raw,
        Arc::clone(&reads),
        Arc::clone(&writes),
    )]);
    let engine = fixture_engine(TestPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://fixture.test:18080/health?region=eu"),
        ProbeResult::HttpResponse { status: 200, .. }
    ));
    assert_eq!(*reads.lock().expect("reads"), header_len);

    let request = String::from_utf8(writes.lock().expect("writes").clone()).expect("request UTF-8");
    assert!(request.starts_with("GET /health?region=eu HTTP/1.1\r\n"));
    assert!(request.contains("\r\nHost: fixture.test:18080\r\n"));
    assert!(request.contains("\r\nConnection: close\r\n"));
}

#[test]
fn oversized_headers_are_protocol_error() {
    let resolver = FixtureResolver::default()
        .with_answer("fixture.test", vec![IpAddr::V4(Ipv4Addr::LOCALHOST)]);
    let oversized = format!(
        "HTTP/1.1 200 OK\r\nX-Large: {}\r\n\r\n",
        "a".repeat(70 * 1024)
    );
    let connector = RecordingConnector::new(vec![Behavior::Stream(
        oversized.into_bytes(),
        Arc::new(Mutex::new(0)),
        Arc::new(Mutex::new(Vec::new())),
    )]);
    let engine = fixture_engine(TestPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://fixture.test:18080/"),
        ProbeResult::Failure {
            kind: ProbeFailureKind::ProtocolError,
            ..
        }
    ));
}

#[test]
fn read_timeout_is_normalized() {
    let resolver = FixtureResolver::default()
        .with_answer("fixture.test", vec![IpAddr::V4(Ipv4Addr::LOCALHOST)]);
    let connector = RecordingConnector::new(vec![Behavior::Timeout]);
    let engine = fixture_engine(TestPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://fixture.test:18080/"),
        ProbeResult::Failure {
            kind: ProbeFailureKind::Timeout,
            ..
        }
    ));
}

#[test]
fn relative_redirect_executes_a_second_validated_hop() {
    let resolver = FixtureResolver::default()
        .with_answer("fixture.test", vec![IpAddr::V4(Ipv4Addr::LOCALHOST)]);
    let calls = resolver.calls();
    let connector = RecordingConnector::new(vec![
        Behavior::Stream(
            response(302, &[("Location", "../next?x=1")], b""),
            Arc::new(Mutex::new(0)),
            Arc::new(Mutex::new(Vec::new())),
        ),
        Behavior::Stream(
            response(204, &[], b""),
            Arc::new(Mutex::new(0)),
            Arc::new(Mutex::new(Vec::new())),
        ),
    ]);
    let attempts = connector.attempts();
    let engine = fixture_engine(TestPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://fixture.test:18080/a/b"),
        ProbeResult::HttpResponse { status: 204, .. }
    ));
    assert_eq!(
        calls.lock().expect("calls").as_slice(),
        ["fixture.test", "fixture.test"]
    );
    assert_eq!(attempts.lock().expect("attempts").len(), 2);
}

#[test]
fn non_default_port_redirect_is_rejected_before_second_connect() {
    let resolver = FixtureResolver::default()
        .with_answer("public.test", vec![IpAddr::V4(Ipv4Addr::new(8, 8, 8, 8))]);
    let connector = RecordingConnector::new(vec![Behavior::Stream(
        response(302, &[("Location", "http://public.test:8080/next")], b""),
        Arc::new(Mutex::new(0)),
        Arc::new(Mutex::new(Vec::new())),
    )]);
    let attempts = connector.attempts();
    let engine = fixture_engine(DestinationPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://public.test/start"),
        ProbeResult::Failure {
            kind: ProbeFailureKind::PolicyRejected,
            ..
        }
    ));
    assert_eq!(attempts.lock().expect("attempts").len(), 1);
}

#[test]
fn redirect_is_revalidated_and_private_redirect_is_rejected_before_connect() {
    let resolver = FixtureResolver::default()
        .with_answer("public.test", vec![IpAddr::V4(Ipv4Addr::new(8, 8, 8, 8))])
        .with_answer("private.test", vec![IpAddr::V4(Ipv4Addr::new(10, 0, 0, 1))]);
    let calls = resolver.calls();
    let connector = RecordingConnector::new(vec![Behavior::Stream(
        response(302, &[("Location", "http://private.test/")], b""),
        Arc::new(Mutex::new(0)),
        Arc::new(Mutex::new(Vec::new())),
    )]);
    let attempts = connector.attempts();
    let engine = fixture_engine(DestinationPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://public.test/"),
        ProbeResult::Failure {
            kind: ProbeFailureKind::PolicyRejected,
            ..
        }
    ));
    assert_eq!(
        calls.lock().expect("calls").as_slice(),
        ["public.test", "private.test"]
    );
    assert_eq!(attempts.lock().expect("attempts").len(), 1);
}

#[test]
fn redirect_limit_is_enforced_by_execution_loop() {
    let resolver = FixtureResolver::default()
        .with_answer("fixture.test", vec![IpAddr::V4(Ipv4Addr::LOCALHOST)]);
    let connector = RecordingConnector::new(
        (0..4)
            .map(|_| {
                Behavior::Stream(
                    response(302, &[("Location", "/next")], b""),
                    Arc::new(Mutex::new(0)),
                    Arc::new(Mutex::new(Vec::new())),
                )
            })
            .collect(),
    );
    let attempts = connector.attempts();
    let engine = fixture_engine(TestPolicy::new(resolver), connector);

    assert!(matches!(
        engine.execute("http://fixture.test:18080/start"),
        ProbeResult::Failure {
            kind: ProbeFailureKind::PolicyRejected,
            ..
        }
    ));
    assert_eq!(attempts.lock().expect("attempts").len(), 4);
}

#[test]
fn invalid_work_budget_normalizes_to_internal_error_without_resolution() {
    let resolver = FixtureResolver::default();
    let calls = resolver.calls();
    let probe = crate::HttpProbe::with_resolver(resolver);
    let work = WorkItem::new(
        "018f22d3-1d6a-7cc0-a37b-46fc3fafd101",
        "https://fixture.test/",
        9_999,
        3,
    );

    let outcome = Probe::probe(&probe, &work).expect("probe port should return normalized outcome");
    assert_eq!(
        outcome,
        ProbeOutcome::Failure {
            duration_ms: 0,
            category: ProbeFailureKind::InternalError.as_str().to_string(),
        }
    );
    assert!(
        calls.lock().expect("resolver calls").is_empty(),
        "invalid work policy must fail before DNS/network execution"
    );
}

#[test]
fn https_redirect_to_http_is_policy_rejected_before_reresolution() {
    let (port, server) = spawn_tls_server(response(
        302,
        &[("Location", "http://fixture.test:18080/down")],
        b"",
    ));
    let resolver = FixtureResolver::default()
        .with_answer("fixture.test", vec![IpAddr::V4(Ipv4Addr::LOCALHOST)]);
    let calls = resolver.calls();
    let engine = ProbeEngine::new(
        TestPolicy::new(resolver),
        DirectConnector,
        test_client_config(),
        Duration::from_secs(2),
    );

    assert!(matches!(
        engine.execute(&format!("https://fixture.test:{port}/start")),
        ProbeResult::Failure {
            kind: ProbeFailureKind::PolicyRejected,
            ..
        }
    ));
    server.join().expect("TLS fixture thread");
    assert_eq!(
        calls.lock().expect("resolver calls").as_slice(),
        ["fixture.test"],
        "HTTPS downgrade must be rejected before redirect DNS/connect"
    );
}

#[test]
fn real_http_fixture_receives_one_get() {
    let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).expect("bind fixture");
    let port = listener.local_addr().expect("fixture address").port();
    let request = Arc::new(Mutex::new(String::new()));
    let captured = Arc::clone(&request);

    let server = thread::spawn(move || {
        let (mut socket, _) = listener.accept().expect("accept HTTP fixture");
        let head = read_request_head(&mut socket);
        *captured.lock().expect("captured request") = head;
        socket
            .write_all(&response(201, &[("Content-Length", "4")], b"BODY"))
            .expect("write HTTP fixture response");
    });

    let resolver = FixtureResolver::default()
        .with_answer("fixture.test", vec![IpAddr::V4(Ipv4Addr::LOCALHOST)]);
    let engine = ProbeEngine::new(
        TestPolicy::new(resolver),
        DirectConnector,
        test_client_config(),
        Duration::from_secs(1),
    );

    assert!(matches!(
        engine.execute(&format!("http://fixture.test:{port}/real")),
        ProbeResult::HttpResponse { status: 201, .. }
    ));
    server.join().expect("HTTP fixture thread");

    let request = request.lock().expect("captured request");
    assert!(request.starts_with("GET /real HTTP/1.1\r\n"));
    assert!(request.contains(&format!("\r\nHost: fixture.test:{port}\r\n")));
}

#[test]
fn real_tls_fixture_verifies_original_hostname() {
    let (port, server) = spawn_tls_server(response(204, &[], b""));
    let resolver = FixtureResolver::default()
        .with_answer("fixture.test", vec![IpAddr::V4(Ipv4Addr::LOCALHOST)]);
    let engine = ProbeEngine::new(
        TestPolicy::new(resolver),
        DirectConnector,
        test_client_config(),
        Duration::from_secs(2),
    );

    assert!(matches!(
        engine.execute(&format!("https://fixture.test:{port}/")),
        ProbeResult::HttpResponse { status: 204, .. }
    ));
    server.join().expect("TLS fixture thread");
}

#[test]
fn tls_hostname_mismatch_is_tls_error_and_does_not_fallback() {
    let (port, server) = spawn_tls_server(response(200, &[], b""));
    let resolver =
        FixtureResolver::default().with_answer("wrong.test", vec![IpAddr::V4(Ipv4Addr::LOCALHOST)]);
    let engine = ProbeEngine::new(
        TestPolicy::new(resolver),
        DirectConnector,
        test_client_config(),
        Duration::from_secs(2),
    );

    assert!(matches!(
        engine.execute(&format!("https://wrong.test:{port}/")),
        ProbeResult::Failure {
            kind: ProbeFailureKind::TlsError,
            ..
        }
    ));
    server.join().expect("TLS fixture thread");
}

fn read_request_head(stream: &mut impl Read) -> String {
    let mut bytes = Vec::new();
    let mut byte = [0_u8; 1];
    while !bytes.ends_with(b"\r\n\r\n") {
        let read = stream.read(&mut byte).expect("read request");
        if read == 0 {
            break;
        }
        bytes.push(byte[0]);
        assert!(bytes.len() < 64 * 1024);
    }
    String::from_utf8(bytes).expect("request head UTF-8")
}

fn spawn_tls_server(response_bytes: Vec<u8>) -> (u16, thread::JoinHandle<()>) {
    let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).expect("bind TLS fixture");
    let port = listener.local_addr().expect("TLS fixture address").port();

    let certificate = CertificateDer::from(SERVER_CERT_DER.to_vec());
    let key = PrivateKeyDer::Pkcs8(PrivatePkcs8KeyDer::from(SERVER_KEY_DER.to_vec()));
    let config = Arc::new(
        ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(vec![certificate], key)
            .expect("TLS server config"),
    );

    let server = thread::spawn(move || {
        let (socket, _) = listener.accept().expect("accept TLS fixture");
        let connection = ServerConnection::new(config).expect("TLS server connection");
        let mut stream = StreamOwned::new(connection, socket);

        if std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            read_request_head(&mut stream)
        }))
        .is_ok()
        {
            let _ = stream.write_all(&response_bytes);
            let _ = stream.flush();
        }
    });

    (port, server)
}

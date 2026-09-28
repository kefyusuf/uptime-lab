use std::{
    io::{self, Read, Write},
    net::{IpAddr, SocketAddr, TcpStream, ToSocketAddrs},
    sync::Arc,
    time::{Duration, Instant},
};

use checker_core::{BoxError, Probe, ProbeOutcome, WorkItem};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned, pki_types::ServerName};

use crate::{DestinationError, DestinationPolicy, ResolveError, Resolver, ValidatedDestination};

const OVERALL_TIMEOUT: Duration = Duration::from_secs(10);
const MAX_HEADER_BYTES: usize = 64 * 1024;
const EXPECTED_TIMEOUT_MS: u64 = 10_000;
const EXPECTED_MAX_REDIRECTS: u8 = 3;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ProbeFailureKind {
    DnsError,
    PolicyRejected,
    Timeout,
    ConnectError,
    TlsError,
    ProtocolError,
    InternalError,
}

impl ProbeFailureKind {
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::DnsError => "dns_error",
            Self::PolicyRejected => "policy_rejected",
            Self::Timeout => "timeout",
            Self::ConnectError => "connect_error",
            Self::TlsError => "tls_error",
            Self::ProtocolError => "protocol_error",
            Self::InternalError => "internal_error",
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ProbeResult {
    HttpResponse {
        duration_ms: u64,
        status: u16,
    },
    Failure {
        duration_ms: u64,
        kind: ProbeFailureKind,
    },
}

#[derive(Clone, Copy, Debug, Default)]
pub struct SystemResolver;

impl Resolver for SystemResolver {
    fn resolve(&self, host: &str) -> Result<Vec<IpAddr>, ResolveError> {
        (host, 0)
            .to_socket_addrs()
            .map(|addresses| addresses.map(|address| address.ip()).collect())
            .map_err(|_| ResolveError)
    }
}

pub struct HttpProbe<R = SystemResolver>
where
    R: Resolver,
{
    engine: ProbeEngine<DestinationPolicy<R>, DirectConnector>,
}

impl HttpProbe<SystemResolver> {
    pub fn new() -> Self {
        Self::with_resolver(SystemResolver)
    }
}

impl Default for HttpProbe<SystemResolver> {
    fn default() -> Self {
        Self::new()
    }
}

impl<R> HttpProbe<R>
where
    R: Resolver,
{
    pub fn with_resolver(resolver: R) -> Self {
        Self {
            engine: ProbeEngine::new(
                DestinationPolicy::new(resolver),
                DirectConnector,
                production_tls_config(),
                OVERALL_TIMEOUT,
            ),
        }
    }

    pub fn execute(&self, target: &str) -> ProbeResult {
        self.engine.execute(target)
    }
}

impl<R> Probe for HttpProbe<R>
where
    R: Resolver,
{
    fn probe(&self, work: &WorkItem) -> Result<ProbeOutcome, BoxError> {
        let result = if work.timeout_ms() != EXPECTED_TIMEOUT_MS
            || work.max_redirects() != EXPECTED_MAX_REDIRECTS
        {
            ProbeResult::Failure {
                duration_ms: 0,
                kind: ProbeFailureKind::InternalError,
            }
        } else {
            self.execute(work.target())
        };

        Ok(match result {
            ProbeResult::HttpResponse {
                duration_ms,
                status,
            } => ProbeOutcome::HttpResponse {
                duration_ms,
                status,
            },
            ProbeResult::Failure { duration_ms, kind } => ProbeOutcome::Failure {
                duration_ms,
                category: kind.as_str().to_string(),
            },
        })
    }
}

pub(crate) trait HopPolicy {
    fn validate(&self, target: &str) -> Result<ValidatedDestination, DestinationError>;

    fn validate_redirect(
        &self,
        current: &ValidatedDestination,
        location: &str,
        redirects_followed: u8,
    ) -> Result<ValidatedDestination, DestinationError>;
}

impl<R> HopPolicy for DestinationPolicy<R>
where
    R: Resolver,
{
    fn validate(&self, target: &str) -> Result<ValidatedDestination, DestinationError> {
        DestinationPolicy::validate(self, target)
    }

    fn validate_redirect(
        &self,
        current: &ValidatedDestination,
        location: &str,
        redirects_followed: u8,
    ) -> Result<ValidatedDestination, DestinationError> {
        DestinationPolicy::validate_redirect(self, current, location, redirects_followed)
    }
}

pub(crate) trait ManagedStream: Read + Write + Send {
    fn set_timeout(&self, remaining: Duration) -> io::Result<()>;
}

impl ManagedStream for TcpStream {
    fn set_timeout(&self, remaining: Duration) -> io::Result<()> {
        self.set_read_timeout(Some(remaining))?;
        self.set_write_timeout(Some(remaining))
    }
}

pub(crate) trait Connector {
    fn connect(
        &self,
        address: SocketAddr,
        remaining: Duration,
    ) -> io::Result<Box<dyn ManagedStream>>;
}

#[derive(Clone, Copy, Debug, Default)]
pub(crate) struct DirectConnector;

impl Connector for DirectConnector {
    fn connect(
        &self,
        address: SocketAddr,
        remaining: Duration,
    ) -> io::Result<Box<dyn ManagedStream>> {
        let stream = TcpStream::connect_timeout(&address, remaining)?;
        stream.set_nodelay(true)?;
        stream.set_read_timeout(Some(remaining))?;
        stream.set_write_timeout(Some(remaining))?;
        Ok(Box::new(stream))
    }
}

pub(crate) struct ProbeEngine<P, C> {
    policy: P,
    connector: C,
    tls_config: Arc<ClientConfig>,
    timeout: Duration,
}

impl<P, C> ProbeEngine<P, C>
where
    P: HopPolicy,
    C: Connector,
{
    pub(crate) const fn new(
        policy: P,
        connector: C,
        tls_config: Arc<ClientConfig>,
        timeout: Duration,
    ) -> Self {
        Self {
            policy,
            connector,
            tls_config,
            timeout,
        }
    }

    pub(crate) fn execute(&self, target: &str) -> ProbeResult {
        let started = Instant::now();
        let deadline = started + self.timeout;

        let mut destination = match self.policy.validate(target) {
            Ok(destination) => destination,
            Err(error) => return destination_failure(started, error),
        };
        let mut redirects_followed = 0_u8;

        loop {
            let head = match self.execute_hop(&destination, deadline) {
                Ok(head) => head,
                Err(error) => return hop_failure(started, error),
            };

            if is_redirect_status(head.status)
                && let Some(location) = head.location.as_deref()
            {
                destination =
                    match self
                        .policy
                        .validate_redirect(&destination, location, redirects_followed)
                    {
                        Ok(destination) => destination,
                        Err(error) => return destination_failure(started, error),
                    };
                redirects_followed = redirects_followed.saturating_add(1);
                continue;
            }

            return ProbeResult::HttpResponse {
                duration_ms: elapsed_ms(started),
                status: head.status,
            };
        }
    }

    fn execute_hop(
        &self,
        destination: &ValidatedDestination,
        deadline: Instant,
    ) -> Result<ResponseHead, HopError> {
        let stream = self.connect_serial(destination, deadline)?;

        match destination.scheme() {
            "http" => send_request_and_read_head(stream, destination, deadline),
            "https" => {
                let server_name = ServerName::try_from(destination.server_name())
                    .map_err(|_| HopError::Internal)?;
                let mut connection =
                    ClientConnection::new(Arc::clone(&self.tls_config), server_name)
                        .map_err(|_| HopError::Internal)?;
                let mut stream = stream;

                while connection.is_handshaking() {
                    connection
                        .complete_io(&mut stream)
                        .map_err(|error| classify_tls_io(error, deadline))?;
                }

                let tls_stream = StreamOwned::new(connection, stream);
                send_request_and_read_head(tls_stream, destination, deadline)
            }
            _ => Err(HopError::Internal),
        }
    }

    fn connect_serial(
        &self,
        destination: &ValidatedDestination,
        deadline: Instant,
    ) -> Result<DeadlineStream, HopError> {
        let mut saw_timeout = false;

        for address in destination.addresses() {
            let remaining = remaining(deadline).map_err(|_| HopError::Timeout)?;
            let socket = SocketAddr::new(*address, destination.port());

            match self.connector.connect(socket, remaining) {
                Ok(stream) => return Ok(DeadlineStream::new(stream, deadline)),
                Err(error) if is_timeout_error(&error) => {
                    saw_timeout = true;
                    if Instant::now() >= deadline {
                        return Err(HopError::Timeout);
                    }
                }
                Err(_) => {}
            }
        }

        if saw_timeout && Instant::now() >= deadline {
            Err(HopError::Timeout)
        } else {
            Err(HopError::Connect)
        }
    }
}

struct DeadlineStream {
    inner: Box<dyn ManagedStream>,
    deadline: Instant,
}

impl DeadlineStream {
    const fn new(inner: Box<dyn ManagedStream>, deadline: Instant) -> Self {
        Self { inner, deadline }
    }

    fn prepare(&self) -> io::Result<()> {
        self.inner.set_timeout(remaining(self.deadline)?)
    }
}

impl Read for DeadlineStream {
    fn read(&mut self, buffer: &mut [u8]) -> io::Result<usize> {
        self.prepare()?;
        self.inner.read(buffer)
    }
}

impl Write for DeadlineStream {
    fn write(&mut self, buffer: &[u8]) -> io::Result<usize> {
        self.prepare()?;
        self.inner.write(buffer)
    }

    fn flush(&mut self) -> io::Result<()> {
        self.prepare()?;
        self.inner.flush()
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ResponseHead {
    status: u16,
    location: Option<String>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum HopError {
    Timeout,
    Connect,
    Tls,
    Protocol,
    Internal,
}

fn send_request_and_read_head(
    mut stream: impl Read + Write,
    destination: &ValidatedDestination,
    deadline: Instant,
) -> Result<ResponseHead, HopError> {
    let request = format!(
        "GET {} HTTP/1.1\r\nHost: {}\r\nConnection: close\r\nAccept: */*\r\nUser-Agent: uptime-lab-checker/0.1\r\n\r\n",
        destination.request_target(),
        destination.authority(),
    );

    stream
        .write_all(request.as_bytes())
        .map_err(|error| classify_protocol_io(error, deadline))?;
    stream
        .flush()
        .map_err(|error| classify_protocol_io(error, deadline))?;

    read_response_head(&mut stream, deadline)
}

fn read_response_head(stream: &mut impl Read, deadline: Instant) -> Result<ResponseHead, HopError> {
    let mut bytes = Vec::with_capacity(1024);
    let mut byte = [0_u8; 1];

    loop {
        if bytes.len() >= MAX_HEADER_BYTES {
            return Err(HopError::Protocol);
        }

        remaining(deadline).map_err(|_| HopError::Timeout)?;
        let read = stream
            .read(&mut byte)
            .map_err(|error| classify_protocol_io(error, deadline))?;

        if read == 0 {
            return Err(HopError::Protocol);
        }

        bytes.push(byte[0]);
        if bytes.ends_with(b"\r\n\r\n") {
            return parse_response_head(&bytes);
        }
    }
}

fn parse_response_head(bytes: &[u8]) -> Result<ResponseHead, HopError> {
    let head = std::str::from_utf8(bytes).map_err(|_| HopError::Protocol)?;
    let mut lines = head.split("\r\n");
    let status_line = lines.next().ok_or(HopError::Protocol)?;
    let mut status_parts = status_line.split_whitespace();

    let version = status_parts.next().ok_or(HopError::Protocol)?;
    if version != "HTTP/1.0" && version != "HTTP/1.1" {
        return Err(HopError::Protocol);
    }

    let raw_status = status_parts.next().ok_or(HopError::Protocol)?;
    if raw_status.len() != 3 || !raw_status.bytes().all(|byte| byte.is_ascii_digit()) {
        return Err(HopError::Protocol);
    }
    let status = raw_status.parse::<u16>().map_err(|_| HopError::Protocol)?;
    if !(100..=599).contains(&status) {
        return Err(HopError::Protocol);
    }

    let mut location = None;
    for line in lines {
        if line.is_empty() {
            break;
        }
        if line.starts_with(' ') || line.starts_with('\t') {
            return Err(HopError::Protocol);
        }

        let (name, value) = line.split_once(':').ok_or(HopError::Protocol)?;
        if name.is_empty()
            || !name
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || byte == b'-')
        {
            return Err(HopError::Protocol);
        }

        if name.eq_ignore_ascii_case("location") {
            if location.is_some() {
                return Err(HopError::Protocol);
            }
            location = Some(value.trim_matches([' ', '\t']).to_string());
        }
    }

    Ok(ResponseHead { status, location })
}

fn production_tls_config() -> Arc<ClientConfig> {
    let roots = RootCertStore::from_iter(webpki_roots::TLS_SERVER_ROOTS.iter().cloned());
    Arc::new(
        ClientConfig::builder()
            .with_root_certificates(roots)
            .with_no_client_auth(),
    )
}

fn remaining(deadline: Instant) -> io::Result<Duration> {
    deadline
        .checked_duration_since(Instant::now())
        .filter(|remaining| !remaining.is_zero())
        .ok_or_else(|| io::Error::new(io::ErrorKind::TimedOut, "probe deadline elapsed"))
}

fn is_timeout_error(error: &io::Error) -> bool {
    matches!(
        error.kind(),
        io::ErrorKind::TimedOut | io::ErrorKind::WouldBlock
    )
}

fn classify_tls_io(error: io::Error, deadline: Instant) -> HopError {
    if is_timeout_error(&error) || Instant::now() >= deadline {
        HopError::Timeout
    } else {
        HopError::Tls
    }
}

fn classify_protocol_io(error: io::Error, deadline: Instant) -> HopError {
    if is_timeout_error(&error) || Instant::now() >= deadline {
        HopError::Timeout
    } else {
        HopError::Protocol
    }
}

fn destination_failure(started: Instant, error: DestinationError) -> ProbeResult {
    let kind = match error {
        DestinationError::DnsFailure => ProbeFailureKind::DnsError,
        DestinationError::PolicyRejected | DestinationError::InvalidUrl => {
            ProbeFailureKind::PolicyRejected
        }
    };
    ProbeResult::Failure {
        duration_ms: elapsed_ms(started),
        kind,
    }
}

fn hop_failure(started: Instant, error: HopError) -> ProbeResult {
    let kind = match error {
        HopError::Timeout => ProbeFailureKind::Timeout,
        HopError::Connect => ProbeFailureKind::ConnectError,
        HopError::Tls => ProbeFailureKind::TlsError,
        HopError::Protocol => ProbeFailureKind::ProtocolError,
        HopError::Internal => ProbeFailureKind::InternalError,
    };
    ProbeResult::Failure {
        duration_ms: elapsed_ms(started),
        kind,
    }
}

fn elapsed_ms(started: Instant) -> u64 {
    u64::try_from(started.elapsed().as_millis())
        .unwrap_or(u64::MAX)
        .min(20_000)
}

fn is_redirect_status(status: u16) -> bool {
    matches!(status, 301 | 302 | 303 | 307 | 308)
}

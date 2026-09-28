use std::{
    error::Error,
    fmt,
    io::{self, Read, Write},
    net::{SocketAddr, TcpStream, ToSocketAddrs},
    sync::Arc,
    time::{Duration, Instant},
};

use checker_core::ShutdownToken;

const IO_SLICE: Duration = Duration::from_millis(50);
const MAX_RESPONSE_HEAD: usize = 32 * 1024;
const MAX_RESPONSE_BODY: usize = 64 * 1024;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HttpRequest {
    pub method: &'static str,
    pub path: String,
    pub headers: Vec<(String, String)>,
    pub body: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HttpResponse {
    pub status: u16,
    pub headers: Vec<(String, String)>,
    pub body: Vec<u8>,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TransportError {
    BeforeSend,
    AfterSend,
    Deadline,
    Cancelled,
    Protocol,
}

pub trait Transport: Send + Sync {
    fn execute(
        &self,
        request: &HttpRequest,
        timeout: Duration,
        shutdown: &ShutdownToken,
    ) -> Result<HttpResponse, TransportError>;
}

impl<T> Transport for Arc<T>
where
    T: Transport + ?Sized,
{
    fn execute(
        &self,
        request: &HttpRequest,
        timeout: Duration,
        shutdown: &ShutdownToken,
    ) -> Result<HttpResponse, TransportError> {
        (**self).execute(request, timeout, shutdown)
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ClientConfigError {
    InvalidBaseUrl,
    Resolve,
}

impl fmt::Display for ClientConfigError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(match self {
            Self::InvalidBaseUrl => "invalid control-plane base URL",
            Self::Resolve => "control-plane address resolution failed",
        })
    }
}

impl Error for ClientConfigError {}

#[derive(Clone, Debug)]
pub struct StdHttpTransport {
    host: String,
    port: u16,
    host_header: String,
}

impl StdHttpTransport {
    pub fn connect(base_url: &str) -> Result<Self, ClientConfigError> {
        let authority = base_url
            .strip_prefix("http://")
            .ok_or(ClientConfigError::InvalidBaseUrl)?;

        let authority = authority.strip_suffix('/').unwrap_or(authority);
        if authority.is_empty()
            || authority.contains('/')
            || authority.contains('?')
            || authority.contains('#')
            || authority.contains('@')
            || authority.chars().any(char::is_whitespace)
        {
            return Err(ClientConfigError::InvalidBaseUrl);
        }

        let (host, port, host_header) = parse_authority(authority)?;
        let addresses = resolve_addresses(&host, port).map_err(|_| ClientConfigError::Resolve)?;
        if addresses.is_empty() {
            return Err(ClientConfigError::Resolve);
        }

        Ok(Self {
            host,
            port,
            host_header,
        })
    }
}

impl Transport for StdHttpTransport {
    fn execute(
        &self,
        request: &HttpRequest,
        timeout: Duration,
        shutdown: &ShutdownToken,
    ) -> Result<HttpResponse, TransportError> {
        if shutdown.is_cancelled() {
            return Err(TransportError::Cancelled);
        }

        let deadline = Instant::now()
            .checked_add(timeout)
            .ok_or(TransportError::Deadline)?;
        let mut stream = self.connect_stream(deadline, shutdown)?;
        stream
            .set_nodelay(true)
            .map_err(|_| TransportError::BeforeSend)?;

        let bytes = encode_request(request, &self.host_header);
        let sent = write_with_deadline(&mut stream, &bytes, deadline, shutdown)?;
        let (head, body_prefix) = read_response_head(&mut stream, deadline, shutdown)
            .map_err(|error| after_send(error, sent))?;
        let parsed = parse_response_head(&head).map_err(|_| TransportError::Protocol)?;

        let body = if parsed.status == 200 {
            read_response_body(
                &mut stream,
                body_prefix,
                &parsed.headers,
                deadline,
                shutdown,
            )
            .map_err(|error| after_send(error, true))?
        } else {
            Vec::new()
        };

        Ok(HttpResponse {
            status: parsed.status,
            headers: parsed.headers,
            body,
        })
    }
}

impl StdHttpTransport {
    fn connect_stream(
        &self,
        deadline: Instant,
        shutdown: &ShutdownToken,
    ) -> Result<TcpStream, TransportError> {
        let addresses =
            resolve_addresses(&self.host, self.port).map_err(|_| TransportError::BeforeSend)?;
        if addresses.is_empty() {
            return Err(TransportError::BeforeSend);
        }

        let mut saw_timeout = false;

        loop {
            for address in &addresses {
                if shutdown.is_cancelled() {
                    return Err(TransportError::Cancelled);
                }
                let remaining = remaining(deadline)?;
                let slice = remaining.min(IO_SLICE);

                match TcpStream::connect_timeout(address, slice) {
                    Ok(stream) => return Ok(stream),
                    Err(error) if is_timeout(&error) => {
                        saw_timeout = true;
                    }
                    Err(_) => {}
                }
            }

            if Instant::now() >= deadline {
                return Err(TransportError::Deadline);
            }
            if !saw_timeout {
                return Err(TransportError::BeforeSend);
            }
        }
    }
}

fn resolve_addresses(host: &str, port: u16) -> io::Result<Vec<SocketAddr>> {
    let mut addresses = Vec::new();
    for address in (host, port).to_socket_addrs()? {
        if !addresses.contains(&address) {
            addresses.push(address);
        }
    }
    Ok(addresses)
}

fn parse_authority(authority: &str) -> Result<(String, u16, String), ClientConfigError> {
    if authority.starts_with('[') {
        let closing = authority
            .find(']')
            .ok_or(ClientConfigError::InvalidBaseUrl)?;
        let host = &authority[1..closing];
        if host.is_empty() {
            return Err(ClientConfigError::InvalidBaseUrl);
        }
        host.parse::<std::net::Ipv6Addr>()
            .map_err(|_| ClientConfigError::InvalidBaseUrl)?;

        let remainder = &authority[closing + 1..];
        let port = if remainder.is_empty() {
            80
        } else {
            parse_port(
                remainder
                    .strip_prefix(':')
                    .ok_or(ClientConfigError::InvalidBaseUrl)?,
            )?
        };
        let host_header = if port == 80 {
            format!("[{host}]")
        } else {
            format!("[{host}]:{port}")
        };
        return Ok((host.to_string(), port, host_header));
    }

    if authority.contains('[') || authority.contains(']') {
        return Err(ClientConfigError::InvalidBaseUrl);
    }

    let colon_count = authority.bytes().filter(|byte| *byte == b':').count();
    if colon_count > 1 {
        return Err(ClientConfigError::InvalidBaseUrl);
    }

    let (host, port) = if colon_count == 1 {
        let (host, raw_port) = authority
            .rsplit_once(':')
            .ok_or(ClientConfigError::InvalidBaseUrl)?;
        (host, parse_port(raw_port)?)
    } else {
        (authority, 80)
    };
    if host.is_empty() {
        return Err(ClientConfigError::InvalidBaseUrl);
    }

    let host_header = if port == 80 {
        host.to_string()
    } else {
        format!("{host}:{port}")
    };
    Ok((host.to_string(), port, host_header))
}

fn parse_port(raw: &str) -> Result<u16, ClientConfigError> {
    if raw.is_empty() || !raw.bytes().all(|byte| byte.is_ascii_digit()) {
        return Err(ClientConfigError::InvalidBaseUrl);
    }
    raw.parse::<u16>()
        .map_err(|_| ClientConfigError::InvalidBaseUrl)
}

fn encode_request(request: &HttpRequest, host_header: &str) -> Vec<u8> {
    let mut encoded = format!(
        "{} {} HTTP/1.1\r\nHost: {}\r\nConnection: close\r\nContent-Length: {}\r\n",
        request.method,
        request.path,
        host_header,
        request.body.len(),
    )
    .into_bytes();

    for (name, value) in &request.headers {
        encoded.extend_from_slice(name.as_bytes());
        encoded.extend_from_slice(b": ");
        encoded.extend_from_slice(value.as_bytes());
        encoded.extend_from_slice(b"\r\n");
    }
    encoded.extend_from_slice(b"\r\n");
    encoded.extend_from_slice(&request.body);
    encoded
}

fn write_with_deadline(
    stream: &mut TcpStream,
    bytes: &[u8],
    deadline: Instant,
    shutdown: &ShutdownToken,
) -> Result<bool, TransportError> {
    let mut offset = 0;
    while offset < bytes.len() {
        if shutdown.is_cancelled() {
            return Err(TransportError::Cancelled);
        }
        stream
            .set_write_timeout(Some(remaining(deadline)?.min(IO_SLICE)))
            .map_err(|_| transport_for_progress(offset))?;

        match stream.write(&bytes[offset..]) {
            Ok(0) => return Err(transport_for_progress(offset)),
            Ok(written) => offset += written,
            Err(error) if is_timeout(&error) => continue,
            Err(_) => return Err(transport_for_progress(offset)),
        }
    }
    stream.flush().map_err(|_| transport_for_progress(offset))?;
    Ok(!bytes.is_empty())
}

fn read_response_head(
    stream: &mut TcpStream,
    deadline: Instant,
    shutdown: &ShutdownToken,
) -> Result<(Vec<u8>, Vec<u8>), TransportError> {
    let mut bytes = Vec::with_capacity(1024);
    let mut buffer = [0_u8; 1024];

    loop {
        if shutdown.is_cancelled() {
            return Err(TransportError::Cancelled);
        }
        if bytes.len() >= MAX_RESPONSE_HEAD {
            return Err(TransportError::Protocol);
        }

        stream
            .set_read_timeout(Some(remaining(deadline)?.min(IO_SLICE)))
            .map_err(|_| TransportError::AfterSend)?;

        match stream.read(&mut buffer) {
            Ok(0) => return Err(TransportError::Protocol),
            Ok(read) => {
                bytes.extend_from_slice(&buffer[..read]);
                if let Some(end) = find_header_end(&bytes) {
                    let body = bytes.split_off(end + 4);
                    bytes.truncate(end + 4);
                    return Ok((bytes, body));
                }
            }
            Err(error) if is_timeout(&error) => continue,
            Err(_) => return Err(TransportError::AfterSend),
        }
    }
}

fn read_response_body(
    stream: &mut TcpStream,
    mut body: Vec<u8>,
    headers: &[(String, String)],
    deadline: Instant,
    shutdown: &ShutdownToken,
) -> Result<Vec<u8>, TransportError> {
    let content_length = header(headers, "content-length")
        .map(|value| value.parse::<usize>().map_err(|_| TransportError::Protocol))
        .transpose()?;

    if let Some(expected) = content_length {
        if expected > MAX_RESPONSE_BODY || body.len() > expected {
            return Err(TransportError::Protocol);
        }

        let mut buffer = [0_u8; 1024];
        while body.len() < expected {
            if shutdown.is_cancelled() {
                return Err(TransportError::Cancelled);
            }
            stream
                .set_read_timeout(Some(remaining(deadline)?.min(IO_SLICE)))
                .map_err(|_| TransportError::AfterSend)?;
            match stream.read(&mut buffer) {
                Ok(0) => return Err(TransportError::Protocol),
                Ok(read) => {
                    let remaining_bytes = expected - body.len();
                    body.extend_from_slice(&buffer[..read.min(remaining_bytes)]);
                }
                Err(error) if is_timeout(&error) => continue,
                Err(_) => return Err(TransportError::AfterSend),
            }
        }
        return Ok(body);
    }

    let mut buffer = [0_u8; 1024];
    loop {
        if shutdown.is_cancelled() {
            return Err(TransportError::Cancelled);
        }
        if body.len() > MAX_RESPONSE_BODY {
            return Err(TransportError::Protocol);
        }

        stream
            .set_read_timeout(Some(remaining(deadline)?.min(IO_SLICE)))
            .map_err(|_| TransportError::AfterSend)?;
        match stream.read(&mut buffer) {
            Ok(0) => return Ok(body),
            Ok(read) => {
                body.extend_from_slice(&buffer[..read]);
                if body.len() > MAX_RESPONSE_BODY {
                    return Err(TransportError::Protocol);
                }
            }
            Err(error) if is_timeout(&error) => continue,
            Err(_) => return Err(TransportError::AfterSend),
        }
    }
}

struct ParsedResponseHead {
    status: u16,
    headers: Vec<(String, String)>,
}

fn parse_response_head(bytes: &[u8]) -> Result<ParsedResponseHead, ()> {
    let head = std::str::from_utf8(bytes).map_err(|_| ())?;
    let mut lines = head.split("\r\n");
    let status_line = lines.next().ok_or(())?;
    let mut parts = status_line.split_whitespace();
    let version = parts.next().ok_or(())?;
    if version != "HTTP/1.0" && version != "HTTP/1.1" {
        return Err(());
    }

    let raw_status = parts.next().ok_or(())?;
    if raw_status.len() != 3 || !raw_status.bytes().all(|byte| byte.is_ascii_digit()) {
        return Err(());
    }
    let status = raw_status.parse::<u16>().map_err(|_| ())?;
    if !(100..=599).contains(&status) {
        return Err(());
    }

    let mut headers = Vec::new();
    for line in lines {
        if line.is_empty() {
            break;
        }
        if line.starts_with(' ') || line.starts_with('\t') {
            return Err(());
        }
        let (name, value) = line.split_once(':').ok_or(())?;
        if name.is_empty()
            || !name
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || byte == b'-')
        {
            return Err(());
        }
        headers.push((name.to_ascii_lowercase(), value.trim().to_string()));
    }

    Ok(ParsedResponseHead { status, headers })
}

pub(crate) fn header<'a>(headers: &'a [(String, String)], name: &str) -> Option<&'a str> {
    headers
        .iter()
        .find(|(header_name, _)| header_name.eq_ignore_ascii_case(name))
        .map(|(_, value)| value.as_str())
}

fn find_header_end(bytes: &[u8]) -> Option<usize> {
    bytes.windows(4).position(|window| window == b"\r\n\r\n")
}

fn remaining(deadline: Instant) -> Result<Duration, TransportError> {
    deadline
        .checked_duration_since(Instant::now())
        .filter(|duration| !duration.is_zero())
        .ok_or(TransportError::Deadline)
}

fn is_timeout(error: &io::Error) -> bool {
    matches!(
        error.kind(),
        io::ErrorKind::TimedOut | io::ErrorKind::WouldBlock
    )
}

fn transport_for_progress(bytes_written: usize) -> TransportError {
    if bytes_written == 0 {
        TransportError::BeforeSend
    } else {
        TransportError::AfterSend
    }
}

fn after_send(error: TransportError, sent: bool) -> TransportError {
    match error {
        TransportError::BeforeSend if sent => TransportError::AfterSend,
        other => other,
    }
}

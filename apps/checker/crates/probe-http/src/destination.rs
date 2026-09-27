use std::{
    collections::HashSet,
    net::{IpAddr, Ipv4Addr, Ipv6Addr},
};

const MAX_REDIRECTS: u8 = 3;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ResolveError;

pub trait Resolver: Send + Sync {
    fn resolve(&self, host: &str) -> Result<Vec<IpAddr>, ResolveError>;
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum DestinationError {
    InvalidUrl,
    DnsFailure,
    PolicyRejected,
}

#[derive(Clone, Debug, Eq, PartialEq)]
enum Host {
    Domain(String),
    Ip(IpAddr),
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ParsedUrl {
    scheme: String,
    authority: String,
    host: Host,
    port: u16,
    path: String,
    query: Option<String>,
    serialized: String,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ValidatedDestination {
    parsed: ParsedUrl,
    addresses: Vec<IpAddr>,
}

impl ValidatedDestination {
    pub fn url(&self) -> &str {
        &self.parsed.serialized
    }

    pub fn scheme(&self) -> &str {
        &self.parsed.scheme
    }

    pub const fn port(&self) -> u16 {
        self.parsed.port
    }

    pub fn addresses(&self) -> &[IpAddr] {
        &self.addresses
    }
}

pub struct DestinationPolicy<R> {
    resolver: R,
}

impl<R> DestinationPolicy<R>
where
    R: Resolver,
{
    pub const fn new(resolver: R) -> Self {
        Self { resolver }
    }

    pub const fn resolver(&self) -> &R {
        &self.resolver
    }

    pub fn validate(&self, target: &str) -> Result<ValidatedDestination, DestinationError> {
        let parsed = parse_absolute_url(target, false)?;
        self.validate_parsed(parsed)
    }

    pub fn validate_redirect(
        &self,
        current: &ValidatedDestination,
        location: &str,
        redirects_followed: u8,
    ) -> Result<ValidatedDestination, DestinationError> {
        if redirects_followed >= MAX_REDIRECTS {
            return Err(DestinationError::PolicyRejected);
        }

        let parsed = resolve_reference(&current.parsed, location)?;
        if current.parsed.scheme == "https" && parsed.scheme == "http" {
            return Err(DestinationError::PolicyRejected);
        }

        self.validate_parsed(parsed)
    }

    fn validate_parsed(&self, parsed: ParsedUrl) -> Result<ValidatedDestination, DestinationError> {
        let addresses = match &parsed.host {
            Host::Ip(address) => vec![normalize_address(*address)],
            Host::Domain(host) => {
                let resolved = self
                    .resolver
                    .resolve(host)
                    .map_err(|_| DestinationError::DnsFailure)?;
                if resolved.is_empty() {
                    return Err(DestinationError::DnsFailure);
                }
                deduplicate_addresses(resolved)
            }
        };

        if addresses.iter().any(|address| !is_public_address(*address)) {
            return Err(DestinationError::PolicyRejected);
        }

        Ok(ValidatedDestination { parsed, addresses })
    }
}

fn parse_absolute_url(raw: &str, allow_fragment: bool) -> Result<ParsedUrl, DestinationError> {
    if raw.is_empty() || raw.chars().any(|ch| ch.is_control()) {
        return Err(DestinationError::InvalidUrl);
    }

    let scheme_end = raw.find(':').ok_or(DestinationError::InvalidUrl)?;
    let raw_scheme = &raw[..scheme_end];
    if !valid_scheme(raw_scheme) {
        return Err(DestinationError::InvalidUrl);
    }
    let scheme = raw_scheme.to_ascii_lowercase();
    let expected_port = match scheme.as_str() {
        "http" => 80,
        "https" => 443,
        _ => return Err(DestinationError::PolicyRejected),
    };

    let rest = raw[scheme_end + 1..]
        .strip_prefix("//")
        .ok_or(DestinationError::InvalidUrl)?;
    if rest.is_empty() {
        return Err(DestinationError::InvalidUrl);
    }

    let authority_end = rest
        .find(|ch| ['/', '?', '#'].contains(&ch))
        .unwrap_or(rest.len());
    let authority = &rest[..authority_end];
    let tail = &rest[authority_end..];

    if authority.is_empty() {
        return Err(DestinationError::InvalidUrl);
    }
    if authority.contains('@') {
        return Err(DestinationError::PolicyRejected);
    }
    if authority
        .chars()
        .any(|ch| ch.is_whitespace() || ch.is_control() || ch == '\\')
    {
        return Err(DestinationError::InvalidUrl);
    }

    let (host, canonical_host, explicit_port) = parse_authority(authority)?;
    let port = explicit_port.unwrap_or(expected_port);
    if port != expected_port {
        return Err(DestinationError::PolicyRejected);
    }

    let (without_fragment, had_fragment) = match tail.split_once('#') {
        Some((before, _)) => (before, true),
        None => (tail, false),
    };
    if had_fragment && !allow_fragment {
        return Err(DestinationError::InvalidUrl);
    }

    let (path, query) = split_path_query(without_fragment);
    let path = if path.is_empty() {
        "/".to_string()
    } else {
        path.to_string()
    };

    let authority = match explicit_port {
        Some(value) => format!("{canonical_host}:{value}"),
        None => canonical_host,
    };
    let serialized = serialize_url(&scheme, &authority, &path, query.as_deref());

    Ok(ParsedUrl {
        scheme,
        authority,
        host,
        port,
        path,
        query,
        serialized,
    })
}

fn parse_authority(authority: &str) -> Result<(Host, String, Option<u16>), DestinationError> {
    if authority.starts_with('[') {
        let closing = authority.find(']').ok_or(DestinationError::InvalidUrl)?;
        let raw_address = &authority[1..closing];
        let address = raw_address
            .parse::<Ipv6Addr>()
            .map_err(|_| DestinationError::InvalidUrl)?;
        let remainder = &authority[closing + 1..];
        let port = if remainder.is_empty() {
            None
        } else {
            let raw_port = remainder
                .strip_prefix(':')
                .ok_or(DestinationError::InvalidUrl)?;
            Some(parse_port(raw_port)?)
        };
        return Ok((Host::Ip(IpAddr::V6(address)), format!("[{address}]"), port));
    }

    if authority.contains('[') || authority.contains(']') {
        return Err(DestinationError::InvalidUrl);
    }

    let colon_count = authority.bytes().filter(|byte| *byte == b':').count();
    if colon_count > 1 {
        return Err(DestinationError::InvalidUrl);
    }

    let (raw_host, port) = if colon_count == 1 {
        let (host, raw_port) = authority
            .rsplit_once(':')
            .ok_or(DestinationError::InvalidUrl)?;
        (host, Some(parse_port(raw_port)?))
    } else {
        (authority, None)
    };

    if raw_host.is_empty()
        || raw_host
            .chars()
            .any(|ch| ch.is_whitespace() || ch.is_control() || matches!(ch, '%' | '\\'))
    {
        return Err(DestinationError::InvalidUrl);
    }

    if let Ok(address) = raw_host.parse::<Ipv4Addr>() {
        return Ok((Host::Ip(IpAddr::V4(address)), address.to_string(), port));
    }

    let host = raw_host.to_ascii_lowercase();
    Ok((Host::Domain(host.clone()), host, port))
}

fn parse_port(raw: &str) -> Result<u16, DestinationError> {
    if raw.is_empty() || !raw.bytes().all(|byte| byte.is_ascii_digit()) {
        return Err(DestinationError::InvalidUrl);
    }
    raw.parse::<u16>().map_err(|_| DestinationError::InvalidUrl)
}

fn split_path_query(value: &str) -> (&str, Option<String>) {
    if value.is_empty() {
        return ("", None);
    }
    if let Some(query) = value.strip_prefix('?') {
        return ("", Some(query.to_string()));
    }
    if let Some((path, query)) = value.split_once('?') {
        return (path, Some(query.to_string()));
    }
    (value, None)
}

fn resolve_reference(base: &ParsedUrl, location: &str) -> Result<ParsedUrl, DestinationError> {
    if location.chars().any(|ch| ch.is_control()) {
        return Err(DestinationError::InvalidUrl);
    }

    let reference = location
        .split_once('#')
        .map_or(location, |(before, _)| before);

    if has_uri_scheme(reference) {
        return parse_absolute_url(reference, true);
    }
    if reference.starts_with("//") {
        let absolute = format!("{}:{reference}", base.scheme);
        return parse_absolute_url(&absolute, true);
    }

    let query_marker = reference.find('?');
    let reference_path = query_marker.map_or(reference, |index| &reference[..index]);
    let reference_query = query_marker.map(|index| reference[index + 1..].to_string());

    let path = if reference_path.is_empty() {
        base.path.clone()
    } else if reference_path.starts_with('/') {
        remove_dot_segments(reference_path)
    } else {
        let base_directory = base
            .path
            .rfind('/')
            .map_or("/", |index| &base.path[..=index]);
        remove_dot_segments(&format!("{base_directory}{reference_path}"))
    };

    let query = if query_marker.is_some() {
        reference_query
    } else if reference_path.is_empty() {
        base.query.clone()
    } else {
        None
    };

    let serialized = serialize_url(&base.scheme, &base.authority, &path, query.as_deref());
    Ok(ParsedUrl {
        scheme: base.scheme.clone(),
        authority: base.authority.clone(),
        host: base.host.clone(),
        port: base.port,
        path,
        query,
        serialized,
    })
}

fn has_uri_scheme(value: &str) -> bool {
    let Some(colon) = value.find(':') else {
        return false;
    };
    let boundary = value
        .find(|ch| ['/', '?', '#'].contains(&ch))
        .unwrap_or(value.len());
    colon < boundary && valid_scheme(&value[..colon])
}

fn valid_scheme(value: &str) -> bool {
    let mut chars = value.chars();
    matches!(chars.next(), Some(first) if first.is_ascii_alphabetic())
        && chars.all(|ch| ch.is_ascii_alphanumeric() || matches!(ch, '+' | '-' | '.'))
}

fn remove_dot_segments(path: &str) -> String {
    let preserve_trailing_slash =
        path.ends_with('/') || path.ends_with("/.") || path.ends_with("/..");
    let mut output: Vec<&str> = Vec::new();

    for segment in path.split('/') {
        match segment {
            "." => {}
            ".." => {
                if output.last().is_some_and(|last| !last.is_empty()) {
                    output.pop();
                }
            }
            _ => output.push(segment),
        }
    }

    if preserve_trailing_slash && output.last().is_none_or(|last| !last.is_empty()) {
        output.push("");
    }

    let mut normalized = output.join("/");
    if path.starts_with('/') && !normalized.starts_with('/') {
        normalized.insert(0, '/');
    }
    if normalized.is_empty() {
        normalized.push('/');
    }
    normalized
}

fn serialize_url(scheme: &str, authority: &str, path: &str, query: Option<&str>) -> String {
    let mut value = format!("{scheme}://{authority}{path}");
    if let Some(query) = query {
        value.push('?');
        value.push_str(query);
    }
    value
}

fn deduplicate_addresses(addresses: Vec<IpAddr>) -> Vec<IpAddr> {
    let mut seen = HashSet::new();
    let mut deduplicated = Vec::new();

    for address in addresses {
        let normalized = normalize_address(address);
        if seen.insert(normalized) {
            deduplicated.push(normalized);
        }
    }

    deduplicated
}

fn normalize_address(address: IpAddr) -> IpAddr {
    match address {
        IpAddr::V4(address) => IpAddr::V4(address),
        IpAddr::V6(address) => mapped_ipv4(address)
            .map(IpAddr::V4)
            .unwrap_or(IpAddr::V6(address)),
    }
}

fn mapped_ipv4(address: Ipv6Addr) -> Option<Ipv4Addr> {
    let segments = address.segments();
    if segments[..5] == [0, 0, 0, 0, 0] && segments[5] == 0xffff {
        let high = segments[6].to_be_bytes();
        let low = segments[7].to_be_bytes();
        return Some(Ipv4Addr::new(high[0], high[1], low[0], low[1]));
    }
    None
}

fn is_public_address(address: IpAddr) -> bool {
    match normalize_address(address) {
        IpAddr::V4(address) => is_public_ipv4(address),
        IpAddr::V6(address) => is_public_ipv6(address),
    }
}

fn is_public_ipv4(address: Ipv4Addr) -> bool {
    ![
        (Ipv4Addr::new(0, 0, 0, 0), 8),
        (Ipv4Addr::new(10, 0, 0, 0), 8),
        (Ipv4Addr::new(100, 64, 0, 0), 10),
        (Ipv4Addr::new(127, 0, 0, 0), 8),
        (Ipv4Addr::new(169, 254, 0, 0), 16),
        (Ipv4Addr::new(172, 16, 0, 0), 12),
        (Ipv4Addr::new(192, 0, 0, 0), 24),
        (Ipv4Addr::new(192, 0, 2, 0), 24),
        (Ipv4Addr::new(192, 88, 99, 0), 24),
        (Ipv4Addr::new(192, 168, 0, 0), 16),
        (Ipv4Addr::new(198, 18, 0, 0), 15),
        (Ipv4Addr::new(198, 51, 100, 0), 24),
        (Ipv4Addr::new(203, 0, 113, 0), 24),
        (Ipv4Addr::new(224, 0, 0, 0), 4),
        (Ipv4Addr::new(240, 0, 0, 0), 4),
    ]
    .into_iter()
    .any(|(network, prefix)| ipv4_in_prefix(address, network, prefix))
}

fn ipv4_in_prefix(address: Ipv4Addr, network: Ipv4Addr, prefix: u8) -> bool {
    let mask = if prefix == 0 {
        0
    } else {
        u32::MAX << (32 - u32::from(prefix))
    };
    (u32::from(address) & mask) == (u32::from(network) & mask)
}

fn is_public_ipv6(address: Ipv6Addr) -> bool {
    let forbidden = [
        (Ipv6Addr::UNSPECIFIED, 96),
        (Ipv6Addr::LOCALHOST, 128),
        (Ipv6Addr::new(0x0064, 0xff9b, 0, 0, 0, 0, 0, 0), 96),
        (Ipv6Addr::new(0x0064, 0xff9b, 0x0001, 0, 0, 0, 0, 0), 48),
        (Ipv6Addr::new(0x0100, 0, 0, 0, 0, 0, 0, 0), 64),
        (Ipv6Addr::new(0x2001, 0, 0, 0, 0, 0, 0, 0), 32),
        (Ipv6Addr::new(0x2001, 0x0002, 0, 0, 0, 0, 0, 0), 48),
        (Ipv6Addr::new(0x2001, 0x0010, 0, 0, 0, 0, 0, 0), 28),
        (Ipv6Addr::new(0x2001, 0x0020, 0, 0, 0, 0, 0, 0), 28),
        (Ipv6Addr::new(0x2001, 0x0db8, 0, 0, 0, 0, 0, 0), 32),
        (Ipv6Addr::new(0x2002, 0, 0, 0, 0, 0, 0, 0), 16),
        (Ipv6Addr::new(0x3fff, 0, 0, 0, 0, 0, 0, 0), 20),
        (Ipv6Addr::new(0x5f00, 0, 0, 0, 0, 0, 0, 0), 16),
        (Ipv6Addr::new(0xfc00, 0, 0, 0, 0, 0, 0, 0), 7),
        (Ipv6Addr::new(0xfe80, 0, 0, 0, 0, 0, 0, 0), 10),
        (Ipv6Addr::new(0xfec0, 0, 0, 0, 0, 0, 0, 0), 10),
        (Ipv6Addr::new(0xff00, 0, 0, 0, 0, 0, 0, 0), 8),
    ];

    !forbidden
        .into_iter()
        .any(|(network, prefix)| ipv6_in_prefix(address, network, prefix))
}

fn ipv6_in_prefix(address: Ipv6Addr, network: Ipv6Addr, prefix: u8) -> bool {
    let mask = if prefix == 0 {
        0
    } else {
        u128::MAX << (128 - u32::from(prefix))
    };
    (u128::from(address) & mask) == (u128::from(network) & mask)
}

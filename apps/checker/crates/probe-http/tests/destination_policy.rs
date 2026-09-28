use std::{
    collections::HashMap,
    net::{IpAddr, Ipv4Addr, Ipv6Addr},
    sync::Mutex,
};

use probe_http::{
    DestinationError, DestinationPolicy, ResolveError, Resolver, ValidatedDestination,
};

#[derive(Default)]
struct FixtureResolver {
    answers: HashMap<String, Result<Vec<IpAddr>, ResolveError>>,
    calls: Mutex<Vec<String>>,
}

impl FixtureResolver {
    fn with_answer(mut self, host: &str, addresses: Vec<IpAddr>) -> Self {
        self.answers.insert(host.to_string(), Ok(addresses));
        self
    }

    fn with_failure(mut self, host: &str) -> Self {
        self.answers.insert(host.to_string(), Err(ResolveError));
        self
    }

    fn calls(&self) -> Vec<String> {
        self.calls.lock().expect("resolver calls lock").clone()
    }
}

impl Resolver for FixtureResolver {
    fn resolve(&self, host: &str) -> Result<Vec<IpAddr>, ResolveError> {
        self.calls
            .lock()
            .expect("resolver calls lock")
            .push(host.to_string());

        self.answers.get(host).cloned().unwrap_or(Err(ResolveError))
    }
}

fn policy(resolver: FixtureResolver) -> DestinationPolicy<FixtureResolver> {
    DestinationPolicy::new(resolver)
}

fn v4(raw: &str) -> IpAddr {
    IpAddr::V4(raw.parse::<Ipv4Addr>().expect("valid IPv4 fixture"))
}

fn v6(raw: &str) -> IpAddr {
    IpAddr::V6(raw.parse::<Ipv6Addr>().expect("valid IPv6 fixture"))
}

fn assert_rejected(target: &str) {
    let resolver = FixtureResolver::default();
    let policy = policy(resolver);
    let error = policy
        .validate(target)
        .expect_err("destination should be rejected");
    assert_eq!(error, DestinationError::PolicyRejected, "target={target}");
}

fn validate_hostname(
    resolver: FixtureResolver,
    target: &str,
) -> Result<ValidatedDestination, DestinationError> {
    policy(resolver).validate(target)
}

#[test]
fn rejects_forbidden_ipv4_literals() {
    for target in [
        "http://0.0.0.0/",
        "http://0.1.2.3/",
        "http://10.0.0.1/",
        "http://100.64.0.1/",
        "http://127.0.0.1/",
        "http://169.254.169.254/",
        "http://172.16.0.1/",
        "http://192.0.0.1/",
        "http://192.0.2.1/",
        "http://192.88.99.1/",
        "http://192.168.1.1/",
        "http://198.18.0.1/",
        "http://198.51.100.1/",
        "http://203.0.113.1/",
        "http://224.0.0.1/",
        "http://240.0.0.1/",
        "http://255.255.255.255/",
    ] {
        assert_rejected(target);
    }
}

#[test]
fn rejects_forbidden_ipv6_literals() {
    for target in [
        "http://[::]/",
        "http://[::1]/",
        "http://[100::1]/",
        "http://[2001:2::1]/",
        "http://[2001:db8::1]/",
        "http://[2001:10::1]/",
        "http://[2001:20::1]/",
        "http://[2002::1]/",
        "http://[3fff::1]/",
        "http://[fc00::1]/",
        "http://[fd12:3456::1]/",
        "http://[fe80::1]/",
        "http://[ff02::1]/",
    ] {
        assert_rejected(target);
    }
}

#[test]
fn ipv4_mapped_ipv6_is_classified_as_effective_ipv4() {
    assert_rejected("http://[::ffff:127.0.0.1]/");
    assert_rejected("http://[::ffff:169.254.169.254]/");

    let validated = policy(FixtureResolver::default())
        .validate("https://[::ffff:8.8.8.8]/")
        .expect("public mapped IPv4 should be allowed");

    assert_eq!(validated.addresses(), &[v4("8.8.8.8")]);
}

#[test]
fn allows_public_ipv4_and_ipv6_literals_without_dns() {
    let resolver = FixtureResolver::default();
    let policy = policy(resolver);

    let ipv4 = policy
        .validate("http://8.8.8.8/")
        .expect("public IPv4 should be allowed");
    assert_eq!(ipv4.addresses(), &[v4("8.8.8.8")]);
    assert_eq!(ipv4.port(), 80);

    let ipv6 = policy
        .validate("https://[2606:4700:4700::1111]/")
        .expect("public IPv6 should be allowed");
    assert_eq!(ipv6.addresses(), &[v6("2606:4700:4700::1111")]);
    assert_eq!(ipv6.port(), 443);

    assert!(policy.resolver().calls().is_empty());
}

#[test]
fn hostname_resolution_is_single_pass_deduplicated_and_order_preserving() {
    let resolver = FixtureResolver::default().with_answer(
        "example.com",
        vec![
            v4("8.8.8.8"),
            v6("2606:4700:4700::1111"),
            v4("8.8.8.8"),
            v4("1.1.1.1"),
        ],
    );
    let policy = policy(resolver);

    let validated = policy
        .validate("https://example.com/path")
        .expect("public answer set should be allowed");

    assert_eq!(
        validated.addresses(),
        &[v4("8.8.8.8"), v6("2606:4700:4700::1111"), v4("1.1.1.1"),]
    );
    assert_eq!(policy.resolver().calls(), vec!["example.com"]);
}

#[test]
fn any_forbidden_dns_answer_rejects_the_entire_hop() {
    let resolver = FixtureResolver::default().with_answer(
        "mixed.example",
        vec![v4("8.8.8.8"), v4("10.0.0.1"), v4("1.1.1.1")],
    );
    let policy = policy(resolver);

    assert_eq!(
        policy.validate("https://mixed.example/"),
        Err(DestinationError::PolicyRejected)
    );
    assert_eq!(policy.resolver().calls(), vec!["mixed.example"]);
}

#[test]
fn empty_and_failed_dns_resolution_are_dns_failures() {
    let empty = policy(FixtureResolver::default().with_answer("empty.example", vec![]));
    assert_eq!(
        empty.validate("https://empty.example/"),
        Err(DestinationError::DnsFailure)
    );

    let failed = policy(FixtureResolver::default().with_failure("failed.example"));
    assert_eq!(
        failed.validate("https://failed.example/"),
        Err(DestinationError::DnsFailure)
    );
}

#[test]
fn rejects_unsupported_schemes_userinfo_and_non_default_ports() {
    let public = || FixtureResolver::default().with_answer("example.com", vec![v4("8.8.8.8")]);

    for target in [
        "ftp://example.com/",
        "ws://example.com/",
        "mailto:security@example.com",
        "http://user@example.com/",
        "https://user:secret@example.com/",
        "http://example.com:443/",
        "https://example.com:80/",
        "http://example.com:8080/",
        "https://example.com:8443/",
    ] {
        let policy = policy(public());
        assert_eq!(
            policy.validate(target),
            Err(DestinationError::PolicyRejected),
            "target={target}"
        );
    }
}

#[test]
fn explicit_default_ports_are_allowed() {
    let http = validate_hostname(
        FixtureResolver::default().with_answer("example.com", vec![v4("8.8.8.8")]),
        "http://example.com:80/",
    )
    .expect("explicit HTTP default port should be allowed");
    assert_eq!(http.port(), 80);

    let https = validate_hostname(
        FixtureResolver::default().with_answer("example.com", vec![v4("8.8.8.8")]),
        "https://example.com:443/",
    )
    .expect("explicit HTTPS default port should be allowed");
    assert_eq!(https.port(), 443);
}

#[test]
fn redirect_revalidates_relative_target_and_resolves_again() {
    let resolver = FixtureResolver::default().with_answer("example.com", vec![v4("8.8.8.8")]);
    let policy = policy(resolver);

    let initial = policy
        .validate("http://example.com/a/b")
        .expect("initial target should validate");
    let redirected = policy
        .validate_redirect(&initial, "../next?x=1", 0)
        .expect("relative redirect should validate");

    assert_eq!(redirected.url(), "http://example.com/next?x=1");
    assert_eq!(
        policy.resolver().calls(),
        vec!["example.com", "example.com"],
        "redirect must not inherit the previous hop resolution"
    );
}

#[test]
fn redirect_rejects_https_downgrade_but_allows_http_upgrade() {
    let resolver = FixtureResolver::default().with_answer("example.com", vec![v4("8.8.8.8")]);
    let policy = policy(resolver);

    let https = policy
        .validate("https://example.com/start")
        .expect("initial HTTPS target should validate");
    assert_eq!(
        policy.validate_redirect(&https, "http://example.com/down", 0),
        Err(DestinationError::PolicyRejected)
    );

    let http = policy
        .validate("http://example.com/start")
        .expect("initial HTTP target should validate");
    let upgraded = policy
        .validate_redirect(&http, "https://example.com/up", 0)
        .expect("HTTP to HTTPS upgrade should validate");
    assert_eq!(upgraded.scheme(), "https");
}

#[test]
fn redirect_target_gets_fresh_destination_policy() {
    let resolver = FixtureResolver::default()
        .with_answer("public.example", vec![v4("8.8.8.8")])
        .with_answer("private.example", vec![v4("10.0.0.1")]);
    let policy = policy(resolver);

    let initial = policy
        .validate("https://public.example/")
        .expect("initial public target should validate");

    assert_eq!(
        policy.validate_redirect(&initial, "https://private.example/", 0),
        Err(DestinationError::PolicyRejected)
    );
    assert_eq!(
        policy.resolver().calls(),
        vec!["public.example", "private.example"]
    );
}

#[test]
fn redirect_limit_is_three() {
    let resolver = FixtureResolver::default().with_answer("example.com", vec![v4("8.8.8.8")]);
    let policy = policy(resolver);
    let initial = policy
        .validate("https://example.com/start")
        .expect("initial target should validate");

    policy
        .validate_redirect(&initial, "/one", 0)
        .expect("first redirect should be allowed");
    policy
        .validate_redirect(&initial, "/two", 1)
        .expect("second redirect should be allowed");
    policy
        .validate_redirect(&initial, "/three", 2)
        .expect("third redirect should be allowed");

    assert_eq!(
        policy.validate_redirect(&initial, "/four", 3),
        Err(DestinationError::PolicyRejected)
    );
}

#[test]
fn malformed_urls_and_redirect_locations_are_not_treated_as_dns_failures() {
    let malformed_policy = policy(FixtureResolver::default());

    assert_eq!(
        malformed_policy.validate("not a URL"),
        Err(DestinationError::InvalidUrl)
    );

    let resolver = FixtureResolver::default().with_answer("example.com", vec![v4("8.8.8.8")]);
    let redirect_policy = policy(resolver);
    let initial = redirect_policy
        .validate("https://example.com/")
        .expect("initial target should validate");

    assert_eq!(
        redirect_policy.validate_redirect(&initial, "http://[::1", 0),
        Err(DestinationError::InvalidUrl)
    );
}

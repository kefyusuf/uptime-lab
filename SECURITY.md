# Security Policy

## Project Status

`uptime-lab` is currently in its foundation phase. Experimental and unauthenticated builds are not suitable for arbitrary public-internet exposure.

Security controls described as future requirements are not claims that the controls are already implemented.

## Reporting a Vulnerability

Use GitHub's private vulnerability-reporting flow when it is available for this repository. Do not open a public issue containing exploit details, credentials, private target information, or proof-of-concept material that would increase risk.

If private vulnerability reporting is not available, do not publish exploit details before the maintainer has established a private channel for receiving them.

## Secrets and Logging

- Never commit real credentials, access tokens, API keys, or production secrets.
- Local secret-bearing `.env` files must remain untracked.
- Logs must not intentionally expose credentials, tokens, authorization material, or sensitive URL components.
- Sanitized evidence should be used in issues and pull requests.

## Network-Probe Threat Model

The checker will initiate outbound requests to user-provided targets. That capability makes the following first-class security concerns:

- server-side request forgery (SSRF),
- DNS rebinding,
- redirect validation,
- private, loopback, link-local, and reserved-address blocking,
- cloud metadata endpoint access,
- explicit target-port policy,
- bounded response-body processing,
- bounded request duration,
- bounded redirect count,
- bounded concurrency and resource use.

The checker implementation must introduce and test these controls before any claim of safe public exposure is made.

## Security Fixes

Security fixes should include regression evidence at the cheapest meaningful layer: domain/unit tests for policy, adapter tests for network behavior, contract tests for exposed semantics, and integration tests only where cross-runtime behavior is required.

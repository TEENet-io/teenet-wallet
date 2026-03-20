# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in teenet-wallet, please report it
responsibly by emailing **security@teenet.io**. Do not open a public issue.

We will acknowledge your report within **48 hours** and work with you to
understand and address the issue before any public disclosure.

## Scope

The following types of issues are considered security vulnerabilities:

- **Authentication or authorization bypass** -- accessing wallets or endpoints
  without proper credentials
- **Private key exposure** -- any path that leaks key material, mnemonics, or
  signing secrets
- **Transaction manipulation** -- tampering with transaction parameters, amounts,
  or destinations
- **Approval bypass** -- circumventing multi-party approval or spending-policy
  controls
- **XSS / CSRF** -- client-side injection or cross-site request forgery in the
  web frontend

## Out of Scope

- Denial-of-service attacks that require privileged network access
- Issues in dependencies that have already been reported upstream
- Social engineering

## Disclosure

We follow coordinated disclosure. Once a fix is released we will credit
reporters (unless anonymity is requested) in the release notes.

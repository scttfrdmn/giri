# Security Policy

## Supported Versions

We provide security fixes for the most recent minor release.

| Version | Supported |
|---------|-----------|
| 0.x (latest) | Yes |
| Earlier releases | No — please upgrade |

## Reporting a Vulnerability

**Please do not file public GitHub issues for security vulnerabilities.**

If you believe you have found a security vulnerability in Giri, please disclose it
responsibly using one of the following methods:

### GitHub Private Security Advisory (preferred)

1. Go to the [Security Advisories](https://github.com/scttfrdmn/giri/security/advisories)
   page.
2. Click **New draft security advisory**.
3. Fill in the title, description, and severity.
4. Submit the draft — it is visible only to you and repository maintainers.

### Alternative: Encrypted Email

If you prefer email, send a description of the vulnerability to the maintainer.
Include:

- A description of the vulnerability and its potential impact
- Steps to reproduce or a minimal proof-of-concept
- Any suggested mitigations you are aware of

## Response Timeline

| Milestone | Target |
|-----------|--------|
| Acknowledgement | Within 2 business days |
| Initial assessment | Within 5 business days |
| Fix or mitigation plan | Within 30 days of acknowledgement |
| Public disclosure | Coordinated with reporter after fix is available |

We follow a coordinated disclosure model: we ask reporters to wait for a fix before
publishing details publicly, and we will credit reporters in the release notes unless
they prefer to remain anonymous.

## Scope

Giri is a static/dynamic analysis tool that **reads** Go programs and produces
reports. It does not execute user programs with elevated privileges, does not handle
network traffic, and does not store credentials.

### In Scope

- Vulnerabilities in Giri's SSA interpreter or shadow memory that allow a specially
  crafted Go program to cause Giri to produce incorrect results (false negatives for
  safety-critical checks)
- Denial-of-service via specially crafted input that causes Giri to hang, exhaust
  memory, or crash
- Path traversal or file-read vulnerabilities in `ssautil.LoadPackages`

### Out of Scope

- Bugs in the target program being analyzed (Giri is designed to *find* these)
- Issues that require physical access to the machine running Giri
- Issues in dependencies (`golang.org/x/tools`) — please report those upstream

## Credits

We appreciate the security research community. Researchers who responsibly disclose
valid vulnerabilities will be credited in the changelog and release notes.

# Security Policy

Relith takes security seriously. This policy covers supported versions and how to report vulnerabilities.

## Reporting a vulnerability

**Do not open a public GitHub issue** for security vulnerabilities.

Please report vulnerabilities privately using GitHub's private vulnerability reporting:

- [https://github.com/cryskram/relith/security/advisories](https://github.com/cryskram/relith/security/advisories)

If you prefer email, you can reach the maintainers directly (the address is listed on the profile that owns the repository).

When reporting, please include:

- The affected version (from `relith version`)
- A description of the vulnerability and its impact
- Steps to reproduce, or a minimal proof of concept
- Suggested fix, if you have one

We aim to acknowledge reports within 5 business days and to ship a fix as soon as a patch is available. We keep reporters informed throughout the process.

## Supported versions

Security patches are applied to the latest stable release and, when practical, backported to the previous minor release.

| Version | Supported |
|---------|-----------|
| Latest | Yes |
| Previous minor | Best-effort |
| Older | No |

## Disclosure policy

We follow a coordinated disclosure process:

1. You report privately.
2. We confirm the report and begin work on a fix.
3. We release a patched version.
4. We publish a security advisory describing the issue once the fix is out.

## Dependency scanning

CI runs `govulncheck` on every push and pull request. If the pipeline flags a reachable vulnerability, treat it as a required fix before merging.

## Scope

This policy applies to the Relith codebase (`cmd/`, `internal/`). Issues in third-party dependencies should be reported upstream where possible; if a dependency has a critical, exploitable vulnerability affecting Relith, report it here as well.

# Security Policy

## Read-Only by Design

a9s is a read-only AWS resource viewer. It **never** makes mutating API calls
to AWS (no Create, Delete, Update, Put, Modify, Terminate, Start, Stop, or
Reboot operations). A CI check uses pattern matching on AWS fetcher code to
catch accidental write API usage, but this is a heuristic safety net, not a
formal guarantee.

## Credential Handling

a9s application code never opens or parses `~/.aws/credentials` directly.
Profile listing reads only `~/.aws/config`. However, the AWS SDK's credential
provider chain (which a9s uses for authentication) may read credential files
internally when resolving access keys. a9s has no control over this SDK behavior.

The official Docker image runs in `--demo` mode by default and makes no AWS
API calls.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | No |

Only the latest release receives security updates.

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Use GitHub's private vulnerability reporting:

1. Go to the [Security tab](https://github.com/k2m30/a9s/security)
2. Click "Report a vulnerability"
3. Provide a description, steps to reproduce, and potential impact

### What to Include

- Type of vulnerability
- Affected file paths or components
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

This is a solo-maintained project. I'll do my best to respond promptly, but
there are no guaranteed SLAs.

## Scope

In scope:

- The a9s binary
- Go dependencies
- Official container images (ghcr.io/k2m30/a9s)
- GitHub Actions workflows

Out of scope:

- Your AWS credentials or configuration
- Third-party tools or distributions
- Vulnerabilities in AWS APIs or the AWS SDK itself

## Dependency Scanning

- **govulncheck**: runs in CI, fails the build on known Go vulnerabilities
- **CodeQL**: GitHub-native static analysis, runs on PRs with Go changes and weekly
- **Dependabot**: automated dependency update PRs

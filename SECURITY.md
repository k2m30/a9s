# Security Policy

## Read-Only by Design

a9s is a read-only AWS resource viewer. It **never** makes mutating API calls
to AWS (no Create, Delete, Update, Put, Modify, Terminate, Start, Stop, or
Reboot operations). This is enforced by a CI check that scans all AWS fetcher
code for write API calls and fails the build if any are detected.

## No Credential File Access

a9s **never** reads `~/.aws/credentials` or any credential files directly.
All authentication is delegated to the AWS SDK's credential provider chain,
which handles credentials in memory without exposing them to application code.
The a9s binary has no code paths that open, parse, or log credential files.

The official Docker image runs in `--demo` mode by default and contains no
AWS credentials. Even if credentials are mounted into the container, demo
mode bypasses all AWS API calls.

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | No |

Only the latest release receives security updates. Please upgrade to the latest
version before reporting a vulnerability.

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Use GitHub's private vulnerability reporting:

1. Go to the [Security tab](https://github.com/k2m30/a9s/security) of this
   repository
2. Click "Report a vulnerability"
3. Provide a description, steps to reproduce, and potential impact

### What to Include

- Type of vulnerability
- Affected file paths or components
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **48 hours**: Acknowledgment of your report
- **7 days**: Initial assessment and severity determination
- **90 days**: Target for fix and release

## Scope

The following are in scope:

- The a9s binary
- Go dependencies
- Official container images (ghcr.io/k2m30/a9s)
- GitHub Actions workflows

The following are out of scope:

- Your AWS credentials or configuration
- Third-party tools or distributions
- Vulnerabilities in AWS APIs

## Dependency Scanning

- **govulncheck**: runs in CI, checks for known Go vulnerabilities with
  symbol-level reachability analysis
- **CodeQL**: GitHub-native SAST scanning
- **Dependabot**: automated dependency update PRs

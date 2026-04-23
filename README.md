# a9s - Terminal UI for AWS

**Like k9s, but for your cloud.**

[![CI](https://github.com/k2m30/a9s/actions/workflows/ci.yml/badge.svg)](https://github.com/k2m30/a9s/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/k2m30/a9s)](https://goreportcard.com/report/github.com/k2m30/a9s)
[![Release](https://img.shields.io/github/v/release/k2m30/a9s)](https://github.com/k2m30/a9s/releases/latest)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Downloads](https://img.shields.io/github/downloads/k2m30/a9s/total)](https://github.com/k2m30/a9s/releases)
[![codecov](https://codecov.io/gh/k2m30/a9s/graph/badge.svg)](https://codecov.io/gh/k2m30/a9s)

![a9s demo](docs/demos/demo.gif)

Browse, inspect, and manage 66 AWS resource types from your terminal. a9s gives you a real-time, keyboard-driven interface to your AWS infrastructure -- no clicking through the console, no memorizing CLI flags.

**Read-only by design.** a9s never makes write calls to AWS. Safe to use in production. Write operations are on the [roadmap](ROADMAP.md) only after the project has proven itself as a trusted tool (10k+ stars).

**No credential storage.** a9s never reads `~/.aws/credentials`. Authentication is delegated entirely to the AWS SDK's credential chain.

**No telemetry.** a9s never phones home.

**Try without AWS.** Run `a9s --demo` to explore the full UI with synthetic data — no AWS account needed. About 30% of demo resources demonstrate pagination with the `M` key.

## Features

- **66 AWS resource types** across 12 service categories
- Real-time resource browsing with vim-style keyboard navigation
- YAML detail view for any resource (full AWS API response)
- Auto-detect and pretty-print JSON in detail and reveal views
- AWS tags flattened as `Key: Value` pairs in detail views for easy reading
- Multi-profile and multi-region support
- Categorized menu (Compute, Storage, Database, Network, Security, CI/CD, and more)
- Column sorting by name, ID, date, or any column position (`1`-`0` keys)
- Filter/search within resource lists
- Horizontal scrolling for wide tables
- Clipboard support (copy resource IDs and YAML)
- 11 built-in color themes (Tokyo Night Dark default, Dracula, Nord, Catppuccin, and more) with custom theme support
- Child view drill-downs (Listeners, Log Streams, Invocations, Tasks, Events, and more)
- Pagination and lazy-loading for large result sets — press `M` to load more (demo mode showcases this)
- Session error log with `!` key — timestamped, scrollable, searchable
- Command mode (`:`) with profile/region switching, navigation, and tab completion
- 11,500+ unit tests

## Installation

### Homebrew (macOS and Linux)

```sh
brew install k2m30/a9s/a9s
```

### Scoop (Windows)

```powershell
scoop bucket add a9s https://github.com/k2m30/scoop-a9s.git
scoop install a9s
```

### Go install

```sh
go install github.com/k2m30/a9s/v3/cmd/a9s@latest
```

### Download binary

Download the latest release for your platform from [GitHub Releases](https://github.com/k2m30/a9s/releases/latest).

Available platforms:
- **macOS**: Intel (amd64) and Apple Silicon (arm64)
- **Linux**: amd64 and arm64
- **Windows**: amd64 and arm64

> **Windows note:** Downloaded binaries may trigger a Microsoft Defender SmartScreen warning because they are not code-signed. Click "More info" → "Run anyway" to proceed, or install via Scoop to avoid this. Windows support is new and has been verified via cross-compilation and CI only — the maintainer does not have a Windows machine. If you encounter any issues, please [open an issue](https://github.com/k2m30/a9s/issues/new).

### Docker

```sh
# Demo mode (no AWS credentials needed)
docker run --rm -it ghcr.io/k2m30/a9s:latest --demo

# Real AWS access on a laptop — mount the whole ~/.aws directory so SSO
# tokens and static credentials resolve, and set AWS_EC2_METADATA_DISABLED
# so missing creds fail fast instead of timing out against the 169.254
# IMDS endpoint.
docker run --rm -it \
  -v ~/.aws:/home/a9s/.aws:ro \
  -e AWS_EC2_METADATA_DISABLED=true \
  ghcr.io/k2m30/a9s:latest

# On an EC2 host that should inherit the instance profile, omit the env
# var so the SDK can reach IMDS and pick up the attached role.
docker run --rm -it ghcr.io/k2m30/a9s:latest
```

For SSO profiles, run `aws sso login --profile <name>` on the host before
starting the container so the cached token exists in `~/.aws/sso/cache`.

To persist per-user view / theme customization across runs, also mount
`~/.a9s`: `-v ~/.a9s:/home/a9s/.a9s`. Without that mount the container
ships with the built-in defaults only.

### Build from source

Requires Go 1.26+.

```sh
git clone https://github.com/k2m30/a9s.git
cd a9s
make build
./a9s
```

## Quick Start

a9s uses the standard [AWS credential chain](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html). Any of these work:
- Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
- AWS config file (`~/.aws/config`) — a9s never reads `~/.aws/credentials`
- EC2 instance metadata / ECS task role / SSO

```sh
a9s                       # use default profile
a9s -p production         # use a specific profile
a9s -r eu-west-1          # override region
a9s -c ec2                # open directly to EC2 instances, skipping the menu
a9s -p prod -c events     # open CloudTrail events list in a specific profile
a9s --version             # print version
a9s --demo                # run with synthetic demo data (no AWS credentials needed)
a9s --no-cache            # disable resource availability cache
a9s --reset-views         # delete view configs and regenerate defaults
a9s --reset-themes        # delete theme files and regenerate defaults
```

## Supported AWS Services

| Category | Resource Types |
|----------|---------------|
| **Compute** | EC2 Instances, ECS Services, ECS Clusters, ECS Tasks, Lambda Functions, Auto Scaling Groups, Elastic Beanstalk, EBS Volumes, EBS Snapshots, AMIs |
| **Containers** | EKS Clusters, EKS Node Groups |
| **Networking** | Load Balancers, Target Groups, Security Groups, VPCs, Subnets, Route Tables, NAT Gateways, Internet Gateways, Elastic IPs, VPC Endpoints, Transit Gateways, Network Interfaces |
| **Databases & Storage** | DB Instances, S3 Buckets, ElastiCache Redis, DB Clusters, DynamoDB Tables, OpenSearch Domains, Redshift Clusters, EFS File Systems, RDS Snapshots, DocDB Snapshots |
| **Monitoring** | CloudWatch Alarms, CloudWatch Log Groups, CloudTrail Trails, CloudTrail Events |
| **Messaging** | SQS Queues, SNS Topics, SNS Subscriptions, EventBridge Rules, Kinesis Streams, MSK Clusters, Step Functions, SES Identities |
| **Secrets & Config** | Secrets Manager, SSM Parameters, KMS Keys |
| **DNS & CDN** | Route 53 Hosted Zones, CloudFront Distributions, ACM Certificates, API Gateways |
| **Security & IAM** | IAM Roles, IAM Policies, IAM Users, IAM Groups, WAF Web ACLs |
| **CI/CD** | CloudFormation Stacks, CodePipelines, CodeBuild Projects, ECR Repositories, CodeArtifact Repos |
| **Data & Analytics** | Glue Jobs, Athena Workgroups |
| **Backup** | Backup Plans |

## Key Bindings

See the **[Key Bindings](https://github.com/k2m30/a9s/wiki/Key-Bindings)** wiki page for the full keyboard reference.

## Child Views (Drill-Downs)

See the **[Child Views](https://github.com/k2m30/a9s/wiki/Child-Views)** wiki page for the full drill-down reference.

## Commands

Press `:` to enter command mode, then type a command:

| Command | Action |
|---------|--------|
| `:q` / `:quit` | Exit a9s |
| `:ctx` / `:profile` | Switch AWS profile |
| `:region` | Switch AWS region |
| `:theme` | Switch color theme |
| `:help` | Show help |
| `:root` / `:main` | Go to main menu |
| `:<resource>` | Jump to resource type (e.g., `:ec2`, `:s3`, `:lambda`) |

All resource short names work as commands.

## Configuration

a9s stores view configuration in `~/.a9s/views/` and theme configuration in `~/.a9s/themes/`. AWS profiles and regions are read from `~/.aws/config`.

- **[View Customization](https://github.com/k2m30/a9s/wiki/View-Customization)** -- customize columns, field paths, and detail views per resource type
- **[Color Themes](https://github.com/k2m30/a9s/wiki/Color-Themes)** -- 11 built-in themes, custom theme creation, and color key reference

## AWS Permissions

a9s claims to be read-only — but a dedicated IAM role with an explicit allow-list lets AWS enforce that guarantee rather than relying on the code. The **[Minimal IAM Profile](https://github.com/k2m30/a9s/wiki/Minimal-IAM-Profile)** wiki page has the full policy JSON covering all 66 resource types, CLI setup steps, and a Terraform module.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NO_COLOR` | Set to any value (e.g., `NO_COLOR=1`) to disable all color output. Follows the [no-color.org](https://no-color.org) standard. Useful for accessibility, scripting, or piping output. |
| `AWS_PROFILE` | Override the active AWS profile (same as `-p` flag). |
| `AWS_REGION` | Override the active AWS region (same as `-r` flag). |

## Why a9s?

### Real-life use cases

- **"Is my deployment healthy?"** — Jump to ECS Services, drill into tasks and events. See which tasks are running, which crashed, and why — without touching the AWS console.
- **"Why are we getting 502s?"** — Check Target Groups → Target Health. Instantly see which targets are unhealthy and the exact reason (health check failed, connection refused, etc.).
- **"What's in this S3 bucket?"** — Browse objects, drill into folders, check sizes and dates. Like a file manager for S3.
- **"Which Lambda is failing?"** — Lambda → Invocations → Log Lines. Three key presses from function list to the actual error stack trace.
- **"What happened during the deployment?"** — CFN Stacks → Stack Events shows every resource operation in real-time: what's being created, what failed, and the exact error message.
- **"Which security groups allow 0.0.0.0/0?"** — Filter security groups, check inbound rules in the YAML detail view. No need to click through dozens of console pages.

## Roadmap

See [ROADMAP.md](ROADMAP.md) for planned features and direction.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## Security

a9s is read-only by design and never makes mutating AWS API calls. See [SECURITY.md](SECURITY.md) for our security policy and how to report vulnerabilities.

## License

GPL-3.0-or-later. See [LICENSE](LICENSE).

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles) by [Charmbracelet](https://charm.sh)
- Inspired by [k9s](https://github.com/derailed/k9s)

# a9s - Terminal UI for AWS

**Like k9s, but for your cloud.**

[![CI](https://github.com/k2m30/a9s/actions/workflows/ci.yml/badge.svg)](https://github.com/k2m30/a9s/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/k2m30/a9s)](https://goreportcard.com/report/github.com/k2m30/a9s)
[![Release](https://img.shields.io/github/v/release/k2m30/a9s)](https://github.com/k2m30/a9s/releases/latest)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Downloads](https://img.shields.io/github/downloads/k2m30/a9s/total)](https://github.com/k2m30/a9s/releases)
[![codecov](https://codecov.io/gh/k2m30/a9s/graph/badge.svg)](https://codecov.io/gh/k2m30/a9s)

![a9s demo](docs/demos/demo.gif)

Browse, inspect, and manage 62 AWS resource types from your terminal. a9s gives you a real-time, keyboard-driven interface to your AWS infrastructure -- no clicking through the console, no memorizing CLI flags.

**Read-only by design.** a9s never makes write calls to AWS. Safe to use in production. Write operations are on the [roadmap](ROADMAP.md) only after the project has proven itself as a trusted tool (10k+ stars).

**No credential storage.** a9s never reads `~/.aws/credentials`. Authentication is delegated entirely to the AWS SDK's credential chain.

**No telemetry.** a9s never phones home.

**Try without AWS.** Run `a9s --demo` to explore the full UI with synthetic data — no AWS account needed. About 30% of demo resources demonstrate pagination with the `M` key.

## Features

- **62 AWS resource types** across 12 service categories
- Real-time resource browsing with vim-style keyboard navigation
- YAML detail view for any resource (full AWS API response)
- Multi-profile and multi-region support
- Categorized menu (Compute, Storage, Database, Network, Security, CI/CD, and more)
- Column sorting by name, ID, or date
- Filter/search within resource lists
- Horizontal scrolling for wide tables
- Clipboard support (copy resource IDs and YAML)
- Tokyo Night Dark color theme
- Child view drill-downs (Listeners, Log Streams, Invocations, Tasks, Events, and more)
- Pagination and lazy-loading for large result sets — press `M` to load more (demo mode showcases this)
- 2,750+ unit tests

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

# Real AWS access
docker run --rm -it -v ~/.aws/config:/home/a9s/.aws/config:ro ghcr.io/k2m30/a9s:latest
```

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
a9s --version             # print version
a9s --demo                # run with synthetic demo data (no AWS credentials needed)
a9s --no-cache            # disable resource availability cache
```

## Supported AWS Services

| Category | Resource Types |
|----------|---------------|
| **Compute** | EC2 Instances, ECS Services, ECS Clusters, ECS Tasks, Lambda Functions, Auto Scaling Groups, Elastic Beanstalk |
| **Containers** | EKS Clusters, EKS Node Groups |
| **Networking** | Load Balancers, Target Groups, Security Groups, VPCs, Subnets, Route Tables, NAT Gateways, Internet Gateways, Elastic IPs, VPC Endpoints, Transit Gateways, Network Interfaces |
| **Databases & Storage** | DB Instances, S3 Buckets, ElastiCache Redis, DB Clusters, DynamoDB Tables, OpenSearch Domains, Redshift Clusters, EFS File Systems, RDS Snapshots, DocDB Snapshots |
| **Monitoring** | CloudWatch Alarms, CloudWatch Log Groups, CloudTrail Trails |
| **Messaging** | SQS Queues, SNS Topics, SNS Subscriptions, EventBridge Rules, Kinesis Streams, MSK Clusters, Step Functions |
| **Secrets & Config** | Secrets Manager, SSM Parameters, KMS Keys |
| **DNS & CDN** | Route 53 Hosted Zones, CloudFront Distributions, ACM Certificates, API Gateways |
| **Security & IAM** | IAM Roles, IAM Policies, IAM Users, IAM Groups, WAF Web ACLs |
| **CI/CD** | CloudFormation Stacks, CodePipelines, CodeBuild Projects, ECR Repositories, CodeArtifact Repos |
| **Data & Analytics** | Glue Jobs, Athena Workgroups |
| **Backup** | Backup Plans, SES Identities |

## Key Bindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `Down` | Move down |
| `k` / `Up` | Move up |
| `g` | Go to top |
| `G` | Go to bottom |
| `Enter` | Open / select |
| `Esc` | Back / close |
| `h` / `Left` | Scroll left |
| `l` / `Right` | Scroll right |
| `PgUp` / `Ctrl+U` | Page up |
| `PgDn` / `Ctrl+D` | Page down |

### Actions

| Key | Action |
|-----|--------|
| `d` | Detail view |
| `y` | YAML view |
| `x` | Reveal (expand) |
| `c` | Copy resource ID to clipboard |
| `i` | IAM identity view |
| `/` | Filter |
| `:` | Command mode |
| `?` | Help |
| `Ctrl+R` | Refresh |
| `e` | Open Service Events (ECS Services) |
| `L` | Open Container Logs (ECS Services) |
| `M` | Load more (paginated lists, also in demo mode) |
| `r` | Open Stack Resources (CFN Stacks) |
| `s` | Open source view (reserved for future child views) |
| `w` | Toggle line wrap (in YAML view) |
| `Tab` | Autocomplete (in command mode) |

### Sorting

| Key | Action |
|-----|--------|
| `N` | Sort by name |
| `I` | Sort by ID |
| `A` | Sort by date |

### General

| Key | Action |
|-----|--------|
| `q` | Quit |
| `Ctrl+C` | Force quit |

## Child Views (Drill-Downs)

Press `Enter` on a resource to explore its nested children. Press `Esc` to go back.

| Parent | Child View | Key | Description |
|--------|-----------|-----|-------------|
| Lambda Functions | Invocations | `Enter` | Recent invocations with status, duration, memory, cold start |
| Lambda Invocations | Log Lines | `Enter` | Full log output for a specific invocation |
| S3 Buckets | Objects | `Enter` | Browse bucket contents, drill into folders |
| Route 53 Zones | DNS Records | `Enter` | View A, CNAME, MX, and other record types |
| Log Groups | Log Streams | `Enter` | Streams sorted by most recent event |
| Log Streams | Log Events | `Enter` | Color-coded log lines (ERROR=red, WARN=yellow) |
| Target Groups | Target Health | `Enter` | Health status per target (healthy/unhealthy/draining) |
| ECS Services | Service Events | `e` | Event timeline (steady state, placement failures, deployments) |
| ECS Services | Tasks | `Enter` | Running and stopped tasks with status, health, stopped reason |
| ECS Services | Container Logs | `L` | Application logs from CloudWatch |
| CFN Stacks | Stack Events | `Enter` | Event timeline showing stack operation progress and status |
| CFN Stacks | Stack Resources | `r` | Logical and physical resources in the stack with status |
| Auto Scaling Groups | Scaling Activities | `Enter` | Scaling activity history with status, description, cause |
| CloudWatch Alarms | Alarm History | `Enter` | State transitions, configuration updates, and action events |
| Load Balancers | Listeners | `Enter` | Port, protocol, action, target, SSL policy, certificate |
| Step Functions | Executions | `Enter` | Execution list with status, duration, start/stop times |
| SFN Executions | Execution History | `Enter` | Step-by-step state machine trace with errors and state names |
| CodeBuild Projects | Builds | `Enter` | Recent builds with status, duration, source version, initiator |
| CodeBuild Builds | Build Logs | `Enter` | Full build output from CloudWatch Logs with phase highlighting |
| CodePipelines | Pipeline Stages | `Enter` | Flattened stage→action view with status coloring and external URLs |
| ECR Repositories | Images | `Enter` | Tags, digest, size, push time, scan findings with severity counts |
| IAM Roles | Attached Policies | `Enter` | Managed and inline policies with type, ARN, admin highlight |
| IAM Groups | Group Members | `Enter` | Member users with ID, creation date, password last used |
| ELB Listeners | Listener Rules | `Enter` | Priority, conditions, action type, target (nested level 2) |
| RDS Instances | RDS Events | `Enter` | Failovers, maintenance, reboots, configuration changes |
| SNS Topics | Subscriptions | `Enter` | Subscribers with protocol, endpoint, confirmation status |
| EventBridge Rules | Targets | `Enter` | Target ARN, input configuration, IAM role |
| Glue Jobs | Job Runs | `Enter` | Execution history with status, duration, DPU usage, errors |

## Commands

Press `:` to enter command mode, then type a command:

| Command | Action |
|---------|--------|
| `:q` / `:quit` | Exit a9s |
| `:ctx` / `:profile` | Switch AWS profile |
| `:region` | Switch AWS region |
| `:help` | Show help |
| `:<resource>` | Jump to resource type (e.g., `:ec2`, `:s3`, `:lambda`) |

All resource short names work as commands.

## Configuration

a9s stores view configuration in `~/.a9s/views/` as per-resource YAML files (e.g., `ec2.yaml`, `s3.yaml`) — optional, sensible defaults are built-in. AWS profiles and regions are read from `~/.aws/config`. a9s never reads `~/.aws/credentials` — authentication is delegated to the AWS SDK credential chain.

## AWS Permissions

a9s uses **read-only** AWS API calls exclusively. The following managed policies provide sufficient access:

- `ReadOnlyAccess` (broad read-only access to all services)
- Or individual service policies like `AmazonEC2ReadOnlyAccess`, `AmazonS3ReadOnlyAccess`, etc.

a9s will gracefully handle permission errors -- resources you don't have access to will show an error message instead of crashing.

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

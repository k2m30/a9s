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
- Multi-profile and multi-region support
- Categorized menu (Compute, Storage, Database, Network, Security, CI/CD, and more)
- Column sorting by name, ID, or date
- Filter/search within resource lists
- Horizontal scrolling for wide tables
- Clipboard support (copy resource IDs and YAML)
- 11 built-in color themes (Tokyo Night Dark default, Dracula, Nord, Catppuccin, and more) with custom theme support
- Child view drill-downs (Listeners, Log Streams, Invocations, Tasks, Events, and more)
- Pagination and lazy-loading for large result sets — press `M` to load more (demo mode showcases this)
- 4,500+ unit tests

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
| **Messaging** | SQS Queues, SNS Topics, SNS Subscriptions, EventBridge Rules, Kinesis Streams, MSK Clusters, Step Functions |
| **Secrets & Config** | Secrets Manager, SSM Parameters, KMS Keys |
| **DNS & CDN** | Route 53 Hosted Zones, CloudFront Distributions, ACM Certificates, API Gateways |
| **Security & IAM** | IAM Roles, IAM Policies, IAM Users, IAM Groups, WAF Web ACLs |
| **CI/CD** | CloudFormation Stacks, CodePipelines, CodeBuild Projects, ECR Repositories, CodeArtifact Repos |
| **Data & Analytics** | Glue Jobs, Athena Workgroups |
| **Backup** | Backup Plans, SES Identities |

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

a9s stores view configuration in `~/.a9s/views/` as per-resource YAML files (e.g., `ec2.yaml`, `s3.yaml`) — optional, sensible defaults are built-in. AWS profiles and regions are read from `~/.aws/config`. a9s never reads `~/.aws/credentials` — authentication is delegated to the AWS SDK credential chain.

## View Customization

Default view config files are auto-created in `~/.a9s/views/` on first launch (one YAML file per resource type). These control which columns appear in list views and which fields show in detail views. Edit any file to customize — a9s never overwrites user-edited files. Delete a file to restore its defaults on next launch.

### File Structure

Each file (e.g., `ec2.yaml`) has two optional sections:

```yaml
list:
  Name:
    width: 24
  State:
    path: State.Name
    width: 12
  Lifecycle:
    key: lifecycle
    width: 12

detail:
  - InstanceId
  - State
  - InstanceType
  - LaunchTime
  - Tags
```

**`list:`** — Ordered map of columns. Each column has:
- **`path:`** — Dot-separated field path into the AWS SDK struct (e.g., `State.Name`)
- **`key:`** — Special computed key (e.g., `lifecycle`, `age`, `status`) — use instead of `path` for derived values
- **`width:`** — Column width in characters

If neither `path` nor `key` is specified, the column title is used as the field name.

**`detail:`** — List of field paths shown in the detail view (press `Enter` on a resource).

### Finding Available Fields

A complete field reference is maintained at `~/.a9s/views_reference.yaml`, automatically updated on each launch. It lists every available field path for each resource type, generated from AWS SDK struct definitions:

```yaml
ec2:  # ec2types.Instance
  - Architecture
  - BlockDeviceMappings[].DeviceName
  - BlockDeviceMappings[].Ebs.VolumeId
  - InstanceId
  - InstanceType
  - State.Code
  - State.Name
  ...
```

Use this file to discover paths you can add to your view configs.

### Examples

**Hide a column:** Remove it from the `list:` section.

**Reorder columns:** Reorder the entries under `list:` — YAML map order is preserved.

**Add a new column:**

```yaml
list:
  AZ:
    path: Placement.AvailabilityZone
    width: 16
```

**Change column width:**

```yaml
list:
  Name:
    width: 40
```

### Lookup Chain

View configs are loaded from two directories in order:
1. `~/.a9s/views/` — global defaults (auto-created on first run)
2. `.a9s/views/` in the current directory — per-project overrides

Per-project files overlay global ones on a per-resource basis.

## Color Themes

a9s ships with 11 built-in color themes, extracted to `~/.a9s/themes/` on first run. Set a theme in `~/.a9s/config.yaml`:

```yaml
theme: "dracula.yaml"
```

Built-in dark themes: `tokyo-night` (default), `catppuccin-mocha`, `dracula`, `nord`, `gruvbox-dark`, `solarized-dark`.
Built-in light themes: `tokyo-night-light`, `catppuccin-latte`, `nord-light`, `gruvbox-light`, `solarized-light`.

> **Note:** Dark themes are designed for dark terminal backgrounds; light themes for light terminal backgrounds. Match your theme to your terminal for best results.

To switch themes at runtime, press `:` and type `theme`. Custom themes: copy any built-in file, edit the colors, and point your config at it. Partial themes inherit missing colors from the default (Tokyo Night Dark). The `NO_COLOR` environment variable always forces monochrome, regardless of theme.

See the **[View Customization](https://github.com/k2m30/a9s/wiki/View-Customization)** wiki page for the full customization guide, and **[Color Themes](https://github.com/k2m30/a9s/wiki/Color-Themes)** for the complete color key reference and custom theme creation.

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

# a9s - Terminal UI for AWS

**Like k9s, but for your cloud.**

[![CI](https://github.com/k2m30/a9s/actions/workflows/ci.yml/badge.svg)](https://github.com/k2m30/a9s/actions/workflows/ci.yml)
[![golangci-lint](https://img.shields.io/badge/golangci--lint-enabled-brightgreen)](.golangci.yml)
[![Release](https://img.shields.io/github/v/release/k2m30/a9s)](https://github.com/k2m30/a9s/releases/latest)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Downloads](https://img.shields.io/github/downloads/k2m30/a9s/total)](https://github.com/k2m30/a9s/releases)
[![codecov](https://codecov.io/gh/k2m30/a9s/graph/badge.svg)](https://codecov.io/gh/k2m30/a9s)

![a9s demo](docs/demos/demo.gif)

Browse, inspect, and manage 70 AWS resource types from your terminal. a9s gives you a real-time, keyboard-driven interface to your AWS infrastructure -- no clicking through the console, no memorizing CLI flags.

**Read-only by design.** a9s never makes write calls to AWS. Safe to use in production. Write operations are on the [roadmap](ROADMAP.md) only after the project has proven itself as a trusted tool (10k+ stars).

**No credential storage.** a9s never reads `~/.aws/credentials`. Authentication is delegated entirely to the AWS SDK's credential chain.

**No telemetry.** a9s never phones home.

**Try without AWS.** Run `a9s --demo` to explore the full UI with synthetic data — no AWS account needed. About 30% of demo resources demonstrate pagination with the `M` key.

## Features

- **70 AWS resource types** across 12 service categories
- **Cost Explorer** — a spend grid that matches your AWS invoice, with pivots, zoom, anomaly markers, and drill-down to the resources behind the numbers (see below)
- **Issue detection** — background health checks mark broken/degraded resources with `!`/`~` and per-type issue counts; `Ctrl+Z` filters to what needs attention
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
- 20,300+ unit tests

## Cost Explorer

**"Why am I spending that much?"** — answered in three keypresses.

![a9s Cost Explorer](docs/demos/costs.gif)

`:costs` opens your spend as a grid — services in rows, months in columns, cells
matching the AWS invoice to the cent. Cells color by period-over-period change
(spikes stand out, noise stays quiet) and cost anomalies carry their root cause
and dollar impact in the footer. From any cell, `Enter` drills down: service →
usage type → (for EC2, last 14 days) the individual instances — and one more
`Enter` opens the standard resource detail view of the machine behind the
number.

- Pivot rows by service, region, linked account, usage type, purchase option, or charge category (`1`-`6`)
- Zoom the time axis from years to months, weeks, and days (`+`/`-`), anchored at the cursor
- Cycle cost metrics — invoice, unblended, amortized, net amortized, blended (`b`)
- Closed months are fetched once and cached on disk per profile: instant on restart, works offline
- Scroll left past the oldest column to load older history, up to the Cost Explorer 13-month horizon
- Works in demo mode (`a9s --demo`, then `:costs`) with a planted cost-spike story you can trace end to end

## Installation

<!-- INCLUDE: install.md -->

## Quick Start

<!-- INCLUDE: quickstart.md -->

## Supported AWS Services

| Category | Resource Types |
|----------|---------------|
| **Compute** | EC2 Instances, ECS Services, ECS Clusters, ECS Tasks, Lambda Functions, Auto Scaling Groups, Elastic Beanstalk, EBS Volumes, EBS Snapshots, AMIs, Launch Templates |
| **Containers** | EKS Clusters, EKS Node Groups |
| **Networking** | Load Balancers, Target Groups, Security Groups, VPCs, Subnets, Route Tables, NAT Gateways, Internet Gateways, Elastic IPs, VPC Endpoints, Transit Gateways, Network Interfaces, Transfer Family, VPC Peering |
| **Databases & Storage** | DB Instances, S3 Buckets, ElastiCache Redis, DB Clusters, DynamoDB Tables, OpenSearch Domains, Redshift Clusters, EFS File Systems, DB Instance Snapshots, DB Cluster Snapshots |
| **Monitoring** | CloudWatch Alarms, CloudWatch Log Groups, CloudTrail Trails, CloudTrail Events |
| **Messaging** | SQS Queues, SNS Topics, SNS Subscriptions, EventBridge Rules, Kinesis Streams, MSK Clusters, Step Functions, SES Identities |
| **Secrets & Config** | Secrets Manager, SSM Parameters, KMS Keys |
| **DNS & CDN** | Route 53 Hosted Zones, CloudFront Distributions, ACM Certificates, API Gateways |
| **Security & IAM** | IAM Roles, IAM Policies, IAM Users, IAM Groups, WAF Web ACLs |
| **CI/CD** | CloudFormation Stacks, CodePipelines, CodeBuild Projects, ECR Repositories, CodeArtifact Repos |
| **Data & Analytics** | Glue Jobs, Athena Workgroups, Managed Airflow |
| **Backup** | Backup Plans |

## Key Bindings

See the **[Key Bindings](https://github.com/k2m30/a9s/wiki/Key-Bindings)** wiki page for the full keyboard reference.

## Child Views (Drill-Downs)

See the **[Child Views](https://github.com/k2m30/a9s/wiki/Child-Views)** wiki page for the full drill-down reference.

## Commands

<!-- INCLUDE: commands.md -->

## Configuration

a9s stores view configuration in `~/.a9s/views/` and theme configuration in `~/.a9s/themes/`. AWS profiles and regions are read from `~/.aws/config`.

- **[View Customization](https://github.com/k2m30/a9s/wiki/View-Customization)** -- customize columns, field paths, and detail views per resource type
- **[Color Themes](https://github.com/k2m30/a9s/wiki/Color-Themes)** -- 11 built-in themes, custom theme creation, and color key reference

## AWS Permissions

a9s claims to be read-only — but a dedicated IAM role with an explicit allow-list lets AWS enforce that guarantee rather than relying on the code. The **[Minimal IAM Profile](https://github.com/k2m30/a9s/wiki/Minimal-IAM-Profile)** wiki page has the full policy JSON covering all 70 resource types, CLI setup steps, and a Terraform module.

## Environment Variables

<!-- INCLUDE: env-vars.md -->

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

<!-- INCLUDE: licensing.md -->

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles) by [Charmbracelet](https://charm.sh)
- Inspired by [k9s](https://github.com/derailed/k9s)

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
- Tokyo Night Dark color theme
- Child view drill-downs (Listeners, Log Streams, Invocations, Tasks, Events, and more)
- Pagination and lazy-loading for large result sets — press `M` to load more (demo mode showcases this)
- 3,100+ unit tests

## Installation

<!-- INCLUDE: install.md -->

## Quick Start

<!-- INCLUDE: quickstart.md -->

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

<!-- INCLUDE: keybindings.md -->

## Child Views (Drill-Downs)

<!-- INCLUDE: childviews.md -->

## Commands

<!-- INCLUDE: commands.md -->

## Configuration

<!-- INCLUDE: config.md -->

## AWS Permissions

<!-- INCLUDE: permissions.md -->

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

GPL-3.0-or-later. See [LICENSE](LICENSE).

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lipgloss](https://github.com/charmbracelet/lipgloss), and [Bubbles](https://github.com/charmbracelet/bubbles) by [Charmbracelet](https://charm.sh)
- Inspired by [k9s](https://github.com/derailed/k9s)

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Version now injected via build-time ldflags (no longer hardcoded)
- Expanded linter configuration with gosec, gocritic, bodyclose, prealloc, noctx
- Enhanced Makefile with security scanning, coverage, and read-only API verification

### Added
- GPL-3.0-or-later license
- Full README with features, installation, key bindings, and service catalog
- CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md, SUPPORT.md
- GitHub Actions CI pipeline (lint, test, build, security, read-only verification)
- CodeQL security scanning
- GoReleaser configuration for multi-platform releases
- Homebrew tap (k2m30/homebrew-a9s)
- Docker image (ghcr.io/k2m30/a9s)
- Cosign binary signing
- Dependabot configuration for Go modules and GitHub Actions
- Issue templates and PR template
- Stale issue/PR automation

## [2.8.0] - 2025-03-18

### Added
- Config directory management with `ConfigDir` / `EnsureConfigDir`

## [2.7.0] - 2025-03-18

### Changed
- Extracted shared view primitives, added render caching for performance

## [2.5.3] - 2025-03-17

### Added
- Grouped menu by category (Compute, Storage, Database, Network, Security, etc.)

### Fixed
- Profile switch region handling
- Stale flash message after profile change

## [2.4.0] - 2025-03-16

### Added
- Route53 DNS records drill-down view

## [2.2.1] - 2025-03-15

### Added
- 330 view-layer tests covering all 52 resource types

## [2.2.0] - 2025-03-15

### Added
- 32 new AWS resource types (ACM, API Gateway, Athena, Auto Scaling, Backup, CloudFront, CodeArtifact, CodeBuild, CodePipeline, ECR, EFS, EIP, ENI, EventBridge, Glue, IAM Groups/Policies/Users, KMS, Kinesis, MSK, OpenSearch, RDS Snapshots, DocumentDB Snapshots, Redshift, Route53, SES, Step Functions, SNS Subscriptions, Transit Gateways, VPC Endpoints, WAF, CloudTrail)

## [2.1.2] - 2025-03-14

### Fixed
- UI lag on fast arrow-key scrolling

## [2.1.0] - 2025-03-14

### Fixed
- Escape key behavior
- Broken columns
- Views usability overhaul

## [2.0.0] - 2025-03-13

### Added
- 20 new AWS resource types (CloudFormation, CloudWatch, CloudWatch Logs, DynamoDB, ECS Clusters/Services/Tasks, ELB, IAM Roles, IGW, Lambda, NAT Gateways, Route Tables, SNS, SQS, SSM, Subnets, Target Groups)

## [1.5.0] - 2025-03-12

### Added
- Menu viewport scrolling
- Parametrized tests for scaling

## [1.4.1] - 2025-03-11

### Added
- Complete TUI rewrite
- VPC, Security Groups, Node Groups resource types

## [1.0.0] - 2025-03-08

### Added
- Initial release
- S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager resource types
- YAML detail view
- Multi-profile and multi-region support
- Tokyo Night Dark theme
- Clipboard support
- Help overlay

[Unreleased]: https://github.com/k2m30/a9s/compare/v2.8.0...HEAD
[2.8.0]: https://github.com/k2m30/a9s/compare/v2.7.0...v2.8.0
[2.7.0]: https://github.com/k2m30/a9s/compare/v2.5.3...v2.7.0
[2.5.3]: https://github.com/k2m30/a9s/compare/v2.4.0...v2.5.3
[2.4.0]: https://github.com/k2m30/a9s/compare/v2.2.1...v2.4.0
[2.2.1]: https://github.com/k2m30/a9s/compare/v2.2.0...v2.2.1
[2.2.0]: https://github.com/k2m30/a9s/compare/v2.1.2...v2.2.0
[2.1.2]: https://github.com/k2m30/a9s/compare/v2.1.0...v2.1.2
[2.1.0]: https://github.com/k2m30/a9s/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/k2m30/a9s/compare/v1.5.0...v2.0.0
[1.5.0]: https://github.com/k2m30/a9s/compare/v1.4.1...v1.5.0
[1.4.1]: https://github.com/k2m30/a9s/compare/v1.0.0...v1.4.1
[1.0.0]: https://github.com/k2m30/a9s/releases/tag/v1.0.0

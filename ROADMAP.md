# Roadmap

This document outlines the planned direction for a9s. Priorities may shift based on community feedback.

## Already Implemented

- **62 AWS resource types** across 12 categories
- **Search and filter** -- `/` to filter resource lists by name
- **Sort columns** -- `N`/`I`/`A` to sort by name, ID, or date
- **Customizable columns** -- `~/.a9s/views.yaml` overrides which fields are displayed per resource type
- **Multi-account** -- works out of the box via assume-role in `~/.aws/config`
- **Demo mode** -- `--demo` flag runs the full UI with synthetic data, no AWS needed

## Short-Term

- **Resource relationships** -- navigate from EC2 to its VPC, Security Groups, EBS volumes
- **Themes** -- additional color themes beyond Tokyo Night Dark
- **Resource actions** -- start/stop/reboot instances, invoke lambdas (opt-in, off by default). Gated on project maturity (10k+ stars). a9s must prove itself as a safe, trusted read-only tool before introducing write operations.

## Medium-Term

- **Cost overlay** -- show estimated monthly cost per resource (via Cost Explorer API)
- **Live tail** -- stream CloudWatch Logs in a split pane

## Long-Term

- **More AWS resource types** -- Cognito, AppSync, Config Rules, GuardDuty, Neptune, SageMaker, and more
- **Tag editor** -- view and edit resource tags inline
- **Bookmarks** -- save frequently accessed resources for quick access

## Non-Goals

- **Plugin system** -- adds complexity without clear value; new resource types are easy to add via PR
- **Terraform/IaC integration** -- a9s is a viewer, not a provisioning tool
- **Telemetry or analytics** -- a9s will never phone home
- **Web UI** -- terminal-first, always

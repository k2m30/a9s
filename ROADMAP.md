# Roadmap

This document outlines the planned direction for a9s. Priorities may shift based on community feedback.

Track progress on the [GitHub Projects board](https://github.com/k2m30/a9s/projects).

## Short-Term

- **More AWS resource types** -- Cognito, AppSync, CloudWatch Alarms, Config Rules, GuardDuty, Inspector, Lightsail, MediaConvert, Neptune, QuickSight, Redshift Serverless, SageMaker, Transfer Family
- **Search and filter** -- filter resource lists by name, tag, or status
- **Sort columns** -- click/key to sort by any column
- **Customizable columns** -- choose which fields to display per resource type

## Medium-Term

- **Resource actions** -- start/stop/reboot instances, invoke lambdas (opt-in, off by default)
- **Resource relationships** -- navigate from EC2 to its VPC, Security Groups, EBS volumes
- **Cost overlay** -- show estimated monthly cost per resource (via Cost Explorer API)
- **Multi-account** -- browse resources across AWS accounts (via assume-role)
- **Tag editor** -- view and edit resource tags inline
- **Bookmarks** -- save frequently accessed resources for quick access

## Long-Term

- **Plugin system** -- user-defined resource types and actions via Go plugins or YAML
- **Custom views** -- user-defined table layouts and detail views
- **Live tail** -- stream CloudWatch Logs in a split pane
- **Notifications** -- alert on resource state changes (instance stopped, alarm triggered)
- **SSO integration** -- native AWS SSO / Identity Center support
- **Themes** -- additional color themes beyond Tokyo Night Dark

## Non-Goals

- **Terraform/IaC integration** -- a9s is a viewer, not a provisioning tool
- **Telemetry or analytics** -- a9s will never phone home
- **Web UI** -- terminal-first, always

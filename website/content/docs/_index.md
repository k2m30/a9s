---
title: "Documentation"
---

## Getting Started

1. [Install a9s](/a9s/install/)
2. Ensure you have [AWS credentials configured](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html)
3. Run `a9s` (or `a9s -p myprofile`)

## Key Bindings

{{< include "keybindings.md" >}}

## Child Views (Drill-Downs)

{{< include "childviews.md" >}}

## Commands

{{< include "commands.md" >}}

## Configuration

{{< include "config.md" >}}

## Environment Variables

{{< include "env-vars.md" >}}

## AWS Permissions

a9s claims to be read-only — but a dedicated IAM role with an explicit allow-list lets AWS enforce that guarantee rather than relying on the code. The **[Minimal IAM Profile](https://github.com/k2m30/a9s/wiki/Minimal-IAM-Profile)** wiki page has the full policy JSON covering all 66 resource types, CLI setup steps, and a Terraform module.

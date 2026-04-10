# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# a9s Development Guidelines

## GitHub
- Repository: `k2m30/a9s` — always use this owner/repo for GitHub API calls, issues, and PRs

## Active Technologies
- Go 1.26+ + Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2, AWS SDK Go v2, yaml.v3, clipboard
- Go 1.26+ + Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2, AWS SDK Go v2 (`service/cloudtrail`), yaml.v3 (012-ct-events-list-redesign)
- N/A (in-memory `Resource.Fields` map; YAML view config on disk) (012-ct-events-list-redesign)
- Go 1.26+ + charm.land/bubbletea v2.0.2, charm.land/lipgloss v2.0.2, charm.land/bubbles v2, AWS SDK Go v2 (`service/cloudtrail`), `encoding/json` (stdlib) (013-ct-event-detail-v2)
- N/A (in-memory parsed event held only for the duration of one detail-view open) (013-ct-event-detail-v2)
- Go 1.26+ + charm.land/bubbletea v2.0.2, charm.land/lipgloss v2.0.2, charm.land/bubbles v2, AWS SDK Go v2 (all currently used services) (014-demo-transport-mock)
- In-process fixture store (per resource type, loaded at startup) (014-demo-transport-mock)
- Go 1.26+ (Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2) + AWS SDK Go v2 (CloudTrail, all current services) (015-ct-events-all-types)
- N/A (in-memory resource cache) (015-ct-events-all-types)

## Project Structure

```text
cmd/
  a9s/           # main binary
  readmegen/     # README.md generator from docs/README.tmpl.md + docs/shared/
  refgen/        # views_reference.yaml generator
internal/
  aws/           # AWS service clients & resource fetchers (top-level + child)
  buildinfo/     # version resolution from ldflags / go install
  config/        # YAML config loading
  demo/          # synthetic fixture data for demo mode
  fieldpath/     # struct field extraction via reflection
  resource/      # generic resource model, registry, child-view definitions
  tui/           # root Bubble Tea app model
    keys/        # key bindings (including child-view triggers: e, L, r, s)
    layout/      # frame rendering
    messages/    # inter-view message types (NavigateMsg, EnterChildViewMsg, etc.)
    styles/      # Tokyo Night Dark palette
    views/       # view models (menu, list, detail, yaml, help, etc.)
tests/
  unit/          # unit tests
  integration/   # integration tests
docs/
  design/        # visual design spec (incl. child-views/ with 24 view levels)
  qa/            # QA user stories
specs/           # feature specifications
```

## Commands

- `make build` — build the binary
- `make test` — run all unit tests (with `-race`)
- `go test ./tests/unit/ -run TestResourceList -count=1 -v` — run a single test by name
- `make lint` — run golangci-lint (MUST pass locally before any push). Note: do NOT include the `run` subcommand when calling golangci-lint directly — rtk treats it as a package path, causing a spurious `/run: directory not found` error.
- `make security` — check for known vulnerabilities via govulncheck (MUST pass locally before any push)
- `make gofix` — check for unfixed `//go:fix inline` directives (e.g. `reflect.Ptr` → `reflect.Pointer`). If it fails, run `go fix -inline ./...` to apply fixes.
- `go run ./cmd/readmegen/ > README.md` — regenerate README.md from template + shared docs (run after any changes to docs/shared/ or docs/README.tmpl.md)
- `go run ./cmd/viewsgen/` — regenerate per-resource YAML files in .a9s/views/ from built-in defaults (run after any changes to defaults.go)
- `go run ./cmd/refgen/ > .a9s/views_reference.yaml` — regenerate the views reference file from AWS SDK struct reflection (dev-time only, no AWS credentials needed). Must be re-run after AWS SDK version updates.
- `go run ./cmd/preview/` — render static TUI design mockups using Lipgloss v2 (no AWS credentials needed). Used as visual truth for design spec compliance.
- `./a9s --demo` — run the app with synthetic fixture data (no AWS credentials needed)

## Prerequisites

- Go 1.26+ (`brew install go`)
- golangci-lint v2.11+ (`brew install golangci-lint`)
- govulncheck (`go install golang.org/x/vuln/cmd/govulncheck@latest`)

## Architecture Principles

- **Read-only by design** — a9s never makes write calls to AWS
- **Bubble Tea v2** — all I/O in `tea.Cmd` closures, views are pure functions
- **Message-driven** — views communicate via typed messages, never import each other
- **Single source of truth** — key bindings in `keys/keys.go`, types in `types.go`, styles in `styles/`

## Code Style

Go 1.26+: Follow standard conventions

## Skills

| Skill | Scope | Usage |
|-------|-------|-------|
| `a9s-common` | All agents | Shell rules, package access rules, build/test commands |
| `a9s-bt-v2` | TUI-touching agents | Bubble Tea v2 / Lipgloss v2 / Bubbles v2 API patterns |
| `a9s-add-resource` | Coder (steps 1-7), QA (steps 8-12) | Split blueprint: coder=implementation, QA=tests |
| `a9s-add-child-view` | Coder (phase 3), QA (phase 2) | Split blueprint: architect scopes, QA tests, coder implements |
| `a9s-add-related-view` | Coder (steps 1-6), QA (steps 7-11) | Split blueprint: add related-resource views per resource type |
| `a9s-implement-issue` | Architect (orchestrator) | End-to-end: analyze → QA stories → design → scope → implement → verify → docs → release |

## Agents

| Agent | Role | Writes to | Rejects without |
|-------|------|-----------|-----------------|
| `a9s-architect` | Scopes tasks, design decisions, interfaces | Nothing (design output only) | N/A (owns scoping) |
| `a9s-coder` | Implementation only — no tests | `internal/`, `cmd/`, `.a9s/` | Exact file scope from architect |
| `a9s-qa` | Tests only — no production code | `tests/unit/` | Exact file scope from architect |
| `a9s-tui-reviewer` | Code review — BT v2 correctness, design compliance | Nothing (read-only) | N/A |
| `a9s-qa-stories` | Given/when/then stories from design spec (no source code) | Nothing (read-only) | N/A |
| `a9s-pm` | Progress tracking, dependency management, releases | Nothing (read-only) | N/A |
| `a9s-integrator` | Cross-package wiring, message flow, app.go | `internal/tui/app.go`, `messages/` | N/A |
| `a9s-fixtures` | Test fixtures from dev-account via AWS MCP | `internal/demo/` | N/A |
| `test-coverage-analyzer` | Test suite analysis, coverage gaps | Nothing (read-only) | N/A |
| `tui-ux-auditor` | UX review, k9s comparison, design guidelines | Nothing (read-only) | N/A |
| `a9s-devops` | AWS practitioner — resource priorities, feature advice | All | N/A |
| `a9s-consistency-checker` | Verifies consistency across code, tests, README, website, config | Nothing (read-only) | N/A |

## Child-View Architecture

Parent→child navigation is data-driven via `ChildViewDef` on `ResourceTypeDef`:

```go
type ChildViewDef struct {
    ChildType      string            // child ShortName (e.g., "s3_objects")
    Key            string            // trigger key: "enter", "e", "L", "r", "s"
    ContextKeys    map[string]string // parent resource → child fetcher params
    DisplayNameKey string            // context key for frame title
    DrillCondition func(Resource) bool // optional filter (e.g., S3 folders only)
}
```

Adding a new child view requires NO changes to `app.go`, `messages.go`, or `resourcelist.go`. See the `a9s-add-child-view` skill for the full checklist.

Key registries: `resource.RegisterChildType()`, `resource.RegisterChildFetcher()`, `demo.RegisterChildDemo()`.

ContextKeys resolution: `"ID"` → Resource.ID, `"Name"` → Resource.Name, `"@parent.x"` → inherited from parent context, else → Resource.Fields[key].

## Related-View Architecture

The right-column related panel is data-driven via `RelatedDef` and `NavigableField` on each resource type:

```go
type RelatedDef struct {
    TargetType       string         // target resource short name (e.g., "vpc")
    DisplayName      string         // display label in the right column
    Checker          RelatedChecker // async checker function
    NeedsTargetCache bool           // true if checker reads target type from ResourceCache (Pattern C)
}

type NavigableField struct {
    FieldPath  string // dot-path into resource fields (e.g., "VpcId")
    TargetType string // resource type to navigate to
}
```

Key registries:
- `resource.RegisterRelated(shortName, []RelatedDef{...})` — registers related checkers
- `resource.RegisterNavigableFields(shortName, []NavigableField{...})` — registers navigable fields
- `resource.RegisterRelatedDemo(shortName, func)` — registers demo-mode checker override

Adding a new related view requires **NO changes** to `app.go`, `detail.go`, `app_related.go`, or `messages.go`. All dispatch, rendering, and navigation are generic.

ContextKeys resolution for related navigation (parallel to ChildView ContextKeys):
- `"ID"` → `Resource.ID`
- `"Name"` → `Resource.Name`
- `"@parent.x"` → inherited from parent context key `x`
- anything else → `Resource.Fields[key]`

## Agent File Access Rules

Agents MUST use targeted file access — never broad globs on large directories.

### DO
- `Glob("internal/aws/{resource}*.go")` — find a specific fetcher
- `Glob("tests/unit/*{resource}*")` — find tests for a specific resource
- `Grep("mock.*{InterfaceName}", "tests/unit/mocks_test.go")` — find a specific mock
- `Glob("internal/demo/fixtures/*.go")` — find a per-service fixture file
- `Glob("internal/demo/fakes/*.go")` — find a typed-fake implementation
- `Grep("func Test.*{Resource}", "tests/unit/qa_detail_child_views_test.go")` — find append point

### DON'T
- `Glob("tests/unit/*.go")` — returns 148 files, most irrelevant
- `Glob("internal/aws/*.go")` — returns 77 files, most irrelevant
- `Glob("internal/demo/*.go")` — only 3 files remain (client.go, handlers.go, transport.go)
- Reading entire cross-cutting files (mocks_test.go, qa_detail_test.go) — grep for the section first

### Demo fixture patterns (`internal/demo/`)

**Typed fakes (014-demo-transport-mock):** All services use strongly-typed Go fake implementations that bypass the HTTP roundtrip. Each fake lives in `internal/demo/fakes/<service>.go` and satisfies the corresponding aggregate interface from `internal/aws/interfaces.go`. Per-service fixture data lives in `internal/demo/fixtures/<service>.go`, owned by the matching fake. The only remaining HTTP transport handler is STS (`handlers.go`), used for `GetCallerIdentity` probes.

`demo.NewServiceClients()` in `internal/demo/client.go` wires all 42 typed fakes into a `*awsclient.ServiceClients`. There is no legacy fixture store (`demoData`, `childDemoData`, `GetRelatedDemo` are all deleted).

#### Per-service fixture file map (`internal/demo/fixtures/`)
| File | Service |
|------|---------|
| `ec2.go` | EC2 (instances, VPCs, SGs, subnets, ENIs, NAT, IGW, EIP, volumes, snapshots, images) |
| `s3.go` | S3 (buckets + objects) |
| `cloudtrail.go` | CloudTrail (trails + events) |
| `rds.go` | RDS (instances, snapshots, events) |
| `elasticache.go` | ElastiCache (Redis clusters) |
| `dynamodb.go` | DynamoDB (tables) |
| `docdb.go` | DocumentDB (clusters, snapshots) |
| `lambda.go` | Lambda (functions, event source mappings) |
| `ecs.go` | ECS (clusters, services, tasks, task definitions) |
| `eks.go` | EKS (clusters, node groups) |
| `asg.go` | Auto Scaling (groups, activities) |
| `eb.go` | Elastic Beanstalk (environments) |
| `elb.go` | ELBv2 (load balancers, target groups, target health, listeners, rules) |
| `iam.go` | IAM (roles, policies, users, groups + all attached/inline policy maps) |
| `waf.go` | WAFv2 (web ACLs, resources) |
| `secretsmanager.go` | Secrets Manager (secrets, values) |
| `ssm.go` | SSM (parameters, values) |
| `kms.go` | KMS (keys, aliases) |
| `route53.go` | Route53 (hosted zones, records) |
| `cloudfront.go` | CloudFront (distributions) |
| `acm.go` | ACM (certificates) |
| `apigw.go` | API Gateway v2 (APIs) |
| `cfn.go` | CloudFormation (stacks, events, resources) |
| `codebuild.go` | CodeBuild (projects, builds) |
| `codepipeline.go` | CodePipeline (pipelines, states) |
| `ecr.go` | ECR (repositories, images) |
| `codeartifact.go` | CodeArtifact (repositories) |
| `cloudwatch.go` | CloudWatch (alarms, alarm history) |
| `cwlogs.go` | CloudWatch Logs (log groups, streams, events) |
| `sqs.go` | SQS (queues, attributes) |
| `sns.go` | SNS (topics, subscriptions) |
| `eventbridge.go` | EventBridge (rules, targets) |
| `kinesis.go` | Kinesis (streams) |
| `sfn.go` | Step Functions (state machines, executions, history) |
| `msk.go` | MSK (clusters) |
| `glue.go` | Glue (jobs, job runs) |
| `athena.go` | Athena (workgroups) |
| `opensearch.go` | OpenSearch (domains) |
| `redshift.go` | Redshift (clusters) |
| `backup.go` | AWS Backup (backup plans) |
| `ses.go` | SES v2 (email identities) |
| `efs.go` | EFS (file systems) |

## Bash Command Rules

- **Never chain bash commands** with `&&`, `;`, `|`, or `cd`. Always use single, standalone commands with absolute paths.
- **Never use `git -C <dir>`** flag. `cd` into the directory first as a standalone command, then run git commands separately.
- **Never use `$()` or backticks** in bash commands. Resolve values first, write intermediates to `$TMPDIR` files, read them in subsequent commands.
- **Commit messages**: write to `$TMPDIR/msg.txt` with the Write tool, then `git commit -F $TMPDIR/msg.txt` as a standalone command.

## Rules

- ALWAYS rebuild binary (`make build`) after ANY code change — version is resolved at build time via `internal/buildinfo`
- Do not make any changes until you have 95%+ confidence in what you need to build. Ask me follow up questions until you reach that confidence
- TDD is non-negotiable: architect scopes both QA and coder tasks; QA writes tests, coder writes implementation. For rigid patterns (resource types, child views) they run in parallel. For novel features, QA goes first.
- ALWAYS test ALL resource types (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager, VPC, SG, Node Groups, etc), not just one
- ALWAYS run `make test`, `make lint`, `make security`, and `make gofix` locally BEFORE pushing. CI is not a debugging tool.
- NEVER delete code, tests, or helpers just to make a linter happy. Understand WHY the code exists first. If it's genuinely dead, remove it. If it serves a purpose (scaffolding, crash-verification tests), use a targeted `//nolint` with a reason comment. If a linter rule produces widespread false positives, fix the rule in `.golangci.yml`.
- NEVER make multiple push-and-check cycles. Get it right locally, push once.
- BEFORE any push, run the `a9s-consistency-checker` agent to verify code/docs/website alignment
- BEFORE any push, run the `test-coverage-analyzer` agent to check for coverage gaps
- BEFORE any push, run the `a9s-architect` agent to verify architecture against `docs/go-codebase-checklist.md` (target: 8.5+/10)
- BEFORE any push, run the full validation integration test against a REAL AWS profile (ask user for the profile name): `A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ -run TestFullRelatedViewValidation -count=1 -v -timeout 600s`. If region is not set, the profile's default region is used. No push without this passing.
- BEFORE any release, update `CHANGELOG.md` with a new version entry (follow [Keep a Changelog](https://keepachangelog.com/) format) and create a matching `releases/vX.Y.Z.md` file with user-facing release notes. Every tagged version MUST have both a changelog entry and a release notes file.
- **Exception**: Docs-only changes (*.md, docs/, website/, specs/, .claude/, LICENSE) do NOT require the pre-push checklist.

## Docs Sync Rule

`docs/shared/` is the single source of truth for content shared between README and website.
- README is generated: edit `docs/README.tmpl.md` or `docs/shared/*.md`, then run `go run ./cmd/readmegen/ > README.md`
- Website uses Hugo `{{< include >}}` shortcodes that resolve to `docs/shared/` via module mount
- **Never edit README.md directly** — it will be overwritten by readmegen

### Counting unit tests

The test count in `docs/README.tmpl.md` and `website/themes/a9s-theme/layouts/index.html` must reflect top-level test functions, NOT subtests. `rtk go test -v` compresses output and hides `--- PASS` lines, so you must bypass it:

```
rtk proxy go test ./tests/unit/ -count=1 -timeout 120s -v > /tmp/a9s-verbose.txt 2>&1
rtk grep -e "--- PASS" -c /tmp/a9s-verbose.txt
```

Round down to the nearest hundred for the public-facing number (e.g., 4,497 → "4,400+").

When code changes affect any of the following, update the shared source and regenerate:
- Key bindings added/removed/changed → `docs/shared/keybindings.md`
- Child views added/removed → `docs/shared/childviews.md`
- Commands added/removed/changed → `docs/shared/commands.md`
- CLI flags changed → `docs/shared/quickstart.md`
- Install methods changed → `docs/shared/install.md`
- Resource types added/removed/renamed → `docs/README.tmpl.md` services table + `website/content/resources.md`
- Go version bumped → `docs/shared/install.md`, CONTRIBUTING.md


## Recent Changes
- 015-ct-events-all-types: Added Go 1.26+ (Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2) + AWS SDK Go v2 (CloudTrail, all current services)
- 014-demo-transport-mock: Added Go 1.26+ + charm.land/bubbletea v2.0.2, charm.land/lipgloss v2.0.2, charm.land/bubbles v2, AWS SDK Go v2 (all currently used services)
- 013-ct-event-detail-v2: Added Go 1.26+ + charm.land/bubbletea v2.0.2, charm.land/lipgloss v2.0.2, charm.land/bubbles v2, AWS SDK Go v2 (`service/cloudtrail`), `encoding/json` (stdlib)

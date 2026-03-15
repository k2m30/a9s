# QA Test Stories: a9s Terminal UI AWS Resource Manager

**Branch**: `001-aws-tui-manager` | **Created**: 2026-03-15 | **Status**: Draft

This document contains comprehensive user experience test stories organized by
category. Each story covers a specific scenario including happy paths, edge
cases, error conditions, and boundary behavior.

---

## 1. Launch & Startup

### QA-001: Launch with valid AWS config and default profile
**Priority**: Critical
**Category**: Launch & Startup
**Precondition**: `~/.aws/config` exists with a `[default]` profile that has `region = us-east-1` and valid credentials
**Steps**:
1. Run `a9s` with no flags
**Expected**: App launches within 2 seconds. Header shows `a9s v0.1.0 | profile: default | us-east-1`. Main menu displays all 7 resource types (S3 Buckets, EC2 Instances, RDS Instances, ElastiCache Redis, DocumentDB Clusters, EKS Clusters, Secrets Manager). Breadcrumbs show `main`. Status bar shows `Ready`.
**Edge case**: This is the baseline happy path that validates the entire startup sequence.

### QA-002: Launch with --profile flag
**Priority**: Critical
**Category**: Launch & Startup
**Precondition**: `~/.aws/config` has profiles `[default]` (us-east-1) and `[profile dev]` (eu-west-1)
**Steps**:
1. Run `a9s --profile dev`
**Expected**: Header shows `profile: dev | eu-west-1`. The profile flag overrides the default profile. Region is read from the `dev` profile's config.
**Edge case**: Ensures CLI flags take effect and region is resolved per-profile.

### QA-003: Launch with --region flag
**Priority**: High
**Category**: Launch & Startup
**Precondition**: `~/.aws/config` exists with default profile region `us-east-1`
**Steps**:
1. Run `a9s --region ap-southeast-1`
**Expected**: Header shows `profile: default | ap-southeast-1`. The --region flag overrides the config file region.
**Edge case**: Validates region override takes precedence over config file.

### QA-004: Launch with both --profile and --region flags
**Priority**: High
**Category**: Launch & Startup
**Precondition**: `~/.aws/config` has `[profile dev]` with `region = eu-west-1`
**Steps**:
1. Run `a9s --profile dev --region us-west-2`
**Expected**: Header shows `profile: dev | us-west-2`. Both flags are respected; the explicit --region overrides the dev profile's configured region.
**Edge case**: Verifies combined flag behavior where explicit region wins over profile default.

### QA-005: Launch with AWS_PROFILE environment variable
**Priority**: High
**Category**: Launch & Startup
**Precondition**: `export AWS_PROFILE=dev`; `~/.aws/config` has `[profile dev]` with `region = eu-west-1`
**Steps**:
1. Run `a9s` with no flags
**Expected**: Header shows `profile: dev | eu-west-1`. The AWS_PROFILE env var is used when no --profile flag is provided.
**Edge case**: Validates environment variable fallback for profile.

### QA-006: Launch with AWS_REGION environment variable
**Priority**: High
**Category**: Launch & Startup
**Precondition**: `export AWS_REGION=sa-east-1`; `~/.aws/config` default profile has `region = us-east-1`
**Steps**:
1. Run `a9s` with no flags
**Expected**: Header shows `profile: default | sa-east-1`. AWS_REGION env var overrides the config file but is itself overridden by --region flag.
**Edge case**: Validates the region resolution order: flag > AWS_REGION > AWS_DEFAULT_REGION > config file > us-east-1.

### QA-007: --profile flag overrides AWS_PROFILE env var
**Priority**: Medium
**Category**: Launch & Startup
**Precondition**: `export AWS_PROFILE=dev`; `~/.aws/config` has both `dev` and `staging` profiles
**Steps**:
1. Run `a9s --profile staging`
**Expected**: Header shows `profile: staging`. The --profile flag takes precedence over the AWS_PROFILE env var.
**Edge case**: Verifies flag-over-env-var precedence.

### QA-008: --region flag overrides AWS_REGION env var
**Priority**: Medium
**Category**: Launch & Startup
**Precondition**: `export AWS_REGION=eu-west-1`
**Steps**:
1. Run `a9s --region us-west-2`
**Expected**: Header shows `us-west-2`. The explicit flag wins over the env var.
**Edge case**: Verifies region flag-over-env-var precedence.

### QA-009: Launch with AWS_DEFAULT_REGION fallback
**Priority**: Medium
**Category**: Launch & Startup
**Precondition**: AWS_REGION is unset; `export AWS_DEFAULT_REGION=ca-central-1`; config file has no region for default profile
**Steps**:
1. Run `a9s` with no flags
**Expected**: Header shows `ca-central-1`. AWS_DEFAULT_REGION is used as second env var fallback.
**Edge case**: Validates the less common AWS_DEFAULT_REGION env var.

### QA-010: Launch with no AWS config file at all
**Priority**: Critical
**Category**: Launch & Startup
**Precondition**: `~/.aws/config` and `~/.aws/credentials` do not exist. No AWS env vars set.
**Steps**:
1. Run `a9s`
**Expected**: App launches with `profile: default | us-east-1` (fallback values). When attempting to load resources, a clear error is displayed: "AWS config error: ..." with suggestion to run `aws configure` or `aws sso login`. App does not crash.
**Edge case**: Users who have never configured AWS should see a helpful message, not a panic.

### QA-011: Launch with invalid/corrupt AWS config file
**Priority**: High
**Category**: Launch & Startup
**Precondition**: `~/.aws/config` contains malformed INI content (e.g., random bytes or incomplete sections)
**Steps**:
1. Run `a9s`
**Expected**: App either launches with fallback defaults (us-east-1) or displays a clear error message about the config parse failure. App does not crash.
**Edge case**: Corrupt config files should be handled gracefully.

### QA-012: Launch with --version flag
**Priority**: Medium
**Category**: Launch & Startup
**Precondition**: None
**Steps**:
1. Run `a9s --version`
**Expected**: Prints version string (e.g., `a9s v0.1.0`) to stdout and exits with code 0. Does not enter the TUI.
**Edge case**: Validates non-interactive flag handling.

### QA-013: Launch with --help flag
**Priority**: Medium
**Category**: Launch & Startup
**Precondition**: None
**Steps**:
1. Run `a9s --help`
**Expected**: Prints usage information including available flags (--profile, --region, --version, --help) and exits with code 0.
**Edge case**: Validates help output completeness.

### QA-014: Launch in a very small terminal (10 columns x 5 rows)
**Priority**: Medium
**Category**: Launch & Startup
**Precondition**: Terminal window resized to 10x5
**Steps**:
1. Run `a9s`
**Expected**: App launches without crashing. Content may be truncated but the header, breadcrumbs, and status bar should still attempt to render. No panic from rendering code.
**Edge case**: Extremely small terminals can cause buffer overflows or negative-size calculations in TUI frameworks.

### QA-015: Launch in a very wide terminal (300 columns)
**Priority**: Low
**Category**: Launch & Startup
**Precondition**: Terminal window set to 300x50
**Steps**:
1. Run `a9s`
**Expected**: App renders correctly. Header stretches to full width. Content area is usable. No rendering artifacts or misaligned columns.
**Edge case**: Very wide terminals can expose layout assumptions about maximum width.

### QA-016: Terminal resize during use
**Priority**: High
**Category**: Launch & Startup
**Precondition**: App is running normally at 120x40
**Steps**:
1. Launch `a9s` and navigate to EC2 resource list
2. Resize terminal to 60x20
3. Resize terminal to 200x60
4. Resize terminal back to 120x40
**Expected**: After each resize, the UI re-renders to fill the new dimensions. WindowSizeMsg is handled and Width/Height are updated. No crash, no garbled output, no loss of cursor position.
**Edge case**: Dynamic resize is common in tiling window managers and tmux splits.

### QA-017: Launch with -p shorthand for --profile
**Priority**: Medium
**Category**: Launch & Startup
**Precondition**: `~/.aws/config` has `[profile dev]`
**Steps**:
1. Run `a9s -p dev`
**Expected**: Same behavior as `a9s --profile dev`. Header shows `profile: dev`.
**Edge case**: Validates short flag aliases.

### QA-018: Launch with -r shorthand for --region
**Priority**: Medium
**Category**: Launch & Startup
**Precondition**: None
**Steps**:
1. Run `a9s -r eu-central-1`
**Expected**: Same behavior as `a9s --region eu-central-1`. Header shows `eu-central-1`.
**Edge case**: Validates short flag aliases.

---

## 2. Main Menu Navigation

### QA-019: Navigate down through all resource types with j key
**Priority**: Critical
**Category**: Main Menu Navigation
**Precondition**: App is on main menu, cursor at position 0 (S3 Buckets)
**Steps**:
1. Press `j` six times
**Expected**: Cursor moves sequentially through: S3 Buckets -> EC2 Instances -> RDS Instances -> ElastiCache Redis -> DocumentDB Clusters -> EKS Clusters -> Secrets Manager. Cursor highlight ("> ") moves with each press. After reaching Secrets Manager (index 6), pressing `j` again does nothing (cursor stays at bottom).
**Edge case**: Validates cursor bounds checking at the bottom of the list.

### QA-020: Navigate up through all resource types with k key
**Priority**: Critical
**Category**: Main Menu Navigation
**Precondition**: App is on main menu, cursor at position 6 (Secrets Manager, after pressing G)
**Steps**:
1. Press `k` six times
**Expected**: Cursor moves up through all 7 items back to S3 Buckets (index 0). Pressing `k` again at the top does nothing.
**Edge case**: Validates cursor bounds checking at the top of the list.

### QA-021: Navigate with arrow keys (Down/Up)
**Priority**: High
**Category**: Main Menu Navigation
**Precondition**: App is on main menu
**Steps**:
1. Press Down arrow 3 times
2. Press Up arrow 2 times
**Expected**: Cursor ends at index 1 (EC2 Instances). Arrow keys function identically to j/k.
**Edge case**: Validates dual keybinding support for navigation.

### QA-022: Jump to top with g
**Priority**: High
**Category**: Main Menu Navigation
**Precondition**: App is on main menu, cursor at index 4
**Steps**:
1. Press `g`
**Expected**: Cursor jumps to index 0 (S3 Buckets).
**Edge case**: Validates instant jump to top of list.

### QA-023: Jump to bottom with G (Shift+g)
**Priority**: High
**Category**: Main Menu Navigation
**Precondition**: App is on main menu, cursor at index 0
**Steps**:
1. Press `G`
**Expected**: Cursor jumps to index 6 (Secrets Manager, the last item).
**Edge case**: Validates instant jump to bottom of list.

### QA-024: Select resource type with Enter
**Priority**: Critical
**Category**: Main Menu Navigation
**Precondition**: App is on main menu, cursor on "EC2 Instances" (index 1)
**Steps**:
1. Press Enter
**Expected**: View transitions to ResourceListView. Loading indicator appears. Breadcrumbs update to `main > EC2 Instances`. CurrentResourceType is set to "ec2". API call is made to fetch EC2 instances.
**Edge case**: Core drill-down interaction.

### QA-025: Select each of the 7 resource types via Enter
**Priority**: Critical
**Category**: Main Menu Navigation
**Precondition**: App is on main menu with valid AWS credentials
**Steps**:
1. For each resource type (index 0 through 6):
   a. Navigate to it using j/k
   b. Press Enter
   c. Verify the correct resource list view loads
   d. Press Escape to return to main menu
**Expected**: Each resource type navigates to its correct view: S3 -> s3, EC2 -> ec2, RDS -> rds, Redis -> redis, DocDB -> docdb, EKS -> eks, Secrets -> secrets. Correct columns appear for each type.
**Edge case**: Ensures all 7 resource types are wired up and accessible from the menu.

### QA-026: Pressing unbound single-character keys on main menu
**Priority**: Medium
**Category**: Main Menu Navigation
**Precondition**: App is on main menu
**Steps**:
1. Press `a`, `b`, `f`, `z`, `1`, `9`, `0`, `-`, `=`, `;`
**Expected**: None of these keys produce any visible effect. The cursor does not move. No error messages appear. The view does not change.
**Edge case**: Unbound keys should be silently ignored, not cause unexpected behavior.

### QA-027: Pressing uppercase unbound keys on main menu
**Priority**: Low
**Category**: Main Menu Navigation
**Precondition**: App is on main menu
**Steps**:
1. Press `N`, `S`, `A` (sort keys that only apply to resource lists)
2. Press `D`, `Y`, `X`, `C` (action keys that only apply to resource lists)
**Expected**: No effect on main menu. Sort and action keys are scoped to resource list views only.
**Edge case**: Sort/action keybindings should not crash or produce errors when used in wrong context.

### QA-028: Rapid key presses (holding j down)
**Priority**: Medium
**Category**: Main Menu Navigation
**Precondition**: App is on main menu, cursor at index 0
**Steps**:
1. Hold down the `j` key for 2 seconds (rapid auto-repeat)
**Expected**: Cursor moves rapidly down to the bottom (index 6) and stops. No visual glitches, no skipped frames, no cursor going out of bounds. Performance remains smooth.
**Edge case**: Rapid input can expose race conditions or buffer overflow in event handling.

### QA-029: Press q on main menu to quit
**Priority**: High
**Category**: Main Menu Navigation
**Precondition**: App is on main menu
**Steps**:
1. Press `q`
**Expected**: Application exits cleanly with exit code 0.
**Edge case**: Validates quit behavior from main menu specifically (q only quits from main menu; elsewhere it acts as "back").

### QA-030: Press q from resource list view (should go back, not quit)
**Priority**: High
**Category**: Main Menu Navigation
**Precondition**: App is viewing EC2 resource list
**Steps**:
1. Press `q`
**Expected**: App navigates back to main menu (same as Escape). Does NOT quit the application.
**Edge case**: Critical behavior difference: q only exits from main menu, acts as back elsewhere.

---

## 3. Colon Commands

### QA-031: Enter and exit command mode
**Priority**: Critical
**Category**: Colon Commands
**Precondition**: App is on main menu in normal mode
**Steps**:
1. Press `:`
2. Observe the status bar
3. Press Escape
**Expected**: Step 2: Status bar shows `:` with cursor, indicating command mode is active. Step 3: Command mode is cancelled, status bar returns to normal. CommandMode is false, CommandText is empty.
**Edge case**: Basic command mode lifecycle.

### QA-032: Execute :ec2 command
**Priority**: Critical
**Category**: Colon Commands
**Precondition**: App is on any view
**Steps**:
1. Press `:`
2. Type `ec2`
3. Press Enter
**Expected**: App navigates to EC2 resource list view. Breadcrumbs show `main > EC2 Instances`. Loading indicator appears while fetching.
**Edge case**: Core command navigation path.

### QA-033: Execute :s3 command
**Priority**: Critical
**Category**: Colon Commands
**Precondition**: App is on any view
**Steps**:
1. Press `:`
2. Type `s3`
3. Press Enter
**Expected**: App navigates to S3 Buckets list view. Breadcrumbs show `main > S3 Buckets`.
**Edge case**: Validates S3 resource type routing.

### QA-034: Execute :rds command
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on any view
**Steps**:
1. Type `:rds` and press Enter
**Expected**: App navigates to RDS Instances list view with correct columns (DB Identifier, Engine, Version, Status, Class, Endpoint, Multi-AZ).
**Edge case**: Validates RDS resource type routing.

### QA-035: Execute :redis command
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on any view
**Steps**:
1. Type `:redis` and press Enter
**Expected**: App navigates to ElastiCache Redis list view with correct columns (Cluster ID, Version, Node Type, Status, Nodes, Endpoint).
**Edge case**: Validates Redis resource type routing.

### QA-036: Execute :docdb command
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on any view
**Steps**:
1. Type `:docdb` and press Enter
**Expected**: App navigates to DocumentDB Clusters list view with correct columns.
**Edge case**: Validates DocumentDB resource type routing.

### QA-037: Execute :eks command
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on any view
**Steps**:
1. Type `:eks` and press Enter
**Expected**: App navigates to EKS Clusters list view with correct columns (Cluster Name, Version, Status, Endpoint, Platform Version).
**Edge case**: Validates EKS resource type routing.

### QA-038: Execute :secrets command
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on any view
**Steps**:
1. Type `:secrets` and press Enter
**Expected**: App navigates to Secrets Manager list view with correct columns (Secret Name, Description, Last Accessed, Last Changed, Rotation).
**Edge case**: Validates Secrets Manager resource type routing.

### QA-039: Execute :ctx command
**Priority**: Critical
**Category**: Colon Commands
**Precondition**: App is on any view, `~/.aws/config` has multiple profiles
**Steps**:
1. Type `:ctx` and press Enter
**Expected**: ProfileSelectView appears listing all AWS profiles. The currently active profile is visually highlighted.
**Edge case**: Profile switching entry point.

### QA-040: Execute :region command
**Priority**: Critical
**Category**: Colon Commands
**Precondition**: App is on any view
**Steps**:
1. Type `:region` and press Enter
**Expected**: RegionSelectView appears listing all 27 hardcoded AWS regions with display names. The currently active region is highlighted.
**Edge case**: Region switching entry point.

### QA-041: Execute :main command
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on EC2 resource list view
**Steps**:
1. Type `:main` and press Enter
**Expected**: App returns to main menu. Breadcrumbs reset to `main`. SelectedIndex resets to 0. StatusMessage is cleared.
**Edge case**: Tests the :main navigation from deep in the app.

### QA-042: Execute :root command (alias for :main)
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on any non-main view
**Steps**:
1. Type `:root` and press Enter
**Expected**: Same behavior as `:main` -- returns to main menu.
**Edge case**: Validates command alias.

### QA-043: Execute :q command
**Priority**: Critical
**Category**: Colon Commands
**Precondition**: App is running
**Steps**:
1. Type `:q` and press Enter
**Expected**: Application exits cleanly with exit code 0.
**Edge case**: Standard vim-style quit command.

### QA-044: Execute :quit command
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is running
**Steps**:
1. Type `:quit` and press Enter
**Expected**: Application exits cleanly with exit code 0. Same as `:q`.
**Edge case**: Validates long-form quit alias.

### QA-045: Unknown command (:foo)
**Priority**: Critical
**Category**: Colon Commands
**Precondition**: App is on any view
**Steps**:
1. Type `:foo` and press Enter
**Expected**: Status bar displays `Unknown command: :foo` with error styling. The current view does not change.
**Edge case**: Verifies error handling for invalid commands.

### QA-046: Unknown command (:lambda)
**Priority**: Medium
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Type `:lambda` and press Enter
**Expected**: Status bar displays `Unknown command: :lambda`. App stays on main menu.
**Edge case**: Tests a plausible but unsupported AWS service name.

### QA-047: Empty command (press : then Enter immediately)
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Press `:`
2. Press Enter immediately without typing anything
**Expected**: No error message. Command mode exits silently. App remains on current view unchanged. The `executeCommand` function handles empty string by returning early.
**Edge case**: Users may accidentally press Enter too quickly.

### QA-048: Command with trailing spaces (:ec2   )
**Priority**: Medium
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Type `:ec2   ` (with trailing spaces) and press Enter
**Expected**: The command is trimmed via `strings.TrimSpace` and treated as `ec2`. App navigates to EC2 resource list.
**Edge case**: Validates whitespace trimming in command parsing.

### QA-049: Command case sensitivity -- :EC2 (uppercase)
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Type `:EC2` and press Enter
**Expected**: App navigates to EC2 resource list. The `FindResourceType` function uses `strings.ToLower` for comparison, so case-insensitive matching should work.
**Edge case**: The contracts specify commands are case-insensitive. Implementation confirms this via `strings.ToLower`.

### QA-050: Command case sensitivity -- :Ec2 (mixed case)
**Priority**: Medium
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Type `:Ec2` and press Enter
**Expected**: App navigates to EC2 resource list. Mixed case is handled by case-insensitive comparison.
**Edge case**: Validates mixed-case input.

### QA-051: Command case sensitivity -- :MAIN (uppercase built-in)
**Priority**: Medium
**Category**: Colon Commands
**Precondition**: App is on EC2 view
**Steps**:
1. Type `:MAIN` and press Enter
**Expected**: The `executeCommand` switch uses exact string matching (`case "main", "root":`), so `:MAIN` will NOT match and will fall through to `FindResourceType("MAIN")` which returns nil, resulting in "Unknown command: :MAIN". This is a potential BUG -- built-in commands (main, root, q, quit, ctx, region) are case-sensitive in the current implementation, while resource type commands are case-insensitive.
**Edge case**: Spec says commands are case-insensitive, but implementation uses exact match for built-in commands. This is a discrepancy to flag.

### QA-052: Escape cancels command mid-typing
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Press `:`
2. Type `ec`
3. Press Escape
**Expected**: Command mode is cancelled. CommandMode becomes false. CommandText becomes empty. Status bar returns to normal. The current view is unchanged.
**Edge case**: Validates mid-input cancellation.

### QA-053: Backspace in command mode
**Priority**: Medium
**Category**: Colon Commands
**Precondition**: App is in command mode
**Steps**:
1. Press `:` then type `ec2`
2. Press Backspace once
3. Observe status bar shows `:ec`
4. Press Backspace twice more
**Expected**: Step 3: CommandText is `ec`. Step 4: After first backspace, CommandText is `e`. After second backspace, CommandText is empty AND CommandMode is automatically deactivated (because CommandText becomes empty).
**Edge case**: When all characters are deleted via backspace, command mode exits automatically. This is implemented in handleCommandMode.

### QA-054: Auto-suggestion display while typing
**Priority**: High
**Category**: Colon Commands
**Precondition**: App is in command mode
**Steps**:
1. Press `:`
2. Type `e`
**Expected**: The autocomplete system (CommandInput.BestMatch) should suggest "ec2" or "eks" as the best match. A dimmed suffix should appear after the typed text showing the remaining characters of the suggestion.
**Edge case**: Auto-suggestions improve discoverability of commands.

### QA-055: Auto-suggestion for partial input :re
**Priority**: Medium
**Category**: Colon Commands
**Precondition**: App is in command mode
**Steps**:
1. Press `:` then type `re`
**Expected**: Auto-suggestion should show "redis" or "region" as best match (whichever comes first in the suggestion list; "region" appears before "redis" in `defaultCommands`, so "region" should be suggested).
**Edge case**: Validates suggestion ordering and partial matching.

### QA-056: Resource type aliases (:buckets, :instances, :databases)
**Priority**: Medium
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Type `:buckets` and press Enter
2. Return to main, type `:instances` and press Enter
3. Return to main, type `:databases` and press Enter
**Expected**: Each alias resolves to the correct resource type: `buckets` -> S3, `instances` -> EC2, `databases` -> RDS. The FindResourceType function checks aliases.
**Edge case**: Validates alias support from ResourceTypeDef.Aliases.

### QA-057: Resource type alias :elasticache
**Priority**: Low
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Type `:elasticache` and press Enter
**Expected**: Navigates to ElastiCache Redis view (alias defined in resource types).
**Edge case**: Long alias name works.

### QA-058: Resource type aliases :k8s and :kubernetes
**Priority**: Low
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Type `:k8s` and press Enter
2. Return to main, type `:kubernetes` and press Enter
**Expected**: Both navigate to EKS Clusters view.
**Edge case**: k8s community-standard aliases.

### QA-059: Resource type alias :sm (Secrets Manager)
**Priority**: Low
**Category**: Colon Commands
**Precondition**: App is on main menu
**Steps**:
1. Type `:sm` and press Enter
**Expected**: Navigates to Secrets Manager view.
**Edge case**: Short alias for verbose service name.

---

## 4. Resource List Views

### QA-060: View EC2 instances with real data
**Priority**: Critical
**Category**: Resource List Views
**Precondition**: AWS account has at least 3 EC2 instances in the active region
**Steps**:
1. Navigate to EC2 instances via `:ec2`
**Expected**: Table displays all EC2 instances with columns: Instance ID, Name, State, Type, Private IP, Public IP, Launch Time. Row count shown as `EC2 Instances (N)`. Cursor is on first row.
**Edge case**: Core data display validation.

### QA-061: View S3 buckets (global listing)
**Priority**: Critical
**Category**: Resource List Views
**Precondition**: AWS account has S3 buckets
**Steps**:
1. Navigate to S3 via `:s3`
**Expected**: Table displays all S3 buckets with columns: Bucket Name, Region, Creation Date. Note: S3 ListBuckets is global and returns all buckets regardless of selected region.
**Edge case**: S3 is unique in returning global results regardless of region parameter.

### QA-062: View RDS instances
**Priority**: High
**Category**: Resource List Views
**Precondition**: AWS account has at least 1 RDS instance
**Steps**:
1. Navigate to RDS via `:rds`
**Expected**: Table displays with columns: DB Identifier, Engine, Engine Version, Status, Class, Endpoint, Multi-AZ.
**Edge case**: Validates RDS-specific column rendering.

### QA-063: View ElastiCache Redis clusters
**Priority**: High
**Category**: Resource List Views
**Precondition**: AWS account has at least 1 Redis cluster
**Steps**:
1. Navigate to Redis via `:redis`
**Expected**: Table displays with columns: Cluster ID, Engine Version, Node Type, Status, Nodes, Endpoint. Only Redis engine clusters are shown (not Memcached).
**Edge case**: Validates Redis engine filtering.

### QA-064: View DocumentDB clusters
**Priority**: High
**Category**: Resource List Views
**Precondition**: AWS account has at least 1 DocumentDB cluster
**Steps**:
1. Navigate to DocumentDB via `:docdb`
**Expected**: Table displays with columns: Cluster ID, Engine Version, Status, Instance Count, Endpoint.
**Edge case**: Validates DocumentDB-specific columns.

### QA-065: View EKS clusters
**Priority**: High
**Category**: Resource List Views
**Precondition**: AWS account has at least 1 EKS cluster
**Steps**:
1. Navigate to EKS via `:eks`
**Expected**: Table displays with columns: Cluster Name, Version, Status, Endpoint, Platform Version.
**Edge case**: EKS requires both list and describe calls; validates two-phase fetch.

### QA-066: View Secrets Manager secrets
**Priority**: High
**Category**: Resource List Views
**Precondition**: AWS account has at least 1 secret in Secrets Manager
**Steps**:
1. Navigate to Secrets via `:secrets`
**Expected**: Table displays with columns: Secret Name, Description, Last Accessed, Last Changed, Rotation Enabled. Secret values are NOT shown in the list view.
**Edge case**: Validates that secret values are not accidentally exposed in list view.

### QA-067: Empty resource list (no instances in region)
**Priority**: High
**Category**: Resource List Views
**Precondition**: Switch to a region with no EC2 instances (e.g., `ap-northeast-3`)
**Steps**:
1. Navigate to EC2 via `:ec2`
**Expected**: After loading, message shows: `No ec2 resources found.` Status message shows: `No ec2 found in ap-northeast-3`.
**Edge case**: Empty results should show a clear message, not an error or blank screen.

### QA-068: Large list (100+ resources) -- scrolling behavior
**Priority**: High
**Category**: Resource List Views
**Precondition**: AWS account has 100+ EC2 instances
**Steps**:
1. Navigate to EC2 via `:ec2`
2. Press `G` to jump to bottom
3. Press `g` to jump to top
4. Hold `j` to scroll rapidly
**Expected**: All 100+ resources are loaded and displayed. Row count is accurate. Cursor can reach every row. Scrolling is smooth without lag. The terminal viewport scrolls to keep the cursor visible.
**Edge case**: Performance and usability at scale.

### QA-069: j/k navigation in resource list
**Priority**: Critical
**Category**: Resource List Views
**Precondition**: EC2 list has 5+ instances loaded
**Steps**:
1. Verify cursor starts at index 0
2. Press `j` three times
3. Press `k` once
**Expected**: Cursor is at index 2. Navigation wraps neither at top nor bottom (stops at bounds).
**Edge case**: Core resource list navigation.

### QA-070: Cursor stays in bounds after filter reduces list
**Priority**: High
**Category**: Resource List Views
**Precondition**: EC2 list has 10 instances loaded, cursor is at index 8
**Steps**:
1. Press `/` and type `prod` (which matches only 2 instances)
**Expected**: SelectedIndex is reset to 0 (the applyFilter method does this). Cursor is on the first filtered result. No out-of-bounds access.
**Edge case**: Cursor position must be clamped when the visible list shrinks.

### QA-071: Row count display accuracy with and without filter
**Priority**: Medium
**Category**: Resource List Views
**Precondition**: EC2 list has 10 instances, 3 of which match "prod"
**Steps**:
1. Observe row count: `EC2 Instances (10)`
2. Press `/` and type `prod`
**Expected**: Row count updates to `EC2 Instances (3/10) filter: prod` showing filtered count and total count.
**Edge case**: Validates accurate count display.

---

## 5. Profile & Region Switching

### QA-072: :ctx lists all profiles with current highlighted
**Priority**: Critical
**Category**: Profile & Region Switching
**Precondition**: `~/.aws/config` has profiles: default, dev, staging, prod. Active profile is `default`.
**Steps**:
1. Type `:ctx` and press Enter
**Expected**: ProfileSelectView shows all 4 profiles in a list. The `default` profile has a distinct visual highlight indicating it is the current active profile.
**Edge case**: Users must be able to identify the currently active profile at a glance.

### QA-073: Switch to a different profile
**Priority**: Critical
**Category**: Profile & Region Switching
**Precondition**: Active profile is `default`. `:ctx` view is showing.
**Steps**:
1. Navigate to `dev` profile using j/k
2. Press Enter
**Expected**: Header updates to show `profile: dev`. ActiveRegion updates to the region configured for the `dev` profile. View returns to main menu. Breadcrumbs reset to `main`. AWS clients are recreated with new credentials. Status message shows `Connected: dev / <region>`.
**Edge case**: Profile switch triggers full client recreation.

### QA-074: Switch to profile with SSO (expired token)
**Priority**: High
**Category**: Profile & Region Switching
**Precondition**: `~/.aws/config` has an SSO profile with an expired SSO session token
**Steps**:
1. Type `:ctx`, select the SSO profile, press Enter
**Expected**: Error message appears in status bar: "AWS config error: ..." with suggestion mentioning `aws sso login`. The app remains functional and navigable. Previous profile/region state is preserved or the error is clearly communicated.
**Edge case**: SSO token expiry is extremely common in enterprise environments.

### QA-075: :region lists all regions with current highlighted
**Priority**: Critical
**Category**: Profile & Region Switching
**Precondition**: Active region is `us-east-1`
**Steps**:
1. Type `:region` and press Enter
**Expected**: RegionSelectView shows all 27 AWS regions with both code and display name (e.g., `us-east-1  US East (N. Virginia)`). The `us-east-1` entry is visually highlighted.
**Edge case**: Region list is hardcoded and should be complete.

### QA-076: Switch to a different region
**Priority**: Critical
**Category**: Profile & Region Switching
**Precondition**: Active region is `us-east-1`. `:region` view is showing.
**Steps**:
1. Navigate to `eu-west-1` using j/k
2. Press Enter
**Expected**: Header updates to show `eu-west-1`. View returns to main menu. AWS clients are recreated for the new region. Status message shows `Connected: <profile> / eu-west-1`.
**Edge case**: Region switch triggers client recreation.

### QA-077: Switch profile then region, verify combined state
**Priority**: High
**Category**: Profile & Region Switching
**Precondition**: Active: default / us-east-1
**Steps**:
1. Type `:ctx`, select `dev`, press Enter
2. Type `:region`, select `eu-central-1`, press Enter
3. Navigate to `:ec2`
**Expected**: Header shows `profile: dev | eu-central-1`. EC2 instances are fetched using the dev profile credentials in the eu-central-1 region.
**Edge case**: Validates that profile and region switches are independent and compose correctly.

### QA-078: Cancel profile selection with Escape
**Priority**: High
**Category**: Profile & Region Switching
**Precondition**: App is in ProfileSelectView after typing `:ctx`
**Steps**:
1. Navigate to a different profile but do NOT press Enter
2. Press Escape
**Expected**: App returns to the previous view. Active profile is unchanged. No client recreation occurs.
**Edge case**: Escape in selector views should revert without side effects.

### QA-079: Cancel region selection with Escape
**Priority**: High
**Category**: Profile & Region Switching
**Precondition**: App is in RegionSelectView after typing `:region`
**Steps**:
1. Navigate to a different region but do NOT press Enter
2. Press Escape
**Expected**: App returns to the previous view. Active region is unchanged.
**Edge case**: Symmetry with profile cancel behavior.

### QA-080: :ctx when no profiles are configured
**Priority**: High
**Category**: Profile & Region Switching
**Precondition**: `~/.aws/config` and `~/.aws/credentials` do not contain any profiles
**Steps**:
1. Type `:ctx` and press Enter
**Expected**: Status bar shows error: `No AWS profiles found. Configure with: aws configure`. App stays on current view.
**Edge case**: Graceful handling of empty profile list.

### QA-081: Navigate region list with g/G
**Priority**: Medium
**Category**: Profile & Region Switching
**Precondition**: App is in RegionSelectView
**Steps**:
1. Press `G` to jump to the bottom of the region list
2. Press `g` to jump to the top
**Expected**: Cursor jumps to last region (sa-east-1 or il-central-1) then back to first (us-east-1). Note: The handleRegionSelectKeys only handles Up, Down, and Enter. g/G are NOT handled in region select view -- this is potentially a missing feature vs the spec.
**Edge case**: Spec says g/G work in all list/table views. Implementation may be missing g/G support in ProfileSelectView and RegionSelectView.

---

## 6. Resource Detail (d key)

### QA-082: d on EC2 instance shows all attributes
**Priority**: Critical
**Category**: Resource Detail
**Precondition**: EC2 list has instances loaded, cursor on an instance
**Steps**:
1. Press `d`
**Expected**: DetailView appears showing all instance attributes as key-value pairs (ID, name, state, type, AMI, VPC, subnet, security groups, tags, launch time, monitoring status, etc.). Title shows resource name. Breadcrumbs update to `main > EC2 Instances > detail`.
**Edge case**: Core describe functionality.

### QA-083: d on S3 bucket shows bucket details
**Priority**: High
**Category**: Resource Detail
**Precondition**: S3 bucket list loaded, cursor on a bucket
**Steps**:
1. Press `d`
**Expected**: DetailView shows bucket attributes (name, region, creation date, and any other available metadata). If DetailData is populated, the view renders all key-value pairs.
**Edge case**: S3 buckets may have fewer detail fields than EC2 instances.

### QA-084: d on RDS instance shows DB details
**Priority**: High
**Category**: Resource Detail
**Precondition**: RDS list loaded, cursor on an instance
**Steps**:
1. Press `d`
**Expected**: DetailView shows all RDS attributes (DB identifier, engine, version, status, class, endpoint, multi-AZ, VPC, security groups, etc.).
**Edge case**: Validates RDS detail data population.

### QA-085: d on ElastiCache Redis cluster
**Priority**: High
**Category**: Resource Detail
**Precondition**: Redis list loaded, cursor on a cluster
**Steps**:
1. Press `d`
**Expected**: DetailView shows Redis cluster attributes (cluster ID, version, node type, status, endpoint, etc.).
**Edge case**: Validates Redis detail data.

### QA-086: d on DocumentDB cluster
**Priority**: High
**Category**: Resource Detail
**Precondition**: DocumentDB list loaded, cursor on a cluster
**Steps**:
1. Press `d`
**Expected**: DetailView shows DocumentDB cluster attributes.
**Edge case**: Validates DocDB detail data.

### QA-087: d on EKS cluster
**Priority**: High
**Category**: Resource Detail
**Precondition**: EKS list loaded, cursor on a cluster
**Steps**:
1. Press `d`
**Expected**: DetailView shows EKS cluster attributes (name, version, status, endpoint, platform version, role ARN, VPC config, etc.).
**Edge case**: Validates EKS detail data.

### QA-088: d on Secrets Manager secret shows metadata only
**Priority**: Critical
**Category**: Resource Detail
**Precondition**: Secrets list loaded, cursor on a secret
**Steps**:
1. Press `d`
**Expected**: DetailView shows secret metadata (name, description, ARN, last accessed, last changed, rotation config). The secret VALUE is NOT displayed -- that requires the `x` key (Reveal).
**Edge case**: Security-critical: describe must not leak secret values.

### QA-089: Scroll up/down in detail view
**Priority**: High
**Category**: Resource Detail
**Precondition**: DetailView is showing a resource with 20+ attributes
**Steps**:
1. Press `j` 10 times to scroll down
2. Press `k` 5 times to scroll up
**Expected**: View scrolls smoothly through key-value pairs. Offset adjusts correctly. Cannot scroll past the first or last item.
**Edge case**: Detail view scrolling uses Offset-based rendering.

### QA-090: g/G in detail view (scroll to top/bottom)
**Priority**: Medium
**Category**: Resource Detail
**Precondition**: DetailView is showing
**Steps**:
1. Press `G` to scroll to bottom
2. Press `g` to scroll to top
**Expected**: Based on current implementation, `g` and `G` are NOT handled in handleDetailKeys (only Up and Down are). Pressing `g` or `G` in detail view has NO effect. This is a gap vs the spec which says scrollable views support g/G.
**Edge case**: Missing feature -- spec says g/G should work in scrollable views but implementation only handles j/k in detail/JSON/reveal views.

### QA-091: Escape from detail view returns to list with cursor preserved
**Priority**: Critical
**Category**: Resource Detail
**Precondition**: Resource list has 10 items, cursor was on item 5 before pressing `d`
**Steps**:
1. Press `d` to enter detail view
2. Press Escape
**Expected**: App returns to ResourceListView. Cursor is restored to index 5 (the position it was at before describe). This works because pushCurrentView saves CursorPos to the navigation history.
**Edge case**: Cursor position preservation is critical for efficient workflows.

### QA-092: d on empty list (no resource selected)
**Priority**: High
**Category**: Resource Detail
**Precondition**: EC2 list loaded but is empty (no instances in region)
**Steps**:
1. Press `d`
**Expected**: Nothing happens. The `handleResourceListKeys` checks `listLen > 0 && m.SelectedIndex < listLen` before processing describe. No crash, no error message.
**Edge case**: Prevents index-out-of-bounds on empty lists.

### QA-093: d when resource has no DetailData
**Priority**: Medium
**Category**: Resource Detail
**Precondition**: Resource list has an item whose DetailData map is nil or empty
**Steps**:
1. Select the resource and press `d`
**Expected**: Status message shows "No detail data available for this resource". View does not change. No error styling.
**Edge case**: Graceful fallback when detail data is not populated.

---

## 7. JSON View (y key)

### QA-094: y shows formatted JSON for EC2 instance
**Priority**: High
**Category**: JSON View
**Precondition**: EC2 list loaded, cursor on an instance that has RawJSON populated
**Steps**:
1. Press `y`
**Expected**: JSONView appears showing the raw API response as formatted JSON. Title shows resource name. Breadcrumbs update to `main > EC2 Instances > json`.
**Edge case**: Core JSON view functionality.

### QA-095: JSON is valid and parseable
**Priority**: High
**Category**: JSON View
**Precondition**: JSON view is displayed
**Steps**:
1. Copy the displayed JSON content
2. Validate it with a JSON parser
**Expected**: The JSON is valid, properly formatted, and parseable. No trailing commas, unclosed brackets, or encoding issues.
**Edge case**: Malformed JSON would be confusing for users who copy-paste it.

### QA-096: Scroll in JSON view
**Priority**: Medium
**Category**: JSON View
**Precondition**: JSON view is showing a large JSON document
**Steps**:
1. Press `j` repeatedly to scroll down
2. Press `k` to scroll back up
**Expected**: Scrolling works line-by-line. Cannot scroll past the top (offset stays >= 0) or beyond the content.
**Edge case**: JSON documents can be very large for resources with many tags/attributes.

### QA-097: Escape from JSON view returns to list
**Priority**: High
**Category**: JSON View
**Precondition**: JSON view is showing
**Steps**:
1. Press Escape
**Expected**: Returns to ResourceListView with cursor position preserved.
**Edge case**: Standard back navigation.

### QA-098: y when resource has no RawJSON
**Priority**: Medium
**Category**: JSON View
**Precondition**: Resource's RawJSON field is empty string
**Steps**:
1. Press `y`
**Expected**: Status message shows "No JSON data available for this resource". View does not change.
**Edge case**: Graceful fallback.

### QA-099: y on each of the 7 resource types
**Priority**: Medium
**Category**: JSON View
**Precondition**: Each resource type has at least one resource loaded
**Steps**:
1. For each resource type: navigate to list, select a resource, press `y`
**Expected**: JSON view renders for each type. The JSON structure reflects the AWS API response for that specific service.
**Edge case**: Ensures RawJSON is populated by all 7 fetch functions.

---

## 8. Secret Reveal (x key)

### QA-100: x on a secret shows the value
**Priority**: Critical
**Category**: Secret Reveal
**Precondition**: Secrets list loaded, cursor on a secret that has a SecretString value
**Steps**:
1. Press `x`
**Expected**: Loading indicator appears. Secret value is fetched from AWS. RevealView appears showing the plain text value. No confirmation dialog (per spec: "without requiring confirmation"). Breadcrumbs update to `main > Secrets Manager > reveal`.
**Edge case**: Core secret reveal functionality.

### QA-101: x on a non-secret resource type
**Priority**: High
**Category**: Secret Reveal
**Precondition**: EC2 list loaded, cursor on an instance
**Steps**:
1. Press `x`
**Expected**: Nothing happens. The implementation checks `m.CurrentResourceType == "secrets"` before processing the reveal action. No error message, no view change.
**Edge case**: x key is scoped to secrets only -- silent no-op elsewhere.

### QA-102: x when secret value is very long
**Priority**: Medium
**Category**: Secret Reveal
**Precondition**: Secret contains a 10KB JSON string as its value
**Steps**:
1. Navigate to secret, press `x`
**Expected**: RevealView shows the full value with scrolling capability. Performance is acceptable. No truncation.
**Edge case**: Secrets can contain large configuration files or certificates.

### QA-103: x when secret value is binary (SecretBinary)
**Priority**: Medium
**Category**: Secret Reveal
**Precondition**: Secret uses SecretBinary instead of SecretString
**Steps**:
1. Navigate to secret, press `x`
**Expected**: The reveal value should handle binary data gracefully -- either show a base64 representation or display an appropriate message. The behavior depends on the `RevealSecret` AWS client function.
**Edge case**: Not all secrets are string-typed.

### QA-104: x when AWS connection is nil
**Priority**: High
**Category**: Secret Reveal
**Precondition**: App has no AWS clients (Clients is nil)
**Steps**:
1. Navigate to secrets list (may show cached data or error)
2. Press `x`
**Expected**: Status message shows "No AWS connection; use :ctx to set profile" with error styling. No crash.
**Edge case**: Defensive nil check on Clients.

### QA-105: x when secret fetch fails (access denied)
**Priority**: High
**Category**: Secret Reveal
**Precondition**: IAM policy denies `secretsmanager:GetSecretValue` but allows `secretsmanager:ListSecrets`
**Steps**:
1. Navigate to secrets list (list loads successfully)
2. Select a secret and press `x`
**Expected**: SecretRevealedMsg arrives with Err set. Status bar shows "Error revealing secret: AccessDeniedException" or similar. Loading indicator is cleared. RevealView is NOT entered.
**Edge case**: Common IAM partial-permission scenario.

### QA-106: Copy secret from reveal view (c key)
**Priority**: Medium
**Category**: Secret Reveal
**Precondition**: RevealView is showing a secret value
**Steps**:
1. Press `c`
**Expected**: The full secret value content is copied to system clipboard. Status message shows "Secret copied to clipboard".
**Edge case**: The handleRevealKeys specifically handles `c` to copy the reveal content (not the ID).

---

## 9. Copy (c key)

### QA-107: c copies EC2 instance ID to clipboard
**Priority**: High
**Category**: Copy
**Precondition**: EC2 list loaded, cursor on instance `i-0abc123def`
**Steps**:
1. Press `c`
**Expected**: Instance ID `i-0abc123def` is copied to system clipboard. Status message shows "Copied: i-0abc123def".
**Edge case**: EC2 uses instance ID as the primary identifier.

### QA-108: c copies S3 bucket name to clipboard
**Priority**: High
**Category**: Copy
**Precondition**: S3 list loaded, cursor on bucket `my-app-data`
**Steps**:
1. Press `c`
**Expected**: Bucket name `my-app-data` is copied to clipboard. Status shows "Copied: my-app-data".
**Edge case**: S3 uses bucket name as ID.

### QA-109: c copies RDS DB identifier
**Priority**: Medium
**Category**: Copy
**Precondition**: RDS list loaded
**Steps**:
1. Press `c`
**Expected**: DB identifier is copied. Format depends on how the RDS fetch populates Resource.ID.
**Edge case**: Validates RDS ID format.

### QA-110: c copies various resource identifiers
**Priority**: Medium
**Category**: Copy
**Precondition**: Each resource type list is loaded
**Steps**:
1. For each of the 7 resource types, select a resource and press `c`
**Expected**: The correct identifier for each type is copied (instance ID for EC2, bucket name for S3, DB identifier for RDS, cluster ID for Redis/DocDB, cluster name for EKS, secret name/ARN for Secrets).
**Edge case**: Different resource types use different ID formats.

### QA-111: c on empty list
**Priority**: Medium
**Category**: Copy
**Precondition**: EC2 list is empty (no instances)
**Steps**:
1. Press `c`
**Expected**: Nothing happens. The guard `listLen > 0 && m.SelectedIndex < listLen` prevents any action.
**Edge case**: No crash on empty list.

### QA-112: c when clipboard is unavailable (SSH session)
**Priority**: Medium
**Category**: Copy
**Precondition**: Running in an SSH session without X11 forwarding or clipboard support
**Steps**:
1. Select a resource and press `c`
**Expected**: Status message shows "Copy failed: ..." with the clipboard error. StatusIsError is true. App continues to function.
**Edge case**: clipboard.WriteAll may fail in environments without display server access.

---

## 10. S3 Drill-Down

### QA-113: Enter on a bucket navigates into it
**Priority**: Critical
**Category**: S3 Drill-Down
**Precondition**: S3 bucket list loaded, cursor on bucket `my-data-bucket`
**Steps**:
1. Press Enter
**Expected**: S3Bucket is set to `my-data-bucket`. S3Prefix is `""`. Loading indicator appears. FetchS3Objects is called with bucket name and empty prefix. Breadcrumbs update to `main > S3 Buckets > my-data-bucket`. Objects/prefixes are displayed.
**Edge case**: Core S3 hierarchical browsing.

### QA-114: Folder-style navigation with prefixes
**Priority**: Critical
**Category**: S3 Drill-Down
**Precondition**: Inside a bucket, list shows folder `logs/` (ID ends with `/`)
**Steps**:
1. Navigate to `logs/` and press Enter
**Expected**: S3Prefix updates to `logs/`. FetchS3Objects is called with the new prefix. Breadcrumbs update to `main > S3 Buckets > my-data-bucket > logs/`. Objects within the `logs/` prefix are shown.
**Edge case**: Validates delimiter-based prefix navigation.

### QA-115: Enter on a nested folder
**Priority**: High
**Category**: S3 Drill-Down
**Precondition**: Inside `logs/` prefix, list shows `2024/` subfolder
**Steps**:
1. Navigate to `2024/` and press Enter
**Expected**: S3Prefix updates to `logs/2024/` (or the full prefix as returned by the ID field). Deeper level objects are shown. Breadcrumbs show the full path.
**Edge case**: Multi-level prefix navigation.

### QA-116: Enter on an S3 object (file, not folder)
**Priority**: High
**Category**: S3 Drill-Down
**Precondition**: Inside a bucket, cursor on an object `report.csv` (ID does NOT end with `/`)
**Steps**:
1. Press Enter
**Expected**: Nothing happens. The code checks `strings.HasSuffix(selected.ID, "/")` and only navigates for folders. Objects are not drillable.
**Edge case**: Only prefixes (folders) are navigable; pressing Enter on a file is a no-op.

### QA-117: Escape goes back to parent prefix
**Priority**: Critical
**Category**: S3 Drill-Down
**Precondition**: Inside bucket `my-data-bucket`, viewing objects at prefix `logs/2024/`
**Steps**:
1. Press Escape
**Expected**: goBack() restores the previous view state from history. S3Prefix reverts to `logs/` (or whatever the parent was). Breadcrumbs update accordingly.
**Edge case**: Back navigation through S3 prefix hierarchy.

### QA-118: Escape from root prefix goes back to bucket list
**Priority**: Critical
**Category**: S3 Drill-Down
**Precondition**: Inside bucket `my-data-bucket`, viewing root-level objects (prefix is `""`)
**Steps**:
1. Press Escape
**Expected**: goBack() returns to the S3 bucket list. S3Bucket is still set from the popped state, but the view shows the bucket list again.
**Edge case**: Exiting from bucket object view back to bucket list.

### QA-119: Breadcrumbs show full S3 path
**Priority**: High
**Category**: S3 Drill-Down
**Precondition**: Navigated into bucket `my-data-bucket` then into prefix `logs/2024/`
**Steps**:
1. Observe breadcrumbs
**Expected**: Breadcrumbs show `main > S3 Buckets > my-data-bucket > logs/2024/`. Each level of the S3 hierarchy is reflected.
**Edge case**: Validates S3-specific breadcrumb logic in updateBreadcrumbs.

### QA-120: Empty bucket (no objects)
**Priority**: High
**Category**: S3 Drill-Down
**Precondition**: S3 bucket exists but contains no objects
**Steps**:
1. Select the empty bucket and press Enter
**Expected**: After loading, message shows `No s3 resources found.` or similar empty state. No error.
**Edge case**: Empty buckets are common (newly created or cleaned up).

### QA-121: Bucket with thousands of objects (pagination)
**Priority**: High
**Category**: S3 Drill-Down
**Precondition**: S3 bucket has 5000+ objects in root prefix
**Steps**:
1. Select the bucket and press Enter
2. Wait for loading
3. Scroll through the list
**Expected**: Per spec, objects should be loaded in pages with the first page appearing immediately. The implementation should handle AWS pagination (S3 returns max 1000 keys per ListObjectsV2 call). All objects are eventually accessible.
**Edge case**: S3 can have millions of objects; pagination is critical for usability and memory.

### QA-122: d on an S3 object shows metadata
**Priority**: High
**Category**: S3 Drill-Down
**Precondition**: Inside a bucket, cursor on a specific object
**Steps**:
1. Press `d`
**Expected**: DetailView shows S3 object metadata: key name, size, content type, last modified, storage class, ETag, encryption status. If DetailData is not populated for S3 objects, status message shows "No detail data available".
**Edge case**: S3 object describe is explicitly called out in the spec.

---

## 11. Filter (/)

### QA-123: / activates filter mode
**Priority**: Critical
**Category**: Filter
**Precondition**: App is in ResourceListView
**Steps**:
1. Press `/`
**Expected**: FilterMode becomes true. Status bar shows `/` with cursor for text input. Filter is set to empty string.
**Edge case**: Basic filter activation.

### QA-124: / does NOT activate in main menu
**Priority**: High
**Category**: Filter
**Precondition**: App is on main menu
**Steps**:
1. Press `/`
**Expected**: Nothing happens. The implementation checks `m.CurrentView == ResourceListView` before entering filter mode. Filter is only available in resource list views.
**Edge case**: Filter is scoped to resource lists only, not the main menu or other views.

### QA-125: Typing filters in real time
**Priority**: Critical
**Category**: Filter
**Precondition**: EC2 list has 10 instances, some with "prod" in their name
**Steps**:
1. Press `/`
2. Type `p` -- observe filtering
3. Type `r` -- observe filtering narrows
4. Type `o` -- observe filtering narrows further
5. Type `d` -- final filter is "prod"
**Expected**: After each keystroke, applyFilter() is called and the list updates in real time. The filter matches against ID, Name, Status, and all Fields values. Only matching resources remain visible.
**Edge case**: Real-time incremental filtering.

### QA-126: Case-insensitive matching
**Priority**: High
**Category**: Filter
**Precondition**: Resources include "Prod-Web-1" and "STAGING-API"
**Steps**:
1. Press `/` and type `prod`
**Expected**: Matches "Prod-Web-1" regardless of case. The FilterResources function uses `strings.ToLower` for comparison.
**Edge case**: Case sensitivity is explicitly called out in the spec.

### QA-127: Filter across all visible columns
**Priority**: High
**Category**: Filter
**Precondition**: EC2 instances loaded, one instance has type "t3.xlarge" but name "web-server"
**Steps**:
1. Press `/` and type `t3.xlarge`
**Expected**: The instance with type "t3.xlarge" appears in filtered results because FilterResources matches against all Fields values, not just name or ID.
**Edge case**: Multi-column filtering is a key usability feature.

### QA-128: Filter matches ID field
**Priority**: Medium
**Category**: Filter
**Precondition**: EC2 instance with ID "i-0abc123"
**Steps**:
1. Press `/` and type `i-0abc`
**Expected**: Instance with that ID prefix appears in results.
**Edge case**: Validates ID-based search.

### QA-129: Filter matches status field
**Priority**: Medium
**Category**: Filter
**Precondition**: EC2 list has instances with various states
**Steps**:
1. Press `/` and type `stopped`
**Expected**: Only stopped instances appear.
**Edge case**: Status field matching.

### QA-130: No matches shows empty state with message
**Priority**: High
**Category**: Filter
**Precondition**: EC2 list has resources loaded
**Steps**:
1. Press `/` and type `zzzznonexistent`
**Expected**: Filtered list is empty. Display shows: `No ec2 resources matching filter: zzzznonexistent`. The current view remains a resource list (not an error).
**Edge case**: Empty filter results should communicate clearly why the list is empty.

### QA-131: Escape clears filter
**Priority**: Critical
**Category**: Filter
**Precondition**: Filter is active showing "prod", filtered results are displayed
**Steps**:
1. Press Escape
**Expected**: FilterMode becomes false. Filter is set to empty string. FilteredResources is set to nil. applyFilter() is called. SelectedIndex resets to 0. Full unfiltered list is displayed.
**Edge case**: Clean filter clearing behavior.

### QA-132: Enter confirms filter (exits filter mode but keeps filter active)
**Priority**: High
**Category**: Filter
**Precondition**: Filter "prod" is typed, filter mode is active
**Steps**:
1. Press Enter
**Expected**: FilterMode becomes false (no more text input). BUT Filter text "prod" is PRESERVED. The filtered results remain visible. User can navigate the filtered list with j/k.
**Edge case**: Important distinction: Enter locks in the filter; Escape clears it.

### QA-133: Filter persists after d (describe) and back
**Priority**: High
**Category**: Filter
**Precondition**: Filter "prod" is active in EC2 list, showing 3 filtered results
**Steps**:
1. Select a resource and press `d`
2. View the detail
3. Press Escape to go back
**Expected**: goBack() restores the view state from history, including the Filter field. The filtered list is restored with "prod" filter still active. Cursor position is preserved.
**Edge case**: The pushCurrentView saves Filter in the ViewState, and goBack restores it.

### QA-134: Filter clears when switching resource types
**Priority**: High
**Category**: Filter
**Precondition**: EC2 list is filtered by "prod"
**Steps**:
1. Type `:rds` and press Enter
**Expected**: Filter is set to "" and FilteredResources is set to nil (both explicitly cleared in executeCommand when navigating to a resource type). RDS list shows all instances unfiltered.
**Edge case**: Filter is resource-type-scoped; switching types clears it.

### QA-135: Filter with special characters
**Priority**: Medium
**Category**: Filter
**Precondition**: EC2 list loaded, some instances have names with dots, hyphens, or underscores
**Steps**:
1. Press `/` and type `web-1`
2. Clear and type `10.0.1`
3. Clear and type `my_bucket`
**Expected**: Each filter matches correctly using substring search. Special characters are treated literally (no regex interpretation).
**Edge case**: Filter is simple substring match, not regex. Characters like `.` are literal.

### QA-136: Filter on empty resource list
**Priority**: Medium
**Category**: Filter
**Precondition**: EC2 list loaded but empty (no instances in region)
**Steps**:
1. Press `/` and type `anything`
**Expected**: Filter mode activates and text is accepted, but the displayed list remains empty. No crash from filtering an empty slice.
**Edge case**: Filtering an already-empty list.

### QA-137: Backspace removes filter characters one by one
**Priority**: Medium
**Category**: Filter
**Precondition**: Filter "prod" is active
**Steps**:
1. Press Backspace once (filter becomes "pro")
2. Observe list updates
3. Press Backspace three more times
**Expected**: After first backspace: filter is "pro", list re-filters. After all backspaces: filter is empty, FilterMode exits automatically (same as command mode), full list is shown.
**Edge case**: Backspace to empty exits filter mode automatically.

---

## 12. Sorting (Shift+N/S/A)

### QA-138: N sorts by name
**Priority**: High
**Category**: Sorting
**Precondition**: EC2 list loaded with instances named "charlie", "alpha", "bravo"
**Steps**:
1. Press `N` (Shift+N)
**Expected**: Resources are sorted alphabetically by name: alpha, bravo, charlie. SelectedIndex resets to 0. Status message shows "Sorted by name".
**Edge case**: Validates name-based sorting.

### QA-139: S sorts by status/state
**Priority**: High
**Category**: Sorting
**Precondition**: EC2 list loaded with instances in various states (running, stopped, terminated)
**Steps**:
1. Press `S` (Shift+S)
**Expected**: Resources are sorted by status field. For EC2, the sort key is "state" (found via findColumnKeyBySubstr fallback to "state" when "status" not found). SelectedIndex resets to 0. Status message shows "Sorted by status".
**Edge case**: The sortResources function tries "status" first, then "state" as fallback for EC2.

### QA-140: A sorts by age/time
**Priority**: High
**Category**: Sorting
**Precondition**: EC2 list loaded with instances launched at different times
**Steps**:
1. Press `A` (Shift+A)
**Expected**: Resources are sorted by the time-related field. For EC2, this is "launch_time". For S3, "creation_date". For Secrets, "last_accessed" or "last_changed". SelectedIndex resets to 0. Status message shows "Sorted by age".
**Edge case**: The sort key discovery iterates through multiple time-related suffixes.

### QA-141: Sort + filter combined
**Priority**: High
**Category**: Sorting
**Precondition**: EC2 list has 20 instances, 5 match filter "prod"
**Steps**:
1. Press `/` and type `prod`, press Enter
2. Press `N` to sort by name
**Expected**: The underlying Resources array is sorted by name. FilteredResources is re-applied (filter is re-run on the now-sorted Resources). The 5 filtered "prod" instances are displayed in alphabetical name order.
**Edge case**: Sort operates on the full list and then filter is re-applied.

### QA-142: Sort stability
**Priority**: Low
**Category**: Sorting
**Precondition**: EC2 list has multiple instances with status "running"
**Steps**:
1. Press `S` to sort by status
2. Observe the order of "running" instances
**Expected**: Go's sort.Slice is NOT stable, so the relative order of instances with the same status is not guaranteed to be preserved. This is a known limitation.
**Edge case**: Users may expect stable sorting but the implementation does not guarantee it.

### QA-143: Sort on empty list
**Priority**: Medium
**Category**: Sorting
**Precondition**: EC2 list is empty
**Steps**:
1. Press `N`, `S`, or `A`
**Expected**: No crash. The sort functions handle empty slices gracefully. Status message may still show "Sorted by name" even for empty lists.
**Edge case**: Edge case for sort on nil/empty data.

### QA-144: Sort for resource type without matching column
**Priority**: Medium
**Category**: Sorting
**Precondition**: View a resource type and try to sort by a column it doesn't have
**Steps**:
1. Navigate to S3 buckets (no "status" or "state" column)
2. Press `S` to sort by status
**Expected**: findColumnKeyBySubstr returns "" for both "status" and "state". Falls back to sorting by Name field. Status message shows "Sorted by status" even though it fell back.
**Edge case**: Fallback sort behavior when the requested column doesn't exist.

### QA-145: Sort keys only work in resource list view
**Priority**: Medium
**Category**: Sorting
**Precondition**: App is on main menu
**Steps**:
1. Press `N`, `S`, or `A`
**Expected**: No effect. Sort keys are handled in handleResourceListKeys, which is only called when CurrentView is ResourceListView.
**Edge case**: Sort key scoping.

---

## 13. History Navigation ([ and ])

### QA-146: Navigate forward several views, [ goes back
**Priority**: High
**Category**: History Navigation
**Precondition**: App is on main menu
**Steps**:
1. Navigate to EC2 (`:ec2`)
2. Select an instance and press `d` (detail view)
3. Press `[` (history back)
**Expected**: Step 3: historyBack() calls goBack(), which pops the last state from history. App returns to EC2 resource list with cursor position preserved.
**Edge case**: Basic history back navigation.

### QA-147: ] goes forward after going back
**Priority**: High
**Category**: History Navigation
**Precondition**: After QA-146 step 3 (back to EC2 list)
**Steps**:
1. Press `]` (history forward)
**Expected**: historyForward() returns the previously popped state. App returns to detail view.
**Edge case**: Forward navigation after back.

### QA-148: History cleared after new navigation
**Priority**: High
**Category**: History Navigation
**Precondition**: On EC2 list, navigated back from detail view (forward history exists)
**Steps**:
1. Press Enter on a different resource (new navigation)
**Expected**: pushCurrentView() calls History.Push(), which clears the forward stack. Pressing `]` after this has no effect.
**Edge case**: New navigation should invalidate forward history (standard browser-like behavior).

### QA-149: Multiple back operations
**Priority**: Medium
**Category**: History Navigation
**Precondition**: main -> EC2 -> instance detail -> back to EC2 -> RDS -> cluster detail
**Steps**:
1. Press `[` (back to RDS list)
2. Press `[` (back to EC2 list)
3. Press `[` (back to main)
4. Press `[` (on main, should do nothing)
**Expected**: Each `[` press goes one step back. When on MainMenuView, goBack() returns without change.
**Edge case**: History stack depth and main menu boundary.

### QA-150: History after profile switch
**Priority**: Medium
**Category**: History Navigation
**Precondition**: Navigated to EC2 list, then switch profile via `:ctx`
**Steps**:
1. After profile switch, press `[`
**Expected**: History contains the state before the profile switch. Navigating back may restore the view but the AWS clients now use the new profile's credentials.
**Edge case**: Profile switch does not clear history in the current implementation. This could lead to confusing state if the user navigates back to a resource list that was loaded with a different profile.

### QA-151: History after region switch
**Priority**: Medium
**Category**: History Navigation
**Precondition**: Viewed EC2 in us-east-1, then switched to eu-west-1
**Steps**:
1. Press `[`
**Expected**: Similar to QA-150, history may contain stale views from the previous region. The data in the Resources slice is from the old region, but new fetches would use the new region.
**Edge case**: Stale history data after context change.

---

## 14. Refresh (Ctrl-R)

### QA-152: Ctrl-R on resource list reloads data
**Priority**: High
**Category**: Refresh
**Precondition**: EC2 list is loaded showing 5 instances
**Steps**:
1. Press Ctrl-R
**Expected**: Loading is set to true. fetchResources() is called again for the current resource type. After loading, ResourcesLoadedMsg arrives with fresh data. List updates to reflect any changes. Status message updates.
**Edge case**: Core manual refresh functionality (no auto-polling per spec).

### QA-153: Ctrl-R on main menu does nothing
**Priority**: Medium
**Category**: Refresh
**Precondition**: App is on main menu
**Steps**:
1. Press Ctrl-R
**Expected**: Nothing happens. The implementation checks `m.CurrentView != MainMenuView` before processing refresh.
**Edge case**: Refresh has no meaning on the main menu.

### QA-154: Loading indicator appears during refresh
**Priority**: Medium
**Category**: Refresh
**Precondition**: EC2 list loaded
**Steps**:
1. Press Ctrl-R
2. Observe the header immediately
**Expected**: Header shows `[loading...]` indicator. Content area shows loading message while data is being fetched.
**Edge case**: Visual feedback during API call.

### QA-155: Ctrl-R while already loading
**Priority**: Medium
**Category**: Refresh
**Precondition**: App is currently loading resources (Loading is true)
**Steps**:
1. Press Ctrl-R again during loading
**Expected**: Another fetchResources() command is dispatched. This could result in two concurrent API calls. The last ResourcesLoadedMsg to arrive will set the final state. No crash, but potentially redundant API call.
**Edge case**: No debounce on refresh. Multiple rapid Ctrl-R presses may create multiple inflight requests.

### QA-156: Ctrl-R in detail view
**Priority**: Medium
**Category**: Refresh
**Precondition**: Viewing EC2 instance detail (DetailView)
**Steps**:
1. Press Ctrl-R
**Expected**: The check `m.CurrentView != MainMenuView` passes, and `m.CurrentResourceType != ""` is true, so fetchResources() is called. However, the user is in DetailView and the refresh reloads the resource list data. The view stays on DetailView but the underlying Resources data is refreshed. This may or may not be the desired behavior.
**Edge case**: Refresh semantics in non-list views could be confusing.

### QA-157: Ctrl-R in ProfileSelectView or RegionSelectView
**Priority**: Low
**Category**: Refresh
**Precondition**: App is in profile or region selection
**Steps**:
1. Press Ctrl-R
**Expected**: CurrentResourceType may be set from a previous resource view. If so, fetchResources() is called, which is unexpected in a selector view. If CurrentResourceType is empty, nothing happens.
**Edge case**: Refresh in selector views may have unintended side effects.

---

## 15. Help (?)

### QA-158: ? shows help overlay
**Priority**: High
**Category**: Help
**Precondition**: App is in normal mode on any view
**Steps**:
1. Press `?`
**Expected**: ShowHelp becomes true. Help overlay renders showing all keybindings grouped into sections: Global (`:`, `/`, `?`, `Esc`, `[`, `]`, `Ctrl-R`, `Ctrl-C`), Navigation (`j/k`, `g/G`, `Enter`), Actions (`d`, `y`, `x`, `c`), Sorting (`N`, `S`, `A`). The overlay has a bordered box and reads "Press any key to close help".
**Edge case**: Help discoverability.

### QA-159: Help shows all keybindings
**Priority**: Medium
**Category**: Help
**Precondition**: Help overlay is displayed
**Steps**:
1. Read all listed keybindings
**Expected**: Every keybinding from the contracts/commands.md is represented: `:`, `/`, `?`, `Esc`, `[`, `]`, `Ctrl-R`, `Ctrl-C`, `j/Down`, `k/Up`, `g`, `G`, `Enter`, `d`, `y`, `x`, `c`, `N`, `S`, `A`. No keybindings are missing.
**Edge case**: Help completeness.

### QA-160: Press any key closes help
**Priority**: High
**Category**: Help
**Precondition**: Help overlay is showing
**Steps**:
1. Press any key (e.g., `a`, `j`, `Escape`, etc.)
**Expected**: ShowHelp is set to false. The help overlay disappears and the previous view is restored. The key that closes help does NOT trigger its normal action (handled by the "if m.ShowHelp" early return in handleNormalMode).
**Edge case**: Help closing swallows the key press -- it does not propagate.

### QA-161: ? again closes help
**Priority**: Medium
**Category**: Help
**Precondition**: Help overlay is showing
**Steps**:
1. Press `?` again
**Expected**: The `?` key is handled by `key.Matches(msg, m.Keys.Help)` which toggles ShowHelp. Since ShowHelp is true, it becomes false. Help overlay closes.
**Edge case**: Toggle behavior -- but note the implementation first checks for Help toggle, then checks ShowHelp. So pressing `?` while help is shown toggles it off (correct behavior).

### QA-162: Help from main menu
**Priority**: Medium
**Category**: Help
**Precondition**: App is on main menu
**Steps**:
1. Press `?`
**Expected**: Help overlay appears over the main menu content.
**Edge case**: Help works from any view.

### QA-163: Help from resource list
**Priority**: Medium
**Category**: Help
**Precondition**: App is on EC2 resource list
**Steps**:
1. Press `?`
**Expected**: Help overlay appears over the resource list content.
**Edge case**: Help works from resource list view.

### QA-164: Help from detail view
**Priority**: Low
**Category**: Help
**Precondition**: App is in detail view
**Steps**:
1. Press `?`
**Expected**: Help overlay appears over the detail content.
**Edge case**: Help works from detail view.

### QA-165: Help overlay respects terminal width
**Priority**: Low
**Category**: Help
**Precondition**: Terminal is 50 columns wide
**Steps**:
1. Press `?`
**Expected**: Help box width is capped at terminal width minus 4 (min 30). Content fits within the box without overflow.
**Edge case**: Responsive help layout.

---

## 16. Error Scenarios

### QA-166: Expired AWS credentials
**Priority**: Critical
**Category**: Error Scenarios
**Precondition**: AWS credentials have expired (temporary STS credentials past their expiry)
**Steps**:
1. Navigate to `:ec2`
**Expected**: APIErrorMsg is received. Error message contains "ExpiredToken". Status bar shows: "Error fetching ec2: credentials expired. Run: aws sso login". Loading indicator clears. App remains responsive and navigable.
**Edge case**: The most common AWS error in enterprise environments.

### QA-167: Access denied to a specific service
**Priority**: High
**Category**: Error Scenarios
**Precondition**: IAM policy allows EC2 but denies RDS describe
**Steps**:
1. Navigate to `:ec2` (works)
2. Navigate to `:rds`
**Expected**: EC2 loads successfully. RDS returns an AccessDeniedException. Status bar shows the error message. User can navigate away to other resource types. App does not crash.
**Edge case**: Partial permissions are common in least-privilege environments.

### QA-168: Network timeout
**Priority**: High
**Category**: Error Scenarios
**Precondition**: Network connectivity is interrupted after app launch
**Steps**:
1. Disconnect network
2. Navigate to `:ec2`
**Expected**: API call times out. APIErrorMsg is received with a timeout/network error. Status bar shows the error. App remains responsive -- user can switch profiles, navigate menus, etc.
**Edge case**: Network issues should not freeze the UI.

### QA-169: AWS API throttling (rate limit)
**Priority**: Medium
**Category**: Error Scenarios
**Precondition**: Many rapid API calls trigger AWS throttling
**Steps**:
1. Rapidly switch between resource types: `:ec2`, `:rds`, `:eks`, `:s3` in quick succession
2. Press Ctrl-R repeatedly
**Expected**: If AWS returns a throttling error, the APIErrorMsg handler displays it in the status bar. The app does not retry automatically (manual refresh only). UI remains responsive.
**Edge case**: Rate limiting is a real concern with manual refresh.

### QA-170: Invalid profile name via --profile flag
**Priority**: High
**Category**: Error Scenarios
**Precondition**: Profile "nonexistent" does not exist in AWS config
**Steps**:
1. Run `a9s --profile nonexistent`
**Expected**: App launches with `profile: nonexistent`. When attempting to connect (InitConnectMsg), awsclient.NewAWSSession fails. Status bar shows "AWS config error: ..." with helpful guidance. App remains on main menu, navigable.
**Edge case**: Typos in profile names are common.

### QA-171: Region with no support for a service
**Priority**: Medium
**Category**: Error Scenarios
**Precondition**: Select a region that doesn't support EKS (hypothetical or new region)
**Steps**:
1. Switch to the region
2. Navigate to `:eks`
**Expected**: AWS API returns a service-not-available error. Status bar shows the error. App allows navigation to other services that are available.
**Edge case**: Not all regions support all services.

### QA-172: Concurrent errors (switch resource type while loading)
**Priority**: Medium
**Category**: Error Scenarios
**Precondition**: App is loading EC2 instances (slow API call)
**Steps**:
1. While loading, type `:rds` and press Enter
**Expected**: A new loading command for RDS is dispatched. The EC2 ResourcesLoadedMsg may arrive later and could overwrite the current state. The implementation does not cancel inflight requests. The last message to arrive wins.
**Edge case**: No request cancellation mechanism. Race condition between old and new fetch results.

### QA-173: InitConnectMsg failure on startup
**Priority**: High
**Category**: Error Scenarios
**Precondition**: AWS config exists but credentials are completely invalid
**Steps**:
1. Run `a9s`
**Expected**: InitConnectMsg handler catches the error from NewAWSSession. Status message shows "AWS config error: ... Try: aws configure or aws sso login". Clients remains nil. App is on main menu and navigable. Resource commands will show "no AWS connection" errors.
**Edge case**: First-time connection failure.

---

## 17. Terminal Edge Cases

### QA-174: Very narrow terminal (40 columns)
**Priority**: Medium
**Category**: Terminal Edge Cases
**Precondition**: Terminal width is 40 columns
**Steps**:
1. Launch `a9s`
2. Navigate to EC2 instances
**Expected**: Header text is truncated or wraps within 40 columns. Table columns may be truncated. The app remains functional (no crash). Column widths in ResourceTypeDef add up to much more than 40 (EC2 totals 128 columns), so content will be clipped.
**Edge case**: Narrow terminals require graceful content truncation.

### QA-175: Very short terminal (10 rows)
**Priority**: Medium
**Category**: Terminal Edge Cases
**Precondition**: Terminal height is 10 rows
**Steps**:
1. Launch `a9s`
2. Navigate to EC2 instances
**Expected**: Header (1 line) + breadcrumbs (1 line) + status bar (1 line) = 3 lines overhead, leaving 7 lines for content. The resource list shows only a few items. Scrolling works correctly within the small viewport.
**Edge case**: Minimal vertical space.

### QA-176: Extremely small terminal (10x5)
**Priority**: Low
**Category**: Terminal Edge Cases
**Precondition**: Terminal is 10 columns x 5 rows
**Steps**:
1. Launch `a9s`
**Expected**: App renders without crashing. Content is heavily truncated. 5 rows may not be enough for header + breadcrumbs + content + status bar, but the app should handle it without panicking.
**Edge case**: Stress test for layout calculations.

### QA-177: Terminal resize during operation
**Priority**: High
**Category**: Terminal Edge Cases
**Precondition**: App is running at 120x40, viewing EC2 list
**Steps**:
1. Resize to 60x20
2. Resize to 200x60
**Expected**: Each resize sends a tea.WindowSizeMsg. Width and Height are updated. renderHeader uses Width for the header style. Help box width adjusts. No crash or garbled output.
**Edge case**: Dynamic resize handling.

### QA-178: NO_COLOR environment variable
**Priority**: Medium
**Category**: Terminal Edge Cases
**Precondition**: `export NO_COLOR=1`
**Steps**:
1. Launch `a9s`
2. Open help overlay
**Expected**: All color output is disabled. The help overlay uses a border without color (the help.go code checks `os.Getenv("NO_COLOR") != ""`). Status bar, header, cursor highlight -- all should be monochrome.
**Edge case**: Accessibility and pipeline-friendly output. Note: The main styles may not check NO_COLOR -- only the help overlay explicitly checks it. This could be a gap.

### QA-179: Non-256-color terminal
**Priority**: Low
**Category**: Terminal Edge Cases
**Precondition**: Terminal supports only 8 colors (e.g., `TERM=xterm`)
**Steps**:
1. Launch `a9s`
**Expected**: Colors degrade gracefully. The spec assumes 256-color support, but lipgloss should handle color downgrading. No crash.
**Edge case**: Not all terminal emulators support 256 colors.

### QA-180: SSH session (clipboard may not work)
**Priority**: Medium
**Category**: Terminal Edge Cases
**Precondition**: Running `a9s` over SSH without X11 forwarding
**Steps**:
1. Navigate to a resource and press `c`
**Expected**: clipboard.WriteAll fails. Status bar shows "Copy failed: ..." error. App continues to function. All other features work normally.
**Edge case**: Clipboard is environment-dependent.

### QA-181: Unicode in resource names/tags
**Priority**: Medium
**Category**: Terminal Edge Cases
**Precondition**: EC2 instance has Name tag containing Unicode characters (e.g., emoji, CJK characters, Arabic text)
**Steps**:
1. Navigate to EC2 list
**Expected**: Unicode characters render correctly in the terminal. Column alignment may be affected by variable-width Unicode characters (e.g., CJK characters are double-width). No crash.
**Edge case**: AWS allows arbitrary Unicode in tags.

### QA-182: Ctrl-C exits from any view
**Priority**: Critical
**Category**: Terminal Edge Cases
**Precondition**: App is in any state (loading, detail view, filter mode, command mode)
**Steps**:
1. Press Ctrl-C
**Expected**: Application exits immediately. The ForceQuit keybinding is checked before any other processing in the Update function. Exit code is 0.
**Edge case**: Emergency exit must always work regardless of application state.

---

## 18. State Consistency

### QA-183: Rapid command switching (:ec2, :rds, :eks in quick succession)
**Priority**: High
**Category**: State Consistency
**Precondition**: App is on main menu with AWS credentials
**Steps**:
1. Type `:ec2` and press Enter
2. Immediately type `:rds` and press Enter
3. Immediately type `:eks` and press Enter
**Expected**: Each command triggers a new fetch. The final view should be EKS. However, if EC2 or RDS fetch completes after EKS fetch starts, the ResourcesLoadedMsg for EC2/RDS could overwrite the Resources slice with stale data. The ResourceType field in ResourcesLoadedMsg should be checked against CurrentResourceType.
**Edge case**: Current implementation does NOT check if the arriving ResourcesLoadedMsg matches the current resource type. A slow EC2 response arriving after switching to EKS would overwrite Resources with EC2 data while showing EKS breadcrumbs. This is a potential race condition bug.

### QA-184: Profile switch while loading resources
**Priority**: High
**Category**: State Consistency
**Precondition**: App just executed `:ec2` and is loading (API call inflight)
**Steps**:
1. Type `:ctx`, select a different profile, press Enter
**Expected**: ProfileSwitchedMsg arrives, recreates clients, resets view to main menu. The inflight EC2 request may still complete and send a ResourcesLoadedMsg. Since the Clients pointer has changed, the response is from the OLD profile's credentials but will be stored in Resources regardless. This is a state consistency concern.
**Edge case**: No cancellation of inflight requests during profile switch.

### QA-185: Region switch while in detail view
**Priority**: Medium
**Category**: State Consistency
**Precondition**: Viewing EC2 instance detail in us-east-1
**Steps**:
1. Type `:region`, select `eu-west-1`, press Enter
**Expected**: RegionSwitchedMsg arrives. Clients are recreated for eu-west-1. View is set to MainMenuView. The detail view content was from us-east-1 and is now gone. Header shows `eu-west-1`.
**Edge case**: Region switch from a deep view should cleanly reset navigation.

### QA-186: Filter active then profile switch -- filter reapplied?
**Priority**: Medium
**Category**: State Consistency
**Precondition**: EC2 list is filtered by "prod" in us-east-1
**Steps**:
1. Type `:ctx`, select a different profile, press Enter
2. Navigate back to `:ec2`
**Expected**: After profile switch, view goes to main menu and filter is not preserved (new navigation to EC2 clears filter). The filter from the previous profile session is lost.
**Edge case**: Filters should not leak across profile switches.

### QA-187: Switching from S3 object view to another resource type
**Priority**: Medium
**Category**: State Consistency
**Precondition**: Inside S3 bucket, viewing objects at prefix `logs/2024/`
**Steps**:
1. Type `:ec2` and press Enter
**Expected**: S3Bucket and S3Prefix should be cleared (executeCommand sets both to ""). EC2 list loads normally. If user returns to `:s3` later, it starts fresh at the bucket list (not inside the previous bucket).
**Edge case**: S3 browsing state is correctly reset when switching resource types.

### QA-188: Command mode does not pass keys to underlying view
**Priority**: High
**Category**: State Consistency
**Precondition**: App is on main menu, user presses `:` to enter command mode
**Steps**:
1. While in command mode, type `j`, `k`, `g`, `d`
**Expected**: These characters are appended to CommandText, not interpreted as navigation or action keys. The handleCommandMode function is called before any view-specific handlers. Status bar shows `:jkgd`.
**Edge case**: Command mode input isolation.

### QA-189: Filter mode does not pass keys to underlying view
**Priority**: High
**Category**: State Consistency
**Precondition**: App is on EC2 list, user presses `/` to enter filter mode
**Steps**:
1. While in filter mode, type `j`, `k`, `d`, `y`, `c`, `x`
**Expected**: These characters are appended to the Filter string, not interpreted as navigation or action keys. The handleFilterMode function handles all keystrokes.
**Edge case**: Filter mode input isolation.

### QA-190: Loading state clears on both success and error
**Priority**: High
**Category**: State Consistency
**Precondition**: App is loading resources (Loading is true)
**Steps**:
1. Wait for ResourcesLoadedMsg or APIErrorMsg to arrive
**Expected**: In both cases, Loading is set to false. The loading indicator disappears from the header. If neither message arrives (lost command), loading indicator would persist indefinitely.
**Edge case**: Loading flag must always be cleared to avoid permanent loading state.

### QA-191: SelectedIndex consistency after sort
**Priority**: Medium
**Category**: State Consistency
**Precondition**: EC2 list with 10 items, cursor at index 7, filter inactive
**Steps**:
1. Press `N` to sort by name
**Expected**: SelectedIndex is reset to 0 (explicitly in handleResourceListKeys after sort). The cursor moves to the first item in the newly sorted list. Previous cursor position is lost.
**Edge case**: Sort resets cursor -- users may lose their place.

### QA-192: Escape from MainMenuView does nothing
**Priority**: Medium
**Category**: State Consistency
**Precondition**: App is on main menu
**Steps**:
1. Press Escape
**Expected**: goBack() is called. Since CurrentView is MainMenuView, it returns immediately with no change. App stays on main menu.
**Edge case**: Escape at the top level should not crash or exit.

### QA-193: Multiple filter entries in succession
**Priority**: Medium
**Category**: State Consistency
**Precondition**: EC2 list loaded
**Steps**:
1. Press `/`, type `prod`, press Enter (locks filter)
2. Press `/`, type `web` (new filter replaces old)
**Expected**: When `/` is pressed the second time, Filter is reset to "" (in handleNormalMode, filter mode sets Filter to ""). New filter "web" is applied. The old "prod" filter is discarded. applyFilter runs against full Resources, not previously filtered results.
**Edge case**: Each filter activation starts fresh against the full dataset.

### QA-194: View renders correctly after every state transition
**Priority**: High
**Category**: State Consistency
**Precondition**: App is running
**Steps**:
1. Execute a sequence: Main -> `:ec2` -> select instance -> `d` -> Escape -> `y` -> Escape -> `:s3` -> Enter bucket -> `d` -> Escape -> Escape -> Escape -> `:ctx` -> select -> `:region` -> select -> `:secrets` -> `x` -> Escape -> `:main`
**Expected**: At every step, the View() function renders correctly: header shows correct profile/region, breadcrumbs reflect current position, content area shows the appropriate view, status bar shows correct context. No lingering state from previous views.
**Edge case**: Complex state machine traversal.

---

## 19. Spec Compliance Issues Found During Review

### QA-195: Case sensitivity gap for built-in commands
**Priority**: High
**Category**: Spec Compliance
**Precondition**: App is running
**Steps**:
1. Type `:MAIN` and press Enter
2. Type `:CTX` and press Enter
3. Type `:QUIT` and press Enter
**Expected per spec**: Commands should work (spec says "commands are case-insensitive"). **Actual expected behavior**: These will fail with "Unknown command" because `executeCommand` uses exact string comparison (`case "main", "root":`) for built-in commands. Only resource-type commands go through case-insensitive `FindResourceType`.
**Edge case**: Discrepancy between spec and implementation. Resource commands (:ec2, :S3) are case-insensitive, but built-in commands (:main, :ctx, :region, :q, :quit, :root) are case-sensitive.

### QA-196: Missing g/G support in scrollable views
**Priority**: Medium
**Category**: Spec Compliance
**Precondition**: App is in DetailView, JSONView, or RevealView
**Steps**:
1. Press `g` to scroll to top
2. Press `G` to scroll to bottom
**Expected per spec**: g/G should jump to top/bottom in all scrollable views. **Actual expected behavior**: g/G are not handled in handleDetailKeys, handleJSONViewKeys, or handleRevealKeys. Only j/k (scroll up/down line by line) work.
**Edge case**: Spec says scrollable views support g/G but implementation doesn't handle them.

### QA-197: Missing g/G support in ProfileSelectView and RegionSelectView
**Priority**: Medium
**Category**: Spec Compliance
**Precondition**: App is in profile or region selector
**Steps**:
1. Press `g` or `G`
**Expected per spec**: Should jump to top/bottom of list. **Actual expected behavior**: Only Up, Down, and Enter are handled in handleProfileSelectKeys and handleRegionSelectKeys.
**Edge case**: Selector views are missing standard navigation shortcuts.

### QA-198: Auto-suggestion rendering not connected to main view
**Priority**: Low
**Category**: Spec Compliance
**Precondition**: App is in command mode
**Steps**:
1. Type `e`
**Expected per spec**: Auto-suggestions appear while typing. The CommandInput.View() method renders suggestions. **Actual expected behavior**: The status bar in renderStatusBar uses `fmt.Sprintf(":%s", m.CommandText)` directly and does NOT use CommandInput.View(). The auto-suggestion UI is implemented in CommandInput but not wired into the main app's View() method.
**Edge case**: Auto-suggestion feature exists but may not be visually rendered.

### QA-199: Status error message auto-clear after 5 seconds
**Priority**: Medium
**Category**: Spec Compliance
**Precondition**: An error message is displayed in the status bar
**Steps**:
1. Trigger an error (e.g., unknown command)
2. Wait 5 seconds
**Expected per spec**: Error messages should "auto-clear after 5 seconds". **Actual expected behavior**: There is no timer or delayed message in the implementation. Error messages persist until overwritten by another action.
**Edge case**: Spec calls for auto-clearing but implementation does not implement it.

### QA-200: S3 listing returns global results regardless of region
**Priority**: Medium
**Category**: Spec Compliance
**Precondition**: Switch to eu-west-1, navigate to `:s3`
**Steps**:
1. Observe bucket list
**Expected per spec**: S3 ListBuckets is global and returns all buckets regardless of region. The app should display all buckets even though region is eu-west-1. Status message should NOT say "No s3 found in eu-west-1" if there are buckets in other regions.
**Edge case**: Region-agnostic behavior for S3 is a special case.

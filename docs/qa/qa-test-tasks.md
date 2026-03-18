# QA Test Implementation Plan

Assigns every QA story from docs/qa/ (01 through 09) to a specific test file.
Each agent runs independently in parallel; no cross-agent dependencies.

**Test harness pattern (shared by all agents):**

```go
// Create root model, set terminal size, get rendered output
m := tui.New("testprofile", "us-east-1")
m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

// Send a message and inspect output
m, cmd = m.Update(someMsg)
plain := stripANSI(m.View().Content)

// Load fixture data into a resource list view
m, _ = m.Update(messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: fixtureEC2Instances()})
```

**Available helpers** (in `tests/unit/helpers_test.go`):
- `stripANSI(s string) string` -- remove ANSI escape codes for content assertions
- `lipglossWidth(s string) int` -- measure visible width of a styled string

**Available fixture functions** (in `tests/unit/fixtures_test.go`):
- `fixtureS3Buckets()` -- 5 S3 buckets (no status)
- `fixtureS3Objects()` -- 1 S3 object
- `fixtureEC2Instances()` -- 6 EC2 instances (5 running, 1 terminated; mixed public IPs)
- `fixtureRDSInstances()` -- 2 RDS instances (both available; docdb + aurora-postgresql)
- `fixtureRedisClusters()` -- 1 Redis cluster (available, empty endpoint)
- `fixtureDocDBClusters()` -- 2 DocumentDB clusters (both available)
- `fixtureEKSClusters()` -- 1 EKS cluster (ACTIVE)
- `fixtureSecrets()` -- 5 secrets (no rotation, no descriptions)

**Key imports for all test files:**
```go
package unit

import (
    "strings"
    "testing"

    tea "charm.land/bubbletea/v2"

    "github.com/k2m30/a9s/internal/resource"
    "github.com/k2m30/a9s/internal/tui"
    "github.com/k2m30/a9s/internal/tui/messages"
)
```

---

## Parallel Group 1: All 9 agents run simultaneously (no inter-dependencies)

---

### Agent QA-1: Main Menu Stories

- **Source:** `docs/qa/01-main-menu.md`
- **Output:** `tests/unit/qa_mainmenu_test.go`
- **Stories to cover:**
  - **A. Resource Type Listing** (6 stories): All 7 types displayed, shortname aliases (:ec2, :s3, etc.), dimmed alias styling, first row selected by default, selected row blue background, non-selected rows normal styling
  - **B. Navigation** (11 stories): j/down, k/up, g/G jump, cursor wrap top-to-bottom and bottom-to-top, Enter opens correct resource list for each type
  - **C. Filter Mode** (10 stories): / enters filter, typing narrows list, case-insensitive, filter text in header, frame title count updates, backspace, Esc clears, Enter confirms, no matches shows empty, selection resets to first visible
  - **D. Command Mode** (14 stories): : enters command mode, :ec2/:s3/:rds/:redis/:docdb/:eks/:secrets navigate, :q/:quit quit, :ctx/:region open selectors, unknown command error flash, Esc cancels, command text in header
  - **E. Help Overlay** (8 stories): ? opens help, four-column layout, navigation keys listed, general keys listed, any key closes, Esc closes, green key hints, "Press any key to close" hint
  - **F. Quit** (5 stories): q quits, ctrl+c force quits, ctrl+c from any state, q does not quit in filter mode, q does not quit in command mode
  - **G. Header Bar** (12 stories): app name accent, version dimmed, profile:region, "? for help" in normal, filter/command text in header, flash messages, flash auto-clear, header one line, header full width
  - **H. Frame / Border** (6 stories): single-line border, title centered, dash padding balanced, frame fills vertical space, dim border color, rows bounded by vertical bars
  - **I. Terminal Size** (6 stories): narrow <60 error, short <7 error, resize restores, exactly 60 OK, exactly 7 OK, narrow omits help hint
  - **J. Edge Cases** (12 stories): filter then help then back, rapid keypresses, multiple j then g, Enter after filter, command mode overrides keys, filter mode overrides keys, Esc no-op, header mode transitions, single input mode, window resize during filter, g/G persistence, consecutive Enter

- **Fixtures needed:** None for most tests (main menu uses hardcoded resource types). Use `fixtureEC2Instances()` only for tests that Enter into a resource list to verify navigation.
- **Views/models to test:** `tui.Model` (root), main menu view
- **Key assertions:**
  - `strings.Contains(plain, "EC2 Instances")` and all 7 types
  - `strings.Contains(plain, ":ec2")` for alias presence
  - `strings.Contains(plain, "resource-types(7)")` for frame title
  - `strings.Contains(plain, "resource-types(1/7)")` for filtered count
  - `strings.Contains(plain, "? for help")` for normal header
  - `strings.Contains(plain, "/filtertext")` for filter header
  - `strings.Contains(plain, ":cmdtext")` for command header
  - Line count == terminal height (24)
  - `lipglossWidth(line) <= terminalWidth` for every line
- **Edge cases:**
  - WindowSizeMsg with Width < 60 should produce "Terminal too narrow"
  - WindowSizeMsg with Height < 7 should produce "Terminal too short"
  - Width == 0 should produce empty View()
  - Consecutive mode switches (filter -> esc -> command -> esc)
  - Send tea.KeyPressMsg{Code: tea.KeyCtrlC} to verify quit

- **Test pattern:**
  1. `m := newRootSizedModel()` (80x24)
  2. For navigation: send `rootKeyPress("j")` / `rootKeyPress("k")` / `rootSpecialKey(tea.KeyEnter)`
  3. For filter: send `rootKeyPress("/")`, then individual characters, then `rootSpecialKey(tea.KeyEscape)`
  4. For command: send `rootKeyPress(":")`, then characters, then `rootSpecialKey(tea.KeyEnter)`
  5. Assert on `stripANSI(rootViewContent(m))`

---

### Agent QA-2: S3 Views Stories

- **Source:** `docs/qa/02-s3-views.md`
- **Output:** `tests/unit/qa_s3_test.go`
- **Stories to cover:**
  - **A. S3 Bucket List** (16 sections, ~50 stories): loading state, empty state, column layout (Bucket Name + Creation Date), frame title "s3(N)", navigation j/k/g/G/PageDown/PageUp, sort N/A, filter /, Enter drills into bucket objects, d opens detail, y opens YAML, c copies, ctrl+r refresh, Esc back, help ?, command mode :, row coloring (no status = plain)
  - **B. S3 Object List** (~30 stories): loading state, empty state, columns (Key/Size/Last Modified), frame title, folder navigation with common prefixes, Enter on object opens detail, navigation, horizontal scroll h/l, filter, sort, copy, detail d and YAML y, refresh, Esc back
  - **C. S3 Detail View** (~10 stories): bucket detail fields (BucketArn, BucketRegion, CreationDate), object detail fields (Name, LastModified, Owner), navigation, actions (y/c/w/Esc)
  - **D. Cross-Cutting** (~5 stories): header consistency across S3 views, view stack Main->Buckets->Objects->Detail->YAML and back, terminal resize, alternating row colors

- **Fixtures needed:** `fixtureS3Buckets()` (5 buckets), `fixtureS3Objects()` (1 object)
- **Views/models to test:** `tui.Model` root, navigated to S3 resource list, S3 object list, detail, YAML
- **Key assertions:**
  - `strings.Contains(plain, "s3(5)")` for frame title with 5 buckets
  - `strings.Contains(plain, "auth-service-dev-state")` for first bucket name
  - `strings.Contains(plain, "Bucket Name")` for column headers
  - `strings.Contains(plain, "Creation Date")` for column headers
  - `strings.Contains(plain, "s3(2/5)")` when filter matches 2
  - After Enter on bucket, frame should not show "yaml" or detail indicators
  - After d on bucket, detail fields present
  - After y on bucket, "yaml" in frame title
  - After Esc from objects, back to bucket list s3(5)
- **Edge cases:**
  - Empty bucket list: `ResourcesLoadedMsg{Resources: []resource.Resource{}}`
  - S3 bucket with no status (all rows plain color)
  - Bucket name longer than 20 chars (truncation)
  - Object list after drilling into bucket via `S3EnterBucketMsg`

- **Test pattern:**
  1. Navigate to S3: `rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "s3"})`
  2. Load buckets: `rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: fixtureS3Buckets()})`
  3. Enter bucket: send Enter key, execute returned cmd, process S3EnterBucketMsg
  4. Load objects: `rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3_objects", Resources: fixtureS3Objects()})`

---

### Agent QA-3: EC2 Views Stories

- **Source:** `docs/qa/03-ec2-views.md`
- **Output:** `tests/unit/qa_ec2_test.go`
- **Stories to cover:**
  - **A. EC2 Instance List** (~60 stories): column layout (6 columns: Instance ID, State, Type, Private IP, Public IP, Launch Time), frame title, header bar, status coloring (running=green, stopped=red, pending=yellow, terminated=dim), row selection (blue overrides), navigation j/k/g/G/h/l, sort N/S/A, filter /, command mode, actions (Enter/d -> detail, y -> YAML, c -> copy, Esc -> back, ? -> help, ctrl+r -> refresh), edge cases (missing public IP, no name tag, terminated visible, very long ID), empty state, loading state, responsive width breakpoints
  - **B. EC2 Detail View** (~30 stories): 13 fields in order (InstanceId through Tags), State nested struct rendering, SecurityGroups array rendering, Tags array rendering, key-value formatting, scrolling, word wrap toggle, actions (y/c/Esc/?/ctrl+r), edge cases (nil public IP, nil Platform, empty SecurityGroups)
  - **C. EC2 YAML View** (~20 stories): full YAML dump, syntax coloring (blue keys, green strings, orange numbers, purple bools, dim null), nested indentation, array rendering, scroll, actions (Esc/c/?/ctrl+r), edge cases (no tags, no public IP, many BlockDeviceMappings)
  - **D. Cross-View Navigation** (8 stories): List->Detail->YAML->Esc->Detail->Esc->List->Esc->MainMenu, direct y from list, filtered row opens correct detail

- **Fixtures needed:** `fixtureEC2Instances()` (6 instances: 5 running + 1 terminated, mixed IPs and names)
- **Views/models to test:** Root model navigated to EC2 list, EC2 detail, EC2 YAML
- **Key assertions:**
  - `strings.Contains(plain, "ec2-instances(6)")` for frame title
  - `strings.Contains(plain, "i-0aaa111111111111a")` for first instance ID
  - `strings.Contains(plain, "Instance ID")` column header
  - `strings.Contains(plain, "running")` in list rows
  - `strings.Contains(plain, "terminated")` for terminated instance
  - After d/Enter: detail view with instance ID in frame title
  - After y: "yaml" in frame title
  - Verify instance with empty public_ip ("kafka", "monitoring") renders correctly
  - Verify terminated instance ("apps") renders with status dim
- **Edge cases:**
  - Instance with empty name ("i-0aaa111111111111a" has Name="")
  - Instance with empty public_ip and empty private_ip (terminated "apps")
  - Filter by "kafka" matches single instance
  - Filter by "10.0" matches by IP substring
  - Sort by N toggles ascending/descending

- **Test pattern:**
  1. Navigate to EC2: `messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"}`
  2. Load fixtures: `messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: fixtureEC2Instances()}`
  3. Navigate j to select different rows, verify with View()
  4. Press d/Enter for detail, verify detail content
  5. Press y for YAML, verify "yaml" in frame

---

### Agent QA-4: RDS Views Stories

- **Source:** `docs/qa/04-rds-views.md`
- **Output:** `tests/unit/qa_rds_test.go`
- **Stories to cover:**
  - **A. RDS List** (~40 stories): navigation to RDS list via menu and :rds, 7 columns (DB Identifier, Engine, Version, Status, Class, Endpoint, Multi-AZ), data mapping, status coloring (available=green, stopped=red, creating=yellow, failed=red), edge cases (null endpoint, multi-engine mix, Aurora, long identifiers), frame title, sorting N/S/A, filtering /, navigation j/k/g/G, loading/error states, header, help
  - **B. RDS Detail** (~20 stories): 10 fields (DBInstanceIdentifier through AvailabilityZone), Endpoint nested object (Address/Port/HostedZoneId), formatting (key blue, value white), scrolling, actions (c/y/w/?/ctrl+r/Esc), edge cases (creating instance with null endpoint, narrow terminal)
  - **C. RDS YAML** (~25 stories): full dump, syntax coloring, YAML structure (Endpoint nested, VpcSecurityGroups array, DBSubnetGroup nested with Subnets array, TagList array), scrolling, actions, edge cases (creating state, many tags, Aurora cluster)
  - **D. Cross-View** (7 stories): List->Detail->YAML and back, command mode from RDS views, filter does not persist to detail

- **Fixtures needed:** `fixtureRDSInstances()` (2 instances: docdb engine + aurora-postgresql)
- **Views/models to test:** Root model navigated to RDS list, RDS detail, RDS YAML
- **Key assertions:**
  - `strings.Contains(plain, "rds-instances(2)")` for frame title
  - `strings.Contains(plain, "docdb-docdb-dev")` for first DB identifier
  - `strings.Contains(plain, "DB Identifier")` column header
  - `strings.Contains(plain, "aurora-postgresql")` for engine
  - `strings.Contains(plain, "available")` for status
  - `strings.Contains(plain, "db.r5.large")` for class
  - `strings.Contains(plain, "No")` for Multi-AZ value
  - After Enter/d: detail with DB identifier in title
  - After y: YAML view with "yaml" in title
- **Edge cases:**
  - Both instances are "available" (no stopped/creating to test color variations from fixture data; test with synthetic data if needed)
  - Long endpoint address truncation in list column
  - Filter by "aurora" matches second instance only

- **Test pattern:**
  1. Navigate to RDS: `messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "rds"}`
  2. Load: `messages.ResourcesLoadedMsg{ResourceType: "rds", Resources: fixtureRDSInstances()}`
  3. Verify columns, then navigate to detail/YAML

---

### Agent QA-5: Redis & DocumentDB Stories

- **Source:** `docs/qa/05-redis-docdb-views.md`
- **Output:** `tests/unit/qa_redis_docdb_test.go`
- **Stories to cover:**
  - **Redis List** (12 stories): REDIS-LIST-01 through REDIS-LIST-12 -- navigation, columns (Cluster ID, Version, Node Type, Status, Nodes, Endpoint), row count in title, status coloring, cursor navigation, horizontal scroll, filter, sort, copy, empty state, null ConfigurationEndpoint
  - **Redis Detail** (8 stories): REDIS-DETAIL-01 through REDIS-DETAIL-08 -- 8 fields (CacheClusterId through PreferredAvailabilityZone), ConfigurationEndpoint nested, status coloring, scroll, copy, back navigation, null endpoint handling
  - **Redis YAML** (8 stories): REDIS-YAML-01 through REDIS-YAML-08 -- full resource YAML, syntax coloring, all available fields, scroll, copy, back navigation, nested arrays
  - **DocumentDB List** (13 stories): DOCDB-LIST-01 through DOCDB-LIST-13 -- navigation, columns (Cluster ID, Version, Status, Instances, Endpoint), member count display, status coloring, cursor navigation, horizontal scroll, filter, sort, copy, empty state
  - **DocumentDB Detail** (10 stories): DOCDB-DETAIL-01 through DOCDB-DETAIL-10 -- 9 fields (DBClusterIdentifier through DBClusterMembers), DBClusterMembers array rendering, writer/reader distinction, status coloring, both endpoints, scroll, copy, back, empty members
  - **DocumentDB YAML** (8 stories): DOCDB-YAML-01 through DOCDB-YAML-08 -- full YAML, syntax coloring, all fields, DBClusterMembers array, scroll, copy, back navigation
  - **Cross-Cutting** (6 stories): help overlay, command mode switch between Redis and DocDB, error handling, refresh

- **Fixtures needed:** `fixtureRedisClusters()` (1 cluster, available, empty endpoint), `fixtureDocDBClusters()` (2 clusters, available)
- **Views/models to test:** Root model navigated to redis list, redis detail, redis YAML, docdb list, docdb detail, docdb YAML
- **Key assertions:**
  - `strings.Contains(plain, "redis(1)")` for Redis frame title
  - `strings.Contains(plain, "elasticache-dev")` for Redis cluster ID
  - `strings.Contains(plain, "7.0.7")` for Redis engine version
  - `strings.Contains(plain, "cache.t2.micro")` for Redis node type
  - `strings.Contains(plain, "docdb(2)")` for DocDB frame title
  - `strings.Contains(plain, "docdb-cluster-dev")` for DocDB cluster ID
  - `strings.Contains(plain, "5.0.0")` for DocDB engine version
  - Empty endpoint renders as blank, not "null" or error
  - After Enter/d on Redis: detail with cluster ID in title
  - After Enter/d on DocDB: detail with cluster ID in title
- **Edge cases:**
  - Redis cluster with empty endpoint (fixtureRedisClusters has endpoint="")
  - DocDB instances column shows count "1" not raw array
  - Filter "elasticache" matches Redis cluster
  - Filter "rds-eu" matches second DocDB cluster
  - Switch between Redis and DocDB via :redis / :docdb commands

- **Test pattern:**
  1. Test Redis: navigate to "redis", load `fixtureRedisClusters()`, verify list/detail/YAML
  2. Test DocDB: navigate to "docdb", load `fixtureDocDBClusters()`, verify list/detail/YAML
  3. Test cross-cutting: command mode navigation between the two

---

### Agent QA-6: EKS & Secrets Manager Stories

- **Source:** `docs/qa/06-eks-secrets-views.md`
- **Output:** `tests/unit/qa_eks_secrets_test.go`
- **Stories to cover:**
  - **EKS List** (6 stories): EKS-LIST-01 through EKS-LIST-06 -- columns (Cluster Name, Version, Status, Endpoint, Platform Version), row coloring (ACTIVE=green, CREATING=yellow, DELETING/FAILED=red), sort, filter, horizontal scroll, copy
  - **EKS Detail** (5 stories): EKS-DETAIL-01 through EKS-DETAIL-05 -- 8 fields (Name through KubernetesNetworkConfig), nested KubernetesNetworkConfig sub-fields, scroll, copy, back navigation
  - **EKS YAML** (4 stories): EKS-YAML-01 through EKS-YAML-04 -- full YAML, syntax coloring, scroll/copy, back navigation from list and detail
  - **EKS Help** (1 story): EKS-HELP-01 -- help screen shows relevant keys
  - **Secrets List** (6 stories): SEC-LIST-01 through SEC-LIST-06 -- columns (Secret Name, Description, Last Accessed, Last Changed, Rotation), plain row coloring (no status concept), sort, filter, horizontal scroll, copy
  - **Secrets Detail** (4 stories): SEC-DETAIL-01 through SEC-DETAIL-04 -- 8 fields (Name through Tags), scroll, copy, back navigation
  - **Secrets YAML** (4 stories): SEC-YAML-01 through SEC-YAML-04 -- full YAML, syntax coloring, scroll/copy, back navigation from list and detail
  - **Secrets Reveal** (7 stories): SEC-REVEAL-01 through SEC-REVEAL-07 -- x opens reveal, red warning header, copy from reveal, Esc closes reveal, x is no-op on non-secret types, JSON secret handling, empty/deleted secret error handling
  - **Cross-Cutting** (8 stories): refresh, command navigation to :eks and :secrets, empty state, error handling (no credentials, insufficient permissions)

- **Fixtures needed:** `fixtureEKSClusters()` (1 cluster, ACTIVE), `fixtureSecrets()` (5 secrets, no rotation)
- **Views/models to test:** Root model navigated to EKS list, EKS detail, EKS YAML, secrets list, secrets detail, secrets YAML, secrets reveal
- **Key assertions:**
  - `strings.Contains(plain, "eks-clusters(1)")` for EKS frame title
  - `strings.Contains(plain, "test-cluster-1")` for EKS cluster name
  - `strings.Contains(plain, "1.31")` for EKS version
  - `strings.Contains(plain, "ACTIVE")` for EKS status
  - `strings.Contains(plain, "secrets(5)")` for secrets frame title
  - `strings.Contains(plain, "integration_test")` for first secret name
  - `strings.Contains(plain, "No")` for rotation_enabled
  - Reveal view: `strings.Contains(plain, "Secret visible")` for red warning
  - x key on EC2 list produces no view change (no-op test)
- **Edge cases:**
  - EKS cluster with long endpoint URL (https://...eks.amazonaws.com)
  - Secrets with empty description (all 5 fixtures have description="")
  - Reveal view requires SecretRevealedMsg to populate
  - Test x key does nothing on non-secrets resource type
  - Filter "dev" matches multiple secrets
  - Sort by N toggles ascending/descending on secret name

- **Test pattern:**
  1. Test EKS: navigate to "eks", load `fixtureEKSClusters()`, verify list/detail/YAML
  2. Test Secrets: navigate to "secrets", load `fixtureSecrets()`, verify list/detail/YAML
  3. Test Reveal: navigate to secrets, load data, press x, send `SecretRevealedMsg`, verify warning header
  4. Test cross-resource: x key on EC2 list is no-op

---

### Agent QA-7: Help, Profile Selector, Region Selector Stories

- **Source:** `docs/qa/07-help-profile-region.md`
- **Output:** `tests/unit/qa_help_profile_region_test.go`
- **Stories to cover:**
  - **Help View** (15 stories): HV-01 through HV-15 -- open help from main menu, resource list, detail, YAML; four-column layout (RESOURCE, GENERAL, NAVIGATION, HOTKEYS); column contents verification; key styling (green bold); close with any key; close with Escape; help replaces content (not overlay); "Press any key to close" hint; help preserves return context (cursor position, active filter)
  - **Profile Selector** (13 stories): PS-01 through PS-13 -- open via :ctx and :profile commands; profile list matches AWS config; current profile indicator "(current)"; count in frame title; navigate with j/k/arrows; select with Enter; select already-current profile; cancel with Escape; selected row styling; single profile configured; profile selector from different views
  - **Region Selector** (11 stories): RS-01 through RS-11 -- open via :region command; region list contents (us-east-1, us-west-2, eu-west-1, etc.); current region indicator; count in title; navigate with j/k; select with Enter; select already-current region; cancel with Escape; selected row styling; region selector from resource list
  - **Flash Messages** (5 stories): FM-01 through FM-05 -- position (header right), auto-clear (~2s), copy confirmation "Copied!", refresh "Refreshing...", flash replaces previous flash
  - **Error Messages** (3 stories): EM-01 through EM-03 -- red styling, error text, auto-clear
  - **Terminal Resize** (8 stories): TR-01 through TR-08 -- resize during list/help/profile/region/detail, minimum width enforcement, minimum height enforcement, recovery from too-small
  - **Profile/Region Switch Data Refresh** (8 stories): PR-01 through PR-08 -- profile switch refreshes list, region switch refreshes list, all resource types refreshed, cursor position reset, loading indicator during switch

- **Fixtures needed:** None for most tests; `fixtureEC2Instances()` for tests that verify data refresh after profile/region switch
- **Views/models to test:** Root model, help view, profile selector view, region selector view
- **Key assertions:**
  - Help: `strings.Contains(plain, "RESOURCE")`, `strings.Contains(plain, "GENERAL")`, `strings.Contains(plain, "NAVIGATION")`, `strings.Contains(plain, "HOTKEYS")`
  - Help: `strings.Contains(plain, "Press any key to close")`
  - Help: `strings.Contains(plain, "help")` in frame title (case-insensitive)
  - Profile: `strings.Contains(plain, "aws-profiles")` in frame title
  - Region: `strings.Contains(plain, "aws-regions")` in frame title
  - Flash: `strings.Contains(plain, "Copied!")` after FlashMsg
  - Error: `strings.Contains(plain, "Error")` after error FlashMsg
  - After ProfileSelectedMsg: header shows new profile name
  - After RegionSelectedMsg: header shows new region name
  - Resize to 40x24: `strings.Contains(plain, "too narrow")`
  - Resize to 80x5: `strings.Contains(plain, "too short")`
- **Edge cases:**
  - Help from filter mode preserves filter state on close
  - Profile selector when only 1 profile exists
  - Region selector: selecting already-current region is a no-op
  - Flash message auto-clear: send ClearFlashMsg with matching Gen
  - Flash message replacement: send two FlashMsg in sequence

- **Test pattern:**
  1. Help: `rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetHelp})`, verify content, press any key, verify return
  2. Profile: `rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetProfile})`, verify frame title, send ProfileSelectedMsg
  3. Region: `rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetRegion})`, verify frame title, send RegionSelectedMsg
  4. Flash: `rootApplyMsg(m, messages.FlashMsg{Text: "Copied!", IsError: false})`, verify header
  5. Resize: `rootApplyMsg(m, tea.WindowSizeMsg{Width: 40, Height: 24})`, verify error message

---

### Agent QA-8: Detail View -- All Resource Types

- **Source:** `docs/qa/08-detail-all-types.md`
- **Output:** `tests/unit/qa_detail_test.go`
- **Stories to cover:**
  - **1. Common Behaviors** (18 stories): C-01 through C-63 -- entering detail via Enter and d, scroll j/k/g/G/h/l, word wrap toggle w, switch to YAML via y, copy c, back Esc, layout/styling (blue keys, white values, 2-space indent), nested struct rendering, empty/null field rendering
  - **2. S3 Bucket Detail** (5 stories): S3-D01 through S3-D14 -- 3 fields (BucketArn, BucketRegion, CreationDate), long ARN wrapping, empty BucketRegion, timestamp format
  - **3. S3 Object Detail** (5 stories): OBJ-D01 through OBJ-D14 -- 3 fields (Name, LastModified, Owner), Owner struct rendering, nil Owner, deep prefix key
  - **4. EC2 Instance Detail** (16 stories): EC2-D01 through EC2-D52 -- 13 fields in order, State nested struct, SecurityGroups array, Tags array, missing PublicIpAddress, missing Platform
  - **5. RDS Instance Detail** (7 stories): RDS-D01 through RDS-D33 -- 10 fields, Endpoint nested struct (Address/Port/HostedZoneId), null Endpoint, MultiAZ Yes/No, AllocatedStorage integer
  - **6. Redis Detail** (4 stories): RED-D01 through RED-D31 -- 8 fields, ConfigurationEndpoint nested, null endpoint, NumCacheNodes integer
  - **7. DocumentDB Detail** (7 stories): DOC-D01 through DOC-D42 -- 9 fields, DBClusterMembers array with writer/reader, empty members, Endpoint/ReaderEndpoint strings, StorageEncrypted Yes/No, Port integer
  - **8. EKS Detail** (8 stories): EKS-D01 through EKS-D40 -- 8 fields, KubernetesNetworkConfig nested (IpFamily, ServiceIpv4Cidr, ElasticLoadBalancing), long Endpoint/Arn/RoleArn
  - **9. Secrets Detail** (10 stories): SEC-D01 through SEC-D41 -- 8 fields, Tags array, RotationEnabled Yes/No, empty Description, nil KmsKeyId, long ARN, detail vs reveal distinction
  - **10. Cross-cutting** (20 stories): X-01 through X-51 -- boolean Yes/No formatting, timestamp YYYY-MM-DD HH:MM:SS, integer rendering, zero integer as empty, nested struct pattern, array pattern, field ordering matches views.yaml

- **Fixtures needed:** All fixture functions -- `fixtureS3Buckets()`, `fixtureS3Objects()`, `fixtureEC2Instances()`, `fixtureRDSInstances()`, `fixtureRedisClusters()`, `fixtureDocDBClusters()`, `fixtureEKSClusters()`, `fixtureSecrets()`
- **Views/models to test:** Root model navigated to each resource type's detail view via `messages.NavigateMsg{Target: messages.TargetDetail, Resource: &res}`
- **Key assertions:**
  - For each type, verify all configured detail fields appear in the rendered output
  - Field order matches views.yaml order (first field appears before second field in output)
  - Nested structs render as multi-line with indentation
  - Arrays render as YAML-style list items
  - Null/empty fields render as empty strings, not "null" or "N/A"
  - Booleans render as "Yes"/"No"
  - Timestamps render as "YYYY-MM-DD HH:MM:SS"
  - Zero integers render as empty string
  - After w toggle: verify long values wrap
  - After y: transitions to YAML view
  - After c: FlashMsg with "Copied" text
  - After Esc: returns to list view
- **Edge cases:**
  - EC2 with nil PublicIpAddress (terminated instance from fixtures)
  - Redis with nil ConfigurationEndpoint (fixture has empty endpoint)
  - RDS with null Endpoint (synthetic resource in creating state)
  - DocDB with empty DBClusterMembers (synthetic resource)
  - Secrets with nil KmsKeyId and empty Description (all fixtures match)
  - Detail view content taller than frame: scroll with j/G

- **Test pattern:**
  1. Navigate to resource list, load fixtures, then send NavigateMsg{Target: TargetDetail, Resource: &firstResource}
  2. Alternative: navigate to list, load fixtures, press Enter or d key
  3. Verify rendered output contains expected fields
  4. Test scroll: send j/k/g/G keys, verify content changes
  5. Test wrap: send w key, verify layout change
  6. Test YAML switch: send y key, verify "yaml" in frame title

---

### Agent QA-9: YAML View -- All Resource Types

- **Source:** `docs/qa/09-yaml-all-types.md`
- **Output:** `tests/unit/qa_yaml_test.go`
- **Stories to cover:**
  - **Frame Title Format** (7 stories): each resource type shows `<resource-id> yaml` in frame title
  - **Syntax Coloring Rules** (6 stories): keys blue, strings green, numbers orange, booleans purple, null dim, indent lines dim -- verified per resource type with specific fields
  - **YAML Structure Rules** (5 stories): 2-space indentation, indent connectors, array `- ` prefix, empty arrays as `[]`, null fields as `null`
  - **Keyboard Controls** (10 stories): j/k/g/G/PageUp/PageDown scroll, w wrap toggle, c copy, Esc back, ? help
  - **Scroll Behavior** (6 stories): line-by-line, jump top/bottom, page scroll, scroll indicator
  - **Wrap Toggle** (4 stories): wrap off (clipped), wrap on (continuation lines), long values (ARNs, endpoints, base64)
  - **Copy** (4 stories): full YAML copied, plain text (no ANSI), "Copied!" flash, failure flash
  - **US-S3** (S3 YAML): bucket YAML structure (Name, CreationDate, BucketArn, BucketRegion), coloring
  - **US-EC2** (EC2 YAML): full instance YAML with nested structures (BlockDeviceMappings, State, SecurityGroups, Tags), coloring, scroll test
  - **US-RDS** (RDS YAML): instance YAML with Endpoint nested, VpcSecurityGroups array, DBSubnetGroup nested, coloring, wrap test
  - **US-REDIS** (Redis YAML): cluster YAML with ConfigurationEndpoint, CacheNodes array with nested Endpoints, coloring
  - **US-DOCDB** (DocDB YAML): cluster YAML with DBClusterMembers array, AvailabilityZones string array, empty AssociatedRoles
  - **US-EKS** (EKS YAML): cluster YAML with CertificateAuthority.Data (very long base64), KubernetesNetworkConfig, ResourcesVpcConfig with nested arrays, Logging
  - **US-SECRETS** (Secrets YAML): secret YAML with RotationRules, VersionIdsToStages map, Tags array
  - **Cross-Resource Test Scenarios** (13 stories): TC-YAML-01 through TC-YAML-13 -- copy/scroll/wrap/empty arrays/null fields/detail-to-YAML navigation/list-to-YAML navigation/help from YAML/special characters/timestamps as strings/deeply nested YAML/boolean values across resources/numeric values across resources

- **Fixtures needed:** All fixture functions -- `fixtureS3Buckets()`, `fixtureS3Objects()`, `fixtureEC2Instances()`, `fixtureRDSInstances()`, `fixtureRedisClusters()`, `fixtureDocDBClusters()`, `fixtureEKSClusters()`, `fixtureSecrets()`
- **Views/models to test:** Root model navigated to each resource type's YAML view via `messages.NavigateMsg{Target: messages.TargetYAML, Resource: &res}`
- **Key assertions:**
  - Frame title contains "<resource-id> yaml"
  - YAML content is present (non-empty view after stripping ANSI)
  - For each resource type, key fields appear in YAML output (e.g., "InstanceId:" for EC2)
  - Nested structures have proper indentation (check for 2-space indented sub-keys)
  - Arrays use `- ` prefix
  - After c: FlashMsg "Copied!" appears
  - After Esc: returns to previous view (detail or list)
  - After ?: help screen appears
  - After w: wrap toggle changes long line rendering
  - Scroll: j/k change visible content
- **Edge cases:**
  - S3 YAML is very short (< 10 lines): no scroll needed
  - EC2 YAML is long (50+ lines): scroll required, test g/G
  - EKS YAML with very long CertificateAuthority.Data base64 string
  - Empty arrays render as `[]`
  - Null fields render as `null`
  - YAML entered from detail view: Esc returns to detail (not list)
  - YAML entered from list view: Esc returns to list (not detail)

- **Test pattern:**
  1. Navigate to resource list, load fixtures
  2. Send `messages.NavigateMsg{Target: messages.TargetYAML, Resource: &resource}` to open YAML
  3. Alternative: from list view, press y key
  4. Verify frame title contains "yaml"
  5. Verify YAML content contains expected keys
  6. Test scroll: send j/G keys
  7. Test copy: send c key, check for FlashMsg via cmd execution
  8. Test back: send Esc, verify return to previous view
  9. Test via detail: first Enter to detail, then y to YAML, then Esc should return to detail

---

## Execution Summary

| Agent | Source File | Output File | Story Count (approx) | Fixture Functions |
|-------|-----------|------------|----------------------|------------------|
| QA-1 | 01-main-menu.md | qa_mainmenu_test.go | ~90 | none (+ fixtureEC2Instances for navigation tests) |
| QA-2 | 02-s3-views.md | qa_s3_test.go | ~95 | fixtureS3Buckets, fixtureS3Objects |
| QA-3 | 03-ec2-views.md | qa_ec2_test.go | ~120 | fixtureEC2Instances |
| QA-4 | 04-rds-views.md | qa_rds_test.go | ~90 | fixtureRDSInstances |
| QA-5 | 05-redis-docdb-views.md | qa_redis_docdb_test.go | ~65 | fixtureRedisClusters, fixtureDocDBClusters |
| QA-6 | 06-eks-secrets-views.md | qa_eks_secrets_test.go | ~50 | fixtureEKSClusters, fixtureSecrets |
| QA-7 | 07-help-profile-region.md | qa_help_profile_region_test.go | ~65 | fixtureEC2Instances (for refresh tests) |
| QA-8 | 08-detail-all-types.md | qa_detail_test.go | ~100 | ALL fixture functions |
| QA-9 | 09-yaml-all-types.md | qa_yaml_test.go | ~80 | ALL fixture functions |
| **Total** | | | **~755** | |

## Agent Instructions (Common)

Each agent MUST:

1. **Read the existing test harness** in `tests/unit/tui_root_test.go` to understand the helper pattern (`newRootSizedModel`, `rootApplyMsg`, `rootViewContent`, `rootKeyPress`, `rootSpecialKey`, `stripANSI`)
2. **Read the fixture file** `tests/unit/fixtures_test.go` to understand available data
3. **Read the source QA story file** to understand every story that must be covered
4. **Write all tests** in `tests/unit/<assigned_file>` using `package unit`
5. **Re-use the existing helpers** -- do not create new helper files
6. **Follow the test pattern**: create model -> set size -> navigate to view -> load data -> send keys -> assert on View() output
7. **Use table-driven tests** where multiple stories share the same setup (e.g., "filter by X" for multiple X values)
8. **Name tests descriptively**: `TestQA_EC2List_StatusColorRunningGreen`, `TestQA_MainMenu_FilterNarrowsList`
9. **Run `go test ./tests/unit/ -count=1 -timeout 120s`** after writing to verify all tests compile and pass
10. **Bump version** in `cmd/a9s/main.go` after changes

## Overlap/Dedup Notes

- Stories in 08 (detail) and 09 (YAML) overlap with resource-specific stories in 02-06. The resource-specific files (QA-2 through QA-6) should focus on list view rendering, navigation, filter, sort, and basic Enter/d/y actions. The detail file (QA-8) and YAML file (QA-9) should focus on the content and formatting of those views across all types.
- If a story is covered by both the resource-specific agent and the detail/YAML agent, the resource-specific agent should test the **transition** (pressing d/y opens the right view) while the detail/YAML agent tests the **content** (correct fields, formatting, nesting).
- Help overlay stories appear in 01 and 07. QA-1 tests help from the main menu; QA-7 tests help from all other views and verifies content/layout in depth.

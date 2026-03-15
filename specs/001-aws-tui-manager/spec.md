# Feature Specification: a9s — Terminal UI AWS Resource Manager

**Feature Branch**: `001-aws-tui-manager`
**Created**: 2026-03-15
**Status**: Draft
**Input**: Terminal UI AWS resource manager inspired by k9s, MVP focused on read operations with k9s-style command navigation for browsing AWS resources (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Launch and Browse Resource Types (Priority: P1)

A cloud engineer launches the application from their terminal. The app reads
their current AWS profile and region from the existing AWS configuration and
displays a main screen showing a list of supported AWS resource types
(S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets). The header shows the
active AWS profile name and region. The user navigates the resource type list
using vim-style keys (j/k or arrow keys) and presses Enter to drill into a
resource type, which shows a table of all resources of that type in the
active account and region.

**Why this priority**: This is the foundation of the entire application.
Without the ability to launch, see resource types, and list resources,
no other feature has value.

**Independent Test**: Can be tested by launching the app with valid AWS
credentials and verifying that the main screen appears with resource types
and that selecting a type shows a table of resources.

**Acceptance Scenarios**:

1. **Given** the user has AWS credentials configured in `~/.aws/config`,
   **When** they launch the application,
   **Then** the main screen displays showing the active profile name in the
   header, the active region, and a list of supported resource types.

2. **Given** the main screen is displayed,
   **When** the user navigates to "EC2" using j/k keys and presses Enter,
   **Then** a table view appears listing all EC2 instances in the active
   account/region with columns for key attributes (instance ID, name, state,
   type, private IP, public IP, launch time).

3. **Given** a resource type list is displayed,
   **When** the user types `:ec2` in command mode,
   **Then** the app navigates directly to the EC2 instances table view,
   identical to selecting it from the resource type list.

4. **Given** the user is viewing a resource list,
   **When** they press Escape or type `:main`,
   **Then** the app returns to the resource types list.

---

### User Story 2 - AWS Profile and Region Switching (Priority: P2)

A DevOps engineer manages resources across multiple AWS accounts and
regions. They use the `:ctx` command to see all profiles from their
`~/.aws/config` file and switch between them. They use `:region` to see
available AWS regions and switch to a different region. After switching,
all resource views reflect the newly selected profile/region.

**Why this priority**: Multi-account and multi-region support is essential
for any practical AWS management workflow. Without this, users are
limited to a single account and region.

**Independent Test**: Can be tested by configuring multiple AWS profiles
in `~/.aws/config`, launching the app, switching profiles via `:ctx`,
and verifying the header updates and resource lists reload.

**Acceptance Scenarios**:

1. **Given** the user has multiple profiles in `~/.aws/config`,
   **When** they type `:ctx`,
   **Then** a list of all available AWS profiles is displayed with the
   currently active profile highlighted.

2. **Given** the profile list is displayed,
   **When** the user selects a different profile and presses Enter,
   **Then** the header updates to show the new profile name and all
   subsequent resource views use the new profile's credentials.

3. **Given** the user is on any view,
   **When** they type `:region`,
   **Then** a list of AWS regions is displayed with the current region
   highlighted.

4. **Given** the region list is displayed,
   **When** the user selects a different region and presses Enter,
   **Then** the header updates to show the new region and subsequent
   resource views use the new region.

5. **Given** the user has switched profile or region,
   **When** they navigate to a resource type,
   **Then** the resource list reflects the resources from the newly
   selected profile and region.

---

### User Story 3 - Resource Detail and Interaction (Priority: P3)

A developer needs to inspect the details of a specific AWS resource.
From any resource list, they use k9s-style action keys: `d` to describe
(view all attributes), `x` to reveal sensitive values (e.g., secret
plaintext), and `c` to copy the resource identifier to the clipboard.
Enter drills into hierarchical resources (S3 buckets → objects). They
can scroll through details and press Escape to return to the list.

**Why this priority**: Listing resources provides an overview, but
engineers frequently need to inspect specific resource properties
(tags, configuration, status details). This is the natural next step
after browsing.

**Independent Test**: Can be tested by navigating to any resource list,
selecting a resource, pressing `d`, and verifying that comprehensive
details are displayed.

**Acceptance Scenarios**:

1. **Given** the user is viewing an EC2 instances list,
   **When** they select an instance and press `d`,
   **Then** a describe view appears showing all instance attributes
   (ID, name, state, type, AMI, VPC, subnet, security groups, tags,
   launch time, monitoring status, etc.) in a scrollable format.

2. **Given** the user is viewing an S3 buckets list,
   **When** they select a bucket and press Enter,
   **Then** the view navigates into the bucket showing a list of
   objects/prefixes (folder-style browsing) with columns for key name,
   size, last modified, and storage class.

3. **Given** the user is inside an S3 bucket viewing objects,
   **When** they select an object and press `d`,
   **Then** a describe view shows the object's metadata (size, content
   type, last modified, storage class, ETag, encryption status).

4. **Given** the user is viewing a Secrets Manager secret,
   **When** they press `x`,
   **Then** the secret value is fetched and displayed as plain text
   in a scrollable view, without requiring confirmation.

5. **Given** the user is viewing any resource list,
   **When** they select a resource and press `c`,
   **Then** the resource's primary identifier (e.g., instance ID, bucket
   name, ARN) is copied to the system clipboard.

6. **Given** the user is viewing a describe or reveal view,
   **When** they press Escape,
   **Then** the app returns to the previous resource list with the same
   cursor position preserved.

---

### User Story 4 - Filter and Search Resources (Priority: P4)

An engineer managing an account with hundreds of resources needs to
quickly find specific resources. They press `/` to enter filter mode
and type a search term. The resource list filters in real time, showing
only rows that match the search term across any visible column. They
clear the filter with Escape.

**Why this priority**: Filtering is essential for usability at scale.
Without it, finding a specific resource in a large list requires manual
scrolling, which defeats the purpose of a terminal UI.

**Independent Test**: Can be tested by viewing a resource list with
multiple items, pressing `/`, typing a partial name, and verifying
only matching resources remain visible.

**Acceptance Scenarios**:

1. **Given** the user is viewing an EC2 instances list with 50 instances,
   **When** they press `/` and type "prod",
   **Then** only instances whose name, ID, or other visible column
   contains "prod" are displayed.

2. **Given** a filter is active and showing filtered results,
   **When** the user presses Escape,
   **Then** the filter is cleared and all resources are displayed again.

3. **Given** the user applies a filter that matches no resources,
   **When** the filter result is empty,
   **Then** the view shows an empty table with a message indicating
   no resources match the filter.

4. **Given** the user is viewing a filtered list,
   **When** they navigate to a resource and press Enter to view details,
   then press Escape to return,
   **Then** the filter is still active and the filtered view is restored.

---

### User Story 5 - Resource-Specific Browsing for All MVP Types (Priority: P5)

A cloud engineer uses the app to browse all supported resource types
in the MVP scope. Each resource type displays a table with relevant
columns specific to that resource. The user can navigate between
different resource types using colon commands (`:s3`, `:ec2`, `:rds`,
`:redis`, `:docdb`, `:eks`, `:secrets`).

**Why this priority**: While User Story 1 covers the core navigation
pattern, this story ensures each specific resource type has appropriate
columns and meaningful data presentation tailored to its attributes.

**Independent Test**: Can be tested by navigating to each resource type
via colon commands and verifying the correct columns and data appear.

**Acceptance Scenarios**:

1. **Given** the user types `:s3`,
   **When** the S3 buckets view loads,
   **Then** a table displays with columns: Bucket Name, Region, Creation Date.

2. **Given** the user types `:ec2`,
   **When** the EC2 view loads,
   **Then** a table displays with columns: Instance ID, Name, State,
   Type, Private IP, Public IP, Launch Time.

3. **Given** the user types `:rds`,
   **When** the RDS view loads,
   **Then** a table displays with columns: DB Identifier, Engine,
   Engine Version, Status, Class, Endpoint, Multi-AZ.

4. **Given** the user types `:redis`,
   **When** the ElastiCache Redis view loads,
   **Then** a table displays with columns: Cluster ID, Engine Version,
   Node Type, Status, Nodes, Endpoint.

5. **Given** the user types `:docdb`,
   **When** the DocumentDB view loads,
   **Then** a table displays with columns: Cluster ID, Engine Version,
   Status, Instance Count, Endpoint.

6. **Given** the user types `:eks`,
   **When** the EKS view loads,
   **Then** a table displays with columns: Cluster Name, Version,
   Status, Endpoint, Platform Version.

7. **Given** the user types `:secrets`,
   **When** the Secrets Manager view loads,
   **Then** a table displays with columns: Secret Name, Description,
   Last Accessed, Last Changed, Rotation Enabled.

---

### Edge Cases

- What happens when AWS credentials are expired or invalid?
  The app MUST display a clear error message indicating the credential
  issue and the profile name, without crashing.

- What happens when a region has no resources of a given type?
  The app MUST show an empty table with a message like "No EC2 instances
  found in us-east-1" rather than an error.

- What happens when the user types an unrecognized command?
  The app MUST display "Unknown command: :foo" in the status area
  and remain on the current view.

- What happens when an API call times out or fails?
  The app MUST display the error in a status bar, keep the UI
  responsive, and allow the user to retry or navigate elsewhere.

- What happens when `~/.aws/config` has no profiles configured?
  The app MUST display a message explaining that no AWS profiles
  were found and how to configure them.

- What happens when the user presses `:ctx` and selects a profile
  that uses SSO with an expired token?
  The app MUST display an error indicating the SSO session is expired
  and suggest running `aws sso login --profile <name>`.

- What happens during S3 browsing when a bucket has thousands of objects?
  The app MUST load objects in pages, showing the first page immediately
  and loading more as the user scrolls.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Application MUST launch as a terminal-based interactive UI
  that takes over the full terminal window.
- **FR-002**: Application MUST read AWS profiles from `~/.aws/config`
  and `~/.aws/credentials` and use the default profile on startup
  (or the profile set via `AWS_PROFILE` environment variable).
- **FR-003**: Application MUST display a persistent header showing the
  active AWS profile name, active region, and application name/version.
- **FR-004**: Application MUST support a colon-command system activated
  by pressing `:`, with auto-suggestions for known commands.
- **FR-005**: Application MUST support these navigation commands:
  `:main` / `:root` (resource types list), `:ctx` (profile list),
  `:region` (region list), `:s3`, `:ec2`, `:rds`, `:redis`, `:docdb`,
  `:eks`, `:secrets` (resource type views), `:q` / `:quit` (exit).
- **FR-006**: Application MUST support k9s-style keybindings: `j`/`k` or
  arrow keys for cursor movement, `g`/`G` for top/bottom, Enter for
  select/drill-in, Escape for back/cancel, `d` for describe (detail view),
  `x` for reveal/decode (e.g., secret values as plain text, no confirmation),
  `c` for copy resource identifier to clipboard, `y` for raw JSON view.
- **FR-007**: Application MUST support filter mode via `/` with
  real-time text matching across all visible columns.
- **FR-008**: Application MUST display resources in tabular format with
  columns appropriate to each resource type.
- **FR-009**: Application MUST provide a detail view for each resource
  showing all available attributes in a scrollable format.
- **FR-010**: Application MUST support S3 hierarchical browsing — entering
  a bucket shows objects/prefixes, and users can navigate into prefixes.
- **FR-011**: Application MUST allow switching AWS profiles via `:ctx`
  and reload resource views with the new profile's credentials.
- **FR-012**: Application MUST allow switching AWS regions via `:region`.
  The selected region is passed as a parameter to all AWS API calls
  (equivalent to `--region`). The app displays whatever AWS returns
  with no client-side filtering — some APIs (e.g., S3 ListBuckets)
  return global results regardless of region.
- **FR-013**: Application MUST display breadcrumbs showing the current
  navigation path (e.g., `main > EC2 > i-abc123`).
- **FR-014**: Application MUST show a help overlay when the user
  presses `?` listing all available commands and keyboard shortcuts.
- **FR-015**: Application MUST support back/forward navigation history
  using `[` and `]` keys.
- **FR-016**: Application MUST handle API errors gracefully by displaying
  error messages in a status area without crashing or freezing.
- **FR-017**: Application MUST support column sorting via keyboard
  shortcuts (e.g., Shift+N for name, Shift+S for status).
- **FR-018**: Application MUST perform API calls asynchronously,
  showing a loading indicator while data is being fetched. Data refresh
  is manual only via `Ctrl-R` (reloads current view). No auto-polling.
- **FR-019**: Application MUST be read-only — no create, update, or
  delete operations on AWS resources.

### Key Entities

- **AWS Profile**: A named configuration in `~/.aws/config` containing
  credentials, region, and other settings. Attributes: name, region,
  SSO configuration (if applicable), role ARN (if applicable).
- **AWS Region**: An AWS geographic region (e.g., us-east-1, eu-west-1).
  Attributes: region code, display name.
- **Resource Type**: A category of AWS resources supported by the app.
  Attributes: name, short command alias, display columns, detail fields.
- **Resource**: A specific AWS resource instance. Attributes vary by
  type but always include an identifier, name/tags, status, and
  type-specific properties.
- **Navigation State**: The current view position in the app hierarchy.
  Attributes: view type, active profile, active region, active filter,
  scroll position, breadcrumb trail, history stack.

### Assumptions

- The user has the AWS CLI installed and configured with at least one
  profile in `~/.aws/config` or `~/.aws/credentials`.
- The user's terminal supports 256 colors and standard ANSI escape codes.
- The user has network access to AWS APIs from their machine.
- The application does not need to manage or store credentials itself —
  it relies entirely on the existing AWS credential chain
  (environment variables, config files, instance profiles).
- Region is a simple pass-through to AWS API calls. The app does not
  apply client-side region filtering — it shows whatever AWS returns.
- Region list is a hardcoded set of currently available AWS regions
  (can be updated in future releases).
- S3 object browsing uses `/` delimiter for folder-style navigation.
- ElastiCache listing is filtered to Redis engine only (not Memcached).

## Clarifications

### Session 2026-03-15

- Q: How should secret values and resource actions be handled (keybinding model)? → A: Follow k9s patterns throughout — `d` describes (metadata/attributes), `x` reveals sensitive values (plain text, no confirmation), `c` copies identifier to clipboard. Apply k9s interaction patterns wherever reasonable.
- Q: How should resource data be refreshed? → A: Manual refresh only via `Ctrl-R`. No auto-polling — AWS API rate limits and potential cost make auto-refresh unsuitable.
- Q: How should S3 listing and region switching interact? → A: Region is a pass-through parameter to AWS API calls (`--region`), no special client-side behavior. The app displays whatever AWS returns. S3 ListBuckets is global and returns all buckets regardless of region.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can launch the application and see a resource type
  list within 2 seconds of invocation.
- **SC-002**: Users can navigate from the main screen to any resource
  list in 2 keystrokes or fewer (one `:` + command, or one Enter).
- **SC-003**: Resource lists load and display within 5 seconds for
  accounts with up to 500 resources of a given type.
- **SC-004**: Profile and region switching completes (header updated,
  ready for navigation) within 1 second.
- **SC-005**: Filter results update within 200ms of each keystroke
  for lists of up to 1000 items.
- **SC-006**: Users can browse all 7 supported resource types
  (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets) using colon commands
  without needing to consult documentation.
- **SC-007**: The application remains responsive (accepts keyboard input)
  even while API calls are in progress.
- **SC-008**: 90% of users familiar with k9s or vim-style navigation
  can complete a resource lookup task on first attempt without help.

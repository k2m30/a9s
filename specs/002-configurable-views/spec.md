# Feature Specification: Configurable Views

**Feature Branch**: `002-configurable-views`
**Created**: 2026-03-15
**Status**: Partial
**Input**: User description: "Make both list view columns and single resource detail views configurable via YAML config files, with a config file lookup chain and a reference generator tool."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Customize List View Columns Per Resource Type (Priority: P1)

As an a9s user, I want the columns shown in each resource list view (S3, EC2, RDS, etc.) to be loaded from a YAML configuration file so that I can control which fields are displayed and how wide each column is, without recompiling the application.

**Why this priority**: This is the core value of the feature — without config-driven columns, the app remains rigid and users cannot tailor the display to their needs.

**Independent Test**: Can be fully tested by placing a `views.yaml` file in the current directory with custom column definitions for one resource type (e.g., EC2 with only 3 columns) and verifying that the app displays exactly those columns with the specified widths.

**Acceptance Scenarios**:

1. **Given** a `views.yaml` file exists in the current working directory with an `ec2` list view defining 3 columns with custom widths, **When** the user launches a9s and navigates to the EC2 view, **Then** only those 3 columns are shown with the specified widths.
2. **Given** no `views.yaml` file exists anywhere in the lookup chain, **When** the user launches a9s, **Then** the app uses built-in default definitions (matching the current hardcoded values) and behaves identically to the pre-feature version.
3. **Given** a `views.yaml` file defines columns for S3 but not for EC2, **When** the user views EC2 resources, **Then** the app falls back to built-in defaults for EC2 while using the configured columns for S3.

---

### User Story 1b - Customize Detail View Fields Per Resource Type (Priority: P1)

As an a9s user, I want to control which fields appear in the single-resource detail view and in what order, so that I see only the relevant attributes without noise from the full API response.

**Why this priority**: The detail view is equally important — AWS API responses contain many fields irrelevant to daily use, and users need to filter that down.

**Independent Test**: Can be tested by adding a `detail` section under a resource type in `views.yaml` with a subset of paths, and verifying that only those fields appear in the specified order when opening a resource's detail view.

**Acceptance Scenarios**:

1. **Given** `views.yaml` defines a detail view for `ec2` with paths `instanceId`, `state`, and `placement` in that order, **When** the user opens the detail view for an EC2 instance, **Then** only those 3 fields are shown, in the configured order (not alphabetical).
2. **Given** a detail path points to a scalar value (e.g., `instanceId`), **When** the detail view renders it, **Then** it displays as `instanceId : i-0abc123`.
3. **Given** a detail path points to a nested object or array (e.g., `securityGroups`, `tags`), **When** the detail view renders it, **Then** it displays as an indented YAML subtree.
4. **Given** `views.yaml` defines list columns for `ec2` but no detail section, **When** the user opens the detail view, **Then** the app falls back to built-in defaults (all available detail fields, sorted alphabetically — current behavior).

---

### User Story 2 - Config File Lookup Chain (Priority: P1)

As an a9s user, I want the application to look for `views.yaml` in multiple locations in a defined priority order so that I can have project-specific, environment-specific, or user-wide configurations.

**Why this priority**: The lookup chain is essential to making the config system practical — users need both per-project and global defaults.

**Independent Test**: Can be tested by placing different `views.yaml` files in each lookup location and verifying the correct one takes effect based on priority.

**Acceptance Scenarios**:

1. **Given** `views.yaml` exists in both `./views.yaml` and `~/.a9s/views.yaml` with different column definitions for S3, **When** the user launches a9s from the directory containing `./views.yaml`, **Then** the columns from `./views.yaml` are used (highest priority).
2. **Given** `views.yaml` exists only in the `A9S_CONFIG_FOLDER` environment variable path, **When** the user launches a9s, **Then** that config file is loaded.
3. **Given** `A9S_CONFIG_FOLDER` is set and contains a `views.yaml`, and `./views.yaml` also exists, **When** the user launches a9s, **Then** `./views.yaml` takes priority over the environment variable path.
4. **Given** `A9S_CONFIG_FOLDER` is not set and `./views.yaml` does not exist, but `~/.a9s/views.yaml` exists, **When** the user launches a9s, **Then** the home directory config is used (lowest priority).

---

### User Story 3 - Reference File for Field Discovery (Priority: P2)

As an a9s user, I want a shipped `views_reference.yaml` file that lists all available field paths for each resource type, so that I know what paths I can use when customizing my `views.yaml`.

As a developer of a9s, I want a development-time tool that generates this reference file via reflection on AWS Go SDK structs, so it stays accurate to the SDK version the app is compiled against without requiring live AWS API calls.

**Why this priority**: This is a productivity accelerator — users can create custom configs much faster when they can see all available fields, but the core feature works without it.

**Independent Test**: Can be tested by running the reference generator and verifying the output lists all fields from the AWS SDK struct for at least one resource type. Can also verify that paths from the reference file work when used in `views.yaml`.

**Acceptance Scenarios**:

1. **Given** the developer runs the reference generator, **Then** a `views_reference.yaml` file is produced listing all available field paths for each supported resource type, generated via reflection on the AWS Go SDK structs (no live AWS calls needed).
2. **Given** the reference file is generated, **When** someone inspects it, **Then** each resource type lists its available dot-notation paths as a simple list.
3. **Given** an end user copies a path from the shipped `views_reference.yaml` into their `views.yaml`, **When** they launch a9s, **Then** that field appears in the corresponding view.

---

### User Story 4 - S3 Object View Configuration (Priority: P2)

As an a9s user, I want the S3 object/prefix browsing view (shown when navigating inside a bucket) to also be configurable via the same YAML file, so that I have full control over all views in the application.

**Why this priority**: Ensures completeness — the S3 object view is a distinct view from the bucket list and should also be configurable.

**Independent Test**: Can be tested by adding an `s3_objects` section to `views.yaml` with custom columns and verifying the in-bucket view uses them.

**Acceptance Scenarios**:

1. **Given** `views.yaml` contains an `s3_objects` list view with columns `key` and `size` only, **When** the user browses inside an S3 bucket, **Then** only those 2 columns are displayed.

---

### Edge Cases

- What happens when `views.yaml` contains a syntax error (invalid YAML)? The app displays a clear error message identifying the file and the parsing issue, then falls back to built-in defaults.
- What happens when a configured path does not match any field in the AWS SDK struct? The column is displayed with an empty value (list view) or the field is omitted (detail view).
- What happens when `views.yaml` defines a view name that does not match any known resource type? The unknown view definition is silently ignored.
- What happens when column width is set to 0 or a negative number? The app treats it as a flexible-width column (same as width=0 behavior today).
- What happens when `A9S_CONFIG_FOLDER` is set but the directory does not exist? The app skips that lookup location and continues to the next.
- What happens when a list view column path points to an array or nested object? The column shows an empty value — list view supports scalar fields only.
- What happens when a detail view path points to a nested object or array? It renders as an indented YAML subtree, preserving the full structure naturally.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST load both list view column definitions and detail view field definitions from a YAML configuration file at startup. — *Implemented*
- **FR-002**: System MUST search for `views.yaml` in the following order: (1) `./views.yaml`, (2) `$A9S_CONFIG_FOLDER/views.yaml`, (3) `~/.a9s/views.yaml`. The first file found is used. — *Implemented*
- **FR-003**: System MUST fall back to built-in default column definitions when no configuration file is found. — *Implemented*
- **FR-004**: System MUST fall back to built-in defaults for any individual resource view not defined in the configuration file (partial config support). — *Implemented*
- **FR-005**: System MUST display a clear error message and fall back to defaults when the configuration file contains invalid YAML. — *Partial: silently falls back to defaults on invalid YAML, no user-facing error message*
- **FR-006**: System MUST support configuring both list and detail views for all resource types: S3 buckets, S3 objects, EC2, RDS, Redis, DocumentDB, EKS, and Secrets Manager. — *Implemented*
- **FR-007**: Each list column definition in the YAML MUST specify a display name (the YAML key), a dot-notation path into the AWS SDK struct, and a width. List columns support scalar fields only. — *Implemented*
- **FR-008**: Each detail view definition in the YAML is an ordered list of dot-notation paths. Scalar values render as `key : value`. Nested objects and arrays render as indented YAML subtrees. — *Implemented*
- **FR-009**: The application MUST extract field values at runtime via reflection on the AWS Go SDK response structs, using the configured dot-notation paths. No per-resource-type display logic. — *Implemented*
- **FR-010**: The application MUST auto-format extracted values based on their Go type: `time.Time` as human-readable date, `bool` as Yes/No, numeric types as-is, strings as-is. — *Implemented*
- **FR-011**: System MUST provide a `views.yaml` file pre-populated with the current default column and detail field definitions as the default configuration shipped with the project. — *Implemented*
- **FR-012**: Project MUST include a development-time reference generator tool that produces a `views_reference.yaml` file listing all available field paths for each resource type, generated via reflection on AWS Go SDK structs. No live AWS API calls required. This tool is not part of the runtime application. — *Implemented*
- **FR-013**: The generated `views_reference.yaml` MUST be shipped as a delivery artifact with the project, serving as documentation for end users to discover available field paths. — *Not implemented: refgen tool exists (cmd/refgen/) but views_reference.yaml is not tracked in the repository*
- **FR-014**: The reference file lists paths as a simple list per resource type, with no widths or display names — those are the user's choice. — *N/A: depends on FR-013 artifact*
- **FR-015**: System MUST preserve column sort capability — list columns defined in config are sortable by default. — *Implemented*
- **FR-016**: System MUST fall back to built-in detail defaults (all available fields, alphabetically sorted) for any resource type whose detail section is not defined in the config. — *Implemented*

### Key Entities

- **View Configuration**: A named collection of list column and detail field definitions for a specific AWS resource type. Identified by the resource short name (e.g., `s3`, `ec2`). Contains two sub-sections: `list` (table columns) and `detail` (ordered field list).
- **List Column Definition**: A single column in the resource list table, consisting of a display name, a dot-notation path into the AWS SDK struct, and a display width. Supports scalar fields only.
- **Detail Field List**: An ordered list of dot-notation paths to include in the detail view. Scalars render as key:value, nested objects/arrays render as YAML subtrees.
- **Config Lookup Chain**: The ordered set of filesystem locations where the system searches for configuration files, with the first match winning.

## Assumptions

- The YAML structure uses the resource short name (e.g., `s3`, `ec2`) as the view key, matching the existing `ShortName` field in the codebase. Each view key contains `list` and/or `detail` sub-sections.
- Paths use dot-notation matching the AWS Go SDK struct field JSON tags (e.g., `instanceId`, `state.name`, `placement.availabilityZone`). The application resolves these via reflection on the SDK response structs.
- The reference generator uses `reflect.TypeOf()` on AWS SDK response structs to enumerate all available paths. No live AWS API calls are needed.
- The lookup chain does NOT merge configs from multiple locations — the first file found wins entirely.
- Column `sortable` is not exposed in the YAML config; all user-defined list columns are sortable by default.
- When a detail view is configured, the configured fields replace the default listing entirely. Only the specified paths are shown, in the specified order.
- Auto-formatting is applied based on Go type detected via reflection: `time.Time` → human-readable date, `bool` → Yes/No, `*string` → dereferenced string, numeric types → string representation. No formatting config is exposed to users.

## Clarifications

### Session 2026-03-15

- Q: How does the `path` field work — is it a key into a pre-processed Go map, or a path into the AWS response? → A: Dot-notation path into the AWS Go SDK struct, resolved via reflection. No per-resource-type display logic on the client side.
- Q: Is the reference generator a runtime feature or a development tool? → A: Development-time only. It generates `views_reference.yaml` via SDK struct reflection (no live AWS calls). The file ships as a delivery artifact.
- Q: How should the reference file be structured? → A: Simple list of available paths per resource type. No widths, no display names — those are the user's choice.
- Q: How should the app handle different Go types when displaying values? → A: Auto-format by detected type (`time.Time` → readable date, `bool` → Yes/No, etc.). No formatting config exposed to users.
- Q: How should arrays and nested objects work? → A: List view supports scalar fields only (arrays show empty). Detail view renders nested objects and arrays as indented YAML subtrees.

## YAML Config Structure

### views.yaml (user config)

```yaml
views:
  ec2:
    list:
      Instance ID:
        path: instanceId
        width: 20
      State:
        path: state.name
        width: 12
      AZ:
        path: placement.availabilityZone
        width: 15
    detail:
      - instanceId
      - state
      - placement
      - securityGroups
      - tags
  s3:
    list:
      Bucket Name:
        path: name
        width: 40
      Creation Date:
        path: creationDate
        width: 22
    detail:
      - name
      - creationDate
  s3_objects:
    list:
      Key:
        path: key
        width: 50
      Size:
        path: size
        width: 12
```

### views_reference.yaml (shipped artifact, generated from SDK struct reflection)

```yaml
ec2:  # ec2types.Instance
  - instanceId
  - imageId
  - state.name
  - state.code
  - instanceType
  - placement.availabilityZone
  - placement.tenancy
  - privateIpAddress
  - publicIpAddress
  - launchTime
  - securityGroups[].groupId
  - securityGroups[].groupName
  - tags[].key
  - tags[].value
  - blockDeviceMappings[].deviceName
  - blockDeviceMappings[].ebs.volumeId
  # ... all fields from the SDK struct
```

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can change displayed columns in list views and fields in detail views for any resource type by editing a single YAML file, without recompiling — verified by modifying `views.yaml` and observing the change on next app launch.
- **SC-002**: The application starts and displays resource views within the same time as before (no perceptible delay from config loading or reflection).
- **SC-003**: 100% of currently supported resource types (S3, S3 objects, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager) are configurable for both list and detail views.
- **SC-004**: The reference generator produces a complete field path list for all supported resource types, generated from SDK struct reflection without live AWS calls.
- **SC-005**: A user unfamiliar with the codebase can customize their views within 5 minutes by consulting `views_reference.yaml` and editing `views.yaml`.
- **SC-006**: Users can reorder detail view fields by changing their position in the YAML file, and the detail view reflects that order.
- **SC-007**: Values are auto-formatted based on Go type (dates readable, bools as Yes/No) without any user configuration.

## Future Work

- Ship `views_reference.yaml` as a tracked artifact in the repository (FR-013)
- Display user-facing error message when `views.yaml` contains invalid YAML (FR-005)

## Related Documents

### QA Stories
- [10 — Configurable Views](../../docs/qa/10-configurable-views.md)

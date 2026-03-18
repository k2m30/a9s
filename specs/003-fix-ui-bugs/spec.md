# Feature Specification: Fix UI Bugs

**Feature Branch**: `003-fix-ui-bugs`
**Created**: 2026-03-16
**Status**: Partial
**Input**: User description: "Fix 15 UI bugs across filter, navigation, scrolling, status line, help, detail view, breadcrumbs, copy, YAML rendering, and config column widths."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Filter Works on Main Menu (Priority: P1)

As a user, I want the filter (`/` key) to work on the main menu view so I can quickly find a resource type by typing.

**Why this priority**: Basic functionality that should work everywhere.

**Independent Test**: Press `/` on main menu, type "ec2", verify the menu items are filtered.

**Acceptance Scenarios**:

1. **Given** the user is on the main menu, **When** they press `/` and type "ec2", **Then** only menu items containing "ec2" are shown.
2. **Given** the user is on the main menu with active filter, **When** they press Escape, **Then** the filter clears (does not exit the app).

---

### User Story 2 - S3 Navigation Preserves Selected Position (Priority: P1)

As a user, I want navigating back from inside an S3 bucket to restore my previously selected item, and navigating back from a prefix to restore the previous position.

**Why this priority**: Critical usability — losing position forces the user to scroll again through potentially hundreds of items.

**Independent Test**: Select the 5th bucket, drill in, press Escape, verify the 5th bucket is still highlighted.

**Acceptance Scenarios**:

1. **Given** the user selects the 5th S3 bucket and drills into it, **When** they press Escape to go back, **Then** the 5th bucket is highlighted.
2. **Given** the user is browsing objects in a prefix and selects the 3rd item, **When** they drill into a sub-prefix and press Escape, **Then** the 3rd item is highlighted.

---

### User Story 3 - Y Key Renders YAML (Priority: P1)

As a user, I want the `y` key to show the resource data as YAML, since the key is labeled `y` for YAML.

**Why this priority**: The key name implies YAML but it currently shows JSON.

**Independent Test**: Press `y` on any resource, verify the output is valid YAML (not JSON braces/brackets).

**Acceptance Scenarios**:

1. **Given** the user selects an EC2 instance and presses `y`, **Then** the raw data is displayed in YAML format.

---

### User Story 4 - Context-Aware Copy (Priority: P1)

As a user, I want the `c` key to copy context-relevant content: resource ID in list view, full detail YAML in detail view.

**Why this priority**: Copying just the ID from detail view is useless.

**Independent Test**: Open detail view, press `c`, paste into editor, verify it contains the full detail content.

**Acceptance Scenarios**:

1. **Given** the user is on the resource list view, **When** they press `c`, **Then** the selected resource's ID is copied to clipboard.
2. **Given** the user is on the detail view, **When** they press `c`, **Then** the full detail content (as rendered) is copied to clipboard.

---

### User Story 5 - Status Line Always at Bottom (Priority: P1)

As a user, I want the status bar to always appear at the last line of the terminal, even when content is short.

**Why this priority**: Floating status line looks broken.

**Acceptance Scenarios**:

1. **Given** the terminal is 40 lines tall and the content is only 5 lines, **Then** the status line renders at line 40.

---

### User Story 6 - Status Messages Clear on Navigation (Priority: P2)

As a user, I want status messages to clear when I navigate to a different view.

**Why this priority**: Stale messages are confusing.

**Acceptance Scenarios**:

1. **Given** a status message is displayed, **When** the user navigates to a different view, **Then** the status message is cleared.

---

### User Story 7 - Breadcrumbs Without "main" Prefix (Priority: P2)

As a user, I want "main" to appear in breadcrumbs only on the main menu page. Other pages omit "main >".

**Acceptance Scenarios**:

1. **Given** the user is on the main menu, **Then** breadcrumbs show "main".
2. **Given** the user is viewing EC2 instances, **Then** breadcrumbs show "EC2 Instances" (not "main > EC2 Instances").
3. **Given** the user is viewing S3 bucket contents, **Then** breadcrumbs show "S3 Buckets > bucket-name (139)".

---

### User Story 8 - All Views Scrollable (Priority: P1)

As a user, I want all views to support both vertical and horizontal scrolling when content exceeds screen dimensions.

**Acceptance Scenarios**:

1. **Given** content is wider than the terminal, **When** the user presses Right, **Then** the view scrolls horizontally.
2. **Given** content is taller than the terminal, **When** the user presses Down, **Then** the view scrolls vertically.

---

### User Story 9 - Horizontal Scroll Clamped to Content (Priority: P1)

As a user, I want horizontal scrolling to stop at the content edge — no scrolling into empty space, no dead presses when scrolling back.

**Acceptance Scenarios**:

1. **Given** the user has scrolled to the rightmost content edge, **When** they press Right again, **Then** the scroll position does not increase.
2. **Given** the user presses Left after max scroll, **Then** the view immediately scrolls left.

---

### User Story 10 - Horizontal Scroll Resets on Navigation (Priority: P2)

As a user, I want horizontal scroll to reset to 0 when navigating between views.

**Acceptance Scenarios**:

1. **Given** the user has scrolled right in EC2 list, **When** they navigate to RDS, **Then** horizontal scroll is 0.

---

### User Story 11 - Context-Sensitive Help (Priority: P2)

As a user, I want the help screen (`?` key) to show only keys relevant to the current view.

**Acceptance Scenarios**:

1. **Given** the user is on the list view and presses `?`, **Then** help shows list-specific keys only.
2. **Given** the user is on the detail view and presses `?`, **Then** help shows detail-specific keys only.

---

### User Story 12 - Header Bar Styling (Priority: P2)

As a user, I want the header bar without blue background, and the version on the right side.

**Acceptance Scenarios**:

1. **Given** the app is running, **Then** the header shows profile and region on the left, version on the right, no blue background.

---

### User Story 13 - Detail View Improvements (Priority: P1)

As a user, I want the detail view to: (a) not show " - Detail" suffix, (b) support `w` to toggle word wrap, (c) reset horizontal scroll when wrap is on, (d) render YAML with `:` immediately after the key.

**Acceptance Scenarios**:

1. **Given** the user opens detail for "my-instance", **Then** the title shows "my-instance" (not "my-instance - Detail").
2. **Given** the user presses `w` in detail view, **Then** long lines wrap to fit terminal width.
3. **Given** wrap is enabled, **Then** horizontal scroll is disabled and reset to 0.
4. **Given** a detail field renders, **Then** YAML shows `Key: value` (colon right after key, single space before value).

---

### User Story 14 - Resource Count in Breadcrumbs, Remove Duplicate Title (Priority: P2)

As a user, I want the resource count in breadcrumbs and the redundant title line removed.

**Acceptance Scenarios**:

1. **Given** the user is browsing 139 objects in a bucket, **Then** breadcrumbs show "S3 Buckets > bucket-name (139)".
2. **Then** the separate "S3: bucket-name (139)" title line below is removed.

---

### User Story 15 - Config Column Widths Applied (Priority: P1)

As a user, I want column width changes in `views.yaml` to take effect.

**Acceptance Scenarios**:

1. **Given** `views.yaml` sets "Bucket Name" width to 60, **When** the app starts, **Then** the Bucket Name column renders at 60 characters.

---

### Edge Cases

- Filter on main menu with no matches shows "No items matching filter" message.
- Pressing Escape on main menu with active filter clears filter first, doesn't exit app.
- Horizontal scroll on empty content doesn't crash.
- Copy on empty detail view copies empty string.
- Wrap toggle on very long single-line values doesn't crash.
- Help screen on main menu shows main-menu-specific keys.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Filter mode (`/` key) MUST work on the main menu view. — *Implemented*
- **FR-002**: Back navigation from S3 bucket/prefix MUST restore the previously selected item index. — *Not implemented: no cursor position stack for S3 back-navigation*
- **FR-003**: The `y` key MUST render resource data as YAML, not JSON. — *Implemented*
- **FR-004**: The `c` key MUST copy resource ID in list view, full detail YAML in detail view. — *Implemented*
- **FR-005**: The status bar MUST always render at the last terminal line. — *Implemented*
- **FR-006**: Status messages MUST clear on navigation change. — *Partial: flash clears on timeout but not explicitly on view navigation*
- **FR-007**: Breadcrumbs MUST show "main" only on the main menu. — *Implemented*
- **FR-008**: Breadcrumbs MUST include resource count (e.g., "bucket-name (139)"). — *Partial: resource list count works, S3 object count format uncertain*
- **FR-009**: The duplicate title line below breadcrumbs MUST be removed. — *Implemented*
- **FR-010**: All views MUST support vertical scrolling. — *Implemented*
- **FR-011**: All views MUST support horizontal scrolling. — *Implemented*
- **FR-012**: Horizontal scroll MUST clamp to content width — no over-scrolling, no dead presses. — *Partial: left scroll clamped, right scroll can overflow beyond content*
- **FR-013**: Horizontal scroll MUST reset to 0 on navigation change. — *Not implemented: horizontal scroll offset persists across navigation*
- **FR-014**: Help screen MUST show only keys for the current view. — *Implemented*
- **FR-015**: Header bar MUST have no blue background. Version on the right. — *Implemented*
- **FR-016**: Detail view title MUST not include " - Detail". — *Implemented*
- **FR-017**: Detail view MUST support `w` key to toggle word wrap. Wrap disables horizontal scroll. — *Implemented*
- **FR-018**: Detail view YAML MUST render `Key: value` (colon right after key). — *Implemented*
- **FR-019**: Column widths from `views.yaml` MUST be applied in list view rendering. — *Implemented*

## Assumptions

- YAML rendering for `y` key uses the same safe-export logic as the detail view (handles unexported fields).
- Filter on main menu filters resource type names by case-insensitive substring.
- Copy in detail view copies the visible rendered text.
- Word wrap breaks at terminal width, mid-word if necessary.
- All 15 bugs are tested FIRST before implementation (TDD per constitution and user instruction).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 15 bug fixes verified by dedicated tests — each has at least one test that catches the original bug.
- **SC-002**: All 8 resource types work correctly with every view — no panics, no empty screens.
- **SC-003**: Navigation state preserved across all back-navigation paths.
- **SC-004**: Horizontal scroll never exceeds content bounds.
- **SC-005**: All existing tests pass with zero regressions.

## Future Work

- S3 back-navigation cursor position restore (FR-002)
- Horizontal scroll clamping on right boundary (FR-012)
- Reset horizontal scroll offset on view navigation (FR-013)
- Clear flash messages explicitly on view navigation (FR-006)

## Related Documents

### Design
- [Detail View Design](../../docs/design/detail-view.md) — Detail view layout redesign

### QA Stories
- [11 — Filtering](../../docs/qa/11-filtering.md)
- [12 — Help Screen](../../docs/qa/12-help-screen.md)

### Architecture
- [Architecture Audit](../../docs/architecture-audit.md) — Root cause analysis of production bugs

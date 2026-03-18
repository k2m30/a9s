# QA Stories: Filtering (/ key)

Filtering activates via the `/` key. It shows a live `/text` indicator in the header
right side, narrows the visible list items by case-insensitive substring match, and
updates the frame title to show `(matched/total)` counts. `Esc` clears the filter;
`Enter` confirms (exits filter mode but keeps the filter applied); `Backspace` removes
the last character.

---

## Navigable Views -- Filter Works

### 11-01: Main menu -- / activates filter mode

**Given** the main menu is displayed with all 7 resource types
**When** the user presses `/`
**Then** filter mode activates: the header right side shows `/` (amber)
**And** subsequent character keystrokes are appended to the filter string
**And** only resource types whose Name or ShortName match the filter substring are displayed

### 11-02: Main menu -- filter "ec2" shows only EC2

**Given** the main menu is displayed
**When** the user presses `/` then types `ec2`
**Then** only "EC2 Instances" is visible in the menu
**And** the header shows `/ec2`
**And** the frame title shows `resource-types(1/7)`

### 11-03: Main menu -- filter "s3" shows only S3

**Given** the main menu is displayed
**When** the user presses `/` then types `s3`
**Then** only "S3 Buckets" is visible
**And** the frame title shows `resource-types(1/7)`

### 11-04: Main menu -- filter "xxx" shows nothing

**Given** the main menu is displayed
**When** the user presses `/` then types `xxx`
**Then** no resource types are shown (empty list or "No resource types" message)
**And** the frame title shows `resource-types(0/7)`

### 11-05: Main menu -- filter is case-insensitive

**Given** the main menu is displayed
**When** the user presses `/` then types `EC2` (uppercase)
**Then** "EC2 Instances" still appears (match is case-insensitive)

### 11-06: Main menu -- backspace removes characters

**Given** the main menu with filter `/ec2` active
**When** the user presses Backspace
**Then** the filter becomes `/ec` and items matching "ec" are shown
**When** the user presses Backspace twice more
**Then** the filter becomes `/` (empty) and all 7 items return

### 11-07: Main menu -- frame title updates with filtered count

**Given** the main menu is displayed
**When** the user presses `/` then types `e`
**Then** the frame title shows `resource-types(N/7)` where N is the number of types matching "e"

### 11-08: Main menu -- Esc clears filter

**Given** the main menu with filter `/ec2` active
**When** the user presses Esc
**Then** filter mode deactivates: all 7 items reappear
**And** the header right side returns to `? for help`
**And** the frame title returns to `resource-types(7)`

### 11-09: Main menu -- Enter confirms filter (keeps narrowed view)

**Given** the main menu with filter `/ec2` active (showing 1 item)
**When** the user presses Enter while in filter mode
**Then** filter mode deactivates (header returns to normal)
**But** the filtered set is preserved -- only EC2 is still shown

### 11-10: Resource list -- / activates live-filter

**Given** a resource list (e.g. ec2) with loaded resources
**When** the user presses `/` then types a filter string
**Then** only resources matching by ID, Name, Status, or any Field value are shown
**And** the header shows `/filter-text`

### 11-11: Resource list -- filter persists across scroll

**Given** a resource list with filter active showing N matching items
**When** the user scrolls down with `j` key
**Then** the filter remains active and only matching items are displayed

### 11-12: Resource list -- filter clears on Esc

**Given** a resource list with filter `/prod` active
**When** the user presses Esc
**Then** all resources reappear and filter mode is deactivated

### 11-13: Resource list -- cursor resets to 0 when filter changes

**Given** a resource list with cursor at row 5
**When** the filter text changes (character typed or deleted)
**Then** the cursor resets to 0 (top of the filtered list)

### 11-14: Resource list -- frame title shows filtered count

**Given** a resource list with 10 total resources
**When** the user types a filter matching 3 resources
**Then** the frame title shows e.g. `ec2(3/10)`

### 11-15: Profile selector -- / should filter profiles by name

**Given** the profile selector is displayed with multiple profiles
**When** the user presses `/` then types part of a profile name
**Then** only profiles matching the substring are shown
**And** the frame title shows `aws-profiles(N/M)` filtered count

### 11-16: Region selector -- / should filter regions by name

**Given** the region selector is displayed with multiple regions
**When** the user presses `/` then types e.g. `us-east`
**Then** only regions matching `us-east` are shown
**And** the frame title shows `aws-regions(N/M)` filtered count

---

## Static Views -- Filter Does NOT Activate

### 11-17: Detail view -- / key is ignored

**Given** the detail view is displayed for a resource
**When** the user presses `/`
**Then** filter mode does NOT activate (header stays `? for help`, no `/` indicator)

### 11-18: YAML view -- / key is ignored

**Given** the YAML view is displayed for a resource
**When** the user presses `/`
**Then** filter mode does NOT activate

### 11-19: Help view -- / key closes help

**Given** the help view is displayed
**When** the user presses `/`
**Then** the help view closes (any key closes help) -- filter mode does NOT activate

### 11-20: Reveal view -- / key is ignored

**Given** the reveal view is displaying a secret value
**When** the user presses `/`
**Then** filter mode does NOT activate

---

## Edge Cases

### 11-21: Filter with special characters (dots, dashes, underscores)

**Given** a resource list with resource names like `api-prod-01`, `app.service`, `my_bucket`
**When** the user types `/api-prod`
**Then** matching works correctly with special characters (literal substring, not regex)

### 11-22: Filter that matches everything (single common letter)

**Given** a resource list with 10 resources all containing the letter "a"
**When** the user types `/a`
**Then** all 10 resources are shown
**And** the frame title shows `ec2(10)` (no filtered count when all match)

### 11-23: Filter then navigate to detail then back -- filter cleared

**Given** a resource list with filter `/prod` active
**When** the user presses Enter to open detail view, then Esc to return
**Then** the filter is cleared (user returns to unfiltered resource list)
**Note**: filter state lives in the root model's cmdInput; popping/pushing resets it.

### 11-24: Filter active then Escape clears, then Escape again goes back

**Given** a resource list with filter `/prod` active
**When** the user presses Esc
**Then** the filter clears but the user stays on the resource list
**When** the user presses Esc again
**Then** the user navigates back to the previous view (main menu)

### 11-25: Very long filter string

**Given** any navigable list view
**When** the user types a 50+ character filter string
**Then** the filter still works (no crash, no truncation of match logic)
**And** the header right side shows the full filter text

### 11-26: Filter mode then resize terminal

**Given** filter mode is active with `/prod` typed
**When** a WindowSizeMsg arrives (terminal resize)
**Then** filter mode remains active with the same filter text
**And** the filtered results are still displayed

### 11-27: Filter on empty resource list

**Given** a resource list that has loaded but contains 0 resources
**When** the user presses `/` and types anything
**Then** the filter activates in the header but the list remains empty
**And** no crash or panic occurs

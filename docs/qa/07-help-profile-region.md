# QA-07: Help View, Profile Selector, Region Selector, and Cross-Cutting Concerns

---

## HELP VIEW

### HV-01: Open help from main menu

**Given** the main menu is displayed
**When** I press `?`
**Then** the frame content is replaced by the help screen
**And** the frame title reads "Help" centered in the top border
**And** the header bar remains unchanged (app name, version, profile:region on left)

### HV-02: Open help from resource list

**Given** a resource list is displayed (e.g., ec2-instances)
**When** I press `?`
**Then** the frame content is replaced by the help screen
**And** the frame title changes to "Help"
**And** my previous position in the resource list is preserved for when I return

### HV-03: Open help from detail view

**Given** a detail view is displayed for any resource
**When** I press `?`
**Then** the frame content is replaced by the help screen
**And** the frame title changes to "Help"

### HV-04: Open help from YAML view

**Given** a YAML view is displayed for any resource
**When** I press `?`
**Then** the frame content is replaced by the help screen
**And** the frame title changes to "Help"

### HV-05: Four-column layout

**Given** the help screen is displayed
**Then** I see exactly four column headers: RESOURCE, GENERAL, NAVIGATION, HOTKEYS
**And** each column header is bold, uppercase, in orange/yellow color
**And** the columns divide the available width into four roughly equal sections

### HV-06: RESOURCE column contents

**Given** the help screen is displayed
**Then** the RESOURCE column lists at minimum:
  - `<esc>` with description "Back"
  - `<q>` with description "Quit"

### HV-07: GENERAL column contents

**Given** the help screen is displayed
**Then** the GENERAL column lists at minimum:
  - `<ctrl-r>` with description "Refresh"
  - `<q>` with description "Quit"
  - `<:>` with description "Command"
  - `</>` with description "Filter"

### HV-08: NAVIGATION column contents

**Given** the help screen is displayed
**Then** the NAVIGATION column lists at minimum:
  - `<j>` with description "Down"
  - `<k>` with description "Up"
  - `<g>` with description "Top"
  - `<G>` with description "Bottom"
  - `<h/l>` with description "Cols"
  - `<enter>` with description "Open"
  - `<d>` with description "Detail"
  - `<y>` with description "YAML"
  - `<c>` with description "Copy ID"
  - `<N/S/A>` with description "Sort"

### HV-09: HOTKEYS column contents

**Given** the help screen is displayed
**Then** the HOTKEYS column lists at minimum:
  - `<?>` with description "Help"
  - `<:>` with description "Command" (or "Cmd")

### HV-10: Key styling in help

**Given** the help screen is displayed
**Then** all key bindings (e.g., `<esc>`, `<j>`, `<ctrl-r>`) are rendered in green bold text
**And** all descriptions (e.g., "Back", "Down") are rendered in plain white text

### HV-11: Close help with any key

**Given** the help screen is displayed
**When** I press any single key (letter, number, arrow, space, etc.)
**Then** the help screen closes
**And** I return to the view I was in before opening help

### HV-12: Close help with Escape

**Given** the help screen is displayed
**When** I press `Escape`
**Then** the help screen closes
**And** I return to the previous view

### HV-13: Help replaces content, not overlay

**Given** the help screen is displayed
**Then** the help content appears inside the frame borders (not as a floating overlay)
**And** the frame's top and bottom borders remain visible
**And** there is no underlying content visible behind the help text

### HV-14: "Press any key to close" hint

**Given** the help screen is displayed
**Then** at the bottom of the help content, I see dim text reading "Press any key to close"

### HV-15: Help preserves return context

**Given** I am on a resource list with cursor on row 5 and a filter "/prod" active
**When** I press `?` to open help
**And** then press any key to close help
**Then** I return to the resource list
**And** the cursor is still on row 5
**And** the filter "/prod" is still active

---

## PROFILE SELECTOR

### PS-01: Open profile selector via :ctx command

**Given** any view is displayed
**When** I press `:` to enter command mode
**And** I type `ctx` and press Enter
**Then** the frame content is replaced by a list of AWS profiles
**And** the frame title reads "aws-profiles(N)" where N is the total number of profiles

### PS-02: Open profile selector via :profile command

**Given** any view is displayed
**When** I press `:` to enter command mode
**And** I type `profile` and press Enter
**Then** the frame content is replaced by the profile selector list
**And** the frame title reads "aws-profiles(N)"

### PS-03: Profile list matches AWS config

**Given** the profile selector is displayed
**Then** the listed profiles match those returned by `aws configure list-profiles`
**And** profiles are sourced from both ~/.aws/config and ~/.aws/credentials

### PS-04: Current profile indicator

**Given** the profile selector is displayed
**And** the current active profile is "prod"
**Then** the entry for "prod" shows "(current)" in dim text next to the profile name
**And** no other entry shows "(current)"

### PS-05: Profile count in frame title

**Given** there are exactly 6 AWS profiles configured
**When** I open the profile selector
**Then** the frame title reads "aws-profiles(6)"

### PS-06: Navigate profiles with j/k

**Given** the profile selector is displayed
**When** I press `j` or the down-arrow key
**Then** the cursor moves down to the next profile in the list
**When** I press `k` or the up-arrow key
**Then** the cursor moves up to the previous profile in the list

### PS-07: Navigate profiles with arrow keys

**Given** the profile selector is displayed
**When** I press the down-arrow key
**Then** the cursor moves to the next profile
**When** I press the up-arrow key
**Then** the cursor moves to the previous profile

### PS-08: Select a different profile with Enter

**Given** the profile selector is displayed
**And** my cursor is on profile "staging" (not the current profile)
**When** I press Enter
**Then** the profile selector closes
**And** the header updates to show "staging" as the active profile (left side: `a9s v0.x.x  staging:us-east-1`)
**And** the application reconnects to AWS using the "staging" profile credentials
**And** I return to the previous view
**And** the previous view refreshes its data using the new profile

### PS-09: Select the already-current profile

**Given** the profile selector is displayed
**And** my cursor is on the profile marked "(current)"
**When** I press Enter
**Then** the profile selector closes
**And** I return to the previous view
**And** no unnecessary reconnection or visual disruption occurs

### PS-10: Cancel profile selection with Escape

**Given** the profile selector is displayed
**When** I press Escape
**Then** the profile selector closes
**And** I return to the previous view
**And** the active profile remains unchanged
**And** no data refresh occurs

### PS-11: Selected row styling in profile list

**Given** the profile selector is displayed
**Then** exactly one row (the cursor row) is highlighted with the standard selected-row styling (blue background, dark foreground, bold)

### PS-12: Single profile configured

**Given** only one AWS profile is configured ("default")
**When** I open the profile selector
**Then** the frame title reads "aws-profiles(1)"
**And** the single entry "default" is listed with "(current)" next to it

### PS-13: Profile selector from different views

**Given** I am on the detail view for an EC2 instance
**When** I open the profile selector via `:ctx`
**And** I select a new profile and press Enter
**Then** I return to the view I was on before opening the profile selector
**And** that view refreshes with data from the new profile

---

## REGION SELECTOR

### RS-01: Open region selector via :region command

**Given** any view is displayed
**When** I press `:` to enter command mode
**And** I type `region` and press Enter
**Then** the frame content is replaced by a list of AWS regions
**And** the frame title reads "aws-regions(N)" where N is the total number of regions

### RS-02: Region list contents

**Given** the region selector is displayed
**Then** I see standard AWS regions including at minimum:
  - us-east-1
  - us-east-2
  - us-west-1
  - us-west-2
  - eu-west-1
  - eu-central-1
  - ap-southeast-1
  - ap-northeast-1

### RS-03: Current region indicator

**Given** the current region is "us-east-1"
**And** the region selector is displayed
**Then** the entry for "us-east-1" shows "(current)" next to it
**And** no other entry shows "(current)"

### RS-04: Region count in frame title

**Given** there are N AWS regions listed
**When** I open the region selector
**Then** the frame title reads "aws-regions(N)" with the correct count

### RS-05: Navigate regions with j/k

**Given** the region selector is displayed
**When** I press `j` or the down-arrow key
**Then** the cursor moves down to the next region
**When** I press `k` or the up-arrow key
**Then** the cursor moves up to the previous region

### RS-06: Navigate regions with arrow keys

**Given** the region selector is displayed
**When** I press the down-arrow key
**Then** the cursor moves to the next region
**When** I press the up-arrow key
**Then** the cursor moves to the previous region

### RS-07: Select a different region with Enter

**Given** the region selector is displayed
**And** my cursor is on "eu-west-1" (not the current region)
**When** I press Enter
**Then** the region selector closes
**And** the header updates to show "eu-west-1" as the active region (e.g., `a9s v0.x.x  prod:eu-west-1`)
**And** the application reconnects to AWS targeting "eu-west-1"
**And** I return to the previous view
**And** the previous view refreshes its data for the new region

### RS-08: Select the already-current region

**Given** the region selector is displayed
**And** my cursor is on the region marked "(current)"
**When** I press Enter
**Then** the region selector closes
**And** I return to the previous view
**And** no unnecessary reconnection or visual disruption occurs

### RS-09: Cancel region selection with Escape

**Given** the region selector is displayed
**When** I press Escape
**Then** the region selector closes
**And** I return to the previous view
**And** the active region remains unchanged
**And** no data refresh occurs

### RS-10: Selected row styling in region list

**Given** the region selector is displayed
**Then** exactly one row (the cursor row) is highlighted with the standard selected-row styling (blue background, dark foreground, bold)

### RS-11: Region selector from resource list

**Given** I am viewing the EC2 instances list for us-east-1
**When** I open the region selector via `:region`
**And** I select "ap-southeast-1" and press Enter
**Then** I return to the EC2 instances list
**And** the list now shows EC2 instances from ap-southeast-1
**And** the header shows "ap-southeast-1" as the active region

---

## CROSS-CUTTING CONCERNS

### Flash Messages

#### FM-01: Flash message position

**Given** any flash message is triggered (success or error)
**Then** the message appears on the right side of the header bar
**And** it replaces the "? for help" hint text while visible

#### FM-02: Flash message auto-clear

**Given** a flash message is displayed
**When** approximately 2 seconds elapse
**Then** the flash message disappears
**And** the header right side reverts to "? for help"

#### FM-03: Copy confirmation flash

**Given** I am on a resource list
**When** I press `c` to copy a resource ID
**Then** a green bold flash message reading "Copied!" appears in the header right
**And** it auto-clears after approximately 2 seconds

#### FM-04: Refresh flash

**Given** I am on any view that loads AWS data
**When** I press `ctrl-r` to refresh
**Then** a flash message reading "Refreshing..." appears in the header right
**And** it is replaced by fresh data once the refresh completes

#### FM-05: Flash message replaces previous flash

**Given** a flash message "Copied!" is currently displayed
**When** another action triggers a new flash message (e.g., another copy)
**Then** the new flash message replaces the previous one
**And** the 2-second auto-clear timer resets

### Error Messages

#### EM-01: Error message styling

**Given** an error occurs (e.g., AWS API failure, no credentials)
**Then** a red bold message appears in the header right side
**And** the message is prefixed with "Error:" or clearly indicates the error

#### EM-02: Error message display

**Given** an API error occurs while loading resources
**Then** the error message appears in the header right
**And** the message text describes the error (e.g., "no credentials", "access denied")

#### EM-03: Error message auto-clear

**Given** an error flash message is displayed
**When** approximately 2 seconds elapse
**Then** the error message disappears
**And** the header right reverts to "? for help"

### Terminal Resize

#### TR-01: Resize during resource list

**Given** a resource list is displayed
**When** I resize the terminal window (wider or narrower)
**Then** the frame borders adapt to the new width
**And** table columns adjust according to width breakpoints
**And** no content is cut off or corrupted

#### TR-02: Resize during help view

**Given** the help screen is displayed
**When** I resize the terminal window
**Then** the four-column layout reflows to fit the new width
**And** the frame borders adapt to the new size

#### TR-03: Resize during profile selector

**Given** the profile selector is displayed
**When** I resize the terminal window
**Then** the frame and profile list adapt to the new dimensions

#### TR-04: Resize during region selector

**Given** the region selector is displayed
**When** I resize the terminal window
**Then** the frame and region list adapt to the new dimensions

#### TR-05: Resize during detail view

**Given** the detail view is displayed
**When** I resize the terminal window
**Then** the key-value layout and frame borders adapt
**And** scroll position is preserved

#### TR-06: Minimum width enforcement

**Given** any view is displayed
**When** I resize the terminal to fewer than 60 columns
**Then** I see an error message: "Terminal too narrow. Please resize."
**And** no partial or garbled content is shown

#### TR-07: Minimum height enforcement

**Given** any view is displayed
**When** I resize the terminal to fewer than 7 lines
**Then** I see an error message: "Terminal too short. Please resize."
**And** no partial or garbled content is shown

#### TR-08: Recovery from too-small terminal

**Given** the terminal is currently too narrow (< 60 cols) and showing the "too narrow" error
**When** I resize the terminal back to 80 or more columns
**Then** the normal view is restored
**And** I return to whichever view I was on before the resize

### Profile/Region Switch Data Refresh

#### PR-01: Profile switch refreshes resource list

**Given** I am viewing EC2 instances for profile "prod"
**When** I switch to profile "staging" via the profile selector
**Then** the EC2 instances list refreshes
**And** the displayed instances are from the "staging" account
**And** the header shows "staging" on the left

#### PR-02: Region switch refreshes resource list

**Given** I am viewing S3 buckets for region "us-east-1"
**When** I switch to region "eu-west-1" via the region selector
**Then** the resource list refreshes
**And** the displayed resources are from "eu-west-1"
**And** the header shows "eu-west-1" on the left

#### PR-03: Profile switch refreshes all resource types

**Given** I switch to a new profile
**When** I subsequently navigate to any resource type (EC2, S3, RDS, Redis, DocumentDB, EKS, Secrets Manager)
**Then** the data shown is from the newly selected profile
**And** no stale data from the previous profile is visible

#### PR-04: Region switch refreshes all resource types

**Given** I switch to a new region
**When** I subsequently navigate to any resource type
**Then** the data shown is from the newly selected region
**And** no stale data from the previous region is visible

#### PR-05: Profile switch resets cursor position

**Given** I am on a resource list with cursor on row 10
**When** I switch profiles and the list refreshes
**Then** the cursor resets to the first row (or remains valid within the new data set)
**And** no out-of-bounds cursor position occurs

#### PR-06: Region switch resets cursor position

**Given** I am on a resource list with cursor on row 10
**When** I switch regions and the list refreshes
**Then** the cursor resets to the first row (or remains valid within the new data set)
**And** no out-of-bounds cursor position occurs

#### PR-07: Profile switch with loading indicator

**Given** I select a new profile
**When** the application is fetching data with the new credentials
**Then** a loading spinner or "Refreshing..." indicator is shown
**And** the UI does not freeze or become unresponsive

#### PR-08: Region switch with loading indicator

**Given** I select a new region
**When** the application is fetching data for the new region
**Then** a loading spinner or "Refreshing..." indicator is shown
**And** the UI does not freeze or become unresponsive

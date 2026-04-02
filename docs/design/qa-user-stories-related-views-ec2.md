## Header

### Story: Normal header shows app, version, and AWS context
**Given:** the user is on the main menu or any EC2-related view in normal mode
**When:** the screen is idle with no active filter, command, or flash message
**Then:** the left side of the header shows the app name, version, and `profile:region`, and the right side shows `? for help`

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Filter mode takes over the right side of the header
**Given:** the user is on a filterable list or detail column
**When:** the user presses `/`
**Then:** the right side of the header changes from `? for help` to an inline filter/search prompt that shows the typed query and cursor

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Command mode takes over the right side of the header
**Given:** the user is on any screen
**When:** the user presses `:`
**Then:** the right side of the header changes from `? for help` to an inline command prompt, while the frame content below stays visible

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Success flashes appear without changing the active view
**Given:** the user performs an action that succeeds and produces feedback, such as copy
**When:** the action completes
**Then:** a success message appears in the header, the frame content does not change, and the success flash later clears automatically

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Error flashes are visible in the header without replacing the frame
**Given:** a load or navigation action fails
**When:** the error is surfaced to the user
**Then:** the header shows an error message on the right side, and the user can still see the current framed content underneath

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Narrow headers may drop the help hint before the main context
**Given:** the terminal is narrower than the comfortable width but still usable
**When:** the user views any screen
**Then:** the app keeps the left-side app and AWS context readable first, and the right-side help hint may disappear if there is not enough space

**AWS comparison:**
aws ec2 describe-instances --max-items 5
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: The screen does not show a separate status bar
**Given:** the user is on any view in the EC2 related-views workflow
**When:** the screen is rendered
**Then:** the layout shows one unframed header line and one framed content area, with no extra status bar at the bottom

**AWS comparison:**
aws ec2 describe-instances --max-items 5
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

## Frame

### Story: Frame title is centered in the top border
**Given:** the user is on a list, detail, YAML, help, selector, or related-result view
**When:** the frame renders
**Then:** the view title appears centered inside the top border instead of on a separate line above the frame

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Help replaces frame content instead of opening as an overlay
**Given:** the user is on the main menu, a resource list, or a detail view
**When:** the user presses `?`
**Then:** the frame content switches to a help screen in the same framed area, rather than drawing a floating dialog over the current view

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Loading state is centered inside the frame
**Given:** a view is still fetching AWS data
**When:** the load is in progress
**Then:** the frame content area shows a centered spinner and fetch message instead of partial rows

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Empty state uses the frame instead of a blank table
**Given:** a list loads successfully but contains no rows for the active account, region, or filter
**When:** the user views the list
**Then:** the frame shows a centered empty-state message with a hint to refresh or change region, rather than an apparently broken blank screen

**AWS comparison:**
aws ec2 describe-instances --filters Name=instance-state-name,Values=terminated
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Global escape returns to the previous framed view
**Given:** the user has navigated from the main menu into a list, detail view, or YAML view
**When:** the user presses `esc` in normal mode
**Then:** the app returns to the previous framed screen in the stack

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Force quit works from any framed screen
**Given:** the user is on any screen
**When:** the user presses `ctrl+c`
**Then:** the application exits immediately from the current view

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

## Main Menu

### Story: Main menu shows resource type names and command aliases
**Given:** the app opens to the main menu
**When:** the user looks at the initial frame
**Then:** the frame shows the resource type names in a vertical list and each row includes the visible `:alias` command on the right

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Main menu cursor wraps from bottom to top and top to bottom
**Given:** the main menu list is focused
**When:** the user presses `j` or `down` past the last row, or `k` or `up` past the first row
**Then:** the selection wraps instead of getting stuck

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Main menu jump keys move to first and last resource type
**Given:** the main menu is focused
**When:** the user presses `g` or `G`
**Then:** the selection jumps to the first or last resource type row

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Enter opens the selected resource type list
**Given:** the EC2 row is selected in the main menu
**When:** the user presses `enter`
**Then:** the main menu is replaced by the EC2 resource list

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Main menu help closes back to the same selection
**Given:** the main menu is focused on a specific resource type
**When:** the user presses `?` and then closes help with any key or `esc`
**Then:** the main menu reappears with the same item still selected

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Main menu command mode can open EC2 directly
**Given:** the user is on the main menu
**When:** the user presses `:`, types `ec2`, and presses `enter`
**Then:** the EC2 list opens without moving the main-menu selection manually

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Main menu command autocomplete is visible before execution
**Given:** the user is in command mode from the main menu
**When:** the user types part of a known command and presses `tab`
**Then:** the visible command input completes to the known command without changing the frame content below

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Main menu command cancel returns to normal mode
**Given:** the user has opened command mode from the main menu
**When:** the user presses `esc`
**Then:** the command prompt disappears and the header returns to `? for help` without leaving the menu

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Quit key is honored only at the main menu
**Given:** the user is at the main menu
**When:** the user presses `q`
**Then:** the application exits from the root context

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

## Profile Selector

### Story: Profile selector shows current and unavailable profiles distinctly
**Given:** the user opens the AWS profile selector
**When:** the selector frame renders
**Then:** the current profile is visibly marked and profiles without credentials are dimmed so they are visually different from selectable healthy entries

**AWS comparison:**
aws configure list-profiles
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Choosing a different profile updates the visible AWS context
**Given:** the profile selector is open
**When:** the user selects a different valid profile and confirms it
**Then:** the app returns to the main flow with the header updated to the new `profile:region`

**AWS comparison:**
aws sts get-caller-identity --profile <profile>
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

## Region Selector

### Story: Region selector returns to the previous screen after selection
**Given:** the user opens the region selector from the current context
**When:** the user chooses a region
**Then:** the app returns to the prior view context with the header region changed

**AWS comparison:**
aws ec2 describe-regions
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Region change leads to fresh EC2 list content
**Given:** the user switches regions and then opens EC2
**When:** the EC2 list loads in the new region
**Then:** the visible rows and counts reflect the newly selected region instead of reusing the old region's screen state

**AWS comparison:**
aws ec2 describe-instances --region <region>
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

## Help

### Story: Help screen shows grouped categories rather than raw key dumps
**Given:** the user presses `?` from a supported view
**When:** the help screen appears
**Then:** the frame shows grouped categories such as resource, general, navigation, and hotkeys in aligned columns

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Any key closes help
**Given:** help is open
**When:** the user presses any ordinary key
**Then:** help closes and the previous screen is restored

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Escape also closes help
**Given:** help is open
**When:** the user presses `esc`
**Then:** help closes and the prior screen is restored

**AWS comparison:**
aws ec2 describe-instances --max-items 1
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

## EC2 Instances List

### Story: Full-width EC2 list shows all configured columns
**Given:** the terminal is wide enough for the full layout
**When:** the EC2 list is open
**Then:** the visible table includes all configured EC2 list columns in order

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Medium-width EC2 list keeps the leftmost configured columns first
**Given:** the terminal is between the standard and full-width breakpoints
**When:** the EC2 list is open
**Then:** the user still sees the leftmost EC2 columns first, starting with Name, State, Lifecycle, and Type, with additional columns available by horizontal scroll

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type

### Story: Narrow but usable EC2 list still exposes the primary columns
**Given:** the terminal is between the minimum usable width and the standard layout width
**When:** the EC2 list is open
**Then:** the user sees a reduced visible subset focused on the leftmost columns instead of a broken wide table

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State

### Story: EC2 list refuses to render as a broken table below the minimum width
**Given:** the terminal is narrower than the minimum supported width
**When:** the user tries to view the EC2 list
**Then:** the screen shows a too-narrow resize message instead of clipped rows

**AWS comparison:**
aws ec2 describe-instances --max-items 5
Expected fields visible: Name, State

### Story: EC2 list refuses to render as a broken table below the minimum height
**Given:** the terminal is shorter than the minimum supported height
**When:** the EC2 list is opened or resized into that state
**Then:** the screen shows a too-short resize message instead of partial table chrome

**AWS comparison:**
aws ec2 describe-instances --max-items 5
Expected fields visible: Name, State

### Story: Running rows are colored differently from stopped and terminated rows
**Given:** the EC2 list contains instances in mixed states
**When:** the rows are rendered
**Then:** running rows appear visually healthy, pending rows appear cautionary, stopped rows appear failed/stopped, and terminated rows appear dimmed

**AWS comparison:**
aws ec2 describe-instances --filters Name=instance-state-name,Values=running,stopped,pending,terminated
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Selected row highlight overrides state-based row coloring
**Given:** the EC2 list contains colored rows for different statuses
**When:** the user moves the cursor onto any row
**Then:** the selected row uses the selection highlight across the full row, regardless of its original status color

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Table headers are shown without separator lines
**Given:** the EC2 list is open
**When:** the user looks at the table header
**Then:** the column names appear as a single header row without vertical pipes and without an underline separator row beneath them

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Cursor movement works one row at a time
**Given:** the EC2 list is focused
**When:** the user presses `j`, `k`, `down`, or `up`
**Then:** the selection moves one visible row at a time

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Jump keys move to first and last EC2 row
**Given:** the EC2 list is focused
**When:** the user presses `g` or `G`
**Then:** the selection jumps to the first or last row in the current list

**AWS comparison:**
aws ec2 describe-instances --max-items 100
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Page navigation moves by visible page height
**Given:** the EC2 list contains more rows than fit in the frame
**When:** the user presses `pgup`, `pgdn`, `ctrl+u`, or `ctrl+d`
**Then:** the view scrolls by larger page-sized increments while keeping a selected row visible

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Horizontal scrolling reveals off-screen columns
**Given:** not all EC2 columns fit in the current width
**When:** the user presses `h`, `l`, `left`, or `right`
**Then:** the visible column window shifts horizontally, and headers stay aligned with the row cells

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Name sort toggles direction on repeated key presses
**Given:** the EC2 list is open
**When:** the user presses `N` repeatedly
**Then:** the list sorts by the visible name column and the active header shows ascending or descending direction

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Status sort toggles direction on repeated key presses
**Given:** the EC2 list is open
**When:** the user presses `S` repeatedly
**Then:** the list sorts by status and the active header shows ascending or descending direction

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Age sort toggles direction on repeated key presses
**Given:** the EC2 list is open
**When:** the user presses `A` repeatedly
**Then:** the list sorts by age or launch time and the active header shows ascending or descending direction

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: EC2 list filter updates the frame title count
**Given:** the EC2 list is open with many rows
**When:** the user presses `/` and types a filter
**Then:** the frame title changes from total count to matched-over-total count, and only matching rows remain visible

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: EC2 list filtering does not highlight matching text inside cells
**Given:** the EC2 list is filtered
**When:** the user looks at the remaining rows
**Then:** only the row set changes; the matched text inside the row cells is not specially highlighted

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: EC2 list filter can be edited with backspace
**Given:** the user is typing into the list filter
**When:** the user presses `backspace`
**Then:** the visible query shortens and the visible row set updates accordingly

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Escape clears an active EC2 list filter before leaving the view
**Given:** the EC2 list has an active filter
**When:** the user presses `esc`
**Then:** the filter is cleared first and the full EC2 list returns, rather than immediately leaving the list view

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Command mode from the EC2 list does not hide the table
**Given:** the EC2 list is open
**When:** the user presses `:`
**Then:** the command input appears in the header while the EC2 table stays visible underneath

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Loading state uses a centered fetch message instead of partial list rows
**Given:** the EC2 list is being fetched or refreshed
**When:** the user waits for rows
**Then:** the frame shows a spinner and fetch message rather than stale partial rows mixed with loading chrome

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Empty EC2 account or region is still a valid screen state
**Given:** the current region has no EC2 instances visible to the user
**When:** the EC2 list finishes loading
**Then:** the user sees an explicit empty-state message rather than an apparently broken blank table

**AWS comparison:**
aws ec2 describe-instances --region <empty-region>
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Copy from the EC2 list produces visible success feedback
**Given:** an EC2 row is selected
**When:** the user presses `c`
**Then:** the current row stays selected and a success flash appears in the header

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Reveal key does not open a secret view from EC2
**Given:** the EC2 list is open
**When:** the user presses `x`
**Then:** no secret reveal screen appears because the current resource type is not Secrets Manager

**AWS comparison:**
aws ec2 describe-instances --max-items 5
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Enter opens EC2 detail from the selected row
**Given:** an EC2 row is selected
**When:** the user presses `enter`
**Then:** the selected instance opens in detail view

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Detail shortcut opens the same EC2 detail screen as Enter
**Given:** an EC2 row is selected
**When:** the user presses `d`
**Then:** the same EC2 detail screen opens as if the user had pressed `enter`

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: YAML shortcut opens the EC2 YAML view
**Given:** an EC2 row is selected
**When:** the user presses `y`
**Then:** the framed content switches to YAML for the selected EC2 resource

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-... --output yaml
Expected fields visible: InstanceId, State, Placement, SecurityGroups, BlockDeviceMappings, Tags

### Story: Refresh re-fetches the EC2 list in place
**Given:** the EC2 list is open
**When:** the user presses `ctrl+r`
**Then:** the list visibly re-enters a loading state and then redraws using fresh AWS data

**AWS comparison:**
aws ec2 describe-instances
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Help from the EC2 list restores the list after close
**Given:** the EC2 list is open
**When:** the user presses `?` and then closes help
**Then:** the EC2 list reappears in the same stack position instead of returning to the main menu

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Escape returns from the EC2 list to the main menu
**Given:** the EC2 list is open in normal mode with no active filter input
**When:** the user presses `esc`
**Then:** the app returns to the main menu

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

## EC2 YAML View

### Story: EC2 YAML view shows raw structure rather than a curated field list
**Given:** the user opens YAML for an EC2 instance
**When:** the YAML view renders
**Then:** the user sees raw nested resource data such as placement, block device mappings, security groups, and tags instead of the shorter curated detail field list

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-... --output yaml
Expected fields visible: InstanceId, State, Placement, SecurityGroups, BlockDeviceMappings, Tags

### Story: YAML view stays inside the standard frame
**Given:** the user opens EC2 YAML
**When:** the screen renders
**Then:** the YAML appears inside the same framed content area used by lists and detail views

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-... --output yaml
Expected fields visible: InstanceId, State, Placement, SecurityGroups, BlockDeviceMappings, Tags

### Story: Escape from YAML returns to the view that launched it
**Given:** the user opened YAML from the EC2 list or from EC2 detail
**When:** the user presses `esc`
**Then:** the user returns to that immediate previous view rather than all the way back to the main menu

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-... --output yaml
Expected fields visible: InstanceId, State, Placement, SecurityGroups, BlockDeviceMappings, Tags

## EC2 Detail

### Story: Wide terminals show EC2 detail and related resources side by side
**Given:** the terminal is at or above the two-column threshold
**When:** the user opens EC2 detail
**Then:** the screen shows detail fields on the left and related resource types on the right, separated by a visible divider

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Focus indicator changes with the active detail column
**Given:** the EC2 detail view is showing both columns
**When:** the user switches focus between the left and right sides
**Then:** the separator visually reflects which side currently has focus

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: EC2 detail shows the configured curated field set instead of raw YAML
**Given:** the user opens EC2 detail from the list
**When:** the detail view renders
**Then:** the screen shows the configured curated EC2 fields in the configured order rather than all raw API keys

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Section headers and nested fields are visibly structured
**Given:** the EC2 detail contains nested objects such as placement, metadata options, and security groups
**When:** the detail view renders
**Then:** section headers are visually distinct and child values are visibly indented beneath them

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: Placement, IamInstanceProfile, SecurityGroups, MetadataOptions, Tags

### Story: Left-column cursor moves row by row across both plain and navigable fields
**Given:** the left side of EC2 detail is focused
**When:** the user presses `j`, `k`, `down`, or `up`
**Then:** the cursor moves one detail row at a time through the visible field list

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Left-column jump keys go to the first and last detail rows
**Given:** the left side of EC2 detail is focused
**When:** the user presses `g` or `G`
**Then:** the cursor jumps to the first or last available detail row

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Detail paging works on the focused column
**Given:** EC2 detail contains more rows than the visible height
**When:** the user presses `pgup`, `pgdn`, `ctrl+u`, or `ctrl+d`
**Then:** the focused detail area scrolls by page-sized increments while keeping the cursor in a visible range

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Long detail values wrap instead of forcing horizontal detail scrolling
**Given:** a detail field contains a long value
**When:** the user views that field in EC2 detail
**Then:** the value wraps across visual lines inside the same field row instead of creating horizontal detail scrolling

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Updated detail flow no longer depends on a word-wrap toggle
**Given:** the user is reading the updated EC2 detail view
**When:** the user compares the screen to older behavior expectations
**Then:** detail content is already wrapped for readability and the updated interaction model centers on row focus and column focus rather than toggling wrap on and off

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Pressing `w` does not introduce a separate wrap mode in the updated detail screen
**Given:** the user is reading the updated EC2 detail view with long wrapped values
**When:** the user presses `w`
**Then:** the screen remains in the same wrapped row-based detail mode and does not switch into a separate on-screen wrap state

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Navigable EC2 field values are visibly different from plain values
**Given:** the EC2 detail includes fields that point to other resources
**When:** the detail view renders
**Then:** navigable values such as VPC, subnet, security group, image, EBS volume, and ENI identifiers are visually distinct from ordinary non-navigable values

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: ImageId, VpcId, SubnetId, SecurityGroups, BlockDeviceMappings, NetworkInterfaces

### Story: Selected navigable fields use row selection instead of underline
**Given:** the cursor lands on a navigable field in EC2 detail
**When:** the row becomes selected
**Then:** the full-row selection styling takes over and the unselected underline cue is no longer the main visual signal

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: ImageId, VpcId, SubnetId, SecurityGroups, BlockDeviceMappings, NetworkInterfaces

### Story: Enter on a plain non-navigable detail row does not leave the view
**Given:** the cursor is on a non-navigable EC2 detail field
**When:** the user presses `enter`
**Then:** the app remains on the current EC2 detail screen because that row does not point anywhere

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: State, InstanceType, PrivateIpAddress, LaunchTime, Architecture, Platform

### Story: Enter on VpcId opens the VPC detail screen
**Given:** the cursor is on the `VpcId` value in EC2 detail
**When:** the user presses `enter`
**Then:** the VPC detail screen opens for that VPC

**AWS comparison:**
aws ec2 describe-vpcs --vpc-ids vpc-...
Expected fields visible: VpcId, CidrBlock, State, IsDefault, InstanceTenancy, DhcpOptionsId, OwnerId, CidrBlockAssociationSet, Ipv6CidrBlockAssociationSet, Tags

### Story: Enter on SubnetId opens the subnet detail screen
**Given:** the cursor is on the `SubnetId` value in EC2 detail
**When:** the user presses `enter`
**Then:** the subnet detail screen opens for that subnet

**AWS comparison:**
aws ec2 describe-subnets --subnet-ids subnet-...
Expected fields visible: SubnetId, VpcId, CidrBlock, AvailabilityZone, AvailabilityZoneId, State, AvailableIpAddressCount, MapPublicIpOnLaunch, DefaultForAz, SubnetArn, OwnerId, Tags

### Story: Enter on a security group ID opens the security group detail screen
**Given:** the cursor is on a `SecurityGroups.GroupId` value in EC2 detail
**When:** the user presses `enter`
**Then:** the security group detail screen opens for that security group

**AWS comparison:**
aws ec2 describe-security-groups --group-ids sg-...
Expected fields visible: GroupId, GroupName, VpcId, Description, OwnerId, SecurityGroupArn, IpPermissions, IpPermissionsEgress, Tags

### Story: Enter on ImageId opens the AMI detail screen
**Given:** the cursor is on the `ImageId` value in EC2 detail
**When:** the user presses `enter`
**Then:** the AMI detail screen opens for that image

**AWS comparison:**
aws ec2 describe-images --image-ids ami-...
Expected fields visible: ImageId, Name, State, Description, Architecture, PlatformDetails, RootDeviceType, VirtualizationType, EnaSupport, BootMode, CreationDate, DeprecationTime, Public, OwnerId, ImageLocation, BlockDeviceMappings, Tags

### Story: Enter on an attached EBS volume ID opens the EBS volume detail screen
**Given:** the cursor is on a `BlockDeviceMappings.Ebs.VolumeId` value in EC2 detail
**When:** the user presses `enter`
**Then:** the EBS volume detail screen opens for that volume

**AWS comparison:**
aws ec2 describe-volumes --volume-ids vol-...
Expected fields visible: VolumeId, State, Size, VolumeType, Iops, Throughput, Encrypted, KmsKeyId, MultiAttachEnabled, AvailabilityZone, CreateTime, Attachments, Tags

### Story: Enter on a network interface ID opens the ENI detail screen
**Given:** the cursor is on a `NetworkInterfaces.NetworkInterfaceId` value in EC2 detail
**When:** the user presses `enter`
**Then:** the ENI detail screen opens for that interface

**AWS comparison:**
aws ec2 describe-network-interfaces --network-interface-ids eni-...
Expected fields visible: NetworkInterfaceId, Status, InterfaceType, VpcId, SubnetId, AvailabilityZone, PrivateIpAddress, PrivateDnsName, MacAddress, Description, OwnerId, RequesterId, RequesterManaged, SourceDestCheck, Groups, Attachment, Association, TagSet

### Story: Left-column search uses the header and highlights matching detail rows
**Given:** the left side of EC2 detail is focused
**When:** the user presses `/`, types a query, and confirms it
**Then:** the header shows the search input, matching detail rows are highlighted, and the cursor jumps to the first match

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Search match indicator is visible in the left detail column
**Given:** left-column search has found one or more matches
**When:** the user looks near the bottom of the left detail area
**Then:** the screen shows a visible match index such as current-over-total matches

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Search next and previous keys only apply to left-column search
**Given:** left-column search results are active
**When:** the user presses `n` or `N`
**Then:** the cursor moves to the next or previous matching detail row

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Search highlighting outranks the navigable underline cue
**Given:** a navigable field also matches the active left-column search
**When:** the match is visible on screen
**Then:** the match highlight becomes the more visible cue while the field still behaves as a navigable target on `enter`

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: ImageId, VpcId, SubnetId, SecurityGroups, BlockDeviceMappings, NetworkInterfaces

### Story: Left-column search persists internally when focus moves away
**Given:** the user has active search results in the left detail column
**When:** the user switches focus to the right column
**Then:** the matches stay part of the left column state, even though `n` and `N` no longer act until focus returns

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Escape cancels detail search input before clearing search results or leaving the view
**Given:** the user is actively typing into left-column search
**When:** the user presses `esc`
**Then:** the search input is canceled first, rather than immediately leaving EC2 detail

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Escape clears existing search results before popping EC2 detail
**Given:** left-column search results are active but the user is no longer typing
**When:** the user presses `esc`
**Then:** the highlights clear first, and only a later `esc` leaves the detail view

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Copy from the left detail column copies the active field value
**Given:** the left side of EC2 detail is focused on a field row
**When:** the user presses `c`
**Then:** the field stays selected and the header shows visible success feedback for the copy action

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: YAML shortcut works from EC2 detail regardless of column focus
**Given:** EC2 detail is open
**When:** the user presses `y` while focused on either the left or right side
**Then:** the EC2 YAML view opens for the current resource

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-... --output yaml
Expected fields visible: InstanceId, State, Placement, SecurityGroups, BlockDeviceMappings, Tags

### Story: Detail help reflects the two-column interaction model
**Given:** the updated EC2 detail view is open
**When:** the user presses `?`
**Then:** help includes the two-column detail interactions such as `Tab`, `h`, `l`, `r`, `/`, `n`, `N`, and refresh

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

## EC2 Related Types

### Story: Related types are visible by default when EC2 detail opens
**Given:** the user opens EC2 detail on a terminal wide enough for the full related view
**When:** the detail screen first appears
**Then:** the related resources column is already visible without requiring a separate open action

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Related rows start dim and become active as availability is discovered
**Given:** EC2 detail has just opened
**When:** the user watches the related column during background checking
**Then:** unresolved rows start dim, and rows that are discovered as available become normal selectable entries

**AWS comparison:**
aws elbv2 describe-target-groups
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Available related rows may show counts when the count is known
**Given:** EC2 detail has related resource types whose count can be determined
**When:** those rows become available
**Then:** the related row label includes a visible `(N)` count beside the display name

**AWS comparison:**
aws autoscaling describe-auto-scaling-instances --instance-ids i-...
Expected fields visible: ASG Name, Min, Max, Desired, Instances, Status

### Story: Available related rows without a cheap count remain selectable without a number
**Given:** EC2 detail has a related type that can be confirmed without cheaply showing a count
**When:** that row becomes available
**Then:** the row becomes selectable but displays no count beside its name

**AWS comparison:**
aws cloudwatch describe-alarms
Expected fields visible: Alarm Name, State, Metric, Namespace, Threshold

### Story: Unavailable related rows remain dim and cannot be selected
**Given:** a related type does not exist for the current EC2 instance
**When:** the related column finishes checking that type
**Then:** that row remains dim and the cursor skips over it during navigation

**AWS comparison:**
aws ec2 describe-addresses --filters Name=instance-id,Values=i-...
Expected fields visible: AllocationId, PublicIp, AssociationId, InstanceId, Domain, NetworkBorderGroup, SubnetId, PrivateIpAddress, NetworkInterfaceId, Tags

### Story: Background check failures are silent on screen
**Given:** a related check is throttled, times out, or otherwise fails
**When:** the user watches the related column
**Then:** the row stays dim and no disruptive popup or modal error interrupts the current detail view

**AWS comparison:**
aws logs describe-log-groups
Expected fields visible: Log Group Name, StoredBytes, RetentionInDays, MetricFilterCount, CreationTime

### Story: Right-column cursor only lands on active rows
**Given:** the right side is focused and the related list contains a mix of available and dim rows
**When:** the user presses `j`, `k`, `down`, or `up`
**Then:** the cursor lands only on selectable available rows

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Right-column jump keys land on the first and last active related row
**Given:** the right side is focused
**When:** the user presses `g` or `G`
**Then:** the cursor jumps to the first or last selectable related row

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Tab switches focus between detail and related columns
**Given:** EC2 detail is visible with both columns shown
**When:** the user presses `Tab`
**Then:** focus moves to the other column instead of leaving the screen

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Shift-Tab also flips focus between the two visible columns
**Given:** EC2 detail is visible with both columns shown
**When:** the user presses `Shift+Tab`
**Then:** focus moves to the other column in the same two-column detail screen

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: H and L switch focus instead of horizontally scrolling the detail view
**Given:** EC2 detail is visible with both columns shown
**When:** the user presses `h` or `l`
**Then:** focus moves left or right between the detail and related columns instead of horizontally scrolling the detail content

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Right-column filter narrows visible related type names live
**Given:** the right column is focused
**When:** the user presses `/` and types a related-type filter
**Then:** the visible related list narrows as the user types instead of waiting for a separate search submit action

**AWS comparison:**
aws elbv2 describe-target-groups
Expected fields visible: Target Group, Port, Protocol, VPC ID, Target Type, Health Check

### Story: Right-column filtering can still show matching dim rows
**Given:** the right column has a filter active
**When:** a row name matches the filter but the row is still unavailable
**Then:** the filtered list can still display that matching row in its dim state

**AWS comparison:**
aws route53 list-resource-record-sets --hosted-zone-id Z...
Expected fields visible: Name, Type, TTL, Values

### Story: Right-column filter state survives a focus switch
**Given:** the right-column related filter is active
**When:** the user switches focus to the left column and then back
**Then:** the filtered related list is still narrowed when focus returns

**AWS comparison:**
aws logs describe-log-groups
Expected fields visible: LogGroupName, LogGroupArn, LogGroupClass, StoredBytes, RetentionInDays, MetricFilterCount, DeletionProtectionEnabled, CreationTime, KmsKeyId, DataProtectionStatus

### Story: Escape clears right-column filtering before leaving EC2 detail
**Given:** the related column has an active filter
**When:** the user presses `esc`
**Then:** the filter clears first and the full related-type list returns, rather than immediately popping the detail view

**AWS comparison:**
aws logs describe-log-groups
Expected fields visible: LogGroupName, LogGroupArn, LogGroupClass, StoredBytes, RetentionInDays, MetricFilterCount, DeletionProtectionEnabled, CreationTime, KmsKeyId, DataProtectionStatus

### Story: Copy from the right column copies the related type label
**Given:** the right side of EC2 detail is focused on an available related row
**When:** the user presses `c`
**Then:** the current row stays selected and the user receives visible copy feedback in the header

**AWS comparison:**
aws cloudwatch describe-alarms
Expected fields visible: Alarm Name, State, Metric, Namespace, Threshold

### Story: Related column can be hidden and the left detail uses the full width
**Given:** EC2 detail is open with the related column visible
**When:** the user presses `r`
**Then:** the related column disappears and the left detail content expands to the full detail width

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Tab has no effect while the related column is hidden
**Given:** the user has toggled the related column off in EC2 detail
**When:** the user presses `Tab`
**Then:** focus remains in the single visible left detail pane

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Toggling related back on restores the related pane
**Given:** the user has hidden the related column
**When:** the user presses `r` again
**Then:** the related pane returns without requiring the user to reopen the detail screen

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Right-column overflow shows a visible scroll indicator
**Given:** the related column contains more rows than fit vertically
**When:** the right side is focused and scrolled away from one end
**Then:** a visible top or bottom overflow indicator shows that more related rows exist off screen

**AWS comparison:**
aws cloudwatch describe-alarms
Expected fields visible: Alarm Name, State, Metric, Namespace, Threshold

### Story: Enter on a single-result related type goes directly to detail
**Given:** the selected related row represents exactly one target resource
**When:** the user presses `enter`
**Then:** the target resource opens directly in detail view instead of first opening a one-row list

**AWS comparison:**
aws ec2 describe-addresses --filters Name=instance-id,Values=i-...
Expected fields visible: AllocationId, PublicIp, AssociationId, InstanceId, Domain, NetworkBorderGroup, SubnetId, PrivateIpAddress, NetworkInterfaceId, Tags

### Story: Enter on a multi-result related type opens a result list
**Given:** the selected related row represents multiple target resources
**When:** the user presses `enter`
**Then:** the user is taken to a list view of those related resources instead of a single detail screen

**AWS comparison:**
aws ec2 describe-snapshots --filters Name=volume-id,Values=vol-...
Expected fields visible: Name, Snapshot ID, State, Volume ID, Size (GiB), Encrypted, Description, Started, Progress

### Story: CloudTrail row is always visible and sorted last
**Given:** EC2 detail is open
**When:** the related rows are displayed
**Then:** CloudTrail Events appears in the related list even if other relationships are unavailable, and it is positioned after the other related types

**AWS comparison:**
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=i-...
Expected fields visible: Event ID, Time, Event Name, User, Source, Read Only

### Story: CloudTrail row does not show an inline count
**Given:** EC2 detail is open
**When:** the user looks at the CloudTrail Events related row
**Then:** the row is visible without an inline numeric count

**AWS comparison:**
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=i-...
Expected fields visible: Event ID, Time, Event Name, User, Source, Read Only

### Story: CloudTrail enter currently gives a visible placeholder response
**Given:** the user selects the CloudTrail Events row from EC2 detail
**When:** the user presses `enter`
**Then:** the screen stays in the current EC2 detail flow and the user sees visible feedback that CloudTrail search is coming soon

**AWS comparison:**
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=i-...
Expected fields visible: Event ID, Time, Event Name, User, Source, Read Only

### Story: Refresh resets the visible related-state resolution and starts over
**Given:** EC2 detail has already resolved some related rows
**When:** the user presses `ctrl+r`
**Then:** the detail is refreshed, related rows visibly return to an unresolved dim state, and then begin resolving again

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

## EC2 Detail Stacked Layout

### Story: Medium-width terminals stack related content below detail content
**Given:** the terminal is wide enough for detail but narrower than the side-by-side threshold
**When:** the user opens EC2 detail
**Then:** the detail fields appear above and the related list appears below, separated by a visible section divider

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Focus switching still works in stacked mode
**Given:** EC2 detail is in stacked layout
**When:** the user presses `Tab`, `h`, or `l`
**Then:** focus moves between the top detail section and the lower related section

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Resize across the side-by-side threshold preserves detail state
**Given:** the user has an active EC2 detail selection, active search or filter, and a chosen focus side
**When:** the terminal is resized from stacked layout to side-by-side layout or back
**Then:** the visible cursor positions, focus target, and search or filter state are preserved

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

## Related Results Lists

### Story: Target Groups list shows target-group columns after EC2 related navigation
**Given:** the EC2 related row for target groups resolves to multiple matches
**When:** the user presses `enter`
**Then:** a target groups list opens with the configured target-group columns visible

**AWS comparison:**
aws elbv2 describe-target-groups
Expected fields visible: Target Group, Port, Protocol, VPC ID, Target Type, Health Check

### Story: Auto Scaling Groups list shows ASG summary columns after EC2 related navigation
**Given:** the EC2 related row for auto scaling groups resolves to one or more matches
**When:** the user opens that related result
**Then:** an Auto Scaling Groups list opens with the configured ASG columns visible

**AWS comparison:**
aws autoscaling describe-auto-scaling-groups
Expected fields visible: ASG Name, Min, Max, Desired, Instances, Status

### Story: CloudWatch Alarms list shows alarm summary columns after EC2 related navigation
**Given:** the EC2 related row for alarms resolves to one or more matches
**When:** the user opens that related result
**Then:** a CloudWatch Alarms list opens with the configured alarm columns visible

**AWS comparison:**
aws cloudwatch describe-alarms
Expected fields visible: Alarm Name, State, Metric, Namespace, Threshold

### Story: CloudFormation Stacks list shows stack summary columns after EC2 related navigation
**Given:** the EC2 related row for CloudFormation stacks resolves to one or more matches
**When:** the user opens that related result
**Then:** a CloudFormation Stacks list opens with the configured stack columns visible

**AWS comparison:**
aws cloudformation describe-stacks
Expected fields visible: Stack Name, Status, Created, Updated, Description

### Story: EKS Node Groups list shows node group summary columns after EC2 related navigation
**Given:** the EC2 related row for EKS node groups resolves to one or more matches
**When:** the user opens that related result
**Then:** an EKS Node Groups list opens with the configured node group columns visible

**AWS comparison:**
aws eks list-nodegroups --cluster-name <cluster>
Expected fields visible: Node Group, Cluster, Status, Instance Types, Desired

### Story: Elastic Beanstalk environments list shows environment summary columns after EC2 related navigation
**Given:** the EC2 related row for Elastic Beanstalk environments resolves to one or more matches
**When:** the user opens that related result
**Then:** an Elastic Beanstalk environments list opens with the configured environment columns visible

**AWS comparison:**
aws elasticbeanstalk describe-environments
Expected fields visible: Environment, Application, Status, Health, Version

### Story: EBS Snapshots list shows snapshot summary columns after EC2 related navigation
**Given:** the EC2 related row for EBS snapshots resolves to one or more matches
**When:** the user opens that related result
**Then:** an EBS Snapshots list opens with the configured snapshot columns visible

**AWS comparison:**
aws ec2 describe-snapshots --filters Name=volume-id,Values=vol-...
Expected fields visible: Name, Snapshot ID, State, Volume ID, Size (GiB), Encrypted, Description, Started, Progress

### Story: Elastic IP list shows EIP summary columns after EC2 related navigation
**Given:** the EC2 related row for Elastic IP resolves to one or more matches
**When:** the user opens that related result
**Then:** an Elastic IP list or direct detail view exposes the configured EIP fields for that relationship

**AWS comparison:**
aws ec2 describe-addresses --filters Name=instance-id,Values=i-...
Expected fields visible: Name, Allocation ID, Public IP, Association, Instance, Domain

### Story: CloudWatch Log Groups list shows log-group summary columns after EC2 related navigation
**Given:** the EC2 related row for CloudWatch log groups resolves to one or more matches
**When:** the user opens that related result
**Then:** a Log Groups list opens with the configured log-group columns visible

**AWS comparison:**
aws logs describe-log-groups
Expected fields visible: Log Group Name, Size, Retention, Metric Filters, Created

### Story: Route 53 records list shows record summary columns after EC2 related navigation
**Given:** the EC2 related row for Route 53 records resolves to one or more matches
**When:** the user opens that related result
**Then:** a Route 53 records list opens with the configured DNS-record columns visible

**AWS comparison:**
aws route53 list-resource-record-sets --hosted-zone-id Z...
Expected fields visible: Name, Type, TTL, Values

### Story: Related-result lists reuse standard list interactions
**Given:** the user opens any related-result list from EC2 detail
**When:** the user navigates with list keys such as `j`, `k`, `g`, `G`, paging keys, `/`, `?`, `c`, `y`, `d`, or `enter`
**Then:** the related-result list behaves like a normal resource list for that target resource type

**AWS comparison:**
aws elbv2 describe-target-groups
Expected fields visible: Target Group, Port, Protocol, VPC ID, Target Type, Health Check

### Story: Related-result list fetch failure is shown as visible screen feedback
**Given:** the user opens a related result and the fetch fails
**When:** the failure is returned
**Then:** the user sees an error in the header and remains in a stable navigable screen rather than dropping into a corrupted partial view

**AWS comparison:**
aws cloudwatch describe-alarms
Expected fields visible: Alarm Name, State, Metric, Namespace, Threshold

## Destination Detail Views

### Story: VPC detail shows the configured VPC detail fields
**Given:** the user navigates from EC2 detail into a VPC
**When:** the VPC detail screen opens
**Then:** the user sees the configured VPC detail fields for that target resource

**AWS comparison:**
aws ec2 describe-vpcs --vpc-ids vpc-...
Expected fields visible: VpcId, CidrBlock, State, IsDefault, InstanceTenancy, DhcpOptionsId, OwnerId, CidrBlockAssociationSet, Ipv6CidrBlockAssociationSet, Tags

### Story: Subnet detail shows the configured subnet detail fields
**Given:** the user navigates from EC2 detail into a subnet
**When:** the subnet detail screen opens
**Then:** the user sees the configured subnet detail fields for that target resource

**AWS comparison:**
aws ec2 describe-subnets --subnet-ids subnet-...
Expected fields visible: SubnetId, VpcId, CidrBlock, AvailabilityZone, AvailabilityZoneId, State, AvailableIpAddressCount, MapPublicIpOnLaunch, DefaultForAz, SubnetArn, OwnerId, Tags

### Story: Security group detail shows the configured security-group detail fields
**Given:** the user navigates from EC2 detail into a security group
**When:** the security group detail screen opens
**Then:** the user sees the configured security-group detail fields for that target resource

**AWS comparison:**
aws ec2 describe-security-groups --group-ids sg-...
Expected fields visible: GroupId, GroupName, VpcId, Description, OwnerId, SecurityGroupArn, IpPermissions, IpPermissionsEgress, Tags

### Story: AMI detail shows the configured image detail fields
**Given:** the user navigates from EC2 detail into an AMI
**When:** the image detail screen opens
**Then:** the user sees the configured AMI detail fields for that target resource

**AWS comparison:**
aws ec2 describe-images --image-ids ami-...
Expected fields visible: ImageId, Name, State, Description, Architecture, PlatformDetails, RootDeviceType, VirtualizationType, EnaSupport, BootMode, CreationDate, DeprecationTime, Public, OwnerId, ImageLocation, BlockDeviceMappings, Tags

### Story: EBS volume detail shows the configured volume detail fields
**Given:** the user navigates from EC2 detail into an attached EBS volume
**When:** the volume detail screen opens
**Then:** the user sees the configured EBS detail fields for that target resource

**AWS comparison:**
aws ec2 describe-volumes --volume-ids vol-...
Expected fields visible: VolumeId, State, Size, VolumeType, Iops, Throughput, Encrypted, KmsKeyId, MultiAttachEnabled, AvailabilityZone, CreateTime, Attachments, Tags

### Story: ENI detail shows the configured network-interface detail fields
**Given:** the user navigates from EC2 detail into a network interface
**When:** the ENI detail screen opens
**Then:** the user sees the configured ENI detail fields for that target resource

**AWS comparison:**
aws ec2 describe-network-interfaces --network-interface-ids eni-...
Expected fields visible: NetworkInterfaceId, Status, InterfaceType, VpcId, SubnetId, AvailabilityZone, PrivateIpAddress, PrivateDnsName, MacAddress, Description, OwnerId, RequesterId, RequesterManaged, SourceDestCheck, Groups, Attachment, Association, TagSet

### Story: Auto Scaling Group detail shows the configured ASG detail fields
**Given:** the user opens an ASG from the EC2 related flow
**When:** the ASG detail screen opens
**Then:** the user sees the configured ASG detail fields for that target resource

**AWS comparison:**
aws autoscaling describe-auto-scaling-groups --auto-scaling-group-names <name>
Expected fields visible: AutoScalingGroupName, AutoScalingGroupARN, MinSize, MaxSize, DesiredCapacity, AvailabilityZones, LaunchConfigurationName, HealthCheckType, HealthCheckGracePeriod, TargetGroupARNs, LoadBalancerNames, SuspendedProcesses, TerminationPolicies, VPCZoneIdentifier, CreatedTime, Tags

### Story: Alarm detail shows the configured alarm detail fields
**Given:** the user opens a CloudWatch alarm from the EC2 related flow
**When:** the alarm detail screen opens
**Then:** the user sees the configured alarm detail fields for that target resource

**AWS comparison:**
aws cloudwatch describe-alarms --alarm-names <name>
Expected fields visible: AlarmName, AlarmArn, StateValue, StateReason, StateUpdatedTimestamp, StateTransitionedTimestamp, MetricName, Namespace, Statistic, Period, EvaluationPeriods, DatapointsToAlarm, Threshold, ComparisonOperator, TreatMissingData, Dimensions, AlarmDescription, AlarmActions, OKActions, InsufficientDataActions, ActionsEnabled

### Story: CloudFormation stack detail shows the configured stack detail fields
**Given:** the user opens a CloudFormation stack from the EC2 related flow
**When:** the stack detail screen opens
**Then:** the user sees the configured stack detail fields for that target resource

**AWS comparison:**
aws cloudformation describe-stacks --stack-name <stack>
Expected fields visible: StackName, StackId, StackStatus, DetailedStatus, StackStatusReason, CreationTime, LastUpdatedTime, DeletionTime, Description, RoleARN, Capabilities, EnableTerminationProtection, DriftInformation, Parameters, Outputs, Tags

### Story: Node group detail shows the configured EKS node-group detail fields
**Given:** the user opens an EKS node group from the EC2 related flow
**When:** the node group detail screen opens
**Then:** the user sees the configured node-group detail fields for that target resource

**AWS comparison:**
aws eks describe-nodegroup --cluster-name <cluster> --nodegroup-name <nodegroup>
Expected fields visible: NodegroupName, ClusterName, Status, InstanceTypes, AmiType, CapacityType, DiskSize, ScalingConfig, NodeRole, NodegroupArn, ReleaseVersion, Version, Subnets, LaunchTemplate, Labels, Taints, Tags, Health, CreatedAt

### Story: Elastic Beanstalk detail shows the configured environment detail fields
**Given:** the user opens an Elastic Beanstalk environment from the EC2 related flow
**When:** the environment detail screen opens
**Then:** the user sees the configured environment detail fields for that target resource

**AWS comparison:**
aws elasticbeanstalk describe-environments --environment-names <env>
Expected fields visible: EnvironmentName, EnvironmentId, ApplicationName, Status, Health, HealthStatus, VersionLabel, SolutionStackName, PlatformArn, EndpointURL, CNAME, DateCreated, DateUpdated, EnvironmentArn

### Story: EIP detail shows the configured Elastic IP detail fields
**Given:** the user opens an Elastic IP from the EC2 related flow
**When:** the EIP detail screen opens
**Then:** the user sees the configured EIP detail fields for that target resource

**AWS comparison:**
aws ec2 describe-addresses --allocation-ids eipalloc-...
Expected fields visible: AllocationId, PublicIp, AssociationId, InstanceId, Domain, NetworkBorderGroup, SubnetId, PrivateIpAddress, NetworkInterfaceId, Tags

### Story: EBS snapshot detail shows the configured snapshot detail fields
**Given:** the user opens an EBS snapshot from the EC2 related flow
**When:** the snapshot detail screen opens
**Then:** the user sees the configured snapshot detail fields for that target resource

**AWS comparison:**
aws ec2 describe-snapshots --snapshot-ids snap-...
Expected fields visible: SnapshotId, State, VolumeId, VolumeSize, Description, Encrypted, KmsKeyId, OwnerId, Progress, StartTime, Tags

### Story: Log group detail shows the configured log-group detail fields
**Given:** the user opens a CloudWatch log group from the EC2 related flow
**When:** the log-group detail screen opens
**Then:** the user sees the configured log-group detail fields for that target resource

**AWS comparison:**
aws logs describe-log-groups --log-group-name-prefix /aws/
Expected fields visible: LogGroupName, LogGroupArn, LogGroupClass, StoredBytes, RetentionInDays, MetricFilterCount, DeletionProtectionEnabled, CreationTime, KmsKeyId, DataProtectionStatus

### Story: Route 53 record detail shows the configured record detail fields
**Given:** the user opens a Route 53 record from the EC2 related flow
**When:** the record detail screen opens
**Then:** the user sees the configured DNS-record detail fields for that target resource

**AWS comparison:**
aws route53 list-resource-record-sets --hosted-zone-id Z...
Expected fields visible: Name, Type, TTL, ResourceRecords, AliasTarget, SetIdentifier, Weight, Region, Failover, GeoLocation, HealthCheckId, MultiValueAnswer

### Story: CloudTrail event detail shows the configured event detail fields
**Given:** the user opens a CloudTrail event detail screen from a CloudTrail list
**When:** the event detail renders
**Then:** the user sees the configured CloudTrail event fields for that event

**AWS comparison:**
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=i-...
Expected fields visible: EventId, EventName, EventTime, EventSource, Username, ReadOnly, AccessKeyId, Resources, CloudTrailEvent

## CloudFormation Stack Resources

### Story: In CloudFormation stack context, uppercase R opens stack resources
**Given:** the user is on a CloudFormation stack resource-list or stack-detail flow reached from EC2 related navigation
**When:** the user presses `R`
**Then:** the app opens the stack resources child list instead of treating the key as the detail related toggle

**AWS comparison:**
aws cloudformation describe-stack-resources --stack-name <stack>
Expected fields visible: Logical ID, Physical ID, Type, Status, Drift, Updated

### Story: Lowercase r still belongs to related toggle in detail context
**Given:** the user is reading a stack detail screen in the updated related-views model
**When:** the user presses `r`
**Then:** the app treats that key as the related-pane visibility toggle for detail, not as the old stack-resources shortcut

**AWS comparison:**
aws cloudformation describe-stacks --stack-name <stack>
Expected fields visible: StackName, StackId, StackStatus, DetailedStatus, StackStatusReason, CreationTime, LastUpdatedTime, DeletionTime, Description, RoleARN, Capabilities, EnableTerminationProtection, DriftInformation, Parameters, Outputs, Tags

### Story: CloudFormation stack resources list shows the configured columns
**Given:** the user has opened stack resources from a CloudFormation stack
**When:** the child list renders
**Then:** the screen shows the configured stack-resource columns for logical ID, physical ID, type, status, drift, and update time

**AWS comparison:**
aws cloudformation describe-stack-resources --stack-name <stack>
Expected fields visible: Logical ID, Physical ID, Type, Status, Drift, Updated

## Cache, Refresh, and Background Update

### Story: Reopening the same EC2 detail in the same session can show already-known related availability immediately
**Given:** the user has already opened a particular EC2 instance detail once in the current session
**When:** the user leaves and then reopens that same instance detail
**Then:** previously resolved related availability can appear immediately instead of every row starting from scratch again

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-...
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Switching to another EC2 instance does not leak old related-state updates onto the new screen
**Given:** the user quickly moves from one EC2 detail view to another while background checks are still resolving
**When:** delayed related results arrive from the earlier instance
**Then:** the new instance detail does not visibly light up with stale related rows from the previous instance

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-aaa i-bbb
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Region or profile changes clear visible related-state assumptions
**Given:** the user has resolved related data in one AWS context
**When:** the user changes region or profile and reopens EC2 detail
**Then:** the related pane reflects the new AWS context instead of visibly reusing stale availability from the previous context

**AWS comparison:**
aws ec2 describe-instances --region <region>
Expected fields visible: InstanceId, State, InstanceType, InstanceLifecycle, ImageId, KeyName, Placement, VpcId, SubnetId, PrivateIpAddress, PrivateDnsName, PublicIpAddress, IamInstanceProfile, SecurityGroups, EbsOptimized, MetadataOptions, LaunchTime, Architecture, Platform, Tags

### Story: Refresh on a related-result list re-fetches that result view in place
**Given:** the user is on a related-result list such as snapshots, alarms, or target groups
**When:** the user presses `ctrl+r`
**Then:** that result view visibly refreshes in place instead of sending the user back to EC2 detail

**AWS comparison:**
aws ec2 describe-snapshots --filters Name=volume-id,Values=vol-...
Expected fields visible: Name, Snapshot ID, State, Volume ID, Size (GiB), Encrypted, Description, Started, Progress

### Story: Background related checking is silent and does not use a spinner per row
**Given:** the user is watching the EC2 related pane while checks resolve
**When:** rows transition from unknown to available or unavailable
**Then:** the pane updates quietly without showing a separate spinner or explicit `checking` label on each row

**AWS comparison:**
aws cloudwatch describe-alarms
Expected fields visible: Alarm Name, State, Metric, Namespace, Threshold

## Resize and Minimum Terminal

### Story: Width changes reflow the current screen without changing the active view
**Given:** the user is in the EC2 related-views workflow
**When:** the terminal width changes but stays within supported ranges
**Then:** the same active screen is reflowed to the new width rather than being replaced by a different view unexpectedly

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Height changes preserve the active view while changing visible row count
**Given:** the user is in the EC2 related-views workflow
**When:** the terminal height changes but stays above the minimum
**Then:** the same screen remains active and the content area grows or shrinks with the new height

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story: Returning from an invalid resize state restores the prior working view
**Given:** the terminal was temporarily too narrow or too short and showed a resize warning
**When:** the user enlarges the terminal back into a supported size
**Then:** the original working view returns instead of forcing the user to restart navigation

**AWS comparison:**
aws ec2 describe-instances --max-items 20
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

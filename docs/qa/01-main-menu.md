# QA User Stories: Main Menu View & Header/Frame Chrome

Scope: the initial screen shown when a9s launches -- the resource-type picker, the header bar, and the surrounding border frame. All stories treat a9s as a black box.

---

## A. Resource Type Listing

### Story: All seven resource types are displayed on launch
**Given:** The user launches a9s in a terminal at least 80 columns wide and 7 lines tall.
**When:** The application finishes loading.
**Then:** The main menu lists exactly seven rows, one per resource type:
1. EC2 Instances
2. S3 Buckets
3. RDS Instances
4. ElastiCache Redis
5. DocumentDB Clusters
6. EKS Clusters
7. Secrets Manager

### Story: Each resource type shows its shortname alias
**Given:** The main menu is displayed.
**When:** The user looks at any row.
**Then:** The row shows the display name on the left and a dimmed shortname alias on the right:
- EC2 Instances -- `:ec2`
- S3 Buckets -- `:s3`
- RDS Instances -- `:rds`
- ElastiCache Redis -- `:redis`
- DocumentDB Clusters -- `:docdb`
- EKS Clusters -- `:eks`
- Secrets Manager -- `:secrets`

### Story: Shortname aliases are rendered in dimmed style
**Given:** The main menu is displayed.
**When:** The user looks at any row.
**Then:** The `:alias` text on the right side of each row appears in a visually dimmed style, clearly less prominent than the display name.

### Story: The first resource type is selected by default
**Given:** The user launches a9s.
**When:** The main menu appears.
**Then:** The first row (EC2 Instances) is highlighted as the selected row.

### Story: Selected row has blue background highlighting
**Given:** The main menu is displayed with any row selected.
**When:** The user looks at the selected row.
**Then:** The selected row is rendered with a blue background (`#7aa2f7`) and dark foreground (`#1a1b26`), bold text, spanning the full row width.

### Story: Non-selected rows use normal styling
**Given:** The main menu is displayed.
**When:** The user looks at any row that is not selected.
**Then:** That row uses the normal foreground color (`#c0caf5`) with no special background.

---

## B. Navigation

### Story: Move cursor down with j
**Given:** The main menu is displayed and the first row is selected.
**When:** The user presses `j`.
**Then:** The selection moves to the second row (S3 Buckets).

### Story: Move cursor down with down arrow
**Given:** The main menu is displayed and the first row is selected.
**When:** The user presses the down-arrow key.
**Then:** The selection moves to the second row (S3 Buckets).

### Story: Move cursor up with k
**Given:** The main menu is displayed and the second row is selected.
**When:** The user presses `k`.
**Then:** The selection moves to the first row (EC2 Instances).

### Story: Move cursor up with up arrow
**Given:** The main menu is displayed and the second row is selected.
**When:** The user presses the up-arrow key.
**Then:** The selection moves to the first row (EC2 Instances).

### Story: Cursor wraps from bottom to top
**Given:** The main menu is displayed and the last row (Secrets Manager) is selected.
**When:** The user presses `j` or the down-arrow key.
**Then:** The selection wraps around to the first row (EC2 Instances).

### Story: Cursor wraps from top to bottom
**Given:** The main menu is displayed and the first row (EC2 Instances) is selected.
**When:** The user presses `k` or the up-arrow key.
**Then:** The selection wraps around to the last row (Secrets Manager).

### Story: Jump to top with g
**Given:** The main menu is displayed and the fifth row is selected.
**When:** The user presses `g`.
**Then:** The selection jumps to the first row (EC2 Instances).

### Story: Jump to bottom with G
**Given:** The main menu is displayed and the first row is selected.
**When:** The user presses `G` (shift+g).
**Then:** The selection jumps to the last row (Secrets Manager).

### Story: g on first row is a no-op
**Given:** The main menu is displayed and the first row is already selected.
**When:** The user presses `g`.
**Then:** The selection remains on the first row; nothing changes.

### Story: G on last row is a no-op
**Given:** The main menu is displayed and the last row is already selected.
**When:** The user presses `G`.
**Then:** The selection remains on the last row; nothing changes.

### Story: Select resource type with Enter
**Given:** The main menu is displayed and the user has navigated to "RDS Instances" (row 3).
**When:** The user presses `Enter`.
**Then:** The application navigates to the RDS Instances resource list view.

### Story: Enter on each resource type opens the correct list
**Given:** The main menu is displayed.
**When:** The user selects each resource type one at a time and presses `Enter`.
**Then:** Each resource type opens its corresponding resource list view:
- EC2 Instances opens the EC2 instance list
- S3 Buckets opens the S3 bucket list
- RDS Instances opens the RDS instance list
- ElastiCache Redis opens the ElastiCache Redis cluster list
- DocumentDB Clusters opens the DocumentDB cluster list
- EKS Clusters opens the EKS cluster list
- Secrets Manager opens the Secrets Manager list

---

## C. Filter Mode (/)

### Story: Pressing / enters filter mode
**Given:** The main menu is displayed in normal mode.
**When:** The user presses `/`.
**Then:** The header right side changes from "? for help" to `/` followed by a cursor, displayed in amber/yellow bold (`#e0af68`).

### Story: Typing in filter mode narrows the resource list
**Given:** The main menu is in filter mode.
**When:** The user types "ec".
**Then:** Only resource types whose display name contains "ec" (case-insensitive) remain visible. "EC2 Instances" and "ElastiCache Redis" and "Secrets Manager" should match. Rows that do not match are hidden.

### Story: Filter is case-insensitive
**Given:** The main menu is in filter mode.
**When:** The user types "EKS".
**Then:** The "EKS Clusters" row is visible, matching despite the case difference.

### Story: Filter text appears in header
**Given:** The main menu is in filter mode and the user has typed "rds".
**When:** The user looks at the header bar.
**Then:** The right side of the header shows `/rds` followed by a cursor character, rendered in amber bold.

### Story: Frame title updates to show matched/total count
**Given:** The main menu is in filter mode and the user has typed "s3".
**When:** The user looks at the frame title.
**Then:** The frame title shows the filtered count format, e.g. `resource-types(1/7)`, reflecting 1 match out of 7 total.

### Story: Backspace removes the last character in filter
**Given:** The main menu is in filter mode and the user has typed "ec2".
**When:** The user presses `Backspace`.
**Then:** The filter text becomes "ec" and the list updates to show all rows matching "ec".

### Story: Esc clears filter and restores all rows
**Given:** The main menu is in filter mode with text "redis" and only one row visible.
**When:** The user presses `Esc`.
**Then:** The filter is cleared, all seven resource types reappear, and the header right side returns to "? for help".

### Story: Enter confirms filter and exits filter mode
**Given:** The main menu is in filter mode with text "doc" showing one result.
**When:** The user presses `Enter`.
**Then:** The filter remains applied (only matching rows shown), but filter input mode is exited. The header right side no longer shows the filter input cursor.

### Story: Filter with no matches shows empty list
**Given:** The main menu is in filter mode.
**When:** The user types "zzz" (a string matching none of the resource types).
**Then:** No resource type rows are shown. The frame title shows `resource-types(0/7)`.

### Story: Selection resets to first visible row after filter
**Given:** The main menu has row 5 selected and the user enters filter mode.
**When:** The user types "s3" which shows only one row.
**Then:** The visible row (S3 Buckets) becomes the selected row.

---

## D. Command Mode (:)

### Story: Pressing : enters command mode
**Given:** The main menu is displayed in normal mode.
**When:** The user presses `:`.
**Then:** The header right side changes from "? for help" to `:` followed by a cursor, displayed in amber/yellow bold (`#e0af68`).

### Story: Navigate to EC2 with :ec2
**Given:** The main menu is in command mode.
**When:** The user types "ec2" and presses `Enter`.
**Then:** The application navigates directly to the EC2 Instances resource list view.

### Story: Navigate to S3 with :s3
**Given:** The main menu is in command mode.
**When:** The user types "s3" and presses `Enter`.
**Then:** The application navigates directly to the S3 Buckets resource list view.

### Story: Navigate to RDS with :rds
**Given:** The main menu is in command mode.
**When:** The user types "rds" and presses `Enter`.
**Then:** The application navigates directly to the RDS Instances resource list view.

### Story: Navigate to Redis with :redis
**Given:** The main menu is in command mode.
**When:** The user types "redis" and presses `Enter`.
**Then:** The application navigates directly to the ElastiCache Redis resource list view.

### Story: Navigate to DocumentDB with :docdb
**Given:** The main menu is in command mode.
**When:** The user types "docdb" and presses `Enter`.
**Then:** The application navigates directly to the DocumentDB Clusters resource list view.

### Story: Navigate to EKS with :eks
**Given:** The main menu is in command mode.
**When:** The user types "eks" and presses `Enter`.
**Then:** The application navigates directly to the EKS Clusters resource list view.

### Story: Navigate to Secrets Manager with :secrets
**Given:** The main menu is in command mode.
**When:** The user types "secrets" and presses `Enter`.
**Then:** The application navigates directly to the Secrets Manager resource list view.

### Story: Quit with :q
**Given:** The main menu is in command mode.
**When:** The user types "q" and presses `Enter`.
**Then:** The application exits cleanly.

### Story: Quit with :quit
**Given:** The main menu is in command mode.
**When:** The user types "quit" and presses `Enter`.
**Then:** The application exits cleanly.

### Story: Open profile selector with :ctx
**Given:** The main menu is in command mode.
**When:** The user types "ctx" and presses `Enter`.
**Then:** The application navigates to the AWS profile selector view.

### Story: Open region selector with :region
**Given:** The main menu is in command mode.
**When:** The user types "region" and presses `Enter`.
**Then:** The application navigates to the AWS region selector view.

### Story: Unknown command shows error flash
**Given:** The main menu is in command mode.
**When:** The user types "foobar" and presses `Enter`.
**Then:** An error flash message appears in the header right side (red text, `#f7768e`), indicating the command is not recognized. The error auto-clears after approximately 2 seconds.

### Story: Esc cancels command mode
**Given:** The main menu is in command mode with partial text ":ec".
**When:** The user presses `Esc`.
**Then:** Command mode is exited, the typed text is discarded, and the header right side returns to "? for help".

### Story: Command text appears in header while typing
**Given:** The main menu is in command mode.
**When:** The user types "red".
**Then:** The header right side shows `:red` followed by a cursor, in amber/yellow bold.

---

## E. Help Overlay (?)

### Story: Pressing ? opens the help screen
**Given:** The main menu is displayed in normal mode.
**When:** The user presses `?`.
**Then:** The help screen replaces the frame content. The frame title shows "Help" centered in the top border.

### Story: Help screen shows key binding categories
**Given:** The help screen is open.
**When:** The user reads the screen.
**Then:** The help screen displays four columns of key bindings: RESOURCE, GENERAL, NAVIGATION, and HOTKEYS, with category headers in orange/yellow bold uppercase.

### Story: Help screen shows navigation keys
**Given:** The help screen is open.
**When:** The user looks at the NAVIGATION column.
**Then:** It lists at minimum: `j` (Down), `k` (Up), `g` (Top), `G` (Bottom), `h/l` (Cols), `Enter` (Open).

### Story: Help screen shows general keys
**Given:** The help screen is open.
**When:** The user looks at the GENERAL column.
**Then:** It lists at minimum: `ctrl-r` (Refresh), `q` (Quit), `:` (Command), `/` (Filter).

### Story: Any key closes the help screen
**Given:** The help screen is open over the main menu.
**When:** The user presses any key (e.g., `a`, `Space`, `Enter`).
**Then:** The help screen closes and the main menu content reappears.

### Story: Esc closes the help screen
**Given:** The help screen is open over the main menu.
**When:** The user presses `Esc`.
**Then:** The help screen closes and the main menu content reappears.

### Story: Help key hints use green color
**Given:** The help screen is open.
**When:** The user looks at the key bindings.
**Then:** Key names (e.g., `<j>`, `<esc>`, `<q>`) are rendered in green bold (`#9ece6a`).

### Story: Help descriptions use plain text color
**Given:** The help screen is open.
**When:** The user looks at the key binding descriptions.
**Then:** Descriptions (e.g., "Down", "Quit", "Back") are rendered in plain white (`#c0caf5`).

### Story: Help screen shows "Press any key to close" hint
**Given:** The help screen is open.
**When:** The user looks at the bottom of the help content.
**Then:** A dimmed hint reading "Press any key to close" is visible.

---

## F. Quit

### Story: q quits from the main menu
**Given:** The main menu is displayed in normal mode (no filter, no command mode).
**When:** The user presses `q`.
**Then:** The application exits cleanly, returning control to the shell.

### Story: ctrl+c force quits from the main menu
**Given:** The main menu is displayed.
**When:** The user presses `ctrl+c`.
**Then:** The application exits immediately, returning control to the shell.

### Story: ctrl+c force quits from any state
**Given:** The main menu is in filter mode, command mode, or help screen.
**When:** The user presses `ctrl+c`.
**Then:** The application exits immediately regardless of the current mode.

### Story: q does not quit when filter mode is active
**Given:** The main menu is in filter mode (/ has been pressed).
**When:** The user types `q`.
**Then:** The letter "q" is appended to the filter text; the application does not quit.

### Story: q does not quit when command mode is active
**Given:** The main menu is in command mode (: has been pressed).
**When:** The user types `q`.
**Then:** The letter "q" is appended to the command text; the application does not quit (unless Enter is pressed to execute `:q`).

---

## G. Header Bar

### Story: Header left side shows app name with accent styling
**Given:** The application has launched.
**When:** The user looks at the header bar.
**Then:** The left side begins with "a9s" rendered in accent blue bold (`#7aa2f7`).

### Story: Header left side shows version in dimmed text
**Given:** The application has launched.
**When:** The user looks at the header bar.
**Then:** Immediately after "a9s", the version string " v1.0.2" appears in dimmed style (`#565f89`).

### Story: Header left side shows profile and region
**Given:** The application has launched with AWS profile "prod" and region "us-east-1".
**When:** The user looks at the header bar.
**Then:** After the version, the text "  prod:us-east-1" appears in bold (`#c0caf5`), with two spaces separating it from the version.

### Story: Header right side shows "? for help" in normal mode
**Given:** The main menu is displayed in normal mode (no filter, no command, no flash).
**When:** The user looks at the right side of the header bar.
**Then:** The text "? for help" is displayed in dimmed style (`#565f89`), right-aligned.

### Story: Header right side shows filter text when filter is active
**Given:** The main menu is in filter mode and the user has typed "eks".
**When:** The user looks at the right side of the header bar.
**Then:** The text "/eks" followed by a cursor is displayed in amber/yellow bold (`#e0af68`), replacing the "? for help" hint.

### Story: Header right side shows command text when command is active
**Given:** The main menu is in command mode and the user has typed "s3".
**When:** The user looks at the right side of the header bar.
**Then:** The text ":s3" followed by a cursor is displayed in amber/yellow bold (`#e0af68`), replacing the "? for help" hint.

### Story: Header right side shows success flash on action
**Given:** The user performs an action that triggers a success flash (e.g., copy).
**When:** The flash message is active.
**Then:** The right side of the header shows the success text (e.g., "Copied!") in green bold (`#9ece6a`), replacing the "? for help" hint.

### Story: Success flash auto-clears after approximately 2 seconds
**Given:** A success flash message is displayed in the header.
**When:** Approximately 2 seconds elapse.
**Then:** The flash message disappears and the header right side returns to "? for help".

### Story: Header right side shows error flash on failure
**Given:** The user performs an action that results in an error (e.g., unknown command).
**When:** The error flash is active.
**Then:** The right side of the header shows the error text (e.g., "Error: unknown command") in red bold (`#f7768e`).

### Story: Error flash auto-clears after approximately 2 seconds
**Given:** An error flash message is displayed in the header.
**When:** Approximately 2 seconds elapse.
**Then:** The flash message disappears and the header right side returns to "? for help".

### Story: Header occupies exactly one line
**Given:** The application is running.
**When:** The user inspects the layout.
**Then:** The header bar takes exactly one line of terminal space, with no border or separator below it.

### Story: Header spans full terminal width
**Given:** The application is running in a terminal of width W.
**When:** The user looks at the header bar.
**Then:** The header spans the full width of the terminal, with the left content left-aligned and the right content right-aligned.

---

## H. Frame / Border

### Story: Frame has a single-line border on all four sides
**Given:** The main menu is displayed.
**When:** The user looks at the frame.
**Then:** The frame is drawn with single-line Unicode box-drawing characters: top-left corner, top-right corner, bottom-left corner, bottom-right corner, horizontal lines, and vertical lines on both sides.

### Story: Frame title shows "resource-types(7)" centered in top border
**Given:** The main menu is displayed with all seven resource types visible (no filter).
**When:** The user looks at the top border of the frame.
**Then:** The text "resource-types(7)" is centered within the top border line, flanked by dashes on both sides.

### Story: Frame title dash padding is balanced
**Given:** The main menu is displayed.
**When:** The user looks at the top border.
**Then:** The dashes on the left and right of the centered title are approximately equal in length (differing by at most one character if the total is odd).

### Story: Frame fills remaining vertical space below the header
**Given:** The application is running in a terminal of height H.
**When:** The user looks at the layout.
**Then:** The header uses 1 line, the frame top border uses 1 line, the frame bottom border uses 1 line, and the frame content area uses H - 3 lines.

### Story: Frame border uses dim border color
**Given:** The main menu is displayed.
**When:** The user looks at the frame border.
**Then:** The border characters are rendered in the dim border color (`#414868`).

### Story: Frame content rows are bounded by vertical bars
**Given:** The main menu is displayed.
**When:** The user looks at any content row.
**Then:** The row is enclosed between left and right vertical bar characters at the edges of the frame.

---

## I. Terminal Size Constraints

### Story: Terminal narrower than 60 columns shows error
**Given:** The user's terminal is fewer than 60 columns wide.
**When:** The application renders.
**Then:** An error message is displayed: "Terminal too narrow. Please resize." No main menu content is shown.

### Story: Terminal shorter than 7 lines shows error
**Given:** The user's terminal is fewer than 7 lines tall.
**When:** The application renders.
**Then:** An error message is displayed: "Terminal too short. Please resize." No main menu content is shown.

### Story: Resizing terminal above minimum restores the UI
**Given:** The application is showing a "Terminal too narrow" or "Terminal too short" error.
**When:** The user resizes the terminal to at least 60 columns wide and at least 7 lines tall.
**Then:** The error disappears and the full main menu UI is rendered correctly.

### Story: Terminal at exactly 60 columns wide renders correctly
**Given:** The user's terminal is exactly 60 columns wide.
**When:** The main menu is displayed.
**Then:** The UI renders without error. Column content may be reduced but the layout is functional.

### Story: Terminal at exactly 7 lines tall renders correctly
**Given:** The user's terminal is exactly 7 lines tall.
**When:** The main menu is displayed.
**Then:** The UI renders without error. The frame content area has 4 lines available (7 - 3 overhead).

### Story: Narrow terminal (60-79 columns) omits help hint in header
**Given:** The user's terminal is between 60 and 79 columns wide.
**When:** The main menu is displayed.
**Then:** The "? for help" hint on the right side of the header may be omitted if there is not enough horizontal space, but the left side content (app name, version, profile:region) is always shown.

---

## J. Combined / Edge Case Interactions

### Story: Filter mode then help then back restores filter
**Given:** The main menu is in filter mode with text "ec".
**When:** The user presses `Esc` to clear the filter, then presses `?` to open help, then presses any key to close help.
**Then:** The main menu is restored with all seven rows visible and no filter active.

### Story: Rapid key presses navigate correctly
**Given:** The main menu is displayed with the first row selected.
**When:** The user rapidly presses `j` five times.
**Then:** The selection moves to the sixth row (EKS Clusters), with each keypress moving down exactly one row.

### Story: Multiple j presses followed by g returns to top
**Given:** The main menu is displayed.
**When:** The user presses `j` four times then presses `g`.
**Then:** The selection is on the first row (EC2 Instances).

### Story: Enter after filtering opens the correct resource
**Given:** The main menu is in filter mode, the user types "doc", and only "DocumentDB Clusters" is visible and selected.
**When:** The user presses `Enter`.
**Then:** The filter is confirmed and the application navigates to the DocumentDB Clusters resource list view.

### Story: Command mode overrides normal key bindings
**Given:** The main menu is in command mode.
**When:** The user presses `j`, `k`, `g`, `G`, `q`, or `?`.
**Then:** Those characters are treated as command input text, not as navigation or quit or help actions.

### Story: Filter mode overrides normal key bindings
**Given:** The main menu is in filter mode.
**When:** The user presses `j`, `k`, `g`, `G`, `q`, or `?`.
**Then:** Those characters are treated as filter input text, not as navigation or quit or help actions.

### Story: Esc on the main menu in normal mode is a no-op
**Given:** The main menu is displayed in normal mode (no filter, no command, no help).
**When:** The user presses `Esc`.
**Then:** Nothing happens. The main menu remains displayed. The application does not quit or navigate away.

### Story: Header transitions correctly between modes
**Given:** The main menu is displayed in normal mode.
**When:** The user presses `/` (entering filter mode), types "s3", presses `Esc` (clearing filter), presses `:` (entering command mode), types "eks", presses `Esc` (canceling command).
**Then:** The header right side transitions: "? for help" -> "/s3" -> "? for help" -> ":eks" -> "? for help". Each transition is immediate and visually correct.

### Story: Only one input mode is active at a time
**Given:** The main menu is in filter mode.
**When:** The user presses `:`.
**Then:** The colon character is added to the filter text. The application does not enter command mode while filter mode is active.

### Story: Only one input mode active -- command prevents filter
**Given:** The main menu is in command mode.
**When:** The user presses `/`.
**Then:** The slash character is added to the command text. The application does not enter filter mode while command mode is active.

### Story: Window resize during filter mode re-renders correctly
**Given:** The main menu is in filter mode with text "red" showing one filtered result.
**When:** The user resizes the terminal window.
**Then:** The UI re-renders at the new size, maintaining the active filter, the filtered results, and the header showing `/red` with cursor.

### Story: Selection persists across g and G jumps
**Given:** The main menu is displayed.
**When:** The user presses `G` to go to bottom, then `g` to go to top, then `G` again.
**Then:** The selection is on the last row (Secrets Manager). Each jump leaves the selection at the expected boundary row.

### Story: Consecutive Enter on same resource opens that view each time
**Given:** The main menu is displayed with EC2 Instances selected, and the user navigates into EC2 and back to the main menu.
**When:** The user presses `Enter` again with EC2 Instances still selected.
**Then:** The application navigates to the EC2 Instances resource list view again without error.

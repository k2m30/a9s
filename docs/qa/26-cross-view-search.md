# QA-26: Cross-View Search Component (Issue #89)

A shared, reusable in-document search component that provides `/` search with `n`/`N`
match navigation across all views that display text content. This is NOT the list
filter (QA-11) -- it searches *within* a single document and highlights matches,
rather than filtering rows in a table.

> **Dependency:** Issue #87 (Policy Document View) depends on this component.
> Stories in `25-policy-document-view.md` section H describe policy-doc-specific
> search scenarios (e.g., searching for `s3:GetObject` in a policy). This file
> covers the *generic component behavior* that applies identically across all
> view types. Section H stories should be read as specializations of the stories
> here.
>
> **Supersedes QA-11 stories 11-17 and 11-18.** Those stories state that `/` is
> ignored in detail and YAML views. Once this component is integrated, `/`
> activates in-document search in those views instead. QA-11 stories 11-17 and
> 11-18 should be amended or removed.

---

## Views That Embed This Component

| View type         | Content searched                             | Example entry path                               |
|-------------------|----------------------------------------------|--------------------------------------------------|
| Detail view       | Key/value fields (all resource types)        | Resource list > select resource > `d`            |
| YAML view         | Full resource YAML (all resource types)      | Resource list > select resource > `y`            |
| Policy Document   | Pretty-printed JSON document                 | IAM Roles > Role Policies > select policy > Enter |
| Log Events        | Log lines (Timestamp + Message)              | Log Groups > Log Streams > select stream > Enter  |
| Build Logs        | Build log lines (Timestamp + Message)        | CodeBuild Projects > Builds > select build > Enter |

---

## A. Activating Search Mode

### 26-A01: Press `/` in detail view activates search input

**Given:** the detail view is displayed for an EC2 instance (fields like Name, InstanceId, Status, etc. visible)
**When:** the user presses `/`
**Then:** the header right side changes from "? for help" to "/" (amber #e0af68, bold) with a text cursor
**And:** the detail content remains unchanged (no highlights yet)

**AWS comparison:**

```
aws ec2 describe-instances --instance-ids i-0abc123 --output yaml
# Then grep for a term -- a9s provides interactive search instead
```

Expected fields visible: all fields from views.yaml ec2 detail section

### 26-A02: Press `/` in YAML view activates search input

**Given:** the YAML view is displayed for an RDS instance showing syntax-colored YAML
**When:** the user presses `/`
**Then:** the header right side changes from "? for help" to "/" (amber #e0af68, bold) with a text cursor
**And:** the YAML content remains unchanged

**AWS comparison:**

```
aws rds describe-db-instances --db-instance-identifier mydb --output yaml | grep "search-term"
```

### 26-A03: Press `/` in policy document view activates search input

**Given:** the policy document view is displaying a pretty-printed JSON IAM policy
**When:** the user presses `/`
**Then:** the header right side changes from "? for help" to "/" (amber #e0af68, bold)

**AWS comparison:**

```
aws iam get-policy-version --policy-arn ARN --version-id v1 --query 'PolicyVersion.Document' --output text
# URL-decode, pretty-print, then grep -- a9s replaces this with interactive search
```

### 26-A04: Press `/` in log events view activates search input

**Given:** the log events view is displayed with log lines from a CloudWatch log stream
**When:** the user presses `/`
**Then:** the header right side changes from "? for help" to "/" (amber #e0af68, bold) with a text cursor

**AWS comparison:**

```
aws logs get-log-events --log-group-name GROUP --log-stream-name STREAM | grep "search-term"
```

Expected fields visible: Timestamp (width 22), Message (width 120)

### 26-A05: Press `/` in build logs view activates search input

**Given:** the build logs view is displayed with log lines from a CodeBuild build
**When:** the user presses `/`
**Then:** the header right side changes from "? for help" to "/" (amber #e0af68, bold) with a text cursor

**AWS comparison:**

```
aws logs get-log-events --log-group-name /aws/codebuild/project --log-stream-name BUILD_ID | grep "ERROR"
```

Expected fields visible: Timestamp (width 22), Message (width 120)

---

## B. Typing a Search Query

### 26-B01: Characters typed appear in the header search input

**Given:** the user has pressed `/` and the search input is active in any text view
**When:** the user types "running"
**Then:** the header right side shows "/running" (amber #e0af68, bold) with a cursor after the last character

### 26-B02: Backspace removes the last character from the search input

**Given:** the search input shows "/running"
**When:** the user presses Backspace
**Then:** the search input shows "/runnin"
**When:** the user presses Backspace six more times
**Then:** the search input shows "/" (empty query)

### 26-B03: Content remains unchanged while typing (before Enter)

**Given:** the search input shows "/running" in a detail view
**When:** the user has not yet pressed Enter
**Then:** the detail content is visually unchanged -- no matches are highlighted yet

### 26-B04: Special characters in search query

**Given:** the search input is active in any text view
**When:** the user types "s3:Get" (contains a colon)
**Then:** the header shows "/s3:Get" -- the colon is treated as a literal character in the search query, not as a command mode trigger

### 26-B05: Search query with dots, dashes, underscores, slashes

**Given:** the search input is active in a YAML view
**When:** the user types "us-east-1a"
**Then:** the header shows "/us-east-1a" and special characters are treated literally

---

## C. Confirming Search (Enter)

### 26-C01: Enter confirms search and highlights all matches

**Given:** the search input shows "/running" in a detail view for an EC2 instance
**When:** the user presses Enter
**Then:** the search input closes (header right reverts to "? for help")
**And:** all occurrences of "running" in the detail content are highlighted with amber background (#e0af68) and dark foreground (#1a1b26)
**And:** the first occurrence is distinguished as the current match with orange background (#ff9e64), dark foreground (#1a1b26), bold
**And:** a match indicator appears showing "[1/N matches]" where N is the total count

### 26-C02: Enter on empty search query does nothing

**Given:** the search input shows "/" (empty query)
**When:** the user presses Enter
**Then:** no search is activated; the view returns to normal mode
**And:** no matches are highlighted; no match indicator appears

### 26-C03: Esc cancels search input without activating search

**Given:** the search input shows "/running"
**When:** the user presses Esc
**Then:** the search input is cleared; the header reverts to "? for help"
**And:** no matches are highlighted; the view returns to normal mode

---

## D. Match Highlighting

### 26-D01: All matches are highlighted with amber background

**Given:** a YAML view for an EC2 instance contains the word "running" in 3 places (e.g., State.Name, an event status, a tag value)
**When:** the user searches for "running" and confirms with Enter
**Then:** all 3 occurrences show amber background (#e0af68) with dark foreground (#1a1b26)
**And:** one of them (the current match) shows orange background (#ff9e64) with dark foreground, bold

### 26-D02: Current match is visually distinct from other matches

**Given:** a search is active with 5 matches, and the current match is #2
**When:** the user looks at the view
**Then:** match #2 has orange background (#ff9e64) and bold text (the current match indicator)
**And:** matches #1, #3, #4, #5 have amber background (#e0af68) and normal weight text
**And:** the two colors are visually distinguishable from each other

### 26-D03: Search highlighting overrides syntax coloring

**Given:** a YAML view has a key "InstanceType" colored blue (#7aa2f7) and a value "t3.medium" colored green (#9ece6a)
**When:** the user searches for "medium"
**Then:** the matched portion "medium" inside "t3.medium" shows amber/orange background, overriding the green syntax color
**And:** the non-matched portion "t3." retains its original green color

### 26-D04: Search highlighting overrides status coloring in detail view

**Given:** a detail view shows "Status: running" where "running" is green (#9ece6a)
**When:** the user searches for "running"
**Then:** the matched "running" text shows amber/orange background, overriding the green status color
**And:** the "Status:" key label retains its blue color

### 26-D05: Partial match within a syntax token

**Given:** a YAML view has the value "us-east-1a" displayed in green (#9ece6a)
**When:** the user searches for "east"
**Then:** only the "east" portion of "us-east-1a" is highlighted with amber/orange background
**And:** "us-" and "-1a" retain their original green color

---

## E. Match Navigation (n/N)

### 26-E01: Press `n` advances to the next match

**Given:** a search is active with 4 matches, currently on match #1
**When:** the user presses `n`
**Then:** the current match advances to #2
**And:** the match indicator updates from "[1/4 matches]" to "[2/4 matches]"
**And:** match #1 reverts from orange to amber background
**And:** match #2 changes from amber to orange background

### 26-E02: Press `n` wraps from last match to first match

**Given:** a search is active with 4 matches, currently on match #4
**When:** the user presses `n`
**Then:** the current match wraps to #1
**And:** the match indicator updates from "[4/4 matches]" to "[1/4 matches]"

### 26-E03: Press `N` moves to the previous match

**Given:** a search is active with 4 matches, currently on match #3
**When:** the user presses `N`
**Then:** the current match moves to #2
**And:** the match indicator updates from "[3/4 matches]" to "[2/4 matches]"

### 26-E04: Press `N` wraps from first match to last match

**Given:** a search is active with 4 matches, currently on match #1
**When:** the user presses `N`
**Then:** the current match wraps to #4
**And:** the match indicator updates from "[1/4 matches]" to "[4/4 matches]"

### 26-E05: n/N with a single match

**Given:** a search is active with exactly 1 match
**When:** the user presses `n`
**Then:** the match indicator stays at "[1/1 matches]" (no change, no error)
**When:** the user presses `N`
**Then:** the match indicator stays at "[1/1 matches]"

---

## F. Match Counter Display

### 26-F01: Match counter shows current position and total

**Given:** a search finds 17 matches in a YAML view
**When:** the search is first confirmed
**Then:** the match indicator shows "[1/17 matches]" in dim text

### 26-F02: Match counter updates as user navigates

**Given:** the match indicator shows "[3/17 matches]"
**When:** the user presses `n`
**Then:** the match indicator updates to "[4/17 matches]"

### 26-F03: Match counter for zero matches

**Given:** the user searches for "nonexistent-term-xyz" in a detail view
**When:** the search is confirmed with Enter
**Then:** the match indicator shows "[0/0 matches]" or "No matches"
**And:** no highlights appear in the content

### 26-F04: Match counter position

**Given:** a search is active with matches
**When:** the user looks at the bottom of the frame
**Then:** the match indicator (e.g., "[3/17 matches]") appears at the bottom of the frame content in dim text (#565f89)

---

## G. Exiting Search

### 26-G01: Esc clears highlights and returns to normal mode

**Given:** a search is active with highlighted matches and match indicator visible
**When:** the user presses Esc
**Then:** all search highlights are removed from the content
**And:** the match indicator disappears
**And:** the viewport position is preserved (does not jump to top)
**And:** the header right side continues to show "? for help"

### 26-G02: Starting a new search replaces the current search

**Given:** a search is active with "running" highlighted (3 matches)
**When:** the user presses `/` again
**Then:** the current search highlights are cleared
**And:** the header right side changes to "/" (search input, ready for new query)
**When:** the user types "stopped" and presses Enter
**Then:** the new search activates with "stopped" matches highlighted
**And:** the previous "running" highlights are gone

### 26-G03: Esc during search input (before Enter) does NOT exit the view

**Given:** the search input is active showing "/running" in a YAML view
**When:** the user presses Esc
**Then:** the search input is cleared; the header reverts to "? for help"
**And:** the user remains in the YAML view (Esc does NOT navigate back)

### 26-G04: Esc during search results THEN Esc again exits the view

**Given:** a search is active with highlighted matches in a detail view
**When:** the user presses Esc
**Then:** search highlights are cleared; the user remains in the detail view in normal mode
**When:** the user presses Esc again
**Then:** the user navigates back to the resource list

---

## H. Empty and No-Match States

### 26-H01: Search in view with no content

**Given:** a detail view displays a resource that has very few fields (e.g., an S3 bucket with only Name and CreationDate)
**When:** the user searches for "nonexistent-text"
**Then:** the match indicator shows "[0/0 matches]" or "No matches"
**And:** no crash or error occurs

### 26-H02: Search term found zero times

**Given:** a YAML view displays an EC2 instance YAML (100+ lines)
**When:** the user searches for "zzz-no-such-value-zzz"
**Then:** no highlights appear in the content
**And:** the match indicator shows "[0/0 matches]" or "No matches"
**And:** pressing `n` has no effect (no match to jump to)

### 26-H03: n/N pressed with zero matches

**Given:** a search is active with 0 matches
**When:** the user presses `n`
**Then:** nothing happens -- no crash, no viewport movement, no indicator change
**When:** the user presses `N`
**Then:** nothing happens

---

## I. Case Sensitivity

### 26-I01: Search is case-insensitive by default

**Given:** a YAML view contains "Status: running" and "RunTimeConfig: ..."
**When:** the user searches for "run"
**Then:** both "run" in "running" and "Run" in "RunTimeConfig" are highlighted as matches
**And:** the match count includes both

**AWS comparison:**

```
aws ec2 describe-instances --output yaml | grep -i "run"
# Case-insensitive search -- a9s should match this behavior
```

### 26-I02: Case-insensitive search matches uppercase keys in YAML

**Given:** a YAML view contains keys like "InstanceId:", "InstanceType:", "PublicIpAddress:"
**When:** the user searches for "instance"
**Then:** "Instance" in "InstanceId" and "Instance" in "InstanceType" are both highlighted
**And:** the match count reflects all case-insensitive matches

### 26-I03: Case-insensitive search in detail view

**Given:** a detail view shows "Name: api-PROD-01" and "Status: RUNNING"
**When:** the user searches for "prod"
**Then:** "PROD" in the Name value is highlighted
**And:** the match count includes it

---

## J. ANSI-Aware Search (Searching in Styled Content)

### 26-J01: Search operates on visible text, not ANSI escape codes

**Given:** a YAML view renders "InstanceType: t3.medium" where "InstanceType" is blue (#7aa2f7) and "t3.medium" is green (#9ece6a) -- both containing ANSI escape codes internally
**When:** the user searches for "t3.medium"
**Then:** the text "t3.medium" is highlighted as a match
**And:** the search does NOT match on any ANSI escape sequence characters

### 26-J02: Search match spans styled boundary

**Given:** a detail view shows "Key: value" where "Key" is blue and "value" is white, with a colon and space in between
**When:** the user searches for "Key: value"
**Then:** the entire "Key: value" span is highlighted as a single match
**And:** the search correctly handles the style boundary between key and value

### 26-J03: Search in syntax-highlighted JSON (policy document)

**Given:** a policy document view displays `"Effect": "Allow"` where "Effect" is blue, the colon is dim, and "Allow" is bright green bold
**When:** the user searches for "Allow"
**Then:** the "Allow" text is highlighted with amber/orange background, overriding the bright green syntax color
**And:** only "Allow" is highlighted, not the surrounding quotes or key

### 26-J04: Search in colored status values (detail view)

**Given:** a detail view shows "Status: running" where "running" is rendered in green (#9ece6a)
**When:** the user searches for "running"
**Then:** the "running" text is highlighted with amber/orange background, overriding the green status color

---

## K. Scroll-to-Match

### 26-K01: Viewport scrolls to make current match visible

**Given:** a YAML view has 200 lines and the current match (match #1) is on line 5 (visible in the viewport)
**When:** the user presses `n` and match #2 is on line 150 (currently off-screen below)
**Then:** the viewport scrolls to make line 150 visible, approximately centering the match vertically in the viewport

### 26-K02: Match at the top of a long document

**Given:** a search has matches at lines 3, 80, and 190 in a 200-line YAML document
**When:** the user presses `N` from match #1 (line 3, wrapping to match #3)
**Then:** the viewport scrolls to line 190, making the last match visible

### 26-K03: Match already visible does not cause unnecessary scroll

**Given:** a search has matches at lines 10, 12, and 14 and the viewport shows lines 1-30
**When:** the user presses `n` from match #1 to match #2
**Then:** the viewport does not scroll (match #2 at line 12 is already visible)
**And:** the orange highlight moves from line 10 to line 12

### 26-K04: Scroll-to-match works with word wrap enabled

**Given:** word wrap is ON and a YAML view has a match on a long line that wraps to 3 display lines
**When:** the user navigates to that match with `n`
**Then:** the viewport scrolls to make the wrapped display line containing the match visible
**And:** the highlight appears on the correct portion of the wrapped text

---

## L. Word Wrap Interaction

### 26-L01: Search results update when word wrap is toggled on

**Given:** a search is active in a YAML view with 5 matches (wrap off)
**When:** the user presses `w` to enable word wrap
**Then:** the same 5 matches remain highlighted in the wrapped content
**And:** the match count stays at "[N/5 matches]"
**And:** match positions adjust to the new line layout

### 26-L02: Search results update when word wrap is toggled off

**Given:** a search is active in a detail view with 3 matches (wrap on)
**When:** the user presses `w` to disable word wrap
**Then:** the same 3 matches remain highlighted
**And:** the match count stays at "[N/3 matches]"

### 26-L03: Long value match visible only after wrapping

**Given:** a YAML view has a very long ARN value that extends beyond the frame width (clipped)
**And:** the search term matches a portion of the ARN that is clipped (not visible)
**When:** the user presses `w` to enable word wrap
**Then:** the previously hidden match becomes visible as the line wraps
**And:** the match is highlighted with amber/orange background

---

## M. Edge Cases

### 26-M01: Search in very long single line

**Given:** a YAML view has a CertificateAuthority.Data field with a base64 string spanning hundreds of characters on one line
**When:** the user searches for a substring within the base64 data (e.g., "LS0t")
**Then:** the match is found and highlighted
**And:** the viewport scrolls horizontally (if wrap is off) or vertically (if wrap is on) to make the match visible

### 26-M02: Multiple matches on the same line

**Given:** a log events view has a log line: "ERROR: failed to process ERROR code in ERROR handler"
**When:** the user searches for "ERROR"
**Then:** all 3 occurrences of "ERROR" on that single line are highlighted
**And:** the match count shows 3 (or more if other lines also contain "ERROR")
**And:** pressing `n` moves through each individual occurrence, not just each line

### 26-M03: Search term at the very beginning of content

**Given:** a YAML view starts with "AmiLaunchIndex: 0" on line 1
**When:** the user searches for "Ami"
**Then:** the "Ami" at the very beginning of the first line is highlighted

### 26-M04: Search term at the very end of content

**Given:** a YAML view ends with "VpcId: vpc-0123456789abcdef0" as the last line
**When:** the user searches for "abcdef0"
**Then:** the "abcdef0" at the end of the last line is highlighted

### 26-M05: Search for a single character

**Given:** the search input is active in any text view
**When:** the user types ":" and presses Enter
**Then:** all colons in the content are highlighted
**And:** the match count may be very high (colons appear in every YAML key line)
**And:** `n`/`N` navigation still works correctly through all matches

### 26-M06: Rapid n/N navigation through many matches

**Given:** a search for ":" finds 150 matches in a YAML view
**When:** the user rapidly presses `n` many times
**Then:** the match counter increments correctly for each press
**And:** the viewport scrolls smoothly to follow the current match
**And:** no visual glitches, lag, or missed updates occur

---

## N. Component Reuse Verification

These stories verify that the search component behaves identically across all 5 view types that embed it.

### 26-N01: Identical activation across all view types

**Given:** the user is in each of the following views, one at a time: Detail (EC2), YAML (RDS), Policy Document, Log Events, Build Logs
**When:** the user presses `/` in each view
**Then:** the header right side changes identically: from "? for help" to "/" (amber #e0af68, bold) in every view

### 26-N02: Identical match highlighting across all view types

**Given:** the user performs a search that finds matches in each of the 5 view types
**When:** the search is confirmed with Enter in each view
**Then:** match highlighting uses the same two colors in every view: amber (#e0af68) for non-current matches, orange (#ff9e64) for the current match
**And:** the match indicator format "[N/M matches]" is identical in every view

### 26-N03: Identical n/N behavior across all view types

**Given:** a search is active with multiple matches in each of the 5 view types
**When:** the user presses `n` and `N` in each view
**Then:** the navigation behavior is identical: `n` goes forward, `N` goes backward, wrapping at boundaries

### 26-N04: Identical Esc behavior across all view types

**Given:** a search is active with highlighted matches in each of the 5 view types
**When:** the user presses Esc in each view
**Then:** all highlights are cleared, the match indicator disappears, and normal mode is restored identically in every view

### 26-N05: Search in detail view for every resource type

**Given:** the detail view is open for each resource type in turn (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager, Lambda, ECS services, CloudFormation stacks, etc.)
**When:** the user presses `/`, types a term known to appear in that resource's detail fields, and presses Enter
**Then:** the search component works correctly for every resource type -- matches are highlighted, `n`/`N` navigates, Esc clears

**AWS comparison:**

```
aws ec2 describe-instances --instance-ids i-xxx --output yaml | grep -i "search-term"
aws rds describe-db-instances --db-instance-identifier X --output yaml | grep -i "search-term"
aws eks describe-cluster --name X --output yaml | grep -i "search-term"
# etc. -- a9s provides interactive in-document search equivalent to piping to grep
```

### 26-N06: Search in YAML view for every resource type

**Given:** the YAML view is open for each resource type in turn
**When:** the user presses `/`, types a search term, and presses Enter
**Then:** the search component works correctly: matches are highlighted in the syntax-colored YAML, `n`/`N` navigates, Esc clears

---

## O. Interaction with Other Key Bindings

### 26-O01: j/k scroll works while search is active

**Given:** a search is active with highlighted matches in a YAML view
**When:** the user presses `j` to scroll down
**Then:** the viewport scrolls down one line
**And:** all search highlights remain visible and intact
**And:** the match indicator stays visible

### 26-O02: g/G jump works while search is active

**Given:** a search is active with highlighted matches in a detail view
**When:** the user presses `G` to jump to the bottom
**Then:** the viewport jumps to the last line
**And:** all search highlights remain visible
**And:** the current match indicator (orange) remains on whichever match was current

### 26-O03: PageUp/PageDown works while search is active

**Given:** a search is active in a YAML view with matches spread across multiple pages
**When:** the user presses PageDown
**Then:** the viewport scrolls down one page
**And:** matches on the new visible page are highlighted
**And:** the current match indicator and counter are unchanged

### 26-O04: Copy (c) works while search is active

**Given:** a search is active with highlighted matches in a YAML view
**When:** the user presses `c`
**Then:** the full content is copied to the clipboard (NOT just matched text)
**And:** the copied text is plain (no ANSI codes, no search highlight markup)
**And:** the header flashes "Copied!" in green (#9ece6a) bold
**And:** after the flash clears, the search highlights remain active

### 26-O05: Help (?) works while search is active

**Given:** a search is active with highlighted matches in a detail view
**When:** the user presses `?`
**Then:** the help screen replaces the content
**When:** the user presses any key to close help
**Then:** the view returns with all search highlights preserved and match indicator intact

### 26-O06: Ctrl+r refresh clears search state

**Given:** a search is active with highlighted matches in any text view
**When:** the user presses Ctrl+r
**Then:** the loading spinner appears; the search highlights are cleared
**When:** the data finishes loading
**Then:** the user is in normal mode (no search active); fresh content is displayed

### 26-O07: Command mode (`:`) works while search is active

**Given:** a search is active with highlighted matches in a YAML view
**When:** the user presses `:`
**Then:** command mode activates (header right shows ":") and the search highlights remain visible behind the command input

---

## P. Help Screen Shows Search Key Bindings

### 26-P01: Help screen in detail view lists search bindings

**Given:** the user is in a detail view
**When:** the user presses `?`
**Then:** the help screen includes `</>` Search, `<n>` Next Match, and `<N>` Prev Match in the key listing

### 26-P02: Help screen in YAML view lists search bindings

**Given:** the user is in a YAML view
**When:** the user presses `?`
**Then:** the help screen includes `</>` Search, `<n>` Next Match, and `<N>` Prev Match

### 26-P03: Help screen in log events view lists search bindings

**Given:** the user is in a log events view
**When:** the user presses `?`
**Then:** the help screen includes `</>` Search, `<n>` Next Match, and `<N>` Prev Match under the appropriate category

### 26-P04: Help screen in build logs view lists search bindings

**Given:** the user is in a build logs view
**When:** the user presses `?`
**Then:** the help screen includes `</>` Search, `<n>` Next Match, and `<N>` Prev Match

---

## Q. Terminal Resize During Search

### 26-Q01: Resize preserves search state

**Given:** a search is active with 5 highlighted matches in a YAML view
**When:** the terminal is resized (wider or narrower)
**Then:** the layout reflows to the new width
**And:** all 5 search highlights are preserved
**And:** the match indicator continues to show the correct count

### 26-Q02: Resize with word wrap on adjusts match positions

**Given:** a search is active with word wrap enabled in a detail view
**When:** the terminal is resized to a narrower width
**Then:** lines re-wrap at the new boundary
**And:** search matches that were on one display line may now span differently
**And:** all matches remain highlighted correctly

### 26-Q03: Resize below minimum terminal size

**Given:** a search is active in any text view
**When:** the terminal is resized below the minimum dimensions (< 60 cols or < 7 rows)
**Then:** an error message appears: "Terminal too narrow. Please resize." or "Terminal too short. Please resize."
**When:** the terminal is resized back above minimum dimensions
**Then:** the view returns with search state preserved (highlights and match indicator intact)

---

## R. Real-World Scenarios

### 26-R01: Finding a specific field in a large EC2 detail view

**Given:** the detail view for an EC2 instance shows 30+ fields including InstanceId, PrivateIpAddress, PublicIpAddress, VpcId, SubnetId, SecurityGroups, Tags, etc.
**When:** the user presses `/`, types "10.0" to find the private IP, and presses Enter
**Then:** the private IP address "10.0.1.42" (or similar) is highlighted
**And:** the viewport scrolls to make the match visible if it was off-screen

**AWS comparison:**

```
aws ec2 describe-instances --instance-ids i-xxx --query 'Reservations[0].Instances[0].PrivateIpAddress'
# a9s provides this with interactive search instead of memorizing query paths
```

### 26-R02: Searching YAML for a security group ID

**Given:** the YAML view for an EC2 instance shows a complex nested structure with SecurityGroups array
**When:** the user searches for "sg-0abc123"
**Then:** the security group ID is highlighted inside the nested SecurityGroups array
**And:** the viewport scrolls to the match if the array is deep in the YAML

**AWS comparison:**

```
aws ec2 describe-instances --instance-ids i-xxx --output yaml | grep "sg-0abc123"
```

### 26-R03: Searching build logs for an error

**Given:** the build logs view shows 240 log lines from a failed CodeBuild build
**When:** the user searches for "FAILED" and presses Enter
**Then:** all log lines containing "FAILED" are highlighted
**And:** the user can press `n` to navigate through each occurrence
**And:** the viewport scrolls to center each match as the user navigates

**AWS comparison:**

```
aws logs get-log-events --log-group-name /aws/codebuild/project --log-stream-name BUILD --query 'events[].message' | grep -i "FAILED"
```

### 26-R04: Searching log events for a request ID

**Given:** the log events view shows hundreds of Lambda invocation log lines
**When:** the user searches for a specific request ID like "c4b2a1d3-e5f6-7890-abcd-ef1234567890"
**Then:** the log lines containing that request ID are highlighted
**And:** pressing `n` moves through them chronologically

**AWS comparison:**

```
aws logs filter-log-events --log-group-name /aws/lambda/my-function --filter-pattern "c4b2a1d3-e5f6-7890-abcd-ef1234567890"
```

### 26-R05: Comparing detail and YAML search for the same resource

**Given:** the user opens detail view for an RDS instance and searches for "available"
**Then:** the status field "available" is highlighted in the detail view
**When:** the user presses Esc (clear search), then `y` to switch to YAML view, then searches for "available" again
**Then:** all occurrences of "available" in the YAML are highlighted -- this may include "DBInstanceStatus: available" and "Status: available" under VpcSecurityGroups
**And:** the YAML search may find more matches than the detail search (since YAML shows all fields)

---

## S. Key Binding Coverage Summary

Every key binding related to the search component appears in at least one story:

| Key | Stories |
|-----|---------|
| `/` (activate search) | 26-A01 through 26-A05, 26-G02 |
| Character input (typing) | 26-B01, 26-B04, 26-B05 |
| Backspace | 26-B02 |
| Enter (confirm search) | 26-C01, 26-C02 |
| Esc (cancel search input) | 26-C03, 26-G03 |
| Esc (clear search results) | 26-G01, 26-G04 |
| `n` (next match) | 26-E01, 26-E02, 26-E05, 26-M06 |
| `N` (previous match) | 26-E03, 26-E04, 26-E05 |
| `w` (word wrap interaction) | 26-L01, 26-L02, 26-L03 |
| `c` (copy during search) | 26-O04 |
| `j`/`k` (scroll during search) | 26-O01 |
| `g`/`G` (jump during search) | 26-O02 |
| `PageUp`/`PageDown` (during search) | 26-O03 |
| `?` (help during search) | 26-O05, 26-P01 through 26-P04 |
| `Ctrl+r` (refresh during search) | 26-O06 |
| `:` (command during search) | 26-O07 |

---

## Cross-References

### Stories in QA-25 (Policy Document View) Section H

Section H of `25-policy-document-view.md` defines 13 search stories (H.1.1 through H.3.6) specific to the policy document context. These stories remain valid as *specializations* of the generic component behavior defined here. Specifically:

- **H.1.1-H.1.4 (Entering Search Mode):** Consistent with stories 26-A03, 26-B01, 26-C01, 26-C03 here. No amendment needed.
- **H.2.1-H.2.7 (Search Results/Navigation):** Consistent with stories 26-D01, 26-E01-E04, 26-G01, 26-G02 here. No amendment needed.
- **H.3.1-H.3.6 (Edge Cases):** These are policy-doc-specific edge cases (e.g., searching for "Effect", case-insensitive "allow", wildcard "*"). They complement rather than duplicate the generic edge cases here (26-H01-H03, 26-I01-I03, 26-M01-M06).

**Recommended amendment to H.2.1:** Story H.2.1 specifies that the match indicator appears "at the bottom of the frame." This file (26-F04) also specifies the bottom-of-frame position. The component should define this position once, and H.2.1 can reference "per the search component spec" rather than defining its own position.

### Stories in QA-11 (Filtering)

- **11-17 (Detail view -- / key is ignored):** SUPERSEDED. With the search component, `/` activates in-document search in the detail view. This story should be removed or replaced with a reference to 26-A01.
- **11-18 (YAML view -- / key is ignored):** SUPERSEDED. With the search component, `/` activates in-document search in the YAML view. This story should be removed or replaced with a reference to 26-A02.

### Distinction from List Filtering (QA-11)

The `/` key serves two different purposes depending on context:

| Context | `/` behavior | Mode name | Stories |
|---------|-------------|-----------|---------|
| List views (resource list, main menu, profile/region selector) | Filters rows by substring match | Filter mode | QA-11 |
| Text content views (detail, YAML, policy doc, log events, build logs) | Searches within document, highlights matches, enables n/N navigation | Search mode | QA-26 (this file) |

The user does not need to know this distinction -- both modes activate with `/` and show amber text in the header right side. The difference is behavioral: filter mode hides non-matching rows; search mode highlights matching text within the current content.

# Child View: Role Policies --> Policy Document (JSON Viewer)

**Status:** Planned
**Tier:** SHOULD-HAVE
**Nesting Level:** 2 (grandchild: IAM Roles --> Role Policies --> Policy Document)

---

## Navigation

- **Entry:** Press Enter on a policy in the Role Policies list
- **Frame title:** `policy-doc --- AmazonS3ReadOnlyAccess (Managed v1)` or `policy-doc --- payment-service-custom-policy (Inline)`
- **View stack:** IAM Roles --> Attached Policies --> Policy Document
- **Esc** returns to Role Policies list
- **New key bindings:** `/` (search), `n`/`N` (next/prev match), `c` (copy document), `w` (word wrap)

---

## AWS API

### Managed Policies
1. `iam:GetPolicy` with `PolicyArn` -- returns policy metadata including `DefaultVersionId` and `Versions` count
2. `iam:GetPolicyVersion` with `PolicyArn` + `VersionId` -- returns URL-encoded JSON `Document`
3. URL-decode the `Document` field
4. Pretty-print with 2-space indent via `json.MarshalIndent`

### Inline Policies
1. `iam:GetRolePolicy` with `RoleName` + `PolicyName` -- returns URL-encoded `PolicyDocument`
2. URL-decode the `PolicyDocument` field
3. Pretty-print with 2-space indent via `json.MarshalIndent`

### Latency
- Fast (<1 second). Managed policies require 2 serial API calls (GetPolicy + GetPolicyVersion).
- Inline policies require 1 call.
- Show spinner during fetch.

### Error Cases
- Policy deleted between listing and viewing: show "Policy not found" error in frame
- Insufficient permissions: show "AccessDenied: unable to read policy document"

---

## Data Model

This view does NOT use views.yaml columns. It is a document viewer, not a list/table. The rendered content is:

1. **Header metadata** (2-3 lines above the JSON, inside the frame)
2. **JSON document** (scrollable, syntax-highlighted)

### Header metadata for managed policies
```
 Policy:   AmazonS3ReadOnlyAccess
 ARN:      arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess
 Version:  v1 (default) -- 1 version(s)
```

### Header metadata for inline policies
```
 Policy:   payment-service-custom-policy
 Type:     Inline Policy (attached to payment-service-execution-role)
```

### Separator
A dim horizontal rule (`---...`) separates the header from the JSON body, consistent with the Reveal View pattern.

---

## JSON Syntax Highlighting

Colors from the Tokyo Night Dark palette, extending the existing YAML highlighting with JSON-specific semantics:

| JSON Element          | Foreground  | Style  | Notes                                    |
|-----------------------|-------------|--------|------------------------------------------|
| Keys                  | `#7aa2f7`   | --     | Reuses `ColYAMLKey` / `ColDetailKey`     |
| String values         | `#9ece6a`   | --     | Reuses `ColYAMLStr`                      |
| `"Allow"`             | `#73daca`   | Bold   | Bright green -- safe, permitted           |
| `"Deny"`              | `#f7768e`   | Bold   | Bright red -- THE critical visual signal  |
| Numbers               | `#ff9e64`   | --     | Reuses `ColYAMLNum`                      |
| Booleans              | `#bb9af7`   | --     | Reuses `ColYAMLBool`                     |
| `null`                | `#565f89`   | Dim    | Reuses `ColYAMLNull`                     |
| Brackets/braces `{}`  | `#565f89`   | --     | Dim structural punctuation               |
| Commas                | `#565f89`   | --     | Dim structural punctuation               |
| ARN patterns          | `#7dcfff`   | --     | `arn:aws:*` strings get cyan treatment   |
| Wildcard `"*"`        | `#f7768e`   | Bold   | Red -- overprivileged resource indicator  |
| Search match          | `#1a1b26`   | --     | Dark fg on `#e0af68` (amber) background  |
| Current search match  | `#1a1b26`   | Bold   | Dark fg on `#ff9e64` (orange) background |

### Highlighting Priority (highest wins)
1. Search match highlighting (overrides all below)
2. `"Allow"` / `"Deny"` / `"*"` semantic highlighting
3. ARN pattern detection (`arn:aws:` prefix in string values)
4. Standard JSON type-based coloring

### ARN Detection
A string value is colored as an ARN if it starts with `arn:aws:` or `arn:aws-cn:` or `arn:aws-us-gov:`. This runs after JSON parsing, only on string values.

---

## ASCII Wireframes

### Wireframe 1: Main View -- Managed Policy Document

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+------------- policy-doc --- AmazonS3ReadOnlyAccess (Managed v1) ---------------+
| Policy:   AmazonS3ReadOnlyAccess                                               |
| ARN:      arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess                       |
| Version:  v1 (default) -- 1 version(s)                                         |
| ------------------------------------------------------------------------------ |
| {                                                                              |
|   "Version": "2012-10-17",                                                     |
|   "Statement": [                                                               |
|     {                                                                          |
|       "Sid": "AllowS3Read",                                                    |
|       "Effect": "Allow",                                                       |
|       "Action": [                                                              |
|         "s3:GetObject",                                                        |
|         "s3:ListBucket"                                                        |
|       ],                                                                       |
|       "Resource": [                                                            |
|         "arn:aws:s3:::my-bucket",                                              |
|         "arn:aws:s3:::my-bucket/*"                                             |
|       ]                                                                        |
|     },                                                                         |
|     {                                                                          |
|       "Sid": "DenyDeleteBucket",                                               |
|       "Effect": "Deny",                                                        |
|       "Action": "s3:DeleteBucket",                                             |
|       "Resource": "*"                                                          |
|     }                                                                          |
|   ]                                                                            |
| }                                                                              |
+--------------------------------------------------------------------------------+
```

Color annotations (cannot be shown in ASCII):
- `"Version"`, `"Statement"`, `"Sid"`, `"Effect"`, `"Action"`, `"Resource"` -- blue `#7aa2f7`
- `"2012-10-17"`, `"AllowS3Read"`, `"s3:GetObject"`, `"s3:ListBucket"`, `"s3:DeleteBucket"`, `"DenyDeleteBucket"` -- green `#9ece6a`
- `"Allow"` -- bright green `#73daca` bold
- `"Deny"` -- bright red `#f7768e` bold
- `"arn:aws:s3:::my-bucket"`, `"arn:aws:s3:::my-bucket/*"` -- cyan `#7dcfff`
- `"*"` (Resource wildcard) -- red `#f7768e` bold
- `{`, `}`, `[`, `]`, `,` -- dim `#565f89`

### Wireframe 2: Search Active -- `/s3:GetObject`

```
 a9s v0.5.0  prod:us-east-1                                       /s3:GetObject|
+------------- policy-doc --- AmazonS3ReadOnlyAccess (Managed v1) ---------------+
| Policy:   AmazonS3ReadOnlyAccess                                               |
| ARN:      arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess                       |
| Version:  v1 (default) -- 1 version(s)                                         |
| ------------------------------------------------------------------------------ |
| {                                                                              |
|   "Version": "2012-10-17",                                                     |
|   "Statement": [                                                               |
|     {                                                                          |
|       "Sid": "AllowS3Read",                                                    |
|       "Effect": "Allow",                                                       |
|       "Action": [                                                              |
|         "[s3:GetObject]",        <-- amber bg, dark fg (current match)         |
|         "s3:ListBucket"                                                        |
|       ],                                                                       |
|       "Resource": [                                                            |
|         "arn:aws:s3:::my-bucket",                                              |
|         "arn:aws:s3:::my-bucket/*"                                             |
|       ]                                                                        |
|     }                                                                          |
|   ]                                                                            |
| }                                                                              |
|                                                                                |
| [1/1 matches]                                                                  |
+--------------------------------------------------------------------------------+
```

Header right side: `/s3:GetObject|` in amber `#e0af68` bold (reuses existing filter input pattern).

Match indicator at the bottom: `[1/1 matches]` in dim. When there are multiple matches: `[2/5 matches]`, `n` jumps to next, `N` jumps to previous. The current match has an orange background `#ff9e64`, other matches have amber background `#e0af68`.

### Wireframe 3: Deny Statement with Wildcard Resource (Danger View)

This shows the most important visual signal -- a Deny statement with a wildcard resource.

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- policy-doc --- admin-override-policy (Inline) ------------------------+
| Policy:   admin-override-policy                                                 |
| Type:     Inline Policy (attached to emergency-access-role)                     |
| ------------------------------------------------------------------------------ |
| {                                                                              |
|   "Version": "2012-10-17",                                                     |
|   "Statement": [                                                               |
|     {                                                                          |
|       "Sid": "DenyAllS3Deletes",                                               |
|       "Effect": "Deny",          <-- RED BOLD -- jumps out immediately          |
|       "Action": [                                                              |
|         "s3:DeleteObject",                                                     |
|         "s3:DeleteBucket"                                                      |
|       ],                                                                       |
|       "Resource": "*"            <-- RED BOLD -- overprivileged wildcard        |
|     },                                                                         |
|     {                                                                          |
|       "Effect": "Allow",         <-- GREEN BOLD -- safe                         |
|       "Action": "s3:GetObject",                                                |
|       "Resource": "arn:aws:s3:::logs-bucket/*"                                 |
|     }                                                                          |
|   ]                                                                            |
| }                                                                              |
+--------------------------------------------------------------------------------+
```

### Wireframe 4: Help Screen

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+------------------------------- Help --------------------------------------------+
| POLICY DOCUMENT       GENERAL              NAVIGATION           HOTKEYS         |
|                                                                                 |
| <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      |
| <c>     Copy Doc      <q>      Quit        <k>       Up         <:>   Command   |
| <w>     Word Wrap                          <g>       Top                        |
| </>     Search                             <G>       Bottom                     |
| <n>     Next Match                         <pgup/dn> Page                       |
| <N>     Prev Match                                                              |
|                                                                                 |
|                       Press any key to close                                    |
+--------------------------------------------------------------------------------+
```

### Wireframe 5: Loading State

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+------------- policy-doc --- AmazonS3ReadOnlyAccess (Managed v1) ---------------+
|                                                                                |
|        . Fetching policy document...                                           |
|                                                                                |
+--------------------------------------------------------------------------------+
```

### Wireframe 6: Error State

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+------------- policy-doc --- deleted-policy (Managed v1) -----------------------+
|                                                                                |
|   Error: NoSuchEntity -- Policy arn:aws:iam::123456789012:policy/deleted-policy |
|   was not found.                                                               |
|                                                                                |
|   Press Esc to go back.                                                        |
|                                                                                |
+--------------------------------------------------------------------------------+
```

---

## Key Bindings

| Key       | Action              | Context            | Notes                                     |
|-----------|---------------------|--------------------|-------------------------------------------|
| `Esc`     | Back                | Always             | Returns to Role Policies list             |
| `j`       | Scroll down         | Normal             | One line down in JSON viewport            |
| `k`       | Scroll up           | Normal             | One line up in JSON viewport              |
| `g`       | Go to top           | Normal             | Jump to first line of document            |
| `G`       | Go to bottom        | Normal             | Jump to last line of document             |
| `PgUp`    | Page up             | Normal             | Scroll up by viewport height              |
| `PgDn`    | Page down           | Normal             | Scroll down by viewport height            |
| `/`       | Start search        | Normal             | Enters search input mode (header right)   |
| `Enter`   | Confirm search      | Search input       | Locks search, jumps to first match        |
| `Esc`     | Cancel search       | Search input       | Clears search, returns to normal          |
| `n`       | Next match          | Search active      | Jump to next occurrence                   |
| `N`       | Previous match      | Search active      | Jump to previous occurrence               |
| `c`       | Copy document       | Normal             | Copy full JSON to clipboard, flash "Copied!" |
| `w`       | Toggle word wrap    | Normal             | Wraps long lines at frame boundary        |
| `?`       | Show help           | Normal             | Replace content with help screen          |
| `q`       | Quit                | Normal             | Exit application                          |
| `Ctrl+r`  | Refresh             | Normal             | Re-fetch policy document from AWS         |

---

## Component States

### Normal
- Header metadata visible at top
- Dim separator line
- Syntax-highlighted JSON fills remaining viewport
- Scroll indicators if content exceeds viewport (dim `#414868`): `... 12 lines below`

### Search Input Active
- Header right changes from `? for help` to `/search-text|` (amber `#e0af68` bold)
- JSON content unchanged until Enter is pressed
- `Esc` cancels and clears the search input

### Search Results Active
- Header right shows `? for help` again (search bar closes after Enter)
- All occurrences of search text highlighted with amber bg `#e0af68`
- Current match highlighted with orange bg `#ff9e64`
- Bottom of frame shows `[N/M matches]` indicator in dim
- `n`/`N` cycles through matches, scrolling viewport as needed
- `/` starts a new search (replaces current)
- `Esc` clears search highlighting and returns to normal

### Word Wrap On
- Long JSON lines wrap at the frame inner width
- Wrapped continuation lines are indented 4 spaces
- A `[wrap]` indicator appears in the header or match indicator area

### Loading
- Spinner centered in frame: `. Fetching policy document...`
- Uses existing `ColSpinner` (`#7aa2f7`)

### Error
- Error message in red `#f7768e` bold
- "Press Esc to go back." hint in dim

### Empty Document
- If the policy document is empty (should not happen in practice): "Policy document is empty."

---

## State Transitions (Msg Types)

| Current State      | Msg Type                  | Next State          |
|--------------------|---------------------------|---------------------|
| Role Policies list | `EnterChildViewMsg`       | Loading             |
| Loading            | `ChildDataMsg` (success)  | Normal              |
| Loading            | `ChildDataMsg` (error)    | Error               |
| Normal             | `tea.KeyMsg` (`/`)        | Search Input        |
| Search Input       | `tea.KeyMsg` (`Enter`)    | Search Results      |
| Search Input       | `tea.KeyMsg` (`Esc`)      | Normal              |
| Search Results     | `tea.KeyMsg` (`n`/`N`)    | Search Results      |
| Search Results     | `tea.KeyMsg` (`Esc`)      | Normal              |
| Search Results     | `tea.KeyMsg` (`/`)        | Search Input        |
| Normal             | `tea.KeyMsg` (`?`)        | Help                |
| Help               | `tea.KeyMsg` (any)        | Normal              |
| Normal             | `tea.KeyMsg` (`Esc`)      | Back to parent      |
| Normal             | `tea.KeyMsg` (`c`)        | Normal + flash      |
| Normal             | `tea.KeyMsg` (`w`)        | Normal (wrap toggled)|
| Normal             | `tea.KeyMsg` (`Ctrl+r`)   | Loading (refresh)   |
| Error              | `tea.KeyMsg` (`Esc`)      | Back to parent      |

---

## Layout Composition

```go
lipgloss.JoinVertical(lipgloss.Left,
    header,           // standard a9s header (reused)
    renderFramedBox(  // standard framed box with centered title
        append(
            metadataLines,  // 2-3 lines: policy name, ARN/type, version
            separatorLine,  // dim horizontal rule
            jsonLines...,   // syntax-highlighted JSON via viewport
        ),
        title,   // "policy-doc --- PolicyName (Managed v1)" or "(Inline)"
        width,
    ),
)
```

### Viewport
- The JSON body uses `bubbles/viewport` for scrolling
- Viewport height = `termHeight - 3 (header + top/bottom border) - metadataLines - 1 (separator)`
- Viewport content is pre-rendered with syntax highlighting (no re-render on scroll)

---

## Responsive Behavior

| Terminal Width | Behavior                                                |
|----------------|---------------------------------------------------------|
| < 60 cols      | ARN in header metadata truncated with `...`             |
| 60-80 cols     | Full content, some long JSON lines may wrap if `w` on   |
| 80-120 cols    | Comfortable -- all metadata and JSON visible            |
| > 120 cols     | Extra padding on right, content does not stretch        |

| Terminal Height | Behavior                                               |
|-----------------|--------------------------------------------------------|
| < 10 rows       | Only JSON body visible, metadata hidden                |
| 10-20 rows      | Metadata + a few JSON lines, scroll indicators shown   |
| > 20 rows       | Full view, most policies fit without scrolling          |

---

## ChildViewDef Registration

```go
// On the role_policies ResourceTypeDef:
ChildViewDefs: []resource.ChildViewDef{
    {
        ChildType:      "policy_document",
        Key:            "enter",
        ContextKeys:    map[string]string{
            "PolicyArn":  "PolicyArn",
            "PolicyName": "PolicyName",
            "PolicyType": "policy_type",   // "Managed" or "Inline"
            "RoleName":   "@parent.RoleName",  // inherited from grandparent
        },
        DisplayNameKey: "PolicyName",
    },
},
```

### Context Key Resolution
- `PolicyArn`: from the selected policy's `Fields["PolicyArn"]` (empty string for inline)
- `PolicyName`: from the selected policy's `Fields["PolicyName"]`
- `PolicyType`: from the computed `policy_type` field ("Managed" or "Inline")
- `RoleName`: inherited from the parent IAM Role context (needed for `GetRolePolicy` call)

---

## Copy Behavior

`c` copies the entire pretty-printed JSON document to the system clipboard (raw JSON without syntax highlighting ANSI codes). Flash message: "Copied!" (green).

---

## Frame Title Format

The frame title embeds policy type and version info:

- Managed: `policy-doc --- AmazonS3ReadOnlyAccess (Managed v1)`
- Managed with multiple versions: `policy-doc --- MyPolicy (Managed v3)`
- Inline: `policy-doc --- payment-service-custom-policy (Inline)`

The version shown is always the default version ID.

---

## Search Implementation Notes

### Search Scope
Search operates on the raw JSON text (before syntax highlighting). This means searching for `s3:Get` will match inside string values, and searching for `Effect` will match JSON keys.

### Match Highlighting
After syntax highlighting is applied, search match positions are overlaid with background colors. The overlay replaces the syntax color for the matched substring:
- All matches: amber bg `#e0af68`, dark fg `#1a1b26`
- Current match: orange bg `#ff9e64`, dark fg `#1a1b26`, bold

### Auto-scroll to Match
When `n`/`N` moves to a new match, the viewport scrolls to center the match line vertically (or as close as possible given document boundaries).

### Case Sensitivity
Search is case-insensitive by default. This matches the existing filter behavior in a9s list views.

---

## Comparison with Existing Views

| Feature                | YAML View      | Build Logs     | Policy Document     |
|------------------------|----------------|----------------|---------------------|
| Content type           | YAML tree      | Log lines      | JSON document       |
| Syntax highlighting    | Yes (5 types)  | Pattern-based  | Yes (8 types)       |
| Header metadata        | No             | No             | Yes (2-3 lines)     |
| Search                 | No (uses /)    | No (uses /)    | Yes (/ with n/N)    |
| Copy                   | Full YAML      | Selected line  | Full JSON           |
| Word wrap              | No             | Yes (`w`)      | Yes (`w`)           |
| Scroll                 | viewport       | viewport       | viewport            |
| Frame pattern          | Standard box   | Standard box   | Standard box        |

This view introduces the first true in-document search with match navigation (`n`/`N`) in a9s. The existing `/` filter on list views filters rows; this searches within a single document. The interaction model is different but the key binding (`/`) is familiar.

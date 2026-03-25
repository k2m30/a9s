# QA User Stories: Policy Document View (Grandchild — Level 2)

Navigation chain: IAM Roles --> Role Policies --> **Policy Document**

This is a **JSON document viewer**, not a list/table view. It displays the actual IAM
policy document (Allow/Deny statements) with syntax highlighting, in-document search,
copy, and word wrap. It is reachable by pressing Enter on any policy row in the Role
Policies child view.

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration. AWS CLI equivalents are cited so testers can verify data parity.

> **Note:** Story C.19.1 and D.4.3 in `22-ecr-rds-iam-views.md` previously stated
> "There is no further child view to drill into from a policy." This file supersedes
> that claim -- Enter on a policy row now opens the Policy Document view.

---

## A. Navigation Into the View

### A.1 Entry from Managed Policy

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I am viewing the Role Policies list for role "payment-service-execution-role". I select a managed policy named "AmazonS3ReadOnlyAccess" (Type = "Managed") and press Enter. | The view transitions to a loading state. A spinner is displayed centered inside the frame. The text reads "Fetching policy document..." (or similar). The frame title shows "policy-doc --- AmazonS3ReadOnlyAccess (Managed v1)" centered in the top border. |
| A.1.2 | The fetch completes successfully. | The spinner disappears. The frame shows: (1) header metadata lines with Policy name, ARN, and version info; (2) a dim horizontal separator; (3) the pretty-printed JSON policy document with syntax highlighting. |
| A.1.3 | I press Esc on the Policy Document view. | I return to the Role Policies list. The cursor is on the same policy ("AmazonS3ReadOnlyAccess") that I had selected. |

**AWS comparison:**
```
aws iam get-policy --policy-arn arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess
# Note DefaultVersionId from output (e.g., "v1")
aws iam get-policy-version \
  --policy-arn arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess \
  --version-id v1 \
  --query 'PolicyVersion.Document' --output text
# URL-decode the result, then pretty-print with 2-space indent
```

### A.2 Entry from Inline Policy

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | I am viewing the Role Policies list. I select an inline policy named "payment-service-custom-policy" (Type = "Inline") and press Enter. | The view transitions to a loading state with spinner. The frame title shows "policy-doc --- payment-service-custom-policy (Inline)". |
| A.2.2 | The fetch completes successfully. | The spinner disappears. The frame shows: (1) header metadata with Policy name and Type ("Inline Policy (attached to payment-service-execution-role)"); (2) a dim horizontal separator; (3) the pretty-printed JSON policy document with syntax highlighting. There is NO "Version" or "ARN" line in the metadata -- only Policy name and Type. |
| A.2.3 | I press Esc. | I return to the Role Policies list on the same inline policy row. |

**AWS comparison:**
```
aws iam get-role-policy \
  --role-name payment-service-execution-role \
  --policy-name payment-service-custom-policy \
  --query 'PolicyDocument' --output text
# URL-decode the result, then pretty-print with 2-space indent
```

### A.3 Entry from Managed Policy with Multiple Versions

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | A managed policy "MyCustomPolicy" has 3 versions, with v3 as the default. I press Enter on it. | The frame title shows "policy-doc --- MyCustomPolicy (Managed v3)". The metadata line reads "Version: v3 (default) -- 3 version(s)". The document displayed is the default version's content. |

**AWS comparison:**
```
aws iam get-policy --policy-arn arn:aws:iam::123456789012:policy/MyCustomPolicy
# Returns: Versions count and DefaultVersionId = "v3"
aws iam get-policy-version \
  --policy-arn arn:aws:iam::123456789012:policy/MyCustomPolicy \
  --version-id v3 \
  --query 'PolicyVersion.Document' --output text
```

---

## B. Loading State

| ID | Story | Expected |
|----|-------|----------|
| B.1 | I press Enter on a policy in the Role Policies list. | A spinner (animated dot, blue #7aa2f7) is displayed centered inside the frame. The text reads "Fetching policy document..." The header shows "? for help" on the right. |
| B.2 | I press keys (j, k, /, c, w) while the spinner is visible. | No actions occur. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| B.3 | I press Esc while the spinner is visible. | I return to the Role Policies list. The fetch may or may not be cancelled, but I am not stuck in the loading state. |
| B.4 | I press Enter on a managed policy (requires two serial API calls: GetPolicy + GetPolicyVersion). | The spinner remains visible while both calls complete. There is no intermediate partial display. The document appears only after both calls succeed. |
| B.5 | I press Enter on an inline policy (requires one API call: GetRolePolicy). | The spinner is visible for a shorter time (single call). The document appears once the call completes. |

---

## C. Error States

| ID | Story | Expected |
|----|-------|----------|
| C.1 | The managed policy was deleted between listing and viewing (NoSuchEntity). | The spinner disappears. A red error message (#f7768e, bold) is displayed inside the frame: "Error: NoSuchEntity -- Policy arn:aws:iam::123456789012:policy/deleted-policy was not found." Below it, a dim hint reads "Press Esc to go back." |
| C.2 | My IAM credentials lack `iam:GetPolicy` or `iam:GetPolicyVersion` permission. | The spinner disappears. A red error message is displayed: "Error: AccessDenied -- unable to read policy document" (or similar). The "Press Esc to go back." hint is visible. |
| C.3 | My IAM credentials lack `iam:GetRolePolicy` permission for an inline policy. | Same behavior as C.2 -- red error message with access denied detail and Esc hint. |
| C.4 | I press Esc on the error state. | I return to the Role Policies list. The cursor is on the same policy row. |
| C.5 | A network timeout occurs during the GetPolicyVersion call. | A red error message describes the timeout. The "Press Esc to go back." hint is visible. No partial document is shown. |

**AWS comparison:**
```
aws iam get-policy --policy-arn arn:aws:iam::123456789012:policy/nonexistent
# Returns: NoSuchEntity error
aws iam get-policy-version --policy-arn arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess \
  --version-id v1
# With no iam:GetPolicyVersion permission: AccessDenied error
```

---

## D. Frame Title

| ID | Story | Expected |
|----|-------|----------|
| D.1 | I open a managed policy "AmazonS3ReadOnlyAccess" with default version v1. | The frame top border shows the title centered: "policy-doc --- AmazonS3ReadOnlyAccess (Managed v1)" with equal-length dashes on both sides. |
| D.2 | I open a managed policy "MyPolicy" with default version v3. | The frame title reads "policy-doc --- MyPolicy (Managed v3)". |
| D.3 | I open an inline policy "payment-service-custom-policy". | The frame title reads "policy-doc --- payment-service-custom-policy (Inline)". |
| D.4 | The frame title text is long (e.g., very long policy name). | The title is centered in the top border. If the title exceeds available space, it is truncated with "..." while keeping the frame border intact. |

---

## E. Header Metadata

### E.1 Managed Policy Metadata

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | I open a managed policy "AmazonS3ReadOnlyAccess". | Three metadata lines appear at the top of the frame content: (1) "Policy: AmazonS3ReadOnlyAccess", (2) "ARN: arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess", (3) "Version: v1 (default) -- 1 version(s)". |
| E.1.2 | Below the metadata lines. | A dim horizontal rule separator (dashes) spans the frame width, visually separating the metadata from the JSON body. |
| E.1.3 | The ARN is very long and the terminal width is less than 60 columns. | The ARN line is truncated with "..." to fit within the frame. |

### E.2 Inline Policy Metadata

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | I open an inline policy "payment-service-custom-policy" attached to role "payment-service-execution-role". | Two metadata lines appear: (1) "Policy: payment-service-custom-policy", (2) "Type: Inline Policy (attached to payment-service-execution-role)". There is NO ARN line and NO Version line. |
| E.2.2 | Below the metadata lines. | A dim horizontal rule separator is present, identical to the managed policy layout. |

---

## F. JSON Document Rendering

### F.1 Pretty-Printing

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | The policy document loads successfully. | The JSON is pretty-printed with 2-space indentation. Nested objects and arrays are properly indented. Every key-value pair is on its own line. |
| F.1.2 | I compare the rendered JSON against the AWS CLI output for the same policy. | The content is identical after URL-decoding and pretty-printing. All Statement blocks, Action arrays, Resource values, and Condition objects are present. |
| F.1.3 | The policy document is a single statement (not an array). | The JSON renders correctly with the Statement as an object, not wrapped in brackets. |
| F.1.4 | The policy document has deeply nested Condition blocks. | All nested conditions (StringEquals, ArnLike, etc.) are properly indented at the correct depth with 2-space indent per level. |

**AWS comparison:**
```
aws iam get-policy-version \
  --policy-arn arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess \
  --version-id v1 \
  --query 'PolicyVersion.Document'
# URL-decode the output (it comes URL-encoded)
# Pretty-print with 2-space indent
# The result should match what a9s displays
```

### F.2 Syntax Highlighting

| ID | Story | Expected |
|----|-------|----------|
| F.2.1 | The JSON document contains keys like "Version", "Statement", "Sid", "Effect", "Action", "Resource". | All JSON keys are rendered in blue (#7aa2f7). |
| F.2.2 | The document contains string values like "2012-10-17", "AllowS3Read", "s3:GetObject". | String values are rendered in green (#9ece6a). |
| F.2.3 | The document contains `"Effect": "Allow"`. | The value "Allow" is rendered in bright green (#73daca) with bold styling. This overrides the default green string color. |
| F.2.4 | The document contains `"Effect": "Deny"`. | The value "Deny" is rendered in bright red (#f7768e) with bold styling. This is the most critical visual signal in the view. |
| F.2.5 | The document contains `"Action": "*"` (wildcard action). | The value "*" is rendered in red (#f7768e) with bold styling, signaling overprivileged access. |
| F.2.6 | The document contains `"Resource": "*"` (wildcard resource). | The value "*" is rendered in red (#f7768e) with bold styling. |
| F.2.7 | The document contains an ARN string like `"arn:aws:s3:::my-bucket/*"`. | The ARN value is rendered in cyan (#7dcfff), distinguishing it from regular string values. |
| F.2.8 | The document contains `"arn:aws-cn:s3:::bucket"` or `"arn:aws-us-gov:s3:::bucket"`. | These are also rendered in cyan (#7dcfff) -- ARN detection works for all AWS partition prefixes. |
| F.2.9 | The document contains numeric values (e.g., in a Condition block). | Numbers are rendered in orange (#ff9e64). |
| F.2.10 | The document contains boolean values (e.g., `"aws:SecureTransport": "false"`). | Note: IAM policy conditions typically use string "true"/"false", not JSON booleans. If actual JSON booleans appear, they are rendered in purple (#bb9af7). |
| F.2.11 | The document contains structural punctuation: `{`, `}`, `[`, `]`, `,`. | These characters are rendered in dim (#565f89), keeping them visually subordinate to the semantic content. |
| F.2.12 | The document contains `null` (rare but possible). | null is rendered in dim (#565f89). |

**AWS comparison:**
```
aws iam get-policy-version \
  --policy-arn arn:aws:iam::aws:policy/AdministratorAccess \
  --version-id v1 --query 'PolicyVersion.Document'
# Expect: {"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}
# In a9s: "Allow" = bright green bold, "*" (both Action and Resource) = red bold
```

### F.3 Highlighting Priority

| ID | Story | Expected |
|----|-------|----------|
| F.3.1 | A search match overlaps with an "Allow" token. | The search match highlighting (amber/orange background) takes precedence over the "Allow" bright green. The matched text shows with amber background and dark foreground. |
| F.3.2 | A search match overlaps with an ARN string. | The search match highlighting overrides the cyan ARN color. |
| F.3.3 | An ARN string contains a `*` wildcard (e.g., `"arn:aws:s3:::bucket/*"`). | The entire string is rendered in cyan (#7dcfff) because ARN detection applies to the full string value. The `*` within the ARN is part of the cyan ARN rendering, not independently red bold. Only a standalone `"*"` value gets the red bold treatment. |
| F.3.4 | The key "Effect" is rendered next to the value "Allow". | The key "Effect" is in blue (#7aa2f7). The colon and whitespace are dim. The value "Allow" is bright green (#73daca) bold. Each token has its own color applied independently. |

---

## G. Scroll Navigation

| ID | Story | Expected |
|----|-------|----------|
| G.1 | The JSON document is longer than the visible viewport (e.g., a policy with 10 statements). | Only the portion that fits is shown. A dim scroll indicator appears at the bottom: "... N lines below" (dim #414868). |
| G.2 | I press j (or down-arrow). | The viewport scrolls down by one line. The metadata header at the top stays fixed above the scrollable JSON area. |
| G.3 | I press k (or up-arrow). | The viewport scrolls up by one line. |
| G.4 | I press g. | The viewport jumps to the top of the document (first line of JSON). |
| G.5 | I press G. | The viewport jumps to the bottom of the document (last line of JSON). |
| G.6 | I press PgDn. | The viewport scrolls down by one full page (viewport height). |
| G.7 | I press PgUp. | The viewport scrolls up by one full page. |
| G.8 | I am at the bottom of the document and press j. | Nothing happens -- the viewport does not scroll past the end. |
| G.9 | I am at the top of the document and press k. | Nothing happens -- the viewport does not scroll past the beginning. |
| G.10 | The entire document fits in the viewport without scrolling. | No scroll indicators appear. j/k are no-ops (no scroll needed). |
| G.11 | I scroll down partway through the document. | The scroll indicator updates: "... N lines above" at top and/or "... N lines below" at bottom. |

---

## H. In-Document Search

### H.1 Entering Search Mode

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | I press / while viewing the policy document. | The header right side changes from "? for help" to "/|" in amber (#e0af68) bold. A text cursor appears. I can type a search query. The JSON content is unchanged. |
| H.1.2 | I type "s3:GetObject" in the search input. | The header right side shows "/s3:GetObject|" in amber bold. The JSON content is still unchanged (search does not activate until Enter). |
| H.1.3 | I press Esc while the search input is active (before pressing Enter). | The search input is cleared. The header reverts to "? for help". No matches are highlighted. I return to normal mode. |
| H.1.4 | I press Enter to confirm the search. | Search mode activates. The header right side reverts to "? for help" (the search bar closes after confirmation). All occurrences of "s3:GetObject" in the document are highlighted. |

### H.2 Search Results

| ID | Story | Expected |
|----|-------|----------|
| H.2.1 | The search term "s3:GetObject" has 3 matches in the document. | All 3 occurrences are highlighted with amber background (#e0af68) and dark foreground (#1a1b26). The current match (first one) has an orange background (#ff9e64) and dark foreground, bold. A match indicator at the bottom of the frame shows "[1/3 matches]" in dim. |
| H.2.2 | I press n. | The current match advances to the second occurrence. The previous match reverts to amber background. The new current match gets orange background bold. The indicator updates to "[2/3 matches]". The viewport scrolls if the new match is off-screen, centering it vertically. |
| H.2.3 | I press n again. | The current match advances to the third occurrence. The indicator shows "[3/3 matches]". |
| H.2.4 | I press n at the last match (3/3). | The current match wraps around to the first occurrence. The indicator shows "[1/3 matches]". |
| H.2.5 | I press N (uppercase). | The current match moves to the previous occurrence. If I was on match 1, it wraps to match 3. |
| H.2.6 | I press Esc while search results are highlighted. | All search highlights are cleared. The match indicator disappears. I return to normal mode with the viewport position preserved. |
| H.2.7 | I press / while search results are active. | A new search input opens (replaces the current search). The previous highlights are cleared. I can type a new search term. |

### H.3 Search Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| H.3.1 | I search for a term that has zero matches (e.g., "dynamodb" in an S3-only policy). | No highlights appear. The match indicator shows "[0/0 matches]" or "No matches". The viewport position is unchanged. |
| H.3.2 | I search for "Effect" (a JSON key). | All occurrences of "Effect" are highlighted, including within key names. Search operates on the raw JSON text. |
| H.3.3 | I search for "allow" (lowercase). | Matches "Allow" because search is case-insensitive by default. |
| H.3.4 | I search for "*". | Matches all standalone wildcard `"*"` values in the document. |
| H.3.5 | The search match spans within a syntax-highlighted token (e.g., searching "Get" within "s3:GetObject"). | The matched portion ("Get") shows amber/orange background. The non-matched portion of the token retains its original syntax color. Search highlighting has highest priority. |
| H.3.6 | I press Enter on an empty search input. | No search is activated. The view returns to normal mode. |

**AWS comparison:**
```
aws iam get-policy-version \
  --policy-arn arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess \
  --version-id v1 --query 'PolicyVersion.Document' --output text | python3 -c "
import sys, json, urllib.parse
doc = json.loads(urllib.parse.unquote(sys.stdin.read()))
text = json.dumps(doc, indent=2)
count = text.lower().count('s3:getobject'.lower())
print(f'Found {count} matches for s3:GetObject')
"
```

---

## I. Copy Behavior

| ID | Story | Expected |
|----|-------|----------|
| I.1 | I press c while viewing the policy document in normal mode. | The entire pretty-printed JSON document is copied to the system clipboard. A green flash message "Copied!" (#9ece6a bold) appears in the header right side. |
| I.2 | The copied text does NOT contain ANSI escape codes. | When I paste into a plain text editor (e.g., Vim, VS Code, Notepad), the JSON is clean with no color codes. It is valid JSON that can be parsed by `jq` or `python -m json.tool`. |
| I.3 | After approximately 2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| I.4 | I press c while search highlights are active. | The full JSON document is still copied (not just matched portions). Flash message appears as normal. |
| I.5 | I paste the copied JSON and compare to the AWS CLI output. | The pasted JSON matches the URL-decoded, pretty-printed (2-space indent) document exactly. |

**AWS comparison:**
```
aws iam get-policy-version \
  --policy-arn arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess \
  --version-id v1 --query 'PolicyVersion.Document' --output text \
  | python3 -c "import sys,json,urllib.parse; print(json.dumps(json.loads(urllib.parse.unquote(sys.stdin.read())),indent=2))"
# The clipboard contents should match this output
```

---

## J. Word Wrap Toggle

| ID | Story | Expected |
|----|-------|----------|
| J.1 | I press w while viewing the policy document. | Long JSON lines that exceed the frame inner width now wrap at the frame boundary. Wrapped continuation lines are indented 4 spaces. A "[wrap]" indicator appears in the header or match indicator area. |
| J.2 | I press w again. | Word wrap is toggled off. Long lines are no longer wrapped -- they extend beyond the visible frame width (horizontal overflow). The "[wrap]" indicator disappears. |
| J.3 | A policy has a very long Resource ARN array on one line (e.g., after pretty-printing, an ARN like "arn:aws:s3:::very-long-bucket-name-for-production-environment-2024/*" exceeds the frame width). Word wrap is ON. | The long line wraps within the frame. The continuation is indented 4 spaces from the left margin. |
| J.4 | Word wrap is ON and I resize the terminal to a narrower width. | The wrap adjusts to the new frame boundary. Lines re-wrap at the new width. |
| J.5 | Word wrap state persists across scroll operations. | Scrolling with j/k/g/G/PgUp/PgDn respects the current wrap setting. Wrapped lines count as multiple display lines for scroll purposes. |

---

## K. Help Screen

| ID | Story | Expected |
|----|-------|----------|
| K.1 | I press ? while viewing the policy document. | The help screen replaces the frame content. The frame title changes to "Help" centered. A four-column layout is displayed with categories: POLICY DOCUMENT, GENERAL, NAVIGATION, HOTKEYS. |
| K.2 | The POLICY DOCUMENT column lists all view-specific bindings. | The column shows: `<esc>` Back, `<c>` Copy Doc, `<w>` Word Wrap, `</>` Search, `<n>` Next Match, `<N>` Prev Match. Keys are green (#9ece6a) bold. Descriptions are plain (#c0caf5). |
| K.3 | The GENERAL column lists global bindings. | The column shows at minimum: `<ctrl-r>` Refresh, `<q>` Quit. |
| K.4 | The NAVIGATION column lists scroll bindings. | The column shows: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<pgup/dn>` Page. |
| K.5 | The HOTKEYS column lists shortcut bindings. | The column shows: `<?>` Help, `<:>` Command. |
| K.6 | A dim message at the bottom reads "Press any key to close". | The message is visible below the four columns. |
| K.7 | I press any key (e.g., j, Esc, Space, Enter). | The help screen closes and the policy document view reappears with all state preserved (scroll position, search highlights if any). |

---

## L. Refresh

| ID | Story | Expected |
|----|-------|----------|
| L.1 | I press Ctrl+r while viewing the policy document. | The spinner appears (loading state). A fresh API call is made to re-fetch the policy document. When complete, the document is re-rendered with updated content. |
| L.2 | The policy was modified between the original load and the refresh. | The refreshed view shows the updated policy document. If the default version changed, the new version is displayed and the frame title reflects the new version. |
| L.3 | I had search highlights active and press Ctrl+r. | Search highlights are cleared during refresh. After the document reloads, I am in normal mode (no search active). |
| L.4 | I had word wrap enabled and press Ctrl+r. | After the document reloads, the word wrap setting is preserved (still ON). |

---

## M. Quit

| ID | Story | Expected |
|----|-------|----------|
| M.1 | I press q while viewing the policy document. | The application exits immediately. |
| M.2 | I press Ctrl+c while viewing the policy document. | The application force-quits immediately. |

---

## N. Command Mode

| ID | Story | Expected |
|----|-------|----------|
| N.1 | I press : while viewing the policy document. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| N.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. The entire IAM Roles --> Role Policies --> Policy Document context is left behind. |
| N.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The policy document view remains unchanged. |

---

## O. Responsive Behavior

### O.1 Terminal Width

| ID | Story | Expected |
|----|-------|----------|
| O.1.1 | Terminal width is less than 60 columns. | The ARN in the header metadata is truncated with "...". The JSON body is still rendered but long lines extend beyond the visible area (unless word wrap is on). |
| O.1.2 | Terminal width is 60-80 columns. | Full content is displayed. Some long JSON lines may not fit without word wrap enabled. |
| O.1.3 | Terminal width is 80-120 columns. | Comfortable viewing. All metadata and most JSON lines are visible without wrapping. |
| O.1.4 | Terminal width exceeds 120 columns. | Extra padding on the right. Content does not stretch to fill the full width. |

### O.2 Terminal Height

| ID | Story | Expected |
|----|-------|----------|
| O.2.1 | Terminal height is less than 10 rows. | Only the JSON body is visible. The metadata header is hidden to prioritize showing the document content. |
| O.2.2 | Terminal height is 10-20 rows. | Metadata + a few JSON lines are visible. Scroll indicators show how many lines are above/below. |
| O.2.3 | Terminal height exceeds 20 rows. | Full view. Most policy documents (single or few statements) fit without scrolling. |

### O.3 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| O.3.1 | I resize the terminal while viewing the policy document. | The layout reflows. The frame border redraws at the new width. If word wrap is on, lines re-wrap at the new boundary. Scroll position is preserved as closely as possible. |
| O.3.2 | I resize below the minimum terminal dimensions (< 60 cols or < 7 rows). | An error message appears: "Terminal too narrow. Please resize." or "Terminal too short. Please resize." |

---

## P. View Stack (Grandchild Navigation)

| ID | Story | Expected |
|----|-------|----------|
| P.1 | Main Menu --> IAM Roles --> select role --> Enter --> Role Policies --> select managed policy --> Enter --> Policy Document --> Esc. | I return to the Role Policies list. The cursor is on the same policy. |
| P.2 | ...continuing from P.1: Esc again. | I return to the IAM Roles list. The cursor is on the same role. |
| P.3 | ...continuing from P.2: Esc again. | I return to the Main Menu. |
| P.4 | Main Menu --> IAM Roles --> Role Policies --> Policy Document (inline) --> Esc --> Esc --> Esc. | Full three-level unwind works correctly. Each Esc pops exactly one level. No state is lost at any intermediate level. |
| P.5 | I open Policy Document, then use : to navigate to S3, then Esc from S3. | I return to the Main Menu (the command navigated away from the IAM context entirely). The IAM Roles --> Role Policies --> Policy Document stack is no longer in memory. |

---

## Q. Empty Document

| ID | Story | Expected |
|----|-------|----------|
| Q.1 | The policy document is empty (should not happen in practice, but defensive). | The metadata is displayed normally. Below the separator, a dim message reads "Policy document is empty." No JSON is rendered. |

---

## R. Real-World Scenarios

### R.1 The 10-Policy Role (Finding a Specific Permission)

| ID | Story | Expected |
|----|-------|----------|
| R.1.1 | A role "data-pipeline-role" has 12 attached policies. I need to find which one grants `s3:GetObject`. I navigate: IAM Roles --> Enter on data-pipeline-role --> Role Policies (12 shown). | The Role Policies list shows all 12 policies. I can see Policy Name, Policy ARN, and Type columns. |
| R.1.2 | I press Enter on the first policy. The document opens. I press / and type "s3:GetObject" and press Enter. | If "s3:GetObject" exists in this policy, matches are highlighted with amber/orange background. The match indicator shows the count (e.g., "[1/2 matches]"). If it does not exist, the indicator shows "[0/0 matches]" or "No matches". |
| R.1.3 | I press Esc (to clear search), then Esc (back to policies list). I select the next policy and press Enter. I search again with /s3:GetObject. | I can rapidly iterate through policies, searching each for the target permission. |
| R.1.4 | On the 4th policy, the search finds 2 matches for "s3:GetObject". I press n to navigate between them. | The first match is highlighted in orange. Pressing n moves to the second match (also orange), and the viewport scrolls to center it. I can inspect the Resource and Condition context around each match. |

**AWS comparison:**
```
# Without a9s, finding which of 12 policies grants s3:GetObject requires:
aws iam list-attached-role-policies --role-name data-pipeline-role
# Then for EACH policy:
aws iam get-policy --policy-arn POLICY_ARN
aws iam get-policy-version --policy-arn POLICY_ARN --version-id vN \
  --query 'PolicyVersion.Document' --output text | python3 -c "..." | grep s3:GetObject
# a9s reduces this from ~24+ CLI commands to Enter + / + type + Enter, repeated per policy
```

### R.2 The Wildcard Mismatch (Debugging Access Denied)

| ID | Story | Expected |
|----|-------|----------|
| R.2.1 | A policy grants `s3:GetObject` on `arn:aws:s3:::my-bucket/prod/*` but the application is requesting `/staging/file.txt` and getting AccessDenied. I open the policy document. | The document renders with syntax highlighting. The Resource ARN `"arn:aws:s3:::my-bucket/prod/*"` is displayed in cyan (#7dcfff). |
| R.2.2 | I search for "my-bucket" using /. | The search highlights all occurrences of "my-bucket" in the document. I can see the exact Resource pattern and notice it only covers `/prod/*`, not `/staging/*`. |
| R.2.3 | The Resource array is clearly visible with correct indentation. | I can immediately see: `"Resource": ["arn:aws:s3:::my-bucket/prod/*"]` -- the path restriction is obvious in the cyan-highlighted ARN. |

**AWS comparison:**
```
aws iam get-policy-version \
  --policy-arn arn:aws:iam::123456789012:policy/s3-read-policy \
  --version-id v1 --query 'PolicyVersion.Document' --output text \
  | python3 -c "import sys,json,urllib.parse; print(json.dumps(json.loads(urllib.parse.unquote(sys.stdin.read())),indent=2))" \
  | grep "my-bucket"
# Shows: "arn:aws:s3:::my-bucket/prod/*"
```

### R.3 The Forgotten Inline Deny (Incident Archaeology)

| ID | Story | Expected |
|----|-------|----------|
| R.3.1 | A role has an inline policy "emergency-deny-2023" added during an incident 3 years ago. The inline policy contains a Deny statement. I open it from the Role Policies list. | The inline policy metadata shows: "Policy: emergency-deny-2023" and "Type: Inline Policy (attached to role-name)". |
| R.3.2 | The JSON document contains `"Effect": "Deny"`. | "Deny" is immediately visible in bright red (#f7768e) bold. This jumps out visually against the blue keys and green strings. There is no way to miss it. |
| R.3.3 | The Deny statement also has `"Resource": "*"`. | The wildcard `"*"` is also rendered in red (#f7768e) bold, doubly emphasizing the severity -- this Deny applies to ALL resources. |
| R.3.4 | Without syntax highlighting, I would need to carefully read every line. | With highlighting, I immediately see red bold tokens and know this is a Deny-all policy without reading every word. |

**AWS comparison:**
```
aws iam get-role-policy \
  --role-name my-role \
  --policy-name emergency-deny-2023 \
  --query 'PolicyDocument' --output text
# Raw output is URL-encoded and unformatted -- hard to visually scan for Deny
```

### R.4 AdministratorAccess Audit (Security Review)

| ID | Story | Expected |
|----|-------|----------|
| R.4.1 | During a security review, I need to confirm what "AdministratorAccess" actually grants. I find the role with this policy, navigate to Role Policies, select "AdministratorAccess" (Type: Managed), and press Enter. | The policy document opens. The frame title shows "policy-doc --- AdministratorAccess (Managed v1)". |
| R.4.2 | The AdministratorAccess policy document renders. | The JSON shows: `"Effect": "Allow"` (bright green bold), `"Action": "*"` (red bold), `"Resource": "*"` (red bold). The two red bold wildcards make it immediately clear this policy grants FULL access to everything. |
| R.4.3 | I press c to copy the document. | The full JSON is copied to clipboard for inclusion in a security audit report. The "Copied!" flash confirms the action. |
| R.4.4 | I paste the JSON into a report. | Clean JSON without ANSI codes: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}` (pretty-printed with 2-space indent). |

**AWS comparison:**
```
aws iam get-policy-version \
  --policy-arn arn:aws:iam::aws:policy/AdministratorAccess \
  --version-id v1 --query 'PolicyVersion.Document' --output text
# Returns URL-encoded: %7B%22Version%22%3A%222012-10-17%22%2C%22Statement%22%3A%5B%7B%22Effect%22%3A%22Allow%22%2C%22Action%22%3A%22%2A%22%2C%22Resource%22%3A%22%2A%22%7D%5D%7D
# Must URL-decode to read: {"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}
# a9s does this decoding + pretty-printing + syntax highlighting automatically
```

---

## S. Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| S.1 | In the Policy Document view, the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms the same header format as all other views. |
| S.2 | The header right side shows "? for help" in normal mode. | Confirmed in dim (#565f89). |
| S.3 | The header right side shows "/search-text|" when search input is active. | Confirmed in amber (#e0af68) bold. |
| S.4 | The header right side shows "Copied!" after pressing c. | Confirmed in green (#9ece6a) bold, auto-clears after approximately 2 seconds. |
| S.5 | The header right side shows an error flash if the fetch fails. | Confirmed in red (#f7768e) bold. |

---

## T. Read-Only Safety

| ID | Story | Expected |
|----|-------|----------|
| T.1 | I interact with the Policy Document view using every available key binding (j, k, g, G, PgUp, PgDn, /, n, N, c, w, ?, Esc, Ctrl+r, :, q). | None of these actions modify the policy in AWS. All operations are read-only. No IAM write API calls (PutRolePolicy, CreatePolicyVersion, etc.) are ever made. |
| T.2 | I press c (copy) on an AdministratorAccess policy. | Only the clipboard is written to. The AWS policy itself is not modified. |
| T.3 | I press Ctrl+r (refresh) on the policy document. | Only read API calls are made (GetPolicy, GetPolicyVersion, or GetRolePolicy). No modifications to the policy. |

---

## U. Key Binding Coverage Summary

Every key binding from the design spec is covered in at least one story:

| Key | Stories |
|-----|---------|
| `Esc` | A.1.3, A.2.3, B.3, C.4, H.1.3, H.2.6, N.3, P.1-P.4 |
| `j` / down-arrow | G.2, G.8 |
| `k` / up-arrow | G.3, G.9 |
| `g` | G.4 |
| `G` | G.5 |
| `PgDn` | G.6 |
| `PgUp` | G.7 |
| `/` | H.1.1, H.1.2, H.2.7 |
| `Enter` (search) | H.1.4, H.3.6 |
| `n` | H.2.2, H.2.3, H.2.4 |
| `N` | H.2.5 |
| `c` | I.1, I.2, I.4, R.4.3 |
| `w` | J.1, J.2, J.3, J.4, J.5 |
| `?` | K.1-K.7 |
| `Ctrl+r` | L.1-L.4 |
| `q` | M.1 |
| `Ctrl+c` | M.2 |
| `:` | N.1-N.3 |

# IAM Identity View Design Specification

Issue: #77
Version: 1.0
Target: Lipgloss v2 + Bubble Tea v2 + Bubbles v2

---

## 1. Overview

Two changes to the a9s TUI:

1. **Header enrichment** -- add account alias (or account ID) and role name
   to the always-visible header line.
2. **Identity detail view** -- a new view triggered by `i` that shows full IAM
   caller identity, session details, and credential source. Follows the existing
   help-screen pattern: replaces the frame content, dismissed with `esc` or any key.

---

## 2. Design Decisions

### 2.1 Key Binding: `i` (lowercase)

| Considered | Verdict | Reason |
|------------|---------|--------|
| `I` | Rejected | Already bound to "sort by ID" in resource list views |
| `W` | Rejected | Already bound to "toggle wrap" in detail/YAML views |
| `i` | **Chosen** | Free across all views. Mnemonic for "identity". Lowercase is natural for a read-only info screen (like `d` for detail, `y` for yaml). |
| `ctrl+i` | Rejected | Equivalent to Tab in many terminals -- unreliable |

The `i` key is **global** -- it works from any view (main menu, resource list,
detail, YAML, selectors). This is consistent with `?` for help being global.

### 2.2 View Type: Frame-content Replacement (Like Help)

The identity view replaces the frame content, same as the help screen. Rationale:

- It is not a resource view (no list, no scrolling, no child navigation).
- It is informational and transient -- the user glances at it and dismisses.
- A modal overlay would add visual complexity for something rarely referenced.
- The help screen established this pattern and users already know it.

Dismissal: `esc` or any key (matches help screen behavior).

### 2.3 Header Format

Current header:

```
 a9s v3.11.0  prod-admin:us-east-1                              ? for help
```

New header with account context and role name:

```
 a9s v3.11.0  prod-admin:us-east-1 (acme-prod) admin-role       ? for help
```

Fallback when no alias is available:

```
 a9s v3.11.0  prod-admin:us-east-1 (123456789012) admin-role    ? for help
```

IAM user (no role):

```
 a9s v3.11.0  my-dev:us-east-1 (111222333444) deploy-bot@example.com  ? for help
```

Rules:
- Account context appears in parentheses after profile:region.
- Role name (or IAM user name) follows the account badge.
- If both alias and account ID are available, show alias (shorter). Full ID is
  in the detail view.
- If the terminal is narrower than 80 columns, the account context and role
  name are omitted (same truncation strategy as "? for help" being omitted on
  narrow terminals).

### 2.4 What NOT to Show

Per the DevOps requirements:
- No permissions/policies (too complex, unreliable via STS alone)
- No Access Key ID (security risk -- never display credentials)
- No User ID (STS user ID is almost never useful to operators)

---

## 3. Header Modification

### 3.1 Layout

The header gains one new element: the account context badge, inserted between
the profile:region and the gap.

```
 ┌─ accent bold    ┌─ dim            ┌─ bold            ┌─ dim            ┌─ dim        gap    ┌─ right content
 │                 │                 │                  │                 │                     │
 a9s               v3.11.0           prod:us-east-1     (acme-prod)       admin-role            ? for help
```

Composition (pseudo-Go):

```go
left  := accentStyle.Render("a9s") +
         dimStyle.Render(" v"+version) +
         boldStyle.Render("  "+profile+":"+region) +
         accountBadge +  // dimStyle.Render(" ("+alias+")")
         roleBadge       // dimStyle.Render(" "+roleName)
right := dimStyle.Render("? for help")
gap   := (w - 2) - lipgloss.Width(left) - lipgloss.Width(right)
```

### 3.2 Account Badge Styles

| Element | Foreground | Background | Style |
|---------|------------|------------|-------|
| Account badge (alias) | `#565f89` (ColDim) | -- | -- |
| Account badge (ID fallback) | `#565f89` (ColDim) | -- | -- |

The badge is intentionally dim -- it provides context without competing with the
profile:region for visual attention.

### 3.3 Narrow Terminal Behavior

| Terminal Width | Account Badge | Role Name | Help Hint |
|----------------|---------------|-----------|-----------|
| >= 100 cols | Shown | Shown | Shown |
| 80-99 cols | Shown | Shown | Shown |
| 60-79 cols | **Omitted** | **Omitted** | May be omitted |
| < 60 cols | Omitted | Omitted | Omitted |

The account badge and role name are the first things to drop when space is tight,
since the full info is available via `i`.

### 3.5 ASCII Wireframes -- Header Variants

Assumed role (120 cols, alias available):

```
 a9s v3.11.0  prod-admin:us-east-1 (acme-prod) admin-role                                             ? for help
```

Assumed role (120 cols, no alias):

```
 a9s v3.11.0  staging-admin:eu-west-1 (555666777888) AccountAccessRole                            ? for help
```

IAM user (120 cols, no alias):

```
 a9s v3.11.0  my-dev:us-east-1 (111222333444) deploy-bot@example.com                                  ? for help
```

Narrow terminal (70 cols, badge + role omitted):

```
 a9s v3.11.0  prod-admin:us-east-1                            ? for help
```

Filter active (badge + role stay):

```
 a9s v3.11.0  prod-admin:us-east-1 (acme-prod) admin-role                                             /running█
```

Flash message (badge + role stay):

```
 a9s v3.11.0  prod-admin:us-east-1 (acme-prod) admin-role                                             Copied!
```

---

## 4. Identity Detail View

### 4.1 View Structure

The identity view replaces the frame content (like the help screen). The frame
title is "Identity". Content is a key-value layout using the existing detail
view styling conventions.

### 4.2 ASCII Wireframe -- Identity View

```
 a9s v3.11.0  prod-admin:us-east-1 (acme-prod) admin-role                                      ? for help
┌──────────────────────────────────── Identity ──────────────────────────────────────────────────────────┐
│                                                                                                       │
│  Account:                                                                                             │
│      Alias:                acme-prod                                                                  │
│      Account ID:           123456789012                                                               │
│                                                                                                       │
│  Caller:                                                                                              │
│      ARN:                  arn:aws:sts::123456789012:assumed-role/admin-role/session-name              │
│      Role:                 admin-role                                                                 │
│      Session:              session-name                                                               │
│                                                                                                       │
│  Session:                                                                                             │
│      Expiry:               2024-01-15 14:30:00 UTC (12m remaining)                                    │
│      Profile:              prod-admin                                                                 │
│      Region:               us-east-1                                                                  │
│      Credential Source:    SSO                                                                        │
│                                                                                                       │
│                                                                                                       │
│                                   c copy ARN  esc/any close                                           │
│                                                                                                       │
└───────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### 4.3 ASCII Wireframe -- IAM User (Not Assumed Role)

```
 a9s v3.11.0  dev:us-west-2 (123456789012) deploy-bot                                            ? for help
┌──────────────────────────────────── Identity ──────────────────────────────────────────────────────────┐
│                                                                                                       │
│  Account:                                                                                             │
│      Alias:                --                                                                         │
│      Account ID:           123456789012                                                               │
│                                                                                                       │
│  Caller:                                                                                              │
│      ARN:                  arn:aws:iam::123456789012:user/deploy-bot                                  │
│      User:                 deploy-bot                                                                 │
│                                                                                                       │
│  Session:                                                                                             │
│      Expiry:               -- (no session token)                                                      │
│      Profile:              dev                                                                        │
│      Region:               us-west-2                                                                  │
│      Credential Source:    profile                                                                    │
│                                                                                                       │
│                                                                                                       │
│                                   c copy ARN  esc/any close                                           │
│                                                                                                       │
└───────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### 4.4 ASCII Wireframe -- Loading State

If the STS GetCallerIdentity call has not completed yet:

```
┌──────────────────────────────────── Identity ──────────────────────────────────────────────────────────┐
│                                                                                                       │
│                                                                                                       │
│                      ⠿ Fetching identity...                                                           │
│                                                                                                       │
│                                                                                                       │
└───────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

### 4.5 ASCII Wireframe -- Error State

If credentials are invalid or expired:

```
┌──────────────────────────────────── Identity ──────────────────────────────────────────────────────────┐
│                                                                                                       │
│                                                                                                       │
│  Error: Unable to locate credentials (NoCredentialProviders)                                          │
│                                                                                                       │
│  Profile:              prod-admin                                                                     │
│  Region:               us-east-1                                                                      │
│                                                                                                       │
│                                   esc/any close                                                       │
│                                                                                                       │
└───────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

Even on error, profile and region are shown (they come from config, not STS).

---

## 5. Color Palette (Identity View Specific)

All colors are from the existing Tokyo Night Dark palette. No new colors are introduced.

| Element | Foreground | Background | Style | Palette Var |
|---------|------------|------------|-------|-------------|
| Section header ("Account:", "Caller:", "Session:") | `#e0af68` | -- | Bold | `ColDetailSec` |
| Key label ("Alias:", "ARN:", etc.) | `#7aa2f7` | -- | -- | `ColDetailKey` |
| Value (normal text) | `#c0caf5` | -- | -- | `ColDetailVal` |
| Value (no session token) | `#565f89` | -- | Dim | `ColDim` |
| Hint line ("c copy ARN  esc/any close") | `#565f89` | -- | -- | `ColDim` |
| Hint keys ("c", "esc/any") | `#9ece6a` | -- | Bold | `ColHelpKey` |
| Account badge in header | `#565f89` | -- | -- | `ColDim` |
| Role name in header | `#565f89` | -- | -- | `ColDim` |
| Error text | `#f7768e` | -- | Bold | `ColError` |
| Loading spinner | `#7aa2f7` | -- | -- | `ColSpinner` |

---

## 6. Layout Composition

### 6.1 Identity View Content Layout

```go
lipgloss.JoinVertical(lipgloss.Left,
    header,     // renderHeader() -- existing, with account badge added
    frameBox,   // renderFrame() with title="Identity", content=identityContent
)
```

Identity content layout (inside the frame):

```go
// Section: Account
secStyle.Render("Account:")
"     " + kStyle.Render(pad("Alias:", kw)) + vStyle.Render(alias)
"     " + kStyle.Render(pad("Account ID:", kw)) + vStyle.Render(accountID)

// Section: Caller
secStyle.Render("Caller:")
"     " + kStyle.Render(pad("ARN:", kw)) + vStyle.Render(arn)
"     " + kStyle.Render(pad("Role:", kw)) + vStyle.Render(roleName)   // or "User:" for IAM users
"     " + kStyle.Render(pad("Session:", kw)) + vStyle.Render(sessName)  // omit for IAM users

// Section: Session
secStyle.Render("Session:")
"     " + kStyle.Render(pad("Expiry:", kw)) + expiryStyle.Render(expiryText)
"     " + kStyle.Render(pad("Profile:", kw)) + vStyle.Render(profile)
"     " + kStyle.Render(pad("Region:", kw)) + vStyle.Render(region)
"     " + kStyle.Render(pad("Credential Source:", kw)) + vStyle.Render(credSource)
```

Key column width (`kw`): 22 characters (matches existing detail view convention).

### 6.2 Hint Line at Bottom

Centered at the bottom of the content area, using the same pattern as the help
screen's "Press any key to close" hint:

```go
lipgloss.Place(innerW, 1, lipgloss.Center, lipgloss.Top,
    hkStyle.Render("c") + dimStyle.Render(" copy ARN  ") +
    hkStyle.Render("esc/any") + dimStyle.Render(" close"))
```

---

## 7. Key Bindings

### 7.1 New Global Key Binding

| Key | Action | Context | Notes |
|-----|--------|---------|-------|
| `i` | Open identity view | All views | Replaces frame content, like `?` for help |

### 7.2 Identity View Key Bindings

| Key | Action | Notes |
|-----|--------|-------|
| `c` | Copy full ARN to clipboard | Flash "Copied!" in header |
| `esc` | Close identity view | Return to previous view |
| any other key | Close identity view | Same dismiss behavior as help |

### 7.3 Updated keys.Map Struct

```go
// Add to keys.Map struct:
Identity key.Binding

// Add to Default():
Identity: key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "identity")),
```

### 7.4 Updated Help Screen

The `i` key should appear in the "OTHER" or "HOTKEYS" column of the help screen
for all view contexts:

```
OTHER
...
<i>      Identity
<?>      Help
```

---

## 8. State Transitions

### 8.1 New Msg Types

| Msg Type | Purpose |
|----------|---------|
| `IdentityLoadedMsg` | STS GetCallerIdentity + IAM ListAccountAliases completed successfully |
| `IdentityErrorMsg` | STS call failed (no credentials, expired, network error) |

### 8.2 Transition Table

| Msg | From State | To State | Notes |
|-----|------------|----------|-------|
| `KeyMsg(i)` | Any non-input view | IdentityView (loading) | Push view stack, fire STS+IAM calls |
| `IdentityLoadedMsg` | IdentityView (loading) | IdentityView (loaded) | Populate fields, also update header badge |
| `IdentityErrorMsg` | IdentityView (loading) | IdentityView (error) | Show error + profile/region |
| `KeyMsg(c)` | IdentityView (loaded) | Flash "Copied!" | Copy ARN to clipboard |
| `KeyMsg(esc)` | IdentityView | Previous view | Pop view stack |
| `KeyMsg(any)` | IdentityView | Previous view | Pop view stack (except `c`) |

### 8.3 Identity Data Lifecycle

The identity is fetched **once** per profile+region combination and cached. It
is re-fetched when:
- The user switches profiles (`ProfileSelectedMsg`)
- The user switches regions (`RegionSelectedMsg`)
- The user explicitly refreshes with `ctrl+r` while in the identity view

The header badge is populated asynchronously on app startup. If the STS call
fails, the header badge simply shows nothing (graceful degradation).

---

## 9. Responsive Behavior

### 9.1 Header -- Width Breakpoints

| Width | Account Badge | Role Name | Help Hint |
|-------|---------------|-----------|-----------|
| >= 100 | `(acme-prod)` | `admin-role` | `? for help` |
| 80-99 | `(acme-prod)` | `admin-role` | `? for help` |
| 60-79 | omitted | omitted | may be omitted |
| < 60 | omitted | omitted | omitted |

### 9.2 Identity View -- Width Breakpoints

| Width | Behavior |
|-------|----------|
| >= 80 | Full layout as wireframed |
| 60-79 | ARN value may wrap or truncate with ellipsis |
| < 60 | "Terminal too narrow" error (existing behavior) |

The ARN is the only value likely to exceed available width. At 80 columns,
innerW = 78, with 1-space indent + 5-space sub-indent + 22-char key column =
50 chars for the value. A typical assumed-role ARN is ~70 chars, so it will
truncate with `...` on 80-col terminals. On 120+ col terminals it fits fully.

### 9.3 Identity View -- Height

The identity view needs roughly 18 lines of content. At minimum terminal height
(7 lines), only the first few fields will be visible (no scroll -- the view is
compact enough that this is acceptable). At 20+ lines, everything fits.

---

## 10. Bubbles Components

| Component | Bubbles Module | Notes |
|-----------|---------------|-------|
| Loading spinner | `bubbles/spinner` | Dot spinner, `#7aa2f7`, shown while STS call is in-flight |
| Content rendering | custom | Key-value layout using lipgloss styles, same as detail view |
| Clipboard | existing `Copy` infrastructure | Reuse the existing copy-to-clipboard mechanism |

No new bubbles components are needed. The identity view is simpler than the help
screen -- it is static key-value content with no columns or pagination.

---

## 11. AWS API Calls

### 11.1 Required Calls

| API | SDK Call | Purpose |
|-----|----------|---------|
| STS | `GetCallerIdentity` | Account ID, ARN, User ID (User ID is fetched but not displayed) |
| IAM | `ListAccountAliases` | Account alias (may be empty) |

### 11.2 Credential Source Detection

The credential source is determined by inspecting the environment and config:

| Condition | Credential Source Label |
|-----------|----------------------|
| `AWS_SESSION_TOKEN` env var set | "environment" |
| `sso_start_url` in profile config | "SSO" |
| `role_arn` + `source_profile` in config | "assumed-role (profile)" |
| `AWS_ACCESS_KEY_ID` env var set | "environment" |
| Default credential chain resolves | "profile" |

### 11.3 Error Handling

| Error | Behavior |
|-------|----------|
| No credentials configured | Show error view, header badge empty |
| Expired credentials | Show error view |
| Network timeout | Show error view, suggest `ctrl+r` to retry |
| IAM ListAccountAliases denied | Graceful degradation: alias shows "--", account ID still shown |

---

## 12. Copyable Content

| Key | What is Copied | Flash Message |
|-----|----------------|---------------|
| `c` (in identity view) | Full ARN string | "Copied!" |

Only the ARN is copyable because it is the value most commonly needed for
pasting into other tools (IAM policies, CLI commands, sharing with teammates).

---

## 13. Data Flow Summary

```
App startup
  └─> async: STS GetCallerIdentity + IAM ListAccountAliases
       ├─> success: cache identity, update header badge
       └─> failure: no header badge (silent)

User presses "i"
  └─> push IdentityView onto view stack
       ├─> if cached: render immediately
       └─> if not cached: show spinner, fire STS+IAM calls
            ├─> IdentityLoadedMsg: render content, cache
            └─> IdentityErrorMsg: render error

User presses "c" (in identity view)
  └─> copy ARN to clipboard, flash "Copied!" in header

User presses esc / any other key
  └─> pop view stack, return to previous view
```

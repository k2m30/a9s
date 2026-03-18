# a9s TUI Rewrite -- Task Breakdown

> Rewriting the TUI layer from `internal/app` (monolith) to `internal/tui` (modular).
> Each task follows TDD: write failing tests first, then implement, then verify.
> Status tracking: mark `[x]` when a task passes its Accept criteria.

---

## Progress Tracker

### Wave 1 -- Foundation (parallel: A, B, C)
- [ ] W1.A.1 RenderFrame constructs manual box with centered title
- [ ] W1.A.2 RenderHeader composes accent/dim/bold left + right-aligned content
- [ ] W1.A.3 PadOrTrunc handles ANSI-aware truncation with ellipsis
- [ ] W1.A.4 CenterTitle produces top border line with title between corners
- [ ] W1.B.1 Palette completeness audit against design spec
- [ ] W1.B.2 RowColorStyle coverage for all status strings
- [ ] W1.B.3 Composed styles audit (missing styles, NO_COLOR correctness)
- [ ] W1.C.1 Fix ClientsReadyMsg to carry ServiceClients pointer
- [ ] W1.C.2 Add missing message types (RefreshMsg, SortMsg)
- [ ] W1.C.3 Verify keys.Map completeness against design spec

### Wave 2 -- Views (parallel: D, E, F)
- [ ] W2.D.1 app.go View() composes RenderHeader + RenderFrame
- [ ] W2.D.2 handleNavigate creates target view, sets size, pushes stack
- [ ] W2.D.3 connectAWS stores ServiceClients on model via ClientsReadyMsg
- [ ] W2.D.4 Load ViewsConfig at startup and pass to child views
- [ ] W2.E.1 ResourceListModel.View renders column headers with sort indicator
- [ ] W2.E.2 ResourceListModel.View renders status-colored rows with selection
- [ ] W2.E.3 ResourceListModel.View handles horizontal scroll offset
- [ ] W2.E.4 ResourceListModel.View handles empty/error states
- [ ] W2.F.1 MainMenuModel.View renders items with cursor and dimmed alias
- [ ] W2.F.2 DetailModel.renderContent builds styled key-value from ViewDef
- [ ] W2.F.3 YAMLModel.renderContent marshals RawStruct and colorizes
- [ ] W2.F.4 HelpModel.View renders 4-column keybinding layout
- [ ] W2.F.5 ProfileModel.View renders list with cursor and "(current)" mark
- [ ] W2.F.6 RegionModel.View renders list with cursor and "(current)" mark
- [ ] W2.F.7 RevealModel.View renders secret with title, separator, metadata

### Wave 3 -- Wiring (sequential: G)
- [ ] W3.G.1 executeCommand dispatches colon-commands to NavigateMsg
- [ ] W3.G.2 Copy action reads resource ID to clipboard, sends FlashMsg
- [ ] W3.G.3 Reveal action fetches secret value, pushes RevealModel
- [ ] W3.G.4 Refresh action re-fetches current resource list from AWS
- [ ] W3.G.5 Switch entrypoint from internal/app to internal/tui in cmd/a9s/main.go

### Wave 4 -- Tests (sequential: H)
- [ ] W4.H.1 Rewrite layout_test.go for new layout package
- [ ] W4.H.2 Rewrite navigation_test.go for view stack push/pop
- [ ] W4.H.3 Rewrite filter_test.go and filter_ui_test.go
- [ ] W4.H.4 Rewrite detail_test.go and detail_config_test.go
- [ ] W4.H.5 Rewrite views_test.go and views_coverage_test.go
- [ ] W4.H.6 Rewrite app_test.go for new root Model
- [ ] W4.H.7 Audit and port remaining tests (qa_*, s3_*, bugs_test.go)

---

## Wave 1 -- Foundation

### W1.A.1 RenderFrame constructs manual box with centered title
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/layout/frame.go`, `/Users/k2m30/projects/a9s/tests/unit/layout_frame_test.go`
- **Do:**
  1. Write tests first in `layout_frame_test.go`:
     - `TestRenderFrame_BasicBox`: 5 content lines, w=40, h=8, title="test(5)". Assert top border contains centered title between dashes, content rows flanked by `|`, bottom border is `+---...---+`. Measure output is exactly h lines tall with each line exactly w visible columns.
     - `TestRenderFrame_EmptyContent`: 0 content lines, verify empty rows are padded to fill height.
     - `TestRenderFrame_TitleLongerThanWidth`: title exceeds available dash space, verify graceful truncation (minimum 1 dash each side).
     - `TestRenderFrame_NoTitle`: empty title string, verify plain top border with no spaces.
  2. Implement `RenderFrame(lines []string, title string, w, h int) string`:
     - Build top border: `borderStyle.Render("+" + leftDashes + " ") + titleStyle.Render(title) + borderStyle.Render(" " + rightDashes + "+")`
     - Use Unicode box-drawing chars: corners `\u250c \u2510 \u2514 \u2518`, horizontal `\u2500`, vertical `\u2502`.
     - Each content row: `borderStyle.Render("|") + PadOrTrunc(line, innerW) + borderStyle.Render("|")` where `innerW = w - 2`.
     - Pad with blank rows if `len(lines) < h - 2` (h minus top and bottom border).
     - Bottom border: `borderStyle.Render("+" + strings.Repeat("-", w-2) + "+")`.
     - Use `styles.ColBorder` for border style, `lipgloss.NewStyle().Foreground(styles.ColHeaderFg).Bold(true)` for title.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestRenderFrame -count=1` passes all four sub-tests
- **Blocked by:** none

### W1.A.2 RenderHeader composes accent/dim/bold left + right-aligned content
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/layout/frame.go`, `/Users/k2m30/projects/a9s/tests/unit/layout_header_test.go`
- **Do:**
  1. Write tests first in `layout_header_test.go`:
     - `TestRenderHeader_NormalMode`: profile="prod", region="us-east-1", version="0.5.0", w=80, rightContent=dim("? for help"). Assert output is 1 line, exactly w visible columns. Assert contains "a9s", "v0.5.0", "prod:us-east-1", "? for help".
     - `TestRenderHeader_NarrowWidth`: w=50, verify right content is omitted or gap is at minimum 1.
     - `TestRenderHeader_EmptyProfileRegion`: verify defaults display gracefully (no ":").
     - `TestRenderHeader_FlashMessage`: rightContent is green "Copied!", verify it appears right-aligned.
  2. Implement `RenderHeader(profile, region, version string, w int, rightContent string) string`:
     - Left: `accentStyle.Render("a9s") + dimStyle.Render(" v"+version) + boldStyle.Render("  "+profile+":"+region)`.
     - Compute gap: `(w - 2) - lipgloss.Width(left) - lipgloss.Width(rightContent)`. Clamp to minimum 1.
     - Return `left + strings.Repeat(" ", gap) + rightContent` wrapped in `lipgloss.NewStyle().Width(w).Padding(0, 1)`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestRenderHeader -count=1` passes
- **Blocked by:** none

### W1.A.3 PadOrTrunc handles ANSI-aware truncation with ellipsis
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/layout/frame.go`, `/Users/k2m30/projects/a9s/tests/unit/layout_padtrunc_test.go`
- **Do:**
  1. Write tests first in `layout_padtrunc_test.go`:
     - `TestPadOrTrunc_Padding`: input "hello" w=10, assert output is "hello     " (5 trailing spaces).
     - `TestPadOrTrunc_ExactFit`: input "hello" w=5, assert output is "hello" unchanged.
     - `TestPadOrTrunc_TruncateWithEllipsis`: input "hello world" w=8, assert output ends with `...` and `lipgloss.Width(output) == 8`.
     - `TestPadOrTrunc_ANSIAware`: input is ANSI-styled "hello" (e.g., bold), w=10, verify padding respects visible width not byte length.
     - `TestPadOrTrunc_ZeroWidth`: w=0, verify returns empty string.
     - `TestPadOrTrunc_WidthOne`: w=1, verify returns single char or ellipsis.
  2. Improve existing `PadOrTrunc` implementation:
     - Use `lipgloss.Width(s)` for visible width measurement (already done).
     - For truncation: use `ansi.Truncate(s, w-1, "...")` from `charm.land/x/ansi` for ANSI-safe truncation instead of rune-based slicing. If that import is not available, use rune-based but strip ANSI codes first.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestPadOrTrunc -count=1` passes
- **Blocked by:** none

### W1.A.4 CenterTitle produces top border line with title between corners
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/layout/frame.go`, `/Users/k2m30/projects/a9s/tests/unit/layout_centertitle_test.go`
- **Do:**
  1. Write tests first in `layout_centertitle_test.go`:
     - `TestCenterTitle_Even`: title="test", w=20. Assert starts with corner char, ends with corner char, title is centered between dashes, total visible width is 20.
     - `TestCenterTitle_Odd`: title="abc", w=21. Assert dashes are balanced (differ by at most 1).
     - `TestCenterTitle_Empty`: title="", w=20. Assert plain top border with all dashes.
     - `TestCenterTitle_WiderThanAvailable`: title="very-long-title-here", w=15. Assert minimum 1 dash each side.
  2. Implement `CenterTitle(title string, w int) string`:
     - Compute `titleVis := lipgloss.Width(title)`.
     - `totalDashes := w - 2 - titleVis - 2` (corners + spaces around title). Clamp to minimum 2.
     - `leftDashes := totalDashes / 2`, `rightDashes := totalDashes - leftDashes`.
     - Return: `borderStyle.Render(corner + dashes + " ") + titleStyle.Render(title) + borderStyle.Render(" " + dashes + corner)`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestCenterTitle -count=1` passes
- **Blocked by:** none

---

### W1.B.1 Palette completeness audit against design spec
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/styles/palette.go`, `/Users/k2m30/projects/a9s/tests/unit/palette_test.go`
- **Do:**
  1. Write tests first in `palette_test.go`:
     - `TestPalette_AllColorsNonEmpty`: iterate over all exported `Col*` vars via reflection, assert none are empty string.
     - `TestPalette_DesignSpecCoverage`: explicitly check each color from design.md section 1 exists: ColHeaderFg (#c0caf5), ColAccent (#7aa2f7), ColDim (#565f89), ColBorder (#414868), ColRowSelectedBg (#7aa2f7), ColRowSelectedFg (#1a1b26), ColRowAltBg (#1e2030), ColRunning (#9ece6a), ColStopped (#f7768e), ColPending (#e0af68), ColTerminated (#565f89), ColDetailKey, ColDetailVal, ColDetailSec, ColYAMLKey, ColYAMLStr, ColYAMLNum, ColYAMLBool, ColYAMLNull, ColYAMLTree, ColHelpKey, ColHelpCat, ColFilter, ColSuccess, ColError, ColSpinner, ColScroll, ColKeyHintKey, ColKeyHintBg, ColKeyHintFg.
     - `TestPalette_MissingWarningText`: verify ColWarning (amber #e0af68) exists if needed by design. If missing, add it.
  2. Add any missing palette colors:
     - Check design spec for "Warning text" (#e0af68) -- this may already be `ColFilter` but should have a distinct semantic name `ColWarning` if used in non-filter contexts.
     - Check for "Overlay bg" (#1a1b26) and "Overlay border" (#7aa2f7) if help screen needs them.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestPalette -count=1` passes
- **Blocked by:** none

### W1.B.2 RowColorStyle coverage for all status strings
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/styles/styles.go`, `/Users/k2m30/projects/a9s/tests/unit/rowcolor_test.go`
- **Do:**
  1. Write tests first in `rowcolor_test.go`:
     - `TestRowColorStyle_Running`: input "running", assert foreground is ColRunning.
     - `TestRowColorStyle_Available`: input "available", assert foreground is ColRunning.
     - `TestRowColorStyle_Active`: input "Active", assert foreground is ColRunning (case-insensitive).
     - `TestRowColorStyle_InUse`: input "in-use", assert foreground is ColRunning.
     - `TestRowColorStyle_Stopped`: input "stopped", assert foreground is ColStopped.
     - `TestRowColorStyle_Failed`: input "failed", assert foreground is ColStopped.
     - `TestRowColorStyle_Error`: input "error", assert foreground is ColStopped.
     - `TestRowColorStyle_Deleting`: input "deleting", assert foreground is ColStopped.
     - `TestRowColorStyle_Deleted`: input "deleted", assert foreground is ColStopped.
     - `TestRowColorStyle_Pending`: input "pending", assert foreground is ColPending.
     - `TestRowColorStyle_Creating`: input "creating", assert foreground is ColPending.
     - `TestRowColorStyle_Modifying`: input "modifying", assert foreground is ColPending.
     - `TestRowColorStyle_Updating`: input "updating", assert foreground is ColPending.
     - `TestRowColorStyle_Terminated`: input "terminated", assert foreground is ColTerminated.
     - `TestRowColorStyle_ShuttingDown`: input "shutting-down", assert foreground is ColTerminated.
     - `TestRowColorStyle_Unknown`: input "whatever", assert foreground is ColHeaderFg.
     - `TestRowColorStyle_Empty`: input "", assert foreground is ColHeaderFg.
  2. Fix any gaps in the switch statement. Current implementation already covers most, but verify:
     - "deleting" and "deleted" map to ColStopped (currently present).
     - "in-use" maps to ColRunning (currently present).
     - "updating" maps to ColPending (currently present).
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestRowColorStyle -count=1` passes
- **Blocked by:** none

### W1.B.3 Composed styles audit (missing styles, NO_COLOR correctness)
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/styles/styles.go`, `/Users/k2m30/projects/a9s/tests/unit/styles_test.go`
- **Do:**
  1. Write tests first in `styles_test.go`:
     - `TestStyles_InitPopulated`: after init(), assert HeaderStyle, TableHeader, RowSelected, RowNormal, RowAlt, BorderNormal, BorderFocused, DetailKey, DetailVal, DetailSection, FlashSuccess, FlashError, FilterActive, DimText, SpinnerStyle are all non-zero-value styles (verify at least one property is set).
     - `TestStyles_NoColor`: set NO_COLOR=1, call init(), verify all styles remain zero-value (plain, no color applied).
     - `TestStyles_MissingYAMLStyles`: verify that YAML-specific styles exist or are constructed inline in the YAML view. If needed, add `YAMLKeyStyle`, `YAMLStrStyle`, `YAMLNumStyle`, `YAMLBoolStyle`, `YAMLNullStyle`, `YAMLTreeStyle` to the init() block.
     - `TestStyles_MissingHelpStyles`: verify that `HelpKeyStyle` and `HelpCatStyle` exist or add them.
  2. Add any missing composed styles to the `init()` function:
     - `YAMLKeyStyle`, `YAMLStrStyle`, `YAMLNumStyle`, `YAMLBoolStyle`, `YAMLNullStyle`, `YAMLTreeStyle`.
     - `HelpKeyStyle`, `HelpCatStyle`.
     - `KeyHintKeyStyle`, `KeyHintDescStyle`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestStyles -count=1` passes
- **Blocked by:** none

---

### W1.C.1 Fix ClientsReadyMsg to carry ServiceClients pointer
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/messages/messages.go`, `/Users/k2m30/projects/a9s/tests/unit/messages_test.go`
- **Do:**
  1. Write tests first in `messages_test.go`:
     - `TestClientsReadyMsg_WithClients`: create a ClientsReadyMsg with a non-nil Clients field, assert Clients is accessible and Err is nil.
     - `TestClientsReadyMsg_WithError`: create with Err set, Clients nil, assert Err message is retrievable.
     - `TestClientsReadyMsg_BothNil`: create with both nil, assert no panic.
  2. Modify `ClientsReadyMsg` in `messages.go`:
     - Add field `Clients *awsclient.ServiceClients` (import `awsclient "github.com/k2m30/a9s/internal/aws"`).
     - This requires adding the import for the `aws` package.
  3. Update `connectAWS` in `app.go` to populate `Clients` field:
     - After `awsclient.NewAWSSession(profile, region)`, call `awsclient.CreateServiceClients(cfg)` and set `msg.Clients`.
  4. Update `ClientsReadyMsg` handler in `app.go Update()` to store: `m.clients = msg.Clients`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestClientsReadyMsg -count=1` passes
- **Blocked by:** none

### W1.C.2 Add missing message types (RefreshMsg, SortMsg)
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/messages/messages.go`, `/Users/k2m30/projects/a9s/tests/unit/messages_test.go`
- **Do:**
  1. Write tests first (append to `messages_test.go`):
     - `TestRefreshMsg_Fields`: create a RefreshMsg with ResourceType="ec2", assert field is accessible.
     - `TestSortMsg_Fields`: create a SortMsg with Field="name" and Ascending=true, assert fields.
  2. Add to `messages.go`:
     ```
     type RefreshMsg struct {
         ResourceType string
     }
     type SortMsg struct {
         Field     string
         Ascending bool
     }
     ```
  3. Verify existing messages match the design spec state transition table (section 6 of design.md). All messages listed there should exist in `messages.go`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run "TestRefreshMsg|TestSortMsg" -count=1` passes
- **Blocked by:** none

### W1.C.3 Verify keys.Map completeness against design spec
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/keys/keys.go`, `/Users/k2m30/projects/a9s/tests/unit/keys_test.go`
- **Do:**
  1. Write tests first in `keys_test.go`:
     - `TestKeysDefault_AllBindingsSet`: call `keys.Default()`, assert every field in the Map struct has non-empty Keys (i.e., `key.Matches` would work). Check all 23 bindings: Up, Down, Top, Bottom, Enter, Escape, Quit, ForceQuit, Help, Refresh, Colon, Filter, Tab, Describe, YAML, Reveal, Copy, ScrollLeft, ScrollRight, SortByName, SortByStatus, SortByAge, ToggleWrap.
     - `TestKeysDefault_HelpTextSet`: assert each binding has non-empty Help text (used by help view).
     - `TestKeysDefault_KeyStrings`: spot-check that Up matches "k" and "up", Down matches "j" and "down", Quit matches "q", ForceQuit matches "ctrl+c".
  2. Verify all keys from design spec section 5 are present. Currently all appear to be defined. If any are missing, add them.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestKeysDefault -count=1` passes
- **Blocked by:** none

---

## Wave 2 -- Views

### W2.D.1 app.go View() composes RenderHeader + RenderFrame
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/app.go`, `/Users/k2m30/projects/a9s/tests/unit/app_view_test.go`
- **Do:**
  1. Write tests first in `app_view_test.go`:
     - `TestModelView_ComposesHeaderAndFrame`: create a Model with width=80, height=24, profile="prod", region="us-east-1", push a mainMenu view. Call View(). Assert output contains "a9s", "prod:us-east-1", "? for help". Assert output contains box-drawing chars for the frame. Assert the frame title contains "resource-types".
     - `TestModelView_ZeroWidth`: width=0, assert returns empty View (already handled).
     - `TestModelView_FilterMode`: set inputMode=modeFilter, cmdInput value="test", assert header right shows "/test".
     - `TestModelView_FlashMode`: set flash active with text "Copied!", assert header right shows "Copied!".
  2. Replace the TODO in `View()`:
     ```go
     func (m Model) View() tea.View {
         if m.width == 0 {
             return tea.NewView("")
         }
         rightContent := m.headerRight()
         header := layout.RenderHeader(m.profile, m.region, Version, m.width, rightContent)
         active := m.activeView()
         content := active.view()
         title := active.frameTitle()
         contentLines := strings.Split(content, "\n")
         frameH := m.height - 1  // 1 line for header
         frame := layout.RenderFrame(contentLines, title, m.width, frameH)
         return tea.NewView(header + "\n" + frame)
     }
     ```
  3. Add `"strings"` and `layout` import to `app.go`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestModelView -count=1` passes
- **Blocked by:** W1.A.1, W1.A.2

### W2.D.2 handleNavigate creates target view, sets size, pushes stack
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/app.go`, `/Users/k2m30/projects/a9s/tests/unit/app_navigate_test.go`
- **Do:**
  1. Write tests first in `app_navigate_test.go`:
     - `TestHandleNavigate_ToResourceList`: send NavigateMsg{Target: TargetResourceList, ResourceType: "ec2"}. Assert stack length is 2. Assert top of stack is a resourceList entry with typeDef.ShortName == "ec2".
     - `TestHandleNavigate_ToDetail`: send NavigateMsg{Target: TargetDetail, Resource: &testResource}. Assert top of stack is a detail entry.
     - `TestHandleNavigate_ToYAML`: send NavigateMsg{Target: TargetYAML, Resource: &testResource}. Assert top of stack is a yaml entry.
     - `TestHandleNavigate_ToReveal`: send NavigateMsg{Target: TargetReveal}. Assert top of stack is a reveal entry.
     - `TestHandleNavigate_ToProfile`: assert top of stack is a profile entry.
     - `TestHandleNavigate_ToRegion`: assert top of stack is a region entry.
     - `TestHandleNavigate_ToHelp`: assert top of stack is a help entry.
     - Test for each of the 7 resource types (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager).
  2. Implement `handleNavigate`:
     ```go
     func (m Model) handleNavigate(msg messages.NavigateMsg) (tea.Model, tea.Cmd) {
         var entry viewEntry
         var cmd tea.Cmd
         switch msg.Target {
         case messages.TargetResourceList:
             typeDef := resource.FindResourceType(msg.ResourceType)
             if typeDef == nil { return m, nil }
             rl := views.NewResourceList(*typeDef, m.viewConfig, m.keys)
             rl.SetSize(m.width, m.height)
             entry = viewEntry{resourceList: &rl}
             cmd = m.fetchResources(msg.ResourceType)
         case messages.TargetDetail:
             if msg.Resource == nil { return m, nil }
             d := views.NewDetail(*msg.Resource, m.viewConfig, m.keys)
             d.SetSize(m.width, m.height)
             entry = viewEntry{detail: &d}
         case messages.TargetYAML:
             if msg.Resource == nil { return m, nil }
             y := views.NewYAML(*msg.Resource, m.keys)
             y.SetSize(m.width, m.height)
             entry = viewEntry{yaml: &y}
         case messages.TargetReveal:
             // Reveal requires fetching secret first -- handled in Wave 3
             return m, nil
         case messages.TargetProfile:
             p := views.NewProfile(m.listProfiles(), m.profile, m.keys)
             p.SetSize(m.width, m.height)
             entry = viewEntry{profile: &p}
         case messages.TargetRegion:
             r := views.NewRegion(m.listRegions(), m.region, m.keys)
             r.SetSize(m.width, m.height)
             entry = viewEntry{region: &r}
         case messages.TargetHelp:
             h := views.NewHelp(m.keys)
             h.SetSize(m.width, m.height)
             entry = viewEntry{help: &h}
         }
         m.pushView(entry)
         return m, cmd
     }
     ```
  3. Add stub methods `fetchResources`, `listProfiles`, `listRegions` on Model.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestHandleNavigate -count=1` passes
- **Blocked by:** W1.C.1

### W2.D.3 connectAWS stores ServiceClients on model via ClientsReadyMsg
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/app.go`, `/Users/k2m30/projects/a9s/tests/unit/app_connect_test.go`
- **Do:**
  1. Write tests first in `app_connect_test.go`:
     - `TestConnectAWS_Success`: mock NewAWSSession (use interface or test helper), send ClientsReadyMsg with non-nil Clients. Assert m.clients is set. Assert no flash error.
     - `TestConnectAWS_Error`: send ClientsReadyMsg with Err set. Assert m.clients remains nil. Assert flash is active and isError is true.
     - `TestConnectAWS_UpdatesProfileRegion`: after ProfileSelectedMsg, assert m.profile changed and connectAWS was called with new profile.
  2. Update `connectAWS` method:
     ```go
     func (m *Model) connectAWS(profile, region string) tea.Cmd {
         return func() tea.Msg {
             cfg, err := awsclient.NewAWSSession(profile, region)
             if err != nil {
                 return messages.ClientsReadyMsg{Err: err}
             }
             clients := awsclient.CreateServiceClients(cfg)
             return messages.ClientsReadyMsg{Clients: clients}
         }
     }
     ```
  3. Update `ClientsReadyMsg` handler in `Update()`:
     ```go
     case messages.ClientsReadyMsg:
         if msg.Err != nil {
             m.flash = flashState{text: msg.Err.Error(), isError: true, active: true}
         } else {
             m.clients = msg.Clients
         }
         return m, nil
     ```
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestConnectAWS -count=1` passes
- **Blocked by:** W1.C.1

### W2.D.4 Load ViewsConfig at startup and pass to child views
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/app.go`, `/Users/k2m30/projects/a9s/tests/unit/app_config_test.go`
- **Do:**
  1. Write tests first in `app_config_test.go`:
     - `TestNewModel_LoadsViewConfig`: create Model with New(), assert viewConfig is non-nil (at minimum the built-in defaults are loaded).
     - `TestNewModel_CustomConfig`: set up a temp views.yaml file, verify viewConfig loads from it.
     - Test for all 7 resource types: verify each has a non-empty ViewDef from the defaults.
  2. Update `New()` to load config:
     ```go
     func New(profile, region string) Model {
         ti := textinput.New()
         k := keys.Default()
         cfg, _ := config.Load()
         if cfg == nil {
             cfg = config.DefaultConfig()
         }
         menu := views.NewMainMenu(k)
         entry := viewEntry{mainMenu: &menu}
         return Model{
             profile:    profile,
             region:     region,
             keys:       k,
             stack:      []viewEntry{entry},
             cmdInput:   ti,
             viewConfig: cfg,
         }
     }
     ```
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestNewModel -count=1` passes
- **Blocked by:** none

---

### W2.E.1 ResourceListModel.View renders column headers with sort indicator
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/resourcelist.go`, `/Users/k2m30/projects/a9s/tests/unit/resourcelist_headers_test.go`
- **Do:**
  1. Write tests first in `resourcelist_headers_test.go`:
     - `TestResourceList_HeaderLine`: create ResourceListModel for "ec2", set size 120x24, load with 3 resources. Call View(). Assert first line of content contains column titles from ViewDef: "Instance ID", "Name", "State", "Type", etc.
     - `TestResourceList_SortIndicatorAsc`: set sort=SortName, sortAsc=true. Assert first column title ends with upward arrow char.
     - `TestResourceList_SortIndicatorDesc`: set sort=SortName, sortAsc=false. Assert first column title ends with downward arrow char.
     - `TestResourceList_NoSortIndicator`: sort=SortNone, assert no arrow chars in any column title.
     - Test with all 7 resource types to verify columns render for each.
  2. Implement the header portion of `View()`:
     - Get `viewDef := config.GetViewDef(m.viewConfig, m.typeDef.ShortName)`.
     - Build column headers from `viewDef.List`, applying `PadOrTrunc(col.Title, col.Width)`.
     - Append sort indicator arrow to the active sort column.
     - Style with `styles.TableHeader`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestResourceList_Header -count=1` passes
- **Blocked by:** W1.A.3, W1.B.2

### W2.E.2 ResourceListModel.View renders status-colored rows with selection
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/resourcelist.go`, `/Users/k2m30/projects/a9s/tests/unit/resourcelist_rows_test.go`
- **Do:**
  1. Write tests first in `resourcelist_rows_test.go`:
     - `TestResourceList_RowRendering`: load with 5 resources having different statuses (running, stopped, pending, terminated, unknown). Call View(). Assert output contains each resource name. Assert row count matches resource count + 1 (header).
     - `TestResourceList_SelectedRow`: cursor=2, assert the 3rd data row uses RowSelected style (blue bg, dark fg).
     - `TestResourceList_StatusColors`: verify "running" resource row is green text, "stopped" is red, "pending" is yellow, "terminated" is dim.
     - `TestResourceList_AlternatingRows`: verify even/odd rows alternate between RowNormal and RowAlt styles.
     - Test with all 7 resource types to verify status coloring works across resource types.
  2. Implement the row-rendering portion of `View()`:
     - For each resource in `m.filteredResources`, extract column values using `fieldpath.ExtractValue(res.RawStruct, col.Path)`.
     - Apply `PadOrTrunc(value, col.Width)` to each cell.
     - Join cells with double-space separator.
     - Apply status color via `styles.RowColorStyle(res.Status)` for non-selected rows.
     - Apply `styles.RowSelected` for the cursor row, full width.
     - Apply `styles.RowAlt` background for odd-indexed non-selected rows.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestResourceList_Row -count=1` passes
- **Blocked by:** W1.A.3, W1.B.2

### W2.E.3 ResourceListModel.View handles horizontal scroll offset
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/resourcelist.go`, `/Users/k2m30/projects/a9s/tests/unit/resourcelist_hscroll_test.go`
- **Do:**
  1. Write tests first in `resourcelist_hscroll_test.go`:
     - `TestResourceList_HScrollZero`: hScrollOffset=0, width=80. Assert first column is visible.
     - `TestResourceList_HScrollRight`: hScrollOffset=2, width=60. Assert first two columns are hidden and third column starts at left edge.
     - `TestResourceList_HScrollBeyondMax`: hScrollOffset=100 (exceeds column count). Assert at least last column is still visible (clamped).
     - `TestResourceList_HScrollHeaderSync`: assert column headers scroll in sync with data rows.
     - Test with EC2 (7 columns), S3 (2 columns), Secrets (5 columns).
  2. Implement horizontal scroll in `View()`:
     - Skip the first `m.hScrollOffset` columns when building each row.
     - Track cumulative width of visible columns. Stop adding columns when cumulative width exceeds `m.width - 2`.
     - Apply same offset to header row so headers stay aligned with data.
     - Clamp `hScrollOffset` to `max(0, len(columns) - 1)`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestResourceList_HScroll -count=1` passes
- **Blocked by:** W2.E.1

### W2.E.4 ResourceListModel.View handles empty/error states
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/resourcelist.go`, `/Users/k2m30/projects/a9s/tests/unit/resourcelist_states_test.go`
- **Do:**
  1. Write tests first in `resourcelist_states_test.go`:
     - `TestResourceList_LoadingState`: loading=true. Assert View() output contains spinner character and "Loading..." text.
     - `TestResourceList_EmptyAfterLoad`: loading=false, allResources is empty slice. Assert View() shows a centered message like "No resources found" or "0 ec2 instances".
     - `TestResourceList_EmptyAfterFilter`: 5 resources loaded but filter matches none. Assert View() shows "No matches" hint.
     - Test empty state for all 7 resource types.
  2. Add empty/error state handling to `View()`:
     ```go
     if !m.loading && len(m.filteredResources) == 0 {
         if m.filterText != "" {
             return styles.DimText.Render("  No matches for \"" + m.filterText + "\"")
         }
         return styles.DimText.Render("  No " + m.typeDef.ShortName + " resources found")
     }
     ```
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestResourceList_.*State -count=1` passes
- **Blocked by:** none

---

### W2.F.1 MainMenuModel.View renders items with cursor and dimmed alias
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/mainmenu.go`, `/Users/k2m30/projects/a9s/tests/unit/mainmenu_view_test.go`
- **Do:**
  1. Write tests first in `mainmenu_view_test.go`:
     - `TestMainMenu_Rendering`: create MainMenuModel, set size 80x24. Call View(). Assert output contains all 7 resource type names: "EC2 Instances", "S3 Buckets", "RDS Instances", "ElastiCache Redis", "DocumentDB Clusters", "EKS Clusters", "Secrets Manager".
     - `TestMainMenu_CursorOnFirst`: cursor=0, assert first item uses RowSelected style.
     - `TestMainMenu_CursorOnThird`: cursor=2, assert third item uses RowSelected style and first item does not.
     - `TestMainMenu_DimmedAliases`: assert output contains dimmed command aliases ":ec2", ":s3", ":rds", ":redis", ":docdb", ":eks", ":secrets".
     - `TestMainMenu_CountFooter`: assert output contains "7 resource types" in dim text.
  2. Implement `View()`:
     - For each item in `m.items`, render: `"  " + PadOrTrunc(item.Name, nameW) + dimStyle.Render(":" + item.ShortName)`.
     - For cursor row, apply `styles.RowSelected` to full width.
     - Append blank line and dim footer: `"  N resource types"`.
     - Use `m.width - 2` as inner width (accounting for frame borders).
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestMainMenu -count=1` passes
- **Blocked by:** W1.A.3

### W2.F.2 DetailModel.renderContent builds styled key-value from ViewDef
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/detail.go`, `/Users/k2m30/projects/a9s/tests/unit/detail_render_test.go`
- **Do:**
  1. Write tests first in `detail_render_test.go`:
     - `TestDetail_RenderContent_EC2`: create DetailModel with an EC2 resource (RawStruct set to a mock EC2 instance struct). Call renderContent(). Assert output contains section headers styled with ColDetailSec. Assert output contains key-value pairs with keys styled in ColDetailKey.
     - `TestDetail_RenderContent_NilFields`: resource with sparse data (some fields nil). Assert no panic and nil fields show as "-" or empty.
     - `TestDetail_RenderContent_AllResourceTypes`: test with mock resources for all 7 types (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets).
     - `TestDetail_RenderContent_UsesViewDef`: supply a custom ViewsConfig that limits detail fields to 2 entries. Assert only those 2 keys appear.
  2. Implement `renderContent()`:
     - Get `viewDef := config.GetViewDef(m.viewConfig, ...)` (need to know resource type -- may need to store it on DetailModel or derive from Resource).
     - For each field path in `viewDef.Detail`:
       - If the path represents a section break (TBD), render section header with `styles.DetailSection`.
       - Otherwise, extract value with `fieldpath.ExtractValue(m.res.RawStruct, path)`.
       - Render: `styles.DetailKey.Render(PadOrTrunc(key, 24)) + styles.DetailVal.Render(value)`.
     - Join all lines with newlines.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestDetail_RenderContent -count=1` passes
- **Blocked by:** W1.A.3, W1.B.1

### W2.F.3 YAMLModel.renderContent marshals RawStruct and colorizes
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/yaml.go`, `/Users/k2m30/projects/a9s/tests/unit/yaml_render_test.go`
- **Do:**
  1. Write tests first in `yaml_render_test.go`:
     - `TestYAML_RenderContent_Basic`: create YAMLModel with a resource whose RawStruct is a simple struct. Call renderContent(). Assert output contains YAML key-value pairs.
     - `TestYAML_Colorize_Keys`: assert YAML keys are styled with ColYAMLKey color.
     - `TestYAML_Colorize_Strings`: assert string values are styled with ColYAMLStr.
     - `TestYAML_Colorize_Numbers`: assert numeric values are styled with ColYAMLNum.
     - `TestYAML_Colorize_Booleans`: assert boolean values are styled with ColYAMLBool.
     - `TestYAML_Colorize_Null`: assert null/empty values are styled with ColYAMLNull.
     - `TestYAML_TreeLines`: assert nested entries have tree connector chars styled with ColYAMLTree.
     - `TestYAML_NilRawStruct`: resource with nil RawStruct. Assert returns fallback message, no panic.
     - Test with mock resources for all 7 types.
  2. Implement `renderContent()`:
     - Convert `m.res.RawStruct` via `fieldpath.ToSafeValue(m.res.RawStruct)` to get an `interface{}` safe for yaml.Marshal.
     - Marshal with `yaml.Marshal(safeVal)`.
     - Pass result through `colorizeYAML(string(yamlBytes))`.
  3. Implement `colorizeYAML(raw string) string`:
     - Split raw YAML by lines.
     - For each line, detect pattern:
       - `key:` at start -> apply ColYAMLKey to key, detect value type.
       - Quoted strings -> ColYAMLStr.
       - Numeric patterns (regex `^-?\d+(\.\d+)?$`) -> ColYAMLNum.
       - `true`/`false` -> ColYAMLBool.
       - `null`/`~`/empty -> ColYAMLNull.
       - Lines starting with `- ` (list items) -> handle key detection after `- `.
     - Prepend tree connector `|` (styled ColYAMLTree) for indented lines.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestYAML -count=1` passes
- **Blocked by:** W1.B.1

### W2.F.4 HelpModel.View renders 4-column keybinding layout
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/help.go`, `/Users/k2m30/projects/a9s/tests/unit/help_view_test.go`
- **Do:**
  1. Write tests first in `help_view_test.go`:
     - `TestHelp_Rendering`: create HelpModel with Default() keys, set size 84x24. Call View(). Assert output contains "RESOURCE", "GENERAL", "NAVIGATION", "HOTKEYS" category headers.
     - `TestHelp_KeyBindings`: assert output contains specific keys: "<esc>", "<ctrl-r>", "<j>", "<k>", "<?>" in their respective columns.
     - `TestHelp_KeyColors`: assert key text uses ColHelpKey color (green).
     - `TestHelp_CategoryColors`: assert category headers use ColHelpCat color (orange).
     - `TestHelp_CloseHint`: assert output contains "Press any key to close" in dim text.
     - `TestHelp_NarrowWidth`: set size 60x24, assert layout degrades gracefully.
  2. Implement `View()`:
     - Compute `colW := (m.width - 4) / 4`.
     - Render category headers row: each header padded to colW with `styles.HelpCatStyle`.
     - Build binding rows (hardcoded from keys.Map Help text):
       - RESOURCE: esc=Back, q=Quit
       - GENERAL: ctrl-r=Refresh, q=Quit, :=Command, /=Filter
       - NAVIGATION: j=Down, k=Up, g=Top, G=Bottom, h/l=Cols, enter=Open, d=Detail, y=YAML, c=Copy ID, N/S/A=Sort
       - HOTKEYS: ?=Help, :=Command
     - For each row, pad columns to colW.
     - Append "Press any key to close" centered in dim text.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestHelp -count=1` passes
- **Blocked by:** W1.B.3

### W2.F.5 ProfileModel.View renders list with cursor and "(current)" mark
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/profile.go`, `/Users/k2m30/projects/a9s/tests/unit/profile_view_test.go`
- **Do:**
  1. Write tests first in `profile_view_test.go`:
     - `TestProfile_Rendering`: create ProfileModel with profiles=["default","prod","staging"], activeProfile="default", set size 80x24. Call View(). Assert output contains all 3 profile names.
     - `TestProfile_CurrentMark`: assert "default" row contains "(current)" annotation.
     - `TestProfile_CursorOnSecond`: cursor=1, assert "prod" row uses RowSelected style.
     - `TestProfile_CountFooter`: assert output contains "3 profiles" dim text.
     - `TestProfile_EmptyProfiles`: profiles=[], assert graceful empty state.
  2. Implement `View()`:
     - For each profile, render: `"  " + profileName`.
     - If profile == m.activeProfile, append ` (current)` in dim text.
     - Apply `styles.RowSelected` to cursor row.
     - Append blank line + dim footer: `"  N profiles"`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestProfile -count=1` passes
- **Blocked by:** W1.A.3

### W2.F.6 RegionModel.View renders list with cursor and "(current)" mark
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/region.go`, `/Users/k2m30/projects/a9s/tests/unit/region_view_test.go`
- **Do:**
  1. Write tests first in `region_view_test.go`:
     - `TestRegion_Rendering`: create RegionModel with regions=["us-east-1","us-west-2","eu-west-1"], activeRegion="us-east-1", set size 80x24. Call View(). Assert output contains all 3 region names.
     - `TestRegion_CurrentMark`: assert "us-east-1" row contains "(current)".
     - `TestRegion_CursorOnSecond`: cursor=1, assert "us-west-2" uses RowSelected style.
     - `TestRegion_CountFooter`: assert output contains "3 regions" dim text.
     - `TestRegion_EmptyRegions`: regions=[], assert graceful empty state.
  2. Implement `View()`:
     - Same structure as ProfileModel.View() but with region names.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestRegion -count=1` passes
- **Blocked by:** W1.A.3

### W2.F.7 RevealModel.View renders secret with title, separator, metadata
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/views/reveal.go`, `/Users/k2m30/projects/a9s/tests/unit/reveal_view_test.go`
- **Do:**
  1. Write tests first in `reveal_view_test.go`:
     - `TestReveal_Rendering`: create RevealModel with secretName="prod/api/db-pass", value="s3cr3t!". Set size 80x24. Call View(). Assert output contains the secret name as a bold title. Assert output contains "s3cr3t!" as the value in green. Assert output contains dim separator line.
     - `TestReveal_HeaderWarning`: call HeaderWarning(). Assert returns red-styled "Secret visible -- press esc to close".
     - `TestReveal_EmptyValue`: value="", assert shows "(empty)" or similar placeholder.
     - `TestReveal_LongValue`: value is 500 chars, verify viewport scrolls.
  2. Update `SetSize` to build structured content (currently just sets `m.value`):
     ```go
     func (m *RevealModel) SetSize(w, h int) {
         // ... existing viewport setup ...
         m.viewport.SetContent(m.renderContent())
     }
     ```
  3. Add `renderContent()`:
     - Line 1: bold secret name.
     - Line 2: dim separator dashes.
     - Line 3: blank.
     - Line 4: green-styled secret value.
     - Line 5: blank.
     - Line 6+: dim metadata (type, rotation info if available).
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestReveal -count=1` passes
- **Blocked by:** W1.B.1

---

## Wave 3 -- Wiring

### W3.G.1 executeCommand dispatches colon-commands to NavigateMsg
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/app.go`, `/Users/k2m30/projects/a9s/tests/unit/app_command_test.go`
- **Do:**
  1. Write tests first in `app_command_test.go`:
     - `TestExecuteCommand_EC2`: input "ec2", assert returns a tea.Cmd that produces NavigateMsg{Target: TargetResourceList, ResourceType: "ec2"}.
     - `TestExecuteCommand_S3`: input "s3", same pattern.
     - `TestExecuteCommand_RDS`: input "rds", same pattern.
     - `TestExecuteCommand_Redis`: input "redis", same pattern.
     - `TestExecuteCommand_DocDB`: input "docdb", same pattern.
     - `TestExecuteCommand_EKS`: input "eks", same pattern.
     - `TestExecuteCommand_Secrets`: input "secrets", same pattern.
     - `TestExecuteCommand_Main`: input "main", assert pops to MainMenu (stack length becomes 1 or clears stack to root).
     - `TestExecuteCommand_Root`: input "root", same as "main".
     - `TestExecuteCommand_Ctx`: input "ctx", assert navigates to profile selector.
     - `TestExecuteCommand_Region`: input "region", assert navigates to region selector.
     - `TestExecuteCommand_Quit`: input "q" or "quit", assert returns tea.Quit.
     - `TestExecuteCommand_Unknown`: input "foobar", assert returns FlashMsg with error text.
     - `TestExecuteCommand_Aliases`: input "instances" (alias for ec2), assert navigates to ec2 resource list.
  2. Implement `executeCommand`:
     ```go
     func (m Model) executeCommand(cmd string) (tea.Model, tea.Cmd) {
         cmd = strings.TrimSpace(cmd)
         switch cmd {
         case "q", "quit":
             return m, tea.Quit
         case "main", "root":
             m.stack = m.stack[:1]
             return m, nil
         case "ctx", "profile":
             return m, func() tea.Msg {
                 return messages.NavigateMsg{Target: messages.TargetProfile}
             }
         case "region":
             return m, func() tea.Msg {
                 return messages.NavigateMsg{Target: messages.TargetRegion}
             }
         default:
             if typeDef := resource.FindResourceType(cmd); typeDef != nil {
                 return m, func() tea.Msg {
                     return messages.NavigateMsg{
                         Target:       messages.TargetResourceList,
                         ResourceType: typeDef.ShortName,
                     }
                 }
             }
             m.flash = flashState{text: "Unknown command: " + cmd, isError: true, active: true}
             return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
                 return messages.ClearFlashMsg{}
             })
         }
     }
     ```
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestExecuteCommand -count=1` passes
- **Blocked by:** W2.D.2

### W3.G.2 Copy action reads resource ID to clipboard, sends FlashMsg
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/app.go`, `/Users/k2m30/projects/a9s/tests/unit/app_copy_test.go`
- **Do:**
  1. Write tests first in `app_copy_test.go`:
     - `TestCopy_ResourceList`: active view is resourceList with cursor on a resource. Send key "c". Assert a CopiedMsg or FlashMsg{Text: "Copied!"} is returned as a command. Assert clipboard contains the resource ID or Name.
     - `TestCopy_DetailView`: active view is detail. Send key "c". Assert copies the full detail content.
     - `TestCopy_NoResource`: active view is resourceList but empty. Assert flash error or no-op.
     - Test copy for all 7 resource types (verify the correct ID field is used for each).
  2. Add copy handling to the key dispatch in `Update()` (normal mode):
     ```go
     if key.Matches(msg, m.keys.Copy) {
         return m.handleCopy()
     }
     ```
  3. Implement `handleCopy()`:
     - If active view is resourceList, get SelectedResource(), copy ID to clipboard.
     - If active view is detail, copy rendered content to clipboard.
     - If active view is yaml, copy YAML text to clipboard.
     - Use `github.com/atotto/clipboard` for clipboard access.
     - Return FlashMsg{Text: "Copied!", IsError: false}.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestCopy -count=1` passes
- **Blocked by:** W2.D.2, W2.E.2

### W3.G.3 Reveal action fetches secret value, pushes RevealModel
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/app.go`, `/Users/k2m30/projects/a9s/tests/unit/app_reveal_test.go`
- **Do:**
  1. Write tests first in `app_reveal_test.go`:
     - `TestReveal_SecretsResource`: active view is resourceList for "secrets" type. Cursor on a secret. Send key "x". Assert a tea.Cmd is returned that would fetch the secret value. Simulate SecretRevealedMsg arriving. Assert stack top is a RevealModel with the secret value.
     - `TestReveal_NonSecretsResource`: active view is resourceList for "ec2". Send key "x". Assert no-op or flash error "Reveal only available for Secrets Manager".
     - `TestReveal_FetchError`: SecretRevealedMsg arrives with Err set. Assert flash error, no RevealModel pushed.
  2. Add reveal key handling:
     ```go
     if key.Matches(msg, m.keys.Reveal) {
         return m.handleReveal()
     }
     ```
  3. Implement `handleReveal()`:
     - Check if active view is resourceList and typeDef.ShortName == "secrets".
     - Get selected resource, extract secret name.
     - Return a tea.Cmd that calls `m.clients.SecretsManager.GetSecretValue(...)`.
  4. Handle `SecretRevealedMsg` in `Update()`:
     - On success: create RevealModel with secret name and value, push to stack.
     - On error: flash error message.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestReveal -count=1` passes
- **Blocked by:** W2.D.3, W2.F.7

### W3.G.4 Refresh action re-fetches current resource list from AWS
- **File(s):** `/Users/k2m30/projects/a9s/internal/tui/app.go`, `/Users/k2m30/projects/a9s/tests/unit/app_refresh_test.go`
- **Do:**
  1. Write tests first in `app_refresh_test.go`:
     - `TestRefresh_ResourceList`: active view is resourceList for "ec2". Send ctrl+r. Assert a tea.Cmd is returned that fetches resources. Assert loading is set back to true on the ResourceListModel.
     - `TestRefresh_MainMenu`: active view is mainMenu. Send ctrl+r. Assert no-op (main menu has nothing to refresh).
     - `TestRefresh_DetailView`: active view is detail. Send ctrl+r. Assert no-op or re-renders from existing data.
     - Test refresh for all 7 resource types.
  2. Add refresh key handling in `Update()` (normal mode):
     ```go
     if key.Matches(msg, m.keys.Refresh) {
         return m.handleRefresh()
     }
     ```
  3. Implement `handleRefresh()`:
     - If active view is resourceList:
       - Set `active.resourceList.loading = true` (need to add a method like `StartLoading()`).
       - Return `m.fetchResources(active.resourceList.typeDef.ShortName)`.
     - Otherwise, no-op.
  4. Implement `fetchResources(resourceType string) tea.Cmd`:
     - Return a tea.Cmd that calls the appropriate AWS API based on resourceType.
     - On success, return `ResourcesLoadedMsg`.
     - On error, return `APIErrorMsg`.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestRefresh -count=1` passes
- **Blocked by:** W2.D.3

### W3.G.5 Switch entrypoint from internal/app to internal/tui in cmd/a9s/main.go
- **File(s):** `/Users/k2m30/projects/a9s/cmd/a9s/main.go`, `/Users/k2m30/projects/a9s/tests/unit/entrypoint_test.go`
- **Do:**
  1. Write tests first in `entrypoint_test.go`:
     - `TestNewTUIModel_InitializesCorrectly`: call `tui.New("prod", "us-east-1")`. Assert returned Model has profile="prod", region="us-east-1", stack length=1, top view is mainMenu.
     - `TestNewTUIModel_DefaultProfile`: call `tui.New("", "")`. Assert Model initializes without error.
  2. Update `cmd/a9s/main.go`:
     - Replace `app.InitStyles()` / `app.NewAppState` with:
       ```go
       import tui "github.com/k2m30/a9s/internal/tui"
       // ...
       tui.Version = version
       m := tui.New(profile, region)
       p := tea.NewProgram(m)
       ```
     - Remove the `internal/app` import.
  3. Bump version constant to "0.6.0" in `cmd/a9s/main.go`.
  4. Rebuild binary: `go build -o a9s ./cmd/a9s/`.
- **Accept:** `go build -o /dev/null /Users/k2m30/projects/a9s/cmd/a9s/` succeeds with no errors
- **Blocked by:** W2.D.1, W2.D.2, W2.D.3, W2.D.4, W3.G.1, W3.G.2, W3.G.3, W3.G.4

---

## Wave 4 -- Test Suite Rewrite

### W4.H.1 Rewrite layout_test.go for new layout package
- **File(s):** `/Users/k2m30/projects/a9s/tests/unit/layout_test.go`
- **Do:**
  1. Read existing `layout_test.go` to understand what it tests against `internal/app`.
  2. Rewrite all test functions to import from `internal/tui/layout` instead of `internal/app` or `internal/ui`.
  3. Update test assertions to match the new function signatures: `RenderFrame(lines, title, w, h)`, `RenderHeader(profile, region, version, w, rightContent)`, `PadOrTrunc(s, w)`, `CenterTitle(title, w)`.
  4. Ensure all existing test scenarios are preserved -- do not drop coverage.
  5. Run and verify all tests pass.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run TestLayout -count=1` passes (0 failures)
- **Blocked by:** W1.A.1, W1.A.2, W1.A.3, W1.A.4

### W4.H.2 Rewrite navigation_test.go for view stack push/pop
- **File(s):** `/Users/k2m30/projects/a9s/tests/unit/navigation_test.go`, `/Users/k2m30/projects/a9s/tests/unit/navigation_roundtrip_test.go`
- **Do:**
  1. Read existing navigation tests.
  2. Rewrite to use `tui.Model` and its `pushView`/`popView` methods (or test via `Update` with `NavigateMsg` and `PopViewMsg`).
  3. Cover all navigation paths from design spec section 6:
     - MainMenu -> ResourceList -> Detail -> YAML (and back via esc).
     - MainMenu -> ResourceList -> Reveal (secrets only).
     - Any view -> Help (and back).
     - Any view -> Profile/Region selector (and back with reconnect).
  4. Test for all 7 resource types in navigation paths.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run "TestNavigation|TestRoundtrip" -count=1` passes
- **Blocked by:** W2.D.2, W3.G.1

### W4.H.3 Rewrite filter_test.go and filter_ui_test.go
- **File(s):** `/Users/k2m30/projects/a9s/tests/unit/filter_test.go`, `/Users/k2m30/projects/a9s/tests/unit/filter_ui_test.go`
- **Do:**
  1. Read existing filter tests.
  2. Rewrite to use `views.ResourceListModel.SetFilter()` and `views.filterResources()`.
  3. Verify:
     - Case-insensitive matching on ID, Name, Status, and Fields values.
     - Empty filter returns all resources.
     - Filter resets cursor to 0.
     - FrameTitle updates to show "(matched/total)" format.
     - Live filter application via `updateFilterMode`.
  4. Test filter with all 7 resource types (each has different field keys).
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run "TestFilter" -count=1` passes
- **Blocked by:** W2.E.1

### W4.H.4 Rewrite detail_test.go and detail_config_test.go
- **File(s):** `/Users/k2m30/projects/a9s/tests/unit/detail_test.go`, `/Users/k2m30/projects/a9s/tests/unit/detail_config_test.go`
- **Do:**
  1. Read existing detail tests.
  2. Rewrite to use `views.DetailModel` and its `renderContent()` method.
  3. Verify:
     - Config-driven field selection (ViewDef.Detail).
     - Section headers render correctly.
     - Key-value alignment with proper padding.
     - Nil field handling (no panics).
     - Viewport scroll works after SetSize.
  4. Test detail rendering for all 7 resource types with representative data.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run "TestDetail" -count=1` passes
- **Blocked by:** W2.F.2

### W4.H.5 Rewrite views_test.go and views_coverage_test.go
- **File(s):** `/Users/k2m30/projects/a9s/tests/unit/views_test.go`, `/Users/k2m30/projects/a9s/tests/unit/views_coverage_test.go`
- **Do:**
  1. Read existing view coverage tests.
  2. Rewrite to exercise all 8 view models: MainMenu, ResourceList, Detail, YAML, Help, Profile, Region, Reveal.
  3. For each view:
     - Test `Init()` returns expected initial state.
     - Test `Update()` handles relevant key messages.
     - Test `View()` returns non-empty string when properly initialized (size set, data loaded).
     - Test `FrameTitle()` returns correct string.
     - Test `SetSize()` updates dimensions.
  4. Test all views for all 7 resource types where applicable.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run "TestViews" -count=1` passes
- **Blocked by:** W2.F.1 through W2.F.7

### W4.H.6 Rewrite app_test.go for new root Model
- **File(s):** `/Users/k2m30/projects/a9s/tests/unit/app_test.go`
- **Do:**
  1. Read existing app_test.go.
  2. Rewrite to test `tui.Model`:
     - `TestModel_Init`: verify Init() returns a Cmd that sends InitConnectMsg.
     - `TestModel_WindowResize`: send WindowSizeMsg, verify all stack views receive new size.
     - `TestModel_ViewStack`: push/pop views, verify stack depth and active view.
     - `TestModel_InputModes`: test filter mode entry/exit, command mode entry/exit.
     - `TestModel_FlashLifecycle`: send FlashMsg, verify active, send ClearFlashMsg, verify cleared.
     - `TestModel_ProfileSwitch`: send ProfileSelectedMsg, verify reconnect triggered.
     - `TestModel_RegionSwitch`: send RegionSelectedMsg, verify reconnect triggered.
  3. Test all resource types in relevant scenarios.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -run "TestModel" -count=1` passes
- **Blocked by:** W2.D.1, W3.G.5

### W4.H.7 Audit and port remaining tests (qa_*, s3_*, bugs_test.go)
- **File(s):** All files in `/Users/k2m30/projects/a9s/tests/unit/qa_*.go`, `/Users/k2m30/projects/a9s/tests/unit/s3_*.go`, `/Users/k2m30/projects/a9s/tests/unit/bugs_test.go`, and other remaining test files.
- **Do:**
  1. Enumerate all test files not already covered by H.1-H.6:
     - `qa_bug_fixes_test.go`, `qa_configurable_views_test.go`, `qa_detail_s3_test.go`, `qa_discrepancies_test.go`, `qa_filter_sort_edge_test.go`, `qa_launch_menu_commands_test.go`, `qa_layout_rendering_test.go`, `qa_resourcelist_profile_test.go`, `qa_s3_object_detail_test.go`, `qa_state_consistency_test.go`, `qa_user_scenarios_test.go`.
     - `s3_bugs_test.go`, `s3_navigation_test.go`, `s3_object_pagination_test.go`, `s3_pagination_test.go`, `s3_perf_test.go`, `s3_profile_test.go`, `s3_render_test.go`.
     - `bugs_test.go`, `column_key_mismatch_test.go`, `horizontal_scroll_test.go`, `separator_test.go`, `ui_test.go`.
  2. For each file:
     - Determine if the test is still relevant to the new TUI architecture.
     - If relevant: rewrite imports and assertions to use `internal/tui/*` packages.
     - If obsolete (tests old `internal/app` internals that no longer exist): mark with a comment explaining why it was removed and what replacement test covers the same behavior.
  3. Ensure no test references `internal/app` -- all should reference `internal/tui`, `internal/tui/views`, `internal/tui/layout`, `internal/tui/styles`, `internal/tui/keys`, or `internal/tui/messages`.
  4. Run full test suite to verify.
- **Accept:** `go test /Users/k2m30/projects/a9s/tests/unit/ -count=1 -timeout 120s` passes with 0 failures
- **Blocked by:** W4.H.1 through W4.H.6

---

## Dependency Graph

```
Wave 1 (parallel):
  A.1 ─┐
  A.2 ─┤
  A.3 ─┼─→ Wave 2
  A.4 ─┘
  B.1 ─┤
  B.2 ─┤
  B.3 ─┘
  C.1 ─┤
  C.2 ─┤
  C.3 ─┘

Wave 2 (parallel within, depends on Wave 1):
  D.1 ─→ needs A.1, A.2
  D.2 ─→ needs C.1
  D.3 ─→ needs C.1
  D.4 ─→ needs nothing (can start early)
  E.1 ─→ needs A.3, B.2
  E.2 ─→ needs A.3, B.2
  E.3 ─→ needs E.1
  E.4 ─→ needs nothing
  F.1 ─→ needs A.3
  F.2 ─→ needs A.3, B.1
  F.3 ─→ needs B.1
  F.4 ─→ needs B.3
  F.5 ─→ needs A.3
  F.6 ─→ needs A.3
  F.7 ─→ needs B.1

Wave 3 (sequential, depends on Wave 2):
  G.1 ─→ needs D.2
  G.2 ─→ needs D.2, E.2
  G.3 ─→ needs D.3, F.7
  G.4 ─→ needs D.3
  G.5 ─→ needs D.1, D.2, D.3, D.4, G.1, G.2, G.3, G.4

Wave 4 (sequential, depends on Wave 3):
  H.1 ─→ needs A.1-A.4
  H.2 ─→ needs D.2, G.1
  H.3 ─→ needs E.1
  H.4 ─→ needs F.2
  H.5 ─→ needs F.1-F.7
  H.6 ─→ needs D.1, G.5
  H.7 ─→ needs H.1-H.6
```

## Estimated Effort

| Wave | Tasks | Estimated Hours | Can Parallelize |
|------|-------|-----------------|-----------------|
| 1    | 10    | 8-12            | Yes (A, B, C)   |
| 2    | 15    | 16-24           | Yes (D, E, F)   |
| 3    | 5     | 8-12            | Partially       |
| 4    | 7     | 12-16           | No              |
| **Total** | **37** | **44-64**  |                 |

# Tasks: Fix UI Bugs

**Input**: Design documents from `/specs/003-fix-ui-bugs/`
**Tests**: MANDATORY FIRST — TDD per constitution and user's explicit instruction.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: P1 Bug Fixes (Critical)

### Bug 1: Filter on Main Menu

- [x] T001 [US1] Test: pressing `/` on main menu activates filter mode, typing filters menu items in `tests/unit/qa_bug_fixes_test.go`
- [x] T002 [US1] Fix filter activation in `internal/app/app.go` — enable filter mode for MainMenuView, not just ResourceListView

### Bug 2: S3 Navigation Position

- [x] T003 [US2] Test: drilling into S3 bucket and pressing Escape restores previous SelectedIndex in `tests/unit/qa_bug_fixes_test.go`
- [x] T004 [US2] Fix navigation stack in `internal/app/app.go` — store SelectedIndex before push, restore on pop

### Bug 3: Y Key → YAML

- [x] T005 [US3] Test: pressing `y` on a resource produces YAML output (no `{` or `[` at start) in `tests/unit/qa_bug_fixes_test.go`
- [x] T006 [US3] Convert JSON view to YAML in `internal/views/jsonview.go` — use yaml.Marshal with ToSafeValue instead of json.MarshalIndent
- [x] T007 [US3] Update `internal/app/app.go` — pass RawStruct to YAML view, rename references

### Bug 4: Context-Aware Copy

- [x] T008 [US4] Test: `c` in list view copies resource ID, `c` in detail view copies full rendered content in `tests/unit/qa_bug_fixes_test.go`
- [x] T009 [US4] Fix copy handler in `internal/app/app.go` — branch by CurrentView: list→ID, detail→rendered detail text

### Bug 5: Status Bar at Bottom

- [x] T010 [US5] Test: View() output always has status bar at the last line, padding with empty lines if content is short in `tests/unit/qa_bug_fixes_test.go`
- [x] T011 [US5] Fix `View()` in `internal/app/app.go` — pad content to fill terminal height before appending status bar

### Bug 8: All Views Scrollable

- [x] T012 [US8] Test: detail view supports horizontal scroll (Right/Left keys shift content) in `tests/unit/qa_bug_fixes_test.go`
- [x] T013 [US8] Add horizontal scroll support to `internal/views/detail.go` — HScrollOffset field, apply in viewConfig/viewLegacy

### Bug 9: Horizontal Scroll Clamped

- [x] T014 [US9] Test: scrolling right beyond content width doesn't increase offset; scrolling left immediately works in `tests/unit/qa_bug_fixes_test.go`
- [x] T015 [US9] Fix scroll clamping in `internal/app/app.go` — clamp HScrollOffset at assignment time, not just at render

### Bug 13: Detail View Improvements

- [x] T016 [US13] Test: detail title has no " - Detail" suffix in `tests/unit/qa_bug_fixes_test.go`
- [x] T017 [US13] Test: `w` key toggles wrap mode, wrap resets horizontal scroll in `tests/unit/qa_bug_fixes_test.go`
- [x] T018 [US13] Test: YAML renders `Key: value` (colon right after key) in `tests/unit/qa_bug_fixes_test.go`
- [x] T019 [US13] Fix detail title in `internal/app/app.go` — remove " - Detail" from NewConfigDetailModel/NewDetailModel calls
- [x] T020 [US13] Add wrap toggle to `internal/views/detail.go` — WrapEnabled bool, `w` key handler, render wrapped lines
- [x] T021 [US13] Fix YAML key formatting in `internal/views/detail.go` — `Key: value` not `Key    : value`

### Bug 15: Config Column Widths

- [x] T022 [US15] Test: column widths from config are used in rendering (not overridden by data expansion) in `tests/unit/qa_bug_fixes_test.go`
- [x] T023 [US15] Fix width handling in `internal/app/app.go` renderResourceList — use configured width as fixed, don't expand based on data

---

## Phase 2: P2 Bug Fixes

### Bug 6: Status Clear on Navigation

- [x] T024 [US6] Test: status message clears when navigating to a different view in `tests/unit/qa_bug_fixes_test.go`
- [x] T025 [US6] Fix: clear StatusMessage in all navigation transitions in `internal/app/app.go`

### Bug 7: Breadcrumbs Without "main"

- [x] T026 [US7] Test: breadcrumbs show "main" only on main menu, omit "main >" elsewhere in `tests/unit/qa_bug_fixes_test.go`
- [x] T027 [US7] Fix updateBreadcrumbs in `internal/app/app.go` — skip "main" prefix for non-main views

### Bug 10: Scroll Reset on Navigation

- [x] T028 [US10] Test: horizontal scroll resets to 0 when navigating between views in `tests/unit/qa_bug_fixes_test.go`
- [x] T029 [US10] Fix: reset HScrollOffset to 0 in all navigation transitions in `internal/app/app.go`

### Bug 11: Context-Sensitive Help

- [x] T030 [US11] Test: help screen shows different keys for list view vs detail view in `tests/unit/qa_bug_fixes_test.go`
- [x] T031 [US11] Refactor `internal/ui/help.go` — accept ViewType parameter, render only relevant keys per view

### Bug 12: Header Styling

- [x] T032 [US12] Test: header has no blue background, version on right side in `tests/unit/qa_bug_fixes_test.go`
- [x] T033 [US12] Fix header rendering in `internal/ui/header.go` or `internal/app/app.go` — remove blue background style, move version right

### Bug 14: Resource Count in Breadcrumbs

- [x] T034 [US14] Test: breadcrumbs include count "(139)" and duplicate title line is removed in `tests/unit/qa_bug_fixes_test.go`
- [x] T035 [US14] Fix breadcrumbs in `internal/app/app.go` — add count to breadcrumb, remove duplicate title from renderResourceList

---

## Phase 3: Polish

- [x] T036 Run `go vet ./...` and fix warnings
- [x] T037 Run `go test ./tests/unit/ -count=1 -timeout 120s` — all tests pass
- [x] T038 Bump version and rebuild binary

---

## Dependencies

- Phase 1 bugs are independent — can be fixed in any order
- Phase 2 bugs are independent — can be fixed in any order
- Phase 3 depends on all fixes complete
- Within each bug: test FIRST, then fix

## Notes

- TESTS FIRST for every single bug fix
- Bump version and rebuild binary after ALL fixes
- Test ALL 8 resource types where applicable

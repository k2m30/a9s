# View-Model State Inventory (PR-B1)

This is the **contract PR-C implements against**. Every mutable field in every TUI view
model is classified into one of four buckets; PR-C lifts the CONTROLLER fields into
`app.ScreenState`, computes DERIVED fields at `Snapshot()` time, leaves RENDERER fields in
the TUI, and drops DELETE fields. It pairs with
[`web-ui-headless-controller-plan.md`](web-ui-headless-controller-plan.md) (§PR-B1, §PR-C).

Buckets:

- **CONTROLLER** — behavior-driving state that must survive across renderers → `ScreenState`.
- **DERIVED** — not stored; computed at `Snapshot()` from controller state + `runtime.Core`
  (e.g. visible rows after filter+sort, badge counts, frame title) → `ViewState`.
- **RENDERER** — never crosses the seam (Bubbles `viewport`/`textinput`/`spinner`, Lipgloss
  styles, terminal width/height, cached rendered strings, immutable per-view config).
- **DELETE** — dead/redundant; not carried forward.

## Per-model classification (CONTROLLER + notable others)

### MainMenuModel (`internal/tui/views/menu.go`)

| Field | Bucket | Note |
|-------|--------|------|
| `filterText` | CONTROLLER | substring filter (≥2 chars) |
| `scroll.cursor` | CONTROLLER | selected item |
| `scrollOffset` | CONTROLLER | category-aware viewport offset |
| `AttentionFilter.enabled` | CONTROLLER | ctrl+z issue-only toggle |
| `availability` / `truncated` | CONTROLLER | per-type counts + lower-bound flags (persisted in cache) |
| `issueCounts` / `issueKnown` / `issueTruncated` | CONTROLLER | per-type badge state (persisted) |
| `filteredItems`, `availChecked/Total`, `enrichChecked/Total` | DERIVED | recomputed from above + task progress |
| `allItems`, `width/height`, `keys`, `renderLinesCache` | RENDERER | catalog/layout/caches |

### ResourceListModel (`internal/tui/views/resourcelist.go`)

| Field | Bucket | Note |
|-------|--------|------|
| `scroll.cursor`, `hScrollOffset` | CONTROLLER | selected row + horizontal scroll |
| `sortColIdx`, `sortAsc` | CONTROLLER | active sort |
| `filterText`, `AttentionFilter.enabled` | CONTROLLER | filter + attention toggle |
| `relatedIDSet`, `fetchFilter` | CONTROLLER | related-nav prefilter + server-side filter params |
| `autoOpenSingleDetail` | CONTROLLER | auto-open detail when one row remains (see open decision) |
| `parentContext` | CONTROLLER→**ScreenContext** | child-fetcher params; contextual, immutable per list |
| `displayName`, `titleSuffix`, `escPops`, `showIssueBadge` | CONTROLLER | child/pivot framing + Esc semantics |
| `pagination` | CONTROLLER | store full `PaginationMeta`, not just cursor (need "has more" + token) |
| `reapplyChecker` + `reapplySource` | CONTROLLER | related checker re-run on each page load |
| `allResources`, `filteredResources`, `issueCount`, `pendingFilter`, `findingsByID`, `truncatedByID`, `enrichment*` | DERIVED | fetch/enrichment results, recomputed |
| `loading`, `loadingMore`, `spinner`, `styledRowCache`, `typeDef`, `viewConfig`, `width/height`, `keys` | RENDERER | lifecycle/widgets/config |

### DetailModel (`internal/tui/views/detail.go`) + rightColumnModel

| Field | Bucket | Note |
|-------|--------|------|
| `wrap` | CONTROLLER | wrap toggle |
| `search.query` + `search.currentIdx` | CONTROLLER | search query + match cursor |
| `rightColVisible` | CONTROLLER | related panel shown |
| `rightCol.focused` | CONTROLLER | related panel has focus |
| `rightCol.cursor`, `rightCol.scrollOffset` | CONTROLLER | related row selection + scroll |
| `rightCol.filterQuery`, `rightCol.filterActive` | CONTROLLER | related-panel filter |
| `rightCol.rows[].checker` | CONTROLLER | per-row checker for load-more reapply |
| `fieldCursor` | CONTROLLER? | **open decision** — only if it must survive detail→detail pivots |
| `res`, `fieldList`, `rightCol.rows[]` (data) | DERIVED | resource + computed fields + related results |
| `viewport`, `ready`, `rightColAutoShown/UserToggled/Width`, `pendingRelatedDispatch`, `plainMode`, `navProvider`, `resourceType`, `viewConfig`, `width/height`, `keys` | RENDERER | widgets/layout/config |

### YAML/JSON Text viewers (`internal/tui/views/yaml.go`)

| Field | Bucket | Note |
|-------|--------|------|
| `wrap` | CONTROLLER | wrap toggle |
| `search.query` + `search.currentIdx` | CONTROLLER | search + cursor |
| `res` | DERIVED | resource data |
| `viewport`, `ready`, `rawText`, `rawTitle`, `resourceType`, `width/height`, `keys` | RENDERER | widgets/mode/config |

### SelectorModel (`internal/tui/views/selector.go`)

| Field | Bucket | Note |
|-------|--------|------|
| `filterText`, `scroll.cursor` | CONTROLLER | filter + selection |
| `filteredItems` | DERIVED | recomputed from `allItems` + filter |
| `allItems`, `activeItem`, `title`, `onSelect`, `width/height`, `keys` | RENDERER | data/config |

### Reveal / Help / Identity (`reveal.go`, `help.go`, `identity.go`)

Transient modals — **no persistent ScreenState**. All fields RENDERER/context. Identity's
fetching latch already lives in `session` (`Core.IdentityFetching()`); rendering the loading
flag into `IdentityBody` is the only PR-C touch. Reveal `wrap` is RENDERER (transient modal).

## Consolidated proposed `ScreenState` (PR-C target)

```go
type ScreenState struct {
    List     *ListState     `json:"list,omitempty"`
    Detail   *DetailState   `json:"detail,omitempty"`
    Text     *TextState     `json:"text,omitempty"`
    Menu     *MenuState     `json:"menu,omitempty"`
    Selector *SelectorState `json:"selector,omitempty"`
    // Help/Identity carry no local state; context only.
}

type ListState struct {
    Filter           string              `json:"filter,omitempty"`
    SortCol          string              `json:"sort_col,omitempty"`
    SortDir          string              `json:"sort_dir,omitempty"`   // "asc" | "desc"
    SelectedRow      int                 `json:"selected_row"`
    ScrollX, ScrollY int                 `json:"scroll_x"`             // ScrollY tagged separately
    AttentionOnly    bool                `json:"attention_only,omitempty"`
    PaginationCursor string              `json:"pagination_cursor,omitempty"`
    HasPagination    bool                `json:"has_pagination,omitempty"`
    ShowIssueBadge   bool                `json:"show_issue_badge,omitempty"`
    RelatedIDSet     map[string]struct{} `json:"related_id_set,omitempty"`
    FetchFilter      map[string]string   `json:"fetch_filter,omitempty"`
    AutoOpenSingle   bool                `json:"auto_open_single,omitempty"`
    ReapplySource    *Resource           `json:"reapply_source,omitempty"` // + checker held controller-side
    DisplayName      string              `json:"display_name,omitempty"`
    TitleSuffix      string              `json:"title_suffix,omitempty"`
    EscPops          bool                `json:"esc_pops,omitempty"`
}

type DetailState struct {
    SearchQuery          string `json:"search_query,omitempty"`
    SearchCursor         int    `json:"search_cursor"`
    Wrap                 bool   `json:"wrap,omitempty"`
    ScrollY              int    `json:"scroll_y"`
    RelatedVisible       bool   `json:"related_visible,omitempty"`
    RelatedFocus         bool   `json:"related_focus,omitempty"`
    RightColFilter       string `json:"right_col_filter,omitempty"`
    RightColFilterActive bool   `json:"right_col_filter_active,omitempty"`
    RightColCursor       int    `json:"right_col_cursor"`
    RightColScrollY      int    `json:"right_col_scroll_y"`
    FieldCursor          int    `json:"field_cursor,omitempty"` // pending open decision
}

type TextState struct {
    Search       string `json:"search,omitempty"`
    SearchCursor int    `json:"search_cursor,omitempty"`
    Wrap         bool   `json:"wrap,omitempty"`
    ScrollY      int    `json:"scroll_y"`
}

type MenuState struct {
    Filter         string          `json:"filter,omitempty"`
    Cursor         int             `json:"cursor"`
    ScrollOffset   int             `json:"scroll_offset"`
    AttentionOnly  bool            `json:"attention_only,omitempty"`
    Availability   map[string]int  `json:"availability,omitempty"`
    Truncated      map[string]bool `json:"truncated,omitempty"`
    IssueCounts    map[string]int  `json:"issue_counts,omitempty"`
    IssueKnown     map[string]bool `json:"issue_known,omitempty"`
    IssueTruncated map[string]bool `json:"issue_truncated,omitempty"`
}

type SelectorState struct {
    Filter string `json:"filter,omitempty"`
    Cursor int    `json:"cursor"`
}
```

## Cross-cutting risks for PR-C

1. **Task-coupled fields.** `fetchFilter`, `relatedIDSet`, `reapplyChecker`+`reapplySource`,
   `parentContext` are read at fetch-dispatch time. Keep them controller-owned and have the
   controller extract them before dispatch — do NOT let `ExecuteTask` read `ScreenState`
   (that would re-introduce the renderer coupling PR-B0 removed). `parentContext` belongs in
   `ScreenContext` (immutable per child list), not mutable `ScreenState`.
2. **Enrichment maps are DERIVED, app-owned.** `findingsByID`/`truncatedByID`/`enrichment*`
   move to an app-owned enrichment store keyed by screen + resource ID, not per-list state;
   row render looks them up at `Snapshot()`.
3. **Related-check reapply.** On `ResourcesLoaded`, if `reapplySource`+checker are set, re-run
   the checker against the new page and merge into `RelatedIDSet`. This is the load-more path.
4. **Pagination needs meta, not just a cursor** — store `HasPagination` so the controller
   knows whether to dispatch load-more vs a fresh fetch.
5. **Shared sub-models** (`scroll`, `AttentionFilter`, `SearchModel`) appear in multiple view
   models — lift the value (cursor / enabled / query+cursor) into each owning `*State`.

## Open decisions (confirm during PR-C, do not guess)

- **`fieldCursor` (Detail)** — CONTROLLER only if the structured-field cursor must survive a
  detail→detail pivot (related navigation). Trace the pivot path first; if it doesn't survive,
  it's RENDERER (recomputed each render). Default lean: CONTROLLER, cheap to persist.
- **`autoOpenSingleDetail`** — set by related-nav, consumed once on data load, never cleared
  in current code. Lift as CONTROLLER but define the clear semantics (clear after the
  auto-open fires, so re-entry doesn't silently auto-open again).
- **`wrap` persistence scope** — per-view (Detail/Text) CONTROLLER; confirm whether it should
  also be a session-wide user preference (out of scope here).

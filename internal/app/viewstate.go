package app

import "github.com/k2m30/a9s/v3/internal/domain"

// ViewState is the renderer-agnostic snapshot that both the TUI and web
// renderers consume and that integration tests assert on. It carries no
// Lipgloss, Bubble Tea, or AWS SDK types — only scalars, slices, and
// structs with JSON tags so it round-trips through encoding/json cleanly.
//
// PR-C fills the Body fields from live controller state. In this PR
// (PR-A) Snapshot() returns a zero-value Body with the correct BodyKind
// derived from the top-of-stack screen.
type ViewState struct {
	Header     Header   `json:"header"`
	FrameTitle string   `json:"frame_title"`
	Footer     []KeyHint `json:"footer,omitempty"`
	HelpContext string   `json:"help_context,omitempty"`
	Body        Body     `json:"body"`
}

// Header mirrors the top bar rendered by internal/tui/layout.
type Header struct {
	Version          string `json:"version"`
	Profile          string `json:"profile"`
	Region           string `json:"region"`
	Mode             string `json:"mode,omitempty"` // "" | "demo" | "web"
	RightSide        string `json:"right_side,omitempty"`
	Flash            Flash  `json:"flash,omitzero"`
	ErrorHintVisible bool   `json:"error_hint_visible,omitempty"`
}

// Flash is the transient status-bar notification.
type Flash struct {
	Text    string `json:"text,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

// KeyHint is one entry in the footer key-binding bar.
type KeyHint struct {
	Key  string `json:"key"`
	Help string `json:"help"`
}

// BodyKind is the discriminator that identifies which Body pointer field
// is populated. It matches the screen kind, not the ScreenID, so renderers
// can switch on it without knowing every registered ScreenID.
type BodyKind string

const (
	BodyKindList     BodyKind = "list"
	BodyKindDetail   BodyKind = "detail"
	BodyKindText     BodyKind = "text"
	BodyKindMenu     BodyKind = "menu"
	BodyKindSelector BodyKind = "selector"
	BodyKindHelp     BodyKind = "help"
	BodyKindIdentity BodyKind = "identity"
	BodyKindUnknown  BodyKind = "unknown"
)

// Body is the tagged union for the screen-kind-specific body content.
// Exactly one of the pointer fields is non-nil; Kind is always set.
type Body struct {
	Kind     BodyKind      `json:"kind"`
	List     *ListBody     `json:"list,omitempty"`
	Detail   *DetailBody   `json:"detail,omitempty"`
	Text     *TextBody     `json:"text,omitempty"`
	Menu     *MenuBody     `json:"menu,omitempty"`
	Selector *SelectorBody `json:"selector,omitempty"`
	Help     *HelpBody     `json:"help,omitempty"`
	Identity *IdentityBody `json:"identity,omitempty"`
}

// ColumnDef describes one column in a list or child-list view.
type ColumnDef struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Width int    `json:"width"`
	Path  string `json:"path,omitempty"`
}

// RowDecorator is a short tag that renderers use to apply per-row
// formatting: "!" = attention/error, "~" = warning, "" = normal.
type RowDecorator string

const (
	DecoratorError   RowDecorator = "!"
	DecoratorWarning RowDecorator = "~"
	DecoratorNormal  RowDecorator = ""
)

// ListRow is one row in a resource-list body.
type ListRow struct {
	Cells      []string     `json:"cells"`
	Decorator  RowDecorator `json:"decorator,omitempty"`
	Severity   string       `json:"severity,omitempty"`
	ResourceID string       `json:"resource_id,omitempty"`
	// Color is the pre-resolved row color tag: "healthy", "warning", "broken",
	// "dim", or "" (normal/no-color). Populated by buildListBody so RenderList
	// can reproduce the exact lipgloss.Style that View() derives from
	// td.ResolveColor(r) without needing a live resource.Resource or typeDef.
	Color string `json:"color,omitempty"`
}

// SortSpec describes the active sort in a list view.
type SortSpec struct {
	Col string `json:"col"`
	Dir string `json:"dir"` // "asc" | "desc"
}

// PaginationInfo describes whether additional pages are available.
type PaginationInfo struct {
	HasMore bool   `json:"has_more,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
}

// ListBody is the body of a resource-list screen.
type ListBody struct {
	Columns             []ColumnDef               `json:"columns,omitempty"`
	Rows                []ListRow                 `json:"rows,omitempty"`
	Selected            int                       `json:"selected"`
	ScrollX             int                       `json:"scroll_x"`
	Filter              string                    `json:"filter,omitempty"`
	Sort                SortSpec                  `json:"sort,omitzero"`
	AttentionOnly       bool                      `json:"attention_only,omitempty"`
	Loading             bool                      `json:"loading,omitempty"`
	Truncated           bool                      `json:"truncated,omitempty"`
	Pagination          PaginationInfo            `json:"pagination,omitzero"`
	EnrichmentFindings  map[string]domain.Finding `json:"enrichment_findings,omitempty"`
	EnrichmentTruncated map[string]bool           `json:"enrichment_truncated,omitempty"`
	// MarkerCol is the full-column-list index (before hscroll) of the identity
	// column that receives the enrichment-finding glyph ("! "/"~ ") prefix.
	// Pre-computed by buildListBody so RenderList does not need typeDef.
	MarkerCol int `json:"marker_col"`
	// LoadingMore is true while an m-key load-more fetch is in flight.
	LoadingMore bool `json:"loading_more,omitempty"`
}

// FieldRow is one key-value pair in a detail view, extended with render-time
// metadata so RenderDetail can reproduce every style branch that
// renderFromFieldList applies without re-running the projector pipeline.
type FieldRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	// IsSection is true for section-header items (rendered with FindingSectionDefault
	// or tier-colored FindingSectionStopped/FindingSectionPending style).
	IsSection bool `json:"is_section,omitempty"`
	// IsHeader is true for sub-section header items (key: style, no value).
	IsHeader bool `json:"is_header,omitempty"`
	// IsSubField is true for indented sub-field items (indent via subFieldIndent).
	IsSubField bool `json:"is_sub_field,omitempty"`
	// IsSpacer is true for blank separator lines — rendered as "".
	IsSpacer bool `json:"is_spacer,omitempty"`
	// IsNavigable is true for navigable (hyperlink-style) fields.
	IsNavigable bool `json:"is_navigable,omitempty"`
	// TargetType is the canonical short name of the resource type this field
	// links to (e.g. "vpc", "sg", "ami"). Non-empty only when IsNavigable is true.
	TargetType string `json:"target_type,omitempty"`
	// NavID is the navigation ID for this field when it differs from Value
	// (e.g. ct-events Principal rows where Value is the display label). When
	// empty, Value is used as the target ID — mirrors fieldpath.FieldItem.NavID.
	NavID string `json:"nav_id,omitempty"`
	// IndentLevel is the sub-field indent depth (1 = phrase, 3 = detail rows).
	IndentLevel int `json:"indent_level,omitempty"`
	// ColorTier is the TierColorStyle selector: "!", "~", "ok", "ct-danger", etc.
	ColorTier string `json:"color_tier,omitempty"`
	// Path is the field path; "Attention" identifies entries in the attention section.
	Path string `json:"path,omitempty"`
}

// FindingRow is one row of supporting evidence for an attention finding.
type FindingRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// AttentionBlock groups the finding header with its supporting rows.
type AttentionBlock struct {
	Code     string       `json:"code"`
	Message  string       `json:"message"`
	Severity string       `json:"severity"`
	Rows     []FindingRow `json:"rows,omitempty"`
	// Tier is the display tier for coloring: "!" = broken/red, "~" = warning/yellow.
	Tier string `json:"tier,omitempty"`
	// RowBucket is the S2 row color bucket used to cap entry colors ("healthy",
	// "warning", "broken", "dim", ""). Set by buildDetailBody from td.ResolveColor.
	RowBucket string `json:"row_bucket,omitempty"`
}

// RelatedBlock is one related-resource panel entry.
type RelatedBlock struct {
	Name        string            `json:"name"`
	Count       int               `json:"count"`
	Items       []FieldRow        `json:"items,omitempty"`
	Loading     bool              `json:"loading,omitempty"`
	Err         bool              `json:"err,omitempty"`
	Approximate bool              `json:"approximate,omitempty"`
	FetchFilter map[string]string `json:"fetch_filter,omitempty"`
	// TargetType is the canonical short name of the target resource type.
	TargetType string `json:"target_type,omitempty"`
}

// DetailBody is the body of a resource-detail screen.
type DetailBody struct {
	// Fields is the ordered list of all rendered field rows (sections + kv pairs
	// + attention sub-rows + spacers), matching the fieldList that
	// renderFromFieldList iterates. RenderDetail iterates this slice directly.
	Fields        []FieldRow       `json:"fields,omitempty"`
	Attention     []AttentionBlock `json:"attention,omitempty"`
	Related       []RelatedBlock   `json:"related,omitempty"`
	RelatedFocused bool            `json:"related_focused,omitempty"`
	// RelatedVisible is true when the related panel should be shown. It mirrors
	// DetailState.RelatedVisible and is also set when the renderer auto-shows
	// the panel (RelatedRows non-nil or defs exist). Used by RenderDetail to
	// gate the side-by-side layout independently of len(Related)>0 (the panel
	// must show even while rows are loading, i.e. Related contains loading rows).
	RelatedVisible bool `json:"related_visible,omitempty"`
	// RelatedCursor is the index in Related of the currently-highlighted row.
	RelatedCursor int    `json:"related_cursor,omitempty"`
	// RelatedScroll is the scroll offset (first visible row index) in the
	// related panel. Together with RelatedCursor it lets renderDetailRelatedFromBody
	// reproduce the exact window that rightColumnModel.View() shows.
	RelatedScroll int `json:"related_scroll,omitempty"`
	// RelatedFilter is the active filter query in the related panel.
	RelatedFilter string `json:"related_filter,omitempty"`
	// RelatedFilterActive is true while the related panel filter input is open.
	RelatedFilterActive bool   `json:"related_filter_active,omitempty"`
	// RelatedSourceType is the short name of the source resource type, used
	// for self-pivot-zero filtering in the related panel.
	RelatedSourceType string `json:"related_source_type,omitempty"`
	Search        string `json:"search,omitempty"`
	SearchCursor  int    `json:"search_cursor,omitempty"`
	Wrap          bool   `json:"wrap,omitempty"`
	// ScrollY is the viewport top-line offset (mirrors DetailState.ScrollY).
	ScrollY       int    `json:"scroll_y,omitempty"`
	// FieldCursor is the index of the highlighted field row (for cursor-selection
	// rendering in RenderDetail).
	FieldCursor   int    `json:"field_cursor,omitempty"`
	// KeyWidth is the pre-computed key-column width so RenderDetail does not
	// need to scan Fields again.
	KeyWidth int `json:"key_width,omitempty"`
}

// SearchMatch is one highlighted match in a text screen.
type SearchMatch struct {
	Line   int `json:"line"`
	ColStart int `json:"col_start"`
	ColEnd   int `json:"col_end"`
}

// TextBody is the body of a YAML/JSON text screen.
type TextBody struct {
	Lines         []string      `json:"lines,omitempty"`
	SearchMatches []SearchMatch `json:"search_matches,omitempty"`
	Wrap          bool          `json:"wrap,omitempty"`
	// ScrollY is the current viewport Y offset (line index of the top visible line).
	ScrollY int `json:"scroll_y,omitempty"`
	// Search is the active query string (empty when no search is active).
	Search string `json:"search,omitempty"`
	// SearchCursor is the index of the currently-highlighted match.
	SearchCursor int `json:"search_cursor,omitempty"`
}

// IssueBadge is the issue-count badge shown on a menu entry.
type IssueBadge struct {
	Count     int  `json:"count"`
	Truncated bool `json:"truncated,omitempty"`
}

// MenuEntry is one entry in the main-menu body.
type MenuEntry struct {
	ShortName    string     `json:"short_name"`
	Display      string     `json:"display"`
	Alias        string     `json:"alias,omitempty"`
	Category     string     `json:"category,omitempty"`
	IssueBadge   IssueBadge `json:"issue_badge,omitzero"`
	Availability int        `json:"availability"`
	// AvailKnown distinguishes a known count (rendered with a "(N)" suffix;
	// confirmed-empty dims) from an unknown one (no suffix, normal style).
	// AvailTruncated drives the "(N+)" lower-bound suffix.
	AvailKnown     bool `json:"avail_known,omitempty"`
	AvailTruncated bool `json:"avail_truncated,omitempty"`
}

// MenuBody is the body of the main-menu screen.
type MenuBody struct {
	Entries       []MenuEntry `json:"entries,omitempty"`
	Selected      int         `json:"selected"`
	Filter        string      `json:"filter,omitempty"`
	AttentionOnly bool        `json:"attention_only,omitempty"`
	Progress      string      `json:"progress,omitempty"`
}

// SelectorBody is the body of a profile/region/theme selector screen.
type SelectorBody struct {
	// Items is the filtered visible item slice (after applying Filter to AllItems).
	Items    []string `json:"items,omitempty"`
	Selected int      `json:"selected"`
	// AllItems is the unfiltered full list, used by FrameTitle to show "N/M" counts.
	AllItems   []string `json:"all_items,omitempty"`
	Filter     string   `json:"filter,omitempty"`
	ActiveItem string   `json:"active_item,omitempty"`
	Title      string   `json:"title,omitempty"`
}

// HelpSection is one titled column of key hints rendered in the help overlay.
// Mirrors the helpGroup structure in internal/tui/views/help.go.
type HelpSection struct {
	Title string    `json:"title"`
	Hints []KeyHint `json:"hints,omitempty"`
}

// HelpBody is the body of the help overlay screen.
// Context names the view that opened help (e.g. "main-menu", "resource-list",
// "detail", "yaml") so renderers can filter or label sections appropriately.
type HelpBody struct {
	Context  string        `json:"context"`
	Sections []HelpSection `json:"sections,omitempty"`
}

// IdentityBody is the body of the caller-identity screen.
// Fields mirror internal/tui/views/IdentityData plus the session context
// (Profile, Region) that the TUI IdentityModel carries separately.
type IdentityBody struct {
	// Account section
	AccountID    string `json:"account_id,omitempty"`
	AccountAlias string `json:"account_alias,omitempty"`

	// Caller section — ARN is always present; role vs user fields are mutually exclusive.
	ARN           string `json:"arn,omitempty"`
	IsAssumedRole bool   `json:"is_assumed_role,omitempty"`
	RoleName      string `json:"role_name,omitempty"`
	SessionName   string `json:"session_name,omitempty"`
	UserName      string `json:"user_name,omitempty"`

	// Session context
	Profile string `json:"profile,omitempty"`
	Region  string `json:"region,omitempty"`

	// Loading/error lifecycle — exactly one of Loading or ErrorMsg is set when
	// the identity fetch is in progress or has failed.
	Loading  bool   `json:"loading,omitempty"`
	ErrorMsg string `json:"error_msg,omitempty"`
}

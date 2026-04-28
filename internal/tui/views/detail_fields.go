// detail_fields.go contains field list construction and field-list-based rendering for DetailModel.
// Specifically: buildFieldList and renderFromFieldList.
package views

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/projection"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// buildFieldList computes m.fieldList by delegating to the per-type DetailProjector
// (or projection.GenericWithConfig as fallback), then converts the returned
// []domain.Section into []fieldpath.FieldItem for the existing renderFromFieldList
// renderer.
func (m *DetailModel) buildFieldList() {
	// Inject type so projector can look up per-type metadata.
	r := m.res
	if r.Type == "" {
		r.Type = m.resourceType
	}
	td := resource.FindResourceType(m.resourceType)
	// Use m.navProvider to resolve navigable fields. The default (set in
	// NewDetail) is resource.GetActiveNavigableFields (ACTIVE-only), which
	// keeps tests isolated from init-time DEFAULT registry entries.
	// TUI construction paths override this with resource.GetNavigableFields
	// (merged ACTIVE+DEFAULT) via SetNavProvider.
	navProv := m.navProvider
	if navProv == nil {
		navProv = resource.GetActiveNavigableFields
	}
	generic := projection.GenericWithConfigAndNavProvider(m.viewConfig, navProv)

	var proj domain.DetailProjector
	if td != nil && td.Project != nil {
		proj = td.Project
	} else {
		proj = generic
	}
	sections := proj(r)
	// Fallback: a custom projector may legitimately return nil for resource
	// shapes it can't render (e.g. ctevent.Project against a stub ct-events
	// resource that only has ID/Name from a related-cache hit, with no raw
	// event body). Without this fallback the detail pane regresses to
	// "No detail data available". The generic projector renders such stubs
	// from r.Fields just fine.
	if len(sections) == 0 && td != nil && td.Project != nil {
		sections = generic(r)
	}
	if td != nil && td.Augment != nil {
		sections = td.Augment(r, sections)
	}
	m.fieldList = sectionsToFieldItems(sections)
	m.injectAttentionSection()
}

// sectionsToFieldItems converts []domain.Section to []fieldpath.FieldItem for
// the existing renderFromFieldList renderer.  Each section with a non-empty
// Title emits a leading FieldItem{IsSection: true}; then each domain.Item is
// converted via domainItemToFieldItem.
func sectionsToFieldItems(sections []domain.Section) []fieldpath.FieldItem {
	if len(sections) == 0 {
		return nil
	}
	var items []fieldpath.FieldItem
	for _, sec := range sections {
		if sec.Title != "" {
			items = append(items, fieldpath.FieldItem{
				IsSection: true,
				Key:       sec.Title,
				Path:      sec.Title,
			})
		}
		for _, it := range sec.Items {
			items = append(items, domainItemToFieldItem(it, sec.Title))
		}
	}
	return items
}

// domainItemToFieldItem maps a domain.Item back to a fieldpath.FieldItem so
// the unchanged renderFromFieldList renderer can consume projector output.
//
// Path is taken directly from it.Path when set, preserving the real field path
// from the projector. Fallback to synthesized paths is used only when it.Path
// is empty, maintaining backward-compatible behaviour for any Items constructed
// without a Path value.
func domainItemToFieldItem(it domain.Item, sectionTitle string) fieldpath.FieldItem {
	fi := fieldpath.FieldItem{
		Key:         it.Label,
		Value:       it.Value,
		Path:        it.Path,
		IsNavigable: it.Navigable,
		TargetType:  it.TargetType,
		ColorTier:   it.Tier,
		NavID:       it.NavID,
	}
	if fi.Path == "" {
		fi.Path = sectionTitle + "." + it.Label
	}
	switch it.Kind {
	case domain.ItemHeader:
		fi.IsHeader = true
		if it.Path == "" {
			fi.Path = it.Label // headers use their own label as path (matches ExtractFieldList)
		}
	case domain.ItemSubfield:
		fi.IsSubField = true
		fi.IndentLevel = it.IndentLevel
		if it.Label == "" {
			// Raw YAML continuation line. Legacy buildFieldList convention:
			// raw lines have Key == Value so renderFromFieldList takes the
			// plain-line branch (no stray ": " prefix).
			fi.Key = it.Value
			// Path: no trailing dot for empty-label subfields.
			if it.Path == "" {
				fi.Path = sectionTitle
			}
		}
	case domain.ItemSpacer:
		fi.IsSpacer = true
	}
	return fi
}

// injectAttentionSection prepends a unified "Attention" section to the field
// list when the resource has one or more active signals — either a Wave 1
// phrase in m.res.Issues or a Wave 2 m.enrichmentFinding. Spec §4 universal
// rule 7: every finding must remain individually visible across S2–S5. The
// list view's Status column shows only the top phrase (with optional `(+N)`
// suffix for multi-finding rows); this section surfaces the full set so the
// operator sees everything at a 3am glance without hunting through raw SDK
// fields.
//
// Entry order: "!" (broken) first, then "~" and others. Within a tier,
// m.res.Issues retains §4 precedence order, with the Wave 2 finding appended
// last at its own tier.
//
// Section header: "Attention (N)" where N is the total entry count. Omitted
// when N == 0 (truly healthy rows).
//
// Color-cap invariant: the glyph (`!`/`~`) carries severity, the color carries
// state. An Attention entry's rendered color must not exceed the row's S2
// color bucket — a Healthy (green) row with a `!` Wave-2 finding keeps the
// `!` glyph but renders in `~` yellow, never red. Otherwise the detail view
// contradicts the list: row-green in S2 and entry-red in S5 tells the operator
// conflicting things about the same resource. The cap applies uniformly
// across resource types (dbc, ec2, ecr, …) — no per-type branching.
func (m *DetailModel) injectAttentionSection() {
	type entry struct {
		tier          string
		primary       string
		rows          []domain.DetailRow
		splitKeyValue bool // when true: Key=primary (raw phrase), Value=glyph+capitalizedPhrase (display)
	}
	var entries []entry
	// PR-03a-views: read r.Findings + r.AttentionDetails when Findings is populated.
	// Fall back to legacy r.Issues + m.enrichmentFinding when Findings is empty.
	if len(m.res.Findings) > 0 {
		for _, f := range m.res.Findings {
			// Only surface issue-severity findings in the Attention section.
			// SevOK / SevDim findings represent healthy / informational states
			// that carry no actionable signal — including them would create a
			// spurious "Attention (1)" block on every healthy resource whose
			// Status was promoted to a Findings entry by DeriveFindings.
			if !f.Severity.IsIssue() {
				continue
			}
			var tier string
			switch f.Severity {
			case domain.SevBroken:
				tier = "!"
			default:
				tier = "~"
			}
			var rows []domain.DetailRow
			if m.res.AttentionDetails != nil {
				if det, ok := m.res.AttentionDetails[f.Code]; ok {
					rows = det.Rows
				}
			}
			// splitKeyValue=true: Key holds the raw phrase (for search and clipboard),
			// Value holds the glyph+capitalized phrase (for TUI display). PlainContent
			// renders "raw phrase: ! Capitalized phrase" so callers can match against
			// the original lowercase text; the TUI viewport renders only the Value to
			// keep the line short enough to fit the viewport.
			entries = append(entries, entry{tier: tier, primary: f.Phrase, rows: rows, splitKeyValue: true})
		}
	} else {
		for _, phrase := range m.res.Issues {
			entries = append(entries, entry{tier: phraseTier(m.resourceType, phrase), primary: phrase})
		}
		if m.enrichmentFinding != nil && m.enrichmentFinding.Summary != "" {
			enrichRows := make([]domain.DetailRow, len(m.enrichmentFinding.Rows))
			for i, r := range m.enrichmentFinding.Rows {
				enrichRows[i] = domain.DetailRow{Label: r.Label, Value: r.Value, Tier: r.Tier}
			}
			entries = append(entries, entry{
				tier:    m.enrichmentFinding.Severity,
				primary: m.enrichmentFinding.Summary,
				rows:    enrichRows,
			})
		}
	}
	if len(entries) == 0 {
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].tier == "!" && entries[j].tier != "!"
	})
	// Resolve the row's S2 color bucket once; used to cap entry colors below.
	rowBucket := resolveRowColorBucket(m.resourceType, m.res)
	headerTier := "~"
	for _, e := range entries {
		if e.tier == "!" {
			headerTier = "!"
			break
		}
	}
	items := make([]fieldpath.FieldItem, 0, 1+len(entries)*2)
	items = append(items, fieldpath.FieldItem{
		IsSection: true,
		Key:       fmt.Sprintf("Attention (%d)", len(entries)),
		Path:      "Attention",
		ColorTier: capTierToRowBucket(headerTier, rowBucket),
	})
	for _, e := range entries {
		glyph := e.tier
		if glyph != "!" && glyph != "~" {
			glyph = "~"
		}
		displayPhrase := capitalizeFirst(e.primary)
		line := glyph + " " + displayPhrase
		entryColor := capTierToRowBucket(e.tier, rowBucket)
		itemKey := line
		itemValue := line
		if e.splitKeyValue {
			// Findings path: Key = raw phrase (for search/clipboard),
			// Value = glyph + capitalized phrase (for TUI display).
			// TUI rendering uses only Value (short, fits viewport) when
			// Path=="Attention" and not in plainMode. PlainContent
			// (plainMode=true) renders "Key: Value" so the raw lowercase
			// phrase is present alongside the capitalized display form.
			itemKey = e.primary
			itemValue = line
		}
		items = append(items, fieldpath.FieldItem{
			IsSubField:  true,
			IndentLevel: 1,
			Key:         itemKey,
			Value:       itemValue,
			Path:        "Attention",
			ColorTier:   entryColor,
		})
		for _, row := range e.rows {
			tier := row.Tier
			if tier == "" {
				tier = e.tier
			}
			items = append(items, fieldpath.FieldItem{
				IsSubField:  true,
				IndentLevel: 3,
				Key:         row.Label,
				Value:       row.Value,
				Path:        "Attention",
				ColorTier:   capTierToRowBucket(tier, rowBucket),
			})
		}
	}
	// Blank line below the Attention block so the section is visually separated
	// from identity / AWS fields that follow.
	items = append(items, fieldpath.FieldItem{IsSpacer: true, Path: "Attention"})
	m.fieldList = append(items, m.fieldList...)
}

// capitalizeFirst returns s with its first rune uppercased. Used for
// presentation in the Attention section — the underlying data (Resource.Issues
// phrases, EnrichmentFinding.Summary) stays canonical lowercase to match §4
// spec vocabulary; only the rendered entry is capitalized for readability.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// resolveRowColorBucket returns the row's S2 color bucket for the given
// resource. Used by the Attention renderer to cap per-entry colors so the
// detail view never shows severity beyond what the list row already signaled.
// Falls back to ColorHealthy when the resource type is unregistered — an
// unregistered type is by definition "we don't know how to classify", which
// means the safest default is to cap any entry to `~`.
func resolveRowColorBucket(resourceType string, r resource.Resource) resource.Color {
	td := resource.FindResourceType(resourceType)
	if td == nil {
		return resource.ColorHealthy
	}
	return td.ResolveColor(r)
}

// capTierToRowBucket returns the effective color-tier string for an Attention
// entry given its severity tier and the row's S2 color bucket.
//
// Rule: `!` (red) is only permitted when the row itself is Broken. On any
// other row (Healthy / Warning / Dim), a `!` severity tier is capped to `~`
// (yellow) for COLOR purposes only — the glyph in front of the phrase still
// shows `!` so the operator sees "important to open" vs "informational".
// The glyph carries severity, the color carries state; this function is the
// seam between them.
//
// Non-`!` tiers are passed through unchanged — `~`, `ok`, `ct-danger`,
// `ct-attention`, `ct-info`, and unknown tiers remain as-is. Unknown tiers
// render as neutral via TierColorStyle's default branch.
func capTierToRowBucket(tier string, rowBucket resource.Color) string {
	if tier == "!" && rowBucket != resource.ColorBroken {
		return "~"
	}
	return tier
}

// phraseTier classifies a Wave 1 S4 phrase into "!" (broken) or "~" (warning
// or info) by delegating to the resource type's Color function. Each type
// owns its broken-vocabulary — dbi knows about "storage-full", ec2 about
// "stopped", etc. — so this dispatcher stays universal.
//
// The phrase is injected as both r.Status and r.Fields["status"] because
// some Color funcs read one, some read the other. Trailing "(+N)" suffix is
// stripped inside the Color funcs themselves (via resource.StripFindingSuffix).
//
// Unknown resource types default to "~" (warning) — safe for info-only
// rendering without false-positive "!" coloring.
func phraseTier(resourceType, phrase string) string {
	td := resource.FindResourceType(resourceType)
	if td == nil {
		return "~"
	}
	probe := resource.Resource{
		Status: phrase,
		Fields: map[string]string{"status": phrase},
	}
	if td.ResolveColor(probe) == resource.ColorBroken {
		return "!"
	}
	return "~"
}

// subFieldIndent returns the left margin for a sub-field at the given indent level.
// Level 1 = 5 spaces, level 2 = 7 spaces, level 3 = 9 spaces, etc.
// This preserves hierarchical YAML indentation in the detail view.
func subFieldIndent(level int) string {
	if level < 1 {
		level = 1
	}
	return " " + strings.Repeat("  ", level+1)
}

// colorizeDetailLine applies detail view key/value styling to a raw YAML line.
// Leading whitespace is stripped — the caller provides indentation via subFieldIndent.
// Uses shared yamlLine tokenization so markers and spacing match plainDetailLine exactly.
func colorizeDetailLine(rawLine string) string {
	yl := parseYAMLLine(rawLine)
	if yl.Key != "" {
		s := yl.Dash + styles.DetailKey.Render(yl.Key+":")
		if yl.Value != "" {
			s += " " + styles.DetailVal.Render(yl.Value)
		}
		return s
	}
	return yl.Dash + styles.DetailVal.Render(yl.Raw)
}

// plainDetailLine formats a raw YAML line as plain text for cursor-row rendering.
// Leading whitespace is stripped — the caller provides indentation via subFieldIndent.
// Uses shared yamlLine tokenization so markers and spacing match colorizeDetailLine exactly.
func plainDetailLine(rawLine string) string {
	return parseYAMLLine(rawLine).plain()
}

// renderFromFieldList renders the structured field list to a string.
// Each FieldItem is rendered according to its type: header, sub-field, navigable, or normal.
// Bug3 fix: applies styles.RowSelected to the cursor row when left column is focused.
// Bug4 fix: suppresses NavigableField underline on the cursor row (RowSelected takes over).
func (m DetailModel) renderFromFieldList() string {
	if len(m.fieldList) == 0 {
		return styles.DimText.Render("  No detail data available")
	}
	// Collect top-level field paths for key width calculation.
	var topPaths []string
	for _, item := range m.fieldList {
		if !item.IsHeader && !item.IsSubField {
			topPaths = append(topPaths, item.Key)
		}
	}
	keyW := computeKeyWidth(topPaths)

	leftFocused := !m.rightCol.IsFocused()

	var lines []string
	for idx, item := range m.fieldList {
		isCursorRow := leftFocused && idx == m.fieldCursor
		var line string
		if item.IsSpacer {
			// Blank-line visual separator — skip all styling; the cursor is
			// also skipped on spacers by the detail cursor navigation.
			lines = append(lines, "")
			continue
		}
		if isCursorRow {
			// Render selected rows without nested foreground/underline styles so
			// labels remain legible on selection background across themes.
			switch {
			case item.IsSection:
				line = " " + item.Key // cursor on section header: plain text (cursor skip handled in detail.go)
			case item.IsHeader:
				line = " " + item.Key + ":"
			case item.IsSubField:
				indent := subFieldIndent(item.IndentLevel)
				// Navigable or injected sub-fields have Key != Value (pre-split by buildFieldList).
				// General sub-fields have Key == Value (raw YAML line).
				if item.Key != item.Value {
					line = indent + item.Key + ": " + item.Value
					break
				}
				// General sub-field: use YAML-style rendering (plain, no colors for cursor row).
				line = subFieldIndent(item.IndentLevel) + plainDetailLine(item.Value)
			default:
				line = " " + text.PadOrTrunc(item.Key+":", keyW) + item.Value
			}
		} else {
			switch {
			case item.IsSection:
				var sectionStyle lipgloss.Style
				switch item.ColorTier {
				case "!":
					sectionStyle = styles.FindingSectionStopped
				case "~":
					sectionStyle = styles.FindingSectionPending
				default:
					sectionStyle = styles.FindingSectionDefault
				}
				line = " " + sectionStyle.Render(item.Key)
			case item.IsHeader:
				line = " " + styles.DetailSection.Render(item.Key+":")
			case item.IsSubField:
				indent := subFieldIndent(item.IndentLevel)
				// Navigable sub-fields have Key != Value (pre-split by buildFieldList).
				if item.IsNavigable && item.Key != item.Value {
					line = indent + styles.DetailKey.Render(item.Key+":") + " " + styles.NavigableField.Render(item.Value)
					break
				}
				// Injected sub-fields with separate Key/Value (e.g., EC2 status checks).
				if item.Key != item.Value {
					// Attention-section entries use splitKeyValue: Key = raw phrase (for
					// search/clipboard), Value = glyph + capitalized phrase (for display).
					// In TUI mode (plainMode=false) render only the Value — the short form
					// fits the viewport without truncation. In plainMode (PlainContent/
					// clipboard) render Key: Value so the raw lowercase phrase is present
					// alongside the capitalized display form.
					if item.Path == "Attention" && !m.plainMode {
						val := item.Value
						if item.ColorTier != "" {
							val = styles.TierColorStyle(item.ColorTier).Render(val)
						}
						line = subFieldIndent(item.IndentLevel) + val
						break
					}
					val := item.Value
					if item.ColorTier != "" {
						val = styles.TierColorStyle(item.ColorTier).Render(val)
					}
					line = indent + styles.DetailKey.Render(item.Key+":") + " " + val
					break
				}
				// General sub-field: YAML-style colorization preserving hierarchy.
				// When ColorTier is set (Attention entries), apply tier coloring to the whole line.
				if item.ColorTier != "" {
					line = subFieldIndent(item.IndentLevel) + styles.TierColorStyle(item.ColorTier).Render(item.Value)
				} else {
					line = subFieldIndent(item.IndentLevel) + colorizeDetailLine(item.Value)
				}
			case item.IsNavigable:
				line = " " + styles.DetailKey.Render(text.PadOrTrunc(item.Key+":", keyW)) + styles.NavigableField.Render(item.Value)
			default:
				label := styles.DetailKey.Render(text.PadOrTrunc(item.Key+":", keyW))
				var value string
				if item.ColorTier != "" {
					value = styles.TierColorStyle(item.ColorTier).Render(item.Value)
				} else {
					value = styles.DetailVal.Render(item.Value)
				}
				line = " " + label + value
			}
		}
		// Bug3 fix: apply background highlight to the cursor row (left focused only).
		// Keep this as background-only to preserve existing ANSI contract checks.
		if isCursorRow {
			// Ensure selection background spans full viewport width, not just text width.
			if m.ready {
				targetW := m.viewport.Width()
				if w := lipgloss.Width(line); targetW > 0 && w < targetW {
					line += strings.Repeat(" ", targetW-w)
				}
			}
			line = lipgloss.NewStyle().Background(styles.ColRowSelectedBg).Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

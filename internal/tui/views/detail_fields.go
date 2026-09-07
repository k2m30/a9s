// SPDX-License-Identifier: GPL-3.0-or-later

// detail_fields.go contains field list construction and field-list-based rendering for DetailModel.
// Specifically: buildFieldList and renderFromFieldList.
package views

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/semantics/projection"
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

// subFieldIndent returns the left margin for a sub-field at the given indent
// level: level 1 is app.AttentionIndentColumns, each level below it two more.
// The controller wraps the Attention sentence against that same constant, so
// the sentence is measured against the margin it is actually painted at.
func subFieldIndent(level int) string {
	if level < 1 {
		level = 1
	}
	return strings.Repeat(" ", app.AttentionIndentColumns+2*(level-1))
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
					// Attention phrase rows (IndentLevel == 1) collapse to value-only on
					// the cursor row too — consistent with the non-cursor branch.
					// AttentionDetails rows (IndentLevel == 3) keep Key: Value labels.
					if item.Path == "Attention" && item.IndentLevel == 1 {
						line = indent + item.Value
						break
					}
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
					// Attention phrase rows (IndentLevel == 1) use splitKeyValue:
					// Key = raw phrase (for search/clipboard), Value = glyph +
					// capitalized phrase (for display). In TUI mode (plainMode=false)
					// render only the Value — the short form fits the viewport without
					// truncation. In plainMode (PlainContent/clipboard) render Key: Value
					// so the raw lowercase phrase is present alongside the capitalized
					// display form. IndentLevel == 1 distinguishes phrase rows from
					// AttentionDetails rows (IndentLevel == 3) which must always render
					// as Key: Value to preserve their labels (e.g. "Action: reboot").
					if item.Path == "Attention" && item.IndentLevel == 1 && !m.plainMode {
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

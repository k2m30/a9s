// SPDX-License-Identifier: GPL-3.0-or-later

// detail_fields.go contains field-list-based rendering for DetailModel.
// Specifically: renderFromFieldList.
package views

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

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
	keyW := computeKeyWidth(topPaths, m.width)

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
				// A row the projection gave no label is a whole line: there is
				// no key to put a colon after.
				if item.Key == "" {
					line = indent + item.Value
					break
				}
				// Navigable or injected sub-fields have Key != Value (pre-split by
				// projection.buildItems).
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
				// A row the projection gave no label is a whole line: there is
				// no key to put a colon after.
				if item.Key == "" {
					val := item.Value
					if item.ColorTier != "" {
						val = styles.TierColorStyle(item.ColorTier).Render(val)
					}
					line = indent + val
					break
				}
				// Navigable sub-fields have Key != Value (pre-split by projection.buildItems).
				if item.IsNavigable && item.Key != item.Value {
					line = indent + styles.DetailKey.Render(item.Key+":") + " " + styles.NavigableField.Render(item.Value)
					break
				}
				// Injected sub-fields with separate Key/Value (e.g., EC2 status checks).
				if item.Key != item.Value {
					// Attention phrase rows (IndentLevel == 1) use splitKeyValue:
					// Key = raw phrase, Value = glyph + capitalized phrase. Only the
					// Value is painted — the short form fits the viewport without
					// truncation. IndentLevel == 1 distinguishes phrase rows from
					// AttentionDetails rows (IndentLevel == 3) which must always render
					// as Key: Value to preserve their labels (e.g. "Action: reboot").
					if item.Path == "Attention" && item.IndentLevel == 1 {
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
				if w := text.Width(line); targetW > 0 && w < targetW {
					line += strings.Repeat(" ", targetW-w)
				}
			}
			line = lipgloss.NewStyle().Background(styles.ColRowSelectedBg).Render(line)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

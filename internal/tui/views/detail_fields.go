// detail_fields.go contains field list construction and field-list-based rendering for DetailModel.
// Specifically: buildFieldList and renderFromFieldList.
package views

import (
	"sort"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/aws/ctdetail"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// buildFieldList computes m.fieldList from the view config and navigable field registry.
// Sets m.fieldList to nil when no config or detail paths are available (falls through to renderFromConfig).
// After calling ExtractFieldList, post-processes sub-fields to mark navigable ones:
// a sub-field under path P whose key K matches navMap["P.K"] is marked IsNavigable
// with TargetType from the navMap, and its Value is set to the extracted sub-value.
func (m *DetailModel) buildFieldList() {
	if m.resourceType == "ct-events" && m.res.Status != "" {
		raw := extractRawCTEventJSON(m.res)
		if raw != "" {
			if event, err := ctdetail.Parse(raw); err == nil {
				event.Status = m.res.Status // propagate severity status into the parsed event
				sections := ctdetail.BuildSections(event)
				m.fieldList = sectionsToFieldItems(sections)
				return
			}
		}
		// Parse failure or missing JSON → fall through to legacy flat path
	}
	var detailPaths []string
	if m.viewConfig != nil {
		vd := config.GetViewDef(m.viewConfig, m.resourceType)
		detailPaths = vd.Detail
	}
	navFields := resource.GetNavigableFields(m.resourceType)
	navMap := make(map[string]string, len(navFields))
	for _, nf := range navFields {
		navMap[nf.FieldPath] = nf.TargetType
	}
	// When the resource has neither a Fields map nor a RawStruct, synthesize a minimal
	// Fields map from the resource's own ID/Name/Status so that bare resources (e.g.,
	// cached stubs navigated to directly) still render their key identifiers.
	// Only apply when RawStruct is nil — if RawStruct is present, ExtractFieldList will
	// extract the correct values from it directly.
	fields := m.res.Fields
	if len(fields) == 0 && m.res.RawStruct == nil && (m.res.ID != "" || m.res.Name != "" || m.res.Status != "") {
		fieldKeys := resource.GetFieldKeys(m.resourceType)
		synth := make(map[string]string, 3)
		if m.res.ID != "" && len(fieldKeys) > 0 {
			// First registered field key is the primary ID field (e.g., "subnet_id").
			synth[fieldKeys[0]] = m.res.ID
		}
		if m.res.Name != "" && len(fieldKeys) > 1 {
			synth[fieldKeys[1]] = m.res.Name
		}
		if m.res.Status != "" && len(fieldKeys) > 2 {
			synth[fieldKeys[2]] = m.res.Status
		}
		if len(synth) > 0 {
			fields = synth
		}
	}
	// Bridge snake_case fetcher output to canonical PascalCase view keys so
	// detail paths and navigable fields continue to work for registered resource types.
	fields = resource.ApplyFieldAliases(m.resourceType, fields)
	if len(detailPaths) == 0 {
		if len(fields) == 0 {
			m.fieldList = nil
			return
		}
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		m.fieldList = fieldpath.ExtractFieldList(nil, fields, keys, nil)
		if m.resourceType == "ec2" {
			m.injectEC2StatusChecks()
		}
		return
	}
	items := fieldpath.ExtractFieldList(m.res.RawStruct, fields, detailPaths, navMap)
	// Post-process: annotate sub-fields that match a navigable path.
	// ExtractFieldList only checks top-level paths; sub-fields need separate matching.
	// Track YAML indentation so nested values like BlockDeviceMappings.Ebs.VolumeId
	// remain navigable without being duplicated as top-level fields.
	currentPath := ""
	ancestorByLevel := map[int]string{}
	for i, item := range items {
		if item.IsHeader {
			currentPath = item.Path
			clear(ancestorByLevel)
			continue
		}
		if !item.IsSubField {
			continue
		}
		if item.Path != currentPath {
			currentPath = item.Path
			clear(ancestorByLevel)
		}
		rawLine := item.Value
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}
		level := 0
		if leading := len(rawLine) - len(strings.TrimLeft(rawLine, " ")); leading > 0 {
			level = leading / 2
		}
		hasDash := strings.HasPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "- ")
		subKey, subVal, hasSep := strings.Cut(trimmed, ":")
		if !hasSep {
			continue
		}
		subKey = strings.TrimSpace(subKey)
		subVal = strings.TrimSpace(subVal)
		for depth := range ancestorByLevel {
			if depth >= level {
				delete(ancestorByLevel, depth)
			}
		}
		pathParts := []string{item.Path}
		for depth := 0; depth < level; depth++ {
			if ancestor, ok := ancestorByLevel[depth]; ok && ancestor != "" {
				pathParts = append(pathParts, ancestor)
			}
		}
		pathParts = append(pathParts, subKey)
		composedPath := strings.Join(pathParts, ".")
		if tt, ok := navMap[composedPath]; ok && subVal != "" {
			items[i].IsNavigable = true
			items[i].TargetType = tt
			// Preserve the YAML list marker so the navigable row aligns
			// with sibling rows rendered via colorizeDetailLine.
			if hasDash {
				items[i].Key = "- " + subKey
			} else {
				items[i].Key = subKey
			}
			items[i].Value = subVal
		}
		if subVal == "" {
			ancestorByLevel[level] = subKey
		}
	}
	m.fieldList = items
	if m.resourceType == "ec2" {
		m.injectEC2StatusChecks()
	}
}

// extractRawCTEventJSON pulls the raw JSON string out of a ct-events resource.
// Returns "" if RawStruct is nil or not a cloudtrailtypes.Event or has nil CloudTrailEvent.
func extractRawCTEventJSON(res resource.Resource) string {
	if res.RawStruct == nil {
		return ""
	}
	ev, ok := res.RawStruct.(cloudtrailtypes.Event)
	if !ok {
		return ""
	}
	if ev.CloudTrailEvent == nil {
		return ""
	}
	return *ev.CloudTrailEvent
}

// sectionsToFieldItems flattens a []ctdetail.Section to []fieldpath.FieldItem.
// Emits one FieldItem{IsSection: true, Key: section.Name} per section header,
// followed by one FieldItem per Row with IsNavigable/TargetType/ColorTier propagated.
func sectionsToFieldItems(sections []ctdetail.Section) []fieldpath.FieldItem {
	var items []fieldpath.FieldItem
	for _, sec := range sections {
		items = append(items, fieldpath.FieldItem{
			IsSection: true,
			Key:       sec.Name,
			Path:      sec.Name,
		})
		for _, row := range sec.Rows {
			items = append(items, fieldpath.FieldItem{
				Key:         row.Key,
				Value:       row.Value,
				Path:        sec.Name + "." + row.Key,
				IsNavigable: row.IsNavigable,
				TargetType:  row.TargetType,
				ColorTier:   row.Severity,
				NavID:       row.NavID,
			})
		}
	}
	return items
}

// statusCheckTier maps an EC2 status check value to a ColorTier string
// for deferred styling via RowColorStyle in the render path.
func statusCheckTier(status string) string {
	switch status {
	case "ok":
		return "ok"
	case "impaired":
		return "impaired"
	case "initializing":
		return "initializing"
	default:
		return ""
	}
}

// injectEC2StatusChecks injects a "Status Checks" section into m.fieldList
// after the "State" section when the instance is running and checks are non-trivial.
func (m *DetailModel) injectEC2StatusChecks() {
	if len(m.fieldList) == 0 {
		return
	}
	// Only inject when instance is running.
	state := m.res.Fields["state"]
	if state != "running" {
		return
	}
	sysStatus := m.res.Fields["system_status"]
	instStatus := m.res.Fields["instance_status"]

	// Omit when both fields are empty.
	if sysStatus == "" && instStatus == "" {
		return
	}
	// Omit when both are "ok" (healthy — no noise).
	if sysStatus == "ok" && instStatus == "ok" {
		return
	}

	// Build the items to inject.
	sysVal := sysStatus
	if sysVal == "" {
		sysVal = "—"
	}
	instVal := instStatus
	if instVal == "" {
		instVal = "—"
	}
	inject := []fieldpath.FieldItem{
		{Key: "Status Checks", IsHeader: true, Path: "StatusChecks"},
		{Key: "System", Value: sysVal, IsSubField: true, Path: "StatusChecks", IndentLevel: 1, ColorTier: statusCheckTier(sysStatus)},
		{Key: "Instance", Value: instVal, IsSubField: true, Path: "StatusChecks", IndentLevel: 1, ColorTier: statusCheckTier(instStatus)},
	}

	// Find the insertion point: after the "State" section header and its sub-fields.
	insertAt := -1
	inStateSection := false
	for i, item := range m.fieldList {
		if item.IsHeader && item.Key == "State" {
			inStateSection = true
			continue
		}
		if inStateSection {
			if item.IsHeader {
				// Found the next section header — insert before it.
				insertAt = i
				break
			}
			// Continue scanning sub-fields of State section.
		}
	}
	if insertAt == -1 {
		// State section was last, or not found — append at end.
		m.fieldList = append(m.fieldList, inject...)
		return
	}

	// Insert at the found position.
	result := make([]fieldpath.FieldItem, 0, len(m.fieldList)+len(inject))
	result = append(result, m.fieldList[:insertAt]...)
	result = append(result, inject...)
	result = append(result, m.fieldList[insertAt:]...)
	m.fieldList = result
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
		if isCursorRow {
			// Render selected rows without nested foreground/underline styles so
			// labels remain legible on selection background across themes.
			switch {
			case item.IsSection:
				line = " " + item.Key // cursor on a section header: plain, no color (cursor skip is handled in detail.go)
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
				line = " " + lipgloss.NewStyle().Bold(true).Render(item.Key)
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
					val := item.Value
					if item.ColorTier != "" {
						val = styles.RowColorStyle(item.ColorTier).Render(val)
					}
					line = indent + styles.DetailKey.Render(item.Key+":") + " " + val
					break
				}
				// General sub-field: YAML-style colorization preserving hierarchy.
				line = subFieldIndent(item.IndentLevel) + colorizeDetailLine(item.Value)
			case item.IsNavigable:
				line = " " + styles.DetailKey.Render(text.PadOrTrunc(item.Key+":", keyW)) + styles.NavigableField.Render(item.Value)
			default:
				label := styles.DetailKey.Render(text.PadOrTrunc(item.Key+":", keyW))
				var value string
				if item.ColorTier != "" {
					value = styles.RowColorStyle(item.ColorTier).Render(item.Value)
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

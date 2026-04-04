// detail_fields.go contains field list construction and field-list-based rendering for DetailModel.
// Specifically: buildFieldList, augmentEC2AliasFields, and renderFromFieldList.
package views

import (
	"sort"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

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
	// Bridge snake_case EC2 fixture/runtime maps to canonical EC2 view keys so
	// detail paths and navigable fields continue to work when ResourceType is ec2.
	fields = augmentEC2AliasFields(m.resourceType, fields)
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
			items[i].Key = subKey
			items[i].Value = subVal
		}
		if subVal == "" {
			ancestorByLevel[level] = subKey
		}
	}
	m.fieldList = items
}

func augmentEC2AliasFields(resourceType string, fields map[string]string) map[string]string {
	if resourceType != "ec2" || len(fields) == 0 {
		return fields
	}
	aliases := map[string]string{
		"instance_id":  "InstanceId",
		"type":         "InstanceType",
		"state":        "State",
		"lifecycle":    "InstanceLifecycle",
		"image_id":     "ImageId",
		"key_name":     "KeyName",
		"vpc_id":       "VpcId",
		"subnet_id":    "SubnetId",
		"private_ip":   "PrivateIpAddress",
		"private_dns":  "PrivateDnsName",
		"public_ip":    "PublicIpAddress",
		"iam_profile":  "IamInstanceProfile",
		"architecture": "Architecture",
		"platform":     "Platform",
		"launch_time":  "LaunchTime",
	}
	needCopy := false
	for from, to := range aliases {
		if v, ok := fields[from]; ok && strings.TrimSpace(v) != "" {
			if _, exists := fields[to]; !exists {
				needCopy = true
				break
			}
		}
	}
	if !needCopy {
		return fields
	}
	out := make(map[string]string, len(fields)+len(aliases))
	for k, v := range fields {
		out[k] = v
	}
	for from, to := range aliases {
		if v, ok := fields[from]; ok && strings.TrimSpace(v) != "" {
			if _, exists := out[to]; !exists {
				out[to] = v
			}
		}
	}
	return out
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
			case item.IsHeader:
				line = " " + item.Key + ":"
			case item.IsSubField:
				if item.IsNavigable && item.Key != "" && !strings.Contains(item.Key, ":") {
					line = "     " + item.Key + ":  " + item.Value
					break
				}
				raw := strings.TrimSpace(item.Value)
				raw = strings.TrimPrefix(raw, "- ")
				subKey, subVal, hasSep := strings.Cut(raw, ": ")
				if hasSep {
					line = "     " + subKey + ":  " + subVal
				} else {
					line = "     " + raw
				}
			default:
				line = " " + text.PadOrTrunc(item.Key+":", keyW) + item.Value
			}
		} else {
			switch {
			case item.IsHeader:
				line = " " + styles.DetailSection.Render(item.Key+":")
			case item.IsSubField:
				// Bug2 fix: render sub-fields as "     Key:  value" with separate key/value styles.
				// Sub-field items have Key == Value (the raw combined line like "Name: web-prod").
				// Split on ": " to extract the key and value parts.
				// For navigable sub-fields, buildFieldList stores Key=subKey and Value=subValue.
				if item.IsNavigable && item.Key != "" && !strings.Contains(item.Key, ":") {
					line = "     " + styles.DetailKey.Render(item.Key+":") + "  " + styles.NavigableField.Render(item.Value)
					break
				}
				raw := strings.TrimSpace(item.Value)
				raw = strings.TrimPrefix(raw, "- ")
				subKey, subVal, hasSep := strings.Cut(raw, ": ")
				if hasSep {
					line = "     " + styles.DetailKey.Render(subKey+":") + "  " + styles.DetailVal.Render(subVal)
				} else {
					line = "     " + styles.DetailVal.Render(raw)
				}
			case item.IsNavigable:
				line = " " + styles.DetailKey.Render(text.PadOrTrunc(item.Key+":", keyW)) + styles.NavigableField.Render(item.Value)
			default:
				line = " " + styles.DetailKey.Render(text.PadOrTrunc(item.Key+":", keyW)) + styles.DetailVal.Render(item.Value)
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

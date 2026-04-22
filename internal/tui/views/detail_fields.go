// detail_fields.go contains field list construction and field-list-based rendering for DetailModel.
// Specifically: buildFieldList and renderFromFieldList.
package views

import (
	"fmt"
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

// flattenTagItems post-processes field items to render tag sections as
// flat Key: Value pairs instead of verbose Key/Value struct sub-fields.
// Only sections with Path "Tags" or "TagList" are flattened.
// YAML/JSON views are unaffected because they don't use fieldList.
func flattenTagItems(items []fieldpath.FieldItem) []fieldpath.FieldItem {
	if len(items) == 0 {
		return items
	}
	result := make([]fieldpath.FieldItem, 0, len(items))
	i := 0
	for i < len(items) {
		item := items[i]
		// Only flatten tag section headers.
		if item.IsHeader && (item.Path == "Tags" || item.Path == "TagList") {
			result = append(result, item)
			i++
			parentPath := item.Path
			// Collect all sub-fields belonging to this header.
			var subs []fieldpath.FieldItem
			for i < len(items) && items[i].IsSubField && items[i].Path == parentPath {
				subs = append(subs, items[i])
				i++
			}
			if len(subs) == 0 {
				// Empty tag section — leave header as-is, nothing to flatten.
				continue
			}
			// Detect simple Key/Value struct-list format: every field in the struct
			// must be either "Key" or "Value". If any element has additional metadata
			// (e.g., ASG TagDescription with PropagateAtLaunch), skip flattening to
			// preserve that information.
			isSimpleTagList := false
			hasExtraFields := false
			for _, sub := range subs {
				trimmed := strings.TrimSpace(sub.Value)
				if rest, ok := strings.CutPrefix(trimmed, "- "); ok {
					k, _, hasSep := strings.Cut(rest, ":")
					if hasSep && strings.EqualFold(strings.TrimSpace(k), "key") {
						isSimpleTagList = true
					}
				} else {
					k, _, hasSep := strings.Cut(trimmed, ":")
					if hasSep {
						field := strings.ToLower(strings.TrimSpace(k))
						if field != "key" && field != "value" {
							hasExtraFields = true
						}
					}
				}
			}
			if !isSimpleTagList || hasExtraFields {
				// Not a simple Key/Value tag list, or has extra metadata — pass through.
				result = append(result, subs...)
				continue
			}
			// Parse the YAML struct-list lines to extract Key/Value tag pairs.
			var currentKey string
			emitPending := func() {
				if currentKey != "" {
					result = append(result, fieldpath.FieldItem{
						IsSubField:  true,
						IndentLevel: 1,
						Key:         currentKey,
						Value:       "",
						Path:        parentPath,
					})
					currentKey = ""
				}
			}
			for _, sub := range subs {
				line := sub.Value
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				// Detect new list element: starts with "- "
				if rest, ok := strings.CutPrefix(trimmed, "- "); ok {
					// Flush any pending key without a Value line (nil Value).
					emitPending()
					k, v, hasSep := strings.Cut(rest, ":")
					if !hasSep {
						continue
					}
					k = strings.TrimSpace(k)
					v = strings.TrimSpace(v)
					// Only handle the "Key:" line of a tag struct element.
					if strings.EqualFold(k, "key") {
						currentKey = v
					}
				} else {
					// Continuation line: "  Value: Y" or other fields (PropagateAtLaunch, etc.)
					k, v, hasSep := strings.Cut(trimmed, ":")
					if !hasSep {
						continue
					}
					k = strings.TrimSpace(k)
					v = strings.TrimSpace(v)
					if strings.EqualFold(k, "value") && currentKey != "" {
						// Emit flat tag item.
						result = append(result, fieldpath.FieldItem{
							IsSubField:  true,
							IndentLevel: 1,
							Key:         currentKey,
							Value:       v,
							Path:        parentPath,
						})
						currentKey = ""
					}
					// Ignore extra fields (PropagateAtLaunch, ResourceId, etc.)
				}
			}
			// Flush final pending key (last tag had nil Value).
			emitPending()
			continue
		}
		result = append(result, item)
		i++
	}
	return result
}

// expandJSONItems post-processes field items to detect JSON strings in values
// and expand them as YAML-formatted sub-fields. Handles both:
//   - top-level scalar items (not IsHeader, not IsSubField, not IsSection) whose Value is JSON
//   - sub-field items whose value portion is JSON
//
// Called after flattenTagItems() but before navigable post-processing.
func expandJSONItems(items []fieldpath.FieldItem) []fieldpath.FieldItem {
	if len(items) == 0 {
		return items
	}
	result := make([]fieldpath.FieldItem, 0, len(items))
	for _, item := range items {
		// Pass through unchanged: headers, sections, navigable items.
		if item.IsHeader || item.IsSection || item.IsNavigable {
			result = append(result, item)
			continue
		}
		if item.IsSubField {
			// Extract value portion after the first ":" separator.
			rawLine := item.Value
			trimmed := strings.TrimSpace(rawLine)
			trimmed = strings.TrimPrefix(trimmed, "- ")
			_, valuePart, hasSep := strings.Cut(trimmed, ":")
			if !hasSep {
				result = append(result, item)
				continue
			}
			valuePart = strings.TrimSpace(valuePart)
			lines := text.TryJSONToYAMLLines(valuePart)
			if lines == nil {
				result = append(result, item)
				continue
			}
			// Emit the key line as a sub-field header (key with empty value).
			// Format as "key:" so parseYAMLLine recognizes it.
			keyPart, _, _ := strings.Cut(trimmed, ":")
			keyLine := keyPart + ":"
			result = append(result, fieldpath.FieldItem{
				Path:        item.Path,
				Key:         keyLine,
				Value:       keyLine,
				IsSubField:  true,
				IndentLevel: item.IndentLevel,
			})
			// Emit expanded YAML lines at IndentLevel + 1.
			for _, line := range lines {
				leading := len(line) - len(strings.TrimLeft(line, " "))
				level := leading/2 + item.IndentLevel + 1
				result = append(result, fieldpath.FieldItem{
					Path:        item.Path,
					Key:         line,
					Value:       line,
					IsSubField:  true,
					IndentLevel: level,
				})
			}
			continue
		}
		// Top-level scalar item: check if Value is JSON.
		lines := text.TryJSONToYAMLLines(item.Value)
		if lines == nil {
			result = append(result, item)
			continue
		}
		// Replace scalar with a header + sub-field lines.
		result = append(result, fieldpath.FieldItem{
			Path:     item.Path,
			Key:      item.Key,
			IsHeader: true,
		})
		for _, line := range lines {
			leading := len(line) - len(strings.TrimLeft(line, " "))
			level := leading/2 + 1
			result = append(result, fieldpath.FieldItem{
				Path:        item.Path,
				Key:         line,
				Value:       line,
				IsSubField:  true,
				IndentLevel: level,
			})
		}
	}
	return result
}

// buildFieldList computes m.fieldList from the view config and navigable field registry.
// Sets m.fieldList to nil when no config or detail paths are available (falls through to renderFromConfig).
// After calling ExtractFieldList, post-processes sub-fields to mark navigable ones:
// a sub-field under path P whose key K matches navMap["P.K"] is marked IsNavigable
// with TargetType from the navMap, and its Value is set to the extracted sub-value.
func (m *DetailModel) buildFieldList() {
	if m.resourceType == "ct-events" && m.res.Status != "" {
		raw := extractRawCTEventJSON(m.res)
		if raw != "" {
			event, err := ctdetail.Parse(raw)
			if err != nil {
				// A CT event resource arrived with a raw JSON blob that cannot be
				// parsed — that's a broken contract (the fetcher guarantees valid
				// JSON). Surface it explicitly instead of silently degrading to
				// the flat Fields path and pretending the page is fine.
				m.fieldList = []fieldpath.FieldItem{{
					Key:   "Error",
					Value: fmt.Sprintf("unable to parse CloudTrail event JSON: %v", err),
				}}
				return
			}
			event.Status = m.res.Status // propagate severity status into the parsed event
			sections := ctdetail.BuildSections(event)
			m.fieldList = sectionsToFieldItems(sections)
			return
		}
		// raw == "" is still a soft fallback: the cache can hold a bare CT event
		// stub (ID/Name/Status only) when the user drill-ins without the full
		// event body. Render what Fields we have instead of erroring out.
	}
	// Extract detail field definitions; split into path-form and key-form.
	var detailFields []config.DetailField
	if m.viewConfig != nil {
		vd := config.GetViewDef(m.viewConfig, m.resourceType)
		detailFields = vd.Detail
	}
	// Build the []string slice of Path values for ExtractFieldList (path-form only).
	// Key-form fields (df.Key != "") live in Fields[] and are injected after.
	var detailPaths []string
	for _, df := range detailFields {
		if df.Path != "" {
			detailPaths = append(detailPaths, df.Path)
		}
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
		m.fieldList = expandJSONItems(flattenTagItems(fieldpath.ExtractFieldList(nil, fields, keys, nil)))
		m.injectIssuesSection()
		if m.resourceType == "ec2" {
			m.injectEC2StatusChecks()
		}
		m.injectEnrichmentSection()
		return
	}
	// Build items from path-form fields.
	pathItems := expandJSONItems(flattenTagItems(fieldpath.ExtractFieldList(m.res.RawStruct, fields, detailPaths, navMap)))

	// Build a map from path → slice of FieldItems for O(1) lookup when interleaving.
	pathItemsByPath := make(map[string][]fieldpath.FieldItem, len(detailPaths))
	for _, item := range pathItems {
		p := item.Path
		pathItemsByPath[p] = append(pathItemsByPath[p], item)
	}

	// Build the final ordered items list preserving detailFields order.
	// Key-form fields are injected as plain key-value rows.
	// Path-form fields use the items extracted by ExtractFieldList.
	var items []fieldpath.FieldItem
	emittedPath := make(map[string]bool, len(detailPaths))
	for _, df := range detailFields {
		if df.Key != "" {
			// Key-form: read from Fields[].
			val := "-"
			if v, ok := m.res.Fields[df.Key]; ok && v != "" {
				val = v
			}
			items = append(items, fieldpath.FieldItem{
				Key:   df.DisplayLabel(),
				Value: val,
				Path:  df.Key,
			})
		} else if !emittedPath[df.Path] {
			// Path-form: emit all FieldItems with this path (may be header + sub-fields).
			// When df.Label is set, override the first (header) item's label so the
			// user-provided "label:" in YAML actually shows up. Sub-field labels are
			// derived from the struct shape and are not overridden.
			emittedPath[df.Path] = true
			pathItems := pathItemsByPath[df.Path]
			if df.Label != "" && len(pathItems) > 0 {
				// Clone to avoid mutating the cached slice — pathItemsByPath may be
				// referenced again on re-render.
				pathItems = append([]fieldpath.FieldItem(nil), pathItems...)
				pathItems[0].Key = df.Label
			}
			items = append(items, pathItems...)
		}
	}

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
	m.injectIssuesSection()
	if m.resourceType == "ec2" {
		m.injectEC2StatusChecks()
	}
	m.injectEnrichmentSection()
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
// for deferred styling via TierColorStyle in the render path.
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

// injectIssuesSection prepends an "Issues" section to the field list when the
// resource has one or more active issue phrases. Spec universal rule 7: "every
// finding individually visible across S2–S5". The list view's Status column
// shows only the top phrase (with optional `(+N)` suffix); this section
// surfaces every entry so the operator can tell at a glance what's wrong,
// without hunting through raw SDK fields.
//
// The section appears at the very top of the field list (above identity / AWS
// fields) because issues are the 3am-glance information. Rendered items reuse
// the existing fieldpath.FieldItem shape (section header + sub-fields) so
// styling, color tiers, and navigation behave identically to other sections.
func (m *DetailModel) injectIssuesSection() {
	if len(m.res.Issues) == 0 {
		return
	}
	items := make([]fieldpath.FieldItem, 0, 1+len(m.res.Issues))
	items = append(items, fieldpath.FieldItem{
		IsSection: true,
		Key:       "Issues",
		Path:      "Issues",
		ColorTier: issuesSectionTier(m.res),
	})
	for _, phrase := range m.res.Issues {
		items = append(items, fieldpath.FieldItem{
			IsSubField:  true,
			IndentLevel: 1,
			Key:         phrase,
			Value:       phrase,
			Path:        "Issues",
			ColorTier:   issuesSectionTier(m.res),
		})
	}
	// Prepend: issues at the TOP, before identity / AWS fields.
	m.fieldList = append(items, m.fieldList...)
}

// issuesSectionTier classifies the issues block for colouring. "!" for Broken
// resources, "~" otherwise (Warning / transitional / Dim all render as "~").
// Healthy rows never reach this code path because len(Issues) == 0.
func issuesSectionTier(r resource.Resource) string {
	// Any issue phrase that exactly matches a broken-bucket phrase promotes
	// the section to "!" severity. Otherwise it is informational ("~").
	brokenPhrases := map[string]bool{
		"failed":                    true,
		"storage-full":              true,
		"restore-error":             true,
		"stopped":                   true,
		"incompatible-network":      true,
		"incompatible-option-group": true,
		"incompatible-parameters":   true,
		"incompatible-restore":      true,
		"encryption key unavailable": true,
	}
	for _, p := range r.Issues {
		if brokenPhrases[p] {
			return "!"
		}
	}
	return "~"
}

// injectEnrichmentSection dispatches to the per-type enrichment injector based on
// m.resourceType. Types without a Wave 2 enricher (e.g. ec2, ddb) are not dispatched.
func (m *DetailModel) injectEnrichmentSection() {
	switch m.resourceType {
	case "dbi", "rds":
		m.injectRDSPendingMaintenance()
	case "ebs":
		m.injectEBSVolumeStatus()
	case "cb":
		m.injectCodeBuildLatestBuild()
	case "tg":
		m.injectTargetGroupHealth()
	case "pipeline":
		m.injectPipelineStageFailure()
	case "sfn":
		m.injectSFNLatestExecution()
	case "glue":
		m.injectGlueLatestRun()
	default:
		// Generic fallback: any non-zero finding gets a "Background Check" section
		// so users see something for resource types whose enricher exists in the
		// registry but doesn't have a per-type renderer here.
		if m.enrichmentFinding != nil {
			m.appendFindingSection("Background Check", "BackgroundCheck")
		}
	}
}

// appendFindingSection appends a named section header and one row per FindingRow
// to m.fieldList. Returns immediately when finding is nil. When Rows is empty
// but Summary is set, falls back to rendering Summary as a single data row.
func (m *DetailModel) appendFindingSection(header, pathKey string) {
	if m.enrichmentFinding == nil {
		return
	}
	rows := m.enrichmentFinding.Rows
	if len(rows) == 0 {
		if m.enrichmentFinding.Summary == "" {
			return
		}
		rows = []resource.FindingRow{{Label: "Summary", Value: m.enrichmentFinding.Summary}}
	}
	items := make([]fieldpath.FieldItem, 0, 1+len(rows))
	items = append(items, fieldpath.FieldItem{
		IsSection: true,
		Key:       header,
		Path:      pathKey,
		ColorTier: m.enrichmentFinding.Severity,
	})
	for _, row := range rows {
		tier := row.Tier
		if tier == "" {
			tier = m.enrichmentFinding.Severity
		}
		items = append(items, fieldpath.FieldItem{
			IsSubField:  true,
			IndentLevel: 1,
			Key:         row.Label,
			Value:       row.Value,
			Path:        pathKey,
			ColorTier:   tier,
		})
	}
	m.fieldList = append(m.fieldList, items...)
}

func (m *DetailModel) injectRDSPendingMaintenance() {
	m.appendFindingSection("Pending Maintenance", "PendingMaintenance")
}

func (m *DetailModel) injectEBSVolumeStatus() {
	m.appendFindingSection("Volume Health", "VolumeHealth")
}

func (m *DetailModel) injectCodeBuildLatestBuild() {
	m.appendFindingSection("Latest Build", "LatestBuild")
}

func (m *DetailModel) injectTargetGroupHealth() {
	m.appendFindingSection("Target Health", "TargetHealth")
}

func (m *DetailModel) injectPipelineStageFailure() {
	m.appendFindingSection("Pipeline State", "PipelineState")
}

func (m *DetailModel) injectSFNLatestExecution() {
	m.appendFindingSection("Latest Execution", "LatestExecution")
}

func (m *DetailModel) injectGlueLatestRun() {
	m.appendFindingSection("Latest Run", "LatestRun")
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
					val := item.Value
					if item.ColorTier != "" {
						val = styles.TierColorStyle(item.ColorTier).Render(val)
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

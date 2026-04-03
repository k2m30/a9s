package views

import (
	"reflect"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"

	"gopkg.in/yaml.v3"
)

// DetailModel renders the key-value describe view using bubbles/viewport for scroll.
type DetailModel struct {
	res                 resource.Resource
	resourceType        string // e.g. "ec2", "s3", "rds" — used to look up correct ViewDef
	viewConfig          *config.ViewsConfig
	viewport            viewport.Model
	ready               bool
	wrap                bool
	width               int
	height              int
	keys                keys.Map
	search              SearchModel
	rightCol            rightColumnModel
	rightColVisible     bool                  // true when explicitly toggled on
	rightColAutoShown   bool                  // true when right column was auto-shown on SetSize (wide terminal + registered defs)
	rightColUserToggled bool                  // true after user explicitly toggles related visibility
	rightColWidth       int                   // width of right column panel (default 32)
	fieldList           []fieldpath.FieldItem // structured field data; nil = not yet computed
	fieldCursor         int                   // index into fieldList for navigable cursor
}

// NewDetail creates a DetailModel for the given resource.
// resourceType identifies which ViewDef to use from the config (e.g. "ec2", "rds").
func NewDetail(res resource.Resource, resourceType string, viewConfig *config.ViewsConfig, k keys.Map) DetailModel {
	if resourceType == "" {
		resourceType = inferDetailResourceType(res)
	}
	return DetailModel{
		resourceType:  resourceType,
		res:           res,
		viewConfig:    viewConfig,
		keys:          k,
		rightColWidth: 32,
	}
}

// inferDetailResourceType provides a conservative fallback for routes that
// navigate to detail without an explicit type. This prevents losing related
// and navigable behavior for common top-level resources.
func inferDetailResourceType(res resource.Resource) string {
	has := func(k string) bool {
		v, ok := res.Fields[k]
		return ok && strings.TrimSpace(v) != ""
	}
	// EC2 signature: infer only from EC2-shaped key sets.
	// Anchor on instance id key plus at least one EC2-specific companion field.
	hasInstanceID := has("InstanceId") || has("instance_id")
	hasEC2Companion := has("ImageId") || has("image_id") ||
		has("VpcId") || has("vpc_id") ||
		has("SubnetId") || has("subnet_id") ||
		has("PrivateIpAddress") || has("private_ip") ||
		has("PublicIpAddress") || has("public_ip") ||
		has("KeyName") || has("key_name") ||
		has("InstanceLifecycle") || has("lifecycle") ||
		has("LaunchTime") || has("launch_time") ||
		has("IamInstanceProfile") || has("iam_instance_profile") ||
		has("SecurityGroups") || has("security_groups")
	if hasInstanceID && hasEC2Companion {
		return "ec2"
	}
	return ""
}

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
		if strings.HasPrefix(trimmed, "- ") {
			trimmed = strings.TrimPrefix(trimmed, "- ")
		}
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

// Init implements tea.Model. No async work.
func (m DetailModel) Init() (DetailModel, tea.Cmd) {
	return m, nil
}

// Update delegates scroll to viewport; handles y (yaml), c (copy), esc (back).
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.RelatedCheckResultMsg:
		// Ignore results for a different resource type.
		if msg.ResourceType != m.resourceType {
			return m, nil
		}
		m.rightCol, _ = m.rightCol.Update(msg)
		return m, nil
	case tea.KeyMsg:
		// Search input mode captures all keys.
		if m.search.IsInputMode() {
			m.search, _ = m.search.Update(msg)
			m.refreshViewportContent()
			return m, nil
		}
		// When right column is focused, delegate navigation keys to it.
		if m.rightColShowing() && m.rightCol.IsFocused() {
			if m.rightCol.IsFiltering() {
				var cmd tea.Cmd
				m.rightCol, cmd = m.rightCol.Update(msg)
				m.refreshViewportContent()
				return m, cmd
			}
			switch {
			case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down),
				key.Matches(msg, m.keys.Enter):
				var cmd tea.Cmd
				m.rightCol, cmd = m.rightCol.Update(msg)
				m.refreshViewportContent()
				return m, cmd
			case key.Matches(msg, m.keys.Search):
				var cmd tea.Cmd
				m.rightCol, cmd = m.rightCol.Update(msg)
				m.refreshViewportContent()
				return m, cmd
			case key.Matches(msg, m.keys.Tab):
				m.rightCol.SetFocused(false)
				m.refreshViewportContent() // update cursor highlight after focus change
				return m, nil
			case key.Matches(msg, m.keys.Escape):
				if m.rightCol.IsFiltering() || m.rightCol.HasFilter() {
					var cmd tea.Cmd
					m.rightCol, cmd = m.rightCol.Update(msg)
					m.refreshViewportContent()
					return m, cmd
				}
				// Esc from focused right column: unfocus (don't pop view)
				m.rightCol.SetFocused(false)
				m.refreshViewportContent() // update cursor highlight after focus change
				return m, nil
			}
			// Other keys (like ToggleRelated, Search, etc.) still handled by detail
		}
		switch {
		case key.Matches(msg, m.keys.ScrollRight):
			// l: focus right column (if showing and not already focused)
			if m.rightColShowing() && !m.rightCol.IsFocused() && m.rightCol.HasActionableRows() {
				m.rightCol.SetFocused(true)
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.ScrollLeft):
			// h: focus left column (if right is focused)
			if m.rightCol.IsFocused() {
				m.rightCol.SetFocused(false)
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.Copy):
			if m.rightCol.IsFocused() {
				name := m.rightCol.SelectedTypeName()
				if name != "" {
					return m, func() tea.Msg {
						return messages.CopiedMsg{Content: name}
					}
				}
				return m, nil
			}
			// Left column: copy the field value at cursor
			if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
				item := m.fieldList[m.fieldCursor]
				val := item.Value
				if val == "" {
					val = item.Key
				}
				return m, func() tea.Msg {
					return messages.CopiedMsg{Content: val}
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			// Ctrl+R: re-trigger related checks if right column is showing
			if m.rightColShowing() {
				defs := resource.GetRelated(m.resourceType)
				m.rightCol = newRightColumn(defs, m.res)
				m.rightCol.keys = m.keys
				m.rightCol.SetSize(m.rightColWidth, m.height)
				return m, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{
						ResourceType:   m.resourceType,
						SourceResource: m.res,
					}
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			if !m.rightCol.IsFocused() && m.fieldList != nil {
				pageSize := m.height - 4
				if pageSize < 1 {
					pageSize = 1
				}
				m.fieldCursor += pageSize
				if m.fieldCursor >= len(m.fieldList) {
					m.fieldCursor = len(m.fieldList) - 1
				}
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			// No fieldList — scroll viewport directly
			if m.ready {
				m.viewport.HalfPageDown()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			if !m.rightCol.IsFocused() && m.fieldList != nil {
				pageSize := m.height - 4
				if pageSize < 1 {
					pageSize = 1
				}
				m.fieldCursor -= pageSize
				if m.fieldCursor < 0 {
					m.fieldCursor = 0
				}
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			// No fieldList — scroll viewport directly
			if m.ready {
				m.viewport.HalfPageUp()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.Search):
			m.search.Activate()
			return m, nil
		case key.Matches(msg, m.keys.SearchNext):
			if m.search.IsActive() && m.search.MatchCount() > 0 {
				m.search.NextMatch()
				m.refreshViewportContent()
				return m, nil
			}
		case key.Matches(msg, m.keys.SearchPrev):
			if m.search.IsActive() && m.search.MatchCount() > 0 {
				m.search.PrevMatch()
				m.refreshViewportContent()
				return m, nil
			}
		case key.Matches(msg, m.keys.Escape):
			if m.search.IsActive() {
				m.search.Deactivate()
				m.refreshViewportContent()
				return m, nil
			}
		case key.Matches(msg, m.keys.Tab):
			if m.rightColShowing() && (m.rightCol.IsFocused() || m.rightCol.HasActionableRows()) {
				if m.rightCol.IsFocused() {
					m.rightCol.SetFocused(false)
				} else {
					m.rightCol.SetFocused(true)
				}
				m.refreshViewportContent() // update cursor highlight after focus change
				return m, nil
			}
		case key.Matches(msg, m.keys.ToggleRelated):
			m.rightColUserToggled = true
			if m.width < 60 {
				return m, nil // silently ignore on narrow terminals
			}
			if m.width < 100 {
				// In stacked mode (80-99), first toggle after auto-show hides immediately.
				if m.rightColAutoShown {
					m.rightColAutoShown = false
					m.rightColVisible = false
					m.recalcViewportWidth()
					return m, nil
				}
				m.rightColVisible = !m.rightColVisible
				if m.rightColVisible {
					defs := resource.GetRelated(m.resourceType)
					m.rightCol = newRightColumn(defs, m.res)
					m.rightCol.keys = m.keys
					m.rightCol.SetSize(m.width, max(1, m.height/2))
					m.recalcViewportWidth()
					return m, func() tea.Msg {
						return messages.RelatedCheckStartedMsg{
							ResourceType:   m.resourceType,
							SourceResource: m.res,
						}
					}
				}
				m.recalcViewportWidth()
				return m, nil
			}
			if m.rightColAutoShown {
				// First explicit toggle hides the auto-shown column.
				m.rightColAutoShown = false
				m.rightColVisible = false
				m.rightCol.SetFocused(false)
				m.recalcViewportWidth()
				return m, nil
			}
			// Normal toggle: flip visible state.
			m.rightColVisible = !m.rightColVisible
			if m.rightColVisible {
				defs := resource.GetRelated(m.resourceType)
				m.rightCol = newRightColumn(defs, m.res)
				m.rightCol.keys = m.keys
				m.rightCol.SetSize(m.rightColWidth, m.height)
				m.recalcViewportWidth()
				return m, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{
						ResourceType:   m.resourceType,
						SourceResource: m.res,
					}
				}
			}
			m.rightCol.SetFocused(false)
			m.recalcViewportWidth()
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			// Navigate to a related resource when pressing Enter on a navigable field.
			// Skip if the right column has focus — it handles its own Enter.
			if m.rightCol.IsFocused() {
				break
			}
			if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
				item := m.fieldList[m.fieldCursor]
				if item.IsNavigable {
					return m, func() tea.Msg {
						return messages.RelatedNavigateMsg{
							TargetType:     item.TargetType,
							SourceResource: m.res,
							SourceType:     m.resourceType,
							TargetID:       item.Value,
						}
					}
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.YAML):
			return m, func() tea.Msg {
				return messages.NavigateMsg{
					Target:   messages.TargetYAML,
					Resource: &m.res,
				}
			}
		case key.Matches(msg, m.keys.ToggleWrap):
			m.wrap = !m.wrap
			m.viewport.SoftWrap = m.wrap
			m.refreshViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.Top):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor > 0 {
				m.fieldCursor = 0
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.Bottom):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor < len(m.fieldList)-1 {
				m.fieldCursor = len(m.fieldList) - 1
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor < len(m.fieldList)-1 {
				m.fieldCursor++
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil // Bug6 fix: clamp at boundary, don't fall through to viewport scroll
		case key.Matches(msg, m.keys.Up):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor > 0 {
				m.fieldCursor--
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil // Bug6 fix: clamp at boundary, don't fall through to viewport scroll
		}
	case tea.KeyPressMsg:
		// Bubble Tea v2 integration specs may emit KeyPressMsg directly.
		// Handle the common navigation keys here.
		if m.rightColShowing() && m.rightCol.IsFocused() {
			if msg.Code == tea.KeyTab {
				m.rightCol.SetFocused(false)
				m.refreshViewportContent()
				return m, nil
			}
			if msg.Code == tea.KeyEscape {
				if m.rightCol.IsFiltering() || m.rightCol.HasFilter() {
					var cmd tea.Cmd
					m.rightCol, cmd = m.rightCol.Update(msg)
					m.refreshViewportContent()
					return m, cmd
				}
				m.rightCol.SetFocused(false)
				m.refreshViewportContent()
				return m, nil
			}
			if msg.Text == "/" || msg.Text == "j" || msg.Text == "k" || msg.Code == tea.KeyUp || msg.Code == tea.KeyDown || msg.Code == tea.KeyEnter || m.rightCol.IsFiltering() {
				var cmd tea.Cmd
				m.rightCol, cmd = m.rightCol.Update(msg)
				m.refreshViewportContent()
				return m, cmd
			}
		}
		if msg.Text == "r" {
			m.rightColUserToggled = true
			if m.width < 60 {
				return m, nil
			}
			if m.width < 100 {
				if m.rightColAutoShown {
					m.rightColAutoShown = false
					m.rightColVisible = false
					m.recalcViewportWidth()
					return m, nil
				}
				m.rightColVisible = !m.rightColVisible
				if m.rightColVisible {
					defs := resource.GetRelated(m.resourceType)
					m.rightCol = newRightColumn(defs, m.res)
					m.rightCol.keys = m.keys
					m.rightCol.SetSize(m.width, max(1, m.height/2))
					m.recalcViewportWidth()
					return m, func() tea.Msg {
						return messages.RelatedCheckStartedMsg{
							ResourceType:   m.resourceType,
							SourceResource: m.res,
						}
					}
				}
				m.recalcViewportWidth()
				return m, nil
			}
			if m.rightColAutoShown {
				m.rightColAutoShown = false
				m.rightColVisible = false
				m.rightCol.SetFocused(false)
				m.recalcViewportWidth()
				return m, nil
			}
			m.rightColVisible = !m.rightColVisible
			if m.rightColVisible {
				defs := resource.GetRelated(m.resourceType)
				m.rightCol = newRightColumn(defs, m.res)
				m.rightCol.keys = m.keys
				m.rightCol.SetSize(m.rightColWidth, m.height)
				m.recalcViewportWidth()
				return m, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{
						ResourceType:   m.resourceType,
						SourceResource: m.res,
					}
				}
			}
			m.rightCol.SetFocused(false)
			m.recalcViewportWidth()
			return m, nil
		}
		if msg.Code == tea.KeyTab {
			if m.rightColShowing() && (m.rightCol.IsFocused() || m.rightCol.HasActionableRows()) {
				m.rightCol.SetFocused(!m.rightCol.IsFocused())
				m.refreshViewportContent()
			}
			return m, nil
		}
		if msg.Code == tea.KeyEnter {
			if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
				item := m.fieldList[m.fieldCursor]
				if item.IsNavigable {
					return m, func() tea.Msg {
						return messages.RelatedNavigateMsg{
							TargetType:     item.TargetType,
							SourceResource: m.res,
							SourceType:     m.resourceType,
							TargetID:       item.Value,
						}
					}
				}
			}
			return m, nil
		}
		if msg.Text == "j" || msg.Code == tea.KeyDown {
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor < len(m.fieldList)-1 {
				m.fieldCursor++
				m.syncViewportToCursor()
				m.refreshViewportContent()
			}
			return m, nil
		}
		if msg.Text == "k" || msg.Code == tea.KeyUp {
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor > 0 {
				m.fieldCursor--
				m.syncViewportToCursor()
				m.refreshViewportContent()
			}
			return m, nil
		}
		if msg.Code == tea.KeyEscape || msg.Text == "esc" {
			if m.search.IsActive() {
				m.search.Deactivate()
				m.refreshViewportContent()
			}
			return m, nil
		}
		if msg.Text == "g" {
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor > 0 {
				m.fieldCursor = 0
				m.syncViewportToCursor()
				m.refreshViewportContent()
			}
			return m, nil
		}
		if msg.Text == "G" {
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor < len(m.fieldList)-1 {
				m.fieldCursor = len(m.fieldList) - 1
				m.syncViewportToCursor()
				m.refreshViewportContent()
			}
			return m, nil
		}
		return m, nil
	}

	// Delegate to viewport for scroll
	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders detail content via viewport.
// When the right column is showing and width >= 100, renders left and right columns side by side
// with a │ separator.
func (m DetailModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	if m.rightColShowing() && m.width >= 60 {
		// Keep RELATED visible at medium widths by using side-by-side layout.
		rightW := m.currentRightColWidth()
		sep := styles.ColSepDim.Render("│")
		if m.rightCol.IsFocused() {
			sep = styles.ColSepAccent.Render("│")
		}
		leftContent := m.viewport.View()
		rightContent := m.rightCol.View()
		leftLines := strings.Split(leftContent, "\n")
		rightLines := strings.Split(rightContent, "\n")
		// Normalise to same number of lines.
		maxLines := len(leftLines)
		if len(rightLines) > maxLines {
			maxLines = len(rightLines)
		}
		leftW := m.width - rightW - 1 // -1 for separator character
		var sb strings.Builder
		for i := range maxLines {
			if i > 0 {
				sb.WriteString("\n")
			}
			left := ""
			if i < len(leftLines) {
				left = leftLines[i]
			}
			right := ""
			if i < len(rightLines) {
				right = rightLines[i]
			}
			// Pad left column to its fixed width so right column aligns correctly.
			padded := left
			leftVisible := lipgloss.Width(left)
			if leftVisible < leftW {
				padded = left + strings.Repeat(" ", leftW-leftVisible)
			}
			sb.WriteString(padded)
			sb.WriteString(sep)
			sb.WriteString(right)
		}
		return sb.String()
	}
	return m.viewport.View()
}

// SetSize initializes or resizes the viewport. Must be called before View().
// On first call, if width >= 100 and related defs are registered, the right
// column is auto-shown (rightColAutoShown = true). The first explicit toggle
// transitions from auto-shown to explicitly-on (still visible). A second toggle
// hides the column.
func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	wasShowing := m.rightColShowing()

	// Auto-show right column when wide enough and related defs exist:
	// - on first SetSize call, and
	// - on later resizes only if user hasn't explicitly toggled visibility.
	if w >= 60 && len(resource.GetRelated(m.resourceType)) > 0 &&
		(!m.ready || (!m.rightColShowing() && !m.rightColUserToggled)) {
		m.rightColAutoShown = true
		m.rightCol = newRightColumn(resource.GetRelated(m.resourceType), m.res)
		m.rightCol.keys = m.keys
	} else if w < 60 && wasShowing {
		m.rightColAutoShown = false
		m.rightColVisible = false
		m.rightCol.SetFocused(false)
	}

	viewportW := w
	if m.rightColShowing() && w >= 60 {
		rightW := m.currentRightColWidth()
		viewportW = w - rightW - 1 // -1 for separator character
		m.rightCol.SetSize(rightW, h)
	}

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(viewportW), viewport.WithHeight(h))
		m.ready = true
	} else {
		m.viewport.SetWidth(viewportW)
		m.viewport.SetHeight(h)
	}
	m.refreshViewportContent()
}

// rightColShowing returns true when the right column should be rendered.
// The column shows when explicitly toggled on OR when auto-shown on entry.
func (m DetailModel) rightColShowing() bool {
	return m.rightColVisible || m.rightColAutoShown
}

// recalcViewportWidth adjusts the viewport width based on the right column visibility.
func (m *DetailModel) recalcViewportWidth() {
	if m.rightColShowing() && m.width >= 60 {
		leftW := m.width - m.currentRightColWidth() - 1 // -1 for separator
		if m.ready {
			m.viewport.SetWidth(leftW)
		}
	} else if m.ready {
		m.viewport.SetWidth(m.width)
	}
	m.refreshViewportContent()
}

func (m DetailModel) currentRightColWidth() int {
	// Keep right panel readable at medium widths while preserving left detail space.
	if m.width <= 0 {
		return m.rightColWidth
	}
	if m.width >= 100 {
		return m.rightColWidth
	}
	w := max(24, m.width/3)
	maxAllowed := max(16, m.width-40) // keep at least 40 cols for left pane
	if w > maxAllowed {
		w = maxAllowed
	}
	return w
}

// syncViewportToCursor adjusts the viewport scroll to keep fieldCursor visible.
func (m *DetailModel) syncViewportToCursor() {
	if !m.ready {
		return
	}
	yOffset := m.viewport.YOffset()
	visibleLines := m.viewport.Height()
	if m.fieldCursor < yOffset {
		m.viewport.SetYOffset(m.fieldCursor)
	} else if m.fieldCursor >= yOffset+visibleLines {
		m.viewport.SetYOffset(m.fieldCursor - visibleLines + 1)
	}
}

// FieldCursor returns the current field cursor index for testing.
func (m DetailModel) FieldCursor() int {
	return m.fieldCursor
}

// refreshViewportContent re-renders content and applies search highlights.
func (m *DetailModel) refreshViewportContent() {
	if m.fieldList == nil {
		m.buildFieldList()
	}
	content := m.renderContent()
	if m.search.IsActive() && m.search.Query() != "" {
		plain := ansi.Strip(content)
		m.search.SetContent(plain)
		var matchLine int
		content, matchLine = m.search.Apply(content)
		if matchLine >= 0 {
			m.viewport.GotoTop()
			m.viewport.SetYOffset(matchLine)
		}
	}
	m.viewport.SetContent(content)
}

// FrameTitle returns the resource identifier.
func (m DetailModel) FrameTitle() string {
	if m.res.Name != "" {
		return m.res.Name
	}
	return m.res.ID
}

// CopyContent returns column-aware clipboard content for the active selection.
func (m DetailModel) CopyContent() (string, string) {
	if m.rightCol.IsFocused() {
		name := m.rightCol.SelectedTypeName()
		if name == "" {
			return "", ""
		}
		return name, "Copied: " + name
	}
	if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
		item := m.fieldList[m.fieldCursor]
		content := item.Value
		if content == "" {
			content = item.Key
		}
		if content == "" {
			return "", ""
		}
		return content, "Copied: " + content
	}
	content := m.RawYAML()
	if content == "" {
		return "", ""
	}
	return content, "Copied detail to clipboard"
}

// GetHelpContext returns HelpFromDetail.
func (m DetailModel) GetHelpContext() HelpContext {
	return HelpFromDetail
}

// IsSearchActive returns true when search is active (input mode or confirmed highlights).
func (m DetailModel) IsSearchActive() bool {
	return m.search.IsActive() || m.rightCol.IsFiltering()
}

// IsSearchInputMode returns true when the search input is capturing keystrokes.
func (m DetailModel) IsSearchInputMode() bool {
	return m.search.IsInputMode() || m.rightCol.IsFiltering()
}

// SearchInfo returns the search state string for the header.
// Input mode: "/query" (or "/" when query is empty), Confirmed: "N/M matches", Inactive: "".
func (m DetailModel) SearchInfo() string {
	if m.rightCol.IsFiltering() {
		return "/" + m.rightCol.FilterQuery()
	}
	if !m.search.IsActive() {
		return ""
	}
	if m.search.IsInputMode() {
		q := m.search.Query()
		return "/" + q
	}
	return m.search.MatchInfo()
}

// ResourceID returns the resource ID for clipboard copy.
func (m DetailModel) ResourceID() string {
	return m.res.ID
}

// ResourceType returns the resource type short name.
func (m DetailModel) ResourceType() string {
	return m.resourceType
}

// SourceResource returns the resource being viewed.
func (m DetailModel) SourceResource() resource.Resource {
	return m.res
}

// NeedsRelatedCheck returns true when the right column was auto-shown
// and checkers have not yet been dispatched. The root model checks this
// after pushing the detail view to emit RelatedCheckStartedMsg.
func (m DetailModel) NeedsRelatedCheck() bool {
	return m.rightColAutoShown
}

// RawYAML returns the resource as YAML for clipboard copy (same format as YAML view).
func (m DetailModel) RawYAML() string {
	var data []byte
	var err error

	if m.res.RawStruct != nil {
		safe := fieldpath.ToSafeValue(reflect.ValueOf(m.res.RawStruct))
		data, err = yaml.Marshal(safe)
	} else if len(m.res.Fields) > 0 {
		data, err = yaml.Marshal(m.res.Fields)
	}

	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

// ConsumesEscapeLocally reports whether Escape should be handled inside the
// detail view instead of by the root view-stack pop logic.
func (m DetailModel) ConsumesEscapeLocally() bool {
	return m.rightCol.IsFocused() || m.rightCol.IsFiltering()
}

// PlainContent returns the detail content as plain text (no ANSI) for clipboard copy.
func (m DetailModel) PlainContent() string {
	content := m.renderContent()
	// Strip ANSI escape codes
	result := make([]byte, 0, len(content))
	i := 0
	for i < len(content) {
		if content[i] == '\x1b' && i+1 < len(content) && content[i+1] == '[' {
			// Skip until we hit a letter
			j := i + 2
			for j < len(content) && (content[j] < 'a' || content[j] > 'z') && (content[j] < 'A' || content[j] > 'Z') {
				j++
			}
			if j < len(content) {
				j++ // skip the letter
			}
			i = j
		} else {
			result = append(result, content[i])
			i++
		}
	}
	return string(result)
}

// renderContent builds the styled key-value lines from the resource.
func (m DetailModel) renderContent() string {
	// Use structured field list when available.
	if m.fieldList != nil {
		return m.renderFromFieldList()
	}

	// Try config-driven rendering.
	if m.viewConfig != nil {
		vd := config.GetViewDef(m.viewConfig, m.resourceType)
		if len(vd.Detail) > 0 {
			keyW := computeKeyWidth(vd.Detail)
			kv := func(key, val string) string {
				return " " + styles.DetailKey.Render(text.PadOrTrunc(key+":", keyW)) + styles.DetailVal.Render(val)
			}
			if lines := m.renderFromConfig(kv); len(lines) > 0 {
				return strings.Join(lines, "\n")
			}
		}
	}

	// Fallback: render from Fields map (no config or no matching ViewDef).
	if len(m.res.Fields) == 0 {
		return styles.DimText.Render("  No detail data available")
	}

	// Sort keys for stable output.
	fieldKeys := make([]string, 0, len(m.res.Fields))
	for k := range m.res.Fields {
		fieldKeys = append(fieldKeys, k)
	}
	sort.Strings(fieldKeys)

	keyW := computeKeyWidth(fieldKeys)
	kv := func(key, val string) string {
		return " " + styles.DetailKey.Render(text.PadOrTrunc(key+":", keyW)) + styles.DetailVal.Render(val)
	}

	var lines []string
	for _, k := range fieldKeys {
		lines = append(lines, kv(k, m.res.Fields[k]))
	}
	return strings.Join(lines, "\n")
}

// computeKeyWidth returns the width needed for the key column: longest key + 1 (for colon), minimum 22.
func computeKeyWidth(keys []string) int {
	w := 22
	for _, k := range keys {
		if len(k)+1 > w {
			w = len(k) + 1
		}
	}
	return w
}

// renderFromConfig looks up the correct ViewDef by resource type and renders detail lines.
// Tries RawStruct extraction first, then falls back to Fields map for each path.
// Empty/nil fields are shown as "-" (not skipped).
func (m DetailModel) renderFromConfig(kv func(string, string) string) []string {
	vd := config.GetViewDef(m.viewConfig, m.resourceType)
	if len(vd.Detail) == 0 {
		return nil
	}
	var lines []string
	for _, path := range vd.Detail {
		val := ""
		// Try Fields map first — fetchers populate Fields with pre-formatted
		// values (e.g., formatted timestamps instead of raw epoch ms).
		if len(m.res.Fields) > 0 {
			// Try exact case-insensitive match
			for k, v := range m.res.Fields {
				if strings.EqualFold(k, path) {
					val = v
					break
				}
			}
			// Try underscore-separated version: "InstanceId" → "instance_id"
			if val == "" {
				snakeKey := toSnakeCase(path)
				if v, ok := m.res.Fields[snakeKey]; ok {
					val = v
				}
			}
		}
		// Fall back to RawStruct extraction for fields not in Fields map
		if val == "" && m.res.RawStruct != nil {
			val = fieldpath.ExtractSubtree(m.res.RawStruct, path)
		}
		if val == "" {
			val = "-"
		}
		if strings.Contains(val, "\n") {
			lines = append(lines, " "+styles.DetailSection.Render(path+":"))
			for subLine := range strings.SplitSeq(val, "\n") {
				lines = append(lines, "     "+styles.DetailVal.Render(subLine))
			}
		} else {
			lines = append(lines, kv(path, val))
		}
	}
	return lines
}

// toSnakeCase converts PascalCase to snake_case: "InstanceId" → "instance_id".
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32) // toLower
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

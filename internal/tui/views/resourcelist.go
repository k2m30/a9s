package views

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/fieldpath"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/styles"
	"github.com/k2m30/a9s/internal/tui/text"
)

// SortField identifies the active sort column.
type SortField int

const (
	SortNone SortField = iota
	SortName
	SortStatus
	SortAge
)

// ResourceListModel is a tea.Model for the resource table view.
type ResourceListModel struct {
	typeDef    resource.ResourceTypeDef
	viewConfig *config.ViewsConfig

	allResources      []resource.Resource
	filteredResources []resource.Resource

	cursor        int
	hScrollOffset int

	sort    SortField
	sortAsc bool

	filterText  string
	s3Bucket    string // non-empty when showing objects inside a bucket
	s3Prefix    string // non-empty when showing objects under a prefix
	r53ZoneId   string // non-empty when showing records inside a hosted zone
	r53ZoneName string // zone name for display

	loading bool
	spinner spinner.Model

	width  int
	height int
	keys   keys.Map

	// rowTextCache caches unstyled row text (renderDataRow output) per
	// filteredResources index. Only the cursor highlight changes between
	// renders during scrolling — the row text itself is identical.
	// Invalidated when resources, filter, sort, hScroll, or width change.
	rowTextCache map[int]string
}

// NewResourceList creates a ResourceListModel in loading state.
func NewResourceList(typeDef resource.ResourceTypeDef, viewConfig *config.ViewsConfig, k keys.Map) ResourceListModel {
	sp := spinner.New()
	return ResourceListModel{
		typeDef:    typeDef,
		viewConfig: viewConfig,
		loading:    true,
		spinner:    sp,
		keys:       k,
	}
}

// NewS3ObjectsList creates a ResourceListModel for objects inside an S3 bucket.
// prefix is optional; pass "" for the root of the bucket.
func NewS3ObjectsList(bucket string, viewConfig *config.ViewsConfig, k keys.Map, prefix ...string) ResourceListModel {
	sp := spinner.New()
	typeDef := resource.ResourceTypeDef{
		Name:      bucket,
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
	}
	pfx := ""
	if len(prefix) > 0 {
		pfx = prefix[0]
	}
	return ResourceListModel{
		typeDef:    typeDef,
		viewConfig: viewConfig,
		s3Bucket:   bucket,
		s3Prefix:   pfx,
		loading:    true,
		spinner:    sp,
		keys:       k,
	}
}

// NewR53RecordsList creates a ResourceListModel for DNS records inside a Route53 hosted zone.
func NewR53RecordsList(zoneId, zoneName string, viewConfig *config.ViewsConfig, k keys.Map) ResourceListModel {
	sp := spinner.New()
	typeDef := resource.ResourceTypeDef{
		Name:      zoneName,
		ShortName: "r53_records",
		Columns:   resource.R53RecordColumns(),
	}
	return ResourceListModel{
		typeDef:     typeDef,
		viewConfig:  viewConfig,
		r53ZoneId:   zoneId,
		r53ZoneName: zoneName,
		loading:     true,
		spinner:     sp,
		keys:        k,
	}
}

// Init starts the spinner tick cycle.
func (m ResourceListModel) Init() (ResourceListModel, tea.Cmd) {
	return m, m.spinner.Tick
}

// Update handles messages: ResourcesLoadedMsg, spinner ticks, key events.
func (m ResourceListModel) Update(msg tea.Msg) (ResourceListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.ResourcesLoadedMsg:
		m.loading = false
		m.allResources = msg.Resources
		m.applyFilter()
		m.rowTextCache = nil
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.filteredResources)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
		case key.Matches(msg, m.keys.Bottom):
			m.cursor = max(0, len(m.filteredResources)-1)
		case key.Matches(msg, m.keys.PageUp):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.cursor -= pageSize
			if m.cursor < 0 {
				m.cursor = 0
			}
		case key.Matches(msg, m.keys.PageDown):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.cursor += pageSize
			if m.cursor >= len(m.filteredResources) {
				m.cursor = max(0, len(m.filteredResources)-1)
			}
		case key.Matches(msg, m.keys.ScrollLeft):
			if m.hScrollOffset > 0 {
				m.hScrollOffset--
				m.rowTextCache = nil
			}
		case key.Matches(msg, m.keys.ScrollRight):
			m.hScrollOffset++
			m.rowTextCache = nil
		case key.Matches(msg, m.keys.Enter):
			if r := m.SelectedResource(); r != nil {
				// Route53 zone list: Enter drills into the zone's records
				if m.typeDef.ShortName == "r53" && m.r53ZoneId == "" {
					zoneId := r.ID
					zoneName := r.Name
					return m, func() tea.Msg {
						return messages.R53EnterZoneMsg{ZoneId: zoneId, ZoneName: zoneName}
					}
				}
				// S3 bucket list: Enter drills into the bucket
				if m.typeDef.ShortName == "s3" && m.s3Bucket == "" {
					bucketName := r.ID
					return m, func() tea.Msg {
						return messages.S3EnterBucketMsg{BucketName: bucketName}
					}
				}
				// S3 object list: Enter on folder navigates into prefix
				if m.s3Bucket != "" && r.Status == "folder" {
					bucket := m.s3Bucket
					prefix := r.ID
					return m, func() tea.Msg {
						return messages.S3NavigatePrefixMsg{Bucket: bucket, Prefix: prefix}
					}
				}
				// Default: open detail view
				return m, func() tea.Msg {
					return messages.NavigateMsg{
						Target:   messages.TargetDetail,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.Describe):
			// d key always opens detail view (never drills into S3)
			if r := m.SelectedResource(); r != nil {
				return m, func() tea.Msg {
					return messages.NavigateMsg{
						Target:   messages.TargetDetail,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.YAML):
			if r := m.SelectedResource(); r != nil {
				return m, func() tea.Msg {
					return messages.NavigateMsg{
						Target:   messages.TargetYAML,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.SortByName):
			if m.sort == SortName {
				m.sortAsc = !m.sortAsc
			} else {
				m.sort = SortName
				m.sortAsc = true
			}
			m.applySortAndFilter()
		case key.Matches(msg, m.keys.SortByStatus):
			if m.sort == SortStatus {
				m.sortAsc = !m.sortAsc
			} else {
				m.sort = SortStatus
				m.sortAsc = true
			}
			m.applySortAndFilter()
		case key.Matches(msg, m.keys.SortByAge):
			if m.sort == SortAge {
				m.sortAsc = !m.sortAsc
			} else {
				m.sort = SortAge
				m.sortAsc = true
			}
			m.applySortAndFilter()
		}
	}
	return m, nil
}

// View renders the table content. Caller wraps in RenderFrame.
func (m ResourceListModel) View() string {
	if m.loading {
		return m.spinner.View() + " Loading..."
	}
	if len(m.filteredResources) == 0 {
		return "No resources found"
	}

	cols := m.resolveColumns()

	// Apply horizontal scroll: skip hScrollOffset columns from the left.
	if m.hScrollOffset > 0 && m.hScrollOffset < len(cols) {
		cols = cols[m.hScrollOffset:]
	} else if m.hScrollOffset >= len(cols) {
		cols = nil
	}

	// Hide rightmost columns that don't fit in width.
	cols = m.fitColumns(cols)

	if len(cols) == 0 {
		return "No resources found"
	}

	// Render header row.
	headerLine := m.renderHeaderRow(cols)

	// Determine visible row count: height minus column header row (1).
	// Frame borders are already excluded from m.height by the root model.
	visibleRows := m.height - 1
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Determine the window of rows to display, keeping cursor centered.
	startRow, endRow := m.visibleWindow(visibleRows)

	var sb strings.Builder
	sb.WriteString(headerLine)

	for i := startRow; i < endRow; i++ {
		sb.WriteString("\n")
		r := m.filteredResources[i]

		// Use cached row text when available (cursor movement doesn't invalidate).
		rowText, ok := m.rowTextCache[i]
		if !ok {
			rowText = m.renderDataRow(cols, r)
			if m.rowTextCache == nil {
				m.rowTextCache = make(map[int]string)
			}
			m.rowTextCache[i] = rowText
		}

		var styled string
		if i == m.cursor {
			styled = styles.RowSelected.Width(m.width).Render(rowText)
		} else {
			styled = styles.RowColorStyle(r.Status).Render(rowText)
		}
		sb.WriteString(styled)
	}

	return sb.String()
}

// listCol is a resolved column definition for rendering.
type listCol struct {
	title string
	width int
	key   string // resource.Fields key (fallback)
	path  string // config-driven path for ExtractScalar
}

// resolveColumns determines the column definitions to use.
func (m ResourceListModel) resolveColumns() []listCol {
	// Check config-driven columns first.
	if m.viewConfig != nil {
		vd := config.GetViewDef(m.viewConfig, m.typeDef.ShortName)
		if len(vd.List) > 0 {
			cols := make([]listCol, len(vd.List))
			for i, lc := range vd.List {
				cols[i] = listCol{
					title: lc.Title,
					width: lc.Width,
					path:  lc.Path,
					key:   lc.Key,
				}
			}
			return cols
		}
	}

	// Fall back to typeDef columns.
	cols := make([]listCol, len(m.typeDef.Columns))
	for i, c := range m.typeDef.Columns {
		cols[i] = listCol{
			title: c.Title,
			width: c.Width,
			key:   c.Key,
		}
	}
	return cols
}

// fitColumns hides rightmost columns that don't fit in the available width.
func (m ResourceListModel) fitColumns(cols []listCol) []listCol {
	if m.width <= 0 {
		return cols
	}
	usedWidth := 1 // leading space
	var fit []listCol
	for _, c := range cols {
		needed := c.width + 2 // column width + 2-space gap
		if usedWidth+needed > m.width && len(fit) > 0 {
			break
		}
		usedWidth += needed
		fit = append(fit, c)
	}
	return fit
}

// renderHeaderRow renders the column header line with sort indicators.
func (m ResourceListModel) renderHeaderRow(cols []listCol) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		title := m.colHeaderTitle(c, i)
		parts[i] = text.PadOrTrunc(title, c.width)
	}
	headerText := " " + strings.Join(parts, "  ")
	return styles.TableHeader.Render(headerText)
}

// colHeaderTitle returns the column title with sort indicator if applicable.
func (m ResourceListModel) colHeaderTitle(c listCol, _ int) string {
	title := c.title
	// Add sort indicator based on active sort field.
	// Match by common patterns: first column is often "name-ish", etc.
	var isActive bool
	switch m.sort {
	case SortName:
		// Name sort applies to column with key "name" or title containing "Name"
		isActive = strings.EqualFold(c.key, "name") || strings.Contains(strings.ToLower(c.title), "name")
	case SortStatus:
		isActive = strings.EqualFold(c.key, "state") || strings.EqualFold(c.key, "status") ||
			strings.Contains(strings.ToLower(c.title), "status") || strings.Contains(strings.ToLower(c.title), "state")
	case SortAge:
		isActive = strings.Contains(strings.ToLower(c.key), "time") || strings.Contains(strings.ToLower(c.key), "date") ||
			strings.Contains(strings.ToLower(c.key), "launch") || strings.Contains(strings.ToLower(c.key), "creation") ||
			strings.Contains(strings.ToLower(c.title), "time") || strings.Contains(strings.ToLower(c.title), "date")
	}
	if isActive {
		if m.sortAsc {
			title += "\u2191"
		} else {
			title += "\u2193"
		}
	}
	return title
}

// renderDataRow renders a single data row.
func (m ResourceListModel) renderDataRow(cols []listCol, r resource.Resource) string {
	cells := make([]string, len(cols))
	for i, c := range cols {
		val := m.extractCellValue(c, r)
		cells[i] = text.PadOrTrunc(val, c.width)
	}
	return " " + strings.Join(cells, "  ")
}

// extractCellValue gets the cell value for a column from a resource.
func (m ResourceListModel) extractCellValue(c listCol, r resource.Resource) string {
	// Try config-driven path via ExtractScalar first (if path set and RawStruct available).
	if c.path != "" && r.RawStruct != nil {
		val := fieldpath.ExtractScalar(r.RawStruct, c.path)
		if val != "" {
			return val
		}
	}
	// Fall back to Fields map.
	if c.key != "" {
		if v, ok := r.Fields[c.key]; ok {
			return v
		}
	}
	// Try matching column title (lowercased) against Fields keys.
	titleLower := strings.ToLower(c.title)
	for k, v := range r.Fields {
		if strings.ToLower(k) == titleLower {
			return v
		}
	}
	return ""
}

// visibleWindow calculates the start and end indices of rows to display.
func (m ResourceListModel) visibleWindow(visibleRows int) (int, int) {
	total := len(m.filteredResources)
	if total <= visibleRows {
		return 0, total
	}

	// Keep cursor centered.
	half := visibleRows / 2
	start := m.cursor - half
	if start < 0 {
		start = 0
	}
	end := start + visibleRows
	if end > total {
		end = total
		start = end - visibleRows
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

// applySortAndFilter re-applies filter and then sorts the filtered results.
func (m *ResourceListModel) applySortAndFilter() {
	m.applyFilter()
	m.sortFiltered()
	m.rowTextCache = nil
}

// sortFiltered sorts filteredResources in place based on current sort settings.
func (m *ResourceListModel) sortFiltered() {
	if m.sort == SortNone {
		return
	}
	sort.SliceStable(m.filteredResources, func(i, j int) bool {
		a := m.filteredResources[i]
		b := m.filteredResources[j]
		var va, vb string
		switch m.sort {
		case SortName:
			va, vb = a.Name, b.Name
		case SortStatus:
			va, vb = a.Status, b.Status
		case SortAge:
			// Use ID as fallback for age sorting (launch_time/creation_date fields)
			va = m.getAgeField(a)
			vb = m.getAgeField(b)
		}
		if m.sortAsc {
			return va < vb
		}
		return va > vb
	})
}

// getAgeField extracts the time-related field for age sorting.
func (m ResourceListModel) getAgeField(r resource.Resource) string {
	for k, v := range r.Fields {
		kl := strings.ToLower(k)
		if strings.Contains(kl, "time") || strings.Contains(kl, "date") ||
			strings.Contains(kl, "launch") || strings.Contains(kl, "creation") {
			return v
		}
	}
	return ""
}

// SetFilter applies a filter; cursor resets to 0.
func (m *ResourceListModel) SetFilter(text string) {
	m.filterText = text
	m.applyFilter()
	m.rowTextCache = nil
	m.cursor = 0
}

// GetFilter returns the current filter text.
func (m *ResourceListModel) GetFilter() string {
	return m.filterText
}

// SetSize updates dimensions.
func (m *ResourceListModel) SetSize(w, h int) {
	if m.width != w {
		m.rowTextCache = nil
	}
	m.width = w
	m.height = h
}

// CopyContent returns the selected resource's ID for clipboard copy.
func (m ResourceListModel) CopyContent() (string, string) {
	if r := m.SelectedResource(); r != nil {
		return r.ID, "Copied: " + r.ID
	}
	return "", ""
}

// GetHelpContext returns HelpFromSecretsList for secrets, HelpFromResourceList otherwise.
func (m ResourceListModel) GetHelpContext() HelpContext {
	if m.typeDef.ShortName == "secrets" {
		return HelpFromSecretsList
	}
	return HelpFromResourceList
}

// SelectedResource returns the resource at cursor, or nil.
func (m ResourceListModel) SelectedResource() *resource.Resource {
	if m.cursor >= 0 && m.cursor < len(m.filteredResources) {
		r := m.filteredResources[m.cursor]
		return &r
	}
	return nil
}

// ResourceType returns the short name of the resource type (e.g., "ec2", "secrets").
func (m ResourceListModel) ResourceType() string {
	return m.typeDef.ShortName
}

// S3Bucket returns the S3 bucket name if this view is showing objects inside a bucket.
// Returns "" for non-S3 views or the bucket list.
func (m ResourceListModel) S3Bucket() string {
	return m.s3Bucket
}

// S3Prefix returns the last-used S3 prefix for this view.
// Returns "" when not applicable.
func (m ResourceListModel) S3Prefix() string {
	return m.s3Prefix
}

// R53ZoneId returns the Route53 hosted zone ID if this view is showing records inside a zone.
func (m ResourceListModel) R53ZoneId() string {
	return m.r53ZoneId
}

// ClearLoading clears the loading state so the view no longer shows a spinner.
func (m *ResourceListModel) ClearLoading() {
	m.loading = false
}

// FrameTitle returns e.g. "ec2-instances(42)" or "ec2-instances(3/42)" when filtered.
// For S3 objects, shows the bucket name.
// During loading, returns just the name without count.
func (m ResourceListModel) FrameTitle() string {
	name := m.typeDef.ShortName
	if m.s3Bucket != "" {
		name = m.s3Bucket
	}
	if m.r53ZoneId != "" {
		name = m.r53ZoneName
	}
	if m.loading {
		return name
	}
	total := len(m.allResources)
	filtered := len(m.filteredResources)
	if m.filterText != "" && filtered != total {
		return name + "(" + itoa(filtered) + "/" + itoa(total) + ")"
	}
	return name + "(" + itoa(total) + ")"
}

// applyFilter filters allResources into filteredResources.
func (m *ResourceListModel) applyFilter() {
	m.filteredResources = FilterResources(m.filterText, m.allResources)
}

// FilterResources returns resources matching the query (case-insensitive).
// Exported so tests can call it directly.
func FilterResources(query string, resources []resource.Resource) []resource.Resource {
	if query == "" {
		return resources
	}
	q := strings.ToLower(query)
	result := make([]resource.Resource, 0)
	for _, r := range resources {
		if strings.Contains(strings.ToLower(r.ID), q) ||
			strings.Contains(strings.ToLower(r.Name), q) ||
			strings.Contains(strings.ToLower(r.Status), q) {
			result = append(result, r)
			continue
		}
		for _, v := range r.Fields {
			if strings.Contains(strings.ToLower(v), q) {
				result = append(result, r)
				break
			}
		}
	}
	return result
}

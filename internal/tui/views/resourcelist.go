package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// ResourceListModel is a tea.Model for the resource table view.
type ResourceListModel struct {
	typeDef    resource.ResourceTypeDef
	viewConfig *config.ViewsConfig

	allResources      []resource.Resource
	filteredResources []resource.Resource

	scroll        ScrollState
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

	// styledRowCache caches fully styled row strings (with cursor highlight
	// or status color applied). On cursor move, only the old and new cursor
	// positions are invalidated. Full invalidation happens when data, filter,
	// sort, width, or hScroll changes.
	styledRowCache map[int]string
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
		m.styledRowCache = nil
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		oldCursor := m.scroll.Cursor()
		switch {
		case key.Matches(msg, m.keys.Up):
			m.scroll.Up()
		case key.Matches(msg, m.keys.Down):
			m.scroll.Down()
		case key.Matches(msg, m.keys.Top):
			m.scroll.Top()
		case key.Matches(msg, m.keys.Bottom):
			m.scroll.Bottom()
		case key.Matches(msg, m.keys.PageUp):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.scroll.PageUp(pageSize)
		case key.Matches(msg, m.keys.PageDown):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.scroll.PageDown(pageSize)
		case key.Matches(msg, m.keys.ScrollLeft):
			if m.hScrollOffset > 0 {
				m.hScrollOffset--
				m.rowTextCache = nil
				m.styledRowCache = nil
			}
		case key.Matches(msg, m.keys.ScrollRight):
			cols := m.resolveColumns()
			visible := cols
			if m.hScrollOffset < len(cols) {
				visible = cols[m.hScrollOffset:]
			}
			// Only scroll if there are columns hidden beyond the right edge.
			if len(m.fitColumns(visible)) < len(visible) {
				m.hScrollOffset++
				m.rowTextCache = nil
				m.styledRowCache = nil
			}
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
		case key.Matches(msg, m.keys.SortByID):
			if m.sort == SortID {
				m.sortAsc = !m.sortAsc
			} else {
				m.sort = SortID
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
		// Invalidate styled row cache for old and new cursor positions.
		if oldCursor != m.scroll.Cursor() {
			delete(m.styledRowCache, oldCursor)
			delete(m.styledRowCache, m.scroll.Cursor())
		}
	}
	return m, nil
}

// View renders the table content. Caller wraps in RenderFrame.
// Pointer receiver so that row caches persist across frames.
func (m *ResourceListModel) View() string {
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
	startRow, endRow := m.scroll.VisibleWindow(visibleRows)

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

		styled, ok2 := m.styledRowCache[i]
		if !ok2 {
			if i == m.scroll.Cursor() {
				styled = styles.RowSelected.Width(m.width).Render(rowText)
			} else {
				styled = styles.RowColorStyle(r.Status).Render(rowText)
			}
			if m.styledRowCache == nil {
				m.styledRowCache = make(map[int]string)
			}
			m.styledRowCache[i] = styled
		}
		sb.WriteString(styled)
	}

	return sb.String()
}

// applySortAndFilter re-applies filter and then sorts the filtered results.
func (m *ResourceListModel) applySortAndFilter() {
	m.applyFilter()
	m.sortFiltered()
	m.rowTextCache = nil
	m.styledRowCache = nil
}

// SetFilter applies a filter; cursor resets to 0.
func (m *ResourceListModel) SetFilter(text string) {
	m.filterText = text
	m.applyFilter()
	m.rowTextCache = nil
	m.styledRowCache = nil
	m.scroll.SetCursor(0)
}

// GetFilter returns the current filter text.
func (m *ResourceListModel) GetFilter() string {
	return m.filterText
}

// SetSize updates dimensions.
func (m *ResourceListModel) SetSize(w, h int) {
	if m.width != w {
		m.rowTextCache = nil
		m.styledRowCache = nil
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
	c := m.scroll.Cursor()
	if c >= 0 && c < len(m.filteredResources) {
		r := m.filteredResources[c]
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
	m.scroll.SetTotal(len(m.filteredResources))
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

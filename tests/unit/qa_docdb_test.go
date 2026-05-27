package unit

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// Helpers for DocumentDB tests
// ===========================================================================

func docdbTypeDef() resource.ResourceTypeDef {
	for _, rt := range resource.AllResourceTypes() {
		if rt.ShortName == "dbc" {
			return rt
		}
	}
	panic("docdb resource type not found")
}

func loadedDocDBModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	td := docdbTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    fixtureDocDBClusters(),
	})
	return m
}

// multiStatusDocDBFixtures returns DocumentDB clusters with different statuses for color tests.
func multiStatusDocDBFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID: "docdb-available", Name: "docdb-available",
			Fields: map[string]string{
				"cluster_id": "docdb-available", "engine_version": "5.0.0",
				"status": "available", "instances": "2",
				"endpoint": "docdb-available.cluster-abc.docdb.amazonaws.com",
			},
			RawStruct: docdbtypes.DBCluster{
				DBClusterIdentifier: aws.String("docdb-available"),
				EngineVersion:       aws.String("5.0.0"),
				Status:              aws.String("available"),
				Endpoint:            aws.String("docdb-available.cluster-abc.docdb.amazonaws.com"),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("docdb-available-inst-1"), IsClusterWriter: aws.Bool(true)},
					{DBInstanceIdentifier: aws.String("docdb-available-inst-2"), IsClusterWriter: aws.Bool(false)},
				},
			},
		},
		{
			ID: "docdb-creating", Name: "docdb-creating",
			Fields: map[string]string{
				"cluster_id": "docdb-creating", "engine_version": "5.0.0",
				"status": "creating", "instances": "0",
				"endpoint": "",
			},
			RawStruct: docdbtypes.DBCluster{
				DBClusterIdentifier: aws.String("docdb-creating"),
				EngineVersion:       aws.String("5.0.0"),
				Status:              aws.String("creating"),
			},
		},
		{
			ID: "docdb-deleting", Name: "docdb-deleting",
			Fields: map[string]string{
				"cluster_id": "docdb-deleting", "engine_version": "5.0.0",
				"status": "deleting", "instances": "1",
				"endpoint": "docdb-deleting.cluster-abc.docdb.amazonaws.com",
			},
			RawStruct: docdbtypes.DBCluster{
				DBClusterIdentifier: aws.String("docdb-deleting"),
				EngineVersion:       aws.String("5.0.0"),
				Status:              aws.String("deleting"),
				Endpoint:            aws.String("docdb-deleting.cluster-abc.docdb.amazonaws.com"),
				DBClusterMembers: []docdbtypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("docdb-deleting-inst-1"), IsClusterWriter: aws.Bool(true)},
				},
			},
		},
	}
}

// ===========================================================================
// DOCDB-LIST-02: DocumentDB list displays correct columns
// ===========================================================================

func TestQA_DocDB_ListColumns(t *testing.T) {
	m := loadedDocDBModel(t)
	out := m.View()

	expectedHeaders := []string{"Cluster ID", "Version", "Status", "Instances", "Endpoint"}
	for _, header := range expectedHeaders {
		if !strings.Contains(out, header) {
			t.Errorf("DocumentDB list view missing column header %q", header)
		}
	}
}

// ===========================================================================
// DOCDB-LIST-03: DocumentDB list populates column data from correct fields
// ===========================================================================

func TestQA_DocDB_ListColumnData(t *testing.T) {
	fixtures := fixtureDocDBClusters()
	m := loadedDocDBModel(t)
	out := m.View()

	for _, r := range fixtures {
		id := r.Fields["cluster_id"]
		if id != "" && !strings.Contains(out, id) {
			t.Errorf("DocumentDB list missing cluster_id %q", id)
		}
	}

	// Verify specific data fields appear
	r := fixtures[0]
	for _, field := range []string{"engine_version", "status", "instances"} {
		val := r.Fields[field]
		if val == "" {
			continue
		}
		if !strings.Contains(out, val) {
			t.Errorf("DocumentDB list missing field %q value %q", field, val)
		}
	}
}

// ===========================================================================
// DOCDB-LIST-04: DocumentDB list row count appears in frame title
// ===========================================================================

func TestQA_DocDB_FrameTitle(t *testing.T) {
	m := loadedDocDBModel(t)
	title := m.FrameTitle()

	expected := "dbc(2)"
	if title != expected {
		t.Errorf("expected FrameTitle() = %q, got %q", expected, title)
	}
}

// ===========================================================================
// DOCDB-LIST-05: DocumentDB list shows member count for Instances column
// ===========================================================================

func TestQA_DocDB_InstancesCount(t *testing.T) {
	m := loadedDocDBModel(t)
	out := m.View()

	// Fixture has "instances": "1" -- verify it shows as "1" not as array data
	if !strings.Contains(out, "1") {
		t.Error("DocumentDB Instances column should show count '1'")
	}
}

// ===========================================================================
// DOCDB-LIST-07: DocumentDB list row coloring by status
// ===========================================================================

func TestQA_DocDB_StatusColoring(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := docdbTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    multiStatusDocDBFixtures(),
	})

	out := m.View()

	for _, status := range []string{"available", "creating", "deleting"} {
		if !strings.Contains(out, status) {
			t.Errorf("DocumentDB list missing status %q in rendered output", status)
		}
	}

	if !strings.Contains(out, "\x1b[") {
		t.Error("DocumentDB list with status colors should contain ANSI escape sequences")
	}
}

// ===========================================================================
// DOCDB-LIST-08: DocumentDB list cursor navigation
// ===========================================================================

func TestQA_DocDB_CursorNavigation(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := docdbTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    multiStatusDocDBFixtures(),
	})

	// Initial selection
	sel := m.SelectedResource()
	if sel == nil || sel.ID != "docdb-available" {
		t.Fatalf("expected initial selection to be docdb-available, got %v", sel)
	}

	// Move down
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	sel = m.SelectedResource()
	if sel == nil || sel.ID != "docdb-creating" {
		t.Errorf("after 'j', expected docdb-creating, got %v", sel)
	}

	// Move up
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
	sel = m.SelectedResource()
	if sel == nil || sel.ID != "docdb-available" {
		t.Errorf("after 'k', expected docdb-available, got %v", sel)
	}

	// Jump to bottom
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "G"})
	sel = m.SelectedResource()
	if sel == nil || sel.ID != "docdb-deleting" {
		t.Errorf("after 'G', expected docdb-deleting, got %v", sel)
	}

	// Jump to top
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "g"})
	sel = m.SelectedResource()
	if sel == nil || sel.ID != "docdb-available" {
		t.Errorf("after 'g', expected docdb-available, got %v", sel)
	}
}

// ===========================================================================
// DOCDB-LIST-10: DocumentDB list filter
// ===========================================================================

func TestQA_DocDB_ListFilter(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := docdbTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    multiStatusDocDBFixtures(),
	})

	m.SetFilter("creating")
	out := m.View()

	if !strings.Contains(out, "docdb-creating") {
		t.Error("filtered DocumentDB list should contain 'docdb-creating'")
	}
	if strings.Contains(out, "docdb-available") {
		t.Error("filtered DocumentDB list should NOT contain 'docdb-available'")
	}

	title := m.FrameTitle()
	if title != "dbc(1/3)" {
		t.Errorf("expected filtered FrameTitle = %q, got %q", "dbc(1/3)", title)
	}

	m.SetFilter("")
	title = m.FrameTitle()
	if title != "dbc(3)" {
		t.Errorf("expected unfiltered FrameTitle = %q, got %q", "dbc(3)", title)
	}
}

// ===========================================================================
// DOCDB-LIST-11: DocumentDB list sorting
// ===========================================================================

func TestQA_DocDB_ListSort(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := docdbTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    multiStatusDocDBFixtures(),
	})

	// Sort by column 0 ('1') -- "Cluster ID" column (key "cluster_id", index 0, 1-indexed key "1")
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "1"})
	out := m.View()
	if !strings.Contains(out, "\u2191") && !strings.Contains(out, "\u2193") {
		t.Error("expected sort indicator arrow after pressing 1 for DocumentDB Cluster ID column")
	}

	// Toggle sort direction
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "1"})
	out2 := m.View()
	if !strings.Contains(out2, "\u2191") && !strings.Contains(out2, "\u2193") {
		t.Error("expected sort indicator to remain after toggling DocumentDB sort direction")
	}

	// Sort by column 1 ('2') -- "Version" column; verify sort happens (selected resource may change)
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "2"})
	sel := m.SelectedResource()
	if sel == nil {
		t.Error("after sort by column 2, should still have a selected DocumentDB resource")
	}
}

// ===========================================================================
// DOCDB-LIST-13: DocumentDB list with no clusters
// ===========================================================================

func TestQA_DocDB_EmptyList(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := docdbTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    []resource.Resource{},
	})

	out := m.View()
	if !strings.Contains(out, "No resources found") {
		t.Errorf("empty DocumentDB list should show 'No resources found', got: %q", out)
	}

	title := m.FrameTitle()
	if title != "dbc(0)" {
		t.Errorf("expected empty FrameTitle = %q, got %q", "dbc(0)", title)
	}
}

// ===========================================================================
// DOCDB-DETAIL-01 / DOCDB-DETAIL-02: DocumentDB detail view
// ===========================================================================

func TestQA_DocDB_DetailView(t *testing.T) {
	fixtures := fixtureDocDBClusters()
	k := keys.Default()
	res := fixtures[0]
	// Use wide viewport to avoid truncation of long endpoint values
	// (right panel auto-shows at width>=60 when related defs are registered, reducing left column)
	m := views.NewDetail(res, "dbc", nil, k)
	m.SetSize(200, 20)
	out := m.View()

	if out == "" || out == "Initializing..." {
		t.Fatal("DocumentDB detail view returned empty or initializing")
	}

	for key, val := range res.Fields {
		if val == "" {
			continue
		}
		if !strings.Contains(out, val) {
			t.Errorf("DocumentDB detail view missing field value for %q: %q", key, val)
		}
	}
}

func TestQA_DocDB_DetailFrameTitle(t *testing.T) {
	fixtures := fixtureDocDBClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewDetail(res, "dbc", nil, k)
	title := m.FrameTitle()

	expected := res.Name
	if expected == "" {
		expected = res.ID
	}
	if title != expected {
		t.Errorf("DocumentDB detail FrameTitle = %q, want %q", title, expected)
	}
}

// ===========================================================================
// DOCDB-DETAIL-05: DocumentDB detail status coloring (per-type Color func)
// ===========================================================================

func TestQA_DocDB_DetailStatusColoring(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	// DocumentDB cluster (dbc) Color func reads Fields["status"].
	td := resource.FindResourceType("dbc")
	if td == nil {
		t.Fatal("dbc resource type not found")
	}
	if td.Color == nil {
		t.Fatal("dbc Color func is nil")
	}

	dbcRes := func(status string) resource.Resource {
		return resource.Resource{
			ID:     "cluster-001",
			Fields: map[string]string{"status": status},
		}
	}

	// Post-refactor Fields["status"] carries the §4 PHRASE, not the raw AWS
	// keyword. Healthy = blank; transitional = "<status>: in progress"; Broken
	// phrases are spelled out per spec §4.
	availableStyle := styles.ColorStyle(td.Color(dbcRes("")))
	if availableStyle.GetForeground() != styles.ColRunning {
		t.Errorf("dbc healthy (blank): expected ColRunning (#9ece6a), got %v", availableStyle.GetForeground())
	}

	creatingStyle := styles.ColorStyle(td.Color(dbcRes("creating: in progress")))
	if creatingStyle.GetForeground() != styles.ColPending {
		t.Errorf("dbc 'creating: in progress': expected ColPending (#e0af68), got %v", creatingStyle.GetForeground())
	}

	// deleting is not in the transitional set per spec §3.1 — only creating,
	// modifying, backing-up, maintenance, upgrading, starting, stopping,
	// resetting-master-credentials, renaming are. Use 'modifying' as the
	// transitional-warning probe.
	modifyingStyle := styles.ColorStyle(td.Color(dbcRes("modifying: in progress")))
	if modifyingStyle.GetForeground() != styles.ColPending {
		t.Errorf("dbc 'modifying: in progress': expected ColPending (Warning per spec), got %v", modifyingStyle.GetForeground())
	}
}

// ===========================================================================
// DOCDB-YAML-01 / DOCDB-YAML-03: DocumentDB YAML view
// ===========================================================================

func TestQA_DocDB_YAMLView(t *testing.T) {
	fixtures := fixtureDocDBClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAML(res, "", k)
	m.SetSize(80, 30)
	out := m.View()

	if out == "" || out == "Initializing..." {
		t.Fatal("DocumentDB YAML view returned empty or initializing")
	}

	// YAML view renders from RawStruct (SDK struct field names) when RawStruct is set
	expectedKeys := []string{"DBClusterIdentifier", "EngineVersion", "Status", "Endpoint", "DBClusterMembers"}
	for _, key := range expectedKeys {
		if !strings.Contains(out, key) {
			t.Errorf("DocumentDB YAML view missing SDK struct key %q", key)
		}
	}
	// Values from the RawStruct should appear
	expectedValues := []string{"test-docdb-cluster", "5.0.0", "available"}
	for _, val := range expectedValues {
		if !strings.Contains(out, val) {
			t.Errorf("DocumentDB YAML view missing value %q", val)
		}
	}
}

func TestQA_DocDB_YAMLFrameTitle(t *testing.T) {
	fixtures := fixtureDocDBClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAML(res, "", k)
	title := m.FrameTitle()

	expected := res.Name + " yaml"
	if res.Name == "" {
		expected = res.ID + " yaml"
	}
	if title != expected {
		t.Errorf("DocumentDB YAML FrameTitle = %q, want %q", title, expected)
	}
}

// ===========================================================================
// DOCDB-YAML-07: DocumentDB YAML raw content for copy
// ===========================================================================

func TestQA_DocDB_YAMLRawContent(t *testing.T) {
	fixtures := fixtureDocDBClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAML(res, "", k)
	raw := m.RawContent()

	if raw == "" {
		t.Fatal("DocumentDB YAML RawContent() returned empty string")
	}

	// RawContent renders from RawStruct (SDK struct field names)
	expectedKeys := []string{"DBClusterIdentifier", "EngineVersion", "Status", "Endpoint", "DBClusterMembers"}
	for _, key := range expectedKeys {
		if !strings.Contains(raw, key) {
			t.Errorf("DocumentDB YAML RawContent missing SDK struct key %q", key)
		}
	}
}

// ===========================================================================
// DocumentDB integration: full root model navigation
// ===========================================================================

func TestQA_DocDB_NavigateFromMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})
	_ = cmd

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbc") {
		t.Errorf("after navigate to DocumentDB, frame should contain 'docdb', got: %s", plain)
	}
}

func TestQA_DocDB_LoadAndDisplayList(t *testing.T) {
	fixtures := fixtureDocDBClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    fixtures,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbc(2)") {
		t.Errorf("after loading DocumentDB, frame title should contain 'docdb(2)', got: %s", plain)
	}
	// Verify column headers are present (cell data requires RawStruct for config-driven paths)
	if !strings.Contains(plain, "Cluster ID") {
		t.Errorf("DocumentDB list should contain 'Cluster ID' column header, got: %s", plain)
	}
}

func TestQA_DocDB_NavigateToDetail(t *testing.T) {
	fixtures := fixtureDocDBClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    fixtures,
	})

	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	plain := stripANSI(rootViewContent(m))
	expected := res.Name
	if expected == "" {
		expected = res.ID
	}
	if !strings.Contains(plain, expected) {
		t.Errorf("DocumentDB detail frame should contain %q, got: %s", expected, plain)
	}
}

func TestQA_DocDB_NavigateToYAML(t *testing.T) {
	fixtures := fixtureDocDBClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    fixtures,
	})

	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: &res,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Errorf("DocumentDB YAML frame should contain 'yaml', got: %s", plain)
	}
}

func TestQA_DocDB_DetailBackNavigation(t *testing.T) {
	fixtures := fixtureDocDBClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    fixtures,
	})

	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	// Pop back
	m, _ = rootApplyMsg(m, messages.PopView{})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbc") {
		t.Errorf("after pop from DocumentDB detail, should return to DocumentDB list, got: %s", plain)
	}
}

// ===========================================================================
// DOCDB command mode: :dbc navigates correctly
// ===========================================================================

func TestQA_DocDB_CommandNavigation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	// Type "dbc"
	for _, r := range "dbc" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	// Press enter
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Error("executeCommand('docdb') should return a command (NavigateMsg)")
	}
}

// ===========================================================================
// DOCDB: Horizontal scroll
// ===========================================================================

func TestQA_DocDB_HorizontalScroll(t *testing.T) {
	fixtures := fixtureDocDBClusters()

	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := docdbTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(50, 20) // very narrow
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    fixtures,
	})

	outBefore := m.View()

	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
	outAfter := m.View()

	if outBefore == outAfter {
		t.Error("expected horizontal scroll to change DocumentDB list output")
	}
}

// ===========================================================================
// DOCDB: Loading shows spinner
// ===========================================================================

func TestQA_DocDB_LoadingSpinner(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := docdbTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()

	out := m.View()
	if !strings.Contains(out, "Loading") {
		t.Error("DocumentDB list in loading state should show 'Loading'")
	}

	title := m.FrameTitle()
	if title != "dbc" {
		t.Errorf("DocumentDB loading FrameTitle = %q, want %q", title, "dbc")
	}
}

// ===========================================================================
// CROSS-CMD-01: Switch between Redis and DocumentDB via command
// ===========================================================================

func TestQA_CrossCommand_SwitchRedisToDocDB(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to Redis
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Fatalf("should be on Redis view, got: %s", plain)
	}

	// Enter command mode and type :dbc
	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, r := range "dbc" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	// Execute the command if returned
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbc") {
		t.Errorf("after :dbc command from Redis, should navigate to DocumentDB, got: %s", plain)
	}
}

func TestQA_CrossCommand_SwitchDocDBToRedis(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to DocumentDB
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})

	// Enter command mode and type :redis
	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, r := range "redis" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Errorf("after :redis command from DocumentDB, should navigate to Redis, got: %s", plain)
	}
}

// ===========================================================================
// DOCDB-LIST-06: DocumentDB list shows zero for cluster with no members
// ===========================================================================

func TestQA_DocDB_ZeroInstancesCount(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := docdbTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()

	fixtures := []resource.Resource{
		{
			ID: "docdb-empty", Name: "docdb-empty",
			Fields: map[string]string{
				"cluster_id": "docdb-empty", "engine_version": "5.0.0",
				"status": "creating", "instances": "0",
				"endpoint": "",
			},
		},
	}
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    fixtures,
	})

	out := m.View()
	if !strings.Contains(out, "0") {
		t.Error("DocumentDB Instances column should show '0' for empty members")
	}
}

// ===========================================================================
// DOCDB: No separator row below headers
// ===========================================================================

func TestQA_DocDB_NoSeparatorBelowHeaders(t *testing.T) {
	m := loadedDocDBModel(t)
	out := m.View()

	lines := strings.SplitSeq(out, "\n")
	for line := range lines {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		allDash := true
		for _, ch := range stripped {
			if ch != '-' && ch != '_' && ch != '=' && ch != ' ' {
				allDash = false
				break
			}
		}
		if allDash && len(stripped) > 5 {
			t.Errorf("found what looks like a separator row in DocumentDB list: %q", stripped)
		}
	}
}

// ===========================================================================
// DOCDB-YAML-08: YAML back navigation via root model
// ===========================================================================

func TestQA_DocDB_YAMLBackNavigation(t *testing.T) {
	fixtures := fixtureDocDBClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate: DocDB list -> YAML -> pop
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    fixtures,
	})
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: &res,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Fatalf("should be on YAML view, got: %s", plain)
	}

	m, _ = rootApplyMsg(m, messages.PopView{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbc") {
		t.Errorf("after pop from DocumentDB YAML, should return to DocumentDB list, got: %s", plain)
	}
}

// ===========================================================================
// DocDB: Full round-trip list -> detail -> yaml -> pop -> pop -> pop
// ===========================================================================

func TestQA_DocDB_FullNavigationRoundTrip(t *testing.T) {
	fixtures := fixtureDocDBClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Main menu -> DocumentDB list
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    fixtures,
	})

	// DocumentDB list -> detail
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	// Detail -> YAML
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: &res,
	})

	// Pop YAML -> detail
	m, _ = rootApplyMsg(m, messages.PopView{})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, res.Name) && !strings.Contains(plain, res.ID) {
		t.Errorf("pop from YAML should return to detail, got: %s", plain)
	}

	// Pop detail -> list
	m, _ = rootApplyMsg(m, messages.PopView{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbc") {
		t.Errorf("pop from detail should return to DocumentDB list, got: %s", plain)
	}

	// Pop list -> main menu
	m, _ = rootApplyMsg(m, messages.PopView{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("pop from DocumentDB list should return to main menu, got: %s", plain)
	}
}

// ===========================================================================
// DocDB: Filter via root model header display
// ===========================================================================

func TestQA_DocDB_FilterHeaderDisplay(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to DocumentDB
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbc",
		Resources:    multiStatusDocDBFixtures(),
	})

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "prod" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/prod") {
		t.Errorf("header should show filter text '/prod' during filter mode, got: %s", plain)
	}
}

// ===========================================================================
// CROSS-HELP-01: Help accessible from DocumentDB view
// ===========================================================================

func TestQA_DocDB_HelpOverlay(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbc",
	})

	// Open help
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Errorf("help overlay should be visible from DocumentDB view, got: %s", plain)
	}
}

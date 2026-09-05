package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/viewport"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ===========================================================================
// Helpers for DocumentDB tests
// ===========================================================================
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
// DOCDB-DETAIL-01 / DOCDB-DETAIL-02: DocumentDB detail view
// ===========================================================================

// TestQA_DocDB_DetailView is the live-seam replacement for the retired
// views.NewDetail(...).View() call (DetailModel.View is dead; see
// specs/022-codebase-cleanup/wave3-map-detail.md) — drives
// Controller.EnsureDetailState + NewTransientDetail.RenderDetail instead.
// Uses a wide viewport to avoid truncation of long endpoint values (right
// panel auto-shows at width>=60 when related defs are registered, reducing
// left column).
func TestQA_DocDB_DetailView(t *testing.T) {
	fixtures := fixtureDocDBClusters()
	res := fixtures[0]
	c := newDetailControllerUnit(t, res, "dbc")

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	vp := viewport.New(viewport.WithWidth(200), viewport.WithHeight(20))
	m := views.NewTransientDetail(200, 20, vp)
	out := m.RenderDetail(*body)

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

// TestQA_DocDB_DetailFrameTitle is the live-seam replacement for the retired
// views.NewDetail(...).FrameTitle() call (DetailModel.FrameTitle is dead) —
// drives Controller.Snapshot().FrameTitle instead (detailFrameTitleLocked
// mirrors the legacy Name-else-ID semantics exactly).
func TestQA_DocDB_DetailFrameTitle(t *testing.T) {
	fixtures := fixtureDocDBClusters()
	res := fixtures[0]
	c := newDetailControllerUnit(t, res, "dbc")
	title := c.Snapshot().FrameTitle

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
	tuitest.ForceColor(t)

	// DocumentDB cluster (dbc) Color func reads Fields["status"].
	td := resource.FindResourceType("dbc")
	if td == nil {
		t.Fatal("dbc resource type not found")
	}
	if td.Color == nil {
		t.Fatal("dbc Color func is nil")
	}

	// The probe carries the finding its phrase came from: severity travels with
	// the finding, so a row holding only a Fields entry has nothing to colour by.
	dbcRes := func(status string, sev domain.Severity) resource.Resource {
		r := resource.Resource{
			ID:     "cluster-001",
			Fields: map[string]string{"status": status},
		}
		if status != "" {
			r.Findings = []domain.Finding{{
				Code: "dbc.probe", Phrase: status, Severity: sev, Source: "wave1",
			}}
		}
		return r
	}

	// Post-refactor Fields["status"] carries the §4 PHRASE, not the raw AWS
	// keyword. Healthy = blank; transitional = "<status>: in progress"; Broken
	// phrases are spelled out per spec §4.
	availableStyle := styles.ColorStyle(td.Color(dbcRes("", domain.SevOK)))
	if availableStyle.GetForeground() != styles.ColRunning {
		t.Errorf("dbc healthy (blank): expected ColRunning (#9ece6a), got %v", availableStyle.GetForeground())
	}

	creatingStyle := styles.ColorStyle(td.Color(dbcRes("creating: in progress", domain.SevWarn)))
	if creatingStyle.GetForeground() != styles.ColPending {
		t.Errorf("dbc 'creating: in progress': expected ColPending (#e0af68), got %v", creatingStyle.GetForeground())
	}

	// deleting is not in the transitional set per spec §3.1 — only creating,
	// modifying, backing-up, maintenance, upgrading, starting, stopping,
	// resetting-master-credentials, renaming are. Use 'modifying' as the
	// transitional-warning probe.
	modifyingStyle := styles.ColorStyle(td.Color(dbcRes("modifying: in progress", domain.SevWarn)))
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
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(80, 30)
	out := strings.Join(m.ContentLines(), "\n")

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

// TestQA_DocDB_YAMLFrameTitle retired: YAMLModel.FrameTitle() is DEAD per
// specs/022-codebase-cleanup/wave3-map-text.md (no production caller), and
// title-string behavior is not resource-type-specific.

// ===========================================================================
// DOCDB-YAML-07: DocumentDB YAML raw content for copy
// ===========================================================================

func TestQA_DocDB_YAMLRawContent(t *testing.T) {
	fixtures := fixtureDocDBClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	raw := stripANSI(strings.Join(m.ContentLines(), "\n"))

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
		Resources:    fixtures, Provenance: messages.FetchProvenanceCanonicalList,
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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

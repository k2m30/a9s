package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/viewport"

	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/config"
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
// Helpers for RDS tests
// ===========================================================================

// rdsTypeDef returns the RDS type definition from the registry.
func rdsTypeDef() resource.ResourceTypeDef {
	td := resource.FindResourceType("dbi")
	if td == nil {
		panic("rds resource type not found")
	}
	return *td
}

// rdsLoadedModel returns a ResourceListModel loaded with fixture RDS data.
func rdsLoadedModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	tuitest.ForceColor(t)

	td := rdsTypeDef()
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(160, 20)
	m, _ = m.Init()
	ctrl.ApplyResourcesLoaded("dbi", fixtureRDSInstances(), nil, false)
	return m
}

// rdsKeyPress creates a tea.KeyPressMsg for a printable character.
func rdsKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// ===========================================================================
// A.2 Column Layout
// ===========================================================================

func TestQA_RDS_ListColumns_ColumnWidths(t *testing.T) {
	// Verify that the resource type definition has the correct column widths per spec.
	td := rdsTypeDef()
	expectedWidths := map[string]int{
		"db_identifier":  28,
		"engine":         12,
		"engine_version": 10,
		// The width the built-in view renders this column at; the view's list
		// and the type's are one.
		"status":   28,
		"class":    16,
		"endpoint": 40,
		"multi_az": 10,
	}
	for _, col := range td.Columns {
		expected, ok := expectedWidths[col.Key]
		if !ok {
			continue
		}
		if col.Width != expected {
			t.Errorf("column %q width: expected %d, got %d", col.Key, expected, col.Width)
		}
	}
}

// ===========================================================================
// A.4 Status Coloring — uses per-type Color func via styles.ColorStyle
// ===========================================================================

// rdsColorResource returns a minimal RDS Resource for a given status value.
// Uses the canonical "status" key (the legacy "db_instance_status" fallback
// was removed in #284).
// rdsColorResource builds a probe row for a status. The severity travels with
// the finding that produced the phrase, so a probe carrying only a Fields entry
// would describe a row the fetcher cannot produce.
func rdsColorResource(dbStatus string) resource.Resource {
	r := resource.Resource{
		ID:     "db-test-001",
		Fields: map[string]string{"status": dbStatus},
	}
	sev, ok := rdsProbeSeverity[dbStatus]
	if ok {
		r.Findings = []domain.Finding{{
			Code: domain.FindingCode("dbi.probe." + dbStatus), Phrase: dbStatus, Severity: sev, Source: "wave1",
		}}
	}
	return r
}

// rdsProbeSeverity mirrors computeDBIFindings for the statuses these tests
// probe: available is healthy and carries no finding at all.
var rdsProbeSeverity = map[string]domain.Severity{ //nolint:gochecknoglobals // test-only lookup table
	"failed":   domain.SevBroken,
	"stopped":  domain.SevBroken,
	"creating": domain.SevWarn,
}

func TestQA_RDS_StatusColor_Available(t *testing.T) {
	tuitest.ForceColor(t)
	td := resource.FindResourceType("rds")
	if td == nil {
		t.Fatal("rds resource type not found")
	}
	r := rdsColorResource("available")
	style := styles.ColorStyle(td.Color(r))
	rendered := style.Render("test-available")
	if styles.NoColorActive() {
		t.Skip("NO_COLOR is set, skipping color assertion")
	}
	if rendered == "test-available" {
		t.Error("ColorStyle(rds.Color({available})) should apply green color styling")
	}
	if style.GetForeground() != styles.ColRunning {
		t.Errorf("rds available: expected ColRunning foreground, got %v", style.GetForeground())
	}
}

func TestQA_RDS_StatusColor_Stopped(t *testing.T) {
	tuitest.ForceColor(t)
	td := resource.FindResourceType("rds")
	if td == nil {
		t.Fatal("rds resource type not found")
	}
	r := rdsColorResource("stopped")
	style := styles.ColorStyle(td.Color(r))
	rendered := style.Render("test-stopped")
	if styles.NoColorActive() {
		t.Skip("NO_COLOR is set, skipping color assertion")
	}
	if rendered == "test-stopped" {
		t.Error("ColorStyle(rds.Color({stopped})) should apply red color styling")
	}
}

func TestQA_RDS_StatusColor_Creating(t *testing.T) {
	tuitest.ForceColor(t)
	td := resource.FindResourceType("rds")
	if td == nil {
		t.Fatal("rds resource type not found")
	}
	r := rdsColorResource("creating")
	style := styles.ColorStyle(td.Color(r))
	rendered := style.Render("test-creating")
	if styles.NoColorActive() {
		t.Skip("NO_COLOR is set, skipping color assertion")
	}
	if rendered == "test-creating" {
		t.Error("ColorStyle(rds.Color({creating})) should apply yellow color styling")
	}
}

func TestQA_RDS_StatusColor_Modifying(t *testing.T) {
	tuitest.ForceColor(t)
	td := resource.FindResourceType("rds")
	if td == nil {
		t.Fatal("rds resource type not found")
	}
	r := rdsColorResource("modifying")
	style := styles.ColorStyle(td.Color(r))
	rendered := style.Render("test-modifying")
	if styles.NoColorActive() {
		t.Skip("NO_COLOR is set, skipping color assertion")
	}
	if rendered == "test-modifying" {
		t.Error("ColorStyle(rds.Color({modifying})) should apply yellow color styling")
	}
}

func TestQA_RDS_StatusColor_Failed(t *testing.T) {
	tuitest.ForceColor(t)
	td := resource.FindResourceType("rds")
	if td == nil {
		t.Fatal("rds resource type not found")
	}
	r := rdsColorResource("failed")
	style := styles.ColorStyle(td.Color(r))
	rendered := style.Render("test-failed")
	if styles.NoColorActive() {
		t.Skip("NO_COLOR is set, skipping color assertion")
	}
	if rendered == "test-failed" {
		t.Error("ColorStyle(rds.Color({failed})) should apply red color styling")
	}
}

func TestQA_RDS_StatusColor_AvailableAndStoppedDifferent(t *testing.T) {
	tuitest.ForceColor(t)
	td := resource.FindResourceType("rds")
	if td == nil {
		t.Fatal("rds resource type not found")
	}

	availStyle := styles.ColorStyle(td.Color(rdsColorResource("available")))
	stopStyle := styles.ColorStyle(td.Color(rdsColorResource("stopped")))

	if availStyle.Render("X") == stopStyle.Render("X") {
		t.Error("available and stopped should have different coloring")
	}
}

func TestQA_RDS_StatusColor_AvailableAndCreatingDifferent(t *testing.T) {
	tuitest.ForceColor(t)
	td := resource.FindResourceType("rds")
	if td == nil {
		t.Fatal("rds resource type not found")
	}

	availStyle := styles.ColorStyle(td.Color(rdsColorResource("available")))
	createStyle := styles.ColorStyle(td.Color(rdsColorResource("creating")))

	if availStyle.Render("X") == createStyle.Render("X") {
		t.Error("available and creating should have different coloring")
	}
}

// ===========================================================================
// A.9 Keyboard Navigation
// ===========================================================================

func TestQA_RDS_Navigation_EnterOpensChildView(t *testing.T) {
	m := rdsLoadedModel(t)

	// Press Enter — dbi now has a child view (dbi_events), so Enter should
	// produce EnterChildViewMsg instead of NavigateMsg.
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on RDS list should return a command")
	}

	msg := cmd()
	child, ok := msg.(messages.EnterChildView)
	if !ok {
		t.Fatalf("Enter should produce EnterChildViewMsg, got %T", msg)
	}
	if child.ChildType != "dbi_events" {
		t.Errorf("ChildType: expected %q, got %q", "dbi_events", child.ChildType)
	}
	if child.DisplayName == "" {
		t.Error("DisplayName should not be empty")
	}
	if child.ParentContext == nil {
		t.Error("ParentContext should not be nil")
	}
	if child.ParentContext["db_identifier"] == "" {
		t.Error("ParentContext should include db_identifier")
	}
}

func TestQA_RDS_Navigation_DOpensDetail(t *testing.T) {
	m := rdsLoadedModel(t)

	_, cmd := m.Update(rdsKeyPress("d"))
	if cmd == nil {
		t.Fatal("'d' on RDS list should return a command")
	}

	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("'d' should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("'d' should navigate to Detail, got target %d", nav.Target)
	}
}

func TestQA_RDS_Navigation_YOpensYAML(t *testing.T) {
	m := rdsLoadedModel(t)

	_, cmd := m.Update(rdsKeyPress("y"))
	if cmd == nil {
		t.Fatal("'y' on RDS list should return a command")
	}

	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("'y' should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetYAML {
		t.Errorf("'y' should navigate to YAML, got target %d", nav.Target)
	}
}

// ===========================================================================
// B. RDS Detail View
// ===========================================================================

// renderRDSDetail builds a Controller (via the package-unit blessed
// newDetailControllerUnit helper) for res/"dbi" with the given *ViewsConfig
// (nil uses the controller's zero-value/default), and renders it via the
// live NewTransientDetail+RenderDetail seam — the replacement for the retired
// views.NewDetail(...).SetSize(...).View() chain (DetailModel.View is dead;
// see specs/022-codebase-cleanup/wave3-map-detail.md). ANSI-stripped: every
// caller does textual (Contains) assertions, not raw-style comparisons.
func renderRDSDetail(t *testing.T, res resource.Resource, viewCfg *config.ViewsConfig) string {
	t.Helper()
	c := newDetailControllerUnit(t, res, "dbi")
	if viewCfg != nil {
		c.SetViewConfig(viewCfg)
	}
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(30))
	m := views.NewTransientDetail(120, 30, vp)
	return stripANSI(m.RenderDetail(*body))
}

func TestQA_RDS_Detail_ContainsAllFields(t *testing.T) {
	res := fixtureRDSInstances()[0]
	out := renderRDSDetail(t, res, nil)
	if out == "Initializing..." || out == "" {
		t.Fatal("Detail view should not be empty or initializing after SetSize")
	}

	// Fields map keys should appear in the detail.
	for fieldKey, fieldVal := range res.Fields {
		if !strings.Contains(out, fieldKey) {
			t.Errorf("Detail missing field key %q", fieldKey)
		}
		if fieldVal != "" && !strings.Contains(out, fieldVal) {
			// Value might be truncated, check first 20 chars.
			prefix := fieldVal
			if len(prefix) > 20 {
				prefix = fieldVal[:20]
			}
			if !strings.Contains(out, prefix) {
				t.Errorf("Detail missing field value %q (prefix %q)", fieldVal, prefix)
			}
		}
	}
}

// TestQA_RDS_Detail_FrameTitle is the live-seam replacement for the retired
// views.NewDetail(...).FrameTitle() call — drives Snapshot().FrameTitle
// instead (detailFrameTitleLocked mirrors the legacy Name-else-ID semantics).
func TestQA_RDS_Detail_FrameTitle(t *testing.T) {
	res := fixtureRDSInstances()[0]
	c := newDetailControllerUnit(t, res, "dbi")

	title := c.Snapshot().FrameTitle
	// This pins that the snapshot's title is the one builder's output, not a
	// second Name-else-ID answer of the controller's own; what the builder
	// produces is pinned in tui6_detail_frame_title_test.go.
	expected := resource.DetailFrameTitle(res.ID, res.Name, resource.DetailTitleOmitsID("dbi"))
	if title != expected {
		t.Errorf("Detail FrameTitle: expected %q, got %q", expected, title)
	}
}

func TestQA_RDS_Detail_EndpointField(t *testing.T) {
	res := fixtureRDSInstances()[0]
	out := renderRDSDetail(t, res, nil)
	endpointAddr := res.Fields["endpoint"]
	if !strings.Contains(out, endpointAddr[:20]) {
		t.Errorf("Detail should show endpoint address %q", endpointAddr)
	}
}

func TestQA_RDS_Detail_WithRawStruct_AllDetailPaths(t *testing.T) {
	// Use an RDS-only config to avoid non-deterministic map iteration
	// over all resource ViewDefs.
	rdsViewDef := config.DefaultViewDef("dbi")
	viewCfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"dbi": rdsViewDef,
		},
	}

	dbIdentifier := "prod-db-01"
	engine := "mysql"
	engineVersion := "8.0.35"
	status := "available"
	class := "db.r5.large"
	storageType := "gp3"
	az := "us-east-1a"

	rawStruct := rdstypes.DBInstance{
		DBInstanceIdentifier: &dbIdentifier,
		Engine:               &engine,
		EngineVersion:        &engineVersion,
		DBInstanceStatus:     &status,
		DBInstanceClass:      &class,
		MultiAZ:              new(true),
		AllocatedStorage:     new(int32(100)),
		StorageType:          &storageType,
		AvailabilityZone:     &az,
		Endpoint: &rdstypes.Endpoint{
			Address: new("prod-db-01.abc123.us-east-1.rds.amazonaws.com"),
			Port:    new(int32(3306)),
		},
	}

	res := resource.Resource{
		ID:        "prod-db-01",
		Name:      "prod-db-01",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}

	plain := renderRDSDetail(t, res, viewCfg)

	// Verify all detail fields from config are present.
	expectedValues := []string{
		"prod-db-01",
		"mysql",
		"8.0.35",
		"available",
		"db.r5.large",
		"gp3",
		"us-east-1a",
	}
	for _, val := range expectedValues {
		if !strings.Contains(plain, val) {
			t.Errorf("Detail with RawStruct missing value %q", val)
		}
	}

	// Endpoint should show as nested with Address and Port.
	if !strings.Contains(plain, "prod-db-01.abc123.us-east-1.rds.amazonaws.com") {
		t.Error("Detail should show Endpoint.Address")
	}
	if !strings.Contains(plain, "3306") {
		t.Error("Detail should show Endpoint Port 3306")
	}
}

func TestQA_RDS_Detail_CreatingInstanceNoEndpoint(t *testing.T) {
	viewCfg := config.DefaultConfig()

	dbIdentifier := "creating-db"
	status := "creating"

	rawStruct := rdstypes.DBInstance{
		DBInstanceIdentifier: &dbIdentifier,
		DBInstanceStatus:     &status,
		// Endpoint is nil during creation.
	}

	res := resource.Resource{
		ID:        "creating-db",
		Name:      "creating-db",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}

	// Should not panic.
	plain := renderRDSDetail(t, res, viewCfg)

	if strings.Contains(plain, "<nil>") {
		t.Error("Detail for creating instance should not show '<nil>'")
	}
	if !strings.Contains(plain, "creating-db") {
		t.Error("Detail for creating instance should show 'creating-db'")
	}
}

// ===========================================================================
// C. RDS YAML View
// ===========================================================================

func TestQA_RDS_YAML_ContainsFieldKeys(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(120, 40)

	out := strings.Join(m.ContentLines(), "\n")
	if out == "Initializing..." || out == "" {
		t.Fatal("YAML view should not be empty or initializing after SetSize")
	}

	// Check that field keys from the resource appear in the YAML.
	for key := range res.Fields {
		if !strings.Contains(out, key) {
			t.Errorf("YAML view missing key %q", key)
		}
	}
}

func TestQA_RDS_YAML_ContainsFieldValues(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(120, 40)

	out := strings.Join(m.ContentLines(), "\n")
	for _, val := range res.Fields {
		if val == "" {
			continue
		}
		if !strings.Contains(out, val) {
			t.Errorf("YAML view missing value %q", val)
		}
	}
}

// TestQA_RDS_YAML_FrameTitle retired: YAMLModel.FrameTitle() is DEAD per
// specs/022-codebase-cleanup/wave3-map-text.md (no production caller), and
// title-string behavior is not resource-type-specific.

func TestQA_RDS_YAML_RawContentNonEmpty(t *testing.T) {
	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewYAMLWithCtrl(res, "", k, nil)

	raw := stripANSI(strings.Join(m.ContentLines(), "\n"))
	if raw == "" {
		t.Error("YAML RawContent should not be empty")
	}
}

func TestQA_RDS_YAML_WithRawStruct(t *testing.T) {
	k := keys.Default()

	dbIdentifier := "test-yaml-db"
	engine := "postgres"
	engineVersion := "14.9"
	status := "available"
	class := "db.r5.large"
	storageType := "gp3"
	az := "us-east-1a"

	rawStruct := rdstypes.DBInstance{
		DBInstanceIdentifier: &dbIdentifier,
		Engine:               &engine,
		EngineVersion:        &engineVersion,
		DBInstanceStatus:     &status,
		DBInstanceClass:      &class,
		MultiAZ:              new(true),
		AllocatedStorage:     new(int32(200)),
		StorageType:          &storageType,
		AvailabilityZone:     &az,
		Endpoint: &rdstypes.Endpoint{
			Address: new("test-yaml-db.abc123.us-east-1.rds.amazonaws.com"),
			Port:    new(int32(5432)),
		},
	}

	res := resource.Resource{
		ID:        "test-yaml-db",
		Name:      "test-yaml-db",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}

	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(120, 50)

	out := strings.Join(m.ContentLines(), "\n")
	plain := stripANSI(out)

	expectedValues := []string{
		"test-yaml-db",
		"postgres",
		"14.9",
		"available",
		"db.r5.large",
		"gp3",
		"us-east-1a",
		"5432",
	}
	for _, val := range expectedValues {
		if !strings.Contains(plain, val) {
			t.Errorf("YAML with RawStruct missing value %q", val)
		}
	}

	// Endpoint should be a nested object with Address and Port.
	if !strings.Contains(plain, "test-yaml-db.abc123.us-east-1.rds.amazonaws.com") {
		t.Error("YAML should contain Endpoint Address")
	}
}

func TestQA_RDS_YAML_CreatingInstanceNoEndpoint(t *testing.T) {
	k := keys.Default()

	dbIdentifier := "yaml-creating-db"
	status := "creating"

	rawStruct := rdstypes.DBInstance{
		DBInstanceIdentifier: &dbIdentifier,
		DBInstanceStatus:     &status,
	}

	res := resource.Resource{
		ID:        "yaml-creating-db",
		Name:      "yaml-creating-db",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}

	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(120, 40)

	// Should not panic.
	out := strings.Join(m.ContentLines(), "\n")
	plain := stripANSI(out)

	if strings.Contains(plain, "<nil>") {
		t.Error("YAML for creating instance should not show '<nil>'")
	}
}

func TestQA_RDS_YAML_SyntaxColoring(t *testing.T) {
	tuitest.ForceColor(t)

	k := keys.Default()
	res := fixtureRDSInstances()[0]
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(120, 40)

	out := strings.Join(m.ContentLines(), "\n")

	// The raw view should contain ANSI sequences (color codes).
	if out == stripANSI(out) {
		t.Error("YAML view should have ANSI color codes when NO_COLOR is not set")
	}
}

// ===========================================================================
// D. Cross-View Interactions (integrated with root model)
// ===========================================================================

func TestQA_RDS_CrossView_ListToDetailAndBack(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS list.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})

	// Load RDS data.
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(), Provenance: messages.FetchProvenanceCanonicalList,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbi(2)") {
		t.Errorf("expected frame title 'rds(2)', got: %s", plain[:min(200, len(plain))])
	}

	// Press Enter to go to detail.
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-docdb-1") {
		t.Errorf("expected detail view for test-docdb-1, got: %s", plain[:min(200, len(plain))])
	}

	// Press Esc to go back to list.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbi") {
		t.Errorf("expected RDS list after Esc, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_RDS_CrossView_ListToYAMLAndBack(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS list.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})

	// Press 'y' to go to YAML.
	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Errorf("expected YAML view, got: %s", plain[:min(200, len(plain))])
	}

	// Press Esc to go back to list.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "dbi") {
		t.Errorf("expected RDS list after Esc from YAML, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_RDS_CrossView_CommandModeNavigation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS via command mode.
	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, r := range "dbi" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("command ':dbi' should return a command")
	}
}

func TestQA_RDS_CrossView_FilterHeaderDisplay(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS list.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})

	// Press '/' to enter filter mode.
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "dbc" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/dbc") {
		t.Errorf("header should show '/dbc' during filter, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_RDS_CrossView_EscFromListReturnsToMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to RDS.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})

	// Press Esc.
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "resource-types") {
		t.Errorf("Esc from RDS list should return to main menu, got: %s", plain[:min(200, len(plain))])
	}
}

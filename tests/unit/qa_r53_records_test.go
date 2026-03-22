package unit

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	demo "github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"

	"gopkg.in/yaml.v3"
)

// ===========================================================================
// R53 Records TUI Drill-Down Test Helpers
// ===========================================================================

// r53ZoneTypeDef returns the R53 hosted zone type definition.
func r53ZoneTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "Route 53 Hosted Zones",
		ShortName: "r53",
		Aliases:   []string{"r53", "route53", "dns", "hosted-zones"},
		Columns: []resource.Column{
			{Key: "zone_id", Title: "Zone ID", Width: 30, Sortable: true},
			{Key: "name", Title: "Name", Width: 36, Sortable: true},
			{Key: "record_count", Title: "Records", Width: 9, Sortable: true},
			{Key: "private_zone", Title: "Private", Width: 9, Sortable: true},
			{Key: "comment", Title: "Comment", Width: 30, Sortable: false},
		},
	}
}

// fixtureR53Zones returns R53 hosted zone fixtures for testing.
func fixtureR53Zones() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "/hostedzone/Z0123456789ABCDEFGHIJ",
			Name:   "acme-corp.com.",
			Status: "",
			Fields: map[string]string{
				"zone_id":      "/hostedzone/Z0123456789ABCDEFGHIJ",
				"name":         "acme-corp.com.",
				"record_count": "42",
				"private_zone": "false",
				"comment":      "Primary public domain for Acme Corp",
			},
		},
		{
			ID:     "/hostedzone/Z1234567890ABCDEFGHIJ",
			Name:   "internal.acme-corp.com.",
			Status: "",
			Fields: map[string]string{
				"zone_id":      "/hostedzone/Z1234567890ABCDEFGHIJ",
				"name":         "internal.acme-corp.com.",
				"record_count": "18",
				"private_zone": "true",
				"comment":      "Private zone for internal service discovery",
			},
		},
		{
			ID:     "/hostedzone/Z2345678901ABCDEFGHIJ",
			Name:   "staging.acme-corp.com.",
			Status: "",
			Fields: map[string]string{
				"zone_id":      "/hostedzone/Z2345678901ABCDEFGHIJ",
				"name":         "staging.acme-corp.com.",
				"record_count": "8",
				"private_zone": "false",
				"comment":      "Staging environment DNS",
			},
		},
	}
}

// fixtureR53Records returns R53 DNS record fixtures for testing.
func fixtureR53Records() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-corp.com.|A",
			Name:   "acme-corp.com.",
			Status: "A",
			Fields: map[string]string{
				"name":   "acme-corp.com.",
				"type":   "A",
				"ttl":    "",
				"values": "ALIAS: d111111abcdef8.cloudfront.net.",
			},
		},
		{
			ID:     "acme-corp.com.|NS",
			Name:   "acme-corp.com.",
			Status: "NS",
			Fields: map[string]string{
				"name":   "acme-corp.com.",
				"type":   "NS",
				"ttl":    "172800",
				"values": "ns-111.awsdns-11.com., ns-222.awsdns-22.net.",
			},
		},
		{
			ID:     "api.acme-corp.com.|CNAME",
			Name:   "api.acme-corp.com.",
			Status: "CNAME",
			Fields: map[string]string{
				"name":   "api.acme-corp.com.",
				"type":   "CNAME",
				"ttl":    "300",
				"values": "api-prod.elb.amazonaws.com.",
			},
		},
		{
			ID:     "mail.acme-corp.com.|MX",
			Name:   "mail.acme-corp.com.",
			Status: "MX",
			Fields: map[string]string{
				"name":   "mail.acme-corp.com.",
				"type":   "MX",
				"ttl":    "300",
				"values": "10 inbound-smtp.us-east-1.amazonaws.com.",
			},
		},
	}
}

// r53LoadedZoneModel creates a root TUI model navigated to R53 zones with data loaded.
func r53LoadedZoneModel() tui.Model {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "r53",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "r53",
		Resources:    fixtureR53Zones(),
	})
	return m
}

// r53LoadedRecordModel creates a root TUI model navigated to R53 -> zone -> records loaded.
func r53LoadedRecordModel() tui.Model {
	m := r53LoadedZoneModel()
	// Press Enter to drill into first zone
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	// Load records
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "r53_records",
		Resources:    fixtureR53Records(),
	})
	return m
}

// r53RLZoneModel creates a standalone ResourceListModel for R53 zones with data loaded.
func r53RLZoneModel() views.ResourceListModel {
	td := r53ZoneTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53",
		Resources:    fixtureR53Zones(),
	})
	return m
}

// r53RLRecordModel creates a standalone ResourceListModel for R53 records inside a zone.
func r53RLRecordModel(zoneId, zoneName string) views.ResourceListModel {
	k := keys.Default()
	m := views.NewR53RecordsList(zoneId, zoneName, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53_records",
		Resources:    fixtureR53Records(),
	})
	return m
}

// r53KeyPress creates a tea.KeyPressMsg for a printable character.
func r53KeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// ===========================================================================
// GAP 1: R53 Records TUI Drill-Down (mirrors S3 drill-down pattern)
// ===========================================================================

// A. R53 Zone List View

// A.1 Loading State

func TestQA_R53_A1_1_ZoneList_LoadingShowsSpinner(t *testing.T) {
	td := r53ZoneTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	out := m.View()
	if !strings.Contains(out, "Loading") {
		t.Errorf("expected spinner text 'Loading' in zone list loading state, got: %q", out)
	}
}

func TestQA_R53_A1_2_ZoneList_FrameTitleDuringLoading(t *testing.T) {
	td := r53ZoneTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	title := m.FrameTitle()
	if title != "r53" {
		t.Errorf("during loading, FrameTitle should be 'r53' (no count), got: %q", title)
	}
}

func TestQA_R53_A1_3_ZoneList_AfterLoad_FrameTitleShowsCount(t *testing.T) {
	m := r53RLZoneModel()
	title := m.FrameTitle()
	if title != "r53(3)" {
		t.Errorf("after loading 3 zones, FrameTitle should be 'r53(3)', got: %q", title)
	}
}

// A.2 Empty State

func TestQA_R53_A2_1_ZoneList_EmptyState(t *testing.T) {
	td := r53ZoneTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53",
		Resources:    []resource.Resource{},
	})

	out := m.View()
	if !strings.Contains(out, "No resources found") {
		t.Errorf("expected empty state message, got: %q", out)
	}

	title := m.FrameTitle()
	if title != "r53(0)" {
		t.Errorf("empty zone list frame title should be 'r53(0)', got: %q", title)
	}
}

// A.3 Column Layout

func TestQA_R53_A3_1_ZoneList_ColumnHeaders(t *testing.T) {
	td := r53ZoneTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(200, 20) // wide enough to show all columns
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53",
		Resources:    fixtureR53Zones(),
	})
	out := m.View()

	for _, col := range []string{"Zone ID", "Name", "Records", "Private", "Comment"} {
		if !strings.Contains(out, col) {
			t.Errorf("zone list should have %q column header", col)
		}
	}
}

func TestQA_R53_A3_2_ZoneList_ShowsZoneData(t *testing.T) {
	m := r53RLZoneModel()
	out := m.View()

	for _, zone := range fixtureR53Zones() {
		if !strings.Contains(out, zone.Fields["name"]) {
			t.Errorf("expected zone name %q in the list view", zone.Fields["name"])
		}
	}
}

// A.4 Enter Key (Drill Into Zone)

func TestQA_R53_A4_1_EnterOnZone_SendsR53EnterZoneMsg(t *testing.T) {
	m := r53RLZoneModel()

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on R53 zone should produce a command")
	}

	msg := cmd()
	zoneMsg, ok := msg.(messages.R53EnterZoneMsg)
	if !ok {
		t.Fatalf("Enter on R53 zone should produce R53EnterZoneMsg, got %T", msg)
	}

	expected := fixtureR53Zones()[0]
	if zoneMsg.ZoneId != expected.ID {
		t.Errorf("R53EnterZoneMsg.ZoneId should be %q, got %q", expected.ID, zoneMsg.ZoneId)
	}
	if zoneMsg.ZoneName != expected.Name {
		t.Errorf("R53EnterZoneMsg.ZoneName should be %q, got %q", expected.Name, zoneMsg.ZoneName)
	}
}

func TestQA_R53_A4_2_EnterOnSecondZone_SendsCorrectZoneMsg(t *testing.T) {
	m := r53RLZoneModel()

	// Move cursor down to the second zone
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on second zone should produce a command")
	}

	msg := cmd()
	zoneMsg, ok := msg.(messages.R53EnterZoneMsg)
	if !ok {
		t.Fatalf("Enter on second zone should produce R53EnterZoneMsg, got %T", msg)
	}

	expected := fixtureR53Zones()[1]
	if zoneMsg.ZoneId != expected.ID {
		t.Errorf("R53EnterZoneMsg.ZoneId should be %q, got %q", expected.ID, zoneMsg.ZoneId)
	}
	if zoneMsg.ZoneName != expected.Name {
		t.Errorf("R53EnterZoneMsg.ZoneName should be %q, got %q", expected.Name, zoneMsg.ZoneName)
	}
}

// ===========================================================================
// B. R53 Records List View (after drill-down)
// ===========================================================================

// B.1 Loading State

func TestQA_R53_B1_1_RecordList_LoadingShowsSpinner(t *testing.T) {
	k := keys.Default()
	m := views.NewR53RecordsList("/hostedzone/ZTEST", "example.com.", nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	out := m.View()
	if !strings.Contains(out, "Loading") {
		t.Errorf("expected spinner 'Loading' text in record list loading state, got: %q", out)
	}
}

func TestQA_R53_B1_2_RecordList_AfterLoad_ShowsData(t *testing.T) {
	m := r53RLRecordModel("/hostedzone/ZTEST", "acme-corp.com.")
	out := m.View()

	if !strings.Contains(out, "acme-corp.com.") {
		t.Error("record list should show record name 'acme-corp.com.'")
	}
}

// B.2 Empty State

func TestQA_R53_B2_1_RecordList_EmptyZone(t *testing.T) {
	k := keys.Default()
	m := views.NewR53RecordsList("/hostedzone/ZEMPTY", "empty-zone.com.", nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53_records",
		Resources:    []resource.Resource{},
	})

	out := m.View()
	if !strings.Contains(out, "No resources found") {
		t.Errorf("empty record list should show 'No resources found', got: %q", out)
	}

	title := m.FrameTitle()
	if !strings.Contains(title, "empty-zone.com.(0)") {
		t.Errorf("empty zone frame title should contain 'empty-zone.com.(0)', got: %q", title)
	}
}

// B.3 Column Layout

func TestQA_R53_B3_1_RecordList_ColumnHeaders(t *testing.T) {
	m := r53RLRecordModel("/hostedzone/ZTEST", "acme-corp.com.")
	out := m.View()

	for _, col := range []string{"Name", "Type", "TTL", "Values"} {
		if !strings.Contains(out, col) {
			t.Errorf("record list should have %q column header", col)
		}
	}
}

func TestQA_R53_B3_2_RecordList_ShowsRecordData(t *testing.T) {
	m := r53RLRecordModel("/hostedzone/ZTEST", "acme-corp.com.")
	out := m.View()

	records := fixtureR53Records()
	for _, rec := range records {
		if !strings.Contains(out, rec.Fields["type"]) {
			t.Errorf("record list should show record type %q", rec.Fields["type"])
		}
	}
}

// B.4 Frame Title (uses zone name, not "r53_records")

func TestQA_R53_B4_1_RecordList_FrameTitleShowsZoneName(t *testing.T) {
	m := r53RLRecordModel("/hostedzone/ZTEST", "acme-corp.com.")
	title := m.FrameTitle()
	if !strings.Contains(title, "acme-corp.com.") {
		t.Errorf("frame title should contain zone name 'acme-corp.com.', got: %q", title)
	}
}

func TestQA_R53_B4_2_RecordList_FrameTitleShowsCount(t *testing.T) {
	m := r53RLRecordModel("/hostedzone/ZTEST", "acme-corp.com.")
	title := m.FrameTitle()
	expected := "acme-corp.com.(4)"
	if title != expected {
		t.Errorf("frame title should be %q, got: %q", expected, title)
	}
}

func TestQA_R53_B4_3_RecordList_ZoneId_Accessor(t *testing.T) {
	m := r53RLRecordModel("/hostedzone/ZTEST", "acme-corp.com.")
	if m.R53ZoneId() != "/hostedzone/ZTEST" {
		t.Errorf("R53ZoneId() should return %q, got %q", "/hostedzone/ZTEST", m.R53ZoneId())
	}
}

// B.5 Enter Key on Record (opens detail view)

func TestQA_R53_B5_1_EnterOnRecord_SendsDetailNavigateMsg(t *testing.T) {
	m := r53RLRecordModel("/hostedzone/ZTEST", "acme-corp.com.")

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on R53 record should produce a command")
	}

	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("Enter on R53 record should produce NavigateMsg (for detail), got %T", msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("Enter on record should navigate to TargetDetail, got: %d", nav.Target)
	}
	if nav.Resource == nil {
		t.Fatal("NavigateMsg.Resource should not be nil")
	}
	if nav.Resource.ID != fixtureR53Records()[0].ID {
		t.Errorf("NavigateMsg.Resource.ID should be %q, got %q", fixtureR53Records()[0].ID, nav.Resource.ID)
	}
}

// B.6 YAML view from record list

func TestQA_R53_B6_1_YKeyOnRecord_SendsYAMLNavigateMsg(t *testing.T) {
	m := r53RLRecordModel("/hostedzone/ZTEST", "acme-corp.com.")

	_, cmd := m.Update(r53KeyPress("y"))
	if cmd == nil {
		t.Fatal("y key on R53 record should produce a command for YAML view")
	}

	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("y key should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetYAML {
		t.Errorf("y key should target YAML view, got: %d", nav.Target)
	}
	if nav.Resource == nil {
		t.Fatal("YAML NavigateMsg.Resource should not be nil")
	}
}

// ===========================================================================
// C. R53EnterZoneMsg Root Model Integration (mirrors S3EnterBucketMsg tests)
// ===========================================================================

func TestQA_R53_C1_EnterZoneMsg_CreatesRecordListView(t *testing.T) {
	tui.Version = "0.6.0"
	m := r53LoadedZoneModel()

	// Simulate R53EnterZoneMsg directly
	m, _ = rootApplyMsg(m, messages.R53EnterZoneMsg{
		ZoneId:   "/hostedzone/Z0123456789ABCDEFGHIJ",
		ZoneName: "acme-corp.com.",
	})

	plain := stripANSI(rootViewContent(m))
	// Should show loading state for the zone or the zone name in the frame
	if !strings.Contains(plain, "acme-corp.com.") {
		t.Errorf("R53EnterZoneMsg should create record list view with zone name, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_R53_C2_EnterZoneMsg_RecordsLoaded_ShowsRecords(t *testing.T) {
	tui.Version = "0.6.0"
	m := r53LoadedZoneModel()

	// Send R53EnterZoneMsg
	m, _ = rootApplyMsg(m, messages.R53EnterZoneMsg{
		ZoneId:   "/hostedzone/Z0123456789ABCDEFGHIJ",
		ZoneName: "acme-corp.com.",
	})

	// Load records
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "r53_records",
		Resources:    fixtureR53Records(),
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "acme-corp.com.") {
		t.Errorf("record list should show zone name in frame, got: %s", plain[:min(300, len(plain))])
	}
	// Verify at least one record type appears
	if !strings.Contains(plain, "NS") {
		t.Errorf("record list should show record type 'NS', got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_R53_C3_EnterZoneMsg_EscapeBackToZoneList(t *testing.T) {
	tui.Version = "0.6.0"
	m := r53LoadedRecordModel()

	// Escape from record list -> back to zone list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "r53(3)") {
		t.Errorf("after escape from records, should be at zone list r53(3), got: %s", plain[:min(300, len(plain))])
	}
}

// ===========================================================================
// D. Full Navigation Stack (mirrors S3 D2_1_FullFlowStack)
// ===========================================================================

func TestQA_R53_D1_FullFlowStack(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// 1. Verify we start at main menu
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Fatalf("should start at main menu, got: %s", plain[:min(200, len(plain))])
	}

	// 2. Navigate to R53 zone list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "r53",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "r53",
		Resources:    fixtureR53Zones(),
	})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "r53(3)") {
		t.Fatalf("should be at R53 zone list with r53(3), got: %s", plain[:min(200, len(plain))])
	}

	// 3. Enter zone -> record list
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "r53_records",
		Resources:    fixtureR53Records(),
	})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "acme-corp.com.") {
		t.Fatalf("should be at record list for acme-corp.com., got: %s", plain[:min(300, len(plain))])
	}

	// 4. Enter record -> detail view
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "acme-corp.com.") {
		t.Fatalf("should be at detail view for first record, got: %s", plain[:min(300, len(plain))])
	}

	// 5. Escape from detail -> back to record list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "acme-corp.com.") {
		t.Errorf("after escape from detail, should be at record list, got: %s", plain[:min(300, len(plain))])
	}

	// 6. Escape from record list -> back to zone list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "r53(3)") {
		t.Errorf("after escape from records, should be at zone list r53(3), got: %s", plain[:min(300, len(plain))])
	}

	// 7. Escape from zone list -> back to main menu
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after escape from zone list, should be at main menu, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_R53_D2_FilterMode_ViaRootModel(t *testing.T) {
	tui.Version = "0.6.0"
	m := r53LoadedZoneModel()

	// Enter filter mode with "/"
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	// Type "internal"
	for _, ch := range "internal" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	plain := stripANSI(rootViewContent(m))
	// Header should show filter text
	if !strings.Contains(plain, "/internal") {
		t.Errorf("header should show active filter '/internal', got: %s", plain[:min(200, len(plain))])
	}
	// Frame title should show filtered count
	if !strings.Contains(plain, "1/3") {
		t.Errorf("frame title should show 1/3 for 'internal' filter, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_R53_D3_FilterMode_EscapeClearsFilter(t *testing.T) {
	tui.Version = "0.6.0"
	m := r53LoadedZoneModel()

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, ch := range "internal" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	// Escape from filter mode
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	// Should show all 3 zones again
	if !strings.Contains(plain, "r53(3)") {
		t.Errorf("escape from filter should restore all zones, got: %s", plain[:min(200, len(plain))])
	}
}

// ===========================================================================
// E. Demo Mode R53 Drill-Down
// ===========================================================================

func TestQA_R53_E1_DemoMode_FetchR53Records(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))

	var m tea.Model = model
	m, _ = m.Update(messages.ClientsReadyMsg{})

	// Navigate to R53 zone list
	var cmd tea.Cmd
	_, cmd = m.Update(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "r53",
	})
	if cmd == nil {
		t.Fatal("NavigateMsg for R53 returned nil cmd; expected a fetch command")
	}

	// Execute and find ResourcesLoadedMsg
	msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	rlm, ok := msg.(messages.ResourcesLoadedMsg)
	if !ok {
		t.Fatalf("expected ResourcesLoadedMsg; got %T", msg)
	}
	if len(rlm.Resources) == 0 {
		t.Error("ResourcesLoadedMsg.Resources is empty; expected demo R53 zone fixtures")
	}
}

func TestQA_R53_E2_DemoMode_FetchR53RecordsDrillDown(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))

	var m tea.Model = model
	m, _ = m.Update(messages.ClientsReadyMsg{})

	// Navigate to R53 zone list
	m, cmd := m.Update(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "r53",
	})

	// Load zones
	msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	m, _ = m.Update(msg)

	// Drill into first zone via R53EnterZoneMsg
	_, cmd = m.Update(messages.R53EnterZoneMsg{
		ZoneId:   "/hostedzone/Z0123456789ABCDEFGHIJ",
		ZoneName: "acme-corp.com.",
	})
	if cmd == nil {
		t.Fatal("R53EnterZoneMsg returned nil cmd; expected a fetch command")
	}

	// The cmd should produce a ResourcesLoadedMsg with R53 record fixtures
	msg2 := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	rlm2, ok := msg2.(messages.ResourcesLoadedMsg)
	if !ok {
		t.Fatalf("expected ResourcesLoadedMsg for records; got %T", msg2)
	}
	if len(rlm2.Resources) == 0 {
		t.Error("R53 record fixtures should not be empty for acme-corp.com zone")
	}
}

func TestQA_R53_E3_DemoMode_UnknownZone_EmptyRecords(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))

	var m tea.Model = model
	m, _ = m.Update(messages.ClientsReadyMsg{})

	// Navigate to R53 zones first
	m, cmd := m.Update(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "r53",
	})
	msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	m, _ = m.Update(msg)

	// Drill into unknown zone
	_, cmd = m.Update(messages.R53EnterZoneMsg{
		ZoneId:   "/hostedzone/ZUNKNOWN",
		ZoneName: "unknown.com.",
	})
	if cmd == nil {
		t.Fatal("R53EnterZoneMsg for unknown zone returned nil cmd")
	}

	msg2 := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	rlm, ok := msg2.(messages.ResourcesLoadedMsg)
	if !ok {
		t.Fatalf("expected ResourcesLoadedMsg; got %T", msg2)
	}
	// Unknown zone should return nil/empty resources
	if len(rlm.Resources) != 0 {
		t.Errorf("unknown zone should return 0 resources, got %d", len(rlm.Resources))
	}
}

// GAP 2 config tests are in qa_r53_records_config_test.go (package unit_test)
// because they use testdataPath which is defined in the unit_test package.

// ===========================================================================
// GAP 3: Demo R53 Record Fixture Quality
// ===========================================================================

// TestDemoR53Zones_RawStruct verifies every R53 zone fixture has valid RawStruct.
func TestDemoR53Zones_RawStruct(t *testing.T) {
	resources, ok := demo.GetResources("r53")
	if !ok {
		t.Fatal("GetResources(\"r53\") returned ok=false")
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil; R53 zone fixtures must populate RawStruct", i, r.ID)
			continue
		}

		zone, ok := r.RawStruct.(r53types.HostedZone)
		if !ok {
			t.Errorf("resource[%d] (%s): RawStruct is %T, want r53types.HostedZone", i, r.ID, r.RawStruct)
			continue
		}

		// Id must match resource ID
		if zone.Id == nil || *zone.Id != r.ID {
			t.Errorf("resource[%d] (%s): HostedZone.Id = %v, want %q", i, r.ID, zone.Id, r.ID)
		}

		// Name must match resource Name
		if zone.Name == nil || *zone.Name != r.Name {
			t.Errorf("resource[%d] (%s): HostedZone.Name = %v, want %q", i, r.ID, zone.Name, r.Name)
		}

		// Config must be set
		if zone.Config == nil {
			t.Errorf("resource[%d] (%s): HostedZone.Config is nil", i, r.ID)
		}
	}
}

// TestDemoR53Zones_RawStruct_YAML verifies R53 zone RawStruct marshals to non-empty YAML.
func TestDemoR53Zones_RawStruct_YAML(t *testing.T) {
	resources, ok := demo.GetResources("r53")
	if !ok {
		t.Fatal("GetResources(\"r53\") returned ok=false")
	}

	for i, r := range resources {
		if r.RawStruct == nil {
			t.Errorf("resource[%d] (%s): RawStruct is nil", i, r.ID)
			continue
		}
		safe := fieldpath.ToSafeValue(reflect.ValueOf(r.RawStruct))
		out, err := yaml.Marshal(safe)
		if err != nil {
			t.Errorf("resource[%d] (%s): yaml.Marshal(RawStruct) failed: %v", i, r.ID, err)
			continue
		}
		if len(out) == 0 {
			t.Errorf("resource[%d] (%s): yaml.Marshal(RawStruct) produced empty output", i, r.ID)
		}
	}
}

// TestDemoR53Records_GetR53Records verifies that GetR53Records works for all registered zones.
func TestDemoR53Records_GetR53Records(t *testing.T) {
	zones, ok := demo.GetResources("r53")
	if !ok {
		t.Fatal("GetResources(\"r53\") returned ok=false")
	}

	for _, zone := range zones {
		t.Run(zone.Name, func(t *testing.T) {
			records, ok := demo.GetR53Records(zone.ID)
			if !ok {
				t.Fatalf("GetR53Records(%q) returned ok=false; expected demo record fixtures", zone.ID)
			}
			if len(records) == 0 {
				t.Fatalf("GetR53Records(%q) returned empty slice; expected non-empty", zone.ID)
			}

			// Every record must have non-nil RawStruct (r53types.ResourceRecordSet)
			for i, r := range records {
				if r.RawStruct == nil {
					t.Errorf("record[%d] (%s): RawStruct is nil", i, r.ID)
					continue
				}
				_, ok := r.RawStruct.(r53types.ResourceRecordSet)
				if !ok {
					t.Errorf("record[%d] (%s): RawStruct is %T, want r53types.ResourceRecordSet", i, r.ID, r.RawStruct)
				}
			}

			// Every record must have required Fields
			requiredFields := []string{"name", "type", "ttl", "values"}
			for i, r := range records {
				for _, key := range requiredFields {
					if _, exists := r.Fields[key]; !exists {
						t.Errorf("record[%d] (%s): missing Fields key %q", i, r.ID, key)
					}
				}
			}
		})
	}
}

// TestDemoR53Records_RawStruct_YAML verifies R53 record RawStruct marshals to non-empty YAML.
func TestDemoR53Records_RawStruct_YAML(t *testing.T) {
	zones, ok := demo.GetResources("r53")
	if !ok {
		t.Fatal("GetResources(\"r53\") returned ok=false")
	}

	for _, zone := range zones {
		records, ok := demo.GetR53Records(zone.ID)
		if !ok {
			continue
		}
		for i, r := range records {
			if r.RawStruct == nil {
				t.Errorf("zone %s record[%d] (%s): RawStruct is nil", zone.Name, i, r.ID)
				continue
			}
			safe := fieldpath.ToSafeValue(reflect.ValueOf(r.RawStruct))
			out, err := yaml.Marshal(safe)
			if err != nil {
				t.Errorf("zone %s record[%d] (%s): yaml.Marshal(RawStruct) failed: %v", zone.Name, i, r.ID, err)
				continue
			}
			if len(out) == 0 {
				t.Errorf("zone %s record[%d] (%s): yaml.Marshal(RawStruct) produced empty output", zone.Name, i, r.ID)
			}
		}
	}
}

// TestDemoR53Records_UnknownZone verifies GetR53Records returns false for unknown zones.
func TestDemoR53Records_UnknownZone(t *testing.T) {
	_, ok := demo.GetR53Records("/hostedzone/ZUNKNOWN")
	if ok {
		t.Error("GetR53Records for unknown zone should return ok=false")
	}
}

// TestDemoR53Records_FieldQuality verifies fixture data has correct field values.
func TestDemoR53Records_FieldQuality(t *testing.T) {
	records, ok := demo.GetR53Records("/hostedzone/Z0123456789ABCDEFGHIJ")
	if !ok {
		t.Fatal("GetR53Records returned ok=false for acme-corp zone")
	}

	// Verify record IDs follow the "name|type" pattern
	for i, r := range records {
		if !strings.Contains(r.ID, "|") {
			t.Errorf("record[%d]: ID %q should contain '|' separator (name|type)", i, r.ID)
		}
	}

	// Verify at least one alias record exists (TTL empty, values starts with "ALIAS:")
	hasAlias := false
	for _, r := range records {
		if strings.HasPrefix(r.Fields["values"], "ALIAS:") {
			hasAlias = true
			if r.Fields["ttl"] != "" {
				t.Errorf("alias record %q should have empty TTL, got %q", r.ID, r.Fields["ttl"])
			}
		}
	}
	if !hasAlias {
		t.Error("expected at least one alias record in acme-corp zone fixtures")
	}

	// Verify multiple record types exist
	types := map[string]bool{}
	for _, r := range records {
		types[r.Fields["type"]] = true
	}
	if len(types) < 2 {
		t.Errorf("expected multiple record types in fixtures, got %d: %v", len(types), types)
	}
}

// TestDemoR53Records_StatusMatchesType verifies Status field equals the record type.
func TestDemoR53Records_StatusMatchesType(t *testing.T) {
	zones, ok := demo.GetResources("r53")
	if !ok {
		t.Fatal("GetResources(\"r53\") returned ok=false")
	}

	for _, zone := range zones {
		records, ok := demo.GetR53Records(zone.ID)
		if !ok {
			continue
		}
		for i, r := range records {
			if r.Status != r.Fields["type"] {
				t.Errorf("zone %s record[%d] (%s): Status=%q != Fields[\"type\"]=%q",
					zone.Name, i, r.ID, r.Status, r.Fields["type"])
			}
		}
	}
}

// TestDemoR53Records_RawStructFieldConsistency verifies RawStruct fields match Fields map.
func TestDemoR53Records_RawStructFieldConsistency(t *testing.T) {
	records, ok := demo.GetR53Records("/hostedzone/Z0123456789ABCDEFGHIJ")
	if !ok {
		t.Fatal("GetR53Records returned ok=false for acme-corp zone")
	}

	for i, r := range records {
		if r.RawStruct == nil {
			continue
		}
		raw, ok := r.RawStruct.(r53types.ResourceRecordSet)
		if !ok {
			continue
		}

		// Verify Name consistency
		if raw.Name != nil && *raw.Name != r.Fields["name"] {
			t.Errorf("record[%d] (%s): RawStruct.Name=%q != Fields[\"name\"]=%q",
				i, r.ID, *raw.Name, r.Fields["name"])
		}

		// Verify Type consistency
		if string(raw.Type) != r.Fields["type"] {
			t.Errorf("record[%d] (%s): RawStruct.Type=%q != Fields[\"type\"]=%q",
				i, r.ID, string(raw.Type), r.Fields["type"])
		}
	}
}

// ===========================================================================
// GAP 4: NewR53RecordsList constructor tests
// ===========================================================================

func TestNewR53RecordsList_TypeDef(t *testing.T) {
	k := keys.Default()
	m := views.NewR53RecordsList("/hostedzone/ZTEST", "test.com.", nil, k)
	m.SetSize(120, 20)

	title := m.FrameTitle()
	// During loading, should show zone name (not "r53_records")
	if title != "test.com." {
		t.Errorf("NewR53RecordsList FrameTitle during loading should be 'test.com.', got: %q", title)
	}
}

func TestNewR53RecordsList_R53ZoneId(t *testing.T) {
	k := keys.Default()
	m := views.NewR53RecordsList("/hostedzone/ZTEST123", "test.com.", nil, k)
	m.SetSize(120, 20)

	if m.R53ZoneId() != "/hostedzone/ZTEST123" {
		t.Errorf("R53ZoneId() should be %q, got %q", "/hostedzone/ZTEST123", m.R53ZoneId())
	}
}

func TestNewR53RecordsList_ColumnsMatch(t *testing.T) {
	k := keys.Default()
	m := views.NewR53RecordsList("/hostedzone/ZTEST", "test.com.", nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53_records",
		Resources:    fixtureR53Records(),
	})

	out := m.View()
	expectedCols := resource.R53RecordColumns()
	for _, col := range expectedCols {
		if !strings.Contains(out, col.Title) {
			t.Errorf("record list should have %q column header from R53RecordColumns()", col.Title)
		}
	}
}

func TestNewR53RecordsList_MultipleTerminalSizes(t *testing.T) {
	sizes := []struct {
		w, h int
	}{
		{40, 10},
		{80, 24},
		{200, 50},
	}

	for _, sz := range sizes {
		t.Run(fmt.Sprintf("w=%d_h=%d", sz.w, sz.h), func(t *testing.T) {
				k := keys.Default()
				m := views.NewR53RecordsList("/hostedzone/ZTEST", "test.com.", nil, k)
				m.SetSize(sz.w, sz.h)
				m, _ = m.Init()
				m, _ = m.Update(messages.ResourcesLoadedMsg{
					ResourceType: "r53_records",
					Resources:    fixtureR53Records(),
				})

				out := m.View()
				if out == "" {
					t.Errorf("View() should not be empty at size %dx%d", sz.w, sz.h)
				}
			},
		)
	}
}

// ===========================================================================
// R53 Records Render Test (mirrors demo_render_test.go pattern)
// ===========================================================================

func TestDemoRender_R53RecordListShowsData(t *testing.T) {
	cfg, _ := config.LoadFrom([]string{".a9s/views.yaml"})

	zones, ok := demo.GetResources("r53")
	if !ok {
		t.Fatal("no demo data for r53")
	}

	td := resource.FindResourceType("r53")
	if td == nil {
		t.Fatal("resource type r53 not found")
	}

	k := keys.Default()
	m := views.NewResourceList(*td, cfg, k)
	m.SetSize(120, 30)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53",
		Resources:    zones,
	})

	output := m.View()
	plain := stripANSI(output)

	expectIn := []string{"acme-corp.com.", "internal.acme-corp.com.", "staging.acme-corp.com."}
	for _, expect := range expectIn {
		if !strings.Contains(plain, expect) {
			t.Errorf("[r53] expected %q in rendered output, not found.\nOutput:\n%s",
				expect, plain)
		}
	}
}

func TestDemoRender_R53RecordsListShowsData(t *testing.T) {
	cfg, _ := config.LoadFrom([]string{".a9s/views.yaml"})

	records, ok := demo.GetR53Records("/hostedzone/Z0123456789ABCDEFGHIJ")
	if !ok {
		t.Fatal("no demo records for acme-corp zone")
	}

	k := keys.Default()
	m := views.NewR53RecordsList("/hostedzone/Z0123456789ABCDEFGHIJ", "acme-corp.com.", cfg, k)
	m.SetSize(120, 30)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53_records",
		Resources:    records,
	})

	output := m.View()
	plain := stripANSI(output)

	// Should show record types
	for _, rt := range []string{"NS", "SOA", "MX"} {
		if !strings.Contains(plain, rt) {
			t.Errorf("[r53_records] expected record type %q in rendered output, not found", rt)
		}
	}
}

package unit_test

// ctrl+z (ActionToggleAttention) shows exactly the rows where
// td.Color(r).IsIssue() is true, for every registered resource type — the same
// rule that drives the main-menu issue badge count.

import (
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// colorEC2 derives color from findings only, so a synthetic EC2 fixture testing
// Color/IsIssue carries the matching Wave-1 Finding.
func appCtrlZEc2StateFinding(state string) []domain.Finding {
	switch state {
	case "pending":
		return []domain.Finding{{Code: "ec2.state.pending", Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"}}
	case "stopping":
		return []domain.Finding{{Code: "ec2.state.stopping", Phrase: "stopping", Severity: domain.SevWarn, Source: "wave1"}}
	case "shutting-down":
		return []domain.Finding{{Code: "ec2.state.shutting-down", Phrase: "shutting down", Severity: domain.SevWarn, Source: "wave1"}}
	case "stopped":
		return []domain.Finding{{Code: "ec2.state.stopped", Phrase: "stopped", Severity: domain.SevWarn, Source: "wave1"}}
	case "terminated":
		return []domain.Finding{{Code: "ec2.state.terminated", Phrase: "terminated", Severity: domain.SevDim, Source: "wave1"}}
	default:
		return nil
	}
}

// appCtrlZModel builds a Controller navigated to shortName's list screen with
// resources already loaded.
func appCtrlZModel(t *testing.T, shortName string, resources []resource.Resource) *app.Controller {
	t.Helper()
	c := openListController(t, shortName)
	c.ApplyResourcesLoaded(shortName, resources, nil, false)
	return c
}

func appCtrlZToggle(c *app.Controller) {
	c.Apply(app.Action{Kind: app.ActionToggleAttention})
}

func appCtrlZVisibleIDs(c *app.Controller) map[string]bool {
	lb := *c.Snapshot().Body.List
	ids := make(map[string]bool, len(lb.Rows))
	for _, row := range lb.Rows {
		ids[row.ResourceID] = true
	}
	return ids
}

func appCtrlZRegisteredShortNames() []string {
	types := resource.AllResourceTypes()
	names := make([]string, 0, len(types))
	for _, td := range types {
		names = append(names, td.ShortName)
	}
	sort.Strings(names)
	return names
}

// appCtrlZRawFieldFixtures carries issue/healthy Fields for resource types
// whose Color func never reads r.Findings (colorSSM, colorSG, colorRTB,
// colorAlarm, colorTrail, colorCTEvents, colorSNSSub in core/aws/catalog_*.go):
// the issue signal comes from the raw Fields the fetcher populates.
type appCtrlZFieldPair struct {
	issue   map[string]string
	healthy map[string]string
}

var appCtrlZRawFieldFixtures = map[string]appCtrlZFieldPair{
	"trail": {
		issue:   map[string]string{"is_logging": "false"},
		healthy: map[string]string{"is_logging": "true", "log_file_validation_enabled": "true"},
	},
	"ssm": {
		issue:   map[string]string{"type": "String", "name": "app_secret"},
		healthy: map[string]string{"type": "String", "name": "app_region"},
	},
	"sns-sub": {
		issue:   map[string]string{"subscription_arn": "PendingConfirmation"},
		healthy: map[string]string{"subscription_arn": "arn:aws:sns:us-east-1:123456789012:topic:sub-id"},
	},
	"rtb": {
		issue:   map[string]string{"blackhole_routes_count": "1", "associations_count": "1", "is_main": "false"},
		healthy: map[string]string{"blackhole_routes_count": "0", "associations_count": "1", "is_main": "false"},
	},
	"alarm": {
		issue:   map[string]string{"state": "ALARM"},
		healthy: map[string]string{"state": "OK", "actions_count": "1"},
	},
	"ct-events": {
		issue:   map[string]string{"status": "ct-danger"},
		healthy: map[string]string{"status": "ct-info"},
	},
}

// appCtrlZHealthyFieldOverrides carries extra Fields for the healthy fixture
// of Finding-driven types whose raw-field fallback path (reached because the
// healthy row has no Finding) treats an all-empty Fields map as a Warning:
// colorIGW, where attachments_count defaults to 0. Without it the healthy
// fixture would itself resolve to an issue color, which the setup-error branch
// below reports rather than passing quietly.
var appCtrlZHealthyFieldOverrides = map[string]map[string]string{
	"igw": {"attachments_count": "1"},
}

// appCtrlZPerTypeResources returns a genuine issue row and a genuine healthy
// row for shortName, built against that type's real Color func so
// TestAppCtrlZInvariant_BadgeCountMatchesVisibleAcrossAllTypes never degrades
// to comparing two empty sets. colorSES only recognizes Source "wave2"
// (never bare "wave1"), so it gets its own Finding source distinct from the
// generic wave1 Finding every other Finding-driven Color func recognizes
// (colorFromAnyFinding accepts any Source, so the bare wave1 Finding colours
// every other registered type).
func appCtrlZPerTypeResources(shortName string) (issue, healthy resource.Resource) {
	issue = resource.Resource{ID: "issue-" + shortName, Name: "issue-" + shortName}
	healthy = resource.Resource{ID: "healthy-" + shortName, Name: "healthy-" + shortName}

	if pair, ok := appCtrlZRawFieldFixtures[shortName]; ok {
		issue.Fields = pair.issue
		healthy.Fields = pair.healthy
		return issue, healthy
	}
	if override, ok := appCtrlZHealthyFieldOverrides[shortName]; ok {
		healthy.Fields = override
	}

	if shortName == "ses" {
		issue.Findings = []domain.Finding{{
			Code: "ses.synthetic-issue", Phrase: "synthetic issue",
			Severity: domain.SevBroken, Source: "wave2:ses",
		}}
		return issue, healthy
	}

	issue.Findings = []domain.Finding{{
		Code: domain.FindingCode(shortName + ".synthetic-issue"), Phrase: "synthetic issue",
		Severity: domain.SevBroken, Source: "wave1",
	}}
	return issue, healthy
}

// Each type gets its own issue/healthy fixture pair (appCtrlZPerTypeResources):
// with a Fields-only seed a Finding-only Color func (e.g. colorEC2) would see an
// empty expected-visible set, and a filter hiding every row would pass.
func TestAppCtrlZInvariant_BadgeCountMatchesVisibleAcrossAllTypes(t *testing.T) {
	for _, short := range appCtrlZRegisteredShortNames() {
		short := short
		t.Run(short, func(t *testing.T) {
			td := resource.FindResourceType(short)
			if td == nil {
				t.Skipf("resource type %q not in registry (may be conditional)", short)
			}
			if td.Color == nil {
				t.Fatalf("%s: Color func is nil — invariant #7 violated", short)
			}

			issueRes, healthyRes := appCtrlZPerTypeResources(short)

			issueColor := td.Color(issueRes)
			if !issueColor.IsIssue() {
				t.Fatalf("%s: test setup error — issue fixture resolved to %v (not an issue); appCtrlZPerTypeResources needs a fixture fix for this type's Color func", short, issueColor)
			}
			healthyColor := td.Color(healthyRes)
			if healthyColor.IsIssue() {
				t.Fatalf("%s: test setup error — healthy fixture resolved to %v (an issue); appCtrlZPerTypeResources needs a fixture fix for this type's Color func", short, healthyColor)
			}

			seed := []resource.Resource{issueRes, healthyRes}
			c := appCtrlZModel(t, short, seed)

			// Count side: GetListIssueCount() (the list frame-title/menu-badge
			// oracle, core/app/list_body.go's listIssueCount) must agree with
			// the same td.Color(r).IsIssue() seed BEFORE the toggle — the
			// filtered-visible-rows check below only proves ActionToggleAttention's
			// OWN filter matches td.Color; it says nothing about whether the
			// separate count aggregation (a different code path) has silently
			// diverged from it.
			if !td.ExcludeFromIssueBadge {
				if got := c.GetListIssueCount(); got != 1 {
					t.Errorf("%s: GetListIssueCount() = %d before ctrl+z, want 1", short, got)
				}
			}

			appCtrlZToggle(c)

			lb := *c.Snapshot().Body.List
			if !lb.AttentionOnly {
				t.Fatalf("%s: AttentionOnly should be true after ActionToggleAttention", short)
			}
			gotVisible := appCtrlZVisibleIDs(c)
			if len(gotVisible) != 1 {
				t.Errorf("%s: visible row count = %d after ctrl+z, want 1 (only the issue row)", short, len(gotVisible))
			}
			if !gotVisible[issueRes.ID] {
				t.Errorf("%s: issue row %q must be visible after ctrl+z", short, issueRes.ID)
			}
			if gotVisible[healthyRes.ID] {
				t.Errorf("%s: healthy row %q must be hidden after ctrl+z", short, healthyRes.ID)
			}
		})
	}
}

func TestAppCtrlZ_EC2_27Rows12Issues(t *testing.T) {
	issueStatuses := []string{
		"stopped", "stopped", "stopped", "stopped", "stopped",
		"stopped", "stopped", "stopped", "stopped",
		"pending", "stopping", "shutting-down",
	}
	var resources []resource.Resource
	for i, s := range issueStatuses {
		resources = append(resources, resource.Resource{
			ID:       "i-issue-" + string(rune('a'+i)),
			Name:     "issue-node-" + string(rune('a'+i)),
			Fields:   map[string]string{"state": s},
			Findings: appCtrlZEc2StateFinding(s),
		})
	}
	for i := 0; i < 14; i++ {
		resources = append(resources, resource.Resource{
			ID:     "i-run-" + string(rune('a'+i)),
			Name:   "healthy-node-" + string(rune('a'+i)),
			Fields: map[string]string{"state": "running"},
		})
	}
	resources = append(resources,
		resource.Resource{ID: "i-term-1", Name: "legacy-app",
			Fields: map[string]string{"state": "terminated"}, Findings: appCtrlZEc2StateFinding("terminated")},
	)
	if got := len(resources); got != 27 {
		t.Fatalf("test setup error: want 27 resources, got %d", got)
	}

	c := appCtrlZModel(t, "ec2", resources)

	if got := len(c.Snapshot().Body.List.Rows); got != 27 {
		t.Fatalf("pre-toggle Rows count: got %d want 27", got)
	}

	appCtrlZToggle(c)
	ids := appCtrlZVisibleIDs(c)

	visibleIssues := 0
	for i := 0; i < len(issueStatuses); i++ {
		if ids["i-issue-"+string(rune('a'+i))] {
			visibleIssues++
		}
	}
	if visibleIssues != 12 {
		t.Errorf("ctrl+z visible issue-rows: got %d, want 12", visibleIssues)
	}

	for i := 0; i < 14; i++ {
		if ids["i-run-"+string(rune('a'+i))] {
			t.Errorf("running row %d must be hidden after ctrl+z (badge invariant)", i)
		}
	}
	if ids["i-term-1"] {
		t.Error("terminated row 'legacy-app' must be hidden after ctrl+z")
	}
}

func TestAppCtrlZ_EC2IssueStatuses_Visible(t *testing.T) {
	ec2td := resource.FindResourceType("ec2")
	if ec2td == nil {
		t.Fatal("ec2 resource type not found in registry")
	}

	// colorEC2 (core/aws/catalog_compute.go) derives color from findings only: a
	// fixture with only raw Fields would make IsIssue false for every row and
	// pass with a broken filter. Each row carries the Findings the enrichment
	// pipeline produces for its state.
	ec2Resources := []resource.Resource{
		{ID: "r-stopped", Name: "row-a-stopped",
			Fields: map[string]string{"state": "stopped"}, Findings: appCtrlZEc2StateFinding("stopped")},
		{ID: "r-stopping", Name: "row-b-stopping",
			Fields: map[string]string{"state": "stopping"}, Findings: appCtrlZEc2StateFinding("stopping")},
		{ID: "r-pending", Name: "row-c-pending",
			Fields: map[string]string{"state": "pending"}, Findings: appCtrlZEc2StateFinding("pending")},
		{ID: "r-impaired", Name: "row-d-impaired",
			Fields: map[string]string{"state": "running", "system_status": "impaired"},
			// Source must be "wave2:<shortName>" — colorFromAnyFinding
			// (core/aws/catalog_color_helpers.go) only recognizes "wave1" or a
			// "wave2:" prefix; the real runtime source (setWave2Finding,
			// core/aws/issue_enrichment.go) is "wave2:" + shortName, never the
			// bare "wave2" that catalog_compute.go's own doc-only Findings
			// table literal uses.
			Findings: []domain.Finding{{Code: "ec2.instance-status-impaired", Phrase: "impaired: system checks failing", Severity: domain.SevBroken, Source: "wave2:ec2"}}},
		{ID: "r-initializing", Name: "row-e-initializing",
			Fields: map[string]string{"state": "running", "instance_status": "initializing"}},
		{ID: "r-running", Name: "row-f-running",
			Fields: map[string]string{"state": "running", "system_status": "ok", "instance_status": "ok"}},
		{ID: "r-terminated", Name: "row-g-terminated",
			Fields: map[string]string{"state": "terminated"}, Findings: appCtrlZEc2StateFinding("terminated")},
		{ID: "r-shutting", Name: "row-h-shutting-down",
			Fields: map[string]string{"state": "shutting-down"}, Findings: appCtrlZEc2StateFinding("shutting-down")},
	}

	c := appCtrlZModel(t, "ec2", ec2Resources)
	appCtrlZToggle(c)
	ids := appCtrlZVisibleIDs(c)

	var sawIssue, sawHealthy bool
	for _, r := range ec2Resources {
		isIssue := ec2td.Color(r).IsIssue()
		if isIssue {
			sawIssue = true
		} else {
			sawHealthy = true
		}
		if isIssue != ids[r.ID] {
			t.Errorf("ec2 row %q (state=%q) visible=%v, want %v (Color.IsIssue=%v)", r.Name, r.Fields["state"], ids[r.ID], isIssue, isIssue)
		}
	}
	if !sawIssue || !sawHealthy {
		t.Fatalf("test setup error: fixture must contain both issue and healthy rows to make the invariant check non-trivial (sawIssue=%v, sawHealthy=%v)", sawIssue, sawHealthy)
	}
}

func TestAppCtrlZ_CTEvents_HidesDimRows(t *testing.T) {
	resources := []resource.Resource{
		{ID: "evt-0001", Name: "read-1", Fields: map[string]string{"status": "ct-info"}},
		{ID: "evt-0002", Name: "write-1", Fields: map[string]string{"status": "ct-attention"}},
		{ID: "evt-0003", Name: "delete-1", Fields: map[string]string{"status": "ct-danger"}},
		{ID: "evt-0004", Name: "read-2", Fields: map[string]string{"status": "ct-info"}},
	}
	c := appCtrlZModel(t, "ct-events", resources)

	if got := len(c.GetListAllResources()); got != 4 {
		t.Fatalf("GetListAllResources before toggle: got %d, want 4", got)
	}

	appCtrlZToggle(c)

	if got := len(c.GetListAllResources()); got != 4 {
		t.Errorf("GetListAllResources after toggle: got %d, want 4 (underlying data must be preserved)", got)
	}

	ids := appCtrlZVisibleIDs(c)
	if !ids["evt-0002"] {
		t.Error("ct-attention row 'write-1' missing from visible set after ctrl+z toggle ON")
	}
	if !ids["evt-0003"] {
		t.Error("ct-danger row 'delete-1' missing from visible set after ctrl+z toggle ON")
	}
	if ids["evt-0001"] {
		t.Error("ct-info row 'read-1' should be hidden after ctrl+z toggle ON")
	}
	if ids["evt-0004"] {
		t.Error("ct-info row 'read-2' should be hidden after ctrl+z toggle ON")
	}
}

func TestAppCtrlZ_Toggles_On_Off_Restores(t *testing.T) {
	resources := []resource.Resource{
		{ID: "evt-0001", Name: "read-1", Fields: map[string]string{"status": "ct-info"}},
		{ID: "evt-0002", Name: "write-1", Fields: map[string]string{"status": "ct-attention"}},
		{ID: "evt-0003", Name: "delete-1", Fields: map[string]string{"status": "ct-danger"}},
		{ID: "evt-0004", Name: "read-2", Fields: map[string]string{"status": "ct-info"}},
	}
	c := appCtrlZModel(t, "ct-events", resources)

	appCtrlZToggle(c)
	appCtrlZToggle(c)

	ids := appCtrlZVisibleIDs(c)
	for _, id := range []string{"evt-0001", "evt-0002", "evt-0003", "evt-0004"} {
		if !ids[id] {
			t.Errorf("row %q missing from visible set after ctrl+z toggle OFF", id)
		}
	}
}

func TestAppCtrlZ_ResetsCursorToTop(t *testing.T) {
	resources := []resource.Resource{
		{ID: "evt-0001", Name: "read-1", Fields: map[string]string{"status": "ct-info"}},
		{ID: "evt-0002", Name: "write-1", Fields: map[string]string{"status": "ct-attention"}},
		{ID: "evt-0003", Name: "delete-1", Fields: map[string]string{"status": "ct-danger"}},
		{ID: "evt-0004", Name: "read-2", Fields: map[string]string{"status": "ct-info"}},
	}
	c := appCtrlZModel(t, "ct-events", resources)

	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	if c.GetListSelectedRow() == 0 {
		t.Fatalf("precondition failed: cursor did not advance past 0 after two ActionMoveDown — cannot exercise the cursor-reset assertion below")
	}

	appCtrlZToggle(c)

	if got := c.GetListSelectedRow(); got != 0 {
		t.Errorf("GetListSelectedRow after ctrl+z toggle ON: got %d, want 0 (cursor must reset to top)", got)
	}
}

func TestAppCtrlZ_EC2_ShowsOnlyIssueRows(t *testing.T) {
	ec2Resources := []resource.Resource{
		{ID: "i-0001", Name: "web-prod",
			Fields: map[string]string{"state": "running"}},
		{ID: "i-0002", Name: "batch-job",
			Fields: map[string]string{"state": "stopped"}, Findings: appCtrlZEc2StateFinding("stopped")},
		{ID: "i-0003", Name: "old-build",
			Fields: map[string]string{"state": "terminated"}, Findings: appCtrlZEc2StateFinding("terminated")},
		{ID: "i-0004", Name: "api-prod",
			Fields: map[string]string{"state": "running"}},
	}
	c := appCtrlZModel(t, "ec2", ec2Resources)
	appCtrlZToggle(c)
	ids := appCtrlZVisibleIDs(c)

	if ids["i-0001"] {
		t.Error("running row 'web-prod' must be HIDDEN after ctrl+z (running is not an issue)")
	}
	if ids["i-0004"] {
		t.Error("running row 'api-prod' must be HIDDEN after ctrl+z (running is not an issue)")
	}
	if !ids["i-0002"] {
		t.Error("stopped row 'batch-job' must be visible after ctrl+z (ColorWarning.IsIssue=true)")
	}
	if ids["i-0003"] {
		t.Error("terminated row 'old-build' must be hidden after ctrl+z (ColorDim.IsIssue=false)")
	}
}

func TestAppCtrlZ_PerControllerState_DoesNotBleed(t *testing.T) {
	ctEventsResources := []resource.Resource{
		{ID: "evt-0001", Name: "read-1", Fields: map[string]string{"status": "ct-info"}},
	}
	ec2Resources := []resource.Resource{
		{ID: "i-0001", Name: "web-prod", Fields: map[string]string{"state": "running"}},
		{ID: "i-0002", Name: "batch-job", Fields: map[string]string{"state": "stopped"}},
	}

	cCT := appCtrlZModel(t, "ct-events", ctEventsResources)
	cEC2 := appCtrlZModel(t, "ec2", ec2Resources)

	appCtrlZToggle(cCT)

	if !cCT.Snapshot().Body.List.AttentionOnly {
		t.Fatal("ct-events controller: AttentionOnly should be true after toggle")
	}
	if cEC2.Snapshot().Body.List.AttentionOnly {
		t.Error("ec2 controller: AttentionOnly should remain false (no bleed from a separate controller instance)")
	}
	ec2IDs := appCtrlZVisibleIDs(cEC2)
	if !ec2IDs["i-0001"] || !ec2IDs["i-0002"] {
		t.Error("ec2 controller: both rows should remain visible (ct-events toggle must not bleed)")
	}
}

// AttentionOnly is a screen-level setting, not tied to a single fetch.
func TestAppCtrlZ_PersistsAcrossReload(t *testing.T) {
	resources := []resource.Resource{
		{ID: "evt-0001", Name: "read-1", Fields: map[string]string{"status": "ct-info"}},
		{ID: "evt-0002", Name: "write-1", Fields: map[string]string{"status": "ct-attention"}},
	}
	c := appCtrlZModel(t, "ct-events", resources)
	appCtrlZToggle(c)
	if !c.GetListAttentionOnly() {
		t.Fatal("AttentionOnly should be true after ctrl+z")
	}

	c.ApplyResourcesLoaded("ct-events", resources, nil, false)

	if !c.GetListAttentionOnly() {
		t.Error("AttentionOnly should still be true after a resource reload (screen setting, not fetch-scoped)")
	}
	ids := appCtrlZVisibleIDs(c)
	if ids["evt-0001"] {
		t.Error("ct-info row 'read-1' should remain hidden after reload")
	}
	if !ids["evt-0002"] {
		t.Error("ct-attention row 'write-1' must remain visible after reload")
	}
}

func TestAppCtrlZ_StatusLineIndicator(t *testing.T) {
	resources := []resource.Resource{
		{ID: "evt-0001", Name: "read-1", Fields: map[string]string{"status": "ct-info"}},
		{ID: "evt-0002", Name: "write-1", Fields: map[string]string{"status": "ct-attention"}},
	}
	c := appCtrlZModel(t, "ct-events", resources)

	if strings.Contains(c.ListFrameTitle(), "[!]") {
		t.Errorf("[!] indicator present before ctrl+z toggle — should only appear when AttentionOnly is active")
	}

	appCtrlZToggle(c)

	if !strings.Contains(c.ListFrameTitle(), "[!]") {
		t.Errorf("[!] indicator not found in ListFrameTitle() after ctrl+z toggle ON: %q", c.ListFrameTitle())
	}
}

package unit_test

import (
	"slices"
	"testing"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// Each constructor states how far the checker looked, and that is the only
// thing that decides whether a zero may be shown as a proven dead end.
func TestRelatedCoverage_ConstructorsCarryCoverage(t *testing.T) {
	cases := []struct {
		name    string
		result  resource.RelatedCheckResult
		target  string
		want    resource.RelatedCoverage
		wantIDs []string
	}{
		{"proven zero", resource.ProvenZero("cfn", "StackSummaries walk completed"), "cfn", resource.CoverageComplete, nil},
		{"exact match set", resource.KnownRelated("sg", []string{"sg-0aaa111111111111a"}, false), "sg", resource.CoverageComplete, []string{"sg-0aaa111111111111a"}},
		{"lower bound with matches", resource.KnownRelated("sg", []string{"sg-0aaa111111111111a"}, true), "sg", resource.CoveragePartial, []string{"sg-0aaa111111111111a"}},
		{"lower bound without matches", resource.KnownRelated("ct-events", nil, true), "ct-events", resource.CoveragePartial, nil},
		{"no discovery path", resource.NoDiscoveryPath("athena"), "athena", resource.CoverageNoPath, nil},
		{"heuristic", resource.HeuristicRelated("tg", []string{"tg-web-prod-01"}), "tg", resource.CoverageHeuristic, []string{"tg-web-prod-01"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.result.Coverage(); got != tc.want {
				t.Errorf("Coverage() = %v, want %v", got, tc.want)
			}
			if got := tc.result.TargetType(); got != tc.target {
				t.Errorf("TargetType() = %q, want %q", got, tc.target)
			}
			if got := tc.result.ResourceIDs(); !slices.Equal(got, tc.wantIDs) {
				t.Errorf("ResourceIDs() = %v, want %v", got, tc.wantIDs)
			}
		})
	}

	pz := resource.ProvenZero("cfn", "StackSummaries walk completed")
	if pz.State() != domain.RelatedResolved || pz.Count() != 0 || pz.Truncated() {
		t.Errorf("ProvenZero = (state %v, count %d, truncated %v), want a resolved, exact zero", pz.State(), pz.Count(), pz.Truncated())
	}
}

// The empty fallback every checker returns when it had nothing to search with
// is not evidence of absence: only ProvenZero may claim a complete zero.
func TestRelatedCoverage_KnownRelatedEmptyIsNotAProvenZero(t *testing.T) {
	r := resource.KnownRelated("cfn", nil, false)
	if r.Coverage() == resource.CoverageComplete {
		t.Errorf("KnownRelated(\"cfn\", nil, false).Coverage() = CoverageComplete; a zero with nothing searched must not be a proven zero")
	}
}

func coverageEC2Resource() resource.Resource {
	return resource.Resource{
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "web-prod-01",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0a1b2c3d4e5f60001",
			"name":        "web-prod-01",
			"state":       "running",
			"vpc_id":      "vpc-0abc123def456789a",
		},
	}
}
func renderRelatedPanel(t *testing.T, src resource.Resource, results map[string]resource.RelatedCheckResult) (map[string]app.RelatedBlock, string) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "us-east-1"
	c := newBlessedController(t, runtime.New(s, nil))
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(src, src.Type)
	c.InitDetailRelatedRows(src.Type)
	for name, r := range results {
		c.Handle(messages.RelatedCheckResult{
			ResourceType:     src.Type,
			SourceResourceID: src.ID,
			DefDisplayName:   name,
			Result:           r,
		})
	}

	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatalf("Snapshot().Body.Detail is nil on an open %s detail", src.Type)
	}
	blocks := map[string]app.RelatedBlock{}
	for _, b := range vs.Body.Detail.Related {
		blocks[b.Name] = b
	}
	vp := viewport.New(viewport.WithWidth(160), viewport.WithHeight(60))
	body := *vs.Body.Detail
	body.RelatedVisible = true
	dm := views.NewTransientDetail(160, 60, vp)
	return blocks, dm.RenderDetail(body)
}

func assertRelatedRow(t *testing.T, blocks map[string]app.RelatedBlock, rendered, name, display string, actionable bool) {
	t.Helper()
	b, ok := blocks[name]
	if !ok {
		t.Fatalf("no related block %q in the detail body", name)
	}
	if b.CountDisplay != display {
		t.Errorf("CountDisplay = %q, want %q", b.CountDisplay, display)
	}
	if b.Actionable != actionable {
		t.Errorf("Actionable = %v, want %v", b.Actionable, actionable)
	}
	text := "  " + name
	if display != "" {
		text += " " + display
	}
	want := styles.RowNormal.Render(text)
	if !actionable {
		want = styles.DimText.Render(text)
	}
	if got := extractRelatedLine(t, rendered, name); got != want {
		t.Errorf("rendered related row\n  got:  %q\n  want: %q", got, want)
	}
}

// Only a complete zero is the dimmed dead end "(0)"; a partial count is a
// bright "(N+)"; a pivot with no discovery path or a heuristic match is a
// blank, navigable row.
func TestRelatedCoverage_RenderedPanel(t *testing.T) {
	blocks, rendered := renderRelatedPanel(t, coverageEC2Resource(), map[string]resource.RelatedCheckResult{
		"Target Groups":       resource.HeuristicRelated("tg", []string{"tg-web-prod-01"}),
		"Auto Scaling Groups": resource.NoDiscoveryPath("asg"),
		"Security Groups":     resource.ProvenZero("sg", "Instance.SecurityGroups"),
		"Network Interfaces":  resource.KnownRelated("eni", []string{"eni-0a1b2c3d4e5f60001"}, true),
		"CloudTrail Events":   resource.KnownRelated("ct-events", nil, true),
		"EBS Volumes":         resource.KnownRelated("ebs", []string{"vol-0a1b2c3d4e5f60001"}, false),
		"Elastic IPs":         resource.KnownRelated("eip", nil, false),
	})

	expect := []struct {
		name       string
		display    string
		actionable bool
	}{
		{"Target Groups", "", true},
		{"Auto Scaling Groups", "", true},
		{"Security Groups", "(0)", false},
		{"Network Interfaces", "(1+)", true},
		{"CloudTrail Events", "(0+)", true},
		{"EBS Volumes", "(1)", true},
	}
	for _, e := range expect {
		t.Run(e.name, func(t *testing.T) {
			assertRelatedRow(t, blocks, rendered, e.name, e.display, e.actionable)
		})
	}

	// The empty KnownRelated fallback is not a proven zero, so it must not
	// render as the dimmed dead end a proven zero is.
	eip, ok := blocks["Elastic IPs"]
	if !ok {
		t.Fatal("no related block \"Elastic IPs\" in the ec2 detail body")
	}
	if eip.CountDisplay == "(0)" && !eip.Actionable {
		t.Errorf("Elastic IPs from KnownRelated(nil, false) renders the dimmed proven zero \"(0)\"")
	}
}

// An API mapped from one custom domain, beside a domain whose API mappings
// run past the walk's cap, shows its certificate as a bright, navigable
// lower bound "(1+)", never an exact "(1)".
func TestRelatedPaging_ApigwACMCappedWalkRendersLowerBound(t *testing.T) {
	r, _ := unit.ApigwACMCappedMappings(t)
	src := resource.Resource{ID: "a1b2c3d4e5", Name: "orders-http-api", Type: "apigw", Fields: map[string]string{"api_id": "a1b2c3d4e5", "name": "orders-http-api"}}
	blocks, rendered := renderRelatedPanel(t, src, map[string]resource.RelatedCheckResult{"ACM Certificates": r})
	assertRelatedRow(t, blocks, rendered, "ACM Certificates", "(1+)", true)
}

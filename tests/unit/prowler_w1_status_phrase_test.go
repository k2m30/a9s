package unit

// prowler_w1_status_phrase_test.go — the Status cell must name the finding
// that decided the row's colour, and a witness must be the only reason its
// own row is coloured.
//
// The row colour takes the highest severity in the slice; the Status cell took
// the first issue-severity finding in slice order. A row whose findings arrive
// warn-then-broken therefore rendered red while reading as the warning, and a
// witness that already carried an older finding never showed its own phrase at
// all. One selector settles both, and these tests pin the selector's rule and
// then check every batch-w1 witness through it on the demo bench.

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// pw1Finding is a synthetic finding with only the fields the selector reads.
func pw1Finding(phrase string, sev domain.Severity) domain.Finding {
	return domain.Finding{
		Code:     domain.FindingCode("test.probe." + phrase),
		Phrase:   phrase,
		Severity: sev,
		Source:   "wave1",
	}
}

// TestTopFinding_HighestSeverityWinsOverSliceOrder pins the rule the row
// colour already follows: the cell names the worst thing wrong, wherever it
// sits in the slice. Taking findings[0] renders a red row as a warning.
func TestTopFinding_HighestSeverityWinsOverSliceOrder(t *testing.T) {
	findings := []domain.Finding{
		pw1Finding("public address", domain.SevWarn),
		pw1Finding("port 22 reachable from the internet", domain.SevBroken),
	}
	got, ok := domain.TopFinding(findings)
	if !ok {
		t.Fatal("TopFinding reported nothing for a non-empty slice")
	}
	if got.Phrase != "port 22 reachable from the internet" {
		t.Errorf("Phrase = %q, want the broken finding's phrase", got.Phrase)
	}
}

// TestTopFinding_SliceOrderDecidesAmongEquals pins the tie-break: two findings
// of the same severity keep the order the fetcher emitted them in, so the cell
// is stable across runs.
func TestTopFinding_SliceOrderDecidesAmongEquals(t *testing.T) {
	findings := []domain.Finding{
		pw1Finding("IMDSv1 allowed", domain.SevWarn),
		pw1Finding("public address", domain.SevWarn),
	}
	got, ok := domain.TopFinding(findings)
	if !ok {
		t.Fatal("TopFinding reported nothing for a non-empty slice")
	}
	if got.Phrase != "IMDSv1 allowed" {
		t.Errorf("Phrase = %q, want the first of two equal-severity findings", got.Phrase)
	}
}

// TestTopFinding_DimOnlyWhenNothingIsAnIssue pins that a dim finding still
// explains a dim row, but never outranks an issue. A terminated instance that
// also allows IMDSv1 reads as the issue, not as the lifecycle state.
func TestTopFinding_DimOnlyWhenNothingIsAnIssue(t *testing.T) {
	dimOnly := []domain.Finding{pw1Finding("terminated", domain.SevDim)}
	got, ok := domain.TopFinding(dimOnly)
	if !ok {
		t.Fatal("TopFinding reported nothing for a dim-only slice")
	}
	if got.Phrase != "terminated" {
		t.Errorf("Phrase = %q, want the dim phrase when nothing is an issue", got.Phrase)
	}

	withIssue := []domain.Finding{
		pw1Finding("terminated", domain.SevDim),
		pw1Finding("IMDSv1 allowed", domain.SevWarn),
	}
	got, ok = domain.TopFinding(withIssue)
	if !ok {
		t.Fatal("TopFinding reported nothing")
	}
	if got.Phrase != "IMDSv1 allowed" {
		t.Errorf("Phrase = %q, want the issue phrase to outrank the dim one", got.Phrase)
	}
}

// TestTopFinding_EmptySliceReportsNothing pins the empty case, which is the
// difference between a blank Status cell and a panic.
func TestTopFinding_EmptySliceReportsNothing(t *testing.T) {
	if _, ok := domain.TopFinding(nil); ok {
		t.Error("TopFinding reported a finding for a nil slice")
	}
	if _, ok := domain.TopFinding([]domain.Finding{}); ok {
		t.Error("TopFinding reported a finding for an empty slice")
	}
}

// ─── demo bench ─────────────────────────────────────────────────────────────

// pw1BenchRow is one demo row with its wave-2 findings folded on, the shape
// the list renders from.
type pw1BenchRow struct {
	typeName string
	res      resource.Resource
}

// pw1ComputeBench builds every batch-w1 type's demo rows and folds each type's
// wave-2 enricher output onto them, so a row's Findings hold what the Status
// cell and the row colour actually read.
func pw1ComputeBench(t *testing.T) []pw1BenchRow {
	t.Helper()
	ec2Fake, ecsFake, lambdaFake, asgFake := fakes.NewEC2(), fakes.NewECS(), fakes.NewLambda(), fakes.NewASG()
	clients := &awsclient.ServiceClients{EC2: ec2Fake, ECS: ecsFake, Lambda: lambdaFake, AutoScaling: asgFake}

	// The sg and ebs lists are cross-referenced by the ec2 and ebs-snap
	// enrichers; the ami list by lt's. Each is drained, because a sibling that
	// sorts onto page two is a cross-reference that silently resolves to
	// nothing.
	cache := resource.ResourceCache{}
	for _, entry := range []struct {
		short string
		fetch func(token string) (resource.FetchResult, error)
	}{
		{"sg", func(token string) (resource.FetchResult, error) {
			return awsclient.FetchSecurityGroupsPage(context.Background(), ec2Fake, token)
		}},
		{"ebs", func(token string) (resource.FetchResult, error) {
			return awsclient.FetchEBSVolumesPage(context.Background(), ec2Fake, token)
		}},
		{"ami", func(token string) (resource.FetchResult, error) {
			return awsclient.FetchAMIsPage(context.Background(), ec2Fake, token)
		}},
	} {
		cache[entry.short] = resource.ResourceCacheEntry{Resources: DrainPages(t, "demo "+entry.short, entry.fetch)}
	}

	var rows []pw1BenchRow
	for _, short := range []string{"ec2", "ami", "ecs-svc", "ecs-task", "lambda", "asg", "ebs-snap", "lt"} {
		td := catalog.FindAny(short)
		if td == nil || td.Fetcher == nil {
			t.Fatalf("%s has no catalog Fetcher", short)
		}
		resources := DrainPages(t, "demo "+short, func(token string) (resource.FetchResult, error) {
			return td.Fetcher(context.Background(), clients, token)
		})
		res := awsclient.IssueEnricherResult{}
		if e, ok := awsclient.Wave2EnricherFor(short); ok && e.Fn != nil {
			var err error
			res, err = e.Fn(context.Background(), clients, resources, cache)
			if err != nil {
				t.Fatalf("demo %s enrich: %v", short, err)
			}
		}
		for i := range resources {
			r := resources[i]
			runtime.ApplyWave2ToRow(&r, *td, res.Findings, res.AttentionDetails)
			rows = append(rows, pw1BenchRow{typeName: short, res: r})
		}
	}
	return rows
}

// pw1WitnessPhrases maps each batch-w1 witness to the phrase its Status cell
// must show. Every one of these is a finding this batch added, so the witness
// exists to display exactly this text.
var pw1WitnessPhrases = []struct {
	typeName string
	id       string
	phrase   string
}{
	{"ecs-task", fixtures.ECSTaskPrivileged, "privileged container"},
	{"ecs-task", fixtures.ECSTaskHostNamespace, "shares the host network or process namespace"},
	{"ecs-task", fixtures.ECSTaskWritableRoot, "writable root filesystem"},
	{"ecs-task", fixtures.ECSTaskNoLogging, "container without log driver"},
	{"ecs-task", fixtures.ECSTaskEnvSecret, "credential in container environment"},
	{"lambda", fixtures.LambdaPublicPolicy, "invokable by anyone"},
	{"lambda", fixtures.LambdaFunctionURLPublic, "function endpoint open without authentication"},
	{"lambda", fixtures.LambdaEnvSecret, "credential in environment variables"},
	{"asg", fixtures.ASGSingleAZ, "single availability zone"},
	{"asg", fixtures.ASGNoELBHealthCheck, "no load balancer health check"},
}

func pw1BenchRowFor(t *testing.T, rows []pw1BenchRow, typeName, id string) resource.Resource {
	t.Helper()
	for _, row := range rows {
		if row.typeName == typeName && row.res.ID == id {
			return row.res
		}
	}
	t.Fatalf("demo row %s/%s not found", typeName, id)
	return resource.Resource{}
}

// TestProwlerW1_BenchContainsEveryWitness pins the bench's own completeness.
// Every other test here looks a witness up and fails when it is absent, so a
// fixture that stops being produced reads as one broken assertion rather than
// as a bench that can no longer see it. This reports the whole gap at once,
// and it is the assertion that fails if the drain ever stops short again.
func TestProwlerW1_BenchContainsEveryWitness(t *testing.T) {
	rows := pw1ComputeBench(t)
	present := map[string]bool{}
	for _, row := range rows {
		present[row.typeName+"/"+row.res.ID] = true
	}
	var missing []string
	for _, w := range pw1WitnessPhrases {
		if !present[w.typeName+"/"+w.id] {
			missing = append(missing, w.typeName+"/"+w.id)
		}
	}
	for _, id := range []string{fixtures.EC2InstanceInternetExposed} {
		if !present["ec2/"+id] {
			missing = append(missing, "ec2/"+id)
		}
	}
	if len(missing) > 0 {
		t.Errorf("the demo bench does not contain %d expected witness row(s): %v\n"+
			"either the fixture is gone or the bench stopped draining before it", len(missing), missing)
	}
	if len(rows) == 0 {
		t.Fatal("the demo bench produced no rows at all")
	}
}

// TestProwlerW1_WitnessPhraseIsTheOneSelected pins that each witness's own
// phrase is the one the Status cell shows. A witness whose phrase loses the
// selection is a fixture that demonstrates nothing.
func TestProwlerW1_WitnessPhraseIsTheOneSelected(t *testing.T) {
	rows := pw1ComputeBench(t)
	for _, w := range pw1WitnessPhrases {
		r := pw1BenchRowFor(t, rows, w.typeName, w.id)
		got, ok := domain.TopFinding(r.Findings)
		if !ok {
			t.Errorf("%s/%s carries no finding at all", w.typeName, w.id)
			continue
		}
		if got.Phrase != w.phrase {
			t.Errorf("%s/%s: Status cell shows %q, want its own %q (all: %+v)",
				w.typeName, w.id, got.Phrase, w.phrase, r.Findings)
		}
	}
}

// TestProwlerW1_WitnessCarriesExactlyOneIssue pins the fixture rule on the
// witness itself: it is coloured by its own finding and nothing else, so the
// demo bench shows one signal per row. ec2 rows 2 and 3 are excluded by
// ruling — an internet-exposed instance necessarily has a public address.
func TestProwlerW1_WitnessCarriesExactlyOneIssue(t *testing.T) {
	rows := pw1ComputeBench(t)
	for _, w := range pw1WitnessPhrases {
		r := pw1BenchRowFor(t, rows, w.typeName, w.id)
		var issues []string
		for _, f := range r.Findings {
			if f.Severity.IsIssue() {
				issues = append(issues, string(f.Code))
			}
		}
		if len(issues) != 1 {
			t.Errorf("%s/%s carries %d issue-severity findings, want 1: %v",
				w.typeName, w.id, len(issues), issues)
		}
	}
}

// TestProwlerW1_ExposedInstanceReadsAsBrokenNotWarned pins the selector on the
// one demo row where the defect was actually reachable: the internet-exposed
// instance carries the wave-1 public-address warning first and the wave-2
// exposure finding second, so slice order and severity disagree. Its row is
// red; taking findings[0] made the cell read as the warning.
func TestProwlerW1_ExposedInstanceReadsAsBrokenNotWarned(t *testing.T) {
	rows := pw1ComputeBench(t)
	r := pw1BenchRowFor(t, rows, "ec2", fixtures.EC2InstanceInternetExposed)

	var sawWarnFirst bool
	for _, f := range r.Findings {
		if f.Code == pw1EC2CodePublicIP {
			sawWarnFirst = true
		}
		if f.Code == pw1EC2CodeInternetExposed {
			break
		}
	}
	if !sawWarnFirst {
		t.Fatalf("fixture no longer stacks the warning ahead of the exposure, so this "+
			"pins nothing; findings = %+v", r.Findings)
	}

	got, ok := domain.TopFinding(r.Findings)
	if !ok {
		t.Fatal("the exposed instance carries no finding")
	}
	if got.Severity != domain.SevBroken {
		t.Errorf("Status cell shows %q (%v), want the broken exposure finding", got.Phrase, got.Severity)
	}
	if got.Code != pw1EC2CodeInternetExposed {
		t.Errorf("Status cell shows %q, want %q", got.Code, pw1EC2CodeInternetExposed)
	}
}

// TestProwlerW1_ColourAndStatusCellAgreeOnEveryDemoRow pins the invariant the
// whole selection change exists for: the row's colour and the phrase in its
// Status cell come from the same finding. Deleting the wave-source filter in
// colorFromAnyFinding widened what the colour considers, so this checks the
// two did not drift apart on any row of the eight batch types.
func TestProwlerW1_ColourAndStatusCellAgreeOnEveryDemoRow(t *testing.T) {
	for _, row := range pw1ComputeBench(t) {
		td := catalog.FindAny(row.typeName)
		if td == nil {
			t.Fatalf("%s not in the catalog", row.typeName)
		}
		top, ok := domain.TopFinding(row.res.Findings)
		if !ok {
			continue // no findings: colour is the type's structural fallback
		}
		want := td.ResolveColor(row.res)
		got := resource.ColorFromSeverity(top.Severity)
		if want != got {
			t.Errorf("%s/%s: row colour %v but Status cell shows %q (%v) — colour and cell disagree; findings=%+v",
				row.typeName, row.res.ID, want, top.Phrase, top.Severity, row.res.Findings)
		}
	}
}

package unit_test

// uninspected_row_sink_test.go — the sink for "a9s could not inspect this row".
//
// A Wave-2 enricher that fails on one row records that row in
// IssueEnricherResult.TruncatedIDs (core/aws/issue_enrichment.go's MarkSkipped
// / markAllUninspected / capAtEnrichmentCap). The fact reaches the session via
// messages.EnrichmentChecked. These pins hold it to the two surfaces an
// operator actually looks at: the list's Status cell and the detail view's
// Attention block. Without them an uninspected row renders exactly like an
// inspected-and-healthy one, which is the one thing a failed check must never
// claim.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// uninspectedEC2Rows returns two EC2 rows that differ only in identity, so any
// difference the pins observe comes from the truncation signal alone.
func uninspectedEC2Rows() []resource.Resource {
	return []resource.Resource{
		{ID: "i-0aaa111111111111a", Name: "web-server", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0aaa111111111111a", "name": "web-server", "state": "running"}},
		{ID: "i-0bbb222222222222b", Name: "api-server", Type: "ec2",
			Fields: map[string]string{"instance_id": "i-0bbb222222222222b", "name": "api-server", "state": "running"}},
	}
}

// uninspectedCheck is the API call the pins below make fail, so the check the
// row's Attention entry has to name is a value this file wrote and not one it
// read back out of the code under test.
const uninspectedCheck = "DescribeInstanceStatus"

// uninspectedDenied is what the SDK hands an enricher when a per-item call is
// refused: an operation error naming the call, wrapping the service's own
// code.
func uninspectedDenied(op string) error {
	return &smithy.OperationError{
		ServiceID:     "EC2",
		OperationName: op,
		Err: &smithy.GenericAPIError{
			Code:    "AccessDenied",
			Message: "User: arn:aws:iam::123456789012:user/example is not authorized to perform: ec2:" + op,
		},
	}
}

// uninspectedResultFor builds a REAL enricher result the way a Wave-2 enricher
// whose per-item call failed does: the shared empty result every enricher
// starts from, then awsclient.MarkSkipped — never a hand-written map literal,
// which would pin the shape the recorder carries the mark in rather than the
// mark itself.
func uninspectedResultFor(ids ...string) awsclient.IssueEnricherResult {
	result, _ := awsclient.InFetcherWave2Sentinel(context.Background(), nil, nil, nil)
	var failures []awsclient.Failure
	for _, id := range ids {
		awsclient.MarkSkipped(&result, id, &failures, uninspectedDenied(uninspectedCheck))
	}
	return result
}

// statusCellFor returns the Status-column cell of the row with the given ID.
func statusCellFor(t *testing.T, body app.ListBody, id string) string {
	t.Helper()
	if body.StatusCol < 0 {
		t.Fatalf("resource type has no Status column (StatusCol=%d) — the pin cannot observe the sink", body.StatusCol)
	}
	for _, row := range body.Rows {
		if row.ResourceID == id {
			if body.StatusCol >= len(row.Cells) {
				t.Fatalf("StatusCol %d out of range for row %q with %d cells", body.StatusCol, id, len(row.Cells))
			}
			return row.Cells[body.StatusCol]
		}
	}
	t.Fatalf("row %q not present in the list body", id)
	return ""
}

// colorFor returns the pre-resolved colour tag of the row with the given ID.
func colorFor(t *testing.T, body app.ListBody, id string) string {
	t.Helper()
	for _, row := range body.Rows {
		if row.ResourceID == id {
			return row.Color
		}
	}
	t.Fatalf("row %q not present in the list body", id)
	return ""
}

// TestUninspectedRow_ListStatusCellSaysNotInspected pins the list surface: the
// row the enricher could not inspect reads "not inspected" in its Status cell,
// its neighbour keeps its lifecycle word, and neither row's colour moves (an
// unknown posture is not an issue — it must not colour the row or bump a
// badge).
func TestUninspectedRow_ListStatusCellSaysNotInspected(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	rows := uninspectedEC2Rows()
	c.ApplyResourcesLoaded("ec2", rows, nil, false)
	core.ObserveRows("ec2", rows, nil, session.OriginFetch, false)

	before := *c.Snapshot().Body.List
	wantHealthy := colorFor(t, before, "i-0aaa111111111111a")

	result := uninspectedResultFor("i-0aaa111111111111a")
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: result.TruncatedIDs,
		Findings:     result.Findings,
	})
	c.ApplyIntents(intents)

	body := *c.Snapshot().Body.List

	if got := statusCellFor(t, body, "i-0aaa111111111111a"); got != "not inspected" {
		t.Errorf("uninspected row Status cell = %q, want %q — a row the enricher could not inspect renders as inspected-and-healthy", got, "not inspected")
	}
	if got := statusCellFor(t, body, "i-0bbb222222222222b"); got == "not inspected" {
		t.Errorf("inspected row Status cell = %q — the truncation mark leaked onto a row the enricher DID answer for", got)
	}
	if got := colorFor(t, body, "i-0aaa111111111111a"); got != wantHealthy {
		t.Errorf("uninspected row Color = %q, want %q (unchanged) — an unknown posture must not colour the row", got, wantHealthy)
	}
}

// TestUninspectedRow_FindingPhraseWinsOverNotInspected pins the precedence the
// spec fixes: a row that carries a finding phrase keeps it. "not inspected"
// only fills a cell that would otherwise say nothing about the check.
func TestUninspectedRow_FindingPhraseWinsOverNotInspected(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	rows := uninspectedEC2Rows()
	rows[0].Findings = []domain.Finding{{
		Code: "ec2.impaired", Phrase: "system check failed",
		Severity: domain.SevBroken, Source: "wave1",
	}}
	c.ApplyResourcesLoaded("ec2", rows, nil, false)
	core.ObserveRows("ec2", rows, nil, session.OriginFetch, false)

	result := uninspectedResultFor("i-0aaa111111111111a")
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: result.TruncatedIDs,
		Findings:     result.Findings,
	})
	c.ApplyIntents(intents)

	body := *c.Snapshot().Body.List
	if got := statusCellFor(t, body, "i-0aaa111111111111a"); got != "system check failed" {
		t.Errorf("Status cell = %q, want the finding phrase %q — a phrase wins over the not-inspected word", got, "system check failed")
	}
}

// TestUninspectedRow_DetailAttentionCarriesNotInspected pins the detail
// surface: the Attention block names the check that did not answer, so the
// operator reads "posture unknown" rather than an absent Attention block,
// which reads as "clean".
func TestUninspectedRow_DetailAttentionCarriesNotInspected(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	rows := uninspectedEC2Rows()
	c.ApplyResourcesLoaded("ec2", rows, nil, false)
	core.ObserveRows("ec2", rows, nil, session.OriginFetch, false)

	result := uninspectedResultFor("i-0aaa111111111111a")
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: result.TruncatedIDs,
		Findings:     result.Findings,
	})
	c.ApplyIntents(intents)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: rows[0].ID},
		},
	})
	c.EnsureDetailState(rows[0], "ec2")

	detail := c.Snapshot().Body.Detail
	if detail == nil {
		t.Fatal("no detail body on the snapshot after EnsureDetailState")
	}
	// The whole Attention block: the entry line and the sentence lines below
	// it are separate FieldRows, so a collector keyed on the phrase alone
	// would never see the sentence it is asserting about.
	var attention []string
	inBlock := false
	for _, f := range detail.Fields {
		switch {
		case f.IsSection && strings.HasPrefix(f.Key, "Attention"):
			inBlock = true
		case f.IsSection:
			inBlock = false
		case inBlock && f.IsSpacer:
			inBlock = false
		case inBlock:
			attention = append(attention, f.Key+"="+f.Value)
		}
	}
	if len(attention) == 0 {
		var keys []string
		for _, f := range detail.Fields {
			keys = append(keys, f.Key)
		}
		t.Fatalf("detail body carries no \"not inspected\" Attention entry for a row the ec2 enricher could not inspect; fields were:\n%s", strings.Join(keys, "\n"))
	}
	// The entry names the check that did not answer: a row whose posture is
	// unknown is a different fact from a row whose posture is unknown BECAUSE
	// DescribeInstanceStatus was refused — the second one the operator can
	// act on, and the resource type alone they can already read off the
	// screen.
	joined := strings.Join(attention, " ")
	want := domain.NotInspectedPhrase + ": " + uninspectedCheck
	if !strings.Contains(joined, want) {
		t.Errorf("the Attention entry reads %v, want one entry %q — the row reached the detail view "+
			"without the check that refused, so the operator is told a check failed and not which",
			attention, want)
	}
	if !strings.Contains(strings.ToLower(joined), "unknown rather than clean") {
		t.Errorf("the Attention entry does not say the posture is unknown rather than clean: %v", attention)
	}
}

// TestUninspectedRow_InspectedNeighbourGetsNoNotInspectedEntry is the negative
// half: the check name must travel with the row it was recorded for. A second
// row the same enricher answered for carries no Attention entry at all — an
// entry there would read as a failed check on a row that passed one.
func TestUninspectedRow_InspectedNeighbourGetsNoNotInspectedEntry(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	rows := uninspectedEC2Rows()
	c.ApplyResourcesLoaded("ec2", rows, nil, false)
	core.ObserveRows("ec2", rows, nil, session.OriginFetch, false)

	result := uninspectedResultFor(rows[0].ID)
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: result.TruncatedIDs,
		Findings:     result.Findings,
	})
	c.ApplyIntents(intents)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: rows[1].ID},
		},
	})
	c.EnsureDetailState(rows[1], "ec2")

	detail := c.Snapshot().Body.Detail
	if detail == nil {
		t.Fatal("no detail body on the snapshot after EnsureDetailState")
	}
	for _, f := range detail.Fields {
		if strings.Contains(strings.ToLower(f.Key), domain.NotInspectedPhrase) {
			t.Errorf("the row the enricher DID answer for carries %q in its detail — the mark leaked "+
				"off the row it was recorded for", f.Key)
		}
	}
}

// TestFindingsOverview_IsGone: Controller.FindingsOverview aggregated
// findings per rule for a cross-type cockpit that was never built, so
// nothing in this repo called it. A caller-less public API is not "wired".
func TestFindingsOverview_IsGone(t *testing.T) {
	root, err := filepath.Abs("../../core")
	if err != nil {
		t.Fatalf("resolve core: %v", err)
	}
	var offenders []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(src), "\n") {
			if strings.Contains(line, "FindingsOverview") {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, fmt.Sprintf("core/%s:%d: %s", rel, i+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk core: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("FindingsOverview is back with no caller:\n%s", strings.Join(offenders, "\n"))
	}
}

// ---------------------------------------------------------------------------
// The tail past EnrichmentCap
// ---------------------------------------------------------------------------

// kmsRotationFake answers GetKeyRotationStatus for every key, so the only
// reason a key can end up uninspected is the cap itself.
type kmsRotationFake struct {
	awsclient.KMSAPI
	mu    sync.Mutex
	calls int
}

func (f *kmsRotationFake) GetKeyPolicy(_ context.Context, _ *kms.GetKeyPolicyInput, _ ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error) {
	return &kms.GetKeyPolicyOutput{Policy: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"kms:*","Resource":"*"}]}`)}, nil
}

func (f *kmsRotationFake) GetKeyRotationStatus(_ context.Context, _ *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: true}, nil
}

// cappedKMSKeys returns n customer-managed keys, ordered so key i is the
// (i+1)-th row of the list.
func cappedKMSKeys(n int) []resource.Resource {
	out := make([]resource.Resource, 0, n)
	for i := range n {
		id := fmt.Sprintf("key-%03d", i+1)
		out = append(out, resource.Resource{
			ID: id, Name: id, Type: "kms",
			Fields: map[string]string{"key_id": id, "alias": "alias/" + id, "status": "Enabled"},
		})
	}
	return out
}

// TestEnrichmentCap_TailIsMarkedUninspected pins the cap half: a real enricher
// asked for more rows than EnrichmentCap inspects the first EnrichmentCap and
// records every row past them as uninspected. Before capAtEnrichmentCap owned
// every cap site, the tail carried no per-row mark at all — it rendered as
// inspected-and-healthy.
func TestEnrichmentCap_TailIsMarkedUninspected(t *testing.T) {
	const total = awsclient.EnrichmentCap + 1
	keys := cappedKMSKeys(total)
	fake := &kmsRotationFake{}
	clients := &awsclient.ServiceClients{KMS: fake}

	result, err := awsclient.EnrichKMSRotation(context.Background(), clients, keys, nil)
	if err != nil {
		t.Fatalf("EnrichKMSRotation returned error: %v", err)
	}

	tail := keys[awsclient.EnrichmentCap].ID
	if _, marked := result.TruncatedIDs[tail]; !marked {
		t.Errorf("row %d (%q) is not in TruncatedIDs — the tail past EnrichmentCap carries no per-row mark, so it renders as inspected-and-healthy", total, tail)
	}
	for _, r := range keys[:awsclient.EnrichmentCap] {
		if _, marked := result.TruncatedIDs[r.ID]; marked {
			t.Errorf("row %q is inside the cap but was marked uninspected", r.ID)
		}
	}
	if fake.calls > awsclient.EnrichmentCap {
		t.Errorf("GetKeyRotationStatus called %d times, want at most %d — the cap did not bound the work", fake.calls, awsclient.EnrichmentCap)
	}
}

// TestEnrichmentCap_FiftyFirstRowRendersNotInspected joins the two halves the
// spec asks for: a REAL capped enricher result, driven through the production
// event path, makes the 51st row of a capped type say "not inspected" on the
// list.
func TestEnrichmentCap_FiftyFirstRowRendersNotInspected(t *testing.T) {
	const total = awsclient.EnrichmentCap + 1
	keys := cappedKMSKeys(total)

	result, err := awsclient.EnrichKMSRotation(context.Background(),
		&awsclient.ServiceClients{KMS: &kmsRotationFake{}}, keys, nil)
	if err != nil {
		t.Fatalf("EnrichKMSRotation returned error: %v", err)
	}

	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "kms"})
	c.ApplyResourcesLoaded("kms", keys, nil, false)
	core.ObserveRows("kms", keys, nil, session.OriginFetch, false)

	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "kms",
		TruncatedIDs: result.TruncatedIDs,
		Findings:     result.Findings,
	})
	c.ApplyIntents(intents)

	body := *c.Snapshot().Body.List
	tail := keys[awsclient.EnrichmentCap].ID
	if got := statusCellFor(t, body, tail); got != domain.NotInspectedPhrase {
		t.Errorf("the %dth row (%q) Status cell = %q, want %q", total, tail, got, domain.NotInspectedPhrase)
	}
	if got := statusCellFor(t, body, keys[0].ID); got == domain.NotInspectedPhrase {
		t.Errorf("row 1 Status cell = %q — a row inside the cap must not read as uninspected", got)
	}
}

// capBypass names one shape that bounds or unbounds an enricher's answer
// without going through the helper that owns it. Every cap in core/aws has one
// owner, and each owner does the half a hand-written site forgets: recording
// what the bound dropped.
var capBypass = []struct {
	pattern *regexp.Regexp
	fix     string
	// enricherOnly limits the pattern to files that build an
	// IssueEnricherResult; other result types share the field name but
	// have one sequential writer and no passes to compose.
	enricherOnly bool
}{
	{
		// A per-item work list trimmed by hand drops its tail silently;
		// only capAtEnrichmentCap records the dropped rows in TruncatedIDs.
		regexp.MustCompile(`min\(len\([^)]*\), EnrichmentCap\)`),
		"route the work list through capAtEnrichmentCap(result, items, resourceIDsOf)",
		false,
	},
	{
		// An account-wide walk that compares its own page counter stops at
		// the right page and says nothing about the rows past it; only
		// walkAccountPages marks them.
		regexp.MustCompile(`pages\s*(>=|<|>|<=)\s*EnrichmentCap`),
		"drive the walk with walkAccountPages(result, resources, idOf, next)",
		false,
	},
	{
		// A bare assignment to the flag composes with none of the passes
		// that raise it, so the last one to finish cleanly erases what an
		// earlier one found.
		regexp.MustCompile(`\.Truncated\s*=[^=]`),
		"raise the flag with SetTruncated(result, cut), or drop it with MarkInformationalOnly(result)",
		true,
	},
	{
		// A builder that trims its own row list to a number of its own
		// choosing drops the rest with nothing said, and no reader can tell
		// a plan with five failures from one with fifty. capRows is the only
		// bound on supporting rows and it closes with "… +K more". A batch
		// size the AWS API imposes is named, not a literal, so it does not
		// match.
		regexp.MustCompile(`\b(min|max)\([^,]*,\s*\d+\)`),
		"hand every row to setWave2Finding / addWave1Rows and let capRows bound the list",
		false,
	},
}

// TestEnrichmentCap_EveryCapSiteRoutesThroughTheHelper is the standing gate for
// the sweep. A new enricher that writes any of the bounds above out by hand
// fails here rather than shipping answered rows beside an unbounded number of
// rows that quietly claim to be clean.
//
// issue_enrichment.go is where all three owners live, so it is the one file
// the scan does not read.
func TestEnrichmentCap_EveryCapSiteRoutesThroughTheHelper(t *testing.T) {
	root, err := filepath.Abs("../../core/aws")
	if err != nil {
		t.Fatalf("resolve core/aws: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read core/aws: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || name == "issue_enrichment.go" {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		enricher := strings.Contains(string(src), "IssueEnricherResult")
		for i, line := range strings.Split(string(src), "\n") {
			for _, b := range capBypass {
				if b.enricherOnly && !enricher {
					continue
				}
				if b.pattern.MatchString(line) {
					t.Errorf("%s:%d bypasses the cap it applies — %s\n    %s",
						name, i+1, b.fix, strings.TrimSpace(line))
				}
			}
		}
	}
}

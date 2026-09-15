// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// on_demand_row_branches_test.go — the branches of the on-demand row lane an
// operator reaches but no other test drives: the op identity a host dedups
// on, the terminal's route for the answer, a task whose rows left the live
// set between dispatch and execution, an answer that failed, an answer that
// survives a sweep dispatched before it, the web host's region command, a
// clean answer's reach into the list store, and the badge's lower-bound flag
// on a truncated page.

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// emptyEnricherResult is the shape a Wave-2 enricher starts from; a nil map
// here reads downstream as "answered nothing" rather than "not asked".
func emptyEnricherResult() awsclient.IssueEnricherResult {
	return awsclient.IssueEnricherResult{
		Findings:         map[string][]domain.Finding{},
		AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{},
		TruncatedIDs:     map[string]string{},
		FieldUpdates:     map[string]map[string]string{},
	}
}

// stubEC2Enricher installs fn as ec2's Wave-2 enricher for the test.
func stubEC2Enricher(t *testing.T, fn func(resources []resource.Resource) (awsclient.IssueEnricherResult, error)) {
	t.Helper()
	awsclient.SetWave2EnricherForTest(t, "ec2", awsclient.IssueEnricher{Fn: func(_ context.Context, _ *awsclient.ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		return fn(resources)
	}})
}

// enrichRowTask returns the KindEnrichRow task a detail operation on row
// dispatches, or fails.
func enrichRowTask(t *testing.T, core *runtime.Core, row resource.Resource) runtime.TaskRequest {
	t.Helper()
	_, tasks := core.BeginDetailOperation("ec2", row, false)
	for _, task := range tasks {
		if task.Key.Kind == runtime.KindEnrichRow {
			return task
		}
	}
	t.Fatalf("the detail operation on %q dispatched no KindEnrichRow task; tasks were %+v", row.ID, tasks)
	return runtime.TaskRequest{}
}

// runEnrichRow opens row's detail, executes its on-demand check and folds the
// answer, returning what the fold produced.
func runEnrichRow(t *testing.T, c *app.Controller, core *runtime.Core, row resource.Resource) ([]runtime.UIIntent, []runtime.TaskRequest) {
	t.Helper()
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: row.ID},
	}})
	c.EnsureDetailState(row, "ec2")
	ev, err := core.ExecuteTask(context.Background(), enrichRowTask(t, core, row))
	if err != nil {
		t.Fatalf("KindEnrichRow: %v", err)
	}
	intents, tasks := core.HandleEvent(ev)
	c.ApplyIntents(intents)
	return intents, tasks
}

// Row 1 — a host that admits detail work by operation identity reads the id
// off the task's payload. Two opens of the same capped row must be
// distinguishable there, or the newer open cannot supersede the older.
func TestEnrichRowTask_CarriesItsDetailOperationID(t *testing.T) {
	stubEC2Enricher(t, func([]resource.Resource) (awsclient.IssueEnricherResult, error) {
		return emptyEnricherResult(), nil
	})
	_, core, rows := cappedEC2List(t)

	first := runtime.TaskOpID(enrichRowTask(t, core, rows[0]).Payload)
	second := runtime.TaskOpID(enrichRowTask(t, core, rows[0]).Payload)

	if first == 0 || second == 0 {
		t.Fatalf("the on-demand check's op ids are %d and %d; a zero id is 'no operation' and defeats op-aware admission", first, second)
	}
	if second <= first {
		t.Errorf("reopening the row gave op id %d, want one strictly newer than %d", second, first)
	}
}

// Row 2 — the terminal's own message switch. An on-demand answer that the
// TUI does not route reaches no screen: the detail keeps reading "not
// inspected" for as long as it is open.
func TestTUI_RowEnrichedReachesTheOpenDetail(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	tuitest.NoColor(t)
	stubEC2Enricher(t, func(resources []resource.Resource) (awsclient.IssueEnricherResult, error) {
		res := emptyEnricherResult()
		for _, r := range resources {
			res.Findings[r.ID] = []domain.Finding{{Code: "ec2.impaired", Phrase: onDemandPhrase, Detail: "The instance failed its system status check.", Severity: domain.SevBroken, Source: "wave2:ec2"}}
		}
		return res, nil
	})

	m := newBlessedModel(t, "codex10-prof", "us-east-1")
	m, _ = tuitest.Step(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	rows := uninspectedEC2Rows()
	m = tuitest.StepModel(m, messages.ClientsReady{Clients: demo.NewServiceClients(), Gen: 1})
	m = tuitest.StepModel(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m = tuitest.StepModel(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	m = tuitest.StepModel(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap},
	})

	m, cmd := tuitest.Step(m, messages.Navigate{Target: messages.TargetDetail, ResourceType: "ec2", Resource: &rows[0]})
	if cmd == nil {
		t.Fatal("opening the capped row's detail issued no command")
	}
	answer := findRowEnriched(t, cmd)
	// The Attention block sentence-cases the phrase it renders.
	if before := tuitest.StripANSI(tuitest.Render(m)); strings.Contains(strings.ToLower(before), onDemandPhrase) {
		t.Fatalf("precondition: the detail already reads %q before the answer is delivered:\n%s", onDemandPhrase, before)
	}

	m = tuitest.StepModel(m, answer)

	view := tuitest.StripANSI(tuitest.Render(m))
	if !strings.Contains(strings.ToLower(view), onDemandPhrase) {
		t.Errorf("the open detail does not show %q after its on-demand answer arrived; view:\n%s", onDemandPhrase, view)
	}
}

// findRowEnriched runs cmd's top-level batch and returns the single
// messages.RowEnriched it produced.
func findRowEnriched(t *testing.T, cmd tea.Cmd) messages.RowEnriched {
	t.Helper()
	out := cmd()
	batch, ok := out.(tea.BatchMsg)
	if !ok {
		if msg, is := out.(messages.RowEnriched); is {
			return msg
		}
		t.Fatalf("the detail open produced %T, not a batch carrying the on-demand answer", out)
	}
	var found []messages.RowEnriched
	for _, c := range batch {
		if msg, is := c().(messages.RowEnriched); is {
			found = append(found, msg)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the detail open produced %d RowEnriched answers, want 1", len(found))
	}
	if found[0].Err != nil {
		t.Fatalf("the on-demand check failed: %v", found[0].Err)
	}
	return found[0]
}

// Row 3 — a task dispatched while the rows were live, executed after a pair
// switch left the type holding the disk copy. A disk row carries no
// RawStruct, so a rule that needs one would skip silently and read as a clean
// answer; the task must answer at the cap instead, without asking the
// enricher.
func TestEnrichRowTask_RowsNoLongerLiveAnswerAtTheCap(t *testing.T) {
	var asked [][]string
	onDemandEC2Enricher(t, &asked)
	_, core, rows := cappedEC2List(t)
	task := enrichRowTask(t, core, rows[0])

	core.Session().Rotate()
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginDisk, false)

	ev, err := core.ExecuteTask(context.Background(), task)
	if err != nil {
		t.Fatalf("KindEnrichRow: %v", err)
	}
	msg, ok := ev.(messages.RowEnriched)
	if !ok {
		t.Fatalf("the on-demand check produced %T, want messages.RowEnriched", ev)
	}
	if !msg.Uninspected || msg.Check != awsclient.CheckCap {
		t.Errorf("the answer is Uninspected=%v Check=%q, want true and %q", msg.Uninspected, msg.Check, awsclient.CheckCap)
	}
	if len(asked) != 0 {
		t.Errorf("the check asked the enricher about %v; a disk-seeded row has nothing to check", asked)
	}
}

// Row 4a — an on-demand check that came back with an error and nothing else
// answered for nobody: the row keeps the mark it had and the operator hears
// why, and nothing is written to disk.
func TestOnDemandAnswer_ErrorAloneKeepsTheMarkAndFlashes(t *testing.T) {
	probeErr := errors.New("DescribeInstanceStatus was throttled")
	stubEC2Enricher(t, func([]resource.Resource) (awsclient.IssueEnricherResult, error) {
		return emptyEnricherResult(), probeErr
	})
	c, core, rows := cappedEC2List(t)

	_, tasks := runEnrichRow(t, c, core, rows[0])

	if got := c.Snapshot().Header.Flash; got.Text != "enrich ec2: "+probeErr.Error() || !got.IsError {
		t.Errorf("the flash is %+v, want the error %q named against ec2", got, probeErr)
	}
	if got := core.EnrichmentTruncatedIDs("ec2")[rows[0].ID]; got != awsclient.CheckCap {
		t.Errorf("after a failed on-demand check the row's mark is %q, want %q", got, awsclient.CheckCap)
	}
	if hasTaskKind(tasks, runtime.TaskKindSaveCache) {
		t.Errorf("a failed on-demand check dispatched a cache save; it learned nothing to save")
	}
	c.Apply(app.Action{Kind: app.ActionBack})
	if got := statusCellFor(t, *c.Snapshot().Body.List, rows[0].ID); got != domain.NotInspectedPhrase {
		t.Errorf("after a failed on-demand check the Status cell reads %q, want %q", got, domain.NotInspectedPhrase)
	}
}

// Row 4b — the same failure, but the check read a field before it failed.
// That much it did answer, so the field lands.
func TestOnDemandAnswer_ErrorWithAFieldLandsTheField(t *testing.T) {
	stubEC2Enricher(t, func(resources []resource.Resource) (awsclient.IssueEnricherResult, error) {
		res := emptyEnricherResult()
		for _, r := range resources {
			res.FieldUpdates[r.ID] = map[string]string{"name": "web-server-renamed"}
		}
		return res, errors.New("DescribeInstanceStatus was throttled")
	})
	c, core, rows := cappedEC2List(t)

	runEnrichRow(t, c, core, rows[0])

	if !detailShowsValue(t, c, "web-server-renamed") {
		t.Errorf("the field the failed check did read never reached the open detail")
	}
	c.Apply(app.Action{Kind: app.ActionBack})
	if got := cellFor(t, *c.Snapshot().Body.List, rows[0].ID, "name"); got != "web-server-renamed" {
		t.Errorf("the name cell reads %q, want the value the failed check did read", got)
	}
}

// Row 5 — a sweep dispatched before the on-demand answer landed reports the
// row at its cap. It never looked at the row, so the answer stands whole: the
// phrase and the rows that support it.
func TestOnDemandAnswer_KeepsItsAttentionDetailsThroughAStaleSweep(t *testing.T) {
	const supportLabel, supportValue = "Status check", "instance reachability failed"
	stubEC2Enricher(t, func(resources []resource.Resource) (awsclient.IssueEnricherResult, error) {
		res := emptyEnricherResult()
		for _, r := range resources {
			res.Findings[r.ID] = []domain.Finding{{Code: "ec2.impaired", Phrase: onDemandPhrase, Detail: "The instance failed its system status check.", Severity: domain.SevBroken, Source: "wave2:ec2"}}
			res.AttentionDetails[r.ID] = map[domain.FindingCode]domain.AttentionDetail{
				"ec2.impaired": {Rows: []domain.DetailRow{{Label: supportLabel, Value: supportValue}}},
			}
		}
		return res, nil
	})
	c, core, rows := cappedEC2List(t)

	runEnrichRow(t, c, core, rows[0])
	if entries := topAttentionEntries(t, c); !strings.Contains(strings.Join(entries, " "), supportValue) {
		t.Fatalf("precondition: the answered row's Attention block reads %v, want the supporting row %q", entries, supportValue)
	}

	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap, rows[1].ID: awsclient.CheckCap},
	})
	c.ApplyIntents(intents)

	entries := topAttentionEntries(t, c)
	joined := strings.Join(entries, " ")
	if !strings.Contains(joined, onDemandPhrase) {
		t.Errorf("after a stale sweep the Attention block reads %v, want the finding %q", entries, onDemandPhrase)
	}
	if !strings.Contains(joined, supportValue) {
		t.Errorf("after a stale sweep the Attention block reads %v, want the supporting row %q the answer carried", entries, supportValue)
	}
}

// Row 5 — a clean on-demand answer is as much an answer as a finding is. A
// sweep dispatched before it reports the row at its cap; the row carries
// nothing for the fold to restore, and must still come out answered rather
// than back under the cap the operator already cleared by opening it.
func TestOnDemandAnswer_CleanAnswerSurvivesAStaleSweep(t *testing.T) {
	stubEC2Enricher(t, func([]resource.Resource) (awsclient.IssueEnricherResult, error) {
		return emptyEnricherResult(), nil
	})
	c, core, rows := cappedEC2List(t)

	runEnrichRow(t, c, core, rows[0])

	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap, rows[1].ID: awsclient.CheckCap},
	})
	c.ApplyIntents(intents)

	set := core.EnrichmentTruncatedIDs("ec2")
	if got, still := set[rows[0].ID]; still {
		t.Errorf("a stale sweep put the cleanly answered row back at %q; the set is %v", got, set)
	}
	if set[rows[1].ID] != awsclient.CheckCap {
		t.Errorf("the row the sweep really did leave at its cap reads %q, want %q", set[rows[1].ID], awsclient.CheckCap)
	}
	c.Apply(app.Action{Kind: app.ActionBack})
	if got := statusCellFor(t, *c.Snapshot().Body.List, rows[0].ID); got != "running" {
		t.Errorf("after a stale sweep the cleanly answered row's Status cell reads %q, want its own state", got)
	}
	if got := statusCellFor(t, *c.Snapshot().Body.List, rows[1].ID); got != domain.NotInspectedPhrase {
		t.Errorf("the other capped row's Status cell reads %q, want %q", got, domain.NotInspectedPhrase)
	}
}

// Row 5 — the row an on-demand check answered for can be gone from the
// account by the time the sweep that was dispatched before it lands. There is
// no row left to restore the answer from, and the fold must neither resurrect
// it under the cap nor fail on its absence.
func TestOnDemandAnswer_AnsweredRowGoneBeforeTheStaleSweepLands(t *testing.T) {
	stubEC2Enricher(t, func(resources []resource.Resource) (awsclient.IssueEnricherResult, error) {
		res := emptyEnricherResult()
		for _, r := range resources {
			res.Findings[r.ID] = []domain.Finding{{Code: "ec2.impaired", Phrase: onDemandPhrase, Severity: domain.SevBroken, Source: "wave2:ec2"}}
		}
		return res, nil
	})
	c, core, rows := cappedEC2List(t)

	runEnrichRow(t, c, core, rows[0])
	c.Apply(app.Action{Kind: app.ActionBack})
	core.ObserveRows("ec2", rows[1:], &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	c.ApplyResourcesLoaded("ec2", rows[1:], &resource.PaginationMeta{IsTruncated: false}, false)

	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap, rows[1].ID: awsclient.CheckCap},
	})
	c.ApplyIntents(intents)

	set := core.EnrichmentTruncatedIDs("ec2")
	if got, still := set[rows[0].ID]; still {
		t.Errorf("the deleted row came back at %q; the set is %v", got, set)
	}
	if set[rows[1].ID] != awsclient.CheckCap {
		t.Errorf("the surviving capped row reads %q, want %q", set[rows[1].ID], awsclient.CheckCap)
	}
	if got, carried := c.GetListEnrichmentFindings("ec2")[rows[0].ID]; carried {
		t.Errorf("the deleted row carries %v in the list store", got)
	}
	body := *c.Snapshot().Body.List
	for _, row := range body.Rows {
		if row.ResourceID == rows[0].ID {
			t.Errorf("the deleted row is back in the list: %+v", row)
		}
	}
	if got := statusCellFor(t, body, rows[1].ID); got != domain.NotInspectedPhrase {
		t.Errorf("the surviving capped row's Status cell reads %q, want %q", got, domain.NotInspectedPhrase)
	}
}

// Row 6 — the web host runs against the clients its bootstrap handed it, so
// the region it was started with is the only one it has. `:region` says so
// rather than opening a selector that cannot switch anything.
func TestWebHost_RegionCommandInDemoModeIsRefused(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	core.SetPreSuppliedClients(demo.NewServiceClients())
	c.SetUIMode("web")

	vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "region"})

	if vs.Body.Kind == app.BodyKindSelector {
		t.Errorf("`:region` opened a selector on the web host: %+v", vs.Body.Selector)
	}
	if got := vs.Header.Flash; got.Text != "region switching is disabled in demo mode" || !got.IsError {
		t.Errorf("the flash is %+v, want the demo-mode refusal", got)
	}
}

// Row 7 — a clean on-demand answer is an answer: the findings an earlier
// sweep left on the row go, and its supporting rows go with them. Every other
// row's stay — the check ran for one row.
func TestOnDemandAnswer_CleanAnswerClearsOnlyThatRowsFindings(t *testing.T) {
	stubEC2Enricher(t, func([]resource.Resource) (awsclient.IssueEnricherResult, error) {
		return emptyEnricherResult(), nil
	})
	c, core, rows := cappedEC2List(t)
	const supportValue = "instance reachability failed"
	prior := []domain.Finding{{Code: "ec2.impaired", Phrase: onDemandPhrase, Severity: domain.SevBroken, Source: "wave2:ec2"}}
	details := map[string]map[domain.FindingCode]domain.AttentionDetail{
		rows[0].ID: {"ec2.impaired": {Rows: []domain.DetailRow{{Label: "Status check", Value: supportValue}}}},
		rows[1].ID: {"ec2.impaired": {Rows: []domain.DetailRow{{Label: "Status check", Value: supportValue}}}},
	}
	c.ApplyIntents([]runtime.UIIntent{runtime.PatchResourceList{
		ResourceType: "ec2",
		Enrichment: &runtime.ListEnrichmentPatch{
			Findings:         map[string][]domain.Finding{rows[0].ID: prior, rows[1].ID: prior},
			AttentionDetails: details,
			TruncatedIDs:     core.EnrichmentTruncatedIDs("ec2"),
		},
	}})
	if got := c.GetListEnrichmentFindings("ec2")[rows[0].ID]; len(got) != 1 {
		t.Fatalf("precondition: the store carries %v for the row, want its one earlier finding", got)
	}

	runEnrichRow(t, c, core, rows[0])

	store := c.GetListEnrichmentFindings("ec2")
	if got, ok := store[rows[0].ID]; ok {
		t.Errorf("a clean answer left %v in the store for the row it answered for", got)
	}
	if got := store[rows[1].ID]; len(got) != 1 {
		t.Errorf("one row's clean answer cleared another row's finding: store carries %v", got)
	}
	if entries := topAttentionEntries(t, c); strings.Contains(strings.Join(entries, " "), supportValue) {
		t.Errorf("the answered row's Attention block still reads %v, want its earlier supporting rows gone", entries)
	}
	c.Apply(app.Action{Kind: app.ActionBack})
	// With no finding left, the Status column falls back to the row's own
	// state — the cell a never-flagged running instance shows.
	if got := statusCellFor(t, *c.Snapshot().Body.List, rows[0].ID); got != "running" {
		t.Errorf("the answered row's Status cell reads %q, want its own state", got)
	}
	if got := statusCellFor(t, *c.Snapshot().Body.List, rows[1].ID); got != onDemandPhrase {
		t.Errorf("the other row's Status cell reads %q, want its own finding %q", got, onDemandPhrase)
	}
}

// Row 8 — the answer patches the menu badge from the rows this session holds.
// A page the fetcher truncated holds fewer rows than the account has, so the
// count it produces is a floor even once no row is left at the check cap.
func TestOnDemandAnswer_TruncatedPageKeepsTheBadgeALowerBound(t *testing.T) {
	stubEC2Enricher(t, func(resources []resource.Resource) (awsclient.IssueEnricherResult, error) {
		res := emptyEnricherResult()
		for _, r := range resources {
			res.Findings[r.ID] = []domain.Finding{{Code: "ec2.impaired", Phrase: onDemandPhrase, Severity: domain.SevBroken, Source: "wave2:ec2"}}
		}
		return res, nil
	})
	c, core := newTestControllerAndCore(t)
	core.Session().Clients = demo.NewServiceClients()
	seedFromDisk(c, onDemandProfile, onDemandRegion)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := uninspectedEC2Rows()
	c.ApplyResourcesLoaded("ec2", rows, &resource.PaginationMeta{IsTruncated: true}, false)
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, false)
	sweepSaveAfterEnrichment(t, core, c, messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap},
	})

	intents, _ := runEnrichRow(t, c, core, rows[0])

	if got := core.EnrichmentTruncatedIDs("ec2"); len(got) != 0 {
		t.Fatalf("precondition: rows are still at the check cap (%v), so the page's own truncation is not what the badge would read", got)
	}
	var patches []runtime.PatchMenu
	for _, in := range intents {
		if p, ok := in.(runtime.PatchMenu); ok {
			patches = append(patches, p)
		}
	}
	if len(patches) != 1 {
		t.Fatalf("the answer produced %d menu patches, want 1", len(patches))
	}
	if !patches[0].Truncated {
		t.Errorf("the menu patch is %+v, want Truncated true — the list's own page is truncated, so its issue count is a floor", patches[0])
	}
}

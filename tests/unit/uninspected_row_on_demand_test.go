// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// uninspected_row_on_demand_test.go — a row the sweep left at its inspection
// cap gets its Wave-2 checks when the operator opens its detail. Which rows
// fall inside the cap is the fetcher's page order, so the rows an operator
// sorted to the top can read "not inspected: stopped at the inspection cap"
// while off-screen rows were inspected; the row the operator is looking at
// must not stay there. The list cap is unchanged, and every other row keeps
// the sweep's answer.

import (
	"context"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

const onDemandPhrase = "system check failed"

// The pair newTestControllerAndCore's session carries.
const onDemandProfile, onDemandRegion = "demo", "us-east-1"

// onDemandEC2Enricher answers "system check failed" for every row it is
// asked about and records what it was asked.
func onDemandEC2Enricher(t *testing.T, asked *[][]string) {
	t.Helper()
	awsclient.SetWave2EnricherForTest(t, "ec2", awsclient.IssueEnricher{Fn: func(_ context.Context, _ *awsclient.ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		var ids []string
		res := awsclient.IssueEnricherResult{
			Findings:         map[string][]domain.Finding{},
			AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{},
			TruncatedIDs:     map[string]string{},
			FieldUpdates:     map[string]map[string]string{},
		}
		for _, r := range resources {
			ids = append(ids, r.ID)
			res.Findings[r.ID] = []domain.Finding{{Code: "ec2.impaired", Phrase: onDemandPhrase, Detail: "The instance failed its system status check.", Severity: domain.SevBroken, Source: "wave2:ec2"}}
		}
		*asked = append(*asked, ids)
		return res, nil
	}})
}

// cappedEC2List opens the ec2 list with two rows the sweep left at its cap.
func cappedEC2List(t *testing.T) (*app.Controller, *runtime.Core, []resource.Resource) {
	t.Helper()
	c, core := newTestControllerAndCore(t)
	core.Session().Clients = demo.NewServiceClients()
	seedFromDisk(c, onDemandProfile, onDemandRegion)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := uninspectedEC2Rows()
	c.ApplyResourcesLoaded("ec2", rows, nil, false)
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	sweepSaveAfterEnrichment(t, core, c, messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap, rows[1].ID: awsclient.CheckCap},
	})
	for _, r := range rows {
		if got := statusCellFor(t, *c.Snapshot().Body.List, r.ID); got != domain.NotInspectedPhrase {
			t.Fatalf("precondition: capped row %q Status cell = %q, want %q", r.ID, got, domain.NotInspectedPhrase)
		}
	}
	return c, core, rows
}

// openDetailWithWorkload opens row's detail the way navigation does and runs
// every KindEnrichRow task the detail operation dispatched through the real
// executor and handler.
func openDetailWithWorkload(t *testing.T, c *app.Controller, core *runtime.Core, row resource.Resource) int {
	t.Helper()
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: row.ID},
	}})
	c.EnsureDetailState(row, "ec2")
	_, tasks := core.BeginDetailOperation("ec2", row, false)
	ran := 0
	for _, task := range tasks {
		if task.Key.Kind != runtime.KindEnrichRow {
			continue
		}
		ev, err := core.ExecuteTask(context.Background(), task)
		if err != nil {
			t.Fatalf("KindEnrichRow: %v", err)
		}
		intents, followUps := core.HandleEvent(ev)
		c.ApplyIntents(intents)
		for _, f := range followUps {
			if f.Key.Kind == runtime.TaskKindSaveCache {
				if _, err := core.ExecuteTask(context.Background(), f); err != nil {
					t.Fatalf("save after the row answer: %v", err)
				}
			}
		}
		ran++
	}
	return ran
}

func TestCappedRow_DetailOpenRunsItsChecks(t *testing.T) {
	var asked [][]string
	onDemandEC2Enricher(t, &asked)
	c, core, rows := cappedEC2List(t)

	if ran := openDetailWithWorkload(t, c, core, rows[0]); ran != 1 {
		t.Fatalf("opening a capped row's detail dispatched %d KindEnrichRow tasks, want 1", ran)
	}
	if len(asked) != 1 || len(asked[0]) != 1 || asked[0][0] != rows[0].ID {
		t.Fatalf("the on-demand check asked the enricher about %v, want exactly [[%q]] — the list cap is unchanged", asked, rows[0].ID)
	}

	entries := topAttentionEntries(t, c)
	joined := strings.Join(entries, " ")
	if !strings.Contains(joined, onDemandPhrase) {
		t.Errorf("the opened row's Attention block reads %v, want the finding %q the on-demand check found", entries, onDemandPhrase)
	}
	if strings.Contains(strings.ToLower(joined), domain.NotInspectedPhrase) {
		t.Errorf("the opened row still reads %q after its checks ran: %v", domain.NotInspectedPhrase, entries)
	}

	set := core.EnrichmentTruncatedIDs("ec2")
	if _, still := set[rows[0].ID]; still {
		t.Errorf("the opened row is still in the session's uninspected set: %v", set)
	}
	if set[rows[1].ID] != awsclient.CheckCap {
		t.Errorf("the other capped row lost its mark: %v — one row's answer is no answer for the rest", set)
	}

	c.Apply(app.Action{Kind: app.ActionBack})
	body := c.Snapshot().Body.List
	if got := statusCellFor(t, *body, rows[0].ID); got != onDemandPhrase {
		t.Errorf("the opened row's Status cell = %q, want %q — the list and the detail must agree", got, onDemandPhrase)
	}
	if got := statusCellFor(t, *body, rows[1].ID); got != domain.NotInspectedPhrase {
		t.Errorf("the other capped row's Status cell = %q, want %q", got, domain.NotInspectedPhrase)
	}

	// The answer is on disk: the row's mark is gone and the other row's stays.
	if got := uninspectedOnDisk(t, onDemandProfile, onDemandRegion, rows[0].ID); got != nil {
		t.Errorf("the answered row still carries uninspected=%q on disk", *got)
	}
	if got := uninspectedOnDisk(t, onDemandProfile, onDemandRegion, rows[1].ID); got == nil || *got != awsclient.CheckCap {
		t.Errorf("the other capped row's mark on disk = %v, want %q", got, awsclient.CheckCap)
	}
	// The badge on disk is the answer's: one issue, still a lower bound
	// while the other row is uninspected.
	if issues, known, lower := issuesOnDisk(t, onDemandProfile, onDemandRegion); issues != 1 || !known || !lower {
		t.Errorf("the badge on disk is %d (known=%v, lower bound=%v), want 1 known and a lower bound", issues, known, lower)
	}

	// A sweep dispatched before the answer lands afterwards and reports the
	// row at its cap: it never looked at the row, so the answer stands.
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap, rows[1].ID: awsclient.CheckCap},
	})
	c.ApplyIntents(intents)
	if set := core.EnrichmentTruncatedIDs("ec2"); set[rows[0].ID] != "" || set[rows[1].ID] != awsclient.CheckCap {
		t.Errorf("after a stale sweep the uninspected set is %v, want only %q capped", set, rows[1].ID)
	}
	body = c.Snapshot().Body.List
	if got := statusCellFor(t, *body, rows[0].ID); got != onDemandPhrase {
		t.Errorf("after a stale sweep the answered row's Status cell = %q, want %q", got, onDemandPhrase)
	}
	if got := c.GetListEnrichmentFindings("ec2")[rows[0].ID]; len(got) != 1 {
		t.Errorf("after a stale sweep the store carries %v for the answered row, want its one finding", got)
	}

	// A global refresh forgets the answer with the marks: the next sweep's
	// cap is the row's state again.
	core.ResetEnrichmentMaps()
	intents, _ = core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap, rows[1].ID: awsclient.CheckCap},
	})
	c.ApplyIntents(intents)
	if set := core.EnrichmentTruncatedIDs("ec2"); set[rows[0].ID] != awsclient.CheckCap {
		t.Errorf("after a global refresh a sweep that capped the row left it answered: set is %v", set)
	}
}

// findingsOnDisk returns the codes the ec2 type file carries for the row.
func findingsOnDisk(t *testing.T, profile, region, id string) []string {
	t.Helper()
	tf, ok := cache.LoadDirForTest(profile, region).Type("ec2")
	if !ok {
		t.Fatal("no ec2 type file on disk")
	}
	for _, row := range tf.Rows {
		if row.ID != id {
			continue
		}
		var codes []string
		for _, f := range row.Findings {
			codes = append(codes, string(f.Code))
		}
		return codes
	}
	t.Fatalf("row %q is not in the ec2 type file", id)
	return nil
}

// One row's answer is saved as one row's: the other capped row keeps its
// mark and the finding an earlier sweep found, on disk as in memory.
func TestCappedRow_AnswerKeepsTheOtherRowsFindingOnDisk(t *testing.T) {
	var asked [][]string
	onDemandEC2Enricher(t, &asked)
	c, core := newTestControllerAndCore(t)
	core.Session().Clients = demo.NewServiceClients()
	seedFromDisk(c, onDemandProfile, onDemandRegion)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := uninspectedEC2Rows()
	c.ApplyResourcesLoaded("ec2", rows, nil, false)
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	prior := []domain.Finding{{Code: "ec2.stopped", Phrase: "stopped by the operator", Severity: domain.SevWarn, Source: "wave2:ec2"}}
	sweepSaveAfterEnrichment(t, core, c, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings:     map[string][]domain.Finding{rows[1].ID: prior},
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap},
	})
	sweepSaveAfterEnrichment(t, core, c, messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap, rows[1].ID: awsclient.CheckCap},
	})
	if got := findingsOnDisk(t, onDemandProfile, onDemandRegion, rows[1].ID); len(got) != 1 {
		t.Fatalf("precondition: the capped row's earlier finding is not on disk: %v", got)
	}

	openDetailWithWorkload(t, c, core, rows[0])
	if got := findingsOnDisk(t, onDemandProfile, onDemandRegion, rows[1].ID); len(got) != 1 {
		t.Errorf("another row's on-demand answer dropped this row's finding from disk: %v", got)
	}
	if got := uninspectedOnDisk(t, onDemandProfile, onDemandRegion, rows[1].ID); got == nil || *got != awsclient.CheckCap {
		t.Errorf("another row's on-demand answer moved this row's mark on disk to %v", got)
	}
}

// An answer dispatched before a refresh is not the one the refresh asked
// for: it lands nowhere.
func TestCappedRow_AnswerFromBeforeARefreshIsDropped(t *testing.T) {
	for name, refresh := range map[string]func(core *runtime.Core){
		"list refresh":   func(core *runtime.Core) { core.RefreshListEnrichment("ec2") },
		"global refresh": func(core *runtime.Core) { core.BumpEnrichmentGen(); core.ResetEnrichmentMaps() },
	} {
		t.Run(name, func(t *testing.T) {
			var asked [][]string
			onDemandEC2Enricher(t, &asked)
			c, core, rows := cappedEC2List(t)
			c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
				ID:      runtime.ScreenDetail,
				Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: rows[0].ID},
			}})
			c.EnsureDetailState(rows[0], "ec2")
			_, tasks := core.BeginDetailOperation("ec2", rows[0], false)
			var ev messages.Event
			for _, task := range tasks {
				if task.Key.Kind != runtime.KindEnrichRow {
					continue
				}
				var err error
				if ev, err = core.ExecuteTask(context.Background(), task); err != nil {
					t.Fatalf("KindEnrichRow: %v", err)
				}
			}
			if ev == nil {
				t.Fatal("the capped row's detail dispatched no on-demand check")
			}
			refresh(core)
			intents, tasks := core.HandleEvent(ev)
			c.ApplyIntents(intents)
			if len(intents) != 0 || len(tasks) != 0 {
				t.Errorf("a pre-refresh answer produced %d intents and %d tasks, want none", len(intents), len(tasks))
			}
			if got := c.GetListEnrichmentFindings("ec2")[rows[0].ID]; len(got) != 0 {
				t.Errorf("a pre-refresh answer landed its finding: %v", got)
			}
			// The next sweep, which caps the row again, is the truth.
			intents, _ = core.HandleEvent(messages.EnrichmentChecked{
				ResourceType: "ec2",
				TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap},
			})
			c.ApplyIntents(intents)
			if set := core.EnrichmentTruncatedIDs("ec2"); set[rows[0].ID] != awsclient.CheckCap {
				t.Errorf("a pre-refresh answer outlived the refresh: the uninspected set is %v", set)
			}
		})
	}
}

func TestCappedRow_AnswerLeavesTheOtherOpenDetailAlone(t *testing.T) {
	var asked [][]string
	onDemandEC2Enricher(t, &asked)
	c, core, rows := cappedEC2List(t)
	// The other capped row's detail is open underneath, carrying a finding
	// an earlier sweep found.
	prior := []domain.Finding{{Code: "ec2.stopped", Phrase: "stopped by the operator", Severity: domain.SevWarn, Source: "wave2:ec2"}}
	c.ApplyIntents([]runtime.UIIntent{runtime.PatchResourceList{
		ResourceType: "ec2",
		Enrichment:   &runtime.ListEnrichmentPatch{Findings: map[string][]domain.Finding{rows[1].ID: prior}, TruncatedIDs: core.EnrichmentTruncatedIDs("ec2")},
	}})
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: rows[1].ID},
	}})
	c.EnsureDetailState(rows[1], "ec2")
	c.ApplyIntents([]runtime.UIIntent{runtime.PatchDetail{ResourceType: "ec2", EnrichmentFindings: map[string][]domain.Finding{rows[1].ID: prior}}})
	if entries := topAttentionEntries(t, c); !strings.Contains(strings.Join(entries, " "), "stopped by the operator") {
		t.Fatalf("precondition: the underlying detail reads %v, want its finding", entries)
	}

	openDetailWithWorkload(t, c, core, rows[0])
	c.Apply(app.Action{Kind: app.ActionBack})
	if entries := topAttentionEntries(t, c); !strings.Contains(strings.Join(entries, " "), "stopped by the operator") {
		t.Errorf("one row's answer cleared the other open detail's finding: %v", entries)
	}
}

func TestCappedRow_OnDemandRefusalNamesTheCall(t *testing.T) {
	awsclient.SetWave2EnricherForTest(t, "ec2", awsclient.IssueEnricher{Fn: func(_ context.Context, _ *awsclient.ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		res := awsclient.IssueEnricherResult{
			Findings:         map[string][]domain.Finding{},
			AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{},
			TruncatedIDs:     map[string]string{},
			FieldUpdates:     map[string]map[string]string{},
		}
		var failures []awsclient.Failure
		for _, r := range resources {
			// The name was read before the call that was refused.
			res.FieldUpdates[r.ID] = map[string]string{"name": "web-server-renamed"}
			awsclient.MarkSkipped(&res, r.ID, &failures, uninspectedDenied(uninspectedCheck))
		}
		return res, nil
	}})
	c, core, rows := cappedEC2List(t)
	// The row carries a finding an earlier sweep found; a refused re-check
	// is no answer and must not fold it away.
	prior := []domain.Finding{{Code: "ec2.impaired", Phrase: onDemandPhrase, Severity: domain.SevBroken, Source: "wave2:ec2"}}
	c.ApplyIntents([]runtime.UIIntent{runtime.PatchResourceList{
		ResourceType: "ec2",
		Enrichment:   &runtime.ListEnrichmentPatch{Findings: map[string][]domain.Finding{rows[0].ID: prior}, TruncatedIDs: core.EnrichmentTruncatedIDs("ec2")},
	}})
	openDetailWithWorkload(t, c, core, rows[0])

	want := domain.NotInspectedPhrase + ": " + uninspectedCheck
	entries := topAttentionEntries(t, c)
	joined := strings.Join(entries, " ")
	if !strings.Contains(joined, want) {
		t.Errorf("a row whose on-demand check was refused reads %v, want %q — the refusal names the call, not the cap", entries, want)
	}
	if got := c.GetListEnrichmentFindings("ec2")[rows[0].ID]; len(got) != 1 {
		t.Errorf("a refused re-check folded the row's earlier finding away: store carries %v", got)
	}
	if !detailShowsValue(t, c, "web-server-renamed") {
		t.Errorf("the field the refused check did read never reached the open detail")
	}
	c.Apply(app.Action{Kind: app.ActionBack})
	if got := cellFor(t, *c.Snapshot().Body.List, rows[0].ID, "name"); got != "web-server-renamed" {
		t.Errorf("the field the refused check did read never reached the list: name cell = %q", got)
	}
	if got := uninspectedOnDisk(t, onDemandProfile, onDemandRegion, rows[0].ID); got == nil || *got != uninspectedCheck {
		t.Errorf("the refusal's mark on disk = %v, want %q — after a restart the row would be re-checked as if only capped", got, uninspectedCheck)
	}
}

// detailShowsValue reports whether the detail on top of the stack carries
// value in one of its fields.
func detailShowsValue(t *testing.T, c *app.Controller, value string) bool {
	t.Helper()
	detail := c.Snapshot().Body.Detail
	if detail == nil {
		t.Fatal("no detail body on the snapshot")
	}
	for _, f := range detail.Fields {
		if f.Value == value {
			return true
		}
	}
	return false
}

func TestSweepFieldUpdate_ReachesTheOpenDetail(t *testing.T) {
	c, core, rows := cappedEC2List(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: rows[1].ID},
	}})
	c.EnsureDetailState(rows[1], "ec2")
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap, rows[1].ID: awsclient.CheckCap},
		FieldUpdates: map[string]map[string]string{rows[1].ID: {"name": "api-server-renamed"}},
	})
	c.ApplyIntents(intents)
	if !detailShowsValue(t, c, "api-server-renamed") {
		t.Errorf("a sweep's field update never reached the open detail")
	}
}

// cellFor returns the cell under the column with the given key.
func cellFor(t *testing.T, body app.ListBody, id, key string) string {
	t.Helper()
	col := -1
	for i, c := range body.Columns {
		if c.Key == key {
			col = i
		}
	}
	if col < 0 {
		t.Fatalf("the list has no %q column; columns are %+v", key, body.Columns)
	}
	for _, row := range body.Rows {
		if row.ResourceID == id {
			return row.Cells[col]
		}
	}
	t.Fatalf("row %q not present in the list body", id)
	return ""
}

func TestRefusedRow_DetailOpenDoesNotRetry(t *testing.T) {
	var asked [][]string
	onDemandEC2Enricher(t, &asked)
	c, core := newTestControllerAndCore(t)
	core.Session().Clients = demo.NewServiceClients()
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := uninspectedEC2Rows()
	c.ApplyResourcesLoaded("ec2", rows, nil, false)
	core.ObserveRows("ec2", rows, nil, session.OriginFetch, false)
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: uninspectedCheck},
	})
	c.ApplyIntents(intents)

	if ran := openDetailWithWorkload(t, c, core, rows[0]); ran != 0 || len(asked) != 0 {
		t.Errorf("a row a call refused dispatched %d KindEnrichRow tasks (asked %v); only the cap is a9s's own bound to retry past", ran, asked)
	}
}

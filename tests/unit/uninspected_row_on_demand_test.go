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
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

const onDemandPhrase = "system check failed"

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
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := uninspectedEC2Rows()
	c.ApplyResourcesLoaded("ec2", rows, nil, false)
	core.ObserveRows("ec2", rows, nil, session.OriginFetch, false)
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap, rows[1].ID: awsclient.CheckCap},
	})
	c.ApplyIntents(intents)
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
		intents, _ := core.HandleEvent(ev)
		c.ApplyIntents(intents)
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
			awsclient.MarkSkipped(&res, r.ID, &failures, uninspectedDenied(uninspectedCheck))
		}
		return res, nil
	}})
	c, core, rows := cappedEC2List(t)
	openDetailWithWorkload(t, c, core, rows[0])

	want := domain.NotInspectedPhrase + ": " + uninspectedCheck
	if entries := topAttentionEntries(t, c); !strings.Contains(strings.Join(entries, " "), want) {
		t.Errorf("a row whose on-demand check was refused reads %v, want %q — the refusal names the call, not the cap", entries, want)
	}
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

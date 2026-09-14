// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// uninspected_row_restart_test.go — the "not inspected" mark outlives the
// session that recorded it. The row and its older findings come back from the
// type file after a restart, so the fact that the last sweep did not verify
// them comes back with them: a row whose check was refused must not read as
// verified until the next sweep.

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

// sweepSaveAfterEnrichment delivers one EnrichmentChecked through the real
// handler and executes the TaskKindSaveCache the completion dispatches — the
// sweep lane's own Wave-2-authoritative save, payload frozen at dispatch.
func sweepSaveAfterEnrichment(t *testing.T, core *runtime.Core, ctrl *app.Controller, ev messages.EnrichmentChecked) {
	t.Helper()
	intents, tasks := core.HandleEvent(ev)
	ctrl.ApplyIntents(intents)
	for _, task := range tasks {
		if task.Key.Kind != runtime.TaskKindSaveCache {
			continue
		}
		if _, err := core.ExecuteTask(context.Background(), task); err != nil {
			t.Fatalf("sweep save: %v", err)
		}
		return
	}
	t.Fatalf("the enrichment completion dispatched no TaskKindSaveCache; tasks were %+v", tasks)
}

// seedFromDisk delivers the AvailabilityCacheLoaded a real boot builds from
// the pair's directory (ExecuteTask(TaskKindLoadAvailCache)).
func seedFromDisk(ctrl *app.Controller, profile, region string) {
	ctrl.Handle(runtime.CacheStoreToEvent(cache.LoadDirForTest(profile, region)))
}

func uninspectedOnDisk(t *testing.T, profile, region, id string) *string {
	t.Helper()
	tf, ok := cache.LoadDirForTest(profile, region).Type("ec2")
	if !ok {
		t.Fatal("no ec2 type file on disk after the sweep save")
	}
	for _, row := range tf.Rows {
		if row.ID == id {
			return row.Uninspected
		}
	}
	t.Fatalf("row %q is not in the ec2 type file; rows were %+v", id, tf.Rows)
	return nil
}

// issuesOnDisk returns the ec2 type file's badge: count, whether it is
// known, and whether it is a lower bound.
func issuesOnDisk(t *testing.T, profile, region string) (int, bool, bool) {
	t.Helper()
	tf, ok := cache.LoadDirForTest(profile, region).Type("ec2")
	if !ok {
		t.Fatal("no ec2 type file on disk")
	}
	return tf.Issues, tf.IssuesKnown, tf.IssuesTruncated
}

func attentionEntries(t *testing.T, ctrl *app.Controller, row resource.Resource) []string {
	t.Helper()
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: row.ID},
	}})
	ctrl.EnsureDetailState(row, "ec2")
	return topAttentionEntries(t, ctrl)
}

// topAttentionEntries reads the Attention block of the detail on top of the
// stack.
func topAttentionEntries(t *testing.T, ctrl *app.Controller) []string {
	t.Helper()
	detail := ctrl.Snapshot().Body.Detail
	if detail == nil {
		t.Fatal("no detail body on the snapshot after EnsureDetailState")
	}
	var attention []string
	inBlock := false
	for _, f := range detail.Fields {
		switch {
		case f.IsSection && strings.HasPrefix(f.Key, "Attention"):
			inBlock = true
		case f.IsSection, inBlock && f.IsSpacer:
			inBlock = false
		case inBlock:
			attention = append(attention, f.Key+"="+f.Value)
		}
	}
	return attention
}

func TestUninspectedRow_MarkSurvivesRestart(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"
	rows := uninspectedEC2Rows()

	core, ctrl := newLiveWebStyleController(t, profile, region)
	seedFromDisk(ctrl, profile, region)
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	result := uninspectedResultFor(rows[0].ID)
	sweepSaveAfterEnrichment(t, core, ctrl, messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: result.TruncatedIDs,
		Findings:     result.Findings,
	})
	if got := uninspectedOnDisk(t, profile, region, rows[0].ID); got == nil || *got != uninspectedCheck {
		t.Fatalf("the refused row's type-file row carries uninspected=%v, want %q", got, uninspectedCheck)
	}
	if got := uninspectedOnDisk(t, profile, region, rows[1].ID); got != nil {
		t.Fatalf("the row the enricher answered for carries uninspected=%q on disk", *got)
	}

	// A save that carries no Wave-2 answer (the list lane after a fetch)
	// keeps the mark: it learned nothing about the check.
	if err := core.SaveTypeRows(
		runtime.SaveTarget{Pair: core.Pair(), Type: "ec2", ExactPopulation: true},
		runtime.SaveContent{Resources: rows, Count: len(rows)},
	); err != nil {
		t.Fatalf("list-lane save: %v", err)
	}
	if got := uninspectedOnDisk(t, profile, region, rows[0].ID); got == nil || *got != uninspectedCheck {
		t.Fatalf("a save with no Wave-2 answer dropped the mark: uninspected=%v", got)
	}

	// Restart: a new session on the same pair, seeded from disk only.
	core2, ctrl2 := newLiveWebStyleController(t, profile, region)
	seedFromDisk(ctrl2, profile, region)
	if got := core2.EnrichmentTruncatedIDs("ec2"); got[rows[0].ID] != uninspectedCheck || len(got) != 1 {
		t.Fatalf("after the restart the session's uninspected set is %v, want {%q: %q}", got, rows[0].ID, uninspectedCheck)
	}
	entries := attentionEntries(t, ctrl2, rows[0])
	want := domain.NotInspectedPhrase + ": " + uninspectedCheck
	if !strings.Contains(strings.Join(entries, " "), want) {
		t.Errorf("straight after a restart the refused row's Attention block reads %v, want an entry %q", entries, want)
	}
}

// A capped row answered on demand straight after a restart, before this
// session's sweep, still updates the badge the next session boots on.
func TestUninspectedRow_AnswerAfterRestartUpdatesTheBadgeOnDisk(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"
	rows := uninspectedEC2Rows()
	var asked [][]string
	onDemandEC2Enricher(t, &asked)

	core, ctrl := newLiveWebStyleController(t, profile, region)
	seedFromDisk(ctrl, profile, region)
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	sweepSaveAfterEnrichment(t, core, ctrl, messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap},
		Findings:     map[string][]domain.Finding{},
	})
	if issues, known, _ := issuesOnDisk(t, profile, region); issues != 0 || !known {
		t.Fatalf("precondition: the badge on disk is %d (known=%v), want a known zero", issues, known)
	}

	core2, ctrl2 := newLiveWebStyleController(t, profile, region)
	core2.Session().Clients = demo.NewServiceClients()
	seedFromDisk(ctrl2, profile, region)
	// Before the live list lands the row is the disk's copy, which carries
	// no RawStruct: no check runs on it, and the row stays at its cap.
	if ran := openDetailWithWorkload(t, ctrl2, core2, rows[0]); ran != 0 || len(asked) != 0 {
		t.Fatalf("a detail opened on a disk-seeded row ran %d checks (asked %v) before the live fetch landed", ran, asked)
	}
	if got := uninspectedOnDisk(t, profile, region, rows[0].ID); got == nil || *got != awsclient.CheckCap {
		t.Fatalf("a check that never ran moved the row's mark on disk to %v", got)
	}
	ctrl2.Apply(app.Action{Kind: app.ActionBack})
	core2.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	if ran := openDetailWithWorkload(t, ctrl2, core2, rows[0]); ran != 1 {
		t.Fatalf("the capped row's detail dispatched %d on-demand checks, want 1", ran)
	}
	if got := uninspectedOnDisk(t, profile, region, rows[0].ID); got != nil {
		t.Errorf("the answered row still carries uninspected=%q on disk", *got)
	}
	if issues, known, lower := issuesOnDisk(t, profile, region); issues != 1 || !known || lower {
		t.Errorf("the badge on disk is %d (known=%v, lower bound=%v) after the answer, want an exact known 1", issues, known, lower)
	}
}

func TestUninspectedRow_NextSweepThatAnswersClearsTheMark(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"
	rows := uninspectedEC2Rows()

	core, ctrl := newLiveWebStyleController(t, profile, region)
	seedFromDisk(ctrl, profile, region)
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	result := uninspectedResultFor(rows[0].ID)
	sweepSaveAfterEnrichment(t, core, ctrl, messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: result.TruncatedIDs,
		Findings:     result.Findings,
	})
	sweepSaveAfterEnrichment(t, core, ctrl, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings:     map[string][]domain.Finding{},
	})
	if got := uninspectedOnDisk(t, profile, region, rows[0].ID); got != nil {
		t.Fatalf("a sweep that answered for the row left uninspected=%q on disk", *got)
	}

	core2, ctrl2 := newLiveWebStyleController(t, profile, region)
	seedFromDisk(ctrl2, profile, region)
	if got := core2.EnrichmentTruncatedIDs("ec2"); len(got) != 0 {
		t.Fatalf("after the restart the session's uninspected set is %v, want empty", got)
	}
	for _, e := range attentionEntries(t, ctrl2, rows[0]) {
		if strings.Contains(strings.ToLower(e), domain.NotInspectedPhrase) {
			t.Errorf("a row the last sweep answered for reads %q after a restart", e)
		}
	}
}

func TestUninspectedRow_SweepOverAShallowerPageKeepsTheDeeperRowsMark(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "pilot-prof", "us-east-1"
	rows := uninspectedEC2Rows()

	core, ctrl := newLiveWebStyleController(t, profile, region)
	seedFromDisk(ctrl, profile, region)
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	result := uninspectedResultFor(rows[0].ID, rows[1].ID)
	sweepSaveAfterEnrichment(t, core, ctrl, messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: result.TruncatedIDs,
		Findings:     result.Findings,
	})

	// A later sweep submits a shallower truncated page holding only the
	// first row and answers for it; the file keeps the deeper row, which this
	// sweep never looked at.
	if err := core.SaveTypeRows(
		runtime.SaveTarget{Pair: core.Pair(), Type: "ec2", ExactPopulation: false},
		runtime.SaveContent{Resources: rows[:1], Count: 1, Wave2Authoritative: true, Uninspected: map[string]string{}},
	); err != nil {
		t.Fatalf("sweep save: %v", err)
	}
	if got := uninspectedOnDisk(t, profile, region, rows[0].ID); got != nil {
		t.Errorf("the row the sweep answered for still carries uninspected=%q", *got)
	}
	if got := uninspectedOnDisk(t, profile, region, rows[1].ID); got == nil || *got != uninspectedCheck {
		t.Errorf("the row the sweep never submitted lost its mark: uninspected=%v — it reads as verified after a restart", got)
	}
}

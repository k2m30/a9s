//go:build integration

package integration

import (
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// democount and the rendered list title state one count for a list whose
// first page is not the whole list.
func TestT12Scenario_DemocountPrintsTheListTitlesCount(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "run", "./cmd/democount")
	cmd.Dir = filepath.Join("..", "..")
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "AWS_REGION=") && !strings.HasPrefix(kv, "AWS_PROFILE=") && !strings.HasPrefix(kv, "AWS_DEFAULT_PROFILE=") {
			env = append(env, kv)
		}
	}
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go run ./cmd/democount: %v", err)
	}
	truncated := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, count, ok := strings.Cut(line, " ")
		digits, plus := strings.CutSuffix(count, "+")
		if !ok || !plus {
			continue
		}
		n, err := strconv.Atoi(digits)
		if err != nil {
			t.Fatalf("democount line %q is not <name> <count>", line)
		}
		truncated[name] = n
	}
	if len(truncated) == 0 {
		t.Fatalf("democount prints no lower-bound count, so no demo list is truncated:\n%s", out)
	}
	for _, name := range slices.Sorted(maps.Keys(truncated)) {
		s := fullIntegrationNewDemoScenario(t)
		s.OpenList(name)
		s.ExpectFrameContains(fullIntegrationFrameCount(name, fullIntegrationCountExpectation{count: truncated[name], truncated: true}))
	}
}

// t12BlockReason is the word each conditional Enter drill's flash must carry:
// the fact of the row that withholds the child view.
var t12BlockReason = map[string]string{
	"logs": "class",
	"sfn":  "Express",
}

func t12PressOn(s *fullIntegrationScenario, r resource.Resource, key string) (detailID, flash string) {
	s.t.Helper()
	s.ApplyFilter(r.ID)
	s.lastFlash = nil
	s.currentResource = nil
	s.Press(key)
	if s.currentResource != nil {
		detailID = s.currentResource.ID
	}
	if s.lastFlash != nil {
		flash = s.lastFlash.Text
	}
	return detailID, flash
}

// On a row whose child view is withheld, Enter says why, in words that name
// what withholds it and that no other type's block shares, and d still opens
// the row's detail.
func TestT12Scenario_EnterOnARowWithoutItsChildViewNamesTheReason(t *testing.T) {
	seen := map[string]string{}
	checked := 0
	for _, td := range resource.AllResourceTypes() {
		for _, ch := range td.Children {
			if ch.Key != "enter" || ch.DrillCondition == nil {
				continue
			}
			word, ok := t12BlockReason[td.ShortName]
			if !ok {
				t.Errorf("%s withholds %s on some rows; name the word its flash must carry in t12BlockReason", td.ShortName, ch.ChildType)
				continue
			}
			s := fullIntegrationNewDemoScenario(t)
			s.OpenList(td.ShortName)
			vetoed := slices.DeleteFunc(slices.Clone(s.currentListResources), func(r resource.Resource) bool { return ch.DrillCondition(domain.Resource(r)) })
			if len(vetoed) == 0 {
				t.Errorf("%s: no demo row for which %s is withheld", td.ShortName, ch.ChildType)
				continue
			}
			for _, r := range vetoed {
				checked++
				id, flash := t12PressOn(s, r, "enter")
				switch {
				case id != "":
					t.Errorf("Enter on %s/%s, which withholds %s, opened %q", td.ShortName, r.ID, ch.ChildType, id)
					s.Back()
				case !strings.Contains(flash, word):
					t.Errorf("Enter on %s/%s, which withholds %s: flash %q does not name %q", td.ShortName, r.ID, ch.ChildType, flash, word)
				}
				if other, dup := seen[flash]; dup && other != td.ShortName && flash != "" {
					t.Errorf("%s and %s withhold their child views with the same flash %q", other, td.ShortName, flash)
				}
				seen[flash] = td.ShortName
				s.lastFlash, s.currentResource = nil, nil
				s.Press("d")
				if s.currentResource == nil || s.currentResource.ID != r.ID {
					t.Errorf("d on %s/%s did not open its detail", td.ShortName, r.ID)
				}
				if s.currentResource != nil {
					s.Back()
				}
			}
		}
	}
	if checked < 4 {
		t.Fatalf("checked %d withheld rows; the demo lost the logs and sfn witnesses", checked)
	}
}

// Opening the detail of a dbi row the sweep left uninspected inspects it on
// demand. While that check runs, the detail says the row's posture is not
// known yet rather than showing a clean row; once it settles, the detail and
// the list state the same findings, and neither still says "not inspected"
// unless both do.
func TestT12Scenario_DBINotInspectedInTheListIsNotInspectedInTheDetail(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	clients := demo.NewServiceClients()
	m := tui.New(demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithNoCache(true))
	t.Cleanup(m.CloseController)
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "t12 Init")
	var cmd tea.Cmd
	m, cmd = fullIntegrationApplyMsg(m, initMsg)
	availMsg := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetched)
		return ok
	})
	var wave2Cmd tea.Cmd
	m, wave2Cmd = fullIntegrationApplyMsg(m, availMsg)
	m = drainWave2Enrichment(t, m, wave2Cmd)

	s := &fullIntegrationScenario{t: t, model: m, clients: clients, profile: demo.DemoProfile, region: demo.DemoRegion, lastRelatedByName: map[string]messages.RelatedCheckResult{}}
	s.OpenList("dbi")
	var marked []resource.Resource
	for _, r := range s.currentListResources {
		unique := !slices.ContainsFunc(s.currentListResources, func(o resource.Resource) bool { return o.ID != r.ID && strings.Contains(o.ID, r.ID) })
		if unique && strings.Contains(s.findRow(r.ID), domain.NotInspectedPhrase) {
			marked = append(marked, r)
		}
	}
	if len(marked) == 0 {
		t.Fatal("after the sweep no dbi row reads not inspected in the list")
	}
	for _, r := range marked {
		s.ApplyFilter(r.ID)
		s.currentResource = nil
		var pending []tea.Msg
		var follow tea.Cmd
		for _, msg := range fullIntegrationCollectCmdMessages(s.applyMsg(fullIntegrationScenarioKeyPress(t, "d"))) {
			if nav, ok := msg.(messages.Navigate); ok && nav.Target == messages.TargetDetail && follow == nil {
				follow = s.applyMsg(nav)
				continue
			}
			pending = append(pending, msg)
		}
		if s.currentResource == nil || s.currentResource.ID != r.ID {
			t.Errorf("dbi/%s: d opened no detail of it", r.ID)
			continue
		}
		if !strings.Contains(strings.ToLower(s.currentView()), domain.NotInspectedPhrase) {
			t.Errorf("dbi/%s: while its on-demand check runs, the detail does not say %q and reads as a clean row", r.ID, domain.NotInspectedPhrase)
		}
		s.drainCmd(follow)
		for _, msg := range pending {
			s.drainCmd(s.applyMsg(msg))
		}
		after := strings.ToLower(s.currentView())
		detailMarked := strings.Contains(after, domain.NotInspectedPhrase)
		s.Back()
		var row resource.Resource
		for _, lr := range s.model.ActiveListResources() {
			if lr.ID == r.ID {
				row = lr
			}
		}
		if row.ID == "" {
			t.Errorf("dbi/%s is not on the list after the detail closes", r.ID)
		}
		listMarked := strings.Contains(s.findRow(r.ID), domain.NotInspectedPhrase)
		if detailMarked != listMarked {
			t.Errorf("dbi/%s after its check: list says not inspected %v, detail says %v", r.ID, listMarked, detailMarked)
		}
		for _, f := range row.Findings {
			if f.Severity.IsIssue() && !strings.Contains(after, strings.ToLower(f.Phrase)) {
				t.Errorf("dbi/%s after its check: the list row carries %q, its detail does not show it", r.ID, f.Phrase)
			}
		}
	}
}

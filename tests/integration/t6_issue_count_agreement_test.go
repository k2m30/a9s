//go:build integration

package integration

import (
	"regexp"
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

func t6SettledDemo(t *testing.T) *fullIntegrationScenario {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	clients := demo.NewServiceClients()
	m := tui.New(demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithNoCache(true))
	t.Cleanup(m.CloseController)
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "t6 Init")
	var cmd tea.Cmd
	m, cmd = fullIntegrationApplyMsg(m, initMsg)
	availMsg := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetched)
		return ok
	})
	m, cmd = fullIntegrationApplyMsg(m, availMsg)
	m = drainWave2Enrichment(t, m, cmd)
	return &fullIntegrationScenario{
		t:                 t,
		model:             m,
		clients:           clients,
		profile:           demo.DemoProfile,
		region:            demo.DemoRegion,
		lastRelatedByName: make(map[string]messages.RelatedCheckResult),
	}
}

var t6AttentionTitle = regexp.MustCompile(`\((\d+)\+? of \d+\+?\)`)

func t6ListNumbers(t *testing.T, s *fullIntegrationScenario, shortName string) (titleIssues, filtered int, ok bool) {
	t.Helper()
	s.OpenList(shortName)
	if s.lastAPIError != nil {
		s.Back()
		return 0, 0, false
	}
	titleIssues, _, _ = s.ListTitleIssueCount()
	s.Press("ctrl+z")
	m := t6AttentionTitle.FindStringSubmatch(s.currentView())
	if m == nil {
		t.Fatalf("%s: the list title under ctrl+z shows no \"(F of T)\" count:\n%s", shortName, firstLines(s.currentView(), 6))
	}
	filtered, _ = strconv.Atoi(m[1])
	s.Press("ctrl+z")
	s.Back()
	return titleIssues, filtered, true
}

// The menu badge, the list title count and the rows ctrl+z keeps are one
// number for every type: the issues-only filter shows exactly the rows the
// count counts (docs/attention-signals.md: the filtered count "already is the
// issue count"). In ctrl+z mode the menu lists a type exactly when its badge
// has text: a counted number or a lower bound ("issues:N+", N = 0 included).
// An ExcludeFromIssueBadge type shows no badge by declaration.
func TestT6_MenuBadgeTitleCountAndIssuesOnlyRowsAgree(t *testing.T) {
	s := t6SettledDemo(t)
	menuView := s.currentView()
	menu := renderedMenuCounts(t, menuView)

	filteredByType := map[string]int{}
	for _, td := range resource.AllResourceTypes() {
		if td.ExcludeFromIssueBadge {
			continue
		}
		badge, shown := menu[td.ShortName]
		if !shown {
			continue
		}
		title, filtered, ok := t6ListNumbers(t, s, td.ShortName)
		if !ok {
			continue
		}
		filteredByType[td.ShortName] = filtered
		if badge.issues != title || title != filtered {
			t.Errorf("%s: menu badge %d, list title count %d, rows under ctrl+z %d — want one number", td.ShortName, badge.issues, title, filtered)
		}
	}
	if len(filteredByType) == 0 {
		t.Fatal("no demo type was compared")
	}

	s.Press("ctrl+z")
	attentionMenu := renderedMenuCounts(t, s.currentView())
	for short, filtered := range filteredByType {
		_, listed := attentionMenu[short]
		badged := regexp.MustCompile(regexp.QuoteMeta(resource.FindResourceType(short).Name) + ` \(\d+\+?\) issues:\d+`).MatchString(menuView)
		if listed != badged {
			t.Errorf("%s: menu in ctrl+z mode lists the type = %v, its badge has text = %v (its ctrl+z list has %d rows)", short, listed, badged, filtered)
		}
	}
}

// A queue whose only finding is the Wave-2 warning "no DLQ configured" is not
// counted by the list title, so ctrl+z on the sqs list does not show it.
func TestT6_SQSWarningOnlyQueueIsNotAnIssuesOnlyRow(t *testing.T) {
	s := t6SettledDemo(t)

	rows, _ := s.model.Core().ProbeResources("sqs")
	var witnesses []string
	for _, r := range rows {
		if len(r.Findings) == 0 {
			continue
		}
		onlyWarnings := true
		for _, f := range r.Findings {
			if f.Severity != domain.SevWarn || !f.IsWave2Sourced() {
				onlyWarnings = false
			}
		}
		if onlyWarnings {
			witnesses = append(witnesses, r.ID)
		}
	}
	if len(witnesses) == 0 {
		t.Fatal("the demo sqs list has no queue whose only findings are Wave-2 warnings; the test has no witness")
	}

	s.OpenList("sqs")
	title, _, _ := s.ListTitleIssueCount()
	s.Press("ctrl+z")
	m := t6AttentionTitle.FindStringSubmatch(s.currentView())
	if m == nil {
		t.Fatalf("the sqs title under ctrl+z shows no \"(F of T)\" count:\n%s", firstLines(s.currentView(), 6))
	}
	filtered, _ := strconv.Atoi(m[1])
	if filtered != title {
		t.Errorf("sqs: ctrl+z shows %d rows, the title counts %d issues", filtered, title)
	}
	for _, id := range witnesses {
		if line := s.findRow(id); line != "" {
			t.Errorf("sqs: ctrl+z shows %q, whose only findings are warnings the count does not count: %s", id, strings.TrimSpace(line))
		}
	}
}

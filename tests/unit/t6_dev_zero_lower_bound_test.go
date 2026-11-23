package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

type t6DevZeroBoundScreens struct {
	title, menu, issuesOnlyMenu string
}

func t6DevZeroBound(t *testing.T, shortName string) t6DevZeroBoundScreens {
	t.Helper()
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 220})
	rows := []resource.Resource{
		{ID: shortName + "-unread", Name: shortName + "-unread", Fields: map[string]string{}},
		{ID: shortName + "-read", Name: shortName + "-read", Fields: map[string]string{}},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: shortName})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: shortName, Resources: rows, Provenance: messages.FetchProvenanceCanonicalList})
	m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: shortName,
		TruncatedIDs: map[string]string{rows[0].ID: "GetResourcePolicy"},
		Findings:     map[string][]domain.Finding{},
	})
	var s t6DevZeroBoundScreens
	s.title = t6DevLineWith(t, stripANSI(rootViewContent(m)), shortName+"(")
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	s.menu = stripANSI(rootViewContent(m))
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	s.issuesOnlyMenu = stripANSI(rootViewContent(m))
	return s
}

func t6DevLineWith(t *testing.T, view, sub string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, sub) {
			return line
		}
	}
	t.Fatalf("no line contains %q:\n%s", sub, view)
	return ""
}

// A count of zero over a row nobody inspected, on a type that can raise a
// "!" Wave-2 finding, is a lower bound: the menu (both modes) and the list
// title say "0+" in the style "N+" has, so the issues-only menu shows why it
// lists the type.
func TestT6Dev_ZeroLowerBoundIsRendered(t *testing.T) {
	const short = "sqs"
	td := resource.FindResourceType(short)
	if td == nil || !declaresBrokenWave2(t, short) {
		t.Fatalf("precondition: %s is not a type that declares a \"!\" Wave-2 finding", short)
	}
	s := t6DevZeroBound(t, short)
	if !strings.Contains(s.title, "sqs(2) !0+") {
		t.Errorf("list title %q, want it to carry the lower bound \"!0+\"", strings.TrimSpace(s.title))
	}
	for mode, view := range map[string]string{"menu": s.menu, "issues-only menu": s.issuesOnlyMenu} {
		if line := t6DevLineWith(t, view, td.Name+" ("); !strings.Contains(line, "issues:0+") {
			t.Errorf("%s line %q, want the lower bound \"issues:0+\"", mode, strings.Join(strings.Fields(line), " "))
		}
	}
}

// On a type that can raise only "~" Wave-2 findings the same marked row says
// nothing about the issue count, which stays an exact zero: no badge, no
// title suffix, and no place in the issues-only menu.
func TestT6Dev_ExactZeroIsNotRendered(t *testing.T) {
	const short = "waf"
	td := resource.FindResourceType(short)
	if td == nil || declaresBrokenWave2(t, short) {
		t.Fatalf("precondition: %s is not a \"~\"-only type", short)
	}
	s := t6DevZeroBound(t, short)
	if strings.Contains(s.title, "!") && !strings.Contains(s.title, "[!]") {
		t.Errorf("list title %q carries an issue suffix for an exact zero", strings.TrimSpace(s.title))
	}
	if line := t6DevLineWith(t, s.menu, td.Name+" ("); strings.Contains(line, "issues:") {
		t.Errorf("menu line %q carries an issue badge for an exact zero", strings.Join(strings.Fields(line), " "))
	}
	if strings.Contains(s.issuesOnlyMenu, td.Name+" (") {
		t.Errorf("%s is listed in the issues-only menu with an exact zero issue count", short)
	}
}

// A badge loaded from the previous session's file — zero issues, a lower
// bound — is the type's badge until a Wave 2 answer replaces it, and the list
// title reads the same fact the menu does.
func TestT6Dev_DiskSeededZeroLowerBoundReachesTheTitle(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 220})
	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries:        map[string]int{"sqs": 2},
		Truncated:      map[string]bool{},
		IssueCounts:    map[string]int{"sqs": 0},
		IssueTruncated: map[string]bool{"sqs": true},
		IssueKnown:     map[string]bool{"sqs": true},
	})
	if line := t6DevLineWith(t, stripANSI(rootViewContent(m)), "SQS Queues ("); !strings.Contains(line, "issues:0+") {
		t.Fatalf("precondition: menu line %q, want the seeded \"issues:0+\"", strings.Join(strings.Fields(line), " "))
	}
	rows := []resource.Resource{
		{ID: "https://sqs.us-east-1.amazonaws.com/123456789012/acme-orders", Name: "acme-orders", Fields: map[string]string{}},
		{ID: "https://sqs.us-east-1.amazonaws.com/123456789012/acme-billing", Name: "acme-billing", Fields: map[string]string{}},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "sqs"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "sqs", Resources: rows, Provenance: messages.FetchProvenanceCanonicalList})
	if title := t6DevLineWith(t, stripANSI(rootViewContent(m)), "sqs("); !strings.Contains(title, "sqs(2) !0+") {
		t.Errorf("list title %q, want the menu's lower bound \"!0+\"", strings.TrimSpace(title))
	}
}

// A live Wave 2 answer with a broken finding, on rows that carried none, puts
// the row into the title's count and into the issues-only list in the frame
// that answer produces: the fold lands the finding on the row before the
// frame is built.
func TestT6Dev_LiveBrokenAnswerReachesTitleAndFilterTogether(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 220})
	rows := []resource.Resource{
		{ID: "i-0abc1111aaa111111", Name: "web-server-1", Fields: map[string]string{"state": "running"}},
		{ID: "i-0abc2222bbb222222", Name: "web-server-2", Fields: map[string]string{"state": "running"}},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: rows, Provenance: messages.FetchProvenanceCanonicalList})
	if title := t6DevLineWith(t, stripANSI(rootViewContent(m)), "ec2("); strings.Contains(title, " !") {
		t.Fatalf("precondition: title %q counts an issue before any answer", strings.TrimSpace(title))
	}
	m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings: map[string][]domain.Finding{
			rows[0].ID: {{Code: "ec2.system-status-impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
	})
	if title := t6DevLineWith(t, stripANSI(rootViewContent(m)), "ec2("); !strings.Contains(title, "ec2(2) !1") {
		t.Errorf("title after the answer %q, want \"ec2(2) !1\"", strings.TrimSpace(title))
	}
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
	filtered := stripANSI(rootViewContent(m))
	if title := t6DevLineWith(t, filtered, "ec2("); !strings.Contains(title, "ec2(1 of 2)") {
		t.Errorf("issues-only title %q, want \"ec2(1 of 2)\"", strings.TrimSpace(title))
	}
	if !strings.Contains(filtered, rows[0].Name) || strings.Contains(filtered, rows[1].Name) {
		t.Errorf("issues-only list does not show exactly %s:\n%s", rows[0].Name, filtered)
	}
}

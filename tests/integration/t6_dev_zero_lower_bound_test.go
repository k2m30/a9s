//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"unicode"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// The demo fixtures cannot hold a lower bound over zero counted rows: every
// type that can raise a "!" Wave-2 finding keeps an issue witness, so the
// state comes from the s3 answer alone.
func TestT6Dev_DemoZeroLowerBoundIsRendered(t *testing.T) {
	awsclient.SetWave2EnricherForTest(t, "s3", awsclient.IssueEnricher{Fn: func(_ context.Context, _ *awsclient.ServiceClients, rows []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		res := awsclient.IssueEnricherResult{TruncatedIDs: map[string]string{}}
		if len(rows) > 0 {
			res.TruncatedIDs[rows[0].ID] = "GetBucketPolicy"
		}
		return res, nil
	}})
	s := t6SettledDemo(t)
	name := resource.FindResourceType("s3").Name

	menuLine := func() string {
		for _, line := range strings.Split(s.currentView(), "\n") {
			if strings.Contains(line, name+" (") {
				return strings.Join(strings.Fields(line), " ")
			}
		}
		return ""
	}
	if line := menuLine(); !strings.Contains(line, "issues:0+") {
		t.Errorf("menu line %q, want the lower bound \"issues:0+\"", line)
	}
	s.Press("ctrl+z")
	if line := menuLine(); !strings.Contains(line, "issues:0+") {
		t.Errorf("issues-only menu line %q, want s3 listed with \"issues:0+\"", line)
	}
	s.Press("ctrl+z")

	s.OpenList("s3")
	if n, truncated, ok := s.ListTitleIssueCount(); !ok || n != 0 || !truncated {
		t.Errorf("s3 list title issue suffix = (%d, lower bound %v, present %v), want \"!0+\"\n%s", n, truncated, ok, firstLines(s.currentView(), 3))
	}
}

func t6DevMenuEntries(view string) map[string]string {
	out := map[string]string{}
	for _, rt := range resource.AllResourceTypes() {
		for _, line := range strings.Split(view, "\n") {
			idx := strings.Index(line, rt.Name+" (")
			if idx == -1 || (idx > 0 && unicode.IsLetter(rune(line[idx-1]))) {
				continue
			}
			entry, _, _ := strings.Cut(line[idx:], "│")
			out[rt.ShortName] = strings.Join(strings.Fields(entry), " ")
		}
	}
	return out
}

// The issues-only menu lists exactly the types whose badge has text, so each
// listed type shows why it is there. CloudTrail events draw no badge and are
// not listed, while their own issues-only list keeps its rows.
func TestT6Dev_IssuesOnlyMenuListsExactlyTheBadgedTypes(t *testing.T) {
	s := t6SettledDemo(t)
	badged := map[string]bool{}
	for short, entry := range t6DevMenuEntries(s.currentView()) {
		badged[short] = strings.Contains(entry, " issues:")
	}
	s.Press("ctrl+z")
	listed := t6DevMenuEntries(s.currentView())
	for short, entry := range listed {
		if !badged[short] {
			t.Errorf("the issues-only menu lists %s with no badge: %q", short, entry)
		}
	}
	for short, has := range badged {
		if _, ok := listed[short]; has && !ok {
			t.Errorf("%s has a badge and the issues-only menu does not list it", short)
		}
	}
	if _, ok := listed["ct-events"]; ok {
		t.Error("the issues-only menu lists CloudTrail Events, which draws no badge")
	}
	s.Press("ctrl+z")

	s.OpenList("ct-events")
	s.Press("ctrl+z")
	m := t6AttentionTitle.FindStringSubmatch(s.currentView())
	if m == nil || m[1] == "0" {
		t.Errorf("ct-events issues-only list keeps no rows:\n%s", firstLines(s.currentView(), 4))
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

//go:build integration

// scenario_cached_menu_badge_test.go — the badge the two writers agree on.
//
// A type's main-menu issue badge has two writers: the list-open lane, which
// records what the list the operator just read showed, and the probe lane,
// which records what the background sweep found without any list being opened.
// They write the same fact, and nothing rendered them side by side, because
// every scenario until now ran with the cache off — which also switches the
// probe lane off. These drive the app as an installation runs it, with the
// cache on in a home of the test's own, and read the badge off the rendered
// menu.
package integration

import (
	"testing"
)

// cachedEC2IssueCount is what the ec2 list itself says its issue count is,
// read off a scenario of its own so the number a probe-lane assertion compares
// against is never one this file wrote down. A hard-coded count would have to
// be re-derived every time a fixture changes, and would pass while both lanes
// were equally wrong.
func cachedEC2IssueCount(t *testing.T) int {
	t.Helper()
	s := fullIntegrationNewDemoScenarioWithCache(t)
	s.OpenList("ec2")
	n, truncated, ok := s.ListTitleIssueCount()
	if !ok {
		t.Fatalf("the open ec2 list renders no issue count in its title, so this file has no badge to follow:\n%s", s.currentView())
	}
	if truncated {
		t.Fatalf("the ec2 list's issue count is a lower bound (!%d+); the demo fixtures answer for every row, so this is not the fixture set these scenarios read", n)
	}
	if n == 0 {
		t.Fatal("the ec2 demo fixtures carry no issues, so a badge assertion about them proves nothing")
	}
	return n
}

// TestCachedScenario_MenuBadgeFollowsAFullListLoad pins the list-open lane's
// half: the operator opens a list, reads the issue count in its title, and
// goes back to the menu. The badge beside the type is the number the list just
// showed.
func TestCachedScenario_MenuBadgeFollowsAFullListLoad(t *testing.T) {
	want := cachedEC2IssueCount(t)

	s := fullIntegrationNewDemoScenarioWithCache(t)
	s.OpenList("ec2")
	s.Back()
	s.ExpectMenuIssueCount("ec2", want)
}

// TestCachedScenario_MenuBadgeFollowsTheProbeLane pins the other writer, and
// it is the one that had no witness. The operator refreshes the main menu,
// which restarts the availability sweep; the sweep inspects every type without
// any list being opened, and the badge it writes is the same fact the list
// would have shown. A menu that renders no badge at all after the sweep is the
// operator being told a type has no issues because nobody looked.
func TestCachedScenario_MenuBadgeFollowsTheProbeLane(t *testing.T) {
	want := cachedEC2IssueCount(t)

	s := fullIntegrationNewDemoScenarioWithCache(t)
	s.Press("ctrl+r")
	s.ExpectMenuIssueCount("ec2", want)
}

// TestCachedScenario_TheTwoLanesWriteTheSameBadge is the disagreement pin: one
// session, both writers, in the order an operator produces them. The account
// did not change between the two writes, so a badge that moves is two lanes
// disagreeing about one fact.
func TestCachedScenario_TheTwoLanesWriteTheSameBadge(t *testing.T) {
	want := cachedEC2IssueCount(t)

	s := fullIntegrationNewDemoScenarioWithCache(t)
	s.OpenList("ec2")
	s.Back()
	s.ExpectMenuIssueCount("ec2", want)

	s.Press("ctrl+r")
	s.ExpectMenuIssueCount("ec2", want)
}

// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_pair_generation_test.go pins row 52: the queued-save guard compares
// the visit unconditionally. A generation of zero is not a pass — it is a pair
// value captured before the session resolved one, and after two switches it
// answers for no visit at all.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// wipfixSaveRuns reports whether the guard let a save prepared for p run.
func wipfixSaveRuns(t *testing.T, s *session.Session, p session.Pair) bool {
	t.Helper()
	ran := false
	if err := s.WithCacheStoreSave(p, func(*cache.Store) ([]cache.WritePlan, error) {
		ran = true
		return nil, nil
	}); err != nil {
		t.Fatalf("the guard returned an error rather than deciding: %v", err)
	}
	return ran
}

// TestQueuedSave_CapturedBeforeThePairResolved pins row 52. The pair value is
// taken while the session still carries the names it was constructed with and
// has entered no visit, so it carries generation zero. The operator then
// switches twice, ending on those same names. A guard that treats zero as
// "nothing to check" lets that save write into a visit it was never prepared
// for — the same defect row 41 closed for a named round trip, reached through
// the escape hatch instead of past the comparison.
func TestQueuedSave_CapturedBeforeThePairResolved(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "us-east-1"

	beforeAnyVisit := s.CurrentPairValue()
	if beforeAnyVisit.Gen != 0 {
		t.Fatalf("precondition: a pair captured before any visit carries generation %d, want 0",
			beforeAnyVisit.Gen)
	}

	s.SetProfileRegion("example-other", "eu-west-1")
	s.SetProfileRegion("example-readonly", "us-east-1")

	now := s.CurrentPairValue()
	if beforeAnyVisit.Profile != now.Profile || beforeAnyVisit.Region != now.Region {
		t.Fatalf("precondition: the captured pair and the current one differ by name")
	}

	ran := false
	if err := s.WithCacheStoreSave(beforeAnyVisit, func(*cache.Store) ([]cache.WritePlan, error) {
		ran = true
		return nil, nil
	}); err != nil {
		t.Fatalf("the guard returned an error rather than declining: %v", err)
	}
	if ran {
		t.Error("a save captured before the session resolved a pair ran after two switches — " +
			"generation zero is a pair that answers for no visit, not a pair that skips the check")
	}
}

// TestQueuedSave_FirstResolvedVisitIsGenerationOne is row 52's other half: the
// first visit has a generation of its own, so a save prepared in it is told
// apart from one prepared before it.
func TestQueuedSave_FirstResolvedVisitIsGenerationOne(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()

	s.SetProfileRegion("example-readonly", "us-east-1")
	first := s.CurrentPairValue()
	if first.Gen != 1 {
		t.Errorf("the first resolved visit carries generation %d, want 1", first.Gen)
	}

	ran := false
	if err := s.WithCacheStoreSave(first, func(*cache.Store) ([]cache.WritePlan, error) {
		ran = true
		return nil, nil
	}); err != nil {
		t.Fatalf("the current visit's own save returned an error: %v", err)
	}
	if !ran {
		t.Error("a save prepared in the visit that is still current was declined")
	}
}

// TestPairVisit_EveryPathThatEstablishesAPairEntersOne is the other half of
// "there is no unset generation": a session that boots straight onto a pair,
// and one whose pair is resolved by the load lane, are both on a numbered
// visit. Left at zero, the visit the session booted into would be the same
// value as having entered none, and the guard would be comparing one answer
// against two meanings again.
func TestPairVisit_EveryPathThatEstablishesAPairEntersOne(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	booted := runtime.Bootstrap("example-readonly", "us-east-1", nil)
	if p := booted.Pair(); p.Gen == 0 {
		t.Error("a session that booted onto a pair carries generation 0 — its visit cannot be " +
			"told from having entered none")
	}

	s := session.New()
	beforeResolve := s.CurrentPairValue()
	s.ResolvePair("example-readonly", "us-east-1")
	if got := s.CurrentPairValue(); got.Gen != 1 {
		t.Errorf("resolving the pair left generation %d, want 1", got.Gen)
	}
	if wipfixSaveRuns(t, s, beforeResolve) {
		t.Error("a save prepared before the pair resolved ran after it did")
	}
}

// TestPairVisit_ResolvingTheSamePairAgainIsNotANewVisit guards the rule from
// the other side. The load lane calls ResolvePair more than once, and a visit
// opened on every call would retire saves the operator is still waiting on
// without anything having changed.
func TestPairVisit_ResolvingTheSamePairAgainIsNotANewVisit(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.ResolvePair("example-readonly", "us-east-1")
	first := s.CurrentPairValue()

	s.ResolvePair("example-readonly", "us-east-1")
	// Both halves are resolved, so this one changes nothing either.
	s.ResolvePair("example-other", "eu-west-1")

	if got := s.CurrentPairValue(); got.Gen != first.Gen {
		t.Errorf("re-resolving an already-resolved pair moved the generation %d -> %d — a visit "+
			"opened without a change retires saves nothing has superseded", first.Gen, got.Gen)
	}
	if !wipfixSaveRuns(t, s, first) {
		t.Error("a save prepared in the current visit was declined after a repeat resolve")
	}
}

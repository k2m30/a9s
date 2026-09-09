// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// cache2_pins_test.go pins the cache2 round: the pair guard's unresolved
// window, the single producer of the availability file, the zero WritePlan,
// and the alias-keyed scan-health guard. Synthetic identifiers only.
package unit_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// ---------------------------------------------------------------------------
// Row 1 — the pair guard must have no unresolved window.
// ---------------------------------------------------------------------------

// TestCache2_LoadAvailabilityCache_StampsResolvedRegion_StaleLoadRejected pins
// the C9 pair guard's pre-connect hole (handlers_availability.go): the guard
// compares a stamped load against the session pair only when the SESSION pair
// is itself fully resolved, so in the window before connect settles — where
// the load lane has already resolved a region from local config but never
// wrote it back — a load stamped with a DIFFERENT pair is applied instead of
// discarded.
//
// The contract: the load lane is the session's region resolution point. Once
// LoadAvailabilityCache has run, the session's own pair is resolved, so the
// guard always has a fact to compare against and a foreign-pair load is
// rejected whenever it lands.
func TestCache2_LoadAvailabilityCache_StampsResolvedRegion_StaleLoadRejected(t *testing.T) {
	t.Setenv("AWS_REGION", "eu-north-1") // deterministic GetDefaultRegion
	root := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", root)

	s := session.New()
	s.Profile = "cache2-prof"
	s.Region = "" // pre-connect: region not resolved yet
	core := runtime.New(s, nil)

	_ = core.LoadAvailabilityCache()

	profile, region := s.CurrentPair()
	if region == "" {
		t.Fatalf("after LoadAvailabilityCache the session pair is (%q, %q) — the load lane resolved a "+
			"region to read the cache from but never stamped it, leaving the C9 pair guard with an "+
			"unresolved window it silently skips", profile, region)
	}

	// A load answering for a DIFFERENT pair must be discarded whole.
	intents, _ := core.HandleEvent(messages.AvailabilityCacheLoaded{
		Profile: "cache2-other-prof",
		Region:  "eu-north-1",
		Entries: map[string]int{"ec2": 4},
	})
	if len(intents) != 0 {
		t.Errorf("a load stamped for pair cache2-other-prof--eu-north-1 produced %d intents against "+
			"session pair %s--%s, want 0 (C9: a result for a pair the operator is not on is dropped whole)",
			len(intents), profile, region)
	}
}

// TestCache2_PreConnectLoadForOwnResolvedPair_StillSeeds keeps the D10/D11
// pre-connect seed alive: the load the lane itself dispatched, stamped with
// the pair it resolved, must still reach the menu.
func TestCache2_PreConnectLoadForOwnResolvedPair_StillSeeds(t *testing.T) {
	t.Setenv("AWS_REGION", "eu-north-1")
	root := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", root)

	s := session.New()
	s.Profile = "cache2-prof"
	s.Region = ""
	core := runtime.New(s, nil)
	_ = core.LoadAvailabilityCache()

	profile, region := s.CurrentPair()
	intents, _ := core.HandleEvent(messages.AvailabilityCacheLoaded{
		Profile: profile,
		Region:  region,
		Entries: map[string]int{"ec2": 4},
	})
	if len(intents) == 0 {
		t.Errorf("the pre-connect load for the lane's own resolved pair %s--%s produced no intents — "+
			"the first frame would render empty (D10/D11)", profile, region)
	}
}

// ---------------------------------------------------------------------------
// Row 2 — a save prepared for a pair the session has left never reaches disk.
// ---------------------------------------------------------------------------

// TestCache2_SaveForLeftPair_RefusedAtChokepoint is why per-pair coalescing in
// the availability writer cannot be observed: whichever payload survives the
// queue, WithCacheStoreSave refuses any save whose pair is no longer the
// session's (C9, session.go). A queued pair-A snapshot is therefore dropped at
// the chokepoint regardless of how the queue coalesced, so widening the queue
// to one slot per pair changes nothing that reaches disk.
func TestCache2_SaveForLeftPair_RefusedAtChokepoint(t *testing.T) {
	_, core := newTestControllerAndCore(t)
	pairA := core.Pair()

	// The operator switches to pair B before the queued A save runs.
	core.Session().SetProfileRegion("cache2-other-prof", "us-east-1")

	if err := core.SaveAvailabilityCache(pairA,
		map[string]int{"ec2": 7}, map[string]bool{}, map[string]int{}, map[string]bool{}, map[string]bool{},
	); err != nil {
		t.Fatalf("SaveAvailabilityCache for the left pair: %v", err)
	}

	aFile := filepath.Join(cache.Root(), pairA.Profile+"--"+pairA.Region, "ec2.yaml")
	if _, err := os.Stat(aFile); err == nil {
		t.Errorf("a save prepared for pair %s--%s landed at %s after the session moved to "+
			"cache2-other-prof--us-east-1 — C9 requires the chokepoint to refuse it",
			pairA.Profile, pairA.Region, aFile)
	}
}

// ---------------------------------------------------------------------------
// Row 3 — one producer of the availability file.
// ---------------------------------------------------------------------------

// TestCache2_MenuAvailabilityPersist_DerivesFromRowStore: the menu-badge
// persistence lane derives every type's count from RowStore, the session's
// single source of truth for observed rows, not from a frozen clone of
// MenuState's five maps; the two disagree whenever RowStore has learned
// something the menu snapshot predates.
// has learned something the menu snapshot predates.
//
// Here s3 is seeded from disk at a truncated 50, then observed live as an exact
// 3. The menu still holds 50. Whatever lands in s3.yaml must be RowStore's 3.
func TestCache2_MenuAvailabilityPersist_DerivesFromRowStore(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	pair := core.Pair()

	// Menu learns a truncated s3 count of 50 from the disk-cache seed.
	c.Handle(messages.AvailabilityCacheLoaded{
		Profile:   pair.Profile,
		Region:    pair.Region,
		Entries:   map[string]int{"s3": 50},
		Truncated: map[string]bool{"s3": true},
	})

	// RowStore then observes s3 live and exact: three buckets, no more pages.
	core.ObserveRows("s3", []resource.Resource{
		{ID: "bucket-alpha", Name: "bucket-alpha", Type: "s3"},
		{ID: "bucket-beta", Name: "bucket-beta", Type: "s3"},
		{ID: "bucket-gamma", Name: "bucket-gamma", Type: "s3"},
	}, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	// An unrelated top-level list load drives the menu persistence lane.
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-cache2aaaa1111", Name: "web-1", Type: "ec2"},
	}, nil, false)

	c.Close() // flushes the availability writer

	store := cache.LoadDirIn(cache.Root(), pair.Profile, pair.Region)
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatalf("no s3.yaml written under %s — the availability lane persisted nothing", cache.Root())
	}
	if tf.Count != 3 || !tf.Exact {
		t.Errorf("persisted s3 = {Count:%d Exact:%t}, want {Count:3 Exact:true} — the availability "+
			"file must be produced from RowStore (the observed truth), not from a frozen MenuState "+
			"snapshot still holding the stale truncated 50", tf.Count, tf.Exact)
	}
}

// ---------------------------------------------------------------------------
// Row 4 — the zero WritePlan is unusable by construction.
// ---------------------------------------------------------------------------

// TestCache2_ZeroWritePlan_RefusedByName pins that committing a hand-built
// zero WritePlan fails naming the plan, rather than surfacing as an
// unattributable MkdirAll error on the empty path.
func TestCache2_ZeroWritePlan_RefusedByName(t *testing.T) {
	root := t.TempDir()
	store := cache.LoadDirIn(filepath.Join(root, "cache"), "cache2-prof", "us-east-1")

	err := store.CommitSave(cache.WritePlan{})
	if err == nil {
		t.Fatal("CommitSave(cache.WritePlan{}) returned nil — a zero plan carries no target and must be refused")
	}
	if !strings.Contains(err.Error(), "WritePlan") {
		t.Errorf("CommitSave(cache.WritePlan{}) = %q, want an error naming WritePlan and PrepareSave — "+
			"a bare-path MkdirAll failure does not tell the reader the plan was never prepared", err)
	}
}

// ---------------------------------------------------------------------------
// Row 5 — the scan-health guard is keyed on the canonical short name.
// ---------------------------------------------------------------------------

// TestCache2_ScanHealthGuard_AliasAndCanonicalAreOneKey pins that a probe
// failure delivered under a type's registry alias ("rds") and under its
// canonical short name ("dbi") is one entry, not two: the per-sweep
// scan-health guard keys on the canonical name, so a redundant delivery under
// the other spelling cannot re-flash the banner.
func TestCache2_ScanHealthGuard_AliasAndCanonicalAreOneKey(t *testing.T) {
	_, core := newTestControllerAndCore(t)
	s := core.Session()
	s.AvailTotal = 4 // keep the sweep mid-flight so completion never runs

	probeErr := errors.New("cache2 synthetic probe failure")

	first, _ := core.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "dbi",
		Err:          probeErr,
		Gen:          s.AvailabilityGen,
	})
	second, _ := core.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "rds", // the same type, delivered under its alias
		Err:          probeErr,
		Gen:          s.AvailabilityGen,
	})

	if got := countCache2Flashes(first); got != 1 {
		t.Fatalf("the first failure produced %d flash intents, want 1", got)
	}
	if got := countCache2Flashes(second); got != 0 {
		t.Errorf("the redundant failure delivered under the alias %q produced %d flash intents, want 0 — "+
			"the scan-health guard must key on the canonical short name so one type is one entry per sweep",
			"rds", got)
	}
}

func countCache2Flashes(intents []runtime.UIIntent) int {
	n := 0
	for _, in := range intents {
		if _, ok := in.(runtime.FlashIntent); ok {
			n++
		}
	}
	return n
}

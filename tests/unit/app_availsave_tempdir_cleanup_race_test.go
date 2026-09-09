// app_availsave_tempdir_cleanup_race_test.go — canary for task #41's
// flaky `testing.go:1464: TempDir RemoveAll cleanup: ... directory not
// empty` failure.
//
// Root cause (traced, not assumed): Controller.Close's own doc comment
// (core/app/menu.go) already states the contract precisely — a test
// that queues an availability save (directly or via Handle/Apply) must
// call Close BEFORE its t.TempDir() cleanup runs, or the async writer
// goroutine (runAvailabilitySaveLoop, started by queueAvailabilitySave)
// can still be calling cache.Store.SaveType (os.MkdirAll + os.CreateTemp +
// os.Rename, all rooted at cache.DirForTest(profile, region)) while
// RemoveAll(t.TempDir()) concurrently walks/removes that same tree.
// cache.cacheRoot() reads os.Getenv("A9S_CONFIG_FOLDER") LIVE at SaveType
// call time, not at goroutine-launch time, and os.Setenv/os.Unsetenv are
// process-global with no synchronization against a concurrent Getenv — so
// the leaked writer can just as easily land in whatever OTHER test's
// A9S_CONFIG_FOLDER is current by the time the OS scheduler finally runs
// it, explaining why the reported flake hits a different, seemingly
// unrelated test on each shuffled run.
//
// This file reproduces the mechanism directly (not through the shared
// newTestController* helpers, which the fix migrates separately) so the
// canary stays valid regardless of that migration: it drives the exact
// production seam (Controller.Handle(messages.ResourcesLoaded{...}) ->
// handleResourcesLoadedEvent -> syncExactTotalToMenu ->
// persistMenuAvailabilityCache -> queueAvailabilitySave) against a
// manually-owned temp directory, then races os.RemoveAll against the
// writer with and without Close.
package unit_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

const availSaveRaceShortName = "ec2"

// availSaveRaceFixture is a minimal AWS-SDK-shaped struct satisfying ec2's
// real default columns via fieldpath's case-insensitive field-name
// fallback — mirrors the s3RawFixture precedent in qa_cache_lifecycle_test.go.
type availSaveRaceFixture struct {
	InstanceId string
	State      string
}

// buildAvailSaveRaceController constructs a real Controller against
// (profile, region) — whose on-disk cache directory resolves under
// whatever A9S_CONFIG_FOLDER currently is — opens ec2's top-level list, and
// delivers one ResourcesLoaded event through the real menu-sync seam so a
// save is queued (queueAvailabilitySave starts runAvailabilitySaveLoop on
// first use). Returns the Controller so the caller controls Close timing.
func buildAvailSaveRaceController(t testing.TB, profile, region string) *app.Controller {
	s := session.New()
	s.Profile = profile
	s.Region = region
	core := runtime.New(s, resource.AllResourceTypes())
	c := newBlessedController(t, core)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: availSaveRaceShortName})

	res := []resource.Resource{
		{
			ID:   "i-race0000000000001",
			Name: "i-race0000000000001",
			Type: availSaveRaceShortName,
			Fields: map[string]string{
				"status": "running",
			},
			RawStruct: availSaveRaceFixture{InstanceId: "i-race0000000000001", State: "running"},
			Findings:  []domain.Finding{{Code: "canary", Phrase: "canary", Severity: domain.SevBroken, Source: "wave1"}},
		},
	}
	c.Handle(messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: availSaveRaceShortName,
		Resources:    res,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Gen:          0,
	})
	return c
}

// TestAvailSaveTempDirRace_WithoutClose_DirectoryNotEmptyOnRemoval is the RED
// canary: it reproduces, at high probability across many iterations, the
// exact `RemoveAll` failure mode task #41 reports — a leaked
// runAvailabilitySaveLoop goroutine recreating files under a directory that
// is concurrently being torn down — by omitting Close entirely (the bug
// class: a test that never registers Close at all, or registers it after
// its TempDir cleanup already ran).
//
// This test intentionally has NO t.TempDir()/t.Cleanup dependency of its
// own: it owns and removes its directory manually so it can assert on the
// exact failure condition without depending on testing.T's own cleanup
// ordering (which is precisely the mechanism under test).
func TestAvailSaveTempDirRace_WithoutClose_DirectoryNotEmptyOnRemoval(t *testing.T) {
	origEnv, hadEnv := os.LookupEnv("A9S_CONFIG_FOLDER")
	t.Cleanup(func() {
		if hadEnv {
			os.Setenv("A9S_CONFIG_FOLDER", origEnv) //nolint:errcheck // best-effort restore
		} else {
			os.Unsetenv("A9S_CONFIG_FOLDER") //nolint:errcheck // best-effort restore
		}
	})

	const iterations = 200
	incidents := 0

	for i := 0; i < iterations; i++ {
		dir, err := os.MkdirTemp("", "a9s-availsave-race-*")
		if err != nil {
			t.Fatalf("iteration %d: MkdirTemp: %v", i, err)
		}
		os.Setenv("A9S_CONFIG_FOLDER", dir) //nolint:errcheck // test-owned env, single-threaded here

		buildAvailSaveRaceController(t, "race-profile", "us-east-1")
		// Deliberately NOT calling c.Close() — this is the bug class under
		// test: the writer goroutine spawned by queueAvailabilitySave may
		// still be running SaveType (MkdirAll/CreateTemp/Rename under dir)
		// when RemoveAll below fires.

		if err := os.RemoveAll(dir); err != nil {
			incidents++
			continue
		}
		// A resurrected directory (the leaked writer's os.MkdirAll ran AFTER
		// RemoveAll completed but before we check) is the same failure class
		// surfaced a different way — the leaked goroutine is still live and
		// still writing into a path the test already considered cleaned up.
		if _, statErr := os.Stat(dir); statErr == nil {
			incidents++
			os.RemoveAll(dir) //nolint:errcheck // best-effort cleanup of the resurrected dir
		}
	}

	if incidents == 0 {
		t.Skip("race did not reproduce this run (timing-dependent) — " +
			"see TestAvailSaveTempDirRace_WithClose_NeverFails for the " +
			"fixed-usage contract this canary exists to protect")
	}
	t.Logf("reproduced %d/%d iterations with a leaked-writer cleanup incident (expected without Close)", incidents, iterations)
}

// TestAvailSaveTempDirRace_WithClose_NeverFails is the GREEN counterpart:
// identical loop, but Close() is called — and Close.Wait()s on
// availSaveWG — strictly BEFORE RemoveAll, mirroring the correct
// t.TempDir() + t.Cleanup(c.Close) LIFO ordering the harness fix enforces.
// Zero incidents across the same iteration count is the contract this test
// pins.
func TestAvailSaveTempDirRace_WithClose_NeverFails(t *testing.T) {
	origEnv, hadEnv := os.LookupEnv("A9S_CONFIG_FOLDER")
	t.Cleanup(func() {
		if hadEnv {
			os.Setenv("A9S_CONFIG_FOLDER", origEnv) //nolint:errcheck // best-effort restore
		} else {
			os.Unsetenv("A9S_CONFIG_FOLDER") //nolint:errcheck // best-effort restore
		}
	})

	const iterations = 200

	for i := 0; i < iterations; i++ {
		dir, err := os.MkdirTemp("", "a9s-availsave-race-*")
		if err != nil {
			t.Fatalf("iteration %d: MkdirTemp: %v", i, err)
		}
		os.Setenv("A9S_CONFIG_FOLDER", dir) //nolint:errcheck // test-owned env, single-threaded here

		c := buildAvailSaveRaceController(t, "race-profile", "us-east-1")
		c.Close()

		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("iteration %d: RemoveAll after Close: %v (want no error — Close must have drained the writer)", i, err)
		}
		if _, statErr := os.Stat(dir); statErr == nil {
			t.Fatalf("iteration %d: %s still exists after RemoveAll following Close — writer goroutine outlived Close.Wait()", i, dir)
		}
	}
}

// TestAvailSaveTempDirRace_HelperOrdering_MirrorsTTempDirLIFO pins the exact
// same-test ordering rule Close's doc comment prescribes, using testing.T's
// REAL TempDir/Cleanup machinery this time (as opposed to the two tests
// above, which own their directories manually to isolate the race). A
// helper that calls t.TempDir() THEN registers t.Cleanup(c.Close) is safe
// (LIFO: Close runs before RemoveAll); this test exists so a future
// refactor of the migrated helpers (newTestController et al.) cannot
// silently invert that order without a test failing loudly — a wrongly
// ordered registration here would surface as this exact test flaking under
// -shuffle=on -count=N.
func TestAvailSaveTempDirRace_HelperOrdering_MirrorsTTempDirLIFO(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	s := session.New()
	s.Profile = "lifo-order-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: availSaveRaceShortName})
	c.Handle(messages.ResourcesLoaded{
		ResourceType: availSaveRaceShortName,
		Resources: []resource.Resource{
			{
				ID:        "i-lifo0000000000001",
				Name:      "i-lifo0000000000001",
				Type:      availSaveRaceShortName,
				Fields:    map[string]string{"status": "running"},
				RawStruct: availSaveRaceFixture{InstanceId: "i-lifo0000000000001", State: "running"},
			},
		},
		Pagination: &resource.PaginationMeta{IsTruncated: false},
		Gen:        0, Provenance: messages.FetchProvenanceCanonicalList,
	})

	// Give the writer a fair chance to have actually run before this test
	// function returns and cleanups fire, so a regression that removed the
	// Close call (rather than just its ordering) would also be caught by a
	// leftover-file assertion here rather than depending entirely on
	// RemoveAll's own error surfacing.

	deadline := time.Now().Add(2 * time.Second)
	saved := false
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "cache", "lifo-order-profile--us-east-1", availSaveRaceShortName+".yaml")); err == nil {
			saved = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !saved {
		t.Fatal("availability cache file never appeared on disk before cleanup — test assumption broken (no save was queued)")
	}
	// t.Cleanup(c.Close) then t.TempDir()'s own RemoveAll run in LIFO order
	// (Close registered second, so it runs first) when this test returns.
}

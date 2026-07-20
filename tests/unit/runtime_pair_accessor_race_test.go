// runtime_pair_accessor_race_test.go — race-regression pin for GitHub issue
// #471: Core.Profile()/Core.Region() must never race a concurrent
// SetProfile()/SetRegion() (both routed through session.SetProfileRegion,
// guarded by session.pairMu). Meaningful only under -race; passes vacuously
// without it.
package unit

import (
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestPairAccessors_ConcurrentSwitchAndRead_RaceClean drives concurrent
// profile/region switches against concurrent Profile()/Region() reads and
// must complete clean under `go test -race`.
func TestPairAccessors_ConcurrentSwitchAndRead_RaceClean(t *testing.T) {
	const iterations = 1000

	core := runtime.New(session.New(), nil)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			core.SetProfile("p-a")
			core.SetRegion("r-b")
		}
	}()

	var profileLen, regionLen int
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			profileLen += len(core.Profile())
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			regionLen += len(core.Region())
		}
	}()

	wg.Wait()

	if profileLen < 0 || regionLen < 0 {
		t.Fatalf("unreachable: len() never negative (profileLen=%d regionLen=%d)", profileLen, regionLen)
	}

	gotProfile := core.Profile()
	if gotProfile != "p-a" {
		t.Errorf("final Profile() = %q, want %q (the only value ever written)", gotProfile, "p-a")
	}
	gotRegion := core.Region()
	if gotRegion != "r-b" {
		t.Errorf("final Region() = %q, want %q (the only value ever written)", gotRegion, "r-b")
	}
}

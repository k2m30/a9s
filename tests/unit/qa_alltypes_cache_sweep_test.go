// For every registered type
// (resource.AllResourceTypes()): a cached render is marked stale, the fresh
// fetch swaps in silently, and a save of one type leaves sibling files
// untouched (docs/design/cache-requirements.md).
//
// A fresh Controller/Core has never observed the type this session, so
// HandleNavigate falls back to the on-disk store when len(tf.Rows) > 0 and
// seeds NavigateResult.CachedEntry; applyNavResult then sets
// List.Loading=false and Refreshing=true before the dispatched fetch runs.
// A messages.ResourcesLoaded with Gen 0 is always accepted and replaces the
// rows wholesale, clearing Refreshing.
//
// The structural classes (paginated, zero-resource, issue-badge-excluded,
// child-list, related-heavy) are located from the registry, not by name.
package unit

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// alltypesSweepPair builds a fresh, hermetic Controller/Core pair backed by
// a per-test temp cache directory, with its own profile+region pair per
// resource type so no cross-type bleed is possible.
func alltypesSweepPair(t *testing.T, profile, region string) (*runtime.Core, *app.Controller) {
	t.Helper()
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return core, ctrl
}

// alltypesSeedTwoRows seeds a real on-disk TypeFile for shortName carrying 2
// realistic rows.
func alltypesSeedTwoRows(t *testing.T, profile, region, shortName string) {
	t.Helper()
	store := cache.LoadDirForTest(profile, region)
	store.Put(shortName, cache.TypeFile{
		HasResources: true,
		Count:        2,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "alltypes-" + shortName + "-row-1", Name: "alltypes-" + shortName + "-name-1", Fields: map[string]string{"region": region}},
			{ID: "alltypes-" + shortName + "-row-2", Name: "alltypes-" + shortName + "-name-2", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seed fixture SaveType(%s): %v", shortName, err)
	}
}

// fileSHA256 returns the sha256 hex digest of path's bytes, or "" if the
// file does not exist (a sibling that was never seeded is a legitimate
// before/after state — its absence must also be stable).
func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		t.Fatalf("reading %s for hash: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

func TestAllTypes_CachedRenderMarkedStale(t *testing.T) {
	types := resource.AllResourceTypes()
	skipped := map[string]string{}

	for _, td := range types {
		t.Run(td.ShortName, func(t *testing.T) {
			if td.Fetcher == nil {
				skipped[td.ShortName] = "no Wave-1 Fetcher registered — cannot be driven generically"
				t.Skip("no Wave-1 Fetcher registered")
			}

			tmp := t.TempDir()
			t.Setenv("A9S_CONFIG_FOLDER", tmp)
			profile, region := "alltypes-"+td.ShortName, "us-east-1"
			alltypesSeedTwoRows(t, profile, region, td.ShortName)

			_, ctrl := alltypesSweepPair(t, profile, region)

			_, tasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})

			snap := ctrl.Snapshot()
			lb := snap.Body.List
			if lb == nil {
				t.Fatalf("Body.List is nil after opening %q from a disk-seeded pair", td.ShortName)
			}
			if len(lb.Rows) != 2 {
				t.Fatalf("%s: ListBody.Rows = %d rows, want 2 (the disk-seeded rows) — cached render did not seed", td.ShortName, len(lb.Rows))
			}
			gotIDs := map[string]bool{}
			for _, r := range lb.Rows {
				gotIDs[r.ResourceID] = true
			}
			for _, want := range []string{"alltypes-" + td.ShortName + "-row-1", "alltypes-" + td.ShortName + "-row-2"} {
				if !gotIDs[want] {
					t.Errorf("%s: ListBody.Rows missing seeded ResourceID %q, got %v", td.ShortName, want, gotIDs)
				}
			}
			if lb.Loading {
				t.Errorf("%s: ListBody.Loading = true, want false — cached rows are already on screen", td.ShortName)
			}
			if !lb.Refreshing {
				t.Errorf("%s: ListBody.Refreshing = false, want true — C3 staleness marker must be set the moment cached rows are seeded, before any fetch result lands", td.ShortName)
			}

			hasFetch := false
			for _, task := range tasks {
				if task.Key.Kind == runtime.KindFetchResources {
					hasFetch = true
				}
			}
			if !hasFetch {
				t.Errorf("%s: no KindFetchResources task dispatched — a cache-seeded render must still re-verify on sight (C1)", td.ShortName)
			}
		})
	}

	if len(skipped) > 5 {
		t.Logf("WARNING: %d types skipped in CachedRenderMarkedStale, exceeds the >5 investigate threshold: %v", len(skipped), skipped)
	}
	if len(skipped) > 0 {
		t.Logf("CachedRenderMarkedStale skipped %d/%d types (no Wave-1 Fetcher): %v", len(skipped), len(types), skipped)
	}
}

func TestAllTypes_SilentSwap(t *testing.T) {
	types := resource.AllResourceTypes()
	skipped := map[string]string{}

	for _, td := range types {
		t.Run(td.ShortName, func(t *testing.T) {
			if td.Fetcher == nil {
				skipped[td.ShortName] = "no Wave-1 Fetcher registered — cannot be driven generically"
				t.Skip("no Wave-1 Fetcher registered")
			}

			tmp := t.TempDir()
			t.Setenv("A9S_CONFIG_FOLDER", tmp)
			profile, region := "alltypes-swap-"+td.ShortName, "us-east-1"
			alltypesSeedTwoRows(t, profile, region, td.ShortName)

			_, ctrl := alltypesSweepPair(t, profile, region)
			ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})

			preSwap := ctrl.Snapshot()
			if preSwap.Body.List == nil || preSwap.Body.List.Loading {
				t.Fatalf("%s: fixture assumption broken — Loading=true before the swap, want a warm cache-seeded render (Loading=false)", td.ShortName)
			}

			newRows := []resource.Resource{
				{ID: "alltypes-" + td.ShortName + "-fresh-1", Name: "alltypes-" + td.ShortName + "-fresh-1", Type: td.ShortName, Fields: map[string]string{"region": region}},
				{ID: "alltypes-" + td.ShortName + "-fresh-2", Name: "alltypes-" + td.ShortName + "-fresh-2", Type: td.ShortName, Fields: map[string]string{"region": region}},
				{ID: "alltypes-" + td.ShortName + "-fresh-3", Name: "alltypes-" + td.ShortName + "-fresh-3", Type: td.ShortName, Fields: map[string]string{"region": region}},
			}
			postSwap, _ := handlePage(ctrl, messages.ResourcesLoaded{
				ResourceType: td.ShortName,
				Resources:    newRows, Provenance: messages.FetchProvenanceCanonicalList,
			})

			lb := postSwap.Body.List
			if lb == nil {
				t.Fatalf("%s: Body.List is nil after ResourcesLoaded swap", td.ShortName)
			}
			if lb.Loading {
				t.Errorf("%s: ListBody.Loading = true after the swap — no Loading flash is permitted between a cache-seeded render and its fetch-confirmed swap", td.ShortName)
			}
			if lb.Refreshing {
				t.Errorf("%s: ListBody.Refreshing = true after ResourcesLoaded landed, want false — cache-first seeding: a fetch result clears the staleness marker", td.ShortName)
			}
			if len(lb.Rows) != 3 {
				t.Fatalf("%s: ListBody.Rows = %d rows after the swap, want 3 (the fresh fetch result, not the 2 seeded rows)", td.ShortName, len(lb.Rows))
			}
			gotIDs := map[string]bool{}
			for _, r := range lb.Rows {
				gotIDs[r.ResourceID] = true
			}
			for _, want := range []string{"alltypes-" + td.ShortName + "-fresh-1", "alltypes-" + td.ShortName + "-fresh-2", "alltypes-" + td.ShortName + "-fresh-3"} {
				if !gotIDs[want] {
					t.Errorf("%s: post-swap Rows missing fresh ResourceID %q, got %v", td.ShortName, want, gotIDs)
				}
			}
			for _, stale := range []string{"alltypes-" + td.ShortName + "-row-1", "alltypes-" + td.ShortName + "-row-2"} {
				if gotIDs[stale] {
					t.Errorf("%s: post-swap Rows still contains stale seeded ResourceID %q — silent swap must fully replace, not merge", td.ShortName, stale)
				}
			}
		})
	}

	if len(skipped) > 5 {
		t.Logf("WARNING: %d types skipped in SilentSwap, exceeds the >5 investigate threshold: %v", len(skipped), skipped)
	}
}

// TestAllTypes_SaveIsolation: for each type T, saving T's TypeFile must leave
// every other registered type's on-disk file byte-identical. Byte-comparing
// all siblings per type stays unit-fast since every file is a few hundred
// bytes of YAML on a t.TempDir().
func TestAllTypes_SaveIsolation(t *testing.T) {
	types := resource.AllResourceTypes()
	allNames := resource.AllShortNames()
	// One shared config root for the whole test; each subtest isolates on disk
	// via its unique "alltypes-iso-<type>" profile (cache dirs are profile-keyed),
	// which lets the ~66 subtests run in parallel. t.Setenv is called on this
	// non-parallel parent (permitted) and restored only after every parallel
	// subtest has joined.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	for _, td := range types {
		t.Run(td.ShortName, func(t *testing.T) {
			t.Parallel()
			profile, region := "alltypes-iso-"+td.ShortName, "us-east-1"

			// Seed every OTHER type plus the type under test, so there are real
			// sibling files on disk to disturb.
			store := cache.LoadDirForTest(profile, region)
			for _, sibling := range allNames {
				store.Put(sibling, cache.TypeFile{
					HasResources: true,
					Count:        1,
					Exact:        true,
					Rows: []cache.Row{
						{ID: "iso-" + sibling + "-1", Name: "iso-" + sibling + "-name-1", Fields: map[string]string{"region": region}},
					},
				})
				if err := store.SaveType(sibling); err != nil {
					t.Fatalf("seed fixture SaveType(%s): %v", sibling, err)
				}
			}

			dir := cache.DirForTest(profile, region)
			before := map[string]string{}
			for _, sibling := range allNames {
				if sibling == td.ShortName {
					continue
				}
				before[sibling] = fileSHA256(t, filepath.Join(dir, sibling+".yaml"))
			}

			store2 := cache.LoadDirForTest(profile, region)
			store2.Put(td.ShortName, cache.TypeFile{
				HasResources: true,
				Count:        9,
				Exact:        true,
				Rows: []cache.Row{
					{ID: "iso-" + td.ShortName + "-new-1", Name: "iso-" + td.ShortName + "-new-name-1", Fields: map[string]string{"region": region}},
				},
			})
			if err := store2.SaveType(td.ShortName); err != nil {
				t.Fatalf("SaveType(%s): %v", td.ShortName, err)
			}

			for _, sibling := range allNames {
				if sibling == td.ShortName {
					continue
				}
				after := fileSHA256(t, filepath.Join(dir, sibling+".yaml"))
				if before[sibling] != after {
					t.Errorf("%s: saving %q disturbed sibling file %q.yaml (hash before=%s after=%s) — C7 violation: a per-type save must never touch another type's file", td.ShortName, td.ShortName, sibling, before[sibling], after)
				}
			}

			// Sanity: the touched type's own file DID change / now reflects the
			// new content, proving the save mechanism actually ran.
			store3 := cache.LoadDirForTest(profile, region)
			tf, ok := store3.Type(td.ShortName)
			if !ok || tf.Count != 9 || len(tf.Rows) != 1 || tf.Rows[0].ID != "iso-"+td.ShortName+"-new-1" {
				t.Errorf("%s: own TypeFile after save+reload = %+v (ok=%v), want the new Count=9/1-row content", td.ShortName, tf, ok)
			}
		})
	}
}

// TestAllTypes_StructuralClass_Paginated finds a type and drives it with a
// TRUNCATED seed (Exact:false, Count > len(Rows)) — the "paginated: s3-style
// truncated fixture" representative. Any Wave-1-fetcher type qualifies; this
// asserts the class-specific behavior (IsTruncated surfaces as
// ListBody.Truncated=true) rather than picking one hardcoded name.
func TestAllTypes_StructuralClass_Paginated(t *testing.T) {
	types := resource.AllResourceTypes()
	var td *resource.ResourceTypeDef
	for i := range types {
		if types[i].Fetcher != nil {
			td = &types[i]
			break
		}
	}
	if td == nil {
		t.Fatal("no registered type with a Wave-1 Fetcher found — cannot sample the paginated structural class")
	}

	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	profile, region := "alltypes-class-paginated", "us-east-1"

	store := cache.LoadDirForTest(profile, region)
	store.Put(td.ShortName, cache.TypeFile{
		HasResources: true,
		Count:        3,
		Exact:        false,
		Rows: []cache.Row{
			{ID: "trunc-" + td.ShortName + "-1", Name: "trunc-" + td.ShortName + "-1", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType(td.ShortName); err != nil {
		t.Fatalf("seed fixture SaveType(%s): %v", td.ShortName, err)
	}

	_, ctrl := alltypesSweepPair(t, profile, region)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})

	lb := ctrl.Snapshot().Body.List
	if lb == nil {
		t.Fatalf("Body.List is nil after opening the truncated-seeded %q", td.ShortName)
	}
	if !lb.Truncated {
		t.Errorf("%s: ListBody.Truncated = false, want true — the seeded TypeFile.Exact=false (truncated first page) must surface as N+", td.ShortName)
	}
	if len(lb.Rows) != 1 {
		t.Errorf("%s: ListBody.Rows = %d, want 1 (the single truncated-page row)", td.ShortName, len(lb.Rows))
	}
}

// TestAllTypes_StructuralClass_ZeroResource pins the empty-not-stale contract: a
// zero-resource TypeFile (HasResources:false, Count:0, Rows:nil) must render
// an EMPTY list, not stale phantom rows — HandleNavigate's disk-store
// fallback only populates CachedEntry when len(tf.Rows) > 0, so a genuinely
// empty type falls through to ensureListState's bare default
// (Loading:true, Refreshing:false, Rows:nil).
func TestAllTypes_StructuralClass_ZeroResource(t *testing.T) {
	types := resource.AllResourceTypes()
	var td *resource.ResourceTypeDef
	for i := range types {
		if types[i].Fetcher != nil {
			td = &types[i]
			break
		}
	}
	if td == nil {
		t.Fatal("no registered type with a Wave-1 Fetcher found — cannot sample the zero-resource structural class")
	}

	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	profile, region := "alltypes-class-zero", "us-east-1"

	store := cache.LoadDirForTest(profile, region)
	store.Put(td.ShortName, cache.TypeFile{
		HasResources: false,
		Count:        0,
		Exact:        true,
		Rows:         nil,
	})
	if err := store.SaveType(td.ShortName); err != nil {
		t.Fatalf("seed fixture SaveType(%s): %v", td.ShortName, err)
	}

	_, ctrl := alltypesSweepPair(t, profile, region)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})

	lb := ctrl.Snapshot().Body.List
	if lb == nil {
		t.Fatalf("Body.List is nil after opening the zero-resource-seeded %q", td.ShortName)
	}
	if len(lb.Rows) != 0 {
		t.Errorf("%s: ListBody.Rows = %d, want 0 — empty-not-stale: a zero-resource cache must never show phantom stale rows", td.ShortName, len(lb.Rows))
	}
	if lb.Refreshing {
		t.Errorf("%s: ListBody.Refreshing = true for a zero-resource seed, want false — an empty cache is not a 'stale content' state, it is a 'nothing known yet' state", td.ShortName)
	}
}

// TestAllTypes_StructuralClass_IssueBadgeExcluded finds a type with
// ExcludeFromIssueBadge=true and asserts its IssuesKnown flag still
// round-trips through save/reload untouched — exclusion from the menu badge
// aggregation must not degrade the type's own persisted issue-known state.
func TestAllTypes_StructuralClass_IssueBadgeExcluded(t *testing.T) {
	types := resource.AllResourceTypes()
	var td *resource.ResourceTypeDef
	for i := range types {
		if types[i].ExcludeFromIssueBadge {
			td = &types[i]
			break
		}
	}
	if td == nil {
		t.Skip("no registered type has ExcludeFromIssueBadge=true — structural class not present in the current registry")
	}

	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	profile, region := "alltypes-class-excluded", "us-east-1"

	store := cache.LoadDirForTest(profile, region)
	store.Put(td.ShortName, cache.TypeFile{
		HasResources: true,
		Count:        1,
		Exact:        true,
		Issues:       2,
		IssuesKnown:  true,
		Rows: []cache.Row{
			{ID: "excl-" + td.ShortName + "-1", Name: "excl-" + td.ShortName + "-1", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType(td.ShortName); err != nil {
		t.Fatalf("seed fixture SaveType(%s): %v", td.ShortName, err)
	}

	reloaded := cache.LoadDirForTest(profile, region)
	tf, ok := reloaded.Type(td.ShortName)
	if !ok {
		t.Fatalf("%s: TypeFile missing after save+reload", td.ShortName)
	}
	if !tf.IssuesKnown {
		t.Errorf("%s: TypeFile.IssuesKnown = false after reload, want true — an issue-badge-excluded type still persists its own issuesKnown state (exclusion only affects the MENU aggregation, not this type's cache)", td.ShortName)
	}
	if tf.Issues != 2 {
		t.Errorf("%s: TypeFile.Issues = %d after reload, want 2", td.ShortName, tf.Issues)
	}
}

// TestAllTypes_StructuralClass_ChildList finds a type with at least one
// registered Children entry and drives the same cached-render assertion as
// TestAllTypes_CachedRenderMarkedStale against its TOP-LEVEL list — child
// lists are session-scoped and never written to disk, so the parent type's
// own top-level cache seed must still work for a type that has children.
func TestAllTypes_StructuralClass_ChildList(t *testing.T) {
	types := resource.AllResourceTypes()
	var td *resource.ResourceTypeDef
	for i := range types {
		if len(types[i].Children) > 0 && types[i].Fetcher != nil {
			td = &types[i]
			break
		}
	}
	if td == nil {
		t.Skip("no registered type with Children and a Wave-1 Fetcher found — structural class not present in the current registry")
	}

	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	profile, region := "alltypes-class-childlist", "us-east-1"
	alltypesSeedTwoRows(t, profile, region, td.ShortName)

	_, ctrl := alltypesSweepPair(t, profile, region)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})

	lb := ctrl.Snapshot().Body.List
	if lb == nil {
		t.Fatalf("Body.List is nil after opening child-list-bearing type %q", td.ShortName)
	}
	if len(lb.Rows) != 2 {
		t.Errorf("%s: ListBody.Rows = %d, want 2 (top-level cache seed unaffected by having child views)", td.ShortName, len(lb.Rows))
	}
	if !lb.Refreshing {
		t.Errorf("%s: ListBody.Refreshing = false, want true — the child-list structural class must still get the C3 staleness marker on its own top-level list", td.ShortName)
	}
}

// TestAllTypes_StructuralClass_RelatedHeavy finds the type with the most
// registered RelatedDef entries and asserts its top-level cached render still
// carries the staleness marker — the related panel is detail-scoped and
// session-only, so it must not interfere with the list-level cache contract.
func TestAllTypes_StructuralClass_RelatedHeavy(t *testing.T) {
	types := resource.AllResourceTypes()
	var td *resource.ResourceTypeDef
	best := 0
	for i := range types {
		if types[i].Fetcher == nil {
			continue
		}
		n := len(resource.GetRelated(types[i].ShortName))
		if n > best {
			best = n
			td = &types[i]
		}
	}
	if td == nil || best == 0 {
		t.Skip("no registered type has any RelatedDef entries — structural class not present in the current registry")
	}

	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	profile, region := "alltypes-class-related", "us-east-1"
	alltypesSeedTwoRows(t, profile, region, td.ShortName)

	_, ctrl := alltypesSweepPair(t, profile, region)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})

	lb := ctrl.Snapshot().Body.List
	if lb == nil {
		t.Fatalf("Body.List is nil after opening related-heavy type %q (%d related defs)", td.ShortName, best)
	}
	if len(lb.Rows) != 2 {
		t.Errorf("%s: ListBody.Rows = %d, want 2", td.ShortName, len(lb.Rows))
	}
	if !lb.Refreshing {
		t.Errorf("%s: ListBody.Refreshing = false, want true — a related-heavy type's own top-level list cache seed must still carry the C3 marker", td.ShortName)
	}
}

// TestAllTypes_RegistryDriveEligibility_SkipCountReported: the number of
// types eligible for the generic drive must not shrink to a handful — a large
// skip count means the generic drive itself, not any one type, is broken.
func TestAllTypes_RegistryDriveEligibility_SkipCountReported(t *testing.T) {
	types := resource.AllResourceTypes()
	if len(types) == 0 {
		t.Fatal("resource.AllResourceTypes() returned zero types — registry is empty, the generic drive has nothing to sweep")
	}
	eligible, skipped := 0, []string{}
	for _, td := range types {
		if td.Fetcher == nil {
			skipped = append(skipped, td.ShortName)
			continue
		}
		eligible++
	}
	t.Logf("all-types sweep eligibility: %d/%d types eligible for the generic drive, %d skipped (no Wave-1 Fetcher): %v", eligible, len(types), len(skipped), skipped)
	if len(skipped) > 5 {
		t.Errorf("%d types skipped the generic all-types drive (no Wave-1 Fetcher registered), exceeds the >5 investigate threshold from §5's dispatch — this needs investigation, not silent acceptance: %v", len(skipped), skipped)
	}
}

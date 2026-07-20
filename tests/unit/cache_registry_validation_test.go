package unit

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestCache_WrongVersionTypeFile_SkippedButSiblingsSurvive writes a per-type
// YAML file stamped with a schema version other than cache.SchemaVersion,
// alongside a well-formed sibling type file, then calls cache.LoadDirForTest and
// verifies the result is safe: it must not crash, the wrong-version type
// must not surface via Store.Type, and the sibling known-good type must
// still load normally. This replaces the round-1
// TestCache_RejectsUnknownResourceKeys concept (per-type-file cache.LoadDirForTest
// has no registry cross-check of its own — an unrecognized-but-well-formed
// type name loads under its own key just like any other; that filtering
// concept died with the single-file cache.Load implementation). What
// round-2's C7 actually promises is "wrong-version or unreadable ⇒ no cache
// for that type only", which is what this test pins instead.
func TestCache_WrongVersionTypeFile_SkippedButSiblingsSurvive(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	const knownType = "ec2"

	dir := cache.DirForTest("testprofile", "us-east-1")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}

	wrongVersionYAML := "version: 999\nhas_resources: true\ncount: 99\n"
	if err := os.WriteFile(filepath.Join(dir, "s3.yaml"), []byte(wrongVersionYAML), 0600); err != nil {
		t.Fatalf("writing wrong-version type file: %v", err)
	}

	store := cache.LoadDirForTest("testprofile", "us-east-1")
	store.Put(knownType, cache.TypeFile{HasResources: true, Count: 5})
	if err := store.SaveType(knownType); err != nil {
		t.Fatalf("SaveType(%s): %v", knownType, err)
	}

	reloaded := cache.LoadDirForTest("testprofile", "us-east-1")
	if reloaded == nil {
		t.Fatal("LoadDir returned nil")
	}

	if _, ok := reloaded.Type("s3"); ok {
		t.Error(`Type("s3") should be absent — a wrong-version type file must not surface as valid cache data (C7)`)
	}
	if got, ok := reloaded.Type(knownType); !ok || got.Count != 5 {
		t.Errorf("Type(%s) = %+v (ok=%v), want Count=5 — a sibling wrong-version file must not affect a healthy type's load (C7)", knownType, got, ok)
	}
}

// TestCache_LoadDirForTestRoundtrip_AllRegisteredTypes verifies that every
// top-level resource type short name registered via AllShortNames() survives
// a Put/SaveType/LoadDirForTest round-trip with identical field values, using
// the live registry so new resource types added in the future are
// automatically covered.
// Bug caught: a newly added resource type whose ShortName contains
// characters that are mis-sanitized by cache.DirForTest, or a yaml tag omission
// that silently drops a field on serialize/deserialize.
func TestCache_LoadDirForTestRoundtrip_AllRegisteredTypes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	allNames := resource.AllShortNames()
	if len(allNames) == 0 {
		t.Fatal("AllShortNames() returned empty — AWS init() may not have run")
	}

	store := cache.LoadDirForTest("testprofile", "us-east-1")
	entries := make(map[string]cache.TypeFile, len(allNames))
	for i, name := range allNames {
		tf := cache.TypeFile{
			HasResources: i%2 == 0,
			Count:        i + 1,
			Exact:        i%3 != 0,
		}
		entries[name] = tf
		store.Put(name, tf)
	}
	for _, name := range allNames {
		if err := store.SaveType(name); err != nil {
			t.Fatalf("SaveType(%s): %v", name, err)
		}
	}

	reloaded := cache.LoadDirForTest("testprofile", "us-east-1")
	if reloaded == nil {
		t.Fatal("cache.LoadDirForTest returned nil after SaveType")
	}

	for _, name := range allNames {
		orig := entries[name]
		got, ok := reloaded.Type(name)
		if !ok {
			t.Errorf("resource %q missing from loaded cache after round-trip", name)
			continue
		}
		if got.HasResources != orig.HasResources {
			t.Errorf("resource %q HasResources: got %v, want %v", name, got.HasResources, orig.HasResources)
		}
		if got.Count != orig.Count {
			t.Errorf("resource %q Count: got %d, want %d", name, got.Count, orig.Count)
		}
		if got.Exact != orig.Exact {
			t.Errorf("resource %q Exact: got %v, want %v", name, got.Exact, orig.Exact)
		}
	}
}

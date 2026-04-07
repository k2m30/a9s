package unit

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestCache_RejectsUnknownResourceKeys writes a YAML cache file containing a
// resource type key that is not in the resource registry, then calls cache.Load
// and verifies the result is safe: it must not crash, and it must either skip the
// unknown key (returns a File with only valid keys) or return an error — but does
// NOT silently accept the unknown key into f.Resources.
//
// EXPECTED FAILURE until cache.Load validates against registry — see CONCERNS.md #10.
// Currently cache.Load silently accepts unknown keys without any validation.
// This test documents the gap so a future fix can be verified here.
func TestCache_RejectsUnknownResourceKeys(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	// Write a YAML cache file with one unknown key and one valid known key.
	cacheDir := filepath.Join(tmpDir, "cache")
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}

	const knownType = "ec2"

	yamlContent := `profile: testprofile
region: us-east-1
checked_at: 2024-01-01T00:00:00Z
resources:
  not_a_real_type:
    has_resources: true
    count: 99
  ` + knownType + `:
    has_resources: true
    count: 5
`
	cachePath := filepath.Join(cacheDir, "testprofile--us-east-1.yaml")
	if err := os.WriteFile(cachePath, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("writing cache file: %v", err)
	}

	// Load must not panic.
	f, err := cache.Load("testprofile", "us-east-1")

	// Acceptable outcome A: returns an error (strict validation).
	if err != nil {
		// An error is fine — strict validation would reject unknown keys.
		return
	}

	if f == nil {
		t.Fatal("cache.Load returned (nil, nil) for a non-empty valid-YAML file — unexpected")
	}

	// Acceptable outcome B: silently skips unknown keys.
	// Assert the unknown key did NOT survive into f.Resources.
	allShortNames := map[string]bool{}
	for _, name := range resource.AllShortNames() {
		allShortNames[name] = true
	}

	for key := range f.Resources {
		if !allShortNames[key] {
			// EXPECTED FAILURE — documents the current gap.
			t.Errorf(
				"cache.Load silently accepted unknown resource key %q — "+
					"no validation against resource registry performed; "+
					"EXPECTED FAILURE until cache.Load validates keys against resource.AllShortNames()",
				key,
			)
		}
	}

	// Sanity check: the known-valid key must have survived.
	if _, ok := f.Resources[knownType]; !ok {
		t.Errorf("cache.Load dropped the known-valid key %q — unexpected", knownType)
	}
}

// TestCache_LoadRoundtrip_AllRegisteredTypes verifies that every top-level resource
// type short name registered via AllShortNames() survives a cache.Save / cache.Load
// round-trip with identical field values. The existing TestCache_Load_AllResourceTypes
// uses a hardcoded subset; this test uses the live registry so new resource types
// added in the future are automatically covered.
// Bug caught: a newly added resource type whose ShortName contains characters
// (e.g., underscores) that are mis-sanitized by cache.Path, or a yaml tag omission
// that silently drops a field on serialize/deserialize.
func TestCache_LoadRoundtrip_AllRegisteredTypes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)

	allNames := resource.AllShortNames()
	if len(allNames) == 0 {
		t.Fatal("AllShortNames() returned empty — AWS init() may not have run")
	}

	entries := make(map[string]cache.Entry, len(allNames))
	for i, name := range allNames {
		entries[name] = cache.Entry{
			HasResources: i%2 == 0,
			Count:        i + 1,
			Truncated:    i%3 == 0,
		}
	}

	original := &cache.File{
		Profile:   "testprofile",
		Region:    "us-east-1",
		Resources: entries,
	}

	if err := cache.Save(original); err != nil {
		t.Fatalf("cache.Save: %v", err)
	}

	loaded, err := cache.Load("testprofile", "us-east-1")
	if err != nil {
		t.Fatalf("cache.Load after Save: %v", err)
	}
	if loaded == nil {
		t.Fatal("cache.Load returned nil after Save")
	}

	for _, name := range allNames {
		orig := entries[name]
		got, ok := loaded.Resources[name]
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
		if got.Truncated != orig.Truncated {
			t.Errorf("resource %q Truncated: got %v, want %v", name, got.Truncated, orig.Truncated)
		}
	}
}

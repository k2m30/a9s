// cache_root_pin_and_path_safety_test.go pins two cache-layer defects.
//
// Defect 1 (session cache root resolved lazily, not pinned at construction):
// core/cache/cache.go's Root() reads os.Getenv("A9S_CONFIG_FOLDER") LIVE, and
// LoadDir(profile, region) captures the resulting dir into a *cache.Store —
// but that Store is built lazily, inside
// Session.ensureCacheStoreLocked (core/session/session.go), on the FIRST
// WithCacheStore/EnsureCacheStore call, not at session.New() time. A
// background availability-save goroutine belonging to an earlier, never-Closed
// Controller performs its first save after a LATER test has re-pointed
// A9S_CONFIG_FOLDER at its own t.TempDir() — the leaked writer's first
// ensureCacheStoreLocked call reads the CURRENT env, not the env that was
// live when its owning session was constructed, and lands in the later
// test's directory mid-RemoveAll ("TempDir RemoveAll cleanup: ... directory
// not empty" — the same failure class as
// TestAvailSaveTempDirRace_WithoutClose_DirectoryNotEmptyOnRemoval in
// app_availsave_tempdir_cleanup_race_test.go, but rooted here in the missing
// pin rather than a missing Close/Wait).
//
// Defect 2 (CodeQL "Uncontrolled data used in path expression",
// core/cache/cache.go:332): Dir(profile, region) glues
// SanitizePathElem(profile)+"--"+SanitizePathElem(region) into ONE path
// element, so a hostile profile/region cannot escape the cache root through
// Dir alone — "--" makes the combined string never equal exactly ".." or
// ".". The real escape is in Store.SaveType: shortName is joined into
// filepath.Join(dir, shortName+".yaml") and os.CreateTemp(dir,
// shortName+".yaml.tmp.*") WITHOUT sanitization. A shortName such as
// "../evil" walks the SaveType out of its own pair directory.
package unit_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestSessionCacheRoot_PinnedAtConstruction_EnvChangeAfterNewDoesNotRedirectWrites
// is the RED canary for defect 1: construct a session while A9S_CONFIG_FOLDER
// points at dirA, then repoint the env at dirB BEFORE the session's first
// cache write. The contract this test pins is that the cache root was
// captured once at session.New()/field-assignment time and is immune to the
// later env change; today's ensureCacheStoreLocked instead reads the env live
// on first use, so the write lands under dirB and this test fails.
func TestSessionCacheRoot_PinnedAtConstruction_EnvChangeAfterNewDoesNotRedirectWrites(t *testing.T) {
	dirA := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dirA)

	s := session.New()
	s.Profile = "testprofile"
	s.Region = "us-east-1"

	dirB := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dirB)

	const shortName = "ec2"
	store := s.EnsureCacheStore()
	if store == nil {
		t.Fatal("EnsureCacheStore returned nil for a resolved profile/region pair")
	}
	store.Put(shortName, cache.TypeFile{HasResources: true, Count: 1})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("SaveType(%s): %v", shortName, err)
	}

	wantPath := filepath.Join(dirA, "cache", "testprofile--us-east-1", shortName+".yaml")
	if _, statErr := os.Stat(wantPath); statErr != nil {
		t.Errorf("expected cache write pinned to construction-time root %s, but %s is missing: %v "+
			"(cache root must be captured once at Session construction, not re-read from "+
			"A9S_CONFIG_FOLDER on first WithCacheStore/EnsureCacheStore call)", dirA, wantPath, statErr)
	}

	dirBCacheRoot := filepath.Join(dirB, "cache")
	if entries, readErr := os.ReadDir(dirBCacheRoot); readErr == nil && len(entries) > 0 {
		t.Errorf("cache write leaked into post-construction env %s (entries: %v) — "+
			"an env change after Session construction must not redirect this session's cache writes",
			dirBCacheRoot, entries)
	}
}

// TestCacheDir_HostileProfileRegion_NeverEscapesCacheRoot is the table-driven
// half of defect 2: for every hostile profile/region pair, cache.DirForTest's
// result must either stay strictly inside cache.Root() or be "" (the
// established no-cache sentinel). The "--" glue in Dir already defeats pure
// "." / ".." traversal at this layer (see file doc comment) — this test
// exists as the regression guard for that property, and as the containment
// contract the SaveType-level test below extends to shortName.
func TestCacheDir_HostileProfileRegion_NeverEscapesCacheRoot(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		region  string
	}{
		{"dotdot-both", "..", ".."},
		{"dot-both", ".", "."},
		{"dotdot-slash-etc-profile", "../../etc", "us-east-1"},
		{"profile-embeds-dotdot-via-slash", "a/../..", "us-east-1"},
		{"windows-style-dotdot-region", "prod", "..\\.."},
		{"space-in-profile", "foo bar", "us-east-1"},
		{"empty-both", "", ""},
		{"benign", "prod", "eu-west-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("A9S_CONFIG_FOLDER", root)

			dir := cache.DirForTest(tc.profile, tc.region)
			if dir == "" {
				return
			}
			cleanDir := filepath.Clean(dir)
			cleanRoot := filepath.Clean(cache.Root())
			if cleanDir != cleanRoot && !strings.HasPrefix(cleanDir, cleanRoot+string(os.PathSeparator)) {
				t.Errorf("Dir(%q, %q) = %q escapes cache root %q", tc.profile, tc.region, dir, cleanRoot)
			}
		})
	}

	root := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", root)
	got := cache.DirForTest("prod", "eu-west-1")
	want := filepath.Join(root, "cache", "prod--eu-west-1")
	if got != want {
		t.Errorf("Dir(prod, eu-west-1) = %q, want %q (benign pair path shape must stay unchanged)", got, want)
	}
}

// TestCacheStoreSaveType_HostileShortName_NeverEscapesPairDir is the RED
// canary for the real defect-2 escape: SaveType joins shortName into the
// on-disk file/temp-file path with no sanitization. For each hostile
// shortName, either SaveType returns an error, or every file that exists
// under root afterward still lives inside the pair directory. Today,
// "../evil" walks SaveType's write (and its os.CreateTemp temp file) out of
// the pair directory into its parent without erroring, so this test fails.
func TestCacheStoreSaveType_HostileShortName_NeverEscapesPairDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", root)

	const profile, region = "prod", "eu-west-1"
	pairDir := filepath.Clean(cache.DirForTest(profile, region))

	hostileShortNames := []string{"../evil", "../../evil"}

	for _, shortName := range hostileShortNames {
		t.Run(shortName, func(t *testing.T) {
			store := cache.LoadDirForTest(profile, region)
			store.Put(shortName, cache.TypeFile{HasResources: true, Count: 1})
			saveErr := store.SaveType(shortName)

			walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				cleanPath := filepath.Clean(path)
				if cleanPath != pairDir && !strings.HasPrefix(cleanPath, pairDir+string(os.PathSeparator)) {
					t.Errorf("SaveType(%q) (err=%v) created file outside its pair directory: %s (pairDir=%s)",
						shortName, saveErr, cleanPath, pairDir)
				}
				return nil
			})
			if walkErr != nil {
				t.Fatalf("WalkDir(%s): %v", root, walkErr)
			}
		})
	}
}

// TestCacheStoreSaveType_BenignShortName_WritesExpectedPath is the
// non-hostile regression companion: an ordinary shortName must still land
// exactly at <pair-dir>/<shortName>.yaml, unaffected by any future
// sanitization fix for the hostile case above.
func TestCacheStoreSaveType_BenignShortName_WritesExpectedPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", root)

	const profile, region, shortName = "prod", "eu-west-1", "ec2"
	store := cache.LoadDirForTest(profile, region)
	store.Put(shortName, cache.TypeFile{HasResources: true, Count: 1})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("SaveType(%q): %v", shortName, err)
	}

	wantPath := filepath.Join(cache.DirForTest(profile, region), shortName+".yaml")
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("expected %s to exist after SaveType(%q): %v", wantPath, shortName, err)
	}
}

// cache.Root() reads A9S_CONFIG_FOLDER live, so a session pins its cache root
// when it is constructed: a cache writer that outlives its test would otherwise
// land in whichever directory a later test points the variable at.
//
// Dir glues profile and region into one path element with "--", so the pair
// can never be exactly "." or ".."; Store.SaveType joins shortName into the
// file path, so shortName must not walk out of the pair directory.
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
			"A9S_CONFIG_FOLDER on first EnsureCacheStore call)", dirA, wantPath, statErr)
	}

	dirBCacheRoot := filepath.Join(dirB, "cache")
	if entries, readErr := os.ReadDir(dirBCacheRoot); readErr == nil && len(entries) > 0 {
		t.Errorf("cache write leaked into post-construction env %s (entries: %v) — "+
			"an env change after Session construction must not redirect this session's cache writes",
			dirBCacheRoot, entries)
	}
}

// cache.DirForTest returns a path inside cache.Root() or "", the no-cache
// sentinel.
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

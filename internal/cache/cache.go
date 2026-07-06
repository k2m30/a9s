// Package cache provides per-profile+region, per-resource-type persistence
// of what the menu and top-level list screens have learned from the AWS
// API. See docs/design/cache-requirements.md for the full contract (C1-C10).
//
// Layout: one directory per profile+region pair
// (<cache root>/<profile>--<region>/), containing one YAML file per resource
// type (<shortName>.yaml). Loading the directory never fails — a missing
// directory yields an empty Store, and an unreadable or wrong-version file is
// skipped (one log line) without affecting sibling files (C7). Saving writes
// ONLY the touched type's file via atomic temp+rename, so a session that only
// touched one type physically cannot disturb another type's file.
package cache

import (
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/internal/domain"
)

// SchemaVersion is the current on-disk format marker. Every TypeFile's
// Version field is stamped with this value on save (C7a: encode/decode is a
// single chokepoint inside this package; a version bump only changes this
// constant plus the encode/decode logic, never callers).
const SchemaVersion = 1

// Row is a render-sufficient snapshot of one list row, persisted alongside
// the type's TypeFile so a cold start can seed the list screen with real
// cells and issue markers before any live fetch completes.
//
// NO RawStruct, color, glyph, or status text is carried on disk (C6): Fields
// must already contain every value a column needs (see
// app.MaterializeListFields, which runs before a page is cached), and
// colors/glyphs/status are derived at render time from Fields + Findings by
// the same classification rules live data uses.
type Row struct {
	ID       string            `yaml:"id"`
	Name     string            `yaml:"name,omitempty"`
	Fields   map[string]string `yaml:"fields,omitempty"`
	Findings []domain.Finding  `yaml:"findings,omitempty"`
}

// deepCopyRows returns a copy of rows in which every element's Fields map
// and Findings slice are freshly allocated, never sharing backing storage
// with rows itself. domain.Finding has no reference-typed fields (see its
// doc comment), so a fresh slice with copied elements is sufficient for
// Findings; Fields (a map) needs an explicit per-row maps.Clone.
//
// Required because Row.Fields commonly ALIASES a live resource.Resource's
// own Fields map (runtime.SaveTypeRows only copies when
// materializeResourceFields actually needs to inject a value — the common
// case leaves Fields pointing at the exact same map the rest of the app
// still holds and mutates). SaveType calls this immediately before
// yaml.Marshal so the marshaled snapshot can never race a concurrent
// mutation of that live map (see SaveType's doc comment).
func deepCopyRows(rows []Row) []Row {
	if rows == nil {
		return nil
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		if r.Fields != nil {
			r.Fields = maps.Clone(r.Fields)
		}
		if r.Findings != nil {
			r.Findings = append([]domain.Finding(nil), r.Findings...)
		}
		out[i] = r
	}
	return out
}

// TypeFile is the on-disk, self-contained state for one resource type within
// one profile+region pair. Version MUST stay the first field (C7a: a future
// encrypted format is detected by this marker before the rest of the file is
// parsed).
type TypeFile struct {
	Version int `yaml:"version"`

	HasResources bool `yaml:"has_resources"`
	Count        int  `yaml:"count"`
	Exact        bool `yaml:"exact,omitempty"`

	Issues          int  `yaml:"issues,omitempty"`
	IssuesKnown     bool `yaml:"issues_known,omitempty"`
	IssuesTruncated bool `yaml:"issues_truncated,omitempty"`

	Rows []Row `yaml:"rows,omitempty"`

	SavedAt time.Time `yaml:"saved_at"`
}

// Dir returns the cache directory path for one profile+region pair:
// <cache root>/<profile>--<region>/. Reuses the profile/region filename
// sanitation from the previous single-file layout (replace path separators
// and spaces with underscores).
func Dir(profile, region string) string {
	root := cacheRoot()
	if root == "" {
		return ""
	}
	safe := func(s string) string {
		s = strings.ReplaceAll(s, "/", "_")
		s = strings.ReplaceAll(s, "\\", "_")
		s = strings.ReplaceAll(s, " ", "_")
		return s
	}
	return filepath.Join(root, safe(profile)+"--"+safe(region))
}

// cacheRoot returns the cache root directory (~/.a9s/cache/), honoring the
// A9S_CONFIG_FOLDER override used by tests.
func cacheRoot() string {
	if folder := os.Getenv("A9S_CONFIG_FOLDER"); folder != "" {
		return filepath.Join(folder, "cache")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".a9s", "cache")
	}
	return ""
}

// Store holds the in-memory, loaded state of every resource type's TypeFile
// for one profile+region pair. Obtained via LoadDir; Put stages a type's new
// state, SaveType persists exactly that one type's file.
type Store struct {
	profile string
	region  string
	types   map[string]TypeFile
}

// LoadDir loads every readable, current-version type file under
// Dir(profile, region) into memory and returns a Store. Never fails: a
// missing directory yields an empty (non-nil) Store; an unreadable or
// wrong-version file is skipped (one log line) without affecting the other
// files' load (C7).
func LoadDir(profile, region string) *Store {
	s := &Store{
		profile: profile,
		region:  region,
		types:   make(map[string]TypeFile),
	}

	dir := Dir(profile, region)
	if dir == "" {
		return s
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		// Missing (or otherwise unreadable) directory: "no cache" for every
		// type. Not an error condition per C1/C7 — a fresh pair has no
		// history yet.
		return s
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		shortName := strings.TrimSuffix(name, ".yaml")
		path := filepath.Join(dir, name)

		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("cache: skipping %s: %v", path, err)
			continue
		}
		if len(data) == 0 {
			log.Printf("cache: skipping %s: empty file", path)
			continue
		}
		var tf TypeFile
		if err := yaml.Unmarshal(data, &tf); err != nil {
			log.Printf("cache: skipping %s: %v", path, err)
			continue
		}
		if tf.Version != SchemaVersion {
			log.Printf("cache: skipping %s: unsupported schema version %d (want %d)", path, tf.Version, SchemaVersion)
			continue
		}
		s.types[shortName] = tf
	}

	return s
}

// Type returns the loaded (or since-Put) TypeFile for shortName.
func (s *Store) Type(shortName string) (TypeFile, bool) {
	tf, ok := s.types[shortName]
	return tf, ok
}

// Types returns every currently-known TypeFile, keyed by resource short
// name. The returned map is a snapshot copy — callers may not mutate the
// Store through it.
func (s *Store) Types() map[string]TypeFile {
	out := make(map[string]TypeFile, len(s.types))
	maps.Copy(out, s.types)
	return out
}

// Put stages tf as shortName's current state, stamping Version and (when tf
// carries a zero-valued SavedAt — the normal case for every caller except a
// deliberate backdate/round-trip) SavedAt=now. A caller that explicitly sets
// a non-zero SavedAt before calling Put (e.g. to preserve a prior save's
// timestamp across a re-Put) has that value honored as-is — C1 requires no
// TTL / age-based discard, so nothing downstream depends on SavedAt being
// "now"; this only avoids clobbering a caller-supplied value. Does not touch
// disk — call SaveType to persist.
func (s *Store) Put(shortName string, tf TypeFile) {
	tf.Version = SchemaVersion
	if tf.SavedAt.IsZero() {
		tf.SavedAt = time.Now()
	}
	s.types[shortName] = tf
}

// SaveType atomically writes shortName's current staged state (as set by
// Put) to its own file — <dir>/<shortName>.yaml — via a temp file in the same
// directory followed by rename. No other type's file is opened or touched
// (C7: per-type files, no merge logic). The directory is created (0700) if
// missing; the written file is 0600.
//
// tf.Rows is deep-copied before Marshal (deepCopyRows): callers stage a
// TypeFile via Put with Rows built from live resource.Resource data (e.g.
// runtime.SaveTypeRows aliases cache.Row.Fields directly onto
// resource.Resource.Fields whenever materializeResourceFields finds nothing
// left to add — no copy is made in that case). When SaveType then runs on a
// different goroutine than the one still holding that live resource (e.g.
// Controller's async availability-cache writer racing a controller-lane
// applyFieldUpdatesToSlice call that mutates the same Fields map in place),
// yaml.Marshal's reflect-based map/slice walk and that mutation can race on
// the identical map — this was a real, reproduced data race (go test -race),
// not a theoretical one. Deep-copying here — the single chokepoint every
// save lane's Marshal goes through — makes the marshaled snapshot fully
// independent of whatever live structure Rows/Fields/Findings originally
// aliased, without requiring any caller to take a lock it doesn't already
// hold or without SaveType itself taking one (Store has none, and adding one
// would only re-serialize callers, not fix the aliasing).
func (s *Store) SaveType(shortName string) error {
	tf, ok := s.types[shortName]
	if !ok {
		return fmt.Errorf("cache: SaveType(%s): no staged state (call Put first)", shortName)
	}
	tf.Rows = deepCopyRows(tf.Rows)

	dir := Dir(s.profile, s.region)
	if dir == "" {
		return fmt.Errorf("cache: cannot determine cache directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating cache directory %s: %w", dir, err)
	}
	// Enforce 0700 even on a pre-existing directory (C7b). gosec's G302 rule
	// flags any Chmod call expecting file-oriented (<=0600) permissions; 0700
	// is the correct, more-restrictive owner-only mode for a DIRECTORY (needs
	// the execute bit to remain traversable by its owner), not a file.
	if err := os.Chmod(dir, 0700); err != nil { //nolint:gosec // 0700 is correct for a directory, not a file
		return fmt.Errorf("setting permissions on cache directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(tf)
	if err != nil {
		return fmt.Errorf("marshaling cache type %s: %w", shortName, err)
	}

	path := filepath.Join(dir, shortName+".yaml")
	tmpFile, err := os.CreateTemp(dir, shortName+".yaml.tmp.*")
	if err != nil {
		return fmt.Errorf("creating cache temp file in %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing cache %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing cache %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting permissions on cache %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming cache %s: %w", path, err)
	}
	return nil
}

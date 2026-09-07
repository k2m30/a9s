// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/logging"
)

// SchemaVersion is the current on-disk format marker. Every TypeFile's
// Version field is stamped with this value on save (C7a: encode/decode is a
// single chokepoint inside this package; a version bump only changes this
// constant plus the encode/decode logic, never callers).
//
// v2 (#463) adds Row.FindingFirstSeen. LoadDirIn also accepts a v1 file,
// backfilling FindingFirstSeen from the file's own SavedAt so a pre-#463
// cache never regresses to "no cache" on the next load.
const SchemaVersion = 2

// Row is a render-sufficient snapshot of one list row, persisted alongside
// the type's TypeFile so a cold start can seed the list screen with real
// cells and issue markers before any live fetch completes.
//
// NO RawStruct, color, glyph, or status text is carried on disk (C6):
// colors/glyphs/status are derived at render time from Fields + Findings by
// the same classification rules live data uses. What Fields carries for a
// column is decided by runtime.saveFieldKey, the one place that knows the
// render cascade's precedence: the value the live cell showed, under the
// single key the replayed row reads it back from.
type Row struct {
	ID       string            `yaml:"id"`
	Name     string            `yaml:"name,omitempty"`
	Fields   map[string]string `yaml:"fields,omitempty"`
	Findings []domain.Finding  `yaml:"findings,omitempty"`

	// FindingFirstSeen is the first observation time of each finding code
	// currently on this row, keyed by domain.FindingCode. Carried forward
	// across saves (core/runtime.stampFindingFirstSeen) while the finding
	// persists; a code absent here that reappears later is treated as newly
	// observed, not re-dated to its original first sighting (#463).
	FindingFirstSeen map[domain.FindingCode]time.Time `yaml:"finding_first_seen,omitempty"`
}

// deepCopyRows returns a copy of rows in which every element's Fields and
// FindingFirstSeen maps and Findings slice are freshly allocated, never
// sharing backing storage with rows itself. domain.Finding has no
// reference-typed fields (see its doc comment), so a fresh slice with copied
// elements is sufficient for Findings; Fields and FindingFirstSeen (maps)
// each need an explicit per-row maps.Clone — any reference-typed field added
// to Row in the future needs the same treatment here, or SaveType's
// marshal-vs-mutation race (below) reopens for that field.
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
		if r.FindingFirstSeen != nil {
			r.FindingFirstSeen = maps.Clone(r.FindingFirstSeen)
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

// DirForTest returns the cache directory path for one profile+region pair
// under the live cache.Root(): <cache root>/<profile>--<region>/. Thin
// wrapper over DirIn using the live root rather than a pinned one. Test-only:
// production always pins its own root (see session.Session.cacheRoot) and
// calls DirIn directly; only tests need the live-root convenience.
func DirForTest(profile, region string) string {
	return DirIn(Root(), profile, region)
}

// DirIn returns the cache directory path for one profile+region pair under
// the given root: <root>/<profile>--<region>/. Reuses the profile/region
// filename sanitation from the previous single-file layout (replace path
// separators and spaces with underscores). Takes root explicitly so a caller
// that pinned its own root (e.g. session.Session.cacheRoot) never re-reads
// Root() on every call.
func DirIn(root, profile, region string) string {
	if root == "" {
		return ""
	}
	dir := filepath.Join(root, SanitizePathElem(profile)+"--"+SanitizePathElem(region))
	// root is the untainted trust boundary (process config, never user input);
	// profile/region are user-controlled. "" is the same "no cache" sentinel
	// every caller (LoadDirIn, SaveType) already treats as "this pair does
	// not resolve" — comparing dir against a root-derived value, rather than
	// against another value built from profile/region, is what makes this a
	// real containment barrier instead of a tainted-vs-tainted comparison.
	cleanRoot := filepath.Clean(root)
	if cleaned := filepath.Clean(dir); cleaned != cleanRoot && !strings.HasPrefix(cleaned, cleanRoot+string(os.PathSeparator)) {
		return ""
	}
	return dir
}

// Root returns the cache root directory (~/.a9s/cache/), honoring the
// A9S_CONFIG_FOLDER override used by tests. Exported so other on-disk cache
// layouts sharing this root (e.g. core/costs' per-profile cost cache)
// derive it from the same single source instead of duplicating the
// A9S_CONFIG_FOLDER/UserHomeDir resolution.
func Root() string {
	if folder := os.Getenv("A9S_CONFIG_FOLDER"); folder != "" {
		return filepath.Join(folder, "cache")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".a9s", "cache")
	}
	return ""
}

// SanitizePathElem replaces path separators and spaces with underscores so s
// is safe to use as one path element (e.g. a profile or region name) in a
// cache file/directory name.
func SanitizePathElem(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// Store holds the in-memory, loaded state of every resource type's TypeFile
// for one profile+region pair. Obtained via LoadDirIn (or LoadDirForTest in
// tests); Put stages a type's new state, SaveType persists exactly that one
// type's file.
//
// dir is captured ONCE, at LoadDirIn construction time, from DirIn(root,
// profile, region) — every subsequent SaveType call reuses this captured value instead
// of recomputing Dir/cacheRoot (which reads A9S_CONFIG_FOLDER live). Without
// this, a Store's save path stays bound to whatever the environment variable
// happens to be at the moment SaveType's goroutine finally runs, not at the
// moment the Store was created — the exact seam a leaked
// runAvailabilitySaveLoop writer exploited to land in a since-repurposed (or
// already-removed) directory. A profile/region switch always constructs a
// brand-new Store via a fresh LoadDir call (see Session.Rotate +
// ensureCacheStoreLocked), so this capture is naturally per-pair and never
// goes stale across a rotation. Always used by pointer (LoadDir/LoadDirIn are
// its only constructors, both returning *Store) — never copy a Store by
// value, since saveMu below must not be duplicated.
type Store struct {
	profile string
	region  string
	dir     string
	types   map[string]TypeFile

	// saveMu serializes CommitSave's disk write (MkdirAll+temp-write+rename)
	// against any other concurrent CommitSave for this SAME Store — i.e. this
	// profile/region pair — so two renames can never interleave. Deliberately
	// separate from a caller's own in-memory lock (session.Session.pairMu):
	// PrepareSave/Put (the cheap, in-memory half of a save) run under pairMu,
	// but CommitSave's I/O runs after pairMu is released, so a slow disk write
	// never blocks an unrelated pairMu-guarded reader. See
	// Session.WithCacheStoreSave's doc comment for the full trade-off this
	// accepts: saveMu stops two commits from tearing each other's write, but
	// does NOT guarantee commit order matches the order the corresponding
	// PrepareSave/Put calls happened in.
	saveMu sync.Mutex
}

// LoadDirForTest loads every readable, current-version type file under
// DirForTest(profile, region) (i.e. under the live cache.Root()) into memory
// and returns a Store. Thin wrapper over LoadDirIn using the live root rather
// than a pinned one. Test-only: production always pins its own root (see
// session.Session.cacheRoot) and calls LoadDirIn directly.
func LoadDirForTest(profile, region string) *Store {
	return LoadDirIn(Root(), profile, region)
}

// LoadDirIn loads every readable, current-version type file under
// DirIn(root, profile, region) into memory and returns a Store. Never fails:
// a missing directory yields an empty (non-nil) Store; an unreadable or
// wrong-version file is skipped (one log line) without affecting the other
// files' load (C7). Takes root explicitly so a caller that pinned its own
// root at construction time (see session.Session.cacheRoot) never re-reads
// Root() — and therefore A9S_CONFIG_FOLDER — on a later, possibly
// differently-configured call.
func LoadDirIn(root, profile, region string) *Store {
	dir := DirIn(root, profile, region)
	s := &Store{
		profile: profile,
		region:  region,
		dir:     dir,
		types:   make(map[string]TypeFile),
	}

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
			logging.L().Warn("cache skip", "file", path, "reason", err)
			continue
		}
		if len(data) == 0 {
			logging.L().Warn("cache skip", "file", path, "reason", "empty file")
			continue
		}
		var tf TypeFile
		if err := yaml.Unmarshal(data, &tf); err != nil {
			logging.L().Warn("cache skip", "file", path, "reason", err)
			continue
		}
		switch tf.Version {
		case SchemaVersion:
			// current format, nothing to backfill.
		case SchemaVersion - 1:
			// v1 file: FindingFirstSeen never existed, so every finding
			// currently on the row is stamped with the file's own SavedAt —
			// the closest available approximation of when it was first
			// observed — rather than losing the row (#463: a pre-existing
			// cache must not regress to "no cache" on the next load).
			for i, r := range tf.Rows {
				if len(r.Findings) == 0 {
					continue
				}
				stamped := make(map[domain.FindingCode]time.Time, len(r.Findings))
				for _, f := range r.Findings {
					stamped[f.Code] = tf.SavedAt
				}
				tf.Rows[i].FindingFirstSeen = stamped
			}
		default:
			logging.L().Warn("cache skip", "file", path, "reason", fmt.Sprintf("unsupported schema version %d (want %d or %d)", tf.Version, SchemaVersion, SchemaVersion-1))
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

// WritePlan is an immutable, already-marshaled write staged by PrepareSave:
// the target directory/path plus the exact bytes to write. Produced while a
// caller's own lock is still held (e.g. session.Session.pairMu) and safe to
// Commit — or, via CommitSave, hand to the Store that produced it — after
// that lock is released, since a WritePlan holds no reference back into the
// Store's types map. See PrepareSave/CommitSave.
type WritePlan struct {
	dir  string
	path string
	data []byte
}

// commit performs the disk write staged by PrepareSave — the directory
// create, temp-file write, and atomic rename SaveType previously did inline.
// Unexported: callers reach it only through Store.CommitSave, which adds the
// saveMu serialization a WritePlan needs against a sibling commit for the
// same Store (a WritePlan by itself is not safe to Commit concurrently
// against another WritePlan for the same path — two renames into the same
// path have no ordering guarantee against each other without it).
func (wp WritePlan) commit() error {
	if err := os.MkdirAll(wp.dir, 0700); err != nil {
		return fmt.Errorf("creating cache directory %s: %w", wp.dir, err)
	}
	// Enforce 0700 even on a pre-existing directory (C7b). gosec's G302 rule
	// flags any Chmod call expecting file-oriented (<=0600) permissions; 0700
	// is the correct, more-restrictive owner-only mode for a DIRECTORY (needs
	// the execute bit to remain traversable by its owner), not a file.
	if err := os.Chmod(wp.dir, 0700); err != nil { //nolint:gosec // 0700 is correct for a directory, not a file
		return fmt.Errorf("setting permissions on cache directory %s: %w", wp.dir, err)
	}

	fname := filepath.Base(wp.path)
	tmpFile, err := os.CreateTemp(wp.dir, fname+".tmp.*")
	if err != nil {
		return fmt.Errorf("creating cache temp file in %s: %w", wp.dir, err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(wp.data); err != nil {
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
	if err := os.Rename(tmpPath, wp.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming cache %s: %w", wp.path, err)
	}
	return nil
}

// PrepareSave is SaveType's non-I/O half: validates shortName, deep-copies
// (deepCopyRows) and yaml.Marshals shortName's current staged state (as set
// by Put) into an immutable WritePlan, and resolves + escape-checks its
// target path. Touches no disk — safe to call while holding a caller's own
// lock (e.g. session.Session.pairMu) for only as long as this in-memory work
// takes, deferring the actual write (CommitSave) until after that lock is
// released. See Session.WithCacheStoreSave, the caller this split exists for.
//
// tf.Rows is deep-copied before Marshal (deepCopyRows): callers stage a
// TypeFile via Put with Rows built from live resource.Resource data (e.g.
// runtime.SaveTypeRows aliases cache.Row.Fields directly onto
// resource.Resource.Fields whenever materializeResourceFields finds nothing
// left to add — no copy is made in that case). Marshal here can run on a
// different goroutine than the one still holding that live resource (e.g.
// Controller's async availability-cache writer racing a controller-lane
// applyFieldUpdatesToSlice call that mutates the same Fields map in place);
// yaml.Marshal's reflect-based map/slice walk and that mutation can race on
// the identical map — this was a real, reproduced data race (go test -race),
// not a theoretical one. Deep-copying here — the single chokepoint every
// save lane's Marshal goes through — makes the marshaled snapshot fully
// independent of whatever live structure Rows/Fields/Findings originally
// aliased, without requiring any caller to take a lock it doesn't already
// hold.
func (s *Store) PrepareSave(shortName string) (WritePlan, error) {
	// filepath.IsLocal is the guard shape static taint analysis recognizes as
	// a path-injection barrier; ContainsAny alone is not. IsLocal alone would
	// still allow a nested element like "sub/evil", so ContainsAny keeps the
	// stricter single-path-element rule this package requires.
	if strings.ContainsAny(shortName, `/\`) || !filepath.IsLocal(shortName) {
		return WritePlan{}, fmt.Errorf("cache: SaveType(%s): shortName must be a single local path element", shortName)
	}
	tf, ok := s.types[shortName]
	if !ok {
		return WritePlan{}, fmt.Errorf("cache: SaveType(%s): no staged state (call Put first)", shortName)
	}
	tf.Rows = deepCopyRows(tf.Rows)

	// dir is s.dir — the root captured once at LoadDirIn construction time, NOT
	// a fresh DirIn(root, s.profile, s.region) recompute (see Store's doc comment) —
	// so a commit that runs on a goroutine outliving its owning Controller
	// always targets the directory that existed when the Store was built,
	// even if A9S_CONFIG_FOLDER has since changed or that directory has
	// since been removed. A leaked writer's commit against a removed dir
	// returns an error from CommitSave (MkdirAll/rename against a deleted
	// parent); every caller in this codebase (queueAvailabilitySave's save
	// loop) already logs and discards a SaveType/CommitSave error rather
	// than panicking, so this failure mode is tolerated by design, not
	// merely by accident.
	dir := s.dir
	if dir == "" {
		return WritePlan{}, fmt.Errorf("cache: cannot determine cache directory")
	}

	data, err := yaml.Marshal(tf)
	if err != nil {
		return WritePlan{}, fmt.Errorf("marshaling cache type %s: %w", shortName, err)
	}

	fname := shortName + ".yaml"
	path := filepath.Join(dir, fname)
	cleanDir := filepath.Clean(dir)
	if cleaned := filepath.Clean(path); cleaned != cleanDir && !strings.HasPrefix(cleaned, cleanDir+string(os.PathSeparator)) {
		return WritePlan{}, fmt.Errorf("cache: SaveType(%s): resolved path %s escapes cache directory %s", shortName, cleaned, cleanDir)
	}

	return WritePlan{dir: dir, path: path, data: data}, nil
}

// CommitSave writes wp to disk (MkdirAll + temp-write + rename), serialized
// against every other CommitSave call for this Store via saveMu so two
// commits can never interleave their renames. The only entry point that
// actually touches disk for a Store — SaveType and Session.WithCacheStoreSave
// both fall through to this.
func (s *Store) CommitSave(wp WritePlan) error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	return wp.commit()
}

// SaveType atomically writes shortName's current staged state (as set by
// Put) to its own file — <dir>/<shortName>.yaml — via a temp file in the same
// directory followed by rename. No other type's file is opened or touched
// (C7: per-type files, no merge logic). The directory is created (0700) if
// missing; the written file is 0600.
//
// A thin PrepareSave+CommitSave composition, for callers that don't need to
// split the in-memory marshal step from the disk write (i.e. every caller
// except Session.WithCacheStoreSave's save lanes, which call PrepareSave and
// CommitSave directly so a caller-held lock can cover only the former).
func (s *Store) SaveType(shortName string) error {
	wp, err := s.PrepareSave(shortName)
	if err != nil {
		return err
	}
	return s.CommitSave(wp)
}

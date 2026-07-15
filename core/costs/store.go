package costs

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/cache"
)

// schemaVersion is the current on-disk format marker (mirrors
// internal/cache.SchemaVersion's role: a single chokepoint for the encode/
// decode format, bumped only when the shape below changes). A file whose
// stored version does not match this constant is set aside to .bak via
// LoadStore's alien-version recovery instead of being misparsed.
const schemaVersion = 3

// anomalyTTL bounds how long a cached anomaly snapshot is trusted before
// Anomalies() reports it as absent.
const anomalyTTL = 24 * time.Hour

// openPeriodTTL bounds how long a cached open-period fetch is trusted
// before Lookup treats it as missing and the caller re-fetches (FR-011).
const openPeriodTTL = 24 * time.Hour

// settlementLag is how long after a period's own End a fetch must land
// before its data is treated as permanent: AWS documents that Cost Explorer
// revises estimated/recent data for roughly 24-72h after the billing period
// closes, so a bucket fetched shortly after closure can still change and
// must stay refetchable, not be promoted to immutable.
const settlementLag = 72 * time.Hour

// periodEntry is one fetched-and-cached period bucket: every Record whose
// Period matches the bucket's key, all fetched together and sharing one
// fetch timestamp.
type periodEntry struct {
	FetchedAt time.Time `yaml:"fetched_at"`
	Records   []Record  `yaml:"records"`
}

// immutableAt reports whether e was fetched at or after p's own End PLUS
// settlementLag — the point CE's own revision window has closed and data
// for p is permanent. This is the single predicate every closure-adjacent
// decision in this file keys on, replacing every hand-rolled `p.Closed(now)`
// check against a cached entry: "is p closed relative to THIS call's own
// now" and "was e itself fetched after p's settlement window closed" are
// different questions — a bucket fetched mid-period (while genuinely open),
// or even shortly after closure but still within settlementLag, does not
// retroactively become immutable just because a LATER caller's now happens
// to fall after p.End; only a fetch that itself landed on/after
// p.End+settlementLag does.
func (e periodEntry) immutableAt(p Period) bool {
	end, err := ParseDate(p.End)
	if err != nil {
		return false
	}
	return !e.FetchedAt.Before(end.Add(settlementLag))
}

// fetchedWhileOpen reports whether e was fetched before p's own End — i.e.
// e captured p while it was still the OPEN, in-progress period, never a
// closed-period post-closure snapshot. MergeCoverage's signal for which
// pre-existing, TTL-expired bucket it may authoritatively refresh off a
// zero-group refetch: only a bucket whose original fetch happened mid-period
// can have gone stale purely from the openPeriodTTL clock running out — a
// bucket fetched AFTER its own period closed (even one still within
// settlementLag, still-settling) was never open-period-TTL gated in the
// first place, and must be left exactly as Merge set it (refreshing it here
// would wrongly promote a still-unsettled closed-period record toward
// immutability).
func (e periodEntry) fetchedWhileOpen(p Period) bool {
	end, err := ParseDate(p.End)
	if err != nil {
		return false
	}
	return e.FetchedAt.Before(end)
}

// queryEntry is the cache state for one Query.CacheKey(): every period
// bucket fetched under that query shape so far.
type queryEntry struct {
	Periods map[string]periodEntry `yaml:"periods,omitempty"`
}

// anomalyBucket is the single cached anomaly snapshot slot. Covered is the
// date range the underlying GetAnomalies call was actually scoped to
// (Query.Range) — a zero-value Covered means "no range recorded" (every
// bucket written before this field existed, or written via PutAnomalies'
// two-argument form), treated as covering any requested window so existing
// TTL-only callers/behavior are unaffected.
type anomalyBucket struct {
	FetchedAt time.Time     `yaml:"fetched_at"`
	Marks     []AnomalyMark `yaml:"marks"`
	Covered   Period        `yaml:"covered,omitempty"`
}

// coversWindow reports whether b's own Covered range fully contains window's
// span. A zero-value Covered (never recorded) is treated as covering
// anything.
func (b *anomalyBucket) coversWindow(window []Period) bool {
	if b.Covered == (Period{}) {
		return true
	}
	if len(window) == 0 {
		return true
	}
	start, end := window[0].Start, window[len(window)-1].End
	return b.Covered.Start <= start && end <= b.Covered.End
}

// fileSchema is the on-disk shape of one profile's cost cache file. Version
// MUST stay first so a future format change is detected before the rest of
// the file is parsed.
type fileSchema struct {
	Version   int                   `yaml:"version"`
	Queries   map[string]queryEntry `yaml:"queries,omitempty"`
	Attrs     map[string]string     `yaml:"attrs,omitempty"`
	Anomalies *anomalyBucket        `yaml:"anomalies,omitempty"`
}

// Store holds the in-memory, loaded state of one profile's cost cache.
// Obtained via LoadStore; Merge/MergeAttrs/PutAnomalies stage new state,
// Save persists it. now is injected everywhere in the policy methods below
// — this package never calls time.Now() itself.
type Store struct {
	profile   string
	path      string
	data      fileSchema
	recovered bool
	// revision counts mutations (Merge/MergeCoverage/MergeAttrs/PutAnomalies/
	// ExpireOpenPeriod) since this Store was loaded — transient, never
	// persisted. The single cheap signal a grid-build memoization key needs
	// to know its cached result may now be stale.
	revision int
	// mu guards data/revision against Save's yaml.Marshal, which runs
	// outside the controller's own c.mu (Controller.Handle releases c.mu
	// before flushing a dirty Store to disk) — a concurrent delivery on
	// another goroutine can still be mutating these same maps via Merge/
	// MergeCoverage/etc while Save's reflection-based marshal walks them.
	mu sync.RWMutex
}

// Revision returns the number of mutations applied to s since it was
// loaded. Monotonically increasing within a Store's lifetime.
func (s *Store) Revision() int { return s.revision }

// CachePath returns the on-disk cache file path for one profile's cost
// data: <cache root>/<profile>--costs.yaml. Cost data is account-scoped,
// not region-scoped (data-model.md), so — unlike internal/cache's
// per-profile+region directories — there is exactly one file per profile.
// Root resolution and filename sanitization come from internal/cache
// (Root/SanitizePathElem) — the same single source internal/cache.Dir uses.
func CachePath(profile string) string {
	root := cache.Root()
	if root == "" {
		return ""
	}
	return filepath.Join(root, cache.SanitizePathElem(profile)+"--costs.yaml")
}

// NewMemoryStore returns an empty, memory-only Store for profile: no disk
// read at construction, and path is left "" so an accidental Save() call
// fails loudly (mirroring CachePath's own "root unresolved" contract)
// rather than silently writing to disk. The NoCache/demo counterpart to
// LoadStore — every other Store method behaves identically once
// constructed, only the disk-backing differs.
func NewMemoryStore(profile string) *Store {
	return &Store{
		profile: profile,
		data: fileSchema{
			Version: schemaVersion,
			Queries: make(map[string]queryEntry),
		},
	}
}

// LoadStore loads profile's cost cache file. Never fails: a missing file
// yields an empty Store; a corrupt or alien-version file is renamed to
// "<path>.bak" and a fresh Store is returned with Recovered() true — the
// caller flashes a warning instead of crashing or silently lying about
// what's cached.
func LoadStore(profile string) *Store {
	s := &Store{
		profile: profile,
		path:    CachePath(profile),
		data: fileSchema{
			Version: schemaVersion,
			Queries: make(map[string]queryEntry),
		},
	}
	if s.path == "" {
		return s
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}

	var parsed fileSchema
	if err := yaml.Unmarshal(raw, &parsed); err != nil || parsed.Version != schemaVersion {
		s.recovered = true
		_ = os.Rename(s.path, s.path+".bak")
		return s
	}
	if parsed.Queries == nil {
		parsed.Queries = make(map[string]queryEntry)
	}
	s.data = parsed
	return s
}

// Recovered reports whether the file this Store loaded from was corrupt or
// an alien schema version and had to be discarded (renamed to .bak).
func (s *Store) Recovered() bool { return s.recovered }

func periodKey(p Period) string { return p.Start + "/" + p.End }

// Lookup returns every cached Record covering a period in window under q's
// shape, and the subset of window periods that are missing. A closed
// period is returned regardless of fetch age (CE data for it can never
// change); an open period is returned only when its cached fetch is less
// than openPeriodTTL old, otherwise it is reported missing so the caller
// re-fetches it.
//
// Each window period first tries an exact cache-bucket match (the common
// case: window and cache share one granularity). When that misses, it
// falls back to aggregating every native cache bucket fully contained in
// the window period — a coarser display bucket (week/year) reading
// natively DAILY/MONTHLY cached records — requiring the matched buckets to
// tile the window period with no gap and no stale open sub-period before
// treating it as found.
func (s *Store) Lookup(q Query, window []Period, now time.Time) (records []Record, missing []Period) {
	entry := s.data.Queries[q.CacheKey()]
	for _, w := range window {
		recs, ok := lookupPeriod(entry, w, now)
		if !ok {
			missing = append(missing, w)
			continue
		}
		records = append(records, recs...)
	}
	return records, missing
}

func lookupPeriod(entry queryEntry, w Period, now time.Time) ([]Record, bool) {
	if bucket, found := entry.Periods[periodKey(w)]; found {
		if bucket.immutableAt(w) || now.Sub(bucket.FetchedAt) < openPeriodTTL {
			return bucket.Records, true
		}
		return nil, false
	}
	return lookupContained(entry.Periods, w, now)
}

// lookupContained aggregates every native bucket in periods whose own
// Period lies fully inside w, requiring the matched buckets — sorted by
// Start — to tile w edge-to-edge with no gap. A stale (TTL-expired) open
// sub-period anywhere inside w fails the whole lookup, mirroring the
// exact-match TTL rule at the aggregate granularity.
func lookupContained(periods map[string]periodEntry, w Period, now time.Time) ([]Record, bool) {
	type native struct {
		period Period
		bucket periodEntry
	}
	var matched []native
	for key, bucket := range periods {
		start, end, ok := strings.Cut(key, "/")
		if !ok || start < w.Start || end > w.End {
			continue
		}
		p := Period{Start: start, End: end}
		if !bucket.immutableAt(p) && now.Sub(bucket.FetchedAt) >= openPeriodTTL {
			return nil, false
		}
		matched = append(matched, native{period: p, bucket: bucket})
	}
	if len(matched) == 0 {
		return nil, false
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].period.Start < matched[j].period.Start })

	if matched[0].period.Start != w.Start {
		return nil, false
	}
	records := append([]Record(nil), matched[0].bucket.Records...)
	cursor := matched[0].period.End
	for _, m := range matched[1:] {
		if m.period.Start != cursor {
			return nil, false
		}
		cursor = m.period.End
		records = append(records, m.bucket.Records...)
	}
	if cursor != w.End {
		return nil, false
	}
	return records, true
}

// Merge stages recs, fetched at now, into q's cache entry. Records are
// grouped into buckets by their own Period. A bucket already fetched AFTER
// its own period closed (periodEntry.immutableAt) is never overwritten —
// closed CE data cannot change, so the first post-closure fetch is
// authoritative forever. Every other bucket (not yet fetched post-closure,
// including one fetched while genuinely open, or not present at all) is
// always overwritten — the whole point of the open-period TTL is picking up
// newer spend as the month progresses, and a legitimate post-closure re-fetch
// must replace a stale pre-closure snapshot rather than being skipped by it.
func (s *Store) Merge(q Query, recs []Record, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	key := q.CacheKey()
	entry := s.data.Queries[key]
	if entry.Periods == nil {
		entry.Periods = make(map[string]periodEntry)
	}

	byPeriod := make(map[string][]Record)
	periodOrder := make([]string, 0)
	for _, r := range recs {
		pk := periodKey(r.Period)
		if _, seen := byPeriod[pk]; !seen {
			periodOrder = append(periodOrder, pk)
		}
		byPeriod[pk] = append(byPeriod[pk], r)
	}

	for _, pk := range periodOrder {
		batch := byPeriod[pk]
		if existing, exists := entry.Periods[pk]; exists && existing.immutableAt(batch[0].Period) {
			continue
		}
		entry.Periods[pk] = periodEntry{FetchedAt: now, Records: batch}
	}

	s.data.Queries[key] = entry
}

// MergeCoverage stamps every NATIVE period tiling covered (translated
// internally via NativeCoveragePeriods(q.Granularity, covered) — callers
// pass the display window exactly as they built it, never a pre-translated
// one, so no caller can forget the translation) as fetched-at-now under q's
// shape. Merge alone can only learn a period was fetched from its own
// records' Period field, so a CE result with zero groups for a period (a
// young account, or spend fully filtered out by q.Filter) is invisible to
// it and Lookup reports that period missing forever, looping the fetch —
// MergeCoverage closes that gap for three cases:
//
//   - No bucket exists yet: a brand-new zero-Records bucket is stamped.
//   - An existing bucket was fetched WHILE ITS OWN PERIOD WAS STILL OPEN
//     (periodEntry.fetchedWhileOpen) and Merge did NOT touch it this round
//     (its FetchedAt still differs from now): this round's authoritative
//     zero-group refetch clears its stale Records and advances FetchedAt —
//     the bucket's own openPeriodTTL clock was the only thing that made it
//     stale, and refreshing FetchedAt alone can never promote it past its
//     own settlementLag immutability check (fetchedWhileOpen can only be
//     true for a bucket immutableAt would call false).
//   - Anything else — a bucket Merge DID just write this round
//     (FetchedAt==now), or one fetched AFTER its own period closed, even
//     one still within settlementLag ("closed but still settling") — is
//     left exactly as Merge set it. Merge is the only call that knows
//     whether THIS bucket's records were actually replaced this round;
//     touching a still-settling closed-period bucket here would wrongly
//     promote it toward immutability off a round that never actually
//     re-confirmed it.
func (s *Store) MergeCoverage(q Query, covered []Period, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	key := q.CacheKey()
	entry := s.data.Queries[key]
	if entry.Periods == nil {
		entry.Periods = make(map[string]periodEntry)
	}
	for _, p := range NativeCoveragePeriods(q.Granularity, covered) {
		pk := periodKey(p)
		existing, exists := entry.Periods[pk]
		switch {
		case !exists:
			entry.Periods[pk] = periodEntry{FetchedAt: now}
		case existing.fetchedWhileOpen(p) && !existing.FetchedAt.Equal(now):
			entry.Periods[pk] = periodEntry{FetchedAt: now}
		}
	}
	s.data.Queries[key] = entry
}

// ExpireOpenPeriod drops q's cached bucket for every NATIVE period tiling
// window (translated internally via NativeCoveragePeriods(q.Granularity,
// window) — same single-translation-point rule as MergeCoverage) that is
// not yet immutableAt its own period, forcing the next Lookup to report it
// missing — Ctrl+R's force-refresh (FR-012): only genuinely still-open
// buckets are force-refetched; a bucket already fetched post-closure is left
// untouched (refetched again only if genuinely absent).
func (s *Store) ExpireOpenPeriod(q Query, window []Period, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	key := q.CacheKey()
	entry, ok := s.data.Queries[key]
	if !ok || entry.Periods == nil {
		return
	}
	for _, p := range NativeCoveragePeriods(q.Granularity, window) {
		pk := periodKey(p)
		if existing, exists := entry.Periods[pk]; exists && existing.immutableAt(p) {
			continue
		}
		delete(entry.Periods, pk)
	}
	s.data.Queries[key] = entry
}

// Attrs returns the cached DimensionValueAttributes map (id -> display
// name), e.g. linked-account id -> account name.
func (s *Store) Attrs() map[string]string {
	if s.data.Attrs == nil {
		return map[string]string{}
	}
	return s.data.Attrs
}

// MergeAttrs merges newly fetched attrs into the cached map.
func (s *Store) MergeAttrs(attrs map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	if s.data.Attrs == nil {
		s.data.Attrs = make(map[string]string, len(attrs))
	}
	maps.Copy(s.data.Attrs, attrs)
}

// DataThrough returns the latest inclusive day covered by q's cached,
// non-empty buckets — the single source buildCostsBody/EnsureCostsState
// derive the footer's "data through" date from, so a warm disk cache (no
// live fetch on this open) still shows a correct value. "" when q has no
// cached records at all. A still-open bucket's own End is capped at now's
// own UTC date — the same "never overclaim spend through days that haven't
// happened yet" rule ApplyCostsLoaded's per-delivery tracking used to apply
// per-record; this derives the identical result from the store instead.
func (s *Store) DataThrough(q Query, now time.Time) string {
	entry := s.data.Queries[q.CacheKey()]
	nowDate := utcDate(now)
	var through string
	for key, bucket := range entry.Periods {
		if len(bucket.Records) == 0 {
			continue
		}
		start, end, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		endT, err := ParseDate(end)
		if err != nil {
			continue
		}
		inclusive := endT.AddDate(0, 0, -1)
		if !(Period{Start: start, End: end}).Closed(now) && nowDate.Before(inclusive) {
			inclusive = nowDate
		}
		if str := FormatDate(inclusive); str > through {
			through = str
		}
	}
	return through
}

// Anomalies returns the cached anomaly marks and whether they are still
// within the single-slot 24h TTL — TTL-only, range-blind: the read path
// (grid cell overlay) wants whatever is cached regardless of which range it
// was originally scoped to. AnomaliesCoverage is the range-aware freshness
// check PlanFetch needs instead.
func (s *Store) Anomalies(now time.Time) (marks []AnomalyMark, ok bool) {
	if s.data.Anomalies == nil {
		return nil, false
	}
	if now.Sub(s.data.Anomalies.FetchedAt) >= anomalyTTL {
		return nil, false
	}
	return s.data.Anomalies.Marks, true
}

// AnomaliesCoverage is Anomalies plus a range check: the cached snapshot
// must also cover window's own span (anomalyBucket.coversWindow), not just
// be within TTL — a snapshot cached under a narrower range (e.g. the
// default trailing-12-months month view) must not be treated as fresh for a
// materially wider request (e.g. a multi-year zoom-out) just because its
// TTL clock hasn't expired yet.
func (s *Store) AnomaliesCoverage(window []Period, now time.Time) (marks []AnomalyMark, ok bool) {
	marks, ok = s.Anomalies(now)
	if !ok {
		return marks, ok
	}
	if !s.data.Anomalies.coversWindow(window) {
		return nil, false
	}
	return marks, true
}

// PutAnomalies replaces the single cached anomaly snapshot. covered is the
// date range the underlying GetAnomalies call was scoped to — a zero-value
// Period is stamped verbatim, treated as covering any window by
// coversWindow (the pre-range-tracking behavior, preserved for any bucket
// whose caller never resolved a real range).
func (s *Store) PutAnomalies(marks []AnomalyMark, now time.Time, covered Period) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision++
	s.data.Anomalies = &anomalyBucket{FetchedAt: now, Marks: marks, Covered: covered}
}

// FetchResult is one completed Cost Explorer fetch: the grid data (Records/
// Attrs) alongside its independently-outcomed anomaly attempt (Anomalies).
// The single shape (*Store).ApplyFetchResult consumes, so a caller can never
// merge grid data and anomaly data through two calls that drift on ordering
// or on what "this fetch" even means.
type FetchResult struct {
	Query     Query
	Records   []Record
	Attrs     map[string]string
	Anomalies AnomalyResult
	Requests  int
	Err       error
}

// ApplyFetchResult merges r into s at now: Records always merge (Merge),
// Attrs merge when present (MergeAttrs) — an anomaly-fetch failure
// (r.Anomalies.Err != nil) never blocks either, since the grid data it
// accompanies is independently valid. The anomaly cache
// (PutAnomalies) writes ONLY when r.Anomalies.Requested is true AND
// r.Anomalies.Err is nil: a skipped attempt (Requested false) neither
// clears a cached mark nor renews its TTL, and a failed attempt
// (Requested true, Err set) leaves the cache exactly as it was too — only
// an authoritative result (requested, no error, possibly zero Marks)
// replaces it, which is also the case that correctly clears a stale mark
// when CE now genuinely reports none.
func (s *Store) ApplyFetchResult(r FetchResult, now time.Time) {
	s.Merge(r.Query, r.Records, now)
	if len(r.Attrs) > 0 {
		s.MergeAttrs(r.Attrs)
	}
	if r.Anomalies.Requested && r.Anomalies.Err == nil {
		s.PutAnomalies(r.Anomalies.Marks, now, r.Query.Range)
	}
}

// Save persists the Store to CachePath(profile) via atomic temp-write +
// rename, mirroring internal/cache's save convention.
func (s *Store) Save() error {
	if s.path == "" {
		return fmt.Errorf("costs: no cache path resolved for profile %q", s.profile)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("costs: creating cache dir: %w", err)
	}

	// The Version stamp is a write, so it takes the full Lock (excluding
	// concurrent mutators AND any other concurrent Save's own read) —
	// separately from the marshal's RLock below, which only needs to
	// exclude mutators, not other readers.
	s.mu.Lock()
	s.data.Version = schemaVersion
	s.mu.Unlock()

	s.mu.RLock()
	out, err := yaml.Marshal(s.data)
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("costs: marshaling cache: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return fmt.Errorf("costs: writing cache: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("costs: renaming cache: %w", err)
	}
	return nil
}

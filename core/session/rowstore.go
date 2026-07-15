// rowstore.go — session-scoped, per-type row store (task #17, the row-store
// unification effort). See docs/design/cache-requirements.md and the
// row-store unification plan for the target design this file implements.
//
// As of Stage 3, RowStore is the SOLE per-type row store: the former
// type-keyed maps (session.ProbeResources, session.ResourceCache,
// session.LazyResourceCache) are gone. A type's rows live in exactly one
// TypeRows entry regardless of which lane wrote them (Wave-1 probe, disk
// seed, top-level fetch, or a sparse FetchByIDs drill) — see Origin/Partial
// below for how that entry distinguishes the roles those three maps used to
// play independently.
//
// Semantics mirror the two in-memory reconciliation rules that already exist
// independently for the per-screen ListState (internal/app/list_body.go:
// dedupAgainstExisting, isStaleReplace) and the on-disk TypeFile
// (internal/runtime/probes.go: reconcileTypeFile, rowIDsAreSubset) so a
// future caller can safely retire either without behavior drift. RowStore
// does not implement C6b Wave-2 carry — that remains reconcileTypeFile's
// concern at the disk chokepoint; a bare Observe/Amend here never inspects
// Finding.Source prefixes.
package session

import (
	"maps"
	"sync"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// Origin identifies which lane produced a TypeRows observation. Ordering
// matters only for the Disk-never-overwrites-Fetch/Probe rule in Observe;
// Probe and Fetch are otherwise treated identically by the store's
// append/replace rules.
type Origin int

const (
	// OriginDisk marks rows seeded from the on-disk per-type cache file (C1:
	// cached content renders before any AWS activity). A Disk-origin
	// observation is rejected when it would overwrite rows already carrying
	// Probe or Fetch origin — a stale disk seed arriving after a live result
	// has already landed must not regress the session's live knowledge.
	OriginDisk Origin = iota
	// OriginProbe marks rows retained by the Wave-1 availability probe
	// (first-page-only — the role the removed session.ProbeResources map
	// used to play).
	OriginProbe
	// OriginFetch marks rows landed from a top-level list fetch — the
	// richest, most page-complete source (the role the removed
	// session.ResourceCache map used to play).
	OriginFetch
)

// TypeRows is the per-type row set the store retains for one canonical
// resource short name.
//
// Invariant: once a resource.Resource value is stored in Rows, that VALUE is
// never mutated in place — every state change (append, replace, enrichment
// fold) produces a new slice and, where a row's own fields change, a new
// Resource value for that row. This is what makes Snapshot/SnapshotAll safe
// to hand out as a bare slice header: the caller's view can never be
// invalidated by a later store write, because a later write never touches
// the backing array the caller already observed. Amend is the only mutator
// that changes row CONTENT, and it does so via copy-on-write (fn returns a
// fresh slice) rather than writing into rows[i] fields directly.
type TypeRows struct {
	// Rows is this type's currently retained row set. Treat as immutable —
	// see the copy-on-write invariant above.
	Rows []resource.Resource
	// Pagination is the most recent pagination state observed for this type's
	// canonical list. nil means unknown (conservatively treated as truncated
	// by callers per C5 — nil pagination is never exact).
	Pagination *resource.PaginationMeta
	// TotalCount is the authoritative total known for this type, which may
	// exceed len(Rows) (C6a: a counts-only observation updates TotalCount
	// without touching Rows).
	TotalCount int
	// Origin records which lane most recently accepted a rows-carrying
	// write for this type.
	Origin Origin
	// Partial marks this entry as sourced from a sparse/lazy read (the
	// LazyResourceCache role — FetchByIDs drills, not a full first page).
	// A subsequent full Observe clears Partial (full-beats-partial).
	//
	// Partial is a TYPE-level flag, not a per-row one: ObservePartial on a
	// type whose entry is already full (Gen!=0 && !Partial) still appends
	// its rows as genuine deeper coverage (dedup-by-ID, same as any other
	// partial add) but leaves Partial=false — a full entry is never
	// downgraded back to partial by a subsequent sparse add. ObservePartial
	// on an unobserved or already-partial entry keeps/sets Partial=true. A
	// full Observe on a partial entry always flips Partial=false
	// (full-beats-partial, see Observe's doc comment).
	Partial bool
	// Gen increments on every accepted write (Observe, ObservePartial,
	// Amend, ObserveCount) so callers can detect whether their prior
	// Snapshot is stale.
	Gen domain.Gen
	// ViewState carries the top-level resource list's interactive state
	// (filter/sort/cursor/h-scroll) across a re-entry into the same
	// resource type. It is orthogonal to Rows/Pagination/TotalCount/Origin/
	// Partial/Gen: Observe/ObserveCount/ObservePartial/Amend all preserve
	// the existing entry's ViewState untouched (a rows-carrying write is
	// never itself a view-state write) — only SetViewState mutates it, and
	// it never touches Rows/Pagination/TotalCount/Origin/Partial/Gen in
	// return. This split is what lets a warm re-entry into a cached list
	// (Core.ResourceCache) restore the exact sort column, sort direction,
	// cursor row, and horizontal scroll offset the user left the list in,
	// without requiring the fetch-result write path (Observe) to know
	// anything about renderer-side interactive state.
	ViewState ListViewState
}

// ListViewState is the renderer-owned interactive state of a top-level
// resource list, retained alongside its rows so a warm re-entry restores
// the exact view the user left (filter text, ctrl+z attention-only toggle,
// sort column/direction, cursor row, horizontal scroll offset). Mirrors the
// subset of domain.ListViewCacheEntry's fields that are NOT derived from
// the fetch result itself (Resources/Pagination/TotalCount already live on
// TypeRows directly).
type ListViewState struct {
	FilterText    string
	AttentionOnly bool
	SortColIdx    int
	SortAsc       bool
	CursorPos     int
	HScrollOffset int
}

// RowStore is the session-scoped, per-type row store: one row set per
// canonical resource short name, keyed by TypeRows.Rows/Pagination/
// TotalCount/Origin/Partial/Gen. All access is serialized on mu so
// concurrent tea.Cmd goroutines (a background sweep save racing a
// foreground list fetch, e.g.) can never interleave a read-modify-write on
// the same type.
type RowStore struct {
	mu    sync.Mutex
	types map[string]TypeRows
}

// NewRowStore constructs an empty RowStore.
func NewRowStore() *RowStore {
	return &RowStore{types: make(map[string]TypeRows)}
}

// dedupAgainstExistingRows mirrors internal/app/list_body.go's
// dedupAgainstExisting: returns the subset of incoming whose ID is not
// already present in existing, preserving incoming's order. Delegates to
// resource.DedupByID, the single-source implementation shared with
// internal/app (session already imports internal/resource; no new
// dependency introduced).
func dedupAgainstExistingRows(existing, incoming []resource.Resource) []resource.Resource {
	return resource.DedupByID(existing, incoming)
}

// rowIDsAreSubsetRows mirrors internal/runtime/probes.go's rowIDsAreSubset:
// reports whether every ID in candidate also appears in superset.
func rowIDsAreSubsetRows(candidate, superset []resource.Resource) bool {
	if len(candidate) == 0 {
		return true
	}
	known := make(map[string]struct{}, len(superset))
	for _, r := range superset {
		known[r.ID] = struct{}{}
	}
	for _, r := range candidate {
		if _, ok := known[r.ID]; !ok {
			return false
		}
	}
	return true
}

// cloneRows returns a defensive copy of rows: a fresh slice, and for each
// row whose Fields map is non-nil, a fresh Fields map — so a caller that
// mutates a Snapshot/SnapshotAll result in place (rows[i].Name = ...,
// rows[i].Fields[k] = ...) can never leak that mutation back into the
// store's own backing array or maps, independent of the store's own
// internal copy-on-write discipline on writes.
func cloneRows(rows []resource.Resource) []resource.Resource {
	if rows == nil {
		return nil
	}
	out := make([]resource.Resource, len(rows))
	for i, r := range rows {
		if r.Fields != nil {
			fields := make(map[string]string, len(r.Fields))
			maps.Copy(fields, r.Fields)
			r.Fields = fields
		}
		out[i] = r
	}
	return out
}

// isStaleReplaceRows mirrors internal/app/list_body.go's isStaleReplace: a
// non-append replace is treated as a stale, out-of-order straggler only when
// incoming is BOTH smaller than existing AND still truncated AND a strict ID
// subset of existing — the same conservative, false-negative-biased shape
// the per-screen ListState guard uses (DEF-18 mechanism A).
func isStaleReplaceRows(existing, incoming []resource.Resource, pagination *resource.PaginationMeta) bool {
	if len(incoming) == 0 || len(incoming) >= len(existing) {
		return false
	}
	if pagination == nil || !pagination.IsTruncated {
		return false
	}
	return rowIDsAreSubsetRows(incoming, existing)
}

// Observe applies a rows-carrying observation for canon (the caller's
// already-canonicalized resource short name) and returns the row set
// actually accepted (the pre-existing rows, unchanged, when the observation
// is rejected) plus the store's per-type generation after the call — a
// rejected observation leaves Gen unchanged.
//
// Semantics, applied in order:
//
//  1. Disk-vs-Fetch/Probe: an OriginDisk observation is rejected over an
//     existing Fetch-origin (or Probe-origin) entry that already carries
//     rows — a disk seed race-losing to an already-landed live result must
//     not regress the session's live knowledge.
//     1b. Probe-vs-Fetch (append=false only): an OriginProbe replace is
//     rejected over an existing Fetch-origin entry that already carries
//     rows — a (possibly smaller, truncated) availability-probe page must
//     never regress rows a live top-level fetch already accumulated via
//     load-more. Probe replacing Probe, or Probe replacing Disk, is still
//     allowed; Fetch replacing Fetch is untouched by this rule.
//  2. Stale replace (append=false only): mirrors isStaleReplace — a smaller,
//     still-truncated, strict-ID-subset replace is rejected as an
//     out-of-order straggler.
//  3. Append: incoming is deduped against the existing rows by stable ID
//     (mirrors dedupAgainstExisting) and appended. Append always accepts —
//     dedup happens to the row set, not to the observation.
//  4. Replace (append=false, not stale): incoming rows replace the existing
//     rows wholesale.
//  5. TotalCount shrink guard (applies to both append and replace): a
//     non-exact incoming pagination (IsTruncated=true, or nil — nil is never
//     exact per DEF-18) only ever RAISES TotalCount to at least len(newRows);
//     it never shrinks a wider TotalCount already known (e.g. seeded by an
//     earlier ObserveCount or a wider prior Observe), since a truncated page
//     explicitly does not claim to be the whole list. Only an EXACT
//     (IsTruncated=false) result is authoritative proof of the new total and
//     is allowed to shrink it — resources can genuinely be deleted between
//     observations.
//
// A non-Disk observation always clears Partial (full-beats-partial is
// handled by ObservePartial's counterpart rule — a plain Observe is by
// definition a full, non-partial observation).
func (s *RowStore) Observe(canon string, rows []resource.Resource, pagination *resource.PaginationMeta, origin Origin, appendPage bool) ([]resource.Resource, domain.Gen) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.types[canon]

	if origin == OriginDisk && len(existing.Rows) > 0 && (existing.Origin == OriginFetch || existing.Origin == OriginProbe) {
		return existing.Rows, existing.Gen
	}

	if !appendPage && origin == OriginProbe && len(existing.Rows) > 0 && existing.Origin == OriginFetch {
		return existing.Rows, existing.Gen
	}

	// C5/DEF-18 mechanism A: the stale-shaped-replace rejection below only
	// applies while the EXISTING entry has not yet reached a confirmed exact
	// total (mirrors internal/app/list_body.go's applyResourcesLoaded, which
	// gates its own isStaleReplace call on !ls.HasPagination). A Ctrl+R full
	// reset legitimately replays the exact same page-1 IDs with
	// IsTruncated=true while the existing entry is ALSO still truncated
	// (never confirmed exact) — that reset must win, matching
	// TestStoryF1_CtrlR_ResetsPagination. Once existing.Pagination reports
	// IsTruncated=false (C5: exact only ever advances), a smaller,
	// still-truncated, ID-subset replace can only be an out-of-order
	// straggler and IS rejected (TestRowStore_Observe_StaleTruncatedSubsetRejectedOnceExact).
	existingIsExact := existing.Pagination != nil && !existing.Pagination.IsTruncated
	if !appendPage && !existing.Partial && existingIsExact && isStaleReplaceRows(existing.Rows, rows, pagination) {
		return existing.Rows, existing.Gen
	}

	var newRows []resource.Resource
	if appendPage {
		newRows = append(append([]resource.Resource(nil), existing.Rows...), dedupAgainstExistingRows(existing.Rows, rows)...)
	} else {
		newRows = rows
	}

	// TotalCount shrink guard: nil pagination is never exact (DEF-18), and an
	// IsTruncated=true page explicitly does not claim to be the whole list —
	// neither is authoritative proof the total shrank, so both only ever
	// raise TotalCount to at least len(newRows), never below the existing
	// known total (e.g. one seeded by an earlier, wider ObserveCount). Only
	// an EXACT (IsTruncated=false) result is authoritative proof of the new
	// total and may shrink it — resources can genuinely be deleted between
	// observations, and an exact fetch confirms that directly. Applies
	// identically to the append path, where newRows is already the
	// accumulated (deduped) slice.
	totalCount := len(newRows)
	if pagination == nil || pagination.IsTruncated {
		if existing.TotalCount > totalCount {
			totalCount = existing.TotalCount
		}
	}

	next := TypeRows{
		Rows:       newRows,
		Pagination: pagination,
		TotalCount: totalCount,
		Origin:     origin,
		Partial:    false,
		Gen:        existing.Gen + 1,
		ViewState:  existing.ViewState,
	}
	s.types[canon] = next
	return next.Rows, next.Gen
}

// ObserveCount applies a counts-only observation for canon (C6a: a
// counts-only observation — e.g. the availability-sweep-to-menu sync-back,
// or a disk-cache-loaded seed with no per-type disk row data — updates
// TotalCount only and never touches Rows, even when this leaves TotalCount
// numerically disagreeing with len(Rows)). Existing Rows, Pagination,
// Origin, and Partial are all carried forward untouched. Creates an entry
// even for a canon RowStore has never seen rows for. Every observation
// (counts-only included) is a write, so Gen is bumped unconditionally —
// callers relying on Gen monotonicity to detect "something changed" must
// see this reflected even when Rows itself is untouched.
func (s *RowStore) ObserveCount(canon string, totalCount int) domain.Gen {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.types[canon]
	next := existing
	next.TotalCount = totalCount
	next.Gen = existing.Gen + 1
	s.types[canon] = next
	return next.Gen
}

// ObservePartial applies a sparse/lazy row observation for canon (the
// LazyResourceCache role — FetchByIDs drills that never see a full first
// page). Marks the entry Partial=true and always accepts (a partial
// observation is cumulative/incremental, never rejected). A subsequent full
// Observe for the same canon always wins on ID collision (full-beats-partial)
// regardless of origin ordering, since a partial entry never blocks a
// richer observation.
//
// Rows are appended-and-deduped against the existing set (partial adds are
// cumulative, mirroring LazyResourceCache's own merge-by-ID behavior) rather
// than replacing wholesale — a lazy add is always incremental, never a
// verified full replace.
func (s *RowStore) ObservePartial(canon string, rows []resource.Resource) ([]resource.Resource, domain.Gen) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.types[canon]
	merged := append(append([]resource.Resource(nil), existing.Rows...), dedupAgainstExistingRows(existing.Rows, rows)...)

	// Partial is a TYPE-level flag: a sparse add against an already-full
	// entry (Gen!=0 && !Partial) is genuine deeper coverage, not a
	// downgrade — the entry stays full. Every other case (unobserved or
	// already-partial) keeps/sets Partial=true, since a partial add is
	// never itself a verified full replace (see doc comment above).
	partial := existing.Gen == 0 || existing.Partial

	next := TypeRows{
		Rows:       merged,
		Pagination: existing.Pagination,
		TotalCount: existing.TotalCount,
		Origin:     existing.Origin,
		Partial:    partial,
		Gen:        existing.Gen + 1,
		ViewState:  existing.ViewState,
	}
	s.types[canon] = next
	return next.Rows, next.Gen
}

// Amend applies fn to canon's currently retained row slice via copy-on-write:
// fn receives the existing []resource.Resource and returns its replacement,
// so no caller ever observes a torn or partially mutated row and no
// previously-returned Snapshot is invalidated by this call (DEF-7:
// mutate-in-place is exactly the bug class this method exists to remove —
// the two enrich-fold implementations in runtime/helpers.go and
// tui/app_enrich_fold.go both mutate resource.Resource fields in place on a
// shared backing array; Amend is their eventual dual-write / replacement
// target). fn is responsible for its own copy-on-write discipline (returning
// a fresh slice/fresh row values rather than mutating its input in place).
// Bumps and returns Gen even when canon has no rows yet, so callers relying
// on Gen monotonicity are never surprised by a no-op Amend on an absent type.
func (s *RowStore) Amend(canon string, fn func([]resource.Resource) []resource.Resource) domain.Gen {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.types[canon]
	next := existing
	next.Rows = fn(existing.Rows)
	next.Gen = existing.Gen + 1
	s.types[canon] = next
	return next.Gen
}

// SetViewState writes canon's ListViewState (filter/sort/cursor/h-scroll)
// without touching Rows, Pagination, TotalCount, Origin, or Partial —
// the renderer-side counterpart to Observe/ObserveCount/ObservePartial/
// Amend, which in turn never touch ViewState (see TypeRows.ViewState's doc
// comment for why the two are kept orthogonal). Creates an entry even for a
// canon RowStore has never observed rows for, mirroring ObserveCount's same
// allowance — a list can be closed (and its view state cached) before any
// row-carrying write has landed for it in a from-cache seed scenario.
// Bumps Gen unconditionally, matching every other RowStore write.
func (s *RowStore) SetViewState(canon string, vs ListViewState) domain.Gen {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.types[canon]
	next := existing
	next.ViewState = vs
	next.Gen = existing.Gen + 1
	s.types[canon] = next
	return next.Gen
}

// Snapshot returns the currently retained TypeRows for canon (the zero value
// when canon has never been observed). The returned Rows is a defensive
// copy (see cloneRows) — the caller may freely mutate the returned slice,
// its elements, or any row's Fields map without ever affecting the store's
// own state or a later Amend/Observe's copy-on-write result.
func (s *RowStore) Snapshot(canon string) TypeRows {
	s.mu.Lock()
	defer s.mu.Unlock()

	tr := s.types[canon]
	tr.Rows = cloneRows(tr.Rows)
	return tr
}

// SnapshotAll returns a snapshot of every retained type's TypeRows, keyed by
// canonical short name. When includePartial is false, types whose current
// entry is Partial-only are omitted — mirrors the target design's "canonical
// seeds/enrich/saves never see Partial data" scope boundary (C6 scope:
// Partial/lazy entries must never poison a type's canonical persisted list).
// The returned map and each TypeRows.Rows slice are defensive copies (same
// immunity as Snapshot).
func (s *RowStore) SnapshotAll(includePartial bool) map[string]TypeRows {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string]TypeRows, len(s.types))
	for canon, tr := range s.types {
		if tr.Partial && !includePartial {
			continue
		}
		tr.Rows = cloneRows(tr.Rows)
		out[canon] = tr
	}
	return out
}

// Delete drops canon's retained type entry entirely, so a subsequent
// Snapshot(canon) sees the zero value (Gen==0, "never observed") rather than
// a Gen!=0 entry with an empty Rows slice ("observed empty this session").
// Callers that need "reset to never-observed" (as opposed to "observed
// empty", which ObserveCount/Observe with an empty rows slice already
// express) use this instead of writing a zero-value TypeRows back in.
func (s *RowStore) Delete(canon string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.types, canon)
}

// Clear drops every retained type entry. Called from Session.Rotate (C9:
// pair switch discards in-memory state atomically) so a stale pair's rows
// can never leak into the next session.
func (s *RowStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.types = make(map[string]TypeRows)
}

// ClearProbeOrigin drops every retained type entry whose Origin is
// OriginProbe or OriginDisk, leaving OriginFetch entries (a top-level list's
// own fetched rows) untouched. Used by the main-menu Ctrl+R refresh path
// (formerly a reset of the now-removed session.ProbeResources/ProbeTruncated
// maps) so the next Wave-1 probe round populates fresh without blanking an
// already-open resource list's live fetch result.
func (s *RowStore) ClearProbeOrigin() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for canon, tr := range s.types {
		if tr.Origin == OriginProbe || tr.Origin == OriginDisk {
			delete(s.types, canon)
		}
	}
}

// ProbeOriginTypeNames returns the canonical short names of every type this
// session has observed via OriginProbe or OriginDisk at least once (Gen!=0),
// the Wave-1-probe/disk-seed role the removed session.ProbeResources map
// used to play. Used by callers that need the same "has this type been
// retained by a Wave-1 probe/disk seed this session" membership test the old
// map provided, without exposing the rows themselves.
//
// Deliberately NOT gated on len(tr.Rows) > 0: an observed-empty type (a live
// Wave-1 probe or disk seed confirmed zero rows this session, Gen!=0 with an
// empty Rows slice — see Observe's Gen-monotonicity contract) is still a
// completed observation and must be included, so a caller sweeping "every
// type the probe/disk-seed pass has touched this round" (e.g. the
// menuRefreshing ack loop) does not stall waiting for an ack that will never
// arrive for a type that legitimately has nothing to show.
func (s *RowStore) ProbeOriginTypeNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(s.types))
	for canon, tr := range s.types {
		if (tr.Origin == OriginProbe || tr.Origin == OriginDisk) && tr.Gen != 0 {
			names = append(names, canon)
		}
	}
	return names
}

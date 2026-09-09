// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// applyResourcesLoaded stores a page of resources per-screen (on ls.Rows).
// When appendPage is true the new slice is appended; otherwise it replaces.
// Mirrors what ResourceListModel.Update does on ResourcesLoaded.
// Called from handleResourcesLoadedEvent (via Handle) and from the public
// ApplyResourcesLoaded test seam in testing.go.
//
// topLevelCanonical marks whether this call targets the canonical top-level,
// unfiltered ScreenResourceList for typeName (not a ScreenChildList, and not
// a filtered/related-nav list — EscPops/ParentContext) — the same C6 scope
// gate maybeSaveResourceListCache/syncExactTotalToMenu already use. Only a
// topLevelCanonical call routes its accepted rows through
// Core.ObserveRows (task #17 wave 1 stage 4: the RowStore is the single
// per-type row source of truth; decision #1 explicitly rejects letting a
// child/related-filtered list's narrower row set poison it). A
// non-topLevelCanonical call keeps writing ls.Rows locally only, exactly as
// before RowStore existed — its content already came from a store read
// (listScreenResources' fallback, or a related-navigate cache hit) and must
// not be re-written back into the store under the same type key.
//
// Every incoming page is run through MaterializeListFields first (Contract B
// render-sufficiency): Path-based, Key-less columns get their scalar value
// written into Fields while RawStruct is still present, so a later on-disk
// cache replay (which never carries RawStruct) renders identical cells.
// Resources that already have Fields populated (e.g. a cache-replay caller
// passing rows with RawStruct==nil) pass through unchanged — see
// MaterializeListFields's early-return.
//
// fetchErr is non-nil when this call carries a partial-success composite
// error (messages.ResourcesLoaded.Err: some resources returned AND something
// failed) — always nil for a cache-replay seed, which has no fetch error
// concept. Per cache contract C4, a non-nil fetchErr installs its own text as
// the screen's error marker (never just preserves whatever marker a prior,
// unrelated failure may have left behind); a hard failure (no resources at
// all) never reaches this method, routing through
// messages.APIError/ClearActiveListLoadingIntent instead.
func (c *Controller) applyResourcesLoaded(ls *ListState, typeName string, resources []resource.Resource, pagination *resource.PaginationMeta, appendPage bool, loadingMore bool, topLevelCanonical bool, fetchErr error, listSeq domain.Gen) {
	resources = c.materializeListFieldsForType(typeName, resources)

	// Silent-swap findings carry: a silent swap (a non-append replace — the common cold-boot shape
	// where a seeded/cached list is replaced by its own verify-refetch) must
	// never let a row's glyph flash off. The fresh resources argument arrives
	// findings-less on a brand-new session (the Wave-2 enrichment store is
	// empty until the sweep re-enriches this session), so the existing
	// re-apply-from-enrichment-store step below (which only reads
	// c.listEnrichmentFindings) is a no-op on that very first swap. Capture
	// the OUTGOING rows' own findings (r.Findings — what a cache-seeded row
	// carries from disk, or what an earlier live enrichment already wrote
	// onto this screen) BEFORE they are overwritten, keyed by resource ID, so
	// they can be carried onto the incoming replacement rows for any ID that
	// survives the swap.
	priorFindings, priorDetails := outgoingRowFindingsByID(ls, func() []resource.Resource {
		return c.cachedResources(typeName)
	})

	// Silent-swap findings carry (continued): a silent swap is exactly the
	// !appendPage replace path below. Fold the captured prior findings onto the incoming
	// resources for any surviving ID BEFORE they land on ls.Rows/RowStore.
	//
	// A row the latest Wave-2 result answered for is re-folded from that
	// result at the end of this function and needs no carry. A row it did NOT
	// answer for — an id it reported as uninspected, and every row when no
	// result has landed at all — does: nothing has re-checked it, and the
	// re-fold has nothing to give it back. Reading the same uninspected set
	// the result carried (runtime.FoldWave2Rows' rule, one decision for both
	// lanes) is what keeps a row that the file and the probe lane still call
	// broken from rendering clean here after a fresh fetch.
	//
	// Merged per-source, never wholesale: a fresh fetch result IS the
	// authoritative statement about Wave-1 state for that row — a row that
	// comes back with zero Wave-1 findings this time means the fetcher-side
	// condition (e.g. an instance's "stopped" state) is resolved, and carrying
	// the old Wave-1 finding forward would make a fixed issue immortal (stale
	// glyph, stale menu badge, stale persisted Findings). Only the "wave2:"
	// portion of priorFindings — the enrichment pass, which runs separately
	// from the fetch and has genuinely not re-checked this row yet — outlives
	// the swap, and only when the incoming row does not already carry its own
	// Wave-2 entry (never clobber a fresh Wave-2 result that already landed on
	// this exact swap; the "wave2:" Source prefix is the same discipline
	// ApplyWave2ToRow uses elsewhere).
	if !appendPage && len(priorFindings) > 0 {
		answeredByLatest := len(c.listEnrichmentFindings(typeName)) > 0
		uninspected := c.listUninspectedIDs(typeName)
		for i := range resources {
			if answeredByLatest && !uninspected[resources[i].ID] {
				continue
			}
			f, ok := priorFindings[resources[i].ID]
			if !ok || len(f) == 0 || hasWave2Finding(resources[i].Findings) {
				continue
			}
			ad := priorDetails[resources[i].ID]
			for _, pf := range f {
				if strings.HasPrefix(pf.Source, "wave2:") {
					resources[i].Findings = append(resources[i].Findings, pf)
					if detail, ok := ad[pf.Code]; ok {
						if resources[i].AttentionDetails == nil {
							resources[i].AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, 1)
						}
						resources[i].AttentionDetails[pf.Code] = detail
					}
				}
			}
		}
	}

	// --- Per-screen storage ------------------------------------------------
	// Writing to ls.Rows ensures that two stacked list screens of the same
	// resource type never share a row slice. Each screen's fetch result lands
	// exclusively on that screen's ListState.
	//
	// Append-dedup backstop (D13): an append must never introduce a row whose ID already
	// exists on the screen. This is a backstop, not the fix for the root
	// cause below — it only prevents a duplicate that already reached this
	// call from becoming visible; the empty-cursor guard is what stops the
	// duplicate fetch from happening in the first place.
	//
	// topLevelCanonical routes the SAME decision (append/replace) through
	// Core.ObserveRows first and adopts its accepted slice onto
	// ls.Rows/ls.RowsGen — RowStore is the reconciler, this screen adopts
	// what it accepted (task #17 wave 1 stage 4). A non-canonical screen
	// (child/filtered/related) keeps the pre-RowStore local-only behavior:
	// its own dedup-append/replace decision, written only to ls.Rows, never
	// observed into the shared per-type store.
	if ls != nil {
		switch {
		case topLevelCanonical:
			accepted, gen := c.core.ObserveRows(typeName, resources, pagination, session.OriginFetch, appendPage)
			ls.Rows = accepted
			ls.RowsGen = gen
			ls.rowsVersion++
		case appendPage:
			ls.Rows = append(ls.Rows, dedupAgainstExisting(ls.Rows, resources)...)
			ls.rowsVersion++
		default:
			ls.Rows = resources
			ls.rowsVersion++
		}
	}

	if ls != nil {
		// Cache-first seeding contract: a fetch result landing clears the
		// in-flight flag ITS OWN REQUEST raised (loadingMore, recorded on the
		// request — never inferred from appendPage, which answers a different
		// question: how the rows merge) via the shared clearFetchInFlight
		// choke point, leaving a
		// concurrently in-flight sibling request's flag (LoadingMore vs
		// Refreshing are allowed to overlap) untouched — the seeded (or
		// now-replaced) rows are confirmed for THIS request only. Callers that
		// seed rows from a cache-first source set Refreshing=true themselves
		// AFTER calling this method, so this clear only ever fires for a
		// genuine fetch-result swap, never undoing the seed-time flag.
		ls.clearFetchInFlight(loadingMore, listSeq)
		// Seed-time provisional total: a fetch result retires the seed's
		// population only when it actually supersedes it — an EXACT result
		// (authoritative proof of the new total, C5), or one that already
		// reaches the seeded population. A still-truncated result shallower
		// than the seed knew has verified only part of the list and cannot
		// downgrade the total to its own row depth; that regression is the
		// "N+" frame where N shrank back to the last-known page.
		if pagination == nil || !pagination.IsTruncated || len(ls.Rows) >= ls.TotalCount {
			ls.TotalCount = 0
		}
		// Per cache contract C4: a successful fetch result clears any outstanding error
		// marker from a previous failed attempt. A partial-failure result
		// (fetchErr non-nil) installs THIS fetch's own error text instead — the
		// fetch that just landed did not actually resolve cleanly, so C4's
		// "keeps the content, swaps the marker for an error marker" applies,
		// and the marker must reflect the failure that just happened, never a
		// stale marker (or no marker at all) left over from an unrelated
		// earlier attempt.
		if fetchErr != nil {
			ls.setFetchError(fetchErr.Error())
		} else {
			ls.LastFetchError = ""
		}
		// Two facts, two fields. HasPagination answers "is there another page
		// to fetch", which only the fetch's own cursor can say. Whether this
		// result may be recorded as the type's exact population is a different
		// question: a partial-success result (some resources landed AND
		// something failed — e.g. the IAM policy fetcher's inline-group
		// enumeration failing while managed-policy pagination reports
		// IsTruncated=false) reached the last page and still cannot speak for
		// the whole population, because a DIFFERENT component of the fetch is
		// what failed. Answering both from HasPagination titled such a list
		// "N+" and offered a load-more with no cursor behind it.
		switch {
		case pagination != nil:
			ls.HasPagination = pagination.IsTruncated
			ls.PaginationCursor = pagination.NextToken
		default:
			ls.HasPagination = false
			ls.PaginationCursor = ""
		}
		// The incompleteness is sticky across one list's pages; the pagination
		// answer is this page's alone.
		ls.FetchIncomplete = (appendPage && ls.FetchIncomplete) || fetchErr != nil
		ls.PopulationUnconfirmed = ls.HasPagination || ls.FetchIncomplete
	}

	// Fresh rows arrive without Wave-2 findings; re-apply the latest known
	// findings from the enrichment store so a silent-swap refetch never leaves
	// the controller rows (and therefore the list-open save path, which
	// persists row findings per C6)
	// glyph-blind until the next EnrichmentChecked. Mirrors the session-side
	// fold, which re-applies onto Core stores after every result lands. Uses
	// listEnrichmentFindings (every independently-evaluated Wave-2 Finding
	// per resource) so a multi-condition resource keeps every Finding across
	// a silent-swap refetch, not just the worst one.
	if known := c.listEnrichmentFindings(typeName); len(known) > 0 {
		// The same uninspected set the result carried: a row the last Wave-2
		// pass could not inspect keeps whatever the silent-swap carry just
		// re-attached, instead of being folded clean by a map that never had
		// an answer for it.
		c.applyRowFindings(typeName, known, c.listEnrichmentDetails(typeName), c.listUninspectedIDs(typeName))
	}
}

// outgoingRowFindingsByID captures the findings (and their companion
// AttentionDetail rows) currently attached to the row set about to be
// replaced by a silent swap (the silent-swap findings carry above), keyed by
// resource ID. Prefers ls.Rows
// (the per-screen store applyResourcesLoaded is about to overwrite) since it
// is the richer, currently-displayed source; falls back to the
// RowStore-backed type cache when ls is nil or carries no rows yet (e.g. the
// very first ResourcesLoaded for a screen whose seed only populated the type
// cache). Rows with no findings are omitted so the caller's
// len(priorFindings) == 0 check short-circuits cheaply when there is nothing
// to carry forward. The second return value is only ever consulted for a
// FindingCode present in the first, so it is captured verbatim (r.
// AttentionDetails itself, not a filtered copy) with no cost to the common
// no-findings path.
func outgoingRowFindingsByID(ls *ListState, cachedRows func() []resource.Resource) (map[string][]domain.Finding, map[string]map[domain.FindingCode]domain.AttentionDetail) {
	// The fallback is a function, not a slice: reading the type cache clones
	// every row it holds, and the common case (a screen that already has rows)
	// never looks at it. Passed eagerly, that clone was a full copy of the
	// type's rows on every result, under the controller lock, for nobody.
	var source []resource.Resource
	if ls != nil && len(ls.Rows) > 0 {
		source = ls.Rows
	} else {
		source = cachedRows()
	}
	if len(source) == 0 {
		return nil, nil
	}
	out := make(map[string][]domain.Finding, len(source))
	var details map[string]map[domain.FindingCode]domain.AttentionDetail
	for _, r := range source {
		if len(r.Findings) > 0 {
			out[r.ID] = r.Findings
		}
		if len(r.AttentionDetails) > 0 {
			if details == nil {
				details = make(map[string]map[domain.FindingCode]domain.AttentionDetail, len(source))
			}
			details[r.ID] = r.AttentionDetails
		}
	}
	return out, details
}

// dedupAgainstExisting returns the subset of incoming whose ID is not already
// present in existing (C2/C6): an append that would introduce a row already
// on the screen is dropped rather than shown twice. Delegates to
// resource.DedupByID, the single-source implementation.
func dedupAgainstExisting(existing, incoming []resource.Resource) []resource.Resource {
	return resource.DedupByID(existing, incoming)
}

// materializeListFieldsForType resolves the column set for typeName the same
// way buildListBody does (fallback typeDef first, then catalog) and runs
// every resource through MaterializeListFields. Resources without a matching
// column set (typeName unregistered, no columns resolved) pass through
// unchanged.
func (c *Controller) materializeListFieldsForType(typeName string, resources []resource.Resource) []resource.Resource {
	if len(resources) == 0 {
		return resources
	}
	columns := c.resolveColumnsLocked(typeName)
	if len(columns) == 0 {
		return resources
	}
	out := make([]resource.Resource, len(resources))
	for i, r := range resources {
		out[i] = MaterializeListFields(r, columns)
	}
	return out
}

// SanitizedRows is the boundary AWS-supplied text crosses to become something
// a9s paints: a tag value, a description or a name is written by whoever can
// tag the resource, and it arrives carrying whatever they put in it, escape
// sequences included. Every lane that hands a page of rows to the controller
// calls this on its way in.
//
// It is called BEFORE the controller lock, never under it: a page is twelve
// thousand rows on a large account, and a per-row pass held against every
// reader is the shape the absorb latency pin exists to forbid. Nothing is
// written in place for the same reason it runs there — the page is still the
// fetch lane's, and the cache writer may already be copying an earlier one
// that shares its rows.
func SanitizedRows(resources []resource.Resource) []resource.Resource {
	out := make([]resource.Resource, len(resources))
	for i := range resources {
		out[i] = resources[i].Sanitized()
	}
	return out
}

// listBodyMemo caches the expensive part of buildListBody's output — the
// resolved columns, the filtered+sorted+decorated row set, and the two
// column-index lookups derived from that column set — keyed on every input
// that can change what they contain. Rebuilt only on a key mismatch; a
// cursor move, spinner tick, or any other re-render whose inputs are
// unchanged reuses columns/rows/identityCol/statusCol verbatim.
//
// Selected/ScrollX/Filter/Sort/AttentionOnly/Loading/Truncated/Pagination/
// EnrichmentFindings/EnrichmentTruncated/LoadingMore/Refreshing/
// LastFetchError are NOT cached here — they are cheap to read fresh from ls
// (and the controller's enrichment maps) on every call, cache or no cache,
// so buildListBody always sets them directly on the returned ListBody.
type listBodyMemo struct {
	valid           bool
	rowsVersion     uint64
	fallbackRowsGen domain.Gen
	filter          string
	attnOnly        bool
	sortCol         string
	sortDir         string
	enrichGen       uint64

	columns []ColumnDef
	rows    []ListRow
	// visible is the filtered+sorted resource set the rows were built from,
	// in the same order. The frame title's count and the two cursor accessors
	// read it instead of running the chain again per call — a second run is
	// O(n) on every keystroke and can disagree with what is on screen.
	visible     []resource.Resource
	identityCol int
	statusCol   int
}

// buildListBody constructs a ListBody from the top list screen's ListState and
// the controller's resource + enrichment caches. Mirrors applySortAndFilter +
// the row/cell extraction in ResourceListModel — producing the same logical
// rows/cells/order/decorators so RenderList parity holds.
//
// The expensive part (column resolution, filter+sort, per-row cell
// extraction/decoration) runs only when ls.bodyMemo's key does not match the
// current inputs — see listBodyMemo. Callers must hold c.mu for WRITE (not
// just read): a cache miss populates ls.bodyMemo, which is a Controller-owned
// mutation of ListState, not a pure read. Controller.Snapshot (the sole
// external entry point that reaches here) acquires c.mu.Lock() for exactly
// this reason.
func (c *Controller) buildListBody(ctx runtime.ScreenContext, ls *ListState) *ListBody {
	typeName := ctx.ResourceType

	td := c.typeDefForLocked(typeName)

	memo := &ls.bodyMemo
	fallbackRowsGen := c.fallbackRowsGenFor(ls, typeName)
	if c.listBodyMemoStale(*memo, ls, fallbackRowsGen) {
		*memo = c.captureListBodyBuild(ls, typeName, td, fallbackRowsGen).run()
	}

	// Clamp selected row against the (possibly cached) visible row count.
	selected := ls.SelectedRow
	if len(memo.rows) > 0 && selected >= len(memo.rows) {
		selected = len(memo.rows) - 1
	}
	if selected < 0 {
		selected = 0
	}

	enrichTruncated := map[string]bool{}
	if c.enrichmentTruncated != nil {
		maps.Copy(enrichTruncated, c.enrichmentTruncated)
	}

	pagination := PaginationInfo{}
	if ls.HasPagination {
		pagination.HasMore = true
		pagination.Cursor = ls.PaginationCursor
	}

	return &ListBody{
		Columns:             memo.columns,
		Rows:                memo.rows,
		Selected:            selected,
		ScrollX:             ls.ScrollX,
		Filter:              ls.Filter,
		Sort:                SortSpec{Col: ls.SortCol, Dir: ls.SortDir},
		AttentionOnly:       ls.AttentionOnly,
		Loading:             ls.Loading,
		Truncated:           ls.HasPagination,
		Pagination:          pagination,
		EnrichmentFindings:  c.listEnrichmentFindings(typeName),
		EnrichmentTruncated: enrichTruncated,
		IdentityCol:         memo.identityCol,
		StatusCol:           memo.statusCol,
		LoadingMore:         ls.LoadingMore,
		Refreshing:          ls.Refreshing,
		LastFetchError:      ls.LastFetchError,
	}
}

// listBodyMemoStale reports whether a memo was built from inputs that have
// since moved. The one owner of that comparison: buildListBody asks it before
// reusing a memo, and installListBodyMemo asks it before adopting one built
// off the lock, so a prewarmed memo can never be installed over state it does
// not describe. Callers must hold c.mu.
func (c *Controller) listBodyMemoStale(memo listBodyMemo, ls *ListState, fallbackRowsGen domain.Gen) bool {
	return !memo.valid ||
		memo.rowsVersion != ls.rowsVersion ||
		memo.fallbackRowsGen != fallbackRowsGen ||
		memo.filter != ls.Filter ||
		memo.attnOnly != ls.AttentionOnly ||
		memo.sortCol != ls.SortCol ||
		memo.sortDir != ls.SortDir ||
		memo.enrichGen != c.enrichmentGen
}

// captureTopListBodyBuild freezes the build inputs of the top screen's list
// body, when there is one and its memo is stale. ok=false means there is
// nothing worth prewarming (no list on top, or its memo already answers), and
// the caller skips straight to the snapshot. Callers must hold c.mu.
func (c *Controller) captureTopListBodyBuild() (listBodyBuild, bool) {
	if len(c.stack) == 0 {
		return listBodyBuild{}, false
	}
	top := c.stack[len(c.stack)-1]
	ls := top.State.List
	if ls == nil {
		return listBodyBuild{}, false
	}
	typeName := top.Ctx.ResourceType
	fallbackRowsGen := c.fallbackRowsGenFor(ls, typeName)
	if !c.listBodyMemoStale(ls.bodyMemo, ls, fallbackRowsGen) {
		return listBodyBuild{}, false
	}
	return c.captureListBodyBuild(ls, typeName, c.typeDefForLocked(typeName), fallbackRowsGen), true
}

// installListBodyMemo adopts a memo built off the lock, onto the screen it was
// built for — but only while it still describes that screen. Anything that
// moved in the meantime (another result landed, a filter changed, enrichment
// arrived) makes it stale, and the next buildListBody rebuilds under the lock
// as it always did: the memo is a cache, never a source of truth. Callers must
// hold c.mu.
func (c *Controller) installListBodyMemo(build listBodyBuild, memo listBodyMemo) {
	if len(c.stack) == 0 {
		return
	}
	top := c.stack[len(c.stack)-1]
	ls := top.State.List
	// The instance, not the type: two screens of one type have equal memo keys
	// whenever their rows, filter and sort agree, so a body built for a screen
	// that has since been popped would install into the one underneath and
	// render its predecessor's rows.
	if ls == nil || ls.instance != build.instance {
		return
	}
	if c.listBodyMemoStale(memo, ls, build.fallbackRowsGen) {
		return
	}
	ls.bodyMemo = memo
}

// fallbackRowsGenFor is the memo key's second half: a screen with no rows of
// its own renders the type's store entry, so that entry's generation is what
// invalidates the memo. Zero for a screen that holds its own rows.
// Callers must hold c.mu.
func (c *Controller) fallbackRowsGenFor(ls *ListState, typeName string) domain.Gen {
	if ls.Rows != nil {
		return 0
	}
	return c.core.AnyOriginResourceCacheGen(typeName)
}

// listVisibleLocked returns the filtered+sorted resource set for ls — the
// rows on screen, in screen order. It reads the body memo when the memo
// answers for the current state, and runs the same chain the build runs when
// it does not. The one caller of that chain outside the build.
//
// Never memoises: a read lock is enough for every caller here, and populating
// the memo is a mutation. A cold screen therefore pays the chain once per
// call until the next snapshot builds the body — which is what these callers
// did on every call, warm or cold, before.
// Callers must hold c.mu (read is enough).
func (c *Controller) listVisibleLocked(ls *ListState, typeName string) []resource.Resource {
	if !c.listBodyMemoStale(ls.bodyMemo, ls, c.fallbackRowsGenFor(ls, typeName)) {
		return ls.bodyMemo.visible
	}
	// One resolution for both halves, the same pairing the off-lock build uses:
	// the filter and the sort read the columns the rows are made of.
	td, columns := c.listColumnsAndTypeLocked(typeName)
	visible := applyListFiltersWith(ls, td, columns, c.listEnrichmentFindings(typeName),
		c.listScreenResources(ls, typeName))
	return listSortResources(columns, td, ls, visible)
}

// listBodyBuild is one list body's build inputs, frozen under the controller
// lock so the O(n log n) filter+sort and the O(n·cols) cell extraction can run
// WITHOUT it — the absorb half of the C4 latency guarantee (a 6000-row result
// used to hold the lock for its whole row pass, and every key press waited
// behind it).
//
// ls is a detached copy of the screen's ListState, and its Rows a shallow copy
// of the resolved row set: the build reads them while other writers keep
// mutating the live screen. A shallow row copy is enough because a row's
// Findings and AttentionDetails are REPLACED by every writer that touches them
// (runtime.ApplyWave2ToRow's discipline, relied on by applyRowFindings' own
// amend), never written into in place — so the structs this copy holds are
// stable for the length of the build.
type listBodyBuild struct {
	ls              *ListState
	typeName        string
	td              *resource.ResourceTypeDef
	instance        domain.Gen
	columns         []ColumnDef
	findings        map[string][]domain.Finding
	uninspected     map[string]bool
	enrichGen       uint64
	fallbackRowsGen domain.Gen
}

// captureListBodyBuild freezes everything run() reads. Callers must hold c.mu.
func (c *Controller) captureListBodyBuild(ls *ListState, typeName string, td *resource.ResourceTypeDef, fallbackRowsGen domain.Gen) listBodyBuild {
	// Build the row set from the per-screen store (Bug 1 fix: uses ls.Rows when
	// available so two stacked same-type screens see their own independent rows).
	detached := ls.cloneForBuild(c.listScreenResources(ls, typeName))
	return listBodyBuild{
		ls:       &detached,
		typeName: typeName,
		instance: ls.instance,
		td:       td,
		// One typeDef per screen (row 39) means one column set: the cells the
		// rows are made of, the strings the text filter compares and the
		// column a sort names are the same list, resolved here once. Resolved
		// from the already-resolved td rather than the catalog, so a test
		// typeDef with non-standard first columns is not silently switched to
		// the built-in defaults.
		columns:         resolveListColumnsForBuild(c.viewConfig, typeName, td),
		findings:        c.listEnrichmentFindings(typeName),
		uninspected:     c.listUninspectedIDs(typeName),
		enrichGen:       c.enrichmentGen,
		fallbackRowsGen: fallbackRowsGen,
	}
}

// run performs the filter+sort and the per-row cell extraction/decoration
// pass, returning a memo stamped with the key its inputs carried. Pure: it
// touches no controller state and holds no lock.
func (b listBodyBuild) run() listBodyMemo {
	// Apply filters (relatedIDSet -> text -> attention).
	visible := applyListFiltersWith(b.ls, b.td, b.columns, b.findings, b.ls.Rows)

	// Sort over the same resolved set the cells come from, so a user-configured
	// sort_key column resolves correctly and the
	// comparator sees the identity election.
	visible = listSortResources(b.columns, b.td, b.ls, visible)

	// Build rows.
	statusCol := resolveListStatusCol(b.columns, b.td)
	rows := make([]ListRow, 0, len(visible))
	for _, r := range visible {
		cells := extractListCells(b.columns, r, b.td)
		severity, colorTag := resolveListRowSeverity(b.td, r)
		// S4: bake the Wave-2 issue-Finding Phrase into the status cell, from the
		// same enrichment findings map that drives the glyph. Without this the web
		// renders a blank Status for flagged rows in live mode (the cell only
		// reflects Wave-1 r.Findings / a FieldUpdate that live never applies),
		// while the glyph still shows — a web/TUI parity break. Mirrors the
		// render-time override in resourcelist.go's renderListDataRow.
		//
		// This override is only a fallback for the case where Wave-2 enrichment
		// has landed in the enrichment-store map but has NOT yet been mutated
		// onto r.Findings (the live-mode lag the comment above describes). When
		// r.Findings already carries a Wave-2 entry for this resource (the demo
		// path and the fold-layer live path both mutate r.Findings directly),
		// extractListCells has already derived the correct cell from
		// domain.StatusPhrase(r.Findings) — including the "<top> (+N)"
		// stacking notation for multi-finding rows. This fallback instead picks
		// the WORST-severity entry from the enrichment-store map's per-resource
		// slice (a resource may carry more than one independently-evaluated
		// Wave-2 condition) and bakes only its bare phrase — no stacking
		// notation — so applying it on top of an already-stacked cell would
		// silently drop the "(+N)" suffix and/or clobber a higher-priority
		// Wave-1 phrase with a same-or-lower-severity Wave-2 one.
		phrased := domain.StatusPhrase(r.Findings) != ""
		if statusCol >= 0 && statusCol < len(cells) && !hasWave2Finding(r.Findings) {
			if fs, ok := b.findings[r.ID]; ok && len(fs) > 0 {
				if f := domain.WorstSeverityFinding(fs); f.Severity.IsIssue() && f.Phrase != "" {
					cells[statusCol] = f.Phrase
					phrased = true
				}
			}
		}
		// A row the enricher could not inspect says so, in the one cell an
		// operator reads for the check's answer. A phrase wins: a row that
		// already reports something concrete is not "unknown". The colour is
		// deliberately left alone — an uninspected row is not an issue, so it
		// must neither tint the row nor bump a badge.
		if statusCol >= 0 && statusCol < len(cells) && !phrased && b.uninspected[r.ID] {
			cells[statusCol] = domain.NotInspectedPhrase
		}
		rows = append(rows, ListRow{
			Cells:      cells,
			Severity:   severity,
			ResourceID: r.ID,
			Color:      colorTag,
		})
	}

	return listBodyMemo{
		valid:           true,
		visible:         visible,
		rowsVersion:     b.ls.rowsVersion,
		fallbackRowsGen: b.fallbackRowsGen,
		filter:          b.ls.Filter,
		attnOnly:        b.ls.AttentionOnly,
		sortCol:         b.ls.SortCol,
		sortDir:         b.ls.SortDir,
		enrichGen:       b.enrichGen,
		columns:         widenStatusColumn(b.columns, rows, statusCol),
		rows:            rows,
		// Resolve the identity column index (full column list, before hscroll).
		identityCol: IdentityColumnIndex(b.columns, b.td),
		statusCol:   statusCol,
	}
}

// setFetchError installs the error marker a failed fetch leaves over the rows
// it could not replace. The text is an AWS response's own message, which
// quotes the input it rejected — a bucket name, a tag value — so it reaches
// the screen by the same lane a row does and crosses the same boundary.
func (ls *ListState) setFetchError(text string) {
	ls.LastFetchError = domain.Sanitize(text)
}

// widenStatusColumn returns columns with the status column wide enough for the
// widest status cell in rows. Every column publishes the width the painter
// fills, and the status column is the one whose content the type cannot
// declare a width for: the phrase is written by an enricher, per row, after
// the column was declared. So it is widened where the rest of the layout is
// decided — a lane with no renderer of its own reads the same number the
// terminal paints, and a caller that narrows it is obeyed.
func widenStatusColumn(columns []ColumnDef, rows []ListRow, statusCol int) []ColumnDef {
	if statusCol < 0 || statusCol >= len(columns) {
		return columns
	}
	w := columns[statusCol].Width
	for _, row := range rows {
		if statusCol < len(row.Cells) {
			w = max(w, lipgloss.Width(row.Cells[statusCol]))
		}
	}
	if w == columns[statusCol].Width {
		return columns
	}
	out := slices.Clone(columns)
	out[statusCol].Width = w
	return out
}

// ListFrameTitle mirrors FrameTitle in ResourceListModel.
func (c *Controller) ListFrameTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return ""
	}
	top := c.stack[len(c.stack)-1]
	return c.buildListFrameTitle(top.Ctx, ls)
}

// buildListFrameTitle computes the frame title string for a list screen,
// mirroring FrameTitle() in resourcelist.go.
//
// Per docs/attention-signals.md §Visualization Surfaces: the " !N" issue
// suffix is UNCONDITIONAL — it renders after the count parentheses on any
// list screen (top-level or ScreenChildList), on any renderer (TUI or web),
// whenever N > 0 and the screen is not in attention-only mode. N is the
// current list's issue count — aggregated the same way as the menu badge
// (Controller.GetListIssueCount's algorithm, mirrored lock-free below since
// buildListFrameTitle already runs under c.mu). N=0 renders no suffix. When
// the Wave-2 enrichment issue count itself is a truncated lower bound
// (c.enrichmentTruncated[typeName]), the suffix becomes " !N+" instead of
// " !N". The suffix is OMITTED entirely in attention-only (ctrl+z) mode,
// where the filtered count already IS the issue count and an " !N" suffix
// would be redundant.
func (c *Controller) buildListFrameTitle(ctx runtime.ScreenContext, ls *ListState) string {
	typeName := ctx.ResourceType
	name := typeName

	td := resource.FindResourceType(typeName)
	if td != nil && td.ListTitle != "" {
		name = td.ListTitle
	}
	if ls.DisplayName != "" {
		name = ls.DisplayName
	}
	if ls.Loading {
		return name
	}

	allResources := c.listScreenResources(ls, typeName)
	total := len(allResources)
	if ls.RelatedIDSet != nil {
		// A related drill's population is what its own check named, not the
		// type's. Its rows arrive as the type's whole list and are prefiltered
		// to the set, so counting the unfiltered set here titles ten instances
		// built from one AMI as the account's whole fleet. The seed-time
		// provisional total below is the type's too, for the same reason.
		total = len(relatedIDSubset(ls, allResources))
	} else {
		// Seed-time provisional total: a seeded-but-unverified list knows the
		// population its seed source reported (cache.TypeFile.Population) while
		// holding only the rows that source retained. Prefer it for display
		// until a fetch result supersedes it (applyResourcesLoaded). Only the
		// displayed total is overridden; filtered still reflects the rows
		// actually on screen.
		total = max(total, ls.TotalCount)
	}
	filtered := len(c.listVisibleLocked(ls, typeName))
	truncated := ls.HasPagination

	totalStr := strconv.Itoa(total)
	if truncated {
		totalStr = strconv.Itoa(total) + "+"
	}

	// Loading-more indicator goes inside the count parentheses, mirroring the
	// baseline FrameTitle: "ec2(200+ loading...)" not "ec2(200+) loading...".
	if ls.LoadingMore {
		return name + "(" + totalStr + " loading...)"
	}

	isAttention := ls.AttentionOnly
	hasTextFilter := ls.Filter != "" && filtered != total

	var title string
	switch {
	case hasTextFilter && isAttention:
		title = name + "(" + strconv.Itoa(filtered) + " of " + totalStr + ")"
	case hasTextFilter:
		title = name + "(" + strconv.Itoa(filtered) + "/" + totalStr + ")"
	case isAttention:
		title = name + "(" + strconv.Itoa(filtered) + " of " + totalStr + ")"
	default:
		title = name + "(" + totalStr + ")"
	}

	if ls.TitleSuffix != "" {
		title += ls.TitleSuffix
	}
	if !isAttention {
		if issueCount := c.listIssueCount(ls, typeName); issueCount > 0 {
			title += " !" + strconv.Itoa(issueCount)
			if c.enrichmentTruncated[typeName] {
				title += "+"
			}
		}
	}
	if isAttention {
		title += " [!]"
	}
	return title
}

// ListSelected returns the resource at the current cursor position in the
// visible (filtered+sorted) row set, plus a navigable bool (always true when
// a resource is present — row-dependent guards are wired in the flip step).
// The selection index is clamped to the visible count before indexing (Bug 2
// fix) so that a refresh that shrinks the list never leaves the cursor pointing
// past the end, causing Enter/copy to silently do nothing while a row is
// visually highlighted.
func (c *Controller) ListSelected() (resource.Resource, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.listSelected()
}

// listSelected is the lock-free core of ListSelected. Callers MUST already hold
// c.mu (e.g. applyLocked dispatching a row-dependent action) — taking the lock
// again would self-deadlock the non-reentrant RWMutex.
func (c *Controller) listSelected() (resource.Resource, bool) {
	ls := c.topListState()
	if ls == nil {
		return resource.Resource{}, false
	}
	top := c.stack[len(c.stack)-1]
	typeName := top.Ctx.ResourceType

	visible := c.listVisibleLocked(ls, typeName)

	if len(visible) == 0 {
		return resource.Resource{}, false
	}
	// Clamp to match buildListBody so the caller always gets the row that the
	// UI is actually highlighting, even when a concurrent refresh shrinks the
	// visible count below the stored SelectedRow.
	idx := ls.SelectedRow
	if idx >= len(visible) {
		idx = len(visible) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return visible[idx], true
}

// GetListIssueCount returns the number of resources with issue status in the
// top list screen's resource type, mirroring IssueCount() in ResourceListModel.
func (c *Controller) GetListIssueCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil || len(c.stack) == 0 {
		return 0
	}
	top := c.stack[len(c.stack)-1]
	return c.listIssueCount(ls, top.Ctx.ResourceType)
}

// listIssueCount is the lock-free core of GetListIssueCount. Callers MUST
// already hold c.mu (e.g. buildListFrameTitle, which runs under c.mu) —
// taking the lock again would self-deadlock the non-reentrant RWMutex.
// Mirrors the ListSelected()/listSelected() split in this file.
func (c *Controller) listIssueCount(ls *ListState, typeName string) int {
	tdp := c.typeDefForLocked(typeName)
	if tdp == nil {
		return 0
	}
	td := *tdp
	// S1 contract: the list-title suffix and the menu sync-back use the SAME
	// aggregation as the menu badge — and the badge never counts types with
	// ExcludeFromIssueBadge (e.g. ct-events, where "issue-colored" rows are
	// historical events, not live problems). Without this, an excluded type
	// would grow a title suffix and, via the app_stack sync-back, a forbidden
	// menu badge after its list was visited.
	if td.ExcludeFromIssueBadge {
		return 0
	}
	// A related drill's badge counts the drill's own rows, through the same
	// prefilter its count and its rows read (relatedIDSubset). Counting the
	// screen's whole row set instead titles ten instances built from one AMI
	// with the account's issue total, so the badge says more issues than the
	// list has rows.
	all := relatedIDSubset(ls, c.listScreenResources(ls, typeName))
	findings := c.listEnrichmentFindings(typeName)
	ic := 0
	for _, r := range all {
		switch {
		case listHasBadgeFinding(r):
			ic++
		case td.ResolveColor(runtime.Wave1Only(r)).IsIssue():
			// S1 contract (docs/attention-signals.md): a lone Wave-2 "~" warn
			// must NOT bump the count — a warning is not an issue. runtime.Wave1Only
			// strips merged Wave-2 findings before ResolveColor so this branch
			// counts Wave-1 issue-colored rows only, matching the menu badge's
			// unifiedIssueCount exactly (both call Wave1Only, so they cannot
			// drift). Wave-2 "!"-severity findings are counted by the
			// listHasBadgeFinding branch above and the findings-map branch below.
			ic++
		case len(r.Findings) == 0:
			if fs, hasFinding := findings[r.ID]; hasFinding && len(fs) > 0 && domain.WorstSeverityFinding(fs).Severity == domain.SevBroken {
				// S1: Wave-2 findings bump the count only at "!" severity —
				// "~ findings do not bump" (docs/attention-signals.md). Wave-1
				// yellow/red rows are already counted by the color branch above,
				// matching the menu badge's probe-side aggregation. A resource
				// may carry more than one independently-evaluated Wave-2
				// condition; it counts once if ANY of them is SevBroken.
				ic++
			}
		}
	}
	return ic
}

// GetListVisibleResources returns the visible (filtered+sorted) resource slice
// for the top list screen, for test introspection.
func (c *Controller) GetListVisibleResources() []resource.Resource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil || len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	typeName := top.Ctx.ResourceType
	return c.listVisibleLocked(ls, typeName)
}

// ApplyListFieldUpdates merges Wave-2 field updates into the cached resource
// slice for typeName. Keyed by resource ID then field key.
// Updates are applied both to the top list screen's ls.Rows (the primary read
// path after the Bug 1 fix) and to the RowStore-backed type cache (for
// callers such as GetListAllResources that don't have a specific ListState).
func (c *Controller) ApplyListFieldUpdates(typeName string, updates map[string]map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyListFieldUpdates(typeName, updates)
}

// applyFieldUpdatesToSlice merges updates into a resource slice, replacing
// each touched row's Fields map rather than writing into it — so it is equally
// correct on a screen's own rows and on the copy Core.AmendRows hands the
// RowStore, and both callers below share it.
func applyFieldUpdatesToSlice(rows []resource.Resource, updates map[string]map[string]string) {
	for i := range rows {
		kvMap, ok := updates[rows[i].ID]
		if !ok {
			continue
		}
		// Replace the map, never write into it — the same copy-on-write the
		// RowStore amend below states. A row's Fields map is shared with every
		// other holder of that row value: the store's own snapshot, a stacked
		// screen's slice, and the off-lock list-body build (listBodyBuild),
		// which reads these maps while this runs. Writing a key in place is a
		// mutation none of them can see coming.
		fresh := make(map[string]string, len(rows[i].Fields)+len(kvMap))
		maps.Copy(fresh, rows[i].Fields)
		maps.Copy(fresh, kvMap)
		rows[i].Fields = fresh
	}
}

// applyListFieldUpdates is the lock-free body of ApplyListFieldUpdates so it can
// be called from the intents handler, which already holds c.mu. Callers must
// hold c.mu (write).
func (c *Controller) applyListFieldUpdates(typeName string, updates map[string]map[string]string) {
	if len(updates) == 0 {
		return
	}
	// Apply to EVERY list screen of this type in the stack (primary read path).
	// PatchResourceList updates every matching ResourceListModel, so the matching
	// list may be stacked beneath a detail or another list. Targeting only the top
	// screen leaves a stacked same-type list's per-screen Rows stale (or wrongly
	// mutates a top list of a different type), and buildListBody prefers ls.Rows
	// over the type cache, so popping back would render stale cell values.
	canon := resource.CanonicalShortName(typeName)
	for i := range c.stack {
		s := &c.stack[i]
		if s.ID != runtime.ScreenResourceList && s.ID != runtime.ScreenChildList {
			continue
		}
		st := s.Ctx.ResourceType
		if td := resource.FindResourceType(st); td != nil {
			st = td.ShortName
		}
		if st != canon || s.State.List == nil {
			continue
		}
		applyFieldUpdatesToSlice(s.State.List.Rows, updates)
		s.State.List.rowsVersion++
	}
	// Also update the RowStore-backed type cache so GetListAllResources etc.
	// see the same values. Amend's copy-on-write contract (RowStore.Amend's
	// doc comment) is the same rule applyFieldUpdatesToSlice already follows,
	// so this walks the fresh slice through it rather than spelling the
	// per-row map clone a second time.
	c.core.AmendRows(canon, func(rows []resource.Resource) []resource.Resource {
		if len(rows) == 0 {
			return rows
		}
		out := make([]resource.Resource, len(rows))
		copy(out, rows)
		applyFieldUpdatesToSlice(out, updates)
		return out
	})
}

// ClearRowFindings strips every Wave-2 finding (domain.Finding with Source
// prefixed "wave2:") from the cached resource rows for typeName, on both
// per-screen ls.Rows and the RowStore-backed type cache — mirroring the
// dual-store walk in applyListFieldUpdates. AttentionDetails entries keyed by
// a stripped Wave-2 Finding's Code are dropped alongside it.
//
// This is the controller-side half of a Ctrl+R refresh's stale-findings
// cleanup. internal/tui's Model.applyEnrichment strips Wave-2 findings from
// the session-owned stores (Core.ResourceCache / LazyResourceCache /
// ProbeResources); those stores no longer alias the controller's rows since
// applyResourcesLoaded materializes copies (MaterializeListFields) into
// ls.Rows / the RowStore. Without this explicit companion call, a
// ResourceListModel constructed from the (now-copied) cache-hit rows retains
// stale Wave-2 findings across Ctrl+R even after the session-side clear.
func (c *Controller) ClearRowFindings(typeName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearRowFindings(typeName)
}

// clearRowFindings is the lock-free body of ClearRowFindings so it can be
// called from contexts that already hold c.mu (write).
func (c *Controller) clearRowFindings(typeName string) {
	clearSlice := func(rows []resource.Resource) {
		for i := range rows {
			runtime.ApplyWave2ToRow(&rows[i], resource.ResourceTypeDef{}, nil, nil)
		}
	}

	canon := resource.CanonicalShortName(typeName)

	// Every list screen of this type in the stack (mirrors applyListFieldUpdates).
	for i := range c.stack {
		s := &c.stack[i]
		if s.ID != runtime.ScreenResourceList && s.ID != runtime.ScreenChildList {
			continue
		}
		st := s.Ctx.ResourceType
		if td := resource.FindResourceType(st); td != nil {
			st = td.ShortName
		}
		if st != canon || s.State.List == nil {
			continue
		}
		clearSlice(s.State.List.Rows)
		s.State.List.rowsVersion++
	}

	// RowStore-backed type cache (GetListAllResources and other
	// typeName-only callers). clearSlice replaces struct fields rather than
	// mutating a shared backing array or map — ApplyWave2ToRow allocates a
	// fresh Findings array and a fresh AttentionDetails map — so it is safe
	// to run directly against the copied slice Amend hands it.
	amend := func(rows []resource.Resource) []resource.Resource {
		if len(rows) == 0 {
			return rows
		}
		out := make([]resource.Resource, len(rows))
		copy(out, rows)
		clearSlice(out)
		return out
	}
	c.core.AmendRows(canon, amend)
	if typeName != canon {
		c.core.AmendRows(typeName, amend)
	}
}

// applyRowFindings is the applying-direction mirror of clearRowFindings: it
// writes Wave-2 findings (and their attention details) onto the controller's
// own row stores — every matching list screen's ls.Rows plus the
// RowStore-backed type cache. Without it, enrichment applied to an OPEN live
// list reaches only the session-owned stores (Core.applyEnrichment) and the
// controller's enrichmentStore glyph map, so the list-open save path (which
// persists from ls.Rows) writes rows with no findings — cached rows then
// reseed glyphless (violating C6's persisted-findings round-trip). Callers must hold c.mu (write); the
// PatchResourceList intent case is the production entry point. A nil
// findings map clears Wave-2 entries, matching runtime.ApplyWave2ToRow's
// contract. findings carries every independently-evaluated Wave-2 Finding
// per resource ID (mirrors runtime.ApplyWave2ToRow's own contract), so a
// multi-condition resource's second Finding is never silently dropped by
// this write.
//
// uninspected comes straight off the intent — the rows the enricher could
// not inspect, decided once by the runtime — and is handed to the shared
// fold (runtime.FoldWave2Rows) unexamined. This function does not re-decide
// which rows the result speaks for; deciding it here as well is how a
// timed-out probe could strip a finding from the screen while the file it
// was saved to kept it.
func (c *Controller) applyRowFindings(typeName string, findings map[string][]domain.Finding, details map[string]map[domain.FindingCode]domain.AttentionDetail, uninspected map[string]bool) {
	canon := typeName
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(typeName); t != nil {
		canon = t.ShortName
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: canon}
	}

	applySlice := func(rows []resource.Resource) {
		runtime.FoldWave2Rows(rows, td, findings, details, uninspected)
	}

	for i := range c.stack {
		s := &c.stack[i]
		if s.ID != runtime.ScreenResourceList && s.ID != runtime.ScreenChildList {
			continue
		}
		st := s.Ctx.ResourceType
		if t := resource.FindResourceType(st); t != nil {
			st = t.ShortName
		}
		if st != canon || s.State.List == nil {
			continue
		}
		applySlice(s.State.List.Rows)
		s.State.List.rowsVersion++
	}

	// RowStore-backed type cache. A shallow copy is enough: applySlice's only
	// statement is ApplyWave2ToRow, which replaces Findings and
	// AttentionDetails with freshly allocated ones rather than writing into
	// the arrays these rows still share with the store's pre-Amend copy.
	amend := func(rows []resource.Resource) []resource.Resource {
		if len(rows) == 0 {
			return rows
		}
		out := make([]resource.Resource, len(rows))
		copy(out, rows)
		applySlice(out)
		return out
	}
	c.core.AmendRows(canon, amend)
	if typeName != canon {
		c.core.AmendRows(typeName, amend)
	}
}

// listHasBadgeFinding reports whether a row's own findings bump the S1 issue
// count. Wave-1 findings (fetcher-written, no "wave2:" Source prefix) count at
// any issue severity — a Wave-1 warning IS the yellow row the menu badge
// counts by color. Wave-2 findings count only at "!" severity: per the S1
// contract "~ findings do not bump", and now that applyRowFindings writes
// Wave-2 findings onto controller rows, counting them at warn severity here
// would reintroduce the very drift the severity gate on the enrichment-store
// branch fixed.
func listHasBadgeFinding(r resource.Resource) bool {
	for _, f := range r.Findings {
		if f.Severity == domain.SevBroken {
			return true
		}
		if f.Severity.IsIssue() && !f.IsWave2Sourced() {
			return true
		}
	}
	return false
}

// listUninspectedIDs returns the rows of typeName that the type's Wave-2
// issue enricher could not inspect — IssueEnricherResult.TruncatedIDs, written
// to the session by Core.HandleEvent(messages.EnrichmentChecked). The session
// is the only store for this fact; the controller reads it rather than keeping
// a copy, so a refresh that clears the session set cannot leave a stale
// "not inspected" on a row the next sweep answered for.
//
// The session keys the set by ShortName (Core.handleEnrichmentChecked
// canonicalizes msg.ResourceType before the write), so a screen opened under
// an alias has to canonicalize its own query to find it.
func (c *Controller) listUninspectedIDs(typeName string) map[string]bool {
	if td := resource.FindResourceType(typeName); td != nil {
		typeName = td.ShortName
	}
	return c.core.EnrichmentTruncatedIDs(typeName)
}

// PushChildListScreen pushes a ScreenChildList for the given resource type
// directly onto the controller stack, bypassing menu/command routing. Used by
// NewChildResourceList to ensure topListState() is non-nil before Patch* calls.
// Calls lock-free applyIntents + ensureListState to avoid double-locking.
func (c *Controller) PushChildListScreen(typeName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenChildList,
		Context: runtime.ScreenContext{ResourceType: typeName},
	}})
	c.ensureListState()
}

// GetListEnrichmentFindings returns the enrichment findings map for typeName.
// Used by renderDataRow to resolve glyph markers without accessing the deleted
// findingsByID field on ResourceListModel.
func (c *Controller) GetListEnrichmentFindings(typeName string) map[string][]domain.Finding {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.listEnrichmentFindings(typeName)
}

// GetListAllResources returns all cached resources for the top list screen's type.
func (c *Controller) GetListAllResources() []resource.Resource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return nil
	}
	if len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	// Prefer per-screen rows (ls.Rows) over the shared type-keyed cache so that
	// a related-navigation list (EscPops, same ResourceType) cannot overwrite the
	// type key and corrupt the top-level list's resource count on cache-back.
	return c.listScreenResources(ls, top.Ctx.ResourceType)
}

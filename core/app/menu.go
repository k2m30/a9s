package app

import (
	"maps"
	"strconv"
	"strings"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// availabilitySavePayload is one snapshot of the menu availability/issue
// state, queued by persistMenuAvailabilityCache for the single writer
// goroutine (runAvailabilitySaveLoop) to persist. Fields mirror
// Core.SaveAvailabilityCache's parameters exactly — the writer passes them
// through unchanged.
type availabilitySavePayload struct {
	avail       map[string]int
	trunc       map[string]bool
	issueCounts map[string]int
	issueTrunc  map[string]bool
	issueKnown  map[string]bool
}

// CostsMenuShortName is the synthetic main-menu entry for the Cost Explorer
// screen. It is spliced into menuAllItems() for cursor/filter/selection
// purposes only — it is never added to resource.AllResourceTypes() itself,
// so it never reaches a probe/enrichment loop or the issue-badge sync.
// Exported so every renderer (TUI, web) that inspects a MenuSelected/MenuEntry
// ShortName to detect the synthetic row compares against this one symbol
// instead of a duplicated "costs" literal.
const CostsMenuShortName = "costs"

var costsMenuTypeDef = resource.ResourceTypeDef{
	ShortName: CostsMenuShortName,
	Name:      "Cost Explorer",
	Category:  "Cost",
}

// menuAllItems returns the full main-menu item list: every registered
// resource type plus the synthetic Cost Explorer entry appended last. This
// is the single source buildMenuBody/MenuSelected/the menu ActionSelect
// branch must use for visibility, cursor movement, and selection so the
// synthetic entry participates in filter/cursor logic identically to a real
// type.
func menuAllItems() []resource.ResourceTypeDef {
	return append(resource.AllResourceTypes(), costsMenuTypeDef)
}

// topMenuState returns the MenuState of the top-of-stack screen if it is
// ScreenMenu, or nil otherwise.
func (c *Controller) topMenuState() *MenuState {
	if len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenMenu {
		return nil
	}
	return c.stack[len(c.stack)-1].State.Menu
}

// rootMenuState returns the MenuState of the root (bottom) screen if it is
// ScreenMenu. Menu intents always target the root menu regardless of which
// screen is currently on top.
func (c *Controller) rootMenuState() *MenuState {
	if len(c.stack) == 0 {
		return nil
	}
	if c.stack[0].ID != runtime.ScreenMenu {
		return nil
	}
	return c.stack[0].State.Menu
}

// hasResourceScreen reports whether the stack contains at least one screen that
// can initiate a reveal (ScreenResourceList, ScreenChildList, or ScreenDetail).
// Used to guard spurious ValueRevealed events delivered when the only screen
// visible is the menu (e.g. late delivery after a profile/region switch).
// Caller must hold c.mu.
func (c *Controller) hasResourceScreen() bool {
	for _, s := range c.stack {
		if s.ID == runtime.ScreenResourceList ||
			s.ID == runtime.ScreenChildList ||
			s.ID == runtime.ScreenDetail {
			return true
		}
	}
	return false
}

// menuVisibleItems returns the resource types visible under the current
// MenuState filter + attention settings. mainmenu.go delegates to this via
// the controller snapshot — this is the single implementation.
func menuVisibleItems(ms *MenuState, all []resource.ResourceTypeDef) []resource.ResourceTypeDef {
	var result []resource.ResourceTypeDef
	if len(ms.Filter) < 2 {
		result = all
	} else {
		q := strings.ToLower(ms.Filter)
		result = make([]resource.ResourceTypeDef, 0, len(all))
		for _, item := range all {
			if strings.Contains(strings.ToLower(item.Name), q) ||
				strings.Contains(strings.ToLower(item.ShortName), q) {
				result = append(result, item)
			}
		}
	}

	if ms.AttentionOnly {
		filtered := make([]resource.ResourceTypeDef, 0, len(result))
		for _, item := range result {
			if menuIsVisibleUnderIssueFilter(ms, item, menuActiveKey(ms, item)) {
				filtered = append(filtered, item)
			}
		}
		result = filtered
	}
	return result
}

// menuIsVisibleUnderIssueFilter determines whether a resource type is shown
// when attention-only mode is active.
//
// item is the catalog entry; activeKey is the key under which intent data is
// stored for this item (from menuActiveKey — may be an alias like "rds" for
// the "dbi" type). mainmenu.go delegates to this via the controller snapshot.
func menuIsVisibleUnderIssueFilter(ms *MenuState, item resource.ResourceTypeDef, activeKey string) bool {
	known := ms.IssueKnown != nil && ms.IssueKnown[activeKey]
	// ExcludeFromIssueBadge types are never probed — hide them in attention mode,
	// even at cold-start, UNLESS issue data was explicitly recorded for them (a
	// real detected issue beats the exclusion). In production these types are
	// never probed, so this is equivalent to an absolute exclusion; the
	// conditional only matters for tests that inject issues directly.
	if item.ExcludeFromIssueBadge && !known {
		return false
	}
	// Unknown non-excluded type: visible only during true cold-start (no type
	// probed anywhere); once any probe lands, unknown types hide.
	if !known {
		return len(ms.IssueKnown) == 0
	}
	if ms.IssueCounts != nil && ms.IssueCounts[activeKey] > 0 {
		return true
	}
	return ms.IssueTruncated != nil && ms.IssueTruncated[activeKey]
}

// menuSkipUnavailable advances the cursor past confirmed-empty resource types.
// mainmenu.go delegates cursor movement to the controller; this is the single
// implementation. The scan itself lives in the shared stepToSelectable helper
// (actions_nav.go) so the related-panel cursor can run the identical
// skip-stepping semantics.
func menuSkipUnavailable(ms *MenuState, visible []resource.ResourceTypeDef, direction int) {
	if ms.Availability == nil || len(visible) == 0 {
		return
	}
	ms.Cursor = stepToSelectable(ms.Cursor, len(visible), direction, func(i int) bool {
		return menuIsConfirmedEmpty(ms, visible[i])
	})
}

// menuIsConfirmedEmpty reports whether item's resource type is confirmed to
// have zero resources (known count, zero, not truncated) — the skip predicate
// menuSkipUnavailable steps over.
func menuIsConfirmedEmpty(ms *MenuState, item resource.ResourceTypeDef) bool {
	key := menuActiveKey(ms, item)
	isTruncated := ms.Truncated != nil && ms.Truncated[key]
	count, known := ms.Availability[key]
	return known && count == 0 && !isTruncated
}

// menuActiveKey returns the key under which intent data for the given
// ResourceTypeDef is stored in a MenuState map. Intents are stored under
// whatever key the runtime emits (e.g. "rds" for the "dbi" type); this
// function resolves that key by checking the item's ShortName and all its
// Aliases in order, returning the first one present in any of the three
// intent maps. Falls back to item.ShortName when nothing is found.
//
// This allows buildMenuBody to expose the same key as the intent used —
// so MenuEntry.ShortName matches what the runtime and tests expect.
func menuActiveKey(ms *MenuState, item resource.ResourceTypeDef) string {
	candidates := make([]string, 0, 1+len(item.Aliases))
	candidates = append(candidates, item.ShortName)
	candidates = append(candidates, item.Aliases...)
	for _, c := range candidates {
		if ms.Availability != nil {
			if _, ok := ms.Availability[c]; ok {
				return c
			}
		}
		if ms.IssueKnown != nil {
			if ms.IssueKnown[c] {
				return c
			}
		}
	}
	return item.ShortName
}

// markMenuSweepAcked records that resource type shortName's background
// availability probe result has landed, clearing it from the Refreshing
// computation in menuRefreshing. Canonicalizes aliases to the registered
// ShortName so an alias-keyed ProbeResources entry (e.g. "rds" retained
// under the canonical "dbi" key) still matches.  Caller must hold c.mu
// (write) — called from Handle, which already holds the lock.
func (c *Controller) markMenuSweepAcked(shortName string) {
	if shortName == "" {
		return
	}
	canon := shortName
	if td := resource.FindResourceType(shortName); td != nil {
		canon = td.ShortName
	}
	if c.menuSweepAcked == nil {
		c.menuSweepAcked = make(map[string]bool)
	}
	c.menuSweepAcked[canon] = true
}

// syncMenuIssueCount applies the monotonic issue-badge guard to ms for canon:
// only raise ms.IssueCounts[canon] (never regress it), and clear a stale
// truncated flag once an equal-count exact (untruncated) observation lands —
// UNLESS the existing truncation was itself set authoritatively by a prior
// Wave-2 enrichment result and the current call is not itself authoritative
// (see authoritative below, axis 2). canon must already be the canonical
// resource short name — callers resolve aliases before calling in. Mirrors
// the issue-count half of the availability sync Contract D pins for
// syncExactTotalToMenu; extracted as the single chokepoint both the sweep
// lane (handle.go's syncExactTotalToMenu) and the in-list Wave-2 enrichment
// lane (list_filter.go's applyEnrichmentState) call, so a session with no
// background sweep (e.g. the web/headless lane) still gets the menu badge
// from in-list enrichment alone.
//
// authoritative distinguishes the two callers' semantics along TWO axes:
//
//  1. Raise-to-zero (the "authoritative && !known" arm): applyEnrichmentState
//     passes true because newIssues IS the Wave-2 result for canon (a genuine
//     zero-issue type must still flip IssueKnown so the menu stops showing it
//     as unswept); syncExactTotalToMenu passes false because its newIssues is
//     derived from bare list rows, which is not authoritative for a zero
//     count (Wave-2 enrichment may simply not have run yet for that list).
//     Without this distinction, a type first observed with newIssues==0 would
//     never set IssueKnown (no arm below fires on a zero-vs-zero comparison),
//     leaving a genuinely clean type stuck "unknown" forever — but relaxing
//     that for the non-authoritative caller would let an un-enriched list's
//     bare-zero falsely claim "no issues known".
//
//  2. Truncation-clear (the "equal-count exact" arm): a rows-derived
//     (non-authoritative) equal-count resync must never clear a truncation
//     flag that an authoritative Wave-2 enrichment result itself set.
//     syncExactTotalToMenu's newTrunc is ls.HasPagination — the LIST'S OWN
//     fetch pagination, which says nothing about whether the enrichment scan
//     that produced curIssues covered every row (e.g. a 55-row exact list
//     whose Wave-2 enrichment cap only scanned the first 50 rows) — the exact
//     row-fetch proves the ROW COUNT is complete, not that the issue COUNT
//     among those rows is. ms.IssueTruncAuthoritative[canon] records whether
//     the CURRENT curIssueTrunc=true was set by an authoritative caller; the
//     clear arm fires only when that is false (the flag was itself seeded
//     non-authoritatively — e.g. by PatchMenu or a prior rows-derived sync —
//     so a later equal-count rows-derived resync is still allowed to clear
//     it) or when the current call IS authoritative (a fresh Wave-2 result
//     always has standing to correct its own prior truncation claim).
//
// Caller must hold c.mu (write).
func (c *Controller) syncMenuIssueCount(ms *MenuState, canon string, newIssues int, newTrunc bool, authoritative bool) {
	curIssues := ms.IssueCounts[canon]
	curIssueTrunc := ms.IssueTruncated[canon]
	curIssueTruncAuthoritative := ms.IssueTruncAuthoritative[canon]
	switch {
	case newIssues > curIssues:
		if ms.IssueCounts == nil {
			ms.IssueCounts = make(map[string]int)
		}
		if ms.IssueKnown == nil {
			ms.IssueKnown = make(map[string]bool)
		}
		if ms.IssueTruncated == nil {
			ms.IssueTruncated = make(map[string]bool)
		}
		if ms.IssueTruncAuthoritative == nil {
			ms.IssueTruncAuthoritative = make(map[string]bool)
		}
		ms.IssueCounts[canon] = newIssues
		ms.IssueKnown[canon] = true
		ms.IssueTruncated[canon] = newTrunc
		ms.IssueTruncAuthoritative[canon] = authoritative && newTrunc
	case authoritative && !ms.IssueKnown[canon]:
		if ms.IssueCounts == nil {
			ms.IssueCounts = make(map[string]int)
		}
		if ms.IssueKnown == nil {
			ms.IssueKnown = make(map[string]bool)
		}
		if ms.IssueTruncated == nil {
			ms.IssueTruncated = make(map[string]bool)
		}
		if ms.IssueTruncAuthoritative == nil {
			ms.IssueTruncAuthoritative = make(map[string]bool)
		}
		ms.IssueCounts[canon] = newIssues
		ms.IssueKnown[canon] = true
		ms.IssueTruncated[canon] = newTrunc
		ms.IssueTruncAuthoritative[canon] = newTrunc
	case newIssues == curIssues && curIssueTrunc && !newTrunc && (authoritative || !curIssueTruncAuthoritative):
		if ms.IssueTruncated == nil {
			ms.IssueTruncated = make(map[string]bool)
		}
		ms.IssueTruncated[canon] = false
		if ms.IssueTruncAuthoritative == nil {
			ms.IssueTruncAuthoritative = make(map[string]bool)
		}
		ms.IssueTruncAuthoritative[canon] = false
	}
}

// persistMenuAvailabilityCache best-effort persists ms's availability/issue
// state to disk (Contract D: the badge must survive a restart). No-op when
// profile or region is unset (no cache file identity to write to). Shared by
// both callers of syncMenuIssueCount — syncExactTotalToMenu (handle.go) and
// applyEnrichmentState (list_filter.go) — which used to each carry their own
// verbatim copy of this five-map-clone-then-save block. Caller must hold c.mu
// (at least read, since it only reads ms and c.core's profile/region).
//
// The actual disk write never runs on this call stack. c.mu is the same lock
// every key event needs (Apply/Handle take it for their whole duration), and
// Core.SaveAvailabilityCache performs synchronous file I/O (temp file,
// chmod, rename) under session.cacheStoreMu — running it inline here would
// serialize user input behind disk latency on every Wave-2 result. Instead
// the five maps are cloned (cheap, still under c.mu — this is the only part
// that must not race the maps' own mutation) and handed to a single writer
// goroutine via a latest-wins, buffer-1 channel: a burst of calls (e.g. the
// startup availability sweep) collapses into whatever the newest snapshot is
// by the time the writer gets to it, rather than queuing one disk write per
// call. A write failure is silently dropped, mirroring the existing
// TaskKindSaveCache/probe-completion paths that already treat cache writes
// as best-effort.
//
// Close (below) is the deterministic shutdown hook: it stops the writer and
// blocks until the last queued snapshot has been persisted, so Contract D's
// "survives a restart" guarantee holds even though no write here ever blocks
// a live key event. Every real Controller owner (TUI, web session) must call
// Close on its own shutdown path — see Close's doc comment for the current
// wiring.
func (c *Controller) persistMenuAvailabilityCache(ms *MenuState) {
	profile, region := c.core.Profile(), c.core.Region()
	if profile == "" || region == "" {
		return
	}
	avail := make(map[string]int, len(ms.Availability))
	maps.Copy(avail, ms.Availability)
	trunc := make(map[string]bool, len(ms.Truncated))
	maps.Copy(trunc, ms.Truncated)
	issueCounts := make(map[string]int, len(ms.IssueCounts))
	maps.Copy(issueCounts, ms.IssueCounts)
	issueTrunc := make(map[string]bool, len(ms.IssueTruncated))
	maps.Copy(issueTrunc, ms.IssueTruncated)
	issueKnown := make(map[string]bool, len(ms.IssueKnown))
	maps.Copy(issueKnown, ms.IssueKnown)
	c.queueAvailabilitySave(availabilitySavePayload{
		avail:       avail,
		trunc:       trunc,
		issueCounts: issueCounts,
		issueTrunc:  issueTrunc,
		issueKnown:  issueKnown,
	})
}

// queueAvailabilitySave hands p to the single availability-cache writer
// goroutine, starting it on first use, and applies latest-wins coalescing: if
// the buffer-1 channel already holds an unconsumed snapshot, that snapshot is
// dropped in favor of p (the writer never blocks a caller by falling behind,
// and a burst of N calls only ever produces the last one's disk write).
// Non-blocking by construction — never runs disk I/O itself. A no-op after
// Close has been called (send on availSaveStop-closed path is guarded by the
// same select, so a very late call harmlessly drops its payload rather than
// panicking on a closed channel — the writer goroutine has already exited by
// then, and Close already flushed the last snapshot it had).
func (c *Controller) queueAvailabilitySave(p availabilitySavePayload) {
	select {
	case <-c.availSaveStop:
		// Close already ran (or is running): the writer is gone or exiting.
		// Dropping here is safe — Close's own drain step already persisted
		// whatever was queued at the time it was called, and no code path
		// depends on a save queued strictly after Close for correctness
		// (Contract D's restart guarantee only concerns the LAST save before
		// a real shutdown, which Close itself owns).
		return
	default:
	}
	c.availSaveOnce.Do(func() {
		c.availSaveWG.Add(1)
		go c.runAvailabilitySaveLoop()
	})
	select {
	case c.availSaveCh <- p:
	default:
		select {
		case <-c.availSaveCh:
		default:
		}
		select {
		case c.availSaveCh <- p:
		default:
		}
	}
}

// runAvailabilitySaveLoop is the single writer goroutine started by
// queueAvailabilitySave. It owns every call to Core.SaveAvailabilityCache for
// the menu-badge persistence path, so the synchronous disk write
// (SaveAvailabilityCache -> WithCacheStore -> store.SaveType's temp
// file+chmod+rename) never runs on a goroutine holding Controller.mu. Exits
// once Close closes availSaveStop, after persisting any snapshot still
// pending in availSaveCh (Close.Wait()s on availSaveWG for exactly this).
func (c *Controller) runAvailabilitySaveLoop() {
	defer c.availSaveWG.Done()
	for {
		select {
		case p := <-c.availSaveCh:
			_ = c.core.SaveAvailabilityCache(p.avail, p.trunc, p.issueCounts, p.issueTrunc, p.issueKnown)
		case <-c.availSaveStop:
			// Drain exactly one more pending snapshot (if any) so a Close
			// racing a just-queued save still persists it, then exit.
			select {
			case p := <-c.availSaveCh:
				_ = c.core.SaveAvailabilityCache(p.avail, p.trunc, p.issueCounts, p.issueTrunc, p.issueKnown)
			default:
			}
			return
		}
	}
}

// Close deterministically shuts down the availability-cache writer: it
// signals runAvailabilitySaveLoop to stop, and blocks until that goroutine
// has persisted any snapshot still queued and returned — so the menu badge's
// final state durably lands on disk (Contract D) even though no write along
// the way ever blocked a live key event. Idempotent and safe to call on a
// Controller that never queued a save (returns immediately without ever
// having started the goroutine). Safe to call more than once or
// concurrently — sync.Once guards the channel close.
//
// Every real Controller owner must call this on its own shutdown path:
//   - TUI: internal/tui.Model exposes it via CloseController, called from
//     cmd/a9s's runProgram.
//   - Web: each per-browser-session Controller (internal/web/construct.go's
//     newSession) is closed when the server's own context is cancelled
//     (internal/web/server.go's ListenAndServe) — sessions are otherwise
//     never individually evicted in this server, so process shutdown is the
//     only reachable hook today.
//
// Tests that queue an availability save (directly or via Handle/Apply) and
// then rely on t.TempDir() cleanup must call Close (e.g. t.Cleanup(c.Close))
// before returning, or the writer goroutine can still be persisting when the
// temp directory is removed.
func (c *Controller) Close() {
	c.availSaveCloseOnce.Do(func() {
		close(c.availSaveStop)
	})
	c.availSaveWG.Wait()
}

// menuRefreshing reports whether a background availability sweep is still in
// flight: true when RowStore holds at least one OriginProbe/OriginDisk type
// whose probe result has not yet been acked via markMenuSweepAcked. This is
// Contract C's MenuBody.Refreshing signal — a cache-seeded startup (RowStore
// populated from the on-disk availability cache before any live probe
// completes) shows Refreshing=true until every retained type's
// AvailabilityChecked result lands. Caller must hold c.mu (at least read).
//
// NOTE (task #17 wave 1 stage 2, row-store unification): this used to read
// session.ProbeResources directly; that field is gone as of stage 2. This
// one-line re-point to core.ProbeOriginTypeNames() is the minimal edit
// needed to keep internal/app compiling — internal/app itself is out of
// scope for stage 2 (its own ResourceCache lanes are Stage 3). Flagged for
// Stage 3 to fold into whatever internal/app's own RowStore migration does.
func (c *Controller) menuRefreshing() bool {
	for _, shortName := range c.core.ProbeOriginTypeNames() {
		canon := shortName
		if td := resource.FindResourceType(shortName); td != nil {
			canon = td.ShortName
		}
		if !c.menuSweepAcked[canon] {
			return true
		}
	}
	return false
}

// buildMenuBody constructs a MenuBody from MenuState + the resource catalog.
// Applies filter + attention + skip-unavailable + badge logic and produces
// renderer-agnostic data. mainmenu.go View() delegates to this via the
// controller snapshot.
func buildMenuBody(ms *MenuState) *MenuBody {
	visible := menuVisibleItems(ms, menuAllItems())

	cursor := ms.Cursor
	if cursor >= len(visible) && len(visible) > 0 {
		cursor = len(visible) - 1
	}

	entries := make([]MenuEntry, 0, len(visible))
	for _, item := range visible {
		// Resolve the key under which intent data was stored for this type.
		// Intents may use an alias ("rds") rather than the canonical ShortName
		// ("dbi"); menuActiveKey finds whichever key has data, falling back to
		// item.ShortName when no intent has been received yet.
		activeKey := menuActiveKey(ms, item)

		alias := ":" + item.ShortName
		if len(item.Aliases) > 0 {
			alias = ":" + item.Aliases[0]
		}

		avail, availKnown := 0, false
		if ms.Availability != nil {
			avail, availKnown = ms.Availability[activeKey]
		}
		availTruncated := ms.Truncated != nil && ms.Truncated[activeKey]
		origin := ""
		if ms.Origin != nil {
			origin = ms.Origin[activeKey]
		}

		badge := IssueBadge{}
		if ms.IssueKnown != nil && ms.IssueKnown[activeKey] {
			cnt := 0
			if ms.IssueCounts != nil {
				cnt = ms.IssueCounts[activeKey]
			}
			trunc := ms.IssueTruncated != nil && ms.IssueTruncated[activeKey]
			badge = IssueBadge{Count: cnt, Truncated: trunc}
		}

		entries = append(entries, MenuEntry{
			ShortName:      activeKey,
			Display:        item.Name,
			Alias:          alias,
			Category:       item.Category,
			IssueBadge:     badge,
			Availability:   avail,
			AvailKnown:     availKnown,
			AvailTruncated: availTruncated,
			Origin:         origin,
		})
	}

	return &MenuBody{
		Entries:       entries,
		Selected:      cursor,
		Filter:        ms.Filter,
		AttentionOnly: ms.AttentionOnly,
		Progress:      menuProgressIndicator(ms),
	}
}

// menuFrameTitle returns the frame-border title string for the main-menu screen.
// mainmenu.go FrameTitle() delegates to this via the controller.
func menuFrameTitle(ms *MenuState) string {
	all := menuAllItems()
	total := len(all)
	visible := menuVisibleItems(ms, all)
	filtered := len(visible)

	var title string
	switch {
	case ms.Filter != "" || ms.AttentionOnly:
		title = "resource-types(" + strconv.Itoa(filtered) + "/" + strconv.Itoa(total) + ")"
	default:
		title = "resource-types(" + strconv.Itoa(total) + ")"
	}
	if ms.AttentionOnly {
		title += " [!]"
	}
	if ms.EnrichTotal > 0 && ms.EnrichChecked < ms.EnrichTotal {
		title += " [enriching " + strconv.Itoa(ms.EnrichChecked) + "/" + strconv.Itoa(ms.EnrichTotal) + "]"
	}
	return title
}

// menuProgressIndicator returns the scan/enrichment progress suffix only —
// empty when no scan is active. This is what MenuBody.Progress carries;
// menuFrameTitle() carries the full frame title string (base + suffix).
func menuProgressIndicator(ms *MenuState) string {
	if ms.EnrichTotal > 0 && ms.EnrichChecked < ms.EnrichTotal {
		return "[enriching " + strconv.Itoa(ms.EnrichChecked) + "/" + strconv.Itoa(ms.EnrichTotal) + "]"
	}
	if ms.AvailTotal > 0 && ms.AvailChecked < ms.AvailTotal {
		return "[checking " + strconv.Itoa(ms.AvailChecked) + "/" + strconv.Itoa(ms.AvailTotal) + "]"
	}
	return ""
}

// menuPageSize is the default cursor jump for PageUp/PageDown when the renderer
// does not supply its viewport page size. The controller is renderer-neutral and
// has no terminal height; 10 matches a typical visible window.
const menuPageSize = 10

// menuPageSizeFor returns the page size for a PageUp/PageDown action: the
// renderer-supplied viewport page size (Action.N) when given, else the default.
// The TUI passes max(height-1, 1) so page movement tracks the live viewport.
func menuPageSizeFor(a Action) int {
	if a.N > 0 {
		return a.N
	}
	return menuPageSize
}

// MenuFrameTitle returns the frame-border title for the main-menu screen,
// delegating to menuFrameTitle with the root MenuState. Returns an empty
// string when the root screen is not a menu.
func (c *Controller) MenuFrameTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil {
		return ""
	}
	return menuFrameTitle(ms)
}

// MenuSelected returns the ResourceTypeDef at the current cursor and a bool
// that is true when navigation is permitted (i.e. the item is not confirmed
// empty). Mirrors the Enter-key guard in ActionSelect.
func (c *Controller) MenuSelected() (resource.ResourceTypeDef, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil {
		return resource.ResourceTypeDef{}, false
	}
	visible := menuVisibleItems(ms, menuAllItems())
	if len(visible) == 0 {
		return resource.ResourceTypeDef{}, false
	}
	// Background issue/availability intents can shrink the visible list while the
	// stored cursor still points past the end. Snapshot clamps the displayed
	// selection to the last visible row, so clamp here too — otherwise Enter on
	// the highlighted last row would hit a stale guard and become a no-op.
	cursor := ms.Cursor
	if cursor >= len(visible) {
		cursor = len(visible) - 1
	}
	selected := visible[cursor]
	if ms.Availability != nil {
		key := menuActiveKey(ms, selected)
		isTruncated := ms.Truncated != nil && ms.Truncated[key]
		if count, known := ms.Availability[key]; known && count == 0 && !isTruncated {
			return selected, false
		}
	}
	return selected, true
}

// GetMenuAvailability returns a copy of the root MenuState availability map.
// Returns nil when no availability data has been recorded.
func (c *Controller) GetMenuAvailability() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.Availability == nil {
		return nil
	}
	cp := make(map[string]int, len(ms.Availability))
	maps.Copy(cp, ms.Availability)
	return cp
}

// GetMenuTruncated returns a copy of the root MenuState truncated map.
// Returns nil when no truncation data has been recorded.
func (c *Controller) GetMenuTruncated() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.Truncated == nil {
		return nil
	}
	cp := make(map[string]bool, len(ms.Truncated))
	maps.Copy(cp, ms.Truncated)
	return cp
}

// GetMenuIssueCounts returns a copy of the root MenuState issue-count map.
// Returns nil when no issue data has been recorded.
func (c *Controller) GetMenuIssueCounts() map[string]int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.IssueCounts == nil {
		return nil
	}
	cp := make(map[string]int, len(ms.IssueCounts))
	maps.Copy(cp, ms.IssueCounts)
	return cp
}

// GetMenuIssueKnown returns a copy of the root MenuState issue-known map.
// Returns nil when no issue data has been recorded.
func (c *Controller) GetMenuIssueKnown() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.IssueKnown == nil {
		return nil
	}
	cp := make(map[string]bool, len(ms.IssueKnown))
	maps.Copy(cp, ms.IssueKnown)
	return cp
}

// GetMenuIssueTruncated returns a copy of the root MenuState issue-truncated map.
// Returns nil when no issue-truncation data has been recorded.
func (c *Controller) GetMenuIssueTruncated() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ms := c.rootMenuState()
	if ms == nil || ms.IssueTruncated == nil {
		return nil
	}
	cp := make(map[string]bool, len(ms.IssueTruncated))
	maps.Copy(cp, ms.IssueTruncated)
	return cp
}

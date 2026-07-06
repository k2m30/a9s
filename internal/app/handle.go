package app

import (
	"maps"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// Handle feeds an event through runtime.Core.HandleEvent, applies the returned
// UIIntents to the screen stack, enqueues returned TaskRequests, and returns
// the updated ViewState plus those TaskRequests.
//
// TASK-RESULT lane: completed background-task results arrive here. The caller
// is responsible for passing only values that implement runtime.Event
// (i.e. messages.Event) — unrecognised concrete types fall through to
// Core.HandleEvent's default nil, nil path.
//
// ResourcesLoaded events are also routed to applyResourcesLoaded so that
// DrainSync and the web renderer populate list rows without going through the
// TUI view stack. The target list screen is found by ResourceType in the
// controller stack, so a late async result for type X lands on X's screen even
// when it is not currently on top.
func (c *Controller) Handle(ev runtime.Event) (ViewState, []runtime.TaskRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()

	intents, tasks := c.core.HandleEvent(ev)
	c.applyIntents(intents)
	// C10: a navigation issued before AWS connect completes must replay once
	// ClientsReady lands. HandleEvent's ClientsReady path (and any other event
	// that can carry PendingRefresh) emits RefreshActiveListIntent for that;
	// applyIntents does not act on it, so route it through the shared helper.
	tasks = append(tasks, c.refreshTasksForIntents(intents)...)

	// Contract C: mark the type's availability sweep acked the moment its
	// AvailabilityChecked result arrives, regardless of whether HandleEvent's
	// central gen-guard treated it as stale. MenuBody.Refreshing tracks
	// wall-clock probe completion (has this type's background check landed
	// yet), not generation validity — a stale-but-arrived result still means
	// the sweep is no longer waiting on that type.
	if msg, ok := ev.(messages.AvailabilityChecked); ok {
		c.markMenuSweepAcked(msg.ResourceType)
	}

	// HandleEvent's central GenStamped guard drops stale events from the intent
	// path, but the row mutation below runs unconditionally. A host that passes
	// task results straight to Handle (headless/web) would otherwise let a late
	// fetch from a previous profile/region overwrite the current list rows, so
	// re-check staleness with the same predicate before mutating the controller.
	if msg, ok := ev.(messages.ResourcesLoaded); ok && !messages.IsStale(msg, c.core) {
		c.handleResourcesLoadedEvent(msg)
		// Web/headless by-ID drill: replace a flagged placeholder list with the
		// target's detail once its single row loads. TUI-safe — Handle is the
		// headless/web entry point; the TUI routes ResourcesLoaded through the
		// HandleResourcesLoadedEvent seam and drills to detail in its own adapter.
		tasks = append(tasks, c.autoOpenSingleDetail()...)
	}

	// messages.ClientsReady is explicitly excluded from HandleEvent
	// (orchestrator.go documents it as TUI-shim-only: the TUI's
	// handleClientsReady computes StackDepth/HasActiveRL from its own view
	// stack before calling Core.HandleClientsReady). The headless/web lane has
	// no TUI shim, so Handle must do the same renderer-shape computation here
	// — mirroring BootstrapLive — or a connect result fed through DrainSync
	// (drainsync.go) never reaches HandleClientsReady at all, and C10's
	// pre-connect-navigation replay never fires on this lane.
	if msg, ok := ev.(messages.ClientsReady); ok && msg.Err == nil {
		crIntents, crTasks := c.core.HandleClientsReady(runtime.ClientsReadyEvent{
			Clients:     msg.Clients,
			Region:      msg.Region,
			Gen:         msg.Gen,
			StackDepth:  len(c.stack),
			HasActiveRL: c.topListState() != nil,
		})
		c.applyIntents(crIntents)
		tasks = append(tasks, crTasks...)
		tasks = append(tasks, c.refreshTasksForIntents(crIntents)...)
	}

	// messages.ValueRevealed is explicitly excluded from HandleEvent
	// (orchestrator.go routes it nowhere) — the controller must handle it
	// directly here. HandleValueRevealed emits PushScreen{ScreenReveal} on
	// success or a FlashIntent on error.
	//
	// Guard: only process when the stack contains a resource-bearing screen
	// (list or detail) — a reveal can only be initiated from those screens.
	// A menu-only stack receiving ValueRevealed is a spurious event (e.g.
	// late delivery after a profile switch) and is silently dropped.
	if msg, ok := ev.(messages.ValueRevealed); ok && !messages.IsStale(msg, c.core) && c.hasResourceScreen() {
		revealed := runtime.ValueRevealedEvent{
			ResourceID: msg.ResourceID,
			Value:      msg.Value,
			Err:        msg.Err,
		}
		revealIntents, revealTasks := c.core.HandleValueRevealed(revealed)
		c.applyIntents(revealIntents)
		tasks = append(tasks, revealTasks...)
	}

	// messages.RelatedCheckBatch is the headless executor's counterpart to the
	// per-def RelatedCheckResult messages the TUI fan-out emits. Route each
	// per-def result through the same Core handler and ApplyDetailRelatedResult
	// path the TUI uses, so DrainSync populates the detail's RelatedRows.
	if batch, ok := ev.(messages.RelatedCheckBatch); ok && !messages.IsStale(batch, c.core) {
		c.handleRelatedCheckBatch(batch)
	}

	// messages.APIError: routed entirely through runtime.Core.HandleEvent
	// (the case added there mirrors this same ConnectGen stand-in + intent
	// application) — DEF-5's orchestrator gap fix. Handling it a second time
	// here would double-apply the ClearActiveListLoadingIntent/FlashIntent
	// the Core already returns via the intents/tasks captured above.

	// messages.IdentityError: the identity fetch failed. Core.HandleEvent routes
	// this through HandleIdentityError which clears IdentityFetching but does not
	// store the error string (it is view-layer state). Store it here so snapshot
	// can build IdentityBody.ErrorMsg. IsStale uses AspectConnect + Gen.
	if msg, ok := ev.(messages.IdentityError); ok && !messages.IsStale(msg, c.core) {
		c.identityLoading = false
		c.identityErrMsg = msg.Err
	}

	return c.snapshot(), tasks
}

// handleResourcesLoadedEvent routes a ResourcesLoaded event to the matching
// list screen in the controller stack. It finds the screen by resolving the
// event's ResourceType (including aliases) against each screen's context,
// so a late result for type X lands on X's screen regardless of which screen
// is currently on top. Staleness is the caller's responsibility — Handle drops
// stale ResourcesLoaded via messages.IsStale before invoking this.
func (c *Controller) handleResourcesLoadedEvent(msg messages.ResourcesLoaded) {
	if msg.ResourceType == "" {
		return
	}
	// Resolve canonical short name (handles aliases like "rds" → "dbi").
	canon := msg.ResourceType
	if td := resource.FindResourceType(msg.ResourceType); td != nil {
		canon = td.ShortName
	}
	// A fetch result belongs to a single list — the active (topmost) one of its
	// type. Apply it to the FIRST matching list from the top and stop; fanning it
	// out to every same-type list would overwrite a stacked filtered/child list's
	// rows onto the list beneath it (and vice-versa).
	for i := len(c.stack) - 1; i >= 0; i-- {
		s := &c.stack[i]
		if s.ID != runtime.ScreenResourceList && s.ID != runtime.ScreenChildList {
			continue
		}
		screenType := s.Ctx.ResourceType
		if td := resource.FindResourceType(screenType); td != nil {
			screenType = td.ShortName
		}
		if screenType != canon {
			continue
		}
		c.applyResourcesLoaded(s.State.List, canon, msg.Resources, msg.Pagination, msg.Append, isTopLevelCanonicalList(s.ID, s.State.List))
		// Contract D: sync the list's now-current row count to the root menu's
		// availability badge here, at the controller level, so both the TUI and
		// web renderer get it — this replaces the TUI-only sync-back that used
		// to run only on pop, in internal/tui/app_stack.go's popRS. Firing on
		// every ResourcesLoaded (not just a load-more that exhausts pagination)
		// preserves the pre-existing "non-exhausted visits syncing counts
		// upward" behavior popRS also provided: the only-increase guard inside
		// syncExactTotalToMenu makes this safe to call unconditionally — a
		// truncated or smaller result never regresses a larger known count.
		c.syncExactTotalToMenu(s, canon)
		// C6 — a filtered related drill's result is a session view; persisted
		// under (type + filter) so the next entry into the same drill seeds
		// instantly (SeedFilteredListFromCache) instead of a bare Loading.
		// Err results are skipped — a partial page must not replay as
		// complete. ls.Rows already holds the append-accumulated, materialized
		// set.
		if ls := s.State.List; ls != nil && msg.Err == nil && len(ls.FetchFilter) > 0 &&
			!isTopLevelCanonicalList(s.ID, ls) {
			c.core.FilteredRowsSet(canon, ls.FetchFilter, ls.Rows, ls.HasPagination, ls.PaginationCursor)
		}
		return
	}
}

// syncExactTotalToMenu applies the load-more-exhaustion exact-total sync-back
// (Contract D) to the root menu's availability + issue-badge state. screen is
// the list screen whose ResourcesLoaded just landed; canon is its canonical
// resource type. Skipped for related/filtered/child-context lists (EscPops or
// a non-nil ParentContext) — those show a filtered subset, not the global
// population.
//
// The availability guard applies three rules, in order of priority:
//  1. Unknown type: always seed it.
//  2. The currently-known count is itself a truncated lower bound (e.g.
//     "10+" from an availability probe) and the new result is exact
//     (untruncated): the exact result always wins, even if numerically
//     smaller than the lower-bound placeholder — an exact 3 is more useful
//     than an unconfirmed "10+".
//  3. Otherwise (both exact, or the new result is itself still truncated):
//     pure directional "never shrink a known count" — update only when
//     newCount is strictly larger than curCount.
//
// This differs from the TUI's original popRS guard
// (`!newTrunc || !known || newCount > curCount`), which would also let ANY
// untruncated result overwrite a larger already-EXACT count. Contract D pins
// the stricter rule 3 explicitly (a smaller exact result must not regress a
// larger already-exact one, e.g. a concurrent fuller probe already landed a
// bigger number) while still requiring rule 2 (an exact result must replace
// a truncated lower-bound placeholder regardless of magnitude).
func (c *Controller) syncExactTotalToMenu(screen *Screen, canon string) {
	ls := screen.State.List
	if ls == nil || ls.EscPops || ls.ParentContext != nil {
		return
	}
	// C6 scope boundary: only the canonical top-level, unfiltered list may
	// reach the persisted per-type cache file. A ScreenChildList never does,
	// even when (by coincidence) its resource type matches canon.
	if screen.ID == runtime.ScreenResourceList {
		c.maybeSaveResourceListCache(ls, canon)
	}
	newCount := len(ls.Rows)
	newTrunc := ls.HasPagination

	ms := c.rootMenuState()
	if ms == nil {
		return
	}
	if ms.Availability == nil {
		ms.Availability = make(map[string]int)
	}
	if ms.Truncated == nil {
		ms.Truncated = make(map[string]bool)
	}
	curCount, known := ms.Availability[canon]
	curTrunc := ms.Truncated[canon]
	if !known || (curTrunc && !newTrunc) || newCount > curCount {
		ms.Availability[canon] = newCount
		ms.Truncated[canon] = newTrunc
	}

	newIssues := c.listIssueCount(ls, canon)
	curIssues := ms.IssueCounts[canon]
	curIssueTrunc := ms.IssueTruncated[canon]
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
		ms.IssueCounts[canon] = newIssues
		ms.IssueKnown[canon] = true
		ms.IssueTruncated[canon] = newTrunc
	case newIssues == curIssues && curIssueTrunc && !newTrunc:
		if ms.IssueTruncated == nil {
			ms.IssueTruncated = make(map[string]bool)
		}
		ms.IssueTruncated[canon] = false
	}

	// Persist the updated availability to disk, mirroring the "survives an
	// app restart" half of Contract D. Best-effort — a write failure here
	// must not surface as a controller error; the existing
	// TaskKindSaveCache/probe-completion paths already treat cache writes as
	// best-effort.
	profile, region := c.core.Profile(), c.core.Region()
	if profile != "" && region != "" {
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
		_ = c.core.SaveAvailabilityCache(avail, trunc, issueCounts, issueTrunc, issueKnown)
	}
}

// maybeSaveResourceListCache persists ls.Rows for canon's canonical
// top-level, unfiltered list to its per-type disk cache file (C6: every
// loaded page, not just the first — ls.Rows already holds append-mode
// accumulation from applyResourcesLoaded). No-op when NoCache is set (C7b)
// or when canon has no resolvable ResourceTypeDef (issue-badge exclusion
// cannot be determined). Best-effort — a write failure is silently dropped,
// mirroring every other cache-write call site in this file.
//
// Callers MUST already have applied the C6 scope gate (top-level
// ScreenResourceList, not EscPops, not ParentContext) before calling this —
// isTopLevelCanonicalList computes it.
//
// task #17 wave 1 stage 4: the per-type materialize/build-rows/write body
// this method used to own directly now lives once in
// runtime.Core.SaveTypeRows, shared with the sweep lane
// (saveProbeResourcesToTypeFiles) — this method's job shrinks to computing
// the two save-lane-specific inputs (issues/issuesKnown via
// c.listIssueCount, exact via ls.HasPagination) and calling it. The
// redundant ObserveRows re-write this method used to perform after the disk
// save is gone too: applyResourcesLoaded (this method's only two callers'
// common ancestor) already routed ls.Rows through Core.ObserveRows before
// either caller reached here, so ls.Rows already IS what the store holds.
func (c *Controller) maybeSaveResourceListCache(ls *ListState, canon string) {
	if ls == nil || c.core.NoCache() {
		return
	}
	td := resource.FindResourceType(canon)
	if td == nil {
		// Issue-badge eligibility unknowable without a ResourceTypeDef.
		return
	}
	issuesKnown := !td.ExcludeFromIssueBadge
	issues := c.listIssueCount(ls, canon)
	exact := !ls.HasPagination
	_ = c.core.SaveTypeRows(canon, ls.Rows, len(ls.Rows), exact, issues, issuesKnown, ls.HasPagination, false)
}

// autoOpenSingleDetail replaces a web/headless by-ID placeholder list with the
// target resource's detail once its single row has loaded. Returns any
// related-check tasks the opened detail dispatches, or nil when no auto-open is
// pending or the target row has not arrived yet. Caller must hold c.mu (write).
//
// TUI-safe: only reached from Handle (the headless/web entry point). The TUI
// drills to by-ID detail in its own adapter and renders from a separate
// renderer stack, so it never opens detail through this controller path.
func (c *Controller) autoOpenSingleDetail() []runtime.TaskRequest {
	if len(c.stack) == 0 {
		return nil
	}
	top := &c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenResourceList && top.ID != runtime.ScreenChildList {
		return nil
	}
	ls := top.State.List
	if ls == nil || !ls.AutoOpenSingle || len(ls.RelatedIDSet) != 1 {
		return nil
	}
	var targetID string
	for id := range ls.RelatedIDSet {
		targetID = id
	}
	if targetID == "" {
		return nil
	}
	var matched *resource.Resource
	for i := range ls.Rows {
		if ls.Rows[i].ID == targetID {
			matched = &ls.Rows[i]
			break
		}
	}
	if matched == nil {
		return nil // target row not loaded yet; keep the placeholder list
	}
	res := *matched
	targetType := top.Ctx.ResourceType
	ls.AutoOpenSingle = false
	// Replace the placeholder list with the resource's detail.
	c.applyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	return c.openRelatedDetail(res, targetType)
}

// HandleResourcesLoadedEvent is the public adapter seam used by the TUI's
// runtime_adapter_resources.go. It routes a ResourcesLoaded message into the
// matching controller list screen's state, replacing the old updateActiveView
// path that routed the message through a stored ResourceListModel.Update(). The
// caller must perform the IsStale check before invoking this.
func (c *Controller) HandleResourcesLoadedEvent(msg messages.ResourcesLoaded) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handleResourcesLoadedEvent(msg)
}

// handleRelatedCheckBatch routes a RelatedCheckBatch (produced by the headless
// executor's runRelatedCheckers) into the stacked detail screen that matches
// the batch's (ResourceType, SourceResourceID). For each per-def result it
// calls HandleRelatedCheckResult on Core (to update the session RelatedCache
// and resource/lazy caches) and then ApplyDetailRelatedResult to merge the
// row into the matching detail's RelatedRows — mirroring the TUI's
// handleRelatedCheckResult path.
//
// The matching detail may not be the topmost screen (e.g. a YAML overlay is
// on top while the detail is stacked underneath). The search walks the stack
// from top to bottom and applies to the FIRST matching ScreenDetail.
//
// Callers must hold c.mu (write).
func (c *Controller) handleRelatedCheckBatch(batch messages.RelatedCheckBatch) {
	// Find the matching detail screen in the stack.
	var targetDetail *DetailState
	for i := len(c.stack) - 1; i >= 0; i-- {
		s := &c.stack[i]
		if s.ID != runtime.ScreenDetail {
			continue
		}
		ds := s.State.Detail
		if ds == nil {
			continue
		}
		if ds.ResourceType != batch.ResourceType || ds.Resource.ID != batch.SourceResourceID {
			continue
		}
		targetDetail = ds
		break
	}

	for _, result := range batch.Results {
		// Route through Core to update session caches (RelatedCache, ResourceCache,
		// LazyResourceCache) — mirrors handleRelatedCheckResult in the TUI adapter.
		intents, _ := c.core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
			ResourceType:     result.ResourceType,
			SourceResourceID: result.SourceResourceID,
			DefDisplayName:   result.DefDisplayName,
			Result:           result.Result,
		})
		c.applyIntents(intents)

		// Merge the row into the matching (possibly stacked) detail's RelatedRows
		// using the targetDetail pointer resolved above.
		if targetDetail == nil {
			continue
		}
		errMsg := ""
		if result.Result.Err != nil {
			errMsg = result.Result.Err.Error()
		}
		mergeDetailRelatedRow(targetDetail, result.DefDisplayName, result.Result.TargetType,
			result.Result.Count, false, errMsg, result.Result.Approximate, result.Result.ResourceIDs, result.Result.FetchFilter)
	}
}

// mergeDetailRelatedRow updates or appends one RelatedRow in ds, matching by
// DisplayName and preserving ResourceIDs. The single merge used by every
// related-result path (result lane, cache replay, batch, async adapter).
func mergeDetailRelatedRow(ds *DetailState, displayName, targetType string, count int, loading bool, errMsg string, approximate bool, resourceIDs []string, fetchFilter map[string]string) {
	targetIdx := -1
	for i := range ds.RelatedRows {
		if ds.RelatedRows[i].DisplayName == displayName {
			targetIdx = i
			break
		}
	}
	// Tight unambiguous fallback for results without DisplayName: match by
	// TargetType ONLY when exactly one row carries that TargetType. Refuse to
	// bind on ambiguity rather than guess. Production always populates
	// DefDisplayName; this branch is test-surface only.
	if targetIdx < 0 && displayName == "" {
		matches := 0
		firstIdx := -1
		for i := range ds.RelatedRows {
			if ds.RelatedRows[i].TargetType == targetType {
				if firstIdx < 0 {
					firstIdx = i
				}
				matches++
			}
		}
		if matches == 1 {
			targetIdx = firstIdx
		} else {
			return
		}
	}
	if targetIdx >= 0 {
		ds.RelatedRows[targetIdx].Count = count
		ds.RelatedRows[targetIdx].Loading = loading
		ds.RelatedRows[targetIdx].Err = errMsg
		ds.RelatedRows[targetIdx].Approximate = approximate
		ds.RelatedRows[targetIdx].ResourceIDs = resourceIDs
		ds.RelatedRows[targetIdx].FetchFilter = fetchFilter
		return
	}
	ds.RelatedRows = append(ds.RelatedRows, DetailRelatedRow{
		TargetType:  targetType,
		DisplayName: displayName,
		Count:       count,
		Loading:     loading,
		Err:         errMsg,
		Approximate: approximate,
		ResourceIDs: resourceIDs,
		FetchFilter: fetchFilter,
	})
}

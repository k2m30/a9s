// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"fmt"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/trace"
)

// traceFoldAcceptance emits a trace.KindFold event recording whether a
// GenStamped detail-operation result (RelatedCheckBatch, RelatedCheckResult,
// EnrichDetailResult) was accepted or rejected by the OperationID acceptance
// check immediately preceding each call site below — the invisible half of
// every "action B landed while action A was still in flight" defect this
// package's fold logic exists to get right. Guarded by trace.Enabled() so
// the rejection-reason string is never built when tracing is off.
func (c *Controller) traceFoldAcceptance(eventType string, operationID domain.Gen, accepted bool) {
	if !trace.Enabled() {
		return
	}
	reason := ""
	if !accepted {
		reason = fmt.Sprintf("operation %d superseded by active %d", operationID, c.core.ActiveDetailOp())
	}
	trace.Emit(trace.Event{
		Kind:        trace.KindFold,
		EventType:   eventType,
		OperationID: uint64(operationID),
		Accepted:    accepted,
		Reason:      reason,
	})
}

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

	intents, tasks := c.core.HandleEvent(ev)
	c.applyIntents(intents)
	// C10: a navigation issued before AWS connect completes must replay once
	// ClientsReady lands. HandleEvent's ClientsReady path (and any other event
	// that can carry PendingRefresh) emits RefreshActiveListIntent for that;
	// applyIntents does not act on it, so route it through the shared helper.
	tasks = append(tasks, c.refreshTasksForIntents(intents)...)

	// Menu-refreshing signal: mark the type's availability sweep acked when
	// its AvailabilityChecked result arrives. Only a result of the CURRENT
	// generation may do so — a probe dispatched before a pair switch or a
	// manual refresh answers for a sweep that no longer exists, and letting
	// it ack would make the menu claim the live sweep had already reached
	// that type.
	if msg, ok := ev.(messages.AvailabilityChecked); ok && !messages.IsStale(msg, c.core) {
		c.markMenuSweepAcked(msg.ResourceType)
	}

	// HandleEvent's central GenStamped guard drops stale events from the intent
	// path, but the row mutation below runs unconditionally. A host that passes
	// task results straight to Handle (headless/web) would otherwise let a late
	// fetch from a previous profile/region overwrite the current list rows, so
	// re-check staleness with the same predicate before mutating the controller.
	if msg, ok := ev.(messages.ResourcesLoaded); ok && !messages.IsStale(msg, c.core) {
		c.handleResourcesLoadedEvent(msg)
		// Reverse-scan reapply, mirroring the TUI (runtime_adapter_resources.go):
		// re-run the source predicate against each loaded page so a truncated
		// "(0+)"/"(N+)" list discovers matches on later pages. Handle already holds
		// c.mu (line 25), so call the lock-free core directly — the exported
		// ApplyReapplyCheckerAgainst would re-lock the non-reentrant RWMutex and
		// self-deadlock. No-op when no reapply-checker is registered for the type.
		if ls := c.topListState(); ls != nil && len(c.stack) > 0 {
			c.reapplyCheckerAgainst(ls, c.stack[len(c.stack)-1].Ctx.ResourceType, msg.Resources)
		}
		// Web/headless by-ID drill: replace a flagged placeholder list with the
		// target's detail once its single row loads. TUI-safe — Handle is the
		// headless/web entry point; the TUI routes ResourcesLoaded through the
		// HandleResourcesLoadedEvent seam and drills to detail in its own adapter.
		tasks = append(tasks, c.autoOpenSingleDetail()...)
	}

	// A by-ID placeholder list's own fetch can also resolve as
	// messages.ByIDFetchFailed (executor.go's KindFetchByIDDetail case)
	// rather than a ResourcesLoaded — this typed outcome is Core.HandleEvent's
	// default nil,nil path (orchestrator.go), so nothing else pops this
	// placeholder; it would otherwise strand the user on a permanently empty
	// list. Never a stranded empty list.
	if msg, ok := ev.(messages.ByIDFetchFailed); ok {
		c.popAutoOpenSinglePlaceholderOnNotFound(msg)
	}

	// messages.ClientsReady is explicitly excluded from HandleEvent
	// (orchestrator.go documents it as TUI-shim-only: the TUI's
	// handleClientsReady computes StackDepth/HasActiveRL from its own view
	// stack before calling Core.HandleClientsReady). The headless/web lane has
	// no TUI shim, so Handle must do the same renderer-shape computation here
	// — mirroring BootstrapLive — or a connect result fed through DrainSync
	// (drainsync.go) never reaches HandleClientsReady at all, and C10's
	// pre-connect-navigation replay never fires on this lane. Routed
	// unconditionally (success AND failure) — Core.HandleClientsReady's own
	// Gen guard drops stale results, and Err != nil routes internally to
	// handleClientsReadyFailure (rollback + error flash + FlashTick task), so
	// a failed connect on this lane surfaces the same way it does in the TUI
	// (internal/tui/app_session.go's handleClientsReady also passes Err
	// unconditionally) instead of being silently dropped.
	if msg, ok := ev.(messages.ClientsReady); ok {
		crIntents, crTasks := c.core.HandleClientsReady(runtime.ClientsReadyEvent{
			Clients:        msg.Clients,
			Err:            msg.Err,
			Region:         msg.Region,
			Gen:            msg.Gen,
			StackDepth:     len(c.stack),
			HasActiveRL:    c.topListState() != nil,
			HasActiveCosts: c.costsStateBeneathOverlay() != nil,
			NewGen:         c.core.ConnectGen(),
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
	// per-def result through the same fold every RelatedCheckResult uses
	// below, so DrainSync and the TUI populate the detail's RelatedRows
	// identically. IsStale checks OperationID against the session's active
	// DetailOperation (messages.AspectDetailOp) — a batch dispatched under an
	// operation the session has since moved past (a fresh open or an
	// explicit refresh bumped DetailOpGen) is dropped whole.
	if batch, ok := ev.(messages.RelatedCheckBatch); ok {
		accepted := !messages.IsStale(batch, c.core)
		c.traceFoldAcceptance("RelatedCheckBatch", batch.OperationID, accepted)
		if accepted {
			c.handleRelatedCheckBatch(batch)
		}
	}

	// messages.RelatedCheckResult is the TUI's per-def progressive-rendering
	// counterpart to RelatedCheckBatch — both lanes fold through the same
	// method, gated by the same OperationID acceptance check.
	if res, ok := ev.(messages.RelatedCheckResult); ok {
		accepted := !messages.IsStale(res, c.core)
		c.traceFoldAcceptance("RelatedCheckResult", res.OperationID, accepted)
		if accepted {
			c.foldRelatedCheckResultLocked(res)
		}
	}

	// messages.EnrichDetailResult delivers a completed on-demand
	// detail-enrichment result. Accepted only when its OperationID matches
	// the session's active DetailOperation — Rotate() bumping/clearing that
	// operation makes every in-flight enrichment result from an earlier
	// operation unacceptable, the same rule RelatedCheckResult/Batch use.
	if msg, ok := ev.(messages.EnrichDetailResult); ok {
		accepted := !messages.IsStale(msg, c.core)
		c.traceFoldAcceptance("EnrichDetailResult", msg.OperationID, accepted)
		if accepted {
			foldIntents, foldTasks := c.foldEnrichDetailResultLocked(msg)
			c.applyIntents(foldIntents)
			tasks = append(tasks, foldTasks...)
		}
	}

	// messages.APIError: routed entirely through runtime.Core.HandleEvent
	// (the case added there mirrors this same ConnectGen stand-in + intent
	// application) — the C4 fetch-failure handling lives in the Core, not
	// here. Handling it a second time
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

	// messages.CostsLoaded is not wired into runtime.Core.HandleEvent: the
	// merge target (CostsState) lives on the controller's screen stack, not
	// on session state Core owns. Controller.Handle is already the shared
	// headless/web/TUI entry point (see the package doc above), so handling
	// it directly here — mirroring the ValueRevealed guard immediately
	// above — reaches every lane without a runtime-side detour. IsStale
	// drops a fetch dispatched under a profile/region since superseded by a
	// later Rotate — the merge must never land in the new session's Store.
	if msg, ok := ev.(messages.CostsLoaded); ok && !messages.IsStale(msg, c.core) {
		if t := c.ApplyCostsLoaded(msg); t != nil {
			tasks = append(tasks, *t)
		}
	}

	tasks = c.stampDispatchSnapshotLocked(tasks)
	vs := c.snapshot()
	dirtyStore := c.costsDirtyStore
	c.costsDirtyStore = nil
	c.mu.Unlock()

	// The yaml.Marshal+os.WriteFile Store.Save performs must not run under
	// c.mu — it would block every other action against the controller for
	// the duration of a disk write. ApplyCostsLoaded marks the store dirty
	// instead of saving synchronously; this flushes it now that the lock is
	// released, re-locking only briefly to surface a write failure as a
	// flash (never a panic, never silently dropped data).
	if dirtyStore != nil {
		if err := dirtyStore.Save(); err != nil {
			c.mu.Lock()
			c.flash = Flash{Text: fmt.Sprintf("costs cache save: %v", err), IsError: true}
			c.mu.Unlock()
		}
	}
	c.flushCacheWrites()

	return vs, tasks
}

// findResourceListScreen scans the screen stack from the top for the list
// screen a result carrying resourceType + provenance belongs to — resolving
// resourceType (including aliases) against each screen's context, so a late
// result for type X lands on X's screen regardless of which screen is
// currently on top. A fetch result belongs to a single list — the active
// (topmost) one of its type — so the first match wins; fanning it out to
// every same-type list would overwrite a stacked filtered/child list's rows
// onto the list beneath it (and vice-versa).
//
// A screen-type match alone is not sufficient: the gate is symmetric — a
// screen only accepts a result whose provenance matches what that screen IS.
// isTopLevelCanonicalList reports whether the screen IS the type's canonical
// top-level list; provenance.CanonicalList() reports whether the result IS a
// canonical-list result. The two must agree:
//   - A canonical screen never accepts a by-ID/filtered/child result sharing
//     this ResourceType — that result's actual target is always a screen
//     pushed AFTER (and therefore found before, in this top-down scan) the
//     canonical list it happens to share a type with, since every
//     by-ID/filtered/child navigation pushes its own screen on top of
//     whatever it navigated from (applyRelatedNavResult).
//   - Symmetrically, a by-ID/filtered/child screen never accepts a
//     canonical-list result: a late canonical refresh sharing this
//     ResourceType is never that drill's data either — its target is the
//     canonical screen the drill was pushed on top of, found deeper in this
//     same scan.
//
// provenance == FetchProvenanceUnknown never satisfies either side:
// CanonicalList() is false for it, which would let it slip through the
// non-canonical branch of the agreement above by accident. An unstamped
// result fails closed (ok=false) rather than matching whichever
// non-canonical screen of its type happens to be topmost.
//
// On a mismatch the screen is skipped — never returned — and the scan
// continues deeper in the stack rather than returning immediately: the true
// target, if its screen is still open, is necessarily found further down,
// and if it already popped there is nothing to strand — a screen that no
// longer exists has no Rows left to apply to.
//
// Shared by handleResourcesLoadedEvent (the fetch-success path) and
// clearActiveListLoadingTarget (the paired fetch-FAILURE path, via
// ClearActiveListLoadingIntent) so a failed request is always routed to the
// exact same screen its paired success would have landed on — one scan, so
// the two paths cannot drift apart and reintroduce the asymmetry where a
// failure lands on whatever screen happens to be on top instead of the one
// that actually issued the failed request.
func (c *Controller) findResourceListScreen(resourceType string, provenance messages.FetchProvenance) (screen *Screen, canon string, ok bool) {
	if resourceType == "" || provenance == messages.FetchProvenanceUnknown {
		return nil, "", false
	}
	canon = resourceType
	if td := resource.FindResourceType(resourceType); td != nil {
		canon = td.ShortName
	}
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
		if isTopLevelCanonicalList(s.ID, s.State.List) != provenance.CanonicalList() {
			continue
		}
		return s, canon, true
	}
	return nil, canon, false
}

// handleResourcesLoadedEvent applies a ResourcesLoaded event to the list
// screen findResourceListScreen resolves it to — see that method for the
// full matching contract. Staleness is the caller's responsibility — Handle
// drops stale ResourcesLoaded via messages.IsStale before invoking this.
func (c *Controller) handleResourcesLoadedEvent(msg messages.ResourcesLoaded) {
	s, canon, ok := c.findResourceListScreen(msg.ResourceType, msg.Provenance)
	if !ok {
		return
	}
	// Superseded-dispatch discard, the same rule each lane's door applies —
	// this seam is called directly by the TUI as well as by Handle, so it
	// carries its own check rather than trusting its callers. Discarding the
	// result still retires the activity flag the discarded request raised:
	// no other completion is coming for it, and a Ctrl+R that supersedes an
	// outstanding load-more clears only Loading/Refreshing, so LoadingMore
	// would stay set for the rest of the session and the "m" key with it.
	if c.core.ListResultSuperseded(msg.ResourceType, msg.ListSeq) {
		if ls := s.State.List; ls != nil {
			ls.clearFetchInFlight(msg.LoadingMore)
		}
		return
	}
	topLevelCanonical := isTopLevelCanonicalList(s.ID, s.State.List)
	c.applyResourcesLoaded(s.State.List, canon, msg.Resources, msg.Pagination, msg.Append, msg.LoadingMore, topLevelCanonical, msg.Err)
	// Exact-total menu sync-back: sync the list's now-current row count to the root menu's
	// availability badge here, at the controller level, so both the TUI and
	// web renderer get it — this replaces the TUI-only sync-back that used
	// to run only on pop, in internal/tui/app_stack.go's popRS. Firing on
	// every ResourcesLoaded (not just a load-more that exhausts pagination)
	// preserves the pre-existing "non-exhausted visits syncing counts
	// upward" behavior popRS also provided: the only-increase guard inside
	// syncExactTotalToMenu makes this safe to call unconditionally — a
	// truncated or smaller result never regresses a larger known count.
	// A fetch that came back refused, with nothing to show, observed
	// nothing: syncing it would overwrite the type's cached count with a
	// zero and call the type verified. Partial success (rows alongside a
	// per-item error) IS an observation and still syncs.
	if msg.Err == nil || len(msg.Resources) > 0 {
		c.syncExactTotalToMenu(s, canon)
	}
	// C6 — a filtered related drill's result is a session view; persisted
	// under (type + filter) so the next entry into the same drill seeds
	// instantly (SeedFilteredListFromCache) instead of a bare Loading.
	// Err results are skipped — a partial page must not replay as
	// complete. ls.Rows already holds the append-accumulated, materialized
	// set.
	if ls := s.State.List; ls != nil && msg.Err == nil && len(ls.FetchFilter) > 0 &&
		!topLevelCanonical {
		c.core.FilteredRowsSet(canon, ls.FetchFilter, ls.Rows, ls.HasPagination, ls.PaginationCursor)
	}
}

// clearActiveListLoadingTarget resolves the *ListState a
// ClearActiveListLoadingIntent (emitted by HandleAPIError on a failed fetch)
// should apply to. When v carries a ResourceType and a non-Unknown
// Provenance — true of every production HandleAPIError dispatch as of the
// failure-routing fix (every messages.APIError construction site pairs it
// with the same Provenance its paired ResourcesLoaded success would carry) —
// it is routed through the exact same findResourceListScreen scan the
// success path uses, so the failure lands on the screen that actually owns
// the request that failed rather than whatever screen happens to be
// topmost (the defect this exists to close: navigating from list A to list
// B before A's fetch fails must never mark B with A's error while stranding
// A's own loading indicator). No match (including an already-popped screen)
// resolves to nil — fails closed, exactly like a ResourcesLoaded success
// with no matching screen.
//
// A zero ResourceType or Provenance means the producer predates this
// contract (a hand-built APIError with no paired fetch, or the
// ClientsReady-wrong-client-type path in internal/tui/runtime_adapter.go's
// emitAPIErrorCmd, which is not scoped to any particular list at all) —
// falls back to the pre-existing top-of-stack list screen behavior for
// those, mirroring FetchResourcesPayload.Provenance's own zero-value grace.
func (c *Controller) clearActiveListLoadingTarget(v runtime.ClearActiveListLoadingIntent) *ListState {
	if v.ResourceType != "" && v.Provenance != messages.FetchProvenanceUnknown {
		s, _, ok := c.findResourceListScreen(v.ResourceType, v.Provenance)
		if !ok {
			return nil
		}
		return s.State.List
	}
	return c.topListState()
}

// syncExactTotalToMenu applies the load-more-exhaustion exact-total sync-back
// to the root menu's availability + issue-badge state. screen is
// the list screen whose ResourcesLoaded just landed; canon is its canonical
// resource type. Skipped for related/filtered/child-context lists (EscPops or
// a non-nil ParentContext) — those show a filtered subset, not the global
// population.
//
// The availability write goes through applyAvailabilityObservation (menu.go),
// the one rule this lane shares with the probe lane.
func (c *Controller) syncExactTotalToMenu(screen *Screen, canon string) {
	ls := screen.State.List
	if ls == nil || ls.EscPops || ls.ParentContext != nil {
		return
	}
	newCount := len(ls.Rows)
	newTrunc := ls.HasPagination

	ms := c.rootMenuState()
	if ms == nil {
		return
	}
	// A list the operator opened and fetched live IS a live check of that
	// type — the same fact the sweep's probe would record — so this lane
	// observes it through the same intents the probe lane emits, rather than
	// writing the menu's maps itself: the type is verified, and its previous
	// probe's failure mark is retired.
	c.applyIntentsLocked([]runtime.UIIntent{
		runtime.PatchMenuAvailability{
			ResourceType: canon,
			Count:        newCount,
			Truncated:    newTrunc,
			Origin:       runtime.OriginVerified,
		},
		runtime.PatchMenuProbeCause{ResourceType: canon},
	})

	// authoritative=false: newIssues here is derived from bare list rows, not
	// a confirmed Wave-2 enrichment result, so a zero must not be treated as
	// a confirmed "no issues" (Wave-2 may not have run for this list yet).
	newIssues := c.listIssueCount(ls, canon)
	c.syncMenuIssueCount(ms, canon, newIssues, newTrunc, false)

	// C6 scope boundary: only the canonical top-level, unfiltered list may
	// reach the persisted per-type cache file. A ScreenChildList never does,
	// even when (by coincidence) its resource type matches canon. Runs AFTER
	// the badge sync above, because the file records what the badge now
	// holds — one observation, reconciled once.
	if screen.ID == runtime.ScreenResourceList {
		c.maybeSaveResourceListCache(ls, canon)
	}

	// Persist the updated availability to disk, mirroring the "survives an
	// app restart" half of the exact-total menu sync-back. Best-effort — a write failure here
	// must not surface as a controller error.
	c.persistMenuAvailabilityCache()
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
// The issue count it writes is the one the menu badge now holds — not a
// second derivation from the same rows. The badge already applied the one
// observation rule (an observation that cannot prove an issue is gone does
// not lower it), and a file that recorded the raw rows-derived number
// instead would disagree with the screen: a restart would then change the
// badge although nothing in the account had answered for it. A type whose
// badge is not known yet writes no issue count at all (issuesKnown=false),
// so the file keeps what it already had.
//
// task #17 wave 1 stage 4: the per-type materialize/build-rows/write body
// this method used to own directly now lives once in
// runtime.Core.SaveTypeRows, shared with the sweep lane
// (saveProbeResourcesToTypeFiles). The
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
	issues, issuesKnown := c.menuIssueBadge(canon)
	issuesKnown = issuesKnown && !td.ExcludeFromIssueBadge
	// Two facts, two fields: "another page exists" is what puts the "+" on
	// the count and offers the m key; "this list cannot claim to be the
	// type's whole population" is what forbids recording it as an exact
	// total. A failed fetch leaves the second true while the first is false.
	exact := !ls.PopulationUnconfirmed
	truncated := ls.HasPagination
	rows := append([]resource.Resource(nil), ls.Rows...)
	pair := c.core.Pair()
	// C4: every input is frozen here, under the lock; the write itself runs
	// after the caller releases it (queueCacheWrite), so a slow filesystem
	// can never stall the next key or a web snapshot behind this save. The
	// frozen pair is what lets the write still be rejected if the operator
	// switches profile before it lands.
	c.queueCacheWrite(func() {
		_ = c.core.SaveTypeRows(pair, canon, rows, len(rows), exact, issues, issuesKnown, truncated, false)
	})
}

// queueCacheWrite stages a cache write whose inputs are already frozen, to
// be run by flushCacheWrites once c.mu is released. Caller must hold c.mu
// (write). Mirrors the costs cache's dirty-store handoff in Handle — the
// controller mutex is the same lock every key event needs, so no disk I/O
// may happen while it is held (C4).
func (c *Controller) queueCacheWrite(write func()) {
	c.pendingCacheWrites = append(c.pendingCacheWrites, write)
}

// flushCacheWrites runs and clears whatever queueCacheWrite staged. Caller
// must NOT hold c.mu.
func (c *Controller) flushCacheWrites() {
	c.mu.Lock()
	pending := c.pendingCacheWrites
	c.pendingCacheWrites = nil
	c.mu.Unlock()
	for _, write := range pending {
		write()
	}
}

// popAutoOpenSinglePlaceholderOnNotFound pops a lane-neutral by-ID auto-open
// placeholder list when its single-target fetch resolves as
// messages.ByIDFetchFailed instead of ResourcesLoaded — the fetch genuinely
// ran and found nothing (e.g. a Cost Explorer resource-drill's ID lives in
// another account/region; CE's resource rows are account/region-wide while a
// session's own fetch is scoped to one region), never a stranded empty list.
// Pops only when msg's TargetType+ID exactly match the placeholder's own
// pending target (the same single-entry RelatedIDSet match
// GetListExactRelatedTargetID uses, inlined here since that exported helper
// takes c.mu itself and this runs while the caller already holds it) — an
// unrelated failure for some other fetch must never pop a placeholder that
// isn't waiting on it. Surfaces the caveat on whatever costs screen is
// revealed beneath, when one exists — the only lane-neutral auto-open origin
// this caveat currently applies to. Caller must hold c.mu (write).
func (c *Controller) popAutoOpenSinglePlaceholderOnNotFound(msg messages.ByIDFetchFailed) {
	if len(c.stack) == 0 {
		return
	}
	top := &c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenResourceList && top.ID != runtime.ScreenChildList {
		return
	}
	if top.Ctx.ResourceType != msg.TargetType {
		return
	}
	ls := top.State.List
	if ls == nil || len(ls.RelatedIDSet) != 1 {
		return
	}
	if _, ok := ls.RelatedIDSet[msg.ID]; !ok {
		return
	}
	c.applyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	if cs := c.topCostsState(); cs != nil {
		cs.ResourceRowNote = fmt.Sprintf(
			"%s — Cost Explorer resource rows are account/region-wide; this session queries one region/account",
			msg.Reason,
		)
	}
}

// autoOpenSingleDetail replaces a web/headless by-ID placeholder list with the
// target resource's detail once its single row has loaded. Returns any
// related-check tasks the opened detail dispatches, or nil when no auto-open is
// pending or the target row has not arrived yet. Caller must hold c.mu (write).
//
// TUI-safe: only reached from Handle (the headless/web entry point). The TUI
// drills to by-ID detail in its own adapter and renders from a separate
// renderer stack, so it never opens detail through this controller path.
//
// Two fallbacks so a by-ID drill never strands the placeholder list forever
// when the target row never lands on the current page — keyed on "was the
// target found", not "is the page empty": a page with unrelated rows but
// not the target is exactly as unresolved as an empty one, and must chase
// the same way:
//
//  1. Target not found, more pages available (HasPagination), no chase
//     already in flight (!LoadingMore): return the KindFetchMore task that
//     paginates further, same shape handleActionLoadMore builds.
//  2. Target not found, no more pages: synthesize a stub Resource via the
//     type's registered StubCreator and open its detail directly — the
//     listing genuinely never surfaced the target (e.g. a cross-account
//     reference), so there is nothing left to page through.
//
// Both fallbacks require ls.Loading to already be false — Handle calls this
// after ANY accepted ResourcesLoaded, not only the one matching this
// placeholder's own target type, so an unrelated type's fetch landing while
// this placeholder's own fetch is still in flight has empty Rows and no
// pagination too: indistinguishable from "fetched, genuinely empty, no more
// pages" by those fields alone. ls.Loading (cleared only by
// applyResourcesLoaded, itself only reached for the screen whose type
// matches the incoming message — handleResourcesLoadedEvent) is the
// existing, already-correct signal for "has this placeholder's own fetch
// ever actually returned" — reused here rather than adding a new field.
//
// If neither fallback applies (no pagination and no StubCreator), the
// placeholder list is left as-is, same as before these fallbacks existed.
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
	targetType := top.Ctx.ResourceType
	if matched == nil {
		if ls.Loading {
			return nil // this placeholder's own fetch has not landed yet; do not infer exhaustion from a page that was simply never fetched
		}
		// A page that HAS rows but not the target is exactly the "never lands
		// on the first page" case fallback 1 exists for — keying this on
		// len(ls.Rows) instead of "was the target found" stranded any target
		// past page 1 on a large listing, silently contradicting this
		// function's own "never stranded" contract.
		if ls.HasPagination && !ls.LoadingMore {
			ls.LoadingMore = true
			return []runtime.TaskRequest{{
				Key: runtime.TaskKey{Kind: runtime.KindFetchMore, Scope: targetType},
				Payload: runtime.FetchMorePayload{
					ContinuationToken: ls.PaginationCursor,
					ParentContext:     ls.ParentContext,
					FetchFilter:       ls.FetchFilter,
					Provenance:        listLane(top.ID, ls),
				},
			}}
		}
		if ls.LoadingMore {
			return nil // a chase is already in flight; wait for its result
		}
		td := resource.FindResourceType(targetType)
		if td == nil {
			td = resource.GetChildType(targetType)
		}
		if td == nil || td.StubCreator == nil {
			return nil // nothing left to chase or synthesize; keep the placeholder list
		}
		stub := td.StubCreator(targetID)
		ls.AutoOpenSingle = false
		c.applyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		return c.openRelatedDetail(stub, targetType)
	}
	res := *matched
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
	// The per-type save this result queues runs after the lock is released
	// (C4), exactly as Handle runs it — deferred first so it fires last.
	defer c.flushCacheWrites()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handleResourcesLoadedEvent(msg)
}

// handleRelatedCheckBatch folds every per-def result in a RelatedCheckBatch
// (produced by the headless executor's runRelatedCheckers) through
// foldRelatedCheckResultLocked — the single fold shared with the
// per-message RelatedCheckResult case in Handle. Callers must hold c.mu
// (write).
func (c *Controller) handleRelatedCheckBatch(batch messages.RelatedCheckBatch) {
	for _, result := range batch.Results {
		c.foldRelatedCheckResultLocked(result)
	}
}

// foldRelatedCheckResultLocked applies one related-checker result: it routes
// the result through Core.HandleRelatedCheckResult to update the session
// caches (RelatedCache, ResourceCache, LazyResourceCache) and merges the row
// into the RelatedRows of EVERY stacked detail screen matching
// (ResourceType, SourceResourceID) — not just the top, since a circular
// drill (e.g. bucket -> trail -> back to the same bucket) can push a second
// ScreenDetail for the same resource without popping the first, and a
// checker in flight can complete after the user has navigated to another
// detail. Shared by the TUI's per-def RelatedCheckResult messages and the
// headless executor's RelatedCheckBatch, so the two lanes cannot diverge.
// Callers must hold c.mu (write) and have already applied the OperationID
// acceptance check (messages.IsStale) — this method applies unconditionally.
func (c *Controller) foldRelatedCheckResultLocked(result messages.RelatedCheckResult) {
	intents, _ := c.core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:       result.ResourceType,
		SourceResourceID:   result.SourceResourceID,
		DefDisplayName:     result.DefDisplayName,
		Result:             result.Result,
		CachedPages:        result.CachedPages,
		LazyAddedResources: result.LazyAddedResources,
		LazyAddError:       result.LazyAddError,
	})
	c.applyIntents(intents)

	errMsg := relatedRowErrorText(result.Result)
	for i := range c.stack {
		if c.stack[i].ID != runtime.ScreenDetail {
			continue
		}
		ds := c.stack[i].State.Detail
		if ds == nil || ds.ResourceType != result.ResourceType || ds.Resource.ID != result.SourceResourceID {
			continue
		}
		mergeDetailRelatedRow(ds, result.DefDisplayName, result.Result.TargetType(),
			result.Result.EffectiveState(), result.Result.Count(), false, errMsg, result.Result.Truncated(), result.Result.ResourceIDs(), result.Result.FetchFilter())
	}
}

// mergeDetailRelatedRow updates or appends one RelatedRow in ds, matching by
// DisplayName and preserving ResourceIDs. The single merge used by every
// related-result path (result lane, cache replay, batch, async adapter).
func mergeDetailRelatedRow(ds *DetailState, displayName, targetType string, state domain.RelatedRowState, count int, loading bool, errMsg string, truncated bool, resourceIDs []string, fetchFilter map[string]string) {
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
		ds.RelatedRows[targetIdx].State = state
		ds.RelatedRows[targetIdx].Count = count
		ds.RelatedRows[targetIdx].Loading = loading
		ds.RelatedRows[targetIdx].Err = errMsg
		ds.RelatedRows[targetIdx].Truncated = truncated
		ds.RelatedRows[targetIdx].ResourceIDs = resourceIDs
		ds.RelatedRows[targetIdx].FetchFilter = fetchFilter
		return
	}
	ds.RelatedRows = append(ds.RelatedRows, DetailRelatedRow{
		TargetType:  targetType,
		DisplayName: displayName,
		State:       state,
		Count:       count,
		Loading:     loading,
		Err:         errMsg,
		Truncated:   truncated,
		ResourceIDs: resourceIDs,
		FetchFilter: fetchFilter,
	})
}

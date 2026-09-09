// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"fmt"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
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
	// Before the lock, because it walks every row of the page: the fetch
	// lane's crossing of the text boundary (SanitizedRows).
	if msg, ok := ev.(messages.ResourcesLoaded); ok {
		msg.Resources = SanitizedRows(msg.Resources)
		ev = msg
	}
	c.mu.Lock()

	intents, tasks := c.core.HandleEvent(ev)
	c.applyIntentsLocked(intents)
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
		// HandleEvent above already ran the one seam that canonicalises this
		// message's type and answers whether it was superseded; the message
		// carries both answers from here on.
		msg = runtime.StampListResult(msg, intents)
		c.handleResourcesLoadedEvent(msg)
		// Reverse-scan reapply, mirroring the TUI (runtime_adapter_resources.go):
		// re-run the source predicate against each loaded page so a truncated
		// "(0+)"/"(N+)" list discovers matches on later pages. Handle already holds
		// c.mu (line 25), so call the lock-free core directly — the exported
		// ApplyReapplyCheckerAgainst would re-lock the non-reentrant RWMutex and
		// self-deadlock. No-op when no reapply-checker is registered for the type.
		// Both steps below act on the list on top, so they run only when the
		// page's owner IS the list on top: a page routed to a screen beneath a
		// same-type drill must neither feed the drill's checker nor fire its
		// auto-open.
		if ls := c.topListState(); ls != nil && len(c.stack) > 0 && ls.instance == msg.ScreenID {
			c.reapplyCheckerAgainst(ls, c.stack[len(c.stack)-1].Ctx.ResourceType, msg.Resources)
			// Web/headless by-ID drill: replace a flagged placeholder list with the
			// target's detail once its single row loads. TUI-safe — Handle is the
			// headless/web entry point; the TUI routes ResourcesLoaded through the
			// HandleResourcesLoadedEvent seam and drills to detail in its own adapter.
			tasks = append(tasks, c.autoOpenSingleDetail()...)
		}
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
		c.applyIntentsLocked(crIntents)
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
		c.applyIntentsLocked(revealIntents)
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
			c.applyIntentsLocked(foldIntents)
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
		// An STS refusal names the profile or role it refused, and the
		// identity screen paints this string: the same boundary as the list's
		// error marker and the costs screen's.
		c.identityErrMsg = domain.Sanitize(msg.Err)
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
	// C4, absorb half: the row pass this event just made stale is the most
	// expensive thing the snapshot below would do, and it is pure. Freeze its
	// inputs here, drop the lock, build, and take the lock back only to swap
	// the built body in — so a 6000-row result holds the lock for the swap and
	// not for its rows.
	build, prewarm := c.captureTopListBodyBuild()
	dirtyStore := c.costsDirtyStore
	c.costsDirtyStore = nil
	c.mu.Unlock()

	var built listBodyMemo
	if prewarm {
		built = build.run()
	}

	c.mu.Lock()
	if prewarm {
		c.installListBodyMemo(build, built)
	}
	vs := c.snapshot()
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
	c.wakeCacheWriter()

	return vs, tasks
}

// findResourceListScreen returns the list screen a result belongs to: the one
// whose instance the request named, or none.
//
// A result carries the identity of the screen that dispatched it
// (runtime.TaskRequest.ScreenID, stamped at both dispatch boundaries and
// echoed back on messages.ResourcesLoaded / messages.APIError). That identity
// is the whole answer. A result naming a screen that has since been popped
// strands nothing, because the screen's rows went with it, and a result naming
// no screen was dispatched by no screen — nothing is waiting on it.
//
// The scan this replaced matched on resource type and lane instead. Two lists
// of one type share both — a filtered drill on a filtered drill, a child list
// on a child list — so it returned whichever was topmost and the deeper one's
// page landed on the newer one. There is nothing weaker to fall back to:
// matching by type is a guess, and a page nobody applies is better than a page
// applied to the wrong list.
//
// Shared by handleResourcesLoadedEvent (the fetch-success path) and
// clearActiveListLoadingTarget (the paired fetch-FAILURE path, via
// ClearActiveListLoadingIntent) so a failed request is always routed to the
// exact same screen its paired success would have landed on — one scan, so
// the two paths cannot drift apart.
func (c *Controller) findResourceListScreen(screenID domain.Gen) (screen *Screen, ok bool) {
	if screenID == 0 {
		return nil, false
	}
	for i := len(c.stack) - 1; i >= 0; i-- {
		s := &c.stack[i]
		if s.State.List != nil && s.State.List.instance == screenID {
			return s, true
		}
	}
	return nil, false
}

// handleResourcesLoadedEvent applies a ResourcesLoaded event to the list
// screen findResourceListScreen resolves it to — see that method for the
// full matching contract. Staleness is the caller's responsibility — Handle
// drops stale ResourcesLoaded via messages.IsStale before invoking this.
func (c *Controller) handleResourcesLoadedEvent(msg messages.ResourcesLoaded) {
	canon := msg.ResourceType
	s, ok := c.findResourceListScreen(msg.ScreenID)
	if !ok {
		// No screen owns this result — it arrived after its list was popped,
		// or none was ever opened. A canonical one still speaks for the type's
		// population, so the store takes it: this is the one RowStore write for
		// a delivery nothing renders, and the reason Core.HandleEvent no longer
		// makes its own.
		//
		// Only a canonical result is eligible, the gate that write carried: a
		// filtered drill, a by-ID lookup and a child fetch share this message
		// shape but are never the type's global population, and must neither
		// replace nor extend the shared per-type entry a canonical fetch owns.
		if msg.Provenance.CanonicalList() && canon != "" && !msg.Superseded {
			c.core.ObserveRows(canon, msg.Resources,
				msg.Pagination, session.OriginFetch, msg.Append)
		}
		return
	}
	// Superseded-dispatch discard, on the answer the runtime seam already gave
	// for this message (StampListResult). Discarding the result still retires
	// the activity flag the discarded request raised: no other completion is
	// coming for it, and a Ctrl+R that supersedes an outstanding load-more
	// clears only Loading/Refreshing, so LoadingMore would stay set for the
	// rest of the session and the "m" key with it.
	if msg.Superseded {
		if ls := s.State.List; ls != nil {
			ls.clearFetchInFlight(msg.LoadingMore, msg.ListSeq)
		}
		return
	}
	topLevelCanonical := isTopLevelCanonicalList(s.ID, s.State.List)
	c.applyResourcesLoaded(s.State.List, canon, msg.Resources, msg.Pagination, msg.Append, msg.LoadingMore, topLevelCanonical, msg.Err, msg.ListSeq)
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
		s, ok := c.findResourceListScreen(v.ScreenID)
		if !ok {
			return nil
		}
		return s.State.List
	}
	return c.topListState()
}

// syncExactTotalToMenu applies the load-more-exhaustion exact-total sync-back
// to the root menu's availability + issue-badge state. screen is the list
// screen whose ResourcesLoaded just landed; canon is the canonical short name
// the message arrived carrying. Skipped for related/filtered/child-context lists (EscPops or
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
// The per-type materialize/build-rows/write body lives once in
// runtime.Core.SaveTypeRows, shared with the sweep lane
// (saveProbeResourcesToTypeFiles). No ObserveRows write follows the disk
// save: applyResourcesLoaded (this method's only two callers'
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
	// The observation these rows belong to. A save frozen at an older
	// generation than one already written for this type is dropped rather
	// than allowed to undo it (Core.SaveTypeRows).
	obsGen := c.core.AnyOriginResourceCacheGen(canon)
	// C4: every input is frozen here, under the lock; the write itself runs
	// after the caller releases it, so a slow filesystem can never stall the
	// next key or a web snapshot behind this save. The frozen pair is what
	// lets the write still be rejected if the operator switches profile
	// before it lands.
	target := runtime.SaveTarget{Pair: pair, ObsGen: obsGen, Type: canon, ExactPopulation: exact}
	content := runtime.SaveContent{
		Resources:       rows,
		Count:           len(rows),
		Issues:          issues,
		IssuesKnown:     issuesKnown,
		IssuesTruncated: truncated,
	}
	c.stageCacheWrite(cacheWrite{run: func() {
		_ = c.core.SaveTypeRows(target, content)
	}})
}

// stageCacheWrite appends one save to the ordered queue. Callers hold c.mu
// (write) and have already frozen the save's inputs — the controller mutex is
// the same lock every key event needs, so no disk I/O may happen while it is
// held (C4), the same handoff the costs cache's dirty store performs in Handle. It never performs the
// write: a type file's yaml.Marshal of the whole row set is the same order of
// work as the row pass the absorb moved off the lock (C4), and running it on
// this goroutine only moves that cost from the lock into the caller's own
// latency — the fetch that triggered the save, and the key press behind it,
// would still wait for a 6000-row marshal.
//
// Staging does not wake the writer: the caller still holds c.mu, and a marshal
// started here competes for CPU with the very hold it is meant to stay out of.
// The wake happens at the seams that have just released the lock (Apply,
// Handle, HandleResourcesLoadedEvent, Close).
func (c *Controller) stageCacheWrite(w cacheWrite) {
	c.cacheWriteMu.Lock()
	c.cacheWriteWG.Add(1)
	c.pendingCacheWrites = append(c.pendingCacheWrites, w)
	c.cacheWriteMu.Unlock()
}

// wakeCacheWriter starts the writer goroutine if it is not running yet and
// tells it there is work. Never blocks. Caller must NOT hold c.mu.
func (c *Controller) wakeCacheWriter() {
	c.cacheWriteOnce.Do(func() {
		c.availSaveWG.Add(1)
		go c.runCacheWriteLoop()
	})
	select {
	case c.cacheWriteWake <- struct{}{}:
	default:
	}
}

// runCacheWriteLoop is the single goroutine that performs the staged saves, in
// the order they were queued — both lanes, so the file a restart reads is the
// one the later decision produced rather than the one that happened to finish
// last. Close's stop signal and wait group are shared with it, so one Close
// drains and waits: the restart guarantee (the last save before a real
// shutdown lands) covers everything on the queue.
func (c *Controller) runCacheWriteLoop() {
	defer c.availSaveWG.Done()
	for {
		select {
		case <-c.cacheWriteWake:
			c.drainCacheWrites()
		case <-c.availSaveStop:
			// Whatever was queued at the time Close was called still has to
			// land, including anything a nudge had not yet woken us for.
			c.drainCacheWrites()
			return
		}
	}
}

// drainCacheWrites performs every staged write, and keeps going while more
// arrive: a save queued while this one runs must not wait for another nudge.
func (c *Controller) drainCacheWrites() {
	for {
		c.cacheWriteMu.Lock()
		pending := c.pendingCacheWrites
		c.pendingCacheWrites = nil
		c.cacheWriteMu.Unlock()
		if len(pending) == 0 {
			return
		}
		for _, w := range pending {
			w.run()
			c.cacheWriteWG.Done()
		}
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
	c.applyIntentsLocked([]runtime.UIIntent{runtime.PopScreen{}})
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
		c.applyIntentsLocked([]runtime.UIIntent{runtime.PopScreen{}})
		return c.openRelatedDetail(stub, targetType)
	}
	res := *matched
	ls.AutoOpenSingle = false
	// Replace the placeholder list with the resource's detail.
	c.applyIntentsLocked([]runtime.UIIntent{runtime.PopScreen{}})
	return c.openRelatedDetail(res, targetType)
}

// HandleResourcesLoadedEvent is the public adapter seam used by the TUI's
// runtime_adapter_resources.go. It routes a ResourcesLoaded message into the
// matching controller list screen's state.
//
// The caller must perform the IsStale check, and must hand in a message the
// runtime seam has already stamped (runtime.StampListResult): the canonical
// short name and the supersession answer are read off the message here, never
// recomputed. A superseded message is still routed — it owes the request that
// raised it the retirement of its activity flag.
func (c *Controller) HandleResourcesLoadedEvent(msg messages.ResourcesLoaded) {
	// Before the lock, exactly as Handle does it: this is the door the TUI
	// uses, so a page that arrives through it crosses the text boundary here
	// or nowhere.
	msg.Resources = SanitizedRows(msg.Resources)
	// The per-type save this result queues runs after the lock is released
	// (C4), exactly as Handle runs it — deferred first so it fires last.
	defer c.wakeCacheWriter()
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
	c.applyIntentsLocked(intents)

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

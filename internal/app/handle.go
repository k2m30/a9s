package app

import (
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

	// messages.APIError: a failed fetch clears the list Loading flag and surfaces
	// an error flash. The TUI bumps flash.gen before calling HandleAPIError; the
	// headless controller has no flash.gen, so ConnectGen serves as stable stand-in
	// (same pattern as ActionSelectProfile/Region). The FlashTick task returned by
	// HandleAPIError is suppressed here — it is only meaningful in a running event
	// loop (TUI/web timer); the headless path has no loop to process it.
	// IsStale is not applicable here — APIError has AcceptZeroGen=true.
	if msg, ok := ev.(messages.APIError); ok {
		apiIntents, _ := c.core.HandleAPIError(runtime.APIErrorEvent{
			Err:    msg.Err,
			NewGen: c.core.ConnectGen(),
		})
		c.applyIntents(apiIntents)
	}

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
		c.applyResourcesLoaded(s.State.List, canon, msg.Resources, msg.Pagination, msg.Append)
		return
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
	for i := range ds.RelatedRows {
		if ds.RelatedRows[i].DisplayName == displayName {
			ds.RelatedRows[i].Count = count
			ds.RelatedRows[i].Loading = loading
			ds.RelatedRows[i].Err = errMsg
			ds.RelatedRows[i].Approximate = approximate
			ds.RelatedRows[i].ResourceIDs = resourceIDs
			ds.RelatedRows[i].FetchFilter = fetchFilter
			return
		}
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


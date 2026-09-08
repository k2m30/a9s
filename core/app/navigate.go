// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"encoding/json"
	"reflect"
	"strings"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"gopkg.in/yaml.v3"
)

// resourceYAMLLines marshals r to plain YAML text (no ANSI coloring) and
// returns the individual lines. Mirrors the source that YAMLModel.RawContent
// uses — RawStruct when present, resource.Fields as fallback — so the
// content is equivalent to what the TUI YAML screen shows (minus syntax color).
//
// Returns a one-element slice with a "No YAML data available" notice when
// the resource carries neither RawStruct nor Fields.
func resourceYAMLLines(r resource.Resource) []string {
	var data []byte
	var err error
	if r.RawStruct != nil {
		safe := fieldpath.ToSafeValue(reflect.ValueOf(r.RawStruct))
		data, err = yaml.Marshal(safe)
	} else if len(r.Fields) > 0 {
		data, err = yaml.Marshal(r.Fields)
	}
	if err != nil || len(data) == 0 {
		return []string{"  No YAML data available"}
	}
	raw := strings.TrimRight(string(data), "\n")
	return strings.Split(raw, "\n")
}

// resourceJSONLines marshals r to indented plain JSON text (no ANSI coloring)
// and returns the individual lines. Mirrors JSONModel.RawContent — RawStruct
// when present, resource.Fields as fallback.
//
// For the JSON case we also try a roundtrip through jsonyaml.TryJSONToYAMLLines
// to validate the JSON is well-formed; the actual output is the MarshalIndent
// string split by newline, which is always valid when MarshalIndent succeeds.
//
// Returns a one-element slice with a "No JSON data available" notice when the
// resource carries neither RawStruct nor Fields.
func resourceJSONLines(r resource.Resource) []string {
	var data []byte
	var err error
	if r.RawStruct != nil {
		data, err = json.MarshalIndent(r.RawStruct, "", "  ")
	} else if len(r.Fields) > 0 {
		data, err = json.MarshalIndent(r.Fields, "", "  ")
	}
	if err != nil || len(data) == 0 {
		return []string{"  No JSON data available"}
	}
	raw := strings.TrimRight(string(data), "\n")
	return strings.Split(raw, "\n")
}

// applyNavResult converts a NavigateResult into PushScreen/ReplaceScreen/PopScreen
// stack operations. Called by Apply after HandleNavigate returns. Returns any
// additional TaskRequests the stack operation itself spawns — callers must
// append these to the tasks HandleNavigate already returned. A list open of
// either kind spawns none: HandleNavigate returns the verification task for
// the cached branch as well as the miss branch, so no adapter decides for
// itself whether a re-entered list is re-verified.
//
// The adapter (not the runtime) decides which ScreenID to push for each kind;
// this method encodes that mapping for the headless controller.
//
// All NavigateResult kinds are handled, including those that require
// selected-row or resource data (PushDetail, PushYAML, PushJSON,
// PushResourceList/Cached, FetchReveal).
func (c *Controller) applyNavResult(res runtime.NavigateResult) []runtime.TaskRequest {
	switch res.Kind {
	case runtime.NavigateKindPopAll:
		// Pop back to the root menu — leave exactly one screen, never empty.
		// (Popping via ApplyIntents would now stop at the len<=1 guard anyway;
		// pop directly to keep the intent clear.)
		if len(c.stack) > 1 {
			c.stack = c.stack[:1]
		}

	case runtime.NavigateKindPushHelp:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenHelp}})

	case runtime.NavigateKindPushRegion:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenRegion}})

	case runtime.NavigateKindPushTheme:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenTheme}})

	case runtime.NavigateKindPushCosts:
		c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
		c.ensureCostsState(Now())
		// HandleNavigate no longer fetches unconditionally (SC-002) —
		// ensureCostsShapeFetched is the sole decider: a warm cache opens
		// with zero CE calls, a cold one gets exactly the one task it needs.
		if cs := c.topCostsState(); cs != nil {
			return costsTaskSlice(c.ensureCostsShapeFetched(cs))
		}

	case runtime.NavigateKindFetchProfiles:
		// No stack change — the adapter starts the fetch task; when the result
		// arrives (ProfilesLoaded), HandleProfilesLoaded pushes ScreenProfileSelector.

	case runtime.NavigateKindFlash, runtime.NavigateKindNoop:
		// No stack change — flash is surfaced via FlashIntent in the intent stream.

	case runtime.NavigateKindPushResourceList, runtime.NavigateKindPushResourceListCached:
		intent := runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: res.ResolvedType},
		}
		if res.ReplaceCurrent {
			c.applyIntents([]runtime.UIIntent{runtime.ReplaceScreen{ID: intent.ID, Context: intent.Context}})
		} else {
			c.applyIntents([]runtime.UIIntent{intent})
		}
		c.ensureListState()
		top := &c.stack[len(c.stack)-1]
		// Cache-first seeding: HandleNavigate attaches CachedEntry on both
		// kinds — the RowStore-retained full entry of a previous visit
		// (NavigateKindPushResourceListCached) and the availability probe's or
		// on-disk cache's first page (NavigateKindPushResourceList). Populate
		// rows immediately so headless/web callers see data without waiting for
		// the fetch round-trip, and mark Refreshing: the verification task
		// HandleNavigate returned for both kinds is still on its way, and
		// cache-first seeding never skips it.
		if res.CachedEntry != nil {
			c.applyResourcesLoaded(top.State.List, res.ResolvedType, res.CachedEntry.Resources, res.CachedEntry.Pagination, false, false, isTopLevelCanonicalList(intent.ID, top.State.List), nil)
			top.State.List.Refreshing = true
			// Set after the seeding call, per SetListTotalCount's ordering note;
			// lock-free here because applyNavResult already runs under c.mu.
			if res.CachedEntry.TotalCount > len(top.State.List.Rows) {
				top.State.List.TotalCount = res.CachedEntry.TotalCount
			}
		}

	case runtime.NavigateKindPushDetail:
		if res.Resource == nil {
			return nil
		}
		intent := runtime.PushScreen{
			ID: runtime.ScreenDetail,
			Context: runtime.ScreenContext{
				ResourceType: res.ResolvedType,
				ResourceID:   res.Resource.ID,
			},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		// Use lock-free variants — applyNavResult is always called while c.mu
		// is already held by Apply or Handle.
		c.ensureDetailState(*res.Resource, res.ResolvedType)
		c.initDetailRelatedRows(res.ResolvedType)
		// beginDetailWorkloadLocked owns the complete workload (enrich +
		// related, cache-replay-suppressed) — no per-half gate at this call
		// site anymore: DispatchEnrich/DispatchRelated (HandleNavigate) were
		// redundant with the builder's own registration checks (HasDetailEnricher
		// / GetRelated) for every NavigateTargetDetail navigation reaching here.
		_, tasks := c.beginDetailWorkloadLocked(res.ResolvedType, *res.Resource, false, false)
		return tasks

	case runtime.NavigateKindPushYAML:
		if res.Resource == nil {
			return nil
		}
		lines := resourceYAMLLines(*res.Resource)
		intent := runtime.PushScreen{
			ID: runtime.ScreenYAML,
			Context: runtime.ScreenContext{
				ResourceType: res.ResolvedType,
				ResourceID:   res.Resource.ID,
			},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureTextState(lines)
		// Dispatch the operation's COMPLETE workload, not just enrich: the
		// related-check task's result still writes RelatedCache (via
		// foldRelatedCheckResultLocked's stack-wide merge) for the detail
		// screen beneath this YAML overlay, or for the next plain-detail open
		// of this same resource — a YAML/JSON view showing no related panel
		// itself is not a reason to strand that write-through.
		_, tasks := c.beginDetailWorkloadLocked(res.ResolvedType, *res.Resource, false, false)
		return tasks

	case runtime.NavigateKindPushJSON:
		if res.Resource == nil {
			return nil
		}
		lines := resourceJSONLines(*res.Resource)
		intent := runtime.PushScreen{
			ID: runtime.ScreenJSON,
			Context: runtime.ScreenContext{
				ResourceType: res.ResolvedType,
				ResourceID:   res.Resource.ID,
			},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureTextState(lines)
		// See NavigateKindPushYAML above — same complete-workload dispatch.
		_, tasks := c.beginDetailWorkloadLocked(res.ResolvedType, *res.Resource, false, false)
		return tasks

	case runtime.NavigateKindFetchReveal:
		// No stack push yet — the push happens when Handle receives
		// messages.ValueRevealed and routes it to HandleValueRevealed.
		// Tasks are returned by Apply's ActionReveal branch directly.
	}
	return nil
}

// ApplyEmitNavigate is the headless/web counterpart of the TUI adapter's
// emitNavigateCmd (internal/tui/runtime_adapter.go): it resolves a
// TaskKindEmitNavigate task's EmitNavigatePayload into the equivalent
// Controller navigation, instead of the payload only reaching the TUI's
// tea.Msg pipeline. TaskKindEmitNavigate is adapter-only from
// Core.ExecuteTask's perspective (ErrAdapterOnlyTask) — DrainSync* calls
// this method for that one kind instead of executing it, so the one-shot
// -c/ActionCommand navigation armed by HandleClientsReady and dispatched by
// handleAvailabilityCacheLoaded (deferred -c navigation, D11) actually lands on the headless
// stack the same way it lands on the TUI's view stack.
func (c *Controller) ApplyEmitNavigate(p runtime.EmitNavigatePayload) []runtime.TaskRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
		Target:       p.Target,
		ResourceType: p.ResourceType,
	})
	tasks = append(tasks, c.applyNavResult(res)...)
	return c.stampDispatchSnapshotLocked(tasks)
}

// ReplayRelatedCache populates the related panel for the top detail screen
// matching (resourceType, res.ID) from any cached RelatedCacheResult entries,
// merging them directly into RelatedRows. Returns true when a cache hit was
// replayed (the caller must NOT also dispatch a fan-out check — D6: no
// re-fan-out over cached data) and false when the type has no registered
// related defs, no matching detail screen is on top of the stack, or the
// cache misses (the caller is responsible for dispatching the fan-out
// check — a KindRelatedCheck TaskRequest from Core.DetailOperationTasks —
// itself; both the TUI and headless/web reach the same task this way).
//
// Exported so the TUI adapter's NavigateKindPushDetail handling
// (runtime_adapter_navigate.go) can share this replay instead of carrying
// its own copy — the headless/web callers reach the same logic via
// applyNavResult's NavigateKindPushDetail case (lock-free, under Apply/Handle's
// already-held c.mu).
func (c *Controller) ReplayRelatedCache(resourceType string, res resource.Resource) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.replayRelatedCache(resourceType, res)
}

// replayRelatedCache is the lock-free implementation of ReplayRelatedCache:
// merges cached results into the top detail screen AND reports whether that
// cache is complete, bailing out (false) unconditionally when no detail
// screen is on top.
//
// beginDetailWorkloadLocked does NOT use this: its suppression decision must
// not depend on a detail screen being present on the stack — a YAML/JSON-only
// open (core/app/navigate.go's PushYAML/PushJSON cases) has no detail state
// at all, but a fully-covered related cache must still suppress its related
// task the same as a plain detail open does (R2 — bailing here on `ds == nil`
// meant every YAML/JSON open re-ran the entire related fan-out against AWS,
// no matter how complete the cache already was). beginDetailWorkloadLocked
// calls relatedCacheCoverage (the pure, screen-independent completeness
// predicate) and mergeRelatedCacheIntoDetail (the panel merge, itself a
// no-op with no detail screen present) separately instead.
//
// Callers must hold c.mu (write).
func (c *Controller) replayRelatedCache(resourceType string, res resource.Resource) bool {
	if c.topDetailState() == nil {
		return false
	}
	cached, complete := c.relatedCacheCoverage(resourceType, res)
	c.mergeRelatedCacheIntoDetail(resourceType, res, cached)
	return complete
}

// relatedCacheCoverage fetches the cached related-check results for
// (resourceType, res.ID) and reports whether they cover EVERY def
// resource.GetRelated(resourceType) registers — an explicit completeness
// answer, not inferred from "did we merge anything" and not gated on any
// detail screen being present (see replayRelatedCache's doc comment for why
// that gate is wrong for this decision). A PARTIAL cache (some defs' checks
// completed in a prior operation, others never finished before this one
// began) reports incomplete, so the caller keeps dispatching the
// related-check task; otherwise the still in-flight defs' rows would be
// stranded in Loading forever, since BeginDetailOperation has already
// invalidated whatever was still running for them under the prior operation
// ID.
//
// Callers must hold c.mu (read or write).
func (c *Controller) relatedCacheCoverage(resourceType string, res resource.Resource) (cached []runtime.RelatedCacheResult, complete bool) {
	defs := resource.GetRelated(resourceType)
	if len(defs) == 0 {
		return nil, false
	}
	ck := runtime.RelatedCacheKey(resourceType, res.ID)
	cached, hit := c.core.RelatedCacheGet(ck)
	if !hit || len(cached) == 0 {
		return cached, false
	}
	haveDef := make(map[string]bool, len(cached))
	for _, entry := range cached {
		haveDef[entry.DefDisplayName] = true
	}
	for _, def := range defs {
		if !haveDef[def.DisplayName] {
			return cached, false
		}
	}
	return cached, true
}

// mergeRelatedCacheIntoDetail merges cached related-check results into the
// top detail screen's RelatedRows when one matching (resourceType, res.ID)
// is on top of the stack. No-op when there is none — a YAML/JSON-only open
// has no panel to populate, even though its related task's result still
// needs to write through the session cache for the plain-detail open (or
// next YAML/JSON open) that follows.
//
// Callers must hold c.mu (write).
func (c *Controller) mergeRelatedCacheIntoDetail(resourceType string, res resource.Resource, cached []runtime.RelatedCacheResult) {
	ds := c.topDetailState()
	if ds == nil || len(cached) == 0 {
		return
	}
	for _, entry := range cached {
		errMsg := relatedRowErrorText(entry.Result)
		mergeDetailRelatedRow(ds, entry.DefDisplayName, entry.Result.TargetType(),
			entry.Result.EffectiveState(), entry.Result.Count(), false, errMsg, entry.Result.Truncated(), entry.Result.ResourceIDs(), entry.Result.FetchFilter())
	}
}

// relatedRowErrorText is the related panel's per-row failure text. One
// producer for the two lanes that build it — a result as it lands
// (handle.go's foldRelatedCheckResultLocked) and the cache replay of an
// earlier one above — so the panel cannot say two things about one fact.
// The words are the ones core/aws owns, never the raw chain, whose wrapper
// preamble repeats the call the row already names.
func relatedRowErrorText(res resource.RelatedCheckResult) string {
	if err := res.Err(); err != nil {
		return awsclient.CauseOf(err)
	}
	return ""
}

// SeedFilteredListFromCache seeds the top list screen from the session
// filtered-rows cache for (targetType, filter). On a hit the rows render
// immediately and Refreshing is armed so the ⟳ marker shows while the
// caller's filtered fetch verifies the seeded content (C3 cache-first,
// C6 instant re-entry). On a miss the screen keeps its Loading state —
// C4 permits the bare Loading only for a never-cached drill.
func (c *Controller) SeedFilteredListFromCache(targetType string, filter map[string]string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seedFilteredListFromCache(targetType, filter)
}

// seedFilteredListFromCache is the lock-free implementation of
// SeedFilteredListFromCache. Callers must hold c.mu (write).
func (c *Controller) seedFilteredListFromCache(targetType string, filter map[string]string) bool {
	if len(filter) == 0 {
		return false
	}
	canon := resource.CanonicalShortName(targetType)
	entry, ok := c.core.FilteredRowsGet(canon, filter)
	if !ok || len(entry.Rows) == 0 {
		return false
	}
	ls := c.topListState()
	if ls == nil {
		return false
	}
	ls.Rows = append([]resource.Resource(nil), entry.Rows...)
	ls.Loading = false
	ls.HasPagination = entry.Truncated
	ls.PopulationUnconfirmed = entry.Truncated
	ls.PaginationCursor = entry.Cursor
	ls.Refreshing = true
	ls.rowsVersion++
	return true
}

// dispatchRelatedNavigate calls HandleRelatedNavigate then applyRelatedNavResult
// and merges the two task slices, preferring extraTasks when the same Key
// appears in both (applyRelatedNavResult returns payload-bearing replacements
// for tasks HandleRelatedNavigate emits without payloads, e.g. KindFetchFiltered).
// All three related-nav callers — ActionSelect keyboard, ActionRelatedSelect
// click, and ActionFieldSelect click — use this shared tail.
func (c *Controller) dispatchRelatedNavigate(ev runtime.RelatedNavigateEvent) []runtime.TaskRequest {
	navRes, tasks := c.core.HandleRelatedNavigate(ev)
	extraTasks := c.applyRelatedNavResult(navRes)
	// Truncated reverse-scan ("(0+)"/"(N+)"): register the reapply-checker so
	// every loaded page re-runs the source predicate and extends the scoped
	// RelatedIDSet — the same reapply the TUI list receives. applyRelatedNavResult
	// already seeded the RelatedIDSet (empty for "(0+)", N for "(N+)"); the fetch
	// task came from HandleRelatedNavigate (KindFetchResources). One path for both
	// counts — the empty-seed "(0+)" is not special-cased.
	if ev.Truncated && ev.Checker != nil {
		// dispatchRelatedNavigate runs under Apply's c.mu; use the lock-free core
		// (the exported PatchListReapplyChecker would re-lock and self-deadlock).
		c.patchListReapplyChecker(ev.Checker, ev.SourceResource)
	}
	if len(extraTasks) == 0 {
		return tasks
	}
	extraKeys := make(map[runtime.TaskKey]struct{}, len(extraTasks))
	for _, t := range extraTasks {
		extraKeys[t.Key] = struct{}{}
	}
	merged := make([]runtime.TaskRequest, 0, len(tasks)+len(extraTasks))
	for _, t := range tasks {
		if _, replaced := extraKeys[t.Key]; !replaced {
			merged = append(merged, t)
		}
	}
	merged = append(merged, extraTasks...)
	return merged
}

// BeginDetailWorkload is the locked, exported form of
// beginDetailWorkloadLocked for callers outside an already-held c.mu — the
// TUI adapter, which has no Controller lock of its own (Bubble Tea's single
// Update() goroutine is its serialization discipline instead).
func (c *Controller) BeginDetailWorkload(rt string, res resource.Resource, refresh, forceRelated bool) (runtime.DetailOperation, []runtime.TaskRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	op, tasks := c.beginDetailWorkloadLocked(rt, res, refresh, forceRelated)
	return op, c.stampDispatchSnapshotLocked(tasks)
}

// beginDetailWorkloadLocked begins a new DetailOperation for (rt, res) and
// returns its COMPLETE workload — the single builder every site that begins
// a detail operation must route through, replacing the former two-call
// Core.BeginDetailOperation + Core.DetailOperationTasks pattern that let a
// caller mint a fresh op ID (invalidating any earlier operation's in-flight
// enrich/related results) while dispatching only one of the two replacement
// tasks, silently stranding the other half forever. Callers append the
// ENTIRE returned slice — there is no "half" left to selectively discard;
// Core.BeginDetailOperation itself now returns that slice directly (no
// *TaskRequest out-params to fold together), and the per-entry-point tests
// in tests/unit/detail_workload_test.go pin that no caller drops an element.
//
// Cache-replay suppression: omits the related task ONLY when
// relatedCacheCoverage reports the cache complete for every registered def
// (D6 — no re-fan-out over cached data) — a screen-independent decision
// (R2): a YAML/JSON-only open has no detail state to merge into
// (mergeRelatedCacheIntoDetail is a no-op there), but its suppression
// decision must be identical to a plain detail open's, or every such open
// re-runs the entire related fan-out against AWS regardless of how complete
// the cache already is. Callers never make this choice themselves; it used
// to be duplicated ad hoc at each call site (applyNavResult's PushDetail
// case, openRelatedDetail's own hand-rolled copy), which is exactly the kind
// of divergence risk a single builder closes.
//
// forceRelated deletes the resource's RelatedCache entry before checking
// coverage above, so a caller whose entire purpose is recomputing a row the
// user is already looking at (resolve-in-place Enter/click, and owner
// decision #38's reveal-on-Back recompute) can never have that recompute
// silently swallowed by its own stale-but-complete cached result — the same
// "delete first" idiom the detail Ctrl+R path already used ad hoc, now
// shared instead of duplicated. This is NOT about preventing duplicate cache
// entries (PatchRelatedCache, core/app/intents.go, replaces per DefDisplayName
// unconditionally, with or without this delete) — it is the only way to force
// relatedCacheCoverage to read "incomplete" for a resource whose cache is
// already fully (but stalely) populated, since completeness alone drives
// suppression. It leaves refresh (enrich cache SkipCache) untouched: these
// callers want a fresh RELATED check, not a forced live re-fetch of cached
// enrichment (SFN/CFN/IAM policy documents etc.) as an unrelated side effect.
//
// Sticky refresh: an explicit refresh (refresh=true) is a demand on the
// RESOURCE, not on the one operation that happened to carry it — a
// non-refresh operation for the same resource beginning before the refresh's
// enrichment has folded (a panel toggle, a related-row retry) must not
// silently downgrade back to cached enrichment. effectiveRefresh inherits any
// still-pending refresh recorded for this resource key
// (session.PendingDetailRefresh, cleared only by a successful
// foldEnrichDetailResultLocked whose operation is >= the recorded one — see
// that method). A refresh request re-arms the entry with this operation's own
// ID even when it was already set, since op IDs only increase.
//
// Callers must hold c.mu (write).
func (c *Controller) beginDetailWorkloadLocked(rt string, res resource.Resource, refresh, forceRelated bool) (runtime.DetailOperation, []runtime.TaskRequest) {
	key := runtime.RelatedCacheKey(rt, res.ID)
	_, pendingRefresh := c.core.PendingDetailRefreshGet(key)
	effectiveRefresh := refresh || pendingRefresh

	op, built := c.core.BeginDetailOperation(rt, res, effectiveRefresh)

	hasEnrich, hasRelated := false, false
	for _, t := range built {
		switch t.Key.Kind {
		case runtime.KindEnrichDetail:
			hasEnrich = true
		case runtime.KindRelatedCheck:
			hasRelated = true
		}
	}
	// Only an enrichment completion retires the record (Core.HandleEnrichDetailResult),
	// and SkipCache is the only thing it feeds — arming it for a type with no
	// enricher would strand an entry nothing can ever clear.
	if refresh && hasEnrich {
		c.core.PendingDetailRefreshSet(key, op.ID)
	}
	if !hasRelated {
		return op, built
	}

	if forceRelated {
		c.core.RelatedCacheDelete(key)
	}
	cached, complete := c.relatedCacheCoverage(rt, res)
	c.mergeRelatedCacheIntoDetail(rt, res, cached)
	if complete {
		tasks := make([]runtime.TaskRequest, 0, len(built)-1)
		for _, t := range built {
			if t.Key.Kind != runtime.KindRelatedCheck {
				tasks = append(tasks, t)
			}
		}
		return op, tasks
	}
	return op, built
}

// openRelatedDetail pushes a detail screen for the already-fetched resource
// cached and begins its detail workload (enrich + related, cache-replay
// suppressed via beginDetailWorkloadLocked — same builder as ActionOpenDetail;
// this method used to carry its own hand-rolled duplicate of that
// cache-replay logic). Shared by the cache-hit related-navigate path
// (NavigationKindDetail) and the web by-ID auto-open path. Caller must hold
// c.mu (write).
func (c *Controller) openRelatedDetail(cached resource.Resource, targetType string) []runtime.TaskRequest {
	c.applyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: targetType, ResourceID: cached.ID},
	}})
	c.ensureDetailState(cached, targetType)
	if c.topDetailState() == nil {
		return nil
	}
	c.initDetailRelatedRows(targetType)
	_, tasks := c.beginDetailWorkloadLocked(targetType, cached, false, false)
	return tasks
}

// applyRelatedNavResult converts a NavigationResult into stack operations and
// returns any additional task requests the result spawns.
//
// NavigationResult carries NavigationKind plus TargetType, TargetID,
// RelatedIDs, FetchFilter, FilterText — no ScreenID; the controller maps
// kind → ScreenID.
func (c *Controller) applyRelatedNavResult(res runtime.NavigationResult) []runtime.TaskRequest {
	switch res.Kind {
	case runtime.NavigationKindResourceList:
		intent := runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: res.TargetType},
		}
		c.applyIntents([]runtime.UIIntent{intent})
		c.ensureListState()

	case runtime.NavigationKindFilteredList:
		// pushByIDPlaceholderList (list_state.go) pushes the screen and always
		// sets EscPops so isTopLevelCanonicalList excludes this screen from
		// the shared RowStore write, the disk-cache persist gate, and the C6
		// FilteredRowsSet seed — mirroring the TUI's own rl.SetEscPops(true)
		// (internal/tui/runtime_adapter_related.go) — across every sub-case
		// below (FetchFilter, exact-ID, truncated, TargetID). When TargetID
		// resolves to a registered FetchByIDs helper it also flags the
		// by-ID auto-open-single-detail drill (RelatedIDSet + AutoOpenSingle),
		// which is mutually exclusive with the TargetID == "" branch below —
		// ResolveRelatedNavigate never sets both TargetID and FetchFilter on
		// the same NavigationResult, so the gate order does not matter.
		if ls := c.pushByIDPlaceholderList(res.TargetType, res.TargetID); ls != nil {
			if res.FilterText != "" {
				ls.Filter = res.FilterText
			}
			if len(res.FetchFilter) > 0 {
				ls.FetchFilter = res.FetchFilter
				c.seedFilteredListFromCache(res.TargetType, res.FetchFilter)
				// HandleRelatedNavigate returns a no-payload KindFetchFiltered task
				// (tested as-is by the QA suite). Replace it here with a payload-
				// bearing version so the executor can invoke the filtered fetcher.
				// The task is still returned even on a cache hit: it is the
				// background verify-refresh that clears Refreshing when it lands.
				return []runtime.TaskRequest{{
					Key:     runtime.TaskKey{Kind: runtime.KindFetchFiltered, Scope: res.TargetType},
					Cache:   runtime.CacheNone,
					Payload: runtime.FetchFilteredPayload{Filter: res.FetchFilter},
				}}
			}
			if res.TargetID == "" {
				if !res.Truncated && len(res.RelatedIDs) > 0 {
					// Exact-ID list: seed rows from the any-lane cache (Partial lane
					// included) so a hit renders without a fetch, and clear Loading.
					// Shared with the TUI via seedRelatedExactRows so the two
					// renderers cannot diverge. Already under c.mu.
					c.seedRelatedExactRows(ls, res.TargetType, res.RelatedIDs)
				} else {
					// Truncated "(0+)"/"(N+)": a non-nil (even EMPTY) set filters to
					// the found IDs, so "(0+)" renders a scoped list with zero rows —
					// never the full target list. The population fetch + reapply-
					// checker extend the set as later pages load.
					set := make(map[string]struct{}, len(res.RelatedIDs))
					for _, id := range res.RelatedIDs {
						if id != "" {
							set[id] = struct{}{}
						}
					}
					ls.RelatedIDSet = set
				}
			}
		}

	case runtime.NavigationKindDetail:
		// The target resource is already cached: NavigationKindDetail is only
		// returned on a cache hit (ResolveRelatedNavigate), and HandleRelatedNavigate
		// returns no fetch task for it ("the adapter serves these from cached
		// state"). Seed the detail synchronously from the cache — this works for
		// every type (most have no by-id fetcher, so a fetch would land on an empty
		// detail). Mirrors ActionOpenDetail's related-panel handling.
		id := res.TargetID
		if id == "" && len(res.RelatedIDs) == 1 {
			id = res.RelatedIDs[0]
		}
		cached, ok := c.core.RelatedCachedResource(res.TargetType, id)
		if !ok {
			// Defensive: cache unexpectedly missing. Land on a filtered list by id
			// instead of an empty detail so navigation still resolves somewhere.
			c.applyIntents([]runtime.UIIntent{runtime.PushScreen{
				ID:      runtime.ScreenResourceList,
				Context: runtime.ScreenContext{ResourceType: res.TargetType},
			}})
			c.ensureListState()
			if ls := c.topListState(); ls != nil && id != "" {
				ls.Filter = id
			}
			return nil
		}
		return c.openRelatedDetail(cached, res.TargetType)

	case runtime.NavigationKindEnterChildView:
		// Delegate to the same path used by ActionChildView but with the
		// target type already resolved. Build a minimal EnterChildViewEvent.
		ev := runtime.EnterChildViewEvent{
			ChildType: res.TargetType,
		}
		intents, tasks := c.core.HandleEnterChildView(ev)
		c.applyIntents(intents)
		if len(c.stack) > 0 {
			top := &c.stack[len(c.stack)-1]
			if top.ID == runtime.ScreenChildList {
				top.Ctx.ResourceType = res.TargetType
				if top.State.List == nil {
					top.State.List = &ListState{Loading: true}
					applyListDefaults(top.State.List, top.Ctx.ResourceType)
				}
				if res.FetchFilter != nil {
					top.State.List.ParentContext = res.FetchFilter
				}
			}
		}
		return tasks

	case runtime.NavigationKindFlash:
		// Flash is surfaced as a FlashIntent by HandleRelatedNavigate — no
		// stack change needed here.

	default:
		// NavigationKindUnknown or future kinds: no-op.
	}
	return nil
}

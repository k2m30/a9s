// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"github.com/k2m30/a9s/v3/core/runtime"
)

// ApplyIntents applies a slice of UIIntents to the controller's screen stack
// and state: stack navigation (Push/Pop/Replace/PopSelector), menu
// availability/issue/progress patches, list enrichment, identity, flash, and
// error-log/hint state. The few remaining variants are intentional no-ops
// (documented at the default case) — renderer-specific or served via another
// controller path, not migration leftovers.
//
// ApplyIntents never panics on a PopScreen against an empty stack.
// It returns the post-apply ViewState snapshot.
func (c *Controller) ApplyIntents(intents []runtime.UIIntent) ViewState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.applyIntents(intents)
}

// applyIntents is the lock-free implementation of ApplyIntents.
// Callers must hold c.mu (write).
func (c *Controller) applyIntents(intents []runtime.UIIntent) ViewState {
	c.applyIntentsLocked(intents)
	return c.snapshot()
}

// applyIntentsLocked applies intents without building a ViewState. For a
// caller already inside a handler that snapshots once at the end — the list
// lane observing its own type, say — rendering a second snapshot per intent
// batch is work nobody reads. Callers must hold c.mu (write).
func (c *Controller) applyIntentsLocked(intents []runtime.UIIntent) {
	for _, intent := range intents {
		switch v := intent.(type) {
		case runtime.PushScreen:
			ctx := v.Context
			// EnterChildView-emitted pushes (HandleEnterChildView) leave Context
			// zero-valued and carry the child type in ChildListPayload instead —
			// resolve it here so the pushed Screen.Ctx.ResourceType is non-empty
			// (topListState/ensureListState key off it, and the renderer builder
			// resolves the same field). Mirrors the TUI adapter's pre-collapse
			// PushScreen case in app_dispatch.go.
			if ctx.ResourceType == "" {
				if clp, ok := v.Payload.(runtime.ChildListPayload); ok {
					ctx.ResourceType = clp.ChildType
				}
			}
			c.stack = append(c.stack, Screen{
				ID:  v.ID,
				Ctx: ctx,
			})

		case runtime.PopScreen:
			// Never pop the root screen (the menu) — mirrors the TUI's popView
			// (app_stack.go), which refuses to pop the last screen. Popping to an
			// empty stack would blank the app to BodyKindUnknown.
			if len(c.stack) > 1 {
				c.stack = c.stack[:len(c.stack)-1]
			}

		case runtime.ReplaceScreen:
			if len(c.stack) == 0 {
				c.stack = append(c.stack, Screen{ID: v.ID, Ctx: v.Context})
			} else {
				c.stack[len(c.stack)-1] = Screen{ID: v.ID, Ctx: v.Context}
			}

		case runtime.PopSelectorIntent:
			// Pop the top screen when it is a selector (profile/region/theme).
			// Emitted by HandleProfileSelected / HandleRegionSelected /
			// HandleThemeSelected so the selector dismisses after confirm.
			if len(c.stack) > 0 {
				top := c.stack[len(c.stack)-1]
				if top.ID == runtime.ScreenProfileSelector ||
					top.ID == runtime.ScreenRegion ||
					top.ID == runtime.ScreenTheme {
					c.stack = c.stack[:len(c.stack)-1]
				}
			}

		case runtime.PatchMenuAvailability:
			if ms := c.rootMenuState(); ms != nil {
				// C1/C9: a live answer outranks any seed. A cache-origin
				// patch that arrives after this type was verified this
				// session is a load that lost the race — applying it would
				// regress the count the operator is looking at back to the
				// disk value and re-dim it as unverified. The issue badge
				// below is subject to the same rule, through the same
				// predicate.
				if !menuObservationOutranksStored(ms, v.ResourceType, v.Origin) {
					break
				}
				// Store under the key as emitted by the runtime (may be an alias
				// such as "rds" for ShortName "dbi"). buildMenuBody resolves the
				// active key per item using menuActiveKey().
				applyAvailabilityObservation(ms, v.ResourceType, v.Count, v.Truncated)
				// Per cache contract C3: track cache-seeded vs live-verified origin
				// independently of the exactness guard above — a truncated
				// sweep result that loses the count/truncated race still
				// means the type WAS live-checked this session, so its
				// origin must still flip to "verified".
				if v.Origin != "" {
					if ms.Origin == nil {
						ms.Origin = make(map[string]string)
					}
					ms.Origin[v.ResourceType] = v.Origin
				}
			}

		case runtime.PatchMenu:
			// A live observation (empty Origin): it assigns, exactly as it
			// always has, but through the one badge writer.
			if ms := c.rootMenuState(); ms != nil {
				c.applyMenuIssueObservation(ms, v.ResourceType, v.Issues, v.Truncated, "")
			}

		case runtime.PatchMenuIssueBatch:
			if ms := c.rootMenuState(); ms != nil && v.Known != nil {
				for name, k := range v.Known {
					if k {
						c.applyMenuIssueObservation(ms, name, v.Counts[name], v.Truncated[name], v.Origin)
					}
				}
			}

		case runtime.PatchMenuCheckProgress:
			if ms := c.rootMenuState(); ms != nil {
				ms.AvailChecked = v.Checked
				ms.AvailTotal = v.Total
			}

		case runtime.PatchMenuProbeCause:
			if ms := c.rootMenuState(); ms != nil {
				if v.Cause == "" {
					delete(ms.ProbeCause, v.ResourceType)
					break
				}
				if ms.ProbeCause == nil {
					ms.ProbeCause = make(map[string]string)
				}
				ms.ProbeCause[v.ResourceType] = v.Cause
			}

		case runtime.PatchMenuEnrichProgress:
			if ms := c.rootMenuState(); ms != nil {
				ms.EnrichChecked = v.Checked
				ms.EnrichTotal = v.Total
			}

		case runtime.MenuClearAvailabilityIntent:
			if ms := c.rootMenuState(); ms != nil {
				ms.ClearAvailability()
			}
			// C9: this intent is the only rotation-visible chokepoint on the
			// Controller — fired by both HandleProfileSelected and
			// HandleRegionSelected (core/runtime/handlers.go), as well as
			// menu Ctrl+R. session.Rotate bumps generation counters but never
			// reaches into the Controller, so without this a stale
			// enrichmentStore[type] entry from a prior profile/region pair can
			// resurrect its glyphs on a same-ID resource under the new pair. A
			// frame may never mix findings from two profile/region pairs.
			c.enrichmentStore = nil
			c.enrichmentDetails = nil
			c.enrichmentTruncated = nil
			c.enrichmentGen++
			// The sweep acknowledgements belong to the availability state
			// this intent clears: a pair switch or a manual refresh starts a
			// new sweep, and a type acknowledged by the previous one must not
			// let the menu report the new one as already finished.
			c.menuSweepAcked = nil

		case runtime.PatchResourceList:
			// Apply enrichment data (findings + issue badge) to the controller's
			// enrichment store. Resource rows themselves arrive via applyResourcesLoaded
			// (called from the task-result lane); this intent carries Wave-2 data only.
			if v.Enrichment != nil {
				// v.Issues is populated by the sole PatchResourceList producer
				// (runtime/handlers_availability.go's HandleEnrichmentChecked) with
				// the unified cross-wave count on every emission; nil is a
				// defensive fallback for a hypothetical future producer that
				// forgets to set it, not an observed case today.
				issueCount, issueTruncated := 0, false
				issuesAuthoritative := v.Issues != nil
				if v.Issues != nil {
					issueCount, issueTruncated = v.Issues.Count, v.Issues.Truncated
				}
				// issuesAuthoritative=false when v.Issues is nil: the 0/false
				// above is a defensive fallback standing in for "no issue-badge
				// result carried by this intent", not a confirmed zero-issue
				// Wave-2 result — it must not flip the menu's IssueKnown flag.
				//
				// v.Enrichment.Findings/AttentionDetails carry every
				// independently-evaluated Wave-2 condition per resource —
				// applyEnrichmentState stores them directly in the controller's
				// single enrichment store; no single-representative reduction or
				// fallback wrapping happens on this write path.
				c.applyEnrichmentState(v.ResourceType, issueCount, issueTruncated, v.Enrichment.Findings, v.Enrichment.AttentionDetails, issuesAuthoritative)
				// applyEnrichmentState only stores findings + the issue badge; the
				// Wave-2 column updates (status/summary) must also reach the cached
				// list rows or enriched columns render stale (ECR/WAF/CodeArtifact).
				c.applyListFieldUpdates(v.ResourceType, v.Enrichment.FieldUpdates)
				// ...and the findings themselves must land on the controller's own
				// rows (ls.Rows / the RowStore-backed type cache) — the list-open
				// save path persists from them, so without this the on-disk cache
				// rows carry no findings and reseed glyphless (violating
				// C6's persisted-findings round-trip). A
				// multi-condition resource keeps every Finding — and every
				// finding's own AttentionDetail — on the row.
				// TruncatedIDs is the runtime's own decision about which rows
				// this result speaks for; the controller folds the same rows
				// the runtime did and re-decides nothing.
				c.applyRowFindings(v.ResourceType, v.Enrichment.Findings, v.Enrichment.AttentionDetails, v.Enrichment.TruncatedIDs)
			}

		case runtime.SetIdentityIntent:
			// SetIdentityIntent is emitted by Core.HandleIdentityLoaded (via
			// HandleEvent) when the identity fetch succeeds. Store the resolved
			// domain mirror so snapshot can build IdentityBody without importing
			// core/aws or inspecting the TUI view stack.
			if v.Identity != nil {
				c.identityResult = v.Identity
				c.identityLoading = false
				c.identityErrMsg = ""
			}

		case runtime.ClearIdentityIntent:
			// Emitted unconditionally by HandleProfileSelected/HandleRegionSelected
			// (never by a same-pair refresh) so the previous pair's ARN is never
			// served — via CopyContent or the identity screen — after a rotation,
			// before the new pair's identity fetch (if any) lands. identityLoading
			// is deliberately left alone: it is owned by handleActionOpenIdentity's
			// fetch-start/fetch-end lifecycle, not this rotation chokepoint.
			c.identityResult = nil
			c.identityErrMsg = ""

		case runtime.FlashIntent:
			// Surface the transient notification (e.g. the API-error flash from
			// HandleAPIError) as Header.Flash; cleared at the start of the next Apply.
			c.flash = Flash{Text: v.Text, IsError: v.IsError}

		case runtime.ClearFlash:
			c.flash = Flash{}

		case runtime.ClearActiveListLoadingIntent:
			// A failed AWS fetch must drop every in-flight indicator on the
			// active list (Loading, LoadingMore, Refreshing) rather than
			// stranding one of them set forever (emitted by HandleAPIError;
			// mirrors ClearListLoading, the TUI-lane equivalent).
			if ls := c.topListState(); ls != nil {
				ls.clearFetchInFlight()
				// Per cache contract C4: a fetch failure over cached content stops the
				// refreshing marker and swaps in an error marker instead —
				// nothing goes blank, rows stay on screen.
				if v.Err != "" {
					ls.LastFetchError = v.Err
				}
			}

		case runtime.SetErrorHintIntent:
			c.showErrorHint = v.Show

		case runtime.AppendErrorHistoryIntent:
			c.errorHistory = append(c.errorHistory, controllerErrorEntry{
				t:       v.Time,
				message: v.Message,
			})

		case runtime.PatchResourceCache:
			c.core.SetResourceCache(v.ResourceType, v.Entry)

		case runtime.PatchRelatedCache:
			// Idempotent per def: REPLACES any existing entry for this
			// DefDisplayName under the key rather than appending a second
			// one. The cache is read for completeness
			// (core/app/navigate.go's relatedCacheCoverage, R2/R3) as well
			// as panel replay, so a re-observed result for an
			// already-covered def must update in place — an append-only
			// write let a re-run of the related-check fan-out (e.g. a
			// YAML/JSON open, whose suppression decision is independent of
			// whether a detail panel is on top) silently double up every
			// def's entry on each pass. This is also the write-through that
			// makes the cache-hit replay path in openSelectedListDetail
			// (controller.go) and openRelatedDetail (navigate.go) hit on a
			// second open of the same detail.
			if v.SourceID != "" {
				key := runtime.RelatedCacheKey(v.ResourceType, v.SourceID)
				existing, _ := c.core.RelatedCacheGet(key)
				entry := runtime.RelatedCacheResult{DefDisplayName: v.DefDisplayName, Result: v.Result}
				replaced := false
				for i := range existing {
					if existing[i].DefDisplayName == v.DefDisplayName {
						existing[i] = entry
						replaced = true
						break
					}
				}
				if !replaced {
					existing = append(existing, entry)
				}
				c.core.RelatedCacheSet(key, existing)
			}

		case runtime.PatchLazyResourceCache:
			c.core.ExtendLazyResourceCache(v.Adds)

		case runtime.PatchDetail:
			// Apply enrichment findings to every stacked detail screen of this
			// resource type — not just the currently active one. When a user has
			// navigated from detail-A to detail-B, enrichment results for both
			// must reach both screens, so popping back to detail-A shows the
			// correct Attention section immediately. Mirrors the TUI adapter's
			// former local PatchDetail case in app_dispatch.go (removed — this is
			// now the single source of truth for both TUI and web/headless).
			if len(v.EnrichmentFindings) == 0 {
				// Nil or empty Findings means all resources of this type have
				// recovered: clear enrichment from every stacked detail screen.
				c.clearDetailFindingsForType(v.ResourceType)
			} else {
				// Clear stale findings from every stacked detail of this type first,
				// so a resource that recovered (absent from the new map) loses its
				// Attention; then re-apply for resources still reporting findings.
				// applyDetailFindingsForResource searches all stacked screens by
				// (type, id), so a stacked-but-not-active detail is still updated.
				// Every finding in EnrichmentFindings[resourceID] is applied — not
				// just a single worst-severity representative — so a
				// multi-condition resource's Attention block shows every
				// independently-evaluated finding on an already-open detail.
				c.clearDetailFindingsForType(v.ResourceType)
				for resourceID, fs := range v.EnrichmentFindings {
					c.applyDetailFindingsForResource(v.ResourceType, resourceID, fs, v.EnrichmentAttentionDetails[resourceID])
				}
			}

		// The remaining intents are renderer-specific or are served through
		// another controller path, so they are intentional no-ops here rather
		// than migration leftovers:
		//   RefreshActiveListIntent — carries no state of its own to apply here;
		//                             the C10 replay it signals is turned into a
		//                             fetch task by refreshTasksForIntents, which
		//                             callers (Handle, BootstrapLive) invoke
		//                             alongside applyIntents.
		//   HeaderInvalidateIntent  — the Header is rebuilt from core on every
		//                             snapshot(); there is nothing to invalidate.
		//   ApplyThemeIntent        — lipgloss theming is a TUI concern; the web
		//                             renderer uses static CSS.
		default:
			_ = v
		}
	}
}

// refreshTasksForIntents scans intents for RefreshActiveListIntent and, when
// present, returns the active-list (or, P5, active costs screen) refresh
// tasks (C10: a navigation/fetch issued before AWS connect completes must
// replay once connected). Shared by Handle and BootstrapLive so the scan is
// not duplicated across the TUI-independent ClientsReady seams. Callers must
// hold c.mu.
func (c *Controller) refreshTasksForIntents(intents []runtime.UIIntent) []runtime.TaskRequest {
	for _, intent := range intents {
		if _, ok := intent.(runtime.RefreshActiveListIntent); ok {
			if tasks := c.activeListRefreshTasks(0); tasks != nil {
				return tasks
			}
			return c.costsRefreshTasks()
		}
	}
	return nil
}

package app

import "github.com/k2m30/a9s/v3/internal/runtime"

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
	for _, intent := range intents {
		switch v := intent.(type) {
		case runtime.PushScreen:
			c.stack = append(c.stack, Screen{
				ID:  v.ID,
				Ctx: v.Context,
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
				if ms.Availability == nil {
					ms.Availability = make(map[string]int)
				}
				if ms.Truncated == nil {
					ms.Truncated = make(map[string]bool)
				}
				// Store under the key as emitted by the runtime (may be an alias
				// such as "rds" for ShortName "dbi"). buildMenuBody resolves the
				// active key per item using menuActiveKey().
				//
				// DEF-2/C5: a truncated probe result must never downgrade an
				// already-exact stored total — mirrors the guard
				// SaveResourceListCache/SaveAvailabilityCache already apply on the
				// disk-persist path (internal/runtime/probes.go). Exactness only
				// ever advances: an untruncated observation always wins; a
				// truncated one only wins when the current entry is itself unknown
				// or already truncated, or reports a count that is not SMALLER than
				// the one already stored (equal or larger) — a same-count update
				// that only flips Truncated (e.g. toggling the lower-bound marker
				// on an unchanged count) is not the "downgrade" C5 forbids; only a
				// truncated result reporting FEWER items than the stored exact
				// total is.
				curCount, known := ms.Availability[v.ResourceType]
				curTruncated := ms.Truncated[v.ResourceType]
				if !v.Truncated || !known || curTruncated || v.Count >= curCount {
					ms.Availability[v.ResourceType] = v.Count
					ms.Truncated[v.ResourceType] = v.Truncated
				}
				// DEF-6/C3: track cache-seeded vs live-verified origin
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
			if ms := c.rootMenuState(); ms != nil {
				if ms.IssueCounts == nil {
					ms.IssueCounts = make(map[string]int)
				}
				if ms.IssueKnown == nil {
					ms.IssueKnown = make(map[string]bool)
				}
				if ms.IssueTruncated == nil {
					ms.IssueTruncated = make(map[string]bool)
				}
				ms.IssueCounts[v.ResourceType] = v.Issues
				ms.IssueKnown[v.ResourceType] = true
				ms.IssueTruncated[v.ResourceType] = v.Truncated
			}

		case runtime.PatchMenuIssueBatch:
			if ms := c.rootMenuState(); ms != nil && v.Known != nil {
				if ms.IssueCounts == nil {
					ms.IssueCounts = make(map[string]int)
				}
				if ms.IssueKnown == nil {
					ms.IssueKnown = make(map[string]bool)
				}
				if ms.IssueTruncated == nil {
					ms.IssueTruncated = make(map[string]bool)
				}
				for name, k := range v.Known {
					if k {
						ms.IssueCounts[name] = v.Counts[name]
						ms.IssueKnown[name] = true
						ms.IssueTruncated[name] = v.Truncated[name]
					}
				}
			}

		case runtime.PatchMenuCheckProgress:
			if ms := c.rootMenuState(); ms != nil {
				ms.AvailChecked = v.Checked
				ms.AvailTotal = v.Total
			}

		case runtime.PatchMenuEnrichProgress:
			if ms := c.rootMenuState(); ms != nil {
				ms.EnrichChecked = v.Checked
				ms.EnrichTotal = v.Total
			}

		case runtime.MenuClearAvailabilityIntent:
			if ms := c.rootMenuState(); ms != nil {
				ms.Availability = nil
				ms.Truncated = nil
				ms.AvailChecked = 0
				ms.AvailTotal = 0
				ms.IssueCounts = nil
				ms.IssueKnown = nil
				ms.IssueTruncated = nil
				ms.EnrichChecked = 0
				ms.EnrichTotal = 0
			}

		case runtime.PatchResourceList:
			// Apply enrichment data (findings + issue badge) to the controller's
			// enrichment store. Resource rows themselves arrive via applyResourcesLoaded
			// (called from the task-result lane); this intent carries Wave-2 data only.
			if v.Enrichment != nil {
				c.applyEnrichmentState(v.ResourceType, 0, false, v.Enrichment.Findings)
				// applyEnrichmentState only stores findings + the issue badge; the
				// Wave-2 column updates (status/summary) must also reach the cached
				// list rows or enriched columns render stale (ECR/WAF/CodeArtifact).
				c.applyListFieldUpdates(v.ResourceType, v.Enrichment.FieldUpdates)
			}

		case runtime.SetIdentityIntent:
			// SetIdentityIntent is emitted by Core.HandleIdentityLoaded (via
			// HandleEvent) when the identity fetch succeeds. Store the resolved
			// domain mirror so snapshot can build IdentityBody without importing
			// internal/aws or inspecting the TUI view stack.
			if v.Identity != nil {
				c.identityResult = v.Identity
				c.identityLoading = false
				c.identityErrMsg = ""
			}

		case runtime.FlashIntent:
			// Surface the transient notification (e.g. the API-error flash from
			// HandleAPIError) as Header.Flash; cleared at the start of the next Apply.
			c.flash = Flash{Text: v.Text, IsError: v.IsError}

		case runtime.ClearFlash:
			c.flash = Flash{}

		case runtime.ClearActiveListLoadingIntent:
			// A failed AWS fetch must drop the spinner on the active list rather
			// than leaving it stuck Loading=true (emitted by HandleAPIError).
			if ls := c.topListState(); ls != nil {
				ls.Loading = false
				// DEF-5/C4: a fetch failure over cached content stops the
				// refreshing marker and swaps in an error marker instead —
				// nothing goes blank, rows stay on screen.
				if v.Err != "" {
					ls.Refreshing = false
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
			// Mirrors the TUI adapter's app_dispatch.go case: append to any
			// existing slice under the runtime.RelatedCacheKey. This is the
			// write-through that makes the cache-hit replay path in
			// openSelectedListDetail (controller.go) and openRelatedDetail
			// (navigate.go) hit on a second open of the same detail.
			if v.SourceID != "" {
				key := runtime.RelatedCacheKey(v.ResourceType, v.SourceID)
				existing, _ := c.core.RelatedCacheGet(key)
				c.core.RelatedCacheSet(key, append(existing, runtime.RelatedCacheResult{
					DefDisplayName: v.DefDisplayName,
					Result:         v.Result,
				}))
			}

		case runtime.PatchLazyResourceCache:
			c.core.ExtendLazyResourceCache(v.Adds)

		// The remaining intents are renderer-specific or are served through
		// another controller path, so they are intentional no-ops here rather
		// than migration leftovers:
		//   PatchDetail             — detail enrichment is applied via
		//                             ApplyDetailFinding (the task-result lane).
		//   RefreshActiveListIntent — refresh runs as the Refresh action's fetch
		//                             task, not as an intent.
		//   HeaderInvalidateIntent  — the Header is rebuilt from core on every
		//                             snapshot(); there is nothing to invalidate.
		//   ApplyThemeIntent        — lipgloss theming is a TUI concern; the web
		//                             renderer uses static CSS.
		default:
			_ = v
		}
	}
	return c.snapshot()
}

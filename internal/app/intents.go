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
				ms.Availability[v.ResourceType] = v.Count
				ms.Truncated[v.ResourceType] = v.Truncated
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
			}

		case runtime.SetErrorHintIntent:
			c.showErrorHint = v.Show

		case runtime.AppendErrorHistoryIntent:
			c.errorHistory = append(c.errorHistory, controllerErrorEntry{
				t:       v.Time,
				message: v.Message,
			})

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
		//   PatchResourceCache / PatchRelatedCache / PatchLazyResourceCache —
		//                             the controller seeds its caches via
		//                             applyResourcesLoaded; these incremental TUI
		//                             cache writes belong to the cache-subsystem
		//                             rework (plan goal 2), not the renderer split.
		default:
			_ = v
		}
	}
	return c.snapshot()
}


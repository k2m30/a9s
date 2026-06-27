package app

import (
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// Controller is the headless app controller. It wraps runtime.Core and
// owns the screen stack.
//
// Both renderers (TUI and web) hold a *Controller. The TUI calls Apply
// for key-driven actions and Handle for async Task results; the web
// renderer does the same over HTTP/SSE. Tests use DrainSync to drive the
// controller synchronously without a terminal or sleep.
//
// Task ownership: Apply and Handle RETURN []runtime.TaskRequest to the
// caller. The controller retains no task state — the host (TUI, web, or
// DrainSync) is responsible for executing and routing task results.
type Controller struct {
	core  *runtime.Core
	stack []Screen
}

// New constructs a Controller backed by the given runtime Core.
func New(core *runtime.Core) *Controller {
	return &Controller{core: core}
}

// Apply translates a semantic Action into the matching Core command, applies
// the returned UIIntents to the screen stack, enqueues returned TaskRequests,
// and returns the updated ViewState plus newly-enqueued TaskRequests.
//
// USER-INTENT lane: each Action.Kind maps to a specific Core.HandleX method.
// PR-B wires the six navigate/session actions that need no selected-row state.
// PR-C-blocked actions (row-dependent) are kept as documented no-ops.
func (c *Controller) Apply(a Action) (ViewState, []runtime.TaskRequest) {
	switch a.Kind {

	// --- Navigate actions (PR-B) ---

	case ActionOpenHelp:
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
		c.applyNavResult(res)
		return c.Snapshot(), tasks

	case ActionBack:
		// Pop a single screen, mirroring the TUI's m.popView() — NOT a full
		// collapse (root-collapse is the "root" Command). Per-view Esc semantics
		// (clear filter/search before popping) arrive with PR-C view state.
		c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		return c.Snapshot(), nil

	case ActionOpenIdentity:
		// The runtime has no NavigateTargetIdentity: the TUI opens the identity
		// screen via direct key-handling (not HandleNavigate). The headless
		// controller pushes ScreenIdentity directly so tests can assert the stack
		// without standing up a full TUI.
		c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenIdentity}})
		// TODO PR-C: set IdentityBody.Loading = true once body state is lifted here.
		fetchTask := runtime.TaskRequest{
			Key:     runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity},
			Payload: runtime.FetchIdentityPayload{},
		}
		return c.Snapshot(), []runtime.TaskRequest{fetchTask}

	// --- Session-selection actions (PR-B) ---

	case ActionSelectProfile:
		// ConnectGen is read pre-Rotate; HandleProfileSelected calls Rotate internally.
		// NewGen is passed as the bumped flash gen for the "Switching to …" tick.
		// The headless controller has no flash.gen to bump, so we pass the current
		// ConnectGen as a stable stand-in — the ClearFlash tick is adapter-owned.
		intents, tasks := c.core.HandleProfileSelected(runtime.ProfileSelectedEvent{
			Profile: a.Arg,
			NewGen:  c.core.ConnectGen(),
		})
		c.ApplyIntents(intents)
		return c.Snapshot(), tasks

	case ActionSelectRegion:
		intents, tasks := c.core.HandleRegionSelected(runtime.RegionSelectedEvent{
			Region: a.Arg,
			NewGen: c.core.ConnectGen(),
		})
		c.ApplyIntents(intents)
		return c.Snapshot(), tasks

	case ActionSelectTheme:
		intents, tasks := c.core.HandleThemeSelected(runtime.ThemeSelectedEvent{
			Theme: a.Arg,
		})
		c.ApplyIntents(intents)
		return c.Snapshot(), tasks

	// --- Command lane (PR-B) ---

	case ActionCommand:
		// Arg carries a colon-command token (mirrors executeCommand in app_input.go).
		// Only arg-driven tokens are dispatched here; tokens that need selected-row
		// or per-screen state are noted as PR-C TODOs below.
		switch a.Arg {
		case "root", "main":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetMainMenu})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		case "profile", "ctx":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetProfile})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		case "region":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetRegion})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		case "theme":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetTheme})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		case "help":
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
			c.applyNavResult(res)
			return c.Snapshot(), tasks

		default:
			// Resource short-name or alias (e.g. "ec2", "s3", "dbi").
			if rt := resource.FindResourceType(a.Arg); rt != nil {
				res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
					Target:       runtime.NavigateTargetResourceList,
					ResourceType: a.Arg,
				})
				c.applyNavResult(res)
				return c.Snapshot(), tasks
			}
			// TODO PR-C: "q"/"quit" needs tea.Quit from the renderer, not the controller.
			// Unknown tokens are silently dropped at this layer; the renderer flashes.
		}
		return c.Snapshot(), nil

	// --- PR-C-blocked actions: need selected-row / per-screen view state ---

	case ActionOpenDetail, ActionSelect,
		ActionOpenYAML, ActionOpenJSON,
		ActionReveal, ActionChildView,
		ActionToggleRelated,
		ActionLoadMore:
		// TODO PR-C: needs selected-row / view state (see plan PR-C)
		return c.Snapshot(), nil
	}

	// All remaining actions (movement, filter, sort, search, copy, refresh, quit,
	// toggle-wrap, toggle-attention, command) are either movement-only or require
	// per-screen state lifted in PR-C. Return current snapshot with no tasks.
	return c.Snapshot(), nil
}

// Handle feeds an event through runtime.Core.HandleEvent, applies the returned
// UIIntents to the screen stack, enqueues returned TaskRequests, and returns
// the updated ViewState plus those TaskRequests.
//
// TASK-RESULT lane: completed background-task results arrive here. The caller
// is responsible for passing only values that implement runtime.Event
// (i.e. messages.Event) — unrecognised concrete types fall through to
// Core.HandleEvent's default nil, nil path.
func (c *Controller) Handle(ev runtime.Event) (ViewState, []runtime.TaskRequest) {
	intents, tasks := c.core.HandleEvent(ev)
	c.ApplyIntents(intents)
	return c.Snapshot(), tasks
}

// ApplyIntents applies a slice of UIIntents to the controller's screen stack.
// Stack-navigation intents (PushScreen / PopScreen / ReplaceScreen) are fully
// implemented. All other intent variants are no-ops with a PR-C marker so
// tests can drive the stack directly without standing up a full renderer.
//
// ApplyIntents never panics on a PopScreen against an empty stack.
// It returns the post-apply ViewState snapshot.
func (c *Controller) ApplyIntents(intents []runtime.UIIntent) ViewState {
	for _, intent := range intents {
		switch v := intent.(type) {
		case runtime.PushScreen:
			c.stack = append(c.stack, Screen{
				ID:  v.ID,
				Ctx: v.Context,
			})

		case runtime.PopScreen:
			if len(c.stack) > 0 {
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

		// TODO PR-C: PatchResourceList mutates state lifted in PR-C
		// TODO PR-C: PatchDetail mutates state lifted in PR-C
		// TODO PR-C: PatchMenu mutates state lifted in PR-C
		// TODO PR-C: PatchMenuAvailability mutates state lifted in PR-C
		// TODO PR-C: PatchMenuIssueBatch mutates state lifted in PR-C
		// TODO PR-C: PatchMenuCheckProgress mutates state lifted in PR-C
		// TODO PR-C: PatchMenuEnrichProgress mutates state lifted in PR-C
		// TODO PR-C: FlashIntent mutates state lifted in PR-C
		// TODO PR-C: ClearFlash mutates state lifted in PR-C
		// TODO PR-C: SetErrorHintIntent mutates state lifted in PR-C
		// TODO PR-C: AppendErrorHistoryIntent mutates state lifted in PR-C
		// TODO PR-C: ClearActiveListLoadingIntent mutates state lifted in PR-C
		// TODO PR-C: MenuClearAvailabilityIntent mutates state lifted in PR-C
		// TODO PR-C: RefreshActiveListIntent mutates state lifted in PR-C
		// TODO PR-C: PatchResourceCache mutates state lifted in PR-C
		// TODO PR-C: PatchRelatedCache mutates state lifted in PR-C
		// TODO PR-C: PatchLazyResourceCache mutates state lifted in PR-C
		// TODO PR-C: SetIdentityIntent mutates state lifted in PR-C
		// TODO PR-C: HeaderInvalidateIntent mutates state lifted in PR-C
		// TODO PR-C: ApplyThemeIntent mutates state lifted in PR-C
		default:
			_ = v
		}
	}
	return c.Snapshot()
}

// Snapshot builds a ViewState from the current controller state. In PR-A
// only the Header, FrameTitle, and BodyKind are populated; full body
// rendering is added in PR-C when per-screen state is lifted here.
//
// Snapshot never panics on an empty stack — it returns a ViewState with
// BodyKindUnknown.
func (c *Controller) Snapshot() ViewState {
	vs := ViewState{
		Header: Header{
			Profile: c.core.Profile(),
			Region:  c.core.Region(),
		},
	}
	if len(c.stack) == 0 {
		vs.Body.Kind = BodyKindUnknown
		return vs
	}
	top := c.stack[len(c.stack)-1]
	vs.FrameTitle = string(top.ID)
	vs.Body.Kind = bodyKindForScreen(top)
	// TODO PR-C: populate vs.Body.{List,Detail,Text,Menu,Selector} from top.State.
	return vs
}

// applyNavResult converts a NavigateResult into PushScreen/ReplaceScreen/PopScreen
// stack operations. Called by Apply after HandleNavigate returns.
//
// The adapter (not the runtime) decides which ScreenID to push for each kind;
// this method encodes that mapping for the headless controller.
//
// Kinds that require selected-row or resource data (PushDetail, PushYAML,
// PushJSON, PushResourceList/Cached, FetchReveal) are deferred to PR-C where
// per-screen state is lifted into the controller.
func (c *Controller) applyNavResult(res runtime.NavigateResult) {
	switch res.Kind {
	case runtime.NavigateKindPopAll:
		// Pop every screen until the stack is empty (return to main menu).
		for len(c.stack) > 0 {
			c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})
		}

	case runtime.NavigateKindPushHelp:
		c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenHelp}})

	case runtime.NavigateKindPushRegion:
		c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenRegion}})

	case runtime.NavigateKindPushTheme:
		c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenTheme}})

	case runtime.NavigateKindFetchProfiles:
		// No stack change — the adapter starts the fetch task; when the result
		// arrives (ProfilesLoaded), HandleProfilesLoaded pushes ScreenProfileSelector.

	case runtime.NavigateKindFlash, runtime.NavigateKindNoop:
		// No stack change — flash is surfaced via FlashIntent in the intent stream.

	// TODO PR-C: NavigateKindPushResourceList / NavigateKindPushResourceListCached
	//            need ScreenContext{ResourceType} + cache entry from the result.
	// TODO PR-C: NavigateKindPushDetail / NavigateKindPushYAML / NavigateKindPushJSON
	//            need ScreenContext{ResourceType, ResourceID} from result.Resource.
	// TODO PR-C: NavigateKindFetchReveal needs result.Resource for the reveal payload.
	}
}

// applyRelatedNavResult converts a NavigationResult into stack operations.
//
// NavigationResult carries NavigationKind plus TargetType, TargetID,
// RelatedIDs, FetchFilter, FilterText — no ScreenID; the controller maps
// kind → ScreenID.
//
// All NavigationKinds (ResourceList, FilteredList, Detail, EnterChildView)
// need selected-row and per-screen state that PR-C lifts into the controller.
//
//nolint:unused // wired in PR-C
func (c *Controller) applyRelatedNavResult(_ runtime.NavigationResult) {
	// TODO PR-C: needs selected-row / view state (see plan PR-C)
}

// bodyKindForScreen maps a Screen to the BodyKind a renderer uses to
// select the correct template/view.
func bodyKindForScreen(s Screen) BodyKind {
	switch s.ID {
	case runtime.ScreenProfileSelector, runtime.ScreenRegion, runtime.ScreenTheme:
		return BodyKindSelector
	case runtime.ScreenReveal:
		return BodyKindDetail
	case runtime.ScreenChildList:
		return BodyKindList
	case runtime.ScreenHelp:
		return BodyKindHelp
	case runtime.ScreenIdentity:
		return BodyKindIdentity
	default:
		// Capability screens and future IDs not yet enumerated here.
		// PR-C will extend this switch as new ScreenIDs are registered.
		return BodyKindUnknown
	}
}

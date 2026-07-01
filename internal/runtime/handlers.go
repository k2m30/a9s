// handlers.go — shell-level handler bodies for the platform-agnostic Core.
//
// Each handler is a (c *Core) Handle* method that consumes a typed
// runtime event, applies session-scoped state changes, and returns
// ([]UIIntent, []TaskRequest) so the core stays free of
// Bubble Tea / Lipgloss / Bubbles types. The TUI adapter wraps each handler
// in a thin (tea.Model, tea.Cmd) shim that translates the
// adapter's messages.* into the runtime event, calls the Core method, and
// translates the returned intents and tasks back into adapter-visible side
// effects.
//
// What lives here:
//
//	HandleFlash          — flash gen bump + history; schedules ClearFlash tick.
//	HandleClearFlash     — flash auto-clear honouring the session-owned gen.
//	HandleAPIError       — AWS error classification + flash with [code] message.
//	HandleClientsReady   — clients/identity wiring + post-connect refresh / boot.
//	HandleProfileSelected— rotate, rollback latch, request reconnect.
//	HandleRegionSelected — mirror of HandleProfileSelected for region.
//	HandleProfilesLoaded — emits PushScreen{ScreenProfileSelector,...}.
//	HandleValueRevealed  — emits PushScreen{ScreenReveal,...} or FlashIntent.
//	HandleEnterChildView — emits PushScreen{ScreenChildList,...} + fetch task.
//	HandleThemeSelected  — emits TaskKindReadThemeFile.
//	HandleThemeFileRead  — parses YAML; on parse OK emits Apply/Pop/Flash +
//	                       Save task; on parse fail emits error flash only
//	                       (Save is gated on parse success).
//
// What stays in the TUI adapter:
//
//   - handleKeyMsg (keyboard dispatch) — owns key.Matches semantics on
//     adapter-owned tea types.
package runtime

import (
	"fmt"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// Flash auto-clear durations. apiErrorFlashDuration is the longer 5 s window
// for AWS errors; flashDuration is the default 2 s window for status flashes.
const (
	flashDuration         = 2 * time.Second
	apiErrorFlashDuration = 5 * time.Second
)

// FlashEvent is the adapter-translated form of messages.Flash used by
// HandleFlash. NewGen is the flash generation the adapter has already
// bumped to on its flashState before calling into the Core; the handler
// echoes it back via FlashTickPayload.Gen so the scheduled ClearFlashMsg
// references the same gen the active flash carries.
type FlashEvent struct {
	Text    string
	IsError bool
	NewGen  domain.Gen
}

// ClearFlashEvent is the adapter-translated form of messages.ClearFlash.
// Gen is the gen the original tea.Tick carried; CurrentGen is the gen the
// adapter has now. The handler treats them as stale when they disagree.
// IsError lets the handler emit SetErrorHintIntent when the cleared flash
// was an error flash (mirrors the adapter's flashState.isError check).
type ClearFlashEvent struct {
	Gen        domain.Gen
	CurrentGen domain.Gen
	IsError    bool
}

// APIErrorEvent is the adapter-translated form of messages.APIError.
// NewGen is the flash generation the adapter has already bumped to before
// calling into the Core; the handler echoes it back via the FlashTickPayload.
type APIErrorEvent struct {
	Err    error
	NewGen domain.Gen
}

// ClientsReadyEvent mirrors the fields of messages.ClientsReady the
// runtime needs to make its dispatch decision. The concrete *ServiceClients
// is kept as any so this file stays free of any tui-only dependency on the
// message type.
//
// StackDepth and HasActiveRL are renderer-shape inputs the adapter computes
// from its view stack. StackDepth == 1 means only the main menu is on
// screen; HasActiveRL is true when the active view is a ResourceListModel.
// The runtime uses these to decide whether to emit the one-shot -c
// navigation and the post-switch refresh.
type ClientsReadyEvent struct {
	Clients     any
	Err         error
	Region      string
	Gen         domain.Gen
	StackDepth  int
	HasActiveRL bool
	NewGen      domain.Gen
}

// ProfileSelectedEvent / RegionSelectedEvent mirror the corresponding
// messages.*Msg shapes. NewGen is the flash generation the adapter has
// already bumped to (used by the "Switching to …" flash tick).
type ProfileSelectedEvent struct {
	Profile string
	NewGen  domain.Gen
}

type RegionSelectedEvent struct {
	Region string
	NewGen domain.Gen
}

// HandleFlash bumps the flash generation, appends an error-history entry
// when the flash carries an error, and schedules the auto-clear tick at
// the default 2 s window.
func (c *Core) HandleFlash(ev FlashEvent) ([]UIIntent, []TaskRequest) {
	intents := []UIIntent{FlashIntent{Text: ev.Text, IsError: ev.IsError}}
	if ev.IsError {
		intents = append(intents, AppendErrorHistoryIntent{
			Time:    time.Now(),
			Message: ev.Text,
		})
	}
	tasks := []TaskRequest{{
		Key:     TaskKey{Kind: TaskKindFlashTick},
		Payload: FlashTickPayload{Gen: ev.NewGen, Duration: flashDuration},
	}}
	return intents, tasks
}

// HandleClearFlash clears the flash when the carried gen matches the
// adapter's current gen, and surfaces the persistent error hint after an
// error flash. Stale gens (rapid re-flash) are silently dropped.
func (c *Core) HandleClearFlash(ev ClearFlashEvent) ([]UIIntent, []TaskRequest) {
	if ev.Gen != ev.CurrentGen {
		return nil, nil
	}
	intents := []UIIntent{ClearFlash{}}
	if ev.IsError {
		intents = append(intents, SetErrorHintIntent{Show: true})
	}
	return intents, nil
}

// HandleAPIError classifies the AWS error, builds a "[code] message" flash
// text, records the error to history, clears the active list's loading
// indicator, and schedules the longer 5 s clear tick used for AWS errors.
func (c *Core) HandleAPIError(ev APIErrorEvent) ([]UIIntent, []TaskRequest) {
	code, message, _ := awsclient.ClassifyAWSError(ev.Err)
	var text string
	if code != "" && code != "Unknown" {
		text = fmt.Sprintf("[%s] %s", code, message)
	} else {
		text = ev.Err.Error()
	}
	intents := []UIIntent{
		FlashIntent{Text: text, IsError: true},
		AppendErrorHistoryIntent{Time: time.Now(), Message: text},
		ClearActiveListLoadingIntent{},
	}
	tasks := []TaskRequest{{
		Key:     TaskKey{Kind: TaskKindFlashTick},
		Payload: FlashTickPayload{Gen: ev.NewGen, Duration: apiErrorFlashDuration},
	}}
	return intents, tasks
}

// HandleClientsReady wires fresh AWS clients into the session, clears the
// rollback latch, fires identity + availability bootstrap tasks, and — on
// failure — rolls back to the previous stable profile/region. A
// PendingRefresh in the success path triggers a list re-fetch with a
// "Connected. Refreshing..." flash.
//
// Stale results (Gen != session.ConnectGen) are dropped: the user switched
// profile/region again while this connect was in flight.
func (c *Core) HandleClientsReady(ev ClientsReadyEvent) ([]UIIntent, []TaskRequest) {
	if ev.Gen != c.session.ConnectGen {
		return nil, nil
	}

	if ev.Err != nil {
		return c.handleClientsReadyFailure(ev)
	}

	return c.handleClientsReadySuccess(ev)
}

// handleClientsReadyFailure rolls Profile / Region back to the captured
// rollback target, emits the error flash + history entry, schedules the
// 5 s clear tick, and — when there are still valid clients (rollback to
// the old session) — refires identity and availability tasks against the
// retained transport so the post-rollback UI is consistent.
func (c *Core) handleClientsReadyFailure(ev ClientsReadyEvent) ([]UIIntent, []TaskRequest) {
	s := c.session
	if s.HasPrevState {
		s.Profile = s.PrevProfile
		s.Region = s.PrevRegion
	}
	s.HasPrevState = false
	s.PrevProfile = ""
	s.PrevRegion = ""
	s.PendingRefresh = false

	errText := ev.Err.Error()
	intents := []UIIntent{
		FlashIntent{Text: errText, IsError: true},
		AppendErrorHistoryIntent{Time: time.Now(), Message: errText},
	}
	tasks := []TaskRequest{{
		Key:     TaskKey{Kind: TaskKindFlashTick},
		Payload: FlashTickPayload{Gen: ev.NewGen, Duration: apiErrorFlashDuration},
	}}

	if s.Clients != nil {
		// P3 invariant: Session.Rotate() (run earlier on the switch attempt
		// that just failed) installed fresh IAMPolicies / IdentityStore /
		// RuleSets on the session. The retained transport still points at
		// the pre-rotate (now-discarded) stores, so Pattern-C related
		// checks (Glue tags, EBS Backup) and IAM lazy-add would read
		// sticky state until the next successful reconnect. Rewire on
		// rollback so the retained transport sees the post-rotate stores.
		s.Clients.SetIAMPolicies(s.IAMPolicies)
		s.Clients.SetIdentityStore(s.IdentityStore)
		s.Clients.SetRuleSets(s.RuleSets)

		s.IdentityFetching = true
		tasks = append(tasks, TaskRequest{
			Key:     TaskKey{Kind: TaskKindFetchIdentity},
			Payload: FetchIdentityPayload{},
		})
		if s.NoCache {
			tasks = append(tasks, TaskRequest{
				Key:     TaskKey{Kind: TaskKindDemoPrefetchCounts},
				Payload: DemoPrefetchCountsPayload{},
			})
		} else {
			tasks = append(tasks, TaskRequest{
				Key:     TaskKey{Kind: TaskKindLoadAvailCache},
				Payload: LoadAvailCachePayload{},
			})
		}
	}
	return intents, tasks
}

// handleClientsReadySuccess installs the new clients (or falls back to the
// pre-supplied demo transport when ev.Clients is nil), defaults Profile /
// Region when empty, dispatches the one-shot -c navigation, and fires the
// identity + availability tasks (plus a refresh-list flash + intent when
// PendingRefresh is set and the active view is a ResourceListModel).
func (c *Core) handleClientsReadySuccess(ev ClientsReadyEvent) ([]UIIntent, []TaskRequest) {
	s := c.session

	// Install the new transport.
	if ev.Clients == nil {
		if s.Clients == nil && s.PreSuppliedClients != nil {
			s.PreSuppliedClients.SetIAMPolicies(s.IAMPolicies)
			s.PreSuppliedClients.SetIdentityStore(s.IdentityStore)
			s.PreSuppliedClients.SetRuleSets(s.RuleSets)
			s.Clients = s.PreSuppliedClients
		}
	} else if clients, ok := ev.Clients.(*awsclient.ServiceClients); ok {
		clients.SetIAMPolicies(s.IAMPolicies)
		clients.SetIdentityStore(s.IdentityStore)
		clients.SetRuleSets(s.RuleSets)
		s.Clients = clients
	} else {
		// Wrong concrete type — surface as APIErrorMsg via the adapter so
		// the existing classification flow handles it.
		wrongType := fmt.Errorf("internal: unexpected ClientsReadyMsg.Clients type %T", ev.Clients)
		return nil, []TaskRequest{{
			Key:     TaskKey{Kind: TaskKindEmitAPIError},
			Payload: EmitAPIErrorPayload{Err: wrongType},
		}}
	}

	s.HasPrevState = false
	s.PrevProfile = ""
	s.PrevRegion = ""

	var tasks []TaskRequest

	// One-shot -c navigation: only fire when we have an unconsumed Command
	// AND the active view is still the main menu (Stack depth 1).
	if s.Command != "" {
		if ev.StackDepth == 1 {
			tasks = append(tasks, TaskRequest{
				Key: TaskKey{Kind: TaskKindEmitNavigate},
				Payload: EmitNavigatePayload{
					Target:       NavigateTargetResourceList,
					ResourceType: s.Command,
				},
			})
		}
		s.Command = ""
	}

	if s.Profile == "" {
		s.Profile = "default"
	}
	if s.Region == "" {
		if ev.Region != "" {
			s.Region = ev.Region
		} else {
			configPath := awsclient.DefaultConfigPath()
			s.Region = awsclient.GetDefaultRegion(configPath, s.Profile)
		}
	}

	// Demo / no-cache: synchronous prefetch instead of the async probe
	// pipeline. Identity fetch is skipped in this mode (synthetic creds).
	if s.NoCache {
		tasks = append(tasks, TaskRequest{
			Key:     TaskKey{Kind: TaskKindDemoPrefetchCounts},
			Payload: DemoPrefetchCountsPayload{},
		})
		intents, refreshTasks := c.maybeRefreshIntents(ev)
		tasks = append(tasks, refreshTasks...)
		return intents, tasks
	}

	// Live AWS path: fetch identity + load disk cache.
	s.IdentityFetching = true
	tasks = append(tasks, TaskRequest{
		Key:     TaskKey{Kind: TaskKindFetchIdentity},
		Payload: FetchIdentityPayload{},
	})
	tasks = append(tasks, TaskRequest{
		Key:     TaskKey{Kind: TaskKindLoadAvailCache},
		Payload: LoadAvailCachePayload{},
	})

	intents, refreshTasks := c.maybeRefreshIntents(ev)
	tasks = append(tasks, refreshTasks...)
	return intents, tasks
}

// maybeRefreshIntents returns the refresh intents + flash when a pending
// post-switch refresh should fire (PendingRefresh is set AND an active
// resource list exists). Clears PendingRefresh so it does not re-fire on
// the next ClientsReadyMsg.
func (c *Core) maybeRefreshIntents(ev ClientsReadyEvent) ([]UIIntent, []TaskRequest) {
	if !c.session.PendingRefresh {
		return nil, nil
	}
	c.session.PendingRefresh = false
	if !ev.HasActiveRL {
		return nil, nil
	}
	return []UIIntent{
		FlashIntent{Text: "Connected. Refreshing..."},
		RefreshActiveListIntent{},
	}, nil
}

// HandleProfileSelected captures the pre-switch profile/region as the
// rollback target (only on first switch — rapid A→B→C keeps A), rotates
// the session, sets the new profile, clears region so the new profile's
// default region resolves, asks the adapter to clear main-menu
// availability and pop the selector, and schedules the new connect.
func (c *Core) HandleProfileSelected(ev ProfileSelectedEvent) ([]UIIntent, []TaskRequest) {
	s := c.session
	hadPrev := s.HasPrevState
	prevProf := s.PrevProfile
	prevReg := s.PrevRegion
	if !hadPrev {
		hadPrev = true
		prevProf = s.Profile
		prevReg = s.Region
	}
	s.Rotate()
	s.HasPrevState = hadPrev
	s.PrevProfile = prevProf
	s.PrevRegion = prevReg
	s.Profile = ev.Profile
	s.Region = ""
	s.PendingRefresh = true

	intents := []UIIntent{
		MenuClearAvailabilityIntent{},
		PopSelectorIntent{},
		FlashIntent{Text: "Switching to " + ev.Profile + "..."},
	}
	tasks := []TaskRequest{
		{
			Key:     TaskKey{Kind: TaskKindConnect},
			Payload: ConnectPayload{Profile: ev.Profile, Region: "", Gen: s.ConnectGen},
		},
		{
			Key:     TaskKey{Kind: TaskKindFlashTick},
			Payload: FlashTickPayload{Gen: ev.NewGen, Duration: flashDuration},
		},
	}
	return intents, tasks
}

// ProfilesLoadedEvent / ValueRevealedEvent / EnterChildViewEvent /
// ThemeSelectedEvent / ThemeFileReadEvent — adapter-translated forms of
// the corresponding messages.* (and TUI-private profilesLoadedMsg) for
// the view-stack handlers.

// ProfilesLoadedEvent carries the list of AWS profiles the adapter loaded
// from disk. HandleProfilesLoaded emits a PushScreen{ScreenProfileSelector}
// whose payload pairs the list with the session's current profile.
type ProfilesLoadedEvent struct {
	Profiles []string
}

// ValueRevealedEvent carries the result of a reveal fetch (`x` key over a
// secret-bearing resource). On Err the handler emits a flash error; on
// success it emits a PushScreen{ScreenReveal} carrying the value.
type ValueRevealedEvent struct {
	ResourceID string
	Value      string
	Err        error
}

// EnterChildViewEvent carries the child-type short name, the parent
// context map used by the fetcher, and the display name rendered as the
// child view's frame title. Unknown ChildType yields a flash error.
type EnterChildViewEvent struct {
	ChildType     string
	ParentContext map[string]string
	DisplayName   string
}

// ThemeSelectedEvent carries the theme filename the user confirmed.
// HandleThemeSelected resolves the path via config.ThemePath; the disk
// read itself runs in a TaskKindReadThemeFile dispatch whose result the
// adapter routes back as messages.ThemeFileRead → HandleThemeFileRead.
type ThemeSelectedEvent struct {
	Theme string
}

// ThemeFileReadEvent carries the bytes read from disk in response to
// the read task, together with any I/O error. HandleThemeFileRead
// branches on Err: read failure emits a flash error; read success emits
// ApplyThemeIntent (carries Bytes + Name; the adapter re-parses
// via styles.ThemeFromYAML before applying), PopSelectorIntent, a
// success FlashIntent ("Theme: <name>"), and a TaskKindSaveThemeConfig
// task.
type ThemeFileReadEvent struct {
	Theme string
	Bytes []byte
	Err   error
	// ParseErr is the renderer-side theme-validation result, computed by the
	// adapter (which owns the styles package) before handing the event to the
	// runtime. Keeping the parse in the adapter is what lets internal/runtime
	// stay renderer-agnostic (SC-009): the runtime branches on a domain-safe
	// error instead of importing internal/tui/styles. Non-nil ⇒ malformed YAML.
	ParseErr error
}

// HandleProfilesLoaded emits a PushScreen{ScreenProfileSelector} carrying
// the profile list paired with the session's currently-active profile so
// the selector can render the "(current)" indicator. No tasks fire — the
// adapter's builder closure constructs the SelectorModel synchronously.
func (c *Core) HandleProfilesLoaded(ev ProfilesLoadedEvent) ([]UIIntent, []TaskRequest) {
	intents := []UIIntent{PushScreen{
		ID: ScreenProfileSelector,
		Payload: ProfileSelectorPayload{
			Profiles: ev.Profiles,
			Current:  c.session.Profile,
		},
	}}
	return intents, nil
}

// HandleValueRevealed branches on Err. On error the handler emits a
// "reveal failed: <err>" flash so the user sees why the reveal aborted;
// on success it emits a PushScreen{ScreenReveal} whose payload carries
// the resource ID and decrypted value the adapter renders.
func (c *Core) HandleValueRevealed(ev ValueRevealedEvent) ([]UIIntent, []TaskRequest) {
	if ev.Err != nil {
		return []UIIntent{FlashIntent{
			Text:    "reveal failed: " + ev.Err.Error(),
			IsError: true,
		}}, nil
	}
	intents := []UIIntent{PushScreen{
		ID: ScreenReveal,
		Payload: RevealPayload{
			ResourceID: ev.ResourceID,
			Value:      ev.Value,
		},
	}}
	return intents, nil
}

// HandleEnterChildView validates ChildType via resource.GetChildType and
// either flashes an "unknown child type" error or emits a
// PushScreen{ScreenChildList} paired with a TaskKindFetchChildResources
// task. The adapter's screen builder constructs the ChildResourceList view
// and runs its Init() command; the fetch task kicks off paginated loading.
func (c *Core) HandleEnterChildView(ev EnterChildViewEvent) ([]UIIntent, []TaskRequest) {
	if resource.GetChildType(ev.ChildType) == nil {
		return []UIIntent{FlashIntent{
			Text:    fmt.Sprintf("unknown child type: %s", ev.ChildType),
			IsError: true,
		}}, nil
	}
	intents := []UIIntent{PushScreen{
		ID:      ScreenChildList,
		Payload: ChildListPayload(ev),
	}}
	tasks := []TaskRequest{{
		Key: TaskKey{Kind: TaskKindFetchChildResources, Scope: ev.ChildType},
		Payload: FetchChildResourcesPayload{
			ChildType:     ev.ChildType,
			ParentContext: ev.ParentContext,
		},
	}}
	return intents, tasks
}

// HandleThemeSelected validates the theme path via config.ThemePath (a
// non-renderer-coupled package) and either flashes an "Invalid theme:
// <err>" error or emits a TaskKindReadThemeFile task. The adapter's task
// closure performs the os.ReadFile and dispatches messages.ThemeFileRead
// → HandleThemeFileRead, which owns the apply/pop/flash/save sequence.
func (c *Core) HandleThemeSelected(ev ThemeSelectedEvent) ([]UIIntent, []TaskRequest) {
	if _, err := config.ThemePath(ev.Theme); err != nil {
		return []UIIntent{FlashIntent{
			Text:    "Invalid theme: " + err.Error(),
			IsError: true,
		}}, nil
	}
	tasks := []TaskRequest{{
		Key:     TaskKey{Kind: TaskKindReadThemeFile, Scope: ev.Theme},
		Payload: ReadThemePayload(ev),
	}}
	return nil, tasks
}

// HandleThemeFileRead is the second half of the theme-selected flow and
// branches on three outcomes:
//
//  1. Read failure → single "Cannot read theme: <err>" error flash, no apply,
//     no pop, no save.
//  2. Read OK, parse failure → single "Bad theme YAML: <err>" error flash,
//     no apply, no pop, no save. The selector stays open so the user can
//     retry. Validation is performed by the adapter (which owns the styles
//     package) and surfaced to the runtime as ev.ParseErr, so internal/runtime
//     never imports a renderer package (SC-009).
//  3. Read OK, parse OK → four results in order: ApplyThemeIntent (carries
//     the YAML Bytes; the adapter re-parses via styles.ThemeFromYAML),
//     PopSelectorIntent, success FlashIntent ("Theme: <name>"), and a
//     TaskKindSaveThemeConfig task that persists the choice to config.yaml.
//
// The parse-then-emit ordering matters: emitting Save unconditionally on
// read success would persist a malformed-YAML theme to disk even though the
// adapter rejected the apply, so Save is gated on a successful parse.
//
// ApplyThemeIntent's payload carries raw bytes (the adapter does the second
// parse for the renderer state); the save-fail UX delta is that the theme
// stays applied for the session even if the config save fails.
func (c *Core) HandleThemeFileRead(ev ThemeFileReadEvent) ([]UIIntent, []TaskRequest) {
	if ev.Err != nil {
		return []UIIntent{FlashIntent{
			Text:    "Cannot read theme: " + ev.Err.Error(),
			IsError: true,
		}}, nil
	}
	if ev.ParseErr != nil {
		return []UIIntent{FlashIntent{
			Text:    "Bad theme YAML: " + ev.ParseErr.Error(),
			IsError: true,
		}}, nil
	}
	intents := []UIIntent{
		ApplyThemeIntent{Bytes: ev.Bytes, Name: ev.Theme},
		PopSelectorIntent{},
		FlashIntent{Text: "Theme: " + ev.Theme},
	}
	tasks := []TaskRequest{{
		Key:     TaskKey{Kind: TaskKindSaveThemeConfig, Scope: ev.Theme},
		Payload: SaveThemeConfigPayload{Theme: ev.Theme},
	}}
	return intents, tasks
}

// HandleRegionSelected mirrors HandleProfileSelected for the region switch.
// Profile is preserved on the new connect; only the region changes.
func (c *Core) HandleRegionSelected(ev RegionSelectedEvent) ([]UIIntent, []TaskRequest) {
	s := c.session
	hadPrev := s.HasPrevState
	prevProf := s.PrevProfile
	prevReg := s.PrevRegion
	if !hadPrev {
		hadPrev = true
		prevProf = s.Profile
		prevReg = s.Region
	}
	s.Rotate()
	s.HasPrevState = hadPrev
	s.PrevProfile = prevProf
	s.PrevRegion = prevReg
	s.Region = ev.Region
	s.PendingRefresh = true

	intents := []UIIntent{
		MenuClearAvailabilityIntent{},
		PopSelectorIntent{},
		FlashIntent{Text: "Switching to " + ev.Region + "..."},
	}
	tasks := []TaskRequest{
		{
			Key:     TaskKey{Kind: TaskKindConnect},
			Payload: ConnectPayload{Profile: s.Profile, Region: ev.Region, Gen: s.ConnectGen},
		},
		{
			Key:     TaskKey{Kind: TaskKindFlashTick},
			Payload: FlashTickPayload{Gen: ev.NewGen, Duration: flashDuration},
		},
	}
	return intents, tasks
}

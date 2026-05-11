// handlers.go — shell-level handler bodies ported from internal/tui in
// Phase-05 PR-05a-h3 (AS-315b / AS-324).
//
// Each ported handler is a (c *Core) Handle* method that consumes a typed
// runtime event, applies session-scoped state changes, and returns
// ([]UIIntent, []TaskRequest) so the platform-agnostic core stays free of
// Bubble Tea / Lipgloss / Bubbles types. The TUI adapter wraps each handler
// in a thin (≤12-line) (tea.Model, tea.Cmd) shim that translates the
// adapter's messages.* into the runtime event, calls the Core method, and
// translates the returned intents and tasks back into adapter-visible side
// effects.
//
// What lives here (this PR):
//
//	HandleFlash          — flash gen bump + history; schedules ClearFlash tick.
//	HandleClearFlash     — flash auto-clear honouring the session-owned gen.
//	HandleAPIError       — AWS error classification + flash with [code] message.
//	HandleClientsReady   — clients/identity wiring + post-connect refresh / boot.
//	HandleProfileSelected— rotate, rollback latch, request reconnect.
//	HandleRegionSelected — mirror of HandleProfileSelected for region.
//
// What stays in the TUI adapter (out of scope):
//
//   - handleKeyMsg (keyboard dispatch) — owns key.Matches semantics on
//     adapter-owned tea types.
//   - handleProfilesLoaded / handleValueRevealed / handleEnterChildView /
//     handleThemeSelected — push concrete views onto the adapter view stack,
//     which requires the screen-builder registry that lands in a successor PR.
package runtime

import (
	"fmt"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
)

// Flash auto-clear durations. apiErrorFlashDuration is the longer 5 s window
// for AWS errors; flashDuration is the default 2 s window for status flashes.
const (
	flashDuration         = 2 * time.Second
	apiErrorFlashDuration = 5 * time.Second
)

// FlashEvent is the adapter-translated form of messages.FlashMsg used by
// HandleFlash. NewGen is the flash generation the adapter has already
// bumped to on its flashState before calling into the Core; the handler
// echoes it back via FlashTickPayload.Gen so the scheduled ClearFlashMsg
// references the same gen the active flash carries.
type FlashEvent struct {
	Text    string
	IsError bool
	NewGen  domain.Gen
}

// ClearFlashEvent is the adapter-translated form of messages.ClearFlashMsg.
// Gen is the gen the original tea.Tick carried; CurrentGen is the gen the
// adapter has now. The handler treats them as stale when they disagree.
// IsError lets the handler emit SetErrorHintIntent when the cleared flash
// was an error flash (mirrors the adapter's flashState.isError check).
type ClearFlashEvent struct {
	Gen        domain.Gen
	CurrentGen domain.Gen
	IsError    bool
}

// APIErrorEvent is the adapter-translated form of messages.APIErrorMsg.
// NewGen is the flash generation the adapter has already bumped to before
// calling into the Core; the handler echoes it back via the FlashTickPayload.
type APIErrorEvent struct {
	Err    error
	NewGen domain.Gen
}

// ClientsReadyEvent mirrors the fields of messages.ClientsReadyMsg the
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

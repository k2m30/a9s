package runtime

import (
	"errors"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// errWrongClientsType is returned by HandleClientsReady when ev.Clients is
// non-nil but is not *awsclient.ServiceClients — an impossible branch in
// production that is guarded here so the adapter's APIError path classifies it.
var errWrongClientsType = errors.New("internal: ClientsReadyEvent.Clients has unexpected concrete type")

// FlashEvent asks the core to display a transient flash message.
type FlashEvent struct {
	Text    string
	IsError bool
	NewGen  int
}

// ClearFlashEvent asks the core to clear the current flash if the gen matches.
type ClearFlashEvent struct {
	Gen        int
	CurrentGen int
	IsError    bool
}

// APIErrorEvent carries an AWS API error to be displayed and logged.
type APIErrorEvent struct {
	Err    error
	NewGen int
}

// ClientsReadyEvent is dispatched by the adapter after an AWS connect attempt
// completes (success or failure).
type ClientsReadyEvent struct {
	Gen         int
	NewGen      int
	Err         error
	Clients     any // *awsclient.ServiceClients or nil
	Region      string
	HasActiveRL bool
	StackDepth  int
}

// ProfileSelectedEvent is dispatched when the user confirms a profile switch.
type ProfileSelectedEvent struct {
	Profile string
	NewGen  int
}

// RegionSelectedEvent is dispatched when the user confirms a region switch.
type RegionSelectedEvent struct {
	Region string
	NewGen int
}

// HandleFlash processes a FlashEvent and returns the resulting intents and tasks.
func (c *Core) HandleFlash(ev FlashEvent) ([]UIIntent, []TaskRequest) {
	intents := []UIIntent{FlashIntent{Text: ev.Text, IsError: ev.IsError}}
	if ev.IsError {
		intents = append(intents, AppendErrorHistoryIntent{Time: time.Now(), Message: ev.Text})
	}
	tasks := []TaskRequest{
		{
			Key:     TaskKey{Kind: TaskKindFlashTick},
			Payload: FlashTickPayload{Gen: ev.NewGen, Duration: 2 * time.Second},
		},
	}
	return intents, tasks
}

// HandleClearFlash processes a ClearFlashEvent. Stale gens are discarded.
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

// HandleAPIError processes an APIErrorEvent, always emitting three intents and
// one 5-second flash tick task.
func (c *Core) HandleAPIError(ev APIErrorEvent) ([]UIIntent, []TaskRequest) {
	intents := []UIIntent{
		FlashIntent{Text: ev.Err.Error(), IsError: true},
		AppendErrorHistoryIntent{Time: time.Now(), Message: ev.Err.Error()},
		ClearActiveListLoadingIntent{},
	}
	tasks := []TaskRequest{
		{
			Key:     TaskKey{Kind: TaskKindFlashTick},
			Payload: FlashTickPayload{Gen: ev.NewGen, Duration: 5 * time.Second},
		},
	}
	return intents, tasks
}

// HandleClientsReady processes the result of an AWS connect attempt. Stale gens
// (ev.Gen != session.ConnectGen) are discarded.
func (c *Core) HandleClientsReady(ev ClientsReadyEvent) ([]UIIntent, []TaskRequest) {
	if ev.Gen != c.session.ConnectGen {
		return nil, nil
	}

	if ev.Err != nil {
		// Failure path — roll back to previous profile/region if available.
		if c.session.HasPrevState {
			c.session.Profile = c.session.PrevProfile
			c.session.Region = c.session.PrevRegion
		}
		c.session.HasPrevState = false
		c.session.PrevProfile = ""
		c.session.PrevRegion = ""
		c.session.PendingRefresh = false
		c.session.ConnectGen = ev.NewGen

		intents := []UIIntent{
			FlashIntent{Text: ev.Err.Error(), IsError: true},
			AppendErrorHistoryIntent{Time: time.Now(), Message: ev.Err.Error()},
		}
		tasks := []TaskRequest{
			{
				Key:     TaskKey{Kind: TaskKindFlashTick},
				Payload: FlashTickPayload{Gen: ev.NewGen, Duration: 5 * time.Second},
			},
		}

		// If we still have existing clients, re-bootstrap identity + cache.
		if c.session.Clients != nil {
			tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindFetchIdentity}, Payload: FetchIdentityPayload{}})
			if c.session.NoCache {
				tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindDemoPrefetchCounts}, Payload: DemoPrefetchCountsPayload{}})
			} else {
				tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindLoadAvailCache}, Payload: LoadAvailCachePayload{}})
			}
		}

		return intents, tasks
	}

	// Success path — install clients.
	if ev.Clients == nil && c.session.Clients == nil && c.session.PreSuppliedClients != nil {
		c.session.Clients = c.session.PreSuppliedClients
	} else if sc, ok := ev.Clients.(*awsclient.ServiceClients); ok {
		c.session.Clients = sc
	} else if ev.Clients != nil {
		// Wrong concrete type — route through APIError task.
		c.session.HasPrevState = false
		c.session.PrevProfile = ""
		c.session.PrevRegion = ""
		c.session.ConnectGen = ev.NewGen
		typeErr := errWrongClientsType
		return []UIIntent{
				FlashIntent{Text: typeErr.Error(), IsError: true},
				AppendErrorHistoryIntent{Time: time.Now(), Message: typeErr.Error()},
			}, []TaskRequest{
				{Key: TaskKey{Kind: TaskKindEmitAPIError}, Payload: EmitAPIErrorPayload{Err: typeErr}},
			}
	}

	c.session.HasPrevState = false
	c.session.PrevProfile = ""
	c.session.PrevRegion = ""
	c.session.ConnectGen = ev.NewGen

	var intents []UIIntent
	var tasks []TaskRequest

	// Handle -c CLI flag navigation.
	if c.session.Command != "" {
		if ev.StackDepth == 1 {
			tasks = append(tasks, TaskRequest{
				Key: TaskKey{Kind: TaskKindEmitNavigate},
				Payload: EmitNavigatePayload{
					Target:       messages.TargetResourceList,
					ResourceType: c.session.Command,
				},
			})
		}
		c.session.Command = ""
	}

	if c.session.NoCache {
		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindDemoPrefetchCounts}, Payload: DemoPrefetchCountsPayload{}})
		if c.session.PendingRefresh {
			if ev.HasActiveRL {
				intents = append(intents, RefreshActiveListIntent{}, FlashIntent{Text: "Connected. Refreshing..."})
			}
			c.session.PendingRefresh = false
		}
		return intents, tasks
	}

	// Live AWS path.
	tasks = append(tasks,
		TaskRequest{Key: TaskKey{Kind: TaskKindFetchIdentity}, Payload: FetchIdentityPayload{}},
		TaskRequest{Key: TaskKey{Kind: TaskKindLoadAvailCache}, Payload: LoadAvailCachePayload{}},
	)
	if c.session.PendingRefresh {
		if ev.HasActiveRL {
			intents = append(intents, RefreshActiveListIntent{}, FlashIntent{Text: "Connected. Refreshing..."})
		}
		c.session.PendingRefresh = false
	}
	return intents, tasks
}

// HandleProfileSelected processes a profile selector confirmation. Captures
// rollback state, rotates the session, and dispatches a connect task.
func (c *Core) HandleProfileSelected(ev ProfileSelectedEvent) ([]UIIntent, []TaskRequest) {
	hadPrev := c.session.HasPrevState
	prevProf := c.session.PrevProfile
	prevReg := c.session.PrevRegion
	if !hadPrev {
		prevProf = c.session.Profile
		prevReg = c.session.Region
	}

	c.session.Rotate()

	c.session.HasPrevState = true
	c.session.PrevProfile = prevProf
	c.session.PrevRegion = prevReg
	c.session.Profile = ev.Profile
	c.session.Region = ""
	c.session.PendingRefresh = true

	intents := []UIIntent{
		MenuClearAvailabilityIntent{},
		PopSelectorIntent{},
		FlashIntent{Text: "Switching to " + ev.Profile + "..."},
	}
	tasks := []TaskRequest{
		{
			Key:     TaskKey{Kind: TaskKindConnect},
			Payload: ConnectPayload{Profile: ev.Profile, Region: "", Gen: c.session.ConnectGen},
		},
		{
			Key:     TaskKey{Kind: TaskKindFlashTick},
			Payload: FlashTickPayload{Gen: ev.NewGen, Duration: 2 * time.Second},
		},
	}
	return intents, tasks
}

// HandleRegionSelected processes a region selector confirmation. Mirrors
// HandleProfileSelected but preserves the current profile.
func (c *Core) HandleRegionSelected(ev RegionSelectedEvent) ([]UIIntent, []TaskRequest) {
	hadPrev := c.session.HasPrevState
	prevProf := c.session.PrevProfile
	prevReg := c.session.PrevRegion
	if !hadPrev {
		prevProf = c.session.Profile
		prevReg = c.session.Region
	}

	capturedProfile := c.session.Profile

	c.session.Rotate()

	c.session.HasPrevState = true
	c.session.PrevProfile = prevProf
	c.session.PrevRegion = prevReg
	c.session.Region = ev.Region
	c.session.PendingRefresh = true

	intents := []UIIntent{
		MenuClearAvailabilityIntent{},
		PopSelectorIntent{},
		FlashIntent{Text: "Switching to " + ev.Region + "..."},
	}
	tasks := []TaskRequest{
		{
			Key:     TaskKey{Kind: TaskKindConnect},
			Payload: ConnectPayload{Profile: capturedProfile, Region: ev.Region, Gen: c.session.ConnectGen},
		},
		{
			Key:     TaskKey{Kind: TaskKindFlashTick},
			Payload: FlashTickPayload{Gen: ev.NewGen, Duration: 2 * time.Second},
		},
	}
	return intents, tasks
}

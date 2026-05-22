// fetch_adapter.go — thin Bubble Tea adapter over runtime.Core fetch methods.
//
// PR-05a-h6 moves fetch-execution logic to internal/runtime/fetchers.go.
// This file bridges the runtime Core methods to the tea.Cmd factories that
// the TUI Update loop and handler files expect. Each method captures ctx and
// clients, delegates to the corresponding Core method, and converts the result
// to TUI message types.
//
// AS-657: every fetch that produces ResourcesLoaded / APIError / IdentityLoaded /
// IdentityError / ValueRevealed now captures the dispatch-time generation counter
// (gen domain.Gen) at the call site. The gen is stamped onto the returned message
// so the app.go handler can discard stale results after a profile/region switch
// (messages.IsStale guard). Callers pass m.core.AvailabilityGen() (for
// resource fetches) or m.core.ConnectGen() (for identity/reveal fetches).
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// profilesLoadedMsg is a TUI-private message carrying AWS profile names
// returned by fetchProfiles.
type profilesLoadedMsg struct {
	profiles []string
}

// fetchResources returns a tea.Cmd that calls Core.FetchResources and
// converts the result to ResourcesLoaded or APIError.
// gen is the AvailabilityGen captured at dispatch time; it is stamped onto the
// returned message so the handler can discard stale results after a switch.
func (m *Model) fetchResources(resourceType string, gen domain.Gen) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	return func() tea.Msg {
		res, err := m.core.FetchResources(ctx, clients, resourceType)
		// Partial-success contract: fetchers may return BOTH a non-empty
		// result.Resources AND a composite error. When that happens we
		// surface the error AND keep the partial Resources; hard failures
		// (no resources at all) route through APIError.
		if err != nil && len(res.Resources) == 0 {
			return messages.APIError{ResourceType: resourceType, Err: err, Gen: gen}
		}
		return messages.ResourcesLoaded{
			ResourceType: resourceType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Err:          err,
			Gen:          gen,
		}
	}
}

// fetchResourcesFiltered returns a tea.Cmd for a server-side filtered fetch.
// gen is the AvailabilityGen captured at dispatch time.
func (m *Model) fetchResourcesFiltered(resourceType string, filter map[string]string, gen domain.Gen) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	return func() tea.Msg {
		res, err := m.core.FetchResourcesFiltered(ctx, clients, resourceType, filter)
		if err != nil && len(res.Resources) == 0 {
			return messages.APIError{ResourceType: resourceType, Err: err, Gen: gen}
		}
		return messages.ResourcesLoaded{
			ResourceType: resourceType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Err:          err,
			Gen:          gen,
		}
	}
}

// fetchAMIDetail returns a tea.Cmd that fetches a single AMI by ID and
// navigates to its detail view.
func (m *Model) fetchAMIDetail(imageID string) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	return func() tea.Msg {
		res, err := m.core.FetchAMIDetail(ctx, clients, imageID)
		if err != nil {
			return messages.Flash{Text: err.Error(), IsError: true}
		}
		return messages.Navigate{
			Target:       messages.TargetDetail,
			ResourceType: "ami",
			Resource:     &res,
		}
	}
}

// fetchChildResources returns a tea.Cmd for paginated child resource loading.
// Child resource fetches use AvailabilityGen so stale results from prior
// profile/region are discarded along with top-level fetches.
func (m *Model) fetchChildResources(childType string, parentCtx map[string]string) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	gen := m.core.AvailabilityGen()
	return func() tea.Msg {
		res, err := m.core.FetchChildResources(ctx, clients, childType, parentCtx)
		if err != nil {
			return messages.APIError{ResourceType: childType, Err: err, Gen: gen}
		}
		return messages.ResourcesLoaded{
			ResourceType: childType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Gen:          gen,
		}
	}
}

// fetchMoreResources returns a tea.Cmd that fetches the next page of a
// paginated resource list using the continuation token from LoadMoreMsg.
// gen is the AvailabilityGen captured at dispatch time.
func (m *Model) fetchMoreResources(msg messages.LoadMore) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	gen := m.core.AvailabilityGen()
	p := runtime.FetchMoreParams{
		ResourceType: msg.ResourceType,
		Token:        msg.ContinuationToken,
		ParentCtx:    msg.ParentContext,
		FetchFilter:  msg.FetchFilter,
	}
	return func() tea.Msg {
		res, err := m.core.FetchMoreResources(ctx, clients, p)
		if err != nil && len(res.Resources) == 0 {
			return messages.APIError{ResourceType: msg.ResourceType, Err: err, Gen: gen}
		}
		return messages.ResourcesLoaded{
			ResourceType: msg.ResourceType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Append:       true,
			Err:          err,
			Gen:          gen,
		}
	}
}

// fetchIdentity returns a tea.Cmd that fetches the AWS caller identity.
// gen is the ConnectGen captured at dispatch time; it is stamped onto the
// returned message so the handler can discard stale results after a switch.
func (m *Model) fetchIdentity(gen domain.Gen) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	return func() tea.Msg {
		identity, err := m.core.FetchIdentity(ctx, clients)
		if err != nil {
			return messages.IdentityError{Err: err.Error(), Gen: gen}
		}
		return messages.IdentityLoaded{Identity: identity, Gen: gen}
	}
}

// fetchProfiles returns a tea.Cmd that reads the local AWS config profiles.
func (m *Model) fetchProfiles() tea.Cmd {
	return func() tea.Msg {
		profiles, err := m.core.FetchProfiles()
		if err != nil {
			return messages.Flash{Text: err.Error(), IsError: true}
		}
		return profilesLoadedMsg{profiles: profiles}
	}
}

// fetchRevealValue returns a tea.Cmd that calls the registered reveal fetcher.
// gen is the ConnectGen captured at dispatch time; it is stamped onto the
// returned message so the handler can discard stale results after a switch.
func (m *Model) fetchRevealValue(resourceType, resourceID string, gen domain.Gen) tea.Cmd {
	ctx, clients := m.appCtx, m.core.Clients()
	return func() tea.Msg {
		value, err := m.core.FetchRevealValue(ctx, clients, resourceType, resourceID)
		if err != nil {
			return messages.ValueRevealed{ResourceType: resourceType, ResourceID: resourceID, Err: err, Gen: gen}
		}
		return messages.ValueRevealed{ResourceType: resourceType, ResourceID: resourceID, Value: value, Gen: gen}
	}
}

// connectAWS returns a tea.Cmd that establishes an AWS session and emits a
// ClientsReadyMsg. gen is a monotonic counter incremented on each
// profile/region switch; stale ClientsReadyMsg values carrying an old gen are
// discarded by handleClientsReady.
func (m *Model) connectAWS(profile, region string, gen domain.Gen) tea.Cmd {
	ctx := m.appCtx
	return func() tea.Msg {
		result, err := m.core.ConnectAWS(ctx, profile, region)
		return messages.ClientsReady{
			Clients: result.Clients,
			Region:  result.Region,
			Gen:     gen,
			Err:     err,
		}
	}
}

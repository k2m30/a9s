// fetch_adapter.go — thin Bubble Tea adapter over runtime.Core fetch methods.
//
// PR-05a-h6 moves fetch-execution logic to internal/runtime/fetchers.go.
// This file bridges the runtime Core methods to the tea.Cmd factories that
// the TUI Update loop and handler files expect. Each method captures ctx and
// clients, delegates to the corresponding Core method, and converts the result
// to TUI message types.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// profilesLoadedMsg is a TUI-private message carrying AWS profile names
// returned by fetchProfiles.
type profilesLoadedMsg struct {
	profiles []string
}

// fetchResources returns a tea.Cmd that calls Core.FetchResources and
// converts the result to ResourcesLoadedMsg or APIErrorMsg.
func (m *Model) fetchResources(resourceType string) tea.Cmd {
	ctx, clients := m.appCtx, m.Clients
	return func() tea.Msg {
		res, err := m.core.FetchResources(ctx, clients, resourceType)
		// Partial-success contract: fetchers may return BOTH a non-empty
		// result.Resources AND a composite error. When that happens we
		// surface the error AND keep the partial Resources; hard failures
		// (no resources at all) route through APIErrorMsg.
		if err != nil && len(res.Resources) == 0 {
			return messages.APIErrorMsg{ResourceType: resourceType, Err: err}
		}
		return messages.ResourcesLoadedMsg{
			ResourceType: resourceType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Err:          err,
		}
	}
}

// fetchResourcesFiltered returns a tea.Cmd for a server-side filtered fetch.
func (m *Model) fetchResourcesFiltered(resourceType string, filter map[string]string) tea.Cmd {
	ctx, clients := m.appCtx, m.Clients
	return func() tea.Msg {
		res, err := m.core.FetchResourcesFiltered(ctx, clients, resourceType, filter)
		if err != nil && len(res.Resources) == 0 {
			return messages.APIErrorMsg{ResourceType: resourceType, Err: err}
		}
		return messages.ResourcesLoadedMsg{
			ResourceType: resourceType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Err:          err,
		}
	}
}

// fetchAMIDetail returns a tea.Cmd that fetches a single AMI by ID and
// navigates to its detail view.
func (m *Model) fetchAMIDetail(imageID string) tea.Cmd {
	ctx, clients := m.appCtx, m.Clients
	return func() tea.Msg {
		res, err := m.core.FetchAMIDetail(ctx, clients, imageID)
		if err != nil {
			return messages.FlashMsg{Text: err.Error(), IsError: true}
		}
		return messages.NavigateMsg{
			Target:       messages.TargetDetail,
			ResourceType: "ami",
			Resource:     &res,
		}
	}
}

// fetchChildResources returns a tea.Cmd for paginated child resource loading.
func (m *Model) fetchChildResources(childType string, parentCtx map[string]string) tea.Cmd {
	ctx, clients := m.appCtx, m.Clients
	return func() tea.Msg {
		res, err := m.core.FetchChildResources(ctx, clients, childType, parentCtx)
		if err != nil {
			return messages.APIErrorMsg{ResourceType: childType, Err: err}
		}
		return messages.ResourcesLoadedMsg{
			ResourceType: childType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
		}
	}
}

// fetchMoreResources returns a tea.Cmd that fetches the next page of a
// paginated resource list using the continuation token from LoadMoreMsg.
func (m *Model) fetchMoreResources(msg messages.LoadMoreMsg) tea.Cmd {
	ctx, clients := m.appCtx, m.Clients
	p := runtime.FetchMoreParams{
		ResourceType: msg.ResourceType,
		Token:        msg.ContinuationToken,
		ParentCtx:    msg.ParentContext,
		FetchFilter:  msg.FetchFilter,
	}
	return func() tea.Msg {
		res, _, err := m.core.FetchMoreResources(ctx, clients, p)
		if err != nil && len(res.Resources) == 0 {
			return messages.APIErrorMsg{ResourceType: msg.ResourceType, Err: err}
		}
		return messages.ResourcesLoadedMsg{
			ResourceType: msg.ResourceType,
			Resources:    res.Resources,
			Pagination:   res.Pagination,
			Append:       true,
			Err:          err,
		}
	}
}

// fetchIdentity returns a tea.Cmd that fetches the AWS caller identity.
func (m *Model) fetchIdentity() tea.Cmd {
	ctx, clients := m.appCtx, m.Clients
	return func() tea.Msg {
		identity, err := m.core.FetchIdentity(ctx, clients)
		if err != nil {
			return messages.IdentityErrorMsg{Err: err.Error()}
		}
		return messages.IdentityLoadedMsg{Identity: identity}
	}
}

// fetchProfiles returns a tea.Cmd that reads the local AWS config profiles.
func (m *Model) fetchProfiles() tea.Cmd {
	return func() tea.Msg {
		profiles, err := m.core.FetchProfiles()
		if err != nil {
			return messages.FlashMsg{Text: err.Error(), IsError: true}
		}
		return profilesLoadedMsg{profiles: profiles}
	}
}

// fetchRevealValue returns a tea.Cmd that calls the registered reveal fetcher.
func (m *Model) fetchRevealValue(resourceType, resourceID string) tea.Cmd {
	ctx, clients := m.appCtx, m.Clients
	return func() tea.Msg {
		value, err := m.core.FetchRevealValue(ctx, clients, resourceType, resourceID)
		if err != nil {
			return messages.ValueRevealedMsg{ResourceType: resourceType, ResourceID: resourceID, Err: err}
		}
		return messages.ValueRevealedMsg{ResourceType: resourceType, ResourceID: resourceID, Value: value}
	}
}

// connectAWS returns a tea.Cmd that establishes an AWS session and emits a
// ClientsReadyMsg. gen is a monotonic counter incremented on each
// profile/region switch; stale ClientsReadyMsg values carrying an old gen are
// discarded by handleClientsReady.
func (m *Model) connectAWS(profile, region string, gen int) tea.Cmd {
	ctx := m.appCtx
	return func() tea.Msg {
		result, err := m.core.ConnectAWS(ctx, profile, region)
		return messages.ClientsReadyMsg{
			Clients: result.Clients,
			Region:  result.Region,
			Gen:     gen,
			Err:     err,
		}
	}
}

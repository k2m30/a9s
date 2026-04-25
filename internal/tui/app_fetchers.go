package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// fetchResources returns a tea.Cmd that calls the appropriate AWS fetcher
// using the resource registry. Child resource types (S3 objects, R53 records)
// are handled by fetchChildResources instead.
func (m *Model) fetchResources(resourceType string) tea.Cmd {
	clients := m.clients
	ctx := m.appCtx
	return func() tea.Msg {
		pf := resource.GetPaginatedFetcher(resourceType)
		if pf == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("unsupported resource type: %s", resourceType),
			}
		}
		result, err := pf(ctx, clients, "")
		// Partial-success contract: fetchers may return BOTH a non-empty
		// result.Resources AND a composite error (per E1-E6 — a per-item
		// failure aggregates instead of aborting). When that happens we
		// MUST surface the error AND keep the partial Resources, otherwise
		// a single inline-group-policy timeout drops the entire policies
		// list. Hard failures (no resources at all) still go through
		// APIErrorMsg.
		if err != nil && len(result.Resources) == 0 {
			return messages.APIErrorMsg{ResourceType: resourceType, Err: err}
		}
		// IsTruncated: populated from first-page result; loaders stop at page 1.
		return messages.ResourcesLoadedMsg{
			ResourceType: resourceType,
			Resources:    result.Resources,
			Pagination:   result.Pagination,
			Err:          err,
		}
	}
}

// fetchResourcesFiltered returns a tea.Cmd that calls the registered FilteredPaginatedFetcher
// for the given resource type with the provided filter parameters.
func (m *Model) fetchResourcesFiltered(resourceType string, filter map[string]string) tea.Cmd {
	clients := m.clients
	ctx := m.appCtx
	return func() tea.Msg {
		if clients == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("AWS clients not initialized"),
			}
		}
		pf := resource.GetFilteredPaginatedFetcher(resourceType)
		if pf == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("no filtered fetcher registered for: %s", resourceType),
			}
		}
		result, err := pf(ctx, clients, filter, "")
		// Partial-success contract — same as fetchResources above.
		if err != nil && len(result.Resources) == 0 {
			return messages.APIErrorMsg{ResourceType: resourceType, Err: err}
		}
		return messages.ResourcesLoadedMsg{
			ResourceType: resourceType,
			Resources:    result.Resources,
			Pagination:   result.Pagination,
			Err:          err,
		}
	}
}

func (m *Model) fetchAMIDetail(imageID string) tea.Cmd {
	clients := m.clients
	ctx := m.appCtx
	return func() tea.Msg {
		if clients == nil {
			return messages.FlashMsg{
				Text:    fmt.Sprintf("AWS clients not initialized; cannot load AMI %s", imageID),
				IsError: true,
			}
		}
		res, err := awsclient.FetchAMIByID(ctx, clients.EC2, imageID)
		if err != nil {
			return messages.FlashMsg{
				Text:    err.Error(),
				IsError: true,
			}
		}
		return messages.NavigateMsg{
			Target:       messages.TargetDetail,
			ResourceType: "ami",
			Resource:     &res,
		}
	}
}

// fetchChildResources returns a tea.Cmd that calls the paginated child fetcher
// for the given child type, passing an empty continuation token for the initial page.
func (m *Model) fetchChildResources(childType string, parentCtx map[string]string) tea.Cmd {
	clients := m.clients
	ctx := m.appCtx
	return func() tea.Msg {
		if clients == nil {
			return messages.APIErrorMsg{
				ResourceType: childType,
				Err:          fmt.Errorf("AWS clients not initialized"),
			}
		}

		pc := resource.ParentContext(parentCtx)

		pf := resource.GetPaginatedChildFetcher(childType)
		if pf == nil {
			return messages.APIErrorMsg{
				ResourceType: childType,
				Err:          fmt.Errorf("unsupported child type: %s", childType),
			}
		}
		result, err := pf(ctx, clients, pc, "")
		if err != nil {
			return messages.APIErrorMsg{ResourceType: childType, Err: err}
		}
		return messages.ResourcesLoadedMsg{
			ResourceType: childType,
			Resources:    result.Resources,
			Pagination:   result.Pagination,
		}
	}
}

// fetchMoreResources returns a tea.Cmd that fetches the next page of a paginated
// resource list using the continuation token from LoadMoreMsg.
func (m *Model) fetchMoreResources(msg messages.LoadMoreMsg) tea.Cmd {
	clients := m.clients
	ctx := m.appCtx
	rt := msg.ResourceType
	token := msg.ContinuationToken
	parentCtx := msg.ParentContext

	return func() tea.Msg {
		if clients == nil {
			return messages.APIErrorMsg{
				ResourceType: rt,
				Err:          fmt.Errorf("AWS clients not initialized"),
			}
		}

		// Partial-success contract: hard failure (no resources) → APIErrorMsg;
		// soft failure (some resources + composite err) → ResourcesLoadedMsg
		// with Err set so the handler routes it through FlashMsg without
		// dropping the partial page.

		// Filtered fetch path: used by related navigation with server-side filters (e.g., CloudTrail LookupAttributes).
		if len(msg.FetchFilter) > 0 {
			pf := resource.GetFilteredPaginatedFetcher(rt)
			if pf != nil {
				result, err := pf(ctx, clients, msg.FetchFilter, token)
				if err != nil && len(result.Resources) == 0 {
					return messages.APIErrorMsg{ResourceType: rt, Err: err}
				}
				return messages.ResourcesLoadedMsg{
					ResourceType: rt,
					Resources:    result.Resources,
					Pagination:   result.Pagination,
					Append:       true,
					Err:          err,
				}
			}
		}

		// Try paginated child fetcher first (for child views).
		if len(parentCtx) > 0 {
			pf := resource.GetPaginatedChildFetcher(rt)
			if pf != nil {
				pc := resource.ParentContext(parentCtx)
				result, err := pf(ctx, clients, pc, token)
				if err != nil && len(result.Resources) == 0 {
					return messages.APIErrorMsg{ResourceType: rt, Err: err}
				}
				return messages.ResourcesLoadedMsg{
					ResourceType: rt,
					Resources:    result.Resources,
					Pagination:   result.Pagination,
					Append:       true,
					Err:          err,
				}
			}
		}

		// Try paginated top-level fetcher.
		pf := resource.GetPaginatedFetcher(rt)
		if pf != nil {
			result, err := pf(ctx, clients, token)
			if err != nil && len(result.Resources) == 0 {
				return messages.APIErrorMsg{ResourceType: rt, Err: err}
			}
			return messages.ResourcesLoadedMsg{
				ResourceType: rt,
				Resources:    result.Resources,
				Pagination:   result.Pagination,
				Append:       true,
				Err:          err,
			}
		}

		return messages.APIErrorMsg{
			ResourceType: rt,
			Err:          fmt.Errorf("no paginated fetcher for: %s", rt),
		}
	}
}

func (m *Model) fetchIdentity() tea.Cmd {
	clients := m.clients
	ctx := m.appCtx
	return func() tea.Msg {
		if clients == nil || clients.STS == nil {
			return messages.IdentityErrorMsg{Err: "AWS clients not initialized"}
		}
		identity, err := awsclient.FetchCallerIdentity(ctx, clients.STS, clients.IAM)
		if err != nil {
			return messages.IdentityErrorMsg{Err: err.Error()}
		}
		return messages.IdentityLoadedMsg{Identity: identity}
	}
}

func (m *Model) fetchProfiles() tea.Cmd {
	return func() tea.Msg {
		configPath := awsclient.DefaultConfigPath()
		profiles, err := awsclient.ListProfiles(configPath)
		if err != nil {
			return messages.FlashMsg{Text: "failed to list profiles: " + err.Error(), IsError: true}
		}
		if len(profiles) == 0 {
			return messages.FlashMsg{Text: "no AWS profiles found", IsError: true}
		}
		return profilesLoadedMsg{profiles: profiles}
	}
}

type profilesLoadedMsg struct {
	profiles []string
}

func (m *Model) fetchRevealValue(resourceType, resourceID string) tea.Cmd {
	clients := m.clients
	ctx := m.appCtx
	return func() tea.Msg {
		if clients == nil {
			return messages.FlashMsg{Text: "AWS clients not initialized", IsError: true}
		}
		fetcher := resource.GetRevealFetcher(resourceType)
		if fetcher == nil {
			return messages.FlashMsg{Text: "no reveal support for " + resourceType, IsError: true}
		}
		value, err := fetcher(ctx, clients, resourceID)
		if err != nil {
			return messages.ValueRevealedMsg{ResourceType: resourceType, ResourceID: resourceID, Err: err}
		}
		return messages.ValueRevealedMsg{ResourceType: resourceType, ResourceID: resourceID, Value: value}
	}
}

func (m *Model) connectAWS(profile, region string, gen int) tea.Cmd {
	ctx := m.appCtx
	return func() tea.Msg {
		// First attempt: let SDK resolve region from env vars + config file.
		cfg, err := awsclient.NewAWSSessionContext(ctx, profile, region)
		if err != nil {
			// If SDK fails due to missing region and we didn't provide one,
			// fall back to config-file / us-east-1 (issue #82 safety net).
			if region == "" && isMissingRegionError(err) {
				configPath := awsclient.DefaultConfigPath()
				fallbackRegion := awsclient.GetDefaultRegion(configPath, profile)
				cfg, err = awsclient.NewAWSSessionContext(ctx, profile, fallbackRegion)
			}
			if err != nil {
				return messages.ClientsReadyMsg{Err: err, Gen: gen}
			}
		}
		// SDK may succeed but leave Region empty (no env var, no config file).
		// Retry with config-file / us-east-1 so API calls don't fail later.
		if cfg.Region == "" && region == "" {
			configPath := awsclient.DefaultConfigPath()
			fallbackRegion := awsclient.GetDefaultRegion(configPath, profile)
			cfg, err = awsclient.NewAWSSessionContext(ctx, profile, fallbackRegion)
			if err != nil {
				return messages.ClientsReadyMsg{Err: err, Gen: gen}
			}
		}
		clients := awsclient.CreateServiceClients(cfg)
		return messages.ClientsReadyMsg{Clients: clients, Region: cfg.Region, Gen: gen}
	}
}

// isMissingRegionError checks if an AWS config error is due to missing region.
func isMissingRegionError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "missing region") || strings.Contains(msg, "could not find region")
}


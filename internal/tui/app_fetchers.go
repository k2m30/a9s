package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// fetchResources returns a tea.Cmd that calls the appropriate AWS fetcher
// using the resource registry. Child resource types (S3 objects, R53 records)
// are handled by fetchChildResources instead.
func (m *Model) fetchResources(resourceType string) tea.Cmd {
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("AWS clients not initialized"),
			}
		}

		ctx := context.Background()
		fetcher := resource.GetFetcher(resourceType)
		if fetcher == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("unsupported resource type: %s", resourceType),
			}
		}
		resources, err := fetcher(ctx, clients)
		if err != nil {
			return messages.APIErrorMsg{ResourceType: resourceType, Err: err}
		}
		return messages.ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
	}
}

// fetchChildResources returns a tea.Cmd that calls the appropriate child fetcher.
// It checks the paginated child registry first (passing empty continuation token
// for the initial page), then falls back to the legacy child fetcher registry.
func (m *Model) fetchChildResources(childType string, parentCtx map[string]string) tea.Cmd {
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.APIErrorMsg{
				ResourceType: childType,
				Err:          fmt.Errorf("AWS clients not initialized"),
			}
		}

		ctx := context.Background()
		pc := resource.ParentContext(parentCtx)

		// Try paginated child fetcher first (initial page with empty token).
		pf := resource.GetPaginatedChildFetcher(childType)
		if pf != nil {
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

		// Fall back to legacy (non-paginated) child fetcher.
		fetcher := resource.GetChildFetcher(childType)
		if fetcher == nil {
			return messages.APIErrorMsg{
				ResourceType: childType,
				Err:          fmt.Errorf("unsupported child type: %s", childType),
			}
		}

		resources, err := fetcher(ctx, clients, pc)
		if err != nil {
			return messages.APIErrorMsg{ResourceType: childType, Err: err}
		}
		return messages.ResourcesLoadedMsg{ResourceType: childType, Resources: resources}
	}
}

// fetchMoreResources returns a tea.Cmd that fetches the next page of a paginated
// resource list using the continuation token from LoadMoreMsg.
func (m *Model) fetchMoreResources(msg messages.LoadMoreMsg) tea.Cmd {
	clients := m.clients
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
		ctx := context.Background()

		// Try paginated child fetcher first (for child views).
		if len(parentCtx) > 0 {
			pf := resource.GetPaginatedChildFetcher(rt)
			if pf != nil {
				pc := resource.ParentContext(parentCtx)
				result, err := pf(ctx, clients, pc, token)
				if err != nil {
					return messages.APIErrorMsg{ResourceType: rt, Err: err}
				}
				return messages.ResourcesLoadedMsg{
					ResourceType: rt,
					Resources:    result.Resources,
					Pagination:   result.Pagination,
					Append:       true,
				}
			}
		}

		// Try paginated top-level fetcher.
		pf := resource.GetPaginatedFetcher(rt)
		if pf != nil {
			result, err := pf(ctx, clients, token)
			if err != nil {
				return messages.APIErrorMsg{ResourceType: rt, Err: err}
			}
			return messages.ResourcesLoadedMsg{
				ResourceType: rt,
				Resources:    result.Resources,
				Pagination:   result.Pagination,
				Append:       true,
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
	return func() tea.Msg {
		if clients == nil {
			return messages.IdentityErrorMsg{Err: "AWS clients not initialized"}
		}
		identity, err := awsclient.FetchCallerIdentity(context.Background(), clients.STS, clients.IAM)
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

func (m *Model) fetchSecretValue(secretName string) tea.Cmd {
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.FlashMsg{Text: "AWS clients not initialized", IsError: true}
		}
		ctx := context.Background()
		value, err := awsclient.RevealSecret(ctx, clients.SecretsManager, secretName)
		if err != nil {
			return messages.FlashMsg{Text: "failed to reveal secret: " + err.Error(), IsError: true}
		}
		return messages.SecretRevealedMsg{SecretName: secretName, Value: value}
	}
}

func (m *Model) connectAWS(profile, region string) tea.Cmd {
	// Resolve region fallback BEFORE the async closure so that NewAWSSession
	// always receives a non-empty region. Without this, the SDK fails with
	// "Missing Region" when ~/.aws/config is absent (issue #82).
	if region == "" {
		configPath := awsclient.DefaultConfigPath()
		region = awsclient.GetDefaultRegion(configPath, profile)
	}
	return func() tea.Msg {
		cfg, err := awsclient.NewAWSSession(profile, region)
		if err != nil {
			return messages.ClientsReadyMsg{Err: err}
		}
		clients := awsclient.CreateServiceClients(cfg)
		return messages.ClientsReadyMsg{Clients: clients}
	}
}

// loadAvailabilityCache returns a tea.Cmd that reads the availability cache from disk.
func (m *Model) loadAvailabilityCache() tea.Cmd {
	profile := m.profile
	region := m.region
	return func() tea.Msg {
		cf, err := cache.Load(profile, region)
		if err != nil || cf == nil {
			// No cache or error — return empty entries, will trigger full re-check
			return messages.AvailabilityCacheLoadedMsg{
				Entries: make(map[string]int),
				Expired: true,
			}
		}
		entries := make(map[string]int, len(cf.Resources))
		truncated := make(map[string]bool)
		for name, entry := range cf.Resources {
			if entry.Error == "" {
				entries[name] = entry.Count
				if entry.Truncated {
					truncated[name] = true
				}
			}
		}
		return messages.AvailabilityCacheLoadedMsg{
			Entries:   entries,
			Truncated: truncated,
			Expired:   cf.IsExpired(cache.DefaultTTL),
		}
	}
}

// probeResourceAvailability returns a tea.Cmd that checks if a resource type
// has any resources by calling its registered fetcher with a timeout.
func (m *Model) probeResourceAvailability(shortName string, gen int) tea.Cmd {
	clients := m.clients
	return func() tea.Msg {
		if clients == nil {
			return messages.AvailabilityCheckedMsg{
				ResourceType: shortName,
				Err:          fmt.Errorf("AWS clients not initialized"),
				Gen:          gen,
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fetcher := resource.GetFetcher(shortName)
		if fetcher == nil {
			return messages.AvailabilityCheckedMsg{
				ResourceType: shortName,
				Err:          fmt.Errorf("no fetcher for %s", shortName),
				Gen:          gen,
			}
		}
		resources, err := fetcher(ctx, clients)
		if err != nil {
			return messages.AvailabilityCheckedMsg{
				ResourceType: shortName,
				Err:          err,
				Gen:          gen,
			}
		}
		return messages.AvailabilityCheckedMsg{
			ResourceType: shortName,
			HasResources: len(resources) > 0,
			Count:        len(resources),
			Gen:          gen,
		}
	}
}

// saveAvailabilityCache returns a tea.Cmd that persists the current availability state to disk.
// No-op in demo mode (no cache files for synthetic data).
func (m *Model) saveAvailabilityCache() tea.Cmd {
	if m.demoMode {
		return nil
	}
	profile := m.profile
	region := m.region

	// Collect availability and truncation from main menu
	var entries map[string]int
	var truncatedMap map[string]bool
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		entries = menu.GetAvailability()
		truncatedMap = menu.GetTruncated()
	}
	if entries == nil {
		return nil
	}

	return func() tea.Msg {
		cf := &cache.File{
			Profile:   profile,
			Region:    region,
			CheckedAt: time.Now(),
			Resources: make(map[string]cache.Entry, len(entries)),
		}
		for name, count := range entries {
			trunc := false
			if truncatedMap != nil {
				trunc = truncatedMap[name]
			}
			cf.Resources[name] = cache.Entry{HasResources: count > 0, Count: count, Truncated: trunc}
		}
		// Best-effort save — don't flash errors for cache write failures
		_ = cache.Save(cf)
		return nil
	}
}


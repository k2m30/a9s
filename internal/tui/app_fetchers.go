package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	ctx := m.appCtx
	return func() tea.Msg {
		if clients == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("AWS clients not initialized"),
			}
		}

		pf := resource.GetPaginatedFetcher(resourceType)
		if pf == nil {
			return messages.APIErrorMsg{
				ResourceType: resourceType,
				Err:          fmt.Errorf("unsupported resource type: %s", resourceType),
			}
		}
		result, err := pf(ctx, clients, "")
		if err != nil {
			return messages.APIErrorMsg{ResourceType: resourceType, Err: err}
		}
		// IsTruncated: populated from first-page result; loaders stop at page 1.
		return messages.ResourcesLoadedMsg{
			ResourceType: resourceType,
			Resources:    result.Resources,
			Pagination:   result.Pagination,
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
		if err != nil {
			return messages.APIErrorMsg{ResourceType: resourceType, Err: err}
		}
		return messages.ResourcesLoadedMsg{
			ResourceType: resourceType,
			Resources:    result.Resources,
			Pagination:   result.Pagination,
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

		// Filtered fetch path: used by related navigation with server-side filters (e.g., CloudTrail LookupAttributes).
		if len(msg.FetchFilter) > 0 {
			pf := resource.GetFilteredPaginatedFetcher(rt)
			if pf != nil {
				result, err := pf(ctx, clients, msg.FetchFilter, token)
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
	ctx := m.appCtx
	return func() tea.Msg {
		if clients == nil {
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
		issueCounts := make(map[string]int)
		issueTruncated := make(map[string]bool)
		issueKnown := make(map[string]bool)
		for name, entry := range cf.Resources {
			if entry.Error == "" {
				entries[name] = entry.Count
				if entry.Truncated {
					truncated[name] = true
				}
				if entry.IssuesKnown {
					issueCounts[name] = entry.Issues
					issueKnown[name] = true
					if entry.IssuesTruncated {
						issueTruncated[name] = true
					}
				}
			}
		}
		return messages.AvailabilityCacheLoadedMsg{
			Entries:        entries,
			Truncated:      truncated,
			Expired:        cf.IsExpired(cache.DefaultTTL),
			IssueCounts:    issueCounts,
			IssueTruncated: issueTruncated,
			IssueKnown:     issueKnown,
		}
	}
}

// probeResourceAvailability returns a tea.Cmd that checks if a resource type
// has any resources by calling its registered fetcher with a timeout.
// Paginated fetchers are tried first so that truncation can be detected and
// reported as "(N+)" in the main menu.
func (m *Model) probeResourceAvailability(shortName string, gen int) tea.Cmd {
	clients := m.clients
	appCtx := m.appCtx
	return func() tea.Msg {
		if clients == nil {
			return messages.AvailabilityCheckedMsg{
				ResourceType: shortName,
				Err:          fmt.Errorf("AWS clients not initialized"),
				Gen:          gen,
			}
		}
		ctx, cancel := context.WithTimeout(appCtx, 10*time.Second)
		defer cancel()

		pf := resource.GetPaginatedFetcher(shortName)
		if pf == nil {
			return messages.AvailabilityCheckedMsg{
				ResourceType: shortName,
				Err:          fmt.Errorf("no fetcher for %s", shortName),
				Gen:          gen,
			}
		}

		result, err := awsclient.RetryOnThrottle(ctx, awsclient.DefaultRetryConfig(), func() (resource.FetchResult, error) {
			return pf(ctx, clients, "")
		})
		if err != nil {
			return messages.AvailabilityCheckedMsg{
				ResourceType: shortName,
				Err:          err,
				Gen:          gen,
			}
		}
		truncated := result.Pagination != nil && result.Pagination.IsTruncated
		// Count issue-status resources (red/yellow only, not green/dim).
		issues := 0
		td := resource.FindResourceType(shortName)
		for _, r := range result.Resources {
			if td != nil && !td.ExcludeFromIssueBadge && td.ResolveColor(r).IsIssue() {
				issues++
			}
		}
		return messages.AvailabilityCheckedMsg{
			ResourceType: shortName,
			HasResources: len(result.Resources) > 0,
			Count:        len(result.Resources),
			Truncated:    truncated,
			Gen:          gen,
			Issues:       issues,
			Resources:    result.Resources,
		}
	}
}

// saveAvailabilityCache returns a tea.Cmd that persists the current availability state to disk.
// No-op when caching is disabled (e.g. demo mode or --no-cache).
func (m *Model) saveAvailabilityCache() tea.Cmd {
	if m.noCache {
		return nil
	}
	profile := m.profile
	region := m.region

	// Collect availability, truncation, and issue counts from main menu.
	var entries map[string]int
	var truncatedMap map[string]bool
	var issueCounts map[string]int
	var issueTruncated map[string]bool
	var issueKnown map[string]bool
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		entries = menu.GetAvailability()
		truncatedMap = menu.GetTruncated()
		issueCounts = menu.GetIssueCounts()
		issueTruncated = menu.GetIssueTruncated()
		issueKnown = menu.GetIssueKnown()
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
			e := cache.Entry{HasResources: count > 0, Count: count, Truncated: trunc}
			if issueKnown[name] {
				e.Issues = issueCounts[name]
				e.IssuesKnown = true
				e.IssuesTruncated = issueTruncated[name]
			}
			cf.Resources[name] = e
		}
		// Best-effort save — don't flash errors for cache write failures
		_ = cache.Save(cf)
		return nil
	}
}

// demoPrefetchCounts returns a tea.Cmd that synchronously calls all registered
// paginated fetchers and returns AvailabilityPrefetchedMsg with all counts
// pre-filled. Used when pre-supplied clients are present and no-cache is active,
// so the main menu shows counts immediately without the async probe pipeline.
func (m *Model) demoPrefetchCounts() tea.Cmd {
	clients := m.clients
	appCtx := m.appCtx
	return func() tea.Msg {
		allNames := resource.AllShortNames()
		entries := make(map[string]int, len(allNames))
		truncated := make(map[string]bool)
		issueCounts := make(map[string]int, len(allNames))
		issueTruncated := make(map[string]bool)
		retainedResources := make(map[string][]resource.Resource, len(allNames))
		ctx, cancel := context.WithTimeout(appCtx, 30*time.Second)
		defer cancel()
		for _, shortName := range allNames {
			pf := resource.GetPaginatedFetcher(shortName)
			if pf == nil {
				continue
			}
			result, err := pf(ctx, clients, "")
			if err != nil {
				continue
			}
			entries[shortName] = len(result.Resources)
			isTrunc := result.Pagination != nil && result.Pagination.IsTruncated
			if isTrunc {
				truncated[shortName] = true
				issueTruncated[shortName] = true
			}
			// Count issue-status resources (red/yellow only).
			issues := 0
			td := resource.FindResourceType(shortName)
			for _, r := range result.Resources {
				if td != nil && !td.ExcludeFromIssueBadge && td.ResolveColor(r).IsIssue() {
					issues++
				}
			}
			issueCounts[shortName] = issues
			// Retain first-page resources for Wave 2 enricher consumption.
			retainedResources[shortName] = result.Resources
		}
		return messages.AvailabilityPrefetchedMsg{
			Entries:        entries,
			Truncated:      truncated,
			IssueCounts:    issueCounts,
			IssueTruncated: issueTruncated,
			Resources:      retainedResources,
		}
	}
}

// refreshResourceListWithEnrichmentRerun wraps the ordinary refresh fetch for
// a top-level list so that the ResourcesLoadedMsg it produces carries an
// enrichment-rerun token. The token is captured at Ctrl+R dispatch time and
// stamped into the message; the ResourcesLoadedMsg handler in app.go checks
// TypeGen in its tail branch to decide whether to seed probeResources and
// dispatch probeEnrichment. APIErrorMsg and any other message pass through
// unchanged.
func (m *Model) refreshResourceListWithEnrichmentRerun(
	rl views.ResourceListModel, tok int,
) tea.Cmd {
	inner := m.refreshResourceList(rl)
	return func() tea.Msg {
		msg := inner()
		if loaded, ok := msg.(messages.ResourcesLoadedMsg); ok {
			loaded.TypeGen = tok
			return loaded
		}
		return msg
	}
}

// buildEnrichQueue returns resource types that have registered enrichers AND
// have retained probe resources, sorted by declarative priority from
// EnricherRegistry[name].Priority (lower values first), then alphabetically
// within the same priority tier. Priority is metadata on the registry entry:
// 10 = batchable (cheap, run first), 100 = default per-resource enricher.
func (m *Model) buildEnrichQueue() []string {
	type pair struct {
		name     string
		priority int
	}

	var ps []pair
	for name, e := range awsclient.EnricherRegistry {
		if _, ok := m.probeResources[name]; !ok {
			continue
		}
		ps = append(ps, pair{name: name, priority: e.Priority})
	}
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].priority != ps[j].priority {
			return ps[i].priority < ps[j].priority
		}
		return ps[i].name < ps[j].name // stable: alphabetical within priority
	})
	queue := make([]string, len(ps))
	for i, p := range ps {
		queue[i] = p.name
	}
	return queue
}

// probeEnrichment returns a tea.Cmd that runs the registered enricher for a
// resource type and returns an EnrichmentCheckedMsg.
// typeGen is the per-type generation counter captured at dispatch time; it is
// embedded in the message so handleEnrichmentChecked can drop stale results.
func (m *Model) probeEnrichment(shortName string, gen int) tea.Cmd {
	clients := m.clients
	appCtx := m.appCtx
	resources := m.probeResources[shortName]
	enricherFn := awsclient.EnricherRegistry[shortName].Fn
	typeGen := m.enrichmentTypeGen[shortName]
	if enricherFn == nil {
		return nil
	}
	return func() tea.Msg {
		if clients == nil {
			return messages.EnrichmentCheckedMsg{
				ResourceType: shortName,
				Err:          fmt.Errorf("AWS clients not initialized"),
				Gen:          gen,
				TypeGen:      typeGen,
			}
		}
		ctx, cancel := context.WithTimeout(appCtx, 10*time.Second)
		defer cancel()

		result, err := awsclient.RetryOnThrottle(ctx, awsclient.DefaultRetryConfig(), func() (awsclient.EnricherResult, error) {
			return enricherFn(ctx, clients, resources)
		})
		if err != nil {
			return messages.EnrichmentCheckedMsg{
				ResourceType: shortName,
				Err:          err,
				Gen:          gen,
				TypeGen:      typeGen,
			}
		}
		return messages.EnrichmentCheckedMsg{
			ResourceType: shortName,
			Issues:       result.IssueCount,
			Truncated:    result.Truncated,
			Findings:     result.Findings,
			Gen:          gen,
			TypeGen:      typeGen,
		}
	}
}

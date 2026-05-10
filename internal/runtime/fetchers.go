// fetchers.go — pure fetch-execution layer for the runtime Core.
//
// PR-05a-h6 moves the fetch-execution logic out of internal/tui/app_fetchers.go
// into the platform-agnostic runtime package. Each method performs the AWS call
// and returns a raw result; adapters wrap the call in platform-specific async
// machinery (e.g. tea.Cmd for the Bubble Tea adapter).
//
// No Bubble Tea, Lipgloss, or Bubbles imports are permitted in this file.
package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ConnectResult carries the resolved AWS clients and effective region returned
// by ConnectAWS. Adapters translate this into a platform-specific "clients
// ready" signal (e.g. messages.ClientsReadyMsg for the Bubble Tea adapter).
type ConnectResult struct {
	Clients *awsclient.ServiceClients
	Region  string
}

// FetchMoreParams carries the parameters for a paginated next-page request.
type FetchMoreParams struct {
	ResourceType string
	Token        string
	ParentCtx    map[string]string
	FetchFilter  map[string]string
}

// FetchResources calls the registered paginated fetcher for resourceType
// and returns the first-page result.
func (c *Core) FetchResources(ctx context.Context, clients *awsclient.ServiceClients, resourceType string) (resource.FetchResult, error) {
	pf := resource.GetPaginatedFetcher(resourceType)
	if pf == nil {
		return resource.FetchResult{}, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
	return pf(ctx, clients, "")
}

// FetchResourcesFiltered calls the registered FilteredPaginatedFetcher for
// resourceType with the provided filter parameters.
func (c *Core) FetchResourcesFiltered(ctx context.Context, clients *awsclient.ServiceClients, resourceType string, filter map[string]string) (resource.FetchResult, error) {
	if clients == nil {
		return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
	}
	pf := resource.GetFilteredPaginatedFetcher(resourceType)
	if pf == nil {
		return resource.FetchResult{}, fmt.Errorf("no filtered fetcher registered for: %s", resourceType)
	}
	return pf(ctx, clients, filter, "")
}

// FetchAMIDetail fetches a single AMI by image ID and returns the resource.
func (c *Core) FetchAMIDetail(ctx context.Context, clients *awsclient.ServiceClients, imageID string) (resource.Resource, error) {
	if clients == nil {
		return resource.Resource{}, fmt.Errorf("AWS clients not initialized; cannot load AMI %s", imageID)
	}
	res, err := awsclient.FetchAMIByID(ctx, clients.EC2, imageID)
	if err != nil {
		return resource.Resource{}, err
	}
	return res, nil
}

// FetchChildResources calls the registered paginated child fetcher for
// childType with the given parent context and returns the first-page result.
func (c *Core) FetchChildResources(ctx context.Context, clients *awsclient.ServiceClients, childType string, parentCtx map[string]string) (resource.FetchResult, error) {
	if clients == nil {
		return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
	}
	pc := resource.ParentContext(parentCtx)
	pf := resource.GetPaginatedChildFetcher(childType)
	if pf == nil {
		return resource.FetchResult{}, fmt.Errorf("unsupported child type: %s", childType)
	}
	return pf(ctx, clients, pc, "")
}

// FetchMoreResources fetches the next page of resources using the given
// params. It tries filtered fetch, then child fetch, then top-level paginated
// fetch — matching the routing logic from the original app_fetchers.go.
// The bool return indicates whether any fetcher was found (false = no fetcher
// registered, error describes the missing type).
func (c *Core) FetchMoreResources(ctx context.Context, clients *awsclient.ServiceClients, p FetchMoreParams) (resource.FetchResult, bool, error) {
	if clients == nil {
		return resource.FetchResult{}, false, fmt.Errorf("AWS clients not initialized")
	}

	if len(p.FetchFilter) > 0 {
		pf := resource.GetFilteredPaginatedFetcher(p.ResourceType)
		if pf != nil {
			res, err := pf(ctx, clients, p.FetchFilter, p.Token)
			return res, true, err
		}
	}

	if len(p.ParentCtx) > 0 {
		pf := resource.GetPaginatedChildFetcher(p.ResourceType)
		if pf != nil {
			pc := resource.ParentContext(p.ParentCtx)
			res, err := pf(ctx, clients, pc, p.Token)
			return res, true, err
		}
	}

	pf := resource.GetPaginatedFetcher(p.ResourceType)
	if pf != nil {
		res, err := pf(ctx, clients, p.Token)
		return res, true, err
	}

	return resource.FetchResult{}, false, fmt.Errorf("no paginated fetcher for: %s", p.ResourceType)
}

// FetchIdentity calls STS GetCallerIdentity and IAM ListAccountAliases.
func (c *Core) FetchIdentity(ctx context.Context, clients *awsclient.ServiceClients) (*awsclient.CallerIdentity, error) {
	if clients == nil || clients.STS == nil {
		return nil, fmt.Errorf("AWS clients not initialized")
	}
	return awsclient.FetchCallerIdentity(ctx, clients.STS, clients.IAM)
}

// FetchProfiles reads the local AWS config file and returns available profile
// names. Returns an error when the file cannot be read or is empty.
func (c *Core) FetchProfiles() ([]string, error) {
	configPath := awsclient.DefaultConfigPath()
	profiles, err := awsclient.ListProfiles(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list profiles: %w", err)
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no AWS profiles found")
	}
	return profiles, nil
}

// FetchRevealValue calls the registered reveal fetcher for the given resource
// type and ID.
func (c *Core) FetchRevealValue(ctx context.Context, clients *awsclient.ServiceClients, resourceType, resourceID string) (string, error) {
	if clients == nil {
		return "", fmt.Errorf("AWS clients not initialized")
	}
	fetcher := resource.GetRevealFetcher(resourceType)
	if fetcher == nil {
		return "", fmt.Errorf("no reveal support for %s", resourceType)
	}
	return fetcher(ctx, clients, resourceID)
}

// ConnectAWS establishes an AWS session for the given profile and region,
// resolving the effective region from the config file or environment when the
// caller supplies an empty string.
func (c *Core) ConnectAWS(ctx context.Context, profile, region string) (ConnectResult, error) {
	configPath := awsclient.DefaultConfigPath()
	bestRegion := region

	cfg, err := awsclient.NewAWSSessionContext(ctx, profile, region)
	if cfg.Region != "" {
		bestRegion = cfg.Region
	} else if r := os.Getenv("AWS_REGION"); r != "" {
		bestRegion = r
	} else if r := os.Getenv("AWS_DEFAULT_REGION"); r != "" {
		bestRegion = r
	}
	if err != nil {
		if region == "" && bestRegion == "" && isMissingRegionErr(err) {
			fallback := awsclient.GetDefaultRegion(configPath, profile)
			cfg2, err2 := awsclient.NewAWSSessionContext(ctx, profile, fallback)
			if cfg2.Region != "" {
				bestRegion = cfg2.Region
			} else {
				bestRegion = fallback
			}
			if err2 == nil {
				clients := awsclient.CreateServiceClients(cfg2)
				return ConnectResult{Clients: clients, Region: bestRegion}, nil
			}
			err = err2
		}
		if bestRegion == "" {
			bestRegion = awsclient.GetDefaultRegion(configPath, profile)
		}
		return ConnectResult{Region: bestRegion}, err
	}
	if cfg.Region == "" && region == "" {
		fallback := awsclient.GetDefaultRegion(configPath, profile)
		cfg2, err2 := awsclient.NewAWSSessionContext(ctx, profile, fallback)
		if err2 != nil {
			if bestRegion == "" {
				bestRegion = fallback
			}
			return ConnectResult{Region: bestRegion}, err2
		}
		if cfg2.Region != "" {
			bestRegion = cfg2.Region
		} else {
			bestRegion = fallback
		}
		cfg = cfg2
	}
	clients := awsclient.CreateServiceClients(cfg)
	return ConnectResult{Clients: clients, Region: bestRegion}, nil
}

// isMissingRegionErr reports whether an AWS config error is due to a missing
// region setting.
func isMissingRegionErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "missing region") || strings.Contains(msg, "could not find region")
}

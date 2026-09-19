package unit

import (
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// writeCacheTypesForModel writes one or more per-type cache.TypeFile entries
// for the given profile/region via the real Store API (Put+SaveType).
func writeCacheTypesForModel(t *testing.T, profile, region string, entries map[string]cache.TypeFile) {
	t.Helper()
	store := cache.LoadDirForTest(profile, region)
	for name, tf := range entries {
		store.Put(name, tf)
	}
	for name := range entries {
		if err := store.SaveType(name); err != nil {
			t.Fatalf("SaveType(%s): %v", name, err)
		}
	}
}

// loadAvailabilityCache runs inside handleClientsReady, so a ClientsReadyMsg
// triggers the full path.
func TestLoadAvailabilityCache_PopulatedCacheReturnsEntries(t *testing.T) {
	withTuiVersion(t, "test")
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	profile := "wave5-cache-profile"
	region := "eu-west-1"

	writeCacheTypesForModel(t, profile, region, map[string]cache.TypeFile{
		"ec2": {HasResources: true, Count: 12},
		"s3":  {HasResources: true, Count: 5},
		"kms": {}, // empty/errored entry excluded from Entries
	})

	clients := demo.NewServiceClients()
	m := newBlessedModel(t, profile, region,
		tui.WithClients(clients),
		tui.WithNoCache(false),
	)
	t.Cleanup(func() { m.CloseController() })
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Trigger handleClientsReady which calls loadAvailabilityCache. Gen:1 —
	// ConnectGen seeds at 1 (session.New()); this model is never rotated.
	_, cmd := rootApplyMsg(m, messages.ClientsReady{
		Clients: clients,
		Region:  region,
		Gen:     1,
	})
	if cmd == nil {
		t.Fatal("ClientsReadyMsg should return a cmd batch containing loadAvailabilityCache cmd")
	}

	var cacheMsg *messages.AvailabilityCacheLoaded
	collectFromCmd(t, cmd, &cacheMsg)

	if cacheMsg == nil {
		t.Fatal("AvailabilityCacheLoadedMsg not found in batch (loadAvailabilityCache not exercised)")
	}
	if cacheMsg.Entries["ec2"] != 12 {
		t.Errorf("Entries[ec2] = %d, want 12", cacheMsg.Entries["ec2"])
	}
	if cacheMsg.Entries["s3"] != 5 {
		t.Errorf("Entries[s3] = %d, want 5", cacheMsg.Entries["s3"])
	}
	if _, ok := cacheMsg.Entries["kms"]; ok {
		t.Errorf("Entries[kms] should be absent (error entry excluded), but was present")
	}
}

func TestLoadAvailabilityCache_IssueFieldsMapped(t *testing.T) {
	withTuiVersion(t, "test")
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	profile := "wave5-issues-profile"
	region := "us-west-2"

	writeCacheTypesForModel(t, profile, region, map[string]cache.TypeFile{
		"ec2": {
			HasResources:    true,
			Count:           8,
			Issues:          3,
			IssuesKnown:     true,
			IssuesTruncated: true,
		},
	})

	clients := demo.NewServiceClients()
	m := newBlessedModel(t, profile, region,
		tui.WithClients(clients),
		tui.WithNoCache(false),
	)
	t.Cleanup(func() { m.CloseController() })
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	_, cmd := rootApplyMsg(m, messages.ClientsReady{
		Clients: clients,
		Region:  region,
		Gen:     1,
	})
	if cmd == nil {
		t.Fatal("ClientsReadyMsg should return a cmd batch")
	}

	var cacheMsg *messages.AvailabilityCacheLoaded
	collectFromCmd(t, cmd, &cacheMsg)
	if cacheMsg == nil {
		t.Fatal("AvailabilityCacheLoadedMsg not found in batch")
	}

	if cacheMsg.IssueCounts["ec2"] != 3 {
		t.Errorf("IssueCounts[ec2] = %d, want 3", cacheMsg.IssueCounts["ec2"])
	}
	if !cacheMsg.IssueKnown["ec2"] {
		t.Errorf("IssueKnown[ec2] = false, want true")
	}
	if !cacheMsg.IssueTruncated["ec2"] {
		t.Errorf("IssueTruncated[ec2] = false, want true")
	}
}

// collectFromCmd recursively walks a tea.BatchMsg / tea.Cmd to find the first
// AvailabilityCacheLoadedMsg, storing it in target if found.
func collectFromCmd(t *testing.T, cmd tea.Cmd, target **messages.AvailabilityCacheLoaded) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	switch v := msg.(type) {
	case messages.AvailabilityCacheLoaded:
		*target = &v
	case tea.BatchMsg:
		for _, sub := range v {
			if *target != nil {
				return
			}
			collectFromCmd(t, sub, target)
		}
	}
}

func TestFetchMoreResources_FilteredFetcherErrorPropagated(t *testing.T) {
	withTuiVersion(t, "test")

	const errFilteredType = "test_filtered_err_wave5"
	resource.SetFilteredPaginatedForTest(errFilteredType, func(
		_ context.Context, _ any, _ map[string]string, _ string,
	) (resource.FetchResult, error) {
		return resource.FetchResult{}, fmt.Errorf("simulated fetcher error")
	})
	t.Cleanup(func() {
		resource.CleanupFilteredPaginatedForTest(errFilteredType)
	})

	clients := demo.NewServiceClients()
	m := newBlessedModel(t, demo.DemoProfile, demo.DemoRegion,
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	_, cmd := rootApplyMsg(m, messages.LoadMore{
		ResourceType:      errFilteredType,
		ContinuationToken: "page-2-token",
		FetchFilter:       map[string]string{"key": "value"},
	})
	if cmd == nil {
		t.Fatal("LoadMoreMsg with failing filtered fetcher should return a cmd")
	}
	msg := cmd()
	apiErr, ok := msg.(messages.APIError)
	if !ok {
		t.Fatalf("expected APIErrorMsg from filtered fetcher error, got %T", msg)
	}
	if apiErr.ResourceType != errFilteredType {
		t.Errorf("ResourceType = %q, want %q", apiErr.ResourceType, errFilteredType)
	}
}

// With nil clients the nil-client guard fires first; demo clients reach the
// no-filtered-fetcher branch.
func TestFetchResourcesFiltered_NoFetcherWithDemoClients(t *testing.T) {
	withTuiVersion(t, "test")

	// ec2 has a paginated fetcher but NOT a filtered paginated fetcher.
	if resource.GetFilteredPaginatedFetcher("ec2") != nil {
		t.Skip("ec2 unexpectedly has a FilteredPaginatedFetcher — test precondition failed")
	}

	clients := demo.NewServiceClients()
	m := newBlessedModel(t, demo.DemoProfile, demo.DemoRegion,
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	const noFFType = "test_no_ff_demo_clients"

	_, cmd := rootApplyMsg(m, messages.LoadMore{
		ResourceType: noFFType,
		FetchFilter:  map[string]string{"k": "v"},
	})
	if cmd == nil {
		t.Fatal("LoadMoreMsg should return a cmd")
	}
	msg := cmd()
	// With demo clients (non-nil) and no FilteredPaginatedFetcher,
	// fetchMoreResources skips to the paginated fetcher path. Since noFFType
	// also has no paginated fetcher, it falls through to "no paginated fetcher for".
	apiErr, ok := msg.(messages.APIError)
	if !ok {
		t.Fatalf("expected APIErrorMsg, got %T", msg)
	}
	if apiErr.ResourceType != noFFType {
		t.Errorf("ResourceType = %q, want %q", apiErr.ResourceType, noFFType)
	}
}

//go:build integration

package integration

import (
	"flag"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

var (
	liveIntegrationProfile = flag.String("a9s.profile", "", "AWS profile for opt-in live full integration test")
	liveIntegrationRegion  = flag.String("a9s.region", "", "optional AWS region override for opt-in live full integration test")
)

func TestLiveFullIntegration_AllResourcesBaseline(t *testing.T) {
	if strings.TrimSpace(*liveIntegrationProfile) == "" {
		t.Skip("set -args -a9s.profile <profile> to run the live AWS full integration test; -a9s.region is optional")
	}

	profile := strings.TrimSpace(*liveIntegrationProfile)
	region := strings.TrimSpace(*liveIntegrationRegion)
	m := tui.New(profile, region, tui.WithNoCache(true))
	// Large enough to render every main-menu resource row at once.
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})

	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "live Init")
	var connectCmd tea.Cmd
	m, connectCmd = fullIntegrationApplyMsg(m, initMsg)
	clientsReadyRaw := fullIntegrationRequireCmdMsg(t, connectCmd, "live AWS connect")
	clientsReady, ok := clientsReadyRaw.(messages.ClientsReadyMsg)
	if !ok {
		t.Fatalf("live AWS connect returned %T, expected messages.ClientsReadyMsg", clientsReadyRaw)
	}
	if clientsReady.Err != nil {
		t.Fatalf("live AWS connect failed for profile=%q region=%q: %v", profile, region, clientsReady.Err)
	}
	if region == "" {
		region = clientsReady.Region
	}
	clients, ok := clientsReady.Clients.(*awsclient.ServiceClients)
	if !ok || clients == nil {
		t.Fatalf("live AWS connect returned clients %T, expected *aws.ServiceClients", clientsReady.Clients)
	}

	expectedTopLevel := fullIntegrationExpectedFirstPageCounts(t, clients)

	var prefetchCmd tea.Cmd
	m, prefetchCmd = fullIntegrationApplyMsg(m, clientsReady)
	availMsg := fullIntegrationExtractMsg(t, prefetchCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetchedMsg)
		return ok
	})
	m, _ = fullIntegrationApplyMsg(m, availMsg)

	fullIntegrationAssertMainMenuCounts(t, m, expectedTopLevel)

	fullIntegrationRunAllResourceBaseline(t, clients, func() tui.Model {
		return fullIntegrationNewReadyModelWithClients(t, profile, region, clients)
	}, expectedTopLevel)
}

func TestLiveFullIntegration_RelatedHopScenarios(t *testing.T) {
	if strings.TrimSpace(*liveIntegrationProfile) == "" {
		t.Skip("set -args -a9s.profile <profile> to run the live AWS full integration test; -a9s.region is optional")
	}

	profile := strings.TrimSpace(*liveIntegrationProfile)
	region := strings.TrimSpace(*liveIntegrationRegion)
	m := tui.New(profile, region, tui.WithNoCache(true))
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})

	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "live Init")
	var connectCmd tea.Cmd
	m, connectCmd = fullIntegrationApplyMsg(m, initMsg)
	clientsReadyRaw := fullIntegrationRequireCmdMsg(t, connectCmd, "live AWS connect")
	clientsReady, ok := clientsReadyRaw.(messages.ClientsReadyMsg)
	if !ok {
		t.Fatalf("live AWS connect returned %T, expected messages.ClientsReadyMsg", clientsReadyRaw)
	}
	if clientsReady.Err != nil {
		t.Fatalf("live AWS connect failed for profile=%q region=%q: %v", profile, region, clientsReady.Err)
	}
	if region == "" {
		region = clientsReady.Region
	}
	clients, ok := clientsReady.Clients.(*awsclient.ServiceClients)
	if !ok || clients == nil {
		t.Fatalf("live AWS connect returned clients %T, expected *aws.ServiceClients", clientsReady.Clients)
	}

	expectedTopLevel := fullIntegrationExpectedFirstPageCounts(t, clients)

	var prefetchCmd tea.Cmd
	m, prefetchCmd = fullIntegrationApplyMsg(m, clientsReady)
	availMsg := fullIntegrationExtractMsg(t, prefetchCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetchedMsg)
		return ok
	})
	m, _ = fullIntegrationApplyMsg(m, availMsg)

	scenarios := []fullIntegrationRelatedHopScenario{
		{
			name:              "ecs-service-cluster-service",
			sourceType:        "ecs-svc",
			firstTargetType:   "ecs",
			firstDisplayName:  "ECS Clusters",
			returnTargetType:  "ecs-svc",
			returnDisplayName: "ECS Services",
		},
	}
	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			fullIntegrationRunRelatedHopScenario(t, clients, &m, expectedTopLevel, scenario)
		})
	}
}

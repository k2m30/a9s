//go:build integration

package integration

import (
	"flag"
	"fmt"
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

func TestLiveFullIntegration_MainCountsAndECSRelatedNavigation(t *testing.T) {
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

	ecsLoaded := fullIntegrationOpenResourceList(t, &m, "ecs-svc")
	expectedECS := expectedTopLevel["ecs-svc"]
	if got := len(ecsLoaded.Resources); got != expectedECS.count {
		t.Fatalf("live ecs-svc list loaded %d resources, expected %d from live fetcher", got, expectedECS.count)
	}
	if expectedECS.count == 0 {
		t.Skipf("live profile=%q region=%q has zero ECS services; cannot run ECS related navigation flow", profile, region)
	}
	fullIntegrationAssertFrameContains(t, m, fullIntegrationFrameCount("ecs-svc", expectedECS))

	firstService, firstServiceResults := fullIntegrationDescribeSelectedResource(t, &m, "ecs-svc")
	expectedFirstService := fullIntegrationExpectedRelatedCounts(t, clients, "ecs-svc", firstService)
	fullIntegrationAssertRelatedResults(t, expectedFirstService, firstServiceResults, "first live ECS service")
	fullIntegrationAssertRelatedCountsInView(t, m, expectedFirstService, "first live ECS service")

	clusterResource, clusterResults := fullIntegrationEnterRelatedSingleDetail(t, &m, "ecs", "ECS Clusters")
	expectedCluster := fullIntegrationExpectedRelatedCounts(t, clients, "ecs", clusterResource)
	fullIntegrationAssertRelatedResults(t, expectedCluster, clusterResults, "related live ECS cluster")
	fullIntegrationAssertRelatedCountsInView(t, m, expectedCluster, "related live ECS cluster")

	ecsServicesCount := expectedCluster["ECS Services"]
	if ecsServicesCount <= 0 {
		t.Fatalf("related live ECS cluster has ECS Services count %d; test needs a navigable service row", ecsServicesCount)
	}
	fullIntegrationEnterRelatedList(t, &m, "ecs-svc", "ECS Services")
	fullIntegrationAssertFrameContains(t, m, fmt.Sprintf("ecs-svc(%d)", ecsServicesCount))

	if ecsServicesCount > 1 {
		m, _ = fullIntegrationApplyMsg(m, fullIntegrationKeyPress("j"))
	}
	secondService, secondServiceResults := fullIntegrationDescribeSelectedResource(t, &m, "ecs-svc")
	expectedSecondService := fullIntegrationExpectedRelatedCounts(t, clients, "ecs-svc", secondService)
	fullIntegrationAssertRelatedResults(t, expectedSecondService, secondServiceResults, "live ECS service reached from cluster related list")
	fullIntegrationAssertRelatedCountsInView(t, m, expectedSecondService, "live ECS service reached from cluster related list")
}

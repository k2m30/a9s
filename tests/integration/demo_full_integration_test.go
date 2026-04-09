//go:build integration

package integration

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

func TestDemoFullIntegration_MainCountsAndECSRelatedNavigation(t *testing.T) {
	clients := demo.NewServiceClients()
	expectedTopLevel := fullIntegrationExpectedFirstPageCounts(t, clients)

	m := tui.New(
		demo.DemoProfile,
		demo.DemoRegion,
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	// Large enough to render every main-menu resource row at once.
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})

	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "demo Init")
	var cmd tea.Cmd
	m, cmd = fullIntegrationApplyMsg(m, initMsg)
	availMsg := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetchedMsg)
		return ok
	})
	m, _ = fullIntegrationApplyMsg(m, availMsg)

	fullIntegrationAssertMainMenuCounts(t, m, expectedTopLevel)

	ecsLoaded := fullIntegrationOpenResourceList(t, &m, "ecs-svc")
	expectedECS := expectedTopLevel["ecs-svc"]
	if got := len(ecsLoaded.Resources); got != expectedECS.count {
		t.Fatalf("ecs-svc list loaded %d resources, expected %d from demo fetcher", got, expectedECS.count)
	}
	if expectedECS.truncated {
		t.Fatalf("test expects full ecs-svc first page, got truncated count %d+", expectedECS.count)
	}
	fullIntegrationAssertFrameContains(t, m, fmt.Sprintf("ecs-svc(%d)", expectedECS.count))

	firstService, firstServiceResults := fullIntegrationDescribeSelectedResource(t, &m, "ecs-svc")
	expectedFirstService := fullIntegrationExpectedRelatedCounts(t, clients, "ecs-svc", firstService)
	fullIntegrationAssertRelatedResults(t, expectedFirstService, firstServiceResults, "first ECS service")
	fullIntegrationAssertRelatedCountsInView(t, m, expectedFirstService, "first ECS service")

	clusterResource, clusterResults := fullIntegrationEnterRelatedSingleDetail(t, &m, "ecs", "ECS Clusters")
	expectedCluster := fullIntegrationExpectedRelatedCounts(t, clients, "ecs", clusterResource)
	fullIntegrationAssertRelatedResults(t, expectedCluster, clusterResults, "related ECS cluster")
	fullIntegrationAssertRelatedCountsInView(t, m, expectedCluster, "related ECS cluster")

	ecsServicesCount := expectedCluster["ECS Services"]
	if ecsServicesCount <= 0 {
		t.Fatalf("related ECS cluster has ECS Services count %d; test needs a navigable service row", ecsServicesCount)
	}
	fullIntegrationEnterRelatedList(t, &m, "ecs-svc", "ECS Services")
	fullIntegrationAssertFrameContains(t, m, fmt.Sprintf("ecs-svc(%d)", ecsServicesCount))

	if ecsServicesCount > 1 {
		m, _ = fullIntegrationApplyMsg(m, fullIntegrationKeyPress("j"))
	}
	secondService, secondServiceResults := fullIntegrationDescribeSelectedResource(t, &m, "ecs-svc")
	expectedSecondService := fullIntegrationExpectedRelatedCounts(t, clients, "ecs-svc", secondService)
	fullIntegrationAssertRelatedResults(t, expectedSecondService, secondServiceResults, "ECS service reached from cluster related list")
	fullIntegrationAssertRelatedCountsInView(t, m, expectedSecondService, "ECS service reached from cluster related list")
}

//go:build integration

package integration

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

func TestDemoFullIntegration_AllResourcesBaseline(t *testing.T) {
	clients := demo.NewServiceClients()
	expectedTopLevel := fullIntegrationCountExpectationsFromCounts(demofixtures.ExpectedTopLevelCounts())

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

	fullIntegrationRunAllResourceBaseline(t, clients, func() tui.Model {
		return fullIntegrationNewReadyModelWithClients(t, demo.DemoProfile, demo.DemoRegion, clients)
	}, fullIntegrationStaticCountResolver(expectedTopLevel))
}

func TestDemoFullIntegration_RelatedHopScenarios(t *testing.T) {
	clients := demo.NewServiceClients()
	expectedTopLevel := fullIntegrationCountExpectationsFromCounts(demofixtures.ExpectedTopLevelCounts())

	m := tui.New(
		demo.DemoProfile,
		demo.DemoRegion,
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})

	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "demo Init")
	var cmd tea.Cmd
	m, cmd = fullIntegrationApplyMsg(m, initMsg)
	availMsg := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
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

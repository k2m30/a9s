//go:build integration

package integration

import (
	"slices"
	"testing"
)

func TestDemoScenarioHarness_CommandRegionShowsDisabledFlash(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	scenario.Command("region")

	scenario.ExpectFlashContains("region switching is disabled in demo mode")
	scenario.ExpectNoAPIError()
}

func TestDemoScenarioHarness_OpenDetailAndFollowRelated(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	user := fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")

	scenario.OpenDetailResource("iam-user", user)
	scenario.ExpectCurrentResourceType("iam-user")
	scenario.ExpectCurrentResourceID(user.ID)
	scenario.ExpectRelatedRow("CloudTrail Events")

	scenario.FollowRelated("CloudTrail Events")

	scenario.ExpectNoAPIError()
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectFrameContains("ct-events(")
}

func TestDemoScenarioHarness_ListFilterAndSort(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	user := fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")

	scenario.OpenList("iam-user")
	scenario.ApplyFilter(user.ID)
	scenario.ExpectFrameContains("iam-user(1/")
	scenario.ExpectViewContains(user.ID)

	scenario.OpenSelectedDetail()
	scenario.ExpectCurrentResourceType("iam-user")
	scenario.ExpectCurrentResourceID(user.ID)

	scenario.Back()
	scenario.Back()

	ids := make([]string, 0, len(scenario.currentListResources))
	for _, res := range scenario.currentListResources {
		ids = append(ids, res.ID)
	}
	slices.Sort(ids)
	if len(ids) < 2 {
		t.Fatalf("demo iam-user list needs at least 2 resources to verify sort behavior")
	}

	scenario.SortByID()
	scenario.OpenSelectedDetail()
	scenario.ExpectCurrentResourceID(ids[0])

	scenario.Back()
	scenario.SortByID()
	scenario.OpenSelectedDetail()
	scenario.ExpectCurrentResourceID(ids[len(ids)-1])
}

func TestDemoScenarioHarness_DetailRelatedAndYAMLSearch(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	user := fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")

	scenario.OpenDetailResource("iam-user", user)

	scenario.StartSearch()
	scenario.Type(user.ID)
	scenario.ExpectHeaderContains("/" + user.ID)
	scenario.ConfirmInput()
	scenario.ExpectHeaderContains("matches")

	scenario.OpenYAML()
	scenario.ExpectFrameContains("yaml")
	scenario.ApplySearch(user.ID)
	scenario.ExpectHeaderContains("matches")

	scenario.Back()
	scenario.Back()
	scenario.Press("tab")
	scenario.StartSearch()
	scenario.Type("CloudTrail")
	scenario.ExpectHeaderContains("/CloudTrail")
	scenario.ConfirmInput()
	scenario.ExpectViewContains("CloudTrail Events")
	scenario.ExpectViewNotContains("IAM Policies (")
}

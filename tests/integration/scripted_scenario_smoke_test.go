//go:build integration

package integration

import "testing"

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

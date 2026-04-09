//go:build integration

package integration

import (
	"os"
	"strings"
	"testing"
)

func TestLiveScenario_Repro_RelatedCloudTrailRegionSwitchNextToken(t *testing.T) {
	profile := strings.TrimSpace(os.Getenv("A9S_REPRO_PROFILE"))
	userNeedle := strings.TrimSpace(os.Getenv("A9S_REPRO_IAM_USER"))
	targetRegion := strings.TrimSpace(os.Getenv("A9S_REPRO_REGION"))

	if profile == "" || userNeedle == "" || targetRegion == "" {
		t.Skip("set A9S_REPRO_PROFILE, A9S_REPRO_IAM_USER, and A9S_REPRO_REGION to run this focused live repro")
	}

	scenario := fullIntegrationNewLiveScenario(t, profile, "")
	user := fullIntegrationMustFindResourceByNameContains(t, scenario.clients, "iam-user", userNeedle)

	scenario.OpenDetailResource("iam-user", user)
	scenario.ExpectCurrentResourceType("iam-user")
	scenario.ExpectCurrentResourceID(user.ID)
	scenario.ExpectRelatedRow("CloudTrail Events")

	scenario.FollowRelated("CloudTrail Events")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")

	scenario.Command("region")
	scenario.ChooseRegion(targetRegion)
	scenario.ExpectCurrentListType("ct-events")

	scenario.LoadMore()
	scenario.ExpectNoAPIError()
	scenario.ExpectCurrentListType("ct-events")
}

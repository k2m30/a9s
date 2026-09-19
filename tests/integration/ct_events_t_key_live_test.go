//go:build integration

package integration

import (
	"os"
	"strings"
	"testing"
)

// Required env vars for the live CloudTrail tests:
//   A9S_CT_PROFILE — AWS profile name (test skipped if not set)
//   A9S_CT_REGION  — AWS region (optional; uses profile default if empty)

func ctLiveScenario(t *testing.T) *fullIntegrationScenario {
	t.Helper()
	profile := strings.TrimSpace(os.Getenv("A9S_CT_PROFILE"))
	region := strings.TrimSpace(os.Getenv("A9S_CT_REGION"))
	if profile == "" {
		t.Skip("set A9S_CT_PROFILE and optionally A9S_CT_REGION to run live CloudTrail tests")
	}
	return fullIntegrationNewLiveScenario(t, profile, region)
}

func TestLiveScenario_TKey_EC2(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "ec2")

	scenario.OpenList("ec2")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

func TestLiveScenario_TKey_Lambda(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "lambda")

	scenario.OpenList("lambda")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

func TestLiveScenario_TKey_S3(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "s3")

	scenario.OpenList("s3")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

func TestLiveScenario_TKey_IAMUser(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")

	scenario.OpenList("iam-user")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

func TestLiveScenario_TKey_RDS(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "dbi")

	scenario.OpenList("dbi")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

func TestLiveScenario_RelatedCT_EC2Detail(t *testing.T) {
	scenario := ctLiveScenario(t)
	ec2 := fullIntegrationMustFindAnyResource(t, scenario.clients, "ec2")

	scenario.OpenDetailResource("ec2", ec2)
	scenario.ExpectRelatedRow("CloudTrail Events")
	scenario.FollowRelated("CloudTrail Events")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

// Back pops the view stack without re-emitting ResourcesLoadedMsg, so the
// frame, not scenario.currentListType, shows the return.
func TestLiveScenario_TKey_EscReturns(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "ec2")

	scenario.OpenList("ec2")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.Back()
	scenario.ExpectFrameContains("ec2(")
}

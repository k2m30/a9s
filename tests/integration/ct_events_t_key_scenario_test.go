//go:build integration

package integration

import (
	"testing"
)

func TestDemoScenario_TKey_EC2List(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("ec2")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

func TestDemoScenario_TKey_LambdaList(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("lambda")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

func TestDemoScenario_TKey_S3List(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("s3")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

func TestDemoScenario_TKey_IAMUserDetail(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	user := fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")
	scenario.OpenDetailResource("iam-user", user)
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

func TestDemoScenario_TKey_NoopOnCtEvents(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("ct-events")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
}

func TestDemoScenario_TKey_NoopOnMainMenu(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.Press("t")
	scenario.ExpectFrameContains("a9s")
}

func TestDemoScenario_RelatedCloudTrail_EC2Detail(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	ec2 := fullIntegrationMustFindAnyResource(t, scenario.clients, "ec2")
	scenario.OpenDetailResource("ec2", ec2)
	scenario.ExpectRelatedRow("CloudTrail Events")
	scenario.FollowRelated("CloudTrail Events")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

// Back pops the view stack without re-emitting ResourcesLoadedMsg, so the
// frame, not scenario.currentListType, shows the return.
func TestDemoScenario_TKey_EscReturns(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("ec2")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.Back()
	scenario.ExpectFrameContains("ec2(")
}

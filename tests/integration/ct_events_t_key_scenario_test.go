//go:build integration

package integration

import (
	"testing"
)

// ct_events_t_key_scenario_test.go — Issue #247: CloudTrail t key integration scenarios.
//
// These tests verify the full pipeline in demo mode:
//   t key → filter construction → navigation → ct-events list loaded.

// TestDemoScenario_TKey_EC2List verifies that pressing t on an EC2 resource
// list navigates to the ct-events list.
func TestDemoScenario_TKey_EC2List(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("ec2")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

// TestDemoScenario_TKey_LambdaList verifies that pressing t on a Lambda
// resource list navigates to the ct-events list.
func TestDemoScenario_TKey_LambdaList(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("lambda")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

// TestDemoScenario_TKey_S3List verifies that pressing t on an S3 resource
// list navigates to the ct-events list.
func TestDemoScenario_TKey_S3List(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("s3")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

// TestDemoScenario_TKey_IAMUserDetail verifies that pressing t while viewing an
// IAM User detail panel navigates to the ct-events list.
func TestDemoScenario_TKey_IAMUserDetail(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	user := fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")
	scenario.OpenDetailResource("iam-user", user)
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

// TestDemoScenario_TKey_NoopOnCtEvents verifies that pressing t while already
// on the ct-events list is a no-op (no recursive navigation).
func TestDemoScenario_TKey_NoopOnCtEvents(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("ct-events")
	scenario.Press("t")
	// Should still be on ct-events, not navigated away
	scenario.ExpectCurrentListType("ct-events")
}

// TestDemoScenario_TKey_NoopOnMainMenu verifies that pressing t on the main
// menu is a no-op and does not navigate away.
func TestDemoScenario_TKey_NoopOnMainMenu(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.Press("t")
	// Should still be on main menu
	scenario.ExpectFrameContains("a9s")
}

// TestDemoScenario_RelatedCloudTrail_EC2Detail verifies that the CloudTrail
// Events related row appears on an EC2 detail panel and navigating it opens
// the ct-events list.
func TestDemoScenario_RelatedCloudTrail_EC2Detail(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	ec2 := fullIntegrationMustFindAnyResource(t, scenario.clients, "ec2")
	scenario.OpenDetailResource("ec2", ec2)
	scenario.ExpectRelatedRow("CloudTrail Events")
	scenario.FollowRelated("CloudTrail Events")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

// TestDemoScenario_TKey_EscReturns verifies that pressing Esc after t-key
// navigation from an EC2 list returns to the EC2 list.
// Note: Back (Esc) pops the view stack without re-emitting ResourcesLoadedMsg,
// so we check the rendered frame rather than scenario.currentListType.
func TestDemoScenario_TKey_EscReturns(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	scenario.OpenList("ec2")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.Back()
	scenario.ExpectFrameContains("ec2(")
}

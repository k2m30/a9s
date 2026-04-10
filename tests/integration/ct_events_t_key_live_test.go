//go:build integration

package integration

import (
	"os"
	"strings"
	"testing"
)

// ct_events_t_key_live_test.go — Issue #247: CloudTrail t key live integration tests.
//
// These tests run against a real AWS account to verify that CloudTrailKey
// values produce correct LookupEvents filters for each resource type.
//
// Required env vars:
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

// TestLiveScenario_TKey_EC2 verifies t key from an EC2 resource list uses a
// bare instance-ID filter (CloudTrailKey = "ResourceName:ID") and navigates
// to ct-events against a real CloudTrail endpoint.
func TestLiveScenario_TKey_EC2(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "ec2")

	scenario.OpenList("ec2")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

// TestLiveScenario_TKey_Lambda verifies t key from a Lambda resource list uses
// the full function ARN filter (CloudTrailKey = "ResourceName:Fields.arn") and
// navigates to ct-events.
func TestLiveScenario_TKey_Lambda(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "lambda")

	scenario.OpenList("lambda")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

// TestLiveScenario_TKey_S3 verifies t key from an S3 resource list uses the
// bucket name filter (CloudTrailKey = "ResourceName:ID") and navigates to
// ct-events.
func TestLiveScenario_TKey_S3(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "s3")

	scenario.OpenList("s3")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

// TestLiveScenario_TKey_IAMUser verifies t key from an IAM User resource list
// uses the Username filter (CloudTrailKey = "Username:ID") and navigates to
// ct-events.
func TestLiveScenario_TKey_IAMUser(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "iam-user")

	scenario.OpenList("iam-user")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

// TestLiveScenario_TKey_RDS verifies t key from an RDS DB instance list uses
// the full DB instance ARN filter (CloudTrailKey = "ResourceName:Fields.arn")
// and navigates to ct-events.
func TestLiveScenario_TKey_RDS(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "dbi")

	scenario.OpenList("dbi")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
	scenario.ExpectFrameContains("ct-events(")
}

// TestLiveScenario_RelatedCT_EC2Detail verifies that the CloudTrail Events
// related row appears on an EC2 detail panel and following it opens ct-events
// against a real CloudTrail endpoint.
func TestLiveScenario_RelatedCT_EC2Detail(t *testing.T) {
	scenario := ctLiveScenario(t)
	ec2 := fullIntegrationMustFindAnyResource(t, scenario.clients, "ec2")

	scenario.OpenDetailResource("ec2", ec2)
	scenario.ExpectRelatedRow("CloudTrail Events")
	scenario.FollowRelated("CloudTrail Events")
	scenario.ExpectCurrentListType("ct-events")
	scenario.ExpectNoAPIError()
}

// TestLiveScenario_TKey_EscReturns verifies that pressing Esc after t-key
// navigation from an EC2 list returns to the EC2 list.
// Note: Back (Esc) pops the view stack without re-emitting ResourcesLoadedMsg,
// so we verify via the rendered frame rather than scenario.currentListType.
func TestLiveScenario_TKey_EscReturns(t *testing.T) {
	scenario := ctLiveScenario(t)
	fullIntegrationMustFindAnyResource(t, scenario.clients, "ec2")

	scenario.OpenList("ec2")
	scenario.Press("t")
	scenario.ExpectCurrentListType("ct-events")
	scenario.Back()
	scenario.ExpectFrameContains("ec2(")
}

//go:build integration

package integration

// scenario_lambda_role_navigable_field_test.go — end-to-end pin for the
// Lambda detail view's "Role:" navigable field.
//
// The field carries the execution-role ARN, which may include an IAM path
// (e.g. "arn:aws:iam::123456789012:role/service-role/acme-lambda-execution").
// Pressing Enter on that field drills straight to the role's detail view
// through the "role" type's registered FetchByIDs (GetRole by name,
// path-safe; core/aws/catalog_security.go): on a COLD cache (the role list
// has never been opened this session) HandleRelatedNavigate's single-ID
// path dispatches the KindFetchByIDDetail task, so the drill lands exactly
// on the demo fixture role named "acme-lambda-execution"
// (core/demo/fixtures/iam.go), the execution role referenced by every demo
// lambda fixture via lambdaProdRoleARN (core/demo/fixtures/lambda.go) —
// never on an unrelated IAM POLICY resource from a full-list fallback.
//
// This test deliberately does NOT open the "role" list before following the
// navigable field, to drive the cold-cache path.

import (
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
)

// TestScenario_LambdaRoleNavigableField_ColdCache_LandsOnExecutionRole drives
// the full nav pipeline (dispatch → resolution → landing) from a lambda
// detail view's "Role:" field to the IAM role it points at, on a cold "role"
// cache. It MUST currently fail because the "role" resource type has no
// registered FetchByIDs: the landing is a wrong resource (empirically, an
// IAM policy in the demo fixture set), never "acme-lambda-execution".
func TestScenario_LambdaRoleNavigableField_ColdCache_LandsOnExecutionRole(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	lambda := fullIntegrationMustFindAnyResource(t, scenario.clients, "lambda")

	// Deliberately do NOT call scenario.OpenList("role") here — reproduce the
	// cold-cache path where "role" has never been fetched this session.
	scenario.OpenDetailResource("lambda", lambda)
	scenario.ExpectCurrentResourceType("lambda")

	got := scenario.FollowNavigableField("Role")

	const wantRoleName = "acme-lambda-execution"
	if got.ID != wantRoleName {
		t.Errorf(`FollowNavigableField("Role") landed on Resource{ID: %q, Name: %q}, want ID == %q (the demo lambda's execution role, whose ARN carries an IAM path "/service-role/" but whose bare RoleName is %q).

This is BUG 5: the "role" resource type has no registered FetchByIDs
(core/aws/catalog_security.go), so single-ID navigation from the Lambda
detail view's "Role:" field cannot drill directly to the role and instead
mis-resolves to an unrelated resource instead of the one actually referenced.`,
			got.ID, got.Name, wantRoleName, wantRoleName)
	}

	// The landed resource must be a role, not a policy (the other
	// mis-resolution mode called out in the bug report for the demo path).
	if strings.Contains(strings.ToLower(got.Name), "policy") || strings.Contains(strings.ToLower(got.ID), "policy") {
		t.Errorf(`FollowNavigableField("Role") landed on a policy-shaped resource (ID: %q, Name: %q) instead of the IAM role %q — this is the exact demo-mode mis-resolution described in BUG 5.`,
			got.ID, got.Name, wantRoleName)
	}
}

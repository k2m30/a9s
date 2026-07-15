package unit

// aws_iam_role_fetch_by_ids_test.go — RED test for BUG 5: the Lambda detail
// view's `Role:` navigable field extracts the bare role name correctly (via
// arnLastSlashSegment, even when the ARN carries an IAM path like
// "arn:aws:iam::...:role/service-role/acme-lambda-execution"), but single-ID
// navigation to "role" mis-resolves because the "role" resource type has no
// registered FetchByIDs helper — unlike its siblings "policy" and "iam-user",
// which both register one (see internal/aws/catalog_security.go).
//
// Without a FetchByIDs, HandleRelatedNavigate's cold-cache single-ID path
// (internal/runtime/handlers_related.go, NavigationKindFilteredList branch)
// falls through to KindFetchResources — a full, unfiltered "role" list fetch —
// instead of KindFetchByIDDetail. The first role in that list (arbitrary sort
// order) lands in the detail view instead of the role the user actually
// pivoted from. This is a "wrong resource" bug, not merely a missing feature.
//
// Fix direction (confirmed, not yet implemented): register a FetchByIDs for
// "role" (GetRole by name, path-safe) mirroring the "policy" registration at
// internal/aws/catalog_security.go:141.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"

	_ "github.com/k2m30/a9s/v3/core/aws" // registers the security-catalog resource types, incl. "role" and "policy"
)

// TestGetFetchByIDs_Role_IsRegistered pins the fix target directly: the
// "role" resource type must have a non-nil FetchByIDs helper registered in
// the catalog, exactly like its siblings "policy" and "iam-user".
//
// Currently RED: internal/aws/catalog_security.go's "IAM Roles" ResourceTypeDef
// (ShortName: "role") has no FetchByIDs field set, so resource.GetFetchByIDs
// returns nil for "role" while it returns non-nil for "policy" and "iam-user".
func TestGetFetchByIDs_Role_IsRegistered(t *testing.T) {
	got := resource.GetFetchByIDs("role")
	if got == nil {
		t.Fatalf(`resource.GetFetchByIDs("role") = nil, want non-nil.

BUG 5: the "role" resource type has no registered FetchByIDs. This means
single-ID navigation to a role (e.g. from the Lambda detail view's "Role:"
navigable field) cannot drill directly to the role's detail view on a cold
cache — it falls back to fetching the full unfiltered role list, landing on
an arbitrary role instead of the one the user pivoted from.

Fix direction: register a FetchByIDs func on the "role" ResourceTypeDef in
internal/aws/catalog_security.go, mirroring the "policy" registration
(same file, ~line 141), backed by IAM GetRole (path-safe: Resource.ID is
the bare RoleName, matching internal/aws/iam_roles.go's FetchIAMRolesPage
output shape).`)
	}
}

// TestGetFetchByIDs_Role_RegisteredLikeSiblingPolicy cross-checks that
// "role" is the ODD ONE OUT relative to its IAM sibling "policy", which
// already has FetchByIDs registered (internal/aws/catalog_security.go:141).
// This pins the asymmetry that root-causes BUG 5 — if this test ever passes
// by "policy" losing its registration instead of "role" gaining one, that is
// itself a regression worth catching.
//
// CORRECTION vs. the original bug report: as of this test's authoring,
// "iam-user" does NOT have a registered FetchByIDs either (only "policy"
// does — verified directly against internal/aws/catalog_security.go, which
// has exactly one `FetchByIDs:` field among the four IAM/security
// ResourceTypeDefs). The bug-5 dispatch's claim that "iam-user" already has
// one is not correct against current source, so this test only asserts the
// sibling that verifiably has it ("policy"), to avoid encoding a false
// premise into a supposedly-passing assertion.
func TestGetFetchByIDs_Role_RegisteredLikeSiblingPolicy(t *testing.T) {
	if resource.GetFetchByIDs("policy") == nil {
		t.Fatalf(`resource.GetFetchByIDs("policy") = nil, want non-nil (sibling registration regressed)`)
	}
	if resource.GetFetchByIDs("role") == nil {
		t.Fatalf(`resource.GetFetchByIDs("role") = nil, want non-nil — "role" must be registered exactly like its sibling "policy". See BUG 5.`)
	}
}

// NOTE on invoking the registered role FetchByIDs with a path-carrying ARN
// (e.g. asserting a fetch for ["acme-lambda-execution"] returns exactly one
// resource with ID == "acme-lambda-execution", including the case where the
// role's ARN carries "/service-role/" but RoleName is the bare
// "acme-lambda-execution"):
//
// This deeper assertion is deliberately NOT included here. As of this test's
// authoring, internal/aws/iam_roles.go has no FetchRoleByIDs-shaped function
// to invoke (only FetchIAMRolesPage, which paginates ALL roles, exists), and
// internal/aws/iam_interfaces.go's IAMGetRoleAPI (GetRole) is declared but not
// yet part of the aggregate IAMAPI interface consumed by ServiceClients.IAM —
// i.e. the fetcher this test would need to call is exactly what the coder's
// fix is expected to add. Once internal/aws/catalog_security.go registers
// FetchByIDs for "role", a follow-up test should extend
// TestGetFetchByIDs_Role_IsRegistered (or add a sibling test) to build a fake
// implementing IAMGetRoleAPI (mirroring the iamGroupFake pattern in
// tests/unit/aws_iam_group_enricher_test.go), invoke the registered
// FetchByIDs with []string{"acme-lambda-execution"}, and assert:
//   - exactly one resource.Resource is returned
//   - its ID == "acme-lambda-execution" (bare RoleName, no path)
//   - this holds even when the fake GetRole's Role.Arn carries
//     ".../role/service-role/acme-lambda-execution" (path-safe: a9s must key
//     off RoleName, not the ARN's path-bearing form).

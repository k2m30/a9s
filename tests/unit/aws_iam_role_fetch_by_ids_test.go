package unit

// aws_iam_role_fetch_by_ids_test.go — the "role" resource type registers a
// FetchByIDs helper (GetRole by name, path-safe), like its siblings "policy"
// and "iam-user" (core/aws/catalog_security.go). The Lambda detail view's
// `Role:` navigable field extracts the bare role name via
// arnLastSlashSegment even when the ARN carries an IAM path like
// "arn:aws:iam::...:role/service-role/acme-lambda-execution"; without a
// FetchByIDs, HandleRelatedNavigate's cold-cache single-ID path
// (core/runtime/handlers_related.go, NavigationKindFilteredList branch)
// would fall through to KindFetchResources — a full, unfiltered "role" list
// fetch — and the first role in that list (arbitrary sort order) would land
// in the detail view instead of the role the user pivoted from.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"

	_ "github.com/k2m30/a9s/v3/core/aws" // registers the security-catalog resource types, incl. "role" and "policy"
)

// TestGetFetchByIDs_Role_RegisteredLikeSiblingPolicy cross-checks "role"
// against its IAM sibling "policy" (core/aws/catalog_security.go): both
// register FetchByIDs. Asserting the sibling too means "policy" losing its
// registration is caught as a regression rather than read as parity.
func TestGetFetchByIDs_Role_RegisteredLikeSiblingPolicy(t *testing.T) {
	if resource.GetFetchByIDs("policy") == nil {
		t.Fatalf(`resource.GetFetchByIDs("policy") = nil, want non-nil (sibling registration regressed)`)
	}
	if resource.GetFetchByIDs("role") == nil {
		t.Fatalf(`resource.GetFetchByIDs("role") = nil, want non-nil — "role" must be registered exactly like its sibling "policy". See BUG 5.`)
	}
}

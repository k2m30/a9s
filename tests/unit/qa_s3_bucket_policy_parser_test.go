package unit_test

// qa_s3_bucket_policy_parser_test.go — edge-case coverage for
// extractBucketPolicyAWSPrincipals and the checkS3Role error paths.
//
// Covers the JSON-shape variation that real bucket policies exhibit:
//   - malformed JSON (invalid policy) → no principals
//   - wildcard Principal (`"*"`) at statement-root → no role ARN
//   - service principal ({"Service": ...}) → skipped
//   - mixed AWS principal list (role ARN + user ARN + account root) →
//     only role ARNs survive
//   - Principal.AWS as []string with wildcards and non-ARN strings
//   - role ARN with service-role path (arn:.../role/service-role/Name)
//     resolves to the bare role name
//
// These directly exercise the branches the coverage analyzer flagged
// as missing: JSON-decode error return and Principal-shape handling.

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// s3FakeClientsWithPolicies returns a ServiceClients whose S3 fake has
// BucketPolicies overridden with the per-test map. Enables exercising
// the bucket-policy parser branches without mutating shared fixtures.
func s3FakeClientsWithPolicies(policies map[string]string) *awsclient.ServiceClients {
	fix := fixtures.NewS3Fixtures()
	fix.BucketPolicies = policies
	return &awsclient.ServiceClients{S3: fakes.NewS3FromFixtures(fix)}
}

// TestS3_Role_MalformedPolicyJSON_Count0 pins the error branch in
// extractBucketPolicyAWSPrincipals: a policy that isn't valid JSON
// must surface as Count=0, not a crash and not -1 (the call itself
// succeeded; the parse didn't).
func TestS3_Role_MalformedPolicyJSON_Count0(t *testing.T) {
	bucket := "mal-json-" + t.Name()
	clients := s3FakeClientsWithPolicies(map[string]string{
		bucket: `{"Version":"2012-10-17","Statement":[{NOT JSON`,
	})
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "any-role", Name: "any-role"}},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), clients, emptyBucketResource(bucket), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 — malformed bucket-policy JSON must parse to zero principals, not fail the checker",
			result.Count)
	}
}

// TestS3_Role_WildcardPrincipal_Count0 guards that Principal:"*" at
// statement root (not as map) is skipped — it's not an IAM role ARN
// and must not produce a false positive.
func TestS3_Role_WildcardPrincipal_Count0(t *testing.T) {
	bucket := "wildcard-principal-" + t.Name()
	clients := s3FakeClientsWithPolicies(map[string]string{
		bucket: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::` + bucket + `/*"}]}`,
	})
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "any-role", Name: "any-role"}},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), clients, emptyBucketResource(bucket), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 — Principal:\"*\" at statement root must not resolve to any role", result.Count)
	}
}

// TestS3_Role_ServicePrincipalOnly_Count0 guards that a policy whose
// only principals are service principals ({"Service": ...}) surfaces
// zero — we don't resolve service principals as role entries.
func TestS3_Role_ServicePrincipalOnly_Count0(t *testing.T) {
	bucket := "service-principal-" + t.Name()
	clients := s3FakeClientsWithPolicies(map[string]string{
		bucket: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"cloudfront.amazonaws.com"},"Action":"s3:GetObject","Resource":"arn:aws:s3:::` + bucket + `/*"}]}`,
	})
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "any-role", Name: "any-role"}},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), clients, emptyBucketResource(bucket), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 — service principals are not IAM roles and must not match", result.Count)
	}
}

// TestS3_Role_MixedAWSPrincipalList_FiltersToRolesOnly exercises the
// []any branch of Principal.AWS with a realistic mix: one role ARN, one
// user ARN, one account-root ARN, and "*". Only the role ARN should
// resolve, and only if the role is present in the local cache.
func TestS3_Role_MixedAWSPrincipalList_FiltersToRolesOnly(t *testing.T) {
	bucket := "mixed-principals-" + t.Name()
	clients := s3FakeClientsWithPolicies(map[string]string{
		bucket: `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": [
          "arn:aws:iam::123456789012:role/target-role",
          "arn:aws:iam::123456789012:user/some-user",
          "arn:aws:iam::123456789012:root",
          "*"
        ]
      },
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::` + bucket + `/*"
    }
  ]
}`,
	})
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "target-role", Name: "target-role"},
				{ID: "some-user", Name: "some-user"}, // present but not a role — must not match
			},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), clients, emptyBucketResource(bucket), cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 — only the one role ARN in a mixed principal list should resolve", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "target-role" {
		t.Errorf("ResourceIDs = %v, want [target-role] — user/root/wildcard principals must not leak into the role pivot",
			result.ResourceIDs)
	}
}

// TestS3_Role_RoleARNNotInCache_DroppedFromResults pins the
// "can't navigate" rule: a bucket policy may name a cross-account or
// recently-deleted role; those don't resolve in the local `role` cache
// and must be dropped rather than surfaced as ghost rows.
func TestS3_Role_RoleARNNotInCache_DroppedFromResults(t *testing.T) {
	bucket := "unresolvable-role-" + t.Name()
	clients := s3FakeClientsWithPolicies(map[string]string{
		bucket: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::999999999999:role/cross-account-ghost"},"Action":"s3:GetObject","Resource":"arn:aws:s3:::` + bucket + `/*"}]}`,
	})
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "local-role", Name: "local-role"}},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), clients, emptyBucketResource(bucket), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 — a role ARN that doesn't resolve in the local cache must be dropped, not surfaced",
			result.Count)
	}
}

// TestS3_Role_ServiceRolePath_StripsToBareName pins the path-stripping
// behavior of roleNameFromARN when the role ARN includes a service-role
// path segment (arn:.../role/service-role/Name) — the local cache keys
// by bare name, so the match must still succeed.
func TestS3_Role_ServiceRolePath_StripsToBareName(t *testing.T) {
	bucket := "service-role-path-" + t.Name()
	clients := s3FakeClientsWithPolicies(map[string]string{
		bucket: `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/service-role/lambda-s3-reader"},"Action":"s3:GetObject","Resource":"arn:aws:s3:::` + bucket + `/*"}]}`,
	})
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "lambda-s3-reader", Name: "lambda-s3-reader"}},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), clients, emptyBucketResource(bucket), cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 — role ARN under /service-role/<name> path must resolve to bare name %q",
			result.Count, "lambda-s3-reader")
	}
}

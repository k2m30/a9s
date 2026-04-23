package unit_test

// qa_s3_related_review_findings_test.go — reveal tests for three external
// review findings on the s3 related-panel enrichment landed with commit
// e6dfbc9. Each test pins the spec-correct join and MUST pass against
// well-formed AWS-realistic fixture data.
//
// P1 r53 — spec §2: "record name matches this bucket's Name (website-
//   endpoint convention requires bucket name == FQDN)" AND "AliasTarget.
//   DNSName matches s3-website-<region>.amazonaws.com.". Current checker
//   substring-matches the DNSName (bucket.s3) — an unrealistic join that
//   silently fails in real AWS.
//
// P2 role — spec §2: "parse the JSON policy document for Statement[].
//   Principal.AWS entries matching IAM role ARNs" from s3:GetBucketPolicy.
//   Current checker walks the role's own policies (inverse direction) —
//   false positives on unrelated roles, false negatives on roles granted
//   access only by the bucket policy.
//
// P3 backup — Fields["resources"] is a comma-joined ARN list; the current
//   strings.Contains check over-matches when a bucket name is a prefix of
//   another bucket's name (prod vs prod-logs). Must match on token
//   boundaries.

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestS3_Related_R53_RealisticAliasResolves pins the spec contract:
// a hosted zone containing a record whose NAME equals the bucket's FQDN
// AND whose AliasTarget.DNSName is the regional s3-website endpoint
// (bucket-name NOT present in DNSName) must resolve Count≥1. This is the
// only shape AWS actually emits.
func TestS3_Related_R53_RealisticAliasResolves(t *testing.T) {
	bucket := "acme-website.example.com" // bucket-name == FQDN per AWS
	// Zone with one realistic S3-website alias.
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "/hostedzone/Z9999999999ABCDEFGHIJ",
					Name: "example.com.",
					Fields: map[string]string{
						// Realistic: record name is the FQDN/bucket; DNSName is the
						// regional endpoint with NO bucket segment.
						"s3website_alias_names": bucket,
						"alias_targets":      "s3-website-us-east-1.amazonaws.com.",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, resource.Resource{ID: bucket, Name: bucket}, cache)
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1 — a realistic Route 53 alias (record NAME == bucket FQDN, DNSName = regional endpoint) must resolve; spec §2 requires the join on record name",
			result.Count)
	}
}

// TestS3_Related_R53_BucketNameInDNSNameDoesNotMatch pins the inverse:
// a record whose DNSName happens to contain "bucket.s3" but whose NAME
// is unrelated must NOT match. (The old implementation matched this —
// producing false positives from any docs-site-or-ELB record that
// happened to mention the bucket's name anywhere in the DNS value.)
func TestS3_Related_R53_BucketNameInDNSNameDoesNotMatch(t *testing.T) {
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "/hostedzone/Z8888888888ABCDEFGHIJ",
					Name: "other.example.com.",
					Fields: map[string]string{
						// Record name doesn't match the bucket; DNSName has
						// "acme-website.s3" only because someone pointed at
						// the direct s3 URL (legitimate AWS config, but not
						// an alias to a bucket we own).
						"s3website_alias_names": "some-cname.other.example.com.",
						"alias_targets":      "acme-website.s3.us-east-1.amazonaws.com.",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, resource.Resource{ID: "acme-website", Name: "acme-website"}, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 — a record whose NAME does not equal the bucket FQDN must not match even if bucket name appears in the DNSName (old substring check was wrong)",
			result.Count)
	}
}

// TestS3_Related_Role_UsesBucketPolicyPrincipals pins the spec: the
// join is keyed off bucket policy Statement[].Principal.AWS role ARNs,
// NOT off role's own inline/attached policies that happen to mention
// the bucket. This test fails until checkS3Role is rewritten to call
// s3:GetBucketPolicy and parse the principals.
func TestS3_Related_Role_UsesBucketPolicyPrincipals(t *testing.T) {
	// The role in cache has NO reference to the bucket in its own
	// policy documents (policy_resources empty). Under the spec-correct
	// direction, the join comes from the BUCKET POLICY naming this role
	// as a principal — so the pivot must still resolve.
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "a9s-demo-s3-access-role",
					Name: "a9s-demo-s3-access-role",
					Fields: map[string]string{
						// Deliberately empty: role's own policies do not
						// mention this bucket. Bucket policy does (fixture).
						"policy_resources": "",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	result := checker(context.Background(), s3FakeClients(), healthyBucketResource(), cache)
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1 — the spec-defined s3→role join is bucket-policy-principal-to-role, not role-policy-resource-to-bucket. The current implementation uses the wrong direction and misses the canonical case",
			result.Count)
	}
}

// TestS3_Related_Role_UnrelatedRolePolicyMentioningBucket_DoesNotMatch
// pins the inverse: a role whose own policy mentions the bucket but is
// NOT a principal in the bucket policy must NOT be reported as related.
// This catches the false-positive class the reviewer called out.
func TestS3_Related_Role_UnrelatedRolePolicyMentioningBucket_DoesNotMatch(t *testing.T) {
	// We use a bucket that has NO bucket policy in the fixture, so no
	// role can be a principal. Any role mentioning the bucket in its own
	// policy must not match — the relationship lives on the bucket side.
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "role-that-mentions-other-bucket",
					Name: "role-that-mentions-other-bucket",
					Fields: map[string]string{
						// Role's policy resources mention the bucket, but
						// the bucket's own policy does not list this role —
						// no true relationship.
						"policy_resources": "arn:aws:s3:::" + fixtures.HealthyBucketName + "/*",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "role")
	// Use a bucket WITHOUT a bucket policy in fixtures (fake returns
	// NoSuchBucketPolicy) so the spec-correct implementation emits 0.
	src := emptyBucketResource("test-only-no-bucket-policy-" + t.Name())
	result := checker(context.Background(), s3FakeClients(), src, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 — a role whose own policy mentions the bucket must not match when the bucket policy does not list the role as a principal",
			result.Count)
	}
}

// TestS3_Related_Backup_PrefixCollisionDoesNotOvermatch pins the token-
// boundary fix. Current strings.Contains against the comma-joined
// resources field over-matches when the bucket name is a prefix of
// another ARN's bucket segment.
func TestS3_Related_Backup_PrefixCollisionDoesNotOvermatch(t *testing.T) {
	// The plan covers "prod-logs" but NOT "prod". A substring-based match
	// would incorrectly report this plan as protecting the "prod" bucket.
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "plan-prod-logs-only",
					Name: "plan-prod-logs-only",
					Fields: map[string]string{
						"resources": "arn:aws:s3:::prod-logs,arn:aws:efs:us-east-1:123:file-system/fs-other",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "backup")
	// Query for bucket "prod" — its ARN is a strict prefix of the plan's
	// "prod-logs" ARN. Must return 0.
	result := checker(context.Background(), nil, emptyBucketResource("prod"), cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 — a plan covering arn:aws:s3:::prod-logs must not match the \"prod\" bucket (prefix-collision over-match in the current implementation)",
			result.Count)
	}
}

// TestS3_Related_Backup_ExactMatchStillResolves guards that the fix
// doesn't break the canonical case — an exact ARN match must still
// resolve.
func TestS3_Related_Backup_ExactMatchStillResolves(t *testing.T) {
	bucket := "prod"
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "plan-prod-exact",
					Name: "plan-prod-exact",
					Fields: map[string]string{
						"resources": "arn:aws:s3:::prod,arn:aws:efs:us-east-1:123:file-system/fs-other",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "backup")
	result := checker(context.Background(), nil, emptyBucketResource(bucket), cache)
	if result.Count < 1 {
		t.Errorf("Count = %d, want ≥1 — exact ARN match must still resolve after the prefix-collision fix",
			result.Count)
	}
}

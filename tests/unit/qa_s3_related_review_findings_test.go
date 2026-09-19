package unit_test

// s3 related-panel joins over
// AWS-realistic fixture data (docs/resources/s3.md):
//   - r53: the record name matches the bucket's Name (the website-endpoint
//     convention requires bucket name == FQDN) and AliasTarget.DNSName is the
//     s3-website-<region> endpoint.
//   - role: role ARNs come from Statement[].Principal.AWS of
//     s3:GetBucketPolicy; a role's own policies that mention the bucket do not
//     relate it.
//   - backup: a plan's selection lists whole ARNs, so "prod" does not match
//     "prod-logs".

import (
	"context"
	"testing"

	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

// TestS3_Related_R53_RealisticAliasResolves: a hosted zone with a record whose
// NAME equals the bucket's FQDN and whose AliasTarget.DNSName is the regional
// s3-website endpoint (no bucket segment) resolves Count≥1. This is the only
// shape AWS emits.
func TestS3_Related_R53_RealisticAliasResolves(t *testing.T) {
	bucket := "acme-website.example.com" // bucket-name == FQDN per AWS
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   "/hostedzone/Z9999999999ABCDEFGHIJ",
					Name: "example.com.",
					Fields: map[string]string{
						// Record name is the FQDN/bucket; DNSName is the regional endpoint with no
						// bucket segment.
						"s3website_alias_names": bucket,
						"alias_targets":         "s3-website-us-east-1.amazonaws.com.",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, resource.Resource{ID: bucket, Name: bucket}, cache)
	if result.Count() < 1 {
		t.Errorf("Count = %d, want ≥1 — a realistic Route 53 alias (record NAME == bucket FQDN, DNSName = regional endpoint) must resolve; spec §2 requires the join on record name",
			result.Count())
	}
}

// A record whose DNSName contains "bucket.s3" but whose NAME is unrelated does
// not match.
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
						"alias_targets":         "acme-website.s3.us-east-1.amazonaws.com.",
					},
				},
			},
		},
	}
	checker := s3CheckerByTarget(t, "r53")
	result := checker(context.Background(), nil, resource.Resource{ID: "acme-website", Name: "acme-website"}, cache)
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 — a record whose NAME does not equal the bucket FQDN must not match even if bucket name appears in the DNSName (old substring check was wrong)",
			result.Count())
	}
}

// The role join is keyed off bucket policy Statement[].Principal.AWS role
// ARNs, not off the role's own inline/attached policies that mention the
// bucket.
func TestS3_Related_Role_UsesBucketPolicyPrincipals(t *testing.T) {
	// The cached role's own policy documents do not reference the bucket
	// (policy_resources empty); the bucket policy names the role as a principal.
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
	if result.Count() < 1 {
		t.Errorf("Count = %d, want ≥1 — the spec-defined s3→role join is bucket-policy-principal-to-role, not role-policy-resource-to-bucket. The current implementation uses the wrong direction and misses the canonical case",
			result.Count())
	}
}

// A role whose own policy mentions the bucket but that is not a principal in
// the bucket policy is not related.
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
	// This bucket has no bucket policy in the fixtures (the fake returns
	// NoSuchBucketPolicy), so the result is 0.
	src := emptyBucketResource("test-only-no-bucket-policy-" + t.Name())
	result := checker(context.Background(), s3FakeClients(), src, cache)
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 — a role whose own policy mentions the bucket must not match when the bucket policy does not list the role as a principal",
			result.Count())
	}
}

// The comma-joined resources field matches on token boundaries, so a bucket
// name that is a prefix of another ARN's bucket segment does not match.
func TestS3_Related_Backup_PrefixCollisionDoesNotOvermatch(t *testing.T) {
	// The plan covers "prod-logs" but NOT "prod". A substring-based match
	// would incorrectly report this plan as protecting the "prod" bucket.
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				unit.BackupPlanRow(t, "plan-prod-logs-only", backuptypes.BackupSelection{
					Resources: []string{"arn:aws:s3:::prod-logs", "arn:aws:efs:us-east-1:123:file-system/fs-other"},
				}),
			},
		},
	}
	checker := s3CheckerByTarget(t, "backup")
	// Query for bucket "prod" — its ARN is a strict prefix of the plan's
	// "prod-logs" ARN. Must return 0.
	result := checker(context.Background(), s3BackupSession(), emptyBucketResource("prod"), cache)
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 — a plan covering arn:aws:s3:::prod-logs must not match the \"prod\" bucket (prefix-collision over-match in the current implementation)",
			result.Count())
	}
}

// An exact ARN match resolves.
func TestS3_Related_Backup_ExactMatchStillResolves(t *testing.T) {
	bucket := "prod"
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				unit.BackupPlanRow(t, "plan-prod-exact", backuptypes.BackupSelection{
					Resources: []string{"arn:aws:s3:::prod", "arn:aws:efs:us-east-1:123:file-system/fs-other"},
				}),
			},
		},
	}
	checker := s3CheckerByTarget(t, "backup")
	result := checker(context.Background(), s3BackupSession(), emptyBucketResource(bucket), cache)
	if result.Count() < 1 {
		t.Errorf("Count = %d, want ≥1 — exact ARN match must still resolve after the prefix-collision fix",
			result.Count())
	}
}

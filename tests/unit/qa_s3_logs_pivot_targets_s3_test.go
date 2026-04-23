package unit_test

// qa_s3_logs_pivot_targets_s3_test.go — reveal tests for the S3 access-log
// pivot bug.
//
// Spec §2 (docs/resources/s3.md, "logs" subsection):
//   "S3 server-access logs are delivered to another S3 bucket, not CloudWatch
//    Logs — the `logs` panel entry is for buckets *receiving* this bucket's
//    access logs". a9s-devops (2026-04-20) reconfirmed: "S3 access logs never
//    go to CloudWatch; the entry means S3 access-log-destination bucket,
//    reachable via GetBucketLogging.LoggingEnabled.TargetBucket."
//
// Bug: the s3 registry emits TargetType="logs" (CloudWatch Log Groups) with
// the destination S3 bucket NAME as the navigation ID. Because a CloudWatch
// log group will never match an S3 bucket name, clicking the "Log Groups"
// pivot row lands the operator on an empty list.
//
// Correct shape: the pivot must target `s3` (the destination bucket is an S3
// bucket) with a DisplayName that names what's on the other side, so the
// operator understands the link is cross-bucket, not cross-service.

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestS3_AccessLogPivot_TargetsS3BucketNotCloudWatch asserts the pivot is
// registered against the `s3` target type (matching the actual destination
// resource kind), NOT `logs` (CloudWatch log groups, which S3 server-access
// logs are never delivered to).
func TestS3_AccessLogPivot_TargetsS3BucketNotCloudWatch(t *testing.T) {
	defs := resource.GetRelated("s3")

	// The access-log pivot MUST exist and target s3 — any other target
	// (e.g. the historical "logs"/CloudWatch) leads the operator to an
	// empty list because the destination is another S3 bucket.
	var found *resource.RelatedDef
	for i, def := range defs {
		// Identify the access-log pivot by DisplayName fragment so the
		// test is robust to a non-final name choice while still pinning
		// the contract: whatever its name, it must target `s3`.
		if def.DisplayName == "Access Log Bucket" {
			found = &defs[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected an s3 pivot with DisplayName %q; got: %v",
			"Access Log Bucket", displayNames(defs))
	}
	if found.TargetType != "s3" {
		t.Errorf("TargetType = %q, want %q — S3 access logs go to another S3 bucket, not CloudWatch Logs",
			found.TargetType, "s3")
	}

	// A stale "Log Groups" / TargetType="logs" registration must NOT
	// coexist: that would double-list the pivot and keep the broken
	// navigation path alive.
	for _, def := range defs {
		if def.DisplayName == "Log Groups" {
			t.Errorf("stale DisplayName %q still registered for s3 — access-log pivot was renamed to %q",
				"Log Groups", "Access Log Bucket")
		}
		if def.TargetType == "logs" {
			t.Errorf("pivot with TargetType=%q must not be registered for s3 — access-log destination is an S3 bucket",
				"logs")
		}
	}
}

// TestS3_AccessLogChecker_ReturnsS3TargetType asserts the checker itself
// emits TargetType="s3" when it resolves a logging destination bucket — the
// runtime uses the checker's return TargetType to seed RelatedCheckResult,
// so a mismatch here would break navigation even if the registration were
// correct.
func TestS3_AccessLogChecker_ReturnsS3TargetType(t *testing.T) {
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("s3") {
		if def.DisplayName == "Access Log Bucket" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("access-log checker not found (pivot not registered under the expected DisplayName)")
	}

	result := checker(context.Background(), s3FakeClients(), healthyBucketResource(), nil)
	if result.TargetType != "s3" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "s3")
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (healthy bucket logs to %q)", result.Count, fixtures.LogsBucketName)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != fixtures.LogsBucketName {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs, fixtures.LogsBucketName)
	}
}

func displayNames(defs []resource.RelatedDef) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.DisplayName)
	}
	return out
}

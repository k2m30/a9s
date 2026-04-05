// issue234_fetcher_pagination_honesty_test.go — Tests for issue #234.
//
// Business rule: A fetcher registered via RegisterPaginated must stop after one
// API page and honestly report IsTruncated. Fetchers that internally loop ALL
// pages before returning violate this contract — they do unbounded work on cold
// start and cause related-view checkers to get dishonest IsTruncated=false.
//
// Three fetchers (eks, kms, ng) previously looped all pages internally while
// returning IsTruncated=false. They have been refactored to call the underlying
// AWS API with a page limit and pass the continuation token through.
//
// opensearch and trail are genuinely unpaginated AWS APIs (ListDomainNames and
// DescribeTrails return all results in one call). IsTruncated=false is honest
// for them — no tests needed.
//
// Positive controls: ec2, tg, s3 are genuinely paginated and must stay registered.
package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ─── Tests for previously-broken fetchers (must remain registered as paginated) ──

// TestContract_PaginatedFetcher_EKS_IsRegistered verifies the EKS cluster
// fetcher is registered as paginated. After the fix, it calls ListClusters
// with MaxResults and stops after one page.
func TestContract_PaginatedFetcher_EKS_IsRegistered(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("eks")
	if fetcher == nil {
		t.Error("eks fetcher MUST be registered as paginated — " +
			"ListClusters supports MaxResults + NextToken for true single-page fetches.")
	}
}

// TestContract_PaginatedFetcher_KMS_IsRegistered verifies the KMS key fetcher
// is registered as paginated. After the fix, it calls ListKeys with Limit and
// stops after one page.
func TestContract_PaginatedFetcher_KMS_IsRegistered(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("kms")
	if fetcher == nil {
		t.Error("kms fetcher MUST be registered as paginated — " +
			"ListKeys supports Limit + Marker for true single-page fetches.")
	}
}

// TestContract_PaginatedFetcher_NodeGroups_IsRegistered verifies the node
// groups fetcher is registered as paginated. After the fix, it calls
// ListClusters and ListNodegroups with MaxResults and stops after one page.
func TestContract_PaginatedFetcher_NodeGroups_IsRegistered(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("ng")
	if fetcher == nil {
		t.Error("ng fetcher MUST be registered as paginated — " +
			"ListClusters and ListNodegroups support MaxResults for single-page fetches.")
	}
}

// ─── Positive controls: genuinely paginated fetchers must remain registered ──

// TestContract_PaginatedFetcher_EC2_IsRegistered verifies that the EC2
// instance fetcher stays registered. DescribeInstances supports MaxResults +
// NextToken for true single-page fetches.
func TestContract_PaginatedFetcher_EC2_IsRegistered(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("ec2")
	if fetcher == nil {
		t.Error("ec2 fetcher MUST be registered as paginated — " +
			"DescribeInstances supports MaxResults + NextToken for true single-page fetches.")
	}
}

// TestContract_PaginatedFetcher_TargetGroups_IsRegistered verifies that the
// target-group fetcher stays registered. DescribeTargetGroups supports
// PageSize + Marker for true single-page fetches.
func TestContract_PaginatedFetcher_TargetGroups_IsRegistered(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("tg")
	if fetcher == nil {
		t.Error("tg fetcher MUST be registered as paginated — " +
			"DescribeTargetGroups supports PageSize + Marker for true single-page fetches.")
	}
}

// TestContract_PaginatedFetcher_S3_IsRegistered verifies that the S3 bucket
// fetcher stays registered. ListBuckets supports MaxBuckets +
// ContinuationToken for true single-page fetches.
func TestContract_PaginatedFetcher_S3_IsRegistered(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("s3")
	if fetcher == nil {
		t.Error("s3 fetcher MUST be registered as paginated — " +
			"ListBuckets supports MaxBuckets + ContinuationToken for true single-page fetches.")
	}
}

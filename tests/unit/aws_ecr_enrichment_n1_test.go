package unit

// aws_ecr_enrichment_n1_test.go — Behavioral tests for the N+1 Wave-2
// EnrichECRRepository. The enricher issues ONE DescribeImages call per repo
// and reads ImageScanFindingsSummary.FindingSeverityCounts inline — no
// per-image DescribeImageScanFindings fan-out.
//
// This replaces aws_ecr_enrichment_test.go which tested the old
// ListImages → DescribeImageScanFindings architecture (up to 11N calls per
// repo; violated the wave-2 N+1 budget and caused ECR timeouts in prod).

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrsvc "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ecrDescribeImagesFake satisfies awsclient.ECRAPI and implements only
// DescribeImages — the single call path the new enricher uses.
type ecrDescribeImagesFake struct {
	awsclient.ECRAPI
	// detailsByRepo maps repositoryName → ImageDetails that DescribeImages
	// returns. Each ImageDetail's ImageScanFindingsSummary.FindingSeverityCounts
	// is what the enricher aggregates.
	detailsByRepo map[string][]ecrtypes.ImageDetail
	// errByRepo maps repositoryName → error (overrides the normal response).
	errByRepo    map[string]error
	callsPerRepo map[string]int
}

func (f *ecrDescribeImagesFake) DescribeImages(
	_ context.Context,
	in *ecrsvc.DescribeImagesInput,
	_ ...func(*ecrsvc.Options),
) (*ecrsvc.DescribeImagesOutput, error) {
	repo := ""
	if in != nil && in.RepositoryName != nil {
		repo = *in.RepositoryName
	}
	if f.callsPerRepo == nil {
		f.callsPerRepo = map[string]int{}
	}
	f.callsPerRepo[repo]++
	if err, ok := f.errByRepo[repo]; ok {
		return nil, err
	}
	return &ecrsvc.DescribeImagesOutput{ImageDetails: f.detailsByRepo[repo]}, nil
}

func ecrImageDetailWithCounts(repo string, counts map[string]int32) ecrtypes.ImageDetail {
	return ecrtypes.ImageDetail{
		RepositoryName: aws.String(repo),
		ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
			FindingSeverityCounts: counts,
		},
	}
}

func ecrRepoResourceN1(name string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"repository_name": name,
			"scan_on_push":    "true",
		},
	}
}

// TestEnrichECRRepository_N1_OneCallPerRepo pins the budget contract: exactly
// one DescribeImages call per repository, regardless of how many images the
// response carries. This is the regression test for the "11N calls" bug.
func TestEnrichECRRepository_N1_OneCallPerRepo(t *testing.T) {
	const repoA = "service-api"
	const repoB = "service-worker"

	fake := &ecrDescribeImagesFake{
		detailsByRepo: map[string][]ecrtypes.ImageDetail{
			repoA: {
				ecrImageDetailWithCounts(repoA, map[string]int32{
					string(ecrtypes.FindingSeverityCritical): 3,
				}),
				ecrImageDetailWithCounts(repoA, map[string]int32{
					string(ecrtypes.FindingSeverityHigh): 7,
				}),
				// 8 more images with empty summaries (would have been 8 extra
				// DescribeImageScanFindings calls under the old architecture —
				// must stay at 0 extra calls under the new one).
				ecrImageDetailWithCounts(repoA, nil),
				ecrImageDetailWithCounts(repoA, nil),
				ecrImageDetailWithCounts(repoA, nil),
				ecrImageDetailWithCounts(repoA, nil),
				ecrImageDetailWithCounts(repoA, nil),
				ecrImageDetailWithCounts(repoA, nil),
				ecrImageDetailWithCounts(repoA, nil),
				ecrImageDetailWithCounts(repoA, nil),
			},
			repoB: {},
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := []resource.Resource{ecrRepoResourceN1(repoA), ecrRepoResourceN1(repoB)}

	_, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := fake.callsPerRepo[repoA]; got != 1 {
		t.Errorf("DescribeImages calls for %q: got %d, want 1 (N+1 budget)", repoA, got)
	}
	if got := fake.callsPerRepo[repoB]; got != 1 {
		t.Errorf("DescribeImages calls for %q: got %d, want 1 (N+1 budget)", repoB, got)
	}
}

// TestEnrichECRRepository_N1_CriticalAggregatesAcrossImages verifies that
// CRITICAL counts aggregate across all images returned by the single
// DescribeImages call and surface as a "!" finding.
func TestEnrichECRRepository_N1_CriticalAggregatesAcrossImages(t *testing.T) {
	const repo = "repo-with-crits"
	fake := &ecrDescribeImagesFake{
		detailsByRepo: map[string][]ecrtypes.ImageDetail{
			repo: {
				ecrImageDetailWithCounts(repo, map[string]int32{
					string(ecrtypes.FindingSeverityCritical): 2,
				}),
				ecrImageDetailWithCounts(repo, map[string]int32{
					string(ecrtypes.FindingSeverityCritical): 1,
					string(ecrtypes.FindingSeverityHigh):     5,
				}),
			},
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := []resource.Resource{ecrRepoResourceN1(repo)}

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[repo]
	if !ok {
		t.Fatalf("expected finding for %q", repo)
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
	if got := result.FieldUpdates[repo]["critical_vulns"]; got != "3" {
		t.Errorf("critical_vulns = %q, want 3 (aggregate across both images)", got)
	}
	if got := result.FieldUpdates[repo]["high_vulns"]; got != "5" {
		t.Errorf("high_vulns = %q, want 5", got)
	}
	if got := result.FieldUpdates[repo]["images_scanned"]; got != "2" {
		t.Errorf("images_scanned = %q, want 2", got)
	}
}

// TestEnrichECRRepository_N1_HighOnlyEmitsTilde verifies that HIGH-only
// findings (no CRITICAL) are classified as "~" and do NOT bump IssueCount.
func TestEnrichECRRepository_N1_HighOnlyEmitsTilde(t *testing.T) {
	const repo = "repo-high-only"
	fake := &ecrDescribeImagesFake{
		detailsByRepo: map[string][]ecrtypes.ImageDetail{
			repo: {
				ecrImageDetailWithCounts(repo, map[string]int32{
					string(ecrtypes.FindingSeverityHigh): 3,
				}),
			},
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := []resource.Resource{ecrRepoResourceN1(repo)}

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[repo]
	if !ok {
		t.Fatalf("expected finding for %q", repo)
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (~ never bumps badge)", result.IssueCount)
	}
}

// TestEnrichECRRepository_N1_CleanRepoEmitsNoFinding verifies a repo whose
// image scan summary reports 0 CRITICAL and 0 HIGH emits no finding.
func TestEnrichECRRepository_N1_CleanRepoEmitsNoFinding(t *testing.T) {
	const repo = "clean-repo"
	fake := &ecrDescribeImagesFake{
		detailsByRepo: map[string][]ecrtypes.ImageDetail{
			repo: {
				ecrImageDetailWithCounts(repo, map[string]int32{
					string(ecrtypes.FindingSeverityCritical): 0,
					string(ecrtypes.FindingSeverityHigh):     0,
				}),
			},
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := []resource.Resource{ecrRepoResourceN1(repo)}

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[repo]; ok {
		t.Errorf("clean repo must not appear in findings; got: %v", result.Findings[repo])
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
	// FieldUpdates still set (even on clean repos) for render-path predictability.
	if got := result.FieldUpdates[repo]["critical_vulns"]; got != "0" {
		t.Errorf("critical_vulns = %q, want 0", got)
	}
}

// TestEnrichECRRepository_N1_EmptyRepoNoPanic verifies a repo with 0 images
// (DescribeImages returns an empty ImageDetails slice) does not panic and
// sets images_scanned=0.
func TestEnrichECRRepository_N1_EmptyRepoNoPanic(t *testing.T) {
	const repo = "empty-repo"
	fake := &ecrDescribeImagesFake{
		detailsByRepo: map[string][]ecrtypes.ImageDetail{repo: {}},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := []resource.Resource{ecrRepoResourceN1(repo)}

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FieldUpdates[repo]["images_scanned"] != "0" {
		t.Errorf("images_scanned = %q, want 0", result.FieldUpdates[repo]["images_scanned"])
	}
}

// TestEnrichECRRepository_N1_DescribeImagesErrorSurfaces verifies that a
// per-repo DescribeImages error aggregates into the returned composite
// error, sets Truncated=true, and populates TruncatedIDs for that repo.
func TestEnrichECRRepository_N1_DescribeImagesErrorSurfaces(t *testing.T) {
	const okRepo = "ok-repo"
	const errRepo = "err-repo"
	fake := &ecrDescribeImagesFake{
		detailsByRepo: map[string][]ecrtypes.ImageDetail{
			okRepo: {ecrImageDetailWithCounts(okRepo, map[string]int32{"CRITICAL": 1})},
		},
		errByRepo: map[string]error{
			errRepo: errors.New("simulated AccessDenied"),
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := []resource.Resource{ecrRepoResourceN1(okRepo), ecrRepoResourceN1(errRepo)}

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("expected composite error when one repo fails")
	}
	if !strings.Contains(err.Error(), errRepo) {
		t.Errorf("composite error must name the failing repo %q; got: %v", errRepo, err)
	}
	if !result.Truncated {
		t.Error("expected Truncated=true when any repo fails")
	}
	if !result.TruncatedIDs[errRepo] {
		t.Errorf("expected TruncatedIDs[%q]=true", errRepo)
	}
	// The successful repo still produces its finding (partial success preserved).
	if _, ok := result.Findings[okRepo]; !ok {
		t.Errorf("partial success: ok-repo finding must survive err-repo failure; got: %v", result.Findings)
	}
}

// TestEnrichECRRepository_N1_RespectsEnrichmentCap verifies that beyond the
// global EnrichmentCap, further repos are not processed and Truncated=true.
func TestEnrichECRRepository_N1_RespectsEnrichmentCap(t *testing.T) {
	count := awsclient.EnrichmentCap + 2
	resources := make([]resource.Resource, count)
	details := map[string][]ecrtypes.ImageDetail{}
	for i := range count {
		name := "cap-repo-" + strconv.Itoa(i)
		resources[i] = ecrRepoResourceN1(name)
		details[name] = []ecrtypes.ImageDetail{
			ecrImageDetailWithCounts(name, map[string]int32{"CRITICAL": 0, "HIGH": 0}),
		}
	}
	fake := &ecrDescribeImagesFake{detailsByRepo: details}
	clients := &awsclient.ServiceClients{ECR: fake}

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Errorf("Truncated must be true when len(resources)=%d > EnrichmentCap=%d",
			count, awsclient.EnrichmentCap)
	}
	// Calls made for repos beyond the cap must be zero.
	uncalled := 0
	for i := awsclient.EnrichmentCap; i < count; i++ {
		name := "cap-repo-" + strconv.Itoa(i)
		if fake.callsPerRepo[name] == 0 {
			uncalled++
		}
	}
	if uncalled == 0 {
		t.Errorf("expected at least one repo beyond EnrichmentCap to receive zero calls; all %d over-cap repos were called", count-awsclient.EnrichmentCap)
	}
}

// TestEnrichECRRepository_N1_NilClient verifies nil clients.ECR returns an
// empty-but-non-nil Findings map without panicking.
func TestEnrichECRRepository_N1_NilClient(t *testing.T) {
	resources := []resource.Resource{ecrRepoResourceN1("any-repo")}

	result, err := awsclient.EnrichECRRepository(context.Background(), &awsclient.ServiceClients{ECR: nil}, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil even with nil client")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings with nil client; got %d", len(result.Findings))
	}
}

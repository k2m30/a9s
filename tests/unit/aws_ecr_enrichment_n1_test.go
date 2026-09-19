package unit

// EnrichECRRepository issues one DescribeImages call per repo and reads
// ImageScanFindingsSummary.FindingSeverityCounts inline; a per-image
// DescribeImageScanFindings fan-out would cost up to 11N calls.

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrsvc "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type ecrDescribeImagesFake struct {
	awsclient.ECRAPI
	detailsByRepo map[string][]ecrtypes.ImageDetail
	errByRepo     map[string]error

	// mu guards callsPerRepo, which is written concurrently: the enricher
	// fans out DescribeImages calls per repo via core/aws.ForEachParallel
	// (EnrichmentParallelism goroutines).
	mu           sync.Mutex
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
	f.mu.Lock()
	if f.callsPerRepo == nil {
		f.callsPerRepo = map[string]int{}
	}
	f.callsPerRepo[repo]++
	f.mu.Unlock()
	if err, ok := f.errByRepo[repo]; ok {
		return nil, err
	}
	return &ecrsvc.DescribeImagesOutput{ImageDetails: f.detailsByRepo[repo]}, nil
}

func (f *ecrDescribeImagesFake) callsFor(repo string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callsPerRepo[repo]
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

	if got := fake.callsFor(repoA); got != 1 {
		t.Errorf("DescribeImages calls for %q: got %d, want 1 (N+1 budget)", repoA, got)
	}
	if got := fake.callsFor(repoB); got != 1 {
		t.Errorf("DescribeImages calls for %q: got %d, want 1 (N+1 budget)", repoB, got)
	}
}

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
	fs, ok := result.Findings[repo]
	if !ok {
		t.Fatalf("expected finding for %q", repo)
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
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
	fs, ok := result.Findings[repo]
	if !ok {
		t.Fatalf("expected finding for %q", repo)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
}

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
	// FieldUpdates still set (even on clean repos) for render-path predictability.
	if got := result.FieldUpdates[repo]["critical_vulns"]; got != "0" {
		t.Errorf("critical_vulns = %q, want 0", got)
	}
}

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
	if _, marked := result.TruncatedIDs[errRepo]; !marked {
		t.Errorf("expected TruncatedIDs[%q]=true", errRepo)
	}
	if _, ok := result.Findings[okRepo]; !ok {
		t.Errorf("partial success: ok-repo finding must survive err-repo failure; got: %v", result.Findings)
	}
}

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
	uncalled := 0
	for i := awsclient.EnrichmentCap; i < count; i++ {
		name := "cap-repo-" + strconv.Itoa(i)
		if fake.callsFor(name) == 0 {
			uncalled++
		}
	}
	if uncalled == 0 {
		t.Errorf("expected at least one repo beyond EnrichmentCap to receive zero calls; all %d over-cap repos were called", count-awsclient.EnrichmentCap)
	}
}

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

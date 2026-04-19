package unit

// aws_ecr_enrichment_test.go — Behavioral tests for the Wave 2 EnrichECRRepository
// implementation using ListImages → DescribeImageScanFindings per image.
//
// ── Coder note ─────────────────────────────────────────────────────────────────
// ECRListImagesAPI and its embedding into ECRAPI already exist in
// internal/aws/interfaces.go. The only coder action required is:
//
//  1. Add ListImages to internal/demo/fakes/ecr.go (ECRFake) so the demo fake
//     satisfies the updated ECRAPI interface.
//
//  2. Implement EnrichECRRepository in internal/aws/ecr_issue_enrichment.go:
//     ListImages (paginated) → cap at 10 per repo → DescribeImageScanFindings
//     per digest → aggregate CRITICAL+HIGH → emit findings + FieldUpdates.
//
// ── Contract assertions ─────────────────────────────────────────────────────────
//  1. Repo with CRITICAL>0 images → finding Severity "!" + FieldUpdates{critical_vulns, high_vulns, images_scanned}.
//  2. Repo with only HIGH>0 images → finding Severity "~" + IssueCount==0.
//  3. Repo with no CRITICAL or HIGH → no finding + FieldUpdates{critical_vulns:"0"}.
//  4. Empty repo (ListImages → []) → FieldUpdates{images_scanned:"0"}, no finding.
//  5. ListImages error for one repo → Truncated==true; other repos still processed.
//  6. DescribeImageScanFindings error for one image → image skipped; images_scanned reflects only successful calls.
//  7. 50 images returned → exactly 10 DescribeImageScanFindings calls (cap = 10 per repo).
//  8. 100 repos → Truncated==true; FieldUpdates count == EnrichmentCap.
//  9. clients.ECR == nil → empty findings result, no panic.
// 10. Paginated ListImages (two pages, 5 images each) → all 10 images scanned when cap >= 10.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrsvc "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ── Fakes ──────────────────────────────────────────────────────────────────────

// ecrEnrichFake implements ECRAPI for the ListImages-based Wave 2 enrichment tests.
// It embeds the aggregate interface and overrides only ListImages and
// DescribeImageScanFindings.
//
// listImagesByRepo maps repositoryName → paginated pages of ImageIdentifiers.
// listImagesErrByRepo maps repositoryName → error (returned instead of pages).
// scanCountsByDigest maps imageDigest → FindingSeverityCounts.
// scanErrByDigest maps imageDigest → error (returned instead of counts).
// listImagesCallsPerRepo counts how many ListImages calls were made per repo.
// scanCallDigests records which image digests DescribeImageScanFindings was called for.
type ecrEnrichFake struct {
	awsclient.ECRAPI
	listImagesByRepo       map[string][][]ecrtypes.ImageIdentifier // repo → pages
	listImagesErrByRepo    map[string]error
	scanCountsByDigest     map[string]map[string]int32 // digest → severity → count
	scanErrByDigest        map[string]error
	listImagesCallsPerRepo map[string]int
	scanCallDigests        []string
}

func (f *ecrEnrichFake) ListImages(
	_ context.Context,
	in *ecrsvc.ListImagesInput,
	_ ...func(*ecrsvc.Options),
) (*ecrsvc.ListImagesOutput, error) {
	repo := ""
	if in != nil && in.RepositoryName != nil {
		repo = *in.RepositoryName
	}
	if f.listImagesCallsPerRepo == nil {
		f.listImagesCallsPerRepo = map[string]int{}
	}
	f.listImagesCallsPerRepo[repo]++

	if f.listImagesErrByRepo != nil {
		if err, ok := f.listImagesErrByRepo[repo]; ok {
			return nil, err
		}
	}

	pages, ok := f.listImagesByRepo[repo]
	if !ok || len(pages) == 0 {
		return &ecrsvc.ListImagesOutput{}, nil
	}

	// Consume the first available page; model pagination via NextToken.
	callIndex := f.listImagesCallsPerRepo[repo] - 1
	if callIndex >= len(pages) {
		return &ecrsvc.ListImagesOutput{}, nil
	}
	page := pages[callIndex]
	var nextToken *string
	if callIndex+1 < len(pages) {
		tok := fmt.Sprintf("page-%d", callIndex+1)
		nextToken = &tok
	}
	return &ecrsvc.ListImagesOutput{
		ImageIds:  page,
		NextToken: nextToken,
	}, nil
}

func (f *ecrEnrichFake) DescribeImageScanFindings(
	_ context.Context,
	in *ecrsvc.DescribeImageScanFindingsInput,
	_ ...func(*ecrsvc.Options),
) (*ecrsvc.DescribeImageScanFindingsOutput, error) {
	digest := ""
	if in != nil && in.ImageId != nil && in.ImageId.ImageDigest != nil {
		digest = *in.ImageId.ImageDigest
	}
	f.scanCallDigests = append(f.scanCallDigests, digest)

	if f.scanErrByDigest != nil {
		if err, ok := f.scanErrByDigest[digest]; ok {
			return nil, err
		}
	}

	counts, ok := f.scanCountsByDigest[digest]
	if !ok {
		return &ecrsvc.DescribeImageScanFindingsOutput{}, nil
	}
	return &ecrsvc.DescribeImageScanFindingsOutput{
		ImageScanFindings: &ecrtypes.ImageScanFindings{
			FindingSeverityCounts: counts,
		},
	}, nil
}

// Compile-time check: ecrEnrichFake satisfies ECRAPI.
// This will fail to compile until ECRAPI embeds ECRListImagesAPI.
var _ awsclient.ECRAPI = (*ecrEnrichFake)(nil)

// ── Helpers ────────────────────────────────────────────────────────────────────

// ecrEnrichResources returns resource.Resource stubs for the given ECR repository names.
func ecrEnrichResources(names ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(names))
	for _, name := range names {
		res = append(res, resource.Resource{
			ID:   "arn:aws:ecr:us-east-1:123456789012:repository/" + name,
			Name: name,
			Fields: map[string]string{
				"repository_name": name,
				"uri":             "123456789012.dkr.ecr.us-east-1.amazonaws.com/" + name,
				"tag_mutability":  "MUTABLE",
				"scan_on_push":    "true",
				"created_at":      "2025-01-15 10:00",
			},
		})
	}
	return res
}

// ecrRepoID returns the resource ID for the given repository name (ARN form).
func ecrRepoID(name string) string {
	return "arn:aws:ecr:us-east-1:123456789012:repository/" + name
}

// makeImagePage returns a slice of ImageIdentifiers with synthetic digests.
// Digests are formatted as "sha256:<prefix>NNN" where NNN is a zero-padded index.
func makeImagePage(prefix string, count int, startIndex int) []ecrtypes.ImageIdentifier {
	ids := make([]ecrtypes.ImageIdentifier, 0, count)
	for i := 0; i < count; i++ {
		digest := fmt.Sprintf("sha256:%s%03d", prefix, startIndex+i)
		ids = append(ids, ecrtypes.ImageIdentifier{
			ImageDigest: aws.String(digest),
		})
	}
	return ids
}

// ── Tests ──────────────────────────────────────────────────────────────────────

// TestEnrichECRRepository_CriticalVulnerabilitiesEmitBangFinding verifies that
// when a repo has an image with CRITICAL>0 vulns, the enricher emits a finding
// with Severity "!" and correct FieldUpdates.
func TestEnrichECRRepository_CriticalVulnerabilitiesEmitBangFinding(t *testing.T) {
	const repo = "prod-api"
	repoID := ecrRepoID(repo)

	fake := &ecrEnrichFake{
		listImagesByRepo: map[string][][]ecrtypes.ImageIdentifier{
			repo: {
				{
					{ImageDigest: aws.String("sha256:aaa")},
					{ImageDigest: aws.String("sha256:bbb")},
				},
			},
		},
		scanCountsByDigest: map[string]map[string]int32{
			"sha256:aaa": {
				string(ecrtypes.FindingSeverityCritical): 2,
				string(ecrtypes.FindingSeverityHigh):     1,
			},
			"sha256:bbb": {
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     3,
			},
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrEnrichResources(repo)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Finding assertions.
	f, ok := result.Findings[repoID]
	if !ok {
		t.Fatalf("expected finding keyed by %q, got findings: %v", repoID, result.Findings)
	}
	if f.Severity != "!" {
		t.Errorf("Severity = %q, want %q", f.Severity, "!")
	}
	if !strings.Contains(f.Summary, "2 CRITICAL") {
		t.Errorf("Summary %q does not mention \"2 CRITICAL\"", f.Summary)
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}

	// FieldUpdates assertions: 2 CRITICAL total, 4 HIGH total (1+3), 2 images scanned.
	fu, ok := result.FieldUpdates[repoID]
	if !ok {
		t.Fatalf("expected FieldUpdates entry for %q", repoID)
	}
	if fu["critical_vulns"] != "2" {
		t.Errorf("critical_vulns = %q, want %q", fu["critical_vulns"], "2")
	}
	if fu["high_vulns"] != "4" {
		t.Errorf("high_vulns = %q, want %q", fu["high_vulns"], "4")
	}
	if fu["images_scanned"] != "2" {
		t.Errorf("images_scanned = %q, want %q", fu["images_scanned"], "2")
	}
}

// TestEnrichECRRepository_HighOnlyEmitsTildeFinding verifies that when a repo
// has only HIGH vulnerabilities (CRITICAL==0), the enricher emits Severity "~"
// and IssueCount is NOT incremented.
func TestEnrichECRRepository_HighOnlyEmitsTildeFinding(t *testing.T) {
	const repo = "staging-web"
	repoID := ecrRepoID(repo)

	fake := &ecrEnrichFake{
		listImagesByRepo: map[string][][]ecrtypes.ImageIdentifier{
			repo: {
				{
					{ImageDigest: aws.String("sha256:hhh001")},
				},
			},
		},
		scanCountsByDigest: map[string]map[string]int32{
			"sha256:hhh001": {
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     5,
			},
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrEnrichResources(repo)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f, ok := result.Findings[repoID]
	if !ok {
		t.Fatalf("expected finding keyed by %q for high-only vulns", repoID)
	}
	if f.Severity != "~" {
		t.Errorf("Severity = %q, want %q (high-only is tilde)", f.Severity, "~")
	}
	// "~" does not contribute to IssueCount per EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
	// Summary should mention the HIGH count.
	if !strings.Contains(f.Summary, "5 HIGH") {
		t.Errorf("Summary %q does not mention \"5 HIGH\"", f.Summary)
	}
}

// TestEnrichECRRepository_CleanRepoEmitsNoFinding verifies that when all images
// in a repo have CRITICAL==0 and HIGH==0, no finding is emitted but FieldUpdates
// still carries critical_vulns:"0".
func TestEnrichECRRepository_CleanRepoEmitsNoFinding(t *testing.T) {
	const repo = "clean"
	repoID := ecrRepoID(repo)

	fake := &ecrEnrichFake{
		listImagesByRepo: map[string][][]ecrtypes.ImageIdentifier{
			repo: {
				{
					{ImageDigest: aws.String("sha256:clean001")},
				},
			},
		},
		scanCountsByDigest: map[string]map[string]int32{
			"sha256:clean001": {
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     0,
			},
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrEnrichResources(repo)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result.Findings[repoID]; ok {
		t.Error("clean repo must NOT produce a finding")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 for clean repo", result.IssueCount)
	}

	// FieldUpdates must still be written with critical_vulns:"0".
	fu, ok := result.FieldUpdates[repoID]
	if !ok {
		t.Fatalf("expected FieldUpdates for %q even on clean repo", repoID)
	}
	if fu["critical_vulns"] != "0" {
		t.Errorf("critical_vulns = %q, want %q", fu["critical_vulns"], "0")
	}
}

// TestEnrichECRRepository_EmptyRepoNoImages verifies that when ListImages returns
// an empty list, FieldUpdates carries images_scanned:"0" and no finding is emitted.
func TestEnrichECRRepository_EmptyRepoNoImages(t *testing.T) {
	const repo = "empty-repo"
	repoID := ecrRepoID(repo)

	fake := &ecrEnrichFake{
		listImagesByRepo: map[string][][]ecrtypes.ImageIdentifier{
			repo: {{}}, // one empty page
		},
		scanCountsByDigest: map[string]map[string]int32{},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrEnrichResources(repo)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result.Findings[repoID]; ok {
		t.Error("empty repo must NOT produce a finding")
	}

	fu, ok := result.FieldUpdates[repoID]
	if !ok {
		t.Fatalf("expected FieldUpdates for empty repo %q", repoID)
	}
	if fu["images_scanned"] != "0" {
		t.Errorf("images_scanned = %q, want %q", fu["images_scanned"], "0")
	}
}

// TestEnrichECRRepository_ListImagesErrorMarksTruncated verifies that when ListImages
// returns an error for one repo, Truncated is set to true and the other repos are
// still processed.
func TestEnrichECRRepository_ListImagesErrorMarksTruncated(t *testing.T) {
	const repoOK = "good-repo"
	const repoBad = "bad-repo"
	repoOKID := ecrRepoID(repoOK)

	fake := &ecrEnrichFake{
		listImagesByRepo: map[string][][]ecrtypes.ImageIdentifier{
			repoOK: {
				{
					{ImageDigest: aws.String("sha256:ok001")},
				},
			},
		},
		listImagesErrByRepo: map[string]error{
			repoBad: errors.New("ecr: ListImages throttled"),
		},
		scanCountsByDigest: map[string]map[string]int32{
			"sha256:ok001": {
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     0,
			},
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	// Process both repos — bad-repo causes ListImages error.
	resources := ecrEnrichResources(repoBad, repoOK)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when ListImages fails for a repo")
	}
	// The good repo was still processed.
	if _, ok := result.FieldUpdates[repoOKID]; !ok {
		t.Errorf("good repo %q must still have FieldUpdates despite bad-repo error", repoOKID)
	}
}

// TestEnrichECRRepository_DescribeScanErrorSkipsImage verifies that when
// DescribeImageScanFindings returns a ScanNotFoundException for one image, the
// image is silently skipped and images_scanned reflects only successfully-scanned
// images. No panic.
func TestEnrichECRRepository_DescribeScanErrorSkipsImage(t *testing.T) {
	const repo = "scan-err-repo"
	repoID := ecrRepoID(repo)

	fake := &ecrEnrichFake{
		listImagesByRepo: map[string][][]ecrtypes.ImageIdentifier{
			repo: {
				{
					{ImageDigest: aws.String("sha256:scanerr001")},
					{ImageDigest: aws.String("sha256:scanok001")},
				},
			},
		},
		scanErrByDigest: map[string]error{
			"sha256:scanerr001": &ecrtypes.ScanNotFoundException{
				Message: aws.String("image scan not found"),
			},
		},
		scanCountsByDigest: map[string]map[string]int32{
			"sha256:scanok001": {
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     0,
			},
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrEnrichResources(repo)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only 1 image successfully scanned (the one that returned ScanNotFoundException is skipped).
	fu, ok := result.FieldUpdates[repoID]
	if !ok {
		t.Fatalf("expected FieldUpdates for %q", repoID)
	}
	if fu["images_scanned"] != "1" {
		t.Errorf("images_scanned = %q, want %q (scan-not-found image skipped)", fu["images_scanned"], "1")
	}
}

// TestEnrichECRRepository_CapsImagesAtN verifies that when ListImages returns 50
// images for a repo, exactly 10 DescribeImageScanFindings calls are made (per-repo
// image cap = 10) and images_scanned == "10".
func TestEnrichECRRepository_CapsImagesAtN(t *testing.T) {
	const repo = "large-repo"
	repoID := ecrRepoID(repo)
	const imageCap = 10

	// 50 images in a single page.
	allImages := makeImagePage("large", 50, 0)
	fake := &ecrEnrichFake{
		listImagesByRepo: map[string][][]ecrtypes.ImageIdentifier{
			repo: {allImages},
		},
		scanCountsByDigest: map[string]map[string]int32{},
	}
	// Pre-populate all 50 digests with zero counts.
	for _, img := range allImages {
		fake.scanCountsByDigest[*img.ImageDigest] = map[string]int32{
			string(ecrtypes.FindingSeverityCritical): 0,
			string(ecrtypes.FindingSeverityHigh):     0,
		}
	}

	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrEnrichResources(repo)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fake.scanCallDigests) != imageCap {
		t.Errorf("DescribeImageScanFindings called %d times, want exactly %d (per-repo cap)",
			len(fake.scanCallDigests), imageCap)
	}

	fu, ok := result.FieldUpdates[repoID]
	if !ok {
		t.Fatalf("expected FieldUpdates for %q", repoID)
	}
	if fu["images_scanned"] != fmt.Sprintf("%d", imageCap) {
		t.Errorf("images_scanned = %q, want %q", fu["images_scanned"], fmt.Sprintf("%d", imageCap))
	}
}

// TestEnrichECRRepository_CapsReposAtEnrichmentCap verifies that when there are
// more repos than EnrichmentCap, Truncated is true and FieldUpdates contains
// exactly EnrichmentCap entries.
func TestEnrichECRRepository_CapsReposAtEnrichmentCap(t *testing.T) {
	const totalRepos = 100
	names := make([]string, totalRepos)
	listImagesByRepo := make(map[string][][]ecrtypes.ImageIdentifier, totalRepos)
	for i := 0; i < totalRepos; i++ {
		name := fmt.Sprintf("repo-%03d", i)
		names[i] = name
		listImagesByRepo[name] = [][]ecrtypes.ImageIdentifier{{}} // empty images
	}

	fake := &ecrEnrichFake{
		listImagesByRepo:   listImagesByRepo,
		scanCountsByDigest: map[string]map[string]int32{},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrEnrichResources(names...)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when repos exceed EnrichmentCap")
	}
	if len(result.FieldUpdates) != awsclient.EnrichmentCap {
		t.Errorf("FieldUpdates len = %d, want %d (EnrichmentCap)", len(result.FieldUpdates), awsclient.EnrichmentCap)
	}
}

// TestEnrichECRRepository_NilClientReturnsEmpty verifies that when clients.ECR is
// nil, the enricher returns an empty non-nil Findings map and no error.
func TestEnrichECRRepository_NilClientReturnsEmpty(t *testing.T) {
	clients := &awsclient.ServiceClients{ECR: nil}
	resources := ecrEnrichResources("prod-api", "staging-web")

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when ECR client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichECRRepository_PaginatedListImages verifies that when ListImages requires
// two pages (5 images each) and the per-repo image cap is >= 10, all 10 images are
// scanned.
func TestEnrichECRRepository_PaginatedListImages(t *testing.T) {
	const repo = "paginated-repo"
	repoID := ecrRepoID(repo)

	page1 := makeImagePage("pg1", 5, 0)
	page2 := makeImagePage("pg2", 5, 0)

	fake := &ecrEnrichFake{
		listImagesByRepo: map[string][][]ecrtypes.ImageIdentifier{
			repo: {page1, page2},
		},
		scanCountsByDigest: map[string]map[string]int32{},
	}
	// Populate zero-count scan results for all 10 images.
	for _, img := range append(page1, page2...) {
		fake.scanCountsByDigest[*img.ImageDigest] = map[string]int32{
			string(ecrtypes.FindingSeverityCritical): 0,
			string(ecrtypes.FindingSeverityHigh):     0,
		}
	}

	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrEnrichResources(repo)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[repoID]
	if !ok {
		t.Fatalf("expected FieldUpdates for %q", repoID)
	}
	if fu["images_scanned"] != "10" {
		t.Errorf("images_scanned = %q, want %q (both pages consumed)", fu["images_scanned"], "10")
	}
}

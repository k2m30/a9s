package unit

import (
	"context"
	"errors"
	"testing"

	ecrsvc "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type ecrScanFindingsFake struct {
	awsclient.ECRAPI
	results   map[string]*ecrtypes.ImageScanFindings
	errByRepo map[string]error
}

func (f *ecrScanFindingsFake) DescribeImageScanFindings(
	_ context.Context,
	in *ecrsvc.DescribeImageScanFindingsInput,
	_ ...func(*ecrsvc.Options),
) (*ecrsvc.DescribeImageScanFindingsOutput, error) {
	name := ""
	if in != nil && in.RepositoryName != nil {
		name = *in.RepositoryName
	}
	if f.errByRepo != nil {
		if err, ok := f.errByRepo[name]; ok {
			return nil, err
		}
	}
	findings, ok := f.results[name]
	if !ok {
		return &ecrsvc.DescribeImageScanFindingsOutput{}, nil
	}
	return &ecrsvc.DescribeImageScanFindingsOutput{ImageScanFindings: findings}, nil
}

// EnrichECRRepository reads ImageScanFindingsSummary from one DescribeImages
// call per repo; the fake synthesises an ImageDetails entry whose summary
// mirrors f.results[repo].
func (f *ecrScanFindingsFake) DescribeImages(
	_ context.Context,
	in *ecrsvc.DescribeImagesInput,
	_ ...func(*ecrsvc.Options),
) (*ecrsvc.DescribeImagesOutput, error) {
	name := ""
	if in != nil && in.RepositoryName != nil {
		name = *in.RepositoryName
	}
	if f.errByRepo != nil {
		if err, ok := f.errByRepo[name]; ok {
			return nil, err
		}
	}
	findings, ok := f.results[name]
	if !ok || findings == nil {
		return &ecrsvc.DescribeImagesOutput{}, nil
	}
	return &ecrsvc.DescribeImagesOutput{
		ImageDetails: []ecrtypes.ImageDetail{
			{
				RepositoryName: in.RepositoryName,
				ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
					FindingSeverityCounts: findings.FindingSeverityCounts,
				},
			},
		},
	}, nil
}

var _ awsclient.ECRAPI = (*ecrScanFindingsFake)(nil)

func ecrRepoResources(names ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(names))
	for _, name := range names {
		res = append(res, resource.Resource{
			ID:   name,
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

func ecrScanFindings(counts map[string]int32) *ecrtypes.ImageScanFindings {
	return &ecrtypes.ImageScanFindings{
		FindingSeverityCounts: counts,
	}
}

const (
	ecrRepo1 = "my-service-api"
	ecrRepo2 = "my-service-worker"
)

func TestEnrichECRRepository_NoFindingsWhenAllCountsZero(t *testing.T) {
	fake := &ecrScanFindingsFake{
		results: map[string]*ecrtypes.ImageScanFindings{
			ecrRepo1: ecrScanFindings(map[string]int32{
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     0,
			}),
			ecrRepo2: ecrScanFindings(map[string]int32{
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     0,
			}),
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrRepoResources(ecrRepo1, ecrRepo2)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

func TestEnrichECRRepository_CriticalFindingsProduceSevBang(t *testing.T) {
	t.Skip("EnrichECRRepository is disabled (see ecr_issue_enrichment.go)")
	fake := &ecrScanFindingsFake{
		results: map[string]*ecrtypes.ImageScanFindings{
			ecrRepo1: ecrScanFindings(map[string]int32{
				string(ecrtypes.FindingSeverityCritical): 2,
				string(ecrtypes.FindingSeverityHigh):     0,
			}),
			ecrRepo2: ecrScanFindings(map[string]int32{
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     0,
			}),
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrRepoResources(ecrRepo1, ecrRepo2)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[ecrRepo1]
	if !ok {
		t.Fatalf("expected finding keyed by %q", ecrRepo1)
	}
	f := fs[0]
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
	if _, ok := result.Findings[ecrRepo2]; ok {
		t.Error("repo-2 must NOT appear in Findings — no critical vulnerabilities")
	}
}

// Severity "~" findings do not count toward IssueCount.
func TestEnrichECRRepository_HighFindingsProduceSevTilde(t *testing.T) {
	t.Skip("EnrichECRRepository is disabled (see ecr_issue_enrichment.go)")
	fake := &ecrScanFindingsFake{
		results: map[string]*ecrtypes.ImageScanFindings{
			ecrRepo1: ecrScanFindings(map[string]int32{
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     5,
			}),
			ecrRepo2: ecrScanFindings(map[string]int32{
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     0,
			}),
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrRepoResources(ecrRepo1, ecrRepo2)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[ecrRepo1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (high vulns)", ecrRepo1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings[ecrRepo2]; ok {
		t.Error("repo-2 must NOT appear in Findings — no vulnerabilities")
	}
}

// A nil ImageScanFindingsSummary means scan-on-push is off or the scan has not
// completed; such images yield no finding, truncation or error.
func TestEnrichECRRepository_UnscannedImagesSkipped(t *testing.T) {
	fake := &ecrScanFindingsFake{
		results: map[string]*ecrtypes.ImageScanFindings{
			ecrRepo2: ecrScanFindings(map[string]int32{
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     0,
			}),
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrRepoResources(ecrRepo1, ecrRepo2)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[ecrRepo1]; ok {
		t.Error("repo-1 must NOT appear in Findings — unscanned repo produces no finding")
	}
	if result.Truncated {
		t.Error("Truncated must be false — missing scan data is operational, not an error")
	}
}

func TestEnrichECRRepository_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{ECR: nil}

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, ecrRepoResources(ecrRepo1, ecrRepo2), nil)
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

func TestEnrichECRRepository_APIErrorSetsTruncatedNoError(t *testing.T) {
	t.Skip("EnrichECRRepository is disabled (see ecr_issue_enrichment.go)")
	apiErr := errors.New("ecr: DescribeImageScanFindings throttled")
	fake := &ecrScanFindingsFake{
		errByRepo: map[string]error{
			ecrRepo1: apiErr,
		},
		results: map[string]*ecrtypes.ImageScanFindings{
			ecrRepo2: ecrScanFindings(map[string]int32{
				string(ecrtypes.FindingSeverityCritical): 0,
				string(ecrtypes.FindingSeverityHigh):     0,
			}),
		},
	}
	clients := &awsclient.ServiceClients{ECR: fake}
	resources := ecrRepoResources(ecrRepo1, ecrRepo2)

	result, err := awsclient.EnrichECRRepository(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[ecrRepo1]; ok {
		t.Error("repo-1 must NOT appear in Findings on generic API error")
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a generic API call fails")
	}
}

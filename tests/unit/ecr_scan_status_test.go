package unit

// A repository's vulnerability counts are the newest image's scan findings,
// and only a scan that produced findings has any: COMPLETE for basic
// scanning, ACTIVE for enhanced scanning, which Amazon Inspector sets once it
// has scanned the image
// (https://docs.aws.amazon.com/inspector/latest/user/enable-disable-scanning-ecr.html).
// Every other status in API_ImageScanStatus
// (https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_ImageScanStatus.html)
// and an image never scanned hold no findings to count, so the repository is
// not inspected and gets no count.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrsvc "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ecrScanStatusFake answers DescribeImages with one image per repository, the
// way basic scanning does (no scan summary), and DescribeImageScanFindings
// with the repository's scan answer; a repository with no answer was never
// scanned.
type ecrScanStatusFake struct {
	awsclient.ECRAPI
	scans map[string]*ecrsvc.DescribeImageScanFindingsOutput
	// described holds DescribeImages' own scan fields per repository, the
	// shape an older basic-scanning answer carries.
	described map[string]ecrtypes.ImageDetail
}

func (f *ecrScanStatusFake) DescribeImages(_ context.Context, in *ecrsvc.DescribeImagesInput, _ ...func(*ecrsvc.Options)) (*ecrsvc.DescribeImagesOutput, error) {
	repo := aws.ToString(in.RepositoryName)
	img := f.described[repo]
	img.RepositoryName = in.RepositoryName
	img.ImageDigest = aws.String("sha256:" + strings.ReplaceAll(repo, "/", "-"))
	return &ecrsvc.DescribeImagesOutput{ImageDetails: []ecrtypes.ImageDetail{img}}, nil
}

func (f *ecrScanStatusFake) DescribeImageScanFindings(_ context.Context, in *ecrsvc.DescribeImageScanFindingsInput, _ ...func(*ecrsvc.Options)) (*ecrsvc.DescribeImageScanFindingsOutput, error) {
	if out, ok := f.scans[aws.ToString(in.RepositoryName)]; ok {
		return out, nil
	}
	return nil, &ecrtypes.ScanNotFoundException{Message: aws.String("Image scan does not exist for the image")}
}

func (f *ecrScanStatusFake) GetRepositoryPolicy(context.Context, *ecrsvc.GetRepositoryPolicyInput, ...func(*ecrsvc.Options)) (*ecrsvc.GetRepositoryPolicyOutput, error) {
	return nil, &ecrtypes.RepositoryPolicyNotFoundException{Message: aws.String("no policy")}
}

func (f *ecrScanStatusFake) GetLifecyclePolicy(context.Context, *ecrsvc.GetLifecyclePolicyInput, ...func(*ecrsvc.Options)) (*ecrsvc.GetLifecyclePolicyOutput, error) {
	return &ecrsvc.GetLifecyclePolicyOutput{LifecyclePolicyText: aws.String(`{"rules":[]}`)}, nil
}

func ecrScanAnswer(status ecrtypes.ScanStatus, findings *ecrtypes.ImageScanFindings) *ecrsvc.DescribeImageScanFindingsOutput {
	return &ecrsvc.DescribeImageScanFindingsOutput{
		ImageScanStatus:   &ecrtypes.ImageScanStatus{Status: status},
		ImageScanFindings: findings,
	}
}

func TestEnrichECRRepository_OnlyAFinishedScanIsCounted(t *testing.T) {
	critical := &ecrtypes.ImageScanFindings{Findings: []ecrtypes.ImageScanFinding{
		{Severity: ecrtypes.FindingSeverityCritical}, {Severity: ecrtypes.FindingSeverityHigh},
	}}
	enhanced := &ecrtypes.ImageScanFindings{EnhancedFindings: []ecrtypes.EnhancedImageScanFinding{
		{Severity: aws.String("HIGH")},
	}}
	fake := &ecrScanStatusFake{
		scans: map[string]*ecrsvc.DescribeImageScanFindingsOutput{
			"acme/complete":     ecrScanAnswer(ecrtypes.ScanStatusComplete, critical),
			"acme/clean":        ecrScanAnswer(ecrtypes.ScanStatusComplete, &ecrtypes.ImageScanFindings{}),
			"acme/enhanced":     ecrScanAnswer(ecrtypes.ScanStatusActive, enhanced),
			"acme/failed":       ecrScanAnswer(ecrtypes.ScanStatusFailed, nil),
			"acme/in-progress":  ecrScanAnswer(ecrtypes.ScanStatusInProgress, nil),
			"acme/pending":      ecrScanAnswer(ecrtypes.ScanStatusPending, nil),
			"acme/unsupported":  ecrScanAnswer(ecrtypes.ScanStatusUnsupportedImage, nil),
			"acme/expired":      ecrScanAnswer(ecrtypes.ScanStatusScanEligibilityExpired, nil),
			"acme/stale-rescan": ecrScanAnswer(ecrtypes.ScanStatusInProgress, critical),
		},
		described: map[string]ecrtypes.ImageDetail{
			"acme/described-failed": {
				ImageScanStatus:          &ecrtypes.ImageScanStatus{Status: ecrtypes.ScanStatusFailed},
				ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{FindingSeverityCounts: map[string]int32{}},
			},
		},
	}
	names := []string{
		"acme/complete", "acme/clean", "acme/enhanced", "acme/failed", "acme/in-progress", "acme/pending",
		"acme/unsupported", "acme/expired", "acme/stale-rescan", "acme/never-scanned", "acme/described-failed",
	}
	var rows []resource.Resource
	for _, n := range names {
		rows = append(rows, resource.Resource{ID: n, Name: n, Fields: map[string]string{"repository_name": n}})
	}
	result, err := awsclient.EnrichECRRepository(context.Background(), &awsclient.ServiceClients{ECR: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichECRRepository: %v", err)
	}

	counted := map[string][2]string{
		"acme/complete": {"1", "1"},
		"acme/clean":    {"0", "0"},
		"acme/enhanced": {"0", "1"},
	}
	for repo, want := range counted {
		fu := result.FieldUpdates[repo]
		if fu["critical_vulns"] != want[0] || fu["high_vulns"] != want[1] || fu["images_scanned"] != "1" {
			t.Errorf("%s: counts = %v, want critical %s high %s over one scanned image", repo, fu, want[0], want[1])
		}
		if check, ok := result.TruncatedIDs[repo]; ok {
			t.Errorf("%s: not inspected (%q), want counted", repo, check)
		}
	}

	unread := map[string]string{
		"acme/failed":           "FAILED",
		"acme/in-progress":      "IN_PROGRESS",
		"acme/pending":          "PENDING",
		"acme/unsupported":      "UNSUPPORTED_IMAGE",
		"acme/expired":          "SCAN_ELIGIBILITY_EXPIRED",
		"acme/stale-rescan":     "IN_PROGRESS",
		"acme/described-failed": "FAILED",
		"acme/never-scanned":    "not scanned",
	}
	for repo, reason := range unread {
		if fu, ok := result.FieldUpdates[repo]; ok {
			t.Errorf("%s: counts written %v, want none for a scan with no findings to count", repo, fu)
		}
		if check, ok := result.TruncatedIDs[repo]; !ok || !strings.Contains(check, reason) {
			t.Errorf("%s: not-inspected check = %q (marked %v), want one naming %q", repo, check, ok, reason)
		}
		if _, ok := result.Findings[repo]; ok {
			t.Errorf("%s: vulnerability finding raised from a scan with no findings to count", repo)
		}
	}
	if !result.Truncated {
		t.Error("Truncated = false with repositories not inspected")
	}
}

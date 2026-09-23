package unit_test

// With Basic Scanning, DescribeImages leaves ImageScanFindingsSummary and
// ImageScanStatus empty (SDK doc on DescribeImages); the severity counts are
// only in DescribeImageScanFindings. A repository whose scan results could not
// be read has no count, which is not the same answer as zero.

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

const t561ECRLifecycleDoc = `{"rules":[{"rulePriority":1,"description":"expire untagged after 14 days","selection":{"tagStatus":"untagged","countType":"sinceImagePushed","countUnit":"days","countNumber":14},"action":{"type":"expire"}}]}`

// t561ECRFake holds one image per repository and answers DescribeImages the
// way Basic Scanning does: no scan summary, no scan status.
type t561ECRFake struct {
	awsclient.ECRAPI
	counts  map[string]map[string]int32 // repo → severity counts DescribeImageScanFindings reports
	scanErr map[string]error            // repo → DescribeImageScanFindings error
}

func t561Digest(repo string) string {
	return "sha256:" + map[string]string{
		"acme/payments-api": "3f1c9a7e5b2d4c6a8e0f1b3d5c7e9a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1e3f",
		"acme/web-frontend": "9b8a7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8b",
		"acme/batch-worker": "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
	}[repo]
}

func (f *t561ECRFake) DescribeImages(_ context.Context, in *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	repo := aws.ToString(in.RepositoryName)
	return &ecr.DescribeImagesOutput{ImageDetails: []ecrtypes.ImageDetail{{
		RegistryId:             aws.String("123456789012"),
		RepositoryName:         aws.String(repo),
		ImageDigest:            aws.String(t561Digest(repo)),
		ImageTags:              []string{"v2.14.0"},
		ImagePushedAt:          aws.Time(time.Date(2026, 9, 1, 10, 30, 0, 0, time.UTC)),
		ImageSizeInBytes:       aws.Int64(98_543_210),
		ImageManifestMediaType: aws.String("application/vnd.docker.distribution.manifest.v2+json"),
		ArtifactMediaType:      aws.String("application/vnd.docker.container.image.v1+json"),
	}}}, nil
}

func (f *t561ECRFake) DescribeImageScanFindings(_ context.Context, in *ecr.DescribeImageScanFindingsInput, _ ...func(*ecr.Options)) (*ecr.DescribeImageScanFindingsOutput, error) {
	repo := aws.ToString(in.RepositoryName)
	if err, ok := f.scanErr[repo]; ok {
		return nil, err
	}
	return &ecr.DescribeImageScanFindingsOutput{
		RegistryId:     aws.String("123456789012"),
		RepositoryName: aws.String(repo),
		ImageId:        &ecrtypes.ImageIdentifier{ImageDigest: aws.String(t561Digest(repo)), ImageTag: aws.String("v2.14.0")},
		ImageScanStatus: &ecrtypes.ImageScanStatus{
			Status:      ecrtypes.ScanStatusComplete,
			Description: aws.String("The scan was completed successfully."),
		},
		ImageScanFindings: &ecrtypes.ImageScanFindings{
			ImageScanCompletedAt:         aws.Time(time.Date(2026, 9, 1, 10, 31, 0, 0, time.UTC)),
			VulnerabilitySourceUpdatedAt: aws.Time(time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)),
			FindingSeverityCounts:        f.counts[repo],
		},
	}, nil
}

func (f *t561ECRFake) GetRepositoryPolicy(_ context.Context, _ *ecr.GetRepositoryPolicyInput, _ ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error) {
	return nil, &ecrtypes.RepositoryPolicyNotFoundException{Message: aws.String("Repository policy does not exist")}
}

func (f *t561ECRFake) GetLifecyclePolicy(_ context.Context, in *ecr.GetLifecyclePolicyInput, _ ...func(*ecr.Options)) (*ecr.GetLifecyclePolicyOutput, error) {
	return &ecr.GetLifecyclePolicyOutput{RepositoryName: in.RepositoryName, LifecyclePolicyText: aws.String(t561ECRLifecycleDoc)}, nil
}

func t561NewECRFake() *t561ECRFake {
	return &t561ECRFake{
		counts: map[string]map[string]int32{
			"acme/payments-api": {"CRITICAL": 2, "HIGH": 5, "MEDIUM": 11},
			"acme/web-frontend": {"MEDIUM": 3, "LOW": 7},
		},
		scanErr: map[string]error{"acme/batch-worker": &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform: ecr:DescribeImageScanFindings",
		}},
	}
}

// The enricher's answer is read off the rows as the app folds it: findings
// through runtime.ApplyWave2ToRow, FieldUpdates merged into Fields.
func TestT561_ECRRepositoryCountsReadFromScanFindings(t *testing.T) {
	enricher, ok := awsclient.Wave2EnricherFor("ecr")
	if !ok || enricher.Fn == nil {
		t.Fatal("ecr has no wave-2 enricher")
	}
	var repos []resource.Resource
	for _, name := range []string{"acme/payments-api", "acme/web-frontend", "acme/batch-worker"} {
		repos = append(repos, resource.Resource{ID: name, Name: name, Fields: map[string]string{"repository_name": name}})
	}
	res, _ := enricher.Fn(context.Background(), &awsclient.ServiceClients{ECR: t561NewECRFake()}, repos, nil) //nolint:errcheck // one repository's failed read is part of the scenario; its row is asserted instead
	td := resource.FindResourceType("ecr")
	for i := range repos {
		maps.Copy(repos[i].Fields, res.FieldUpdates[repos[i].ID])
		runtime.ApplyWave2ToRow(&repos[i], *td, res.Findings, res.AttentionDetails)
	}

	paying := t561Row(t, repos, "acme/payments-api")
	if !t561Has(paying.Findings, "ecr.vulnerabilities") {
		t.Errorf("acme/payments-api: findings = %v, want ecr.vulnerabilities", t561Codes(paying.Findings))
	}
	for _, f := range paying.Findings {
		if f.Code == "ecr.vulnerabilities" && f.Phrase != "2 critical, 5 high vulnerabilities" {
			t.Errorf("acme/payments-api: phrase = %q, want %q", f.Phrase, "2 critical, 5 high vulnerabilities")
		}
	}
	if got := td.ResolveColor(paying); got != resource.ColorBroken {
		t.Errorf("acme/payments-api: colour = %v, want %v", got, resource.ColorBroken)
	}
	if f := paying.Fields; f["critical_vulns"] != "2" || f["high_vulns"] != "5" {
		t.Errorf("acme/payments-api: critical/high = %q/%q, want 2/5", f["critical_vulns"], f["high_vulns"])
	}

	clean := t561Row(t, repos, "acme/web-frontend")
	if t561Has(clean.Findings, "ecr.vulnerabilities") || t561Has(clean.Findings, "ecr.vulnerabilities-high") {
		t.Errorf("acme/web-frontend: medium and low only, findings = %v", t561Codes(clean.Findings))
	}
	if f := clean.Fields; f["critical_vulns"] != "0" || f["high_vulns"] != "0" {
		t.Errorf("acme/web-frontend: critical/high = %q/%q, want 0/0 (read and proven clean)", f["critical_vulns"], f["high_vulns"])
	}

	unread := t561Row(t, repos, "acme/batch-worker")
	if t561Has(unread.Findings, "ecr.vulnerabilities") || t561Has(unread.Findings, "ecr.vulnerabilities-high") {
		t.Errorf("acme/batch-worker: findings = %v on a repository whose scan results could not be read", t561Codes(unread.Findings))
	}
	if f := unread.Fields; f["critical_vulns"] != "" || f["high_vulns"] != "" {
		t.Errorf("acme/batch-worker: critical/high = %q/%q, want no count for a read that failed", f["critical_vulns"], f["high_vulns"])
	}
	if _, marked := res.TruncatedIDs["acme/batch-worker"]; !marked {
		t.Errorf("acme/batch-worker: scan read failed but the row is not marked not inspected; TruncatedIDs=%v", res.TruncatedIDs)
	}
}

func TestT561_ECRImageChildCountsReadFromScanFindings(t *testing.T) {
	fetch := resource.GetPaginatedChildFetcher("ecr_images")
	if fetch == nil {
		t.Fatal("ecr_images has no child fetcher")
	}
	clients := &awsclient.ServiceClients{ECR: t561NewECRFake()}
	image := func(repo string) resource.Resource {
		t.Helper()
		out, err := fetch(context.Background(), clients, resource.ParentContext{
			"repository_name": repo,
			"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/" + repo,
		}, "")
		if err != nil {
			t.Fatalf("%s: child fetch: %v", repo, err)
		}
		if len(out.Resources) != 1 {
			t.Fatalf("%s: %d images, want 1", repo, len(out.Resources))
		}
		return out.Resources[0]
	}

	vuln := image("acme/payments-api")
	if got := vuln.Fields["finding_counts"]; got != "2C 5H 11M" {
		t.Errorf("acme/payments-api image: Findings column = %q, want %q", got, "2C 5H 11M")
	}
	if !t561Has(vuln.Findings, awsclient.CodeECRImageCritical) {
		t.Errorf("acme/payments-api image: findings = %v, want %s", t561Codes(vuln.Findings), awsclient.CodeECRImageCritical)
	}
	if got := resource.GetChildType("ecr_images").ResolveColor(vuln); got != resource.ColorBroken {
		t.Errorf("acme/payments-api image: colour = %v, want %v", got, resource.ColorBroken)
	}

	clean := image("acme/web-frontend")
	if got := clean.Fields["finding_counts"]; got != "3M 7L" {
		t.Errorf("acme/web-frontend image: Findings column = %q, want %q", got, "3M 7L")
	}
	if t561Has(clean.Findings, awsclient.CodeECRImageCritical) || t561Has(clean.Findings, awsclient.CodeECRImageHigh) {
		t.Errorf("acme/web-frontend image: findings = %v on medium and low only", t561Codes(clean.Findings))
	}

	if got := clean.Fields["scan_status"]; got == "?" {
		t.Errorf("acme/web-frontend image: Scan Status = %q on an image whose scan was read", got)
	}
	if got := resource.GetChildType("ecr_images").ResolveColor(clean); got != resource.ColorHealthy {
		t.Errorf("acme/web-frontend image: colour = %v, want %v", got, resource.ColorHealthy)
	}
}

// An image whose scan results could not be read is not inspected: its scan
// status and counts read "?", as the repository row does, and it is not
// painted healthy. A blank there is indistinguishable from a clean image.
func TestT561_ECRImageWithUnreadableScanIsNotInspected(t *testing.T) {
	out, err := resource.GetPaginatedChildFetcher("ecr_images")(context.Background(), &awsclient.ServiceClients{ECR: t561NewECRFake()}, resource.ParentContext{
		"repository_name": "acme/batch-worker",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/batch-worker",
	}, "")
	if err != nil {
		t.Fatalf("child fetch: %v", err)
	}
	if len(out.Resources) != 1 {
		t.Fatalf("%d images, want 1", len(out.Resources))
	}
	unread := out.Resources[0]
	if got := unread.Fields["scan_status"]; got != "?" {
		t.Errorf("Scan Status = %q for a scan read that failed, want %q", got, "?")
	}
	if got := unread.Fields["finding_counts"]; got != "?" {
		t.Errorf("Findings column = %q for a scan read that failed, want %q", got, "?")
	}
	if t561Has(unread.Findings, awsclient.CodeECRImageCritical) || t561Has(unread.Findings, awsclient.CodeECRImageHigh) {
		t.Errorf("findings = %v on an image whose scan could not be read", t561Codes(unread.Findings))
	}
	if got := resource.GetChildType("ecr_images").ResolveColor(unread); got == resource.ColorHealthy {
		t.Errorf("colour = %v: an image whose scan could not be read is painted healthy", got)
	}
}

package unit

// aws_s3_enricher_test.go — Behavioral tests for EnrichS3PublicAccessBlock.
//
// Contract assertions:
//   - GetPublicAccessBlock is called once per S3 bucket resource (keyed by Name).
//   - All 3 buckets have all 4 PAB flags true → 0 findings.
//   - Bucket returns NoSuchPublicAccessBlockConfiguration error → finding for that bucket, severity "~".
//   - Bucket has BlockPublicAcls=false (other flags true) → finding for that bucket, severity "~".
//   - clients.S3 == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - Bucket returns generic AccessDenied error → 0 findings, Truncated=true, no error.

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// s3PublicAccessBlockFake implements S3API for PAB enrichment testing.
// It embeds the interface and overrides only GetPublicAccessBlock.
// The results map is keyed by bucket name so the fake can serve different
// responses per bucket resource.
type s3PublicAccessBlockFake struct {
	awsclient.S3API
	// results maps bucket name → PAB configuration.
	results map[string]*s3types.PublicAccessBlockConfiguration
	// errByBucket maps bucket name → error; overrides results when set.
	errByBucket map[string]error
}

func (f *s3PublicAccessBlockFake) GetPublicAccessBlock(
	_ context.Context,
	in *s3.GetPublicAccessBlockInput,
	_ ...func(*s3.Options),
) (*s3.GetPublicAccessBlockOutput, error) {
	name := ""
	if in != nil && in.Bucket != nil {
		name = *in.Bucket
	}
	if f.errByBucket != nil {
		if err, ok := f.errByBucket[name]; ok {
			return nil, err
		}
	}
	cfg := f.results[name]
	return &s3.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: cfg}, nil
}

// Compile-time check: s3PublicAccessBlockFake satisfies S3API.
var _ awsclient.S3API = (*s3PublicAccessBlockFake)(nil)

// s3BucketResources returns a slice of S3 Resource stubs with the given bucket names.
// Name is set to the bucket name because EnrichS3PublicAccessBlock keys by Name.
func s3BucketResources(names ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(names))
	for _, name := range names {
		res = append(res, resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"bucket_name": name,
			},
		})
	}
	return res
}

// allBlockedPAB returns a PublicAccessBlockConfiguration with all 4 flags set to true.
func allBlockedPAB() *s3types.PublicAccessBlockConfiguration {
	return &s3types.PublicAccessBlockConfiguration{
		BlockPublicAcls:       aws.Bool(true),
		IgnorePublicAcls:      aws.Bool(true),
		BlockPublicPolicy:     aws.Bool(true),
		RestrictPublicBuckets: aws.Bool(true),
	}
}

// noPABError returns a smithy.GenericAPIError with code
// "NoSuchPublicAccessBlockConfiguration", matching the detection logic in
// EnrichS3PublicAccessBlock (errors.As + apiErr.ErrorCode()).
func noPABError() error {
	return &smithy.GenericAPIError{
		Code:    "NoSuchPublicAccessBlockConfiguration",
		Message: "The public access block configuration was not found",
	}
}

// TestEnrichS3PublicAccessBlock_AllBlockedProducesNoFindings verifies that when all
// 3 buckets have all 4 PAB flags set to true, no findings are produced.
func TestEnrichS3PublicAccessBlock_AllBlockedProducesNoFindings(t *testing.T) {
	fake := &s3PublicAccessBlockFake{
		results: map[string]*s3types.PublicAccessBlockConfiguration{
			"my-bucket-alpha":   allBlockedPAB(),
			"my-bucket-bravo":   allBlockedPAB(),
			"my-bucket-charlie": allBlockedPAB(),
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := s3BucketResources("my-bucket-alpha", "my-bucket-bravo", "my-bucket-charlie")

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
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

// TestEnrichS3PublicAccessBlock_NoPABConfigProducesFindingSevTilde verifies that when
// bucket-1 returns NoSuchPublicAccessBlockConfiguration, a finding with severity "~"
// is produced for bucket-1. The other two buckets (fully blocked) produce no finding.
func TestEnrichS3PublicAccessBlock_NoPABConfigProducesFindingSevTilde(t *testing.T) {
	fake := &s3PublicAccessBlockFake{
		errByBucket: map[string]error{
			"bucket-one": noPABError(),
		},
		results: map[string]*s3types.PublicAccessBlockConfiguration{
			"bucket-two":   allBlockedPAB(),
			"bucket-three": allBlockedPAB(),
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := s3BucketResources("bucket-one", "bucket-two", "bucket-three")

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["bucket-one"]
	if !ok {
		t.Fatalf("expected finding keyed by %q", "bucket-one")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings["bucket-two"]; ok {
		t.Error("bucket-two must NOT appear in Findings — it has full PAB")
	}
	if _, ok := result.Findings["bucket-three"]; ok {
		t.Error("bucket-three must NOT appear in Findings — it has full PAB")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
	if result.Truncated {
		t.Error("Truncated must be false — only PAB-absent error, not a generic API error")
	}
}

// TestEnrichS3PublicAccessBlock_PartialPABProducesFinding verifies that when
// bucket-1 has BlockPublicAcls=false and the other three flags true, a finding with
// severity "~" is produced for bucket-1. The other two buckets produce no finding.
func TestEnrichS3PublicAccessBlock_PartialPABProducesFinding(t *testing.T) {
	partial := &s3types.PublicAccessBlockConfiguration{
		BlockPublicAcls:       aws.Bool(false), // the offending flag
		IgnorePublicAcls:      aws.Bool(true),
		BlockPublicPolicy:     aws.Bool(true),
		RestrictPublicBuckets: aws.Bool(true),
	}
	fake := &s3PublicAccessBlockFake{
		results: map[string]*s3types.PublicAccessBlockConfiguration{
			"bucket-one":   partial,
			"bucket-two":   allBlockedPAB(),
			"bucket-three": allBlockedPAB(),
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := s3BucketResources("bucket-one", "bucket-two", "bucket-three")

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["bucket-one"]
	if !ok {
		t.Fatalf("expected finding for %q (BlockPublicAcls=false)", "bucket-one")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings["bucket-two"]; ok {
		t.Error("bucket-two must NOT appear in Findings — it has full PAB")
	}
	if _, ok := result.Findings["bucket-three"]; ok {
		t.Error("bucket-three must NOT appear in Findings — it has full PAB")
	}
}

// TestEnrichS3PublicAccessBlock_NilClientReturnsEmptyFindingsNoError verifies that
// when clients.S3 is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichS3PublicAccessBlock_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{S3: nil}

	result, err := awsclient.EnrichS3PublicAccessBlock(
		context.Background(), clients,
		s3BucketResources("bucket-one", "bucket-two", "bucket-three"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when S3 client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichS3PublicAccessBlock_GenericAPIErrorSetsTruncatedNoFindings verifies that
// when bucket-1 returns a generic AccessDenied error (not NoSuchPublicAccessBlockConfiguration),
// the enricher sets Truncated=true, produces 0 findings for bucket-1, and does not
// propagate the error.
func TestEnrichS3PublicAccessBlock_GenericAPIErrorSetsTruncatedNoFindings(t *testing.T) {
	accessDenied := errors.New("AccessDenied: Access Denied")
	fake := &s3PublicAccessBlockFake{
		errByBucket: map[string]error{
			"bucket-one": accessDenied,
		},
		results: map[string]*s3types.PublicAccessBlockConfiguration{
			"bucket-two":   allBlockedPAB(),
			"bucket-three": allBlockedPAB(),
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := s3BucketResources("bucket-one", "bucket-two", "bucket-three")

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a generic API error occurs")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on generic API error, got %d", len(result.Findings))
	}
}

package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// s3PABFake dispatches GetPublicAccessBlock per bucket from a pre-built map.
// Semantics mirror fixtures.S3Fixtures.PublicAccessBlockConfigs:
//   - key present, non-nil value → return that output (may carry nil inner config).
//   - key present, nil value     → return NoSuchPublicAccessBlockConfiguration error.
//   - key absent                 → return empty output (all flags nil/false).
//   - "error-bucket"             → return a generic AccessDenied error.
type s3PABFake struct {
	// A nil *s3.GetPublicAccessBlockOutput signals NoSuchPublicAccessBlockConfiguration.
	configs      map[string]*s3.GetPublicAccessBlockOutput
	errorBuckets map[string]bool
	// codedErrors maps bucket name → smithy error code (e.g. "NoSuchBucket",
	// "NotFound", "AccessDenied"); GetPublicAccessBlock returns
	// &smithy.GenericAPIError{Code: code} for that bucket. Takes priority over
	// errorBuckets/configs.
	codedErrors map[string]string
	// rawErrors maps bucket name → a caller-supplied error returned verbatim,
	// bypassing smithy.APIError entirely (e.g. a plain network error). Takes
	// priority over codedErrors/errorBuckets/configs.
	rawErrors map[string]error
}

func (f *s3PABFake) GetPublicAccessBlock(
	_ context.Context,
	input *s3.GetPublicAccessBlockInput,
	_ ...func(*s3.Options),
) (*s3.GetPublicAccessBlockOutput, error) {
	if input.Bucket == nil {
		return nil, &smithy.GenericAPIError{Code: "InvalidBucketName", Message: "bucket required"}
	}
	bucket := *input.Bucket
	if err, ok := f.rawErrors[bucket]; ok {
		return nil, err
	}
	if code, ok := f.codedErrors[bucket]; ok {
		return nil, &smithy.GenericAPIError{Code: code, Message: "synthetic " + code + " for " + bucket}
	}
	if f.errorBuckets[bucket] {
		return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "access denied"}
	}
	cfg, ok := f.configs[bucket]
	if !ok {
		return &s3.GetPublicAccessBlockOutput{}, nil
	}
	if cfg == nil {
		return nil, &smithy.GenericAPIError{
			Code:    "NoSuchPublicAccessBlockConfiguration",
			Message: "The public access block configuration was not found",
		}
	}
	return cfg, nil
}

// ListBuckets and the other S3API methods are stubs: the enrichment tests
// build resources directly and call only GetPublicAccessBlock.
func (f *s3PABFake) ListBuckets(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return &s3.ListBucketsOutput{}, nil
}

func (f *s3PABFake) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}

func (f *s3PABFake) GetBucketNotificationConfiguration(_ context.Context, _ *s3.GetBucketNotificationConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return &s3.GetBucketNotificationConfigurationOutput{}, nil
}

func pabResource(name string) resource.Resource {
	return resource.Resource{
		ID:     name,
		Name:   name,
		Fields: map[string]string{"name": name},
	}
}

// s3 PAB findings are always "~" (SevWarn): account-level PAB may still
// override, so a missing or partial bucket-level block is never certain
// public exposure.
func assertFindingShape(t *testing.T, findings map[string][]domain.Finding, key string) domain.Finding {
	t.Helper()
	fs, ok := findings[key]
	if !ok {
		t.Fatalf("expected finding for %q; Findings keys = %v", key, findingKeys(findings))
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("[%s] Severity = %v, want SevWarn (s3 Wave 2 PAB findings have no Broken tier)", key, f.Severity)
	}
	const wantPhrase = "public access block incomplete"
	if f.Phrase != wantPhrase {
		t.Errorf("[%s] Phrase = %q, want %q", key, f.Phrase, wantPhrase)
	}
	return f
}

func rowMap(rows []domain.DetailRow) map[string]string {
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Label] = r.Value
	}
	return m
}

func TestS3_Enrich_HealthyBucket_NoFinding(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"healthy-bucket": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(true),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(true),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource("healthy-bucket")}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	if _, ok := result.Findings["healthy-bucket"]; ok {
		t.Error("expected no finding for healthy bucket with all PAB flags true")
	}
	if _, ok := result.FieldUpdates["healthy-bucket"]; ok {
		t.Error("expected no FieldUpdates for healthy bucket with all PAB flags true")
	}
}

func TestS3_Enrich_NoPAB_Configuration(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"a9s-demo-nopab": nil, // explicit nil → NoSuchPublicAccessBlockConfiguration
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource("a9s-demo-nopab")}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-nopab")

	if strings.Contains(finding.Phrase, "no public access block configuration") {
		t.Errorf("Phrase must not embed Row content; got %q", finding.Phrase)
	}

	rows := rowMap(result.AttentionDetails["a9s-demo-nopab"][finding.Code].Rows)
	if rows["Status"] != "no public access block configuration" {
		t.Errorf("Rows[Status] = %q, want %q", rows["Status"], "no public access block configuration")
	}
	if rows["Account-level PAB"] != "may still apply" {
		t.Errorf("Rows[Account-level PAB] = %q, want %q", rows["Account-level PAB"], "may still apply")
	}

	updates, ok := result.FieldUpdates["a9s-demo-nopab"]
	if !ok {
		t.Fatal("FieldUpdates missing entry for a9s-demo-nopab")
	}
	if updates["status"] != "public access block incomplete" {
		t.Errorf("FieldUpdates[status] = %q, want %q", updates["status"], "public access block incomplete")
	}
	if _, hasOld := updates["public_access"]; hasOld {
		t.Error("FieldUpdates must not contain the deprecated 'public_access' key")
	}
}

func TestS3_Enrich_PartialPAB_SingleFlagFalse(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"a9s-demo-partial-pab": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(false),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(true),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource("a9s-demo-partial-pab")}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-partial-pab")

	if strings.Contains(finding.Phrase, "BlockPublicAcls") || strings.Contains(finding.Phrase, "false") {
		t.Errorf("Phrase must not embed Row content; got %q", finding.Phrase)
	}

	rows := rowMap(result.AttentionDetails["a9s-demo-partial-pab"][finding.Code].Rows)
	// The row is labelled in plain words; the SDK flag name rides in the value,
	// where an identifier is allowed and is what the operator greps the console for.
	// The value reads "off", not "false": a Go bool literal describes the SDK
	// field, not the account (TestNetworkingRowValues_AreWordsNotLiterals).
	if rows["Block public access control lists"] != "off (BlockPublicAcls)" {
		t.Errorf("Rows[Block public access control lists] = %q, want %q",
			rows["Block public access control lists"], "off (BlockPublicAcls)")
	}
	if rows["Account-level PAB"] != "may still apply" {
		t.Errorf("Rows[Account-level PAB] = %q, want %q", rows["Account-level PAB"], "may still apply")
	}

	updates, ok := result.FieldUpdates["a9s-demo-partial-pab"]
	if !ok {
		t.Fatal("FieldUpdates missing entry for a9s-demo-partial-pab")
	}
	if updates["status"] != "public access block incomplete" {
		t.Errorf("FieldUpdates[status] = %q, want %q", updates["status"], "public access block incomplete")
	}
}

func TestS3_Enrich_PartialPAB_MultipleFlagsFalse(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"a9s-demo-multifail-pab": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(false),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(false),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource("a9s-demo-multifail-pab")}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-multifail-pab")

	const wantPhrase = "public access block incomplete"
	if finding.Phrase != wantPhrase {
		t.Errorf("Phrase = %q, want %q (must be stable across instances)", finding.Phrase, wantPhrase)
	}
	if strings.Contains(finding.Phrase, "BlockPublicAcls") || strings.Contains(finding.Phrase, "BlockPublicPolicy") {
		t.Errorf("Phrase must not embed Row content; got %q", finding.Phrase)
	}

	rows := rowMap(result.AttentionDetails["a9s-demo-multifail-pab"][finding.Code].Rows)
	// The row is labelled in plain words; the SDK flag name rides in the value,
	// where an identifier is allowed and is what the operator greps the console for.
	// The value reads "off", not "false": a Go bool literal describes the SDK
	// field, not the account (TestNetworkingRowValues_AreWordsNotLiterals).
	if rows["Block public access control lists"] != "off (BlockPublicAcls)" {
		t.Errorf("Rows[Block public access control lists] = %q, want %q",
			rows["Block public access control lists"], "off (BlockPublicAcls)")
	}
	if rows["Block public bucket policy"] != "off (BlockPublicPolicy)" {
		t.Errorf("Rows[Block public bucket policy] = %q, want %q",
			rows["Block public bucket policy"], "off (BlockPublicPolicy)")
	}
}

func TestS3_Enrich_NilPABConfiguration_TreatedAsNoPAB(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"a9s-demo-nilcfg": {
				PublicAccessBlockConfiguration: nil,
			},
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource("a9s-demo-nilcfg")}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-nilcfg")

	if strings.Contains(finding.Phrase, "no public access block configuration") {
		t.Errorf("Phrase must not embed Row content; got %q", finding.Phrase)
	}

	updates, ok := result.FieldUpdates["a9s-demo-nilcfg"]
	if !ok {
		t.Fatal("FieldUpdates missing entry for a9s-demo-nilcfg")
	}
	if updates["status"] != "public access block incomplete" {
		t.Errorf("FieldUpdates[status] = %q, want %q", updates["status"], "public access block incomplete")
	}
}

// An error other than NoSuchPublicAccessBlockConfiguration leaves the data
// incomplete: no finding, the bucket marked in TruncatedIDs, and the failure
// in the composite error.
func TestS3_Enrich_UnknownAPIError_NoFinding(t *testing.T) {
	fake := &s3PABFake{
		configs:      map[string]*s3.GetPublicAccessBlockOutput{},
		errorBuckets: map[string]bool{"error-bucket": true},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource("error-bucket")}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("expected non-nil composite error when GetPublicAccessBlock returns generic error; got nil")
	}
	// The aggregate names the call, not the type: the type comes from the
	// registry key at the surface, and a type in the label would render it
	// twice ("enrich s3: s3-enrich: ...").
	if !strings.Contains(err.Error(), "bucket posture") {
		t.Errorf("err must name the pass, \"bucket posture\"; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "error-bucket") {
		t.Errorf("err must name the failing bucket \"error-bucket\"; got %q", err.Error())
	}

	if _, ok := result.Findings["error-bucket"]; ok {
		t.Error("expected no finding when GetPublicAccessBlock returns generic error (data is incomplete)")
	}
	if _, marked := result.TruncatedIDs["error-bucket"]; !marked {
		t.Error("TruncatedIDs[error-bucket] must be true when enrichment incomplete due to API error")
	}
}

func TestS3_Enrich_NilS3Client_GracefulEmpty(t *testing.T) {
	clients := &awsclient.ServiceClients{S3: nil}
	result, err := awsclient.EnrichS3Posture(context.Background(), clients, nil, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil even when S3 client is nil")
	}
	if result.TruncatedIDs == nil {
		t.Error("TruncatedIDs must not be nil even when S3 client is nil")
	}
}

func TestS3_Enrich_IssueCount_FourBuckets(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"a9s-demo-healthy": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(true),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(true),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
			"a9s-demo-nopab": nil,
			"a9s-demo-partial-pab": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(false),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(true),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
			"a9s-demo-multifail-pab": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(false),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(false),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
			"a9s-demo-nilcfg": {
				PublicAccessBlockConfiguration: nil,
			},
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{
		pabResource("a9s-demo-healthy"),
		pabResource("a9s-demo-nopab"),
		pabResource("a9s-demo-partial-pab"),
		pabResource("a9s-demo-multifail-pab"),
		pabResource("a9s-demo-nilcfg"),
	}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	tildeCount := 0
	bangCount := 0
	for _, fs := range result.Findings {
		for _, f := range fs {
			switch f.Severity {
			case domain.SevWarn:
				tildeCount++
			case domain.SevBroken:
				bangCount++
			}
		}
	}
	if tildeCount != 4 {
		t.Errorf("expected 4 '~' findings (4 PAB-issue fixtures), got %d; Findings keys = %v",
			tildeCount, findingKeys(result.Findings))
	}
	if bangCount != 0 {
		t.Errorf("expected 0 '!' findings — s3 Wave 2 PAB findings have no Broken tier, got %d", bangCount)
	}

	if _, ok := result.Findings["a9s-demo-healthy"]; ok {
		t.Error("healthy bucket must not have a finding")
	}
}

// docs/attention-signals.md: a missing or partial bucket-level PAB block is
// a risk, not a certainty (account-level PAB may still apply), so it never
// paints a row red.
func TestS3_Enrich_NeverEmitsBrokenSeverity(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"a9s-demo-nopab": nil,
			"a9s-demo-allflagsfalse": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(false),
					IgnorePublicAcls:      aws.Bool(false),
					BlockPublicPolicy:     aws.Bool(false),
					RestrictPublicBuckets: aws.Bool(false),
				},
			},
			"a9s-demo-nilcfg": {PublicAccessBlockConfiguration: nil},
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{
		pabResource("a9s-demo-nopab"),
		pabResource("a9s-demo-allflagsfalse"),
		pabResource("a9s-demo-nilcfg"),
	}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}
	for id, fs := range result.Findings {
		for _, f := range fs {
			if f.Severity == domain.SevBroken {
				t.Errorf("[%s] Severity = SevBroken, want SevWarn — s3 Wave 2 PAB findings have no Broken tier (even all four flags false)", id)
			}
		}
	}
}

func TestS3_Enrich_U11_SummaryStable_NeverContainsRowValues(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"a9s-demo-nopab": nil,
			"a9s-demo-partial-pab": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(false),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(true),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
			"a9s-demo-multifail-pab": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(false),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(false),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
			"a9s-demo-nilcfg": {
				PublicAccessBlockConfiguration: nil,
			},
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{
		pabResource("a9s-demo-nopab"),
		pabResource("a9s-demo-partial-pab"),
		pabResource("a9s-demo-multifail-pab"),
		pabResource("a9s-demo-nilcfg"),
	}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	for id, findings := range result.Findings {
		for _, finding := range findings {
			for _, row := range result.AttentionDetails[id][finding.Code].Rows {
				if row.Value != "" && strings.Contains(finding.Phrase, row.Value) {
					t.Errorf("[%s] Phrase %q must not contain Row value %q (U11 Phrase≠Rows separation)",
						id, finding.Phrase, row.Value)
				}
			}
		}
	}
}

// On the pinned SDK (s3 v1.102.1) GetPublicAccessBlock has no modeled
// errors, so every failure is a smithy.GenericAPIError carrying the body's
// Code verbatim. A bucket deleted after ListBuckets yields "NoSuchBucket" and
// an empty-body 404 "NotFound": both are a silent truncation, never a
// "public access block incomplete" finding.

func TestS3_Enrich_NoSuchBucket_SilentTruncation_NoFinding(t *testing.T) {
	const deletedBucket = "deleted-bucket"
	fake := &s3PABFake{
		codedErrors: map[string]string{deletedBucket: "NoSuchBucket"},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource(deletedBucket)}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("composite error must be nil when NoSuchBucket is the only failure; got %v", err)
	}
	// This enricher only ever emits the s3.public-access-block-incomplete
	// finding, so "no finding for this bucket" is equivalent to "no false
	// public-access-block-incomplete finding for a deleted bucket".
	if fs, ok := result.Findings[deletedBucket]; ok {
		t.Errorf("expected no finding for a deleted bucket (NoSuchBucket); got %v", fs)
	}
	if _, ok := result.FieldUpdates[deletedBucket]; ok {
		t.Error("expected no FieldUpdates for a deleted bucket (NoSuchBucket)")
	}
	if _, marked := result.TruncatedIDs[deletedBucket]; !marked {
		t.Error("TruncatedIDs[deleted-bucket] must be true — data incomplete because the bucket no longer exists")
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a bucket's PAB state cannot be determined (deleted mid-sweep)")
	}
}

func TestS3_Enrich_NotFound_SilentTruncation_NoFinding(t *testing.T) {
	const deletedBucket = "gone-empty-body-bucket"
	fake := &s3PABFake{
		codedErrors: map[string]string{deletedBucket: "NotFound"},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource(deletedBucket)}

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("composite error must be nil when NotFound is the only failure; got %v", err)
	}
	if fs, ok := result.Findings[deletedBucket]; ok {
		t.Errorf("expected no finding for a deleted bucket (NotFound); got %v", fs)
	}
	if _, ok := result.FieldUpdates[deletedBucket]; ok {
		t.Error("expected no FieldUpdates for a deleted bucket (NotFound)")
	}
	if _, marked := result.TruncatedIDs[deletedBucket]; !marked {
		t.Error("TruncatedIDs[gone-empty-body-bucket] must be true — data incomplete because the bucket no longer exists")
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a bucket's PAB state cannot be determined (deleted mid-sweep)")
	}
}

func TestS3_Enrich_NonNotFoundErrors_StillAggregate(t *testing.T) {
	cases := []struct {
		name   string
		bucket string
		fake   func(bucket string) *s3PABFake
	}{
		{
			name:   "AccessDenied",
			bucket: "locked-bucket",
			fake: func(bucket string) *s3PABFake {
				return &s3PABFake{codedErrors: map[string]string{bucket: "AccessDenied"}}
			},
		},
		{
			name:   "non-APIError/connection-reset",
			bucket: "flaky-bucket",
			fake: func(bucket string) *s3PABFake {
				return &s3PABFake{rawErrors: map[string]error{bucket: fmt.Errorf("connection reset")}}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clients := &awsclient.ServiceClients{S3: c.fake(c.bucket)}
			resources := []resource.Resource{pabResource(c.bucket)}

			result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
			if err == nil {
				t.Fatalf("expected non-nil composite error for %s; NoSuchBucket/NotFound silent-truncation must not swallow other errors", c.name)
			}
			if !strings.Contains(err.Error(), c.bucket) {
				t.Errorf("composite error must name the failing bucket %q; got %q", c.bucket, err.Error())
			}
			if fs, ok := result.Findings[c.bucket]; ok {
				t.Errorf("expected no finding for %s; got %v", c.name, fs)
			}
			if _, marked := result.TruncatedIDs[c.bucket]; !marked {
				t.Errorf("TruncatedIDs[%s] must be true for %s", c.bucket, c.name)
			}
		})
	}
}

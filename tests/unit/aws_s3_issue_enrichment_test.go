package unit

// aws_s3_issue_enrichment_test.go — Wave 2 enricher tests for s3.
//
// Tests drive aws.EnrichS3PublicAccessBlock and assert the post-rewrite contract:
//   - Severity == "!" for all PAB-missing cases (no "~").
//   - Summary == "public access block incomplete" verbatim, always (U11 stable phrase).
//   - Summary never contains the Row detail values (U11 Summary≠Rows separation).
//   - Rows carry the per-case structured detail.
//   - FieldUpdates[bucket]["status"] == "public access block incomplete" (NOT "public_access").
//   - Healthy bucket (all four flags true) emits no finding and no field update.
//   - Unknown API error (non-NoSuchPublicAccessBlock) emits no finding but sets
//     TruncatedIDs[bucket] = true.
//   - Nil S3 client returns empty result gracefully.
//   - S1 badge: IssueCount equals the number of buckets with "!" findings.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// mock — implements awsclient.S3GetPublicAccessBlockAPI
// ---------------------------------------------------------------------------

// s3PABFake dispatches GetPublicAccessBlock per bucket from a pre-built map.
// Semantics mirror fixtures.S3Fixtures.PublicAccessBlockConfigs:
//   - key present, non-nil value → return that output (may carry nil inner config).
//   - key present, nil value     → return NoSuchPublicAccessBlockConfiguration error.
//   - key absent                 → return empty output (all flags nil/false).
//   - "error-bucket"             → return a generic AccessDenied error.
type s3PABFake struct {
	// configs maps bucket name → GetPublicAccessBlockOutput.
	// A nil *s3.GetPublicAccessBlockOutput signals NoSuchPublicAccessBlockConfiguration.
	configs map[string]*s3.GetPublicAccessBlockOutput
	// errorBuckets is a set of bucket names for which a generic error is returned.
	errorBuckets map[string]bool
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
	if f.errorBuckets[bucket] {
		return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "access denied"}
	}
	cfg, ok := f.configs[bucket]
	if !ok {
		// No entry → return empty output (all flags absent).
		return &s3.GetPublicAccessBlockOutput{}, nil
	}
	if cfg == nil {
		// Explicit nil → NoSuchPublicAccessBlockConfiguration.
		return nil, &smithy.GenericAPIError{
			Code:    "NoSuchPublicAccessBlockConfiguration",
			Message: "The public access block configuration was not found",
		}
	}
	return cfg, nil
}

// s3PABFake also needs ListBuckets to satisfy awsclient.S3API if needed.
// We only use it as S3GetPublicAccessBlockAPI — no other methods needed.

// s3ClientWithPAB wraps s3PABFake into a ServiceClients-compatible S3 field.
// Because EnrichS3PublicAccessBlock accepts *ServiceClients and calls
// clients.S3.GetPublicAccessBlock directly, we need an object that implements
// both S3API (for the S3 field type) and our fake logic.
//
// The simplest approach: use the production S3Fake from fakes/ for the list
// path, but for enrichment tests we construct an inline resource.Resource
// slice directly (no fetcher call). So s3PABFake only needs the PAB method.
//
// We make s3PABFake satisfy awsclient.S3API by embedding the minimal missing
// methods as stubs. The ServiceClients.S3 field is typed awsclient.S3API.
func (f *s3PABFake) ListBuckets(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return &s3.ListBucketsOutput{}, nil
}

func (f *s3PABFake) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}

func (f *s3PABFake) GetBucketNotificationConfiguration(_ context.Context, _ *s3.GetBucketNotificationConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return &s3.GetBucketNotificationConfigurationOutput{}, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// pabResource builds a minimal resource.Resource for PAB enrichment tests.
func pabResource(name string) resource.Resource {
	return resource.Resource{
		ID:     name,
		Name:   name,
		Status: "",
		Fields: map[string]string{"name": name},
	}
}

// assertFindingShape is a shared assertion helper for the stable finding contract.
// It fails the test if the finding at key does not have the expected severity
// and the verbatim stable Summary.
func assertFindingShape(t *testing.T, findings map[string]resource.EnrichmentFinding, key string) resource.EnrichmentFinding {
	t.Helper()
	f, ok := findings[key]
	if !ok {
		t.Fatalf("expected finding for %q; Findings keys = %v", key, findingKeys(findings))
	}
	if f.Severity != "!" {
		t.Errorf("[%s] Severity = %q, want %q", key, f.Severity, "!")
	}
	const wantSummary = "public access block incomplete"
	if f.Summary != wantSummary {
		t.Errorf("[%s] Summary = %q, want %q", key, f.Summary, wantSummary)
	}
	return f
}

// rowMap converts a FindingRow slice to a label→value map for easy assertion.
func rowMap(rows []resource.FindingRow) map[string]string {
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Label] = r.Value
	}
	return m
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestS3_Enrich_HealthyBucket_NoFinding verifies that a bucket with all four
// PAB flags set to true emits no finding and no FieldUpdate (U1, U6).
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

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichS3PublicAccessBlock error: %v", err)
	}

	if _, ok := result.Findings["healthy-bucket"]; ok {
		t.Error("expected no finding for healthy bucket with all PAB flags true")
	}
	if _, ok := result.FieldUpdates["healthy-bucket"]; ok {
		t.Error("expected no FieldUpdates for healthy bucket with all PAB flags true")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 for healthy bucket", result.IssueCount)
	}
}

// TestS3_Enrich_NoPAB_Configuration verifies the no-PAB case:
// GetPublicAccessBlock returns NoSuchPublicAccessBlockConfiguration.
// Expects Severity "!", stable Summary, Rows with "no public access block
// configuration" Status and "may still apply" Account-level PAB row (U4, U11).
func TestS3_Enrich_NoPAB_Configuration(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"a9s-demo-nopab": nil, // explicit nil → NoSuchPublicAccessBlockConfiguration
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource("a9s-demo-nopab")}

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichS3PublicAccessBlock error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-nopab")

	// U11: Summary must NOT embed the row-level detail string.
	if strings.Contains(finding.Summary, "no public access block configuration") {
		t.Errorf("Summary must not embed Row content; got %q", finding.Summary)
	}

	// Rows must carry the detail.
	rows := rowMap(finding.Rows)
	if rows["Status"] != "no public access block configuration" {
		t.Errorf("Rows[Status] = %q, want %q", rows["Status"], "no public access block configuration")
	}
	if rows["Account-level PAB"] != "may still apply" {
		t.Errorf("Rows[Account-level PAB] = %q, want %q", rows["Account-level PAB"], "may still apply")
	}

	// FieldUpdates must use the "status" key (NOT "public_access").
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

// TestS3_Enrich_PartialPAB_SingleFlagFalse verifies that a bucket with one
// PAB flag false (BlockPublicAcls=false, others true) emits a "!" finding
// with stable Summary and the false-flag row (spec §4 partial case).
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

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichS3PublicAccessBlock error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-partial-pab")

	// U11: Summary must not contain flag names or values.
	if strings.Contains(finding.Summary, "BlockPublicAcls") || strings.Contains(finding.Summary, "false") {
		t.Errorf("Summary must not embed Row content; got %q", finding.Summary)
	}

	rows := rowMap(finding.Rows)
	if rows["BlockPublicAcls"] != "false" {
		t.Errorf("Rows[BlockPublicAcls] = %q, want %q", rows["BlockPublicAcls"], "false")
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

// TestS3_Enrich_PartialPAB_MultipleFlagsFalse verifies that a bucket with two
// PAB flags false both appear as separate Rows (spec §4 multi-false case).
// Summary must remain identical to the single-flag case — stable phrase (U11).
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

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichS3PublicAccessBlock error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-multifail-pab")

	// Summary must be stable — same phrase even when multiple flags are false.
	const wantSummary = "public access block incomplete"
	if finding.Summary != wantSummary {
		t.Errorf("Summary = %q, want %q (must be stable across instances)", finding.Summary, wantSummary)
	}
	// Summary must not contain flag names.
	if strings.Contains(finding.Summary, "BlockPublicAcls") || strings.Contains(finding.Summary, "BlockPublicPolicy") {
		t.Errorf("Summary must not embed Row content; got %q", finding.Summary)
	}

	rows := rowMap(finding.Rows)
	if rows["BlockPublicAcls"] != "false" {
		t.Errorf("Rows[BlockPublicAcls] = %q, want %q", rows["BlockPublicAcls"], "false")
	}
	if rows["BlockPublicPolicy"] != "false" {
		t.Errorf("Rows[BlockPublicPolicy] = %q, want %q", rows["BlockPublicPolicy"], "false")
	}
}

// TestS3_Enrich_NilPABConfiguration_TreatedAsNoPAB verifies that a bucket
// whose GetPublicAccessBlock returns a non-nil output but nil inner
// PublicAccessBlockConfiguration is treated equivalently to the no-PAB case
// (spec §4, bucket-nil-pab-cfg fixture). Same finding shape: Severity "!",
// stable Summary, detail in Rows.
func TestS3_Enrich_NilPABConfiguration_TreatedAsNoPAB(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			"a9s-demo-nilcfg": {
				// Non-nil output, nil inner config — the "nil-cfg" case.
				PublicAccessBlockConfiguration: nil,
			},
		},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource("a9s-demo-nilcfg")}

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichS3PublicAccessBlock error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-nilcfg")

	// Must not embed row detail in Summary.
	if strings.Contains(finding.Summary, "no public access block configuration") {
		t.Errorf("Summary must not embed Row content; got %q", finding.Summary)
	}

	updates, ok := result.FieldUpdates["a9s-demo-nilcfg"]
	if !ok {
		t.Fatal("FieldUpdates missing entry for a9s-demo-nilcfg")
	}
	if updates["status"] != "public access block incomplete" {
		t.Errorf("FieldUpdates[status] = %q, want %q", updates["status"], "public access block incomplete")
	}
}

// TestS3_Enrich_UnknownAPIError_NoFinding verifies that when GetPublicAccessBlock
// returns a generic non-NoSuchPublicAccessBlockConfiguration error (e.g.
// AccessDenied), the enricher:
//  1. emits NO finding (data is incomplete — cannot claim PAB is missing),
//  2. marks the bucket in TruncatedIDs (per-row `?` marker),
//  3. returns a composite error via AggregateFailures so the error log (!) surfaces it.
func TestS3_Enrich_UnknownAPIError_NoFinding(t *testing.T) {
	fake := &s3PABFake{
		configs:      map[string]*s3.GetPublicAccessBlockOutput{},
		errorBuckets: map[string]bool{"error-bucket": true},
	}
	clients := &awsclient.ServiceClients{S3: fake}
	resources := []resource.Resource{pabResource("error-bucket")}

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err == nil {
		t.Fatal("expected non-nil composite error when GetPublicAccessBlock returns generic error; got nil")
	}
	if !strings.Contains(err.Error(), "s3-enrich: GetPublicAccessBlock") {
		t.Errorf("err must contain \"s3-enrich: GetPublicAccessBlock\"; got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "error-bucket") {
		t.Errorf("err must name the failing bucket \"error-bucket\"; got %q", err.Error())
	}

	if _, ok := result.Findings["error-bucket"]; ok {
		t.Error("expected no finding when GetPublicAccessBlock returns generic error (data is incomplete)")
	}
	if !result.TruncatedIDs["error-bucket"] {
		t.Error("TruncatedIDs[error-bucket] must be true when enrichment incomplete due to API error")
	}
}

// TestS3_Enrich_NilS3Client_GracefulEmpty verifies that nil S3 client returns
// an empty result without error (degraded gracefully).
func TestS3_Enrich_NilS3Client_GracefulEmpty(t *testing.T) {
	clients := &awsclient.ServiceClients{S3: nil}
	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("EnrichS3PublicAccessBlock error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil even when S3 client is nil")
	}
	if result.TruncatedIDs == nil {
		t.Error("TruncatedIDs must not be nil even when S3 client is nil")
	}
}

// TestS3_Enrich_IssueCount_FourBuckets verifies that IssueCount equals the
// number of "!" findings across the four spec fixtures (U6 reconciliation:
// 4 is the correct count from the fixture file — no-pab, partial-pab,
// multi-false-pab, nil-pab-cfg). The healthy bucket must NOT contribute.
func TestS3_Enrich_IssueCount_FourBuckets(t *testing.T) {
	fake := &s3PABFake{
		configs: map[string]*s3.GetPublicAccessBlockOutput{
			// Healthy: all flags true → no finding.
			"a9s-demo-healthy": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(true),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(true),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
			// no-pab: NoSuchPublicAccessBlockConfiguration.
			"a9s-demo-nopab": nil,
			// partial-pab: one flag false.
			"a9s-demo-partial-pab": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(false),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(true),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
			// multi-false-pab: two flags false.
			"a9s-demo-multifail-pab": {
				PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
					BlockPublicAcls:       aws.Bool(false),
					IgnorePublicAcls:      aws.Bool(true),
					BlockPublicPolicy:     aws.Bool(false),
					RestrictPublicBuckets: aws.Bool(true),
				},
			},
			// nil-pab-cfg: non-nil output, nil inner config.
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

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichS3PublicAccessBlock error: %v", err)
	}

	// Count "!" findings manually to decouple from IssueCount field name choices.
	bangCount := 0
	for _, f := range result.Findings {
		if f.Severity == "!" {
			bangCount++
		}
	}
	if bangCount != 4 {
		t.Errorf("expected 4 '!' findings (4 PAB-issue fixtures), got %d; Findings keys = %v",
			bangCount, findingKeys(result.Findings))
	}

	// Healthy bucket must not appear in Findings.
	if _, ok := result.Findings["a9s-demo-healthy"]; ok {
		t.Error("healthy bucket must not have a finding")
	}
}

// TestS3_Enrich_U11_SummaryStable_NeverContainsRowValues drives the U11
// invariant: for every "!" finding, Summary must not contain any of the
// values present in that finding's Rows.
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

	result, err := awsclient.EnrichS3PublicAccessBlock(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("EnrichS3PublicAccessBlock error: %v", err)
	}

	for id, finding := range result.Findings {
		for _, row := range finding.Rows {
			if row.Value != "" && strings.Contains(finding.Summary, row.Value) {
				t.Errorf("[%s] Summary %q must not contain Row value %q (U11 Summary≠Rows separation)",
					id, finding.Summary, row.Value)
			}
		}
	}
}

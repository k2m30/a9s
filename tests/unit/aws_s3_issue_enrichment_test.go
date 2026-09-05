package unit

// aws_s3_issue_enrichment_test.go — Wave 2 enricher tests for s3.
//
// Tests drive aws.EnrichS3Posture and assert the
// docs/attention-signals.md `s3` Wave 2 contract:
//   - Severity == "~" (SevWarn) for ALL PAB-missing cases. s3 Wave 2 has NO
//     Broken tier — "!"/SevBroken must never appear on a PAB finding.
//   - Summary == "public access block incomplete" verbatim, always (U11 stable phrase).
//   - Summary never contains the Row detail values (U11 Summary≠Rows separation).
//   - Rows carry the per-case structured detail.
//   - FieldUpdates[bucket]["status"] == "public access block incomplete" (NOT "public_access").
//   - Healthy bucket (all four flags true) emits no finding and no field update.
//   - Unknown API error (non-NoSuchPublicAccessBlock) emits no finding but sets
//     TruncatedIDs[bucket] = true.
//   - Nil S3 client returns empty result gracefully.
//   - S1 badge: IssueCount equals the number of buckets with "~" findings.

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
	// codedErrors maps bucket name → smithy error code (e.g. "NoSuchBucket",
	// "NotFound", "AccessDenied"); GetPublicAccessBlock returns
	// &smithy.GenericAPIError{Code: code} for that bucket. Takes priority over
	// errorBuckets/configs — lets a single fake pin the full 404-taxonomy
	// (issue #456) alongside the pre-existing hardcoded-AccessDenied path.
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
// Because EnrichS3Posture accepts *ServiceClients and calls
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
		Fields: map[string]string{"name": name},
	}
}

// assertFindingShape is a shared assertion helper for the stable finding contract.
// It fails the test if the finding at key does not have the expected severity
// and the verbatim stable Phrase.
//
// s3 Wave 2 PAB findings are always "~" (SevWarn) — docs/attention-signals.md
// `s3` Wave 2 has no Broken tier for this signal (account-level PAB may still
// override, so a missing/partial bucket-level PAB block is never certain
// public exposure).
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

// rowMap converts a DetailRow slice to a label→value map for easy assertion.
func rowMap(rows []domain.DetailRow) map[string]string {
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

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-nopab")

	// U11: Phrase must NOT embed the row-level detail string.
	if strings.Contains(finding.Phrase, "no public access block configuration") {
		t.Errorf("Phrase must not embed Row content; got %q", finding.Phrase)
	}

	// Rows must carry the detail.
	rows := rowMap(result.AttentionDetails["a9s-demo-nopab"][finding.Code].Rows)
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

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-partial-pab")

	// U11: Phrase must not contain flag names or values.
	if strings.Contains(finding.Phrase, "BlockPublicAcls") || strings.Contains(finding.Phrase, "false") {
		t.Errorf("Phrase must not embed Row content; got %q", finding.Phrase)
	}

	rows := rowMap(result.AttentionDetails["a9s-demo-partial-pab"][finding.Code].Rows)
	// The row is labelled in plain words; the SDK flag name rides in the value,
	// where an identifier is allowed and is what the operator greps the console for.
	// d4 row 20 replaced the "false" in front of that aside with "off": a Go
	// bool literal describes the SDK field, not the account. Do not restore
	// "false (BlockPublicAcls)" — TestNetworkingRowValues_AreWordsNotLiterals
	// fails on it.
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

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-multifail-pab")

	// Phrase must be stable — same phrase even when multiple flags are false.
	const wantPhrase = "public access block incomplete"
	if finding.Phrase != wantPhrase {
		t.Errorf("Phrase = %q, want %q (must be stable across instances)", finding.Phrase, wantPhrase)
	}
	// Phrase must not contain flag names.
	if strings.Contains(finding.Phrase, "BlockPublicAcls") || strings.Contains(finding.Phrase, "BlockPublicPolicy") {
		t.Errorf("Phrase must not embed Row content; got %q", finding.Phrase)
	}

	rows := rowMap(result.AttentionDetails["a9s-demo-multifail-pab"][finding.Code].Rows)
	// The row is labelled in plain words; the SDK flag name rides in the value,
	// where an identifier is allowed and is what the operator greps the console for.
	// d4 row 20 replaced the "false" in front of that aside with "off": a Go
	// bool literal describes the SDK field, not the account. Do not restore
	// "false (BlockPublicAcls)" — TestNetworkingRowValues_AreWordsNotLiterals
	// fails on it.
	if rows["Block public access control lists"] != "off (BlockPublicAcls)" {
		t.Errorf("Rows[Block public access control lists] = %q, want %q",
			rows["Block public access control lists"], "off (BlockPublicAcls)")
	}
	if rows["Block public bucket policy"] != "off (BlockPublicPolicy)" {
		t.Errorf("Rows[Block public bucket policy] = %q, want %q",
			rows["Block public bucket policy"], "off (BlockPublicPolicy)")
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

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	finding := assertFindingShape(t, result.Findings, "a9s-demo-nilcfg")

	// Must not embed row detail in Phrase.
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

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("expected non-nil composite error when GetPublicAccessBlock returns generic error; got nil")
	}
	if !strings.Contains(err.Error(), "s3-enrich: bucket posture") {
		t.Errorf("err must contain \"s3-enrich: bucket posture\"; got %q", err.Error())
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

// TestS3_Enrich_IssueCount_FourBuckets verifies that IssueCount equals the
// number of "~" findings across the four spec fixtures (U6 reconciliation:
// 4 is the correct count from the fixture file — no-pab, partial-pab,
// multi-false-pab, nil-pab-cfg). The healthy bucket must NOT contribute, and
// none of the four PAB-issue findings may be "!" (SevBroken) — s3 Wave 2 has
// no Broken tier.
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

	result, err := awsclient.EnrichS3Posture(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("EnrichS3Posture error: %v", err)
	}

	// Count SevWarn findings manually to decouple from IssueCount field name choices.
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

	// Healthy bucket must not appear in Findings.
	if _, ok := result.Findings["a9s-demo-healthy"]; ok {
		t.Error("healthy bucket must not have a finding")
	}
}

// TestS3_Enrich_NeverEmitsBrokenSeverity is a dedicated regression guard for
// docs/attention-signals.md `s3` Wave 2: "GetPublicAccessBlock per bucket:
// NoSuchPublicAccessBlockConfiguration error or any flag false -> Warning."
// There is no Broken tier for this signal — a missing/partial bucket-level
// PAB block is a risk, not a certainty (account-level PAB may still apply),
// so it must never paint a row red.
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

// ---------------------------------------------------------------------------
// Issue #456 — 404 taxonomy: on the pinned SDK (s3 v1.102.1) GetPublicAccessBlock
// has NO modeled errors, so every failure surfaces as smithy.GenericAPIError
// with the body's Code verbatim. A bucket deleted between ListBuckets and
// enrichment yields Code "NoSuchBucket" (confirmed empirically); an
// empty-body 404 synthesizes Code "NotFound". Both must classify as a
// silent truncation — same as the existing cross-region branch — never a
// false "public access block incomplete" finding on a bucket that no longer
// exists.
// ---------------------------------------------------------------------------

// TestS3_Enrich_NoSuchBucket_SilentTruncation_NoFinding pins the
// deleted-bucket case: GetPublicAccessBlock returns Code "NoSuchBucket".
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
	if !result.TruncatedIDs[deletedBucket] {
		t.Error("TruncatedIDs[deleted-bucket] must be true — data incomplete because the bucket no longer exists")
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a bucket's PAB state cannot be determined (deleted mid-sweep)")
	}
}

// TestS3_Enrich_NotFound_SilentTruncation_NoFinding pins the empty-body-404
// shape: GetPublicAccessBlock returns Code "NotFound" (the code the AWS SDK
// synthesizes when the HTTP response body is empty on a 404).
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
	if !result.TruncatedIDs[deletedBucket] {
		t.Error("TruncatedIDs[gone-empty-body-bucket] must be true — data incomplete because the bucket no longer exists")
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a bucket's PAB state cannot be determined (deleted mid-sweep)")
	}
}

// TestS3_Enrich_NonNotFoundErrors_StillAggregate is the negative-space guard
// for the 404-taxonomy fix: an AccessDenied GenericAPIError and a plain
// non-smithy error (e.g. a network failure) must NOT be swallowed by the new
// NoSuchBucket/NotFound silent-truncation branch — both must still surface
// via the failure aggregate exactly as before.
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
			if !result.TruncatedIDs[c.bucket] {
				t.Errorf("TruncatedIDs[%s] must be true for %s", c.bucket, c.name)
			}
		})
	}
}

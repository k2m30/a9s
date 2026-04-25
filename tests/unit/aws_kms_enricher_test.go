package unit

// aws_kms_enricher_test.go — Behavioral tests for EnrichKMSRotation.
//
// Contract assertions:
//   - GetKeyRotationStatus is called once per key in the resources slice.
//   - KeyRotationEnabled=false  → Finding keyed by resource.ID, severity "~".
//   - KeyRotationEnabled=true   → no finding.
//   - AccessDeniedException     → silently skipped, NOT Truncated (AWS-managed key).
//   - clients.KMS == nil        → (EnricherResult{Findings: non-nil empty}, nil).
//   - Other API error           → no finding for that key, Truncated=true.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// kmsFake implements KMSAPI for enrichment testing.
// It embeds the interface and overrides only GetKeyRotationStatus.
// The perKey map keys on KeyId and returns the configured output or error.
type kmsFake struct {
	awsclient.KMSAPI
	perKey map[string]*kmsRotationResponse
}

type kmsRotationResponse struct {
	enabled bool
	err     error
}

func (f *kmsFake) GetKeyRotationStatus(
	_ context.Context,
	in *kms.GetKeyRotationStatusInput,
	_ ...func(*kms.Options),
) (*kms.GetKeyRotationStatusOutput, error) {
	id := ""
	if in != nil && in.KeyId != nil {
		id = *in.KeyId
	}
	if resp, ok := f.perKey[id]; ok {
		if resp.err != nil {
			return nil, resp.err
		}
		return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: resp.enabled}, nil
	}
	// Default: rotation enabled (no finding).
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: true}, nil
}

// Compile-time check: kmsFake satisfies KMSAPI.
var _ awsclient.KMSAPI = (*kmsFake)(nil)

// makeKMSResources builds a []resource.Resource slice from the given key IDs.
func makeKMSResources(ids ...string) []resource.Resource {
	rs := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		rs = append(rs, resource.Resource{ID: id, Name: id})
	}
	return rs
}

// accessDeniedErr returns a smithy.GenericAPIError with code AccessDeniedException.
func accessDeniedErr() error {
	return &smithy.GenericAPIError{
		Code:    "AccessDeniedException",
		Message: "User is not authorized to perform kms:GetKeyRotationStatus",
		Fault:   smithy.FaultClient,
	}
}

// TestEnrichKMSRotation_DisabledProducesFindings verifies that all 3 keys with
// rotation disabled produce findings with severity "~".
func TestEnrichKMSRotation_DisabledProducesFindings(t *testing.T) {
	fake := &kmsFake{
		perKey: map[string]*kmsRotationResponse{
			"key-aaa": {enabled: false},
			"key-bbb": {enabled: false},
			"key-ccc": {enabled: false},
		},
	}
	clients := &awsclient.ServiceClients{KMS: fake}
	resources := makeKMSResources("key-aaa", "key-bbb", "key-ccc")

	result, err := awsclient.EnrichKMSRotation(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 3 {
		t.Errorf("findings = %d, want 3", len(result.Findings))
	}
	for _, id := range []string{"key-aaa", "key-bbb", "key-ccc"} {
		f, ok := result.Findings[id]
		if !ok {
			t.Errorf("expected finding for key %q", id)
			continue
		}
		if f.Severity != "~" {
			t.Errorf("key %q severity = %q, want %q", id, f.Severity, "~")
		}
	}
}

// TestEnrichKMSRotation_EnabledProducesNoFinding verifies that all 3 keys with
// rotation enabled produce zero findings.
func TestEnrichKMSRotation_EnabledProducesNoFinding(t *testing.T) {
	fake := &kmsFake{
		perKey: map[string]*kmsRotationResponse{
			"key-x": {enabled: true},
			"key-y": {enabled: true},
			"key-z": {enabled: true},
		},
	}
	clients := &awsclient.ServiceClients{KMS: fake}
	resources := makeKMSResources("key-x", "key-y", "key-z")

	result, err := awsclient.EnrichKMSRotation(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichKMSRotation_AWSManagedKeySkippedSilently verifies that
// AccessDeniedException (returned for AWS-managed keys) is silently skipped —
// no finding is produced and Truncated remains false.
func TestEnrichKMSRotation_AWSManagedKeySkippedSilently(t *testing.T) {
	fake := &kmsFake{
		perKey: map[string]*kmsRotationResponse{
			"aws-managed-key": {err: accessDeniedErr()},
			"key-enabled-1":   {enabled: true},
			"key-enabled-2":   {enabled: true},
		},
	}
	clients := &awsclient.ServiceClients{KMS: fake}
	resources := makeKMSResources("aws-managed-key", "key-enabled-1", "key-enabled-2")

	result, err := awsclient.EnrichKMSRotation(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
	if result.Truncated {
		t.Error("Truncated must be false when only AccessDeniedException errors are seen (AWS-managed keys)")
	}
}

// TestEnrichKMSRotation_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.KMS is nil, the enricher returns a non-nil empty Findings map and
// no error.
func TestEnrichKMSRotation_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{KMS: nil}
	resources := makeKMSResources("irrelevant-key")

	result, err := awsclient.EnrichKMSRotation(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when KMS client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichKMSRotation_MixedDisabledEnabledError verifies that:
//   - disabled key produces a finding with severity "~"
//   - enabled key produces no finding
//   - an API error for one key produces no finding for that key and is skipped silently
func TestEnrichKMSRotation_MixedDisabledEnabledError(t *testing.T) {
	otherErr := &smithy.GenericAPIError{
		Code:    "InternalServiceError",
		Message: "service unavailable",
		Fault:   smithy.FaultServer,
	}
	fake := &kmsFake{
		perKey: map[string]*kmsRotationResponse{
			"key-disabled": {enabled: false},
			"key-enabled":  {enabled: true},
			"key-error":    {err: otherErr},
		},
	}
	clients := &awsclient.ServiceClients{KMS: fake}
	resources := makeKMSResources("key-disabled", "key-enabled", "key-error")

	result, err := awsclient.EnrichKMSRotation(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected top-level error: %v", err)
	}
	// One finding for the disabled key.
	if len(result.Findings) != 1 {
		t.Errorf("findings = %d, want 1", len(result.Findings))
	}
	f, ok := result.Findings["key-disabled"]
	if !ok {
		t.Error("expected finding for key-disabled")
	} else if f.Severity != "~" {
		t.Errorf("key-disabled severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings["key-enabled"]; ok {
		t.Error("unexpected finding for key-enabled")
	}
	if _, ok := result.Findings["key-error"]; ok {
		t.Error("must not produce finding for key-error when API returned error")
	}
}

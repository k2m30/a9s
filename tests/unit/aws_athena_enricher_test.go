package unit

// aws_athena_enricher_test.go — Behavioral tests for EnrichAthenaWorkGroup.
//
// Contract assertions:
//   - GetWorkGroup is called once per Athena workgroup resource.
//   - Both WGs have EnforceWorkGroupConfiguration=true and non-nil EncryptionConfiguration → 0 findings.
//   - WG-1 has EnforceWorkGroupConfiguration=false → 1 finding sev "~" containing "Enforce".
//   - WG-1 has EncryptionConfiguration=nil → 1 finding sev "~" containing "encryption".
//   - clients.Athena == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - Generic API error for a resource → 0 findings for that resource, Truncated=true, no error.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	athenasvc "github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// athenaGetWorkGroupFake implements AthenaAPI for enrichment testing.
// It embeds the aggregate interface and overrides only GetWorkGroup.
// The results map is keyed by workgroup name so the fake can serve different
// responses per resource. errByName overrides results when set.
type athenaGetWorkGroupFake struct {
	awsclient.AthenaAPI
	// results maps workgroup name → WorkGroup.
	results map[string]*athenatypes.WorkGroup
	// errByName maps workgroup name → error; overrides results when set.
	errByName map[string]error
}

func (f *athenaGetWorkGroupFake) GetWorkGroup(
	_ context.Context,
	in *athenasvc.GetWorkGroupInput,
	_ ...func(*athenasvc.Options),
) (*athenasvc.GetWorkGroupOutput, error) {
	name := ""
	if in != nil && in.WorkGroup != nil {
		name = *in.WorkGroup
	}
	if f.errByName != nil {
		if err, ok := f.errByName[name]; ok {
			return nil, err
		}
	}
	wg, ok := f.results[name]
	if !ok {
		return &athenasvc.GetWorkGroupOutput{}, nil
	}
	return &athenasvc.GetWorkGroupOutput{WorkGroup: wg}, nil
}

// Compile-time check: athenaGetWorkGroupFake satisfies AthenaAPI.
var _ awsclient.AthenaAPI = (*athenaGetWorkGroupFake)(nil)

// athenaWorkGroupResources returns a slice of Athena Resource stubs with the given
// workgroup names.
func athenaWorkGroupResources(names ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(names))
	for _, name := range names {
		res = append(res, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "ENABLED",
			Fields: map[string]string{
				"name":        name,
				"state":       "ENABLED",
				"description": "test workgroup " + name,
			},
		})
	}
	return res
}

// athenaWorkGroup builds a WorkGroup with the provided EnforceWorkGroupConfiguration
// flag and optional EncryptionConfiguration (via ResultConfiguration).
func athenaWorkGroup(name string, enforce bool, encryptionCfg *athenatypes.EncryptionConfiguration) *athenatypes.WorkGroup {
	wgCfg := &athenatypes.WorkGroupConfiguration{
		EnforceWorkGroupConfiguration: aws.Bool(enforce),
	}
	if encryptionCfg != nil {
		wgCfg.ResultConfiguration = &athenatypes.ResultConfiguration{
			EncryptionConfiguration: encryptionCfg,
		}
	}
	return &athenatypes.WorkGroup{
		Name:          aws.String(name),
		Configuration: wgCfg,
	}
}

// sseS3Encryption returns a minimal EncryptionConfiguration using SSE_S3.
func sseS3Encryption() *athenatypes.EncryptionConfiguration {
	return &athenatypes.EncryptionConfiguration{
		EncryptionOption: athenatypes.EncryptionOptionSseS3,
	}
}

const (
	athenaWG1 = "primary"
	athenaWG2 = "analytics"
)

// TestEnrichAthenaWorkGroup_EnforcedEncryptedProducesNoFindings verifies that when both
// workgroups have EnforceWorkGroupConfiguration=true and a non-nil
// EncryptionConfiguration, the enricher produces 0 findings and IssueCount=0.
func TestEnrichAthenaWorkGroup_EnforcedEncryptedProducesNoFindings(t *testing.T) {
	fake := &athenaGetWorkGroupFake{
		results: map[string]*athenatypes.WorkGroup{
			athenaWG1: athenaWorkGroup(athenaWG1, true, sseS3Encryption()),
			athenaWG2: athenaWorkGroup(athenaWG2, true, sseS3Encryption()),
		},
	}
	clients := &awsclient.ServiceClients{Athena: fake}
	resources := athenaWorkGroupResources(athenaWG1, athenaWG2)

	result, err := awsclient.EnrichAthenaWorkGroup(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichAthenaWorkGroup_NotEnforcedProducesFindingSevTilde verifies that when WG-1
// has EnforceWorkGroupConfiguration=false, a finding with severity "~" and a summary
// containing "Enforce" is produced for WG-1 only. WG-2 is correctly configured and
// produces no finding.
func TestEnrichAthenaWorkGroup_NotEnforcedProducesFindingSevTilde(t *testing.T) {
	fake := &athenaGetWorkGroupFake{
		results: map[string]*athenatypes.WorkGroup{
			athenaWG1: athenaWorkGroup(athenaWG1, false, sseS3Encryption()),
			athenaWG2: athenaWorkGroup(athenaWG2, true, sseS3Encryption()),
		},
	}
	clients := &awsclient.ServiceClients{Athena: fake}
	resources := athenaWorkGroupResources(athenaWG1, athenaWG2)

	result, err := awsclient.EnrichAthenaWorkGroup(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[athenaWG1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (not enforced)", athenaWG1)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if !strings.Contains(f.Phrase, "Enforce") {
		t.Errorf("summary %q must contain \"Enforce\"", f.Phrase)
	}
	if _, ok := result.Findings[athenaWG2]; ok {
		t.Error("WG-2 must NOT appear in Findings — it is correctly configured")
	}
	// "~" severity does NOT contribute to IssueCount per the EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichAthenaWorkGroup_NoEncryptionProducesFindingSevTilde verifies that when WG-1
// has EncryptionConfiguration=nil (ResultConfiguration is nil or EncryptionConfiguration
// field is absent), a finding with severity "~" and a summary containing "encryption"
// is produced for WG-1 only.
func TestEnrichAthenaWorkGroup_NoEncryptionProducesFindingSevTilde(t *testing.T) {
	fake := &athenaGetWorkGroupFake{
		results: map[string]*athenatypes.WorkGroup{
			athenaWG1: athenaWorkGroup(athenaWG1, true, nil), // no encryption
			athenaWG2: athenaWorkGroup(athenaWG2, true, sseS3Encryption()),
		},
	}
	clients := &awsclient.ServiceClients{Athena: fake}
	resources := athenaWorkGroupResources(athenaWG1, athenaWG2)

	result, err := awsclient.EnrichAthenaWorkGroup(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[athenaWG1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no encryption)", athenaWG1)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if !strings.Contains(strings.ToLower(f.Phrase), "encryption") {
		t.Errorf("summary %q must contain \"encryption\"", f.Phrase)
	}
	if _, ok := result.Findings[athenaWG2]; ok {
		t.Error("WG-2 must NOT appear in Findings — it has encryption configured")
	}
	// "~" severity does NOT contribute to IssueCount per the EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichAthenaWorkGroup_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.Athena is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichAthenaWorkGroup_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{Athena: nil}

	result, err := awsclient.EnrichAthenaWorkGroup(context.Background(), clients, athenaWorkGroupResources(athenaWG1, athenaWG2), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when Athena client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichAthenaWorkGroup_APIErrorSetsTruncatedNoError verifies that when the API
// call for WG-1 returns a generic error, the enricher sets Truncated=true, produces
// 0 findings for that workgroup, and does not propagate the error.
func TestEnrichAthenaWorkGroup_APIErrorSetsTruncatedNoError(t *testing.T) {
	apiErr := errors.New("athena: GetWorkGroup throttled")
	fake := &athenaGetWorkGroupFake{
		errByName: map[string]error{
			athenaWG1: apiErr,
		},
		results: map[string]*athenatypes.WorkGroup{
			athenaWG2: athenaWorkGroup(athenaWG2, true, sseS3Encryption()),
		},
	}
	clients := &awsclient.ServiceClients{Athena: fake}
	resources := athenaWorkGroupResources(athenaWG1, athenaWG2)

	result, err := awsclient.EnrichAthenaWorkGroup(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when a generic API call fails")
	}
}

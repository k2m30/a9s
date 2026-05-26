package unit

// aws_iam_role_enricher_test.go — Behavioral tests for EnrichIAMRoleLastUsed.
//
// Contract:
//   - GetRole is called once per role resource (keyed by role name).
//   - Role with RoleLastUsed.LastUsedDate = now → 0 findings.
//   - Role with RoleLastUsed.LastUsedDate = now-100d → 1 finding sev "~" (dormant).
//   - Role with RoleLastUsed = nil (never used) → 1 finding sev "~" (dormant).
//   - Role with Path starting with /aws-service-role/ → skipped (0 findings) even if never used.
//   - clients.IAM == nil → 0 findings, no error.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// iamGetRoleFake implements IAMAPI for role enrichment testing.
// It embeds the interface and overrides only GetRole.
// The results map is keyed by RoleName so the fake can serve different
// responses per resource.
type iamGetRoleFake struct {
	awsclient.IAMAPI
	// results maps RoleName → Role.
	results map[string]*iamtypes.Role
	// errByName maps RoleName → error; overrides results when set.
	errByName map[string]error
}

func (f *iamGetRoleFake) GetRole(
	_ context.Context,
	in *iam.GetRoleInput,
	_ ...func(*iam.Options),
) (*iam.GetRoleOutput, error) {
	name := ""
	if in != nil && in.RoleName != nil {
		name = *in.RoleName
	}
	if f.errByName != nil {
		if err, ok := f.errByName[name]; ok {
			return nil, err
		}
	}
	role, ok := f.results[name]
	if !ok {
		return &iam.GetRoleOutput{}, nil
	}
	return &iam.GetRoleOutput{Role: role}, nil
}

// Compile-time check: iamGetRoleFake satisfies IAMAPI.
var _ awsclient.IAMAPI = (*iamGetRoleFake)(nil)

// iamRoleResources returns a slice of 3 role Resource stubs.
// roles[0]=role-1, roles[1]=role-2, roles[2]=role-3.
func iamRoleResources() []resource.Resource {
	return []resource.Resource{
		{
			ID:   "role-1",
			Name: "role-1",
			Fields: map[string]string{
				"role_name": "role-1",
				"path":      "/",
			},
		},
		{
			ID:   "role-2",
			Name: "role-2",
			Fields: map[string]string{
				"role_name": "role-2",
				"path":      "/",
			},
		},
		{
			ID:   "role-3",
			Name: "role-3",
			Fields: map[string]string{
				"role_name": "role-3",
				"path":      "/",
			},
		},
	}
}

// iamRoleWithLastUsed builds a Role with RoleLastUsed set to the given offset from now.
func iamRoleWithLastUsed(name, path string, offset time.Duration) *iamtypes.Role {
	lastUsed := time.Now().Add(offset)
	return &iamtypes.Role{
		RoleName: aws.String(name),
		Path:     aws.String(path),
		RoleLastUsed: &iamtypes.RoleLastUsed{
			LastUsedDate: &lastUsed,
		},
	}
}

// iamRoleNeverUsed builds a Role with RoleLastUsed nil (no LastUsedDate).
func iamRoleNeverUsed(name, path string) *iamtypes.Role {
	return &iamtypes.Role{
		RoleName:     aws.String(name),
		Path:         aws.String(path),
		RoleLastUsed: nil,
	}
}

// TestEnrichIAMRoleLastUsed_RecentlyUsedProducesNoFindings verifies that when
// role-1 was last used today, no finding is produced for it.
func TestEnrichIAMRoleLastUsed_RecentlyUsedProducesNoFindings(t *testing.T) {
	fake := &iamGetRoleFake{
		results: map[string]*iamtypes.Role{
			"role-1": iamRoleWithLastUsed("role-1", "/", -1*time.Hour),
			"role-2": iamRoleWithLastUsed("role-2", "/", -1*time.Hour),
			"role-3": iamRoleWithLastUsed("role-3", "/", -1*time.Hour),
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamRoleResources()

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, resources, nil)
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

// TestEnrichIAMRoleLastUsed_DormantProducesFindingSevTilde verifies that when
// role-1 was last used 100 days ago, a finding with severity "~" is produced
// for role-1. role-2 and role-3 (recently used) produce no finding.
func TestEnrichIAMRoleLastUsed_DormantProducesFindingSevTilde(t *testing.T) {
	fake := &iamGetRoleFake{
		results: map[string]*iamtypes.Role{
			"role-1": iamRoleWithLastUsed("role-1", "/", -100*24*time.Hour),
			"role-2": iamRoleWithLastUsed("role-2", "/", -1*time.Hour),
			"role-3": iamRoleWithLastUsed("role-3", "/", -1*time.Hour),
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamRoleResources()

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["role-1"]
	if !ok {
		t.Fatalf("expected finding keyed by %q (dormant role)", "role-1")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings["role-2"]; ok {
		t.Error("role-2 must NOT appear in Findings — it was recently used")
	}
	if _, ok := result.Findings["role-3"]; ok {
		t.Error("role-3 must NOT appear in Findings — it was recently used")
	}
	// "~" findings do not contribute to IssueCount.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichIAMRoleLastUsed_NeverUsedProducesFindingSevTilde verifies that when
// role-1 has RoleLastUsed=nil (never used), a finding with severity "~" is
// produced for role-1. role-2 and role-3 (recently used) produce no finding.
func TestEnrichIAMRoleLastUsed_NeverUsedProducesFindingSevTilde(t *testing.T) {
	fake := &iamGetRoleFake{
		results: map[string]*iamtypes.Role{
			"role-1": iamRoleNeverUsed("role-1", "/"),
			"role-2": iamRoleWithLastUsed("role-2", "/", -1*time.Hour),
			"role-3": iamRoleWithLastUsed("role-3", "/", -1*time.Hour),
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamRoleResources()

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["role-1"]
	if !ok {
		t.Fatalf("expected finding keyed by %q (never-used role)", "role-1")
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if _, ok := result.Findings["role-2"]; ok {
		t.Error("role-2 must NOT appear in Findings — it was recently used")
	}
	if _, ok := result.Findings["role-3"]; ok {
		t.Error("role-3 must NOT appear in Findings — it was recently used")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichIAMRoleLastUsed_ServiceLinkedRoleSkipped verifies that a role with
// Path=/aws-service-role/... is skipped even when LastUsedDate is nil, producing
// no finding.
func TestEnrichIAMRoleLastUsed_ServiceLinkedRoleSkipped(t *testing.T) {
	svcLinkedResources := []resource.Resource{
		{
			ID:   "AWSServiceRoleForEC2",
			Name: "AWSServiceRoleForEC2",
			Fields: map[string]string{
				"role_name": "AWSServiceRoleForEC2",
				"path":      "/aws-service-role/ec2.amazonaws.com/",
			},
		},
	}
	fake := &iamGetRoleFake{
		results: map[string]*iamtypes.Role{
			"AWSServiceRoleForEC2": iamRoleNeverUsed("AWSServiceRoleForEC2", "/aws-service-role/ec2.amazonaws.com/"),
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, svcLinkedResources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for service-linked role, got %d: %v", len(result.Findings), result.Findings)
	}
}

// TestEnrichIAMRoleLastUsed_NilClientReturnsEmptyFindingsNoError verifies that
// when clients.IAM is nil the enricher returns a non-nil empty Findings map
// and no error.
func TestEnrichIAMRoleLastUsed_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{IAM: nil}

	result, err := awsclient.EnrichIAMRoleLastUsed(context.Background(), clients, iamRoleResources(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when IAM client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

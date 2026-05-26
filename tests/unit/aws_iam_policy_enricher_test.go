package unit

// aws_iam_policy_enricher_test.go — Behavioral tests for EnrichIAMPolicy.
//
// Contract:
//   - GetPolicy + GetPolicyVersion called once per customer-managed policy.
//   - Policy ARN is extracted from r.RawStruct (iamtypes.Policy.Arn).
//   - Policy document Effect=Allow with specific (non-wildcard) actions → 0 findings.
//   - Policy document Effect=Allow, Action="*", Resource="*" → 1 finding sev "!" (admin star).
//   - Policy with ARN prefix arn:aws:iam::aws:policy/ (AWS-managed) → skipped, 0 findings.
//   - clients.IAM == nil → 0 findings, no error.

import (
	"context"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// iamPolicyFake implements IAMAPI for policy enrichment testing.
// It embeds the interface and overrides GetPolicy and GetPolicyVersion.
// Both result maps are keyed by PolicyArn.
type iamPolicyFake struct {
	awsclient.IAMAPI
	// getPolicyResults maps PolicyArn → GetPolicyOutput.
	getPolicyResults map[string]*iam.GetPolicyOutput
	// getPolicyVersionResults maps PolicyArn → GetPolicyVersionOutput.
	getPolicyVersionResults map[string]*iam.GetPolicyVersionOutput
	// errByArn maps PolicyArn → error; applies to GetPolicy when set.
	errByArn map[string]error
}

func (f *iamPolicyFake) GetPolicy(
	_ context.Context,
	in *iam.GetPolicyInput,
	_ ...func(*iam.Options),
) (*iam.GetPolicyOutput, error) {
	arn := ""
	if in != nil && in.PolicyArn != nil {
		arn = *in.PolicyArn
	}
	if f.errByArn != nil {
		if err, ok := f.errByArn[arn]; ok {
			return nil, err
		}
	}
	out, ok := f.getPolicyResults[arn]
	if !ok {
		return &iam.GetPolicyOutput{}, nil
	}
	return out, nil
}

func (f *iamPolicyFake) GetPolicyVersion(
	_ context.Context,
	in *iam.GetPolicyVersionInput,
	_ ...func(*iam.Options),
) (*iam.GetPolicyVersionOutput, error) {
	arn := ""
	if in != nil && in.PolicyArn != nil {
		arn = *in.PolicyArn
	}
	if f.errByArn != nil {
		if err, ok := f.errByArn[arn]; ok {
			return nil, err
		}
	}
	out, ok := f.getPolicyVersionResults[arn]
	if !ok {
		return &iam.GetPolicyVersionOutput{}, nil
	}
	return out, nil
}

// Compile-time check: iamPolicyFake satisfies IAMAPI.
var _ awsclient.IAMAPI = (*iamPolicyFake)(nil)

// pathEncodeDoc path-encodes a policy document string matching the encoding
// IAM returns for PolicyVersion.Document (RFC 3986 percent-encoding).
func pathEncodeDoc(doc string) string {
	return url.PathEscape(doc)
}

// iamPolicyGetPolicyOutput builds a GetPolicyOutput for the given ARN with
// DefaultVersionId set to "v1".
func iamPolicyGetPolicyOutput(arn, versionID string) *iam.GetPolicyOutput {
	return &iam.GetPolicyOutput{
		Policy: &iamtypes.Policy{
			Arn:              aws.String(arn),
			DefaultVersionId: aws.String(versionID),
		},
	}
}

// iamPolicyGetVersionOutput builds a GetPolicyVersionOutput with the given
// path-encoded policy document.
func iamPolicyGetVersionOutput(doc string) *iam.GetPolicyVersionOutput {
	encoded := pathEncodeDoc(doc)
	return &iam.GetPolicyVersionOutput{
		PolicyVersion: &iamtypes.PolicyVersion{
			Document:         aws.String(encoded),
			IsDefaultVersion: true,
			VersionId:        aws.String("v1"),
		},
	}
}

// safePolicyDoc is a policy document with specific (non-wildcard) actions.
const safePolicyDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["ec2:DescribeInstances","ec2:DescribeVpcs"],"Resource":"*"}]}`

// adminStarPolicyDoc is a policy document with Effect=Allow, Action=*, Resource=*.
const adminStarPolicyDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`

const (
	iamPolicyARN1    = "arn:aws:iam::123456789012:policy/MyReadOnlyPolicy"
	iamPolicyARN2    = "arn:aws:iam::123456789012:policy/MyAdminPolicy"
	iamPolicyARN3    = "arn:aws:iam::123456789012:policy/MyOtherPolicy"
	iamAWSManagedARN = "arn:aws:iam::aws:policy/AdministratorAccess"
)

// iamPolicyResource builds a single customer-managed policy Resource stub.
// RawStruct is set to iamtypes.Policy so extractIAMPolicyARN can read the ARN.
// r.ID is set to the ARN to match how findings are keyed.
func iamPolicyResource(arn, name string) resource.Resource {
	return resource.Resource{
		ID:   arn,
		Name: name,
		Fields: map[string]string{
			"policy_name":      name,
			"policy_type":      "managed",
			"attachment_count": "0",
			"path":             "/",
		},
		RawStruct: iamtypes.Policy{
			Arn:              aws.String(arn),
			PolicyName:       aws.String(name),
			DefaultVersionId: aws.String("v1"),
		},
	}
}

// TestEnrichIAMPolicy_SafePolicyProducesNoFindings verifies that when all policies
// have safe (non-wildcard) action lists, no findings are produced.
func TestEnrichIAMPolicy_SafePolicyProducesNoFindings(t *testing.T) {
	fake := &iamPolicyFake{
		getPolicyResults: map[string]*iam.GetPolicyOutput{
			iamPolicyARN1: iamPolicyGetPolicyOutput(iamPolicyARN1, "v1"),
			iamPolicyARN2: iamPolicyGetPolicyOutput(iamPolicyARN2, "v1"),
			iamPolicyARN3: iamPolicyGetPolicyOutput(iamPolicyARN3, "v1"),
		},
		getPolicyVersionResults: map[string]*iam.GetPolicyVersionOutput{
			iamPolicyARN1: iamPolicyGetVersionOutput(safePolicyDoc),
			iamPolicyARN2: iamPolicyGetVersionOutput(safePolicyDoc),
			iamPolicyARN3: iamPolicyGetVersionOutput(safePolicyDoc),
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := []resource.Resource{
		iamPolicyResource(iamPolicyARN1, "MyReadOnlyPolicy"),
		iamPolicyResource(iamPolicyARN2, "MyAdminPolicy"),
		iamPolicyResource(iamPolicyARN3, "MyOtherPolicy"),
	}

	result, err := awsclient.EnrichIAMPolicy(context.Background(), clients, resources, nil)
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

// TestEnrichIAMPolicy_AdminStarProducesFindingSevBang verifies that when policy-2
// has Effect=Allow, Action=*, Resource=*, a finding with severity "!" is produced
// for policy-2 only. policy-1 and policy-3 (safe) produce no finding.
func TestEnrichIAMPolicy_AdminStarProducesFindingSevBang(t *testing.T) {
	fake := &iamPolicyFake{
		getPolicyResults: map[string]*iam.GetPolicyOutput{
			iamPolicyARN1: iamPolicyGetPolicyOutput(iamPolicyARN1, "v1"),
			iamPolicyARN2: iamPolicyGetPolicyOutput(iamPolicyARN2, "v1"),
			iamPolicyARN3: iamPolicyGetPolicyOutput(iamPolicyARN3, "v1"),
		},
		getPolicyVersionResults: map[string]*iam.GetPolicyVersionOutput{
			iamPolicyARN1: iamPolicyGetVersionOutput(safePolicyDoc),
			iamPolicyARN2: iamPolicyGetVersionOutput(adminStarPolicyDoc),
			iamPolicyARN3: iamPolicyGetVersionOutput(safePolicyDoc),
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := []resource.Resource{
		iamPolicyResource(iamPolicyARN1, "MyReadOnlyPolicy"),
		iamPolicyResource(iamPolicyARN2, "MyAdminPolicy"),
		iamPolicyResource(iamPolicyARN3, "MyOtherPolicy"),
	}

	result, err := awsclient.EnrichIAMPolicy(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[iamPolicyARN2]
	if !ok {
		t.Fatalf("expected finding keyed by %q (admin star policy)", iamPolicyARN2)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("severity = %v, want %v", f.Severity, "!")
	}
	if _, ok := result.Findings[iamPolicyARN1]; ok {
		t.Error("policy-1 must NOT appear in Findings — it is a safe policy")
	}
	if _, ok := result.Findings[iamPolicyARN3]; ok {
		t.Error("policy-3 must NOT appear in Findings — it is a safe policy")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichIAMPolicy_AWSManagedSkipped verifies that a policy with an
// AWS-managed ARN (arn:aws:iam::aws:policy/...) is skipped even when it would
// otherwise produce an admin-star finding.
func TestEnrichIAMPolicy_AWSManagedSkipped(t *testing.T) {
	awsManagedResource := resource.Resource{
		ID:   iamAWSManagedARN,
		Name: "AdministratorAccess",
		Fields: map[string]string{
			"policy_name":      "AdministratorAccess",
			"policy_type":      "managed",
			"attachment_count": "5",
			"path":             "/",
		},
		RawStruct: iamtypes.Policy{
			Arn:              aws.String(iamAWSManagedARN),
			PolicyName:       aws.String("AdministratorAccess"),
			DefaultVersionId: aws.String("v1"),
		},
	}
	// Even if the fake has an admin-star document, the enricher should skip it.
	fake := &iamPolicyFake{
		getPolicyResults: map[string]*iam.GetPolicyOutput{
			iamAWSManagedARN: iamPolicyGetPolicyOutput(iamAWSManagedARN, "v1"),
		},
		getPolicyVersionResults: map[string]*iam.GetPolicyVersionOutput{
			iamAWSManagedARN: iamPolicyGetVersionOutput(adminStarPolicyDoc),
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}

	result, err := awsclient.EnrichIAMPolicy(context.Background(), clients, []resource.Resource{awsManagedResource}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for AWS-managed policy, got %d: %v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (AWS-managed policy skipped)", result.IssueCount)
	}
}

// TestEnrichIAMPolicy_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.IAM is nil the enricher returns a non-nil empty Findings map and
// no error.
func TestEnrichIAMPolicy_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{IAM: nil}
	resources := []resource.Resource{
		iamPolicyResource(iamPolicyARN1, "MyReadOnlyPolicy"),
		iamPolicyResource(iamPolicyARN2, "MyAdminPolicy"),
		iamPolicyResource(iamPolicyARN3, "MyOtherPolicy"),
	}

	result, err := awsclient.EnrichIAMPolicy(context.Background(), clients, resources, nil)
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

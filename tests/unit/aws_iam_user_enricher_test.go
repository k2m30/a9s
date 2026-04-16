package unit

// aws_iam_user_enricher_test.go — Behavioral tests for EnrichIAMUserMFA.
//
// Contract assertions:
//   - GetLoginProfile success + MFADevices=[device-1] → 0 findings.
//   - GetLoginProfile success + MFADevices=[] → 1 finding sev "!" (no MFA on console user).
//   - GetLoginProfile NoSuchEntityException (no console) + recent access key → 0 findings.
//   - GetLoginProfile success + MFA=[device] + AccessKey.CreateDate=now-100d → 1 finding sev "~".
//   - clients.IAM == nil → 0 findings, no error.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// iamUserMFAFake implements IAMAPI for IAM user MFA enrichment testing.
// It embeds IAMAPI and overrides GetLoginProfile, ListMFADevices, ListAccessKeys.
// All maps are keyed by UserName.
type iamUserMFAFake struct {
	awsclient.IAMAPI

	// loginProfileErr maps UserName → error from GetLoginProfile.
	// A nil error value means success (profile exists).
	// Missing key means success (no override).
	loginProfileErr map[string]error

	// mfaDevicesByUser maps UserName → slice of MFADevice.
	mfaDevicesByUser map[string][]iamtypes.MFADevice

	// accessKeysByUser maps UserName → slice of AccessKeyMetadata.
	accessKeysByUser map[string][]iamtypes.AccessKeyMetadata
}

func (f *iamUserMFAFake) GetLoginProfile(
	_ context.Context,
	in *iam.GetLoginProfileInput,
	_ ...func(*iam.Options),
) (*iam.GetLoginProfileOutput, error) {
	name := ""
	if in != nil && in.UserName != nil {
		name = *in.UserName
	}
	if err, ok := f.loginProfileErr[name]; ok {
		return nil, err
	}
	return &iam.GetLoginProfileOutput{
		LoginProfile: &iamtypes.LoginProfile{UserName: aws.String(name)},
	}, nil
}

func (f *iamUserMFAFake) ListMFADevices(
	_ context.Context,
	in *iam.ListMFADevicesInput,
	_ ...func(*iam.Options),
) (*iam.ListMFADevicesOutput, error) {
	name := ""
	if in != nil && in.UserName != nil {
		name = *in.UserName
	}
	devices := f.mfaDevicesByUser[name]
	return &iam.ListMFADevicesOutput{MFADevices: devices}, nil
}

func (f *iamUserMFAFake) ListAccessKeys(
	_ context.Context,
	in *iam.ListAccessKeysInput,
	_ ...func(*iam.Options),
) (*iam.ListAccessKeysOutput, error) {
	name := ""
	if in != nil && in.UserName != nil {
		name = *in.UserName
	}
	keys := f.accessKeysByUser[name]
	return &iam.ListAccessKeysOutput{AccessKeyMetadata: keys}, nil
}

// Compile-time check: iamUserMFAFake satisfies IAMAPI.
var _ awsclient.IAMAPI = (*iamUserMFAFake)(nil)

// iamUserResources returns a slice of 2 iam-user Resource stubs.
func iamUserResources(names ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(names))
	for _, name := range names {
		res = append(res, resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"user_name": name,
				"user_id":   "AIDA" + name,
				"path":      "/",
			},
		})
	}
	return res
}

// noSuchEntityErr returns a smithy GenericAPIError that simulates
// IAM's NoSuchEntityException for GetLoginProfile (user has no console password).
func noSuchEntityErr() error {
	return &smithy.GenericAPIError{Code: "NoSuchEntityException", Message: "Login Profile for User alice cannot be found."}
}

// iamAccessKey builds an AccessKeyMetadata with the given UserName and CreateDate offset.
func iamAccessKey(userName string, offset time.Duration) iamtypes.AccessKeyMetadata {
	createDate := time.Now().Add(offset)
	return iamtypes.AccessKeyMetadata{
		UserName:    aws.String(userName),
		AccessKeyId: aws.String("AKIA" + userName),
		Status:      iamtypes.StatusTypeActive,
		CreateDate:  &createDate,
	}
}

// TestEnrichIAMUserMFA_WithMFAProducesNoFindings verifies that a console user
// with an MFA device registered produces no findings.
func TestEnrichIAMUserMFA_WithMFAProducesNoFindings(t *testing.T) {
	fake := &iamUserMFAFake{
		mfaDevicesByUser: map[string][]iamtypes.MFADevice{
			"alice": {{SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/alice")}},
			"bob":   {{SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/bob")}},
		},
		accessKeysByUser: map[string][]iamtypes.AccessKeyMetadata{
			"alice": {iamAccessKey("alice", -10*24*time.Hour)},
			"bob":   {iamAccessKey("bob", -5*24*time.Hour)},
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamUserResources("alice", "bob")

	result, err := awsclient.EnrichIAMUserMFA(context.Background(), clients, resources)
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

// TestEnrichIAMUserMFA_NoMFAProducesFindingSevBang verifies that a console user
// with no MFA device produces a finding with severity "!".
func TestEnrichIAMUserMFA_NoMFAProducesFindingSevBang(t *testing.T) {
	fake := &iamUserMFAFake{
		// alice: console user, no MFA devices
		mfaDevicesByUser: map[string][]iamtypes.MFADevice{
			"alice": {},
			"bob":   {{SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/bob")}},
		},
		accessKeysByUser: map[string][]iamtypes.AccessKeyMetadata{},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamUserResources("alice", "bob")

	result, err := awsclient.EnrichIAMUserMFA(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["alice"]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no MFA)", "alice")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if _, ok := result.Findings["bob"]; ok {
		t.Error("bob must NOT appear in Findings — bob has MFA")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1", result.IssueCount)
	}
}

// TestEnrichIAMUserMFA_NoConsoleNoOldKeysProducesNoFindings verifies that a user
// with no console access (GetLoginProfile returns NoSuchEntityException) and a
// recent access key produces no findings.
func TestEnrichIAMUserMFA_NoConsoleNoOldKeysProducesNoFindings(t *testing.T) {
	fake := &iamUserMFAFake{
		loginProfileErr: map[string]error{
			"alice": noSuchEntityErr(),
			"bob":   noSuchEntityErr(),
		},
		mfaDevicesByUser: map[string][]iamtypes.MFADevice{},
		accessKeysByUser: map[string][]iamtypes.AccessKeyMetadata{
			"alice": {iamAccessKey("alice", -10*24*time.Hour)},
			"bob":   {iamAccessKey("bob", -5*24*time.Hour)},
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamUserResources("alice", "bob")

	result, err := awsclient.EnrichIAMUserMFA(context.Background(), clients, resources)
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

// TestEnrichIAMUserMFA_OldAccessKeyProducesFindingSevTilde verifies that a console
// user with MFA but an access key older than 90 days produces a finding with
// severity "~". MFA is present so no "!" finding for the same user.
func TestEnrichIAMUserMFA_OldAccessKeyProducesFindingSevTilde(t *testing.T) {
	fake := &iamUserMFAFake{
		// alice: console, has MFA, but access key is 100d old
		mfaDevicesByUser: map[string][]iamtypes.MFADevice{
			"alice": {{SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/alice")}},
			"bob":   {{SerialNumber: aws.String("arn:aws:iam::123456789012:mfa/bob")}},
		},
		accessKeysByUser: map[string][]iamtypes.AccessKeyMetadata{
			"alice": {iamAccessKey("alice", -100*24*time.Hour)},
			"bob":   {iamAccessKey("bob", -5*24*time.Hour)},
		},
	}
	clients := &awsclient.ServiceClients{IAM: fake}
	resources := iamUserResources("alice", "bob")

	result, err := awsclient.EnrichIAMUserMFA(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["alice"]
	if !ok {
		t.Fatalf("expected finding keyed by %q (old access key)", "alice")
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings["bob"]; ok {
		t.Error("bob must NOT appear in Findings — bob has a recent key")
	}
	// "~" findings do not contribute to IssueCount.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichIAMUserMFA_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.IAM is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichIAMUserMFA_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{IAM: nil}

	result, err := awsclient.EnrichIAMUserMFA(context.Background(), clients, iamUserResources("alice", "bob"))
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

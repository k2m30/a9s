package unit

// codex2_round4_test.go — MarkSkipped owns the vanished-resource race, so
// every wave-2 enricher gets it without a site edit.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestMarkSkipped_AVanishedResourceIsUninspectedNotAFailure pins the rule for
// all of MarkSkipped's call sites at once: the row is uninspected either way,
// but a resource that went away between the list call and the per-item call
// is the race IsNotFoundErr classifies and must not enter the aggregate.
func TestMarkSkipped_AVanishedResourceIsUninspectedNotAFailure(t *testing.T) {
	gone := &iamtypes.NoSuchEntityException{Message: aws.String("Role with name r cannot be found.")}
	denied := &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized"}

	result := awsclient.IssueEnricherResult{
		Findings:     map[string][]domain.Finding{},
		TruncatedIDs: map[string]bool{},
	}
	var failures []awsclient.Failure

	awsclient.MarkSkipped(&result, "vanished", &failures, gone)
	awsclient.MarkSkipped(&result, "refused", &failures, denied)

	if !result.TruncatedIDs["vanished"] {
		t.Error("a vanished row must still render uninspected")
	}
	if !result.TruncatedIDs["refused"] {
		t.Error("a refused row must render uninspected")
	}
	if len(failures) != 1 {
		t.Fatalf("failures = %+v, want only the refused call", failures)
	}
	if failures[0].ID != "refused" {
		t.Errorf("failures[0].ID = %q, want %q — the race is not a recorded failure",
			failures[0].ID, "refused")
	}
}

// codex2VanishFake is w4RoleAdminFake's GetRole with one role deleted between
// the list call and this one, and one refused.
type codex2VanishFake struct {
	awsclient.IAMAPI
	gone   map[string]bool
	denied map[string]bool
}

func (f *codex2VanishFake) GetRole(_ context.Context, in *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	name := aws.ToString(in.RoleName)
	switch {
	case f.gone[name]:
		return nil, &iamtypes.NoSuchEntityException{
			Message: aws.String("Role with name " + name + " cannot be found."),
		}
	case f.denied[name]:
		return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized to perform: iam:GetRole"}
	}
	return &iam.GetRoleOutput{Role: &iamtypes.Role{RoleName: in.RoleName, Path: aws.String("/")}}, nil
}

func (f *codex2VanishFake) ListAttachedRolePolicies(_ context.Context, in *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	name := aws.ToString(in.RoleName)
	if f.gone[name] {
		return nil, &iamtypes.NoSuchEntityException{
			Message: aws.String("Role with name " + name + " cannot be found."),
		}
	}
	return &iam.ListAttachedRolePoliciesOutput{}, nil
}

// TestEnrichIAMRoleLastUsed_VanishedRoleIsARace drives one of the five IAM
// sites end to end through the registered enricher: the role's row is
// uninspected and the composite error is empty, because nothing failed — the
// role simply stopped existing.
func TestEnrichIAMRoleLastUsed_VanishedRoleIsARace(t *testing.T) {
	fake := &codex2VanishFake{gone: map[string]bool{"acme-gone-role": true}}
	res, err := awsclient.EnrichIAMRoleLastUsed(context.Background(),
		&awsclient.ServiceClients{IAM: fake, Region: "us-east-1"},
		[]resource.Resource{
			w4RoleResource("acme-live-role", "/"),
			w4RoleResource("acme-gone-role", "/"),
		}, nil)

	if err != nil {
		t.Errorf("err = %v, want nil — a role deleted between the list call and "+
			"the per-role call is a race, not a failure to report", err)
	}
	if !res.TruncatedIDs["acme-gone-role"] {
		t.Error("TruncatedIDs[acme-gone-role] = false; the row must render \"?\", not clean")
	}
	if res.TruncatedIDs["acme-live-role"] {
		t.Error("the role that answered must not be marked uninspected")
	}
}

// TestEnrichIAMRoleLastUsed_RefusedRoleIsStillAFailure is the negative
// control on the same lane: the guard must not swallow a denial.
func TestEnrichIAMRoleLastUsed_RefusedRoleIsStillAFailure(t *testing.T) {
	fake := &codex2VanishFake{denied: map[string]bool{"acme-denied-role": true}}
	res, err := awsclient.EnrichIAMRoleLastUsed(context.Background(),
		&awsclient.ServiceClients{IAM: fake, Region: "us-east-1"},
		[]resource.Resource{
			w4RoleResource("acme-live-role", "/"),
			w4RoleResource("acme-denied-role", "/"),
		}, nil)

	if err == nil {
		t.Fatal("err = nil for a denied GetRole, want the partial failure")
	}
	if !awsclient.IsAccessDenied(err) {
		t.Errorf("class = %q, want %q", awsclient.ErrClass(err), awsclient.ClassAccessDenied)
	}
	if !res.TruncatedIDs["acme-denied-role"] {
		t.Error("a refused row must still render uninspected")
	}
}

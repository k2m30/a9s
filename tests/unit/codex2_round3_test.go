package unit

// codex2_round3_test.go — an IAM entity that vanished between the list call
// and the per-ID describe is the deleted-resource race, not a failure.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestIsNotFoundErr_CoversTheIAMSpellingOfAVanishedEntity pins the code the
// one not-found table lacked. IAM answers NoSuchEntity for a role, user,
// group, policy or instance profile that is gone, so without it every IAM
// per-ID lane reads an ordinary race as a failure to report.
func TestIsNotFoundErr_CoversTheIAMSpellingOfAVanishedEntity(t *testing.T) {
	for _, err := range []error{
		&iamtypes.NoSuchEntityException{Message: aws.String("Role with name acme-deploy-role cannot be found.")},
		&smithy.GenericAPIError{Code: "NoSuchEntity", Message: "cannot be found"},
		// A response the SDK could not bind to the modeled type carries the
		// exception name itself.
		&smithy.GenericAPIError{Code: "NoSuchEntityException", Message: "cannot be found"},
	} {
		if !awsclient.IsNotFoundErr(err) {
			t.Errorf("IsNotFoundErr(%v) = false, want true", err)
		}
	}
}

// TestIsNotFoundErr_LeavesTheSubConfigurationAbsencesAlone is the negative
// control and the reason the sweep is not "add every code that says
// not-found". These codes mean "this optional configuration is not set",
// which is an ANSWER about a live resource. Folding them in would mark a
// perfectly healthy bucket, table or function as data-incomplete.
func TestIsNotFoundErr_LeavesTheSubConfigurationAbsencesAlone(t *testing.T) {
	for _, code := range []string{
		"NoSuchBucketPolicy",
		"NoSuchCORSConfiguration",
		"NoSuchLifecycleConfiguration",
		"NoSuchTagSet",
		"NoSuchPublicAccessBlockConfiguration",
		"ServerSideEncryptionConfigurationNotFoundError",
		"ObjectLockConfigurationNotFoundError",
		"RepositoryPolicyNotFoundException",
		"PolicyNotFound",
	} {
		if awsclient.IsNotFoundErr(&smithy.GenericAPIError{Code: code, Message: "x"}) {
			t.Errorf("IsNotFoundErr(%s) = true; that code says a configuration is unset, "+
				"not that the resource is gone", code)
		}
	}
}

type codex2GoneRoleFake struct {
	awsclient.IAMAPI
	gone   map[string]bool
	denied map[string]bool
}

func (f *codex2GoneRoleFake) GetRole(_ context.Context, in *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	name := aws.ToString(in.RoleName)
	if f.gone[name] {
		return nil, &iamtypes.NoSuchEntityException{
			Message: aws.String("Role with name " + name + " cannot be found."),
		}
	}
	if f.denied[name] {
		return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized to perform: iam:GetRole"}
	}
	return &iam.GetRoleOutput{Role: &iamtypes.Role{
		RoleName: aws.String(name),
		RoleId:   aws.String("AROA00000000000EXAMPLE"),
		Path:     aws.String("/"),
		Arn:      aws.String("arn:aws:iam::123456789012:role/" + name),
	}}, nil
}

func (f *codex2GoneRoleFake) ListRolePolicies(context.Context, *iam.ListRolePoliciesInput, ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	return &iam.ListRolePoliciesOutput{}, nil
}

func (f *codex2GoneRoleFake) GetRolePolicy(context.Context, *iam.GetRolePolicyInput, ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error) {
	return nil, &iamtypes.NoSuchEntityException{Message: aws.String("no inline policy")}
}

// TestFetchRolesByIDs_AVanishedRoleIsARaceNotAFailure pins the contract
// IsNotFoundErr's doc states: a resource that existed at list time and was
// deleted before the per-ID describe landed is an operational race — no
// finding, and kept out of the failure aggregate — while a refused call is
// still a failure the operator is told about.
func TestFetchRolesByIDs_AVanishedRoleIsARaceNotAFailure(t *testing.T) {
	fetch := resource.GetFetchByIDs("role")
	if fetch == nil {
		t.Fatal(`no FetchByIDs registered for "role"`)
	}

	t.Run("deleted between list and describe", func(t *testing.T) {
		fake := &codex2GoneRoleFake{gone: map[string]bool{"acme-gone-role": true}}
		rows, err := fetch(context.Background(), &awsclient.ServiceClients{IAM: fake},
			[]string{"acme-live-role", "acme-gone-role"})
		if err != nil {
			t.Errorf("err = %v, want nil — a vanished role is a race, not a failure to report", err)
		}
		if len(rows) != 1 || rows[0].ID != "acme-live-role" {
			t.Errorf("rows = %+v, want just acme-live-role", rows)
		}
	})

	t.Run("a refused call is still a failure", func(t *testing.T) {
		fake := &codex2GoneRoleFake{denied: map[string]bool{"acme-denied-role": true}}
		rows, err := fetch(context.Background(), &awsclient.ServiceClients{IAM: fake},
			[]string{"acme-live-role", "acme-denied-role"})
		if err == nil {
			t.Error("err = nil for a denied GetRole, want the partial failure")
		} else if !awsclient.IsAccessDenied(err) {
			t.Errorf("class = %q, want %q", awsclient.ErrClass(err), awsclient.ClassAccessDenied)
		}
		if len(rows) != 1 {
			t.Errorf("got %d rows, want the one that resolved", len(rows))
		}
	})
}

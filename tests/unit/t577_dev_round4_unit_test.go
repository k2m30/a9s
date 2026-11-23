package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

type t577InlineDeniedPolicyFake struct {
	awsManagedPolicyFake
}

func (f *t577InlineDeniedPolicyFake) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	return nil, errors.New("AccessDenied: not authorized to perform iam:ListGroups")
}

// A refused inline group walk costs the inline policies, never the
// AWS-managed ones: those resolve by GetPolicy whatever the walk answered,
// and the walk's failure is still reported.
func TestT577Dev_AWSManagedPolicyResolvesWhenTheInlineWalkFails(t *testing.T) {
	const name = "ReadOnlyAccess"
	arn := "arn:aws:iam::aws:policy/" + name
	fake := &t577InlineDeniedPolicyFake{awsManagedPolicyFake{
		policiesByARN: map[string]iamtypes.Policy{
			arn: {Arn: aws.String(arn), PolicyName: aws.String(name), IsAttachable: true, Path: aws.String("/")},
		},
	}}

	results, err := awsclient.FetchIAMPoliciesByIDsFull(context.Background(), fake, []string{name}, session.NewPolicyStore(), "aws")

	if len(results) != 1 || results[0].Name != name {
		t.Errorf("results = %+v, want the AWS-managed %s resolved", results, name)
	}
	if err == nil || !strings.Contains(err.Error(), "ListGroups") {
		t.Errorf("err = %v, want the refused inline walk reported", err)
	}
	if err != nil && strings.Contains(err.Error(), "policy FetchByIDs failed") {
		t.Errorf("err = %v, want no failure for the policy that resolved", err)
	}
}

type t577RoleCountFake struct {
	awsclient.IAMAPI
}

func (f *t577RoleCountFake) GetRole(_ context.Context, _ *iam.GetRoleInput, _ ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	return nil, errors.New("AccessDenied: not authorized to perform iam:GetRole")
}

// A by-ID fetch counts its failures against the requests it sent: an empty
// or repeated ID sends none.
func TestT577Dev_ByIDFailureCountIsOutOfTheRequestsSent(t *testing.T) {
	_, err := awsclient.FetchRolesByIDs(context.Background(), &t577RoleCountFake{}, []string{"app-role", "app-role", ""})
	if err == nil || !strings.Contains(err.Error(), "failed for 1 of 1 ") {
		t.Errorf("err = %v, want the one GetRole sent counted once", err)
	}
}

// Opening JSON on top of YAML in the TUI folds the JSON operation's
// enrichment into both screens: popping back reveals the YAML screen
// enriched, as the web lane shows it.
func TestT577Dev_TUIEnrichmentRewritesTheBuriedTextScreen(t *testing.T) {
	m := newEnrichApp(t)
	res := rolePolicyRes("arn:aws:iam::123456789012:policy/stacked", "stacked", "Managed")

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetYAML, ResourceType: "role_policies", Resource: &res})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetJSON, ResourceType: "role_policies", Resource: &res})

	enriched := withDocument(res, map[string]any{
		"Version":   "2012-10-17",
		"Statement": []any{map[string]any{"Effect": "Allow", "Action": "s3:GetObject", "Resource": "*"}},
	})
	m, _ = rootApplyMsg(m, messages.EnrichDetailResult{ResourceType: "role_policies", ResourceID: res.ID, EnrichedRes: enriched})
	if top := stripANSI(rootViewContent(m)); !strings.Contains(top, "s3:GetObject") {
		t.Fatalf("precondition: the JSON screen does not show the enriched document:\n%s", top)
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	if yaml := stripANSI(rootViewContent(m)); !strings.Contains(yaml, "s3:GetObject") {
		t.Errorf("the revealed YAML screen does not show the enriched document:\n%s", yaml)
	}
}

package unit_test

// aws6_role_policies_documents_test.go — every managed policy the demo
// account MODELS carries its document.
//
// The role_policies detail asks for a policy's document (GetPolicy for the
// default version id, then GetPolicyVersion). A policy the fixtures list but
// give no document to answers NoSuchEntity there, and the detail reports a
// failure for a policy the account plainly holds — a fixture gap wearing the
// costume of a product defect.
//
// The rule is the same one the demo fakes follow: the fixtures register what
// the pivots ask for. A NAME the fixtures never register is a different case
// and stays a failure; that one is pinned in
// aws6_managed_policy_and_s3_prefix_test.go.

import (
	"context"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

func TestEveryModelledManagedPolicyHasItsDocument(t *testing.T) {
	clients := demo.NewServiceClients()
	getPol, ok := clients.IAM.(awsclient.IAMGetPolicyAPI)
	if !ok {
		t.Fatal("demo IAM client does not support GetPolicy")
	}
	getVer, ok := clients.IAM.(awsclient.IAMGetPolicyVersionAPI)
	if !ok {
		t.Fatal("demo IAM client does not support GetPolicyVersion")
	}

	// Every managed policy a demo role attaches AND the account models. The
	// retired name is excluded by construction: it is registered nowhere, so
	// it is not in the list the fixtures hold.
	modelled := map[string]bool{}
	page, err := clients.IAM.ListPolicies(context.Background(), &iam.ListPoliciesInput{})
	if err != nil {
		t.Fatalf("ListPolicies: %v", err)
	}
	for i := range page.Policies {
		modelled[aws.ToString(page.Policies[i].Arn)] = true
	}
	if len(modelled) == 0 {
		t.Fatal("the demo account models no policy at all")
	}

	var missing []string
	for _, arn := range demoAttachedPolicyARNs(t, clients) {
		if !modelled[arn] {
			continue
		}
		if _, err := awsclient.FetchManagedPolicyDocument(context.Background(), getPol, getVer, arn); err != nil {
			missing = append(missing, arn+": "+err.Error())
		}
	}
	sort.Strings(missing)
	for _, m := range missing {
		t.Errorf("a demo role attaches a policy the account models, and its document cannot be read: %s — "+
			"register the document in core/demo/fixtures/iam.go; the role_policies detail asks for it", m)
	}

	// The negative case: the retired name really is unmodelled, so the
	// exclusion above is not quietly excusing every policy.
	retired := "arn:aws:iam::aws:policy/" + fixtures.RetiredManagedPolicyName
	if modelled[retired] {
		t.Errorf("%s is modelled; it exists to be the name the account cannot read", retired)
	}
}

// demoAttachedPolicyARNs returns every managed-policy ARN a demo role
// attaches.
func demoAttachedPolicyARNs(t *testing.T, clients *awsclient.ServiceClients) []string {
	t.Helper()
	ctx := context.Background()
	roles, err := clients.IAM.ListRoles(ctx, &iam.ListRolesInput{})
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	seen := map[string]bool{}
	var out []string
	for i := range roles.Roles {
		attached, err := clients.IAM.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
			RoleName: roles.Roles[i].RoleName,
		})
		if err != nil {
			continue
		}
		for _, ap := range attached.AttachedPolicies {
			arn := aws.ToString(ap.PolicyArn)
			if arn == "" || seen[arn] {
				continue
			}
			seen[arn] = true
			out = append(out, arn)
		}
	}
	sort.Strings(out)
	return out
}

package unit

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// IAM applies a Deny whose condition matches: this one denies every caller
// that does not present the external ID, so the grant survives for every
// principal in any account that presents it. The external ID is a value any
// caller can present, not a scope (IAM User Guide, Access to AWS accounts
// owned by third parties: "The external ID is not a special value that you
// need to create explicitly"), so the wildcard grant stays public on a trust
// policy as on a resource policy.
func TestTrustDenyUnlessExternalIDLeavesTheWildcardOpen(t *testing.T) {
	doc := mustParse(t, policyOf(
		`{"Effect":"Allow","Principal":{"AWS":"*"},"Action":"sts:AssumeRole"}`,
		`{"Effect":"Deny","Principal":{"AWS":"*"},"Action":"sts:AssumeRole","Condition":{"StringNotEquals":{"sts:ExternalId":"acme-ext-7f3a"}}}`,
	))
	if trust := iampolicy.Evaluate(doc, "123456789012"); !trust.Public || trust.Conditioned {
		t.Errorf("EvaluateTrust: Public=%v Conditioned=%v, want Public=true Conditioned=false", trust.Public, trust.Conditioned)
	}
	if res := iampolicy.Evaluate(doc, "123456789012"); !res.Public {
		t.Errorf("Evaluate: sts:ExternalId scopes nothing on a resource policy, yet Public=false")
	}
}

// A secret listed without its ARN, in a session STS could not identify, has
// no source for its owning account: a 12-digit principal in its policy can
// be neither foreign nor own, so the row is marked, never flagged.
func TestSecretWithoutARNAndSTSMarksOwnAccountUnknown(t *testing.T) {
	doc := policyOf(`{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/acme-app"},"Action":"secretsmanager:GetSecretValue","Resource":"*"}`)
	r := resource.Resource{ID: "acme-db", Name: "acme-db", Type: "secrets", Fields: map[string]string{"secret_name": "acme-db", "status": "OK"}}
	clients := identityUnavailable(&awsclient.ServiceClients{
		SecretsManager: &w4SecretsFake{policies: map[string]string{"acme-db": doc}}, Region: "us-east-1",
	})
	res, _ := awsclient.EnrichSecretsPolicy(context.Background(), clients, []resource.Resource{r}, nil) //nolint:errcheck // judged by its marks
	if hasCode(res.Findings["acme-db"], "secrets.cross-account-policy") {
		t.Errorf("own account unknown, yet the secret is reported as granting another account")
	}
	if got := res.TruncatedIDs["acme-db"]; got != awsclient.CheckOwnAccountUnknown {
		t.Errorf("TruncatedIDs[acme-db] = %q, want %q", got, awsclient.CheckOwnAccountUnknown)
	}
}

// A topic policy that does not parse says nothing about who it grants: the
// role pivot is unknown, not a proven zero.
func TestSNSTopicPolicyThatDoesNotParseLeavesTheRolePivotUnknown(t *testing.T) {
	arn := w5TopicARN("acme-alerts")
	f := newW5SNSFake()
	f.topicAttrs[arn] = map[string]string{"TopicArn": arn, "Policy": unparseablePolicy}
	got := relatedCheckerFor(t, "sns", "role")(context.Background(), &awsclient.ServiceClients{SNS: f}, w5TopicRes("acme-alerts"), resource.ResourceCache{})
	if got.State() != domain.RelatedUnknown {
		t.Errorf("state = %v (count %d), want unknown", got.State(), got.Count())
	}
}

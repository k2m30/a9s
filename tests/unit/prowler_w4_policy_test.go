package unit

// prowler_w4_policy_test.go — behavioural tests for the batch-w4 policy row:
// a customer-managed policy whose allowed actions add up to a known
// privilege-escalation path.

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w4CodePolicyPrivEsc   domain.FindingCode = "policy.privilege-escalation"
	w4CodePolicyAdminStar domain.FindingCode = "iam-policy.admin-star"
	w4SourcePolicyWave2                      = "wave2:iam-policy"
)

// The lambda escalation path: pass a role to a function you create and then
// invoke it. Exactly one known combination matches this action set.
const w4PolicyPrivEscDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
	`"Action":["iam:PassRole","lambda:CreateFunction","lambda:InvokeFunction"],"Resource":"*"}]}`

const w4PolicyBenignDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
	`"Action":["s3:GetObject","s3:ListBucket"],"Resource":"arn:aws:s3:::acme-reports/*"}]}`

const w4PolicyAdminStarDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
	`"Action":"*","Resource":"*"}]}`

// Full control of IAM without being a literal admin-star policy: matches
// every IAM-only escalation combination at once.
const w4PolicyIAMStarDoc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow",` +
	`"Action":"iam:*","Resource":"*"}]}`

const w4PolicyPrivEscCombo = "PassRole+CreateLambda+Invoke"

// w4PolicyFake serves GetPolicy + GetPolicyVersion keyed by policy ARN.
type w4PolicyFake struct {
	awsclient.IAMAPI
	// docs maps policy ARN → policy document.
	docs map[string]string
	// errByARN maps policy ARN → error from GetPolicy.
	errByARN map[string]error
}

func (f *w4PolicyFake) GetPolicy(
	_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options),
) (*iam.GetPolicyOutput, error) {
	arn := aws.ToString(in.PolicyArn)
	if err, ok := f.errByARN[arn]; ok {
		return nil, err
	}
	if _, ok := f.docs[arn]; !ok {
		return nil, errors.New("NoSuchEntity: policy not found")
	}
	return &iam.GetPolicyOutput{Policy: &iamtypes.Policy{
		Arn:              in.PolicyArn,
		DefaultVersionId: aws.String("v3"),
	}}, nil
}

func (f *w4PolicyFake) GetPolicyVersion(
	_ context.Context, in *iam.GetPolicyVersionInput, _ ...func(*iam.Options),
) (*iam.GetPolicyVersionOutput, error) {
	doc, ok := f.docs[aws.ToString(in.PolicyArn)]
	if !ok {
		return nil, errors.New("NoSuchEntity: policy version not found")
	}
	return &iam.GetPolicyVersionOutput{PolicyVersion: &iamtypes.PolicyVersion{
		VersionId: in.VersionId,
		// AWS returns the document percent-encoded.
		Document: aws.String(url.QueryEscape(doc)),
	}}, nil
}

var _ awsclient.IAMAPI = (*w4PolicyFake)(nil)

// w4PolicyResource builds a policy resource the way the policy fetcher does,
// carrying the typed SDK struct the enricher reads the ARN from.
func w4PolicyResource(name, arn string) resource.Resource {
	return resource.Resource{
		ID: arn, Name: name, Type: "policy",
		Fields:    map[string]string{"policy_name": name, "attachment_count": "2", "is_attachable": "true"},
		RawStruct: iamtypes.Policy{PolicyName: aws.String(name), Arn: aws.String(arn)},
	}
}

func w4CustomerPolicyARN(name string) string {
	return "arn:aws:iam::123456789012:policy/" + name
}

func w4EnrichPolicies(t *testing.T, fake *w4PolicyFake, rs []resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	clients := &awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}
	res, err := awsclient.EnrichIAMPolicy(context.Background(), clients, rs, nil)
	if err != nil {
		t.Fatalf("EnrichIAMPolicy: %v", err)
	}
	return res
}

// TestW4PolicyPrivilegeEscalation pins the positive case: a customer-managed
// policy granting a complete escalation path is reported with the path named,
// so the operator can see which grant to remove rather than reading the
// document themselves.
func TestW4PolicyPrivilegeEscalation(t *testing.T) {
	escARN := w4CustomerPolicyARN("acme-lambda-deployer")
	benignARN := w4CustomerPolicyARN("acme-reports-reader")
	fake := &w4PolicyFake{docs: map[string]string{
		escARN:    w4PolicyPrivEscDoc,
		benignARN: w4PolicyBenignDoc,
	}}
	res := w4EnrichPolicies(t, fake, []resource.Resource{
		w4PolicyResource("acme-lambda-deployer", escARN),
		w4PolicyResource("acme-reports-reader", benignARN),
	})

	w4AssertFinding(t, res.Findings[escARN], w4CodePolicyPrivEsc,
		"allows privilege escalation: "+w4PolicyPrivEscCombo, domain.SevBroken, w4SourcePolicyWave2)
	w4AssertRows(t, res.AttentionDetails[escARN], w4CodePolicyPrivEsc,
		[]domain.DetailRow{{Label: "Combo", Value: w4PolicyPrivEscCombo}})

	w4AssertNoCode(t, res.Findings[benignARN], w4CodePolicyPrivEsc)
}

// TestW4PolicyPrivEscSkipsAWSManaged pins that AWS-managed policies are out of
// scope. The operator cannot edit them, and reporting them would flag every
// account identically.
func TestW4PolicyPrivEscSkipsAWSManaged(t *testing.T) {
	awsARN := "arn:aws:iam::aws:policy/AWSLambda_FullAccess"
	fake := &w4PolicyFake{docs: map[string]string{awsARN: w4PolicyPrivEscDoc}}
	res := w4EnrichPolicies(t, fake, []resource.Resource{w4PolicyResource("AWSLambda_FullAccess", awsARN)})
	w4AssertNoCode(t, res.Findings[awsARN], w4CodePolicyPrivEsc)
}

// TestW4PolicyAdminIsReportedOnceAsAdmin pins that an admin policy — which
// technically satisfies every escalation path — produces the admin finding
// only. Reporting both would say the same thing twice on one row.
func TestW4PolicyAdminIsReportedOnceAsAdmin(t *testing.T) {
	adminARN := w4CustomerPolicyARN("acme-break-glass-admin")
	fake := &w4PolicyFake{docs: map[string]string{adminARN: w4PolicyAdminStarDoc}}
	res := w4EnrichPolicies(t, fake, []resource.Resource{w4PolicyResource("acme-break-glass-admin", adminARN)})

	if _, ok := w4FindingByCode(t, res.Findings[adminARN], w4CodePolicyAdminStar); !ok {
		t.Errorf("admin-star finding missing; got %s", w4CodesOf(res.Findings[adminARN]))
	}
	w4AssertNoCode(t, res.Findings[adminARN], w4CodePolicyPrivEsc)
}

// TestW4PolicyPrivEscComboRowsAreCapped pins the row cap: a policy matching
// more escalation paths than fit in the detail view lists ten and says how
// many were left out, rather than growing the panel without bound.
func TestW4PolicyPrivEscComboRowsAreCapped(t *testing.T) {
	doc, err := iampolicy.Parse(w4PolicyIAMStarDoc)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	combos := doc.PrivilegeEscalation()
	if len(combos) <= 10 {
		t.Fatalf("fixture matches %d combos, need >10 to exercise the cap", len(combos))
	}

	arn := w4CustomerPolicyARN("acme-iam-operator")
	fake := &w4PolicyFake{docs: map[string]string{arn: w4PolicyIAMStarDoc}}
	res := w4EnrichPolicies(t, fake, []resource.Resource{w4PolicyResource("acme-iam-operator", arn)})

	w4AssertFinding(t, res.Findings[arn], w4CodePolicyPrivEsc,
		"allows privilege escalation: "+combos[0], domain.SevBroken, w4SourcePolicyWave2)

	want := make([]domain.DetailRow, 0, 11)
	for _, combo := range combos[:10] {
		want = append(want, domain.DetailRow{Label: "Combo", Value: combo})
	}
	want = append(want, domain.DetailRow{
		Label: "Combo",
		Value: fmt.Sprintf("… +%d more", len(combos)-10),
	})
	w4AssertRows(t, res.AttentionDetails[arn], w4CodePolicyPrivEsc, want)
}

// TestW4PolicyPrivEscDocumentFetchFailureIsUnknown pins that a policy whose
// document could not be read is marked unknown rather than silently clean,
// and that the rest of the batch is still evaluated.
func TestW4PolicyPrivEscDocumentFetchFailureIsUnknown(t *testing.T) {
	deniedARN := w4CustomerPolicyARN("acme-denied-policy")
	escARN := w4CustomerPolicyARN("acme-lambda-deployer")
	fake := &w4PolicyFake{
		docs:     map[string]string{escARN: w4PolicyPrivEscDoc},
		errByARN: map[string]error{deniedARN: errors.New("AccessDenied: not authorized to GetPolicy")},
	}
	res := w4EnrichPolicies(t, fake, []resource.Resource{
		w4PolicyResource("acme-denied-policy", deniedARN),
		w4PolicyResource("acme-lambda-deployer", escARN),
	})

	if !res.TruncatedIDs[deniedARN] {
		t.Errorf("TruncatedIDs[%s] = false, want true", deniedARN)
	}
	w4AssertNoCode(t, res.Findings[deniedARN], w4CodePolicyPrivEsc)
	w4AssertFinding(t, res.Findings[escARN], w4CodePolicyPrivEsc,
		"allows privilege escalation: "+w4PolicyPrivEscCombo, domain.SevBroken, w4SourcePolicyWave2)
}

// TestW4PolicyPrivEscBeyondCap pins the cap semantics for a "!"-severity
// enricher: more policies than the per-pass budget means the issue count is a
// lower bound, so the result reports itself truncated.
func TestW4PolicyPrivEscBeyondCap(t *testing.T) {
	docs := map[string]string{}
	rs := make([]resource.Resource, 0, awsclient.EnrichmentCap+1)
	for i := range awsclient.EnrichmentCap + 1 {
		name := fmt.Sprintf("acme-policy-%03d", i)
		arn := w4CustomerPolicyARN(name)
		docs[arn] = w4PolicyPrivEscDoc
		rs = append(rs, w4PolicyResource(name, arn))
	}
	res := w4EnrichPolicies(t, &w4PolicyFake{docs: docs}, rs)

	if !res.Truncated {
		t.Errorf("Truncated = false with %d resources and a cap of %d",
			len(rs), awsclient.EnrichmentCap)
	}
	if len(res.Findings) != awsclient.EnrichmentCap {
		t.Errorf("evaluated %d policies, want exactly the cap %d", len(res.Findings), awsclient.EnrichmentCap)
	}
}

// TestW4PolicyPrivEscNilClient pins the nil-client contract.
func TestW4PolicyPrivEscNilClient(t *testing.T) {
	res, err := awsclient.EnrichIAMPolicy(context.Background(), &awsclient.ServiceClients{},
		[]resource.Resource{w4PolicyResource("acme-lambda-deployer", w4CustomerPolicyARN("acme-lambda-deployer"))}, nil)
	if err != nil {
		t.Fatalf("EnrichIAMPolicy: %v", err)
	}
	if res.Findings == nil || res.TruncatedIDs == nil || res.FieldUpdates == nil {
		t.Fatalf("result maps must be non-nil")
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %v, want empty", res.Findings)
	}
}

// TestW4PolicyFindingDef pins the registry row for the new policy code.
func TestW4PolicyFindingDef(t *testing.T) {
	def := w4FindingDef(t, "policy", w4CodePolicyPrivEsc)
	if !strings.Contains(def.Phrase, "<combo>") {
		t.Errorf("Phrase = %q; a phrase with a variable part is registered with a <placeholder>", def.Phrase)
	}
	if def.Phrase != "allows privilege escalation: <combo>" {
		t.Errorf("Phrase = %q, want %q", def.Phrase, "allows privilege escalation: <combo>")
	}
	if def.Severity != domain.SevBroken {
		t.Errorf("Severity = %v, want SevBroken", def.Severity)
	}
	if def.Source != "wave2" {
		t.Errorf("Source = %q, want %q", def.Source, "wave2")
	}
}

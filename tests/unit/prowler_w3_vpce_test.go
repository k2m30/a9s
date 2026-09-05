package unit

// prowler_w3_vpce_test.go — vpce.policy-open.
//
// An endpoint policy that grants every action to every principal is the AWS
// default on a gateway endpoint, and it means any principal that can reach the
// endpoint's route table can use it against any bucket or table in the
// partition. It is evaluated through the shared iampolicy engine so a scoped
// or condition-guarded grant is correctly read as fine.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w3CodeVPCEPolicyOpen = domain.FindingCode("vpce.policy-open")
	w3CodeVPCEPending    = domain.FindingCode("vpce.state.pending")
	w3CodeVPCEDeleted    = domain.FindingCode("vpce.state.deleted")

	w3PhraseVPCEPolicyOpen = "endpoint policy open to any principal"
)

// w3VPCEDefaultPolicy is the full-access document AWS attaches to a gateway
// endpoint when the caller supplies none.
const w3VPCEDefaultPolicy = `{
  "Version": "2008-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "*",
      "Resource": "*"
    }
  ]
}`

// w3VPCEScopedPolicy grants a narrow action set to one account.
const w3VPCEScopedPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": "arn:aws:iam::123456789012:root"},
      "Action": ["s3:GetObject", "s3:PutObject"],
      "Resource": "arn:aws:s3:::acme-artifacts/*"
    }
  ]
}`

// w3VPCEConditionedPolicy is the hardened pattern: wildcard principal fenced
// by an organisation condition. Prowler and a9s both treat it as scoped.
const w3VPCEConditionedPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "*",
      "Resource": "*",
      "Condition": {"StringEquals": {"aws:PrincipalOrgID": "o-abc123def4"}}
    }
  ]
}`

type w3VPCEAPI struct{ endpoints []ec2types.VpcEndpoint }

func (f w3VPCEAPI) DescribeVpcEndpoints(_ context.Context, _ *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	return &ec2.DescribeVpcEndpointsOutput{VpcEndpoints: f.endpoints}, nil
}

func w3FetchVPCEs(t *testing.T, eps ...ec2types.VpcEndpoint) []resource.Resource {
	t.Helper()
	res, err := awsclient.FetchVPCEndpointsPage(context.Background(), w3VPCEAPI{endpoints: eps}, "")
	if err != nil {
		t.Fatalf("FetchVPCEndpointsPage: %v", err)
	}
	return res.Resources
}

// w3VPCE builds a realistic gateway endpoint row. policy "" models the field
// AWS omits on an interface endpoint with no policy support.
func w3VPCE(id, state, policy string) ec2types.VpcEndpoint {
	ep := ec2types.VpcEndpoint{
		VpcEndpointId:   aws.String(id),
		ServiceName:     aws.String("com.amazonaws.us-east-1.s3"),
		VpcEndpointType: ec2types.VpcEndpointTypeGateway,
		State:           ec2types.State(state),
		VpcId:           aws.String("vpc-0abc123"),
		RouteTableIds:   []string{"rtb-0abc123"},
	}
	if policy != "" {
		ep.PolicyDocument = aws.String(policy)
	}
	return ep
}

func TestW3VPCEPolicyOpen_DefaultFullAccessFlagged(t *testing.T) {
	rows := w3FetchVPCEs(t, w3VPCE("vpce-0aa11bb22cc33dd44", "Available", w3VPCEDefaultPolicy))

	f, ok := w3FindingByCode(rows[0].Findings, w3CodeVPCEPolicyOpen)
	if !ok {
		t.Fatalf("default full-access endpoint policy produced no %s; findings=%+v", w3CodeVPCEPolicyOpen, rows[0].Findings)
	}
	if f.Phrase != w3PhraseVPCEPolicyOpen {
		t.Errorf("Phrase = %q, want %q", f.Phrase, w3PhraseVPCEPolicyOpen)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Severity = %v, want SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Source = %q, want %q", f.Source, "wave1")
	}
	if f.Detail == "" {
		t.Error("Detail is empty; every finding carries an operator sentence")
	}
	w3AssertRows(t, rows[0].AttentionDetails[w3CodeVPCEPolicyOpen].Rows, [][2]string{
		{"Principal", "*"},
		{"Actions", "*"},
	})
}

// TestW3VPCEPolicyOpen_StatementAsObject pins the shared parser's other legal
// shape: AWS accepts a bare Statement object as well as an array, and an
// endpoint written that way is exactly as open.
func TestW3VPCEPolicyOpen_StatementAsObject(t *testing.T) {
	const doc = `{"Version":"2008-10-17","Statement":{"Effect":"Allow","Principal":"*","Action":"*","Resource":"*"}}`
	rows := w3FetchVPCEs(t, w3VPCE("vpce-0aa11bb22cc33dd44", "Available", doc))
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeVPCEPolicyOpen); !ok {
		t.Errorf("single-object Statement produced no %s; findings=%+v", w3CodeVPCEPolicyOpen, rows[0].Findings)
	}
}

func TestW3VPCEPolicyOpen_HealthyPolicies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		policy string
	}{
		{"scoped to one account", w3VPCEScopedPolicy},
		{"wildcard principal fenced by an org condition", w3VPCEConditionedPolicy},
		{"no policy document", ""},
		{"unparseable document", `{"Statement": [`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := w3FetchVPCEs(t, w3VPCE("vpce-0aa11bb22cc33dd44", "Available", tc.policy))
			if f, ok := w3FindingByCode(rows[0].Findings, w3CodeVPCEPolicyOpen); ok {
				t.Errorf("got %s (%q), want no finding", f.Code, f.Phrase)
			}
		})
	}
}

// TestW3VPCEPolicyOpen_PublicButNarrowActions pins the batch rule that the
// public statement must also grant "*": a wildcard principal restricted to a
// couple of read actions is the pattern operators use deliberately.
func TestW3VPCEPolicyOpen_PublicButNarrowActions(t *testing.T) {
	const doc = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::acme-public/*"}]}`
	rows := w3FetchVPCEs(t, w3VPCE("vpce-0aa11bb22cc33dd44", "Available", doc))
	if f, ok := w3FindingByCode(rows[0].Findings, w3CodeVPCEPolicyOpen); ok {
		t.Errorf("got %s (%q) for a narrow-action public statement, want no finding", f.Code, f.Phrase)
	}
}

// TestW3VPCEPolicyOpen_IndependentOfState pins that a transitional endpoint
// still reports its policy — two conditions, two findings.
func TestW3VPCEPolicyOpen_IndependentOfState(t *testing.T) {
	rows := w3FetchVPCEs(t, w3VPCE("vpce-0aa11bb22cc33dd44", "Pending", w3VPCEDefaultPolicy))

	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeVPCEPending); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeVPCEPending, rows[0].Findings)
	}
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeVPCEPolicyOpen); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeVPCEPolicyOpen, rows[0].Findings)
	}
	if len(rows[0].Findings) != 2 {
		t.Errorf("got %d findings %+v, want exactly 2", len(rows[0].Findings), rows[0].Findings)
	}
}

// TestW3VPCEPolicyOpen_DeletedEmitsNoPostureFinding pins the disposal rule: a
// deleted endpoint routes nothing, so its policy is not actionable.
func TestW3VPCEPolicyOpen_DeletedEmitsNoPostureFinding(t *testing.T) {
	rows := w3FetchVPCEs(t, w3VPCE("vpce-0aa11bb22cc33dd44", "Deleted", w3VPCEDefaultPolicy))

	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeVPCEPolicyOpen); ok {
		t.Errorf("deleted endpoint carries %s; findings=%+v", w3CodeVPCEPolicyOpen, rows[0].Findings)
	}
	if _, ok := w3FindingByCode(rows[0].Findings, w3CodeVPCEDeleted); !ok {
		t.Errorf("missing %s; findings=%+v", w3CodeVPCEDeleted, rows[0].Findings)
	}
}

// TestW3VPCEPolicyOpen_PerRowIsolation pins that one open endpoint in a page
// does not colour its neighbours.
func TestW3VPCEPolicyOpen_PerRowIsolation(t *testing.T) {
	rows := w3FetchVPCEs(t,
		w3VPCE("vpce-0scoped000000000", "Available", w3VPCEScopedPolicy),
		w3VPCE("vpce-0open0000000000", "Available", w3VPCEDefaultPolicy),
		w3VPCE("vpce-0nopolicy000000", "Available", ""),
	)
	for _, r := range rows {
		_, got := w3FindingByCode(r.Findings, w3CodeVPCEPolicyOpen)
		want := r.ID == "vpce-0open0000000000"
		if got != want {
			t.Errorf("%s: has %s = %v, want %v", r.ID, w3CodeVPCEPolicyOpen, got, want)
		}
	}
}

// TestW3VPCEPolicyOpen_ConditionValuesAsArray pins that a condition written
// with a list of values still fences the wildcard principal. IAM accepts both
// a scalar and an array for every condition value, and a parser that only
// handles the scalar would report a hardened multi-org endpoint as wide open.
func TestW3VPCEPolicyOpen_ConditionValuesAsArray(t *testing.T) {
	const doc = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "*",
      "Resource": "*",
      "Condition": {"StringEquals": {"aws:PrincipalOrgID": ["o-abc123def4", "o-zzz999yyy8"]}}
    }
  ]
}`
	rows := w3FetchVPCEs(t, w3VPCE("vpce-0aa11bb22cc33dd44", "Available", doc))
	if f, ok := w3FindingByCode(rows[0].Findings, w3CodeVPCEPolicyOpen); ok {
		t.Errorf("got %s (%q) for an org-fenced policy written with array condition values, want no finding", f.Code, f.Phrase)
	}
}

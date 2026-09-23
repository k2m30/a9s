package unit_test

// DescribeVpcEndpoints returns State in the service's own spelling —
// "available", "pendingAcceptance", "failed" — while the SDK enum constants
// read "Available", "PendingAcceptance", "Failed". Every lifecycle verdict has
// to be reached on the value as it arrives on the wire.

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

type t561VPCEAPI struct{ endpoints []ec2types.VpcEndpoint }

func (f t561VPCEAPI) DescribeVpcEndpoints(_ context.Context, _ *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	return &ec2.DescribeVpcEndpointsOutput{VpcEndpoints: f.endpoints}, nil
}

// t561OpenPolicy is the full-access document AWS attaches to a gateway
// endpoint created without one.
const t561OpenPolicy = `{"Version":"2008-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"*","Resource":"*"}]}`

const t561ScopedPolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-artifacts/*"}]}`

func t561Endpoint(id, state, policy string) ec2types.VpcEndpoint {
	return ec2types.VpcEndpoint{
		VpcEndpointId:     aws.String(id),
		ServiceName:       aws.String("com.amazonaws.us-east-1.s3"),
		VpcEndpointType:   ec2types.VpcEndpointTypeGateway,
		State:             ec2types.State(state),
		VpcId:             aws.String("vpc-0abc1234def567890"),
		RouteTableIds:     []string{"rtb-0abc1234def567890"},
		PolicyDocument:    aws.String(policy),
		OwnerId:           aws.String("123456789012"),
		PrivateDnsEnabled: aws.Bool(false),
	}
}

func t561FetchEndpoint(t *testing.T, state, policy string) resource.Resource {
	t.Helper()
	res, err := awsclient.FetchVPCEndpointsPage(context.Background(), t561VPCEAPI{endpoints: []ec2types.VpcEndpoint{t561Endpoint("vpce-0aa11bb22cc33dd44", state, policy)}}, "")
	if err != nil {
		t.Fatalf("FetchVPCEndpointsPage: %v", err)
	}
	if len(res.Resources) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Resources))
	}
	return res.Resources[0]
}

func t561Codes(fs []domain.Finding) []domain.FindingCode {
	out := make([]domain.FindingCode, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return out
}

func t561Has(fs []domain.Finding, code domain.FindingCode) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

// Each state as AWS sends it yields its lifecycle finding and the finding's
// colour, both on the fetched row and on a row that reaches the classifier
// with its findings stripped (the colour recomputed from Fields).
func TestT561_VPCEStateAsAWSSendsIt(t *testing.T) {
	td := resource.FindResourceType("vpce")
	if td == nil {
		t.Fatal("vpce not registered")
	}
	cases := []struct {
		state string
		code  domain.FindingCode
		color resource.Color
	}{
		{"available", "", resource.ColorHealthy},
		{"pendingAcceptance", awsclient.CodeVPCEStatePendingAcceptance, resource.ColorWarning},
		{"pending", awsclient.CodeVPCEStatePending, resource.ColorWarning},
		{"deleting", awsclient.CodeVPCEStateDeleting, resource.ColorWarning},
		{"deleted", awsclient.CodeVPCEStateDeleted, resource.ColorDim},
		{"rejected", awsclient.CodeVPCEStateRejected, resource.ColorBroken},
		{"failed", awsclient.CodeVPCEStateFailed, resource.ColorBroken},
		{"expired", awsclient.CodeVPCEStateExpired, resource.ColorBroken},
		{"partial", awsclient.CodeVPCEStatePartial, resource.ColorBroken},
	}
	for _, c := range cases {
		t.Run(c.state, func(t *testing.T) {
			r := t561FetchEndpoint(t, c.state, t561ScopedPolicy)
			if c.code == "" {
				if len(r.Findings) != 0 {
					t.Errorf("an available endpoint with a scoped policy carries %v, want none", t561Codes(r.Findings))
				}
			} else if got := t561Codes(r.Findings); len(got) != 1 || got[0] != c.code {
				t.Errorf("findings = %v, want [%s]", got, c.code)
			}
			if got := td.ResolveColor(r); got != c.color {
				t.Errorf("row colour = %v, want %v", got, c.color)
			}
			r.Findings = nil
			if got := td.ResolveColor(r); got != c.color {
				t.Errorf("row colour recomputed from Fields = %v, want %v", got, c.color)
			}
		})
	}
}

// An endpoint being torn down carries no live exposure, so an open policy on a
// deleting or deleted endpoint is not reported; on any other state it is, and
// stacks beside the lifecycle finding.
func TestT561_VPCEOpenPolicyOnTornDownEndpointIsNotReported(t *testing.T) {
	cases := []struct {
		state string
		want  []domain.FindingCode
	}{
		{"available", []domain.FindingCode{awsclient.CodeVPCEPolicyOpen}},
		{"pending", []domain.FindingCode{awsclient.CodeVPCEStatePending, awsclient.CodeVPCEPolicyOpen}},
		{"deleting", []domain.FindingCode{awsclient.CodeVPCEStateDeleting}},
		{"deleted", []domain.FindingCode{awsclient.CodeVPCEStateDeleted}},
	}
	for _, c := range cases {
		t.Run(c.state, func(t *testing.T) {
			r := t561FetchEndpoint(t, c.state, t561OpenPolicy)
			got := t561Codes(r.Findings)
			if len(got) != len(c.want) {
				t.Fatalf("findings = %v, want %v", got, c.want)
			}
			for _, code := range c.want {
				if !t561Has(r.Findings, code) {
					t.Errorf("findings = %v, missing %s", got, code)
				}
			}
		})
	}
}

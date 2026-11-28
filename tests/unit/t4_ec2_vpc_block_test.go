package unit

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

type t4VPCBlockFake struct {
	*pw1EC2EnrichFake
	mode       ec2types.InternetGatewayBlockMode
	exclusions []ec2types.VpcBlockPublicAccessExclusion
	optErr     error
	exclErr    error

	callMu    sync.Mutex
	optReads  int
	exclReads int
}

func (f *t4VPCBlockFake) DescribeVpcBlockPublicAccessOptions(_ context.Context, _ *ec2.DescribeVpcBlockPublicAccessOptionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcBlockPublicAccessOptionsOutput, error) {
	f.callMu.Lock()
	f.optReads++
	f.callMu.Unlock()
	if f.optErr != nil {
		return nil, f.optErr
	}
	return &ec2.DescribeVpcBlockPublicAccessOptionsOutput{VpcBlockPublicAccessOptions: &ec2types.VpcBlockPublicAccessOptions{
		AwsAccountId:             aws.String("123456789012"),
		AwsRegion:                aws.String("us-east-1"),
		InternetGatewayBlockMode: f.mode,
		ExclusionsAllowed:        ec2types.VpcBlockPublicAccessExclusionsAllowedAllowed,
		State:                    ec2types.VpcBlockPublicAccessStateUpdateComplete,
		ManagedBy:                ec2types.ManagedByAccount,
	}}, nil
}

func (f *t4VPCBlockFake) DescribeVpcBlockPublicAccessExclusions(_ context.Context, _ *ec2.DescribeVpcBlockPublicAccessExclusionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcBlockPublicAccessExclusionsOutput, error) {
	f.callMu.Lock()
	f.exclReads++
	f.callMu.Unlock()
	if f.exclErr != nil {
		return nil, f.exclErr
	}
	return &ec2.DescribeVpcBlockPublicAccessExclusionsOutput{VpcBlockPublicAccessExclusions: f.exclusions}, nil
}

func (f *t4VPCBlockFake) reads() (options, exclusions int) {
	f.callMu.Lock()
	defer f.callMu.Unlock()
	return f.optReads, f.exclReads
}

func t4Exclusion(resourceType, id string, mode ec2types.InternetGatewayExclusionMode, state ec2types.VpcBlockPublicAccessExclusionState) ec2types.VpcBlockPublicAccessExclusion {
	return ec2types.VpcBlockPublicAccessExclusion{
		ExclusionId:                  aws.String("exclusion-0t4" + id[len(id)-6:]),
		ResourceArn:                  aws.String("arn:aws:ec2:us-east-1:123456789012:" + resourceType + "/" + id),
		InternetGatewayExclusionMode: mode,
		State:                        state,
	}
}

const t4ExposedID = "i-0t4exposed00aaaa1"

func t4ExposedInstance(publicIP string, sg string) ec2types.Instance {
	return pw1Instance(t4ExposedID, "running", publicIP, ec2types.HttpTokensStateRequired, sg)
}

func t4EnrichEC2(t *testing.T, fake *t4VPCBlockFake, inst ec2types.Instance, groups ...ec2types.SecurityGroup) awsclient.IssueEnricherResult {
	t.Helper()
	cache := pw1SGCache(t, groups...)
	cache["rtb"] = pw1PublicRTB()
	res, _ := awsclient.EnrichEC2InstanceStatus(context.Background(), &awsclient.ServiceClients{EC2: fake}, pw1FetchEC2(t, inst), cache) //nolint:errcheck // a refused read is an input in some cases; the row state is asserted
	return res
}

func t4SSHOpen() ec2types.SecurityGroup { return pw1SG("sg-0t4ssh0000aaaaa1", 22, 22, false) }

// block-ingress: "Block all internet traffic to the VPCs in this Region
// (except for VPCs or subnets which are excluded)"; block-bidirectional blocks
// traffic to and from internet gateways, with the same exception.
func TestT4_EC2_VPCBlockPublicAccessClosesTheInternetPath(t *testing.T) {
	for _, tc := range []struct {
		mode    ec2types.InternetGatewayBlockMode
		exposed bool
	}{
		{ec2types.InternetGatewayBlockModeBlockIngress, false},
		{ec2types.InternetGatewayBlockModeBlockBidirectional, false},
		{ec2types.InternetGatewayBlockModeOff, true},
	} {
		t.Run(string(tc.mode), func(t *testing.T) {
			fake := &t4VPCBlockFake{pw1EC2EnrichFake: &pw1EC2EnrichFake{}, mode: tc.mode}
			res := t4EnrichEC2(t, fake, t4ExposedInstance("203.0.113.60", "sg-0t4ssh0000aaaaa1"), t4SSHOpen())
			if got := hasCode(res.Findings[t4ExposedID], pw1EC2CodeInternetExposed); got != tc.exposed {
				t.Errorf("mode %s: %s present = %v, want %v", tc.mode, pw1EC2CodeInternetExposed, got, tc.exposed)
			}
			if cause, marked := res.TruncatedIDs[t4ExposedID]; marked {
				t.Errorf("mode %s: instance marked not inspected (%q); every read answered", tc.mode, cause)
			}
		})
	}
}

func TestT4_EC2_ExclusionThatAllowsIngressKeepsTheInstanceExposed(t *testing.T) {
	const done = ec2types.VpcBlockPublicAccessExclusionStateCreateComplete
	bidir := ec2types.InternetGatewayExclusionModeAllowBidirectional
	for _, tc := range []struct {
		name       string
		mode       ec2types.InternetGatewayBlockMode
		exclusions []ec2types.VpcBlockPublicAccessExclusion
		exposed    bool
	}{
		{"vpc excluded", ec2types.InternetGatewayBlockModeBlockIngress,
			[]ec2types.VpcBlockPublicAccessExclusion{t4Exclusion("vpc", pw1VPC, bidir, done)}, true},
		{"subnet excluded", ec2types.InternetGatewayBlockModeBlockBidirectional,
			[]ec2types.VpcBlockPublicAccessExclusion{t4Exclusion("subnet", pw1PublicSubnet, bidir, done)}, true},
		{"egress-only exclusion", ec2types.InternetGatewayBlockModeBlockBidirectional,
			[]ec2types.VpcBlockPublicAccessExclusion{t4Exclusion("vpc", pw1VPC, ec2types.InternetGatewayExclusionModeAllowEgress, done)}, false},
		{"another vpc excluded", ec2types.InternetGatewayBlockModeBlockIngress,
			[]ec2types.VpcBlockPublicAccessExclusion{t4Exclusion("vpc", "vpc-0t4other00000001", bidir, done)}, false},
		{"deleted exclusion", ec2types.InternetGatewayBlockModeBlockIngress,
			[]ec2types.VpcBlockPublicAccessExclusion{t4Exclusion("vpc", pw1VPC, bidir, ec2types.VpcBlockPublicAccessExclusionStateDeleteComplete)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &t4VPCBlockFake{pw1EC2EnrichFake: &pw1EC2EnrichFake{}, mode: tc.mode, exclusions: tc.exclusions}
			res := t4EnrichEC2(t, fake, t4ExposedInstance("203.0.113.61", "sg-0t4ssh0000aaaaa1"), t4SSHOpen())
			if got := hasCode(res.Findings[t4ExposedID], pw1EC2CodeInternetExposed); got != tc.exposed {
				t.Errorf("%s present = %v, want %v", pw1EC2CodeInternetExposed, got, tc.exposed)
			}
		})
	}
}

func TestT4_EC2_RefusedVPCBlockReadMarksTheInstance(t *testing.T) {
	for _, tc := range []struct {
		name string
		fake *t4VPCBlockFake
		op   string
	}{
		{"options refused", &t4VPCBlockFake{pw1EC2EnrichFake: &pw1EC2EnrichFake{},
			optErr: t4Refusal("EC2", "DescribeVpcBlockPublicAccessOptions")}, "DescribeVpcBlockPublicAccessOptions"},
		{"exclusions refused", &t4VPCBlockFake{pw1EC2EnrichFake: &pw1EC2EnrichFake{}, mode: ec2types.InternetGatewayBlockModeBlockIngress,
			exclErr: t4Refusal("EC2", "DescribeVpcBlockPublicAccessExclusions")}, "DescribeVpcBlockPublicAccessExclusions"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := t4EnrichEC2(t, tc.fake, t4ExposedInstance("203.0.113.62", "sg-0t4ssh0000aaaaa1"), t4SSHOpen())
			cause, marked := res.TruncatedIDs[t4ExposedID]
			if !marked {
				t.Fatalf("TruncatedIDs = %v, want %s marked: its verdict depends on the unread VPC block", res.TruncatedIDs, t4ExposedID)
			}
			if !strings.Contains(cause, tc.op) {
				t.Errorf("cause = %q, want it to name %s", cause, tc.op)
			}
			if hasCode(res.Findings[t4ExposedID], pw1EC2CodeInternetExposed) {
				t.Errorf("findings = %v, want no %s from an unread VPC block", res.Findings[t4ExposedID], pw1EC2CodeInternetExposed)
			}
		})
	}
}

func TestT4_EC2_VPCBlockIsNotReadWithoutAPublicPath(t *testing.T) {
	for _, tc := range []struct {
		name  string
		inst  ec2types.Instance
		group ec2types.SecurityGroup
	}{
		{"no public address", t4ExposedInstance("", "sg-0t4ssh0000aaaaa1"), t4SSHOpen()},
		{"no open port", t4ExposedInstance("203.0.113.63", "sg-0t4https00aaaa1"), pw1ClosedSG("sg-0t4https00aaaa1")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &t4VPCBlockFake{pw1EC2EnrichFake: &pw1EC2EnrichFake{}, mode: ec2types.InternetGatewayBlockModeOff}
			t4EnrichEC2(t, fake, tc.inst, tc.group)
			if opt, excl := fake.reads(); opt+excl != 0 {
				t.Errorf("VPC block public access read %d+%d times, want 0: no instance has a path from the internet", opt, excl)
			}
		})
	}
}

func TestT4_EC2_VPCBlockIsReadOncePerRegion(t *testing.T) {
	fake := &t4VPCBlockFake{pw1EC2EnrichFake: &pw1EC2EnrichFake{}, mode: ec2types.InternetGatewayBlockModeBlockIngress}
	clients := &awsclient.ServiceClients{EC2: fake}
	cache := pw1SGCache(t, t4SSHOpen())
	cache["rtb"] = pw1PublicRTB()
	rows := pw1FetchEC2(t,
		pw1Instance("i-0t4batch000aaaa1", "running", "203.0.113.64", ec2types.HttpTokensStateRequired, "sg-0t4ssh0000aaaaa1"),
		pw1Instance("i-0t4batch000aaaa2", "running", "203.0.113.65", ec2types.HttpTokensStateRequired, "sg-0t4ssh0000aaaaa1"),
	)
	for range 2 {
		if _, err := awsclient.EnrichEC2InstanceStatus(context.Background(), clients, rows, cache); err != nil {
			t.Fatalf("EnrichEC2InstanceStatus: %v", err)
		}
	}
	if opt, excl := fake.reads(); opt != 1 || excl > 1 {
		t.Errorf("options read %d times, exclusions %d, over two passes of two instances; want 1 and at most 1", opt, excl)
	}
}

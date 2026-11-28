package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

type t5WAF struct {
	w4WAFFake
	defaults map[string]wafv2types.DefaultAction
}

func (f *t5WAF) GetWebACL(ctx context.Context, in *wafv2.GetWebACLInput, opts ...func(*wafv2.Options)) (*wafv2.GetWebACLOutput, error) {
	out, err := f.w4WAFFake.GetWebACL(ctx, in, opts...)
	if err == nil {
		d := f.defaults[aws.ToString(in.Name)]
		out.WebACL.DefaultAction = &d
	}
	return out, err
}

// A web ACL applies its DefaultAction to every request no rule matches, so an
// ACL with no rules and DefaultAction Block blocks everything and one with
// DefaultAction Allow lets everything through.
func TestT5WAFNoRulesDependsOnDefaultAction(t *testing.T) {
	fake := &t5WAF{
		w4WAFFake: w4WAFFake{rules: map[string][]wafv2types.Rule{
			"acme-open-acl":   nil,
			"acme-closed-acl": nil,
			"acme-ruled-acl":  {w4BlockRule("acme-geo-block")},
		}},
		defaults: map[string]wafv2types.DefaultAction{
			"acme-open-acl":   {Allow: &wafv2types.AllowAction{}},
			"acme-closed-acl": {Block: &wafv2types.BlockAction{}},
			"acme-ruled-acl":  {Allow: &wafv2types.AllowAction{}},
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake, Region: "us-east-1"}
	res, err := awsclient.EnrichWAFLogging(context.Background(), clients, []resource.Resource{
		w4WAFResource("acme-open-acl"), w4WAFResource("acme-closed-acl"), w4WAFResource("acme-ruled-acl"),
	}, nil)
	if err != nil {
		t.Fatalf("EnrichWAFLogging: %v", err)
	}

	open := res.Findings[w4WAFARN("acme-open-acl")]
	i := -1
	for j, f := range open {
		if f.Code == w4CodeWAFNoRules {
			i = j
		}
	}
	if i < 0 {
		t.Fatalf("acme-open-acl (no rules, DefaultAction Allow): no %s; findings=%+v", w4CodeWAFNoRules, open)
	}
	allowSentence := open[i].Detail

	for _, f := range res.Findings[w4WAFARN("acme-closed-acl")] {
		if f.Code == w4CodeWAFNoRules && f.Detail == allowSentence {
			t.Errorf("acme-closed-acl (no rules, DefaultAction Block) carries %s with the Allow ACL's sentence %q", f.Code, f.Detail)
		}
	}
	if _, marked := res.TruncatedIDs[w4WAFARN("acme-closed-acl")]; marked {
		t.Errorf("acme-closed-acl marked not inspected, want inspected")
	}
	w4AssertNoCode(t, res.Findings[w4WAFARN("acme-ruled-acl")], w4CodeWAFNoRules)
}

type t5VPCWorld struct {
	awsclient.EC2API
	vpcs    []ec2types.Vpc
	subnets []ec2types.Subnet
}

func (f *t5VPCWorld) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{Vpcs: f.vpcs}, nil
}

func (f *t5VPCWorld) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return &ec2.DescribeSubnetsOutput{Subnets: f.subnets}, nil
}

func (f *t5VPCWorld) DescribeFlowLogs(_ context.Context, _ *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	return &ec2.DescribeFlowLogsOutput{}, nil
}

// A VPC shared through AWS RAM is owned by another account; a participant
// cannot create a flow log on it, so its missing flow logs are the owner's to
// fix, not this account's.
func TestT5SharedVPCHasNoFlowLogFinding(t *testing.T) {
	fake := &t5VPCWorld{
		vpcs: []ec2types.Vpc{
			{VpcId: aws.String("vpc-0a1b2c3d4e5f60001"), OwnerId: aws.String(t5Account), CidrBlock: aws.String("10.10.0.0/16"), State: ec2types.VpcStateAvailable, IsDefault: aws.Bool(false),
				Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("acme-own")}}},
			{VpcId: aws.String("vpc-0a1b2c3d4e5f60002"), OwnerId: aws.String("210987654321"), CidrBlock: aws.String("10.20.0.0/16"), State: ec2types.VpcStateAvailable, IsDefault: aws.Bool(false),
				Tags: []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("acme-shared")}}},
		},
		subnets: []ec2types.Subnet{
			{SubnetId: aws.String("subnet-0a1b2c3d4e5f60001"), VpcId: aws.String("vpc-0a1b2c3d4e5f60001"), OwnerId: aws.String(t5Account), CidrBlock: aws.String("10.10.1.0/24")},
			{SubnetId: aws.String("subnet-0a1b2c3d4e5f60002"), VpcId: aws.String("vpc-0a1b2c3d4e5f60002"), OwnerId: aws.String(t5Account), CidrBlock: aws.String("10.20.1.0/24")},
		},
	}
	page, err := awsclient.FetchVPCsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchVPCsPage: %v", err)
	}
	identity := session.NewIdentityStore()
	identity.Set(t5Account, nil)
	clients := &awsclient.ServiceClients{EC2: fake, Region: "us-east-1"}
	clients.SetIdentityStore(identity)

	res, err := awsclient.EnrichVPCFlowLogs(context.Background(), clients, page.Resources, nil)
	if err != nil {
		t.Fatalf("EnrichVPCFlowLogs: %v", err)
	}
	if !t5Has(res.Findings["vpc-0a1b2c3d4e5f60001"], "vpc.no-flow-logs") {
		t.Errorf("own VPC without flow logs: no vpc.no-flow-logs; findings=%+v", res.Findings["vpc-0a1b2c3d4e5f60001"])
	}
	if t5Has(res.Findings["vpc-0a1b2c3d4e5f60002"], "vpc.no-flow-logs") {
		t.Errorf("VPC owned by 210987654321 carries vpc.no-flow-logs, want none")
	}
}

type t5PolicyList struct {
	*t5IAM
	policies []iamtypes.Policy
}

func (f *t5PolicyList) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{Policies: f.policies}, nil
}

// A policy used as a permissions boundary is in use even with no attachment:
// AttachmentCount counts attachments only, PermissionsBoundaryUsageCount
// counts the principals it bounds.
func TestT5BoundaryOnlyPolicyIsNotUnattached(t *testing.T) {
	iamWorld := newT5IAM()
	iamWorld.docs[t5PolicyARN("acme-dev-boundary")] = t5ScopedDoc
	iamWorld.docs[t5PolicyARN("acme-leftover")] = t5ScopedDoc
	policy := func(name string, attached, boundary int32) iamtypes.Policy {
		return iamtypes.Policy{
			Arn: aws.String(t5PolicyARN(name)), PolicyName: aws.String(name), PolicyId: aws.String("ANPAEXAMPLE" + name),
			Path: aws.String("/"), DefaultVersionId: aws.String("v1"), IsAttachable: true,
			AttachmentCount: aws.Int32(attached), PermissionsBoundaryUsageCount: aws.Int32(boundary),
			CreateDate: t5Created(), UpdateDate: t5Created(),
		}
	}
	fake := &t5PolicyList{t5IAM: iamWorld, policies: []iamtypes.Policy{
		policy("acme-dev-boundary", 0, 3),
		policy("acme-leftover", 0, 0),
	}}
	page, err := awsclient.FetchIAMPoliciesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchIAMPoliciesPage: %v", err)
	}
	rows := map[string]resource.Resource{}
	for _, r := range page.Resources {
		rows[r.Name] = r
	}
	res, err := awsclient.EnrichIAMPolicy(context.Background(), &awsclient.ServiceClients{IAM: fake}, page.Resources, nil)
	if err != nil {
		t.Fatalf("EnrichIAMPolicy: %v", err)
	}

	boundary := rows["acme-dev-boundary"]
	if t5Has(boundary.Findings, "iam-policy.orphan-unattached") {
		t.Errorf("acme-dev-boundary (boundary of 3 principals) carries iam-policy.orphan-unattached")
	}
	if got := res.FieldUpdates[boundary.ID]["risk"]; got != "" {
		t.Errorf("acme-dev-boundary Status = %q, want %q", got, "")
	}

	leftover := rows["acme-leftover"]
	if !t5Has(leftover.Findings, "iam-policy.orphan-unattached") {
		t.Errorf("acme-leftover (no attachment, no boundary use): no iam-policy.orphan-unattached; findings=%+v", leftover.Findings)
	}
	if got := res.FieldUpdates[leftover.ID]["risk"]; got != "unattached" {
		t.Errorf("acme-leftover Status = %q, want %q", got, "unattached")
	}
}

// In a dual-stack VPC AWS creates the default group's egress as all traffic to
// 0.0.0.0/0 and to ::/0, and DescribeSecurityGroups returns both ranges in one
// permission. A rule anyone added lands in the same permission when its
// protocol matches, so the ranges, not the permission count, are the evidence.
func TestT5DefaultSGDualStackEgressIsAWSDefault(t *testing.T) {
	allTraffic := func(v4 []string, v6 []string) ec2types.IpPermission {
		p := ec2types.IpPermission{IpProtocol: aws.String("-1")}
		for _, c := range v4 {
			p.IpRanges = append(p.IpRanges, ec2types.IpRange{CidrIp: aws.String(c)})
		}
		for _, c := range v6 {
			p.Ipv6Ranges = append(p.Ipv6Ranges, ec2types.Ipv6Range{CidrIpv6: aws.String(c)})
		}
		return p
	}
	defaultGroup := func(id string, ingress, egress []ec2types.IpPermission) ec2types.SecurityGroup {
		return ec2types.SecurityGroup{
			GroupId: aws.String(id), GroupName: aws.String("default"), VpcId: aws.String("vpc-0a1b2c3d4e5f60001"),
			OwnerId: aws.String(t5Account), Description: aws.String("default VPC security group"),
			IpPermissions: ingress, IpPermissionsEgress: egress,
		}
	}
	untouched := allTraffic([]string{"0.0.0.0/0"}, []string{"::/0"})
	rows := w3FetchSGs(t,
		defaultGroup("sg-0a1b2c3d4e5f60001", nil, []ec2types.IpPermission{untouched}),
		defaultGroup("sg-0a1b2c3d4e5f60002", []ec2types.IpPermission{w3TCPFromInternet(22)}, []ec2types.IpPermission{untouched}),
		defaultGroup("sg-0a1b2c3d4e5f60003", nil, []ec2types.IpPermission{untouched, {
			IpProtocol: aws.String("tcp"), FromPort: aws.Int32(443), ToPort: aws.Int32(443),
			Ipv6Ranges: []ec2types.Ipv6Range{{CidrIpv6: aws.String("2001:db8::/32")}},
		}}),
		defaultGroup("sg-0a1b2c3d4e5f60004", nil, []ec2types.IpPermission{allTraffic([]string{"0.0.0.0/0", "10.0.0.0/8"}, []string{"::/0"})}),
	)
	want := map[string]bool{
		"sg-0a1b2c3d4e5f60001": false,
		"sg-0a1b2c3d4e5f60002": true,
		"sg-0a1b2c3d4e5f60003": true,
		"sg-0a1b2c3d4e5f60004": true,
	}
	for _, r := range rows {
		_, got := w3FindingByCode(r.Findings, w3CodeSGDefaultWithRules)
		if got != want[r.ID] {
			t.Errorf("%s: %s = %v, want %v; findings=%+v", r.ID, w3CodeSGDefaultWithRules, got, want[r.ID], r.Findings)
		}
	}
}

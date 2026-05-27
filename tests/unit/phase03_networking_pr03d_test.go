package unit_test

// Why: tgw "failed" is not in the SDK TransitGatewayState enum but is
// handled as a string case in types_networking.go Color func — the test
// uses a raw ec2types.TransitGatewayState("failed") cast to match.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2svc "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// =============================================================================
// VPC
// =============================================================================

// TestPR03d_VPCFetcher_PendingEmitsWarnFinding asserts that a VPC in the
// "pending" state emits one SevWarn Finding with CodeVPCStatePending and
// no Status string.
func TestPR03d_VPCFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03dVPCMock{
		vpcs: []ec2types.Vpc{
			{
				VpcId:     aws.String("vpc-0pending12345"),
				CidrBlock: aws.String("10.10.0.0/16"),
				State:     ec2types.VpcStatePending,
				IsDefault: aws.Bool(false),
				OwnerId:   aws.String("000000000000"),
			},
		},
	}

	resources, err := awsclient.FetchVPCs(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchVPCs: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for pending VPC", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeVPCStatePending {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeVPCStatePending)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
	if r.Fields["state"] != "pending" {
		t.Errorf("Fields[\"state\"]: got %q, want %q (state must be preserved in Fields)", r.Fields["state"], "pending")
	}
}

// TestPR03d_VPCColor_ReadsWave1First pins that the vpc Color func evaluates
// Findings before the legacy Fields["state"] switch. Pre-migration the Color
// func does not read Findings at all, so this test will fail until the Color
// func is updated.
//
// Setup: Finding{SevBroken, wave1} + Fields["state"]="available"
// Expect: ColorBroken (Findings wins, not legacy "available"→ColorHealthy)
func TestPR03d_VPCColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("vpc")
	if td == nil {
		t.Fatal("vpc type def not found in registry")
	}

	r := resource.Resource{
		Type: "vpc",
		Findings: []domain.Finding{
			{Code: awsclient.CodeVPCStatePending, Phrase: "pending", Severity: domain.SevBroken, Source: "wave1"},
		},
		Fields: map[string]string{"state": "available"},
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("vpc Color with wave1 SevBroken + state=available: got %v, want ColorBroken (Findings must take precedence over legacy state switch)", got)
	}
}

// ---------------------------------------------------------------------------
// VPC mock
// ---------------------------------------------------------------------------

type pr03dVPCMock struct {
	vpcs []ec2types.Vpc
}

func (m *pr03dVPCMock) DescribeVpcs(
	_ context.Context,
	_ *ec2svc.DescribeVpcsInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeVpcsOutput, error) {
	return &ec2svc.DescribeVpcsOutput{Vpcs: m.vpcs}, nil
}

// =============================================================================
// SUBNET
// =============================================================================

// TestPR03d_SubnetFetcher_PendingEmitsWarnFinding asserts that a subnet in
// "pending" state emits one SevWarn Finding with CodeSubnetStatePending.
func TestPR03d_SubnetFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03dSubnetMock{
		subnets: []ec2types.Subnet{
			{
				SubnetId:                aws.String("subnet-0abc1234pending"),
				VpcId:                   aws.String("vpc-01234abcd"),
				CidrBlock:               aws.String("10.0.1.0/24"),
				AvailabilityZone:        aws.String("us-east-1a"),
				State:                   ec2types.SubnetStatePending,
				AvailableIpAddressCount: aws.Int32(0),
			},
		},
	}

	result, err := awsclient.FetchSubnetsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchSubnetsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for pending subnet", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeSubnetStatePending {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeSubnetStatePending)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_SubnetFetcher_UnavailableEmitsBrokenFinding asserts that a subnet
// in "unavailable" state emits one SevBroken Finding with
// CodeSubnetStateUnavailable.
func TestPR03d_SubnetFetcher_UnavailableEmitsBrokenFinding(t *testing.T) {
	mock := &pr03dSubnetMock{
		subnets: []ec2types.Subnet{
			{
				SubnetId:                aws.String("subnet-0def5678unavail"),
				VpcId:                   aws.String("vpc-01234abcd"),
				CidrBlock:               aws.String("10.0.2.0/24"),
				AvailabilityZone:        aws.String("us-east-1b"),
				State:                   ec2types.SubnetStateUnavailable,
				AvailableIpAddressCount: aws.Int32(0),
			},
		},
	}

	result, err := awsclient.FetchSubnetsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchSubnetsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for unavailable subnet", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeSubnetStateUnavailable {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeSubnetStateUnavailable)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_SubnetColor_ReadsWave1First pins that the subnet Color func
// evaluates Findings before the legacy Fields["state"] switch.
//
// Setup: Finding{SevBroken, wave1} + Fields["state"]="available"
// Expect: ColorBroken (Findings wins over legacy "available"→ColorHealthy)
func TestPR03d_SubnetColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("subnet")
	if td == nil {
		t.Fatal("subnet type def not found in registry")
	}

	r := resource.Resource{
		Type: "subnet",
		Findings: []domain.Finding{
			{Code: awsclient.CodeSubnetStateUnavailable, Phrase: "unavailable", Severity: domain.SevBroken, Source: "wave1"},
		},
		Fields: map[string]string{"state": "available"},
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("subnet Color with wave1 SevBroken + state=available: got %v, want ColorBroken", got)
	}
}

// ---------------------------------------------------------------------------
// Subnet mock
// ---------------------------------------------------------------------------

type pr03dSubnetMock struct {
	subnets []ec2types.Subnet
}

func (m *pr03dSubnetMock) DescribeSubnets(
	_ context.Context,
	_ *ec2svc.DescribeSubnetsInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeSubnetsOutput, error) {
	return &ec2svc.DescribeSubnetsOutput{Subnets: m.subnets}, nil
}

// =============================================================================
// ELB (Elastic Load Balancer v2)
// =============================================================================

// TestPR03d_ELBFetcher_ProvisioningEmitsWarnFinding asserts that a load
// balancer in "provisioning" state emits one SevWarn Finding with
// CodeELBStateProvisioning.
func TestPR03d_ELBFetcher_ProvisioningEmitsWarnFinding(t *testing.T) {
	mock := &pr03dELBMock{
		lbs: []elbv2types.LoadBalancer{
			{
				LoadBalancerName: aws.String("staging-alb"),
				LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:000000000000:loadbalancer/app/staging-alb/abc123"),
				DNSName:          aws.String("staging-alb.us-east-1.elb.amazonaws.com"),
				Type:             elbv2types.LoadBalancerTypeEnumApplication,
				Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
				State: &elbv2types.LoadBalancerState{
					Code: elbv2types.LoadBalancerStateEnumProvisioning,
				},
				VpcId: aws.String("vpc-01234abcd"),
			},
		},
	}

	result, err := awsclient.FetchLoadBalancersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchLoadBalancersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for provisioning ELB", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeELBStateProvisioning {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeELBStateProvisioning)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_ELBFetcher_FailedEmitsBrokenFinding asserts that a load balancer
// in "failed" state emits one SevBroken Finding with CodeELBStateFailed.
func TestPR03d_ELBFetcher_FailedEmitsBrokenFinding(t *testing.T) {
	mock := &pr03dELBMock{
		lbs: []elbv2types.LoadBalancer{
			{
				LoadBalancerName: aws.String("broken-nlb"),
				LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:000000000000:loadbalancer/net/broken-nlb/def456"),
				DNSName:          aws.String("broken-nlb.us-east-1.elb.amazonaws.com"),
				Type:             elbv2types.LoadBalancerTypeEnumNetwork,
				Scheme:           elbv2types.LoadBalancerSchemeEnumInternal,
				State: &elbv2types.LoadBalancerState{
					Code: elbv2types.LoadBalancerStateEnumFailed,
				},
				VpcId: aws.String("vpc-01234abcd"),
			},
		},
	}

	result, err := awsclient.FetchLoadBalancersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchLoadBalancersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed ELB", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeELBStateFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeELBStateFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_ELBColor_ReadsWave1First pins that the elb Color func evaluates
// Findings before the legacy Fields["state"] switch.
//
// Setup: Finding{SevBroken, wave1} + Fields["state"]="active"
// Expect: ColorBroken (Findings wins over legacy "active"→ColorHealthy)
func TestPR03d_ELBColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("elb")
	if td == nil {
		t.Fatal("elb type def not found in registry")
	}

	r := resource.Resource{
		Type: "elb",
		Findings: []domain.Finding{
			{Code: awsclient.CodeELBStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
		},
		Fields: map[string]string{"state": "active"},
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("elb Color with wave1 SevBroken + state=active: got %v, want ColorBroken", got)
	}
}

// ---------------------------------------------------------------------------
// ELB mock
// ---------------------------------------------------------------------------

type pr03dELBMock struct {
	lbs []elbv2types.LoadBalancer
}

func (m *pr03dELBMock) DescribeLoadBalancers(
	_ context.Context,
	_ *elbv2svc.DescribeLoadBalancersInput,
	_ ...func(*elbv2svc.Options),
) (*elbv2svc.DescribeLoadBalancersOutput, error) {
	return &elbv2svc.DescribeLoadBalancersOutput{LoadBalancers: m.lbs}, nil
}

// =============================================================================
// IGW (Internet Gateway)
// =============================================================================

// TestPR03d_IGWFetcher_AttachingEmitsWarnFinding asserts that an IGW in the
// "attaching" attachment state emits one SevWarn Finding with
// CodeIGWStateAttaching.
func TestPR03d_IGWFetcher_AttachingEmitsWarnFinding(t *testing.T) {
	mock := &pr03dIGWMock{
		igws: []ec2types.InternetGateway{
			{
				InternetGatewayId: aws.String("igw-0abc1234attaching"),
				Attachments: []ec2types.InternetGatewayAttachment{
					{
						VpcId: aws.String("vpc-01234abcd"),
						State: ec2types.AttachmentStatusAttaching,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchInternetGatewaysPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchInternetGatewaysPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for attaching IGW", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeIGWStateAttaching {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeIGWStateAttaching)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_IGWColor_ReadsWave1First pins that the igw Color func evaluates
// Findings before the legacy Fields["state"] / attachments_count switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["state"]="attached", attachments_count="1"
// Expect: ColorWarning (Findings wins; without Findings the legacy switch
// returns ColorHealthy for state=attached + attachments=1).
func TestPR03d_IGWColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("igw")
	if td == nil {
		t.Fatal("igw type def not found in registry")
	}

	r := resource.Resource{
		Type: "igw",
		Findings: []domain.Finding{
			{Code: awsclient.CodeIGWStateAttaching, Phrase: "attaching", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{
			"state":             "attached",
			"attachments_count": "1",
		},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("igw Color with wave1 SevWarn + state=attached,attachments=1: got %v, want ColorWarning", got)
	}
}

// ---------------------------------------------------------------------------
// IGW mock
// ---------------------------------------------------------------------------

type pr03dIGWMock struct {
	igws []ec2types.InternetGateway
}

func (m *pr03dIGWMock) DescribeInternetGateways(
	_ context.Context,
	_ *ec2svc.DescribeInternetGatewaysInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeInternetGatewaysOutput, error) {
	return &ec2svc.DescribeInternetGatewaysOutput{InternetGateways: m.igws}, nil
}

// =============================================================================
// NAT (NAT Gateway)
// =============================================================================

// TestPR03d_NATFetcher_PendingEmitsWarnFinding asserts that a NAT gateway in
// "pending" state emits one SevWarn Finding with CodeNATStatePending.
func TestPR03d_NATFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03dNATMock{
		nats: []ec2types.NatGateway{
			{
				NatGatewayId: aws.String("nat-0abc1234pending56"),
				VpcId:        aws.String("vpc-01234abcd"),
				SubnetId:     aws.String("subnet-01234abcd"),
				State:        ec2types.NatGatewayStatePending,
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-nat")},
				},
			},
		},
	}

	result, err := awsclient.FetchNatGatewaysPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchNatGatewaysPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for pending NAT", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeNATStatePending {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeNATStatePending)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_NATFetcher_FailedEmitsBrokenFinding asserts that a NAT gateway
// in "failed" state emits one SevBroken Finding with CodeNATStateFailed.
func TestPR03d_NATFetcher_FailedEmitsBrokenFinding(t *testing.T) {
	mock := &pr03dNATMock{
		nats: []ec2types.NatGateway{
			{
				NatGatewayId: aws.String("nat-0def5678failed90"),
				VpcId:        aws.String("vpc-01234abcd"),
				SubnetId:     aws.String("subnet-01234abcd"),
				State:        ec2types.NatGatewayStateFailed,
			},
		},
	}

	result, err := awsclient.FetchNatGatewaysPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchNatGatewaysPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed NAT", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeNATStateFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeNATStateFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_NATColor_ReadsWave1First pins that the nat Color func evaluates
// Findings before the legacy Fields["state"] switch.
//
// Setup: Finding{SevBroken, wave1} + Fields["state"]="available"
// Expect: ColorBroken (Findings wins over legacy "available"→ColorHealthy)
func TestPR03d_NATColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("nat")
	if td == nil {
		t.Fatal("nat type def not found in registry")
	}

	r := resource.Resource{
		Type: "nat",
		Findings: []domain.Finding{
			{Code: awsclient.CodeNATStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
		},
		Fields: map[string]string{"state": "available"},
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("nat Color with wave1 SevBroken + state=available: got %v, want ColorBroken", got)
	}
}

// ---------------------------------------------------------------------------
// NAT mock
// ---------------------------------------------------------------------------

type pr03dNATMock struct {
	nats []ec2types.NatGateway
}

func (m *pr03dNATMock) DescribeNatGateways(
	_ context.Context,
	_ *ec2svc.DescribeNatGatewaysInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeNatGatewaysOutput, error) {
	return &ec2svc.DescribeNatGatewaysOutput{NatGateways: m.nats}, nil
}

// =============================================================================
// VPCE (VPC Endpoint)
// =============================================================================

// TestPR03d_VPCEFetcher_PendingEmitsWarnFinding asserts that a VPC endpoint
// in "Pending" state emits one SevWarn Finding with CodeVPCEStatePending.
func TestPR03d_VPCEFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03dVPCEMock{
		endpoints: []ec2types.VpcEndpoint{
			{
				VpcEndpointId:   aws.String("vpce-0abc1234pending56"),
				ServiceName:     aws.String("com.amazonaws.us-east-1.s3"),
				VpcEndpointType: ec2types.VpcEndpointTypeInterface,
				State:           ec2types.StatePending,
				VpcId:           aws.String("vpc-01234abcd"),
			},
		},
	}

	result, err := awsclient.FetchVPCEndpointsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchVPCEndpointsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for pending VPCE", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeVPCEStatePending {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeVPCEStatePending)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_VPCEFetcher_FailedEmitsBrokenFinding asserts that a VPC endpoint
// in "Failed" state emits one SevBroken Finding with CodeVPCEStateFailed.
func TestPR03d_VPCEFetcher_FailedEmitsBrokenFinding(t *testing.T) {
	mock := &pr03dVPCEMock{
		endpoints: []ec2types.VpcEndpoint{
			{
				VpcEndpointId:   aws.String("vpce-0def5678failed90"),
				ServiceName:     aws.String("com.amazonaws.us-east-1.ec2"),
				VpcEndpointType: ec2types.VpcEndpointTypeInterface,
				State:           ec2types.StateFailed,
				VpcId:           aws.String("vpc-01234abcd"),
			},
		},
	}

	result, err := awsclient.FetchVPCEndpointsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchVPCEndpointsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed VPCE", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeVPCEStateFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeVPCEStateFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_VPCEColor_ReadsWave1First pins that the vpce Color func evaluates
// Findings before the legacy Fields["state"] switch.
//
// Setup: Finding{SevBroken, wave1} + Fields["state"]="Available"
// Expect: ColorBroken (Findings wins over legacy "Available"→ColorHealthy)
func TestPR03d_VPCEColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("vpce")
	if td == nil {
		t.Fatal("vpce type def not found in registry")
	}

	r := resource.Resource{
		Type: "vpce",
		Findings: []domain.Finding{
			{Code: awsclient.CodeVPCEStateFailed, Phrase: "Failed", Severity: domain.SevBroken, Source: "wave1"},
		},
		Fields: map[string]string{"state": "Available"},
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("vpce Color with wave1 SevBroken + state=Available: got %v, want ColorBroken", got)
	}
}

// ---------------------------------------------------------------------------
// VPCE mock
// ---------------------------------------------------------------------------

type pr03dVPCEMock struct {
	endpoints []ec2types.VpcEndpoint
}

func (m *pr03dVPCEMock) DescribeVpcEndpoints(
	_ context.Context,
	_ *ec2svc.DescribeVpcEndpointsInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeVpcEndpointsOutput, error) {
	return &ec2svc.DescribeVpcEndpointsOutput{VpcEndpoints: m.endpoints}, nil
}

// =============================================================================
// TGW (Transit Gateway)
// =============================================================================

// TestPR03d_TGWFetcher_PendingEmitsWarnFinding asserts that a transit gateway
// in "pending" state emits one SevWarn Finding with CodeTGWStatePending.
func TestPR03d_TGWFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03dTGWMock{
		tgws: []ec2types.TransitGateway{
			{
				TransitGatewayId: aws.String("tgw-0abc1234pending56"),
				OwnerId:          aws.String("000000000000"),
				State:            ec2types.TransitGatewayStatePending,
				Description:      aws.String("main transit gateway"),
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String("prod-tgw")},
				},
			},
		},
	}

	result, err := awsclient.FetchTransitGatewaysPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchTransitGatewaysPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for pending TGW", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeTGWStatePending {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeTGWStatePending)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_TGWFetcher_FailedEmitsBrokenFinding asserts that a transit gateway
// in "failed" state emits one SevBroken Finding with CodeTGWStateFailed.
//
// NOTE: "failed" is not in the TransitGatewayState SDK enum but IS handled
// by the Color func. The test uses a raw cast to match that Color branch.
func TestPR03d_TGWFetcher_FailedEmitsBrokenFinding(t *testing.T) {
	mock := &pr03dTGWMock{
		tgws: []ec2types.TransitGateway{
			{
				TransitGatewayId: aws.String("tgw-0def5678failed90"),
				OwnerId:          aws.String("000000000000"),
				State:            ec2types.TransitGatewayState("failed"), // raw cast: not in SDK enum
			},
		},
	}

	result, err := awsclient.FetchTransitGatewaysPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchTransitGatewaysPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed TGW", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeTGWStateFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeTGWStateFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03d_TGWColor_ReadsWave1First pins that the tgw Color func evaluates
// Findings before the legacy Fields["state"] switch.
//
// Setup: Finding{SevBroken, wave1} + Fields["state"]="available"
// Expect: ColorBroken (Findings wins over legacy "available"→ColorHealthy)
func TestPR03d_TGWColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("tgw")
	if td == nil {
		t.Fatal("tgw type def not found in registry")
	}

	r := resource.Resource{
		Type: "tgw",
		Findings: []domain.Finding{
			{Code: awsclient.CodeTGWStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
		},
		Fields: map[string]string{"state": "available"},
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("tgw Color with wave1 SevBroken + state=available: got %v, want ColorBroken", got)
	}
}

// ---------------------------------------------------------------------------
// TGW mock
// ---------------------------------------------------------------------------

type pr03dTGWMock struct {
	tgws []ec2types.TransitGateway
}

func (m *pr03dTGWMock) DescribeTransitGateways(
	_ context.Context,
	_ *ec2svc.DescribeTransitGatewaysInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeTransitGatewaysOutput, error) {
	return &ec2svc.DescribeTransitGatewaysOutput{TransitGateways: m.tgws}, nil
}

// =============================================================================
// RTB (Route Table) — special case: drop Status write, no Findings emitted
// =============================================================================

// TestPR03d_RTBFetcher_NeverEmitsFindingsOrStatus asserts that the route table
// fetcher NEVER writes Status (isMain is structural metadata, not a health
// state) and NEVER emits Findings (rtb's Color is structural: blackhole routes
// and unassociated non-main tables). Fields["is_main"] must still be present.
//
// Migration: remove `Status: isMain` from the Resource literal in rtb.go.
// Findings: none — rtb has no lifecycle findings to emit.
func TestPR03d_RTBFetcher_NeverEmitsFindingsOrStatus(t *testing.T) {
	cases := []struct {
		name    string
		isMain  bool
		assocs  []ec2types.RouteTableAssociation
	}{
		{
			name:   "main route table",
			isMain: true,
			assocs: []ec2types.RouteTableAssociation{
				{Main: aws.Bool(true)},
			},
		},
		{
			name:   "non-main route table with subnet association",
			isMain: false,
			assocs: []ec2types.RouteTableAssociation{
				{Main: aws.Bool(false), SubnetId: aws.String("subnet-01234abcd")},
			},
		},
		{
			name:   "non-main unassociated route table",
			isMain: false,
			assocs: []ec2types.RouteTableAssociation{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expectedIsMain := "false"
			if tc.isMain {
				expectedIsMain = "true"
			}

			mock := &pr03dRTBMock{
				rtbs: []ec2types.RouteTable{
					{
						RouteTableId: aws.String("rtb-0abc1234567890ef"),
						VpcId:        aws.String("vpc-01234abcd"),
						Associations: tc.assocs,
						Routes:       []ec2types.Route{},
					},
				},
			}

			result, err := awsclient.FetchRouteTablesPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("FetchRouteTablesPage: unexpected error: %v", err)
			}
			if len(result.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(result.Resources))
			}
			r := result.Resources[0]

			// Status must be empty — "is_main" is NOT a lifecycle state.
			// No Findings — rtb has no lifecycle states to emit.
			if len(r.Findings) != 0 {
				t.Errorf("Findings: got %d, want 0 (rtb emits no wave1 Findings)", len(r.Findings))
			}
			// Fields["is_main"] must still be present for Color's structural check.
			if got := r.Fields["is_main"]; got != expectedIsMain {
				t.Errorf("Fields[\"is_main\"]: got %q, want %q", got, expectedIsMain)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RTB mock
// ---------------------------------------------------------------------------

type pr03dRTBMock struct {
	rtbs []ec2types.RouteTable
}

func (m *pr03dRTBMock) DescribeRouteTables(
	_ context.Context,
	_ *ec2svc.DescribeRouteTablesInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeRouteTablesOutput, error) {
	return &ec2svc.DescribeRouteTablesOutput{RouteTables: m.rtbs}, nil
}

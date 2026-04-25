package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ===========================================================================
// Security Groups — EC2 DescribeSecurityGroups (NextToken)
// ===========================================================================

type mockSGPaginatedClient struct {
	outputs []*ec2.DescribeSecurityGroupsOutput
	inputs  []*ec2.DescribeSecurityGroupsInput
	err     error
	callIdx int
}

func (m *mockSGPaginatedClient) DescribeSecurityGroups(
	ctx context.Context,
	params *ec2.DescribeSecurityGroupsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeSecurityGroupsOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeSecurityGroupsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchSecurityGroups_Pagination(t *testing.T) {
	mock := &mockSGPaginatedClient{
		outputs: []*ec2.DescribeSecurityGroupsOutput{
			{
				NextToken: aws.String("page2-token"),
				SecurityGroups: []ec2types.SecurityGroup{
					{GroupId: aws.String("sg-page1-001"), GroupName: aws.String("web-sg"), VpcId: aws.String("vpc-123"), Description: aws.String("Web")},
					{GroupId: aws.String("sg-page1-002"), GroupName: aws.String("db-sg"), VpcId: aws.String("vpc-123"), Description: aws.String("DB")},
				},
			},
			{
				SecurityGroups: []ec2types.SecurityGroup{
					{GroupId: aws.String("sg-page2-001"), GroupName: aws.String("app-sg"), VpcId: aws.String("vpc-456"), Description: aws.String("App")},
				},
			},
		},
	}

	resources, err := awsclient.FetchSecurityGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if resources[0].ID != "sg-page1-001" {
			t.Errorf("expected %q, got %q", "sg-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if resources[2].ID != "sg-page2-001" {
			t.Errorf("expected %q, got %q", "sg-page2-001", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// Subnets — EC2 DescribeSubnets (NextToken)
// ===========================================================================

type mockSubnetPaginatedClient struct {
	outputs []*ec2.DescribeSubnetsOutput
	inputs  []*ec2.DescribeSubnetsInput
	err     error
	callIdx int
}

func (m *mockSubnetPaginatedClient) DescribeSubnets(
	ctx context.Context,
	params *ec2.DescribeSubnetsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeSubnetsOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeSubnetsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchSubnets_Pagination(t *testing.T) {
	mock := &mockSubnetPaginatedClient{
		outputs: []*ec2.DescribeSubnetsOutput{
			{
				NextToken: aws.String("page2-token"),
				Subnets: []ec2types.Subnet{
					{
						SubnetId:                aws.String("subnet-page1-001"),
						VpcId:                   aws.String("vpc-123"),
						CidrBlock:               aws.String("10.0.1.0/24"),
						AvailabilityZone:        aws.String("us-east-1a"),
						State:                   ec2types.SubnetStateAvailable,
						AvailableIpAddressCount: aws.Int32(250),
					},
				},
			},
			{
				Subnets: []ec2types.Subnet{
					{
						SubnetId:                aws.String("subnet-page2-001"),
						VpcId:                   aws.String("vpc-456"),
						CidrBlock:               aws.String("10.0.2.0/24"),
						AvailabilityZone:        aws.String("us-east-1b"),
						State:                   ec2types.SubnetStateAvailable,
						AvailableIpAddressCount: aws.Int32(200),
					},
					{
						SubnetId:                aws.String("subnet-page2-002"),
						VpcId:                   aws.String("vpc-456"),
						CidrBlock:               aws.String("10.0.3.0/24"),
						AvailabilityZone:        aws.String("us-east-1c"),
						State:                   ec2types.SubnetStateAvailable,
						AvailableIpAddressCount: aws.Int32(100),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchSubnets(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_subnet", func(t *testing.T) {
		if resources[0].ID != "subnet-page1-001" {
			t.Errorf("expected %q, got %q", "subnet-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_subnets", func(t *testing.T) {
		if resources[1].ID != "subnet-page2-001" {
			t.Errorf("expected %q, got %q", "subnet-page2-001", resources[1].ID)
		}
		if resources[2].ID != "subnet-page2-002" {
			t.Errorf("expected %q, got %q", "subnet-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// VPCs — EC2 DescribeVpcs (NextToken)
// ===========================================================================

type mockVPCPaginatedClient struct {
	outputs []*ec2.DescribeVpcsOutput
	inputs  []*ec2.DescribeVpcsInput
	err     error
	callIdx int
}

func (m *mockVPCPaginatedClient) DescribeVpcs(
	ctx context.Context,
	params *ec2.DescribeVpcsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeVpcsOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeVpcsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchVPCs_Pagination(t *testing.T) {
	mock := &mockVPCPaginatedClient{
		outputs: []*ec2.DescribeVpcsOutput{
			{
				NextToken: aws.String("page2-token"),
				Vpcs: []ec2types.Vpc{
					{VpcId: aws.String("vpc-page1-001"), CidrBlock: aws.String("10.0.0.0/16"), State: ec2types.VpcStateAvailable, IsDefault: aws.Bool(true)},
				},
			},
			{
				Vpcs: []ec2types.Vpc{
					{VpcId: aws.String("vpc-page2-001"), CidrBlock: aws.String("172.16.0.0/16"), State: ec2types.VpcStateAvailable, IsDefault: aws.Bool(false)},
					{VpcId: aws.String("vpc-page2-002"), CidrBlock: aws.String("192.168.0.0/16"), State: ec2types.VpcStateAvailable, IsDefault: aws.Bool(false)},
				},
			},
		},
	}

	resources, err := awsclient.FetchVPCs(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_vpc", func(t *testing.T) {
		if resources[0].ID != "vpc-page1-001" {
			t.Errorf("expected %q, got %q", "vpc-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_vpcs", func(t *testing.T) {
		if resources[1].ID != "vpc-page2-001" {
			t.Errorf("expected %q, got %q", "vpc-page2-001", resources[1].ID)
		}
		if resources[2].ID != "vpc-page2-002" {
			t.Errorf("expected %q, got %q", "vpc-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// VPC Endpoints — EC2 DescribeVpcEndpoints (NextToken)
// ===========================================================================

type mockVPCEPaginatedClient struct {
	outputs []*ec2.DescribeVpcEndpointsOutput
	inputs  []*ec2.DescribeVpcEndpointsInput
	err     error
	callIdx int
}

func (m *mockVPCEPaginatedClient) DescribeVpcEndpoints(
	ctx context.Context,
	params *ec2.DescribeVpcEndpointsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeVpcEndpointsOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeVpcEndpointsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchVPCEndpoints_Pagination(t *testing.T) {
	mock := &mockVPCEPaginatedClient{
		outputs: []*ec2.DescribeVpcEndpointsOutput{
			{
				NextToken: aws.String("page2-token"),
				VpcEndpoints: []ec2types.VpcEndpoint{
					{VpcEndpointId: aws.String("vpce-page1-001"), ServiceName: aws.String("com.amazonaws.s3"), VpcEndpointType: ec2types.VpcEndpointTypeGateway, State: ec2types.StateAvailable, VpcId: aws.String("vpc-123")},
				},
			},
			{
				VpcEndpoints: []ec2types.VpcEndpoint{
					{VpcEndpointId: aws.String("vpce-page2-001"), ServiceName: aws.String("com.amazonaws.ec2"), VpcEndpointType: ec2types.VpcEndpointTypeInterface, State: ec2types.StateAvailable, VpcId: aws.String("vpc-456")},
					{VpcEndpointId: aws.String("vpce-page2-002"), ServiceName: aws.String("com.amazonaws.sqs"), VpcEndpointType: ec2types.VpcEndpointTypeInterface, State: ec2types.StateAvailable, VpcId: aws.String("vpc-456")},
				},
			},
		},
	}

	resources, err := awsclient.FetchVPCEndpoints(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_vpce", func(t *testing.T) {
		if resources[0].ID != "vpce-page1-001" {
			t.Errorf("expected %q, got %q", "vpce-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_vpces", func(t *testing.T) {
		if resources[1].ID != "vpce-page2-001" {
			t.Errorf("expected %q, got %q", "vpce-page2-001", resources[1].ID)
		}
		if resources[2].ID != "vpce-page2-002" {
			t.Errorf("expected %q, got %q", "vpce-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// NAT Gateways — EC2 DescribeNatGateways (NextToken)
// ===========================================================================

type mockNATPaginatedClient struct {
	outputs []*ec2.DescribeNatGatewaysOutput
	inputs  []*ec2.DescribeNatGatewaysInput
	err     error
	callIdx int
}

func (m *mockNATPaginatedClient) DescribeNatGateways(
	ctx context.Context,
	params *ec2.DescribeNatGatewaysInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeNatGatewaysOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeNatGatewaysOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchNatGateways_Pagination(t *testing.T) {
	mock := &mockNATPaginatedClient{
		outputs: []*ec2.DescribeNatGatewaysOutput{
			{
				NextToken: aws.String("page2-token"),
				NatGateways: []ec2types.NatGateway{
					{NatGatewayId: aws.String("nat-page1-001"), VpcId: aws.String("vpc-123"), SubnetId: aws.String("subnet-aaa"), State: ec2types.NatGatewayStateAvailable},
				},
			},
			{
				NatGateways: []ec2types.NatGateway{
					{NatGatewayId: aws.String("nat-page2-001"), VpcId: aws.String("vpc-456"), SubnetId: aws.String("subnet-bbb"), State: ec2types.NatGatewayStateAvailable},
					{NatGatewayId: aws.String("nat-page2-002"), VpcId: aws.String("vpc-456"), SubnetId: aws.String("subnet-ccc"), State: ec2types.NatGatewayStatePending},
				},
			},
		},
	}

	resources, err := awsclient.FetchNatGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_nat", func(t *testing.T) {
		if resources[0].ID != "nat-page1-001" {
			t.Errorf("expected %q, got %q", "nat-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_nats", func(t *testing.T) {
		if resources[1].ID != "nat-page2-001" {
			t.Errorf("expected %q, got %q", "nat-page2-001", resources[1].ID)
		}
		if resources[2].ID != "nat-page2-002" {
			t.Errorf("expected %q, got %q", "nat-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// Internet Gateways — EC2 DescribeInternetGateways (NextToken)
// ===========================================================================

type mockIGWPaginatedClient struct {
	outputs []*ec2.DescribeInternetGatewaysOutput
	inputs  []*ec2.DescribeInternetGatewaysInput
	err     error
	callIdx int
}

func (m *mockIGWPaginatedClient) DescribeInternetGateways(
	ctx context.Context,
	params *ec2.DescribeInternetGatewaysInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInternetGatewaysOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeInternetGatewaysOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchInternetGateways_Pagination(t *testing.T) {
	mock := &mockIGWPaginatedClient{
		outputs: []*ec2.DescribeInternetGatewaysOutput{
			{
				NextToken: aws.String("page2-token"),
				InternetGateways: []ec2types.InternetGateway{
					{InternetGatewayId: aws.String("igw-page1-001"), Attachments: []ec2types.InternetGatewayAttachment{{VpcId: aws.String("vpc-123"), State: ec2types.AttachmentStatusAttached}}},
				},
			},
			{
				InternetGateways: []ec2types.InternetGateway{
					{InternetGatewayId: aws.String("igw-page2-001"), Attachments: []ec2types.InternetGatewayAttachment{{VpcId: aws.String("vpc-456"), State: ec2types.AttachmentStatusAttached}}},
					{InternetGatewayId: aws.String("igw-page2-002")},
				},
			},
		},
	}

	resources, err := awsclient.FetchInternetGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_igw", func(t *testing.T) {
		if resources[0].ID != "igw-page1-001" {
			t.Errorf("expected %q, got %q", "igw-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_igws", func(t *testing.T) {
		if resources[1].ID != "igw-page2-001" {
			t.Errorf("expected %q, got %q", "igw-page2-001", resources[1].ID)
		}
		if resources[2].ID != "igw-page2-002" {
			t.Errorf("expected %q, got %q", "igw-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// ENI — EC2 DescribeNetworkInterfaces (NextToken)
// ===========================================================================

type mockENIPaginatedClient struct {
	outputs []*ec2.DescribeNetworkInterfacesOutput
	inputs  []*ec2.DescribeNetworkInterfacesInput
	err     error
	callIdx int
}

func (m *mockENIPaginatedClient) DescribeNetworkInterfaces(
	ctx context.Context,
	params *ec2.DescribeNetworkInterfacesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeNetworkInterfacesOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeNetworkInterfacesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchNetworkInterfaces_Pagination(t *testing.T) {
	mock := &mockENIPaginatedClient{
		outputs: []*ec2.DescribeNetworkInterfacesOutput{
			{
				NextToken: aws.String("page2-token"),
				NetworkInterfaces: []ec2types.NetworkInterface{
					{NetworkInterfaceId: aws.String("eni-page1-001"), Status: ec2types.NetworkInterfaceStatusInUse, InterfaceType: ec2types.NetworkInterfaceTypeInterface, VpcId: aws.String("vpc-123"), PrivateIpAddress: aws.String("10.0.1.5")},
				},
			},
			{
				NetworkInterfaces: []ec2types.NetworkInterface{
					{NetworkInterfaceId: aws.String("eni-page2-001"), Status: ec2types.NetworkInterfaceStatusAvailable, InterfaceType: ec2types.NetworkInterfaceTypeInterface, VpcId: aws.String("vpc-456"), PrivateIpAddress: aws.String("10.0.2.10")},
					{NetworkInterfaceId: aws.String("eni-page2-002"), Status: ec2types.NetworkInterfaceStatusInUse, InterfaceType: ec2types.NetworkInterfaceTypeInterface, VpcId: aws.String("vpc-456"), PrivateIpAddress: aws.String("10.0.2.20")},
				},
			},
		},
	}

	resources, err := awsclient.FetchNetworkInterfaces(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_eni", func(t *testing.T) {
		if resources[0].ID != "eni-page1-001" {
			t.Errorf("expected %q, got %q", "eni-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_enis", func(t *testing.T) {
		if resources[1].ID != "eni-page2-001" {
			t.Errorf("expected %q, got %q", "eni-page2-001", resources[1].ID)
		}
		if resources[2].ID != "eni-page2-002" {
			t.Errorf("expected %q, got %q", "eni-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// Route Tables — EC2 DescribeRouteTables (NextToken)
// ===========================================================================

type mockRTBPaginatedClient struct {
	outputs []*ec2.DescribeRouteTablesOutput
	inputs  []*ec2.DescribeRouteTablesInput
	err     error
	callIdx int
}

func (m *mockRTBPaginatedClient) DescribeRouteTables(
	ctx context.Context,
	params *ec2.DescribeRouteTablesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeRouteTablesOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeRouteTablesOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchRouteTables_Pagination(t *testing.T) {
	mock := &mockRTBPaginatedClient{
		outputs: []*ec2.DescribeRouteTablesOutput{
			{
				NextToken: aws.String("page2-token"),
				RouteTables: []ec2types.RouteTable{
					{RouteTableId: aws.String("rtb-page1-001"), VpcId: aws.String("vpc-123"), Routes: []ec2types.Route{{DestinationCidrBlock: aws.String("0.0.0.0/0")}}},
				},
			},
			{
				RouteTables: []ec2types.RouteTable{
					{RouteTableId: aws.String("rtb-page2-001"), VpcId: aws.String("vpc-456"), Routes: []ec2types.Route{{DestinationCidrBlock: aws.String("10.0.0.0/8")}, {DestinationCidrBlock: aws.String("0.0.0.0/0")}}},
					{RouteTableId: aws.String("rtb-page2-002"), VpcId: aws.String("vpc-456")},
				},
			},
		},
	}

	resources, err := awsclient.FetchRouteTables(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_rtb", func(t *testing.T) {
		if resources[0].ID != "rtb-page1-001" {
			t.Errorf("expected %q, got %q", "rtb-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_rtbs", func(t *testing.T) {
		if resources[1].ID != "rtb-page2-001" {
			t.Errorf("expected %q, got %q", "rtb-page2-001", resources[1].ID)
		}
		if resources[2].ID != "rtb-page2-002" {
			t.Errorf("expected %q, got %q", "rtb-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// Transit Gateways — EC2 DescribeTransitGateways (NextToken)
// ===========================================================================

type mockTGWPaginatedClient struct {
	outputs []*ec2.DescribeTransitGatewaysOutput
	inputs  []*ec2.DescribeTransitGatewaysInput
	err     error
	callIdx int
}

func (m *mockTGWPaginatedClient) DescribeTransitGateways(
	ctx context.Context,
	params *ec2.DescribeTransitGatewaysInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeTransitGatewaysOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &ec2.DescribeTransitGatewaysOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchTransitGateways_Pagination(t *testing.T) {
	mock := &mockTGWPaginatedClient{
		outputs: []*ec2.DescribeTransitGatewaysOutput{
			{
				NextToken: aws.String("page2-token"),
				TransitGateways: []ec2types.TransitGateway{
					{TransitGatewayId: aws.String("tgw-page1-001"), State: ec2types.TransitGatewayStateAvailable, OwnerId: aws.String("111122223333"), Description: aws.String("Main TGW")},
				},
			},
			{
				TransitGateways: []ec2types.TransitGateway{
					{TransitGatewayId: aws.String("tgw-page2-001"), State: ec2types.TransitGatewayStateAvailable, OwnerId: aws.String("111122223333"), Description: aws.String("Secondary TGW")},
					{TransitGatewayId: aws.String("tgw-page2-002"), State: ec2types.TransitGatewayStatePending, OwnerId: aws.String("444455556666"), Description: aws.String("New TGW")},
				},
			},
		},
	}

	resources, err := awsclient.FetchTransitGateways(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_tgw", func(t *testing.T) {
		if resources[0].ID != "tgw-page1-001" {
			t.Errorf("expected %q, got %q", "tgw-page1-001", resources[0].ID)
		}
	})

	t.Run("page2_tgws", func(t *testing.T) {
		if resources[1].ID != "tgw-page2-001" {
			t.Errorf("expected %q, got %q", "tgw-page2-001", resources[1].ID)
		}
		if resources[2].ID != "tgw-page2-002" {
			t.Errorf("expected %q, got %q", "tgw-page2-002", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// ELB — ELBv2 DescribeLoadBalancers (Marker/NextMarker)
// ===========================================================================

type mockELBPaginatedClient struct {
	outputs []*elbv2.DescribeLoadBalancersOutput
	inputs  []*elbv2.DescribeLoadBalancersInput
	err     error
	callIdx int
}

func (m *mockELBPaginatedClient) DescribeLoadBalancers(
	ctx context.Context,
	params *elbv2.DescribeLoadBalancersInput,
	optFns ...func(*elbv2.Options),
) (*elbv2.DescribeLoadBalancersOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &elbv2.DescribeLoadBalancersOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchLoadBalancers_Pagination(t *testing.T) {
	mock := &mockELBPaginatedClient{
		outputs: []*elbv2.DescribeLoadBalancersOutput{
			{
				NextMarker: aws.String("page2-marker"),
				LoadBalancers: []elbv2types.LoadBalancer{
					{
						LoadBalancerName: aws.String("page1-lb-1"),
						DNSName:          aws.String("page1-lb-1.elb.amazonaws.com"),
						Type:             elbv2types.LoadBalancerTypeEnumApplication,
						Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
						State:            &elbv2types.LoadBalancerState{Code: elbv2types.LoadBalancerStateEnumActive},
						VpcId:            aws.String("vpc-123"),
						LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/app/page1-lb-1/abc123"),
					},
				},
			},
			{
				LoadBalancers: []elbv2types.LoadBalancer{
					{
						LoadBalancerName: aws.String("page2-lb-1"),
						DNSName:          aws.String("page2-lb-1.elb.amazonaws.com"),
						Type:             elbv2types.LoadBalancerTypeEnumNetwork,
						Scheme:           elbv2types.LoadBalancerSchemeEnumInternal,
						State:            &elbv2types.LoadBalancerState{Code: elbv2types.LoadBalancerStateEnumActive},
						VpcId:            aws.String("vpc-456"),
						LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/net/page2-lb-1/def456"),
					},
					{
						LoadBalancerName: aws.String("page2-lb-2"),
						DNSName:          aws.String("page2-lb-2.elb.amazonaws.com"),
						Type:             elbv2types.LoadBalancerTypeEnumApplication,
						Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
						State:            &elbv2types.LoadBalancerState{Code: elbv2types.LoadBalancerStateEnumProvisioning},
						VpcId:            aws.String("vpc-456"),
						LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/app/page2-lb-2/ghi789"),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchLoadBalancers(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_lb", func(t *testing.T) {
		if resources[0].ID != "page1-lb-1" {
			t.Errorf("expected %q, got %q", "page1-lb-1", resources[0].ID)
		}
	})

	t.Run("page2_lbs", func(t *testing.T) {
		if resources[1].ID != "page2-lb-1" {
			t.Errorf("expected %q, got %q", "page2-lb-1", resources[1].ID)
		}
		if resources[2].ID != "page2-lb-2" {
			t.Errorf("expected %q, got %q", "page2-lb-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_marker", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].Marker == nil || *mock.inputs[1].Marker != "page2-marker" {
			t.Errorf("Marker not forwarded to page 2: got %v, want %q", mock.inputs[1].Marker, "page2-marker")
		}
	})
}

// ===========================================================================
// Target Groups — ELBv2 DescribeTargetGroups (Marker/NextMarker)
// ===========================================================================

type mockTGPaginatedClient struct {
	outputs []*elbv2.DescribeTargetGroupsOutput
	inputs  []*elbv2.DescribeTargetGroupsInput
	err     error
	callIdx int
}

func (m *mockTGPaginatedClient) DescribeTargetGroups(
	ctx context.Context,
	params *elbv2.DescribeTargetGroupsInput,
	optFns ...func(*elbv2.Options),
) (*elbv2.DescribeTargetGroupsOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &elbv2.DescribeTargetGroupsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchTargetGroups_Pagination(t *testing.T) {
	mock := &mockTGPaginatedClient{
		outputs: []*elbv2.DescribeTargetGroupsOutput{
			{
				NextMarker: aws.String("page2-marker"),
				TargetGroups: []elbv2types.TargetGroup{
					{TargetGroupName: aws.String("page1-tg-1"), Port: aws.Int32(80), Protocol: elbv2types.ProtocolEnumHttp, VpcId: aws.String("vpc-123"), TargetType: elbv2types.TargetTypeEnumInstance, HealthCheckPath: aws.String("/health"), TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:targetgroup/page1-tg-1/abc")},
				},
			},
			{
				TargetGroups: []elbv2types.TargetGroup{
					{TargetGroupName: aws.String("page2-tg-1"), Port: aws.Int32(443), Protocol: elbv2types.ProtocolEnumHttps, VpcId: aws.String("vpc-456"), TargetType: elbv2types.TargetTypeEnumIp, HealthCheckPath: aws.String("/"), TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:targetgroup/page2-tg-1/def")},
					{TargetGroupName: aws.String("page2-tg-2"), Port: aws.Int32(8080), Protocol: elbv2types.ProtocolEnumHttp, VpcId: aws.String("vpc-456"), TargetType: elbv2types.TargetTypeEnumInstance, HealthCheckPath: aws.String("/status"), TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:targetgroup/page2-tg-2/ghi")},
				},
			},
		},
	}

	resources, err := awsclient.FetchTargetGroups(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_tg", func(t *testing.T) {
		if resources[0].ID != "page1-tg-1" {
			t.Errorf("expected %q, got %q", "page1-tg-1", resources[0].ID)
		}
	})

	t.Run("page2_tgs", func(t *testing.T) {
		if resources[1].ID != "page2-tg-1" {
			t.Errorf("expected %q, got %q", "page2-tg-1", resources[1].ID)
		}
		if resources[2].ID != "page2-tg-2" {
			t.Errorf("expected %q, got %q", "page2-tg-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_marker", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].Marker == nil || *mock.inputs[1].Marker != "page2-marker" {
			t.Errorf("Marker not forwarded to page 2: got %v, want %q", mock.inputs[1].Marker, "page2-marker")
		}
	})
}

// ===========================================================================
// Redis — ElastiCache DescribeReplicationGroups (Marker)
// ===========================================================================

type mockRedisPaginatedClient struct {
	outputs []*elasticache.DescribeReplicationGroupsOutput
	inputs  []*elasticache.DescribeReplicationGroupsInput
	err     error
	callIdx int
}

func (m *mockRedisPaginatedClient) DescribeReplicationGroups(
	ctx context.Context,
	params *elasticache.DescribeReplicationGroupsInput,
	optFns ...func(*elasticache.Options),
) (*elasticache.DescribeReplicationGroupsOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &elasticache.DescribeReplicationGroupsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchRedis_Pagination(t *testing.T) {
	mock := &mockRedisPaginatedClient{
		outputs: []*elasticache.DescribeReplicationGroupsOutput{
			{
				Marker: aws.String("page2-marker"),
				ReplicationGroups: []elasticachetypes.ReplicationGroup{
					{ReplicationGroupId: aws.String("page1-redis-rg"), Status: aws.String("available"), Engine: aws.String("redis"), MemberClusters: []string{"page1-redis-rg-001"}},
				},
			},
			{
				ReplicationGroups: []elasticachetypes.ReplicationGroup{
					{ReplicationGroupId: aws.String("page2-redis-rg"), Status: aws.String("available"), Engine: aws.String("redis"), MemberClusters: []string{"page2-redis-rg-001", "page2-redis-rg-002"}},
				},
			},
		},
	}

	resources, err := awsclient.FetchRedis(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 2 {
			t.Fatalf("expected 2 redis resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_redis", func(t *testing.T) {
		if resources[0].ID != "page1-redis-rg" {
			t.Errorf("expected %q, got %q", "page1-redis-rg", resources[0].ID)
		}
	})

	t.Run("page2_redis", func(t *testing.T) {
		if resources[1].ID != "page2-redis-rg" {
			t.Errorf("expected %q, got %q", "page2-redis-rg", resources[1].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_marker", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].Marker == nil || *mock.inputs[1].Marker != "page2-marker" {
			t.Errorf("Marker not forwarded to page 2: got %v, want %q", mock.inputs[1].Marker, "page2-marker")
		}
	})
}

// ===========================================================================
// DocumentDB — DocDB DescribeDBClusters (Marker)
// ===========================================================================

type mockDocDBPaginatedClient struct {
	outputs []*docdb.DescribeDBClustersOutput
	inputs  []*docdb.DescribeDBClustersInput
	err     error
	callIdx int
}

func (m *mockDocDBPaginatedClient) DescribeDBClusters(
	ctx context.Context,
	params *docdb.DescribeDBClustersInput,
	optFns ...func(*docdb.Options),
) (*docdb.DescribeDBClustersOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &docdb.DescribeDBClustersOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchDocDBClusters_Pagination(t *testing.T) {
	mock := &mockDocDBPaginatedClient{
		outputs: []*docdb.DescribeDBClustersOutput{
			{
				Marker: aws.String("page2-marker"),
				DBClusters: []docdbtypes.DBCluster{
					{DBClusterIdentifier: aws.String("page1-docdb-1"), EngineVersion: aws.String("5.0.0"), Status: aws.String("available"), Endpoint: aws.String("page1-docdb-1.cluster-abc.us-east-1.docdb.amazonaws.com")},
				},
			},
			{
				DBClusters: []docdbtypes.DBCluster{
					{DBClusterIdentifier: aws.String("page2-docdb-1"), EngineVersion: aws.String("4.0.0"), Status: aws.String("available"), Endpoint: aws.String("page2-docdb-1.cluster-def.us-east-1.docdb.amazonaws.com")},
					{DBClusterIdentifier: aws.String("page2-docdb-2"), EngineVersion: aws.String("5.0.0"), Status: aws.String("creating"), Endpoint: aws.String("page2-docdb-2.cluster-ghi.us-east-1.docdb.amazonaws.com")},
				},
			},
		},
	}

	resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_docdb", func(t *testing.T) {
		if resources[0].ID != "page1-docdb-1" {
			t.Errorf("expected %q, got %q", "page1-docdb-1", resources[0].ID)
		}
	})

	t.Run("page2_docdbs", func(t *testing.T) {
		if resources[1].ID != "page2-docdb-1" {
			t.Errorf("expected %q, got %q", "page2-docdb-1", resources[1].ID)
		}
		if resources[2].ID != "page2-docdb-2" {
			t.Errorf("expected %q, got %q", "page2-docdb-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_marker", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].Marker == nil || *mock.inputs[1].Marker != "page2-marker" {
			t.Errorf("Marker not forwarded to page 2: got %v, want %q", mock.inputs[1].Marker, "page2-marker")
		}
	})
}

// ===========================================================================
// DocDB Snapshots — DocDB DescribeDBClusterSnapshots (Marker)
// ===========================================================================

type mockDocDBSnapshotsPaginatedClient struct {
	outputs []*docdb.DescribeDBClusterSnapshotsOutput
	inputs  []*docdb.DescribeDBClusterSnapshotsInput
	err     error
	callIdx int
}

func (m *mockDocDBSnapshotsPaginatedClient) DescribeDBClusterSnapshots(
	ctx context.Context,
	params *docdb.DescribeDBClusterSnapshotsInput,
	optFns ...func(*docdb.Options),
) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &docdb.DescribeDBClusterSnapshotsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchDocDBClusterSnapshots_Pagination(t *testing.T) {
	mock := &mockDocDBSnapshotsPaginatedClient{
		outputs: []*docdb.DescribeDBClusterSnapshotsOutput{
			{
				Marker: aws.String("page2-marker"),
				DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{
					{DBClusterSnapshotIdentifier: aws.String("page1-snap-1"), DBClusterIdentifier: aws.String("docdb-prod"), Status: aws.String("available"), Engine: aws.String("docdb"), SnapshotType: aws.String("manual")},
				},
			},
			{
				DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{
					{DBClusterSnapshotIdentifier: aws.String("page2-snap-1"), DBClusterIdentifier: aws.String("docdb-dev"), Status: aws.String("available"), Engine: aws.String("docdb"), SnapshotType: aws.String("automated")},
					{DBClusterSnapshotIdentifier: aws.String("page2-snap-2"), DBClusterIdentifier: aws.String("docdb-prod"), Status: aws.String("creating"), Engine: aws.String("docdb"), SnapshotType: aws.String("manual")},
				},
			},
		},
	}

	resources, err := awsclient.FetchDocDBClusterSnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_snap", func(t *testing.T) {
		if resources[0].ID != "page1-snap-1" {
			t.Errorf("expected %q, got %q", "page1-snap-1", resources[0].ID)
		}
	})

	t.Run("page2_snaps", func(t *testing.T) {
		if resources[1].ID != "page2-snap-1" {
			t.Errorf("expected %q, got %q", "page2-snap-1", resources[1].ID)
		}
		if resources[2].ID != "page2-snap-2" {
			t.Errorf("expected %q, got %q", "page2-snap-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_marker", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].Marker == nil || *mock.inputs[1].Marker != "page2-marker" {
			t.Errorf("Marker not forwarded to page 2: got %v, want %q", mock.inputs[1].Marker, "page2-marker")
		}
	})
}

// ===========================================================================
// DB Instance Snapshots — RDS DescribeDBSnapshots (Marker)
// ===========================================================================

type mockDBISnapshotsPaginatedClient struct {
	outputs []*rds.DescribeDBSnapshotsOutput
	inputs  []*rds.DescribeDBSnapshotsInput
	err     error
	callIdx int
}

func (m *mockDBISnapshotsPaginatedClient) DescribeDBSnapshots(
	ctx context.Context,
	params *rds.DescribeDBSnapshotsInput,
	optFns ...func(*rds.Options),
) (*rds.DescribeDBSnapshotsOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &rds.DescribeDBSnapshotsOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchDBISnapshots_Pagination(t *testing.T) {
	mock := &mockDBISnapshotsPaginatedClient{
		outputs: []*rds.DescribeDBSnapshotsOutput{
			{
				Marker: aws.String("page2-marker"),
				DBSnapshots: []rdstypes.DBSnapshot{
					{DBSnapshotIdentifier: aws.String("page1-snap-1"), DBInstanceIdentifier: aws.String("mydb-prod"), Status: aws.String("available"), Engine: aws.String("mysql"), SnapshotType: aws.String("manual")},
				},
			},
			{
				DBSnapshots: []rdstypes.DBSnapshot{
					{DBSnapshotIdentifier: aws.String("page2-snap-1"), DBInstanceIdentifier: aws.String("mydb-dev"), Status: aws.String("available"), Engine: aws.String("postgres"), SnapshotType: aws.String("automated")},
					{DBSnapshotIdentifier: aws.String("page2-snap-2"), DBInstanceIdentifier: aws.String("mydb-prod"), Status: aws.String("creating"), Engine: aws.String("mysql"), SnapshotType: aws.String("manual")},
				},
			},
		},
	}

	resources, err := awsclient.FetchDBISnapshots(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_snap", func(t *testing.T) {
		if resources[0].ID != "page1-snap-1" {
			t.Errorf("expected %q, got %q", "page1-snap-1", resources[0].ID)
		}
	})

	t.Run("page2_snaps", func(t *testing.T) {
		if resources[1].ID != "page2-snap-1" {
			t.Errorf("expected %q, got %q", "page2-snap-1", resources[1].ID)
		}
		if resources[2].ID != "page2-snap-2" {
			t.Errorf("expected %q, got %q", "page2-snap-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_marker", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].Marker == nil || *mock.inputs[1].Marker != "page2-marker" {
			t.Errorf("Marker not forwarded to page 2: got %v, want %q", mock.inputs[1].Marker, "page2-marker")
		}
	})
}

// ===========================================================================
// Redshift — Redshift DescribeClusters (Marker)
// ===========================================================================

type mockRedshiftPaginatedClient struct {
	outputs []*redshift.DescribeClustersOutput
	inputs  []*redshift.DescribeClustersInput
	err     error
	callIdx int
}

func (m *mockRedshiftPaginatedClient) DescribeClusters(
	ctx context.Context,
	params *redshift.DescribeClustersInput,
	optFns ...func(*redshift.Options),
) (*redshift.DescribeClustersOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.err != nil {
		return nil, m.err
	}
	if m.callIdx >= len(m.outputs) {
		return &redshift.DescribeClustersOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchRedshiftClusters_Pagination(t *testing.T) {
	mock := &mockRedshiftPaginatedClient{
		outputs: []*redshift.DescribeClustersOutput{
			{
				Marker: aws.String("page2-marker"),
				Clusters: []redshifttypes.Cluster{
					{
						ClusterIdentifier: aws.String("page1-rs-1"),
						ClusterStatus:     aws.String("available"),
						NodeType:          aws.String("dc2.large"),
						NumberOfNodes:     aws.Int32(2),
						DBName:            aws.String("dev"),
						Endpoint:          &redshifttypes.Endpoint{Address: aws.String("page1-rs-1.abc.us-east-1.redshift.amazonaws.com")},
						MasterUsername:    aws.String("admin"),
					},
				},
			},
			{
				Clusters: []redshifttypes.Cluster{
					{
						ClusterIdentifier: aws.String("page2-rs-1"),
						ClusterStatus:     aws.String("available"),
						NodeType:          aws.String("ra3.xlplus"),
						NumberOfNodes:     aws.Int32(4),
						DBName:            aws.String("prod"),
						Endpoint:          &redshifttypes.Endpoint{Address: aws.String("page2-rs-1.def.us-east-1.redshift.amazonaws.com")},
						MasterUsername:    aws.String("admin"),
					},
					{
						ClusterIdentifier: aws.String("page2-rs-2"),
						ClusterStatus:     aws.String("creating"),
						NodeType:          aws.String("dc2.8xlarge"),
						NumberOfNodes:     aws.Int32(8),
						DBName:            aws.String("analytics"),
						MasterUsername:    aws.String("admin"),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchRedshiftClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_rs", func(t *testing.T) {
		if resources[0].ID != "page1-rs-1" {
			t.Errorf("expected %q, got %q", "page1-rs-1", resources[0].ID)
		}
	})

	t.Run("page2_rs", func(t *testing.T) {
		if resources[1].ID != "page2-rs-1" {
			t.Errorf("expected %q, got %q", "page2-rs-1", resources[1].ID)
		}
		if resources[2].ID != "page2-rs-2" {
			t.Errorf("expected %q, got %q", "page2-rs-2", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_marker", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].Marker == nil || *mock.inputs[1].Marker != "page2-marker" {
			t.Errorf("Marker not forwarded to page 2: got %v, want %q", mock.inputs[1].Marker, "page2-marker")
		}
	})
}

// ===========================================================================
// EIP — EC2 DescribeAddresses (no pagination — single call, skip test)
// Note: DescribeAddresses does NOT support pagination in the AWS API.
// The output has no NextToken. This is intentional — EIP is excluded from
// the pagination batch.
// ===========================================================================

// ---------------------------------------------------------------------------
// Error propagation tests for Batch 2 paginated fetchers
// ---------------------------------------------------------------------------

func TestBatch2Pagination_ErrorPropagation(t *testing.T) {
	testErr := fmt.Errorf("test API error")

	t.Run("sg_error", func(t *testing.T) {
		mock := &mockSGPaginatedClient{err: testErr}
		_, err := awsclient.FetchSecurityGroups(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("subnet_error", func(t *testing.T) {
		mock := &mockSubnetPaginatedClient{err: testErr}
		_, err := awsclient.FetchSubnets(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("vpc_error", func(t *testing.T) {
		mock := &mockVPCPaginatedClient{err: testErr}
		_, err := awsclient.FetchVPCs(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("vpce_error", func(t *testing.T) {
		mock := &mockVPCEPaginatedClient{err: testErr}
		_, err := awsclient.FetchVPCEndpoints(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("nat_error", func(t *testing.T) {
		mock := &mockNATPaginatedClient{err: testErr}
		_, err := awsclient.FetchNatGateways(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("igw_error", func(t *testing.T) {
		mock := &mockIGWPaginatedClient{err: testErr}
		_, err := awsclient.FetchInternetGateways(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("eni_error", func(t *testing.T) {
		mock := &mockENIPaginatedClient{err: testErr}
		_, err := awsclient.FetchNetworkInterfaces(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("rtb_error", func(t *testing.T) {
		mock := &mockRTBPaginatedClient{err: testErr}
		_, err := awsclient.FetchRouteTables(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("tgw_error", func(t *testing.T) {
		mock := &mockTGWPaginatedClient{err: testErr}
		_, err := awsclient.FetchTransitGateways(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("elb_error", func(t *testing.T) {
		mock := &mockELBPaginatedClient{err: testErr}
		_, err := awsclient.FetchLoadBalancers(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("tg_error", func(t *testing.T) {
		mock := &mockTGPaginatedClient{err: testErr}
		_, err := awsclient.FetchTargetGroups(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("redis_error", func(t *testing.T) {
		mock := &mockRedisPaginatedClient{err: testErr}
		_, err := awsclient.FetchRedis(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("docdb_error", func(t *testing.T) {
		mock := &mockDocDBPaginatedClient{err: testErr}
		_, err := awsclient.FetchDocDBClusters(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("docdb_snap_error", func(t *testing.T) {
		mock := &mockDocDBSnapshotsPaginatedClient{err: testErr}
		_, err := awsclient.FetchDocDBClusterSnapshots(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("rds_snap_error", func(t *testing.T) {
		mock := &mockDBISnapshotsPaginatedClient{err: testErr}
		_, err := awsclient.FetchDBISnapshots(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("redshift_error", func(t *testing.T) {
		mock := &mockRedshiftPaginatedClient{err: testErr}
		_, err := awsclient.FetchRedshiftClusters(context.Background(), mock)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// EC2Fake implements aws.EC2API against fixture data loaded at construction time.
type EC2Fake struct {
	fix *fixtures.EC2Fixtures
}

// NewEC2 constructs an EC2Fake backed by fixture data from the fixtures package.
func NewEC2() *EC2Fake {
	return &EC2Fake{fix: fixtures.NewEC2Fixtures()}
}

func (f *EC2Fake) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{Reservations: f.fix.Reservations}, nil
}

func (f *EC2Fake) DescribeInstanceStatus(_ context.Context, input *ec2.DescribeInstanceStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	if len(input.InstanceIds) == 0 {
		return &ec2.DescribeInstanceStatusOutput{InstanceStatuses: f.fix.InstanceStatuses}, nil
	}
	want := make(map[string]struct{}, len(input.InstanceIds))
	for _, id := range input.InstanceIds {
		want[id] = struct{}{}
	}
	var out []ec2types.InstanceStatus
	for _, s := range f.fix.InstanceStatuses {
		if s.InstanceId == nil {
			continue
		}
		if _, ok := want[*s.InstanceId]; ok {
			out = append(out, s)
		}
	}
	return &ec2.DescribeInstanceStatusOutput{InstanceStatuses: out}, nil
}

func (f *EC2Fake) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{Vpcs: f.fix.Vpcs}, nil
}

func (f *EC2Fake) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{SecurityGroups: f.fix.SecurityGroups}, nil
}

func (f *EC2Fake) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return &ec2.DescribeSubnetsOutput{Subnets: f.fix.Subnets}, nil
}

func (f *EC2Fake) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{RouteTables: f.fix.RouteTables}, nil
}

func (f *EC2Fake) DescribeNatGateways(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{NatGateways: f.fix.NatGateways}, nil
}

func (f *EC2Fake) DescribeInternetGateways(_ context.Context, _ *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	return &ec2.DescribeInternetGatewaysOutput{InternetGateways: f.fix.InternetGateways}, nil
}

func (f *EC2Fake) DescribeAddresses(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{Addresses: f.fix.Addresses}, nil
}

func (f *EC2Fake) DescribeTransitGateways(_ context.Context, _ *ec2.DescribeTransitGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	return &ec2.DescribeTransitGatewaysOutput{TransitGateways: f.fix.TransitGateways}, nil
}

// DescribeTransitGatewayAttachments filters by transit-gateway-id and resource-type
// when those filters are present (matching the live checkTGWVPC call pattern).
func (f *EC2Fake) DescribeTransitGatewayAttachments(_ context.Context, input *ec2.DescribeTransitGatewayAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	if len(input.Filters) == 0 {
		return &ec2.DescribeTransitGatewayAttachmentsOutput{TransitGatewayAttachments: f.fix.TGWAttachments}, nil
	}

	var tgwIDs, resourceTypes []string
	for _, filter := range input.Filters {
		if filter.Name == nil {
			continue
		}
		switch *filter.Name {
		case "transit-gateway-id":
			tgwIDs = filter.Values
		case "resource-type":
			resourceTypes = filter.Values
		}
	}

	tgwSet := toSet(tgwIDs)
	rtSet := toSet(resourceTypes)

	var out []ec2types.TransitGatewayAttachment
	for _, att := range f.fix.TGWAttachments {
		if len(tgwSet) > 0 {
			if att.TransitGatewayId == nil || !tgwSet[*att.TransitGatewayId] {
				continue
			}
		}
		if len(rtSet) > 0 {
			if !rtSet[string(att.ResourceType)] {
				continue
			}
		}
		out = append(out, att)
	}
	return &ec2.DescribeTransitGatewayAttachmentsOutput{TransitGatewayAttachments: out}, nil
}

func (f *EC2Fake) DescribeVpcEndpoints(_ context.Context, _ *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	return &ec2.DescribeVpcEndpointsOutput{VpcEndpoints: f.fix.VpcEndpoints}, nil
}

func (f *EC2Fake) DescribeNetworkInterfaces(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return &ec2.DescribeNetworkInterfacesOutput{NetworkInterfaces: f.fix.NetworkInterfaces}, nil
}

func (f *EC2Fake) DescribeVolumes(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{Volumes: f.fix.Volumes}, nil
}

func (f *EC2Fake) DescribeSnapshots(_ context.Context, _ *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return &ec2.DescribeSnapshotsOutput{Snapshots: f.fix.Snapshots}, nil
}

func (f *EC2Fake) DescribeImages(_ context.Context, input *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	if len(input.ImageIds) == 0 {
		return &ec2.DescribeImagesOutput{Images: f.fix.Images}, nil
	}
	want := toSet(input.ImageIds)
	var out []ec2types.Image
	for _, img := range f.fix.Images {
		if img.ImageId != nil && want[*img.ImageId] {
			out = append(out, img)
		}
	}
	return &ec2.DescribeImagesOutput{Images: out}, nil
}

// DescribeVolumeStatus is a stub for the Wave 2 enrichment interface.
func (f *EC2Fake) DescribeVolumeStatus(_ context.Context, _ *ec2.DescribeVolumeStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumeStatusOutput, error) {
	return &ec2.DescribeVolumeStatusOutput{}, nil
}

// DescribeFlowLogs is a stub for the Wave 2 enrichment interface.
// Returns empty flow logs so all demo VPCs appear without active flow logs.
func (f *EC2Fake) DescribeFlowLogs(_ context.Context, _ *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	return &ec2.DescribeFlowLogsOutput{}, nil
}

// toSet converts a string slice into a lookup map.
func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

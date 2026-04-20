package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// EC2DescribeInstancesAPI defines the interface for the EC2 DescribeInstances operation.
type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// EC2FetchInstancesAPI combines DescribeInstances and DescribeInstanceStatus,
// which are both required by FetchEC2InstancesPage (status enrichment uses the second).
// EC2DescribeInstanceStatusAPI is defined in the Wave 2 enrichment section below.
type EC2FetchInstancesAPI interface {
	EC2DescribeInstancesAPI
	EC2DescribeInstanceStatusAPI
}

// EC2DescribeVpcsAPI defines the interface for the EC2 DescribeVpcs operation.
type EC2DescribeVpcsAPI interface {
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
}

// EC2DescribeSecurityGroupsAPI defines the interface for the EC2 DescribeSecurityGroups operation.
type EC2DescribeSecurityGroupsAPI interface {
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
}

// EC2DescribeSubnetsAPI defines the interface for the EC2 DescribeSubnets operation.
type EC2DescribeSubnetsAPI interface {
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
}

// EC2DescribeRouteTablesAPI defines the interface for the EC2 DescribeRouteTables operation.
type EC2DescribeRouteTablesAPI interface {
	DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
}

// EC2DescribeNatGatewaysAPI defines the interface for the EC2 DescribeNatGateways operation.
type EC2DescribeNatGatewaysAPI interface {
	DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
}

// EC2DescribeInternetGatewaysAPI defines the interface for the EC2 DescribeInternetGateways operation.
type EC2DescribeInternetGatewaysAPI interface {
	DescribeInternetGateways(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error)
}

// EC2DescribeAddressesAPI defines the interface for the EC2 DescribeAddresses operation.
type EC2DescribeAddressesAPI interface {
	DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
}

// EC2DescribeTransitGatewaysAPI defines the interface for the EC2 DescribeTransitGateways operation.
type EC2DescribeTransitGatewaysAPI interface {
	DescribeTransitGateways(ctx context.Context, params *ec2.DescribeTransitGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error)
}

// EC2DescribeTransitGatewayAttachmentsAPI defines the interface for the EC2
// DescribeTransitGatewayAttachments operation.
type EC2DescribeTransitGatewayAttachmentsAPI interface {
	DescribeTransitGatewayAttachments(ctx context.Context, params *ec2.DescribeTransitGatewayAttachmentsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error)
}

// EC2DescribeTransitGatewayVpcAttachmentsAPI enumerates subnets attached
// to a Transit Gateway via VPC attachments. Used to resolve tgw→subnet.
type EC2DescribeTransitGatewayVpcAttachmentsAPI interface {
	DescribeTransitGatewayVpcAttachments(ctx context.Context, params *ec2.DescribeTransitGatewayVpcAttachmentsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error)
}

// EC2DescribeTransitGatewayRouteTablesAPI defines the interface for the EC2
// DescribeTransitGatewayRouteTables operation.
type EC2DescribeTransitGatewayRouteTablesAPI interface {
	DescribeTransitGatewayRouteTables(ctx context.Context, params *ec2.DescribeTransitGatewayRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayRouteTablesOutput, error)
}

// EC2DescribeVpcEndpointsAPI defines the interface for the EC2 DescribeVpcEndpoints operation.
type EC2DescribeVpcEndpointsAPI interface {
	DescribeVpcEndpoints(ctx context.Context, params *ec2.DescribeVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error)
}

// EC2DescribeNetworkInterfacesAPI defines the interface for the EC2 DescribeNetworkInterfaces operation.
type EC2DescribeNetworkInterfacesAPI interface {
	DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
}

// EC2DescribeVolumesAPI defines the interface for the EC2 DescribeVolumes operation.
type EC2DescribeVolumesAPI interface {
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

// EC2DescribeSnapshotsAPI defines the interface for the EC2 DescribeSnapshots operation.
type EC2DescribeSnapshotsAPI interface {
	DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
}

// EC2DescribeImagesAPI defines the interface for the EC2 DescribeImages operation.
type EC2DescribeImagesAPI interface {
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
}

// EC2DescribeInstanceStatusAPI defines the interface for the EC2 DescribeInstanceStatus operation.
type EC2DescribeInstanceStatusAPI interface {
	DescribeInstanceStatus(ctx context.Context, params *ec2.DescribeInstanceStatusInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error)
}

// EC2DescribeVolumeStatusAPI defines the interface for the EC2 DescribeVolumeStatus operation.
type EC2DescribeVolumeStatusAPI interface {
	DescribeVolumeStatus(ctx context.Context, params *ec2.DescribeVolumeStatusInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumeStatusOutput, error)
}

// EC2DescribeFlowLogsAPI defines the interface for the EC2 DescribeFlowLogs operation.
// Used by EnrichVPCFlowLogs to check whether flow logs are active for each VPC.
type EC2DescribeFlowLogsAPI interface {
	DescribeFlowLogs(ctx context.Context, params *ec2.DescribeFlowLogsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error)
}

// EC2DescribeLaunchTemplateVersionsAPI for asg→ami, asg→role, asg→sg via LaunchTemplate.
type EC2DescribeLaunchTemplateVersionsAPI interface {
	DescribeLaunchTemplateVersions(ctx context.Context, params *ec2.DescribeLaunchTemplateVersionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error)
}

// EC2API is the aggregate interface covering all EC2 operations used by a9s fetchers.
// *ec2.Client structurally satisfies this interface.
type EC2API interface {
	EC2DescribeInstancesAPI
	EC2DescribeVpcsAPI
	EC2DescribeSecurityGroupsAPI
	EC2DescribeSubnetsAPI
	EC2DescribeRouteTablesAPI
	EC2DescribeNatGatewaysAPI
	EC2DescribeInternetGatewaysAPI
	EC2DescribeAddressesAPI
	EC2DescribeTransitGatewaysAPI
	EC2DescribeTransitGatewayAttachmentsAPI
	EC2DescribeTransitGatewayVpcAttachmentsAPI
	EC2DescribeTransitGatewayRouteTablesAPI
	EC2DescribeVpcEndpointsAPI
	EC2DescribeNetworkInterfacesAPI
	EC2DescribeVolumesAPI
	EC2DescribeSnapshotsAPI
	EC2DescribeImagesAPI
	EC2DescribeInstanceStatusAPI         // Wave 2 enrichment
	EC2DescribeVolumeStatusAPI           // Wave 2 enrichment
	EC2DescribeFlowLogsAPI               // Wave 2 enrichment
	EC2DescribeLaunchTemplateVersionsAPI // asg→ami, asg→role, asg→sg
}

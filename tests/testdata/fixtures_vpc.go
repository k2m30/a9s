package testdata

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// RealVPCs returns sanitized VPC data based on real AWS structure.
// Account: 123456789012 (sanitized)
// Region: us-east-1
// Total VPCs: 2
func RealVPCs() []ec2types.Vpc {
	return []ec2types.Vpc{
		{
			VpcId:           aws.String("vpc-0aaa1111bbb2222cc"),
			CidrBlock:       aws.String("10.0.0.0/16"),
			State:           ec2types.VpcStateAvailable,
			IsDefault:       aws.Bool(false),
			OwnerId:         aws.String("123456789012"),
			DhcpOptionsId:   aws.String("dopt-0aaa111111111111a"),
			InstanceTenancy: ec2types.TenancyDefault,
			CidrBlockAssociationSet: []ec2types.VpcCidrBlockAssociation{
				{
					AssociationId: aws.String("vpc-cidr-assoc-0aaa11111111111a"),
					CidrBlock:     aws.String("10.0.0.0/16"),
					CidrBlockState: &ec2types.VpcCidrBlockState{
						State: ec2types.VpcCidrBlockStateCodeAssociated,
					},
				},
			},
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("dev-vpc")},
			},
		},
		{
			VpcId:           aws.String("vpc-0ddd3333eee4444ff"),
			CidrBlock:       aws.String("172.31.0.0/16"),
			State:           ec2types.VpcStateAvailable,
			IsDefault:       aws.Bool(true),
			OwnerId:         aws.String("123456789012"),
			DhcpOptionsId:   aws.String("dopt-0aaa111111111111a"),
			InstanceTenancy: ec2types.TenancyDefault,
			CidrBlockAssociationSet: []ec2types.VpcCidrBlockAssociation{
				{
					AssociationId: aws.String("vpc-cidr-assoc-0bbb22222222222b"),
					CidrBlock:     aws.String("172.31.0.0/16"),
					CidrBlockState: &ec2types.VpcCidrBlockState{
						State: ec2types.VpcCidrBlockStateCodeAssociated,
					},
				},
			},
			Tags: []ec2types.Tag{},
		},
	}
}

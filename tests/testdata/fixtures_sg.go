package testdata

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// RealSecurityGroups returns sanitized Security Group data based on real AWS structure.
// Account: 123456789012 (sanitized)
// Region: us-east-1
// Total security groups: 21
func RealSecurityGroups() []ec2types.SecurityGroup {
	return []ec2types.SecurityGroup{
		{
			GroupId:          aws.String("sg-0aa0000000000001a"),
			GroupName:        aws.String("migration-sg"),
			Description:      aws.String("Security group for DocumentDB"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000001a"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("docdb-sg")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(27017),
					ToPort:     aws.Int32(27017),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000001a")},
					},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
						{CidrIp: aws.String("10.0.0.0/20"), Description: aws.String("Allow DocumentDB access from application subnets")},
						{CidrIp: aws.String("10.0.16.0/20"), Description: aws.String("Allow DocumentDB access from application subnets")},
						{CidrIp: aws.String("10.0.32.0/20"), Description: aws.String("Allow DocumentDB access from application subnets")},
						{CidrIp: aws.String("10.0.48.0/24"), Description: aws.String("Allow DocumentDB access from application subnets")},
						{CidrIp: aws.String("10.0.49.0/24"), Description: aws.String("Allow DocumentDB access from application subnets")},
						{CidrIp: aws.String("10.0.50.0/24"), Description: aws.String("Allow DocumentDB access from application subnets")},
						{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("Allow DocumentDB access from application subnets")},
						{CidrIp: aws.String("10.12.0.0/16"), Description: aws.String("Allow DocumentDB access from application subnets")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000001a")},
					},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("Allow all outbound traffic")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000002b"),
			GroupName:        aws.String("node-to-node-traffic"),
			Description:      aws.String("Security group for test-cluster-1"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000002b"),
			Tags: []ec2types.Tag{
				{Key: aws.String("name"), Value: aws.String("test-cluster-1")},
				{Key: aws.String("Name"), Value: aws.String("allow-node-to-node-traffic")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("All protocols")},
						{CidrIp: aws.String("172.20.0.0/16"), Description: aws.String("All protocols")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000003c"),
			GroupName:        aws.String("msk-sg"),
			Description:      aws.String("MSK Security Group Dev"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000003c"),
			Tags:             []ec2types.Tag{},
			IpPermissions:    []ec2types.IpPermission{},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000004d"),
			GroupName:        aws.String("app-efs"),
			Description:      aws.String("Allow NFS from EKS nodes to EFS"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000004d"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Environment"), Value: aws.String("dev")},
				{Key: aws.String("Name"), Value: aws.String("app-efs")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(2049),
					ToPort:     aws.Int32(2049),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("NFS from EKS nodes"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000005e")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000005e"),
			GroupName:        aws.String("test-cluster-1-node"),
			Description:      aws.String("EKS node shared security group"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000005e"),
			Tags: []ec2types.Tag{
				{Key: aws.String("kubernetes.io/cluster/test-cluster-1"), Value: aws.String("owned")},
				{Key: aws.String("karpenter.sh/discovery"), Value: aws.String("test-cluster-1")},
				{Key: aws.String("name"), Value: aws.String("test-cluster-1")},
				{Key: aws.String("Name"), Value: aws.String("test-cluster-1-node")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(6443),
					ToPort:     aws.Int32(6443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Cluster API to node 6443/tcp webhook"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000012c")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Node to node all ports/protocols"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000005e")},
					},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("Allow all traffic from internal k8s network")},
						{CidrIp: aws.String("172.20.0.0/16"), Description: aws.String("Allow all traffic from internal k8s network")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(80),
					ToPort:     aws.Int32(443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("elbv2.k8s.aws/targetGroupBinding=shared"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa000000000000d0")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(9443),
					ToPort:     aws.Int32(9443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Cluster API to node 9443/tcp webhook"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000012c")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(1025),
					ToPort:     aws.Int32(65535),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Node to node ingress on ephemeral ports"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000005e")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(8443),
					ToPort:     aws.Int32(8443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Cluster API to node 8443/tcp webhook"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000012c")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(10250),
					ToPort:     aws.Int32(10250),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Cluster API to node kubelets"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000012c")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("udp"),
					FromPort:   aws.Int32(53),
					ToPort:     aws.Int32(53),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Node to node CoreDNS UDP"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000005e")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(53),
					ToPort:     aws.Int32(53),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Node to node CoreDNS"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000005e")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(443),
					ToPort:     aws.Int32(443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Cluster API to node groups"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000012c")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(4443),
					ToPort:     aws.Int32(4443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Cluster API to node 4443/tcp webhook"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000012c")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("Allow all egress")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000006f"),
			GroupName:        aws.String("ci-runner-ubuntu-sg"),
			Description:      aws.String("Github Actions Runner security group"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000006f"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("ci-runner-ubuntu")},
				{Key: aws.String("ghr:ssm_config_path"), Value: aws.String("/ci-runners/default/config")},
				{Key: aws.String("ghr:environment"), Value: aws.String("ci-runner-ubuntu")},
				{Key: aws.String("OS"), Value: aws.String("Ubuntu")},
			},
			IpPermissions: []ec2types.IpPermission{},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges: []ec2types.Ipv6Range{
						{CidrIpv6: aws.String("::/0")},
					},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000007a"),
			GroupName:        aws.String("elasticache-dev-20250530145309259600000001"),
			Description:      aws.String("Security group for elasticache-dev"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000007a"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("elasticache-dev")},
				{Key: aws.String("name"), Value: aws.String("Elasticache/Redis SG")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(6379),
					ToPort:           aws.Int32(6379),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("10.0.0.0/20"), Description: aws.String("Redis")},
						{CidrIp: aws.String("10.0.16.0/20"), Description: aws.String("Redis")},
						{CidrIp: aws.String("10.0.32.0/20"), Description: aws.String("Redis")},
						{CidrIp: aws.String("10.0.48.0/24"), Description: aws.String("Redis")},
						{CidrIp: aws.String("10.0.49.0/24"), Description: aws.String("Redis")},
						{CidrIp: aws.String("10.0.50.0/24"), Description: aws.String("Redis")},
						{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("Redis")},
						{CidrIp: aws.String("10.12.0.0/16"), Description: aws.String("Redis")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000008b"),
			GroupName:        aws.String("allow-http-https-ssh"),
			Description:      aws.String("Managed by Terraform"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000008b"),
			Tags:             []ec2types.Tag{},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(80),
					ToPort:           aws.Int32(80),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(22),
					ToPort:           aws.Int32(22),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(11434),
					ToPort:           aws.Int32(11434),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("10.0.0.0/16")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(443),
					ToPort:           aws.Int32(443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000009c"),
			GroupName:        aws.String("eks-cluster-sg-test-cluster-1"),
			Description:      aws.String("EKS created security group applied to ENI that is attached to EKS Control Plane master nodes, as well as any managed workloads."),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000009c"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:eks:cluster-name"), Value: aws.String("test-cluster-1")},
				{Key: aws.String("Name"), Value: aws.String("eks-cluster-sg-test-cluster-1")},
				{Key: aws.String("name"), Value: aws.String("test-cluster-1")},
				{Key: aws.String("kubernetes.io/cluster/test-cluster-1"), Value: aws.String("owned")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000009c")},
					},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(443),
					ToPort:     aws.Int32(443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000010a")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa000000000000ad"),
			GroupName:        aws.String("vpc-endpoints"),
			Description:      aws.String("VPC endpoint security group"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa000000000000ad"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("vpc-endpoints")},
				{Key: aws.String("Endpoint"), Value: aws.String("true")},
				{Key: aws.String("Project"), Value: aws.String("Secret")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("Allow all outbound traffic")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(443),
					ToPort:           aws.Int32(443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("HTTPS from VPC")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{},
		},
		{
			GroupId:          aws.String("sg-0aa000000000000be"),
			GroupName:        aws.String("media-efs"),
			Description:      aws.String("Allow NFS from EKS nodes to EFS"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa000000000000be"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("media-efs")},
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(2049),
					ToPort:     aws.Int32(2049),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("NFS from EKS nodes"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000005e")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa000000000000cf"),
			GroupName:        aws.String("k8s-ingress-external"),
			Description:      aws.String("[k8s] Managed SecurityGroup for LoadBalancer"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa000000000000cf"),
			Tags: []ec2types.Tag{
				{Key: aws.String("service.k8s.aws/resource"), Value: aws.String("ManagedLBSecurityGroup")},
				{Key: aws.String("service.k8s.aws/stack"), Value: aws.String("ingress/external-ingress-nginx-controller")},
				{Key: aws.String("elbv2.k8s.aws/cluster"), Value: aws.String("test-cluster-1")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(80),
					ToPort:           aws.Int32(80),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(443),
					ToPort:           aws.Int32(443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa000000000000d0"),
			GroupName:        aws.String("k8s-traffic-shared"),
			Description:      aws.String("[k8s] Shared Backend SecurityGroup for LoadBalancer"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa000000000000d0"),
			Tags: []ec2types.Tag{
				{Key: aws.String("elbv2.k8s.aws/resource"), Value: aws.String("backend-sg")},
				{Key: aws.String("elbv2.k8s.aws/cluster"), Value: aws.String("test-cluster-1")},
			},
			IpPermissions: []ec2types.IpPermission{},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa000000000000e1"),
			GroupName:        aws.String("k8s-ingress-internal"),
			Description:      aws.String("[k8s] Managed SecurityGroup for LoadBalancer"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa000000000000e1"),
			Tags: []ec2types.Tag{
				{Key: aws.String("service.k8s.aws/stack"), Value: aws.String("ingress/internal-ingress-nginx-controller")},
				{Key: aws.String("elbv2.k8s.aws/cluster"), Value: aws.String("test-cluster-1")},
				{Key: aws.String("service.k8s.aws/resource"), Value: aws.String("ManagedLBSecurityGroup")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(80),
					ToPort:           aws.Int32(80),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(443),
					ToPort:           aws.Int32(443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa000000000000f2"),
			GroupName:        aws.String("vpn-sg"),
			Description:      aws.String("Managed by Terraform"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa000000000000f2"),
			Tags:             []ec2types.Tag{},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(22),
					ToPort:           aws.Int32(22),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol:       aws.String("udp"),
					FromPort:         aws.Int32(49959),
					ToPort:           aws.Int32(49959),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(443),
					ToPort:           aws.Int32(443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
				{
					IpProtocol:       aws.String("udp"),
					FromPort:         aws.Int32(59412),
					ToPort:           aws.Int32(59412),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000010a"),
			GroupName:        aws.String("ci-runner-sg"),
			Description:      aws.String("Github Actions Runner security group"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000010a"),
			Tags: []ec2types.Tag{
				{Key: aws.String("ghr:environment"), Value: aws.String("ci-runner")},
				{Key: aws.String("Name"), Value: aws.String("ci-runner")},
				{Key: aws.String("ghr:ssm_config_path"), Value: aws.String("/ci-runners/default/config")},
			},
			IpPermissions:       []ec2types.IpPermission{},
			IpPermissionsEgress: []ec2types.IpPermission{},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000011b"),
			GroupName:        aws.String("default"),
			Description:      aws.String("default VPC security group"),
			VpcId:            aws.String("vpc-0ddd3333eee4444ff"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000011b"),
			Tags:             []ec2types.Tag{},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000011b")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000012c"),
			GroupName:        aws.String("test-cluster-1-cluster"),
			Description:      aws.String("EKS cluster security group"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000012c"),
			Tags: []ec2types.Tag{
				{Key: aws.String("name"), Value: aws.String("test-cluster-1")},
				{Key: aws.String("Name"), Value: aws.String("test-cluster-1-cluster")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol: aws.String("tcp"),
					FromPort:   aws.Int32(443),
					ToPort:     aws.Int32(443),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{
						{Description: aws.String("Node groups to cluster API"), UserId: aws.String("123456789012"), GroupId: aws.String("sg-0aa0000000000005e")},
					},
					IpRanges:      []ec2types.IpRange{},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000013d"),
			GroupName:        aws.String("launch-wizard-1"),
			Description:      aws.String("launch-wizard-1 created 2025-03-17T12:46:39.310Z"),
			VpcId:            aws.String("vpc-0ddd3333eee4444ff"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000013d"),
			Tags:             []ec2types.Tag{},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(22),
					ToPort:           aws.Int32(22),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("-1"),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("0.0.0.0/0")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000014e"),
			GroupName:        aws.String("default"),
			Description:      aws.String("default VPC security group"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000014e"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("dev-vpc")},
			},
			IpPermissions:       []ec2types.IpPermission{},
			IpPermissionsEgress: []ec2types.IpPermission{},
		},
		{
			GroupId:          aws.String("sg-0aa0000000000015f"),
			GroupName:        aws.String("rds-dev-20250530163123214600000001"),
			Description:      aws.String("Security group for rds-dev"),
			VpcId:            aws.String("vpc-0aaa1111bbb2222cc"),
			OwnerId:          aws.String("123456789012"),
			SecurityGroupArn: aws.String("arn:aws:ec2:us-east-1:123456789012:security-group/sg-0aa0000000000015f"),
			Tags: []ec2types.Tag{
				{Key: aws.String("name"), Value: aws.String("RDS Security group")},
				{Key: aws.String("Name"), Value: aws.String("rds-dev")},
			},
			IpPermissions: []ec2types.IpPermission{
				{
					IpProtocol:       aws.String("tcp"),
					FromPort:         aws.Int32(5432),
					ToPort:           aws.Int32(5432),
					UserIdGroupPairs: []ec2types.UserIdGroupPair{},
					IpRanges: []ec2types.IpRange{
						{CidrIp: aws.String("10.0.0.0/20"), Description: aws.String("PostgreSQL")},
						{CidrIp: aws.String("10.0.16.0/20"), Description: aws.String("PostgreSQL")},
						{CidrIp: aws.String("10.0.32.0/20"), Description: aws.String("PostgreSQL")},
						{CidrIp: aws.String("10.0.48.0/24"), Description: aws.String("PostgreSQL")},
						{CidrIp: aws.String("10.0.49.0/24"), Description: aws.String("PostgreSQL")},
						{CidrIp: aws.String("10.0.50.0/24"), Description: aws.String("PostgreSQL")},
						{CidrIp: aws.String("10.0.0.0/16"), Description: aws.String("PostgreSQL")},
						{CidrIp: aws.String("10.12.0.0/16"), Description: aws.String("PostgreSQL")},
					},
					Ipv6Ranges:    []ec2types.Ipv6Range{},
					PrefixListIds: []ec2types.PrefixListId{},
				},
			},
			IpPermissionsEgress: []ec2types.IpPermission{},
		},
	}
}

package unit

// qa_navigable_fields_contract_test.go — per-resource-type navigable-field
// contract.
//
// This file is NOT a PR #273 regression pin. It surfaced accidentally
// during PR #273 review as a fundamental gap in the detail view's
// navigable-field coverage: RegisterNavigableFields entries had drifted
// away from the AWS API cross-references that should drive them.
//
// The detail view (DetailModel) underlines fields that are registered via
// resource.RegisterNavigableFields so the user can press Enter and jump to
// a filtered list of the target resource type. Any cross-reference that
// AWS exposes on the list/describe response and that a9s already knows how
// to browse as its own resource type MUST be navigable — otherwise the
// user sees the value printed but has no way to drill into it.
//
// The user's screenshot for a VPC Endpoint detail view showed VpcId
// rendered as navigable (good) but SubnetIds, NetworkInterfaceIds, and
// Groups[].GroupId printed as plain sub-list items with no navigable
// affordance — all three fields are registered AWS cross-references to
// other a9s-browseable types. That shape of gap exists across multiple
// resource types today.
//
// Contract: navigableContracts lists, for every registered shortName, the
// AWS API response field paths that MUST be navigable and the target
// resource type they point at. Rows are anchored to the AWS API reference
// so the contract is driven by AWS, not by the current a9s implementation.
//
// Tests:
//
//   (A) TestNavigableFields_AllExpectedFieldsRegistered — every
//       navigableContracts row's FieldPath must appear in
//       resource.GetNavigableFields(shortName) with the same TargetType.
//       Failures reveal missing RegisterNavigableFields entries.
//
//   (B) TestNavigableFields_TargetTypesAreRegistered — every navigable
//       TargetType must be a registered resource type. Otherwise pressing
//       Enter on the underlined value goes nowhere.
//
//   (C) TestNavigableFields_NoOrphans — every currently-registered
//       NavigableField must have a corresponding row in
//       navigableContracts. Adding an entry in source code requires
//       adding/updating a contract row with the AWS API rationale.

import (
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

type navContract struct {
	shortName  string
	apiDoc     string
	fieldPath  string
	targetType string
	reasoning  string
}

// navigableContracts is the single source of truth for "this field on this
// resource type's AWS API response must be navigable." Rows derive from
// the AWS API reference documentation, NOT from current a9s registrations.
// A row may describe a field that is not yet registered — that is the
// whole point (test A will fail until the registration is added).
//
// Rows sorted alphabetically by (shortName, fieldPath).
var navigableContracts = []navContract{
	// ami — AMIs
	{shortName: "ami", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeImages.html", fieldPath: "BlockDeviceMappings.Ebs.SnapshotId", targetType: "ebs-snap", reasoning: "Image.BlockDeviceMappings[].Ebs.SnapshotId references EBS snapshots backing the AMI."},

	// asg — Auto Scaling Groups
	{shortName: "asg", apiDoc: "https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html", fieldPath: "TargetGroupARNs", targetType: "tg", reasoning: "AutoScalingGroup.TargetGroupARNs is the list of ALB/NLB target groups the ASG registers instances with."},
	{shortName: "asg", apiDoc: "https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html", fieldPath: "VPCZoneIdentifier", targetType: "subnet", reasoning: "AutoScalingGroup.VPCZoneIdentifier is a comma-separated list of subnet IDs the ASG launches into."},

	// cb — CodeBuild Projects
	{shortName: "cb", apiDoc: "https://docs.aws.amazon.com/codebuild/latest/APIReference/API_Project.html", fieldPath: "ServiceRole", targetType: "role", reasoning: "Project.ServiceRole is the IAM role ARN CodeBuild assumes during builds."},
	{shortName: "cb", apiDoc: "https://docs.aws.amazon.com/codebuild/latest/APIReference/API_ProjectArtifacts.html", fieldPath: "EncryptionKey", targetType: "kms", reasoning: "Project.EncryptionKey is the KMS key used to encrypt build output artifacts."},
	{shortName: "cb", apiDoc: "https://docs.aws.amazon.com/codebuild/latest/APIReference/API_VpcConfig.html", fieldPath: "VpcConfig.VpcId", targetType: "vpc", reasoning: "Project.VpcConfig.VpcId — if a project runs in a VPC it references a VPC."},
	{shortName: "cb", apiDoc: "https://docs.aws.amazon.com/codebuild/latest/APIReference/API_VpcConfig.html", fieldPath: "VpcConfig.Subnets", targetType: "subnet", reasoning: "Project.VpcConfig.Subnets — VPC subnets the build container runs in."},
	{shortName: "cb", apiDoc: "https://docs.aws.amazon.com/codebuild/latest/APIReference/API_VpcConfig.html", fieldPath: "VpcConfig.SecurityGroupIds", targetType: "sg", reasoning: "Project.VpcConfig.SecurityGroupIds — security groups attached to the build container."},

	// cfn — CloudFormation
	{shortName: "cfn", apiDoc: "https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html", fieldPath: "RoleARN", targetType: "role", reasoning: "Stack.RoleARN is the service role CloudFormation uses to create/update stack resources."},
	{shortName: "cfn", apiDoc: "https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html", fieldPath: "NotificationARNs", targetType: "sns", reasoning: "Stack.NotificationARNs is a list of SNS topics CloudFormation publishes stack events to."},

	// ct-events — CloudTrail Events
	{shortName: "ct-events", apiDoc: "https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html", fieldPath: "user", targetType: "iam-user", reasoning: "CloudTrail Event user identity (Type=IAMUser) links to IAM Users."},
	{shortName: "ct-events", apiDoc: "https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html", fieldPath: "role_name", targetType: "role", reasoning: "CloudTrail Event user identity (Type=AssumedRole) session carries the role name."},

	// dbc — DocumentDB Clusters. docdb_types.DBCluster.DBSubnetGroup is *string
	// (just the subnet-group name), not a struct — VPC/Subnet navigation is
	// surfaced via the related-panel checkers (checkDbcVPC, checkDbcSubnet),
	// not navigable fields.
	{shortName: "dbc", apiDoc: "https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBCluster.html", fieldPath: "VpcSecurityGroups.VpcSecurityGroupId", targetType: "sg", reasoning: "DBCluster.VpcSecurityGroups[].VpcSecurityGroupId references SGs attached to the cluster."},
	{shortName: "dbc", apiDoc: "https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBCluster.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "DBCluster.KmsKeyId is the KMS key used for storage encryption."},

	// dbi — RDS DB Instances
	{shortName: "dbi", apiDoc: "https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html", fieldPath: "VpcSecurityGroups.VpcSecurityGroupId", targetType: "sg", reasoning: "DBInstance.VpcSecurityGroups[].VpcSecurityGroupId — SGs attached to the instance."},
	{shortName: "dbi", apiDoc: "https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html", fieldPath: "DBSubnetGroup.VpcId", targetType: "vpc", reasoning: "DBInstance subnet group VpcId — VPC of the instance."},
	{shortName: "dbi", apiDoc: "https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSubnetGroup.html", fieldPath: "DBSubnetGroup.Subnets.SubnetIdentifier", targetType: "subnet", reasoning: "DBSubnetGroup.Subnets[].SubnetIdentifier — subnets the instance spans."},
	{shortName: "dbi", apiDoc: "https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "DBInstance.KmsKeyId is the KMS key used for storage encryption."},

	// ddb — DynamoDB Tables
	{shortName: "ddb", apiDoc: "https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_SSEDescription.html", fieldPath: "SSEDescription.KMSMasterKeyArn", targetType: "kms", reasoning: "TableDescription.SSEDescription.KMSMasterKeyArn is the KMS key used for server-side encryption."},

	// dbc-snap — DocumentDB Snapshots
	{shortName: "dbc-snap", apiDoc: "https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBClusterSnapshot.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "DBClusterSnapshot.VpcId — the VPC the source cluster was in."},
	{shortName: "dbc-snap", apiDoc: "https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBClusterSnapshot.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "DBClusterSnapshot.KmsKeyId — the KMS key used to encrypt the snapshot."},

	// eb-rule — EventBridge Rules
	{shortName: "eb-rule", apiDoc: "https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_Rule.html", fieldPath: "RoleArn", targetType: "role", reasoning: "Rule.RoleArn is the IAM role EventBridge assumes when invoking rule targets."},

	// ebs — EBS Volumes
	{shortName: "ebs", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html", fieldPath: "Attachments.InstanceId", targetType: "ec2", reasoning: "Volume.Attachments[].InstanceId — the EC2 instance the volume is attached to."},
	{shortName: "ebs", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "Volume.KmsKeyId is the KMS key for volume encryption."},

	// ebs-snap — EBS Snapshots
	{shortName: "ebs-snap", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html", fieldPath: "VolumeId", targetType: "ebs", reasoning: "Snapshot.VolumeId references the source EBS volume."},
	{shortName: "ebs-snap", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "Snapshot.KmsKeyId — the KMS key the snapshot is encrypted with."},

	// ec2 — EC2 Instances
	{shortName: "ec2", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "Instance.VpcId — the VPC the instance is in."},
	{shortName: "ec2", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html", fieldPath: "SubnetId", targetType: "subnet", reasoning: "Instance.SubnetId — the subnet the instance's primary ENI is in."},
	{shortName: "ec2", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html", fieldPath: "ImageId", targetType: "ami", reasoning: "Instance.ImageId — the AMI the instance was launched from."},
	{shortName: "ec2", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceBlockDeviceMapping.html", fieldPath: "BlockDeviceMappings.Ebs.VolumeId", targetType: "ebs", reasoning: "Instance.BlockDeviceMappings[].Ebs.VolumeId — attached EBS volumes."},
	{shortName: "ec2", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_GroupIdentifier.html", fieldPath: "SecurityGroups.GroupId", targetType: "sg", reasoning: "Instance.SecurityGroups[].GroupId — attached SGs."},
	{shortName: "ec2", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceNetworkInterface.html", fieldPath: "NetworkInterfaces.NetworkInterfaceId", targetType: "eni", reasoning: "Instance.NetworkInterfaces[].NetworkInterfaceId — ENIs attached to the instance."},
	{shortName: "ec2", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_IamInstanceProfile.html", fieldPath: "IamInstanceProfile.Arn", targetType: "role", reasoning: "Instance.IamInstanceProfile.Arn — the IAM instance profile (maps to a role)."},

	// ecr — ECR Repositories
	{shortName: "ecr", apiDoc: "https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_EncryptionConfiguration.html", fieldPath: "EncryptionConfiguration.KmsKey", targetType: "kms", reasoning: "Repository.EncryptionConfiguration.KmsKey — KMS key for image encryption when EncryptionType=KMS."},

	// ecs — ECS Clusters
	{shortName: "ecs", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ExecuteCommandConfiguration.html", fieldPath: "Configuration.ExecuteCommandConfiguration.KmsKeyId", targetType: "kms", reasoning: "Cluster.Configuration.ExecuteCommandConfiguration.KmsKeyId — KMS key for ECS Exec session encryption."},

	// ecs-svc — ECS Services
	{shortName: "ecs-svc", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html", fieldPath: "ClusterArn", targetType: "ecs", reasoning: "Service.ClusterArn — the ECS cluster this service runs on."},
	{shortName: "ecs-svc", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html", fieldPath: "RoleArn", targetType: "role", reasoning: "Service.RoleArn — legacy service-linked IAM role (present when not using awsvpc)."},
	{shortName: "ecs-svc", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_AwsVpcConfiguration.html", fieldPath: "NetworkConfiguration.AwsvpcConfiguration.Subnets", targetType: "subnet", reasoning: "Service.NetworkConfiguration.AwsvpcConfiguration.Subnets — subnets tasks are placed in."},
	{shortName: "ecs-svc", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_AwsVpcConfiguration.html", fieldPath: "NetworkConfiguration.AwsvpcConfiguration.SecurityGroups", targetType: "sg", reasoning: "Service.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups — SGs attached to tasks."},
	{shortName: "ecs-svc", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_LoadBalancer.html", fieldPath: "LoadBalancers.TargetGroupArn", targetType: "tg", reasoning: "Service.LoadBalancers[].TargetGroupArn — target groups that receive traffic for this service."},

	// ecs-task — ECS Tasks
	{shortName: "ecs-task", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Task.html", fieldPath: "ClusterArn", targetType: "ecs", reasoning: "Task.ClusterArn — cluster the task runs on."},

	// efs — EFS File Systems
	{shortName: "efs", apiDoc: "https://docs.aws.amazon.com/efs/latest/ug/API_FileSystemDescription.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "FileSystemDescription.KmsKeyId — KMS key for at-rest encryption."},

	// eip — Elastic IPs
	{shortName: "eip", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html", fieldPath: "InstanceId", targetType: "ec2", reasoning: "Address.InstanceId — the EC2 instance the EIP is associated with."},
	{shortName: "eip", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html", fieldPath: "NetworkInterfaceId", targetType: "eni", reasoning: "Address.NetworkInterfaceId — the ENI the EIP is associated with."},

	// eks — EKS Clusters
	{shortName: "eks", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_VpcConfigResponse.html", fieldPath: "ResourcesVpcConfig.VpcId", targetType: "vpc", reasoning: "Cluster.ResourcesVpcConfig.VpcId — the VPC cluster resources live in."},
	{shortName: "eks", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_VpcConfigResponse.html", fieldPath: "ResourcesVpcConfig.ClusterSecurityGroupId", targetType: "sg", reasoning: "Cluster.ResourcesVpcConfig.ClusterSecurityGroupId — SG EKS creates for control-plane↔node comms."},
	{shortName: "eks", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_VpcConfigResponse.html", fieldPath: "ResourcesVpcConfig.SubnetIds", targetType: "subnet", reasoning: "Cluster.ResourcesVpcConfig.SubnetIds — subnets the cluster API endpoints use."},
	{shortName: "eks", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_VpcConfigResponse.html", fieldPath: "ResourcesVpcConfig.SecurityGroupIds", targetType: "sg", reasoning: "Cluster.ResourcesVpcConfig.SecurityGroupIds — additional SGs attached to the control-plane ENIs."},
	{shortName: "eks", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html", fieldPath: "RoleArn", targetType: "role", reasoning: "Cluster.RoleArn — the IAM role EKS assumes for cluster operations."},

	// elb — Load Balancers (ELBv2)
	{shortName: "elb", apiDoc: "https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancer.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "LoadBalancer.VpcId — the VPC the load balancer is in."},
	{shortName: "elb", apiDoc: "https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancer.html", fieldPath: "SecurityGroups", targetType: "sg", reasoning: "LoadBalancer.SecurityGroups — SGs attached to the ALB (not present on NLBs)."},
	{shortName: "elb", apiDoc: "https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_AvailabilityZone.html", fieldPath: "AvailabilityZones.SubnetId", targetType: "subnet", reasoning: "LoadBalancer.AvailabilityZones[].SubnetId — subnets the ELB exposes listeners in."},

	// eni — Network Interfaces
	{shortName: "eni", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterface.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "NetworkInterface.VpcId — the VPC the ENI is in."},
	{shortName: "eni", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterface.html", fieldPath: "SubnetId", targetType: "subnet", reasoning: "NetworkInterface.SubnetId — the subnet the ENI is in."},
	{shortName: "eni", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_GroupIdentifier.html", fieldPath: "Groups.GroupId", targetType: "sg", reasoning: "NetworkInterface.Groups[].GroupId — attached SGs."},
	{shortName: "eni", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterfaceAttachment.html", fieldPath: "Attachment.InstanceId", targetType: "ec2", reasoning: "NetworkInterface.Attachment.InstanceId — instance the ENI is attached to (when attached)."},
	{shortName: "eni", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterfaceAssociation.html", fieldPath: "Association.AllocationId", targetType: "eip", reasoning: "NetworkInterface.Association.AllocationId — EIP allocation associated with the ENI."},

	// glue — Glue Jobs
	{shortName: "glue", apiDoc: "https://docs.aws.amazon.com/glue/latest/webapi/API_Job.html", fieldPath: "Role", targetType: "role", reasoning: "Job.Role — IAM role Glue assumes to run the job."},

	// igw — Internet Gateways
	{shortName: "igw", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InternetGatewayAttachment.html", fieldPath: "Attachments.VpcId", targetType: "vpc", reasoning: "InternetGateway.Attachments[].VpcId — VPC the IGW is attached to."},

	// lambda — Lambda Functions
	{shortName: "lambda", apiDoc: "https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html", fieldPath: "Role", targetType: "role", reasoning: "FunctionConfiguration.Role — IAM role the function executes as."},
	{shortName: "lambda", apiDoc: "https://docs.aws.amazon.com/lambda/latest/api/API_VpcConfigResponse.html", fieldPath: "VpcConfig.VpcId", targetType: "vpc", reasoning: "FunctionConfiguration.VpcConfig.VpcId — VPC the function runs in, if configured."},
	{shortName: "lambda", apiDoc: "https://docs.aws.amazon.com/lambda/latest/api/API_VpcConfigResponse.html", fieldPath: "VpcConfig.SubnetIds", targetType: "subnet", reasoning: "FunctionConfiguration.VpcConfig.SubnetIds — subnets the function's ENIs live in."},
	{shortName: "lambda", apiDoc: "https://docs.aws.amazon.com/lambda/latest/api/API_VpcConfigResponse.html", fieldPath: "VpcConfig.SecurityGroupIds", targetType: "sg", reasoning: "FunctionConfiguration.VpcConfig.SecurityGroupIds — SGs attached to the function's ENIs."},
	{shortName: "lambda", apiDoc: "https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html", fieldPath: "KMSKeyArn", targetType: "kms", reasoning: "FunctionConfiguration.KMSKeyArn — KMS key used to encrypt env vars."},

	// logs — CloudWatch Log Groups
	{shortName: "logs", apiDoc: "https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_LogGroup.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "LogGroup.KmsKeyId — KMS key for log data encryption."},

	// msk — MSK Clusters
	{shortName: "msk", apiDoc: "https://docs.aws.amazon.com/msk/1.0/apireference/v1-clusters.html", fieldPath: "Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId", targetType: "kms", reasoning: "Cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId — KMS key for broker data-volume encryption."},

	// nat — NAT Gateways
	{shortName: "nat", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGateway.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "NatGateway.VpcId — VPC the NAT is in."},
	{shortName: "nat", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGateway.html", fieldPath: "SubnetId", targetType: "subnet", reasoning: "NatGateway.SubnetId — subnet the NAT lives in (must be public)."},
	{shortName: "nat", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGatewayAddress.html", fieldPath: "NatGatewayAddresses.AllocationId", targetType: "eip", reasoning: "NatGateway.NatGatewayAddresses[].AllocationId — EIPs allocated to the NAT."},

	// ng — EKS Node Groups
	{shortName: "ng", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html", fieldPath: "ClusterName", targetType: "eks", reasoning: "Nodegroup.ClusterName — parent EKS cluster."},
	{shortName: "ng", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html", fieldPath: "NodeRole", targetType: "role", reasoning: "Nodegroup.NodeRole — IAM role nodes assume to join the cluster."},
	{shortName: "ng", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html", fieldPath: "Subnets", targetType: "subnet", reasoning: "Nodegroup.Subnets — subnets worker nodes are placed in."},

	// opensearch — OpenSearch Domains
	{shortName: "opensearch", apiDoc: "https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_EncryptionAtRestOptions.html", fieldPath: "EncryptionAtRestOptions.KmsKeyId", targetType: "kms", reasoning: "DomainStatus.EncryptionAtRestOptions.KmsKeyId — KMS key for at-rest encryption."},
	{shortName: "opensearch", apiDoc: "https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_VPCDerivedInfo.html", fieldPath: "VPCOptions.VPCId", targetType: "vpc", reasoning: "DomainStatus.VPCOptions.VPCId — VPC the domain is attached to."},
	{shortName: "opensearch", apiDoc: "https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_VPCDerivedInfo.html", fieldPath: "VPCOptions.SubnetIds", targetType: "subnet", reasoning: "DomainStatus.VPCOptions.SubnetIds — subnets the domain's ENIs live in."},
	{shortName: "opensearch", apiDoc: "https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_VPCDerivedInfo.html", fieldPath: "VPCOptions.SecurityGroupIds", targetType: "sg", reasoning: "DomainStatus.VPCOptions.SecurityGroupIds — SGs attached to the domain ENIs."},

	// redis — ElastiCache Redis. Security Groups live on MemberCluster objects
	// (DescribeCacheClusters), not on the ReplicationGroup RawStruct — SG
	// navigation is surfaced via the checkRedisSG related-panel checker.
	{shortName: "redis", apiDoc: "https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ReplicationGroup.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "ReplicationGroup.KmsKeyId — KMS key for at-rest encryption."},

	// dbi-snap — DB Instance Snapshots
	{shortName: "dbi-snap", apiDoc: "https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html", fieldPath: "DBInstanceIdentifier", targetType: "dbi", reasoning: "DBSnapshot.DBInstanceIdentifier — source DB instance."},
	{shortName: "dbi-snap", apiDoc: "https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "DBSnapshot.KmsKeyId — KMS key the snapshot is encrypted with."},
	// DBSnapshot has no VpcId field per the AWS SDK — vpc pivot is reachable via the dbi cross-ref, not a direct nav field.

	// rtb — Route Tables
	{shortName: "rtb", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RouteTable.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "RouteTable.VpcId — VPC the route table belongs to."},
	{shortName: "rtb", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RouteTableAssociation.html", fieldPath: "Associations.SubnetId", targetType: "subnet", reasoning: "RouteTable.Associations[].SubnetId — explicit subnet associations."},
	{shortName: "rtb", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Route.html", fieldPath: "Routes.NatGatewayId", targetType: "nat", reasoning: "RouteTable.Routes[].NatGatewayId — NAT gateway target for default/out routes."},
	{shortName: "rtb", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Route.html", fieldPath: "Routes.GatewayId", targetType: "igw", reasoning: "RouteTable.Routes[].GatewayId — IGW target."},
	{shortName: "rtb", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Route.html", fieldPath: "Routes.NetworkInterfaceId", targetType: "eni", reasoning: "RouteTable.Routes[].NetworkInterfaceId — ENI target (e.g. a firewall instance)."},
	{shortName: "rtb", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Route.html", fieldPath: "Routes.TransitGatewayId", targetType: "tgw", reasoning: "RouteTable.Routes[].TransitGatewayId — TGW target."},
	{shortName: "rtb", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Route.html", fieldPath: "Routes.VpcPeeringConnectionId", targetType: "vpc", reasoning: "RouteTable.Routes[].VpcPeeringConnectionId — peer VPC target (navigates to vpc)."},

	// secrets — Secrets Manager
	{shortName: "secrets", apiDoc: "https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_SecretListEntry.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "SecretListEntry.KmsKeyId — KMS key used to encrypt the secret."},
	{shortName: "secrets", apiDoc: "https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_SecretListEntry.html", fieldPath: "RotationLambdaARN", targetType: "lambda", reasoning: "SecretListEntry.RotationLambdaARN — Lambda that rotates the secret."},

	// sg — Security Groups
	{shortName: "sg", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_SecurityGroup.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "SecurityGroup.VpcId — VPC the SG belongs to."},

	// sns-sub — SNS Subscriptions
	{shortName: "sns-sub", apiDoc: "https://docs.aws.amazon.com/sns/latest/api/API_Subscription.html", fieldPath: "TopicArn", targetType: "sns", reasoning: "Subscription.TopicArn — parent SNS topic."},

	// ssm — SSM Parameters
	{shortName: "ssm", apiDoc: "https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html", fieldPath: "KeyId", targetType: "kms", reasoning: "ParameterMetadata.KeyId — KMS key for SecureString parameters."},

	// subnet — Subnets
	{shortName: "subnet", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Subnet.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "Subnet.VpcId — parent VPC."},

	// tg — Target Groups
	{shortName: "tg", apiDoc: "https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_TargetGroup.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "TargetGroup.VpcId — VPC the TG's targets live in."},
	{shortName: "tg", apiDoc: "https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_TargetGroup.html", fieldPath: "LoadBalancerArns", targetType: "elb", reasoning: "TargetGroup.LoadBalancerArns — ALBs/NLBs attached to this TG."},

	// trail — CloudTrail Trails
	{shortName: "trail", apiDoc: "https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html", fieldPath: "S3BucketName", targetType: "s3", reasoning: "Trail.S3BucketName — S3 bucket trail logs are written to."},
	{shortName: "trail", apiDoc: "https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html", fieldPath: "KmsKeyId", targetType: "kms", reasoning: "Trail.KmsKeyId — KMS key for log encryption."},
	{shortName: "trail", apiDoc: "https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html", fieldPath: "SnsTopicARN", targetType: "sns", reasoning: "Trail.SnsTopicARN — SNS topic for log-file-delivered notifications."},
	{shortName: "trail", apiDoc: "https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html", fieldPath: "CloudWatchLogsLogGroupArn", targetType: "logs", reasoning: "Trail.CloudWatchLogsLogGroupArn — log group the trail streams to."},
	{shortName: "trail", apiDoc: "https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html", fieldPath: "CloudWatchLogsRoleArn", targetType: "role", reasoning: "Trail.CloudWatchLogsRoleArn — IAM role trail assumes to write to CloudWatch Logs."},

	// vpce — VPC Endpoints  (the user's screenshot bug class)
	{shortName: "vpce", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "VpcEndpoint.VpcId — VPC the endpoint lives in."},
	{shortName: "vpce", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html", fieldPath: "SubnetIds", targetType: "subnet", reasoning: "VpcEndpoint.SubnetIds — subnets the interface endpoint's ENIs are placed in. Shown in the user's screenshot but not registered — MISSING."},
	{shortName: "vpce", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html", fieldPath: "NetworkInterfaceIds", targetType: "eni", reasoning: "VpcEndpoint.NetworkInterfaceIds — ENIs backing the endpoint. Shown in screenshot but not registered — MISSING."},
	{shortName: "vpce", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_SecurityGroupIdentifier.html", fieldPath: "Groups.GroupId", targetType: "sg", reasoning: "VpcEndpoint.Groups[].GroupId — SGs attached to the interface endpoint. Shown in screenshot but not registered — MISSING."},
	{shortName: "vpce", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html", fieldPath: "RouteTableIds", targetType: "rtb", reasoning: "VpcEndpoint.RouteTableIds — route tables the gateway endpoint is associated with."},

	// tgw — Transit Gateways
	{shortName: "tgw", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TransitGateway.html", fieldPath: "", targetType: "", reasoning: "TransitGateway has no built-in cross-resource ID fields beyond Options (ASNs, CIDRs). No navigable fields required today; attachments are a separate API."},

	// Types with no cross-reference on the list/describe response — no navigable fields.
	// Declaring as empty-row sentinels so test C (orphan detection) accepts the absence.
	{shortName: "acm", apiDoc: "https://docs.aws.amazon.com/acm/latest/APIReference/API_CertificateSummary.html", fieldPath: "", targetType: "", reasoning: "ListCertificates summary has no cross-resource IDs; DomainValidationOptions refers to Route53 but a9s doesn't jump on that today."},
	{shortName: "alarm", apiDoc: "https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricAlarm.html", fieldPath: "", targetType: "", reasoning: "MetricAlarm.Dimensions may reference other resources but the dimension names/values are arbitrary; no guaranteed target-type mapping."},
	{shortName: "apigw", apiDoc: "https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis.html", fieldPath: "", targetType: "", reasoning: "GetApis response has no cross-resource IDs a9s browses today."},
	{shortName: "athena", apiDoc: "https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroupSummary.html", fieldPath: "", targetType: "", reasoning: "WorkGroup list has no cross-resource IDs."},
	{shortName: "backup", apiDoc: "https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupPlansListMember.html", fieldPath: "", targetType: "", reasoning: "BackupPlansListMember has no IDs that point at other a9s types."},
	{shortName: "cf", apiDoc: "https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_DistributionSummary.html", fieldPath: "", targetType: "", reasoning: "DistributionSummary.AliasICPRecordals and Origins[].DomainName don't map cleanly to any a9s type."},
	{shortName: "codeartifact", apiDoc: "https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_RepositorySummary.html", fieldPath: "", targetType: "", reasoning: "Repo summary has no cross-resource IDs."},
	{shortName: "eb", apiDoc: "https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html", fieldPath: "", targetType: "", reasoning: "Beanstalk environment refers to application name (string) and config templates, not IDs a9s currently browses."},
	{shortName: "iam-group", apiDoc: "https://docs.aws.amazon.com/IAM/latest/APIReference/API_Group.html", fieldPath: "", targetType: "", reasoning: "Group list entry has no cross-resource IDs."},
	{shortName: "iam-user", apiDoc: "https://docs.aws.amazon.com/IAM/latest/APIReference/API_User.html", fieldPath: "", targetType: "", reasoning: "User list entry has no cross-resource IDs."},
	{shortName: "kinesis", apiDoc: "https://docs.aws.amazon.com/kinesis/latest/APIReference/API_StreamDescriptionSummary.html", fieldPath: "", targetType: "", reasoning: "StreamDescriptionSummary has no cross-resource IDs."},
	{shortName: "kms", apiDoc: "https://docs.aws.amazon.com/kms/latest/APIReference/API_KeyListEntry.html", fieldPath: "", targetType: "", reasoning: "KMS key listing has no cross-resource IDs."},
	{shortName: "pipeline", apiDoc: "https://docs.aws.amazon.com/codepipeline/latest/APIReference/API_PipelineSummary.html", fieldPath: "", targetType: "", reasoning: "Pipeline summary has no cross-resource IDs; stage-level actions do but those are fetched via GetPipelineState, out of list scope."},
	{shortName: "policy", apiDoc: "https://docs.aws.amazon.com/IAM/latest/APIReference/API_Policy.html", fieldPath: "", targetType: "", reasoning: "Policy list has no cross-resource IDs."},
	{shortName: "r53", apiDoc: "https://docs.aws.amazon.com/Route53/latest/APIReference/API_HostedZone.html", fieldPath: "", targetType: "", reasoning: "HostedZone summary has no cross-resource IDs."},
	{shortName: "redshift", apiDoc: "https://docs.aws.amazon.com/redshift/latest/APIReference/API_Cluster.html", fieldPath: "VpcId", targetType: "vpc", reasoning: "Cluster.VpcId — the VPC the cluster is in. Not registered today — MISSING."},
	{shortName: "role", apiDoc: "https://docs.aws.amazon.com/IAM/latest/APIReference/API_Role.html", fieldPath: "", targetType: "", reasoning: "Role list entry has no cross-resource IDs (inline/attached policies are separate APIs)."},
	{shortName: "s3", apiDoc: "https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListBuckets.html", fieldPath: "", targetType: "", reasoning: "ListBuckets response has only name + CreationDate."},
	{shortName: "ses", apiDoc: "https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_IdentityInfo.html", fieldPath: "", targetType: "", reasoning: "IdentityInfo has no cross-resource IDs."},
	{shortName: "sfn", apiDoc: "https://docs.aws.amazon.com/step-functions/latest/apireference/API_StateMachineListItem.html", fieldPath: "RoleArn", targetType: "role", reasoning: "StateMachineListItem has RoleArn when fetched via DescribeStateMachine. Not registered today — MISSING."},
	{shortName: "sns", apiDoc: "https://docs.aws.amazon.com/sns/latest/api/API_Topic.html", fieldPath: "", targetType: "", reasoning: "Topic summary has only TopicArn; attributes (KmsMasterKeyId) are fetched via GetTopicAttributes, out of list scope."},
	{shortName: "sqs", apiDoc: "https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html", fieldPath: "", targetType: "", reasoning: "Queue attributes are fetched separately; KmsMasterKeyId is one but a9s doesn't surface it on the list today."},
	{shortName: "vpc", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Vpc.html", fieldPath: "", targetType: "", reasoning: "VPC has no cross-resource IDs on DescribeVpcs (associations are separate)."},
	{shortName: "waf", apiDoc: "https://docs.aws.amazon.com/waf/latest/APIReference/API_WebACLSummary.html", fieldPath: "", targetType: "", reasoning: "WebACL summary has no cross-resource IDs."},
}

// buildContractIndex returns shortName -> set of {fieldPath: targetType} from navigableContracts.
// Sentinel rows with fieldPath=="" are included as an empty entry so test C can skip them.
func buildContractIndex() map[string]map[string]string {
	idx := make(map[string]map[string]string, len(navigableContracts))
	for _, c := range navigableContracts {
		if idx[c.shortName] == nil {
			idx[c.shortName] = make(map[string]string)
		}
		if c.fieldPath != "" {
			idx[c.shortName][c.fieldPath] = c.targetType
		}
	}
	return idx
}

// TestNavigableFields_AllExpectedFieldsRegistered asserts every navigableContracts
// row with a non-empty fieldPath is backed by resource.RegisterNavigableFields
// for the same shortName with the same TargetType.
func TestNavigableFields_AllExpectedFieldsRegistered(t *testing.T) {
	idx := buildContractIndex()
	for shortName, expected := range idx {
		t.Run(shortName, func(t *testing.T) {
			registered := make(map[string]string, len(resource.GetNavigableFields(shortName)))
			for _, nf := range resource.GetNavigableFields(shortName) {
				registered[nf.FieldPath] = nf.TargetType
			}
			for path, want := range expected {
				got, ok := registered[path]
				if !ok {
					c := findContract(shortName, path)
					t.Errorf("%s.%s missing RegisterNavigableFields registration (expected TargetType=%q)\n  AWS API: %s\n  Reasoning: %s",
						shortName, path, want, c.apiDoc, c.reasoning)
					continue
				}
				if got != want {
					t.Errorf("%s.%s registered with TargetType=%q, want %q per AWS API schema", shortName, path, got, want)
				}
			}
		})
	}
}

// TestNavigableFields_TargetTypesAreRegistered asserts every navigable
// TargetType names a registered resource type. Otherwise Enter on the
// underlined field navigates to a nonexistent type.
func TestNavigableFields_TargetTypesAreRegistered(t *testing.T) {
	registered := make(map[string]bool, len(resource.AllResourceTypes()))
	for _, td := range resource.AllResourceTypes() {
		registered[td.ShortName] = true
	}
	for _, c := range navigableContracts {
		if c.targetType == "" {
			continue
		}
		if !registered[c.targetType] {
			t.Errorf("%s.%s declared TargetType=%q but no resource type with that shortName is registered",
				c.shortName, c.fieldPath, c.targetType)
		}
	}
}

// TestNavigableFields_NoOrphans asserts every currently-registered
// NavigableField has a corresponding row in navigableContracts. A source
// RegisterNavigableFields call without a contract row indicates the
// contract table is out of date.
func TestNavigableFields_NoOrphans(t *testing.T) {
	idx := buildContractIndex()
	var orphans []string
	for _, td := range resource.AllResourceTypes() {
		declared := idx[td.ShortName]
		for _, nf := range resource.GetNavigableFields(td.ShortName) {
			if declared == nil {
				orphans = append(orphans, td.ShortName+"."+nf.FieldPath+" (no contract rows)")
				continue
			}
			if _, ok := declared[nf.FieldPath]; !ok {
				orphans = append(orphans, td.ShortName+"."+nf.FieldPath+" (missing from contract table)")
			}
		}
	}
	sort.Strings(orphans)
	if len(orphans) > 0 {
		t.Errorf("registered NavigableField entries without a matching navigableContracts row:\n  %v\n\n"+
			"For each: add a navContract row citing the AWS API doc and reasoning, or remove the "+
			"unwanted registration.", orphans)
	}
}

// TestNavigableFields_EveryRegisteredTypeHasContractRow asserts every
// registered resource type has at least one row in navigableContracts
// (possibly a sentinel with fieldPath="" declaring "no navigable fields").
// This ensures new types are explicitly considered.
func TestNavigableFields_EveryRegisteredTypeHasContractRow(t *testing.T) {
	hasRow := make(map[string]bool, len(navigableContracts))
	for _, c := range navigableContracts {
		hasRow[c.shortName] = true
	}
	var missing []string
	for _, td := range resource.AllResourceTypes() {
		if !hasRow[td.ShortName] {
			missing = append(missing, td.ShortName)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("registered resource types without a navigableContracts row: %v\n\n"+
			"Add at least one row per type — either a real navigable field with AWS API URL "+
			"and reasoning, or a sentinel row with fieldPath=\"\" explaining why the type has "+
			"no cross-resource IDs on its list/describe response.", missing)
	}
}

func findContract(shortName, fieldPath string) navContract {
	for _, c := range navigableContracts {
		if c.shortName == shortName && c.fieldPath == fieldPath {
			return c
		}
	}
	return navContract{}
}

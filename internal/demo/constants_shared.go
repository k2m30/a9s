package demo

// Shared fixture identifiers used across multiple demo fixture files.
// Keep cross-resource references centralized to avoid drift.
//
// Naming convention for related-resource constants:
//
//	related{SourceCamel}{TargetCamel}ID[N]
//
// {SourceCamel} and {TargetCamel} are the resource short names in PascalCase:
// ec2→EC2, rds→RDS, s3→S3, tg→TG, ebs→EBS, asg→ASG, ng→NG, cfn→CFN, etc.
// Add a numeric suffix (1, 2, …) only when a source has multiple IDs for the same target type.
// Example: relatedEC2AlarmID1, relatedEC2AlarmID2, relatedEC2TGID (single — no suffix).
const (
	prodVPCID    = "vpc-0abc123def456789a"
	stagingVPCID = "vpc-0def456789abc123d"

	prodPublicSubnetA  = "subnet-0aaa111111111111a"
	prodPublicSubnetB  = "subnet-0bbb222222222222b"
	prodPrivateSubnetA = "subnet-0ccc333333333333c"
	prodPrivateSubnetB = "subnet-0ddd444444444444d"
	stagingSubnetA     = "subnet-0eee555555555555e"
	stagingSubnetB     = "subnet-0fff666666666666f"

	ecsClusterArnServices = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"
	ecsClusterArnBatch    = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-batch"

	// Security Group IDs — referenced by EC2, ELB, ENI, RDS, Redis, EKS, and SG fixtures.
	prodWebALBSGID    = "sg-0aaa111111111111a" // acme-web-alb-sg
	prodAPIInternalSGID = "sg-0bbb222222222222b" // acme-api-internal-sg
	prodRDSSGID       = "sg-0ccc333333333333c" // acme-rds-sg / acme-worker-sg
	prodDBProxySGID   = "sg-0ddd444444444444d" // acme-db-proxy-sg

	// AMI IDs — referenced by EC2 fixtures (ImageId field) and AMI fixtures.
	prodAMIID1 = "ami-0a1b2c3d4e5f60001" // Amazon Linux 2023 (x86_64)
	prodAMIID2 = "ami-0a1b2c3d4e5f60002" // Amazon Linux 2023 (arm64)
	prodAMIID3 = "ami-0a1b2c3d4e5f60003" // custom AMI

	// IAM Role ARNs — referenced by CodeBuild, CodePipeline, CFN, Lambda, ECS, EKS, and RDS fixtures.
	prodEKSNodeRoleARN    = "arn:aws:iam::123456789012:role/acme-eks-node-role"
	prodLambdaRoleARN     = "arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"
	prodCIDeployRoleARN   = "arn:aws:iam::123456789012:role/acme-ci-deploy-role"
	prodEKSClusterRoleARN = "arn:aws:iam::123456789012:role/eks-cluster-role"

	// IAM Instance Profile ARN — referenced by EC2 IamInstanceProfile field.
	prodInstanceProfileARN = "arn:aws:iam::123456789012:instance-profile/acme-rds-monitoring"

	// ELB name, ARN and DNS name — referenced by TG, R53, CloudFront, and ECS fixtures.
	// ELB fixture ID = name (matches production fetcher which uses LoadBalancerName as ID).
	prodELBName = "acme-prod-web"
	prodELBARN  = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"
	prodELBDNS  = "acme-prod-web-1234567890.us-east-1.elb.amazonaws.com"

	// ECR image URI — referenced by ECS task definition and ECR fixtures.
	prodECRAPIImageURI = "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"

	// S3 bucket names — referenced by CloudFront origins, R53 alias records, and
	// notification config fixtures.
	prodStaticAssetsBucket = "webapp-assets-prod"
	prodLogsBucket         = "data-pipeline-logs"

	// CloudFront distribution domain and ARN — referenced by R53 alias records.
	prodCFDomain = "d111111abcdef8.cloudfront.net"
	prodCFARN    = "arn:aws:cloudfront::123456789012:distribution/E1A2B3C4D5E6F7"

	// ACM certificate ARN — referenced by ELB listeners, CloudFront, and API GW.
	prodACMCertARN1 = "arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111"
	prodACMCertARN2 = "arn:aws:acm:us-east-1:123456789012:certificate/b2c3d4e5-6789-01ab-cdef-222222222222"

	// EKS cluster name — referenced by EKS, Node Group, CW Log Group, and EC2 tag fixtures.
	prodEKSClusterName = "acme-prod"

	// Lambda function names — referenced by CW Log Group naming conventions.
	// Log group for a Lambda fn is /aws/lambda/{FunctionName}.
	lambdaProcessOrdersFnName = "process-orders"

	relatedEC2TGID        = "acme-web-tg"
	relatedEC2ASGID       = "acme-web-prod-asg"
	relatedEC2AlarmID1    = "api-high-error-rate"
	relatedEC2AlarmID2    = "rds-cpu-utilization"
	relatedEC2EIPID       = "eipalloc-0aaa111111111111a"
	relatedEC2SnapshotID1 = "snap-0a1b2c3d4e5f60001"
	relatedEC2SnapshotID2 = "snap-0a1b2c3d4e5f60002"

	relatedEC2EBSVolID1   = "vol-0a1b2c3d4e5f60001"
	relatedEC2EBSVolID2   = "vol-0a1b2c3d4e5f60002"
	relatedEC2TrailEvent1    = "evt-0a1b2c3d4e5f60002"
	relatedEC2NGNodeGroupID  = "general-pool"

	relatedAlarmSNSID = "arn:aws:sns:us-east-1:123456789012:alarm-notifications"

	relatedAMIEC2ID   = "i-0a1b2c3d4e5f60001"
	relatedAMISnapID1 = "snap-0a1b2c3d4e5f60001"

	relatedASGEC2ID1    = "i-0a1b2c3d4e5f60001"
	relatedASGEC2ID2    = "i-0a1b2c3d4e5f60002"
	relatedASGEC2ID3    = "i-0a1b2c3d4e5f60003"
	relatedASGEC2ID4    = "i-0a1b2c3d4e5f60009"
	relatedASGTGID      = "acme-web-tg"
	relatedASGSubnetID1 = "subnet-0aaa111111111111a"
	relatedASGSubnetID2 = "subnet-0bbb222222222222b"
	relatedASGSubnetID3 = "subnet-0ccc333333333333c"

	relatedApigwLambdaID = "api-gateway-authorizer"
	relatedApigwLogsID   = "/aws/lambda/api-gateway-authorizer"
	relatedApigwWAFID    = "a1b2c3d4-5678-90ab-cdef-111111111111"

	relatedAthenaS3ID  = "data-pipeline-logs"
	relatedAthenaKMSID = "a1b2c3d4-5678-90ab-cdef-111111111111"

	relatedBackupRoleID = "acme-lambda-execution"

	relatedCbRoleID     = "acme-ci-deploy-role"
	relatedCbPipelineID = "acme-api-deploy"

	relatedCfS3ID  = "webapp-assets-prod"
	relatedCfELBID = "acme-prod-web"
)

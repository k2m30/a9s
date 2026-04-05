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
)

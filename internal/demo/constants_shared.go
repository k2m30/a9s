package demo

// Shared fixture identifiers used across multiple demo fixture files.
// Keep cross-resource references centralized to avoid drift.
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
	relatedEC2TrailEvent1 = "evt-0a1b2c3d4e5f60002"
)

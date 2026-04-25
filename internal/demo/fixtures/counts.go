package fixtures

import (
	"strings"

	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// ExpectedTopLevelCounts returns an independent top-level count oracle for demo
// integration tests, derived directly from the typed fixture datasets rather
// than from registered app fetchers.
func ExpectedTopLevelCounts() map[string]int {
	ec2 := NewEC2Fixtures()
	ecs := NewECSFixtures()
	eks := NewEKSFixtures()
	rds := NewRDSFixtures()
	docdb := NewDBCFixtures()
	iam := NewIAMFixtures()
	kms := NewKMSFixtures()
	s3 := NewS3Fixtures()
	lambda := NewLambdaFixtures()
	elb := NewELBFixtures()
	return map[string]int{
		"ec2":          countEC2Instances(ec2),
		"ecs-svc":      len(ecs.Services),
		"ecs":          len(ecs.Clusters),
		"ecs-task":     len(ecs.Tasks),
		"lambda":       len(lambda.Functions),
		"asg":          len(NewASGFixtures().AutoScalingGroups),
		"eb":           len(NewEBFixtures().Environments),
		"ebs":          len(ec2.Volumes),
		"ebs-snap":     len(ec2.Snapshots),
		"ami":          len(ec2.Images),
		"eks":          len(eks.Clusters),
		"ng":           countEKSNodegroups(eks),
		"elb":          len(elb.LoadBalancers),
		"tg":           len(elb.TargetGroups),
		"sg":           len(ec2.SecurityGroups),
		"vpc":          len(ec2.Vpcs),
		"subnet":       len(ec2.Subnets),
		"rtb":          len(ec2.RouteTables),
		"nat":          len(ec2.NatGateways),
		"igw":          len(ec2.InternetGateways),
		"eip":          len(ec2.Addresses),
		"vpce":         len(ec2.VpcEndpoints),
		"tgw":          len(ec2.TransitGateways),
		"eni":          len(ec2.NetworkInterfaces),
		"dbi":          len(rds.DBInstances),
		"s3":           len(s3.Buckets),
		"redis":        countRedisEngineReplicationGroups(NewRedisFixtures()),
		"dbc":          len(docdb.DBClusters),
		"ddb":          len(NewDDBFixtures().Tables),
		"opensearch":   len(NewOpenSearchFixtures().Domains),
		"redshift":     len(NewRedshiftFixtures().Clusters),
		"efs":          len(NewEFSFixtures().FileSystems),
		"dbi-snap":     len(rds.DBSnapshots),
		"docdb-snap":   len(docdb.DBClusterSnapshots),
		"alarm":        len(NewCloudWatchFixtures().Alarms),
		"logs":         len(NewCWLogsFixtures().LogGroups),
		"trail":        len(NewCloudTrailFixtures().Trails),
		"ct-events":    len(NewCloudTrailFixtures().Events),
		"sqs":          len(NewSQSFixtures().Queues),
		"sns":          len(NewSNSFixtures().Topics),
		"sns-sub":      len(NewSNSFixtures().Subscriptions),
		"eb-rule":      len(NewEventBridgeFixtures().Rules),
		"kinesis":      len(NewKinesisFixtures().Streams),
		"msk":          len(NewMSKFixtures().Clusters),
		"sfn":          len(NewSFNFixtures().StateMachines),
		"secrets":      len(NewSecretsFixtures().Secrets),
		"ssm":          len(NewSSMFixtures().Parameters),
		"kms":          countCustomerManagedKMSKeys(kms),
		"r53":          len(NewR53Fixtures().HostedZones),
		"cf":           len(NewCloudFrontFixtures().Distributions),
		"acm":          len(NewACMFixtures().Certificates),
		"apigw":        len(NewAPIGWFixtures().APIs),
		"role":         len(iam.Roles),
		"policy":       countTopLevelIAMPolicies(iam),
		"iam-user":     len(iam.Users),
		"iam-group":    len(iam.Groups),
		"waf":          len(NewWAFFixtures().WebACLSummaries),
		"cfn":          len(NewCFNFixtures().Stacks),
		"pipeline":     len(NewCodePipelineFixtures().Pipelines),
		"cb":           len(NewCodeBuildFixtures().Projects),
		"ecr":          len(NewECRFixtures().Repositories),
		"codeartifact": len(NewCodeArtifactFixtures().Repositories),
		"glue":         len(NewGlueFixtures().Jobs),
		"athena":       len(NewAthenaFixtures().WorkGroups),
		"backup":       len(NewBackupFixtures().Plans),
		"ses":          len(NewSESFixtures().Identities),
	}
}

func countEC2Instances(f *EC2Fixtures) int {
	total := 0
	for _, reservation := range f.Reservations {
		total += len(reservation.Instances)
	}
	return total
}

func countEKSNodegroups(f *EKSFixtures) int {
	total := 0
	for _, nodegroups := range f.Nodegroups {
		total += len(nodegroups)
	}
	return total
}

func countCustomerManagedKMSKeys(f *KMSFixtures) int {
	total := 0
	for _, meta := range f.Keys {
		if meta != nil && meta.KeyManager == kmstypes.KeyManagerTypeCustomer {
			total++
		}
	}
	return total
}

// countRedisEngineReplicationGroups mirrors the redis fetcher's engine filter
// (internal/aws/redis.go uses strings.EqualFold): RGs with Engine != "redis"
// (e.g. valkey, memcached fixtures) are excluded from the top-level list. The
// oracle must apply the same case-insensitive filter so the main-menu count
// matches what the fetcher renders regardless of casing in fixture or live
// data.
func countRedisEngineReplicationGroups(f *RedisFixtures) int {
	total := 0
	for _, rg := range f.ReplicationGroups {
		if rg.Engine == nil {
			continue
		}
		if strings.EqualFold(*rg.Engine, "redis") {
			total++
		}
	}
	return total
}

func countCustomerManagedIAMPolicies(f *IAMFixtures) int {
	total := 0
	for _, policy := range f.Policies {
		if policy.Arn != nil && IsCustomerManagedPolicyARN(*policy.Arn) {
			total++
		}
	}
	return total
}

// countTopLevelIAMPolicies mirrors the policy fetcher's output shape (see
// internal/aws/iam_policies.go): customer-managed policies plus every inline
// group policy surfaced by ListGroupPolicies. The oracle must include both so
// the main-menu count matches what operators see.
func countTopLevelIAMPolicies(f *IAMFixtures) int {
	total := countCustomerManagedIAMPolicies(f)
	for _, names := range f.InlineGroupPolicies {
		total += len(names)
	}
	return total
}

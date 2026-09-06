// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"strings"

	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// ExpectedTopLevelTruncationForTest reports which demo types answer their first
// page with more behind it, so a caller comparing a rendered count against
// ExpectedTopLevelCountsForTest knows which ones carry the "+" marker. Derived
// from the same two numbers as the count itself. Test-only: no production
// caller.
func ExpectedTopLevelTruncationForTest() map[string]bool {
	return map[string]bool{
		"logs": len(NewCWLogsFixtures().LogGroups) > LogGroupsPageSize,
	}
}

// ExpectedTopLevelCountsForTest returns an independent top-level count oracle
// for demo integration tests, derived directly from the typed fixture
// datasets rather than from registered app fetchers. Test-only: no
// production caller.
func ExpectedTopLevelCountsForTest() map[string]int {
	ec2 := NewEC2Fixtures()
	mwaaFix := NewMWAAFixtures()
	transferFix := NewTransferFixtures()
	ddb := NewDDBFixtures()
	openSearch := NewOpenSearchFixtures()
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
		"eks":          len(eks.Clusters) + len(eks.DeniedClusters) + len(eks.UnavailableClusters),
		"ng":           countEKSNodegroups(eks),
		"lt":           len(NewLTFixtures().LaunchTemplates),
		"elb":          len(elb.LoadBalancers),
		"tg":           len(elb.TargetGroups),
		"sg":           len(ec2.SecurityGroups),
		"vpc":          len(ec2.Vpcs),
		"subnet":       len(ec2.Subnets),
		"rtb":          len(ec2.RouteTables),
		"nat":          len(ec2.NatGateways),
		"igw":          len(ec2.InternetGateways),
		"vpc-peer":     len(NewVpcPeerFixtures().Connections),
		"eip":          len(ec2.Addresses),
		"vpce":         len(ec2.VpcEndpoints),
		"tgw":          len(ec2.TransitGateways),
		"eni":          len(ec2.NetworkInterfaces),
		"dbi":          len(rds.DBInstances),
		"s3":           len(s3.Buckets),
		"redis":        countRedisEngineReplicationGroups(NewRedisFixtures()),
		"dbc":          len(docdb.DBClusters) + len(rds.DBClusters),
		"ddb":          len(ddb.Tables) + len(ddb.DeniedNames) + len(ddb.UnavailableNames),
		"opensearch":   len(openSearch.Domains) + len(openSearch.UnavailableNames),
		"redshift":     len(NewRedshiftFixtures().Clusters),
		"efs":          len(NewEFSFixtures().FileSystems),
		"dbi-snap":     len(rds.DBSnapshots),
		"dbc-snap":     len(docdb.DBClusterSnapshots) + len(rds.DBClusterSnapshots),
		"alarm":        len(NewCloudWatchFixtures().Alarms),
		"logs":         min(len(NewCWLogsFixtures().LogGroups), LogGroupsPageSize),
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
		// The a9s apigw list merges both lanes: v2 HTTP APIs and v1 REST APIs.
		"apigw":        len(NewAPIGWFixtures().APIs) + len(NewAPIGWV1Fixtures().RestApis),
		"role":         len(iam.Roles),
		"policy":       countTopLevelIAMPolicies(iam),
		"iam-user":     len(iam.Users),
		"iam-group":    len(iam.Groups),
		"waf":          len(NewWAFFixtures().WebACLSummaries) + len(NewWAFFixtures().CloudFrontWebACLSummaries),
		"cfn":          len(NewCFNFixtures().Stacks),
		"pipeline":     len(NewCodePipelineFixtures().Pipelines),
		"cb":           len(NewCodeBuildFixtures().Projects),
		"ecr":          len(NewECRFixtures().Repositories),
		"codeartifact": len(NewCodeArtifactFixtures().Repositories),
		"glue":         len(NewGlueFixtures().Jobs),
		"athena":       len(NewAthenaFixtures().WorkGroups),
		"mwaa":         len(mwaaFix.Environments) + len(mwaaFix.DeniedNames) + len(mwaaFix.UnavailableNames),
		"transfer":     len(transferFix.ListedServers),
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
	for _, names := range f.DeniedNodegroups {
		total += len(names)
	}
	for _, names := range f.UnavailableNodegroups {
		total += len(names)
	}
	return total
}

func countCustomerManagedKMSKeys(f *KMSFixtures) int {
	total := 0
	for _, meta := range f.KeyList {
		if meta != nil && meta.KeyManager == kmstypes.KeyManagerTypeCustomer {
			total++
		}
	}
	return total
}

// countRedisEngineReplicationGroups mirrors the redis fetcher's engine filter
// (core/aws/redis.go uses strings.EqualFold): RGs with Engine != "redis"
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
// core/aws/iam_policies.go): customer-managed policies plus every inline
// group policy surfaced by ListGroupPolicies. The oracle must include both so
// the main-menu count matches what operators see.
func countTopLevelIAMPolicies(f *IAMFixtures) int {
	total := countCustomerManagedIAMPolicies(f)
	for _, names := range f.InlineGroupPolicies {
		total += len(names)
	}
	return total
}

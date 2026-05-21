package unit_test

import (
	"sync"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// relatedDefsSnapshot captures all related defs on first access, before any
// test has had a chance to mutate the registry via RegisterRelated. We can't
// take this snapshot in init() because init() runs before TestMain, and
// catalog.Find / catalog.All panic until SetTypes has been called by
// aws.Install in TestMain.
var (
	relatedDefsSnapshot     map[string][]resource.RelatedDef
	relatedDefsSnapshotOnce sync.Once
)

func snapshotRelatedDefs() map[string][]resource.RelatedDef {
	relatedDefsSnapshotOnce.Do(func() {
		snap := make(map[string][]resource.RelatedDef)
		for _, rt := range resource.AllResourceTypes() {
			defs := resource.GetRelated(rt.ShortName)
			if len(defs) > 0 {
				copied := make([]resource.RelatedDef, len(defs))
				copy(copied, defs)
				snap[rt.ShortName] = copied
			}
		}
		relatedDefsSnapshot = snap
	})
	return relatedDefsSnapshot
}

// TestRelatedDefs_AllDisplayNamesNonEmpty verifies that every registered
// RelatedDef (as of package init) has a non-empty DisplayName.
// An empty DisplayName produces a blank row label in the right column.
func TestRelatedDefs_AllDisplayNamesNonEmpty(t *testing.T) {
	for shortName, defs := range snapshotRelatedDefs() {
		for i, def := range defs {
			if def.DisplayName == "" {
				t.Errorf("%s: related def[%d] (TargetType=%q) has empty DisplayName",
					shortName, i, def.TargetType)
			}
		}
	}
}

// TestRelatedDefs_GoldenDisplayNames verifies that specific (resourceType,
// targetType) pairs have the exact expected DisplayName strings. This catches
// regressions where a display name is accidentally renamed or mistyped.
// Uses the package-init snapshot to avoid test-isolation contamination.
func TestRelatedDefs_GoldenDisplayNames(t *testing.T) {
	type key struct {
		shortName  string
		targetType string
	}
	// Golden map: (resourceShortName, targetType) → expected DisplayName.
	// Derived from internal/aws/*_related.go and verified against source.
	golden := map[key]string{
		// eb (Elastic Beanstalk) — required minimum
		{"eb", "cfn"}:  "CloudFormation Stack",
		{"eb", "logs"}: "Log Groups",
		{"eb", "asg"}:  "Auto Scaling Groups",
		{"eb", "ec2"}:  "EC2 Instances",

		// eb-rule (EventBridge Rule) — required minimum
		{"eb-rule", "role"}: "IAM Role",

		// ec2
		{"ec2", "tg"}:        "Target Groups",
		{"ec2", "asg"}:       "Auto Scaling Groups",
		{"ec2", "alarm"}:     "CloudWatch Alarms",
		{"ec2", "ng"}:        "EKS Node Groups",
		{"ec2", "cfn"}:       "CloudFormation Stacks",
		{"ec2", "eip"}:       "Elastic IPs",
		{"ec2", "ebs"}:       "EBS Volumes",
		{"ec2", "ebs-snap"}:  "EBS Snapshots",
		{"ec2", "ct-events"}: "CloudTrail Events",

		// vpc
		{"vpc", "subnet"}: "Subnets",
		{"vpc", "sg"}:     "Security Groups",
		{"vpc", "ec2"}:    "EC2 Instances",
		{"vpc", "elb"}:    "Load Balancers",
		{"vpc", "nat"}:    "NAT Gateways",
		{"vpc", "igw"}:    "Internet Gateways",
		{"vpc", "rtb"}:    "Route Tables",
		{"vpc", "vpce"}:   "VPC Endpoints",
		{"vpc", "cfn"}:    "CloudFormation",

		// sg
		{"sg", "vpc"}: "VPC",
		{"sg", "ec2"}: "EC2 Instances",
		{"sg", "eni"}: "Network Interfaces",
		{"sg", "elb"}: "Load Balancers",
		{"sg", "cfn"}: "CloudFormation",
		{"sg", "sg"}:  "Referencing SGs",

		// elb
		{"elb", "tg"}:    "Target Groups",
		{"elb", "alarm"}: "CW Alarms",
		{"elb", "cfn"}:   "CloudFormation",
		{"elb", "r53"}:   "Route 53 Records",

		// lambda
		{"lambda", "role"}:    "IAM Roles",
		{"lambda", "alarm"}:   "CW Alarms",
		{"lambda", "sqs"}:     "SQS Queues",
		{"lambda", "cfn"}:     "CloudFormation",
		{"lambda", "logs"}:    "Log Groups",
		{"lambda", "eb-rule"}: "EventBridge Rules",

		// dbi (RDS)
		{"dbi", "sg"}:       "Security Groups",
		{"dbi", "kms"}:      "KMS Key",
		{"dbi", "subnet"}:   "Subnets",
		{"dbi", "alarm"}:    "CloudWatch Alarms",
		{"dbi", "dbi-snap"}: "DB Instance Snapshots",
		{"dbi", "secrets"}:  "Secrets Manager",
		{"dbi", "logs"}:     "Log Groups",

		// dbc (DocumentDB)
		{"dbc", "sg"}:      "Security Groups",
		{"dbc", "alarm"}:   "CloudWatch Alarms",
		{"dbc", "secrets"}: "Secrets Manager",
		{"dbc", "logs"}:    "Log Groups",

		// eks
		{"eks", "ng"}:    "Node Groups",
		{"eks", "alarm"}: "CloudWatch Alarms",
		{"eks", "cfn"}:   "CloudFormation Stacks",
		{"eks", "logs"}:  "Log Groups",

		// ng (node groups)
		{"ng", "eks"}:  "EKS Clusters",
		{"ng", "role"}: "IAM Roles",
		{"ng", "asg"}:  "Auto Scaling Groups",
		{"ng", "ec2"}:  "EC2 Instances",

		// asg
		{"asg", "ec2"}:    "EC2 Instances",
		{"asg", "tg"}:     "Target Groups",
		{"asg", "subnet"}: "Subnets",
		{"asg", "alarm"}:  "CloudWatch Alarms",
		{"asg", "ng"}:     "EKS Node Groups",

		// kms
		{"kms", "ebs"}:     "EBS Volumes",
		{"kms", "dbi"}:     "RDS Instances",
		{"kms", "secrets"}: "Secrets Manager",
		{"kms", "s3"}:      "S3 Buckets",

		// secrets
		{"secrets", "kms"}:    "KMS Keys",
		{"secrets", "lambda"}: "Lambda (rotation)",
		{"secrets", "dbi"}:    "RDS Instances",
		{"secrets", "cfn"}:    "CloudFormation",

		// s3
		{"s3", "trail"}:  "CloudTrail Trails",
		{"s3", "cf"}:     "CloudFront",
		{"s3", "lambda"}: "Lambda (notifications)",
		{"s3", "cfn"}:    "CloudFormation",

		// ebs
		{"ebs", "ec2"}:      "EC2 Instance",
		{"ebs", "ebs-snap"}: "EBS Snapshots",
		{"ebs", "kms"}:      "KMS Key",

		// ebs-snap
		{"ebs-snap", "ami"}: "AMIs",
		{"ebs-snap", "ebs"}: "EBS Volume",
		{"ebs-snap", "ec2"}: "EC2 Instance",
		{"ebs-snap", "kms"}: "KMS Key",

		// logs (CloudWatch Logs)
		{"logs", "lambda"}: "Lambda Functions",
		{"logs", "alarm"}:  "CW Alarms",

		// alarm (CloudWatch Alarms)
		{"alarm", "sns"}: "SNS Topics",
		{"alarm", "asg"}: "Auto Scaling Groups",

		// ecs (ECS Clusters)
		{"ecs", "ecs-svc"}: "ECS Services",
		{"ecs", "alarm"}:   "CloudWatch Alarms",
		{"ecs", "cfn"}:     "CloudFormation Stacks",

		// ecs-svc
		{"ecs-svc", "ecs"}:   "ECS Clusters",
		{"ecs-svc", "tg"}:    "Target Groups",
		{"ecs-svc", "alarm"}: "CloudWatch Alarms",
		{"ecs-svc", "cfn"}:   "CloudFormation Stacks",
		{"ecs-svc", "elb"}:   "Load Balancers",
		{"ecs-svc", "logs"}:  "Log Groups",

		// ecs-task
		{"ecs-task", "ecs-svc"}: "ECS Services",
		{"ecs-task", "ecs"}:     "ECS Clusters",
		{"ecs-task", "logs"}:    "Log Groups",

		// cfn (CloudFormation)
		{"cfn", "role"}: "IAM Roles",
		{"cfn", "cfn"}:  "Related Stacks",

		// sqs
		{"sqs", "sns-sub"}: "SNS Subscriptions",
		{"sqs", "alarm"}:   "CloudWatch Alarms",
		{"sqs", "lambda"}:  "Lambda Functions",
		{"sqs", "sqs"}:     "Dead Letter Queues",

		// sns
		// sns→cfn dropped (Explicitly excluded: tag-heuristic only).
		{"sns", "alarm"}:   "CloudWatch Alarms",
		{"sns", "sns-sub"}: "Subscriptions",

		// sns-sub
		{"sns-sub", "sns"}:    "SNS Topic",
		{"sns-sub", "lambda"}: "Lambda Function",
		{"sns-sub", "sqs"}:    "SQS Queue",

		// r53
		{"r53", "elb"}: "Load Balancers",
		{"r53", "cf"}:  "CloudFront",
		{"r53", "acm"}: "ACM Certificates",

		// cf (CloudFront)
		{"cf", "s3"}:  "S3 Buckets (origin)",
		{"cf", "elb"}: "Load Balancers (origin)",
		{"cf", "waf"}: "WAF Web ACLs",
		{"cf", "acm"}: "ACM Certificates",
		{"cf", "r53"}: "Route 53 Zones",

		// acm
		{"acm", "elb"}:   "Load Balancers",
		{"acm", "cf"}:    "CloudFront Distros",
		{"acm", "apigw"}: "API Gateways",
		{"acm", "r53"}:   "Route 53 Zones",

		// apigw
		{"apigw", "lambda"}: "Lambda Functions",
		{"apigw", "logs"}:   "Log Groups",
		{"apigw", "waf"}:    "WAF Web ACLs",

		// waf
		{"waf", "elb"}:   "Load Balancers",
		{"waf", "apigw"}: "API Gateways",
		{"waf", "cf"}:    "CloudFront",

		// iam roles
		{"role", "lambda"}: "Lambda Functions",
		{"role", "glue"}:   "Glue Jobs",
		{"role", "ng"}:     "Node Groups",
		{"role", "policy"}: "IAM Policies",
		{"role", "ec2"}:    "EC2 Instances",

		// iam policies
		{"policy", "role"}:      "IAM Roles",
		{"policy", "iam-user"}:  "IAM Users",
		{"policy", "iam-group"}: "IAM Groups",

		// iam users
		{"iam-user", "iam-group"}: "IAM Groups",
		{"iam-user", "policy"}:    "IAM Policies",
		{"iam-user", "ct-events"}: "CloudTrail Events",

		// iam groups
		{"iam-group", "iam-user"}: "IAM Users",
		{"iam-group", "policy"}:   "IAM Policies",

		// tg (target groups)
		{"tg", "elb"}:     "Load Balancers",
		{"tg", "ecs-svc"}: "ECS Services",
		{"tg", "asg"}:     "Auto Scaling Groups",
		{"tg", "alarm"}:   "CW Alarms",

		// eni
		{"eni", "ec2"}: "EC2 Instances",
		{"eni", "sg"}:  "Security Groups",
		{"eni", "eip"}: "Elastic IPs",

		// eip
		{"eip", "ec2"}: "EC2 Instances",
		{"eip", "eni"}: "Network Interfaces",
		{"eip", "nat"}: "NAT Gateways",

		// igw
		{"igw", "vpc"}: "VPCs",
		{"igw", "rtb"}: "Route Tables",

		// nat
		{"nat", "vpc"}:    "VPCs",
		{"nat", "subnet"}: "Subnets",
		{"nat", "rtb"}:    "Route Tables",

		// subnet
		{"subnet", "ec2"}: "EC2 Instances",
		{"subnet", "eni"}: "Network Interfaces",
		{"subnet", "nat"}: "NAT Gateways",
		{"subnet", "elb"}: "Load Balancers",
		{"subnet", "rtb"}: "Route Tables",
		{"subnet", "cfn"}: "CloudFormation",

		// glue
		{"glue", "role"}:  "IAM Roles",
		{"glue", "alarm"}: "CW Alarms",
		{"glue", "cfn"}:   "CloudFormation Stacks",
		{"glue", "logs"}:  "Log Groups",

		// sfn (Step Functions)
		// sfn→cfn dropped (Explicitly excluded: tag-heuristic only).
		{"sfn", "alarm"}:   "CloudWatch Alarms",
		{"sfn", "logs"}:    "Log Groups",
		{"sfn", "role"}:    "IAM Role",
		{"sfn", "eb-rule"}: "EventBridge Rules",

		// ddb (DynamoDB)
		{"ddb", "kms"}:    "KMS Key",
		{"ddb", "lambda"}: "Lambda Functions",
		{"ddb", "alarm"}:  "CloudWatch Alarms",

		// kinesis
		{"kinesis", "lambda"}: "Lambda Functions",
		{"kinesis", "alarm"}:  "CW Alarms",
		{"kinesis", "cfn"}:    "CloudFormation",

		// ecr
		{"ecr", "lambda"}: "Lambda Functions",
		{"ecr", "cb"}:     "CodeBuild Projects",
		{"ecr", "cfn"}:    "CloudFormation Stacks",

		// cb (CodeBuild)
		{"cb", "logs"}:     "Log Groups",
		{"cb", "role"}:     "IAM Roles",
		{"cb", "pipeline"}: "CodePipelines",

		// pipeline (CodePipeline)
		{"pipeline", "cb"}:   "CodeBuild Projects",
		{"pipeline", "role"}: "IAM Roles",

		// backup
		{"backup", "role"}: "IAM Roles",

		// efs
		{"efs", "kms"}:    "KMS Keys",
		{"efs", "cfn"}:    "CloudFormation Stacks",
		{"efs", "lambda"}: "Lambda Functions",
		{"efs", "sg"}:     "Security Groups",
		{"efs", "subnet"}: "Subnets",

		// ses
		// ses→cfn dropped (Explicitly excluded: tag-heuristic only).
		{"ses", "r53"}: "Route 53 (DNS)",

		// athena
		{"athena", "s3"}:  "S3 Buckets (results)",
		{"athena", "kms"}: "KMS Keys",

		// redis
		{"redis", "alarm"}: "CW Alarms",
		{"redis", "cfn"}:   "CloudFormation",
		{"redis", "sg"}:    "Security Groups",

		// ami
		{"ami", "ec2"}:      "EC2 Instances",
		{"ami", "ebs-snap"}: "EBS Snapshots",
		{"ami", "asg"}:      "Auto Scaling Groups",

		// dbi-snap
		{"dbi-snap", "dbi"}: "DB Instances",
		{"dbi-snap", "kms"}: "KMS Keys",

		// dbc-snap
		{"dbc-snap", "dbc"}: "DocumentDB Cluster",

		// ssm
		{"ssm", "kms"}: "KMS Key",

		// opensearch
		{"opensearch", "alarm"}: "CW Alarms",
		{"opensearch", "cfn"}:   "CloudFormation",
		{"opensearch", "logs"}:  "Log Groups",

		// codeartifact
		// codeartifact→cb dropped (Explicitly excluded: unanimous sometimes — no first-class AWS field).

		// ct-events (CloudTrail Events)
		{"ct-events", "role"}:     "IAM Roles",
		{"ct-events", "iam-user"}: "IAM Users",
		{"ct-events", "ec2"}:      "EC2 Instances",
		{"ct-events", "s3"}:       "S3 Buckets",
		{"ct-events", "lambda"}:   "Lambda Functions",
		{"ct-events", "kms"}:      "KMS Keys",
		{"ct-events", "secrets"}:  "Secrets",
		{"ct-events", "vpce"}:     "VPC Endpoints",
		{"ct-events", "sg"}:       "Security Groups",
		{"ct-events", "ddb"}:      "DynamoDB Tables",
		{"ct-events", "cfn"}:      "CloudFormation Stacks",
	}

	for k, wantName := range golden {
		defs, ok := snapshotRelatedDefs()[k.shortName]
		if !ok {
			t.Errorf("%s: no related defs in snapshot (resource type not registered or has no related defs)",
				k.shortName)
			continue
		}
		found := false
		for _, def := range defs {
			if def.TargetType == k.targetType {
				found = true
				if def.DisplayName != wantName {
					t.Errorf("%s → %s: DisplayName = %q, want %q",
						k.shortName, k.targetType, def.DisplayName, wantName)
				}
				break
			}
		}
		if !found {
			t.Errorf("%s: no RelatedDef with TargetType=%q in snapshot (expected DisplayName=%q)",
				k.shortName, k.targetType, wantName)
		}
	}
}

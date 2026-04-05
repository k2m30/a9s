package demo

import "github.com/k2m30/a9s/v3/internal/resource"

func init() {
	resource.RegisterRelatedDemo("acm", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "elb", Count: 0},
			{TargetType: "cf", Count: 0},
			{TargetType: "apigw", Count: 0},
			{TargetType: "r53", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("ec2", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "tg", Count: 1, ResourceIDs: []string{relatedEC2TGID}},
			{TargetType: "asg", Count: 1, ResourceIDs: []string{relatedEC2ASGID}},
			{TargetType: "alarm", Count: 2, ResourceIDs: []string{relatedEC2AlarmID1, relatedEC2AlarmID2}},
			{TargetType: "cfn", Count: 0},
			{TargetType: "eip", Count: 1, ResourceIDs: []string{relatedEC2EIPID}},
			{TargetType: "ebs-snap", Count: 2, ResourceIDs: []string{relatedEC2SnapshotID1, relatedEC2SnapshotID2}},
			{TargetType: "ebs", Count: 2, ResourceIDs: []string{relatedEC2EBSVolID1, relatedEC2EBSVolID2}},
			{TargetType: "ng", Count: 1, ResourceIDs: []string{relatedEC2NGNodeGroupID}},
			{TargetType: "ct-events", Count: 1, ResourceIDs: []string{relatedEC2TrailEvent1}},
		}
	})

	resource.RegisterRelatedDemo("alarm", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "sns", Count: 1, ResourceIDs: []string{relatedAlarmSNSID}},
			{TargetType: "asg", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("ami", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ec2", Count: 1, ResourceIDs: []string{relatedAMIEC2ID}},
			{TargetType: "ebs-snap", Count: 1, ResourceIDs: []string{relatedAMISnapID1}},
			{TargetType: "asg", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("apigw", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "lambda", Count: 1, ResourceIDs: []string{relatedApigwLambdaID}},
			{TargetType: "logs", Count: 1, ResourceIDs: []string{relatedApigwLogsID}},
			{TargetType: "waf", Count: 1, ResourceIDs: []string{relatedApigwWAFID}},
		}
	})

	resource.RegisterRelatedDemo("athena", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "s3", Count: 1, ResourceIDs: []string{relatedAthenaS3ID}},
			{TargetType: "kms", Count: 1, ResourceIDs: []string{relatedAthenaKMSID}},
		}
	})

	resource.RegisterRelatedDemo("backup", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedBackupRoleID}},
		}
	})

	resource.RegisterRelatedDemo("asg", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "ec2", Count: 4, ResourceIDs: []string{relatedASGEC2ID1, relatedASGEC2ID2, relatedASGEC2ID3, relatedASGEC2ID4}},
			{TargetType: "tg", Count: 1, ResourceIDs: []string{relatedASGTGID}},
			{TargetType: "subnet", Count: 3, ResourceIDs: []string{relatedASGSubnetID1, relatedASGSubnetID2, relatedASGSubnetID3}},
			{TargetType: "alarm", Count: 0},
			{TargetType: "ng", Count: 0},
		}
	})

	resource.RegisterRelatedDemo("cb", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "logs", Count: 0},
			{TargetType: "role", Count: 1, ResourceIDs: []string{relatedCbRoleID}},
			{TargetType: "pipeline", Count: 1, ResourceIDs: []string{relatedCbPipelineID}},
		}
	})

	resource.RegisterRelatedDemo("cf", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "s3", Count: 1, ResourceIDs: []string{relatedCfS3ID}},
			{TargetType: "elb", Count: 1, ResourceIDs: []string{relatedCfELBID}},
			{TargetType: "waf", Count: 0},
			{TargetType: "acm", Count: 0},
			{TargetType: "r53", Count: 0},
		}
	})
}

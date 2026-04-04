package demo

import "github.com/k2m30/a9s/v3/internal/resource"

func init() {
	resource.RegisterRelatedDemo("ec2", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "tg", Count: 1, ResourceIDs: []string{relatedEC2TGID}},
			{TargetType: "asg", Count: 1, ResourceIDs: []string{relatedEC2ASGID}},
			{TargetType: "alarm", Count: 2, ResourceIDs: []string{relatedEC2AlarmID1, relatedEC2AlarmID2}},
			{TargetType: "cfn", Count: 0},
			{TargetType: "eip", Count: 1, ResourceIDs: []string{relatedEC2EIPID}},
			{TargetType: "ebs-snap", Count: 2, ResourceIDs: []string{relatedEC2SnapshotID1, relatedEC2SnapshotID2}},
			{TargetType: "ebs", Count: 2, ResourceIDs: []string{relatedEC2EBSVolID1, relatedEC2EBSVolID2}},
			{TargetType: "ng", Count: 0},
			{TargetType: "ct-events", Count: 1, ResourceIDs: []string{relatedEC2TrailEvent1}},
		}
	})
}

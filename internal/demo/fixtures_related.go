package demo

import "github.com/k2m30/a9s/v3/internal/resource"

func init() {
	resource.RegisterRelatedDemo("ec2", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "tg", Count: 1, ResourceIDs: []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/demo-web-tg/abc123"}},
			{TargetType: "asg", Count: 1, ResourceIDs: []string{"demo-web-asg"}},
			{TargetType: "alarm", Count: 2, ResourceIDs: []string{"demo-ec2-cpu-high", "demo-ec2-status-check"}},
			{TargetType: "cfn", Count: 0},
		}
	})
}

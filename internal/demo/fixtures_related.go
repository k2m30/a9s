package demo

import "github.com/k2m30/a9s/v3/internal/resource"

func init() {
	resource.RegisterRelatedDemo("ec2", func(res resource.Resource) []resource.RelatedCheckResult {
		return []resource.RelatedCheckResult{
			{TargetType: "tg", Count: 1, ResourceIDs: []string{"acme-web-tg"}},
			{TargetType: "asg", Count: 1, ResourceIDs: []string{"acme-web-prod-asg"}},
			{TargetType: "alarm", Count: 2, ResourceIDs: []string{"api-high-error-rate", "rds-cpu-utilization"}},
			{TargetType: "cfn", Count: 0},
			{TargetType: "eip", Count: 1, ResourceIDs: []string{"eipalloc-0aaa111111111111a"}},
			{TargetType: "ebs-snap", Count: 2, ResourceIDs: []string{"snap-0a1b2c3d4e5f60001", "snap-0a1b2c3d4e5f60002"}},
		}
	})
}

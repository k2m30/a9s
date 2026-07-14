package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// EC2DescribeLaunchTemplatesAPI defines the interface for the EC2
// DescribeLaunchTemplates operation — the list call for Launch Templates.
// EC2DescribeLaunchTemplateVersionsAPI (ec2_interfaces.go) already covers the
// per-template "$Default" describe call the lt fetcher funds every related-
// panel pivot and Wave 2 signal from (docs/resources/lt.md §1).
type EC2DescribeLaunchTemplatesAPI interface {
	DescribeLaunchTemplates(ctx context.Context, params *ec2.DescribeLaunchTemplatesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplatesOutput, error)
}

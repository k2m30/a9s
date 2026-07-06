package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/glue"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// GlueFake implements aws.GlueAPI against fixture data loaded at construction time.
type GlueFake struct {
	fix *fixtures.GlueFixtures
}

// NewGlue constructs a GlueFake backed by fixture data from the fixtures package.
func NewGlue() *GlueFake {
	return &GlueFake{fix: fixtures.NewGlueFixtures()}
}

func (f *GlueFake) GetJobs(_ context.Context, _ *glue.GetJobsInput, _ ...func(*glue.Options)) (*glue.GetJobsOutput, error) {
	return &glue.GetJobsOutput{Jobs: f.fix.Jobs}, nil
}

func (f *GlueFake) GetJobRuns(_ context.Context, input *glue.GetJobRunsInput, _ ...func(*glue.Options)) (*glue.GetJobRunsOutput, error) {
	var jobName string
	if input != nil && input.JobName != nil {
		jobName = *input.JobName
	}
	return &glue.GetJobRunsOutput{JobRuns: f.fix.JobRuns[jobName]}, nil
}

// GetSecurityConfiguration returns the named security configuration from
// fixture data. Backs the glue:kms related-panel pivot (checkGlueKMS).
func (f *GlueFake) GetSecurityConfiguration(_ context.Context, input *glue.GetSecurityConfigurationInput, _ ...func(*glue.Options)) (*glue.GetSecurityConfigurationOutput, error) {
	var name string
	if input != nil && input.Name != nil {
		name = *input.Name
	}
	cfg, ok := f.fix.SecurityConfigurations[name]
	if !ok {
		return &glue.GetSecurityConfigurationOutput{}, nil
	}
	return &glue.GetSecurityConfigurationOutput{SecurityConfiguration: &cfg}, nil
}

// GetTags returns tags for a Glue resource ARN from fixture data. Backs the
// glue:cfn related-panel pivot (checkGlueCFN).
func (f *GlueFake) GetTags(_ context.Context, input *glue.GetTagsInput, _ ...func(*glue.Options)) (*glue.GetTagsOutput, error) {
	var arn string
	if input != nil && input.ResourceArn != nil {
		arn = *input.ResourceArn
	}
	return &glue.GetTagsOutput{Tags: f.fix.TagsByResourceARN[arn]}, nil
}

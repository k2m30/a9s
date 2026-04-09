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

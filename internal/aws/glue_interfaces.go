package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/glue"
)

// GlueGetJobsAPI defines the interface for the Glue GetJobs operation.
type GlueGetJobsAPI interface {
	GetJobs(ctx context.Context, params *glue.GetJobsInput, optFns ...func(*glue.Options)) (*glue.GetJobsOutput, error)
}

// GlueGetJobRunsAPI defines the interface for the Glue GetJobRuns operation.
type GlueGetJobRunsAPI interface {
	GetJobRuns(ctx context.Context, params *glue.GetJobRunsInput, optFns ...func(*glue.Options)) (*glue.GetJobRunsOutput, error)
}

// GlueGetSecurityConfigurationAPI defines the interface for GetSecurityConfiguration.
type GlueGetSecurityConfigurationAPI interface {
	GetSecurityConfiguration(ctx context.Context, params *glue.GetSecurityConfigurationInput, optFns ...func(*glue.Options)) (*glue.GetSecurityConfigurationOutput, error)
}

// GlueGetTagsAPI defines the interface for the Glue GetTags operation.
type GlueGetTagsAPI interface {
	GetTags(ctx context.Context, params *glue.GetTagsInput, optFns ...func(*glue.Options)) (*glue.GetTagsOutput, error)
}

// GlueAPI is the aggregate interface covering all Glue operations used by a9s fetchers.
// *glue.Client structurally satisfies this interface.
type GlueAPI interface {
	GlueGetJobsAPI
	GlueGetJobRunsAPI
}

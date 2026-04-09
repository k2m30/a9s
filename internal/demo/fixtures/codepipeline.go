package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
)

// CodePipelineFixtures holds typed fixture data for CodePipeline.
type CodePipelineFixtures struct {
	Pipelines []cptypes.PipelineSummary
}

func mustParseCPTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewCodePipelineFixtures constructs CodePipelineFixtures from the canonical demo data.
func NewCodePipelineFixtures() *CodePipelineFixtures {
	return &CodePipelineFixtures{
		Pipelines: []cptypes.PipelineSummary{
			{
				Name:          aws.String("acme-api-deploy"),
				PipelineType:  cptypes.PipelineTypeV2,
				ExecutionMode: cptypes.ExecutionModeQueued,
				Version:       aws.Int32(3),
				Created:       aws.Time(mustParseCPTime("2025-04-10T09:00:00+00:00")),
				Updated:       aws.Time(mustParseCPTime("2026-03-20T11:30:00+00:00")),
			},
			{
				Name:         aws.String("acme-frontend-deploy"),
				PipelineType: cptypes.PipelineTypeV2,
				Version:      aws.Int32(5),
				Created:      aws.Time(mustParseCPTime("2025-05-15T14:00:00+00:00")),
				Updated:      aws.Time(mustParseCPTime("2026-03-19T16:45:00+00:00")),
			},
			{
				Name:         aws.String("acme-infra-pipeline"),
				PipelineType: cptypes.PipelineTypeV1,
				Version:      aws.Int32(12),
				Created:      aws.Time(mustParseCPTime("2024-08-20T08:30:00+00:00")),
				Updated:      aws.Time(mustParseCPTime("2026-03-10T10:00:00+00:00")),
			},
		},
	}
}

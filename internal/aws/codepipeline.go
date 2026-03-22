package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/codepipeline"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("pipeline", []string{"name", "pipeline_type", "version", "created", "updated"})
	resource.Register("pipeline", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCodePipelines(ctx, c.CodePipeline)
	})
}

// FetchCodePipelines calls the CodePipeline ListPipelines API and converts
// the response into a slice of generic Resource structs.
func FetchCodePipelines(ctx context.Context, api CodePipelineListPipelinesAPI) ([]resource.Resource, error) {
	output, err := api.ListPipelines(ctx, &codepipeline.ListPipelinesInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching CodePipeline pipelines: %w", err)
	}

	var resources []resource.Resource

	for _, pl := range output.Pipelines {
		name := ""
		if pl.Name != nil {
			name = *pl.Name
		}

		pipelineType := string(pl.PipelineType)

		created := ""
		if pl.Created != nil {
			created = pl.Created.Format("2006-01-02 15:04:05")
		}

		updated := ""
		if pl.Updated != nil {
			updated = pl.Updated.Format("2006-01-02 15:04:05")
		}

		version := ""
		if pl.Version != nil {
			version = fmt.Sprintf("%d", *pl.Version)
		}

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"name":          name,
				"pipeline_type": pipelineType,
				"created":       created,
				"updated":       updated,
				"version":       version,
			},
			RawStruct:  pl,
		}

		resources = append(resources, r)
	}

	return resources, nil
}

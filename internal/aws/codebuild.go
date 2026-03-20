package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/codebuild"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("cb", []string{"name", "source_type", "description", "last_modified"})
	resource.Register("cb", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCodeBuildProjects(ctx, c.CodeBuild, c.CodeBuild)
	})
}

// FetchCodeBuildProjects performs a two-step fetch:
// 1. ListProjects to get project names
// 2. BatchGetProjects to get full project details
func FetchCodeBuildProjects(
	ctx context.Context,
	listAPI CodeBuildListProjectsAPI,
	batchAPI CodeBuildBatchGetProjectsAPI,
) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListProjects(ctx, &codebuild.ListProjectsInput{})
	if err != nil {
		return nil, err
	}

	if len(listOutput.Projects) == 0 {
		return []resource.Resource{}, nil
	}

	batchOutput, err := batchAPI.BatchGetProjects(ctx, &codebuild.BatchGetProjectsInput{
		Names: listOutput.Projects,
	})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, project := range batchOutput.Projects {
		name := ""
		if project.Name != nil {
			name = *project.Name
		}

		description := ""
		if project.Description != nil {
			description = *project.Description
		}

		sourceType := ""
		if project.Source != nil {
			sourceType = string(project.Source.Type)
		}

		lastModified := ""
		if project.LastModified != nil {
			lastModified = project.LastModified.Format("2006-01-02T15:04:05Z07:00")
		}

		arn := ""
		if project.Arn != nil {
			arn = *project.Arn
		}

		serviceRole := ""
		if project.ServiceRole != nil {
			serviceRole = *project.ServiceRole
		}

		created := ""
		if project.Created != nil {
			created = project.Created.Format("2006-01-02T15:04:05Z07:00")
		}

		// Build DetailData
		detail := map[string]string{
			"Project Name": name,
			"Description":  description,
			"Source Type":   sourceType,
			"ARN":           arn,
			"Service Role":  serviceRole,
			"Last Modified": lastModified,
			"Created":       created,
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(project, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"name":          name,
				"source_type":   sourceType,
				"description":   description,
				"last_modified": lastModified,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  project,
		}

		resources = append(resources, r)
	}

	return resources, nil
}

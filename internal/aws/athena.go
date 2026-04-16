package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("athena", []string{"workgroup_name", "state", "description", "engine_version"})

	resource.RegisterPaginated("athena", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAthenaWorkgroupsPage(ctx, c.Athena, continuationToken)
	})

	resource.RegisterRelated("athena", []resource.RelatedDef{
	})
}

// FetchAthenaWorkgroups calls the Athena ListWorkGroups API and converts the
// response into a slice of generic Resource structs.
func FetchAthenaWorkgroups(ctx context.Context, api AthenaListWorkGroupsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchAthenaWorkgroupsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchAthenaWorkgroupsPage fetches a single page of Athena workgroups.
func FetchAthenaWorkgroupsPage(ctx context.Context, api AthenaListWorkGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &athena.ListWorkGroupsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListWorkGroups(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Athena workgroups: %w", err)
	}

	var resources []resource.Resource

	for _, wg := range output.WorkGroups {
		wgName := ""
		if wg.Name != nil {
			wgName = *wg.Name
		}

		state := string(wg.State)

		description := ""
		if wg.Description != nil {
			description = *wg.Description
		}

		creationTime := ""
		if wg.CreationTime != nil {
			creationTime = wg.CreationTime.Format("2006-01-02 15:04")
		}

		engineVersion := ""
		if wg.EngineVersion != nil && wg.EngineVersion.EffectiveEngineVersion != nil {
			engineVersion = *wg.EngineVersion.EffectiveEngineVersion
		}

		r := resource.Resource{
			ID:     wgName,
			Name:   wgName,
			Status: state,
			Fields: map[string]string{
				"workgroup_name": wgName,
				"state":          state,
				"description":    description,
				"creation_time":  creationTime,
				"engine_version": engineVersion,
			},
			RawStruct: wg,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

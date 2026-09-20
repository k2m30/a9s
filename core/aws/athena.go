// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchAthenaWorkgroupsPage fetches a single page of Athena workgroups. When
// api also implements AthenaGetWorkGroupAPI, each workgroup is enriched with
// result_output_location (and any other per-config fields) so sibling
// checkers like checkS3Athena can resolve via cache scan. This is the
// related-panel contract: pivots pointing at athena must actually work.
// Cost: N+1 per page (bounded by DefaultPageSize; GetWorkGroup is cheap).
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

	getAPI, _ := api.(AthenaGetWorkGroupAPI)

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

		outputLocation := ""
		costCap := ""
		if getAPI != nil && wgName != "" {
			if wgOut, wgErr := getAPI.GetWorkGroup(ctx, &athena.GetWorkGroupInput{WorkGroup: aws.String(wgName)}); wgErr == nil &&
				wgOut != nil && wgOut.WorkGroup != nil &&
				wgOut.WorkGroup.Configuration != nil {
				cfg := wgOut.WorkGroup.Configuration
				if cfg.ResultConfiguration != nil && cfg.ResultConfiguration.OutputLocation != nil {
					outputLocation = *cfg.ResultConfiguration.OutputLocation
				}
				// A nil cutoff means the workgroup caps nothing, which reads
				// as an empty cell rather than a zero-byte budget.
				if cfg.BytesScannedCutoffPerQuery != nil {
					costCap = formatBytes(*cfg.BytesScannedCutoffPerQuery)
				}
			}
		}

		r := resource.Resource{
			ID:   wgName,
			Name: wgName,
			Fields: map[string]string{
				"workgroup_name":         wgName,
				"state":                  state,
				"description":            description,
				"creation_time":          creationTime,
				"engine_version":         engineVersion,
				"result_output_location": outputLocation,
				"cost_cap":               costCap,
			},
			RawStruct: wg,
		}

		r.Findings = athenaStateFindings(state)

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

// athenaStateFindings is the one predicate for a workgroup's state, which is
// admin-controlled and blocks all query execution while DISABLED. colorAthena
// runs it over Fields for rows built outside the fetcher.
func athenaStateFindings(state string) []domain.Finding {
	if state == "DISABLED" {
		return []domain.Finding{wave1Finding(athenaCodeWorkgroupDisabled)}
	}
	return nil
}

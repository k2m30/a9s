package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchCBBuilds performs a two-step fetch:
// 1. ListBuildsForProject (single page) to collect build IDs
// 2. BatchGetBuilds in chunks of 100 (API limit) to get full build details
//
// When continuationToken is provided, it resumes ListBuildsForProject from
// that token. A single ListBuildsForProject call is made per invocation;
// IsTruncated and NextToken are forwarded as pagination metadata for the
// caller to request the next page.
func FetchCBBuilds(
	ctx context.Context,
	listAPI CodeBuildListBuildsForProjectAPI,
	batchAPI CodeBuildBatchGetBuildsAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	projectName := parentCtx["project_name"]

	// Step 1: Fetch one page of build IDs
	listInput := &codebuild.ListBuildsForProjectInput{
		ProjectName: &projectName,
	}
	if continuationToken != "" {
		listInput.NextToken = &continuationToken
	}

	listOutput, err := listAPI.ListBuildsForProject(ctx, listInput)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing builds for %s: %w", projectName, err)
	}

	pageIDs := listOutput.Ids

	if len(pageIDs) == 0 {
		return resource.FetchResult{
			Resources: []resource.Resource{},
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				TotalHint:   0,
				PageSize:    0,
			},
		}, nil
	}

	// Step 2: BatchGetBuilds in chunks of 100
	var resources []resource.Resource

	for i := 0; i < len(pageIDs); i += 100 {
		end := min(i+100, len(pageIDs))
		chunk := pageIDs[i:end]

		batchOutput, err := batchAPI.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{
			Ids: chunk,
		})
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("batch get builds for %s: %w", projectName, err)
		}

		for _, build := range batchOutput.Builds {
			resources = append(resources, convertCBBuild(build))
		}
	}

	nextToken := ""
	isTruncated := false
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
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

// convertCBBuild converts a single cbtypes.Build into a generic Resource.
func convertCBBuild(build cbtypes.Build) resource.Resource {
	buildID := ""
	if build.Id != nil {
		buildID = *build.Id
	}

	buildNumber := ""
	name := ""
	if build.BuildNumber != nil {
		buildNumber = fmt.Sprintf("%d", *build.BuildNumber)
		name = "#" + buildNumber
	}

	status := string(build.BuildStatus)

	startTime := ""
	if build.StartTime != nil {
		startTime = build.StartTime.UTC().Format("2006-01-02 15:04")
	}

	endTime := ""
	if build.EndTime != nil {
		endTime = build.EndTime.UTC().Format("2006-01-02 15:04")
	}

	duration := ""
	if build.EndTime != nil && build.StartTime != nil {
		duration = FormatHumanDuration(build.EndTime.Sub(*build.StartTime))
	} else if build.EndTime == nil && build.StartTime != nil {
		duration = "~" + FormatHumanDuration(time.Now().UTC().Sub(*build.StartTime))
	}

	sourceVersion := ""
	if build.SourceVersion != nil {
		sourceVersion = *build.SourceVersion
	}

	resolvedSourceVersion := ""
	if build.ResolvedSourceVersion != nil {
		resolvedSourceVersion = *build.ResolvedSourceVersion
	}

	sourceVersionShort := shortSHA(sourceVersion, resolvedSourceVersion)

	initiator := ""
	if build.Initiator != nil {
		initiator = *build.Initiator
	}

	buildArn := ""
	if build.Arn != nil {
		buildArn = *build.Arn
	}

	currentPhase := ""
	if build.CurrentPhase != nil {
		currentPhase = *build.CurrentPhase
	}

	logGroupName := ""
	logStreamName := ""
	if build.Logs != nil {
		if build.Logs.GroupName != nil {
			logGroupName = *build.Logs.GroupName
		}
		if build.Logs.StreamName != nil {
			logStreamName = *build.Logs.StreamName
		}
	}

	return resource.Resource{
		ID:     buildID,
		Name:   name,
		Status: status,
		Fields: map[string]string{
			"build_number":            buildNumber,
			"build_status":            status,
			"start_time":              startTime,
			"end_time":                endTime,
			"duration":                duration,
			"source_version_short":    sourceVersionShort,
			"initiator":               initiator,
			"build_id":                buildID,
			"build_arn":               buildArn,
			"current_phase":           currentPhase,
			"source_version":          sourceVersion,
			"resolved_source_version": resolvedSourceVersion,
			"log_group_name":          logGroupName,
			"log_stream_name":         logStreamName,
		},
		RawStruct: build,
	}
}

// shortSHA returns the first 8 characters of the source version or resolved
// source version. Prefers SourceVersion, falls back to ResolvedSourceVersion.
func shortSHA(sourceVersion, resolvedSourceVersion string) string {
	v := sourceVersion
	if v == "" {
		v = resolvedSourceVersion
	}
	if len(v) > 8 {
		return v[:8]
	}
	return v
}

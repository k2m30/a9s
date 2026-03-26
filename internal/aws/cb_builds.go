package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

const maxCBBuilds = 200

func init() {
	resource.RegisterFieldKeys("cb_builds", []string{
		"build_number", "build_status", "start_time", "end_time",
		"duration", "source_version_short", "initiator", "build_id",
		"build_arn", "current_phase", "source_version",
		"resolved_source_version", "log_group_name", "log_stream_name",
	})

	resource.RegisterPaginatedChild("cb_builds", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCBBuilds(ctx, c.CodeBuild, c.CodeBuild, parentCtx, continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "CodeBuild Builds",
		ShortName: "cb_builds",
		Columns:   resource.CBBuildColumns(),
		CopyField: "build_id",
		Children: []resource.ChildViewDef{{
			ChildType:      "cb_build_logs",
			Key:            "enter",
			ContextKeys:    map[string]string{"log_group_name": "log_group_name", "log_stream_name": "log_stream_name", "build_number": "build_number"},
			DisplayNameKey: "build_number",
			DrillCondition: func(r resource.Resource) bool {
				return r.Fields["log_group_name"] != ""
			},
			DrillBlockMessage: "Build logs not available in CloudWatch",
		}},
	})
}

// FetchCBBuilds performs a two-step fetch:
// 1. ListBuildsForProject (paginated) to collect build IDs up to maxCBBuilds
// 2. BatchGetBuilds in chunks of 100 (API limit) to get full build details
//
// When continuationToken is provided, it resumes ListBuildsForProject from
// that token. When the cap is reached and more pages exist,
// FetchResult.Pagination.IsTruncated is set to true with a NextToken for
// continuation.
func FetchCBBuilds(
	ctx context.Context,
	listAPI CodeBuildListBuildsForProjectAPI,
	batchAPI CodeBuildBatchGetBuildsAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	projectName := parentCtx["project_name"]

	// Step 1: Collect all build IDs via pagination
	var allIDs []string
	var nextToken *string
	if continuationToken != "" {
		nextToken = &continuationToken
	}

	var lastAPINextToken string

	for {
		input := &codebuild.ListBuildsForProjectInput{
			ProjectName: &projectName,
			NextToken:   nextToken,
		}

		output, err := listAPI.ListBuildsForProject(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing builds for %s: %w", projectName, err)
		}

		allIDs = append(allIDs, output.Ids...)

		if output.NextToken != nil {
			lastAPINextToken = *output.NextToken
		} else {
			lastAPINextToken = ""
		}

		if len(allIDs) >= maxCBBuilds {
			allIDs = allIDs[:maxCBBuilds]
			break
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	if len(allIDs) == 0 {
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

	for i := 0; i < len(allIDs); i += 100 {
		end := i + 100
		if end > len(allIDs) {
			end = len(allIDs)
		}
		chunk := allIDs[i:end]

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

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: lastAPINextToken != "",
			NextToken:   lastAPINextToken,
			PageSize:    len(resources),
			TotalHint:   len(resources),
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
		startTime = build.StartTime.UTC().Format("2006-01-02 15:04:05")
	}

	endTime := ""
	if build.EndTime != nil {
		endTime = build.EndTime.UTC().Format("2006-01-02 15:04:05")
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
			"initiator":              initiator,
			"build_id":               buildID,
			"build_arn":              buildArn,
			"current_phase":          currentPhase,
			"source_version":         sourceVersion,
			"resolved_source_version": resolvedSourceVersion,
			"log_group_name":         logGroupName,
			"log_stream_name":        logStreamName,
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

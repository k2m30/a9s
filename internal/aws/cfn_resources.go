package aws

import (
	"context"
	"fmt"

	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// cfnResourceFindings returns wave1 findings derived from a stack-resource
// status. *_FAILED → broken; *_IN_PROGRESS → warn; DELETE_COMPLETE → dim.
// Steady-state *_COMPLETE rows emit no finding (render healthy).
func cfnResourceFindings(status cfntypes.ResourceStatus) []domain.Finding {
	s := string(status)
	if s == "" {
		return nil
	}
	switch s {
	case "DELETE_COMPLETE":
		return []domain.Finding{{Code: CodeCfnResourceDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"}}
	}
	if strings.HasSuffix(s, "_FAILED") {
		return []domain.Finding{{Code: CodeCfnResourceFailed, Phrase: strings.ToLower(strings.ReplaceAll(s, "_", " ")), Severity: domain.SevBroken, Source: "wave1"}}
	}
	if strings.HasSuffix(s, "_IN_PROGRESS") {
		return []domain.Finding{{Code: CodeCfnResourceInProgress, Phrase: strings.ToLower(strings.ReplaceAll(s, "_", " ")), Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}

// FetchCfnResources calls the CloudFormation ListStackResources API and converts
// the response into a FetchResult with pagination support. A single API call is
// made per invocation; IsTruncated and NextToken are forwarded as pagination
// metadata for the caller to request the next page.
func FetchCfnResources(
	ctx context.Context,
	api CFNListStackResourcesAPI,
	stackName string,
	continuationToken string,
) (resource.FetchResult, error) {
	input := &cloudformation.ListStackResourcesInput{
		StackName: &stackName,
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListStackResources(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing CloudFormation stack resources for %s: %w", stackName, err)
	}

	var resources []resource.Resource
	for _, summary := range output.StackResourceSummaries {
		resources = append(resources, convertCfnResource(summary))
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

// convertCfnResource converts a single CloudFormation StackResourceSummary into a generic Resource.
func convertCfnResource(summary cfntypes.StackResourceSummary) resource.Resource {
	logicalResourceID := ""
	if summary.LogicalResourceId != nil {
		logicalResourceID = *summary.LogicalResourceId
	}

	physicalResourceID := ""
	if summary.PhysicalResourceId != nil {
		physicalResourceID = *summary.PhysicalResourceId
	}

	resourceType := ""
	if summary.ResourceType != nil {
		resourceType = *summary.ResourceType
	}

	resourceStatus := string(summary.ResourceStatus)

	driftStatus := ""
	if summary.DriftInformation != nil {
		driftStatus = string(summary.DriftInformation.StackResourceDriftStatus)
	}

	lastUpdated := ""
	if summary.LastUpdatedTimestamp != nil {
		lastUpdated = summary.LastUpdatedTimestamp.UTC().Format("2006-01-02 15:04")
	}

	return resource.Resource{
		ID:       logicalResourceID,
		Name:     logicalResourceID,
		Findings: cfnResourceFindings(summary.ResourceStatus),
		Fields: map[string]string{
			"logical_resource_id":  logicalResourceID,
			"physical_resource_id": physicalResourceID,
			"resource_type":        resourceType,
			"resource_status":      resourceStatus,
			"drift_status":         driftStatus,
			"last_updated":         lastUpdated,
		},
		RawStruct: summary,
	}
}

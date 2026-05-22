// cfn_issue_enrichment.go — Wave 2 issue enrichment for the cfn resource type.
package aws

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// EnrichCFNStackEvents calls DescribeStackEvents for each stack (first page only,
// up to EnrichmentCap stacks). It scans the most recent events client-side for any
// resource with ResourceStatus ending in "_FAILED". A failed resource event produces
// a "!" finding: "recent resource failure: <ResourceType>/<LogicalResourceId>".
// This surfaces hidden failures that are not reflected in the top-level StackStatus.
func EnrichCFNStackEvents(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.CloudFormation == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		stackName := r.Fields["stack_name"]
		if stackName == "" {
			stackName = r.ID
		}
		if stackName == "" {
			continue
		}
		total++
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cloudformation.DescribeStackEventsOutput, error) {
			return clients.CloudFormation.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
				StackName: aws.String(stackName),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		// Scan events from the first page for any resource with a _FAILED status.
		// The API returns events in reverse-chronological order; we inspect all
		// events on the first page to catch recent failures.
		var failedRows []resource.FindingRow
		for _, ev := range out.StackEvents {
			status := string(ev.ResourceStatus)
			if !strings.HasSuffix(status, "_FAILED") {
				continue
			}
			logicalID := ""
			if ev.LogicalResourceId != nil {
				logicalID = *ev.LogicalResourceId
			}
			resourceType := ""
			if ev.ResourceType != nil {
				resourceType = *ev.ResourceType
			}
			reason := ""
			if ev.ResourceStatusReason != nil {
				reason = *ev.ResourceStatusReason
			}
			label := resourceType
			if label == "" {
				label = logicalID
			} else if logicalID != "" {
				label = resourceType + "/" + logicalID
			}
			row := resource.FindingRow{Label: label, Value: status, Tier: "!"}
			failedRows = append(failedRows, row)
			if reason != "" {
				failedRows = append(failedRows, resource.FindingRow{Label: "Reason", Value: reason, Tier: "~"})
			}
		}
		if len(failedRows) == 0 {
			continue
		}
		key := r.ID
		if key == "" {
			key = stackName
		}
		findings[key] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  fmt.Sprintf("recent resource failure: %s", failedRows[0].Label),
			Rows:     failedRows,
		}
	}
	return IssueEnricherResult{IssueCount: len(findings), Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings},
		AggregateFailures("cfn-enrich: DescribeStackEvents", failures, total)
}

// EnrichCFNCombined merges findings from EnrichCFNStackEvents and EnrichCFNDrift.
// CFNStackEvents provides "!" findings for recent resource failures; EnrichCFNDrift
// adds "~" findings for stacks that have drifted from their template.
// On ID conflict, CFNStackEvents findings take precedence (they carry "!" severity).
// IssueCount = CFNStackEvents.IssueCount (drift adds 0). Truncated = either truncated.
// Partial findings from each sub-enricher are preserved even when they return an error (E5).
func EnrichCFNCombined(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	eventsResult, eventsErr := EnrichCFNStackEvents(ctx, clients, resources, nil)
	driftResult, driftErr := EnrichCFNDrift(ctx, clients, resources, nil)

	// Combine sub-enricher errors; partial findings are preserved below (E5).
	var combinedErr error
	switch {
	case eventsErr != nil && driftErr != nil:
		combinedErr = fmt.Errorf("%v; %v", eventsErr, driftErr)
	case eventsErr != nil:
		combinedErr = eventsErr
	case driftErr != nil:
		combinedErr = driftErr
	}

	merged := make(map[string]resource.EnrichmentFinding, len(eventsResult.Findings)+len(driftResult.Findings))
	// Drift findings go in first; stack-events findings overwrite on conflict.
	maps.Copy(merged, driftResult.Findings)
	maps.Copy(merged, eventsResult.Findings)
	// Merge field updates from both sub-enrichers (drift wins on conflict since
	// it writes drift_status; stack-events doesn't write field updates).
	mergedUpdates := make(map[string]map[string]string)
	for id, kvMap := range driftResult.FieldUpdates {
		mergedUpdates[id] = make(map[string]string, len(kvMap))
		maps.Copy(mergedUpdates[id], kvMap)
	}
	for id, kvMap := range eventsResult.FieldUpdates {
		if mergedUpdates[id] == nil {
			mergedUpdates[id] = make(map[string]string, len(kvMap))
		}
		maps.Copy(mergedUpdates[id], kvMap)
	}
	// Merge TruncatedIDs: union of both sub-enricher maps.
	mergedTruncatedIDs := make(map[string]bool, len(eventsResult.TruncatedIDs)+len(driftResult.TruncatedIDs))
	maps.Copy(mergedTruncatedIDs, eventsResult.TruncatedIDs)
	maps.Copy(mergedTruncatedIDs, driftResult.TruncatedIDs)
	return IssueEnricherResult{
		IssueCount:   eventsResult.IssueCount,
		Truncated:    eventsResult.Truncated || driftResult.Truncated,
		TruncatedIDs: mergedTruncatedIDs,
		Findings:     merged,
		FieldUpdates: mergedUpdates,
	}, combinedErr
}

// EnrichCFNDrift calls DescribeStacks per stack (up to EnrichmentCap stacks) to
// read DriftInformation.StackDriftStatus. A status of DRIFTED produces a "~" finding
// "stack drifted from template". IN_SYNC and NOT_CHECKED stacks produce no finding.
// Severity "~" findings do not contribute to IssueCount.
func EnrichCFNDrift(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.CloudFormation == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		stackName := r.Fields["stack_name"]
		if stackName == "" {
			stackName = r.ID
		}
		if stackName == "" {
			continue
		}
		total++
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cloudformation.DescribeStacksOutput, error) {
			return clients.CloudFormation.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
				StackName: aws.String(stackName),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		if len(out.Stacks) == 0 {
			continue
		}
		stack := out.Stacks[0]
		key := r.ID
		if key == "" {
			key = stackName
		}
		if stack.DriftInformation != nil {
			driftStatus := string(stack.DriftInformation.StackDriftStatus)
			fieldUpdates[key] = map[string]string{
				"drift_status": driftStatus,
			}
			if driftStatus == "DRIFTED" {
				findings[key] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  "stack drifted from template",
					Rows: []resource.FindingRow{
						{Label: "Drift Status", Value: driftStatus, Tier: "~"},
					},
				}
			}
		}
	}
	// "~" findings do not contribute to IssueCount per the IssueEnricherResult contract.
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates},
		AggregateFailures("cfn-enrich: DescribeStacks", failures, total)
}

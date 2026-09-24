// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// cfn_issue_enrichment.go — Wave 2 issue enrichment for the cfn resource type.
package aws

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// cfn canonical FindingCodes.
const (
	cfnCodeRecentResourceFailure domain.FindingCode = "cfn.recent-resource-failure"
	cfnCodeStackDrifted          domain.FindingCode = "cfn.stack-drifted"
)

// EnrichCFNStackEvents calls DescribeStackEvents for each stack (first page only,
// up to EnrichmentCap stacks). It scans the most recent events client-side for any
// resource with ResourceStatus ending in "_FAILED". A failed resource event produces
// a "!" finding: "recent resource failure: <ResourceType>/<LogicalResourceId>".
// This surfaces hidden failures that are not reflected in the top-level StackStatus.
func EnrichCFNStackEvents(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
	}
	if clients.CloudFormation == nil {
		return result, nil
	}
	truncated := false
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		stackName := r.Fields["stack_name"]
		if stackName == "" {
			stackName = r.ID
		}
		if stackName == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := firstPage(ctx, func() (*cloudformation.DescribeStackEventsOutput, error) {
			return clients.CloudFormation.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
				StackName: aws.String(stackName),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			truncated = true
			return
		}
		// Scan events from the first page for any resource with a _FAILED status.
		// The API returns events in reverse-chronological order; we inspect all
		// events on the first page to catch recent failures.
		var failedRows []domain.DetailRow
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
			name := resourceType
			if name == "" {
				name = logicalID
			} else if logicalID != "" {
				name = resourceType + "/" + logicalID
			}
			failedRows = append(failedRows,
				domain.DetailRow{Label: "Failed resource", Value: name, Tier: "!"},
				domain.DetailRow{Label: "Status", Value: domain.HumanizeStatusPhrase(status), Tier: "!"})
			if reason != "" {
				failedRows = append(failedRows, domain.DetailRow{Label: "Reason", Value: reason, Tier: "~"})
			}
		}
		if len(failedRows) == 0 {
			return
		}
		key := r.ID
		if key == "" {
			key = stackName
		}
		setWave2Finding(&result, key, cfnCodeRecentResourceFailure, failedRows)

	})

	SetTruncated(&result, truncated)
	return result, errors.Join(loopErr, AggregateFailures("DescribeStackEvents", failures, total))
}

// EnrichCFNCombined merges findings from EnrichCFNStackEvents and EnrichCFNDrift.
// CFNStackEvents provides "!" findings for recent resource failures; EnrichCFNDrift
// adds "~" findings for stacks that have drifted from their template. Both
// fold through mergeRowResult, so a stack that failed and drifted keeps both
// findings, events first.
// Partial findings from each sub-enricher are preserved even when they return an error.
func EnrichCFNCombined(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	eventsResult, eventsErr := EnrichCFNStackEvents(ctx, clients, resources, nil)
	driftResult, driftErr := EnrichCFNDrift(ctx, clients, resources, nil)

	merged := newRowResult()
	var mu sync.Mutex
	var failures []Failure
	mergeRowResult(&mu, &merged, &failures, eventsResult, nil)
	mergeRowResult(&mu, &merged, &failures, driftResult, nil)
	return merged, errors.Join(eventsErr, driftErr)
}

// EnrichCFNDrift calls DescribeStacks per stack (up to EnrichmentCap stacks) to
// read DriftInformation.StackDriftStatus. A status of DRIFTED produces a "~" finding
// "stack drifted from template". IN_SYNC and NOT_CHECKED stacks produce no finding.
func EnrichCFNDrift(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.CloudFormation == nil {
		return result, nil
	}
	truncated := false
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		stackName := r.Fields["stack_name"]
		if stackName == "" {
			stackName = r.ID
		}
		if stackName == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()
		out, err := firstPage(ctx, func() (*cloudformation.DescribeStacksOutput, error) {
			return clients.CloudFormation.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
				StackName: aws.String(stackName),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			truncated = true
			return
		}
		if len(out.Stacks) == 0 {
			return
		}
		stack := out.Stacks[0]
		key := r.ID
		if key == "" {
			key = stackName
		}
		if stack.DriftInformation != nil {
			driftStatus := string(stack.DriftInformation.StackDriftStatus)
			result.FieldUpdates[key] = map[string]string{
				"drift_status": domain.HumanizeStatusPhrase(driftStatus),
			}
			if driftStatus == "DRIFTED" {
				setWave2Finding(&result, key, cfnCodeStackDrifted, []domain.DetailRow{
					{Label: "Drift Status", Value: domain.HumanizeStatusPhrase(driftStatus), Tier: "~"},
				})

			}
		}
	})

	SetTruncated(&result, truncated)
	return result, errors.Join(loopErr, AggregateFailures("DescribeStacks", failures, total))
}

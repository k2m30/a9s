// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchCloudFormationStacksPage fetches a single page of CloudFormation stacks.
func FetchCloudFormationStacksPage(ctx context.Context, api CFNDescribeStacksAPI, continuationToken string) (resource.FetchResult, error) {
	input := &cloudformation.DescribeStacksInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeStacks(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CloudFormation stacks: %w", err)
	}

	var resources []resource.Resource

	for _, stack := range output.Stacks {
		stackName := ""
		if stack.StackName != nil {
			stackName = *stack.StackName
		}

		status := string(stack.StackStatus)

		creationTime := ""
		if stack.CreationTime != nil {
			creationTime = stack.CreationTime.Format("2006-01-02 15:04")
		}

		lastUpdated := ""
		if stack.LastUpdatedTime != nil {
			lastUpdated = stack.LastUpdatedTime.Format("2006-01-02 15:04")
		}

		description := ""
		if stack.Description != nil {
			description = *stack.Description
		}

		arn := ""
		if stack.StackId != nil {
			arn = *stack.StackId
		}

		r := resource.Resource{
			ID:       stackName,
			Name:     stackName,
			Findings: cfnStackFindings(status),

			Fields: map[string]string{
				"stack_name":    stackName,
				"status":        status,
				"creation_time": creationTime,
				"last_updated":  lastUpdated,
				"description":   description,
				"arn":           arn,
			},
			RawStruct: stack,
		}

		addCFNPostureFindings(&r, stack)
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

// cfnStatusWords renders a stack status as the words an operator says.
// "UPDATE_ROLLBACK_FAILED" is the SDK's spelling; "update rollback failed" is
// the sentence, and the rendered-surface rulings ban the former from the list
// status cell every cfn finding draws in.
func cfnStatusWords(status string) string {
	return strings.ToLower(domain.HumanizeStatusPhrase(status))
}

func cfnStackFindings(status string) []domain.Finding {
	switch status {
	case "ROLLBACK_COMPLETE", "ROLLBACK_FAILED",
		"UPDATE_ROLLBACK_COMPLETE", "UPDATE_ROLLBACK_FAILED",
		"IMPORT_ROLLBACK_COMPLETE", "IMPORT_ROLLBACK_FAILED":
		return []domain.Finding{{Code: CodeCFNStackRollback, Phrase: cfnStatusWords(status), Severity: domain.SevBroken, Source: "wave1"}}
	case "DELETE_COMPLETE":
		return []domain.Finding{{Code: CodeCFNStackDeleted, Phrase: cfnStatusWords(status), Severity: domain.SevDim, Source: "wave1"}}
	}
	if strings.HasSuffix(status, "_FAILED") {
		return []domain.Finding{{Code: CodeCFNStackFailed, Phrase: cfnStatusWords(status), Severity: domain.SevBroken, Source: "wave1"}}
	}
	if strings.HasSuffix(status, "_IN_PROGRESS") {
		return []domain.Finding{{Code: CodeCFNStackInProgress, Phrase: cfnStatusWords(status), Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}

// cfnStackIsTearingDown reports whether a stack has nothing left to fix: it is
// on its way out, or it never built. Posture findings ask the operator to
// change a setting, and there is no setting worth changing on either.
func cfnStackIsTearingDown(status string) bool {
	return strings.HasPrefix(status, "DELETE_") || strings.HasSuffix(status, "_FAILED")
}

// addCFNPostureFindings evaluates the two w6b posture signals against the
// DescribeStacks response the fetcher already holds. Each is independent of
// the other and of the lifecycle finding above (contract rule 4).
func addCFNPostureFindings(r *resource.Resource, stack cfntypes.Stack) {
	if cfnStackIsTearingDown(string(stack.StackStatus)) {
		return
	}

	// A nested stack cannot hold its own protection: CloudFormation refuses
	// the setting on a child and deletes it with the root. Reporting it would
	// name a setting the operator cannot change.
	if stack.ParentId == nil && !aws.ToBool(stack.EnableTerminationProtection) {
		addWave1Finding(r, CodeCFNTerminationProtectionOff, "termination protection off", cfnTerminationProtectionOffDetail, domain.SevWarn)
	}

	outputs := make(map[string]string, len(stack.Outputs))
	for _, o := range stack.Outputs {
		outputs[aws.ToString(o.OutputKey)] = aws.ToString(o.OutputValue)
	}
	addSecretScanFinding(r, CodeCFNOutputSecret, "credential in stack outputs", cfnOutputSecretDetail, outputs)
}

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

		secretRows := secretScanRows(cfnStackOutputs(stack))
		protection, outputSecret := cfnPostureOf(stack, len(secretRows) > 0)

		r := withWave1(resource.Resource{
			ID:   stackName,
			Name: stackName,

			Fields: map[string]string{
				"stack_name":             stackName,
				"status":                 status,
				"creation_time":          creationTime,
				"last_updated":           lastUpdated,
				"description":            description,
				"arn":                    arn,
				"termination_protection": protection,
				"output_secret":          outputSecret,
			},
			RawStruct: stack,
		}, withState("cfn", status, cfnPostureFindings(protection, outputSecret), cfnStatusWords(status))...)

		addCFNPostureRows(&r, secretRows)
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
// the sentence the list status cell of every cfn finding shows.
func cfnStatusWords(status string) string {
	return strings.ToLower(domain.HumanizeStatusPhrase(status))
}

// cfnBuiltStates is every stack status but those of a create that failed
// (CREATE_FAILED) or is being undone (ROLLBACK_IN_PROGRESS, ROLLBACK_FAILED,
// ROLLBACK_COMPLETE): such a stack never built what its template describes,
// and of ROLLBACK_COMPLETE the AWS CloudFormation User Guide says "When in
// this state, only a delete operation can be performed". Protection against
// deletion is advice against the action that clears it, and against the
// delete an operator already ran on a DELETE_FAILED stack. A stack in
// REVIEW_IN_PROGRESS is "expected StackId but without any templates or
// resources", so there is nothing to protect yet.
func cfnBuiltStates() []string {
	var out []string
	for _, s := range enumValues[cfntypes.StackStatus]() {
		switch {
		case s == string(cfntypes.StackStatusCreateFailed), strings.HasPrefix(s, "ROLLBACK_"),
			s == string(cfntypes.StackStatusDeleteFailed), s == string(cfntypes.StackStatusReviewInProgress):
			continue
		}
		out = append(out, s)
	}
	return out
}

// The words the fetcher writes into Fields for the two settings a stack is
// judged on beyond its status; a word from no vocabulary is reported on like
// an absent one.
const (
	cfnProtectionOn  = "on"
	cfnProtectionOff = "off"

	cfnOutputSecretPresent = "yes"
	cfnOutputSecretAbsent  = "no"
)

// cfnPostureOf derives the two words. The protection word is empty for a
// nested stack, whose protection CloudFormation refuses to set separately
// from its root's.
//
// secretInOutputs is the caller's single scan of the stack outputs; the word
// and the supporting rows are then the same reading of the same outputs.
func cfnPostureOf(stack cfntypes.Stack, secretInOutputs bool) (protection, outputSecret string) {
	if stack.ParentId == nil {
		protection = cfnProtectionOn
		if !aws.ToBool(stack.EnableTerminationProtection) {
			protection = cfnProtectionOff
		}
	}
	outputSecret = cfnOutputSecretAbsent
	if secretInOutputs {
		outputSecret = cfnOutputSecretPresent
	}
	return protection, outputSecret
}

func cfnStackOutputs(stack cfntypes.Stack) map[string]string {
	outputs := make(map[string]string, len(stack.Outputs))
	for _, o := range stack.Outputs {
		outputs[aws.ToString(o.OutputKey)] = aws.ToString(o.OutputValue)
	}
	return outputs
}

// cfnPostureFindings returns the two posture signals; which of them stand in
// the stack's status is the state table's answer.
func cfnPostureFindings(protection, outputSecret string) []domain.Finding {
	var out []domain.Finding
	if protection == cfnProtectionOff {
		out = append(out, wave1Finding(CodeCFNTerminationProtectionOff))
	}
	if outputSecret == cfnOutputSecretPresent {
		out = append(out, wave1Finding(CodeCFNOutputSecret))
	}
	return out
}

// addCFNPostureRows attaches the rows naming where the credential is and what
// kind it looks like. The value itself never leaves the scanner.
func addCFNPostureRows(r *resource.Resource, secretRows []domain.DetailRow) {
	if hasFinding(r.Findings, CodeCFNOutputSecret) {
		addWave1Rows(r, CodeCFNOutputSecret, secretRows...)
	}
}

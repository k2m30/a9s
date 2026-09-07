// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/k2m30/a9s/v3/core/catalog"
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

		r := resource.Resource{
			ID:       stackName,
			Name:     stackName,
			Findings: cfnStackFindings(status),

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
		}

		addCFNPostureRows(&r, protection, outputSecret, secretRows)
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
		return []domain.Finding{wave1Finding(CodeCFNStackRollback, cfnStatusWords(status), domain.SevBroken)}
	case "DELETE_COMPLETE":
		return []domain.Finding{wave1Finding(CodeCFNStackDeleted, cfnStatusWords(status), domain.SevDim)}
	}
	if strings.HasSuffix(status, "_FAILED") {
		return []domain.Finding{wave1Finding(CodeCFNStackFailed, cfnStatusWords(status), domain.SevBroken)}
	}
	if strings.HasSuffix(status, "_IN_PROGRESS") {
		return []domain.Finding{wave1Finding(CodeCFNStackInProgress, cfnStatusWords(status), domain.SevWarn)}
	}
	return nil
}

// cfnStackIsTearingDown reports whether a stack has nothing left to fix: it is
// on its way out, or it never built. Posture findings ask the operator to
// change a setting, and there is no setting worth changing on either.
func cfnStackIsTearingDown(status string) bool {
	return strings.HasPrefix(status, "DELETE_") || strings.HasSuffix(status, "_FAILED")
}

// The words the fetcher writes into Fields for the two settings a stack is
// judged on beyond its status. cfnPostureFindings reads these rather than the
// DescribeStacks struct, so a row rebuilt from Fields alone reaches the same
// verdict, and a word from no vocabulary is reported on like an absent one.
const (
	cfnProtectionOn  = "on"
	cfnProtectionOff = "off"

	cfnOutputSecretPresent = "yes"
	cfnOutputSecretAbsent  = "no"
)

// cfnPostureOf derives the two words. Both are empty for a stack there is
// nothing to fix on — one on its way out, one that never built — and the
// protection word is empty for a nested stack, whose protection
// CloudFormation refuses to set separately from its root's.
//
// secretInOutputs is the caller's single scan of the stack outputs; the word
// and the supporting rows are then the same reading of the same outputs.
func cfnPostureOf(stack cfntypes.Stack, secretInOutputs bool) (protection, outputSecret string) {
	if cfnStackIsTearingDown(string(stack.StackStatus)) {
		return "", ""
	}
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

// cfnPostureFindings returns the two w6b signals. Each is independent of the
// other and of the lifecycle finding (contract rule 4).
func cfnPostureFindings(protection, outputSecret string) []domain.Finding {
	var out []domain.Finding
	if protection == cfnProtectionOff {
		out = append(out, domain.Finding{
			Code: CodeCFNTerminationProtectionOff, Phrase: "termination protection off",
			Detail: catalog.Detail(CodeCFNTerminationProtectionOff), Severity: domain.SevWarn, Source: "wave1",
		})
	}
	if outputSecret == cfnOutputSecretPresent {
		out = append(out, domain.Finding{
			Code: CodeCFNOutputSecret, Phrase: "credential in stack outputs",
			Detail: catalog.Detail(CodeCFNOutputSecret), Severity: domain.SevBroken, Source: "wave1",
		})
	}
	return out
}

// addCFNPostureRows appends the posture findings and, for the output-secret
// one, the rows naming where the credential is and what kind it looks like.
// The value itself never leaves the scanner.
func addCFNPostureRows(r *resource.Resource, protection, outputSecret string, secretRows []domain.DetailRow) {
	r.Findings = append(r.Findings, cfnPostureFindings(protection, outputSecret)...)
	if outputSecret == cfnOutputSecretPresent {
		addWave1Rows(r, CodeCFNOutputSecret, secretRows...)
	}
}

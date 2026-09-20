// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/secretscan"
)

// ssmCodePlaintextSensitive is the canonical FindingCode for a String-type
// parameter whose name suggests it holds a credential, stored unencrypted.
const ssmCodePlaintextSensitive domain.FindingCode = "ssm.value.plaintext-sensitive"

// ssmCodeStaleValue is the canonical FindingCode for a parameter that has
// not been modified in over 365 days.
const ssmCodeStaleValue domain.FindingCode = "ssm.value.stale"

// FetchSSMParametersPage calls the SSM DescribeParameters API and returns a single
// page of parameters. Pass an empty continuationToken for the first page.
func FetchSSMParametersPage(ctx context.Context, api SSMDescribeParametersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ssm.DescribeParametersInput{MaxResults: aws.Int32(DefaultPageSize)}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeParameters(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching SSM parameters: %w", err)
	}

	var resources []resource.Resource
	for _, param := range output.Parameters {
		paramName := ""
		if param.Name != nil {
			paramName = *param.Name
		}

		paramType := string(param.Type)

		version := ""
		if param.Version != 0 {
			version = fmt.Sprintf("%d", param.Version)
		}

		lastModified := ""
		if param.LastModifiedDate != nil {
			lastModified = param.LastModifiedDate.Format("2006-01-02 15:04")
		}

		description := ""
		if param.Description != nil {
			description = *param.Description
		}

		findings := ssmColorFindings(paramName, paramType, param.LastModifiedDate)
		r := resource.Resource{
			ID:   paramName,
			Name: paramName,
			Fields: map[string]string{
				"name":          paramName,
				"type":          paramType,
				"version":       version,
				"last_modified": lastModified,
				"description":   description,
				"risk":          ssmRiskWord(findings),
			},
			Findings:  findings,
			RawStruct: param,
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

// ssmColorFindings is the one judgment of a parameter: an unencrypted value
// whose name says it holds a credential first, then a value nothing has
// changed in over a year, whatever its type. The Status cell, the row colour
// and the Attention block all read it. A parameter's path is the name over
// its value, so the name says "credential" here exactly when it says so on
// an environment variable or a task definition — secretscan's key rule.
func ssmColorFindings(paramName, paramType string, lastModifiedDate *time.Time) []domain.Finding {
	if paramType == "String" && secretscan.KeyNamesCredential(paramName) {
		return []domain.Finding{wave1Finding(ssmCodePlaintextSensitive)}
	}
	if lastModifiedDate != nil && time.Since(*lastModifiedDate) > 365*24*time.Hour {
		return []domain.Finding{wave1Finding(ssmCodeStaleValue)}
	}
	return nil
}

// ssmRiskWord is the Status column's word for what ssmColorFindings found,
// the one the risk vocabulary (risk_column.go) spells for an operator. It is
// what a row rebuilt from its cached Fields shows, having no findings of its
// own to word.
func ssmRiskWord(findings []domain.Finding) string {
	top, ok := domain.TopFinding(findings)
	if !ok {
		return ""
	}
	switch top.Code {
	case ssmCodePlaintextSensitive:
		return riskPlaintextValue
	case ssmCodeStaleValue:
		return riskStaleValue
	}
	return ""
}

// RevealSSMParameter calls the SSM GetParameter API with decryption enabled
// and returns the parameter value string.
func RevealSSMParameter(ctx context.Context, api SSMGetParameterAPI, paramName string) (string, error) {
	output, err := api.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &paramName,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("revealing SSM parameter: %w", err)
	}
	if output.Parameter == nil || output.Parameter.Value == nil {
		return "", nil
	}
	return *output.Parameter.Value, nil
}

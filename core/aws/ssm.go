// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ssmSensitiveSuffixes is the set of name suffixes colorSSM treats as
// sensitive when found on a plaintext (String) parameter.
var ssmSensitiveSuffixes = []string{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	"_password", "_secret", "_token", "_apikey",
	"_api_key", "_credentials", "_passwd",
}

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

		risk := ""
		lowerName := strings.ToLower(paramName)
		if paramType == "SecureString" && param.LastModifiedDate != nil && time.Since(*param.LastModifiedDate) > 365*24*time.Hour {
			risk = riskStaleValue
		} else if paramType == "String" && (strings.Contains(lowerName, "/password") || strings.Contains(lowerName, "/secret") || strings.Contains(lowerName, "/token")) {
			risk = riskPlaintextValue
		}

		r := resource.Resource{
			ID:   paramName,
			Name: paramName,
			Fields: map[string]string{
				"name":          paramName,
				"type":          paramType,
				"version":       version,
				"last_modified": lastModified,
				"description":   description,
				"risk":          risk,
			},
			Findings:  ssmColorFindings(paramName, paramType, param.LastModifiedDate),
			RawStruct: param,
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
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

// ssmColorFindings mirrors colorSSM's own precedence (plaintext-sensitive
// String parameter wins first, then a stale last-modified date on any type)
// so the list Status cell / detail Attention block always explain the
// non-healthy color.
func ssmColorFindings(paramName, paramType string, lastModifiedDate *time.Time) []domain.Finding {
	name := strings.ToLower(paramName)
	if paramType == "String" {
		for _, suffix := range ssmSensitiveSuffixes {
			if strings.HasSuffix(name, suffix) {
				return []domain.Finding{wave1Finding(ssmCodePlaintextSensitive)}
			}
		}
	}
	if lastModifiedDate != nil && time.Since(*lastModifiedDate) > 365*24*time.Hour {
		return []domain.Finding{wave1Finding(ssmCodeStaleValue)}
	}
	return nil
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

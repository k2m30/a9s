// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchECRRepositoriesPage fetches a single page of ECR repositories.
func FetchECRRepositoriesPage(ctx context.Context, api ECRDescribeRepositoriesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ecr.DescribeRepositoriesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeRepositories(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching ECR repositories: %w", err)
	}

	var resources []resource.Resource

	for _, repo := range output.Repositories {
		repoName := ""
		if repo.RepositoryName != nil {
			repoName = *repo.RepositoryName
		}

		uri := ""
		if repo.RepositoryUri != nil {
			uri = *repo.RepositoryUri
		}

		tagMutability := string(repo.ImageTagMutability)

		scanOnPush := "false"
		if repo.ImageScanningConfiguration != nil && repo.ImageScanningConfiguration.ScanOnPush {
			scanOnPush = "true"
		}

		createdAt := ""
		if repo.CreatedAt != nil {
			createdAt = repo.CreatedAt.Format("2006-01-02 15:04")
		}

		r := resource.Resource{
			ID:   repoName,
			Name: repoName,
			Fields: map[string]string{
				"repository_name": repoName,
				"uri":             uri,
				"tag_mutability":  tagMutability,
				"scan_on_push":    scanOnPush,
				"created_at":      createdAt,
			},
			RawStruct: repo,
		}

		addECRPostureFindings(&r, repo)
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

// addECRPostureFindings evaluates the two w6b posture signals that
// DescribeRepositories already answers. Neither carries a supporting row: the
// phrase states the whole fact, and a row repeating it would print that fact
// twice one line apart.
func addECRPostureFindings(r *resource.Resource, repo ecrtypes.Repository) {
	// An absent scanning configuration is the same exposure as an explicit
	// false — AWS does not scan either way — so this is one of the documented
	// places where a missing field is a finding.
	if repo.ImageScanningConfiguration == nil || !repo.ImageScanningConfiguration.ScanOnPush {
		r.Findings = append(r.Findings, domain.Finding{
			Code: CodeECRScanOnPushOff, Phrase: "scan on push off",
			Detail: ecrScanOnPushOffDetail, Severity: domain.SevWarn, Source: "wave1",
		})
	}

	// Only MUTABLE is the finding. IMMUTABLE_WITH_EXCLUSION still pins the
	// tags that matter, and an empty value is unknown rather than mutable, so
	// this must not be written as "anything that is not IMMUTABLE".
	if repo.ImageTagMutability == ecrtypes.ImageTagMutabilityMutable {
		r.Findings = append(r.Findings, domain.Finding{
			Code: CodeECRMutableTags, Phrase: "tags are mutable",
			Detail: ecrMutableTagsDetail, Severity: domain.SevWarn, Source: "wave1",
		})
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/glue"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchGlueJobsPage fetches a single page of Glue jobs.
func FetchGlueJobsPage(ctx context.Context, api GlueGetJobsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &glue.GetJobsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.GetJobs(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Glue jobs: %w", err)
	}

	var resources []resource.Resource

	for _, job := range output.Jobs {
		jobName := ""
		if job.Name != nil {
			jobName = *job.Name
		}

		role := ""
		if job.Role != nil {
			role = *job.Role
		}

		glueVersion := ""
		if job.GlueVersion != nil {
			glueVersion = *job.GlueVersion
		}

		workerType := string(job.WorkerType)

		numWorkers := ""
		if job.NumberOfWorkers != nil {
			numWorkers = strconv.Itoa(int(*job.NumberOfWorkers))
		}

		maxRetries := strconv.Itoa(int(job.MaxRetries))

		createdOn := ""
		if job.CreatedOn != nil {
			createdOn = job.CreatedOn.Format("2006-01-02 15:04")
		}

		lastModified := ""
		if job.LastModifiedOn != nil {
			lastModified = job.LastModifiedOn.Format("2006-01-02 15:04")
		}

		commandName := ""
		if job.Command != nil && job.Command.Name != nil {
			commandName = *job.Command.Name
		}

		r := resource.Resource{
			ID:   jobName,
			Name: jobName,
			Fields: map[string]string{
				"job_name":      jobName,
				"role":          role,
				"glue_version":  glueVersion,
				"worker_type":   workerType,
				"num_workers":   numWorkers,
				"max_retries":   maxRetries,
				"created_on":    createdOn,
				"last_modified": lastModified,
				"command":       commandName,
			},
			RawStruct: job,
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

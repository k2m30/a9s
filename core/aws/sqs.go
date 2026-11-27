// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// SQSQueueAttributesRow preserves queue attributes in structured form for
// detail/YAML views and tests.
type SQSQueueAttributesRow struct {
	QueueURL   string
	QueueName  string
	Attributes map[string]string
}

// FetchSQSQueuesPage performs a two-step fetch: ListQueues (single page) to get
// URLs, then GetQueueAttributes per queue for details.
// Pass an empty continuationToken for the first page.
func FetchSQSQueuesPage(ctx context.Context, listAPI SQSListQueuesAPI, attrAPI SQSGetQueueAttributesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &sqs.ListQueuesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	listOutput, err := listAPI.ListQueues(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing SQS queues: %w", err)
	}

	var resources []resource.Resource
	var tally describeTally
	for _, queueURL := range listOutput.QueueUrls {
		queueName := queueURL
		if parts := strings.Split(queueURL, "/"); len(parts) > 0 {
			queueName = parts[len(parts)-1]
		}
		rows := tally.one("GetQueueAttributes", queueURL, queueName, func() (*resource.Resource, error) {
			attrOutput, err := attrAPI.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
				QueueUrl: &queueURL,
				AttributeNames: []sqstypes.QueueAttributeName{
					sqstypes.QueueAttributeNameAll,
				},
			})
			if err != nil {
				return nil, err
			}
			if attrOutput.Attributes == nil {
				return nil, UnusableAnswerErr{Call: "GetQueueAttributes", Field: "queue attributes"}
			}
			attrs := attrOutput.Attributes
			r := resource.Resource{
				ID:   queueName,
				Name: queueName,
				Fields: map[string]string{
					"queue_name":         queueName,
					"queue_url":          queueURL,
					"arn":                attrs["QueueArn"],
					"approx_messages":    attrs["ApproximateNumberOfMessages"],
					"approx_not_visible": attrs["ApproximateNumberOfMessagesNotVisible"],
					"delay_seconds":      attrs["DelaySeconds"],
					"kms_key_id":         attrs["KmsMasterKeyId"],
				},
				RawStruct: SQSQueueAttributesRow{
					QueueURL:   queueURL,
					QueueName:  queueName,
					Attributes: attrs,
				},
			}
			return &r, nil
		}, func(err error) resource.Resource {
			r := degradedRow(queueName, err)
			r.Fields["queue_name"] = queueName
			r.Fields["queue_url"] = queueURL
			return r
		})
		resources = append(resources, rows...)
	}

	nextToken := ""
	isTruncated := false
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
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
	}, tally.err("sqs: GetQueueAttributes")
}

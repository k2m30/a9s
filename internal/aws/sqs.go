package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// SQSQueueAttributesRow preserves queue attributes in structured form for
// detail/YAML views and tests.
type SQSQueueAttributesRow struct {
	QueueURL   string
	QueueName  string
	Attributes map[string]string
}

func init() {
	resource.RegisterFieldKeys("sqs", []string{"queue_name", "queue_url", "arn", "approx_messages", "approx_not_visible", "delay_seconds"})

	resource.RegisterPaginated("sqs", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		listAPI, ok := c.SQS.(SQSListQueuesAPI)
		if !ok {
			return resource.FetchResult{}, fmt.Errorf("SQS client does not support ListQueues")
		}
		return FetchSQSQueuesPage(ctx, listAPI, c.SQS, continuationToken)
	})
}

// FetchSQSQueues calls the SQS ListQueues/GetQueueAttributes APIs and returns
// all pages of queues. Used by tests; the production path uses the per-page fetcher for pagination.
func FetchSQSQueues(ctx context.Context, listAPI SQSListQueuesAPI, attrAPI SQSGetQueueAttributesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchSQSQueuesPage(ctx, listAPI, attrAPI, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
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

	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sqs.ListQueuesOutput, error) {
		return listAPI.ListQueues(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing SQS queues: %w", err)
	}

	total := len(listOutput.QueueUrls)
	var resources []resource.Resource
	var failures []string
	for _, queueURL := range listOutput.QueueUrls {
		attrOutput, attrErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sqs.GetQueueAttributesOutput, error) {
			return attrAPI.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
				QueueUrl: &queueURL,
				AttributeNames: []sqstypes.QueueAttributeName{
					sqstypes.QueueAttributeNameAll,
				},
			})
		})
		if attrErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", queueURL, attrErr.Error()))
			continue
		}

		attrs := attrOutput.Attributes

		// Extract queue name from URL (last segment after /)
		queueName := queueURL
		if parts := strings.Split(queueURL, "/"); len(parts) > 0 {
			queueName = parts[len(parts)-1]
		}

		approxMessages := attrs["ApproximateNumberOfMessages"]
		approxNotVisible := attrs["ApproximateNumberOfMessagesNotVisible"]
		delaySeconds := attrs["DelaySeconds"]
		queueArn := attrs["QueueArn"]

		r := resource.Resource{
			ID:    queueName,
			Name:  queueName,
			Fields: map[string]string{
				"queue_name":         queueName,
				"queue_url":          queueURL,
				"arn":                queueArn,
				"approx_messages":    approxMessages,
				"approx_not_visible": approxNotVisible,
				"delay_seconds":      delaySeconds,
			},
			RawStruct: SQSQueueAttributesRow{
				QueueURL:   queueURL,
				QueueName:  queueName,
				Attributes: attrs,
			},
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
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
	}, AggregateFailures("sqs: GetQueueAttributes", failures, total)
}

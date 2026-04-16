// kinesis_related.go contains Kinesis Data Stream related-resource checker functions.
package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("kinesis", []resource.RelatedDef{
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkKinesisLambda, NeedsTargetCache: false},
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkKinesisAlarms, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkKinesisCFN, NeedsTargetCache: false},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkKinesisKMS},
	})

	// kinesisstypes.StreamSummary (list response): no navigable fields — KeyId/EncryptionType
	// are on DescribeStream's StreamDescriptionSummary, not the list summary used as RawStruct.
}

// checkKinesisLambda returns Count: 0 because Kinesis stream event source
// mappings are not available in the list API — the relationship cannot be
// determined from cache alone.
func checkKinesisLambda(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
}

// checkKinesisCFN returns Count: 0 because Kinesis stream tags are not included
// in the ListStreams response — the CFN relationship cannot be determined from
// cache alone.
func checkKinesisCFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

// checkKinesisAlarms checks the cache for CloudWatch alarms with StreamName dimension matching this stream.
func checkKinesisAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	streamName := res.ID
	if streamName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := kinesisRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "StreamName" && d.Value != nil && *d.Value == streamName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}

// checkKinesisKMS is a stub. The Kinesis ListStreams response returns
// StreamSummary objects which do not include EncryptionType or KeyId —
// those fields are only on DescribeStream's StreamDescriptionSummary.
func checkKinesisKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
}

// kinesisRelatedResources returns the resource list for target from cache or by fetching the first page.
func kinesisRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

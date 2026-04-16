package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDdbKMS reads SSEDescription.KMSMasterKeyArn from the TableDescription RawStruct.
// Pattern F — no cache needed.
func checkDdbKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	table, ok := assertStruct[ddbtypes.TableDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if table.SSEDescription == nil || table.SSEDescription.KMSMasterKeyArn == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	arn := *table.SSEDescription.KMSMasterKeyArn
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx == len(arn)-1 {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := arn[idx+1:]
	return relatedResult("kms", []string{keyID})
}

// checkDdbAlarm searches the alarm cache for alarms with a "TableName" dimension
// matching this DynamoDB table's name.
// Pattern D — dimension-based lookup.
func checkDdbAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	tableName := res.ID
	if tableName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := ddbRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == "TableName" && d.Value != nil && *d.Value == tableName {
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








// ddbRelatedResources returns the resource list for target from cache or by fetching the first page.
func ddbRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// checkDdbLambda finds Lambda functions wired to this DynamoDB table's stream
// (Pattern A — live API). DDB Streams are consumed through
// lambda:ListEventSourceMappings; the EventSourceArn on each mapping matches
// the table's LatestStreamArn. Lambda FunctionConfiguration does not embed
// event-source info, so there is no cache-only path. Returns Count: -1 when
// no live clients are available.
func checkDdbLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	table, ok := assertStruct[ddbtypes.TableDescription](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	if table.LatestStreamArn == nil || *table.LatestStreamArn == "" {
		// Streams not enabled on this table — no Lambda triggers are possible.
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	streamARN := *table.LatestStreamArn
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Lambda == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		EventSourceArn: &streamARN,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	var ids []string
	for _, m := range out.EventSourceMappings {
		if m.FunctionArn == nil {
			continue
		}
		parts := strings.Split(*m.FunctionArn, ":")
		name := parts[len(parts)-1]
		if name != "" {
			ids = append(ids, name)
		}
	}
	return relatedResult("lambda", ids)
}

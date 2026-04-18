// sqs_related.go contains SQS queue related-resource checker functions.
package aws

import (
	"context"
	"encoding/json"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("sqs", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSQSAlarm, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSQSLambda, NeedsTargetCache: false},
		{TargetType: "sqs", DisplayName: "Dead Letter Queues", Checker: checkSQSSQS, NeedsTargetCache: true},
		{TargetType: "sns-sub", DisplayName: "SNS Subscriptions", Checker: checkSQSSNSSub, NeedsTargetCache: true},
		{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkSQSSNS, NeedsTargetCache: true},
		{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkSQSEbRule, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSQSKMS},
	})

	// SQS RawStruct is a Fields map (QueueUrl + Attributes string map) — KmsMasterKeyId and
	// RedrivePolicy are embedded in the Attributes string map, not struct fields; no NavigableField path applies.
}

// checkSQSSNS resolves the SNS topics publishing to this queue by scanning the
// sns-sub cache: any subscription with protocol=sqs and Endpoint matching this
// queue's ARN maps back to its topic_arn. Pattern C — reverse two-hop.
func checkSQSSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	queueARN := ""
	if raw, ok := assertStruct[SQSQueueAttributesRow](res.RawStruct); ok {
		queueARN = raw.Attributes["QueueArn"]
	}
	queueName := res.ID
	if queueARN == "" && queueName == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}

	subList, truncated, err := sqsRelatedResources(ctx, clients, cache, "sns-sub")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1, Err: err}
	}
	if subList == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}

	topicSet := make(map[string]struct{})
	for _, sub := range subList {
		if sub.Fields["protocol"] != "sqs" {
			continue
		}
		endpoint := sub.Fields["endpoint"]
		if endpoint == "" {
			continue
		}
		match := false
		if queueARN != "" && strings.Contains(endpoint, queueARN) {
			match = true
		} else if queueName != "" && strings.HasSuffix(endpoint, ":"+queueName) {
			match = true
		}
		if !match {
			continue
		}
		if ta := sub.Fields["topic_arn"]; ta != "" {
			topicSet[ta] = struct{}{}
		}
	}

	if len(topicSet) == 0 {
		if truncated {
			return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}

	var ids []string
	for arn := range topicSet {
		ids = append(ids, arn)
	}
	return relatedResult("sns", ids)
}

// checkSQSSNSSub searches the sns-sub cache for subscriptions where protocol=sqs
// and the endpoint ARN contains this queue's ARN.
// Pattern C — reverse lookup in sns-sub cache.
func checkSQSSNSSub(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// Attempt to retrieve the queue ARN from the raw struct attributes first.
	queueARN := ""
	if raw, ok := assertStruct[SQSQueueAttributesRow](res.RawStruct); ok {
		queueARN = raw.Attributes["QueueArn"]
	}
	// Fall back to constructing a partial match from the queue name.
	queueName := res.ID
	if queueARN == "" && queueName == "" {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}

	subList, truncated, err := sqsRelatedResources(ctx, clients, cache, "sns-sub")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1, Err: err}
	}
	if subList == nil {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}

	var ids []string
	for _, subRes := range subList {
		if subRes.Fields["protocol"] != "sqs" {
			continue
		}
		endpoint := subRes.Fields["endpoint"]
		if endpoint == "" {
			continue
		}
		// Match by full ARN or queue name as a suffix.
		if (queueARN != "" && strings.Contains(endpoint, queueARN)) ||
			(queueName != "" && strings.HasSuffix(endpoint, ":"+queueName)) {
			ids = append(ids, subRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}
	return relatedResult("sns-sub", ids)
}

// checkSQSAlarm searches the alarm cache for CloudWatch alarms in the AWS/SQS
// namespace with a QueueName dimension matching this queue's name.
// Pattern C — reverse lookup in alarm cache.
func checkSQSAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	queueName := res.ID
	if queueName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := sqsRelatedResources(ctx, clients, cache, "alarm")
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
		if alarm.Namespace == nil || *alarm.Namespace != "AWS/SQS" {
			continue
		}
		for _, dim := range alarm.Dimensions {
			if dim.Name != nil && *dim.Name == "QueueName" &&
				dim.Value != nil && *dim.Value == queueName {
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

// sqsRedriveTarget extracts the deadLetterTargetArn from a RedrivePolicy JSON string.
// Returns empty string if not present or invalid.
func sqsRedriveTarget(redrivePolicy string) string {
	if redrivePolicy == "" {
		return ""
	}
	var policy struct {
		DeadLetterTargetArn string `json:"deadLetterTargetArn"`
	}
	if err := json.Unmarshal([]byte(redrivePolicy), &policy); err != nil {
		return ""
	}
	return policy.DeadLetterTargetArn
}

// checkSQSSQS finds DLQ relationships for this SQS queue in both directions:
// - Forward: queues that this queue sends dead letters to (via RedrivePolicy)
// - Reverse: queues that use this queue as their DLQ
// Pattern C — scans the SQS cache.
func checkSQSSQS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	thisARN := ""
	if raw, ok := assertStruct[SQSQueueAttributesRow](res.RawStruct); ok {
		thisARN = raw.Attributes["QueueArn"]
	}
	thisName := res.ID

	sqsList, truncated, err := sqsRelatedResources(ctx, clients, cache, "sqs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: -1, Err: err}
	}
	if sqsList == nil {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: -1}
	}

	// Forward: find the DLQ that this queue targets.
	var forwardDLQARN string
	if raw, ok := assertStruct[SQSQueueAttributesRow](res.RawStruct); ok {
		forwardDLQARN = sqsRedriveTarget(raw.Attributes["RedrivePolicy"])
	}

	idSet := make(map[string]struct{})
	for _, sqsRes := range sqsList {
		if sqsRes.ID == thisName {
			// Skip self.
			continue
		}
		raw, ok := assertStruct[SQSQueueAttributesRow](sqsRes.RawStruct)
		if !ok {
			continue
		}
		candidateARN := raw.Attributes["QueueArn"]

		// Forward: this queue's RedrivePolicy points to candidateARN.
		if forwardDLQARN != "" && candidateARN != "" && forwardDLQARN == candidateARN {
			idSet[sqsRes.ID] = struct{}{}
		}

		// Reverse: candidate queue's RedrivePolicy points to thisARN.
		if thisARN != "" {
			dlqTarget := sqsRedriveTarget(raw.Attributes["RedrivePolicy"])
			if dlqTarget != "" && dlqTarget == thisARN {
				idSet[sqsRes.ID] = struct{}{}
			}
		} else if thisName != "" {
			// Fallback: match by queue name suffix.
			dlqTarget := sqsRedriveTarget(raw.Attributes["RedrivePolicy"])
			if dlqTarget != "" && strings.HasSuffix(dlqTarget, ":"+thisName) {
				idSet[sqsRes.ID] = struct{}{}
			}
		}
	}

	var ids []string
	for id := range idSet {
		ids = append(ids, id)
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: -1}
	}
	return relatedResult("sqs", ids)
}

// sqsRelatedResources returns the cached resource list for the given target type,
// or fetches the first page via the registered paginated fetcher.
func sqsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}


// checkSQSLambda calls lambda:ListEventSourceMappings to find Lambda functions
// triggered by this SQS queue (Pattern A — direct API call).
func checkSQSLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	row, ok := res.RawStruct.(SQSQueueAttributesRow)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	queueARN := row.Attributes["QueueArn"]
	if queueARN == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		EventSourceArn: &queueARN,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	var ids []string
	for _, m := range out.EventSourceMappings {
		if m.FunctionArn != nil {
			// Extract function name from ARN (last segment after ":")
			parts := strings.Split(*m.FunctionArn, ":")
			ids = append(ids, parts[len(parts)-1])
		}
	}
	return relatedResult("lambda", ids)
}

// checkSQSKMS is a stub. The SQS RawStruct is a flat Fields map (QueueUrl +
// Attributes string values) — KmsMasterKeyId is embedded in the Attributes
// string map rather than a typed struct field, so it cannot be extracted via
// assertStruct. Use res.Fields["kms_key_id"] or KmsMasterKeyId attribute directly.
func checkSQSKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	keyID := res.Fields["kms_key_id"]
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{keyID})
}

// checkSQSEbRule resolves EventBridge rules that target this SQS queue.
// Pattern C: one events:ListRuleNamesByTarget call using the queue ARN.
// Queue ARN is read from SQSQueueAttributesRow.Attributes["QueueArn"].
// Count = len(RuleNames).
func checkSQSEbRule(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	queueARN := ""
	if raw, ok := assertStruct[SQSQueueAttributesRow](res.RawStruct); ok {
		queueARN = raw.Attributes["QueueArn"]
	}
	if queueARN == "" {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.EventBridge == nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	api, ok := c.EventBridge.(EventBridgeListRuleNamesByTargetAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eventbridge.ListRuleNamesByTargetOutput, error) {
		return api.ListRuleNamesByTarget(ctx, &eventbridge.ListRuleNamesByTargetInput{TargetArn: &queueARN})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1, Err: err}
	}
	return relatedResult("eb-rule", out.RuleNames)
}



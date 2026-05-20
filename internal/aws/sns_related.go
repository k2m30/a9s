// sns_related.go contains SNS topic related-resource checker functions.
package aws

import (
	"context"
	"encoding/json"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// snsGetTopicAttrs wraps GetTopicAttributes in RetryOnThrottle. Returns nil on
// any failure (unsupported client, API error, empty output).
func snsGetTopicAttrs(ctx context.Context, clients any, topicARN string) map[string]string {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.SNS == nil {
		return nil
	}
	api, ok := c.SNS.(SNSGetTopicAttributesAPI)
	if !ok {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sns.GetTopicAttributesOutput, error) {
		return api.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: &topicARN})
	})
	if err != nil || out == nil {
		return nil
	}
	return out.Attributes
}

func init() {
	catalog.RegisterRelated("sns", []domain.RelatedDef{
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSNSAlarm, NeedsTargetCache: false},
		{TargetType: "sns-sub", DisplayName: "Subscriptions", Checker: checkSNSSub, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSNSKMS, NeedsTargetCache: false},
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkSNSRole, NeedsTargetCache: false},
	})

	// snstypes topic: detail view renders only TopicArn — no cross-ref fields (KmsMasterKeyId,
	// subscriptions, delivery policies are GetTopicAttributes results, not in the list RawStruct).
}

// checkSNSAlarm searches the alarm cache for alarms whose AlarmActions, OKActions,
// or InsufficientDataActions reference this SNS topic ARN.
// Pattern C — reverse lookup in alarm cache.
func checkSNSAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	alarmList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "alarm")
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
		}
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
		if snsAlarmReferences(alarm, topicARN) {
			ids = append(ids, alarmRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkSNSSub searches the sns-sub cache for subscriptions whose topic_arn
// matches this SNS topic's ARN (Pattern C — reverse lookup in sns-sub cache).
func checkSNSSub(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		topicARN = res.ID
	}
	if topicARN == "" {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}

	subList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "sns-sub")
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1, Err: err}
	}
	if subList == nil {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}

	var ids []string
	for _, subRes := range subList {
		if subRes.Fields["topic_arn"] == topicARN {
			ids = append(ids, subRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("sns-sub")
	}
	return relatedResult("sns-sub", ids)
}

// checkSNSKMS resolves the KMS key used for at-rest encryption of this SNS topic
// via GetTopicAttributes (Pattern C: 1 API call, attribute "KmsMasterKeyId").
func checkSNSKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		topicARN = res.ID
	}
	if topicARN == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	attrs := snsGetTopicAttrs(ctx, clients, topicARN)
	if attrs == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	keyID := attrs["KmsMasterKeyId"]
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{arnLastSegment(keyID)})
}

// checkSNSRole extracts IAM role principals from the SNS topic's access policy
// (GetTopicAttributes "Policy"). Pattern C: 1 API call, offline JSON parse. Role
// names are extracted from Principal.AWS values that look like role ARNs.
func checkSNSRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		topicARN = res.ID
	}
	if topicARN == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	attrs := snsGetTopicAttrs(ctx, clients, topicARN)
	if attrs == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	policy := attrs["Policy"]
	if policy == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	seen := map[string]struct{}{}
	extractRoleNamesFromPolicy([]byte(policy), seen)
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	return relatedResult("role", names)
}

// extractRoleNamesFromPolicy walks a JSON IAM policy and records role names from
// Principal.AWS entries whose value looks like an IAM role ARN
// (arn:aws:iam::ACCT:role/NAME).
func extractRoleNamesFromPolicy(doc []byte, seen map[string]struct{}) {
	var raw any
	if err := json.Unmarshal(doc, &raw); err != nil {
		return
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if k == "AWS" {
					addPolicyPrincipal(val, seen)
				}
				walk(val)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(raw)
}

// addPolicyPrincipal handles Principal.AWS which can be a string or a []string.
// Only role ARNs (":role/") are recorded.
func addPolicyPrincipal(v any, seen map[string]struct{}) {
	switch x := v.(type) {
	case string:
		if strings.Contains(x, ":role/") {
			seen[arnRoleName(x)] = struct{}{}
		}
	case []any:
		for _, it := range x {
			if s, ok := it.(string); ok && strings.Contains(s, ":role/") {
				seen[arnRoleName(s)] = struct{}{}
			}
		}
	}
}

// snsAlarmReferences reports whether any of the alarm's action lists contain
// an ARN that matches or contains the given SNS topic ARN.
func snsAlarmReferences(alarm cwtypes.MetricAlarm, topicARN string) bool {
	for _, arn := range alarm.AlarmActions {
		if strings.Contains(arn, topicARN) || arn == topicARN {
			return true
		}
	}
	for _, arn := range alarm.OKActions {
		if strings.Contains(arn, topicARN) || arn == topicARN {
			return true
		}
	}
	for _, arn := range alarm.InsufficientDataActions {
		if strings.Contains(arn, topicARN) || arn == topicARN {
			return true
		}
	}
	return false
}

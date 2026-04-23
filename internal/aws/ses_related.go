// ses_related.go contains SES related-resource checker functions.
package aws

import (
	"context"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkSESR53 searches the R53 cache for hosted zones whose domain matches the
// SES identity domain. Pattern N — naming convention.
//
// EMAIL_ADDRESS identities: extract domain after "@".
// DOMAIN identities: use the identity name directly.
// Hosted zone names have a trailing dot (e.g. "acme-corp.com.") which is stripped
// before comparison.
func checkSESR53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domain := sesIdentityDomain(res)
	if domain == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}

	r53List, truncated, err := sesRelatedResources(ctx, clients, cache, "r53")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1, Err: err}
	}
	if r53List == nil {
		// Honest zero — target fetcher not registered or cache empty (r53 not yet fetched).
		return resource.ApproximateZero("r53")
	}

	var ids []string
	for _, zone := range r53List {
		zoneName := strings.TrimSuffix(zone.Name, ".")
		if strings.EqualFold(zoneName, domain) || strings.HasSuffix(domain, "."+zoneName) {
			ids = append(ids, zone.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("r53")
	}
	return relatedResult("r53", ids)
}

// sesIdentityDomain extracts the domain from a SES identity resource.
// For EMAIL_ADDRESS identities (containing "@"), it returns the part after "@".
// For DOMAIN identities, it returns the identity name directly.
func sesIdentityDomain(res resource.Resource) string {
	name := res.ID
	if name == "" {
		return ""
	}
	// EMAIL_ADDRESS: extract domain after @
	if idx := strings.LastIndex(name, "@"); idx >= 0 {
		return name[idx+1:]
	}
	// DOMAIN: use as-is
	return name
}

// sesRelatedResources returns the resource list for target from cache or by fetching the first page.
func sesRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// sesConfigSetName resolves the ConfigurationSetName for the given SES identity by
// calling sesv2:GetEmailIdentity. Returns "" if none is configured or on error.
func sesConfigSetName(ctx context.Context, c *ServiceClients, identityName string) string {
	if c == nil || c.SESv2 == nil || identityName == "" {
		return ""
	}
	api, ok := c.SESv2.(SESv2GetEmailIdentityAPI)
	if !ok {
		return ""
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sesv2.GetEmailIdentityOutput, error) {
		return api.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{EmailIdentity: &identityName})
	})
	if err != nil || out == nil || out.ConfigurationSetName == nil {
		return ""
	}
	return *out.ConfigurationSetName
}

// sesEventDestinations calls sesv2:GetConfigurationSetEventDestinations for the
// given configuration set name. Returns nil on error or missing config set.
func sesEventDestinations(ctx context.Context, c *ServiceClients, configSetName string) (*sesv2.GetConfigurationSetEventDestinationsOutput, error) {
	api, ok := c.SESv2.(SESv2GetConfigurationSetEventDestinationsAPI)
	if !ok {
		return nil, nil //nolint:nilnil // nil,nil signals "interface not satisfied" — caller treats it as 0 results
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sesv2.GetConfigurationSetEventDestinationsOutput, error) {
		return api.GetConfigurationSetEventDestinations(ctx, &sesv2.GetConfigurationSetEventDestinationsInput{
			ConfigurationSetName: &configSetName,
		})
	})
	return out, err
}

// sesReceiptRuleSetCache is a per-ServiceClients cache for the SES v1
// DescribeActiveReceiptRuleSet call. It is populated once per fetch batch and
// shared across all N identity checkers so the API is called at most once
// regardless of how many identities are in the list.
type sesReceiptRuleSetCache struct {
	once   sync.Once
	output *ses.DescribeActiveReceiptRuleSetOutput
	err    error
}

// sesRuleSetCacheMu protects sesRuleSetCaches map access.
var sesRuleSetCacheMu sync.Mutex

// sesRuleSetCaches maps *ServiceClients pointer to a per-clients cache so that
// successive calls to checkSESLambda / checkSESS3 within the same fetch batch
// hit the same cached result.
var sesRuleSetCaches = map[*ServiceClients]*sesReceiptRuleSetCache{}

// sesActiveReceiptRuleSet calls SES v1 DescribeActiveReceiptRuleSet exactly once
// per ServiceClients instance, caching the result for reuse across all identity
// rows in the batch.
func sesActiveReceiptRuleSet(ctx context.Context, c *ServiceClients) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	if c == nil || c.SES == nil {
		return nil, nil //nolint:nilnil // no SES v1 client; caller treats nil as "no rule set"
	}

	sesRuleSetCacheMu.Lock()
	cache, ok := sesRuleSetCaches[c]
	if !ok {
		cache = &sesReceiptRuleSetCache{}
		sesRuleSetCaches[c] = cache
	}
	sesRuleSetCacheMu.Unlock()

	cache.once.Do(func() {
		cache.output, cache.err = c.SES.DescribeActiveReceiptRuleSet(ctx, &ses.DescribeActiveReceiptRuleSetInput{})
	})
	return cache.output, cache.err
}

// checkSESEbRule checks the SES identity's configuration-set event destinations for
// EventBridge destinations. For each destination with EventBridgeDestination non-nil,
// returns EventBusArn. Also returns "default" if a destination has no explicit EventBusArn
// pattern (the only supported bus is the default bus per AWS docs, so EventBusArn is always set).
// API: sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations.
func checkSESEbRule(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	identityName := res.ID
	if identityName == "" {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1}
	}
	configSetName := sesConfigSetName(ctx, c, identityName)
	if configSetName == "" {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	out, err := sesEventDestinations(ctx, c, configSetName)
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1, Err: err}
	}
	if out == nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	var ids []string
	for _, dest := range out.EventDestinations {
		if dest.EventBridgeDestination == nil {
			continue
		}
		busARN := ""
		if dest.EventBridgeDestination.EventBusArn != nil {
			busARN = *dest.EventBridgeDestination.EventBusArn
		}
		if busARN == "" {
			busARN = "default"
		}
		ids = append(ids, busARN)
	}
	return relatedResult("eb-rule", ids)
}

// checkSESKinesis checks the SES identity's configuration-set event destinations for
// Kinesis Firehose destinations (note: Firehose, not Kinesis Data Streams).
// Returns DeliveryStreamArn for each KinesisFirehoseDestination.
// API: sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations.
func checkSESKinesis(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// Note: Firehose, not Kinesis Data Streams. SESv2 delivers to Firehose only.
	identityName := res.ID
	if identityName == "" {
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: -1}
	}
	configSetName := sesConfigSetName(ctx, c, identityName)
	if configSetName == "" {
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: 0}
	}
	out, err := sesEventDestinations(ctx, c, configSetName)
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: -1, Err: err}
	}
	if out == nil {
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: 0}
	}
	var ids []string
	for _, dest := range out.EventDestinations {
		if dest.KinesisFirehoseDestination == nil {
			continue
		}
		if dest.KinesisFirehoseDestination.DeliveryStreamArn != nil && *dest.KinesisFirehoseDestination.DeliveryStreamArn != "" {
			ids = append(ids, *dest.KinesisFirehoseDestination.DeliveryStreamArn)
		}
	}
	return relatedResult("kinesis", ids)
}

// checkSESLambda discovers Lambda functions invoked by SES v1 inbound receipt rules.
// Calls ses:DescribeActiveReceiptRuleSet (SES v1) once per fetch batch and walks
// Rules[].Actions[].LambdaAction.FunctionArn. Returns Count: 0 for accounts with
// no active receipt rule set (pure outbound SES) — operator-honest absence.
func checkSESLambda(ctx context.Context, clients any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	out, err := sesActiveReceiptRuleSet(ctx, c)
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if out == nil {
		// No active rule set — pure outbound account. Operator-honest 0.
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	arns := sesLambdaARNsFromRules(out.Rules)
	return relatedResult("lambda", arns)
}

// checkSESS3 discovers S3 buckets where SES v1 inbound receipt rules deposit received mail.
// Calls ses:DescribeActiveReceiptRuleSet (SES v1) once per fetch batch and walks
// Rules[].Actions[].S3Action.BucketName. Returns Count: 0 for accounts with no active
// receipt rule set — operator-honest absence.
func checkSESS3(ctx context.Context, clients any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	out, err := sesActiveReceiptRuleSet(ctx, c)
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}
	if out == nil {
		// No active rule set — pure outbound account. Operator-honest 0.
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	buckets := sesS3BucketsFromRules(out.Rules)
	return relatedResult("s3", buckets)
}

// sesLambdaARNsFromRules walks ReceiptRule actions and collects LambdaAction.FunctionArn values.
func sesLambdaARNsFromRules(rules []sestypes.ReceiptRule) []string {
	seen := map[string]struct{}{}
	var arns []string
	for _, rule := range rules {
		for _, action := range rule.Actions {
			if action.LambdaAction == nil || action.LambdaAction.FunctionArn == nil {
				continue
			}
			arn := *action.LambdaAction.FunctionArn
			if arn == "" {
				continue
			}
			if _, exists := seen[arn]; !exists {
				seen[arn] = struct{}{}
				arns = append(arns, arn)
			}
		}
	}
	return arns
}

// sesS3BucketsFromRules walks ReceiptRule actions and collects S3Action.BucketName values.
func sesS3BucketsFromRules(rules []sestypes.ReceiptRule) []string {
	seen := map[string]struct{}{}
	var buckets []string
	for _, rule := range rules {
		for _, action := range rule.Actions {
			if action.S3Action == nil || action.S3Action.BucketName == nil {
				continue
			}
			bucket := *action.S3Action.BucketName
			if bucket == "" {
				continue
			}
			if _, exists := seen[bucket]; !exists {
				seen[bucket] = struct{}{}
				buckets = append(buckets, bucket)
			}
		}
	}
	return buckets
}

// checkSESSns checks the SES identity's configuration-set event destinations for
// SNS topics (SnsDestination.TopicArn).
// API: sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations → SnsDestination.TopicArn.
func checkSESSns(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	identityName := res.ID
	if identityName == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	configSetName := sesConfigSetName(ctx, c, identityName)
	if configSetName == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	out, err := sesEventDestinations(ctx, c, configSetName)
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1, Err: err}
	}
	if out == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	var ids []string
	for _, dest := range out.EventDestinations {
		if dest.SnsDestination == nil {
			continue
		}
		if dest.SnsDestination.TopicArn != nil && *dest.SnsDestination.TopicArn != "" {
			ids = append(ids, *dest.SnsDestination.TopicArn)
		}
	}
	return relatedResult("sns", ids)
}

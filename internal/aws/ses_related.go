// ses_related.go contains SES related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

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
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
	}

	var ids []string
	for _, zone := range r53List {
		zoneName := strings.TrimSuffix(zone.Name, ".")
		if strings.EqualFold(zoneName, domain) || strings.HasSuffix(domain, "."+zoneName) {
			ids = append(ids, zone.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
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




// SES identity list RawStruct (sesv2types.IdentityInfo) exposes only IdentityName,
// IdentityType, SendingEnabled, VerificationStatus. Relationships to EventBridge,
// Kinesis, Lambda, S3, and SNS require per-identity calls (GetEmailIdentity →
// ConfigurationSetName, then GetConfigurationSetEventDestinations).
// SES v1 receipt-rule APIs (DescribeActiveReceiptRuleSet, GetIdentityNotificationAttributes)
// are not available in SESv2 SDK; their paths are noted below but not implemented.

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

// checkSESLambda checks the SES identity's configuration-set event destinations for
// Lambda function ARNs. SES v1 receipt-rule LambdaAction is not available via SESv2 SDK;
// only the SNS/EB/Kinesis paths are available. Returns 0 if no Lambda destinations found.
// Note: ses:DescribeActiveReceiptRuleSet (SES v1 only) is not available in this client.
func checkSESLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// SES v1 receipt-rule LambdaAction path (ses:DescribeActiveReceiptRuleSet) is
	// unavailable in the SESv2 SDK. Only configuration-set event destinations are checked.
	identityName := res.ID
	if identityName == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	configSetName := sesConfigSetName(ctx, c, identityName)
	if configSetName == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	// SESv2 GetConfigurationSetEventDestinations does not expose a Lambda destination type.
	// Lambda invocations come through SES v1 receipt rules only.
	_ = configSetName
	return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
}

// checkSESS3 checks the SES identity's inbound receipt rules for S3 store actions.
// SES v1 receipt-rule S3Action path (ses:DescribeActiveReceiptRuleSet) is unavailable
// in the SESv2 SDK. Returns 0 for valid identities (cannot resolve S3 buckets).
// Returns -1 for invalid RawStruct.
// Note: ses:DescribeActiveReceiptRuleSet (SES v1 only) is not available in this client.
func checkSESS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// Validate that the resource is a real SES identity — wrong RawStruct signals -1.
	if _, ok := assertStruct[sesv2types.IdentityInfo](res.RawStruct); !ok {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	// SES v1 receipt-rule S3Action path (ses:DescribeActiveReceiptRuleSet) is
	// unavailable in the SESv2 SDK. Cannot resolve S3 buckets from receipt rules.
	return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
}

// checkSESSns checks the SES identity's configuration-set event destinations for
// SNS topics (SnsDestination.TopicArn). Also includes SES v1 notification attributes
// and receipt-rule SNS actions, but those require the SES v1 API (GetIdentityNotificationAttributes,
// DescribeActiveReceiptRuleSet) which is unavailable in the SESv2 SDK.
// API: sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations → SnsDestination.TopicArn.
func checkSESSns(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	// SES v1 GetIdentityNotificationAttributes and DescribeActiveReceiptRuleSet are
	// unavailable in SESv2 SDK. Only configuration-set SnsDestination is checked.
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






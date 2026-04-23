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
	if truncated {
		return truncatedResultSES("r53", ids)
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
// DescribeActiveReceiptRuleSet call. Successful responses are memoized per
// *ServiceClients; errors are not memoized so transient failures retry on the
// next call instead of locking the client for its lifetime.
type sesReceiptRuleSetCache struct {
	mu     sync.Mutex
	output *ses.DescribeActiveReceiptRuleSetOutput
}

// sesRuleSetCacheMu protects sesRuleSetCaches map access.
var sesRuleSetCacheMu sync.Mutex

// sesRuleSetCaches maps *ServiceClients pointer to a per-clients cache so that
// successive calls to checkSESLambda / checkSESS3 within the same fetch batch
// hit the same cached result.
var sesRuleSetCaches = map[*ServiceClients]*sesReceiptRuleSetCache{}

// sesActiveReceiptRuleSet calls SES v1 DescribeActiveReceiptRuleSet at most once
// per ServiceClients instance when the call succeeds. Errors are not cached: a
// follow-up call after an error retries the API. A follow-up call after a
// successful response returns the cached output without a network round-trip.
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

	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.output != nil {
		return cache.output, nil
	}
	out, err := c.SES.DescribeActiveReceiptRuleSet(ctx, &ses.DescribeActiveReceiptRuleSetInput{})
	if err != nil {
		return nil, err
	}
	cache.output = out
	return out, nil
}

// checkSESEbRule checks the SES identity's configuration-set event destinations for
// EventBridge destinations, then scans the eb-rule cache for rules whose EventBusName
// matches one of the bus names extracted from EventBusArn.
// API: sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations
// → extract bus names → scan eb-rule cache → return matching rule IDs.
func checkSESEbRule(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	identityName := res.ID
	if identityName == "" {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		// Without a client we cannot query SESv2 to discover EventBridge bus names.
		// Return Count=0 (early exit — no-client path cannot produce results).
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
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

	// Collect bus names from EventBridge destinations.
	busNames := map[string]struct{}{}
	for _, dest := range out.EventDestinations {
		if dest.EventBridgeDestination == nil {
			continue
		}
		busARN := ""
		if dest.EventBridgeDestination.EventBusArn != nil {
			busARN = *dest.EventBridgeDestination.EventBusArn
		}
		name := extractEventBusName(busARN)
		if name != "" {
			busNames[name] = struct{}{}
		}
	}
	if len(busNames) == 0 {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
	}

	// Scan the eb-rule cache for rules on matching buses.
	ebRules, truncated, cacheErr := sesRelatedResources(ctx, clients, cache, "eb-rule")
	if cacheErr != nil {
		return resource.RelatedCheckResult{TargetType: "eb-rule", Count: -1, Err: cacheErr}
	}
	if ebRules == nil {
		return resource.ApproximateZero("eb-rule")
	}

	var ids []string
	for _, rule := range ebRules {
		if _, ok := busNames[rule.Fields["event_bus"]]; ok {
			ids = append(ids, rule.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("eb-rule")
	}
	if truncated {
		return truncatedResultSES("eb-rule", ids)
	}
	return relatedResult("eb-rule", ids)
}

// extractEventBusName extracts the bus name from an EventBridge event bus ARN.
// ARN format: arn:aws:events:REGION:ACCOUNT:event-bus/NAME
// If the input contains no "/", it is returned as-is (handles already-extracted names).
// Returns "" for empty input.
func extractEventBusName(arn string) string {
	if arn == "" {
		return ""
	}
	if idx := strings.LastIndex(arn, "/"); idx >= 0 {
		return arn[idx+1:]
	}
	return arn
}

// sesRuleAppliesToIdentity reports whether a receipt rule should be considered
// when computing related resources for the given SES identity.
//
// Scoping rules:
//   - len(rule.Recipients) == 0 → rule applies to all identities.
//   - For EMAIL_ADDRESS identities (identityType == "EMAIL_ADDRESS"):
//     a recipient matches if it equals the identity exactly, equals the
//     identity's domain, or is a parent domain of the identity's domain.
//   - For DOMAIN identities: a recipient matches if it equals the domain
//     exactly, is a subdomain of it, or is an email address whose domain
//     equals or is a subdomain of the identity domain.
//
// Empty-string recipient entries are skipped conservatively. If every entry
// is an empty string (rare data corruption), the rule is treated as applying
// to all (AWS normalises absent Recipients to nil; a non-nil all-empty slice
// is anomalous).
func sesRuleAppliesToIdentity(rule sestypes.ReceiptRule, identityName, identityType string) bool {
	if len(rule.Recipients) == 0 {
		return true
	}

	// Collect non-empty recipients.
	var valid []string
	for _, r := range rule.Recipients {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			valid = append(valid, trimmed)
		}
	}
	// All entries are empty strings — treat conservatively as "applies to all".
	if len(valid) == 0 {
		return true
	}

	switch identityType {
	case "EMAIL_ADDRESS":
		// Derive domain from identity (e.g. "billing@sub.acme.com" → "sub.acme.com").
		domain := ""
		if idx := strings.LastIndex(identityName, "@"); idx >= 0 {
			domain = identityName[idx+1:]
		}
		for _, r := range valid {
			// Exact email match.
			if strings.EqualFold(r, identityName) {
				return true
			}
			// Recipient is the identity's domain or a parent domain.
			if domain != "" {
				rLower := strings.ToLower(r)
				dLower := strings.ToLower(domain)
				if rLower == dLower || strings.HasSuffix(dLower, "."+rLower) {
					return true
				}
			}
		}
	default:
		// DOMAIN identity.
		domainLower := strings.ToLower(identityName)
		for _, r := range valid {
			rLower := strings.ToLower(r)
			// Recipient is an email address — check its domain.
			rDomain := rLower
			if idx := strings.LastIndex(rLower, "@"); idx >= 0 {
				rDomain = rLower[idx+1:]
			}
			// rDomain equals the identity domain or is a subdomain of it.
			if rDomain == domainLower || strings.HasSuffix(rDomain, "."+domainLower) {
				return true
			}
		}
	}
	return false
}

// checkSESLambda discovers Lambda functions invoked by SES v1 inbound receipt
// rules that apply to the given identity. Calls ses:DescribeActiveReceiptRuleSet
// (SES v1) once per fetch batch, filters by Recipients scoping, then walks
// Rules[].Actions[].LambdaAction.FunctionArn and extracts the function name
// (the segment after ":function:") so the returned IDs match the lambda
// resource type's ID format (function names, not ARNs). Returns Count: 0 for
// accounts with no active receipt rule set (pure outbound SES) — operator-honest
// absence.
func checkSESLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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
	var filtered []sestypes.ReceiptRule
	for _, rule := range out.Rules {
		if sesRuleAppliesToIdentity(rule, res.ID, res.Fields["identity_type"]) {
			filtered = append(filtered, rule)
		}
	}
	names := sesLambdaNamesFromRules(filtered)
	return relatedResult("lambda", names)
}

// checkSESS3 discovers S3 buckets where SES v1 inbound receipt rules deposit
// received mail for the given identity. Calls ses:DescribeActiveReceiptRuleSet
// (SES v1) once per fetch batch, filters by Recipients scoping, then walks
// Rules[].Actions[].S3Action.BucketName. Returns Count: 0 for accounts with no
// active receipt rule set — operator-honest absence.
func checkSESS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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
	var filtered []sestypes.ReceiptRule
	for _, rule := range out.Rules {
		if sesRuleAppliesToIdentity(rule, res.ID, res.Fields["identity_type"]) {
			filtered = append(filtered, rule)
		}
	}
	buckets := sesS3BucketsFromRules(filtered)
	return relatedResult("s3", buckets)
}

// sesLambdaNamesFromRules walks ReceiptRule actions, extracts the function name
// from each LambdaAction.FunctionArn, and returns deduplicated function names.
// Function names (not ARNs) are returned so they match the lambda resource
// type's ID format.
func sesLambdaNamesFromRules(rules []sestypes.ReceiptRule) []string {
	seen := map[string]struct{}{}
	var names []string
	for _, rule := range rules {
		for _, action := range rule.Actions {
			if action.LambdaAction == nil || action.LambdaAction.FunctionArn == nil {
				continue
			}
			arn := *action.LambdaAction.FunctionArn
			if arn == "" {
				continue
			}
			name := lambdaARNToName(arn)
			if name == "" {
				continue
			}
			if _, exists := seen[name]; !exists {
				seen[name] = struct{}{}
				names = append(names, name)
			}
		}
	}
	return names
}

// lambdaARNToName extracts the function name from a Lambda function ARN.
// Returns "" for unparseable input. Handles version/alias suffix by taking
// only the segment after "function:".
// ARN format: arn:aws:lambda:REGION:ACCOUNT:function:FUNCTION_NAME[:VERSION_OR_ALIAS]
func lambdaARNToName(arn string) string {
	const marker = ":function:"
	_, tail, found := strings.Cut(arn, marker)
	if !found {
		return ""
	}
	// Strip version/alias suffix (":v1" or ":$LATEST" or ":alias")
	if colon := strings.Index(tail, ":"); colon >= 0 {
		tail = tail[:colon]
	}
	return tail
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

// truncatedResultSES returns a RelatedCheckResult with Approximate=true when the
// target cache is truncated and matches were found. Later pages may contain
// additional matches, so the displayed count is a lower bound — rendered as "(N+)".
func truncatedResultSES(target string, ids []string) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: target, Count: len(ids), ResourceIDs: ids, Approximate: true}
}

// InvalidateSESRuleSetCache drops the cached DescribeActiveReceiptRuleSet
// response for the given client. Called from the Ctrl+R handler so that
// receipt-rule changes are picked up without waiting for a profile/region
// switch to rebuild *ServiceClients.
func InvalidateSESRuleSetCache(c *ServiceClients) {
	if c == nil {
		return
	}
	sesRuleSetCacheMu.Lock()
	delete(sesRuleSetCaches, c)
	sesRuleSetCacheMu.Unlock()
}

// ClearAllSESRuleSetCaches drops every cached receipt rule set across all
// *ServiceClients keys. Called from the profile/region-switch handler so
// stale session state cannot leak across reconnects.
func ClearAllSESRuleSetCaches() {
	sesRuleSetCacheMu.Lock()
	sesRuleSetCaches = make(map[*ServiceClients]*sesReceiptRuleSetCache)
	sesRuleSetCacheMu.Unlock()
}

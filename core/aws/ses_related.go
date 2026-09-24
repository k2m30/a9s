// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ses_related.go contains SES related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"

	"github.com/k2m30/a9s/v3/core/resource"
)

// ruleSetStore is the unexported shape core/aws expects from a
// per-Session SES rule set cache. session.RuleSetStore satisfies this via
// duck-typing — core/aws cannot import core/session without a cycle,
// so the local interface mirrors the methods needed here.
type ruleSetStore interface {
	Get() (any, bool)
	Set(any)
	Clear()
	GetOrFetch(context.Context, func(context.Context) (any, error)) (any, error)
}

// checkSESR53 finds the public hosted zone the SES identity's domain is
// written in (publicZoneHolding): the domain of an address identity, or the
// domain identity itself.
func checkSESR53(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	domain := canonicalDNS(sesIdentityDomain(res))
	if domain == "" {
		return foundNone("r53", "domain")
	}

	r53List, truncated, err := relatedResourcesFor(ctx, clients, cache, "r53")
	if err != nil {
		return ReadFailed("r53", err)
	}
	if r53List == nil {
		return NotRead("r53")
	}

	ids := publicZoneHolding(r53List, domain)
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("r53", nil, true)
	}
	if truncated {
		return truncatedResultSES("r53", ids)
	}
	return relatedResultTrunc("r53", ids, false)
}

// sesIdentityDomain extracts the domain from a SES identity resource.
// For EMAIL_ADDRESS identities (containing "@"), it returns the part after "@".
// For DOMAIN identities, it returns the identity name directly.
func sesIdentityDomain(res resource.Resource) string {
	if domain, ok := emailDomain(res.ID); ok {
		return domain
	}
	return res.ID
}

// sesConfigSetName resolves the ConfigurationSetName for the given SES identity by
// calling sesv2:GetEmailIdentity. Returns ("", nil) when the call succeeded and
// no configuration set is attached — a proven absence. Returns ("", err) when
// the client/interface is unavailable or the call failed, so callers can tell
// "genuinely nothing configured" apart from "could not determine".
func sesConfigSetName(ctx context.Context, c *ServiceClients, identityName string) (string, error) {
	if c == nil || c.SESv2 == nil || identityName == "" {
		return "", errClientMissing
	}
	api, ok := c.SESv2.(SESv2GetEmailIdentityAPI)
	if !ok {
		return "", errClientMissing
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sesv2.GetEmailIdentityOutput, error) {
		return api.GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{EmailIdentity: &identityName})
	})
	if err != nil {
		return "", err
	}
	if out == nil || out.ConfigurationSetName == nil {
		return "", nil
	}
	return *out.ConfigurationSetName, nil
}

// sesEventDestinations calls sesv2:GetConfigurationSetEventDestinations for the
// given configuration set name and keeps the enabled destinations: a disabled
// one sends no events
// (https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_EventDestination.html).
// Returns nil on error or missing config set.
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
	if out == nil {
		return nil, err
	}
	enabled := *out
	enabled.EventDestinations = nil
	for _, d := range out.EventDestinations {
		if d.Enabled {
			enabled.EventDestinations = append(enabled.EventDestinations, d)
		}
	}
	return &enabled, err
}

// sesActiveReceiptRuleSet returns the active SES v1 receipt rule set for this
// session. The store is per-session; singleflight coalesces concurrent misses
// into one upstream call. Swap (not Clear) is required on Ctrl+R refresh so
// late-completing fetches land on the orphaned old store, never the new one.
func sesActiveReceiptRuleSet(ctx context.Context, c *ServiceClients) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	if c == nil || c.SES == nil {
		return nil, nil //nolint:nilnil // no SES v1 client; caller treats nil as "no rule set"
	}
	store := c.RuleSets()
	if store == nil {
		// No store wired (test path or pre-init); fall back to direct fetch
		// without caching or coalescing.
		return c.SES.DescribeActiveReceiptRuleSet(ctx, &ses.DescribeActiveReceiptRuleSetInput{})
	}
	v, err := store.GetOrFetch(ctx, func(fetchCtx context.Context) (any, error) {
		return c.SES.DescribeActiveReceiptRuleSet(fetchCtx, &ses.DescribeActiveReceiptRuleSetInput{})
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		// (nil, nil) is the documented sentinel for "no active SES receipt
		// rule set configured" — absence is a valid state, not an error.
		return nil, nil //nolint:nilnil // intentional absence sentinel (see comment above)
	}
	out, ok := v.(*ses.DescribeActiveReceiptRuleSetOutput)
	if !ok {
		return nil, nil //nolint:nilnil // type mismatch; treat as "no rule set"
	}
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
		return keyMissing("eb-rule", "identityName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		// Without a client we cannot query SESv2 to discover EventBridge bus names.
		return NotRead("eb-rule")
	}
	configSetName, csErr := sesConfigSetName(ctx, c, identityName)
	if csErr != nil {
		return ReadFailed("eb-rule", csErr)
	}
	if configSetName == "" {
		// GetEmailIdentity succeeded and confirmed no configuration set — proven zero.
		return foundNone("eb-rule", "configSetName")
	}
	out, err := sesEventDestinations(ctx, c, configSetName)
	if err != nil {
		return ReadFailed("eb-rule", err)
	}
	if out == nil {
		// sesEventDestinations' (nil, nil) sentinel means the interface was
		// not satisfied — we could not attempt the call, not a proven zero.
		return NotRead("eb-rule")
	}

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
		return foundNone("eb-rule", "busNames")
	}

	ebRules, truncated, cacheErr := relatedResourcesFor(ctx, clients, cache, "eb-rule")
	if cacheErr != nil {
		return ReadFailed("eb-rule", cacheErr)
	}
	if ebRules == nil {
		return NotRead("eb-rule")
	}

	var ids []string
	for _, rule := range ebRules {
		if _, ok := busNames[rule.Fields["event_bus"]]; ok {
			ids = append(ids, rule.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("eb-rule", nil, true)
	}
	if truncated {
		return truncatedResultSES("eb-rule", ids)
	}
	return relatedResultTrunc("eb-rule", ids, false)
}

// sesActiveRulesFor is the receipt rules that process the identity's mail:
// the active ones ("If true, the receipt rule is active",
// https://docs.aws.amazon.com/ses/latest/APIReference/API_ReceiptRule.html)
// that apply to it.
func sesActiveRulesFor(rules []sestypes.ReceiptRule, identityName string) []sestypes.ReceiptRule {
	var out []sestypes.ReceiptRule
	for _, rule := range rules {
		if rule.Enabled && sesRuleAppliesToIdentity(rule, identityName) {
			out = append(out, rule)
		}
	}
	return out
}

// sesRuleAppliesToIdentity reports whether a receipt rule should be considered
// when computing related resources for the given SES identity: a rule with no
// recipient condition applies to every verified domain, and one with
// conditions applies when one of them matches (sesRecipientMatches).
//
// Empty-string recipient entries are skipped conservatively. If every entry
// is an empty string (rare data corruption), the rule is treated as applying
// to all (AWS normalises absent Recipients to nil; a non-nil all-empty slice
// is anomalous).
func sesRuleAppliesToIdentity(rule sestypes.ReceiptRule, identityName string) bool {
	if len(rule.Recipients) == 0 {
		return true
	}

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

	return slices.ContainsFunc(valid, func(r string) bool { return sesRecipientMatches(r, identityName) })
}

// checkSESLambda discovers Lambda functions invoked by SES v1 inbound receipt
// rules that apply to the given identity. Calls ses:DescribeActiveReceiptRuleSet
// (SES v1) once per fetch batch, filters by Recipients scoping, then walks
// Rules[].Actions[].LambdaAction.FunctionArn. Returns Count: 0 for
// accounts with no active receipt rule set (pure outbound SES) — operator-honest
// absence.
func checkSESLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("lambda")
	}
	out, err := sesActiveReceiptRuleSet(ctx, c)
	if err != nil {
		return ReadFailed("lambda", err)
	}
	if out == nil {
		// No active rule set — pure outbound account. Operator-honest 0.
		return foundNone("lambda", "out")
	}
	filtered := sesActiveRulesFor(out.Rules, res.ID)
	return relatedRefs("lambda", sesLambdaARNsFromRules(filtered), refContext(clients, cache, "lambda"))
}

// checkSESS3 discovers S3 buckets where SES v1 inbound receipt rules deposit
// received mail for the given identity. Calls ses:DescribeActiveReceiptRuleSet
// (SES v1) once per fetch batch, filters by Recipients scoping, then walks
// Rules[].Actions[].S3Action.BucketName. Returns Count: 0 for accounts with no
// active receipt rule set — operator-honest absence.
func checkSESS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("s3")
	}
	out, err := sesActiveReceiptRuleSet(ctx, c)
	if err != nil {
		return ReadFailed("s3", err)
	}
	if out == nil {
		// No active rule set — pure outbound account. Operator-honest 0.
		return foundNone("s3", "out")
	}
	filtered := sesActiveRulesFor(out.Rules, res.ID)
	buckets := sesS3BucketsFromRules(filtered)
	return relatedResultTrunc("s3", buckets, false)
}

// sesLambdaARNsFromRules walks ReceiptRule actions and collects
// LambdaAction.FunctionArn values.
func sesLambdaARNsFromRules(rules []sestypes.ReceiptRule) []string {
	var arns []string
	for _, rule := range rules {
		for _, action := range rule.Actions {
			if action.LambdaAction != nil && action.LambdaAction.FunctionArn != nil && *action.LambdaAction.FunctionArn != "" {
				arns = append(arns, *action.LambdaAction.FunctionArn)
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
		return keyMissing("sns", "identityName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("sns")
	}
	configSetName, csErr := sesConfigSetName(ctx, c, identityName)
	if csErr != nil {
		return ReadFailed("sns", csErr)
	}
	if configSetName == "" {
		// GetEmailIdentity succeeded and confirmed no configuration set — proven zero.
		return foundNone("sns", "configSetName")
	}
	out, err := sesEventDestinations(ctx, c, configSetName)
	if err != nil {
		return ReadFailed("sns", err)
	}
	if out == nil {
		// sesEventDestinations' (nil, nil) sentinel means the interface was
		// not satisfied — we could not attempt the call, not a proven zero.
		return NotRead("sns")
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
	return relatedResultTrunc("sns", ids, false)
}

// truncatedResultSES returns a RelatedCheckResult with Truncated=true when the
// target cache is truncated and matches were found. Later pages may contain
// additional matches, so the displayed count is a lower bound — rendered as "(N+)".
func truncatedResultSES(target string, ids []string) resource.RelatedCheckResult {
	return relatedResultTrunc(target, ids, true)
}

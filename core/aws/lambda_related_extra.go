// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lambda_related_extra.go contains additional Lambda related-resource
// checkers required by docs/related-resources.md beyond the core set in
// lambda_related.go. Most are reverse-cache-scans (target cache → Lambda ARN
// match) or live API calls via ListEventSourceMappings for stream-based
// triggers.
package aws

import (
	"context"
	"strings"

	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// --- Forward Pattern F (data already in RawStruct) ---

// checkLambdaSubnet extracts subnet IDs from Lambda VpcConfig.SubnetIds.
func checkLambdaSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if fn.VpcConfig == nil {
		return resource.KnownRelated("subnet", nil, false)
	}
	var ids []string
	for _, s := range fn.VpcConfig.SubnetIds {
		if s != "" {
			ids = append(ids, s)
		}
	}
	return relatedResult("subnet", ids)
}

// checkLambdaEFS extracts EFS access-point ARNs from Lambda FileSystemConfigs
// and returns the filesystem IDs. The Arn points to an access point
// (arn:aws:elasticfilesystem:region:account:access-point/fsap-xxx) which
// itself references a filesystem; without a live efs:DescribeAccessPoints
// call we cannot resolve fsap→fs-id, so we return the access-point IDs here.
// Downstream routing can translate fsap→fs via the access-point cache if it
// exists; otherwise these IDs at least surface the link.
func checkLambdaEFS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("efs")
	}
	var ids []string
	for _, cfg := range fn.FileSystemConfigs {
		if cfg.Arn == nil || *cfg.Arn == "" {
			continue
		}
		arn := *cfg.Arn
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			ids = append(ids, arn[idx+1:])
		}
	}
	if len(ids) == 0 {
		return resource.KnownRelated("efs", nil, false)
	}
	return relatedResult("efs", ids)
}

// --- Reverse cache scans (target cache references this Lambda) ---

// checkLambdaAPIGW scans the apigw cache for HTTP/REST APIs that integrate
// with this Lambda function. apigatewayv2.Api struct does not embed
// integrations, so without a GetIntegrations API call this is undeterminable.
// We truncated by searching for the function name in the api's Name or
// Tags — a weak signal, but better than Count:0 when a real match exists.
func checkLambdaAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.KnownRelated("apigw", nil, false)
	}
	apiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "apigw")
	if err != nil {
		return resource.ErrorRelated("apigw", err)
	}
	if apiList == nil {
		return resource.UnknownRelated("apigw")
	}
	var ids []string
	for _, apiRes := range apiList {
		api, ok := assertStruct[apigwtypes.Api](apiRes.RawStruct)
		if !ok {
			continue
		}
		if api.Tags != nil {
			if v, ok := api.Tags[fnName]; ok && v != "" {
				ids = append(ids, apiRes.ID)
				continue
			}
		}
		if api.Name != nil && strings.Contains(*api.Name, fnName) {
			ids = append(ids, apiRes.ID)
		}
	}
	return relatedResultTrunc("apigw", ids, truncated)
}

// checkLambdaCF scans the cloudfront cache for Lambda@Edge distributions that
// associate this Lambda function. The cf fetcher joins both
// DefaultCacheBehavior and CacheBehaviors[] LambdaFunctionAssociations at
// fetch time (zero extra calls) into the comma-joined
// Fields["lambda_function_arns"]. Lambda@Edge associations always reference a
// specific published VERSION (never $LATEST), so matching requires an
// unversioned-ARN-prefix comparison against this function's base ARN.
func checkLambdaCF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnARN := ""
	if fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct); ok && fn.FunctionArn != nil {
		fnARN = *fn.FunctionArn
	}
	if fnARN == "" {
		return unreadZero(res, resource.KnownRelated("cf", nil, false))
	}
	cfList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cf")
	if err != nil {
		return resource.ErrorRelated("cf", err)
	}
	if cfList == nil {
		return resource.UnknownRelated("cf")
	}
	wantPrefix := fnARN + ":"
	var ids []string
	for _, cfRes := range cfList {
		joined := cfRes.Fields["lambda_function_arns"]
		if joined == "" {
			continue
		}
		for assocARN := range strings.SplitSeq(joined, ",") {
			if strings.HasPrefix(assocARN, wantPrefix) {
				ids = append(ids, cfRes.ID)
				break
			}
		}
	}
	return unreadZeroScanned(res, len(cfList), relatedResultTrunc("cf", ids, truncated))
}

// checkLambdaDDB scans this Lambda's event source mappings for DynamoDB
// stream ARNs and returns the table names (last path segment after "table/").
// Requires live Lambda client for ListEventSourceMappings.
func checkLambdaDDB(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.KnownRelated("ddb", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("ddb")
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		FunctionName: &fnName,
	})
	if err != nil {
		return resource.ErrorRelated("ddb", err)
	}
	seen := make(map[string]struct{})
	for _, m := range out.EventSourceMappings {
		if m.EventSourceArn == nil {
			continue
		}
		arn := *m.EventSourceArn
		if !strings.Contains(arn, ":dynamodb:") {
			continue
		}
		// ARN form: arn:aws:dynamodb:region:account:table/NAME/stream/TIMESTAMP
		_, rest, ok := strings.Cut(arn, "table/")
		if !ok {
			continue
		}
		if before, _, hasSep := strings.Cut(rest, "/"); hasSep {
			rest = before
		}
		if rest != "" {
			seen[rest] = struct{}{}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	return relatedResult("ddb", ids)
}

// checkLambdaKinesis scans this Lambda's event source mappings for Kinesis
// stream ARNs. Pattern A — live API.
func checkLambdaKinesis(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.KnownRelated("kinesis", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("kinesis")
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		FunctionName: &fnName,
	})
	if err != nil {
		return resource.ErrorRelated("kinesis", err)
	}
	seen := make(map[string]struct{})
	for _, m := range out.EventSourceMappings {
		if m.EventSourceArn == nil {
			continue
		}
		arn := *m.EventSourceArn
		if !strings.Contains(arn, ":kinesis:") {
			continue
		}
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			seen[arn[idx+1:]] = struct{}{}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	return relatedResult("kinesis", ids)
}

// checkLambdaMSK scans this Lambda's event source mappings for MSK cluster
// ARNs. Pattern A — live API.
func checkLambdaMSK(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.KnownRelated("msk", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("msk")
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		FunctionName: &fnName,
	})
	if err != nil {
		return resource.ErrorRelated("msk", err)
	}
	seen := make(map[string]struct{})
	for _, m := range out.EventSourceMappings {
		if m.EventSourceArn == nil {
			continue
		}
		arn := *m.EventSourceArn
		if !strings.Contains(arn, ":kafka:") {
			continue
		}
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			seen[arn[idx+1:]] = struct{}{}
		} else {
			seen[arn] = struct{}{}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	return relatedResult("msk", ids)
}

// checkLambdaCTEvents scans the ct-events cache for events whose Resources
// include this Lambda function (Pattern C — reverse).
func checkLambdaCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	evList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.ErrorRelated("ct-events", err)
	}
	if evList == nil {
		return resource.UnknownRelated("ct-events")
	}
	var ids []string
	for _, evRes := range evList {
		ev, ok := assertStruct[cloudtrailtypes.Event](evRes.RawStruct)
		if !ok {
			continue
		}
		for _, r := range ev.Resources {
			if r.ResourceName == nil {
				continue
			}
			if *r.ResourceName == fnName || strings.HasSuffix(*r.ResourceName, ":function:"+fnName) {
				ids = append(ids, evRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("ct-events", ids, truncated)
}

// --- Reverse lookups that require fields not in the cached struct ---
//
// The following checkers reverse-look the target cache for references back to
// this Lambda. When the target cache's struct doesn't carry the reference
// (e.g. ELB target groups list, SNS subscription list entries), the checker
// needs additional API calls per candidate (N+1) to resolve the link. We
// return an unknown result when no live client is available and otherwise do
// a bounded scan.

// checkLambdaTG scans the tg cache for target groups whose TargetType is
// "lambda" and, for each such TG, calls ELBv2 DescribeTargetHealth to resolve
// its registered targets — Target.Id for a Lambda-type TG is the function
// ARN. Only lambda-type TGs trigger a call, bounding the fan-out to the
// number of Lambda TGs in the account (typically small). Per-TG failures are
// aggregated per the error contract rather than silently skipped.
func checkLambdaTG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnARN := ""
	if fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct); ok && fn.FunctionArn != nil {
		fnARN = *fn.FunctionArn
	}
	fnName := res.ID
	if fnARN == "" && fnName == "" {
		return resource.KnownRelated("tg", nil, false)
	}
	tgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "tg")
	if err != nil {
		return resource.ErrorRelated("tg", err)
	}
	if tgList == nil {
		return resource.UnknownRelated("tg")
	}

	var lambdaTGs []resource.Resource
	for _, tgRes := range tgList {
		if tgRes.Fields["target_type"] == "lambda" {
			lambdaTGs = append(lambdaTGs, tgRes)
		}
	}
	if len(lambdaTGs) == 0 {
		if truncated {
			return relatedResultTrunc("tg", nil, true)
		}
		return resource.KnownRelated("tg", nil, false)
	}

	c, sok := clients.(*ServiceClients)
	if !sok || c == nil || c.ELBv2 == nil {
		return resource.UnknownRelated("tg")
	}
	healthAPI, hok := c.ELBv2.(ELBv2DescribeTargetHealthAPI)
	if !hok {
		return resource.UnknownRelated("tg")
	}

	var ids []string
	var failures []Failure
	for _, tgRes := range lambdaTGs {
		tgArn := tgRes.Fields["target_group_arn"]
		if tgArn == "" {
			if tg, ok := assertStruct[elbv2types.TargetGroup](tgRes.RawStruct); ok && tg.TargetGroupArn != nil {
				tgArn = *tg.TargetGroupArn
			}
		}
		if tgArn == "" {
			continue
		}
		out, healthErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*elbv2.DescribeTargetHealthOutput, error) {
			return healthAPI.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{TargetGroupArn: &tgArn})
		})
		if healthErr != nil {
			failures = append(failures, FailedCall(tgRes.ID, healthErr))
			continue
		}
		for _, thd := range out.TargetHealthDescriptions {
			if thd.Target == nil || thd.Target.Id == nil {
				continue
			}
			targetID := *thd.Target.Id
			if (fnARN != "" && targetID == fnARN) ||
				(fnName != "" && strings.HasSuffix(targetID, ":function:"+fnName)) {
				ids = append(ids, tgRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && !truncated {
		// Nothing was confirmed and the tg cache page was complete: any
		// failures here are a plain fetch failure, not a truncation signal.
		if aggErr := AggregateFailures("lambda-related: DescribeTargetHealth", failures, len(lambdaTGs)); aggErr != nil {
			return resource.ErrorRelated("tg", aggErr)
		}
	}
	// Some DescribeTargetHealth calls may have failed: ids is a proven subset,
	// not necessarily exhaustive. Truncated (not Errored) keeps the row
	// actionable rather than discarding confirmed matches as a dead end.
	return relatedResultTrunc("tg", ids, truncated || len(failures) > 0)
}

// checkLambdaSNS scans the sns cache and surfaces topics that subscribe this
// Lambda (i.e. where this function is a subscription endpoint).
// DescribeTopic alone doesn't list subscriptions, so we check the sns-sub cache.
func checkLambdaSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnARN := ""
	if fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct); ok && fn.FunctionArn != nil {
		fnARN = *fn.FunctionArn
	}
	fnName := res.ID
	if fnARN == "" && fnName == "" {
		return resource.KnownRelated("sns", nil, false)
	}
	subList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sns-sub")
	if err != nil {
		return resource.ErrorRelated("sns", err)
	}
	if subList == nil {
		return resource.UnknownRelated("sns")
	}
	topicSet := make(map[string]struct{})
	for _, subRes := range subList {
		if subRes.Fields["protocol"] != "lambda" {
			continue
		}
		endpoint := subRes.Fields["endpoint"]
		if endpoint == "" {
			continue
		}
		if (fnARN != "" && endpoint == fnARN) ||
			(fnName != "" && strings.HasSuffix(endpoint, ":function:"+fnName)) {
			if topic := subRes.Fields["topic_arn"]; topic != "" {
				topicSet[topic] = struct{}{}
			}
		}
	}
	var ids []string
	for t := range topicSet {
		ids = append(ids, t)
	}
	return relatedResultTrunc("sns", ids, truncated)
}

// checkLambdaSNSSub scans the sns-sub cache for subscriptions where this
// Lambda is the endpoint (Pattern C — reverse).
func checkLambdaSNSSub(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnARN := ""
	if fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct); ok && fn.FunctionArn != nil {
		fnARN = *fn.FunctionArn
	}
	fnName := res.ID
	if fnARN == "" && fnName == "" {
		return resource.KnownRelated("sns-sub", nil, false)
	}
	subList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sns-sub")
	if err != nil {
		return resource.ErrorRelated("sns-sub", err)
	}
	if subList == nil {
		return resource.UnknownRelated("sns-sub")
	}
	var ids []string
	for _, subRes := range subList {
		if subRes.Fields["protocol"] != "lambda" {
			continue
		}
		endpoint := subRes.Fields["endpoint"]
		if (fnARN != "" && endpoint == fnARN) ||
			(fnName != "" && strings.HasSuffix(endpoint, ":function:"+fnName)) {
			ids = append(ids, subRes.ID)
		}
	}
	return relatedResultTrunc("sns-sub", ids, truncated)
}

// checkLambdaS3 scans the s3 cache for buckets that have a notification
// target equal to this Lambda. The bucket cache entry populates
// Fields["notification_lambda"] if the fetcher enriched it.
func checkLambdaS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnARN := ""
	if fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct); ok && fn.FunctionArn != nil {
		fnARN = *fn.FunctionArn
	}
	fnName := res.ID
	if fnARN == "" && fnName == "" {
		return resource.KnownRelated("s3", nil, false)
	}
	s3List, truncated, err := relatedResourcesFor(ctx, clients, cache, "s3")
	if err != nil {
		return resource.ErrorRelated("s3", err)
	}
	if s3List == nil {
		return resource.UnknownRelated("s3")
	}
	var ids []string
	for _, bRes := range s3List {
		n := bRes.Fields["notification_lambda"]
		if n == "" {
			continue
		}
		if (fnARN != "" && n == fnARN) ||
			(fnName != "" && strings.HasSuffix(n, ":function:"+fnName)) {
			ids = append(ids, bRes.ID)
		}
	}
	return relatedResultTrunc("s3", ids, truncated)
}

// checkLambdaENI scans the eni cache for ENIs attached to this Lambda's
// hyperplane (VPC-attached functions get AWS-managed requester-managed ENIs).
// Per docs/resources/lambda.md, the match is RequesterId=="AWS Lambda VPC
// ENI" or Description starting with "AWS Lambda VPC ENI-<FunctionName>-".
func checkLambdaENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.KnownRelated("eni", nil, false)
	}
	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return resource.ErrorRelated("eni", err)
	}
	if eniList == nil {
		return resource.UnknownRelated("eni")
	}
	// RequesterId=="AWS Lambda VPC ENI" identifies the ENI as Lambda-managed;
	// the Description prefix is what disambiguates which function it
	// belongs to, since RequesterId alone is identical across every
	// VPC-attached function's ENIs.
	wantPrefix := "AWS Lambda VPC ENI-" + fnName + "-"
	var ids []string
	for _, eniRes := range eniList {
		if eniRes.Fields["requester_id"] != "AWS Lambda VPC ENI" {
			continue
		}
		if !strings.HasPrefix(eniRes.Fields["description"], wantPrefix) {
			continue
		}
		ids = append(ids, eniRes.ID)
	}
	return relatedResultTrunc("eni", ids, truncated)
}

// checkLambdaSecrets scans this Lambda's environment-variable values for
// references to secrets-manager secret ARNs. FunctionConfiguration.Environment.Variables
// is a map[string]string; we search values for a Secrets Manager ARN.
func checkLambdaSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("secrets")
		}
		return resource.KnownRelated("secrets", nil, false)
	}
	if fn.Environment == nil || len(fn.Environment.Variables) == 0 {
		return resource.KnownRelated("secrets", nil, false)
	}
	arnSet := make(map[string]struct{})
	for _, v := range fn.Environment.Variables {
		if _, isSecret := ARNForService(v, "secretsmanager"); isSecret {
			arnSet[v] = struct{}{}
		}
	}
	if len(arnSet) == 0 {
		return resource.KnownRelated("secrets", nil, false)
	}
	secretList, truncated, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.ErrorRelated("secrets", err)
	}
	if secretList == nil {
		return resource.UnknownRelated("secrets")
	}
	var ids []string
	for _, sRes := range secretList {
		if _, match := arnSet[sRes.ID]; match {
			ids = append(ids, sRes.ID)
			continue
		}
		if arn := sRes.Fields["arn"]; arn != "" {
			if _, match := arnSet[arn]; match {
				ids = append(ids, sRes.ID)
			}
		}
	}
	return relatedResultTrunc("secrets", ids, truncated)
}

// checkLambdaSSM scans Lambda's environment-variable values for SSM parameter
// name references. Values matching "/" or starting with "/aws/..." that also
// exist in the ssm cache (Parameter Store) are returned.
func checkLambdaSSM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("ssm")
		}
		return resource.KnownRelated("ssm", nil, false)
	}
	if fn.Environment == nil || len(fn.Environment.Variables) == 0 {
		return resource.KnownRelated("ssm", nil, false)
	}
	candidates := make(map[string]struct{})
	for _, v := range fn.Environment.Variables {
		if strings.HasPrefix(v, "/") {
			candidates[v] = struct{}{}
		}
	}
	if len(candidates) == 0 {
		return resource.KnownRelated("ssm", nil, false)
	}
	ssmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ssm")
	if err != nil {
		return resource.ErrorRelated("ssm", err)
	}
	if ssmList == nil {
		return resource.UnknownRelated("ssm")
	}
	var ids []string
	for _, pRes := range ssmList {
		if _, match := candidates[pRes.ID]; match {
			ids = append(ids, pRes.ID)
			continue
		}
		if _, match := candidates[pRes.Name]; match {
			ids = append(ids, pRes.ID)
		}
	}
	return relatedResultTrunc("ssm", ids, truncated)
}

// Ensure cwtypes stays imported for future alarm-related extensions.
var _ = cwtypes.MetricAlarm{}

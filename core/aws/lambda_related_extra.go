// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// lambda_related_extra.go contains additional Lambda related-resource
// checkers required by docs/related-resources.md beyond the core set in
// lambda_related.go. Most are reverse-cache-scans (target cache → Lambda ARN
// match) or live API calls via ListEventSourceMappings for stream-based
// triggers.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkLambdaSubnet extracts subnet IDs from Lambda VpcConfig.SubnetIds.
func checkLambdaSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	if fn.VpcConfig == nil {
		return foundNone("subnet", "fn.VpcConfig")
	}
	var ids []string
	for _, s := range fn.VpcConfig.SubnetIds {
		if s != "" {
			ids = append(ids, s)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkLambdaEFS reads the file systems behind Lambda FileSystemConfigs: each
// Arn names an access point, which the efs resolver maps to its file system.
func checkLambdaEFS(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return NotRead("efs")
	}
	var refs []string
	for _, cfg := range fn.FileSystemConfigs {
		if cfg.Arn != nil {
			refs = append(refs, *cfg.Arn)
		}
	}
	return relatedRefs("efs", refs, refContext(clients, cache, "efs"))
}

// checkLambdaAPIGW reads which APIs invoke this function from their
// integrations: an HTTP or WebSocket API invokes a function through an
// integration whose IntegrationUri is the function's ARN, and only
// apigatewayv2:GetIntegrations per API returns them. A REST API keeps its
// integrations per method, which this pivot does not read, so a REST API
// leaves the count a lower bound.
func checkLambdaAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return foundNone("apigw", "fnName")
	}
	apiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "apigw")
	if err != nil {
		return ReadFailed("apigw", err)
	}
	if apiList == nil {
		return NotRead("apigw")
	}
	rc := refContext(clients, cache, "lambda")
	var ids []string
	var reads rowReads
	for _, apiRes := range apiList {
		if apiRes.Fields["protocol"] == "REST" {
			reads.missed()
			continue
		}
		items, complete, err := apigwListIntegrations(ctx, clients, apiRes.ID)
		if err != nil {
			reads.fail(apiRes.ID, err)
			continue
		}
		reads.read++
		truncated = truncated || !complete
		if slices.ContainsFunc(items, func(item apigwtypes.Integration) bool {
			return lambdaRefNamesFunction(lambdaIntegrationARN(aws.ToString(item.IntegrationUri)), res.ID, rc)
		}) {
			ids = append(ids, apiRes.ID)
		}
	}
	return reads.answer("apigw", "lambda-related: GetIntegrations", ids, truncated)
}

// checkLambdaCF scans the cloudfront cache for Lambda@Edge distributions that
// associate this Lambda function. The cf fetcher joins both
// DefaultCacheBehavior and CacheBehaviors[] LambdaFunctionAssociations at
// fetch time (zero extra calls) into the comma-joined
// Fields["lambda_function_arns"]. Lambda@Edge associations always reference a
// specific published VERSION (never $LATEST), which the lambda resolver reads
// back to the function the version belongs to.
func checkLambdaCF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnARN := ""
	if fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct); ok && fn.FunctionArn != nil {
		fnARN = *fn.FunctionArn
	}
	if fnARN == "" {
		return unreadZero(res, foundNone("cf", "fnARN"))
	}
	cfList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cf")
	if err != nil {
		return ReadFailed("cf", err)
	}
	if cfList == nil {
		return NotRead("cf")
	}
	rc := refContext(clients, cache, "lambda")
	var ids []string
	for _, cfRes := range cfList {
		joined := cfRes.Fields["lambda_function_arns"]
		if joined == "" {
			continue
		}
		for assocARN := range strings.SplitSeq(joined, ",") {
			if lambdaRefNamesFunction(assocARN, res.ID, rc) {
				ids = append(ids, cfRes.ID)
				break
			}
		}
	}
	return unreadZeroScanned(res, len(cfList), relatedResultTrunc("cf", ids, truncated))
}

// checkLambdaDDB scans this Lambda's event source mappings for DynamoDB
// stream ARNs, read through the ddb resolver.
// Requires live Lambda client for ListEventSourceMappings.
func checkLambdaDDB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return foundNone("ddb", "fnName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return NotRead("ddb")
	}
	return lambdaEventSourceRefs(ctx, c.Lambda, fnName, "ddb", "dynamodb", refContext(clients, cache, "ddb"))
}

// checkLambdaKinesis scans this Lambda's event source mappings for Kinesis
// stream ARNs.
func checkLambdaKinesis(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return foundNone("kinesis", "fnName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return NotRead("kinesis")
	}
	return lambdaEventSourceRefs(ctx, c.Lambda, fnName, "kinesis", "kinesis", refContext(clients, cache, "kinesis"))
}

// checkLambdaMSK scans this Lambda's event source mappings for MSK cluster
// ARNs.
func checkLambdaMSK(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return foundNone("msk", "fnName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return NotRead("msk")
	}
	return lambdaEventSourceRefs(ctx, c.Lambda, fnName, "msk", "kafka", refContext(clients, cache, "msk"))
}

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
	if res.ID == "" {
		return foundNone("tg", "res.ID")
	}
	rc := refContext(clients, cache, "lambda")
	tgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "tg")
	if err != nil {
		return ReadFailed("tg", err)
	}
	if tgList == nil {
		return NotRead("tg")
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
		return foundNone("tg", "lambdaTGs")
	}

	c, sok := clients.(*ServiceClients)
	if !sok || c == nil || c.ELBv2 == nil {
		return NotRead("tg")
	}
	healthAPI, hok := c.ELBv2.(ELBv2DescribeTargetHealthAPI)
	if !hok {
		return NotRead("tg")
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
			if lambdaRefNamesFunction(*thd.Target.Id, res.ID, rc) {
				ids = append(ids, tgRes.ID)
				break
			}
		}
	}
	if aggErr := AggregateFailures("lambda-related: DescribeTargetHealth", failures, len(lambdaTGs)); aggErr != nil &&
		len(ids) == 0 && !truncated && len(failures) == len(lambdaTGs) {
		// Every target group refused its read and the tg cache page was
		// complete: nothing was established about any of them, which is a
		// fetch failure rather than a lower bound over what was read.
		return ReadFailed("tg", aggErr)
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
	if res.ID == "" {
		return foundNone("sns", "res.ID")
	}
	rc := refContext(clients, cache, "lambda")
	subList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sns-sub")
	if err != nil {
		return ReadFailed("sns", err)
	}
	if subList == nil {
		return NotRead("sns")
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
		if lambdaRefNamesFunction(endpoint, res.ID, rc) {
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
// Lambda is the endpoint.
func checkLambdaSNSSub(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return foundNone("sns-sub", "res.ID")
	}
	rc := refContext(clients, cache, "lambda")
	subList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sns-sub")
	if err != nil {
		return ReadFailed("sns-sub", err)
	}
	if subList == nil {
		return NotRead("sns-sub")
	}
	var ids []string
	for _, subRes := range subList {
		if subRes.Fields["protocol"] != "lambda" {
			continue
		}
		if lambdaRefNamesFunction(subRes.Fields["endpoint"], res.ID, rc) {
			ids = append(ids, subRes.ID)
		}
	}
	return relatedResultTrunc("sns-sub", ids, truncated)
}

// checkLambdaS3 scans the s3 cache for buckets that have a notification
// target equal to this Lambda. The bucket cache entry populates
// Fields["notification_lambda"] if the fetcher enriched it.
func checkLambdaS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return foundNone("s3", "res.ID")
	}
	rc := refContext(clients, cache, "lambda")
	s3List, truncated, err := relatedResourcesFor(ctx, clients, cache, "s3")
	if err != nil {
		return ReadFailed("s3", err)
	}
	if s3List == nil {
		return NotRead("s3")
	}
	var ids []string
	for _, bRes := range s3List {
		// A bucket whose notification lookup was refused or could not be made
		// from its region may well notify this function. Nothing here can tell,
		// so the count is a lower bound — the same thing the three forward
		// pivots render for that bucket.
		if bRes.Fields["notification_error"] != "" || bRes.Fields["notification_truncated"] == "true" {
			truncated = true
		}
		// The field is the bucket's whole comma-joined destination list; a
		// bucket that notifies this function second is still this function's
		// bucket.
		for n := range strings.SplitSeq(bRes.Fields["notification_lambda"], ",") {
			if n == "" {
				continue
			}
			if lambdaRefNamesFunction(n, res.ID, rc) {
				ids = append(ids, bRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("s3", ids, truncated)
}

// checkLambdaENI scans the eni cache for the Hyperplane ENIs this
// VPC-attached function uses (hyperplaneENIServes).
func checkLambdaENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return NotRead("eni")
	}
	if fn.VpcConfig == nil || len(fn.VpcConfig.SubnetIds) == 0 {
		return foundNone("eni", "fn.VpcConfig")
	}
	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return ReadFailed("eni", err)
	}
	if eniList == nil {
		return NotRead("eni")
	}
	var ids []string
	for _, eniRes := range eniList {
		if eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct); ok && hyperplaneENIServes(eni, fn) {
			ids = append(ids, eniRes.ID)
		}
	}
	return relatedResultTrunc("eni", ids, truncated)
}

// checkLambdaSecrets scans this Lambda's environment-variable values for
// references to secrets-manager secret ARNs. FunctionConfiguration.Environment.Variables
// is a map[string]string; we search values for a Secrets Manager ARN.
func checkLambdaSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return NotRead("secrets")
	}
	if fn.Environment == nil || len(fn.Environment.Variables) == 0 {
		return foundNone("secrets", "fn.Environment.Variables")
	}
	arnSet := make(map[string]struct{})
	for _, v := range fn.Environment.Variables {
		if _, isSecret := ARNForService(v, "secretsmanager"); isSecret {
			arnSet[v] = struct{}{}
		}
	}
	if len(arnSet) == 0 {
		return foundNone("secrets", "arnSet")
	}
	secretList, truncated, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return ReadFailed("secrets", err)
	}
	if secretList == nil {
		return NotRead("secrets")
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
		return NotRead("ssm")
	}
	if fn.Environment == nil || len(fn.Environment.Variables) == 0 {
		return foundNone("ssm", "fn.Environment.Variables")
	}
	candidates := make(map[string]struct{})
	for _, v := range fn.Environment.Variables {
		if strings.HasPrefix(v, "/") {
			candidates[v] = struct{}{}
		}
	}
	if len(candidates) == 0 {
		return foundNone("ssm", "candidates")
	}
	ssmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ssm")
	if err != nil {
		return ReadFailed("ssm", err)
	}
	if ssmList == nil {
		return NotRead("ssm")
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

var _ = cwtypes.MetricAlarm{}

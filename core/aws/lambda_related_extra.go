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
		return resource.UnknownRelated("subnet")
	}
	if fn.VpcConfig == nil {
		return resource.ProvenZero("subnet", "fn.VpcConfig")
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
		return resource.UnknownRelated("efs")
	}
	var refs []string
	for _, cfg := range fn.FileSystemConfigs {
		if cfg.Arn != nil {
			refs = append(refs, *cfg.Arn)
		}
	}
	return relatedRefs("efs", refs, refContext(clients, cache, "efs"))
}

// checkLambdaAPIGW offers the APIs that name this function in a tag or in
// their own Name, as candidates. Which function an API invokes is in its
// integrations, which apigatewayv2.Api does not embed and only
// GetIntegrations per API returns; a name or a tag key is free text an
// operator chooses, so the row shows candidates rather than a count.
func checkLambdaAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.ProvenZero("apigw", "fnName")
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
	return heuristicResult("apigw", ids, truncated)
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
		return unreadZero(res, resource.ProvenZero("cf", "fnARN"))
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
// stream ARNs, read through the ddb resolver.
// Requires live Lambda client for ListEventSourceMappings.
func checkLambdaDDB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.ProvenZero("ddb", "fnName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("ddb")
	}
	return lambdaEventSourceRefs(ctx, c.Lambda, fnName, "ddb", ":dynamodb:", refContext(clients, cache, "ddb"))
}

// checkLambdaKinesis scans this Lambda's event source mappings for Kinesis
// stream ARNs.
func checkLambdaKinesis(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.ProvenZero("kinesis", "fnName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("kinesis")
	}
	return lambdaEventSourceRefs(ctx, c.Lambda, fnName, "kinesis", ":kinesis:", refContext(clients, cache, "kinesis"))
}

// checkLambdaMSK scans this Lambda's event source mappings for MSK cluster
// ARNs.
func checkLambdaMSK(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.ProvenZero("msk", "fnName")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("msk")
	}
	return lambdaEventSourceRefs(ctx, c.Lambda, fnName, "msk", ":kafka:", refContext(clients, cache, "msk"))
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
	fnARN := ""
	if fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct); ok && fn.FunctionArn != nil {
		fnARN = *fn.FunctionArn
	}
	fnName := res.ID
	if fnARN == "" && fnName == "" {
		return resource.ProvenZero("tg", "fnName")
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
		return resource.ProvenZero("tg", "lambdaTGs")
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
		return resource.ProvenZero("sns", "fnName")
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
// Lambda is the endpoint.
func checkLambdaSNSSub(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnARN := ""
	if fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct); ok && fn.FunctionArn != nil {
		fnARN = *fn.FunctionArn
	}
	fnName := res.ID
	if fnARN == "" && fnName == "" {
		return resource.ProvenZero("sns-sub", "fnName")
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
		return resource.ProvenZero("s3", "fnName")
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
			if (fnARN != "" && n == fnARN) ||
				(fnName != "" && strings.HasSuffix(n, ":function:"+fnName)) {
				ids = append(ids, bRes.ID)
				break
			}
		}
	}
	return relatedResultTrunc("s3", ids, truncated)
}

// checkLambdaENI scans the eni cache for the hyperplane ENIs EC2 creates for
// this VPC-attached function. The interface type says Lambda owns the ENI;
// the Description, "AWS Lambda VPC ENI-<FunctionName>-<uuid>", is what says
// which function it belongs to.
func checkLambdaENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.ProvenZero("eni", "fnName")
	}
	eniList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eni")
	if err != nil {
		return resource.ErrorRelated("eni", err)
	}
	if eniList == nil {
		return resource.UnknownRelated("eni")
	}
	var ids []string
	for _, eniRes := range eniList {
		eni, ok := assertStruct[ec2types.NetworkInterface](eniRes.RawStruct)
		if !ok || !isLambdaENI(eni) {
			continue
		}
		if lambdaFunctionNameFromENIDescription(aws.ToString(eni.Description)) == fnName {
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
		if res.RawStruct == nil {
			return resource.UnknownRelated("secrets")
		}
		return resource.KnownRelated("secrets", nil, false)
	}
	if fn.Environment == nil || len(fn.Environment.Variables) == 0 {
		return resource.ProvenZero("secrets", "fn.Environment.Variables")
	}
	arnSet := make(map[string]struct{})
	for _, v := range fn.Environment.Variables {
		if _, isSecret := ARNForService(v, "secretsmanager"); isSecret {
			arnSet[v] = struct{}{}
		}
	}
	if len(arnSet) == 0 {
		return resource.ProvenZero("secrets", "arnSet")
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
		return resource.ProvenZero("ssm", "fn.Environment.Variables")
	}
	candidates := make(map[string]struct{})
	for _, v := range fn.Environment.Variables {
		if strings.HasPrefix(v, "/") {
			candidates[v] = struct{}{}
		}
	}
	if len(candidates) == 0 {
		return resource.ProvenZero("ssm", "candidates")
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

var _ = cwtypes.MetricAlarm{}

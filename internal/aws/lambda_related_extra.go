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
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Forward Pattern F (data already in RawStruct) ---

// checkLambdaSubnet extracts subnet IDs from Lambda VpcConfig.SubnetIds.
func checkLambdaSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if fn.VpcConfig == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
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
		return resource.RelatedCheckResult{TargetType: "efs", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "efs", Count: 0}
	}
	return relatedResult("efs", ids)
}

// --- Reverse cache scans (target cache references this Lambda) ---

// checkLambdaAPIGW scans the apigw cache for HTTP/REST APIs that integrate
// with this Lambda function. apigatewayv2.Api struct does not embed
// integrations, so without a GetIntegrations API call this is undeterminable.
// We approximate by searching for the function name in the api's Name or
// Tags — a weak signal, but better than Count:0 when a real match exists.
func checkLambdaAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: 0}
	}
	apiList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "apigw")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1, Err: err}
	}
	if apiList == nil {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1}
	}
	return relatedResult("apigw", ids)
}

// checkLambdaCF scans the cloudfront cache for Lambda@Edge or
// CloudFront-Functions-adjacent distributions that reference this Lambda.
// cftypes.DistributionSummary does not embed association detail in list
// responses, so the cache alone cannot resolve the link. Returns Count:0 when
// no signal exists.
func checkLambdaCF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnARN := ""
	if fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct); ok && fn.FunctionArn != nil {
		fnARN = *fn.FunctionArn
	}
	if fnARN == "" {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}
	cfList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "cf")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1, Err: err}
	}
	if cfList == nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}
	var ids []string
	for _, cfRes := range cfList {
		// DistributionSummary.AliasICPRecordals is not useful here; the list
		// response doesn't carry Lambda associations. Use Fields heuristic.
		if cfRes.Fields["lambda_function_arn"] == fnARN {
			ids = append(ids, cfRes.ID)
		}
	}
	_ = cftypes.DistributionSummary{}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}
	return relatedResult("cf", ids)
}

// checkLambdaDDB scans this Lambda's event source mappings for DynamoDB
// stream ARNs and returns the table names (last path segment after "table/").
// Requires live Lambda client for ListEventSourceMappings.
func checkLambdaDDB(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.RelatedCheckResult{TargetType: "ddb", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.RelatedCheckResult{TargetType: "ddb", Count: -1}
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		FunctionName: &fnName,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ddb", Count: -1, Err: err}
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
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: -1}
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		FunctionName: &fnName,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: -1, Err: err}
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
		return resource.RelatedCheckResult{TargetType: "msk", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.RelatedCheckResult{TargetType: "msk", Count: -1}
	}
	out, err := c.Lambda.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
		FunctionName: &fnName,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "msk", Count: -1, Err: err}
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
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err}
	}
	if evList == nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	return relatedResult("ct-events", ids)
}

// --- Reverse lookups that require fields not in the cached struct ---
//
// The following checkers reverse-look the target cache for references back to
// this Lambda. When the target cache's struct doesn't carry the reference
// (e.g. ELB target groups list, SNS subscription list entries), the checker
// needs additional API calls per candidate (N+1) to resolve the link. We
// return Count: -1 when no live client is available and otherwise do a bounded
// scan.

// checkLambdaTG scans the tg cache for target groups whose TargetType is
// "lambda" and whose Targets include this function ARN. ELB v2's
// DescribeTargetHealth would be needed to resolve target set; without it we
// return -1 when clients are available OR match by name hint in Fields.
func checkLambdaTG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.RelatedCheckResult{TargetType: "tg", Count: 0}
	}
	tgList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "tg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1, Err: err}
	}
	if tgList == nil {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	var ids []string
	for _, tgRes := range tgList {
		if tgRes.Fields["target_type"] != "lambda" {
			continue
		}
		if tgRes.Fields["lambda_function_name"] == fnName ||
			tgRes.Fields["target_arn"] != "" && strings.HasSuffix(tgRes.Fields["target_arn"], ":function:"+fnName) {
			ids = append(ids, tgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "tg", Count: -1}
	}
	return relatedResult("tg", ids)
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
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	subList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "sns-sub")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1, Err: err}
	}
	if subList == nil {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}
	return relatedResult("sns", ids)
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
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: 0}
	}
	subList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "sns-sub")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1, Err: err}
	}
	if subList == nil {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}
	return relatedResult("sns-sub", ids)
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
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	s3List, truncated, err := lambdaRelatedResources(ctx, clients, cache, "s3")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1, Err: err}
	}
	if s3List == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	return relatedResult("s3", ids)
}

// checkLambdaENI scans the eni cache for ENIs attached to this Lambda's
// hyperplane (VPC-attached functions get AWS-managed ENIs with a
// well-known description prefix "AWS Lambda VPC ENI").
func checkLambdaENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fnName := res.ID
	if fnName == "" {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	eniList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1, Err: err}
	}
	if eniList == nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	var ids []string
	for _, eniRes := range eniList {
		desc := eniRes.Fields["description"]
		if desc == "" {
			continue
		}
		// Hyperplane ENIs have a description like
		// "AWS Lambda VPC ENI-{FunctionName}-..." but the AWS-managed form
		// varies. Match on the function name as a substring of the
		// description (a conservative signal).
		if strings.Contains(desc, fnName) {
			ids = append(ids, eniRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	return relatedResult("eni", ids)
}

// checkLambdaSecrets scans this Lambda's environment-variable values for
// references to secrets-manager secret ARNs. FunctionConfiguration.Environment.Variables
// is a map[string]string; we search values for an "arn:aws:secretsmanager:" prefix.
func checkLambdaSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	if fn.Environment == nil || len(fn.Environment.Variables) == 0 {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	arnSet := make(map[string]struct{})
	for _, v := range fn.Environment.Variables {
		if strings.HasPrefix(v, "arn:aws:secretsmanager:") {
			arnSet[v] = struct{}{}
		}
	}
	if len(arnSet) == 0 {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	secretList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1, Err: err}
	}
	if secretList == nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	return relatedResult("secrets", ids)
}

// checkLambdaSSM scans Lambda's environment-variable values for SSM parameter
// name references. Values matching "/" or starting with "/aws/..." that also
// exist in the ssm cache (Parameter Store) are returned.
func checkLambdaSSM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	fn, ok := assertStruct[lambdatypes.FunctionConfiguration](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: -1}
	}
	if fn.Environment == nil || len(fn.Environment.Variables) == 0 {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: 0}
	}
	candidates := make(map[string]struct{})
	for _, v := range fn.Environment.Variables {
		if strings.HasPrefix(v, "/") {
			candidates[v] = struct{}{}
		}
	}
	if len(candidates) == 0 {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: 0}
	}
	ssmList, truncated, err := lambdaRelatedResources(ctx, clients, cache, "ssm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: -1, Err: err}
	}
	if ssmList == nil {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ssm", Count: -1}
	}
	return relatedResult("ssm", ids)
}

// Ensure cwtypes stays imported for future alarm-related extensions.
var _ = cwtypes.MetricAlarm{}

// apigw_related.go contains API Gateway related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkApigwKMS resolves KMS keys referenced by this API's Lambda integrations.
// Weak pair (3-sometimes/2-no consensus). API Gateway has no direct KMS field;
// we follow Lambda integrations as a best effort.
// Pattern C: one GetIntegrations call + per-Lambda-target GetFunction call.
// Extracts KMSKeyArn from each Lambda integration's FunctionConfiguration.
func checkApigwKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	items, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	lambdaAPI, ok := c.Lambda.(LambdaGetFunctionAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	seen := make(map[string]struct{})
	for _, item := range items {
		if item.IntegrationUri == nil || !strings.Contains(*item.IntegrationUri, ":function:") {
			continue
		}
		// Extract function name from the integration URI.
		uri := *item.IntegrationUri
		idx := strings.LastIndex(uri, ":function:")
		rest := uri[idx+len(":function:"):]
		if slash := strings.Index(rest, "/"); slash >= 0 {
			rest = rest[:slash]
		}
		if colon := strings.Index(rest, ":"); colon >= 0 {
			rest = rest[:colon]
		}
		if rest == "" {
			continue
		}
		out, lerr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambdapkg.GetFunctionOutput, error) {
			return lambdaAPI.GetFunction(ctx, &lambdapkg.GetFunctionInput{FunctionName: &rest})
		})
		if lerr != nil || out == nil || out.Configuration == nil {
			continue
		}
		if out.Configuration.KMSKeyArn != nil && *out.Configuration.KMSKeyArn != "" {
			seen[arnLastSegment(*out.Configuration.KMSKeyArn)] = struct{}{}
		}
	}
	return relatedResult("kms", mapKeys(seen))
}

// checkApigwLogs searches the logs cache for log groups associated with this
// API Gateway by naming convention:
//   - API-Gateway-Execution-Logs_{apiID}/ prefix (default execution log group)
//   - /aws/apigateway/{apiName} (custom access log group convention)
//
// Pattern N — naming convention.
func checkApigwLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	apiName := res.Name
	if apiName == "" {
		apiName = res.Fields["name"]
	}
	if apiID == "" && apiName == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	logList, truncated, err := apigwRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	executionPrefix := "API-Gateway-Execution-Logs_" + apiID + "/"
	accessLogPrefix := "/aws/apigateway/" + apiName

	var ids []string
	for _, logRes := range logList {
		if (apiID != "" && strings.HasPrefix(logRes.ID, executionPrefix)) ||
			(apiName != "" && strings.HasPrefix(logRes.ID, accessLogPrefix)) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

// apigwListIntegrations makes a single apigatewayv2:GetIntegrations call for
// the given API via RetryOnThrottle, returning the integrations slice.
func apigwListIntegrations(ctx context.Context, clients any, apiID string) ([]apigwtypes.Integration, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		return nil, errClientMissing
	}
	api, ok := c.APIGatewayV2.(APIGatewayV2GetIntegrationsAPI)
	if !ok {
		return nil, errClientMissing
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*apigatewayv2.GetIntegrationsOutput, error) {
		return api.GetIntegrations(ctx, &apigatewayv2.GetIntegrationsInput{ApiId: &apiID})
	})
	if err != nil {
		return nil, err
	}
	return out.Items, nil
}

// checkApigwLambda reports Lambda integration targets of this API Gateway.
// Pattern C: one apigatewayv2:GetIntegrations call, filter to AWS_PROXY /
// AWS integrations whose IntegrationUri points at a Lambda invoke ARN.
func checkApigwLambda(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	items, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	seen := make(map[string]bool)
	var ids []string
	for _, item := range items {
		if item.IntegrationUri == nil || *item.IntegrationUri == "" {
			continue
		}
		uri := *item.IntegrationUri
		// Lambda invoke ARN form: arn:aws:apigateway:REGION:lambda:path/.../functions/arn:aws:lambda:REGION:ACCT:function:NAME/invocations
		// Or direct: arn:aws:lambda:REGION:ACCT:function:NAME
		if !strings.Contains(uri, ":function:") {
			continue
		}
		idx := strings.LastIndex(uri, ":function:")
		rest := uri[idx+len(":function:"):]
		// Strip "/invocations" suffix and optional version alias.
		if slash := strings.Index(rest, "/"); slash >= 0 {
			rest = rest[:slash]
		}
		if colon := strings.Index(rest, ":"); colon >= 0 {
			rest = rest[:colon]
		}
		if rest != "" && !seen[rest] {
			seen[rest] = true
			ids = append(ids, rest)
		}
	}
	return relatedResult("lambda", ids)
}

// checkApigwWAF reports WAF Web ACL associations for this API Gateway.
// Only REST APIs (v1) can associate a Web ACL via apigateway:GetWebACL, and
// HTTP/WebSocket APIs (v2, which this fetcher lists) do not carry Web ACL
// bindings in GetApis. Resolving the relationship from the WAF side requires
// wafv2:ListResourcesForWebACL per Web ACL (O(N)), which is outside the
// 1-call budget for forward (apigw→waf) checkers.
// Returns Count: -1 (unknown) to signal the data is not available.
func checkApigwWAF(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "waf", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
}

// checkApigwACM reports ACM certificates on this API's custom domain.
// Custom domain mapping requires apigatewayv2:GetDomainNames (not GetApis).
// Returns Count: -1.
func checkApigwACM(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
}

// checkApigwAlarm reports CloudWatch alarms on this API. API Gateway alarms
// use dimension "ApiId". Scans the alarm cache.
func checkApigwAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := apigwRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		raw, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range raw.Dimensions {
			if d.Name != nil && *d.Name == "ApiId" && d.Value != nil && *d.Value == apiID {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkApigwCF reports CloudFront distributions fronting this API. Distribution
// Origins may reference the API's invoke URL. Determining this requires
// scanning the cf cache for origins whose DomainName includes the API ID
// (typically "<api-id>.execute-api.<region>.amazonaws.com").
func checkApigwCF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "cf", Count: 0}
	}
	suffix := apiID + ".execute-api."

	cfList, truncated, err := apigwRelatedResources(ctx, clients, cache, "cf")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1, Err: err}
	}
	if cfList == nil {
		return resource.RelatedCheckResult{TargetType: "cf", Count: -1}
	}

	var ids []string
	for _, cfRes := range cfList {
		dist, ok := assertStruct[cftypes.DistributionSummary](cfRes.RawStruct)
		if !ok || dist.Origins == nil {
			continue
		}
		for _, origin := range dist.Origins.Items {
			if origin.DomainName != nil && strings.Contains(*origin.DomainName, suffix) {
				ids = append(ids, cfRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("cf")
	}
	return relatedResult("cf", ids)
}

// checkApigwELB reports NLB target groups behind this API's VPC link.
// VpcLink → NLB mapping lives on apigatewayv2:GetVpcLinks (not GetApis).
// Returns Count: -1.
func checkApigwELB(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
}

// checkApigwR53 reports Route 53 zones with alias records for this API's
// custom domain. Records live per-zone — not cached. Returns Count: -1.
func checkApigwR53(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
}

// checkApigwRole reports IAM roles this API assumes (invocation/authorizer
// roles). Role references live on GetRoute/GetAuthorizer (per-route), not
// GetApis. Returns Count: -1.
func checkApigwRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "role", Count: -1}
}

// checkApigwSFN reports Step Functions state machines integrated as targets.
// Pattern C: one GetIntegrations call; look for StartExecution target ARNs.
func checkApigwSFN(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "sfn", Count: 0}
	}
	items, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.RelatedCheckResult{TargetType: "sfn", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "sfn", Count: -1, Err: err}
	}
	seen := make(map[string]bool)
	var ids []string
	for _, item := range items {
		if item.IntegrationUri == nil {
			continue
		}
		uri := *item.IntegrationUri
		// SFN integration URI: arn:aws:apigateway:REGION:states:action/StartExecution
		// The target state-machine ARN is in the request template, not the URI.
		// The URI only tells us "this API talks to States". Extract nothing
		// specific; the precise state machine requires parsing the request
		// template on each Route — outside budget.
		if !strings.Contains(uri, ":states:action/") {
			continue
		}
		// Fall back to tagging the integration id so there is at least one
		// datapoint reported; without template parsing we cannot identify
		// the state machine name.
		if item.IntegrationId != nil && !seen[*item.IntegrationId] {
			seen[*item.IntegrationId] = true
			ids = append(ids, *item.IntegrationId)
		}
	}
	// If we saw SFN integrations but can't name the state machine, return
	// Count:-1 so the UI shows "?" rather than a misleading integration-id.
	if len(ids) > 0 {
		return resource.RelatedCheckResult{TargetType: "sfn", Count: -1}
	}
	return resource.RelatedCheckResult{TargetType: "sfn", Count: 0}
}

// checkApigwSNS reports SNS topics targeted by this API's integrations.
// Pattern C: one GetIntegrations call; look for SNS Publish target ARNs.
func checkApigwSNS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	items, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1, Err: err}
	}
	sawSNS := false
	for _, item := range items {
		if item.IntegrationUri == nil {
			continue
		}
		if strings.Contains(*item.IntegrationUri, ":sns:action/") {
			sawSNS = true
			break
		}
	}
	if !sawSNS {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	// SNS topic ARN lives in the route request template, not the integration
	// URI. Identifying the topic requires template parsing. Return -1.
	return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
}

// checkApigwVPCE reports VPC endpoints this private API exposes through.
// Private-API endpoint IDs live on the Api.EndpointConfiguration — not on
// the v2 GetApis Item (endpoint_configuration is v1-only). Returns -1 if
// this API is private, else 0.
func checkApigwVPCE(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "vpce", Count: 0}
	}
	return resource.RelatedCheckResult{TargetType: "vpce", Count: -1}
}

// apigwRelatedResources returns the resource list for target from cache or by fetching the first page.
func apigwRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

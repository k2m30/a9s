// apigw_related.go contains API Gateway related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/core/resource"
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
			return resource.UnknownRelated("kms")
		}
		return resource.ErrorRelated("kms", err)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("kms")
	}
	lambdaAPI, ok := c.Lambda.(LambdaGetFunctionAPI)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	seen := make(map[string]struct{})
	var failures []string
	total := 0
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
		total++
		out, lerr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambdapkg.GetFunctionOutput, error) {
			return lambdaAPI.GetFunction(ctx, &lambdapkg.GetFunctionInput{FunctionName: &rest})
		})
		if lerr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", rest, lerr))
			continue
		}
		if out == nil || out.Configuration == nil {
			continue
		}
		if out.Configuration.KMSKeyArn != nil && *out.Configuration.KMSKeyArn != "" {
			seen[arnLastSegment(*out.Configuration.KMSKeyArn)] = struct{}{}
		}
	}
	ids := mapKeys(seen)
	result := relatedResult("kms", ids)
	result.Err = AggregateFailures("apigw-related: GetFunction", failures, total)
	return result
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

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
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
	return relatedResultTrunc("logs", ids, truncated)
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
			return resource.UnknownRelated("lambda")
		}
		return resource.ErrorRelated("lambda", err)
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

// checkApigwACM reports ACM certificates attached to this API's custom domain names.
// Enumerates GetDomainNames, then per domain calls GetApiMappings to check if the
// domain maps to this API. For matching domains, harvests CertificateArn from each
// DomainNameConfiguration.
func checkApigwACM(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		return resource.UnknownRelated("acm")
	}
	dnAPI, ok := c.APIGatewayV2.(APIGatewayV2GetDomainNamesAPI)
	if !ok {
		return resource.UnknownRelated("acm")
	}
	mapAPI, ok := c.APIGatewayV2.(APIGatewayV2GetApiMappingsAPI)
	if !ok {
		return resource.UnknownRelated("acm")
	}
	// Enumerate all custom domain names (one call).
	dn, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*apigatewayv2.GetDomainNamesOutput, error) {
		return dnAPI.GetDomainNames(ctx, &apigatewayv2.GetDomainNamesInput{})
	})
	if err != nil {
		return resource.ErrorRelated("acm", err)
	}
	seen := make(map[string]struct{})
	var failures []string
	total := 0
	for _, d := range dn.Items {
		if d.DomainName == nil {
			continue
		}
		total++
		// Per domain: get its mappings; check if any maps to this API.
		m, merr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*apigatewayv2.GetApiMappingsOutput, error) {
			return mapAPI.GetApiMappings(ctx, &apigatewayv2.GetApiMappingsInput{DomainName: d.DomainName})
		})
		if merr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", *d.DomainName, merr))
			continue
		}
		if m == nil {
			continue
		}
		matched := false
		for _, am := range m.Items {
			if am.ApiId != nil && *am.ApiId == apiID {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		// Harvest CertificateArn from each domain configuration.
		for _, dcfg := range d.DomainNameConfigurations {
			if dcfg.CertificateArn != nil && *dcfg.CertificateArn != "" {
				seen[arnLastSegment(*dcfg.CertificateArn)] = struct{}{}
			}
		}
	}
	ids := mapKeys(seen)
	result := relatedResult("acm", ids)
	result.Err = AggregateFailures("apigw-related: GetApiMappings", failures, total)
	return result
}

// checkApigwAlarm reports CloudWatch alarms on this API. API Gateway alarms
// use dimension "ApiId". Scans the alarm cache.
func checkApigwAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := relatedResourcesFor(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.ErrorRelated("alarm", err)
	}
	if alarmList == nil {
		return resource.UnknownRelated("alarm")
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
	return relatedResultTrunc("alarm", ids, truncated)
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

	cfList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cf")
	if err != nil {
		return resource.ErrorRelated("cf", err)
	}
	if cfList == nil {
		return resource.UnknownRelated("cf")
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
	return relatedResultTrunc("cf", ids, truncated)
}

// checkApigwELB reports the Network Load Balancer behind this API's VPC
// link. Pattern C: apigatewayv2:GetIntegrations to find VPC_LINK integrations
// and their ConnectionId (the VpcLink ID), apigatewayv2:GetVpcLinks
// (account-wide, not API-scoped) to resolve that VpcLink's subnet/security-
// group set, then intersect against the already-loaded elb cache's NLBs by
// AvailabilityZones[].SubnetId / SecurityGroups membership.
func checkApigwELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	items, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.UnknownRelated("elb")
		}
		return resource.ErrorRelated("elb", err)
	}
	var vpcLinkIDs []string
	seenLinks := make(map[string]struct{})
	for _, item := range items {
		if item.ConnectionType != apigwtypes.ConnectionTypeVpcLink || item.ConnectionId == nil || *item.ConnectionId == "" {
			continue
		}
		if _, dup := seenLinks[*item.ConnectionId]; dup {
			continue
		}
		seenLinks[*item.ConnectionId] = struct{}{}
		vpcLinkIDs = append(vpcLinkIDs, *item.ConnectionId)
	}
	if len(vpcLinkIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		return resource.UnknownRelated("elb")
	}
	vpcLinkAPI, ok := c.APIGatewayV2.(APIGatewayV2GetVpcLinksAPI)
	if !ok {
		return resource.UnknownRelated("elb")
	}

	wantedSubnets := make(map[string]struct{})
	wantedSGs := make(map[string]struct{})
	var nextToken *string
	// GetVpcLinks is account-wide and paginated; VpcLinks are few per
	// account, but the token is honored until exhausted rather than assuming
	// a single page.
	for {
		var input apigatewayv2.GetVpcLinksInput
		if nextToken != nil {
			input.NextToken = nextToken
		}
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*apigatewayv2.GetVpcLinksOutput, error) {
			return vpcLinkAPI.GetVpcLinks(ctx, &input)
		})
		if err != nil {
			return resource.ErrorRelated("elb", err)
		}
		if out == nil {
			break
		}
		for _, link := range out.Items {
			if link.VpcLinkId == nil {
				continue
			}
			if _, wanted := seenLinks[*link.VpcLinkId]; !wanted {
				continue
			}
			for _, s := range link.SubnetIds {
				wantedSubnets[s] = struct{}{}
			}
			for _, sg := range link.SecurityGroupIds {
				wantedSGs[sg] = struct{}{}
			}
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}
	if len(wantedSubnets) == 0 && len(wantedSGs) == 0 {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	elbList, truncated, fetchErr := relatedResourcesFor(ctx, clients, cache, "elb")
	if fetchErr != nil {
		return resource.ErrorRelated("elb", fetchErr)
	}
	if elbList == nil {
		return resource.UnknownRelated("elb")
	}

	var ids []string
	for _, elbRes := range elbList {
		lb, ok := assertStruct[elbv2types.LoadBalancer](elbRes.RawStruct)
		if !ok {
			continue
		}
		matched := false
		for _, az := range lb.AvailabilityZones {
			if az.SubnetId != nil {
				if _, found := wantedSubnets[*az.SubnetId]; found {
					matched = true
					break
				}
			}
		}
		if !matched {
			for _, sg := range lb.SecurityGroups {
				if _, found := wantedSGs[sg]; found {
					matched = true
					break
				}
			}
		}
		if matched {
			ids = append(ids, elbRes.ID)
		}
	}
	return relatedResultTrunc("elb", ids, truncated)
}

// checkApigwRole reports IAM roles this API assumes to call the integration
// target or to run a request authorizer. Pattern C: reuses the
// apigatewayv2:GetIntegrations call (Integration.CredentialsArn) already
// made by the lambda/kms/sfn/sns pivots, plus one apigatewayv2:GetAuthorizers
// call (Authorizer.AuthorizerCredentialsArn) — role ARNs reduced to bare
// RoleName so the role cache's FetchByIDs resolves them.
func checkApigwRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	seen := make(map[string]struct{})

	items, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil && !errors.Is(err, errClientMissing) {
		return resource.ErrorRelated("role", err)
	}
	for _, item := range items {
		if item.CredentialsArn != nil && *item.CredentialsArn != "" {
			seen[arnRoleName(*item.CredentialsArn)] = struct{}{}
		}
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		if len(seen) > 0 {
			return relatedResult("role", mapKeys(seen))
		}
		return resource.UnknownRelated("role")
	}
	authAPI, ok := c.APIGatewayV2.(APIGatewayV2GetAuthorizersAPI)
	if !ok {
		if len(seen) > 0 {
			return relatedResult("role", mapKeys(seen))
		}
		return resource.UnknownRelated("role")
	}
	authOut, authErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*apigatewayv2.GetAuthorizersOutput, error) {
		return authAPI.GetAuthorizers(ctx, &apigatewayv2.GetAuthorizersInput{ApiId: &apiID})
	})
	if authErr != nil {
		if len(seen) > 0 {
			return relatedResult("role", mapKeys(seen))
		}
		return resource.ErrorRelated("role", authErr)
	}
	if authOut != nil {
		for _, a := range authOut.Items {
			if a.AuthorizerCredentialsArn != nil && *a.AuthorizerCredentialsArn != "" {
				seen[arnRoleName(*a.AuthorizerCredentialsArn)] = struct{}{}
			}
		}
	}

	return relatedResult("role", mapKeys(seen))
}

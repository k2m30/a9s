// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// apigw_related.go contains API Gateway related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkApigwKMS resolves KMS keys referenced by this API's Lambda integrations.
// API Gateway has no direct KMS field; Lambda integrations are followed as a
// best effort: one GetIntegrations call + per-Lambda-target GetFunction call.
// Extracts KMSKeyArn from each Lambda integration's FunctionConfiguration.
func checkApigwKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return foundNone("kms", "apiID")
	}
	items, complete, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return NotRead("kms")
		}
		return ReadFailed("kms", err)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return NotRead("kms")
	}
	lambdaAPI, ok := c.Lambda.(LambdaGetFunctionAPI)
	if !ok {
		return NotRead("kms")
	}
	var refs []string
	var failures []Failure
	total := 0
	lambdaRC := refContext(clients, cache, "lambda")
	for _, item := range items {
		fn, ok := resource.ResolveRef("lambda", lambdaIntegrationARN(aws.ToString(item.IntegrationUri)), lambdaRC)
		if !ok {
			continue
		}
		total++
		out, lerr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambdapkg.GetFunctionOutput, error) {
			return lambdaAPI.GetFunction(ctx, &lambdapkg.GetFunctionInput{FunctionName: &fn})
		})
		if lerr != nil {
			failures = append(failures, FailedCall(fn, lerr))
			continue
		}
		if out == nil || out.Configuration == nil {
			continue
		}
		if out.Configuration.KMSKeyArn != nil && *out.Configuration.KMSKeyArn != "" {
			refs = append(refs, *out.Configuration.KMSKeyArn)
		}
	}
	ids, lowerBound, err := kmsResolve(ctx, clients, cache, kmsRegion(refs), refs)
	if len(ids) == 0 {
		// Every function refused its read: nothing was established about any
		// of them, which is a fetch failure rather than a lower bound over
		// what was read.
		if aggErr := AggregateFailures("apigw-related: GetFunction", failures, total); aggErr != nil && len(failures) == total {
			return ReadFailed("kms", aggErr)
		}
		if err != nil {
			return ReadFailed("kms", err)
		}
	}
	// Some calls may have failed: ids is a proven subset, not necessarily
	// exhaustive. Truncated (not Errored) keeps the row actionable rather
	// than discarding confirmed matches as a dead end.
	return relatedResultTrunc("kms", ids, lowerBound || len(failures) > 0 || !complete)
}

// checkApigwLogs searches the logs cache for log groups associated with this
// API Gateway by naming convention:
//   - API-Gateway-Execution-Logs_{apiID}/ prefix (default execution log group)
//   - /aws/apigateway/{apiName} (custom access log group convention)
func checkApigwLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	apiName := res.Name
	if apiName == "" {
		apiName = res.Fields["name"]
	}
	if apiID == "" && apiName == "" {
		return foundNone("logs", "apiName")
	}

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return ReadFailed("logs", err)
	}
	if logList == nil {
		return NotRead("logs")
	}

	var executionLogs, accessLogs string
	if apiID != "" {
		executionLogs = "API-Gateway-Execution-Logs_" + apiID
	}
	if apiName != "" {
		accessLogs = "/aws/apigateway/" + apiName
	}
	return relatedResultTrunc("logs", logGroupsUnder(logList, executionLogs, accessLogs), truncated)
}

// apigwListIntegrations walks apigatewayv2:GetIntegrations for the given
// API. complete is false when the walk stopped at the page cap.
func apigwListIntegrations(ctx context.Context, clients any, apiID string) (items []apigwtypes.Integration, complete bool, err error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		return nil, false, errClientMissing
	}
	api, ok := c.APIGatewayV2.(APIGatewayV2GetIntegrationsAPI)
	if !ok {
		return nil, false, errClientMissing
	}
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.Integration, *string, error) {
		out, err := api.GetIntegrations(ctx, &apigatewayv2.GetIntegrationsInput{ApiId: &apiID, NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.NextToken, nil
	})
}

// checkApigwLambda reports Lambda integration targets of this API Gateway.
// One apigatewayv2:GetIntegrations call, filter to AWS_PROXY /
// AWS integrations whose IntegrationUri points at a Lambda invoke ARN.
func checkApigwLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return foundNone("lambda", "apiID")
	}
	items, complete, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return NotRead("lambda")
		}
		return ReadFailed("lambda", err)
	}
	var arns []string
	for _, item := range items {
		arns = append(arns, lambdaIntegrationARN(aws.ToString(item.IntegrationUri)))
	}
	return listedRelated(ctx, clients, cache, "lambda", arns, !complete)
}

// checkApigwACM reports ACM certificates attached to this API's custom domain names.
// Enumerates GetDomainNames, then per domain calls GetApiMappings to check if the
// domain maps to this API. For matching domains, harvests CertificateArn from each
// DomainNameConfiguration.
func checkApigwACM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return foundNone("acm", "apiID")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		return NotRead("acm")
	}
	dnAPI, ok := c.APIGatewayV2.(APIGatewayV2GetDomainNamesAPI)
	if !ok {
		return NotRead("acm")
	}
	mapAPI, ok := c.APIGatewayV2.(APIGatewayV2GetApiMappingsAPI)
	if !ok {
		return NotRead("acm")
	}
	domains, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.DomainName, *string, error) {
		out, err := dnAPI.GetDomainNames(ctx, &apigatewayv2.GetDomainNamesInput{NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.NextToken, nil
	})
	if err != nil {
		return ReadFailed("acm", err)
	}
	var refs []string
	var failures []Failure
	total := 0
	for _, d := range domains {
		if d.DomainName == nil {
			continue
		}
		total++
		mappings, mappingsComplete, merr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.ApiMapping, *string, error) {
			out, err := mapAPI.GetApiMappings(ctx, &apigatewayv2.GetApiMappingsInput{DomainName: d.DomainName, NextToken: token})
			if err != nil {
				return nil, nil, err
			}
			return out.Items, out.NextToken, nil
		})
		if merr != nil {
			failures = append(failures, FailedCall(*d.DomainName, merr))
			continue
		}
		matched := slices.ContainsFunc(mappings, func(am apigwtypes.ApiMapping) bool { return aws.ToString(am.ApiId) == apiID })
		if !matched {
			complete = complete && mappingsComplete
			continue
		}
		for _, dcfg := range d.DomainNameConfigurations {
			if dcfg.CertificateArn != nil && *dcfg.CertificateArn != "" {
				refs = append(refs, *dcfg.CertificateArn)
			}
		}
	}
	ids, dropped := resolveRefs("acm", refs, refContext(clients, cache, "acm"))
	if len(ids) == 0 {
		// Every domain refused its read: nothing was established about any of
		// them, which is a fetch failure rather than a lower bound over what
		// was read.
		if aggErr := AggregateFailures("apigw-related: GetApiMappings", failures, total); aggErr != nil && len(failures) == total {
			return ReadFailed("acm", aggErr)
		}
	}
	// Some GetApiMappings calls may have failed: ids is a proven subset, not
	// necessarily exhaustive. Truncated (not Errored) keeps the row
	// actionable rather than discarding confirmed matches as a dead end.
	return relatedResultTrunc("acm", ids, dropped || len(failures) > 0 || !complete)
}

// checkApigwAlarm reports CloudWatch alarms on this API. API Gateway alarms
// use dimension "ApiId". Scans the alarm cache.
func checkApigwAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "apigw", res)
}

// checkApigwCF reports CloudFront distributions fronting this API: those
// with an origin whose host is the API's own invoke host,
// "<api-id>.execute-api.<region>.amazonaws.com", whose leading label is the
// API's id.
func checkApigwCF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return foundNone("cf", "apiID")
	}

	cfList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cf")
	if err != nil {
		return ReadFailed("cf", err)
	}
	if cfList == nil {
		return NotRead("cf")
	}

	var ids []string
	for _, cfRes := range cfList {
		dist, ok := assertStruct[cftypes.DistributionSummary](cfRes.RawStruct)
		if !ok || dist.Origins == nil {
			continue
		}
		if slices.ContainsFunc(dist.Origins.Items, func(o cftypes.Origin) bool {
			return executeAPIHostID(canonicalDNS(aws.ToString(o.DomainName))) == apiID
		}) {
			ids = append(ids, cfRes.ID)
		}
	}
	return relatedResultTrunc("cf", ids, truncated)
}

// checkApigwELB reports the load balancers behind this API's private
// integrations. "For an HTTP API private integration, specify the ARN of an
// Application Load Balancer listener, Network Load Balancer listener, or AWS
// Cloud Map service" in IntegrationUri; a listener belongs to one load
// balancer, and a Cloud Map service names none, which leaves the count a
// lower bound.
// https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis-apiid-integrations.html
func checkApigwELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return foundNone("elb", "apiID")
	}
	items, complete, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return NotRead("elb")
		}
		return ReadFailed("elb", err)
	}
	var refs []string
	for _, item := range items {
		if item.ConnectionType == apigwtypes.ConnectionTypeVpcLink {
			refs = append(refs, aws.ToString(item.IntegrationUri))
		}
	}
	return listedRelated(ctx, clients, cache, "elb", refs, !complete)
}

// checkApigwRole reports IAM roles this API assumes to call the integration
// target or to run a request authorizer. Reuses the
// apigatewayv2:GetIntegrations call (Integration.CredentialsArn) already
// made by the lambda/kms/sfn/sns pivots, plus one apigatewayv2:GetAuthorizers
// call (Authorizer.AuthorizerCredentialsArn) — role ARNs reduced to bare
// RoleName so the role cache's FetchByIDs resolves them.
func checkApigwRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return foundNone("role", "apiID")
	}

	var refs []string
	rc := refContext(clients, cache, "role")

	items, complete, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil && !errors.Is(err, errClientMissing) {
		return ReadFailed("role", err)
	}
	for _, item := range items {
		if item.CredentialsArn != nil && *item.CredentialsArn != "" {
			refs = append(refs, *item.CredentialsArn)
		}
	}

	// Every branch below that bails out with only the CredentialsArn-derived
	// refs (never having reached GetAuthorizers) reports it as a
	// truncated lower bound, not an exact count: authorizer-credential roles
	// may still exist and were never checked.
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		if len(refs) > 0 {
			ids, _ := resolveRefs("role", refs, rc)
			return relatedResultTrunc("role", ids, true)
		}
		return NotRead("role")
	}
	authAPI, ok := c.APIGatewayV2.(APIGatewayV2GetAuthorizersAPI)
	if !ok {
		if len(refs) > 0 {
			ids, _ := resolveRefs("role", refs, rc)
			return relatedResultTrunc("role", ids, true)
		}
		return NotRead("role")
	}
	authorizers, authComplete, authErr := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.Authorizer, *string, error) {
		out, err := authAPI.GetAuthorizers(ctx, &apigatewayv2.GetAuthorizersInput{ApiId: &apiID, NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.NextToken, nil
	})
	if authErr != nil {
		if len(refs) > 0 {
			ids, _ := resolveRefs("role", refs, rc)
			return relatedResultTrunc("role", ids, true)
		}
		return ReadFailed("role", authErr)
	}
	for _, a := range authorizers {
		if a.AuthorizerCredentialsArn != nil && *a.AuthorizerCredentialsArn != "" {
			refs = append(refs, *a.AuthorizerCredentialsArn)
		}
	}

	ids, dropped := resolveRefs("role", refs, rc)
	return relatedResultTrunc("role", ids, dropped || !complete || !authComplete)
}

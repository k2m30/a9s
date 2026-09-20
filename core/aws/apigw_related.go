// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// apigw_related.go contains API Gateway related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
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
		return resource.ProvenZero("kms", "apiID")
	}
	items, complete, err := apigwListIntegrations(ctx, clients, apiID)
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
	ids, lowerBound, err := kmsResolve(ctx, clients, cache, refs)
	if len(ids) == 0 {
		// Nothing was confirmed: any failures are a plain fetch failure, not
		// a truncation signal (there is no larger population left unseen).
		if aggErr := AggregateFailures("apigw-related: GetFunction", failures, total); aggErr != nil {
			return resource.ErrorRelated("kms", aggErr)
		}
		if err != nil {
			return resource.ErrorRelated("kms", err)
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
		return resource.ProvenZero("logs", "apiName")
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
		return resource.ProvenZero("lambda", "apiID")
	}
	items, complete, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.UnknownRelated("lambda")
		}
		return resource.ErrorRelated("lambda", err)
	}
	var arns []string
	for _, item := range items {
		arns = append(arns, lambdaIntegrationARN(aws.ToString(item.IntegrationUri)))
	}
	ids, dropped := resolveRefs("lambda", arns, refContext(clients, cache, "lambda"))
	return relatedResultTrunc("lambda", ids, dropped || !complete)
}

// checkApigwACM reports ACM certificates attached to this API's custom domain names.
// Enumerates GetDomainNames, then per domain calls GetApiMappings to check if the
// domain maps to this API. For matching domains, harvests CertificateArn from each
// DomainNameConfiguration.
func checkApigwACM(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.ProvenZero("acm", "apiID")
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
	domains, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.DomainName, *string, error) {
		out, err := dnAPI.GetDomainNames(ctx, &apigatewayv2.GetDomainNamesInput{NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.NextToken, nil
	})
	if err != nil {
		return resource.ErrorRelated("acm", err)
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
		// Nothing was confirmed: any failures are a plain fetch failure, not
		// a truncation signal (there is no larger population left unseen).
		if aggErr := AggregateFailures("apigw-related: GetApiMappings", failures, total); aggErr != nil {
			return resource.ErrorRelated("acm", aggErr)
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
		return resource.ProvenZero("cf", "apiID")
	}

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
		if slices.ContainsFunc(dist.Origins.Items, func(o cftypes.Origin) bool {
			return executeAPIHostID(canonicalDNS(aws.ToString(o.DomainName))) == apiID
		}) {
			ids = append(ids, cfRes.ID)
		}
	}
	return relatedResultTrunc("cf", ids, truncated)
}

// checkApigwELB reports the Network Load Balancer behind this API's VPC
// link. Uses apigatewayv2:GetIntegrations to find VPC_LINK integrations
// and their ConnectionId (the VpcLink ID), apigatewayv2:GetVpcLinks
// (account-wide, not API-scoped) to resolve that VpcLink's subnet/security-
// group set, then intersect against the already-loaded elb cache's NLBs by
// AvailabilityZones[].SubnetId / SecurityGroups membership.
func checkApigwELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return resource.ProvenZero("elb", "apiID")
	}

	items, integrationsComplete, err := apigwListIntegrations(ctx, clients, apiID)
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
		return resource.ProvenZero("elb", "vpcLinkIDs")
	}

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		return resource.UnknownRelated("elb")
	}
	vpcLinkAPI, ok := c.APIGatewayV2.(APIGatewayV2GetVpcLinksAPI)
	if !ok {
		return resource.UnknownRelated("elb")
	}

	// GetVpcLinks is account-wide, not API-scoped.
	links, linksComplete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.VpcLink, *string, error) {
		out, callErr := vpcLinkAPI.GetVpcLinks(ctx, &apigatewayv2.GetVpcLinksInput{NextToken: token})
		if callErr != nil {
			return nil, nil, callErr
		}
		return out.Items, out.NextToken, nil
	})
	if err != nil {
		return resource.ErrorRelated("elb", err)
	}
	wantedSubnets := make(map[string]struct{})
	wantedSGs := make(map[string]struct{})
	for _, link := range links {
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
	complete := integrationsComplete && linksComplete
	if len(wantedSubnets) == 0 && len(wantedSGs) == 0 {
		return relatedResultTrunc("elb", nil, !complete)
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
	return relatedResultTrunc("elb", ids, truncated || !complete)
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
		return resource.ProvenZero("role", "apiID")
	}

	var refs []string
	rc := refContext(clients, cache, "role")

	items, complete, err := apigwListIntegrations(ctx, clients, apiID)
	if err != nil && !errors.Is(err, errClientMissing) {
		return resource.ErrorRelated("role", err)
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
		return resource.UnknownRelated("role")
	}
	authAPI, ok := c.APIGatewayV2.(APIGatewayV2GetAuthorizersAPI)
	if !ok {
		if len(refs) > 0 {
			ids, _ := resolveRefs("role", refs, rc)
			return relatedResultTrunc("role", ids, true)
		}
		return resource.UnknownRelated("role")
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
		return resource.ErrorRelated("role", authErr)
	}
	for _, a := range authorizers {
		if a.AuthorizerCredentialsArn != nil && *a.AuthorizerCredentialsArn != "" {
			refs = append(refs, *a.AuthorizerCredentialsArn)
		}
	}

	ids, dropped := resolveRefs("role", refs, rc)
	return relatedResultTrunc("role", ids, dropped || !complete || !authComplete)
}

// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// apigw_related.go contains API Gateway related-resource checker functions.
package aws

import (
	"context"
	"errors"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwv1types "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkApigwKMS resolves KMS keys referenced by this API's Lambda integrations.
// API Gateway has no direct KMS field; Lambda integrations are followed as a
// best effort: the integrations read once, then one GetFunction per function.
// Extracts KMSKeyArn from each Lambda integration's FunctionConfiguration.
func checkApigwKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return keyMissing("kms", "apiID")
	}
	items, complete, err := apigwIntegrations(ctx, clients, res)
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
		fn, ok := resource.ResolveRef("lambda", lambdaIntegrationARN(item.uri), lambdaRC)
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
		return keyMissing("logs", "apiName")
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

// apigwIntegration is one integration of an API, whichever API Gateway
// holds it: the backend it calls, the role it calls it as, and, for a private
// integration, the load balancers the VPC link reaches.
type apigwIntegration struct {
	uri, credentials, vpcLink string
	loadBalancers             []string
}

// apigwIntegrations reads the API's integrations where AWS keeps them. A REST
// API's live in API Gateway v1, on each method of each resource
// (GetResources with the "methods" embed,
// https://docs.aws.amazon.com/apigateway/latest/api/API_GetResources.html),
// and a private one reaches the targetArns of its VPC link. An HTTP or
// WebSocket API's are apigatewayv2:GetIntegrations, and a private one's
// IntegrationUri is the listener it forwards to. complete is false when a walk
// stopped at its cap.
func apigwIntegrations(ctx context.Context, clients any, res resource.Resource) ([]apigwIntegration, bool, error) {
	if res.Fields["protocol"] == "REST" {
		return apigwRESTIntegrations(ctx, clients, res.ID)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		return nil, false, errClientMissing
	}
	api, ok := c.APIGatewayV2.(APIGatewayV2GetIntegrationsAPI)
	if !ok {
		return nil, false, errClientMissing
	}
	v2, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.Integration, *string, error) {
		out, err := api.GetIntegrations(ctx, &apigatewayv2.GetIntegrationsInput{ApiId: aws.String(res.ID), NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.NextToken, nil
	})
	var items []apigwIntegration
	for _, i := range v2 {
		item := apigwIntegration{uri: aws.ToString(i.IntegrationUri), credentials: aws.ToString(i.CredentialsArn)}
		if i.ConnectionType == apigwtypes.ConnectionTypeVpcLink {
			item.loadBalancers = []string{item.uri}
		}
		items = append(items, item)
	}
	return items, complete, err
}

// apigwRESTIntegrations reads a REST API's method integrations, each private
// one's load balancers from its VPC link's targetArns
// (https://docs.aws.amazon.com/apigateway/latest/api/API_VpcLink.html).
func apigwRESTIntegrations(ctx context.Context, clients any, apiID string) ([]apigwIntegration, bool, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV1 == nil {
		return nil, false, errClientMissing
	}
	api, ok := c.APIGatewayV1.(APIGatewayV1GetResourcesAPI)
	if !ok {
		return nil, false, errClientMissing
	}
	resources, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwv1types.Resource, *string, error) {
		out, err := api.GetResources(ctx, &apigateway.GetResourcesInput{RestApiId: aws.String(apiID), Embed: []string{"methods"}, Position: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.Position, nil
	})
	if err != nil {
		return nil, false, err
	}
	var items []apigwIntegration
	links := false
	for _, r := range resources {
		for _, m := range r.ResourceMethods {
			i := m.MethodIntegration
			if i == nil {
				continue
			}
			item := apigwIntegration{uri: aws.ToString(i.Uri), credentials: aws.ToString(i.Credentials)}
			if i.ConnectionType == apigwv1types.ConnectionTypeVpcLink {
				item.vpcLink = aws.ToString(i.ConnectionId)
				links = true
			}
			items = append(items, item)
		}
	}
	if !links {
		return items, complete, nil
	}
	linkAPI, ok := c.APIGatewayV1.(APIGatewayV1GetVpcLinksAPI)
	if !ok {
		return nil, false, errClientMissing
	}
	vpcLinks, linksComplete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwv1types.VpcLink, *string, error) {
		out, linkErr := linkAPI.GetVpcLinks(ctx, &apigateway.GetVpcLinksInput{Position: token})
		if linkErr != nil {
			return nil, nil, linkErr
		}
		return out.Items, out.Position, nil
	})
	if err != nil {
		return nil, false, err
	}
	for i := range items {
		if l := slices.IndexFunc(vpcLinks, func(l apigwv1types.VpcLink) bool { return aws.ToString(l.Id) == items[i].vpcLink }); items[i].vpcLink != "" && l >= 0 {
			items[i].loadBalancers = vpcLinks[l].TargetArns
		}
	}
	return items, complete && linksComplete, nil
}

// checkApigwLambda reports the functions this API's integrations invoke:
// integration URIs that are a Lambda invoke ARN.
func checkApigwLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return keyMissing("lambda", "apiID")
	}
	items, complete, err := apigwIntegrations(ctx, clients, res)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return NotRead("lambda")
		}
		return ReadFailed("lambda", err)
	}
	var arns []string
	for _, item := range items {
		arns = append(arns, lambdaIntegrationARN(item.uri))
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
		return keyMissing("acm", "apiID")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.APIGatewayV2 == nil {
		return NotRead("acm")
	}
	dnAPI, ok := c.APIGatewayV2.(APIGatewayV2GetDomainNamesAPI)
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
		mapped := apigwDomainAPIs(ctx, c, *d.DomainName, d.RoutingMode)
		if !slices.Contains(mapped.ids, apiID) {
			if mapped.failed {
				failures = append(failures, FailedCall(*d.DomainName, mapped.failure))
				continue
			}
			complete = complete && !mapped.partial && !mapped.unread
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

// apigwDomainAPIs is the one reader of the APIs a custom domain sends
// traffic to, in the places its routing mode reads: routing rules, API
// mappings, or both, rules first
// (https://docs.aws.amazon.com/apigateway/latest/developerguide/rest-api-routing-mode.html).
// A mode left empty reads both.
func apigwDomainAPIs(ctx context.Context, clients any, domain string, mode apigwtypes.RoutingMode) relatedRead {
	var reads []relatedRead
	if mode != apigwtypes.RoutingModeApiMappingOnly {
		reads = append(reads, apigwDomainRoutingRules(ctx, clients, domain))
	}
	if mode != apigwtypes.RoutingModeRoutingRuleOnly {
		reads = append(reads, apigwDomainMappings(ctx, clients, domain))
	}
	return joinReads(reads...)
}

// apigwDomainRoutingRules reads the APIs a domain's routing rules invoke:
// each rule's actions invoke a REST API stage by InvokeApi.ApiId
// (https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/domainnames-domainname-routingrules.html).
func apigwDomainRoutingRules(ctx context.Context, clients any, domain string) relatedRead {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return unreadBy(errClientMissing)
	}
	api, ok := c.APIGatewayV2.(APIGatewayV2ListRoutingRulesAPI)
	if !ok {
		return unreadBy(errClientMissing)
	}
	rules, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.RoutingRule, *string, error) {
		out, pageErr := api.ListRoutingRules(ctx, &apigatewayv2.ListRoutingRulesInput{DomainName: aws.String(domain), NextToken: token})
		if pageErr != nil {
			return nil, nil, pageErr
		}
		return out.RoutingRules, out.NextToken, nil
	})
	if err != nil {
		return unreadBy(err)
	}
	read := relatedRead{partial: !complete}
	for _, rule := range rules {
		for _, action := range rule.Actions {
			if action.InvokeApi != nil {
				read.ids = append(read.ids, aws.ToString(action.InvokeApi.ApiId))
			}
		}
	}
	return read
}

// apigwDomainMappings reads the APIs a domain's API mappings name. API
// mappings connect HTTP and REST API stages to a domain
// (https://docs.aws.amazon.com/apigateway/latest/developerguide/rest-api-mappings.html,
// https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/domainnames-domainname-apimappings.html).
// An edge-optimized domain maps REST APIs by base path mapping
// (https://docs.aws.amazon.com/apigateway/latest/developerguide/how-to-edge-optimized-custom-domain-name.html,
// https://docs.aws.amazon.com/apigateway/latest/api/API_GetBasePathMappings.html),
// which no page says GetApiMappings returns, so a domain the v2 read maps
// nothing on is read again through them.
func apigwDomainMappings(ctx context.Context, clients any, domain string) relatedRead {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return unreadBy(errClientMissing)
	}
	mapAPI, ok := c.APIGatewayV2.(APIGatewayV2GetApiMappingsAPI)
	if !ok {
		return unreadBy(errClientMissing)
	}
	mappings, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.ApiMapping, *string, error) {
		out, err := mapAPI.GetApiMappings(ctx, &apigatewayv2.GetApiMappingsInput{DomainName: aws.String(domain), NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.NextToken, nil
	})
	if err != nil {
		return unreadBy(err)
	}
	read := relatedRead{partial: !complete}
	for _, m := range mappings {
		read.ids = append(read.ids, aws.ToString(m.ApiId))
	}
	if len(mappings) > 0 {
		return read
	}
	baseAPI, ok := c.APIGatewayV1.(APIGatewayV1GetBasePathMappingsAPI)
	if !ok {
		return unreadBy(errClientMissing)
	}
	bases, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwv1types.BasePathMapping, *string, error) {
		out, pageErr := baseAPI.GetBasePathMappings(ctx, &apigateway.GetBasePathMappingsInput{DomainName: aws.String(domain), Position: token})
		if pageErr != nil {
			return nil, nil, pageErr
		}
		return out.Items, out.Position, nil
	})
	if err != nil {
		return unreadBy(err)
	}
	read = relatedRead{partial: !complete}
	for _, b := range bases {
		read.ids = append(read.ids, aws.ToString(b.RestApiId))
	}
	return read
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
		return keyMissing("cf", "apiID")
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
// lower bound. A REST API's private integration reaches the load balancers
// of its VPC link.
// https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis-apiid-integrations.html
func checkApigwELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	apiID := res.ID
	if apiID == "" {
		return keyMissing("elb", "apiID")
	}
	items, complete, err := apigwIntegrations(ctx, clients, res)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return NotRead("elb")
		}
		return ReadFailed("elb", err)
	}
	var refs []string
	for _, item := range items {
		refs = append(refs, item.loadBalancers...)
	}
	return listedRelated(ctx, clients, cache, "elb", refs, !complete)
}

// checkApigwRole reports IAM roles this API assumes to call an integration's
// backend (the integration's credentials) or to run an authorizer (the
// authorizer's credentials), read from the API Gateway that holds the API.
func checkApigwRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return keyMissing("role", "apiID")
	}
	items, complete, err := apigwIntegrations(ctx, clients, res)
	integrations := relatedRead{partial: !complete}
	if err != nil {
		integrations = unreadBy(err)
	}
	refs, authComplete, err := apigwAuthorizerRoles(ctx, clients, res)
	authorizers := relatedRead{partial: !authComplete}
	if err != nil {
		authorizers = unreadBy(err)
	}
	for _, item := range items {
		refs = append(refs, item.credentials)
	}
	ids, dropped := resolveRefs("role", refs, refContext(clients, cache, "role"))
	return relatedAnswer("role", joinReads(integrations, authorizers, relatedRead{ids: ids, partial: dropped}))
}

// apigwAuthorizerRoles reads the roles the API's authorizers run as: a REST
// API's from API Gateway v1 GetAuthorizers (authorizerCredentials), an HTTP
// or WebSocket API's from apigatewayv2 GetAuthorizers
// (AuthorizerCredentialsArn).
func apigwAuthorizerRoles(ctx context.Context, clients any, res resource.Resource) ([]string, bool, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return nil, false, errClientMissing
	}
	var roles []string
	if res.Fields["protocol"] == "REST" {
		v1, isV1 := c.APIGatewayV1.(APIGatewayV1GetAuthorizersAPI)
		if !isV1 {
			return nil, false, errClientMissing
		}
		auths, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwv1types.Authorizer, *string, error) {
			out, err := v1.GetAuthorizers(ctx, &apigateway.GetAuthorizersInput{RestApiId: aws.String(res.ID), Position: token})
			if err != nil {
				return nil, nil, err
			}
			return out.Items, out.Position, nil
		})
		for _, a := range auths {
			roles = append(roles, aws.ToString(a.AuthorizerCredentials))
		}
		return roles, complete, err
	}
	api, ok := c.APIGatewayV2.(APIGatewayV2GetAuthorizersAPI)
	if !ok {
		return nil, false, errClientMissing
	}
	auths, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]apigwtypes.Authorizer, *string, error) {
		out, err := api.GetAuthorizers(ctx, &apigatewayv2.GetAuthorizersInput{ApiId: aws.String(res.ID), NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.Items, out.NextToken, nil
	})
	for _, a := range auths {
		roles = append(roles, aws.ToString(a.AuthorizerCredentialsArn))
	}
	return roles, complete, err
}

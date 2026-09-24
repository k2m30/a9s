package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwv1types "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type t570DevMappingsDenied struct{ awsclient.APIGatewayV2API }

func (t570DevMappingsDenied) GetApiMappings(context.Context, *apigatewayv2.GetApiMappingsInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApiMappingsOutput, error) {
	return nil, errors.New("AccessDeniedException")
}

func (d t570DevMappingsDenied) ListRoutingRules(ctx context.Context, in *apigatewayv2.ListRoutingRulesInput, opts ...func(*apigatewayv2.Options)) (*apigatewayv2.ListRoutingRulesOutput, error) {
	return d.APIGatewayV2API.(awsclient.APIGatewayV2ListRoutingRulesAPI).ListRoutingRules(ctx, in, opts...)
}

const t570DevDomainARN = "arn:aws:apigateway:us-east-1::/domainnames/" + fixtures.PublicAPIGWDomainName

// ACM names an API Gateway custom domain in InUseBy; the APIs are the ones
// its API mappings name.
func TestT570Dev_ACMAPIGW_FollowsTheDomainsMappings(t *testing.T) {
	c := t570Demo()
	cert := t570List(t, c, "acm")[0]
	c.ACM = &t570ACM{ACMAPI: c.ACM, cert: cert.ID, inUse: []string{t570DevDomainARN}}
	t570Exact(t, t570Pivot(t, c, cert, "acm", "apigw"), fixtures.PublicAPIGWID)
}

func TestT570Dev_ACMAPIGW_DeniedMappingsKeepTheDirectAPI(t *testing.T) {
	c := t570Demo()
	cert := t570List(t, c, "acm")[0]
	c.ACM = &t570ACM{ACMAPI: c.ACM, cert: cert.ID, inUse: []string{
		t570DevDomainARN, "arn:aws:apigateway:us-east-1::/restapis/direct1",
	}}
	c.APIGatewayV2 = t570DevMappingsDenied{c.APIGatewayV2}
	r := t570Pivot(t, c, cert, "acm", "apigw")
	if r.State() != domain.RelatedResolved || !r.Truncated() || r.Failure() == nil {
		t.Fatalf("state %v truncated %v failure %v, want a lower bound carrying the failure", r.State(), r.Truncated(), r.Failure())
	}
	if got := t570IDs(r); len(got) != 1 || got[0] != "direct1" {
		t.Errorf("ResourceIDs = %v, want [direct1]", got)
	}
}

func TestT570Dev_ACMAPIGW_DeniedMappingsAloneAreTheError(t *testing.T) {
	c := t570Demo()
	cert := t570List(t, c, "acm")[0]
	c.ACM = &t570ACM{ACMAPI: c.ACM, cert: cert.ID, inUse: []string{t570DevDomainARN}}
	c.APIGatewayV2 = t570DevMappingsDenied{c.APIGatewayV2}
	if r := t570Pivot(t, c, cert, "acm", "apigw"); r.Err() == nil {
		t.Errorf("state %v, want the mapping read's error", r.State())
	}
}

type t570DevBasePaths struct {
	awsclient.APIGatewayV1API
	restAPI string
}

func (f t570DevBasePaths) GetBasePathMappings(_ context.Context, in *apigateway.GetBasePathMappingsInput, _ ...func(*apigateway.Options)) (*apigateway.GetBasePathMappingsOutput, error) {
	if aws.ToString(in.DomainName) != "edge.acme-corp.com" {
		return &apigateway.GetBasePathMappingsOutput{}, nil
	}
	return &apigateway.GetBasePathMappingsOutput{Items: []apigwv1types.BasePathMapping{{BasePath: aws.String("(none)"), RestApiId: aws.String(f.restAPI), Stage: aws.String("prod")}}}, nil
}

// An edge-optimized domain maps its REST API by base path mapping, which the
// v2 API-mapping read does not name.
func TestT570Dev_ACMAPIGW_EdgeDomainReadsBasePathMappings(t *testing.T) {
	c := t570Demo()
	cert := t570List(t, c, "acm")[0]
	c.ACM = &t570ACM{ACMAPI: c.ACM, cert: cert.ID, inUse: []string{"arn:aws:apigateway:us-east-1::/domainnames/edge.acme-corp.com"}}
	c.APIGatewayV1 = t570DevBasePaths{APIGatewayV1API: c.APIGatewayV1, restAPI: "edgerest01"}
	t570Exact(t, t570Pivot(t, c, cert, "acm", "apigw"), "edgerest01")
}

type t570DevRulesDenied struct{ awsclient.APIGatewayV2API }

func (t570DevRulesDenied) ListRoutingRules(context.Context, *apigatewayv2.ListRoutingRulesInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.ListRoutingRulesOutput, error) {
	return nil, errors.New("AccessDeniedException")
}

func t570DevWildcardCert(t *testing.T, c *awsclient.ServiceClients) resource.Resource {
	t.Helper()
	for _, r := range t570List(t, c, "acm") {
		if r.Fields["certificate_arn"] == fixtures.ProdACMCertARN2 || r.ID == fixtures.ProdACMCertARN2 {
			return r
		}
	}
	t.Fatalf("no acm row for %s", fixtures.ProdACMCertARN2)
	return resource.Resource{}
}

// A domain in ROUTING_RULE_ONLY mode sends traffic by its routing rules'
// InvokeApi actions; both ends of the certificate-API relation read them.
func TestT570Dev_RoutingRuleDomainCountsOnBothEnds(t *testing.T) {
	c := t570Demo()
	cert := t570DevWildcardCert(t, c)
	t570Contains(t, t570Pivot(t, c, cert, "acm", "apigw"), []string{fixtures.PublicAPIGWID, fixtures.APIGWRESTTracingOff}, nil)
	api := t570Row(t, c, "apigw", fixtures.APIGWRESTTracingOff)
	t570Exact(t, t570Pivot(t, c, api, "apigw", "acm"), cert.ID)
}

// A refused routing-rules read keeps the APIs the mappings name as a lower
// bound and carries the refusal.
func TestT570Dev_RoutingRulesRefusedKeepTheMappings(t *testing.T) {
	c := t570Demo()
	cert := t570DevWildcardCert(t, c)
	c.APIGatewayV2 = t570DevRulesDenied{c.APIGatewayV2}
	r := t570Pivot(t, c, cert, "acm", "apigw")
	if r.State() != domain.RelatedResolved || !r.Truncated() || r.Failure() == nil {
		t.Fatalf("state %v truncated %v failure %v, want a lower bound carrying the failure", r.State(), r.Truncated(), r.Failure())
	}
	if got := t570IDs(r); len(got) != 1 || got[0] != fixtures.PublicAPIGWID {
		t.Errorf("ResourceIDs = %v, want [%s]", got, fixtures.PublicAPIGWID)
	}
}

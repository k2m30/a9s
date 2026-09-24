package unit_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// acmDescribeCertMock implements awsclient.ACMAPI for related-checker tests.
// It embeds the interface to satisfy unused methods and overrides DescribeCertificate.
type acmDescribeCertMock struct {
	awsclient.ACMAPI
	// inUseBy is the slice returned in Certificate.InUseBy.
	inUseBy []string
	// domainValidationOptions is the slice returned in Certificate.DomainValidationOptions.
	domainValidationOptions []acmtypes.DomainValidation
}

func (m *acmDescribeCertMock) DescribeCertificate(
	_ context.Context,
	_ *acm.DescribeCertificateInput,
	_ ...func(*acm.Options),
) (*acm.DescribeCertificateOutput, error) {
	return &acm.DescribeCertificateOutput{
		Certificate: &acmtypes.CertificateDetail{
			InUseBy:                 m.inUseBy,
			DomainValidationOptions: m.domainValidationOptions,
		},
	}, nil
}

var _ awsclient.ACMAPI = (*acmDescribeCertMock)(nil)

// acmRelatedClients returns a *awsclient.ServiceClients with ACM set to mock.
func acmRelatedClients(mock awsclient.ACMAPI) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{ACM: mock}
}

// TestRelated_ACM_ELB_ALBShape: an ALB ARN (:loadbalancer/app/<name>/<id>)
// produces the load balancer name as the resource ID.
func TestRelated_ACM_ELB_ALBShape(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-alb"
	source := resource.Resource{
		ID:   certARN,
		Name: "example.com",
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
			DomainName:     aws.String("example.com"),
		},
	}

	albARN := "arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/app/my-alb/1234567890abcdef"
	mock := &acmDescribeCertMock{inUseBy: []string{albARN}}
	clients := acmRelatedClients(mock)

	checker := acmCheckerByTarget(t, "elb")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (ALB in InUseBy)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-alb" {
		t.Errorf("ResourceIDs = %v, want [my-alb]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

// TestRelated_ACM_ELB_NLBShape: an NLB ARN (:loadbalancer/net/<name>/<id>)
// produces the load balancer name.
func TestRelated_ACM_ELB_NLBShape(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-nlb"
	source := resource.Resource{
		ID:   certARN,
		Name: "example.com",
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}

	nlbARN := "arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/net/my-nlb/abcdef1234567890"
	mock := &acmDescribeCertMock{inUseBy: []string{nlbARN}}
	clients := acmRelatedClients(mock)

	checker := acmCheckerByTarget(t, "elb")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (NLB in InUseBy)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-nlb" {
		t.Errorf("ResourceIDs = %v, want [my-nlb]", result.ResourceIDs())
	}
}

// TestRelated_ACM_ELB_ClassicShape: a Classic Load Balancer ARN
// (:loadbalancer/<name>, no /app/ or /net/ segment) names no row of the elb
// list, which holds ELBv2 load balancers only (DescribeLoadBalancers of
// elasticloadbalancingv2), so the certificate's ELB row is 0.
func TestRelated_ACM_ELB_ClassicShape(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-classic"
	source := resource.Resource{
		ID:   certARN,
		Name: "example.com",
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}

	classicARN := "arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/my-classic-elb"
	mock := &acmDescribeCertMock{inUseBy: []string{classicARN}}
	clients := acmRelatedClients(mock)

	checker := acmCheckerByTarget(t, "elb")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 0 || len(result.ResourceIDs()) != 0 {
		t.Errorf("Count = %d, ResourceIDs = %v, want none (a Classic Load Balancer is not an elb row)", result.Count(), result.ResourceIDs())
	}
}

// TestRelated_ACM_ELB_NonLBARNSkipped: an InUseBy ARN that is not a
// :loadbalancer/ ARN (e.g. an API Gateway domain) produces Count:0.
func TestRelated_ACM_ELB_NonLBARNSkipped(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-nolb"
	source := resource.Resource{
		ID: certARN,
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}

	apigwARN := "arn:aws:apigateway:us-east-1::/domainnames/api.example.com"
	mock := &acmDescribeCertMock{inUseBy: []string{apigwARN}}
	clients := acmRelatedClients(mock)

	checker := acmCheckerByTarget(t, "elb")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no LB ARN in InUseBy)", result.Count())
	}
}

// acmAPIGWDomainSource is a certificate whose InUseBy names one API Gateway
// custom domain, with the clients the acm -> apigw checker reads through.
func acmAPIGWDomainSource(v2 awsclient.APIGatewayV2API) (resource.Resource, *awsclient.ServiceClients) {
	const certARN = "arn:aws:acm:us-east-1:123456789012:certificate/abc-apigw"
	source := resource.Resource{
		ID: certARN,
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}
	clients := acmRelatedClients(&acmDescribeCertMock{inUseBy: []string{"arn:aws:apigateway:us-east-1::/domainnames/api.example.com"}})
	clients.APIGatewayV2 = v2
	return source, clients
}

// acmAPIGWMappingsDenied refuses GetApiMappings the way IAM does for a role
// without apigateway:GET on the domain.
type acmAPIGWMappingsDenied struct{ *fakeAPIGWV2ACM }

func (acmAPIGWMappingsDenied) GetApiMappings(context.Context, *apigatewayv2.GetApiMappingsInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApiMappingsOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "User is not authorized to perform: apigateway:GET"}
}

// TestRelated_ACM_APIGW_DomainnamesARN: InUseBy names a custom domain, not an
// API; the APIs served with the certificate are the ones the domain's API
// mappings name
// (https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/domainnames-domainname-apimappings.html).
func TestRelated_ACM_APIGW_DomainnamesARN(t *testing.T) {
	v2 := &fakeAPIGWV2ACM{mappings: map[string][]apigwv2types.ApiMapping{
		"api.example.com": {
			{ApiId: aws.String("a1b2c3d4e5"), ApiMappingKey: aws.String("orders"), Stage: aws.String("prod")},
			{ApiId: aws.String("f6g7h8i9j0"), ApiMappingKey: aws.String("users"), Stage: aws.String("prod")},
		},
		"other.example.com": {
			{ApiId: aws.String("zzzzzzzzzz"), Stage: aws.String("prod")},
		},
	}}
	source, clients := acmAPIGWDomainSource(v2)

	result := acmCheckerByTarget(t, "apigw")(context.Background(), clients, source, resource.ResourceCache{})

	if result.State() != domain.RelatedResolved || result.Err() != nil {
		t.Fatalf("State = %v, Err = %v, want resolved with no error", result.State(), result.Err())
	}
	got := result.ResourceIDs()
	sort.Strings(got)
	if strings.Join(got, ",") != "a1b2c3d4e5,f6g7h8i9j0" {
		t.Errorf("ResourceIDs = %v, want [a1b2c3d4e5 f6g7h8i9j0] (the APIs mapped on api.example.com)", got)
	}
	if result.Truncated() {
		t.Error("Truncated = true, want false: every mapping page was read")
	}
}

// TestRelated_ACM_APIGW_DomainnamesARN_NoAPIGatewayClient: without an API
// Gateway client the domain's mappings are not read, so the APIs it serves are
// unknown, not a proven 0.
func TestRelated_ACM_APIGW_DomainnamesARN_NoAPIGatewayClient(t *testing.T) {
	source, clients := acmAPIGWDomainSource(nil)

	result := acmCheckerByTarget(t, "apigw")(context.Background(), clients, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v (IDs %v), want RelatedUnknown: the domain's mappings were not read", result.State(), result.ResourceIDs())
	}
}

// TestRelated_ACM_APIGW_DomainnamesARN_MappingsRefused: a refused
// GetApiMappings is a failed read of the only place the APIs are named, so the
// row is an error carrying AWS's error.
func TestRelated_ACM_APIGW_DomainnamesARN_MappingsRefused(t *testing.T) {
	source, clients := acmAPIGWDomainSource(acmAPIGWMappingsDenied{&fakeAPIGWV2ACM{}})

	result := acmCheckerByTarget(t, "apigw")(context.Background(), clients, source, resource.ResourceCache{})

	if result.State() != domain.RelatedError {
		t.Errorf("State = %v (IDs %v), want RelatedError", result.State(), result.ResourceIDs())
	}
	if result.Err() == nil || !strings.Contains(result.Err().Error(), "AccessDeniedException") {
		t.Errorf("Err = %v, want the AccessDeniedException from GetApiMappings", result.Err())
	}
}

// TestRelated_ACM_APIGW_RestapisARN: an InUseBy ARN containing /restapis/
// yields the rest API ID (segment after "restapis").
func TestRelated_ACM_APIGW_RestapisARN(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-restapi"
	source := resource.Resource{
		ID: certARN,
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}

	restapiARN := "arn:aws:apigateway:us-east-1::/restapis/abc123xyz"
	mock := &acmDescribeCertMock{inUseBy: []string{restapiARN}}
	clients := acmRelatedClients(mock)

	checker := acmCheckerByTarget(t, "apigw")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (restapis ARN in InUseBy)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "abc123xyz" {
		t.Errorf("ResourceIDs = %v, want [abc123xyz]", result.ResourceIDs())
	}
}

// TestRelated_ACM_APIGW_NoMatchARN: an InUseBy ARN that is not a
// /domainnames/ or /restapis/ ARN produces Count:0.
func TestRelated_ACM_APIGW_NoMatchARN(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-noapigw"
	source := resource.Resource{
		ID: certARN,
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}

	lbARN := "arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/app/my-alb/1234567890abcdef"
	mock := &acmDescribeCertMock{inUseBy: []string{lbARN}}
	clients := acmRelatedClients(mock)

	checker := acmCheckerByTarget(t, "apigw")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (LB ARN does not match APIGW patterns)", result.Count())
	}
}

// makeACMR53Zone returns a resource.Resource representing a Route 53 hosted zone
// with the zone name in Fields["name"] (as read by checkACMR53).
func makeACMR53Zone(id, zoneName string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   zoneName,
		Fields: map[string]string{"name": zoneName},
	}
}

// TestRelated_ACM_R53_ZoneSuffixMatch: a DNS validation record name whose
// suffix matches the zone name produces the zone ID in ResourceIDs.
func TestRelated_ACM_R53_ZoneSuffixMatch(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-r53"
	source := resource.Resource{
		ID: certARN,
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}

	// Validation record name: _abc123.example.com → matches zone "example.com"
	validationRecordName := "_abc123.example.com"
	mock := &acmDescribeCertMock{
		domainValidationOptions: []acmtypes.DomainValidation{
			{
				ResourceRecord: &acmtypes.ResourceRecord{
					Name: aws.String(validationRecordName),
				},
			},
		},
	}
	clients := acmRelatedClients(mock)

	zone := makeACMR53Zone("/hostedzone/Z0EXAMPLEZONE01", "example.com")
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zone}},
	}

	checker := acmCheckerByTarget(t, "r53")
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (validation record suffix matches zone)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != zone.ID {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs(), zone.ID)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

// TestRelated_ACM_R53_ZoneNoMatch: when no zone suffix matches the validation
// record name, Count is 0.
func TestRelated_ACM_R53_ZoneNoMatch(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-r53-nm"
	source := resource.Resource{
		ID: certARN,
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}

	mock := &acmDescribeCertMock{
		domainValidationOptions: []acmtypes.DomainValidation{
			{
				ResourceRecord: &acmtypes.ResourceRecord{
					Name: aws.String("_abc123.example.com"),
				},
			},
		},
	}
	clients := acmRelatedClients(mock)

	unrelatedZone := makeACMR53Zone("/hostedzone/Z0OTHER", "other.io")
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{unrelatedZone}},
	}

	checker := acmCheckerByTarget(t, "r53")
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (unrelated zone in cache)", result.Count())
	}
}

// TestRelated_ACM_R53_NoCertDomainValidation: when DescribeCertificate returns
// an empty DomainValidationOptions slice, Count is 0 (nothing to match).
func TestRelated_ACM_R53_NoCertDomainValidation(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-r53-nodv"
	source := resource.Resource{
		ID: certARN,
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}

	mock := &acmDescribeCertMock{domainValidationOptions: nil}
	clients := acmRelatedClients(mock)

	zone := makeACMR53Zone("/hostedzone/Z0EXAMPLEZONE02", "example.com")
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zone}},
	}

	checker := acmCheckerByTarget(t, "r53")
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no validation records)", result.Count())
	}
}

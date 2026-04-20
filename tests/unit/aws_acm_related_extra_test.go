package unit_test

// aws_acm_related_extra_test.go — coverage restoration for acm_related.go:
//   checkACMELB (ALB/NLB ARN shape + Classic ELB shape + non-LB ARN skipped)
//   checkACMAPIGW (domainnames ARN + restapis ARN)
//   checkACMR53 (zone-suffix match from cache, no-match from cache)
//
// checkEIPECS / checkEIPECSSvc / checkEIPECSTask / checkEIPLogs are genuine
// stubs that unconditionally return Count:-1 for non-empty IDs; they are
// intentionally not tested here.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
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

// Compile-time: acmDescribeCertMock satisfies ACMAPI.
var _ awsclient.ACMAPI = (*acmDescribeCertMock)(nil)

// acmRelatedClients returns a *awsclient.ServiceClients with ACM set to mock.
func acmRelatedClients(mock awsclient.ACMAPI) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{ACM: mock}
}

// ---------------------------------------------------------------------------
// checkACMELB — ARN parsing for ALB/NLB and Classic ELB shapes
// ---------------------------------------------------------------------------

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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (ALB in InUseBy)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-alb" {
		t.Errorf("ResourceIDs = %v, want [my-alb]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (NLB in InUseBy)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-nlb" {
		t.Errorf("ResourceIDs = %v, want [my-nlb]", result.ResourceIDs)
	}
}

// TestRelated_ACM_ELB_ClassicShape: a Classic ELB ARN
// (:loadbalancer/<name>, no /app/ or /net/ segment) produces the name.
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (classic ELB in InUseBy)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-classic-elb" {
		t.Errorf("ResourceIDs = %v, want [my-classic-elb]", result.ResourceIDs)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no LB ARN in InUseBy)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkACMAPIGW — domainnames and restapis ARN parsing
// ---------------------------------------------------------------------------

// TestRelated_ACM_APIGW_DomainnamesARN: an InUseBy ARN containing
// /domainnames/ yields the domain name (last segment after final "/").
func TestRelated_ACM_APIGW_DomainnamesARN(t *testing.T) {
	const certARN = "arn:aws:acm:us-east-1:111122223333:certificate/abc-apigw"
	source := resource.Resource{
		ID: certARN,
		RawStruct: acmtypes.CertificateSummary{
			CertificateArn: aws.String(certARN),
		},
	}

	domainARN := "arn:aws:apigateway:us-east-1::/domainnames/api.example.com"
	mock := &acmDescribeCertMock{inUseBy: []string{domainARN}}
	clients := acmRelatedClients(mock)

	checker := acmCheckerByTarget(t, "apigw")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (domainnames ARN in InUseBy)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "api.example.com" {
		t.Errorf("ResourceIDs = %v, want [api.example.com]", result.ResourceIDs)
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (restapis ARN in InUseBy)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abc123xyz" {
		t.Errorf("ResourceIDs = %v, want [abc123xyz]", result.ResourceIDs)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (LB ARN does not match APIGW patterns)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkACMR53 — zone-suffix matching from R53 zone cache
// ---------------------------------------------------------------------------

// makeACMR53Zone returns a resource.Resource representing a Route 53 hosted zone
// with the zone name in Fields["name"] (as read by checkACMR53).
func makeACMR53Zone(id, zoneName string) resource.Resource {
	return resource.Resource{
		ID:   id,
		Name: zoneName,
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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (validation record suffix matches zone)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != zone.ID {
		t.Errorf("ResourceIDs = %v, want [%q]", result.ResourceIDs, zone.ID)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (unrelated zone in cache)", result.Count)
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

	// No DomainValidationOptions — empty slice.
	mock := &acmDescribeCertMock{domainValidationOptions: nil}
	clients := acmRelatedClients(mock)

	zone := makeACMR53Zone("/hostedzone/Z0EXAMPLEZONE02", "example.com")
	cache := resource.ResourceCache{
		"r53": resource.ResourceCacheEntry{Resources: []resource.Resource{zone}},
	}

	checker := acmCheckerByTarget(t, "r53")
	result := checker(context.Background(), clients, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no validation records)", result.Count)
	}
}

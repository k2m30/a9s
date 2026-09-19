package unit

// NoSuchBucketPolicy, NoSuchCORSConfiguration and
// NoSuchLifecycleConfiguration mean the bucket has no such configuration,
// and a cross-region rejection (PermanentRedirect,
// IllegalLocationConstraintException) is no failure of the bucket: either
// leaves that field nil with no error while the sibling calls still populate
// theirs.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type enrichS3Fake struct {
	getPolicyFn    func(*s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error)
	getCorsFn      func(*s3.GetBucketCorsInput) (*s3.GetBucketCorsOutput, error)
	getLifecycleFn func(*s3.GetBucketLifecycleConfigurationInput) (*s3.GetBucketLifecycleConfigurationOutput, error)
}

func (f *enrichS3Fake) GetBucketPolicy(_ context.Context, in *s3.GetBucketPolicyInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	if f.getPolicyFn != nil {
		return f.getPolicyFn(in)
	}
	return &s3.GetBucketPolicyOutput{}, nil
}
func (f *enrichS3Fake) GetBucketCors(_ context.Context, in *s3.GetBucketCorsInput, _ ...func(*s3.Options)) (*s3.GetBucketCorsOutput, error) {
	if f.getCorsFn != nil {
		return f.getCorsFn(in)
	}
	return &s3.GetBucketCorsOutput{}, nil
}
func (f *enrichS3Fake) GetBucketLifecycleConfiguration(_ context.Context, in *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	if f.getLifecycleFn != nil {
		return f.getLifecycleFn(in)
	}
	return &s3.GetBucketLifecycleConfigurationOutput{}, nil
}

func (f *enrichS3Fake) ListBuckets(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return &s3.ListBucketsOutput{}, nil
}
func (f *enrichS3Fake) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}
func (f *enrichS3Fake) GetBucketNotificationConfiguration(_ context.Context, _ *s3.GetBucketNotificationConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return &s3.GetBucketNotificationConfigurationOutput{}, nil
}
func (f *enrichS3Fake) GetPublicAccessBlock(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	return &s3.GetPublicAccessBlockOutput{}, nil
}

var _ awsclient.S3API = (*enrichS3Fake)(nil)
var _ awsclient.S3GetBucketPolicyAPI = (*enrichS3Fake)(nil)
var _ awsclient.S3GetBucketCorsAPI = (*enrichS3Fake)(nil)
var _ awsclient.S3GetBucketLifecycleAPI = (*enrichS3Fake)(nil)

type enrichS3FakeNoBucketAPIs struct{}

func (f *enrichS3FakeNoBucketAPIs) ListBuckets(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return &s3.ListBucketsOutput{}, nil
}
func (f *enrichS3FakeNoBucketAPIs) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}
func (f *enrichS3FakeNoBucketAPIs) GetBucketNotificationConfiguration(_ context.Context, _ *s3.GetBucketNotificationConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return &s3.GetBucketNotificationConfigurationOutput{}, nil
}
func (f *enrichS3FakeNoBucketAPIs) GetPublicAccessBlock(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	return &s3.GetPublicAccessBlockOutput{}, nil
}

var _ awsclient.S3API = (*enrichS3FakeNoBucketAPIs)(nil)

func s3Enricher(t *testing.T) resource.DetailEnricher {
	t.Helper()
	e := resource.GetDetailEnricher("s3")
	if e == nil {
		t.Fatal("s3 detail enricher not registered")
	}
	return e
}

func makeS3Ctx(client awsclient.S3API) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{Clients: &awsclient.ServiceClients{S3: client}}
}

const s3TestBucketName = "acme-app-logs-prod"

const s3TestPolicyJSON = `{"Version":"2012-10-17","Statement":[{"Sid":"AllowCloudFrontOAI","Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:role/cloudfront-oai"},"Action":"s3:GetObject","Resource":"arn:aws:s3:::acme-app-logs-prod/*"}]}`

func makeS3Bucket(name string) s3types.Bucket {
	var namePtr *string
	if name != "" {
		namePtr = aws.String(name)
	}
	return s3types.Bucket{Name: namePtr}
}

func makeS3Res(name string) resource.Resource {
	return resource.Resource{ID: name, RawStruct: makeS3Bucket(name)}
}

func fullS3Fake() *enrichS3Fake {
	return &enrichS3Fake{
		getPolicyFn: func(_ *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
			return &s3.GetBucketPolicyOutput{Policy: aws.String(s3TestPolicyJSON)}, nil
		},
		getCorsFn: func(_ *s3.GetBucketCorsInput) (*s3.GetBucketCorsOutput, error) {
			return &s3.GetBucketCorsOutput{
				CORSRules: []s3types.CORSRule{{
					AllowedMethods: []string{"GET", "PUT"},
					AllowedOrigins: []string{"https://app.example.com"},
					AllowedHeaders: []string{"*"},
					MaxAgeSeconds:  aws.Int32(3000),
				}},
			}, nil
		},
		getLifecycleFn: func(_ *s3.GetBucketLifecycleConfigurationInput) (*s3.GetBucketLifecycleConfigurationOutput, error) {
			return &s3.GetBucketLifecycleConfigurationOutput{
				Rules: []s3types.LifecycleRule{{
					ID:         aws.String("expire-old-logs"),
					Status:     s3types.ExpirationStatusEnabled,
					Expiration: &s3types.LifecycleExpiration{Days: aws.Int32(90)},
				}},
			}, nil
		},
	}
}

func TestEnrichS3_WrongClientsType_ReturnsError(t *testing.T) {
	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	_, err := enricher(context.Background(), "not-a-detail-ctx", res)
	if err == nil {
		t.Fatal("expected error for wrong clients type, got nil")
	}
}

func TestEnrichS3_NilDetailEnrichmentCtx_ReturnsError(t *testing.T) {
	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	_, err := enricher(context.Background(), (*awsclient.DetailEnrichmentCtx)(nil), res)
	if err == nil {
		t.Fatal("expected error for nil DetailEnrichmentCtx, got nil")
	}
}

func TestEnrichS3_NilClients_ReturnsError(t *testing.T) {
	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)
	ctx := &awsclient.DetailEnrichmentCtx{Clients: nil}

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error for nil Clients, got nil")
	}
}

func TestEnrichS3_WrongRawStructType_ReturnsError(t *testing.T) {
	enricher := s3Enricher(t)
	res := resource.Resource{ID: s3TestBucketName, RawStruct: "not-a-bucket"}

	_, err := enricher(context.Background(), makeS3Ctx(fullS3Fake()), res)
	if err == nil {
		t.Fatal("expected error for wrong RawStruct type, got nil")
	}
}

func TestEnrichS3_EmptyBucketName_ReturnsError(t *testing.T) {
	enricher := s3Enricher(t)
	res := makeS3Res("")

	_, err := enricher(context.Background(), makeS3Ctx(fullS3Fake()), res)
	if err == nil {
		t.Fatal("expected error for bucket with no name, got nil")
	}
}

func TestEnrichS3_FullSuccess_AllThreePayloadsAttached(t *testing.T) {
	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	got, err := enricher(context.Background(), makeS3Ctx(fullS3Fake()), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	enriched, ok := got.RawStruct.(awsclient.BucketEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want BucketEnriched", got.RawStruct)
	}

	policy, ok := enriched.Policy.(map[string]any)
	if !ok {
		t.Fatalf("enriched.Policy = %T, want map[string]any (parsed JSON)", enriched.Policy)
	}
	if policy["Version"] != "2012-10-17" {
		t.Errorf("enriched.Policy[Version] = %v, want 2012-10-17", policy["Version"])
	}

	if len(enriched.CORSRules) != 1 || len(enriched.CORSRules[0].AllowedOrigins) != 1 || enriched.CORSRules[0].AllowedOrigins[0] != "https://app.example.com" {
		t.Errorf("enriched.CORSRules = %+v, want one rule allowing https://app.example.com", enriched.CORSRules)
	}

	if len(enriched.LifecycleRules) != 1 || enriched.LifecycleRules[0].ID == nil || *enriched.LifecycleRules[0].ID != "expire-old-logs" {
		t.Errorf("enriched.LifecycleRules = %+v, want one rule named expire-old-logs", enriched.LifecycleRules)
	}
}

func TestEnrichS3_NoSuchBucketPolicy_PolicyNilNoError(t *testing.T) {
	fake := fullS3Fake()
	fake.getPolicyFn = func(_ *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "NoSuchBucketPolicy"}
	}

	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	got, err := enricher(context.Background(), makeS3Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error for absent bucket policy: %v", err)
	}
	enriched := got.RawStruct.(awsclient.BucketEnriched)
	if enriched.Policy != nil {
		t.Errorf("enriched.Policy = %v, want nil for NoSuchBucketPolicy", enriched.Policy)
	}
	if len(enriched.CORSRules) == 0 {
		t.Error("enriched.CORSRules should still be populated when only the policy call is absent")
	}
}

func TestEnrichS3_NoSuchCORSConfiguration_CORSRulesNilNoError(t *testing.T) {
	fake := fullS3Fake()
	fake.getCorsFn = func(_ *s3.GetBucketCorsInput) (*s3.GetBucketCorsOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "NoSuchCORSConfiguration"}
	}

	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	got, err := enricher(context.Background(), makeS3Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error for absent CORS configuration: %v", err)
	}
	enriched := got.RawStruct.(awsclient.BucketEnriched)
	if enriched.CORSRules != nil {
		t.Errorf("enriched.CORSRules = %v, want nil for NoSuchCORSConfiguration", enriched.CORSRules)
	}
	if enriched.Policy == nil {
		t.Error("enriched.Policy should still be populated when only the CORS call is absent")
	}
}

func TestEnrichS3_NoSuchLifecycleConfiguration_LifecycleRulesNilNoError(t *testing.T) {
	fake := fullS3Fake()
	fake.getLifecycleFn = func(_ *s3.GetBucketLifecycleConfigurationInput) (*s3.GetBucketLifecycleConfigurationOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "NoSuchLifecycleConfiguration"}
	}

	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	got, err := enricher(context.Background(), makeS3Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error for absent lifecycle configuration: %v", err)
	}
	enriched := got.RawStruct.(awsclient.BucketEnriched)
	if enriched.LifecycleRules != nil {
		t.Errorf("enriched.LifecycleRules = %v, want nil for NoSuchLifecycleConfiguration", enriched.LifecycleRules)
	}
	if enriched.Policy == nil {
		t.Error("enriched.Policy should still be populated when only the lifecycle call is absent")
	}
}

func TestEnrichS3_PermanentRedirect_PolicyNilNoError(t *testing.T) {
	fake := fullS3Fake()
	fake.getPolicyFn = func(_ *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "PermanentRedirect"}
	}

	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	got, err := enricher(context.Background(), makeS3Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error for cross-region PermanentRedirect: %v", err)
	}
	enriched := got.RawStruct.(awsclient.BucketEnriched)
	if enriched.Policy != nil {
		t.Errorf("enriched.Policy = %v, want nil for PermanentRedirect (cross-region)", enriched.Policy)
	}
	if len(enriched.CORSRules) == 0 {
		t.Error("enriched.CORSRules should still be populated when only the policy call is cross-region")
	}
	if len(enriched.LifecycleRules) == 0 {
		t.Error("enriched.LifecycleRules should still be populated when only the policy call is cross-region")
	}
}

func TestEnrichS3_IllegalLocationConstraint_CORSRulesNilNoError(t *testing.T) {
	fake := fullS3Fake()
	fake.getCorsFn = func(_ *s3.GetBucketCorsInput) (*s3.GetBucketCorsOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "IllegalLocationConstraintException"}
	}

	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	got, err := enricher(context.Background(), makeS3Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error for cross-region IllegalLocationConstraintException: %v", err)
	}
	enriched := got.RawStruct.(awsclient.BucketEnriched)
	if enriched.CORSRules != nil {
		t.Errorf("enriched.CORSRules = %v, want nil for IllegalLocationConstraintException (cross-region)", enriched.CORSRules)
	}
	if enriched.Policy == nil {
		t.Error("enriched.Policy should still be populated when only the CORS call is cross-region")
	}
	if len(enriched.LifecycleRules) == 0 {
		t.Error("enriched.LifecycleRules should still be populated when only the CORS call is cross-region")
	}
}

func TestEnrichS3_AllThreeCallsCrossRegion_WrapperAttachedAllNilNoError(t *testing.T) {
	fake := fullS3Fake()
	fake.getPolicyFn = func(_ *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "PermanentRedirect"}
	}
	fake.getCorsFn = func(_ *s3.GetBucketCorsInput) (*s3.GetBucketCorsOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "IllegalLocationConstraintException"}
	}
	fake.getLifecycleFn = func(_ *s3.GetBucketLifecycleConfigurationInput) (*s3.GetBucketLifecycleConfigurationOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "PermanentRedirect"}
	}

	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	got, err := enricher(context.Background(), makeS3Ctx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error when all three calls are cross-region: %v", err)
	}
	enriched, ok := got.RawStruct.(awsclient.BucketEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want BucketEnriched (wrapper must still attach on an all-cross-region bucket)", got.RawStruct)
	}
	if enriched.Policy != nil {
		t.Errorf("enriched.Policy = %v, want nil", enriched.Policy)
	}
	if enriched.CORSRules != nil {
		t.Errorf("enriched.CORSRules = %v, want nil", enriched.CORSRules)
	}
	if enriched.LifecycleRules != nil {
		t.Errorf("enriched.LifecycleRules = %v, want nil", enriched.LifecycleRules)
	}
}

func TestEnrichS3_RealError_PropagatedAndResUnchanged(t *testing.T) {
	fake := fullS3Fake()
	fake.getPolicyFn = func(_ *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
		return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "not authorized to get bucket policy"}
	}

	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	got, err := enricher(context.Background(), makeS3Ctx(fake), res)
	if err == nil {
		t.Fatal("expected error for AccessDenied (not a benign-absence code), got nil")
	}
	if _, ok := got.RawStruct.(awsclient.BucketEnriched); ok {
		t.Error("res.RawStruct must stay the original type when enrichment fails, got BucketEnriched")
	}
}

func TestEnrichS3_GenericAPIError_Propagated(t *testing.T) {
	fake := fullS3Fake()
	fake.getPolicyFn = func(_ *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
		return nil, errFake("GetBucketPolicy: connection reset")
	}

	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	_, err := enricher(context.Background(), makeS3Ctx(fake), res)
	if err == nil {
		t.Fatal("expected error from non-smithy API failure, got nil")
	}
}

// A client without the per-bucket calls is an error, as for every other
// detail enricher.
func TestEnrichS3_ClientWithoutBucketAPIs_ReturnsError(t *testing.T) {
	enricher := s3Enricher(t)
	res := makeS3Res(s3TestBucketName)

	got, err := enricher(context.Background(), makeS3Ctx(&enrichS3FakeNoBucketAPIs{}), res)
	if err == nil {
		t.Fatal("expected error when S3 client supports none of GetBucketPolicy/GetBucketCors/GetBucketLifecycleConfiguration, got nil")
	}
	if !strings.Contains(err.Error(), "does not support") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "does not support")
	}
	if _, ok := got.RawStruct.(awsclient.BucketEnriched); ok {
		t.Error("res.RawStruct must stay the original type when enrichment fails, got BucketEnriched")
	}
}

func TestEnrichS3_BucketEnrichedRawStruct_Accepted(t *testing.T) {
	enricher := s3Enricher(t)

	res := resource.Resource{
		ID: s3TestBucketName,
		RawStruct: awsclient.BucketEnriched{
			Bucket: makeS3Bucket(s3TestBucketName),
			Policy: "stale-previous-policy",
		},
	}

	got, err := enricher(context.Background(), makeS3Ctx(fullS3Fake()), res)
	if err != nil {
		t.Fatalf("unexpected error on BucketEnriched re-enrichment: %v", err)
	}
	enriched, ok := got.RawStruct.(awsclient.BucketEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want BucketEnriched", got.RawStruct)
	}
	policy, ok := enriched.Policy.(map[string]any)
	if !ok || policy["Version"] != "2012-10-17" {
		t.Errorf("enriched.Policy = %v, want refreshed parsed policy", enriched.Policy)
	}
}

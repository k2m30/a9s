package unit

// aws_lambda_detail_enrich_test.go — coverage for enrichLambda
// (core/aws/lambda_detail_enrichment.go), the on-demand detail enricher
// registered for the "lambda" resource type (#261).
//
// Covers:
//   - wrong clients type / nil DetailEnrichmentCtx / nil Clients → error
//     (lambda has no DetailDocs dependency — uncached, per the contract)
//   - wrong RawStruct type → error
//   - missing function name and ARN → error
//   - FunctionName preferred over FunctionArn when both are present
//   - success: GetFunction's Configuration replaces the embedded list-level
//     config (State/StateReason/LastUpdateStatus land there); Code/Concurrency attached
//   - out.Configuration == nil leaves the original list-level config in place
//   - FunctionEnriched re-enrichment path accepted as RawStruct
//   - API error propagated
//   - registry sanity: GetDetailEnricher("lambda") non-nil

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ---------------------------------------------------------------------------
// enrichLambdaFake — narrow LambdaAPI fake with a call counter
// ---------------------------------------------------------------------------

type enrichLambdaFake struct {
	getFunctionFn    func(*lambda.GetFunctionInput) (*lambda.GetFunctionOutput, error)
	getFunctionCalls int
}

func (f *enrichLambdaFake) GetFunction(_ context.Context, in *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	f.getFunctionCalls++
	if f.getFunctionFn != nil {
		return f.getFunctionFn(in)
	}
	return &lambda.GetFunctionOutput{}, nil
}

func (f *enrichLambdaFake) ListFunctions(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	return &lambda.ListFunctionsOutput{}, nil
}
func (f *enrichLambdaFake) ListEventSourceMappings(_ context.Context, _ *lambda.ListEventSourceMappingsInput, _ ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	return &lambda.ListEventSourceMappingsOutput{}, nil
}
func (f *enrichLambdaFake) ListTags(_ context.Context, _ *lambda.ListTagsInput, _ ...func(*lambda.Options)) (*lambda.ListTagsOutput, error) {
	return &lambda.ListTagsOutput{}, nil
}

var _ awsclient.LambdaAPI = (*enrichLambdaFake)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func lambdaEnricher(t *testing.T) resource.DetailEnricher {
	t.Helper()
	e := resource.GetDetailEnricher("lambda")
	if e == nil {
		t.Fatal("lambda detail enricher not registered")
	}
	return e
}

func makeLambdaCtx(client awsclient.LambdaAPI) *awsclient.DetailEnrichmentCtx {
	return &awsclient.DetailEnrichmentCtx{Clients: &awsclient.ServiceClients{Lambda: client}}
}

const lambdaTestArn = "arn:aws:lambda:us-east-1:123456789012:function:process-payment"
const lambdaTestName = "process-payment"

func makeLambdaCfg(name, arn string) lambdatypes.FunctionConfiguration {
	var namePtr, arnPtr *string
	if name != "" {
		namePtr = aws.String(name)
	}
	if arn != "" {
		arnPtr = aws.String(arn)
	}
	return lambdatypes.FunctionConfiguration{
		FunctionName: namePtr,
		FunctionArn:  arnPtr,
		Runtime:      lambdatypes.RuntimeNodejs20x,
		MemorySize:   aws.Int32(512),
		Timeout:      aws.Int32(30),
	}
}

func makeLambdaRes(name, arn string) resource.Resource {
	return resource.Resource{ID: arn, RawStruct: makeLambdaCfg(name, arn)}
}

// ---------------------------------------------------------------------------
// Tests: invalid context
// ---------------------------------------------------------------------------

func TestEnrichLambda_WrongClientsType_ReturnsError(t *testing.T) {
	enricher := lambdaEnricher(t)
	res := makeLambdaRes(lambdaTestName, lambdaTestArn)

	_, err := enricher(context.Background(), "not-a-detail-ctx", res)
	if err == nil {
		t.Fatal("expected error for wrong clients type, got nil")
	}
}

func TestEnrichLambda_NilDetailEnrichmentCtx_ReturnsError(t *testing.T) {
	enricher := lambdaEnricher(t)
	res := makeLambdaRes(lambdaTestName, lambdaTestArn)

	_, err := enricher(context.Background(), (*awsclient.DetailEnrichmentCtx)(nil), res)
	if err == nil {
		t.Fatal("expected error for nil DetailEnrichmentCtx, got nil")
	}
}

func TestEnrichLambda_NilClients_ReturnsError(t *testing.T) {
	enricher := lambdaEnricher(t)
	res := makeLambdaRes(lambdaTestName, lambdaTestArn)
	ctx := &awsclient.DetailEnrichmentCtx{Clients: nil}

	_, err := enricher(context.Background(), ctx, res)
	if err == nil {
		t.Fatal("expected error for nil Clients, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: bad RawStruct / missing name+ARN
// ---------------------------------------------------------------------------

func TestEnrichLambda_WrongRawStructType_ReturnsError(t *testing.T) {
	enricher := lambdaEnricher(t)
	res := resource.Resource{ID: lambdaTestArn, RawStruct: "not-a-function"}

	_, err := enricher(context.Background(), makeLambdaCtx(&enrichLambdaFake{}), res)
	if err == nil {
		t.Fatal("expected error for wrong RawStruct type, got nil")
	}
}

func TestEnrichLambda_NoNameOrArn_ReturnsError(t *testing.T) {
	enricher := lambdaEnricher(t)
	res := makeLambdaRes("", "")

	_, err := enricher(context.Background(), makeLambdaCtx(&enrichLambdaFake{}), res)
	if err == nil {
		t.Fatal("expected error for function with no name or ARN, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: success — Configuration replaces embedded, Code/Concurrency attached
// ---------------------------------------------------------------------------

func TestEnrichLambda_Success_ConfigurationReplacedCodeAndConcurrencyAttached(t *testing.T) {
	fake := &enrichLambdaFake{
		getFunctionFn: func(_ *lambda.GetFunctionInput) (*lambda.GetFunctionOutput, error) {
			return &lambda.GetFunctionOutput{
				Configuration: &lambdatypes.FunctionConfiguration{
					FunctionName:     aws.String(lambdaTestName),
					FunctionArn:      aws.String(lambdaTestArn),
					Runtime:          lambdatypes.RuntimeNodejs20x,
					State:            lambdatypes.StateActive,
					StateReason:      aws.String("The function is ready to be invoked."),
					LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
				},
				Code: &lambdatypes.FunctionCodeLocation{
					RepositoryType: aws.String("S3"),
					Location:       aws.String("https://prod-lambda-code.s3.amazonaws.com/process-payment.zip?X-Amz-..."),
				},
				Concurrency: &lambdatypes.Concurrency{
					ReservedConcurrentExecutions: aws.Int32(10),
				},
			}, nil
		},
	}

	enricher := lambdaEnricher(t)
	res := makeLambdaRes(lambdaTestName, lambdaTestArn)

	got, err := enricher(context.Background(), makeLambdaCtx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.getFunctionCalls != 1 {
		t.Errorf("GetFunction called %d times, want 1", fake.getFunctionCalls)
	}

	enriched, ok := got.RawStruct.(awsclient.FunctionEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want FunctionEnriched", got.RawStruct)
	}
	if enriched.State != lambdatypes.StateActive {
		t.Errorf("enriched.State = %v, want Active (from GetFunction's Configuration)", enriched.State)
	}
	if enriched.StateReason == nil || *enriched.StateReason != "The function is ready to be invoked." {
		t.Errorf("enriched.StateReason = %v, want the ready reason", enriched.StateReason)
	}
	if enriched.LastUpdateStatus != lambdatypes.LastUpdateStatusSuccessful {
		t.Errorf("enriched.LastUpdateStatus = %v, want Successful", enriched.LastUpdateStatus)
	}
	if enriched.Code == nil || enriched.Code.Location == nil {
		t.Fatal("enriched.Code must carry the deployment package location")
	}
	if enriched.Concurrency == nil || enriched.Concurrency.ReservedConcurrentExecutions == nil || *enriched.Concurrency.ReservedConcurrentExecutions != 10 {
		t.Errorf("enriched.Concurrency = %v, want ReservedConcurrentExecutions=10", enriched.Concurrency)
	}
}

func TestEnrichLambda_NilConfiguration_KeepsOriginalEmbeddedConfig(t *testing.T) {
	fake := &enrichLambdaFake{
		getFunctionFn: func(_ *lambda.GetFunctionInput) (*lambda.GetFunctionOutput, error) {
			return &lambda.GetFunctionOutput{
				Configuration: nil,
				Code:          &lambdatypes.FunctionCodeLocation{RepositoryType: aws.String("S3")},
			}, nil
		},
	}

	enricher := lambdaEnricher(t)
	res := makeLambdaRes(lambdaTestName, lambdaTestArn)

	got, err := enricher(context.Background(), makeLambdaCtx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enriched := got.RawStruct.(awsclient.FunctionEnriched)
	if enriched.FunctionName == nil || *enriched.FunctionName != lambdaTestName {
		t.Errorf("enriched.FunctionName = %v, want original %q preserved when Configuration is nil", enriched.FunctionName, lambdaTestName)
	}
	if enriched.Code == nil {
		t.Error("enriched.Code should still be attached even when Configuration is nil")
	}
}

func TestEnrichLambda_FunctionNamePreferredOverArn(t *testing.T) {
	var seenName *string
	fake := &enrichLambdaFake{
		getFunctionFn: func(in *lambda.GetFunctionInput) (*lambda.GetFunctionOutput, error) {
			seenName = in.FunctionName
			return &lambda.GetFunctionOutput{}, nil
		},
	}

	enricher := lambdaEnricher(t)
	res := makeLambdaRes(lambdaTestName, lambdaTestArn)

	if _, err := enricher(context.Background(), makeLambdaCtx(fake), res); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenName == nil || *seenName != lambdaTestName {
		t.Errorf("GetFunctionInput.FunctionName = %v, want FunctionName %q preferred over ARN", seenName, lambdaTestName)
	}
}

func TestEnrichLambda_FallsBackToArn_WhenNameEmpty(t *testing.T) {
	var seenName *string
	fake := &enrichLambdaFake{
		getFunctionFn: func(in *lambda.GetFunctionInput) (*lambda.GetFunctionOutput, error) {
			seenName = in.FunctionName
			return &lambda.GetFunctionOutput{}, nil
		},
	}

	enricher := lambdaEnricher(t)
	res := makeLambdaRes("", lambdaTestArn)

	if _, err := enricher(context.Background(), makeLambdaCtx(fake), res); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenName == nil || *seenName != lambdaTestArn {
		t.Errorf("GetFunctionInput.FunctionName = %v, want fallback ARN %q", seenName, lambdaTestArn)
	}
}

// ---------------------------------------------------------------------------
// Tests: re-enrichment path
// ---------------------------------------------------------------------------

func TestEnrichLambda_FunctionEnrichedRawStruct_Accepted(t *testing.T) {
	fake := &enrichLambdaFake{
		getFunctionFn: func(_ *lambda.GetFunctionInput) (*lambda.GetFunctionOutput, error) {
			return &lambda.GetFunctionOutput{
				Configuration: &lambdatypes.FunctionConfiguration{
					FunctionName: aws.String(lambdaTestName),
					State:        lambdatypes.StateActive,
				},
			}, nil
		},
	}
	enricher := lambdaEnricher(t)

	res := resource.Resource{
		ID: lambdaTestArn,
		RawStruct: awsclient.FunctionEnriched{
			FunctionConfiguration: makeLambdaCfg(lambdaTestName, lambdaTestArn),
		},
	}

	got, err := enricher(context.Background(), makeLambdaCtx(fake), res)
	if err != nil {
		t.Fatalf("unexpected error on FunctionEnriched re-enrichment: %v", err)
	}
	enriched, ok := got.RawStruct.(awsclient.FunctionEnriched)
	if !ok {
		t.Fatalf("RawStruct = %T, want FunctionEnriched", got.RawStruct)
	}
	if enriched.State != lambdatypes.StateActive {
		t.Errorf("enriched.State = %v, want Active", enriched.State)
	}
}

// ---------------------------------------------------------------------------
// Tests: API error propagation
// ---------------------------------------------------------------------------

func TestEnrichLambda_APIError_Propagated(t *testing.T) {
	fake := &enrichLambdaFake{
		getFunctionFn: func(_ *lambda.GetFunctionInput) (*lambda.GetFunctionOutput, error) {
			return nil, errFake("GetFunction: access denied")
		},
	}
	enricher := lambdaEnricher(t)
	res := makeLambdaRes(lambdaTestName, lambdaTestArn)

	_, err := enricher(context.Background(), makeLambdaCtx(fake), res)
	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: registry sanity
// ---------------------------------------------------------------------------

func TestDetailEnricherRegistry_Lambda_IsNonNil(t *testing.T) {
	e := resource.GetDetailEnricher("lambda")
	if e == nil {
		t.Fatal("lambda detail enricher must be registered and non-nil")
	}
}

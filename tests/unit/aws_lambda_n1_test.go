package unit

// Tests for GitHub issue #221: Lambda paginated fetcher issues N+1 API calls
// (ListEventSourceMappings per function) when it should call none.
//
// These tests FAIL against the current implementation and PASS once the fix
// (use FetchLambdaFunctionsPage instead of FetchLambdaFunctionsPageWithEventSources)
// lands.

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	lambdasdk "github.com/aws/aws-sdk-go-v2/service/lambda"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"

	// Side-effect import: triggers init() which calls resource.RegisterPaginated("lambda", …)
	_ "github.com/k2m30/a9s/v3/internal/aws"
)

// countingLambdaRoundTripper wraps the HTTP transport and counts calls to
// ListEventSourceMappings (identified by the /event-source-mappings/ path).
type countingLambdaRoundTripper struct {
	eventSourceCalls atomic.Int64
	listFunctionsXML string // JSON to return for ListFunctions
}

func (t *countingLambdaRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Lambda REST API: ListEventSourceMappings path is /2015-03-31/event-source-mappings/
	if strings.Contains(req.URL.Path, "event-source-mappings") {
		t.eventSourceCalls.Add(1)
		// Return a valid empty response so the call doesn't error.
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"EventSourceMappings":[]}`)),
		}, nil
	}

	// All other Lambda calls (ListFunctions) get the canned function list.
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(t.listFunctionsXML)),
	}, nil
}

func newLambdaClientWithCountingTransport(transport *countingLambdaRoundTripper) *lambdasdk.Client {
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", "SESSION"),
		HTTPClient:  &http.Client{Transport: transport},
	}
	return lambdasdk.NewFromConfig(cfg)
}

// TestLambdaPaginatedFetcher_DoesNotCallEventSourceAPI verifies that the
// registered paginated fetcher for "lambda" makes zero ListEventSourceMappings
// calls regardless of how many functions are returned.
//
// BUG (issue #221): the current implementation calls
// FetchLambdaFunctionsPageWithEventSources which issues one
// ListEventSourceMappings call per function, turning a single-page list into
// 1+N API calls.
func TestLambdaPaginatedFetcher_DoesNotCallEventSourceAPI(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("lambda")
	if fetcher == nil {
		t.Fatal("paginated fetcher for 'lambda' not registered — ensure internal/aws package is imported")
	}

	listJSON := `{
		"Functions": [
			{
				"FunctionName": "fn-one",
				"Runtime": "go1.x",
				"MemorySize": 128,
				"Timeout": 30,
				"Handler": "bootstrap",
				"LastModified": "2025-01-15T10:00:00.000+0000",
				"CodeSize": 5242880,
				"FunctionArn": "arn:aws:lambda:us-east-1:111122223333:function:fn-one",
				"Role": "arn:aws:iam::111122223333:role/lambda-exec",
				"PackageType": "Zip",
				"Architectures": ["arm64"]
			},
			{
				"FunctionName": "fn-two",
				"Runtime": "python3.12",
				"MemorySize": 256,
				"Timeout": 60,
				"Handler": "index.handler",
				"LastModified": "2025-02-20T12:30:00.000+0000",
				"CodeSize": 1048576,
				"FunctionArn": "arn:aws:lambda:us-east-1:111122223333:function:fn-two",
				"Role": "arn:aws:iam::111122223333:role/lambda-exec",
				"PackageType": "Zip",
				"Architectures": ["x86_64"]
			}
		]
	}`

	transport := &countingLambdaRoundTripper{listFunctionsXML: listJSON}
	lambdaClient := newLambdaClientWithCountingTransport(transport)

	clients := &awsclient.ServiceClients{
		Lambda: lambdaClient,
	}

	result, err := fetcher(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("fetcher returned unexpected error: %v", err)
	}

	if len(result.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(result.Resources))
	}

	calls := transport.eventSourceCalls.Load()
	if calls != 0 {
		t.Errorf("paginated Lambda fetcher made %d ListEventSourceMappings call(s); want 0 (N+1 bug: issue #221)", calls)
	}
}

// TestLambdaPaginatedFetcher_DoesNotCallEventSourceAPI_ManyFunctions verifies
// the N+1 fix holds with a larger function count (5 functions → still 0
// event-source calls, not 5).
func TestLambdaPaginatedFetcher_DoesNotCallEventSourceAPI_ManyFunctions(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("lambda")
	if fetcher == nil {
		t.Fatal("paginated fetcher for 'lambda' not registered")
	}

	var fnEntries strings.Builder
	fnEntries.WriteString(`{"Functions":[`)
	for i := range 5 {
		if i > 0 {
			fnEntries.WriteString(",")
		}
		fnEntries.WriteString(`{`)
		fnEntries.WriteString(`"FunctionName":"fn-`)
		fnEntries.WriteString(string(rune('a' + i)))
		fnEntries.WriteString(`","Runtime":"python3.12","MemorySize":128,"Timeout":30,`)
		fnEntries.WriteString(`"Handler":"index.handler","LastModified":"2025-01-01T00:00:00.000+0000",`)
		fnEntries.WriteString(`"CodeSize":1024,"FunctionArn":"arn:aws:lambda:us-east-1:111122223333:function:fn-`)
		fnEntries.WriteString(string(rune('a' + i)))
		fnEntries.WriteString(`","Role":"arn:aws:iam::111122223333:role/r","PackageType":"Zip","Architectures":["x86_64"]}`)
	}
	fnEntries.WriteString(`]}`)

	transport := &countingLambdaRoundTripper{listFunctionsXML: fnEntries.String()}
	lambdaClient := newLambdaClientWithCountingTransport(transport)

	clients := &awsclient.ServiceClients{Lambda: lambdaClient}

	result, err := fetcher(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("fetcher returned unexpected error: %v", err)
	}

	if len(result.Resources) != 5 {
		t.Errorf("expected 5 resources, got %d", len(result.Resources))
	}

	calls := transport.eventSourceCalls.Load()
	if calls != 0 {
		t.Errorf("paginated Lambda fetcher made %d ListEventSourceMappings call(s) for 5 functions; want 0 (N+1 bug: issue #221)", calls)
	}
}

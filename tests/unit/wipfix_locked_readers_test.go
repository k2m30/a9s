// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_locked_readers_test.go covers the siblings of the two readers row 53
// named: every method that reads the cost store's shared state, driven against
// a writer, plus the map one of them hands out. And row 54's other half: the
// verification is asked for once per row that needs one, and not at all for a
// row whose read failed outright.
package unit_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestEveryCostsReader_IsSafeUnderAConcurrentWriter covers what row 53's own
// pin does not: Lookup, Partial, DataThrough, Revision and Attrs read the same
// structures the anomaly readers do, from the same render path, and each was
// unlocked for the same reason. Meaningful only under -race.
func TestEveryCostsReader_IsSafeUnderAConcurrentWriter(t *testing.T) {
	store := costs.NewMemoryStore("example-readonly")
	now := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	window := wipfixMonths(2025, 12)
	q := costs.Query{Granularity: string(costs.GranularityMonth),
		GroupBy: []costs.Dimension{costs.DimensionService}, Range: wipfixSpan(window)}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := range 200 {
			store.ApplyFetchResult(costs.FetchResult{
				Query:    q,
				Coverage: window,
				Records: []costs.Record{{
					Period: window[0], Keys: []string{"Amazon EC2"},
					Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: float64(i), Unit: "USD"}},
				}},
				Anomalies: costs.AnomalyResult{Requested: true, Truncated: i%2 == 0,
					Marks: []costs.AnomalyMark{wipfixMark(window[0], fmt.Sprintf("m-%d", i))}},
			}, now)
			store.MergeAttrs(map[string]string{fmt.Sprintf("acct-%d", i): "Example Account"})
		}
	}()
	go func() {
		defer wg.Done()
		for range 200 {
			_, _ = store.Lookup(q, window, now)
			_ = store.Partial(q, window, now)
			_ = store.DataThrough(q, now)
			_ = store.Revision()
			for k := range store.Attrs() {
				_ = k
			}
			_, _ = store.Anomalies(now)
			_, _ = store.AnomaliesCoverage(window, now)
			_, _ = store.AnomalyOverlay(window, now)
		}
	}()
	wg.Wait()
}

// TestAttrs_HandsOutACopy pins the other half of that read. MergeAttrs copies
// INTO the stored map, so returning the map itself hands the render path
// entries a fetch is writing, after the lock this method takes is gone.
func TestAttrs_HandsOutACopy(t *testing.T) {
	store := costs.NewMemoryStore("example-readonly")
	store.MergeAttrs(map[string]string{"acct-1": "One"})

	got := store.Attrs()
	got["acct-1"] = "tampered"
	got["acct-2"] = "added"

	again := store.Attrs()
	if again["acct-1"] != "One" {
		t.Errorf("the store's own attrs read %q after a caller wrote to the map it was handed",
			again["acct-1"])
	}
	if _, ok := again["acct-2"]; ok {
		t.Error("a key a caller added to its copy reached the store")
	}
}

// A row whose policy read failed outright has nothing for the verifier to
// settle, and must not pay for a call.
type lambdaRealErrFake struct {
	awsclient.LambdaAPI
	verified *int
}

func (f *lambdaRealErrFake) GetPolicy(context.Context, *lambda.GetPolicyInput, ...func(*lambda.Options)) (*lambda.GetPolicyOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "denied"}
}

func (f *lambdaRealErrFake) ListFunctionUrlConfigs(context.Context, *lambda.ListFunctionUrlConfigsInput, ...func(*lambda.Options)) (*lambda.ListFunctionUrlConfigsOutput, error) {
	return &lambda.ListFunctionUrlConfigsOutput{}, nil
}

func (f *lambdaRealErrFake) GetFunction(context.Context, *lambda.GetFunctionInput, ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	*f.verified++
	return &lambda.GetFunctionOutput{}, nil
}

func TestEnrichLambdaPosture_RealErrorNeverReachesTheVerifier(t *testing.T) {
	verified := 0
	res, _ := awsclient.EnrichLambdaPosture(context.Background(),
		&awsclient.ServiceClients{Lambda: &lambdaRealErrFake{verified: &verified}},
		[]resource.Resource{{ID: "example-fn", Name: "example-fn",
			Fields: map[string]string{"function_name": "example-fn"}}}, nil)
	if verified != 0 {
		t.Errorf("GetFunction was called %d time(s) for a row whose policy read failed outright — "+
			"there is nothing for it to settle", verified)
	}
	if !res.TruncatedIDs["example-fn"] {
		t.Error("a row whose policy read was denied is not marked uninspected")
	}
}

// Every row that needs a verification gets exactly one.
type lambdaCountingVerifier struct {
	awsclient.LambdaAPI
	mu    sync.Mutex
	calls map[string]int
}

func (f *lambdaCountingVerifier) GetPolicy(context.Context, *lambda.GetPolicyInput, ...func(*lambda.Options)) (*lambda.GetPolicyOutput, error) {
	return nil, lambdaNotFound()
}

func (f *lambdaCountingVerifier) ListFunctionUrlConfigs(context.Context, *lambda.ListFunctionUrlConfigsInput, ...func(*lambda.Options)) (*lambda.ListFunctionUrlConfigsOutput, error) {
	return nil, lambdaNotFound()
}

func (f *lambdaCountingVerifier) GetFunction(_ context.Context, in *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[*in.FunctionName]++
	return &lambda.GetFunctionOutput{}, nil
}

func TestEnrichLambdaPosture_OneVerificationPerRowThatNeedsIt(t *testing.T) {
	fake := &lambdaCountingVerifier{calls: map[string]int{}}
	rows := []resource.Resource{
		{ID: "fn-a", Name: "fn-a", Fields: map[string]string{"function_name": "fn-a"}},
		{ID: "fn-b", Name: "fn-b", Fields: map[string]string{"function_name": "fn-b"}},
	}
	res, _ := awsclient.EnrichLambdaPosture(context.Background(),
		&awsclient.ServiceClients{Lambda: fake}, rows, nil)

	for _, id := range []string{"fn-a", "fn-b"} {
		if fake.calls[id] != 1 {
			t.Errorf("%s was verified %d time(s), want exactly 1", id, fake.calls[id])
		}
		if res.TruncatedIDs[id] {
			t.Errorf("%s is uninspected though its verification succeeded and found it present", id)
		}
	}
}

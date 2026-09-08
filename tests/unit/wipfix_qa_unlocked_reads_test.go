// Two shared structures read outside the lock that guards their writers: the
// cost store's anomaly buckets, and the lambda posture batch's result mutex
// held across a network call.
package unit_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestAnomalyOverlay_ReadsUnderTheSameLockItsWritersHold pins row 53. Every
// writer of the anomaly buckets takes the store's mutex; a reader that does
// not is reading a pointer another goroutine is replacing. The grid's overlay
// is read on the render path while a fetch result lands from its task, so the
// two genuinely run at once — the race detector is the observable, and this
// test is only meaningful under -race.
func TestAnomalyOverlay_ReadsUnderTheSameLockItsWritersHold(t *testing.T) {
	store := costs.NewMemoryStore("example-readonly")
	now := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	window := wipfixMonths(2025, 12)

	var wg sync.WaitGroup
	wg.Add(2)

	// The writer: a fetch result landing, over and over, replacing both the
	// authoritative bucket and the capped one.
	go func() {
		defer wg.Done()
		for i := range 300 {
			store.ApplyFetchResult(costs.FetchResult{
				Query:    costs.Query{Granularity: string(costs.GranularityMonth), GroupBy: []costs.Dimension{costs.DimensionService}, Range: wipfixSpan(window)},
				Coverage: window,
				Anomalies: costs.AnomalyResult{
					Requested: true,
					Truncated: i%2 == 0,
					Marks:     []costs.AnomalyMark{wipfixMark(window[0], fmt.Sprintf("mark-%d", i))},
				},
			}, now)
		}
	}()

	// The reader: the render path asking what to overlay.
	go func() {
		defer wg.Done()
		for range 300 {
			_, _ = store.AnomalyOverlay(window, now)
			_, _ = store.AnomaliesCoverage(window, now)
		}
	}()

	wg.Wait()
}

// lambdaSlowVerifierFake answers the policy read with a public policy for
// every function but one, which gets the absent-resource code and so goes on
// to the verification. That verification blocks until the test releases it.
type lambdaSlowVerifierFake struct {
	awsclient.LambdaAPI
	slowID    string
	release   chan struct{}
	verifying chan struct{}
	polled    chan string
}

func (f *lambdaSlowVerifierFake) GetPolicy(
	_ context.Context, in *lambda.GetPolicyInput, _ ...func(*lambda.Options),
) (*lambda.GetPolicyOutput, error) {
	name := aws.ToString(in.FunctionName)
	f.polled <- name
	if name == f.slowID {
		return nil, lambdaNotFound()
	}
	return &lambda.GetPolicyOutput{Policy: aws.String(
		`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*",` +
			`"Action":"lambda:InvokeFunction","Resource":"*"}]}`)}, nil
}

func (f *lambdaSlowVerifierFake) ListFunctionUrlConfigs(
	_ context.Context, _ *lambda.ListFunctionUrlConfigsInput, _ ...func(*lambda.Options),
) (*lambda.ListFunctionUrlConfigsOutput, error) {
	return &lambda.ListFunctionUrlConfigsOutput{}, nil
}

func (f *lambdaSlowVerifierFake) GetFunction(
	_ context.Context, _ *lambda.GetFunctionInput, _ ...func(*lambda.Options),
) (*lambda.GetFunctionOutput, error) {
	close(f.verifying)
	<-f.release
	return &lambda.GetFunctionOutput{}, nil
}

// TestEnrichLambdaPosture_OneSlowVerificationDoesNotStallTheBatch pins row 54.
// The verification is a network call; the mutex it runs under exists to record
// results. Holding the mutex across the call makes every other function in the
// batch wait on one function's round trip, and the batch's worker slots fill
// with goroutines blocked on it, so functions further down are never asked
// about at all.
func TestEnrichLambdaPosture_OneSlowVerificationDoesNotStallTheBatch(t *testing.T) {
	const total = 24 // three times the batch's parallelism
	rows := make([]resource.Resource, total)
	rows[0] = resource.Resource{ID: "slow-fn", Name: "slow-fn",
		Fields: map[string]string{"function_name": "slow-fn"}}
	for i := 1; i < total; i++ {
		id := fmt.Sprintf("example-fn-%02d", i)
		rows[i] = resource.Resource{ID: id, Name: id, Fields: map[string]string{"function_name": id}}
	}

	fake := &lambdaSlowVerifierFake{
		slowID:    "slow-fn",
		release:   make(chan struct{}),
		verifying: make(chan struct{}),
		polled:    make(chan string, total),
	}

	done := make(chan awsclient.IssueEnricherResult, 1)
	go func() {
		res, _ := awsclient.EnrichLambdaPosture(context.Background(),
			&awsclient.ServiceClients{Lambda: fake}, rows, nil)
		done <- res
	}()

	// The verification is under way and answering nothing.
	select {
	case <-fake.verifying:
	case <-time.After(10 * time.Second):
		t.Fatal("the verification never started")
	}

	// Every other function must still be asked about while it blocks.
	seen := map[string]bool{}
	deadline := time.After(10 * time.Second)
	for len(seen) < total {
		select {
		case name := <-fake.polled:
			seen[name] = true
		case <-deadline:
			close(fake.release)
			<-done
			t.Fatalf("only %d of %d functions were asked about while one verification was in "+
				"flight — the batch's other rows are waiting on a network call they have no "+
				"stake in, and its worker slots are held by goroutines blocked on it",
				len(seen), total)
		}
	}

	close(fake.release)
	res := <-done

	// The results are the same ones the serialised batch would have produced.
	if res.TruncatedIDs["slow-fn"] {
		t.Error("slow-fn is recorded uninspected — its verification succeeded and found it present")
	}
	if got := codesOf(res.Findings["slow-fn"]); len(got) != 0 {
		t.Errorf("slow-fn carries %v, want none — its policy read found no policy", got)
	}
	for i := 1; i < total; i++ {
		id := fmt.Sprintf("example-fn-%02d", i)
		if len(res.Findings[id]) == 0 {
			t.Errorf("%s carries no finding — its policy is open to every principal", id)
		}
	}
}

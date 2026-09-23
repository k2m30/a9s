package unit_test

// ListFunctions returns none of State, StateReasonCode or LastUpdateStatus
// (the SDK documents them as GetFunction-only), so a function's lifecycle is
// only known after a GetFunction read. A function nobody read is unknown, not
// Active.

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// t561LambdaFake holds each function's full configuration and answers the way
// Lambda does: ListFunctions without the lifecycle fields, GetFunction with
// them.
type t561LambdaFake struct {
	awsclient.LambdaAPI
	fns    []lambdatypes.FunctionConfiguration
	getErr map[string]error

	mu    sync.Mutex
	reads map[string]int
}

func (f *t561LambdaFake) ListFunctions(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	out := make([]lambdatypes.FunctionConfiguration, len(f.fns))
	for i, fn := range f.fns {
		fn.State = ""
		fn.StateReason = nil
		fn.StateReasonCode = ""
		fn.LastUpdateStatus = ""
		fn.LastUpdateStatusReason = nil
		fn.LastUpdateStatusReasonCode = ""
		out[i] = fn
	}
	return &lambda.ListFunctionsOutput{Functions: out}, nil
}

func (f *t561LambdaFake) GetFunction(_ context.Context, in *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	name := aws.ToString(in.FunctionName)
	f.mu.Lock()
	f.reads[name]++
	f.mu.Unlock()
	if err, ok := f.getErr[name]; ok {
		return nil, err
	}
	for _, fn := range f.fns {
		if aws.ToString(fn.FunctionName) == name {
			cfg := fn
			return &lambda.GetFunctionOutput{
				Configuration: &cfg,
				Code:          &lambdatypes.FunctionCodeLocation{RepositoryType: aws.String("S3")},
			}, nil
		}
	}
	return nil, &lambdatypes.ResourceNotFoundException{Message: aws.String("Function not found: " + name)}
}

func (f *t561LambdaFake) GetPolicy(_ context.Context, in *lambda.GetPolicyInput, _ ...func(*lambda.Options)) (*lambda.GetPolicyOutput, error) {
	return nil, &lambdatypes.ResourceNotFoundException{Message: aws.String("The resource you requested does not exist.")}
}

func (f *t561LambdaFake) ListFunctionUrlConfigs(_ context.Context, _ *lambda.ListFunctionUrlConfigsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionUrlConfigsOutput, error) {
	return &lambda.ListFunctionUrlConfigsOutput{}, nil
}

// t561Fn is a healthy function as GetFunction describes it: active, last
// update successful, a dead-letter queue configured.
func t561Fn(name string, runtime lambdatypes.Runtime) lambdatypes.FunctionConfiguration {
	return lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String(name),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name),
		Runtime:          runtime,
		Handler:          aws.String("app.handler"),
		Role:             aws.String("arn:aws:iam::123456789012:role/acme-lambda-exec"),
		MemorySize:       aws.Int32(256),
		Timeout:          aws.Int32(30),
		CodeSize:         1_845_221,
		LastModified:     aws.String("2026-08-14T09:21:05.000+0000"),
		PackageType:      lambdatypes.PackageTypeZip,
		State:            lambdatypes.StateActive,
		LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		DeadLetterConfig: &lambdatypes.DeadLetterConfig{TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:acme-dlq")},
	}
}

// t561LambdaRows runs the list fetch and the type's registered wave-2 pass,
// then folds the answer onto the rows the way the app does: findings through
// runtime.ApplyWave2ToRow, FieldUpdates merged into Fields.
func t561LambdaRows(t *testing.T, fake *t561LambdaFake) ([]resource.Resource, awsclient.IssueEnricherResult) {
	t.Helper()
	fake.reads = map[string]int{}
	page, err := awsclient.FetchLambdaFunctionsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage: %v", err)
	}
	enricher, ok := awsclient.Wave2EnricherFor("lambda")
	if !ok || enricher.Fn == nil {
		t.Fatal("lambda has no wave-2 enricher")
	}
	res, _ := enricher.Fn(context.Background(), &awsclient.ServiceClients{Lambda: fake}, page.Resources, nil) //nolint:errcheck // a failed read is part of the scenario; its row is asserted instead
	td := resource.FindResourceType("lambda")
	rows := page.Resources
	for i := range rows {
		rows[i].Fields = maps.Clone(rows[i].Fields)
		maps.Copy(rows[i].Fields, res.FieldUpdates[rows[i].ID])
		runtime.ApplyWave2ToRow(&rows[i], *td, res.Findings, res.AttentionDetails)
	}
	return rows, res
}

func t561Row(t *testing.T, rows []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no row %s", id)
	return resource.Resource{}
}

// t561LifecycleFns is one function per lifecycle answer GetFunction gives:
// active, failed with a reason code, pending, and active with a failed last
// update.
func t561LifecycleFns() []lambdatypes.FunctionConfiguration {
	active := t561Fn("acme-orders-api", lambdatypes.RuntimePython312)

	failed := t561Fn("acme-image-resizer", lambdatypes.RuntimePython312)
	failed.State = lambdatypes.StateFailed
	failed.StateReasonCode = lambdatypes.StateReasonCode("SubnetOutOfIPAddresses")
	failed.StateReason = aws.String("Lambda was unable to create the function's network interface because the subnet has no free IP addresses.")

	pending := t561Fn("acme-report-builder", lambdatypes.Runtime("python3.13"))
	pending.State = lambdatypes.StatePending
	pending.StateReasonCode = lambdatypes.StateReasonCode("Creating")
	pending.StateReason = aws.String("The function is being created.")

	updateFailed := t561Fn("acme-billing-sync", lambdatypes.RuntimePython312)
	updateFailed.LastUpdateStatus = lambdatypes.LastUpdateStatusFailed
	updateFailed.LastUpdateStatusReasonCode = lambdatypes.LastUpdateStatusReasonCode("InvalidSecurityGroup")
	updateFailed.LastUpdateStatusReason = aws.String("The security group sg-0abc1234def567890 does not exist.")

	return []lambdatypes.FunctionConfiguration{active, failed, pending, updateFailed}
}

func TestT561_LambdaLifecycleReadFromGetFunction(t *testing.T) {
	unreadable := t561Fn("acme-audit-export", lambdatypes.RuntimePython312)

	fake := &t561LambdaFake{
		fns: append(t561LifecycleFns(), unreadable),
		getErr: map[string]error{"acme-audit-export": &smithy.GenericAPIError{
			Code:    "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform: lambda:GetFunction",
		}},
	}
	rows, res := t561LambdaRows(t, fake)
	td := resource.FindResourceType("lambda")

	// Every Status cell goes through domain.HumanizeStatusPhrase, so the
	// lifecycle word renders lower-case.
	cases := []struct {
		id    string
		code  domain.FindingCode
		cell  string
		color resource.Color
	}{
		{"acme-orders-api", "", "active", resource.ColorHealthy},
		{"acme-image-resizer", awsclient.CodeLambdaStateFailed, "failed", resource.ColorBroken},
		{"acme-report-builder", awsclient.CodeLambdaStatePending, "pending", resource.ColorWarning},
		{"acme-billing-sync", awsclient.CodeLambdaLastUpdateFailed, "last update failed to apply", resource.ColorBroken},
	}
	for _, c := range cases {
		r := t561Row(t, rows, c.id)
		if c.code == "" {
			if len(r.Findings) != 0 {
				t.Errorf("%s: an active function with a DLQ carries %v, want none", c.id, t561Codes(r.Findings))
			}
		} else if top, ok := domain.TopFinding(r.Findings); !ok || top.Code != c.code {
			t.Errorf("%s: top finding = %q (all %v), want %s", c.id, top.Code, t561Codes(r.Findings), c.code)
		}
		if got := td.ResolveColor(r); got != c.color {
			t.Errorf("%s: row colour = %v, want %v", c.id, got, c.color)
		}
		cell, ok := listStatusCellFor(t, *td, rows, c.id)
		if !ok {
			t.Fatalf("%s: no list row", c.id)
		}
		if cell != c.cell {
			t.Errorf("%s: Status cell = %q, want %q", c.id, cell, c.cell)
		}
	}

	// A function whose GetFunction read failed is uninspected; nothing about
	// it may read Active or clean on a call that never answered.
	r := t561Row(t, rows, "acme-audit-export")
	if _, marked := res.TruncatedIDs["acme-audit-export"]; !marked {
		t.Errorf("acme-audit-export: GetFunction failed but the row is not marked not inspected; TruncatedIDs=%v", res.TruncatedIDs)
	}
	if got := r.Fields["state"]; got == string(lambdatypes.StateActive) {
		t.Errorf("acme-audit-export: state = %q on a function whose read failed", got)
	}
	if cell, _ := listStatusCellFor(t, *td, rows, "acme-audit-export"); strings.EqualFold(cell, string(lambdatypes.StateActive)) {
		t.Errorf("acme-audit-export: Status cell = %q on a function whose read failed", cell)
	}

	for name, n := range fake.reads {
		if n != 1 {
			t.Errorf("%s: GetFunction called %d times, want once per function", name, n)
		}
	}
}

// Rows past the enrichment cap were never read, so none of them may read
// Active; every row inside the cap reads its real state.
func TestT561_LambdaStatePastTheCapIsNotInspected(t *testing.T) {
	fns := make([]lambdatypes.FunctionConfiguration, awsclient.EnrichmentCap+3)
	for i := range fns {
		fns[i] = t561Fn(fmt.Sprintf("acme-worker-%03d", i), lambdatypes.RuntimePython312)
	}
	rows, res := t561LambdaRows(t, &t561LambdaFake{fns: fns})

	past := 0
	for _, r := range rows {
		if _, marked := res.TruncatedIDs[r.ID]; marked {
			past++
			if r.Fields["state"] == string(lambdatypes.StateActive) {
				t.Errorf("%s: past the cap, never read, but state = %q", r.ID, r.Fields["state"])
			}
			continue
		}
		if got := r.Fields["state"]; got != string(lambdatypes.StateActive) {
			t.Errorf("%s: read by GetFunction as Active, but state = %q", r.ID, got)
		}
	}
	if past != 3 {
		t.Errorf("%d rows marked not inspected, want the 3 past EnrichmentCap=%d", past, awsclient.EnrichmentCap)
	}
}

// The end-of-life finding follows AWS's deprecation date: python3.8 was
// deprecated years ago, python3.13 is supported for years to come.
func TestT561_LambdaEndOfLifeRuntimeByDeprecationDate(t *testing.T) {
	fake := &t561LambdaFake{fns: []lambdatypes.FunctionConfiguration{
		t561Fn("acme-legacy-etl", lambdatypes.Runtime("python3.8")),
		t561Fn("acme-current-etl", lambdatypes.Runtime("python3.13")),
	}}
	page, err := awsclient.FetchLambdaFunctionsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage: %v", err)
	}
	td := resource.FindResourceType("lambda")

	old := t561Row(t, page.Resources, "acme-legacy-etl")
	if !t561Has(old.Findings, awsclient.CodeLambdaDeprecatedRuntime) {
		t.Errorf("python3.8: findings = %v, want %s", t561Codes(old.Findings), awsclient.CodeLambdaDeprecatedRuntime)
	}
	if got := td.ResolveColor(old); got != resource.ColorBroken {
		t.Errorf("python3.8: row colour = %v, want %v", got, resource.ColorBroken)
	}

	current := t561Row(t, page.Resources, "acme-current-etl")
	if t561Has(current.Findings, awsclient.CodeLambdaDeprecatedRuntime) {
		t.Errorf("python3.13: carries %s, a runtime still supported", awsclient.CodeLambdaDeprecatedRuntime)
	}
	if got := td.ResolveColor(current); got != resource.ColorHealthy {
		t.Errorf("python3.13: row colour = %v, want %v", got, resource.ColorHealthy)
	}
}

// The end-of-life runtime and the missing dead-letter queue are independent
// facts about a function; one never hides the other.
func TestT561_LambdaEndOfLifeAndNoDLQAreBothReported(t *testing.T) {
	both := t561Fn("acme-legacy-webhook", lambdatypes.Runtime("python3.8"))
	both.DeadLetterConfig = nil
	noDLQ := t561Fn("acme-current-webhook", lambdatypes.Runtime("python3.13"))
	noDLQ.DeadLetterConfig = nil

	page, err := awsclient.FetchLambdaFunctionsPage(context.Background(), &t561LambdaFake{fns: []lambdatypes.FunctionConfiguration{both, noDLQ}}, "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage: %v", err)
	}

	r := t561Row(t, page.Resources, "acme-legacy-webhook")
	for _, code := range []domain.FindingCode{awsclient.CodeLambdaDeprecatedRuntime, awsclient.CodeLambdaNoDLQ} {
		if !t561Has(r.Findings, code) {
			t.Errorf("acme-legacy-webhook (python3.8, no DLQ): findings = %v, missing %s", t561Codes(r.Findings), code)
		}
	}

	r = t561Row(t, page.Resources, "acme-current-webhook")
	if got := t561Codes(r.Findings); len(got) != 1 || got[0] != awsclient.CodeLambdaNoDLQ {
		t.Errorf("acme-current-webhook (python3.13, no DLQ): findings = %v, want [%s]", got, awsclient.CodeLambdaNoDLQ)
	}
}

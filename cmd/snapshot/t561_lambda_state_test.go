package main

// ListFunctions returns none of the six lifecycle fields; GetFunction's
// Configuration carries them. The collector reads them there, once per listed
// function.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// t561LambdaConfigs is each function as GetFunction describes it.
var t561LambdaConfigs = map[string]map[string]any{ //nolint:gochecknoglobals // test-only table
	"acme-orders-api": {
		"State": "Active", "LastUpdateStatus": "Successful",
	},
	"acme-image-resizer": {
		"State": "Failed", "StateReasonCode": "SubnetOutOfIPAddresses",
		"StateReason":      "Lambda was unable to create the function's network interface because the subnet has no free IP addresses.",
		"LastUpdateStatus": "Successful",
	},
	"acme-billing-sync": {
		"State": "Active", "LastUpdateStatus": "Failed", "LastUpdateStatusReasonCode": "InvalidSecurityGroup",
		"LastUpdateStatusReason": "The security group sg-0abc1234def567890 does not exist.",
	},
}

type t561LambdaTransport struct {
	mu    sync.Mutex
	reads map[string]int
}

func (tr *t561LambdaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := func(name string) map[string]any {
		return map[string]any{
			"FunctionName": name,
			"FunctionArn":  "arn:aws:lambda:us-east-1:123456789012:function:" + name,
			"Runtime":      "python3.12",
			"Handler":      "app.handler",
			"DeadLetterConfig": map[string]any{
				"TargetArn": "arn:aws:sqs:us-east-1:123456789012:acme-dlq",
			},
		}
	}
	var out any
	name := strings.Trim(strings.TrimPrefix(req.URL.Path, "/2015-03-31/functions"), "/")
	if name == "" {
		var fns []map[string]any
		for _, n := range []string{"acme-orders-api", "acme-image-resizer", "acme-billing-sync"} {
			fns = append(fns, base(n))
		}
		out = map[string]any{"Functions": fns}
	} else {
		tr.mu.Lock()
		tr.reads[name]++
		tr.mu.Unlock()
		cfg := base(name)
		for k, v := range t561LambdaConfigs[name] {
			cfg[k] = v
		}
		out = map[string]any{
			"Configuration": cfg,
			"Code":          map[string]any{"RepositoryType": "S3"},
		}
	}
	b, _ := json.Marshal(out) //nolint:errcheck // test-built maps always marshal
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}, nil
}

func TestCaptureLambda_LifecycleFieldsComeFromGetFunction(t *testing.T) {
	tr := &t561LambdaTransport{reads: map[string]int{}}
	cfg := aws.Config{
		Region:           "us-east-1",
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		HTTPClient:       &http.Client{Transport: tr},
		RetryMaxAttempts: 1,
	}
	got, err := captureLambda(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureLambda: %v", err)
	}
	data, ok := got.(lambdaData)
	if !ok || len(data.Functions) != 3 {
		t.Fatalf("captureLambda returned %#v, want three functions", got)
	}

	for _, fn := range data.Functions {
		want := t561LambdaConfigs[fn.FunctionName]
		str := func(k string) string {
			s, _ := want[k].(string) //nolint:errcheck // an absent field is the empty string
			return s
		}
		captured := map[string][2]string{
			"State":                      {fn.State, str("State")},
			"StateReason":                {fn.StateReason, str("StateReason")},
			"StateReasonCode":            {fn.StateReasonCode, str("StateReasonCode")},
			"LastUpdateStatus":           {fn.LastUpdateStatus, str("LastUpdateStatus")},
			"LastUpdateStatusReason":     {fn.LastUpdateStatusReason, str("LastUpdateStatusReason")},
			"LastUpdateStatusReasonCode": {fn.LastUpdateStatusReasonCode, str("LastUpdateStatusReasonCode")},
		}
		for field, gw := range captured {
			if gw[0] != gw[1] {
				t.Errorf("%s: %s = %q, want %q from GetFunction", fn.FunctionName, field, gw[0], gw[1])
			}
		}
		if fn.Runtime != "python3.12" || !fn.HasDeadLetterConfig {
			t.Errorf("%s: runtime/DLQ = %q/%v, want python3.12/true from the list", fn.FunctionName, fn.Runtime, fn.HasDeadLetterConfig)
		}
	}
	for _, name := range []string{"acme-orders-api", "acme-image-resizer", "acme-billing-sync"} {
		if n := tr.reads[name]; n != 1 {
			t.Errorf("%s: GetFunction called %d times, want once per listed function", name, n)
		}
	}
}

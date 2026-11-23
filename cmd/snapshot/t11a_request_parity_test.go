package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
)

// legitimateFirstRequestDifferences lists the top-level types whose collector
// asks AWS a different first question than the app's fetcher, keyed by short
// name, with the reason the two requests cannot be the same.
var legitimateFirstRequestDifferences = map[string]string{}

// paginationFields are the input members that only size or resume a page;
// the question a request asks AWS is the rest of its input.
var paginationFields = map[string]bool{
	"MaxResults":        true,
	"MaxRecords":        true,
	"MaxItems":          true,
	"MaxKeys":           true,
	"MaxBuckets":        true,
	"Limit":             true,
	"PageSize":          true,
	"NextToken":         true,
	"Marker":            true,
	"NextMarker":        true,
	"Position":          true,
	"ContinuationToken": true,
	"StartingToken":     true,
	"PaginationToken":   true,
}

type answerFunc func(service, op string, in any) (any, error)

// answeringConfig is an aws.Config whose every SDK call is answered by answer
// before it is signed or sent, so nothing reaches the network.
func answeringConfig(answer answerFunc) aws.Config {
	return aws.Config{
		Region:           "eu-west-1",
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		RetryMaxAttempts: 1,
		APIOptions: []func(*middleware.Stack) error{func(st *middleware.Stack) error {
			return st.Initialize.Add(middleware.InitializeMiddlewareFunc("t11aAnswer",
				func(ctx context.Context, in middleware.InitializeInput, _ middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
					out, err := answer(awsmiddleware.GetServiceID(ctx), awsmiddleware.GetOperationName(ctx), in.Parameters)
					return middleware.InitializeOutput{Result: out}, middleware.Metadata{}, err
				}), middleware.After)
		}},
	}
}

var errRequestRecorded = errors.New("request recorded")

// firstRequest records the service, operation and input of the first SDK
// call a run sends and fails every call. GetCallerIdentity is answered with a
// synthetic identity instead: it names the account, it asks nothing about the
// resources a type lists.
type firstRequest struct {
	mu      sync.Mutex
	set     bool
	service string
	op      string
	input   any
}

func (r *firstRequest) config() aws.Config {
	return answeringConfig(func(service, op string, in any) (any, error) {
		if service == "STS" && op == "GetCallerIdentity" {
			return &sts.GetCallerIdentityOutput{
				Account: aws.String("123456789012"),
				Arn:     aws.String("arn:aws:iam::123456789012:user/example-readonly"),
				UserId:  aws.String("AIDAEXAMPLEUSERID"),
			}, nil
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if !r.set {
			r.set, r.service, r.op, r.input = true, service, op, in
		}
		return nil, errRequestRecorded
	})
}

func (r *firstRequest) String() string {
	b, _ := json.Marshal(r.input) //nolint:errchkjson // SDK input structs always marshal
	return fmt.Sprintf("%s.%s %s", r.service, r.op, b)
}

func (r *firstRequest) question(t *testing.T) map[string]any {
	t.Helper()
	b, err := json.Marshal(r.input)
	if err != nil {
		t.Fatalf("marshal %T: %v", r.input, err)
	}
	m := map[string]any{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal %T: %v", r.input, err)
	}
	for k := range m {
		if paginationFields[k] {
			delete(m, k)
		}
	}
	return m
}

// The collector is the oracle the web e2e checks compare the app against, so
// for each type the app lists it must ask AWS the same question the app's
// fetcher asks: same service, same operation, same input apart from page size
// and page token. A type the app lists with no capture, or with a capture a
// full run never executes, has no oracle at all.
func TestT11a_CollectorSendsTheAppsFirstRequestForEveryTopLevelType(t *testing.T) {
	awsclient.Install()
	for _, td := range catalog.All() {
		name := td.ShortName
		t.Run(name, func(t *testing.T) {
			capture, ok := captures[name]
			if !ok {
				t.Fatalf("no capture is registered for top-level type %q", name)
			}
			if !slices.Contains(captureOrder, name) {
				t.Errorf("capture %q is registered but a full run (captureOrder) never executes it", name)
			}
			if reason, ok := legitimateFirstRequestDifferences[name]; ok {
				if reason == "" {
					t.Fatalf("%s is declared a legitimate difference with no reason", name)
				}
				t.Skipf("declared difference: %s", reason)
			}

			fetch := resource.GetPaginatedFetcher(name)
			if fetch == nil {
				t.Fatalf("no fetcher is registered for top-level type %q", name)
			}
			app := &firstRequest{}
			_, _ = fetch(context.Background(), awsclient.CreateServiceClients(app.config()), "") //nolint:errcheck // every call fails by design; only the request matters

			col := &firstRequest{}
			_, _ = capture(context.Background(), col.config()) //nolint:errcheck // every call fails by design; only the request matters

			if !app.set {
				t.Fatalf("the app's fetcher sent no request")
			}
			if !col.set {
				t.Fatalf("the collector sent no request; the app sends %s", app)
			}
			if app.service != col.service || app.op != col.op || !reflect.DeepEqual(app.question(t), col.question(t)) {
				t.Errorf("first request differs\n  app:       %s\n  collector: %s", app, col)
			}
		})
	}
}

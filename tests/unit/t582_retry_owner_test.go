package unit_test

// t582_retry_owner_test.go — throttle retry has one owner: the SDK retryer
// configured where CreateServiceClients builds the clients. Every call made
// through those clients is retried on a throttle and never on a refusal, with
// one attempt budget for every service and no client-side retry quota, so a
// burst of parallel throttles during a fan-out does not cut retries short.
//
// Clients are built by the real constructor over an aws.Config whose HTTP
// client is a stub, so each attempt is a real SDK request the stub counts.

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

type t582Protocol int

const (
	t582RestXML t582Protocol = iota // S3
	t582Query                       // IAM, CloudFormation, Elastic Beanstalk
	t582EC2Query
	t582JSON10 // DynamoDB
)

type t582Answer struct {
	status int
	code   string // "" answers success, or a bare status with no error body
}

// t582Stub answers every request from script, by attempt number.
type t582Stub struct {
	proto  t582Protocol
	ok     string
	n      atomic.Int64
	script func(attempt int64) t582Answer
}

func (s *t582Stub) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		_, _ = io.Copy(io.Discard, req.Body)
		_ = req.Body.Close()
	}
	a := s.script(s.n.Add(1))
	body, ctype := s.ok, "text/xml"
	switch {
	case a.status == http.StatusOK:
	case a.code == "":
		body = ""
	default:
		body = t582ErrorBody(s.proto, a.code)
	}
	if s.proto == t582JSON10 {
		ctype = "application/x-amz-json-1.0"
	}
	h := http.Header{}
	h.Set("Content-Type", ctype)
	h.Set("X-Amzn-Requestid", "4f8a2c1e-0b3d-4e5f-9a6b-7c8d9e0f1a2b")
	return &http.Response{
		StatusCode:    a.status,
		Status:        fmt.Sprintf("%d %s", a.status, http.StatusText(a.status)),
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       req,
	}, nil
}

func t582ErrorBody(p t582Protocol, code string) string {
	switch p {
	case t582RestXML:
		return `<?xml version="1.0" encoding="UTF-8"?><Error><Code>` + code + `</Code><Message>Please reduce your request rate.</Message><RequestId>4F8A2C1E0B3D4E5F</RequestId></Error>`
	case t582EC2Query:
		return `<?xml version="1.0" encoding="UTF-8"?><Response><Errors><Error><Code>` + code + `</Code><Message>Request limit exceeded.</Message></Error></Errors><RequestID>4f8a2c1e-0b3d-4e5f-9a6b-7c8d9e0f1a2b</RequestID></Response>`
	case t582JSON10:
		return `{"__type":"com.amazonaws.dynamodb.v20120810#` + code + `","message":"Rate exceeded"}`
	default:
		return `<ErrorResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><Error><Type>Sender</Type><Code>` + code + `</Code><Message>Rate exceeded</Message></Error><RequestId>4f8a2c1e-0b3d-4e5f-9a6b-7c8d9e0f1a2b</RequestId></ErrorResponse>`
	}
}

// t582Service is one sampled service: its wire protocol, a success body for
// the one read the test issues, and that read through the ServiceClients
// field production code uses.
type t582Service struct {
	name  string
	proto t582Protocol
	ok    string
	call  func(ctx context.Context, c *awsclient.ServiceClients) error
}

var t582Services = []t582Service{
	{"s3", t582RestXML,
		`<?xml version="1.0" encoding="UTF-8"?><ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Owner><ID>7e2f</ID></Owner><Buckets></Buckets></ListAllMyBucketsResult>`,
		func(ctx context.Context, c *awsclient.ServiceClients) error {
			_, err := c.S3.ListBuckets(ctx, &s3.ListBucketsInput{})
			return err
		}},
	{"iam", t582Query,
		`<ListRolesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"><ListRolesResult><IsTruncated>false</IsTruncated><Roles></Roles></ListRolesResult><ResponseMetadata><RequestId>4f8a2c1e</RequestId></ResponseMetadata></ListRolesResponse>`,
		func(ctx context.Context, c *awsclient.ServiceClients) error {
			_, err := c.IAM.ListRoles(ctx, &iam.ListRolesInput{})
			return err
		}},
	{"ec2", t582EC2Query,
		`<?xml version="1.0" encoding="UTF-8"?><DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/"><requestId>4f8a2c1e</requestId><reservationSet/></DescribeInstancesResponse>`,
		func(ctx context.Context, c *awsclient.ServiceClients) error {
			_, err := c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
			return err
		}},
	{"cloudformation", t582Query,
		`<DescribeStacksResponse xmlns="http://cloudformation.amazonaws.com/doc/2010-05-15/"><DescribeStacksResult><Stacks/></DescribeStacksResult><ResponseMetadata><RequestId>4f8a2c1e</RequestId></ResponseMetadata></DescribeStacksResponse>`,
		func(ctx context.Context, c *awsclient.ServiceClients) error {
			_, err := c.CloudFormation.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{})
			return err
		}},
	{"dynamodb", t582JSON10, `{"TableNames":[]}`,
		func(ctx context.Context, c *awsclient.ServiceClients) error {
			_, err := c.DynamoDB.ListTables(ctx, &dynamodb.ListTablesInput{})
			return err
		}},
	{"elasticbeanstalk", t582Query,
		`<DescribeEnvironmentsResponse xmlns="http://elasticbeanstalk.amazonaws.com/docs/2010-12-01/"><DescribeEnvironmentsResult><Environments/></DescribeEnvironmentsResult><ResponseMetadata><RequestId>4f8a2c1e</RequestId></ResponseMetadata></DescribeEnvironmentsResponse>`,
		func(ctx context.Context, c *awsclient.ServiceClients) error {
			_, err := c.ElasticBeanstalk.DescribeEnvironments(ctx, &elasticbeanstalk.DescribeEnvironmentsInput{})
			return err
		}},
}

// t582Clients builds the session's client set the way production does, over
// the stub. The backoff seam keeps a retry from waiting out real seconds; it
// must be set before construction, where the retryer is configured.
func t582Clients(t *testing.T, svc t582Service, script func(int64) t582Answer) (*awsclient.ServiceClients, *t582Stub) {
	t.Helper()
	t.Cleanup(awsclient.SetRetryMaxBackoffForTest(time.Millisecond))
	stub := &t582Stub{proto: svc.proto, ok: svc.ok, script: script}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "example-secret", ""),
		HTTPClient:  &http.Client{Transport: stub},
	}
	return awsclient.CreateServiceClients(cfg), stub
}

func t582APICode(err error) string {
	var api smithy.APIError
	if errors.As(err, &api) {
		return api.ErrorCode()
	}
	return ""
}

// A throttle answer — by error code, or by a bare HTTP 429 with no code AWS
// would recognise — is retried, and the read succeeds on the second attempt.
func TestT582_ThrottleOnceIsRetriedOnceOnEveryService(t *testing.T) {
	throttles := []t582Answer{
		{http.StatusBadRequest, "ThrottlingException"},
		{http.StatusBadRequest, "Throttling"},
		{http.StatusBadRequest, "TooManyRequestsException"},
		{http.StatusBadRequest, "RequestLimitExceeded"},
		{http.StatusBadRequest, "ProvisionedThroughputExceededException"},
		{http.StatusTooManyRequests, ""},
	}
	for _, svc := range t582Services {
		for _, th := range throttles {
			name := svc.name + "/" + th.code
			if th.code == "" {
				name = svc.name + "/HTTP429"
			}
			t.Run(name, func(t *testing.T) {
				c, stub := t582Clients(t, svc, func(n int64) t582Answer {
					if n == 1 {
						return th
					}
					return t582Answer{status: http.StatusOK}
				})
				err := svc.call(context.Background(), c)
				if err != nil {
					t.Errorf("call failed after a single throttle: %v", err)
				}
				if got := stub.n.Load(); got != 2 {
					t.Errorf("stub saw %d requests, want 2 (one throttle, one retry)", got)
				}
			})
		}
	}
}

// A refusal is final: the caller gets it from the first attempt, and nothing
// is sent again.
func TestT582_RefusalIsNeverRetried(t *testing.T) {
	refusals := []t582Answer{
		{http.StatusForbidden, "AccessDenied"},
		{http.StatusForbidden, "AccessDeniedException"},
		{http.StatusForbidden, "UnauthorizedOperation"},
		{http.StatusBadRequest, "ValidationException"},
	}
	for _, svc := range t582Services {
		for _, rf := range refusals {
			t.Run(svc.name+"/"+rf.code, func(t *testing.T) {
				c, stub := t582Clients(t, svc, func(int64) t582Answer { return rf })
				err := svc.call(context.Background(), c)
				if got := t582APICode(err); got != rf.code {
					t.Errorf("caller got error code %q (err %v), want %q", got, err, rf.code)
				}
				if got := stub.n.Load(); got != 1 {
					t.Errorf("stub saw %d requests, want 1", got)
				}
			})
		}
	}
}

func t582AlwaysThrottle(int64) t582Answer {
	return t582Answer{http.StatusBadRequest, "ThrottlingException"}
}

// Every service gets the same attempt budget, because it is configured once,
// and a call that never stops being throttled uses all of it.
func TestT582_PersistentThrottleUsesOneAttemptBudgetEverywhere(t *testing.T) {
	attempts := map[string]int64{}
	for _, svc := range t582Services {
		c, stub := t582Clients(t, svc, t582AlwaysThrottle)
		err := svc.call(context.Background(), c)
		if got := t582APICode(err); got != "ThrottlingException" {
			t.Errorf("%s: caller got error code %q (err %v), want ThrottlingException", svc.name, got, err)
		}
		attempts[svc.name] = stub.n.Load()
	}
	want := attempts["s3"]
	if want < 2 {
		t.Fatalf("s3 made %d attempts under a persistent throttle, want a retry", want)
	}
	for name, got := range attempts {
		if got != want {
			t.Errorf("%s made %d attempts, s3 made %d: the attempt budget differs by service %v", name, got, want, attempts)
		}
	}
}

// A fan-out that hits a throttle on many reads at once is when retries
// matter most. A client-side retry quota drains under such a burst and fails
// the remaining reads early; with none, every read uses its full budget.
func TestT582_ParallelThrottleBurstIsNotCutShortByARetryQuota(t *testing.T) {
	const burst = 200
	for _, svc := range t582Services {
		t.Run(svc.name, func(t *testing.T) {
			c, stub := t582Clients(t, svc, t582AlwaysThrottle)
			if err := svc.call(context.Background(), c); t582APICode(err) != "ThrottlingException" {
				t.Fatalf("single call: err %v, want ThrottlingException", err)
			}
			budget := stub.n.Load()

			c, stub = t582Clients(t, svc, t582AlwaysThrottle)
			var wg sync.WaitGroup
			var early atomic.Int64
			var sample atomic.Value
			for range burst {
				wg.Go(func() {
					err := svc.call(context.Background(), c)
					if t582APICode(err) != "ThrottlingException" {
						early.Add(1)
						sample.Store(fmt.Sprint(err))
					}
				})
			}
			wg.Wait()
			if got, want := stub.n.Load(), int64(burst)*budget; got != want {
				t.Errorf("%d parallel throttled calls made %d requests, want %d (%d attempts each)", burst, got, want, budget)
			}
			if n := early.Load(); n > 0 {
				t.Errorf("%d of %d calls ended without the throttle error, e.g. %v", n, burst, sample.Load())
			}
		})
	}
}

// Retry belongs to the SDK retryer configured once in CreateServiceClients. A
// per-call wrapper, or any loop that waits between attempts, is a second
// retry layer on top of it. a9s has no other reason to sleep in a loop, so a
// timer wait inside a loop is reported wherever it appears.
func TestT582_NoPerCallRetryInSource(t *testing.T) {
	banned := map[string]bool{
		"RetryOnThrottle":            true,
		"RetryConfig":                true,
		"DefaultRetryConfig":         true,
		"SetRetryConfigForTest":      true,
		"retryConfigOverrideForTest": true,
	}
	waits := map[string]bool{"After": true, "Sleep": true, "NewTimer": true, "AfterFunc": true, "Tick": true}

	var idents []string
	loopWaits := map[string]bool{} // a wait inside nested loops is one site
	fset := token.NewFileSet()
	for _, root := range []string{"core", "internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.Ident:
					if banned[n.Name] {
						idents = append(idents, fmt.Sprintf("%s: %s", fset.Position(n.Pos()), n.Name))
					}
				case *ast.ForStmt:
					t582TimerWaits(fset, n.Body, waits, loopWaits)
				case *ast.RangeStmt:
					t582TimerWaits(fset, n.Body, waits, loopWaits)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(idents) > 0 {
		t.Errorf("%d references to the per-call retry wrapper remain:\n%s", len(idents), strings.Join(idents, "\n"))
	}
	if len(loopWaits) > 0 {
		loops := slices.Sorted(maps.Keys(loopWaits))
		t.Errorf("%d timer waits inside a loop (a retry loop beside the SDK retryer):\n%s", len(loops), strings.Join(loops, "\n"))
	}
}

func t582TimerWaits(fset *token.FileSet, body *ast.BlockStmt, waits, out map[string]bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "time" && waits[sel.Sel.Name] {
			out[fmt.Sprintf("%s: time.%s", fset.Position(call.Pos()), sel.Sel.Name)] = true
		}
		return true
	})
}

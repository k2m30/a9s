package unit_test

import (
	"context"
	"go/ast"
	"go/types"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"golang.org/x/tools/go/packages"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

func t582DevOnce(a t582Answer) func(int64) t582Answer {
	return func(n int64) t582Answer {
		if n == 1 {
			return a
		}
		return t582Answer{status: http.StatusOK}
	}
}

// t582DevClients is t582Clients with the backoff cap a test chooses; 0 is the
// production cap.
func t582DevClients(t *testing.T, svc t582Service, script func(int64) t582Answer, backoff time.Duration) (*awsclient.ServiceClients, *t582Stub) {
	t.Helper()
	t.Cleanup(awsclient.SetRetryMaxBackoffForTest(backoff))
	stub := &t582Stub{proto: svc.proto, ok: svc.ok, script: script}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "example-secret", ""),
		HTTPClient:  &http.Client{Transport: stub},
	}
	return awsclient.CreateServiceClients(cfg), stub
}

// S3 throttles as SlowDown on a 503, and a client set built for another
// Region goes through the same constructor: both are retried by the one
// owner, once, and the read succeeds.
func TestT582Dev_SlowDownAndOtherRegionClientsAreRetried(t *testing.T) {
	s3svc, ddb := t582Services[0], t582Services[4]

	c, stub := t582Clients(t, s3svc, t582DevOnce(t582Answer{http.StatusServiceUnavailable, "SlowDown"}))
	if err := s3svc.call(context.Background(), c); err != nil {
		t.Errorf("s3 SlowDown: call failed after one throttle: %v", err)
	}
	if got := stub.n.Load(); got != 2 {
		t.Errorf("s3 SlowDown: stub saw %d requests, want 2", got)
	}

	c, stub = t582Clients(t, ddb, t582DevOnce(t582Answer{status: http.StatusTooManyRequests}))
	if err := ddb.call(context.Background(), c.InRegion("eu-west-1")); err != nil {
		t.Errorf("eu-west-1 set: call failed after one 429: %v", err)
	}
	if got := stub.n.Load(); got != 2 {
		t.Errorf("eu-west-1 set: stub saw %d requests, want 2", got)
	}
}

// Every code the retryer retries as a throttle, and a bare 429 whose code no
// protocol models, is the throttled class on every surface that names a
// failure: what is retried as a throttle is reported as one.
func TestT582Dev_EveryRetriedThrottleIsTheThrottledClass(t *testing.T) {
	for code := range retry.DefaultThrottleErrorCodes {
		err := &smithy.GenericAPIError{Code: code, Message: "Rate exceeded", Fault: smithy.FaultClient}
		if got := awsclient.ErrClass(err); got != awsclient.ClassThrottled {
			t.Errorf("ErrClass(%s) = %q, want %q", code, got, awsclient.ClassThrottled)
		}
	}

	ddb := t582Services[4]
	c, _ := t582Clients(t, ddb, func(int64) t582Answer { return t582Answer{status: http.StatusTooManyRequests} })
	err := ddb.call(context.Background(), c)
	if err == nil {
		t.Fatal("an always-429 read succeeded")
	}
	if got := awsclient.ErrClass(err); got != awsclient.ClassThrottled {
		t.Errorf("ErrClass(bare 429 after retries) = %q, want %q (err %v)", got, awsclient.ClassThrottled, err)
	}
}

// A read still throttled when the caller's deadline leaves no room for
// another backoff ends before the deadline, as a throttle, never as a
// timeout that points the operator at the network.
func TestT582Dev_ThrottleStoppedByTheDeadlineStaysThrottled(t *testing.T) {
	ddb := t582Services[4]
	c, stub := t582DevClients(t, ddb, t582AlwaysThrottle, 600*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := ddb.call(ctx, c)
	if ctx.Err() != nil {
		t.Errorf("the read returned after its deadline (err %v)", err)
	}
	if got := awsclient.ErrClass(err); got != awsclient.ClassThrottled {
		t.Errorf("ErrClass = %q, want %q (err %v)", got, awsclient.ClassThrottled, err)
	}
	if n := stub.n.Load(); n < 2 {
		t.Errorf("stub saw %d requests, want a retry before the deadline stopped them", n)
	}
}

// The shortest deadline a9s puts on a read is 5 s. Under the production
// backoff every configured attempt of a persistently throttled read runs
// inside it.
func TestT582Dev_EveryAttemptFitsTheShortestDeadline(t *testing.T) {
	ddb := t582Services[4]
	c, stub := t582Clients(t, ddb, t582AlwaysThrottle)
	_ = ddb.call(context.Background(), c)
	budget := stub.n.Load()

	c, stub = t582DevClients(t, ddb, t582AlwaysThrottle, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := ddb.call(ctx, c)
	if got := stub.n.Load(); got != budget {
		t.Errorf("a 5 s deadline let %d of %d attempts run (err %v)", got, budget, err)
	}
}

// The session config carries the same retryer, so the credential clients the
// SDK builds while loading it (STS AssumeRole, SSO, web identity) retry like
// every service client.
func TestT582Dev_SessionConfigCarriesTheRetryer(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config")
	if err := os.WriteFile(cfgFile, []byte("[default]\nregion = us-east-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", cfgFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_MAX_ATTEMPTS", "")
	t.Setenv("AWS_RETRY_MODE", "")

	cfg, err := awsclient.NewAWSSessionContext(context.Background(), "", "us-east-1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Retryer == nil {
		t.Fatal("session config has no retryer; credential clients get the SDK default")
	}

	ddb := t582Services[4]
	c, stub := t582Clients(t, ddb, t582AlwaysThrottle)
	_ = ddb.call(context.Background(), c)
	if got, want := cfg.Retryer().MaxAttempts(), int(stub.n.Load()); got != want {
		t.Errorf("session retryer allows %d attempts, service clients make %d", got, want)
	}
}

// A loop around an SDK call that waits between turns is a retry beside the
// SDK retryer, whatever it waits with: SleepWithContext, a select on a
// Done channel, or a time wait. An SDK call is any method whose last
// parameter is the service's ...func(*Options).
func TestT582Dev_NoRetryLoopWaitsAroundACall(t *testing.T) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  filepath.Join("..", ".."),
	}
	pkgs, err := loadPackagesOnce(cfg, "./core/...", "./internal/...", "./cmd/...")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			t.Fatalf("load %s: %v", pkg.PkgPath, pkg.Errors)
		}
		for _, f := range pkg.Syntax {
			if strings.HasSuffix(pkg.Fset.Position(f.Pos()).Filename, "_test.go") {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				var body *ast.BlockStmt
				switch l := n.(type) {
				case *ast.ForStmt:
					body = l.Body
				case *ast.RangeStmt:
					body = l.Body
				default:
					return true
				}
				wait, call := t582DevLoopWaitAndCall(pkg.TypesInfo, body)
				if wait != nil && call {
					found[pkg.Fset.Position(wait.Pos()).String()] = true
				}
				return true
			})
		}
	}
	for _, site := range slices.Sorted(maps.Keys(found)) {
		t.Errorf("%s waits inside a loop around an SDK call: a retry beside the SDK retryer", site)
	}
}

func t582DevLoopWaitAndCall(info *types.Info, body *ast.BlockStmt) (wait ast.Node, sdkCall bool) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectStmt:
			for _, cc := range n.Body.List {
				if t582DevReceivesFromDone(cc.(*ast.CommClause).Comm) && wait == nil {
					wait = n
				}
			}
		case *ast.CallExpr:
			sel, ok := n.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if wait == nil && (sel.Sel.Name == "SleepWithContext" || t582DevIsTimeWait(info, sel)) {
				wait = n
			}
			if t582DevIsSDKCall(info, n) {
				sdkCall = true
			}
		}
		return true
	})
	return wait, sdkCall
}

func t582DevReceivesFromDone(comm ast.Stmt) bool {
	var x ast.Expr
	switch s := comm.(type) {
	case *ast.ExprStmt:
		x = s.X
	case *ast.AssignStmt:
		if len(s.Rhs) == 1 {
			x = s.Rhs[0]
		}
	}
	recv, ok := x.(*ast.UnaryExpr)
	if !ok {
		return false
	}
	call, ok := recv.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Done"
}

func t582DevIsSDKCall(info *types.Info, call *ast.CallExpr) bool {
	sig, ok := info.TypeOf(call.Fun).(*types.Signature)
	if !ok || !sig.Variadic() || sig.Params().Len() == 0 {
		return false
	}
	last, ok := sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice)
	if !ok {
		return false
	}
	fn, ok := last.Elem().(*types.Signature)
	if !ok || fn.Params().Len() != 1 {
		return false
	}
	ptr, ok := fn.Params().At(0).Type().(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	return ok && named.Obj().Name() == "Options" && named.Obj().Pkg() != nil &&
		strings.HasPrefix(named.Obj().Pkg().Path(), "github.com/aws/aws-sdk-go-v2/service/")
}

func t582DevIsTimeWait(info *types.Info, sel *ast.SelectorExpr) bool {
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	name, ok := info.Uses[pkg].(*types.PkgName)
	if !ok || name.Imported().Path() != "time" {
		return false
	}
	switch sel.Sel.Name {
	case "Sleep", "After", "NewTimer", "AfterFunc", "Tick", "NewTicker":
		return true
	}
	return false
}

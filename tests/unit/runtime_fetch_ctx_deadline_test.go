// runtime_fetch_ctx_deadline_test.go — the centralized ctx-deadline wrap on
// Core's interactive fetch entry points.
//
// internal/tui/fetch_adapter.go forwards a single cancel-only appCtx
// (context.WithCancel, captured once in internal/tui/app.go for the entire
// app run) to every interactive fetch lane, and core/aws has no
// context.WithTimeout of its own, so each of Core's interactive fetch entry
// points (core/runtime/fetchers.go) wraps the incoming ctx in a bounded
// context.WithTimeout before forwarding it to the registered fetcher / AWS
// SDK call. Both the TUI and web adapters inherit the deadline regardless of
// what the caller itself set up; a stalled network call cannot spin the
// caller forever.
//
// Capture strategy per lane:
//   - FetchResources / FetchResourcesFiltered / FetchChildResources /
//     FetchMoreResources / FetchRevealValue: a test-only fetcher registered
//     via resource.SetPaginatedForTest / SetFilteredPaginatedForTest /
//     SetPaginatedChildForTest / SetRevealFetcherForTest (the established
//     fake-fetcher seam, see tests/unit/aws_related_fetch_empty_test.go and
//     friends) captures the ctx it actually receives.
//   - FetchIdentity: Core.FetchIdentity forwards ctx to
//     awsclient.FetchCallerIdentity(ctx, clients.STS, clients.IAM), and
//     clients.STS is a concrete *sts.Client (core/aws/client.go) — not an
//     injectable interface. A real *sts.Client is built with a custom
//     aws.HTTPClient transport that captures the *http.Request's ctx and
//     returns a synthetic error before any real network I/O, so the pin
//     verifies the deadline reaches all the way to the AWS SDK call
//     boundary, not just a registry seam.
//
// The upper bound (120s) is pinned as "existence + sane bound", not an exact
// value: these pins only fail if a deadline is missing entirely or absurdly
// large, never on the exact chosen duration.
package unit

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// fetchDeadlineUpperBound is the sane upper bound pinned for every lane in
// this file — "the implementation will pick a specific value", so this is
// deliberately generous rather than exact.
const fetchDeadlineUpperBound = 120 * time.Second

// assertBoundedDeadline fails the test unless ctx is non-nil, carries a
// deadline (ctx.Deadline() ok==true), and that deadline is a positive,
// bounded duration from now.
func assertBoundedDeadline(t *testing.T, lane string, ctx context.Context) {
	t.Helper()
	if ctx == nil {
		t.Fatalf("%s: the fetcher/API call was never invoked with a ctx — cannot verify a deadline", lane)
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("%s: ctx.Deadline() returned ok=false — this lane forwards a ctx with no deadline, so a stalled call on it would hang forever", lane)
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		t.Fatalf("%s: ctx deadline %s is already in the past (remaining=%s)", lane, deadline, remaining)
	}
	if remaining > fetchDeadlineUpperBound {
		t.Fatalf("%s: ctx deadline is %s from now, want a bounded deadline <= %s", lane, remaining, fetchDeadlineUpperBound)
	}
}

// TestFetchResources_WrapsCtxWithBoundedDeadline pins FetchResources.
//
// RED today: FetchResources forwards ctx verbatim to the registered
// PaginatedFetcher with no WithTimeout wrap.
func TestFetchResources_WrapsCtxWithBoundedDeadline(t *testing.T) {
	const shortName = "deadline-test-fetchresources"
	var captured context.Context
	resource.SetPaginatedForTest(shortName, func(ctx context.Context, clients any, token string) (resource.FetchResult, error) {
		captured = ctx
		return resource.FetchResult{}, errors.New("stub fetcher: no real AWS call in this test")
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(shortName) })

	core := runtime.New(session.New(), nil)
	_, _ = core.FetchResources(context.Background(), nil, shortName)

	assertBoundedDeadline(t, "FetchResources", captured)
}

// TestFetchResourcesFiltered_WrapsCtxWithBoundedDeadline pins
// FetchResourcesFiltered.
func TestFetchResourcesFiltered_WrapsCtxWithBoundedDeadline(t *testing.T) {
	const shortName = "deadline-test-fetchresourcesfiltered"
	var captured context.Context
	resource.SetFilteredPaginatedForTest(shortName, func(ctx context.Context, clients any, filter map[string]string, token string) (resource.FetchResult, error) {
		captured = ctx
		return resource.FetchResult{}, errors.New("stub filtered fetcher: no real AWS call in this test")
	})
	t.Cleanup(func() { resource.CleanupFilteredPaginatedForTest(shortName) })

	core := runtime.New(session.New(), nil)
	clients := &awsclient.ServiceClients{}
	_, _ = core.FetchResourcesFiltered(context.Background(), clients, shortName, map[string]string{"resource": "r-1"})

	assertBoundedDeadline(t, "FetchResourcesFiltered", captured)
}

// TestFetchChildResources_WrapsCtxWithBoundedDeadline pins
// FetchChildResources.
//
// RED today: FetchChildResources forwards ctx verbatim to the registered
// PaginatedChildFetcher with no WithTimeout wrap.
func TestFetchChildResources_WrapsCtxWithBoundedDeadline(t *testing.T) {
	const shortName = "deadline-test-fetchchildresources"
	var captured context.Context
	resource.SetPaginatedChildForTest(shortName, func(ctx context.Context, clients any, parentCtx resource.ParentContext, token string) (resource.FetchResult, error) {
		captured = ctx
		return resource.FetchResult{}, errors.New("stub child fetcher: no real AWS call in this test")
	})
	t.Cleanup(func() { resource.CleanupPaginatedChildForTest(shortName) })

	core := runtime.New(session.New(), nil)
	clients := &awsclient.ServiceClients{}
	_, _ = core.FetchChildResources(context.Background(), clients, shortName, map[string]string{"parentID": "p-1"})

	assertBoundedDeadline(t, "FetchChildResources", captured)
}

// TestFetchMoreResources_WrapsCtxWithBoundedDeadline pins FetchMoreResources
// on its plain-paginated routing branch (no FetchFilter, no ParentCtx).
//
// RED today: FetchMoreResources forwards ctx verbatim to whichever fetcher it
// routes to, with no WithTimeout wrap.
func TestFetchMoreResources_WrapsCtxWithBoundedDeadline(t *testing.T) {
	const shortName = "deadline-test-fetchmoreresources"
	var captured context.Context
	resource.SetPaginatedForTest(shortName, func(ctx context.Context, clients any, token string) (resource.FetchResult, error) {
		captured = ctx
		return resource.FetchResult{}, errors.New("stub fetcher: no real AWS call in this test")
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(shortName) })

	core := runtime.New(session.New(), nil)
	clients := &awsclient.ServiceClients{}
	_, _ = core.FetchMoreResources(context.Background(), clients, runtime.FetchMoreParams{
		ResourceType: shortName,
		Token:        "next-token",
	})

	assertBoundedDeadline(t, "FetchMoreResources", captured)
}

// TestFetchRevealValue_WrapsCtxWithBoundedDeadline pins FetchRevealValue.
//
// RED today: FetchRevealValue forwards ctx verbatim to the registered
// RevealFetcher with no WithTimeout wrap.
func TestFetchRevealValue_WrapsCtxWithBoundedDeadline(t *testing.T) {
	const shortName = "deadline-test-fetchrevealvalue"
	var captured context.Context
	resource.SetRevealFetcherForTest(shortName, func(ctx context.Context, clients any, resourceID string) (string, error) {
		captured = ctx
		return "", errors.New("stub reveal fetcher: no real AWS call in this test")
	})
	t.Cleanup(func() { resource.CleanupRevealFetcherForTest(shortName) })

	core := runtime.New(session.New(), nil)
	clients := &awsclient.ServiceClients{}
	_, _ = core.FetchRevealValue(context.Background(), clients, shortName, "resource-1")

	assertBoundedDeadline(t, "FetchRevealValue", captured)
}

// ctxCaptureHTTPClient is an aws.HTTPClient that records the ctx of the first
// *http.Request it sees and returns a synthetic error immediately — no real
// network I/O ever happens, so a *sts.Client built with this transport is
// hermetic and fast regardless of network availability or credentials.
type ctxCaptureHTTPClient struct {
	mu       sync.Mutex
	captured context.Context
}

func (c *ctxCaptureHTTPClient) Do(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	if c.captured == nil {
		c.captured = req.Context()
	}
	c.mu.Unlock()
	return nil, errors.New("ctxCaptureHTTPClient: network access disabled in test")
}

func (c *ctxCaptureHTTPClient) Captured() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.captured
}

// TestFetchIdentity_WrapsCtxWithBoundedDeadline pins FetchIdentity all the
// way to the STS SDK call boundary.
//
// RED today: FetchIdentity forwards ctx verbatim into
// awsclient.FetchCallerIdentity -> stsClient.GetCallerIdentity with no
// WithTimeout wrap.
func TestFetchIdentity_WrapsCtxWithBoundedDeadline(t *testing.T) {
	transport := &ctxCaptureHTTPClient{}
	stsClient := sts.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIAFAKE00000EXAMPLE", "fakeSecretAccessKeyFakeSecretAccessKey00", ""),
		HTTPClient:  transport,
		Retryer:     func() aws.Retryer { return aws.NopRetryer{} },
	})

	core := runtime.New(session.New(), nil)
	clients := &awsclient.ServiceClients{STS: stsClient}
	_, _ = core.FetchIdentity(context.Background(), clients)

	assertBoundedDeadline(t, "FetchIdentity", transport.Captured())
}

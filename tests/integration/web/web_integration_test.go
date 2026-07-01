//go:build integration

// Package webintegration drives the real web server (internal/web) over HTTP
// in demo mode and asserts on GET /state JSON. All tests are deterministic —
// demo fetchers are synchronous, DrainSync runs inline, no sleeps needed.
//
// Gated behind the `integration` build tag (like the rest of tests/integration)
// so `make test` does not spin up real HTTP servers; `make integration` runs them.
//
// Coverage:
//   - Security contract: 403 without token, Cache-Control, no CORS, reveal blocked
//   - Startup: fresh session shows main menu
//   - All resource types: ActionCommand → list, columns present, rich types have rows
//   - Deeper list flows: sort, filter, cursor navigation
//   - Back navigation: full stack unwind
//   - Isolated sessions: two clients share server but have independent state
//   - Child views: wired via ActionCommand
//   - Harness-has-teeth: asserts a property the fix introduced; documents regression
//
// open-detail, open-yaml, open-json, child-view, and load-more are now wired
// through the controller (no longer stubs); the keyboard-driven navigation for
// those flows is exercised by the Playwright suite in tests/e2e.
package webintegration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/web"
)

// bgctx is a package-level background context used in place of context.Background()
// for HTTP requests — satisfies the noctx linter without threading t.Context()
// (which is Go 1.21+ and the tests are run under Go 1.26+, but keeps helpers simple).
var bgctx = context.Background() //nolint:gochecknoglobals // shared test context, not mutable state

// =============================================================================
// Harness helpers
// =============================================================================

// client is a test HTTP client that carries a session cookie and the per-run
// auth token. It is the single interface through which all tests talk to the
// server.
type client struct {
	baseURL string
	token   string
	http    *http.Client
}

// startServer boots a real web.Server on an ephemeral port in demo mode.
// It returns a ready client and a cleanup function that cancels the server.
// The function blocks until the server's readyCh is closed, so the returned
// client is immediately usable — no sleep or poll needed.
func startServer(t *testing.T) (*client, func()) {
	t.Helper()

	token, err := web.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	viewCfg := config.SharedDefaultConfig()
	srv := web.NewServer(
		demo.DemoProfile,
		demo.DemoRegion,
		"",            // no --command pre-navigation
		"127.0.0.1:0", // OS-assigned port
		token,
		true,  // demoMode
		true,  // noCache
		false, // allowReveal — keep reveal blocked
		viewCfg,
	)

	ctx, cancel := context.WithCancel(context.Background())
	readyCh := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		if listenErr := srv.ListenAndServe(ctx, readyCh); listenErr != nil && ctx.Err() == nil {
			errCh <- listenErr
		}
	}()

	// Wait for the server to bind (readyCh closed) — or fail fast if ListenAndServe
	// returns a startup error before binding (address taken/invalid, localhost
	// listening denied), instead of blocking on readyCh until the global timeout.
	select {
	case <-readyCh:
	case err := <-errCh:
		cancel()
		t.Fatalf("web server failed to start: %v", err)
	}

	jar, _ := cookiejar.New(nil)
	c := &client{
		baseURL: "http://" + srv.Addr(),
		token:   srv.Token(),
		http:    &http.Client{Jar: jar},
	}

	// Handshake: GET / to obtain the session cookie. The session cookie is set
	// in the response from the index endpoint — no token needed for the initial
	// page load.
	handshakeReq, _ := http.NewRequestWithContext(bgctx, http.MethodGet, c.baseURL+"/", nil)
	resp, handshakeErr := c.http.Do(handshakeReq)
	if handshakeErr != nil {
		cancel()
		t.Fatalf("initial GET /: %v", handshakeErr)
	}
	_ = resp.Body.Close()

	cleanup := func() { cancel() }
	return c, cleanup
}

// state fetches GET /state with the token in the query parameter and decodes
// the response into app.ViewState. The query-param path is consistent with
// EventSource, which cannot set custom headers.
func (c *client) state(t *testing.T) app.ViewState {
	t.Helper()
	u := c.baseURL + "/state?token=" + c.token
	req, _ := http.NewRequestWithContext(bgctx, http.MethodGet, u, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("GET /state: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /state: status %d body=%q", resp.StatusCode, body)
	}
	var vs app.ViewState
	if decErr := json.NewDecoder(resp.Body).Decode(&vs); decErr != nil {
		t.Fatalf("decode /state: %v", decErr)
	}
	return vs
}

// stateNoToken performs GET /state with no token and returns the HTTP status.
// Used by security tests that must observe 403 responses.
func (c *client) stateNoToken(t *testing.T) int {
	t.Helper()
	req, _ := http.NewRequestWithContext(bgctx, http.MethodGet, c.baseURL+"/state", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("GET /state (no token): %v", err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// action sends POST /action with the token in the X-A9S-Token header (CSRF
// protection) and kind+arg as form-encoded body. It fatals on transport or
// non-2xx response.
func (c *client) action(t *testing.T, kind app.ActionKind, arg string) {
	t.Helper()
	form := url.Values{}
	form.Set("kind", string(kind))
	if arg != "" {
		form.Set("arg", arg)
	}
	req, err := http.NewRequestWithContext(bgctx, http.MethodPost, c.baseURL+"/action", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("http.NewRequestWithContext POST /action: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-A9S-Token", c.token) // CSRF header

	resp, doErr := c.http.Do(req)
	if doErr != nil {
		t.Fatalf("POST /action kind=%q arg=%q: %v", kind, arg, doErr)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.Fatalf("POST /action kind=%q arg=%q: status %d", kind, arg, resp.StatusCode)
	}
}

// actionNoToken sends POST /action with NO auth token (neither header nor
// query) and returns the HTTP status. Used to verify auth rejection.
func (c *client) actionNoToken(t *testing.T, kind app.ActionKind) int {
	t.Helper()
	form := url.Values{}
	form.Set("kind", string(kind))
	req, err := http.NewRequestWithContext(bgctx, http.MethodPost, c.baseURL+"/action", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("http.NewRequestWithContext: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Deliberately omit both X-A9S-Token header and token query param.

	resp, doErr := c.http.Do(req)
	if doErr != nil {
		t.Fatalf("POST /action (no token): %v", doErr)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// responseHeaders fetches GET /state with a valid token and returns the
// response headers. Used by security tests.
func (c *client) responseHeaders(t *testing.T) http.Header {
	t.Helper()
	req, _ := http.NewRequestWithContext(bgctx, http.MethodGet, c.baseURL+"/state?token="+c.token, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("GET /state for headers: %v", err)
	}
	_ = resp.Body.Close()
	return resp.Header
}

// =============================================================================
// 1. SECURITY CONTRACT
// =============================================================================

// TestWebSecurity_StateWithNoToken_Returns403 verifies that /state without a
// token responds 403. This is the primary auth gate — every live-AWS-state
// endpoint must reject unauthenticated requests.
func TestWebSecurity_StateWithNoToken_Returns403(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	status := c.stateNoToken(t)
	if status != http.StatusForbidden {
		t.Errorf("expected 403, got %d — unauthenticated /state must be rejected", status)
	}
}

// TestWebSecurity_ActionWithNoToken_Returns403 verifies that POST /action
// without any token is rejected with 403.
func TestWebSecurity_ActionWithNoToken_Returns403(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	status := c.actionNoToken(t, app.ActionMoveDown)
	if status != http.StatusForbidden {
		t.Errorf("expected 403, got %d — unauthenticated POST /action must be rejected", status)
	}
}

// TestWebSecurity_ResponseHasCacheControlNoStore verifies that every response
// carries Cache-Control: no-store — AWS state must never be cached by
// proxies, CDNs, or browser caches.
func TestWebSecurity_ResponseHasCacheControlNoStore(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	headers := c.responseHeaders(t)
	cc := headers.Get("Cache-Control")
	if !strings.Contains(cc, "no-store") {
		t.Errorf("expected Cache-Control: no-store, got %q", cc)
	}
}

// TestWebSecurity_ResponseHasNoCORSHeader verifies that responses do not carry
// Access-Control-Allow-Origin. The server must never be CORS-accessible —
// live AWS state is sensitive and must not be readable by cross-origin pages.
func TestWebSecurity_ResponseHasNoCORSHeader(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	headers := c.responseHeaders(t)
	if origin := headers.Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected no Access-Control-Allow-Origin header, got %q", origin)
	}
}

// TestWebSecurity_ActionReveal_BlockedByDefault verifies that ActionReveal is
// rejected with 403 when the server was started with allowReveal=false.
// Revealing secret values over HTTP is off by default — requires an explicit
// opt-in flag at server construction time.
func TestWebSecurity_ActionReveal_BlockedByDefault(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	form := url.Values{}
	form.Set("kind", string(app.ActionReveal))
	req, err := http.NewRequestWithContext(bgctx, http.MethodPost, c.baseURL+"/action", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("http.NewRequestWithContext: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-A9S-Token", c.token)

	resp, doErr := c.http.Do(req)
	if doErr != nil {
		t.Fatalf("POST /action reveal: %v", doErr)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for ActionReveal with allowReveal=false, got %d", resp.StatusCode)
	}
}

// =============================================================================
// 2. STARTUP — fresh session must show main menu
// =============================================================================

// TestWebStartup_FreshSession_ShowsMenu verifies that a fresh session starts
// on the main menu screen with non-empty entries. This is the entry state for
// every web session.
func TestWebStartup_FreshSession_ShowsMenu(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindMenu {
		t.Fatalf("expected Body.Kind==%q, got %q", app.BodyKindMenu, vs.Body.Kind)
	}
	if vs.Body.Menu == nil {
		t.Fatal("Body.Menu is nil on fresh session")
	}
	if len(vs.Body.Menu.Entries) == 0 {
		t.Error("Body.Menu.Entries is empty — main menu must list resource types")
	}
}

// TestWebStartup_Header_HasDemoProfileAndRegion verifies the header reflects
// the demo profile and region used to construct the server.
func TestWebStartup_Header_HasDemoProfileAndRegion(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	vs := c.state(t)

	if vs.Header.Profile != demo.DemoProfile {
		t.Errorf("Header.Profile=%q, want %q", vs.Header.Profile, demo.DemoProfile)
	}
	if vs.Header.Region != demo.DemoRegion {
		t.Errorf("Header.Region=%q, want %q", vs.Header.Region, demo.DemoRegion)
	}
}

// =============================================================================
// 3. ALL RESOURCE TYPES — list navigation
// =============================================================================

// richDemoTypes lists short names known to have populated demo fixtures.
// These types MUST assert len(Rows) > 0 in the list coverage test.
// Types not listed here may legitimately have zero rows (e.g. child-only types,
// ct-events without a filter, types with no demo fixtures seeded yet).
var richDemoTypes = map[string]bool{
	"ec2":        true,
	"s3":         true,
	"dbi":        true,
	"lambda":     true,
	"ecs":        true,
	"iam-user":   true,
	"role":       true,
	"secrets":    true,
	"eks":        true,
	"redis":      true,
	"dbc":        true,
	"sg":         true,
	"vpc":        true,
	"ddb":        true,
	"elb":        true,
	"efs":        true,
	"asg":        true,
	"cfn":        true,
	"kms":        true,
	"backup":     true,
	"logs":       true,
	"alarm":      true,
	"cf":         true,
	"r53":        true,
	"ecr":        true,
	"sns":        true,
	"sqs":        true,
	"sfn":        true,
	"ssm":        true,
	"eb":         true,
	"redshift":   true,
	"opensearch": true,
	"dbi-snap":   true,
	"dbc-snap":   true,
	"ses":        true,
	"kinesis":    true,
	"glue":       true,
	"acm":        true,
	"waf":        true,
}

// TestWebAllResourceTypes_List_NavigatesAndShowsColumns verifies that for every
// resource type: ActionCommand navigates to a list screen, and the list has
// columns defined. For rich types (those with demo fixtures), asserts rows > 0.
//
// This test guards the PR-D regression: before the cached-nav applyResourcesLoaded
// fix, navigating via ActionCommand after a cached navigate would return a list
// screen with Body.List.Rows == nil even for richly-seeded demo types.
func TestWebAllResourceTypes_List_NavigatesAndShowsColumns(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Fatal("AllResourceTypes() returned empty — catalog not installed")
	}

	// Guard against stale richDemoTypes keys: every key MUST be a registered short
	// name, else the Rows>0 assertion below is silently skipped and a regression in
	// that list passes unnoticed (the cwlogs/cloudwatch/cloudfront/rds drift this
	// guard catches).
	registered := make(map[string]bool, len(allTypes))
	for _, rt := range allTypes {
		registered[rt.ShortName] = true
	}
	for k := range richDemoTypes {
		if !registered[k] {
			t.Errorf("richDemoTypes key %q is not a registered resource short name — "+
				"its Rows>0 assertion is silently skipped; fix the stale/aliased name", k)
		}
	}

	for _, rt := range allTypes {
		rt := rt // capture
		t.Run(rt.ShortName, func(t *testing.T) {
			// Each sub-test uses its own isolated server + session so resource
			// types do not interfere with each other's navigation state.
			c, cleanup := startServer(t)
			defer cleanup()

			// Navigate to the resource list.
			c.action(t, app.ActionCommand, rt.ShortName)

			vs := c.state(t)

			if vs.Body.Kind != app.BodyKindList {
				t.Fatalf("after ActionCommand(%q): Body.Kind=%q, want %q",
					rt.ShortName, vs.Body.Kind, app.BodyKindList)
			}
			if vs.Body.List == nil {
				t.Fatal("Body.List is nil")
			}
			if len(vs.Body.List.Columns) == 0 {
				t.Errorf("Body.List.Columns is empty — resource type %q must have at least one column", rt.ShortName)
			}

			if richDemoTypes[rt.ShortName] {
				if len(vs.Body.List.Rows) == 0 {
					t.Errorf("Body.List.Rows is empty for %q — demo fixtures must populate rows "+
						"(this catches the cached-nav applyResourcesLoaded regression)", rt.ShortName)
				} else {
					// Verify the first row has a populated ResourceID and cells.
					first := vs.Body.List.Rows[0]
					if first.ResourceID == "" {
						t.Errorf("Rows[0].ResourceID is empty for %q — rows must carry an ID", rt.ShortName)
					}
					if len(first.Cells) == 0 {
						t.Errorf("Rows[0].Cells is empty for %q — rows must have cell values", rt.ShortName)
					}
				}
			}
			// Types not in richDemoTypes may legitimately have zero rows (child-only
			// types, ct-events without a filter, etc.). For these we assert only
			// Kind==list and columns present — which is still meaningful because it
			// proves the navigation path is wired and the type is registered.
		})
	}
}

// =============================================================================
// 4. HARNESS-HAS-TEETH TEST
// =============================================================================

// TestWebHarnessTeeth_EC2ListHasRows_Regression is the "harness has teeth" test
// required by the PR-E plan. It asserts a property the current code satisfies —
// len(Body.List.Rows) > 0 for ec2 — and documents that before the PR-D fix this
// assertion would fail.
//
// Regression story:
//   - Pre-fix: ActionCommand pushed a list screen via the controller's Apply path.
//     DrainSync executed the fetch task and called Handle with the ResourcesLoaded
//     event. But Handle's result lane dispatched only the original 6 events; the
//     ResourcesLoaded case was missing. The event was a no-op: rows were never
//     written to the controller screen stack. Snapshot().Body.List.Rows was nil.
//   - Post-fix (PR-D): the Handle method routes ResourcesLoaded through
//     applyResourcesLoaded, which finds the topmost matching list screen on the
//     stack and sets its Rows. Snapshot().Body.List.Rows is populated.
//   - Therefore: len > 0 passes now; it would have returned 0 before — the
//     harness catches the exact regression that PR-D fixed.
func TestWebHarnessTeeth_EC2ListHasRows_Regression(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	c.action(t, app.ActionCommand, "ec2")

	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("Body.Kind=%q, want %q", vs.Body.Kind, app.BodyKindList)
	}
	if vs.Body.List == nil {
		t.Fatal("Body.List is nil")
	}

	// ASSERTION WITH TEETH: passes post-PR-D-fix; would have been 0 before.
	// Before the fix: DrainSync ran the fetch task, but ResourcesLoaded was not
	// dispatched on the Handle result lane — list screen stayed empty.
	// After the fix: applyResourcesLoaded populates the rows on Handle.
	if len(vs.Body.List.Rows) == 0 {
		t.Errorf("Body.List.Rows is empty for ec2 — " +
			"pre-PR-D-fix this was always 0 because ResourcesLoaded was not wired " +
			"on the controller Handle lane; if you see this failure, applyResourcesLoaded " +
			"is broken or was reverted")
	}

	// Secondary: first row must carry an instance ID.
	if len(vs.Body.List.Rows) > 0 {
		row := vs.Body.List.Rows[0]
		if row.ResourceID == "" {
			t.Error("Rows[0].ResourceID is empty — ResourceID must be propagated from demo fixtures")
		}
	}
}

// =============================================================================
// 5. LIST NAVIGATION — cursor and back
// =============================================================================

// navigateToListWithRows is a helper that navigates to the named resource list
// and verifies it has rows. Returns the list body or fatals.
func navigateToListWithRows(t *testing.T, c *client, shortName string) *app.ListBody {
	t.Helper()
	c.action(t, app.ActionCommand, shortName)
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("[%s] expected list, got %q", shortName, vs.Body.Kind)
	}
	if vs.Body.List == nil {
		t.Fatalf("[%s] Body.List is nil", shortName)
	}
	if len(vs.Body.List.Rows) == 0 {
		t.Fatalf("[%s] Body.List.Rows is empty — need rows to run this test", shortName)
	}
	return vs.Body.List
}

// TestWebCursorNavigation_MoveDownAdvancesSelected verifies that ActionMoveDown
// increments Body.List.Selected when the list has more than one row.
func TestWebCursorNavigation_MoveDownAdvancesSelected(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	lb := navigateToListWithRows(t, c, "ec2")
	if len(lb.Rows) < 2 {
		t.Skip("ec2 list has fewer than 2 rows — move-down test not meaningful")
	}

	initialSelected := lb.Selected

	c.action(t, app.ActionMoveDown, "")
	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("after move-down: Body.Kind=%q", vs.Body.Kind)
	}
	if vs.Body.List.Selected <= initialSelected {
		t.Errorf("Selected did not advance: initial=%d current=%d", initialSelected, vs.Body.List.Selected)
	}
}

// TestWebCursorNavigation_MoveUp_AtTop_DoesNotGoNegative verifies that
// ActionMoveUp when already at row 0 does not wrap to a negative index.
func TestWebCursorNavigation_MoveUp_AtTop_DoesNotGoNegative(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	navigateToListWithRows(t, c, "ec2")

	c.action(t, app.ActionMoveUp, "")
	vs := c.state(t)

	if vs.Body.List.Selected < 0 {
		t.Errorf("Selected went negative after move-up at top: %d", vs.Body.List.Selected)
	}
}

// TestWebCursorNavigation_MoveTop_ResetsToZero verifies that ActionMoveTop
// moves the cursor to row 0.
func TestWebCursorNavigation_MoveTop_ResetsToZero(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	lb := navigateToListWithRows(t, c, "ec2")
	if len(lb.Rows) < 3 {
		t.Skip("ec2 list has fewer than 3 rows — move-top test not meaningful")
	}

	// Move down twice then move to top.
	c.action(t, app.ActionMoveDown, "")
	c.action(t, app.ActionMoveDown, "")
	c.action(t, app.ActionMoveTop, "")

	vs := c.state(t)
	if vs.Body.List.Selected != 0 {
		t.Errorf("after move-top: Selected=%d, want 0", vs.Body.List.Selected)
	}
}

// TestWebNavigation_BackFromList_ReturnsToMenu verifies that ActionBack from a
// resource list returns to the main menu.
func TestWebNavigation_BackFromList_ReturnsToMenu(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	c.action(t, app.ActionCommand, "ec2")
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("expected list after command ec2, got %q", vs.Body.Kind)
	}

	c.action(t, app.ActionBack, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindMenu {
		t.Fatalf("after back from list: expected menu, got %q", vs.Body.Kind)
	}
}

// TestWebNavigation_CommandTwice_NavigatesToSameType verifies that issuing
// ActionCommand for the same type twice (back → command again) returns a fresh
// list with rows.
func TestWebNavigation_CommandTwice_NavigatesToSameType(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	c.action(t, app.ActionCommand, "lambda")
	c.action(t, app.ActionBack, "")
	c.action(t, app.ActionCommand, "lambda")

	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("second command lambda: expected list, got %q", vs.Body.Kind)
	}
	if len(vs.Body.List.Rows) == 0 {
		t.Error("second command lambda: rows are empty — rows must persist across re-navigation")
	}
}

// =============================================================================
// 6. FILTER
// =============================================================================

// TestWebFilter_SetFilter_StoresFilterInBody verifies that ActionSetFilter with
// a known substring stores the value in Body.List.Filter.
func TestWebFilter_SetFilter_StoresFilterInBody(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	lb := navigateToListWithRows(t, c, "ec2")

	// Use the first row's first non-empty cell as the needle.
	var needle string
	for _, row := range lb.Rows {
		for _, cell := range row.Cells {
			if cell != "" {
				needle = cell
				break
			}
		}
		if needle != "" {
			break
		}
	}
	if needle == "" {
		t.Skip("no non-empty cell found in ec2 list — filter test not meaningful")
	}

	c.action(t, app.ActionSetFilter, needle)
	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("after set-filter: Body.Kind=%q, want %q", vs.Body.Kind, app.BodyKindList)
	}
	if vs.Body.List.Filter != needle {
		t.Errorf("Body.List.Filter=%q, want %q", vs.Body.List.Filter, needle)
	}
}

// TestWebFilter_SetFilter_ClearWithEmpty_ResetsFilter verifies that applying an
// empty filter clears the filter state.
func TestWebFilter_SetFilter_ClearWithEmpty_ResetsFilter(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	lb := navigateToListWithRows(t, c, "ec2")

	var needle string
	for _, row := range lb.Rows {
		if len(row.Cells) > 0 && row.Cells[0] != "" {
			needle = row.Cells[0]
			break
		}
	}
	if needle == "" {
		t.Skip("no cell value for filter test")
	}

	c.action(t, app.ActionSetFilter, needle)
	c.action(t, app.ActionSetFilter, "") // clear
	vs := c.state(t)

	if vs.Body.List.Filter != "" {
		t.Errorf("Body.List.Filter=%q after clearing, want empty string", vs.Body.List.Filter)
	}
}

// =============================================================================
// 7. SORT
// =============================================================================

// firstSortableColumn returns the first column key that is non-empty (i.e.
// eligible for ActionSort). Many list columns have empty Key because they are
// rendered from denormalised fields that do not map to a sortable path; only
// columns with an explicit Key string participate in sort.
func firstSortableColumn(t *testing.T, lb *app.ListBody) string {
	t.Helper()
	for _, col := range lb.Columns {
		if col.Key != "" {
			return col.Key
		}
	}
	return ""
}

// TestWebSort_Sort_SetsSortCol verifies that ActionSort with a valid non-empty
// column key updates Sort.Col in the list body.
func TestWebSort_Sort_SetsSortCol(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	lb := navigateToListWithRows(t, c, "ec2")
	col := firstSortableColumn(t, lb)
	if col == "" {
		t.Skip("ec2 list has no columns with a non-empty Key — sort test not meaningful")
	}

	c.action(t, app.ActionSort, col)
	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("after sort: Body.Kind=%q, want list", vs.Body.Kind)
	}
	if vs.Body.List.Sort.Col != col {
		t.Errorf("Body.List.Sort.Col=%q, want %q", vs.Body.List.Sort.Col, col)
	}
}

// TestWebSort_Sort_SecondCallTogglesDir verifies that sorting by the same column
// a second time toggles the sort direction from "asc" to "desc".
func TestWebSort_Sort_SecondCallTogglesDir(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	lb := navigateToListWithRows(t, c, "ec2")
	col := firstSortableColumn(t, lb)
	if col == "" {
		t.Skip("ec2 list has no columns with a non-empty Key — sort toggle test not meaningful")
	}

	c.action(t, app.ActionSort, col)
	vs1 := c.state(t)
	dir1 := vs1.Body.List.Sort.Dir

	c.action(t, app.ActionSort, col)
	vs2 := c.state(t)
	dir2 := vs2.Body.List.Sort.Dir

	if dir1 == dir2 {
		t.Errorf("sort direction did not toggle on second call: both %q (col=%q)", dir1, col)
	}
}

// =============================================================================
// 8. ISOLATED SESSIONS
// =============================================================================

// TestWebIsolatedSessions_TwoClientsAreIndependent verifies that two clients
// with different session cookies have independent controller state — navigating
// in one does not affect the other.
func TestWebIsolatedSessions_TwoClientsAreIndependent(t *testing.T) {
	c1, cleanup := startServer(t)
	defer cleanup()

	// c2 shares the same server but uses a fresh cookie jar — distinct session.
	jar2, _ := cookiejar.New(nil)
	c2 := &client{
		baseURL: c1.baseURL,
		token:   c1.token,
		http:    &http.Client{Jar: jar2},
	}
	// Handshake for c2.
	req2, err := http.NewRequestWithContext(bgctx, http.MethodGet, c2.baseURL+"/", nil)
	if err != nil {
		t.Fatalf("c2 NewRequestWithContext: %v", err)
	}
	resp, err := c2.http.Do(req2)
	if err != nil {
		t.Fatalf("c2 GET /: %v", err)
	}
	_ = resp.Body.Close()

	// Navigate c1 to ec2 list.
	c1.action(t, app.ActionCommand, "ec2")

	// c2 must still be on the menu.
	vs2 := c2.state(t)
	if vs2.Body.Kind != app.BodyKindMenu {
		t.Errorf("c2 should be on menu (got %q) — sessions must be isolated from each other", vs2.Body.Kind)
	}

	// c1 must be on the list.
	vs1 := c1.state(t)
	if vs1.Body.Kind != app.BodyKindList {
		t.Errorf("c1 should be on list (got %q)", vs1.Body.Kind)
	}
}

// =============================================================================
// 9. HELP SCREEN
// =============================================================================

// TestWebHelp_OpenHelp_ShowsHelpBody verifies ActionOpenHelp navigates to the
// help overlay screen.
func TestWebHelp_OpenHelp_ShowsHelpBody(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	c.action(t, app.ActionOpenHelp, "")
	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindHelp {
		t.Fatalf("after open-help: Body.Kind=%q, want %q", vs.Body.Kind, app.BodyKindHelp)
	}
}

// TestWebHelp_BackFromHelp_ReturnsToMenu verifies that ActionBack from the help
// screen returns to whichever screen opened it (menu in this case).
func TestWebHelp_BackFromHelp_ReturnsToMenu(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	c.action(t, app.ActionOpenHelp, "")
	c.action(t, app.ActionBack, "")
	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindMenu {
		t.Fatalf("after back from help: expected menu, got %q", vs.Body.Kind)
	}
}

// =============================================================================
// 10. MULTIPLE RESOURCE TYPES — deeper list state
// =============================================================================

// TestWebMultipleTypes_SwitchBetweenTypes verifies that navigating between two
// different resource types (ec2 → lambda) gives each its own fresh list state.
func TestWebMultipleTypes_SwitchBetweenTypes(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	c.action(t, app.ActionCommand, "ec2")
	vsEC2 := c.state(t)
	if vsEC2.Body.Kind != app.BodyKindList {
		t.Fatalf("ec2: expected list, got %q", vsEC2.Body.Kind)
	}

	c.action(t, app.ActionBack, "")
	c.action(t, app.ActionCommand, "lambda")
	vsLambda := c.state(t)
	if vsLambda.Body.Kind != app.BodyKindList {
		t.Fatalf("lambda: expected list, got %q", vsLambda.Body.Kind)
	}

	// The two lists should have different columns (ec2 has instance-specific
	// columns, lambda has function-specific columns).
	if len(vsEC2.Body.List.Columns) > 0 && len(vsLambda.Body.List.Columns) > 0 {
		if vsEC2.Body.List.Columns[0].Key == vsLambda.Body.List.Columns[0].Key {
			t.Logf("ec2 and lambda share the same first column key %q — may be expected for short names", vsEC2.Body.List.Columns[0].Key)
		}
	}
	// Primarily assert both lists have rows.
	if len(vsEC2.Body.List.Rows) == 0 {
		t.Error("ec2 list rows empty")
	}
	if len(vsLambda.Body.List.Rows) == 0 {
		t.Error("lambda list rows empty")
	}
}

// TestWebRichTypes_AllHaveNonEmptyRows spot-checks a spread of rich types that
// all have demo fixtures: s3, dbi, ecs, eks, secrets, ddb, redis.
func TestWebRichTypes_AllHaveNonEmptyRows(t *testing.T) {
	checks := []string{"s3", "dbi", "ecs", "eks", "secrets", "ddb", "redis"}

	for _, shortName := range checks {
		shortName := shortName
		t.Run(shortName, func(t *testing.T) {
			c, cleanup := startServer(t)
			defer cleanup()

			c.action(t, app.ActionCommand, shortName)
			vs := c.state(t)

			if vs.Body.Kind != app.BodyKindList {
				t.Fatalf("[%s] expected list, got %q", shortName, vs.Body.Kind)
			}
			if len(vs.Body.List.Rows) == 0 {
				t.Errorf("[%s] Rows is empty — demo fixtures must populate rows", shortName)
			}
			if len(vs.Body.List.Rows) > 0 && vs.Body.List.Rows[0].ResourceID == "" {
				t.Errorf("[%s] Rows[0].ResourceID is empty", shortName)
			}
		})
	}
}

// =============================================================================
// 11. LIST → DETAIL — open-detail flow (PR-E new, wired in commit 2fb04f2b)
// =============================================================================

// navigateToDetail is a helper: navigates list → selects first row → opens
// detail. Returns the DetailBody or fatals. Callers must start their own server.
func navigateToDetail(t *testing.T, c *client, shortName string) *app.DetailBody {
	t.Helper()
	navigateToListWithRows(t, c, shortName)

	// open-detail uses the currently-selected row (index 0 by default).
	c.action(t, app.ActionOpenDetail, "")
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("[%s] after open-detail: Body.Kind=%q, want %q",
			shortName, vs.Body.Kind, app.BodyKindDetail)
	}
	if vs.Body.Detail == nil {
		t.Fatalf("[%s] Body.Detail is nil after open-detail", shortName)
	}
	return vs.Body.Detail
}

// TestWebDetail_OpenDetail_ShowsDetailBody verifies that ActionOpenDetail from
// a list with a selected row navigates to a detail screen with non-empty Fields.
// Tests a representative spread: ec2, s3, dbi, lambda, ecs.
func TestWebDetail_OpenDetail_ShowsDetailBody(t *testing.T) {
	// Representative spread of rich types with distinct field structures.
	types := []string{"ec2", "s3", "dbi", "lambda", "ecs"}

	for _, shortName := range types {
		shortName := shortName
		t.Run(shortName, func(t *testing.T) {
			c, cleanup := startServer(t)
			defer cleanup()

			db := navigateToDetail(t, c, shortName)

			// The detail body must carry Fields — the headless resourceFieldList builder
			// must have populated at least one key-value pair for any real resource.
			if len(db.Fields) == 0 {
				t.Errorf("[%s] Body.Detail.Fields is empty — detail projector must populate fields", shortName)
			}
		})
	}
}

// TestWebDetail_OpenDetail_RelatedVisibleForTypesWithRelatedDefs verifies that
// types registered with at least one RelatedDef show RelatedVisible==true in
// the detail body. Tests dbi and lambda (both have inline Related defs).
func TestWebDetail_OpenDetail_RelatedVisibleForTypesWithRelatedDefs(t *testing.T) {
	// dbi and lambda are confirmed to have Related: []domain.RelatedDef{...} in
	// the catalog. Their demo fixtures are populated, so detail always has a row.
	relatedTypes := []string{"dbi", "lambda"}

	for _, shortName := range relatedTypes {
		shortName := shortName
		t.Run(shortName, func(t *testing.T) {
			c, cleanup := startServer(t)
			defer cleanup()

			db := navigateToDetail(t, c, shortName)

			if !db.RelatedVisible {
				t.Errorf("[%s] Body.Detail.RelatedVisible=false — types with RelatedDefs must show the related panel", shortName)
			}
			// Related slice may be non-nil even when all rows are still loading.
			// Asserting non-nil is sufficient to prove the panel was initialised.
			if db.Related == nil {
				t.Errorf("[%s] Body.Detail.Related is nil — related panel entries must be initialised", shortName)
			}
		})
	}
}

// TestWebDetail_BackFromDetail_ReturnsToList verifies that ActionBack from a
// detail screen returns to the parent resource list.
func TestWebDetail_BackFromDetail_ReturnsToList(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	navigateToDetail(t, c, "ec2")

	c.action(t, app.ActionBack, "")
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("after back from detail: expected list, got %q", vs.Body.Kind)
	}
}

// =============================================================================
// 12. DETAIL → YAML / JSON — text-body flows (PR-E new)
// =============================================================================

// TestWebYAML_OpenYAML_ShowsTextBodyWithLines verifies that ActionOpenYAML from
// a detail screen pushes a text body with real YAML content. Empty Lines is the
// regression: the headless resourceYAMLLines builder must produce at least one
// line for any real resource.
func TestWebYAML_OpenYAML_ShowsTextBodyWithLines(t *testing.T) {
	// ec2 is known to produce ~40 YAML lines in demo mode (verified over HTTP).
	// Test a spread to catch per-type projector gaps.
	types := []string{"ec2", "dbi", "lambda", "s3"}

	for _, shortName := range types {
		shortName := shortName
		t.Run(shortName, func(t *testing.T) {
			c, cleanup := startServer(t)
			defer cleanup()

			navigateToDetail(t, c, shortName)

			c.action(t, app.ActionOpenYAML, "")
			vs := c.state(t)

			if vs.Body.Kind != app.BodyKindText {
				t.Fatalf("[%s] after open-yaml from detail: Body.Kind=%q, want %q",
					shortName, vs.Body.Kind, app.BodyKindText)
			}
			if vs.Body.Text == nil {
				t.Fatalf("[%s] Body.Text is nil after open-yaml", shortName)
			}
			// ASSERTION WITH TEETH: empty Lines == regression in the headless
			// resourceYAMLLines builder (serialises nothing).
			if len(vs.Body.Text.Lines) == 0 {
				t.Errorf("[%s] Body.Text.Lines is empty after open-yaml — "+
					"the headless YAML builder must produce content; "+
					"empty Lines means the projector returned a zero-value resource", shortName)
			}
		})
	}
}

// TestWebYAML_OpenJSON_ShowsTextBodyWithLines verifies that ActionOpenJSON from
// a detail screen produces a text body with JSON content.
func TestWebYAML_OpenJSON_ShowsTextBodyWithLines(t *testing.T) {
	types := []string{"ec2", "dbi"}

	for _, shortName := range types {
		shortName := shortName
		t.Run(shortName, func(t *testing.T) {
			c, cleanup := startServer(t)
			defer cleanup()

			navigateToDetail(t, c, shortName)

			c.action(t, app.ActionOpenJSON, "")
			vs := c.state(t)

			if vs.Body.Kind != app.BodyKindText {
				t.Fatalf("[%s] after open-json from detail: Body.Kind=%q, want %q",
					shortName, vs.Body.Kind, app.BodyKindText)
			}
			if vs.Body.Text == nil {
				t.Fatalf("[%s] Body.Text is nil after open-json", shortName)
			}
			if len(vs.Body.Text.Lines) == 0 {
				t.Errorf("[%s] Body.Text.Lines is empty after open-json — JSON builder must produce content", shortName)
			}
		})
	}
}

// TestWebYAML_BackFromYAML_ReturnsToDetail verifies that ActionBack from a YAML
// text screen returns to the detail screen (unwinds one stack frame).
func TestWebYAML_BackFromYAML_ReturnsToDetail(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	navigateToDetail(t, c, "ec2")

	c.action(t, app.ActionOpenYAML, "")
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindText {
		t.Fatalf("expected text after open-yaml, got %q", vs.Body.Kind)
	}

	c.action(t, app.ActionBack, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("after back from yaml: expected detail, got %q", vs.Body.Kind)
	}
}

// TestWebYAML_OpenJSON_BackReturnsToDetail verifies back-from-JSON also returns
// to detail.
func TestWebYAML_OpenJSON_BackReturnsToDetail(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	navigateToDetail(t, c, "ec2")

	c.action(t, app.ActionOpenJSON, "")
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindText {
		t.Fatalf("expected text after open-json, got %q", vs.Body.Kind)
	}

	c.action(t, app.ActionBack, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("after back from json: expected detail, got %q", vs.Body.Kind)
	}
}

// TestWebYAML_OpenYAML_DirectFromList verifies that ActionOpenYAML can also be
// invoked directly from the list screen (selectedResourceForAction resolves
// from the selected list row when no detail is on the stack).
func TestWebYAML_OpenYAML_DirectFromList(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	navigateToListWithRows(t, c, "ec2")

	c.action(t, app.ActionOpenYAML, "")
	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindText {
		t.Fatalf("open-yaml from list: Body.Kind=%q, want %q", vs.Body.Kind, app.BodyKindText)
	}
	if vs.Body.Text == nil || len(vs.Body.Text.Lines) == 0 {
		t.Errorf("open-yaml from list: Lines empty — must produce YAML content for selected row")
	}
}

// =============================================================================
// 13. CHILD VIEWS — ActionChildView with trigger keys (PR-E new)
// =============================================================================

// childViewCase describes one child-view scenario to drive via HTTP.
type childViewCase struct {
	// parentType is the resource short name that declares Children.
	parentType string
	// triggerKey is the Key field from the ChildViewDef (e.g. "enter", "e", "L").
	triggerKey string
	// expectedChildType is the ChildType value expected after navigation.
	// We do not assert on it directly but document for reviewer clarity.
	expectedChildType string
}

// TestWebChildView_ActionChildView_NavigatesToChildList verifies that
// ActionChildView with a registered trigger key, called from a list screen
// with a selected row, pushes a child-list screen (Body.Kind==list).
//
// The child list must have Columns defined. Rows may be empty (child fetcher
// runs async in the real runtime; in demo-sync mode they may be populated).
//
// Trigger keys are the Key field from ChildViewDef, NOT raw keystrokes:
//
//	ecs-svc: "enter"→ecs_tasks, "e"→ecs_svc_events, "L"→ecs_svc_logs
//	lambda:  "enter"→lambda_invocations
//	asg:     "enter"→asg_activities
//	dbi:     "enter"→dbi_events
//	s3:      "enter"→s3_objects
//	sfn:     "enter"→sfn_executions
func TestWebChildView_ActionChildView_NavigatesToChildList(t *testing.T) {
	cases := []childViewCase{
		// ecs-svc has three children with three distinct trigger keys.
		{parentType: "ecs-svc", triggerKey: "enter", expectedChildType: "ecs_tasks"},
		{parentType: "ecs-svc", triggerKey: "e", expectedChildType: "ecs_svc_events"},
		{parentType: "ecs-svc", triggerKey: "L", expectedChildType: "ecs_svc_logs"},
		// lambda: single child with "enter".
		{parentType: "lambda", triggerKey: "enter", expectedChildType: "lambda_invocations"},
		// asg: single child with "enter".
		{parentType: "asg", triggerKey: "enter", expectedChildType: "asg_activities"},
		// dbi: single child with "enter".
		{parentType: "dbi", triggerKey: "enter", expectedChildType: "dbi_events"},
		// s3: single child with "enter".
		{parentType: "s3", triggerKey: "enter", expectedChildType: "s3_objects"},
		// sfn: single child with "enter".
		{parentType: "sfn", triggerKey: "enter", expectedChildType: "sfn_executions"},
	}

	for _, tc := range cases {
		tc := tc
		name := tc.parentType + "/" + tc.triggerKey
		t.Run(name, func(t *testing.T) {
			c, cleanup := startServer(t)
			defer cleanup()

			// Navigate to the parent list and verify it has rows.
			navigateToListWithRows(t, c, tc.parentType)

			// Dispatch the child-view action with the trigger key as Arg.
			c.action(t, app.ActionChildView, tc.triggerKey)
			vs := c.state(t)

			// The controller must have pushed a child-list screen.
			if vs.Body.Kind != app.BodyKindList {
				t.Fatalf("[%s key=%s] after ActionChildView: Body.Kind=%q, want %q — "+
					"controller must push ScreenChildList; if this returns the same screen "+
					"the trigger key dispatch is broken",
					tc.parentType, tc.triggerKey, vs.Body.Kind, app.BodyKindList)
			}
			if vs.Body.List == nil {
				t.Fatalf("[%s key=%s] Body.List is nil after ActionChildView", tc.parentType, tc.triggerKey)
			}
			// Columns must be non-empty — the child type must be registered with columns.
			if len(vs.Body.List.Columns) == 0 {
				t.Errorf("[%s key=%s] Body.List.Columns is empty — child type %q must have columns",
					tc.parentType, tc.triggerKey, tc.expectedChildType)
			}
		})
	}
}

// TestWebChildView_BackFromChildList_ReturnsToParentList verifies that ActionBack
// from a child list pops back to the parent resource list.
func TestWebChildView_BackFromChildList_ReturnsToParentList(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	navigateToListWithRows(t, c, "ecs-svc")

	c.action(t, app.ActionChildView, "enter") // → ecs_tasks child list
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("expected child list after ActionChildView, got %q", vs.Body.Kind)
	}

	c.action(t, app.ActionBack, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("after back from child list: expected parent list, got %q", vs.Body.Kind)
	}
}

// TestWebChildView_FromDetail_ActionChildView_NavigatesToChildList verifies that
// ActionChildView works when called from the detail screen (the controller's
// selectedResourceForAction resolves from the top detail screen's resource).
func TestWebChildView_FromDetail_ActionChildView_NavigatesToChildList(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// Navigate to detail first.
	navigateToDetail(t, c, "ecs-svc")

	// Trigger child view from detail.
	c.action(t, app.ActionChildView, "enter")
	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("ActionChildView from detail: Body.Kind=%q, want %q — "+
			"controller must resolve resource from detail screen",
			vs.Body.Kind, app.BodyKindList)
	}
}

// =============================================================================
// 14. RELATED PANEL NAVIGATION (PR-E new)
// =============================================================================

// TestWebRelated_ToggleFocus_SetsRelatedFocused verifies that ActionToggleFocus
// on a detail screen with a related panel sets RelatedFocused=true.
func TestWebRelated_ToggleFocus_SetsRelatedFocused(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// dbi has RelatedDefs and demo fixtures, so RelatedVisible will be true.
	db := navigateToDetail(t, c, "dbi")
	if !db.RelatedVisible {
		t.Skip("dbi detail: RelatedVisible=false — cannot test focus toggle without a visible panel")
	}

	c.action(t, app.ActionToggleFocus, "")
	vs := c.state(t)

	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("after toggle-focus: Body.Kind=%q, want %q", vs.Body.Kind, app.BodyKindDetail)
	}
	if vs.Body.Detail == nil {
		t.Fatal("Body.Detail is nil after toggle-focus")
	}
	if !vs.Body.Detail.RelatedFocused {
		t.Errorf("Body.Detail.RelatedFocused=false after ActionToggleFocus — "+
			"toggle-focus must move focus to the related panel")
	}
}

// TestWebRelated_ToggleFocusTwice_RestoresFieldFocus verifies that a second
// ActionToggleFocus returns focus to the field column.
func TestWebRelated_ToggleFocusTwice_RestoresFieldFocus(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	db := navigateToDetail(t, c, "dbi")
	if !db.RelatedVisible {
		t.Skip("dbi detail: RelatedVisible=false — cannot test focus toggle without a visible panel")
	}

	c.action(t, app.ActionToggleFocus, "")
	c.action(t, app.ActionToggleFocus, "")
	vs := c.state(t)

	if vs.Body.Detail == nil {
		t.Fatal("Body.Detail is nil after double toggle-focus")
	}
	if vs.Body.Detail.RelatedFocused {
		t.Errorf("Body.Detail.RelatedFocused=true after two toggles — second toggle must restore field focus")
	}
}

// TestWebRelated_SelectFocusedRow_NavigatesStack verifies that ActionSelect
// while RelatedFocused navigates the stack (pushes a new screen). The exact
// Kind depends on the related def's NavigationKind:
//
//   - NavigationKindResourceList → list
//   - NavigationKindFilteredList → list
//   - NavigationKindDetail       → detail
//
// We assert that the Kind changed away from detail (the stack was mutated),
// which covers the NavigationKind variants. If the related panel has no loaded
// rows, we skip rather than produce a vacuous assertion.
func TestWebRelated_SelectFocusedRow_NavigatesStack(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	db := navigateToDetail(t, c, "dbi")
	if !db.RelatedVisible {
		t.Skip("dbi detail: RelatedVisible=false — cannot test related-navigate")
	}

	// Focus the related panel.
	c.action(t, app.ActionToggleFocus, "")
	vs := c.state(t)
	if vs.Body.Detail == nil || !vs.Body.Detail.RelatedFocused {
		t.Skip("related panel not focused after toggle-focus — cannot test navigate")
	}

	// Check whether any related row is non-loading (required for navigation).
	hasLoaded := false
	for _, rel := range vs.Body.Detail.Related {
		if !rel.Loading && !rel.Err {
			hasLoaded = true
			break
		}
	}
	if !hasLoaded {
		t.Fatal("related rows still loading after open-detail — the headless related-check (runRelatedCheckers) " +
			"must populate counts via DrainSync so the panel is navigable; a regression there would surface here")
	}

	// Select the focused row (RelatedCursor=0 by default).
	c.action(t, app.ActionSelect, "")
	vs = c.state(t)

	// The stack must have changed — the detail screen is no longer on top.
	// (Either a list or a detail for the related resource was pushed.)
	if vs.Body.Kind == app.BodyKindDetail && vs.Body.Detail != nil && vs.Body.Detail.RelatedFocused {
		// Still on the same detail screen with focus still on related panel
		// means navigation was a no-op — that is the broken state.
		t.Errorf("ActionSelect on related panel did not navigate: still on detail with RelatedFocused=true — "+
			"HandleRelatedNavigate must produce a non-Unknown NavigationKind for dbi's related rows")
	}
	// The resulting Kind should be list or detail (never menu/text/help).
	switch vs.Body.Kind {
	case app.BodyKindList, app.BodyKindDetail:
		// Correct: stack was mutated by applyRelatedNavResult.
	default:
		t.Errorf("after related-panel select: unexpected Body.Kind=%q — expected list or detail", vs.Body.Kind)
	}
}

// =============================================================================
// 15. REVEAL STAYS BLOCKED (PR-E confirm)
// =============================================================================

// TestWebReveal_ActionReveal_BlockedOver HTTP confirms that ActionReveal is
// rejected 403 even from a detail context where reveal would otherwise make
// sense. The server is constructed with allowReveal=false (startServer default).
func TestWebReveal_ActionReveal_BlockedFromDetailContext(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// Navigate to a detail screen so the controller has a resource context.
	navigateToDetail(t, c, "secrets")

	// Post reveal directly, bypassing the client.action helper (which fatals on
	// non-2xx) so we can observe the 403.
	form := url.Values{}
	form.Set("kind", string(app.ActionReveal))
	req, err := http.NewRequestWithContext(bgctx, http.MethodPost, c.baseURL+"/action", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("NewRequestWithContext: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-A9S-Token", c.token)

	resp, doErr := c.http.Do(req)
	if doErr != nil {
		t.Fatalf("POST /action reveal: %v", doErr)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("ActionReveal from detail context: expected 403, got %d — "+
			"reveal must remain blocked when allowReveal=false regardless of navigation state", resp.StatusCode)
	}
}

// =============================================================================
// 16. BACK UNWINDS DEEP STACK (PR-E new)
// =============================================================================

// TestWebBack_DeepStack_UnwindsCorrectly exercises the full menu→list→detail→
// yaml→back→back→back→back unwind sequence and verifies each transition is
// correct. This catches stack-corruption bugs introduced by the new Apply lanes.
func TestWebBack_DeepStack_UnwindsCorrectly(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// menu (start)
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindMenu {
		t.Fatalf("initial state: expected menu, got %q", vs.Body.Kind)
	}

	// → list
	c.action(t, app.ActionCommand, "ec2")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("after command ec2: expected list, got %q", vs.Body.Kind)
	}
	if len(vs.Body.List.Rows) == 0 {
		t.Fatal("ec2 list has no rows — cannot navigate to detail")
	}

	// → detail
	c.action(t, app.ActionOpenDetail, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("after open-detail: expected detail, got %q", vs.Body.Kind)
	}

	// → yaml
	c.action(t, app.ActionOpenYAML, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindText {
		t.Fatalf("after open-yaml: expected text, got %q", vs.Body.Kind)
	}
	if vs.Body.Text == nil || len(vs.Body.Text.Lines) == 0 {
		t.Errorf("YAML text body is empty — regression in headless YAML builder")
	}

	// back → detail
	c.action(t, app.ActionBack, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("back from yaml: expected detail, got %q", vs.Body.Kind)
	}

	// back → list
	c.action(t, app.ActionBack, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("back from detail: expected list, got %q", vs.Body.Kind)
	}

	// back → menu
	c.action(t, app.ActionBack, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindMenu {
		t.Fatalf("back from list: expected menu, got %q", vs.Body.Kind)
	}
}

// TestWebBack_ChildViewStack_UnwindsCorrectly verifies that the child-view stack
// also unwinds correctly: menu→list→child-list→back→list→back→menu.
func TestWebBack_ChildViewStack_UnwindsCorrectly(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// → list (ecs-svc, which has children)
	navigateToListWithRows(t, c, "ecs-svc")

	// → child list
	c.action(t, app.ActionChildView, "enter")
	vs := c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("after ActionChildView: expected child list, got %q", vs.Body.Kind)
	}

	// back → parent list
	c.action(t, app.ActionBack, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindList {
		t.Fatalf("back from child list: expected parent list, got %q", vs.Body.Kind)
	}

	// back → menu
	c.action(t, app.ActionBack, "")
	vs = c.state(t)
	if vs.Body.Kind != app.BodyKindMenu {
		t.Fatalf("back from parent list: expected menu, got %q", vs.Body.Kind)
	}
}

// =============================================================================
// REGRESSION: Fix 2 — GET /body is token-gated, read-only, and non-mutating
// =============================================================================

// bodyRaw performs GET /body with the given token header value and returns
// the HTTP status code and response body string.
func (c *client) bodyRaw(t *testing.T, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(bgctx, http.MethodGet, c.baseURL+"/body", nil)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext GET /body: %v", err)
	}
	if token != "" {
		req.Header.Set("X-A9S-Token", token)
	}
	resp, doErr := c.http.Do(req)
	if doErr != nil {
		t.Fatalf("GET /body: %v", doErr)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// TestWebBody_WithValidToken_Returns200WithHTML verifies that GET /body with a
// valid X-A9S-Token header returns 200 and non-empty HTML after navigating to a
// resource list.
//
// Pre-fix failure: /body did not exist; the SSE update handler called GET /action
// with an empty form body to refresh the fragment, which applied a no-op action
// and triggered notifySubscribers, creating an infinite SSE→GET /action→SSE loop.
func TestWebBody_WithValidToken_Returns200WithHTML(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// Navigate to a resource list so the body fragment contains list markup.
	c.action(t, app.ActionCommand, "ec2")

	status, html := c.bodyRaw(t, c.token)
	if status != http.StatusOK {
		t.Fatalf("GET /body with valid token: status=%d, want 200", status)
	}
	if html == "" {
		t.Fatal("GET /body with valid token: response body is empty, want HTML fragment")
	}
	// The rendered list fragment must contain a table or list element. The exact
	// markup is template-owned but a non-empty response that includes the word
	// "list" (from data-kind="list" or class attributes) is the minimum signal.
	if !strings.Contains(html, "list") {
		t.Errorf("GET /body: response does not contain 'list' — expected rendered list fragment, got: %q", html[:min(len(html), 200)])
	}
}

// TestWebBody_WithNoToken_Returns403 verifies that GET /body without a token is
// rejected with 403 — /body is an authenticated endpoint.
//
// Pre-fix failure: /body did not exist. Without the endpoint the implicit
// fallback was a 404, which the SSE client silently ignored rather than failing.
func TestWebBody_WithNoToken_Returns403(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	status, _ := c.bodyRaw(t, "") // no token
	if status != http.StatusForbidden {
		t.Errorf("GET /body with no token: status=%d, want 403", status)
	}
}

// TestWebBody_IsReadOnly verifies that calling GET /body does not mutate state:
// the Body.Kind observed after two successive /body calls is the same, and a
// subsequent GET /state shows the same Body.Kind (proving /body has no Apply).
//
// Pre-fix failure: /body did not exist. Without the endpoint the SSE refresh
// path routed to POST /action with an empty body, which mutated LastApplied and
// triggered spurious notifySubscribers calls.
func TestWebBody_IsReadOnly(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// Navigate to a list so we have a non-menu state to inspect.
	c.action(t, app.ActionCommand, "ec2")

	// Capture kind before /body calls.
	vsBefore := c.state(t)
	kindBefore := vsBefore.Body.Kind

	// Call /body twice.
	status1, _ := c.bodyRaw(t, c.token)
	if status1 != http.StatusOK {
		t.Fatalf("first GET /body: status=%d", status1)
	}
	status2, _ := c.bodyRaw(t, c.token)
	if status2 != http.StatusOK {
		t.Fatalf("second GET /body: status=%d", status2)
	}

	// State must be unchanged.
	vsAfter := c.state(t)
	if vsAfter.Body.Kind != kindBefore {
		t.Errorf("GET /body mutated state: Body.Kind changed from %q to %q — /body must be read-only", kindBefore, vsAfter.Body.Kind)
	}
}

// min is a local helper so the test file does not need to import math.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

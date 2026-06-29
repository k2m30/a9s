// Package webintegration drives the real web server (internal/web) over HTTP
// in demo mode and asserts on GET /state JSON. All tests are deterministic —
// demo fetchers are synchronous, DrainSync runs inline, no sleeps needed.
//
// Coverage at PR-E:
//   - Security contract: 403 without token, Cache-Control, no CORS, reveal blocked
//   - Startup: fresh session shows main menu
//   - All resource types: ActionCommand → list, columns present, rich types have rows
//   - Deeper list flows: sort, filter, cursor navigation (wired in PR-C list state)
//   - Back navigation: full stack unwind
//   - Isolated sessions: two clients share server but have independent state
//   - Child views: wired via ActionCommand (PR-C will add open-detail path)
//   - Harness-has-teeth: asserts a property the fix introduced; documents regression
//
// Actions that are PR-C-blocked stubs (open-detail, open-yaml, child-view,
// load-more) are not tested here — they return the current snapshot unchanged
// and would produce vacuous assertions. PR-E tests cover the PR-D deliverables.
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
		"",             // no --command pre-navigation
		"127.0.0.1:0", // OS-assigned port
		token,
		true,  // demoMode
		true,  // noCache
		false, // allowReveal — keep reveal blocked
		viewCfg,
	)

	ctx, cancel := context.WithCancel(context.Background())
	readyCh := make(chan struct{})

	go func() {
		if listenErr := srv.ListenAndServe(ctx, readyCh); listenErr != nil && ctx.Err() == nil {
			t.Logf("ListenAndServe: %v", listenErr)
		}
	}()

	// Wait for the server to bind; readyCh is closed when the port is known.
	<-readyCh

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
	"iam-role":   true,
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
	"rds":        true,
	"backup":     true,
	"cwlogs":     true,
	"cloudwatch": true,
	"cloudfront": true,
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

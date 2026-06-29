//go:build integration

package webintegration

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
)

// TestWebDNSRebind_RejectsNonLoopbackHost guards the dnsRebindGuard middleware
// introduced in internal/web/server.go. The guard must reject any request whose
// Host header is not a loopback address (127.x.x.x / ::1 / localhost), and any
// request that carries a non-loopback Origin header. Without this guard a hostile
// page whose DNS resolves to 127.0.0.1 could drive the local server.
//
// All four assertions must FAIL if dnsRebindGuard is removed — that is the
// definition of "harness has teeth" for this security control.
func TestWebDNSRebind_RejectsNonLoopbackHost(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// -------------------------------------------------------------------------
	// 1. GET / with a spoofed non-loopback Host → 403
	//
	// Setting req.Host makes Go send that value as the HTTP Host header.  The
	// TCP connection still targets 127.0.0.1 — exactly the DNS-rebinding
	// scenario: the hostile page's hostname resolved to loopback but the Host
	// header carries the attacker's domain.  The token is included so the only
	// possible reason for 403 is the Host guard, not missing auth.
	// -------------------------------------------------------------------------
	t.Run("NonLoopbackHost_IndexPage_Returns403", func(t *testing.T) {
		req, err := http.NewRequestWithContext(bgctx, http.MethodGet,
			c.baseURL+"/?token="+c.token, nil)
		if err != nil {
			t.Fatalf("NewRequestWithContext: %v", err)
		}
		req.Host = "evil.example.com" // spoof Host header — DNS-rebinding vector

		resp, doErr := c.http.Do(req)
		if doErr != nil {
			t.Fatalf("GET / with evil host: %v", doErr)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("DNS-rebinding protection FAILED: GET / with Host=evil.example.com returned %d, want 403 — "+
				"dnsRebindGuard must reject non-loopback Host headers", resp.StatusCode)
		}
	})

	// -------------------------------------------------------------------------
	// 2. Control: GET / with the default loopback Host → 200
	//
	// Leaving req.Host empty lets Go set it to the TCP target (127.0.0.1:port),
	// which is a loopback address. The guard must pass this through.
	// -------------------------------------------------------------------------
	t.Run("LoopbackHost_IndexPage_Returns200", func(t *testing.T) {
		req, err := http.NewRequestWithContext(bgctx, http.MethodGet,
			c.baseURL+"/?token="+c.token, nil)
		if err != nil {
			t.Fatalf("NewRequestWithContext: %v", err)
		}
		// req.Host left at zero value — Go uses 127.0.0.1:port from the URL.

		resp, doErr := c.http.Do(req)
		if doErr != nil {
			t.Fatalf("GET / with loopback host: %v", doErr)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("DNS-rebinding protection misconfigured: GET / with loopback Host returned %d, want 200 — "+
				"dnsRebindGuard must allow loopback Host headers; body=%q",
				resp.StatusCode, body)
		}
	})

	// -------------------------------------------------------------------------
	// 3. POST /action with a valid token but Origin: http://evil.example.com → 403
	//
	// The Origin header is set by browsers on cross-origin requests. A page at
	// http://evil.example.com whose DNS resolves to 127.0.0.1 would send this
	// header automatically. The guard must reject it regardless of valid auth.
	// -------------------------------------------------------------------------
	t.Run("NonLoopbackOrigin_Action_Returns403", func(t *testing.T) {
		form := url.Values{}
		form.Set("kind", string(app.ActionMoveDown))
		req, err := http.NewRequestWithContext(bgctx, http.MethodPost,
			c.baseURL+"/action", strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatalf("NewRequestWithContext POST /action: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-A9S-Token", c.token)
		req.Header.Set("Origin", "http://evil.example.com") // non-loopback Origin

		resp, doErr := c.http.Do(req)
		if doErr != nil {
			t.Fatalf("POST /action with evil Origin: %v", doErr)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("DNS-rebinding protection FAILED: POST /action with Origin=http://evil.example.com returned %d, want 403 — "+
				"dnsRebindGuard must reject non-loopback Origin headers even when auth token is valid",
				resp.StatusCode)
		}
	})

	// -------------------------------------------------------------------------
	// 4. (Optional) GET /static/app.js with spoofed Host → 403
	//
	// Static assets are wrapped in dnsRebindGuard so an attacker cannot use them
	// as an oracle to probe whether the server is running. If this returns 200 the
	// guard has a gap — the guard must cover the /static/ route.
	// -------------------------------------------------------------------------
	t.Run("NonLoopbackHost_StaticAsset_Returns403", func(t *testing.T) {
		req, err := http.NewRequestWithContext(bgctx, http.MethodGet,
			c.baseURL+"/static/app.js", nil)
		if err != nil {
			t.Fatalf("NewRequestWithContext GET /static/app.js: %v", err)
		}
		req.Host = "evil.example.com" // spoof Host header

		resp, doErr := c.http.Do(req)
		if doErr != nil {
			t.Fatalf("GET /static/app.js with evil host: %v", doErr)
		}
		_ = resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("DNS-rebinding protection FAILED: GET /static/app.js with Host=evil.example.com returned %d, want 403 — "+
				"static assets must be guarded by dnsRebindGuard so attackers cannot use them as an oracle",
				resp.StatusCode)
		}
	})
}

//go:build integration

package webintegration

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
)

// dnsRebindGuard in core/web/server.go rejects any request whose Host header is
// not a loopback address (127.x.x.x / ::1 / localhost), and any request that
// carries a non-loopback Origin header. Without it a hostile page whose DNS
// resolves to 127.0.0.1 could drive the local server.
func TestWebDNSRebind_RejectsNonLoopbackHost(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	// Setting req.Host makes Go send that value as the HTTP Host header.  The
	// TCP connection still targets 127.0.0.1 — exactly the DNS-rebinding
	// scenario: the hostile page's hostname resolved to loopback but the Host
	// header carries the attacker's domain.  The token is included so the only
	// possible reason for 403 is the Host guard, not missing auth.
	t.Run("NonLoopbackHost_IndexPage_Returns403", func(t *testing.T) {
		req, err := http.NewRequestWithContext(bgctx, http.MethodGet,
			c.baseURL+"/?token="+c.token, nil)
		if err != nil {
			t.Fatalf("NewRequestWithContext: %v", err)
		}
		req.Host = "evil.example.com"

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

	// Leaving req.Host empty lets Go send the TCP target (127.0.0.1:port), a
	// loopback Host.
	t.Run("LoopbackHost_IndexPage_Returns200", func(t *testing.T) {
		req, err := http.NewRequestWithContext(bgctx, http.MethodGet,
			c.baseURL+"/?token="+c.token, nil)
		if err != nil {
			t.Fatalf("NewRequestWithContext: %v", err)
		}

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

	// The Origin header is set by browsers on cross-origin requests. A page at
	// http://evil.example.com whose DNS resolves to 127.0.0.1 would send this
	// header automatically. The guard must reject it regardless of valid auth.
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
		req.Header.Set("Origin", "http://evil.example.com")

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

	// Static assets are wrapped in dnsRebindGuard so an attacker cannot use them
	// as an oracle to probe whether the server is running.
	t.Run("NonLoopbackHost_StaticAsset_Returns403", func(t *testing.T) {
		req, err := http.NewRequestWithContext(bgctx, http.MethodGet,
			c.baseURL+"/static/app.js", nil)
		if err != nil {
			t.Fatalf("NewRequestWithContext GET /static/app.js: %v", err)
		}
		req.Host = "evil.example.com"

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

//go:build integration

package webintegration

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestWebStatic_AppJS_Served asserts GET /static/app.js serves the real app.js
// content. The embed roots files at "static/app.js", so the FS is rooted at
// "static/" via fs.Sub; StripPrefix("/static/") on an unrooted FS turns the
// request into "app.js", the page loads no JavaScript and every key is dead.
// An API test never executes the page's <script>, so only a static-asset
// check sees it.
func TestWebStatic_AppJS_Served(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	req, _ := http.NewRequestWithContext(bgctx, http.MethodGet, c.baseURL+"/static/app.js", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("GET /static/app.js: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /static/app.js: status %d, want 200 — a 404 means the browser loads no JS and every key is dead", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "sendAction") {
		t.Errorf("GET /static/app.js did not return app.js (no 'sendAction' in body)")
	}
}

// TestWebIndex_WiresAppJS guards the page wiring app.js depends on: the script
// tag, the data-token on <body>, and the #body / #loading-indicator elements.
// If any is missing, keystrokes either 403 (empty token) or throw before fetch.
func TestWebIndex_WiresAppJS(t *testing.T) {
	c, cleanup := startServer(t)
	defer cleanup()

	req, _ := http.NewRequestWithContext(bgctx, http.MethodGet, c.baseURL+"/?token="+c.token, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	for _, want := range []string{`src="/static/app.js"`, `data-token="`, `id="body"`, `id="loading-indicator"`} {
		if !strings.Contains(html, want) {
			t.Errorf("GET / page is missing %q — app.js cannot wire keys without it", want)
		}
	}
}

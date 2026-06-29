//go:build integration

package webintegration

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestWebStatic_AppJS_Served guards the embed-FS rooting regression. The embed
// roots files at "static/app.js"; serving http.FS(staticFS) with
// StripPrefix("/static/") turned the request into "app.js" — not in the FS — so
// GET /static/app.js 404'd, the browser loaded NO JavaScript, and every key was
// dead. The fix roots the FS at "static/" via fs.Sub. This asserts the asset is
// actually served with the real app.js content.
//
// (curl-only API tests never execute the page's <script>, so they could not
// catch this — hence this explicit static-asset check.)
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

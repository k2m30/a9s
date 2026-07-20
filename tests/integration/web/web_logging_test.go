//go:build integration

// web_logging_test.go — pins CONTRACT 3(a) of the observability slice: a
// request-logging middleware wraps core/web's mux, active only when the
// logging facility (core/logging, new package) is enabled, logging a JSON
// line (msg "web request", attrs method/path/status/duration_ms) per
// handled request through it.
//
// CONTRACT 3(b) — the discarded render error at core/web/handlers.go:156
// (`if err := renderPage(w, vs, s.token); err != nil { http.Error(w,
// "render error", ...) }`) — is intentionally NOT pinned here.
//
// Feasibility note (per this contract's own skip clause): forcing
// renderPage's html/template.ExecuteTemplate call to return an error needs
// either (a) a writer that fails on Write() — handleIndex takes the real
// http.ResponseWriter directly; no injectable writer seam exists at that
// call site for an HTTP-driven integration test to reach — or (b) a
// Kind/nil-mismatched app.ViewState (e.g. Body.Kind claiming "list" while
// Body.List is nil), which the real demo Controller's Snapshot() does not
// produce through any public HTTP action sequence this harness can drive
// (it is an internal invariant the Controller maintains, not something the
// public /action API can violate). Both routes require a change to
// production code (an injectable writer, or a test-only Controller seam)
// that is out of this file's scope — flagged back rather than worked
// around. 3(b) is left to the coder's line-level care, not test-pinned.
package webintegration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/logging"
)

// resetLoggingToDisabled is registered first (t.Cleanup runs LIFO, so it
// fires LAST) in every test in this file that enables the facility —
// core/logging is a process-wide singleton, and this integration binary's
// package has no other file touching it, but leaving it enabled with a
// t.TempDir() path that gets deleted at test end is still untidy hygiene
// this defends against.
func resetLoggingToDisabled(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = logging.Setup("")
	})
}

// -----------------------------------------------------------------------
// (a) Enabled: every handled request logs a "web request" line with
// method/path/status/duration_ms.
// -----------------------------------------------------------------------

func TestWebLogging_RequestMiddleware_EnabledLogsRequestLine(t *testing.T) {
	resetLoggingToDisabled(t)

	logPath := filepath.Join(t.TempDir(), "a9s-web.log")
	closeFn, err := logging.Setup(logPath)
	if err != nil {
		t.Fatalf("logging.Setup(%q): %v", logPath, err)
	}
	defer func() { _ = closeFn() }()

	c, cleanup := startServer(t)
	defer cleanup()

	_ = c.state(t) // GET /state?token=... — a normal, successful request.

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", logPath, err)
	}
	content := string(data)
	if !strings.Contains(content, "web request") {
		t.Errorf("log file missing a \"web request\" line, got:\n%s", content)
	}
	if !strings.Contains(content, "GET") || !strings.Contains(content, "/state") {
		t.Errorf("log file's request line missing method/path (GET /state), got:\n%s", content)
	}
	if !strings.Contains(content, "duration_ms") {
		t.Errorf("log file's request line missing a duration_ms attribute, got:\n%s", content)
	}
	if !strings.Contains(content, "200") {
		t.Errorf("log file's request line missing the 200 status, got:\n%s", content)
	}
}

// -----------------------------------------------------------------------
// (b) Disabled: a request adds nothing to the log file.
// -----------------------------------------------------------------------

func TestWebLogging_RequestMiddleware_DisabledLogsNothing(t *testing.T) {
	resetLoggingToDisabled(t)

	logPath := filepath.Join(t.TempDir(), "a9s-web.log")
	closeFn, err := logging.Setup(logPath)
	if err != nil {
		t.Fatalf("logging.Setup(%q): %v", logPath, err)
	}

	c, cleanup := startServer(t)
	defer cleanup()

	_ = c.state(t)
	_ = closeFn()

	before, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) after the enabled request: %v", logPath, err)
	}

	disableFn, err := logging.Setup("")
	if err != nil {
		t.Fatalf("logging.Setup(\"\"): %v", err)
	}
	defer func() { _ = disableFn() }()

	_ = c.state(t)

	after, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) after the disabled request: %v", logPath, err)
	}
	if string(after) != string(before) {
		t.Errorf("a request made while the logging facility is disabled must add nothing to the log file; before=%dB after=%dB\nbefore:\n%s\nafter:\n%s", len(before), len(after), before, after)
	}
}

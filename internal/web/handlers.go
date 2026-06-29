package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/k2m30/a9s/v3/internal/app"
)

const (
	// tokenHeader is the custom request header carrying the per-run auth token.
	// Using a custom header (not a form field) prevents cross-site form submissions
	// from forging actions — browsers enforce same-origin policy on custom headers.
	tokenHeader = "X-A9S-Token" //nolint:gosec // not a credential — it's a header name used as a CSRF check

	// sessionCookieName is the name of the session-selector cookie.
	sessionCookieName = "a9s_session"
)

// GenerateToken produces a cryptographically-random 32-byte hex token.
// Exported so cmd/a9s/main.go can generate the token before constructing Server.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateSessionID produces a cryptographically-random 16-byte hex session ID.
func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// noStore writes the Cache-Control: no-store header (mandatory on all responses).
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

// tokenOK validates the per-run token using constant-time comparison.
// It checks the X-A9S-Token header first, then falls back to the "token"
// query parameter (used by SSE EventSource which cannot set custom headers).
func (s *Server) tokenOK(r *http.Request) bool {
	candidate := r.Header.Get(tokenHeader)
	if candidate == "" {
		candidate = r.URL.Query().Get("token")
	}
	// constant-time compare to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(s.token)) == 1
}

// requireSession resolves (or creates) the session for the request and returns
// the sessionEntry. It sets the session cookie on w if it was newly created.
func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) *sessionEntry {
	var sessionID string
	if c, err := r.Cookie(sessionCookieName); err == nil {
		sessionID = c.Value
	}
	if sessionID == "" {
		sessionID = generateSessionID()
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
	}
	return s.getOrCreateSession(sessionID)
}

// handleIndex renders the full HTML page from the current ViewState snapshot.
// GET /
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	noStore(w)
	entry := s.requireSession(w, r)

	entry.mu.Lock()
	vs := entry.ctrl.Snapshot()
	entry.mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderPage(w, vs, s.token); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// handleAction decodes a semantic Action from the request body (JSON or form),
// applies it to the session controller, and returns the updated body fragment.
// POST /action
func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	noStore(w)

	// CSRF: require token in the custom header (not a form field).
	if !s.tokenOK(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	entry := s.requireSession(w, r)

	var action app.Action
	ct := r.Header.Get("Content-Type")
	if ct == "application/json" || (len(ct) > 16 && ct[:16] == "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&action); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	} else {
		// Form-encoded fallback: kind=<kind>&arg=<arg>
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB limit against memory exhaustion
		_ = r.ParseForm()
		action.Kind = app.ActionKind(r.FormValue("kind"))
		action.Arg = r.FormValue("arg")
	}

	// Block ActionReveal unless --web-allow-reveal is set.
	if action.Kind == app.ActionReveal && !s.allowReveal {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	entry.mu.Lock()
	_, tasks := entry.ctrl.Apply(action)
	app.DrainSync(entry.ctrl, tasks)
	vs := entry.ctrl.Snapshot()
	entry.mu.Unlock()

	// Notify SSE subscribers that state changed.
	s.notifySubscribers(entry)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderMainFragment(w, vs); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// handleBody renders only the body fragment for the current session state.
// GET /body — token-gated, no Apply, no notifySubscribers.
// Used by the SSE "update" handler in app.js to refresh the body without
// triggering another SSE event (which would create an infinite loop).
func (s *Server) handleBody(w http.ResponseWriter, r *http.Request) {
	noStore(w)

	if !s.tokenOK(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	entry := s.requireSession(w, r)

	entry.mu.Lock()
	vs := entry.ctrl.Snapshot()
	entry.mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderMainFragment(w, vs); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// handleState returns the current ViewState as JSON.
// GET /state
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	noStore(w)

	if !s.tokenOK(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	entry := s.requireSession(w, r)

	entry.mu.Lock()
	vs := entry.ctrl.Snapshot()
	entry.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(vs); err != nil {
		// Headers already sent; nothing to do.
		_ = err
	}
}

// handleEvents serves a Server-Sent Events stream. It sends a "ping" event
// on connection, a "update" event whenever the session state changes, and
// a heartbeat every 15 seconds to keep the connection alive through proxies.
// GET /events
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	noStore(w)

	if !s.tokenOK(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	entry := s.requireSession(w, r)

	// SSE requires a Flusher.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.subscribe(entry)
	defer s.unsubscribe(entry, ch)

	// Initial ping to confirm the connection is established.
	_, _ = fmt.Fprintf(w, "event: ping\ndata: connected\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			_, _ = fmt.Fprintf(w, "event: update\ndata: state-changed\n\n")
			flusher.Flush()
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// subscriber management — each session keeps a set of SSE listener channels.

type subscriberSet struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

// We store a subscriber set per sessionEntry using a sync.Map on the server.
// Use a separate map here keyed by entry pointer.
var globalSubscribers sync.Map // key: *sessionEntry → *subscriberSet

func (s *Server) subscribe(entry *sessionEntry) chan struct{} {
	ch := make(chan struct{}, 1)
	v, _ := globalSubscribers.LoadOrStore(entry, &subscriberSet{
		subs: make(map[chan struct{}]struct{}),
	})
	ss := v.(*subscriberSet)
	ss.mu.Lock()
	ss.subs[ch] = struct{}{}
	ss.mu.Unlock()
	return ch
}

func (s *Server) unsubscribe(entry *sessionEntry, ch chan struct{}) {
	v, ok := globalSubscribers.Load(entry)
	if !ok {
		return
	}
	ss := v.(*subscriberSet)
	ss.mu.Lock()
	delete(ss.subs, ch)
	ss.mu.Unlock()
}

func (s *Server) notifySubscribers(entry *sessionEntry) {
	v, ok := globalSubscribers.Load(entry)
	if !ok {
		return
	}
	ss := v.(*subscriberSet)
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for ch := range ss.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package logging is an opt-in, process-wide JSON debug-log facility.
// Nothing in a9s writes to stderr while the TUI owns the terminal or while
// the web server is serving requests, so every call site logs unconditionally
// through L() — when the facility is disabled (the default), L() returns a
// discard-backed *slog.Logger and every call is a cheap no-op.
//
// Setup is meant to be called exactly ONCE, at process startup
// (cmd/a9s/main.go, before the TUI takes the terminal or the web server
// starts accepting connections) — never from a goroutine that might race a
// concurrent L() call from elsewhere in the app. Tests are the one place
// Setup is called repeatedly per process; they do so sequentially and
// restore Setup("") via t.Cleanup, per the last-call-wins contract below.
//
// Renderer-agnostic: stdlib only (log/slog, os).
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
)

var (
	mu      sync.RWMutex
	logger  = slog.New(slog.DiscardHandler)
	enabled bool
)

// Setup installs the process-wide log destination. path == "" disables the
// facility (Enabled() reports false; L() returns a discard-backed logger).
// A non-empty path opens (creating if necessary, mode 0600) that file for
// append and installs a JSON slog handler writing to it.
//
// On success, close releases whatever this call opened (a no-op func for
// path == "") — callers should defer it. On failure (path is set but cannot
// be opened), Setup returns a non-nil error and leaves the facility
// disabled, regardless of what a prior call had installed.
//
// The last call wins: a later Setup call replaces whatever an earlier one
// installed. Not safe to call concurrently with itself, only with L()/Enabled().
func Setup(path string) (close func() error, err error) {
	if path == "" {
		mu.Lock()
		logger = slog.New(slog.DiscardHandler)
		enabled = false
		mu.Unlock()
		return func() error { return nil }, nil
	}

	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if openErr != nil {
		mu.Lock()
		logger = slog.New(slog.DiscardHandler)
		enabled = false
		mu.Unlock()
		return nil, fmt.Errorf("logging: opening %q: %w", path, openErr)
	}

	mu.Lock()
	logger = slog.New(slog.NewJSONHandler(f, nil))
	enabled = true
	mu.Unlock()

	return f.Close, nil
}

// Enabled reports whether Setup last installed a real (non-discard) log
// destination.
func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabled
}

// L returns the current process-wide logger. Never nil — discard-backed
// when the facility is disabled, so every call site may log unconditionally.
func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}

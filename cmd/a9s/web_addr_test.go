package main

import (
	"testing"
)

// TestRequireLoopbackAddr_TableDriven is the security regression test for the
// requireLoopbackAddr bind-address guard (commit 56910d32 fix #1).
//
// Pre-fix failure: requireLoopbackAddr did not exist; the web server bound to
// whatever address the caller supplied, including 0.0.0.0 and routable IPs,
// exposing the unauthenticated SSE stream and other endpoints to any host on
// the network before the session token was even exchanged.
//
// The test must return an error for all non-loopback addresses and nil for all
// loopback addresses.
func TestRequireLoopbackAddr_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		// ── must reject (non-loopback / all-interfaces) ──────────────────────
		{
			name:    "port-only binds all interfaces",
			addr:    ":7682",
			wantErr: true,
		},
		{
			name:    "0.0.0.0 binds all IPv4 interfaces",
			addr:    "0.0.0.0:7682",
			wantErr: true,
		},
		{
			name:    ":: binds all IPv6 interfaces",
			addr:    "[::]:7682",
			wantErr: true,
		},
		{
			name:    "routable private IPv4 192.168.1.5",
			addr:    "192.168.1.5:7682",
			wantErr: true,
		},
		{
			name:    "malformed addr (no port)",
			addr:    "localhost",
			wantErr: true,
		},
		// ── must accept (loopback) ────────────────────────────────────────────
		{
			name:    "127.0.0.1 port 0 (OS-assigned)",
			addr:    "127.0.0.1:0",
			wantErr: false,
		},
		{
			name:    "localhost explicit port",
			addr:    "localhost:7682",
			wantErr: false,
		},
		{
			name:    "IPv6 loopback ::1",
			addr:    "[::1]:7682",
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := requireLoopbackAddr(tc.addr)
			if tc.wantErr && err == nil {
				t.Errorf("requireLoopbackAddr(%q) = nil, want non-nil error — non-loopback address must be rejected", tc.addr)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("requireLoopbackAddr(%q) = %v, want nil — loopback address must be accepted", tc.addr, err)
			}
		})
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/web"
)

// A JSON action body is capped like a form body: a token holder cannot make
// the server allocate an arbitrary request.
func TestWebServer_JSONActionBodyIsCapped(t *testing.T) {
	token, err := web.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	srv := web.NewServer(demo.DemoProfile, demo.DemoRegion, "", "127.0.0.1:0", token, true, true, false, config.SharedDefaultConfig(), "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	go func() { _ = srv.ListenAndServe(ctx, ready) }()
	<-ready

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}
	post := func(body []byte) int {
		t.Helper()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+srv.Addr()+"/action", bytes.NewReader(body))
		req.Header.Set("X-A9S-Token", token)
		req.Header.Set("Content-Type", "application/json")
		resp, doErr := client.Do(req)
		if doErr != nil {
			t.Fatalf("POST /action: %v", doErr)
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+srv.Addr()+"/?token="+token, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	oversized := []byte(`{"kind":"command","arg":"` + strings.Repeat("a", 2<<20) + `"}`)
	if got := post(oversized); got != http.StatusBadRequest {
		t.Errorf("a 2 MiB JSON action answered %d, want %d", got, http.StatusBadRequest)
	}
}

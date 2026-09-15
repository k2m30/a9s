// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/cookiejar"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/web"
)

// Shutting the web server down cancels the background work its sessions
// started: a call in flight is told to stop, not left to run past the cache
// writer.
func TestWebServer_ShutdownCancelsBackgroundWork(t *testing.T) {
	token, err := web.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	srv := web.NewServer(demo.DemoProfile, demo.DemoRegion, "", "127.0.0.1:0", token, true, true, false, config.SharedDefaultConfig(), "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	served := make(chan error, 1)
	go func() { served <- srv.ListenAndServe(ctx, ready) }()
	<-ready

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}
	do := func(method, path string, body []byte) {
		t.Helper()
		req, _ := http.NewRequestWithContext(context.Background(), method, "http://"+srv.Addr()+path, bytes.NewReader(body))
		req.Header.Set("X-A9S-Token", token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		_ = resp.Body.Close()
	}
	do(http.MethodGet, "/?token="+token, nil)

	started := make(chan struct{}, 1)
	stopped := make(chan error, 1)
	resource.SetRelatedForTest("ec2", []resource.RelatedDef{{
		TargetType:  "sg",
		DisplayName: "security groups",
		Checker: func(ctx context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
			started <- struct{}{}
			<-ctx.Done()
			stopped <- ctx.Err()
			return resource.RelatedCheckResult{}
		},
	}})
	t.Cleanup(func() { resource.CleanupRelatedForTest("ec2") })
	do(http.MethodPost, "/action", []byte(`{"kind":"command","arg":"ec2"}`))
	do(http.MethodPost, "/action", []byte(`{"kind":"select"}`))
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("opening the detail never reached the related check in the background")
	}

	cancel()
	select {
	case err := <-stopped:
		if err == nil {
			t.Error("the enricher's context ended without an error")
		}
	case <-time.After(8 * time.Second):
		t.Error("shutdown left the background enrichment running")
	}
	select {
	case <-served:
	case <-time.After(8 * time.Second):
		t.Error("ListenAndServe did not return after shutdown")
	}
}

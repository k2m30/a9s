package main

// AWS serves every CLOUDFRONT-scope wafv2 operation from the us-east-1
// endpoint alone. A collector run from any other region that reads only the
// REGIONAL scope writes an empty array for an account that has edge ACLs, and
// the captured picture is narrower than the app's.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// wafScopeTransport answers ListWebACLs and GetWebACL, recording the region
// each request was signed for alongside the scope it asked about.
type wafScopeTransport struct {
	mu    sync.Mutex
	asked map[string]string // scope → region
}

func (tr *wafScopeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body) //nolint:errcheck // an unreadable body decodes as an empty request
	var in struct {
		Scope string
		Name  string
		Id    string
	}
	_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body carries no scope

	region := ""
	if _, cred, ok := strings.Cut(req.Header.Get("Authorization"), "Credential="); ok {
		if parts := strings.Split(cred, "/"); len(parts) > 3 {
			region = parts[2]
		}
	}

	var out map[string]any
	if strings.HasSuffix(req.Header.Get("X-Amz-Target"), "ListWebACLs") {
		tr.mu.Lock()
		tr.asked[in.Scope] = region
		tr.mu.Unlock()
		out = map[string]any{"WebACLs": []map[string]any{{
			"Name": strings.ToLower(in.Scope) + "-acl",
			"Id":   "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
			"ARN":  "arn:aws:wafv2:" + region + ":123456789012:webacl/x/1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
		}}}
	} else {
		out = map[string]any{"WebACL": map[string]any{"Name": in.Name, "Id": in.Id, "Rules": []any{}}}
	}
	b, _ := json.Marshal(out) //nolint:errcheck // test-built maps always marshal
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}, nil
}

func TestCaptureWAF_ReadsTheCloudFrontScopeFromUSEast1InEveryRegion(t *testing.T) {
	tr := &wafScopeTransport{asked: map[string]string{}}
	cfg := aws.Config{
		Region:           "eu-west-1",
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		HTTPClient:       &http.Client{Transport: tr},
		RetryMaxAttempts: 1,
	}

	got, err := captureWAF(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureWAF: %v", err)
	}
	data, ok := got.(wafData)
	if !ok {
		t.Fatalf("captureWAF returned %T, want wafData", got)
	}
	if len(data.WebACLs) != 2 {
		t.Errorf("captured %d web ACLs from an account holding one per scope: %+v", len(data.WebACLs), data.WebACLs)
	}
	if region := tr.asked["CLOUDFRONT"]; region != "us-east-1" {
		t.Errorf("the CLOUDFRONT scope was listed against %q, want us-east-1", region)
	}
	if region := tr.asked["REGIONAL"]; region != "eu-west-1" {
		t.Errorf("the REGIONAL scope was listed against %q, want the profile's own region", region)
	}
}

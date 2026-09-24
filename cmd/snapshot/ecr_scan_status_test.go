package main

// The collector records a repository's totals only when the app counts them:
// the newest image's scan is COMPLETE or ACTIVE (core/aws ecrScanUnread). A
// scan in any other API_ImageScanStatus state, or an image never scanned,
// records its status and no totals.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// ecrStatusScans is each repository's DescribeImageScanFindings answer; a
// repository missing here answers ScanNotFoundException.
var ecrStatusScans = map[string]map[string]any{ //nolint:gochecknoglobals // test-only table
	"acme/complete": {
		"imageScanStatus":   map[string]any{"status": "COMPLETE"},
		"imageScanFindings": map[string]any{"findings": []map[string]any{{"name": "CVE-2025-0001", "severity": "CRITICAL"}}},
	},
	"acme/enhanced": {
		"imageScanStatus":   map[string]any{"status": "ACTIVE"},
		"imageScanFindings": map[string]any{"enhancedFindings": []map[string]any{{"severity": "HIGH"}}},
	},
	"acme/failed": {
		"imageScanStatus": map[string]any{"status": "FAILED", "description": "UnsupportedImageError: The operating system is not supported."},
	},
	"acme/rescanning": {
		"imageScanStatus":   map[string]any{"status": "IN_PROGRESS"},
		"imageScanFindings": map[string]any{"findings": []map[string]any{{"name": "CVE-2025-0002", "severity": "CRITICAL"}}},
	},
}

type ecrStatusTransport struct{}

func (ecrStatusTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body) //nolint:errcheck // an unreadable body decodes as an empty request
	var in struct {
		RepositoryName string `json:"repositoryName"`
	}
	_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body carries no parameters
	target := req.Header.Get("X-Amz-Target")
	switch {
	case strings.HasSuffix(target, ".DescribeRepositories"):
		var repos []map[string]any
		for _, name := range []string{"acme/complete", "acme/enhanced", "acme/failed", "acme/rescanning", "acme/never-scanned"} {
			repos = append(repos, map[string]any{"repositoryName": name, "repositoryArn": "arn:aws:ecr:us-east-1:123456789012:repository/" + name})
		}
		return t561JSON(200, map[string]any{"repositories": repos}), nil
	case strings.HasSuffix(target, ".DescribeImages"):
		return t561JSON(200, map[string]any{"imageDetails": []map[string]any{{
			"repositoryName": in.RepositoryName,
			"imageDigest":    "sha256:" + strings.ReplaceAll(in.RepositoryName, "/", "-"),
			"imagePushedAt":  1.7567208e9,
		}}}), nil
	case strings.HasSuffix(target, ".DescribeImageScanFindings"):
		if out, ok := ecrStatusScans[in.RepositoryName]; ok {
			return t561JSON(200, out), nil
		}
		return t561JSON(400, map[string]any{"__type": "ScanNotFoundException", "message": "Image scan does not exist for the image"}), nil
	}
	return t561JSON(200, map[string]any{}), nil
}

func TestCaptureECR_TotalsOnlyForAScanWithFindings(t *testing.T) {
	cfg := aws.Config{
		Region:           "us-east-1",
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		HTTPClient:       &http.Client{Transport: ecrStatusTransport{}},
		RetryMaxAttempts: 1,
	}
	got, err := captureECR(context.Background(), cfg)
	if err != nil {
		t.Fatalf("captureECR: %v", err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var data struct {
		Repositories []struct {
			RepositoryName string `json:"repository_name"`
			Images         []struct {
				ScanStatus string `json:"scan_status"`
			} `json:"images"`
			CriticalTotal *int `json:"critical_total"`
			HighTotal     *int `json:"high_total"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	want := map[string]struct {
		status   string
		totals   bool
		critical int
		high     int
	}{
		"acme/complete":      {"COMPLETE", true, 1, 0},
		"acme/enhanced":      {"ACTIVE", true, 0, 1},
		"acme/failed":        {"FAILED", false, 0, 0},
		"acme/rescanning":    {"IN_PROGRESS", false, 0, 0},
		"acme/never-scanned": {"", false, 0, 0},
	}
	for _, r := range data.Repositories {
		w := want[r.RepositoryName]
		if len(r.Images) != 1 || r.Images[0].ScanStatus != w.status {
			t.Errorf("%s: images = %+v, want one with scan_status %q", r.RepositoryName, r.Images, w.status)
		}
		switch {
		case !w.totals && (r.CriticalTotal != nil || r.HighTotal != nil):
			t.Errorf("%s: totals %v/%v recorded, want none for a scan with no findings to count", r.RepositoryName, r.CriticalTotal, r.HighTotal)
		case w.totals && (r.CriticalTotal == nil || r.HighTotal == nil || *r.CriticalTotal != w.critical || *r.HighTotal != w.high):
			t.Errorf("%s: totals = %v/%v, want %d/%d", r.RepositoryName, r.CriticalTotal, r.HighTotal, w.critical, w.high)
		}
	}
	if len(data.Repositories) != len(want) {
		t.Errorf("%d repositories recorded, want %d (raw %s)", len(data.Repositories), len(want), raw)
	}
}

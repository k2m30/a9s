package main

// The collector records the scan evidence the app reads: the newest image by
// imagePushedAt over every DescribeImages page (the API documents no order),
// its severity counts tallied from findings[] over every
// DescribeImageScanFindings page (Basic Scanning leaves them off
// DescribeImages) with a failed read's error code kept, and the critical and
// high totals of that image.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

type t561ECRImage struct {
	digest string
	pushed float64
	counts map[string]int
	denied bool
}

// t561ECRRepos: acme/payments-api has three images on its first page and its
// newest, on the second page; acme/batch-worker has one image whose scan read
// is denied.
var t561ECRRepos = map[string][][]t561ECRImage{ //nolint:gochecknoglobals // test-only table
	"acme/payments-api": {
		{
			{digest: "sha256:3f1c9a7e5b2d4c6a8e0f1b3d5c7e9a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1e3f", pushed: 1.7567208e9, counts: map[string]int{"CRITICAL": 2, "HIGH": 5}},
			{digest: "sha256:9b8a7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8b", pushed: 1.7560000e9, counts: map[string]int{"CRITICAL": 1, "MEDIUM": 4}},
			{digest: "sha256:1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b", pushed: 1.7550000e9, counts: map[string]int{"HIGH": 3}},
		},
		{
			{digest: "sha256:7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b9a8f7e6d", pushed: 1.7580000e9, counts: map[string]int{"CRITICAL": 9, "HIGH": 2, "LOW": 1}},
		},
	},
	"acme/batch-worker": {
		{
			{digest: "sha256:5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b", pushed: 1.7567208e9, denied: true},
		},
	},
}

type t561ECRTransport struct {
	mu            sync.Mutex
	imagePages    int
	scanPages     int
	scanRequested map[string]bool
}

// t561ScanFindings is the findings[] one scan returns for counts, split over
// two pages: page 0 holds the first half, page 1 the rest.
func t561ScanFindings(counts map[string]int, page int) []map[string]any {
	var all []map[string]any
	for _, sev := range slices.Sorted(maps.Keys(counts)) {
		for i := range counts[sev] {
			cve := fmt.Sprintf("CVE-2025-%d%04d", len(sev), i+1)
			all = append(all, map[string]any{
				"name":        cve,
				"severity":    sev,
				"uri":         "https://security-tracker.debian.org/tracker/" + cve,
				"description": "Out-of-bounds read in libexample before 1.2.3.",
				"attributes": []map[string]any{
					{"key": "package_name", "value": "libexample"},
					{"key": "package_version", "value": "1.2.2-1"},
				},
			})
		}
	}
	half := (len(all) + 1) / 2
	if page == 0 {
		return all[:half]
	}
	return all[half:]
}

func t561JSON(status int, v any) *http.Response {
	b, _ := json.Marshal(v) //nolint:errcheck // test-built maps always marshal
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Body:       io.NopCloser(strings.NewReader(string(b))),
	}
}

func (tr *t561ECRTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body) //nolint:errcheck // an unreadable body decodes as an empty request
	var in struct {
		RepositoryName string `json:"repositoryName"`
		NextToken      string `json:"nextToken"`
		ImageID        struct {
			ImageDigest string `json:"imageDigest"`
		} `json:"imageId"`
	}
	_ = json.Unmarshal(body, &in) //nolint:errcheck // an empty body carries no parameters

	target := req.Header.Get("X-Amz-Target")
	switch {
	case strings.HasSuffix(target, ".DescribeRepositories"):
		var repos []map[string]any
		for _, name := range []string{"acme/payments-api", "acme/batch-worker"} {
			repos = append(repos, map[string]any{
				"repositoryName":             name,
				"repositoryArn":              "arn:aws:ecr:us-east-1:123456789012:repository/" + name,
				"registryId":                 "123456789012",
				"imageScanningConfiguration": map[string]any{"scanOnPush": true},
			})
		}
		return t561JSON(200, map[string]any{"repositories": repos}), nil

	case strings.HasSuffix(target, ".DescribeImages"):
		tr.mu.Lock()
		tr.imagePages++
		tr.mu.Unlock()
		pages := t561ECRRepos[in.RepositoryName]
		page := 0
		if in.NextToken != "" {
			page = 1
		}
		var details []map[string]any
		for _, img := range pages[page] {
			details = append(details, map[string]any{
				"registryId":     "123456789012",
				"repositoryName": in.RepositoryName,
				"imageDigest":    img.digest,
				"imageTags":      []string{"v2.14.0"},
				"imagePushedAt":  img.pushed,
			})
		}
		out := map[string]any{"imageDetails": details}
		if page+1 < len(pages) {
			out["nextToken"] = "page-2"
		}
		return t561JSON(200, out), nil

	case strings.HasSuffix(target, ".DescribeImageScanFindings"):
		tr.mu.Lock()
		tr.scanRequested[in.ImageID.ImageDigest] = true
		tr.scanPages++
		tr.mu.Unlock()
		for _, pages := range t561ECRRepos {
			for _, page := range pages {
				for _, img := range page {
					if img.digest != in.ImageID.ImageDigest {
						continue
					}
					if img.denied {
						return t561JSON(400, map[string]any{
							"__type":  "AccessDeniedException",
							"message": "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform: ecr:DescribeImageScanFindings",
						}), nil
					}
					page := 0
					if in.NextToken != "" {
						page = 1
					}
					out := map[string]any{
						"registryId":      "123456789012",
						"repositoryName":  in.RepositoryName,
						"imageId":         map[string]any{"imageDigest": img.digest},
						"imageScanStatus": map[string]any{"status": "COMPLETE", "description": "The scan was completed successfully."},
						"imageScanFindings": map[string]any{
							"findingSeverityCounts": img.counts,
							"findings":              t561ScanFindings(img.counts, page),
						},
					}
					if page == 0 {
						out["nextToken"] = "findings-2"
					}
					return t561JSON(200, out), nil
				}
			}
		}
	}
	return t561JSON(200, map[string]any{}), nil
}

// t561CapturedRepo is the JSON shape snapshot.json carries per repository.
type t561CapturedRepo struct {
	RepositoryName string `json:"repository_name"`
	Images         []struct {
		ImageDigest           string           `json:"image_digest"`
		FindingSeverityCounts map[string]int32 `json:"finding_severity_counts"`
		ScanErrorCode         string           `json:"scan_error_code"`
	} `json:"images"`
	CriticalTotal *int `json:"critical_total"`
	HighTotal     *int `json:"high_total"`
}

func TestCaptureECR_RecordsTheScanEvidenceTheAppReads(t *testing.T) {
	tr := &t561ECRTransport{scanRequested: map[string]bool{}}
	cfg := aws.Config{
		Region:           "us-east-1",
		Credentials:      credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "EXAMPLESECRETKEY", ""),
		HTTPClient:       &http.Client{Transport: tr},
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
		Repositories []t561CapturedRepo `json:"repositories"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	repos := map[string]t561CapturedRepo{}
	for _, r := range data.Repositories {
		repos[r.RepositoryName] = r
	}

	if tr.imagePages != 3 {
		t.Errorf("DescribeImages called %d times, want 3: both pages of acme/payments-api and the one page of acme/batch-worker", tr.imagePages)
	}
	const newest = "sha256:7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c0b9a8f7e6d"
	for _, older := range []string{
		"sha256:3f1c9a7e5b2d4c6a8e0f1b3d5c7e9a1b3c5d7e9f1a3b5c7d9e1f3a5b7c9d1e3f",
		"sha256:9b8a7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8b",
		"sha256:1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b",
	} {
		if tr.scanRequested[older] {
			t.Errorf("scan findings read for %s, an older image the app does not inspect", older)
		}
	}
	if tr.scanPages != 3 {
		t.Errorf("DescribeImageScanFindings called %d times, want 3: both pages for the newest acme/payments-api image and the denied acme/batch-worker read", tr.scanPages)
	}

	paying := repos["acme/payments-api"]
	if len(paying.Images) != 1 || paying.Images[0].ImageDigest != newest {
		t.Fatalf("acme/payments-api: images recorded = %+v, want only the newest %s (raw %s)", paying.Images, newest, raw)
	}
	if want := map[string]int32{"CRITICAL": 9, "HIGH": 2, "LOW": 1}; !maps.Equal(paying.Images[0].FindingSeverityCounts, want) {
		t.Errorf("acme/payments-api newest image: counts = %v, want %v (tallied over both findings pages)", paying.Images[0].FindingSeverityCounts, want)
	}
	if paying.CriticalTotal == nil || *paying.CriticalTotal != 9 || paying.HighTotal == nil || *paying.HighTotal != 2 {
		t.Errorf("acme/payments-api: critical/high totals = %v/%v, want 9/2", paying.CriticalTotal, paying.HighTotal)
	}

	worker := repos["acme/batch-worker"]
	if len(worker.Images) != 1 {
		t.Fatalf("acme/batch-worker: %d images recorded, want 1 (raw %s)", len(worker.Images), raw)
	}
	if got := worker.Images[0].ScanErrorCode; got != "AccessDeniedException" {
		t.Errorf("acme/batch-worker: scan_error_code = %q, want %q", got, "AccessDeniedException")
	}
	if len(worker.Images[0].FindingSeverityCounts) != 0 {
		t.Errorf("acme/batch-worker: counts = %v for an image whose scan read was denied", worker.Images[0].FindingSeverityCounts)
	}
	// The only image's findings were never read, so the repository's totals
	// are unknown: absent, never a 0/0 that reads as a clean scan.
	if worker.CriticalTotal != nil || worker.HighTotal != nil {
		t.Errorf("acme/batch-worker: critical/high totals = %v/%v after the scan read was denied, want both absent", worker.CriticalTotal, worker.HighTotal)
	}
}

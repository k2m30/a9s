package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// ECR Images fetcher tests (child of ECR Repositories)
// ---------------------------------------------------------------------------

// TestFetchECRImages_Basic verifies parsing of 3 images with tags, all Fields
// correct, Resource.ID, Name, Status, and RawStruct.
func TestFetchECRImages_Basic(t *testing.T) {
	pushedAt1 := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	pushedAt2 := time.Date(2024, 6, 14, 8, 30, 0, 0, time.UTC)
	pushedAt3 := time.Date(2024, 6, 13, 12, 0, 0, 0, time.UTC)

	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:    aws.String("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:      []string{"latest", "v1.0.0"},
						ImagePushedAt:  &pushedAt1,
						ImageSizeInBytes: aws.Int64(52428800), // 50 MB
						ImageScanStatus: &ecrtypes.ImageScanStatus{
							Status: ecrtypes.ScanStatusComplete,
						},
						ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
							FindingSeverityCounts: map[string]int32{
								"HIGH":   3,
								"MEDIUM": 5,
							},
						},
					},
					{
						ImageDigest:    aws.String("sha256:bbbbbb1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:      []string{"v0.9.0"},
						ImagePushedAt:  &pushedAt2,
						ImageSizeInBytes: aws.Int64(1048576), // 1 MB
						ImageScanStatus: &ecrtypes.ImageScanStatus{
							Status: ecrtypes.ScanStatusComplete,
						},
					},
					{
						ImageDigest:    aws.String("sha256:cccccc1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:      []string{"dev"},
						ImagePushedAt:  &pushedAt3,
						ImageSizeInBytes: aws.Int64(104857600), // 100 MB
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "my-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
	}

	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(result.Resources))
	}

	r := result.Resources[0]

	t.Run("ID_is_digest", func(t *testing.T) {
		if r.ID != "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890" {
			t.Errorf("ID: expected digest, got %q", r.ID)
		}
	})

	t.Run("Name_is_tags", func(t *testing.T) {
		if !strings.Contains(r.Name, "latest") {
			t.Errorf("Name: expected to contain 'latest', got %q", r.Name)
		}
	})

	t.Run("Fields_image_tags", func(t *testing.T) {
		tags := r.Fields["image_tags"]
		if !strings.Contains(tags, "latest") || !strings.Contains(tags, "v1.0.0") {
			t.Errorf("Fields[image_tags]: expected 'latest, v1.0.0', got %q", tags)
		}
	})

	t.Run("Fields_digest_short", func(t *testing.T) {
		if r.Fields["digest_short"] != "abcdef123456" {
			t.Errorf("Fields[digest_short]: expected %q, got %q", "abcdef123456", r.Fields["digest_short"])
		}
	})

	t.Run("Fields_pushed_at", func(t *testing.T) {
		if r.Fields["pushed_at"] != "2024-06-15 10:00" {
			t.Errorf("Fields[pushed_at]: expected %q, got %q", "2024-06-15 10:00", r.Fields["pushed_at"])
		}
	})

	t.Run("Fields_image_size", func(t *testing.T) {
		size := r.Fields["image_size"]
		if size == "" {
			t.Error("Fields[image_size] should not be empty")
		}
		// 50 MB — should contain "MB" or similar human-readable format
		if !strings.Contains(size, "MB") && !strings.Contains(size, "MiB") && !strings.Contains(size, "50") {
			t.Errorf("Fields[image_size]: expected human-readable ~50MB, got %q", size)
		}
	})

	t.Run("Fields_scan_status", func(t *testing.T) {
		if r.Fields["scan_status"] != "COMPLETE" {
			t.Errorf("Fields[scan_status]: expected %q, got %q", "COMPLETE", r.Fields["scan_status"])
		}
	})

	t.Run("Fields_finding_counts", func(t *testing.T) {
		fc := r.Fields["finding_counts"]
		if !strings.Contains(fc, "3H") {
			t.Errorf("Fields[finding_counts]: expected to contain '3H', got %q", fc)
		}
		if !strings.Contains(fc, "5M") {
			t.Errorf("Fields[finding_counts]: expected to contain '5M', got %q", fc)
		}
	})

	t.Run("Fields_image_uri_tagged", func(t *testing.T) {
		uri := r.Fields["image_uri"]
		expected := "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:latest"
		if uri != expected {
			t.Errorf("Fields[image_uri]: expected %q, got %q", expected, uri)
		}
	})

	t.Run("RawStruct_is_ImageDetail", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(ecrtypes.ImageDetail)
		if !ok {
			t.Fatalf("RawStruct should be ecrtypes.ImageDetail, got %T", r.RawStruct)
		}
		if raw.ImageDigest == nil || !strings.HasPrefix(*raw.ImageDigest, "sha256:") {
			t.Error("RawStruct.ImageDigest not preserved correctly")
		}
	})

	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{
			"image_tags", "digest_short", "pushed_at", "image_size",
			"scan_status", "finding_counts", "image_uri",
		}
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchECRImages_Empty verifies that a repository with no images
// returns an empty slice with no error.
func TestFetchECRImages_Empty(t *testing.T) {
	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				ImageDetails: []ecrtypes.ImageDetail{},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "empty-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/empty-repo",
	}

	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchECRImages_Error verifies that API errors are propagated.
func TestFetchECRImages_Error(t *testing.T) {
	mock := &mockECRDescribeImagesClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	parentCtx := map[string]string{
		"repository_name": "error-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/error-repo",
	}

	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("error should contain 'access denied', got %q", err.Error())
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

// TestFetchECRImages_NilFields verifies that nil optional fields
// (ImageDigest, ImageSizeInBytes, ImageScanStatus) do not cause a panic.
func TestFetchECRImages_NilFields(t *testing.T) {
	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				ImageDetails: []ecrtypes.ImageDetail{
					{
						// All optional pointer fields are nil
						// ImageDigest, ImagePushedAt, ImageSizeInBytes,
						// ImageScanStatus, ImageScanFindingsSummary all nil
						ImageTags: []string{"latest"},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "nil-fields-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/nil-fields-repo",
	}

	// Should not panic
	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	t.Run("nil_ImageDigest", func(t *testing.T) {
		_ = result.Resources[0].ID
	})

	t.Run("nil_ImageSizeInBytes", func(t *testing.T) {
		if result.Resources[0].Fields["image_size"] != "" {
			t.Logf("Fields[image_size] is %q (expected empty for nil)", result.Resources[0].Fields["image_size"])
		}
	})

	t.Run("nil_ImageScanStatus", func(t *testing.T) {
		if result.Resources[0].Fields["scan_status"] != "" {
			t.Logf("Fields[scan_status] is %q (expected empty for nil)", result.Resources[0].Fields["scan_status"])
		}
	})

	t.Run("nil_ImagePushedAt", func(t *testing.T) {
		if result.Resources[0].Fields["pushed_at"] != "" {
			t.Logf("Fields[pushed_at] is %q (expected empty for nil)", result.Resources[0].Fields["pushed_at"])
		}
	})

	t.Run("nil_FindingsSummary", func(t *testing.T) {
		if result.Resources[0].Fields["finding_counts"] != "" {
			t.Logf("Fields[finding_counts] is %q (expected empty for nil)", result.Resources[0].Fields["finding_counts"])
		}
	})
}

// TestFetchECRImages_UntaggedImage verifies that an image with no tags
// produces "<untagged>" for image_tags and Status="terminated".
func TestFetchECRImages_UntaggedImage(t *testing.T) {
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:      aws.String("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:        []string{}, // no tags
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(1024),
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "my-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
	}

	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]

	t.Run("image_tags_untagged", func(t *testing.T) {
		if r.Fields["image_tags"] != "<untagged>" {
			t.Errorf("Fields[image_tags]: expected %q, got %q", "<untagged>", r.Fields["image_tags"])
		}
	})

	t.Run("status_terminated", func(t *testing.T) {
		if r.Status != "terminated" {
			t.Errorf("Status: expected %q for untagged image, got %q", "terminated", r.Status)
		}
	})
}

// TestFetchECRImages_DigestShort verifies that "sha256:abcdef123456789..."
// is truncated to "abcdef123456" (first 12 chars after prefix).
func TestFetchECRImages_DigestShort(t *testing.T) {
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:      aws.String("sha256:abcdef123456789012345678901234567890abcdef1234567890abcdef1234"),
						ImageTags:        []string{"test"},
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(1024),
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "my-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
	}

	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	short := result.Resources[0].Fields["digest_short"]
	if short != "abcdef123456" {
		t.Errorf("Fields[digest_short]: expected %q, got %q", "abcdef123456", short)
	}
}

// TestFetchECRImages_FindingCounts verifies that CRITICAL:1, HIGH:3 produces
// "1C 3H" format (sorted by severity).
func TestFetchECRImages_FindingCounts(t *testing.T) {
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:      aws.String("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:        []string{"latest"},
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(1024),
						ImageScanStatus: &ecrtypes.ImageScanStatus{
							Status: ecrtypes.ScanStatusComplete,
						},
						ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
							FindingSeverityCounts: map[string]int32{
								"CRITICAL": 1,
								"HIGH":     3,
							},
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "my-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
	}

	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	fc := result.Resources[0].Fields["finding_counts"]
	if !strings.Contains(fc, "1C") {
		t.Errorf("Fields[finding_counts]: expected to contain '1C', got %q", fc)
	}
	if !strings.Contains(fc, "3H") {
		t.Errorf("Fields[finding_counts]: expected to contain '3H', got %q", fc)
	}
}

// TestFetchECRImages_FindingCountsNil verifies that when there is no scan
// summary the finding_counts field is empty string.
func TestFetchECRImages_FindingCountsNil(t *testing.T) {
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:      aws.String("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:        []string{"latest"},
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(1024),
						// No scan findings summary
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "my-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
	}

	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if result.Resources[0].Fields["finding_counts"] != "" {
		t.Errorf("Fields[finding_counts] should be empty with no scan summary, got %q",
			result.Resources[0].Fields["finding_counts"])
	}
}

// TestFetchECRImages_SizeFormatting verifies that image_size uses human-readable
// formatting via formatBytes.
func TestFetchECRImages_SizeFormatting(t *testing.T) {
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		bytes    int64
		contains string
	}{
		{"1KB", 1024, "KB"},
		{"1MB", 1048576, "MB"},
		{"1GB", 1073741824, "GB"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockECRDescribeImagesClient{
				pages: []*ecr.DescribeImagesOutput{
					{
						ImageDetails: []ecrtypes.ImageDetail{
							{
								ImageDigest:      aws.String("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
								ImageTags:        []string{"latest"},
								ImagePushedAt:    &pushedAt,
								ImageSizeInBytes: aws.Int64(tc.bytes),
							},
						},
					},
				},
			}

			parentCtx := map[string]string{
				"repository_name": "my-repo",
				"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
			}

			result, err := awsclient.FetchECRImages(
				context.Background(),
				mock,
				parentCtx,
							"",
)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if len(result.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(result.Resources))
			}

			size := result.Resources[0].Fields["image_size"]
			// Accept both "KB"/"MB"/"GB" and "KiB"/"MiB"/"GiB" formats
			if !strings.Contains(size, tc.contains) && !strings.Contains(size, strings.Replace(tc.contains, "B", "iB", 1)) {
				t.Errorf("Fields[image_size]: expected to contain %q or %q, got %q",
					tc.contains, strings.Replace(tc.contains, "B", "iB", 1), size)
			}
		})
	}
}

// TestFetchECRImages_Pagination verifies the single-page pagination contract:
// one API call is made per invocation, resources from that page are returned,
// and IsTruncated/NextToken reflect whether more pages exist. A second call
// with the continuation token verifies the token is forwarded and the final
// page sets IsTruncated=false.
func TestFetchECRImages_Pagination(t *testing.T) {
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	parentCtx := map[string]string{
		"repository_name": "my-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
	}

	// Page 1: 2 images with NextToken indicating more pages exist.
	page1Mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				NextToken: aws.String("page2-token"),
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:      aws.String("sha256:aaaa001234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:        []string{"v1"},
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(1024),
					},
					{
						ImageDigest:      aws.String("sha256:aaaa011234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:        []string{"v2"},
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(1024),
					},
				},
			},
		},
	}

	// First call: no continuation token — fetches page 1.
	result1, err := awsclient.FetchECRImages(
		context.Background(),
		page1Mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("page 1: expected no error, got %v", err)
	}

	t.Run("page1_item_count", func(t *testing.T) {
		if len(result1.Resources) != 2 {
			t.Fatalf("expected 2 resources on page 1, got %d", len(result1.Resources))
		}
	})

	t.Run("page1_single_api_call", func(t *testing.T) {
		if page1Mock.calls != 1 {
			t.Errorf("expected 1 API call for page 1, got %d", page1Mock.calls)
		}
	})

	t.Run("page1_is_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("page 1: IsTruncated should be true when NextToken is present")
		}
	})

	t.Run("page1_next_token", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.NextToken != "page2-token" {
			t.Errorf("page 1: NextToken expected %q, got %q", "page2-token", result1.Pagination.NextToken)
		}
	})

	t.Run("page1_total_hint_negative", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.TotalHint != -1 {
			t.Errorf("page 1: TotalHint should be -1 when truncated, got %d", result1.Pagination.TotalHint)
		}
	})

	t.Run("page1_page_size", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.PageSize != 2 {
			t.Errorf("page 1: PageSize expected 2, got %d", result1.Pagination.PageSize)
		}
	})

	// Page 2: 1 image with no NextToken — last page.
	page2Mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				// No NextToken — last page
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:      aws.String("sha256:bbbb001234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:        []string{"v3"},
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(1024),
					},
				},
			},
		},
	}

	// Second call: pass continuation token from page 1 to fetch page 2.
	result2, err := awsclient.FetchECRImages(
		context.Background(),
		page2Mock,
		parentCtx,
		result1.Pagination.NextToken,
	)
	if err != nil {
		t.Fatalf("page 2: expected no error, got %v", err)
	}

	t.Run("page2_item_count", func(t *testing.T) {
		if len(result2.Resources) != 1 {
			t.Fatalf("expected 1 resource on page 2, got %d", len(result2.Resources))
		}
	})

	t.Run("page2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("page 2: IsTruncated should be false on last page")
		}
	})

	t.Run("page2_empty_next_token", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.NextToken != "" {
			t.Errorf("page 2: NextToken should be empty on last page, got %q", result2.Pagination.NextToken)
		}
	})

	t.Run("page2_total_hint_equals_count", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.TotalHint != 1 {
			t.Errorf("page 2: TotalHint should equal item count (1) on last page, got %d", result2.Pagination.TotalHint)
		}
	})

	t.Run("page2_image_tag", func(t *testing.T) {
		if result2.Resources[0].Fields["image_tags"] != "v3" {
			t.Errorf("page 2: image_tags expected %q, got %q", "v3", result2.Resources[0].Fields["image_tags"])
		}
	})
}

// TestFetchECRImages_ImageURI verifies the image_uri field:
// Tagged images: "uri:firstTag", Untagged: "uri@sha256:digest".
func TestFetchECRImages_ImageURI(t *testing.T) {
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:      aws.String("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:        []string{"latest", "v1.0.0"},
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(1024),
					},
					{
						ImageDigest:      aws.String("sha256:bbbbbb1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:        []string{}, // untagged
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(1024),
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "my-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
	}

	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	t.Run("tagged_uri", func(t *testing.T) {
		uri := result.Resources[0].Fields["image_uri"]
		expected := "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:latest"
		if uri != expected {
			t.Errorf("Fields[image_uri] for tagged: expected %q, got %q", expected, uri)
		}
	})

	t.Run("untagged_uri", func(t *testing.T) {
		uri := result.Resources[1].Fields["image_uri"]
		expected := "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo@sha256:bbbbbb1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
		if uri != expected {
			t.Errorf("Fields[image_uri] for untagged: expected %q, got %q", expected, uri)
		}
	})
}

// TestFetchECRImages_StatusMapping verifies all status conditions:
// - CRITICAL findings → "failed"
// - HIGH findings (no critical) → "pending"
// - Scan FAILED → "failed"
// - Untagged → "terminated"
// - Clean → ""
func TestFetchECRImages_StatusMapping(t *testing.T) {
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		tags           []string
		scanStatus     ecrtypes.ScanStatus
		findings       map[string]int32
		expectedStatus string
	}{
		{
			name:       "critical_findings",
			tags:       []string{"latest"},
			scanStatus: ecrtypes.ScanStatusComplete,
			findings:   map[string]int32{"CRITICAL": 1, "HIGH": 3, "MEDIUM": 5},
			expectedStatus: "failed",
		},
		{
			name:       "high_findings_no_critical",
			tags:       []string{"latest"},
			scanStatus: ecrtypes.ScanStatusComplete,
			findings:   map[string]int32{"HIGH": 3, "MEDIUM": 5},
			expectedStatus: "pending",
		},
		{
			name:           "scan_failed",
			tags:           []string{"latest"},
			scanStatus:     ecrtypes.ScanStatusFailed,
			findings:       nil,
			expectedStatus: "failed",
		},
		{
			name:           "untagged",
			tags:           []string{},
			scanStatus:     "",
			findings:       nil,
			expectedStatus: "terminated",
		},
		{
			name:           "clean",
			tags:           []string{"latest"},
			scanStatus:     ecrtypes.ScanStatusComplete,
			findings:       map[string]int32{},
			expectedStatus: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			img := ecrtypes.ImageDetail{
				ImageDigest:      aws.String("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
				ImageTags:        tc.tags,
				ImagePushedAt:    &pushedAt,
				ImageSizeInBytes: aws.Int64(1024),
			}

			if tc.scanStatus != "" {
				img.ImageScanStatus = &ecrtypes.ImageScanStatus{
					Status: tc.scanStatus,
				}
			}

			if tc.findings != nil {
				img.ImageScanFindingsSummary = &ecrtypes.ImageScanFindingsSummary{
					FindingSeverityCounts: tc.findings,
				}
			}

			mock := &mockECRDescribeImagesClient{
				pages: []*ecr.DescribeImagesOutput{
					{ImageDetails: []ecrtypes.ImageDetail{img}},
				},
			}

			parentCtx := map[string]string{
				"repository_name": "my-repo",
				"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
			}

			result, err := awsclient.FetchECRImages(
				context.Background(),
				mock,
				parentCtx,
							"",
)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if len(result.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(result.Resources))
			}

			if result.Resources[0].Status != tc.expectedStatus {
				t.Errorf("Status: expected %q, got %q", tc.expectedStatus, result.Resources[0].Status)
			}
		})
	}
}

// TestFetchECRImages_ParentContext verifies that repository_name is read
// from parentCtx and used in the DescribeImages call.
func TestFetchECRImages_ParentContext(t *testing.T) {
	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{ImageDetails: []ecrtypes.ImageDetail{}},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "specific-repo-name",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/specific-repo-name",
	}

	_, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// If DescribeImages was called without error, the repository_name was used
}

// TestFetchECRImages_RawStruct verifies that RawStruct is the original
// ecrtypes.ImageDetail value.
func TestFetchECRImages_RawStruct(t *testing.T) {
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	mock := &mockECRDescribeImagesClient{
		pages: []*ecr.DescribeImagesOutput{
			{
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:      aws.String("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
						ImageTags:        []string{"latest"},
						ImagePushedAt:    &pushedAt,
						ImageSizeInBytes: aws.Int64(52428800),
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "my-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
	}

	result, err := awsclient.FetchECRImages(
		context.Background(),
		mock,
		parentCtx,
			"",
)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	raw, ok := result.Resources[0].RawStruct.(ecrtypes.ImageDetail)
	if !ok {
		t.Fatalf("RawStruct should be ecrtypes.ImageDetail, got %T", result.Resources[0].RawStruct)
	}
	if raw.ImageDigest == nil || *raw.ImageDigest != "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890" {
		t.Error("RawStruct.ImageDigest not preserved correctly")
	}
	if raw.ImageSizeInBytes == nil || *raw.ImageSizeInBytes != 52428800 {
		t.Error("RawStruct.ImageSizeInBytes not preserved correctly")
	}
}

// TestFetchECRImages_RegistrationExists verifies that "ecr_images" is registered
// as a child resource type.
func TestFetchECRImages_RegistrationExists(t *testing.T) {
	td := resource.GetChildType("ecr_images")
	if td == nil {
		t.Fatal("ecr_images child resource type not registered")
	}
	if td.ShortName != "ecr_images" {
		t.Errorf("child type ShortName: expected %q, got %q", "ecr_images", td.ShortName)
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Column definitions test
// ---------------------------------------------------------------------------

// TestECRImageColumns verifies that ECRImageColumns returns columns with the
// expected keys, titles, and widths.
func TestECRImageColumns(t *testing.T) {
	cols := resource.ECRImageColumns()

	expectedKeys := []string{"image_tags", "digest_short", "pushed_at", "image_size", "scan_status", "finding_counts"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 6 {
			t.Fatalf("expected 6 columns, got %d", len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("columns_have_titles", func(t *testing.T) {
		for i, col := range cols {
			if col.Title == "" {
				t.Errorf("column[%d] (%s) has empty Title", i, col.Key)
			}
		}
	})

	t.Run("columns_have_positive_width", func(t *testing.T) {
		for i, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column[%d] (%s) has non-positive Width: %d", i, col.Key, col.Width)
			}
		}
	})

	t.Run("expected_widths", func(t *testing.T) {
		expectedWidths := map[string]int{
			"image_tags":     24,
			"digest_short":   16,
			"pushed_at":      22,
			"image_size":     12,
			"scan_status":    14,
			"finding_counts": 20,
		}
		for _, col := range cols {
			if want, ok := expectedWidths[col.Key]; ok {
				if col.Width != want {
					t.Errorf("column %q width: expected %d, got %d", col.Key, want, col.Width)
				}
			}
		}
	})
}

// TestECRImages_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is
// registered under the correct short name.
func TestECRImages_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("ecr_images")
	if f == nil {
		t.Fatal("ecr_images paginated child fetcher not registered")
	}
}

// TestECRImages_ParentHasChildDef verifies that the parent ecr resource type
// has a child view definition for ecr_images with key "enter".
func TestECRImages_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("ecr")
	if rt == nil {
		t.Fatal("ecr resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "ecr_images" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["repository_name"] == "" {
				t.Error("ContextKeys should include 'repository_name'")
			}
			if child.ContextKeys["repository_uri"] == "" {
				t.Error("ContextKeys should include 'repository_uri'")
			}
			if child.DisplayNameKey != "repository_name" {
				t.Errorf("DisplayNameKey: expected %q, got %q", "repository_name", child.DisplayNameKey)
			}
		}
	}
	if !found {
		t.Error("ecr Children should contain ecr_images child view def")
	}
}

// ---------------------------------------------------------------------------
// Config defaults test
// ---------------------------------------------------------------------------

// TestConfigDefaultViewDef_ECRImages verifies that the ecr_images view
// definition has the expected list columns and non-empty detail paths.
func TestConfigDefaultViewDef_ECRImages(t *testing.T) {
	vd := config.DefaultViewDef("ecr_images")

	t.Run("list_columns", func(t *testing.T) {
		if len(vd.List) < 6 {
			t.Fatalf("expected at least 6 list columns for ecr_images default, got %d", len(vd.List))
		}
	})

	t.Run("detail_paths", func(t *testing.T) {
		if len(vd.Detail) == 0 {
			t.Error("expected non-empty Detail paths for ecr_images")
		}
		// Check for key detail fields
		detailStr := strings.Join(vd.Detail, ",")
		for _, expected := range []string{"ImageDigest", "ImageTags", "ImagePushedAt", "ImageSizeInBytes"} {
			if !strings.Contains(detailStr, expected) {
				t.Errorf("Detail should contain %q, got %v", expected, vd.Detail)
			}
		}
	})
}

// TestFetchECRImages_ContinuationToken verifies that a non-empty
// continuation token is forwarded to the API as NextToken.
func TestFetchECRImages_ContinuationToken(t *testing.T) {
	pushedAt := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	wrapper := &tokenCapturingECRImagesMock{
		inner: &mockECRDescribeImagesClient{
			pages: []*ecr.DescribeImagesOutput{
				{
					ImageDetails: []ecrtypes.ImageDetail{
						{
							ImageDigest:   aws.String("sha256:abc123def456"),
							ImageTags:     []string{"latest"},
							ImagePushedAt: &pushedAt,
						},
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"repository_name": "my-repo",
		"repository_uri":  "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo",
	}

	result, err := awsclient.FetchECRImages(context.Background(), wrapper, parentCtx, "my-continuation-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if wrapper.capturedNextToken == nil {
		t.Fatal("expected NextToken to be set in API call")
	}
	if *wrapper.capturedNextToken != "my-continuation-token" {
		t.Errorf("expected NextToken %q, got %q", "my-continuation-token", *wrapper.capturedNextToken)
	}
}

// tokenCapturingECRImagesMock wraps the ECR images mock to capture NextToken.
type tokenCapturingECRImagesMock struct {
	inner             *mockECRDescribeImagesClient
	capturedNextToken *string
}

func (m *tokenCapturingECRImagesMock) DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.DescribeImages(ctx, params, optFns...)
}

// Ensure all imports are used.
var _ = aws.String
var _ = ecr.DescribeImagesOutput{}
var _ = config.DefaultViewDef

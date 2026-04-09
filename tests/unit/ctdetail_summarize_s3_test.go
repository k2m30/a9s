package unit

import (
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/internal/aws/ctdetail"
)

// TestCTDetailSummarizeS3_PutObject verifies that SummarizeS3 emits the residual metadata
// field (contentLength) after ExtractTarget has already lifted bucketName and key.
// The summarizer receives only cleaned params — bucketName and key are absent.
func TestCTDetailSummarizeS3_PutObject(t *testing.T) {
	// cleaned params: bucketName + key already lifted by ExtractTarget
	params := map[string]any{
		"contentLength": float64(1234),
	}
	rows := ctdetail.SummarizeS3("PutObject", params)
	found := false
	for _, r := range rows {
		if r.Key == "contentLength" {
			found = true
			if r.Value == "" {
				t.Errorf("PutObject: contentLength row has empty Value; want non-empty")
			}
		}
	}
	if !found {
		t.Errorf("PutObject: expected row with Key=contentLength; got %v", rows)
	}
}

// TestCTDetailSummarizeS3_GetObject verifies that SummarizeS3 emits a row for the "range"
// header field. bucketName and key are absent (lifted upstream).
func TestCTDetailSummarizeS3_GetObject(t *testing.T) {
	params := map[string]any{
		"range": "bytes=0-1023",
	}
	rows := ctdetail.SummarizeS3("GetObject", params)
	found := false
	for _, r := range rows {
		if r.Key == "range" {
			found = true
			if r.Value != "bytes=0-1023" {
				t.Errorf("GetObject: range row Value=%q; want %q", r.Value, "bytes=0-1023")
			}
		}
	}
	if !found {
		t.Errorf("GetObject: expected row with Key=range; got %v", rows)
	}
}

// TestCTDetailSummarizeS3_PutBucketPolicy verifies that the policy document field
// is emitted as a row. bucketName is lifted by ExtractTarget catch-all.
func TestCTDetailSummarizeS3_PutBucketPolicy(t *testing.T) {
	params := map[string]any{
		"policy": `{"Version":"2012-10-17"}`,
	}
	rows := ctdetail.SummarizeS3("PutBucketPolicy", params)
	found := false
	for _, r := range rows {
		if r.Key == "policy" {
			found = true
			if r.Value == "" {
				t.Errorf("PutBucketPolicy: policy row has empty Value")
			}
		}
	}
	if !found {
		t.Errorf("PutBucketPolicy: expected row with Key=policy; got %v", rows)
	}
}

// TestCTDetailSummarizeS3_ListBuckets verifies that empty params returns a non-nil empty slice.
// ListBuckets has no request parameters after TARGET extraction.
func TestCTDetailSummarizeS3_ListBuckets(t *testing.T) {
	rows := ctdetail.SummarizeS3("ListBuckets", map[string]any{})
	if rows == nil {
		t.Fatal("SummarizeS3(ListBuckets, {}) returned nil; want non-nil []Row{}")
	}
	if len(rows) != 0 {
		t.Errorf("SummarizeS3(ListBuckets, {}): expected 0 rows; got %d: %v", len(rows), rows)
	}
}

// TestCTDetailSummarizeS3_ResidualRowsNotNavigable verifies that the S3 summarizer's
// residual metadata rows (contentLength, range, policy, etc.) are NOT marked navigable.
// TARGET extraction already handles bucket/object navigability upstream.
func TestCTDetailSummarizeS3_ResidualRowsNotNavigable(t *testing.T) {
	cases := []struct {
		eventName string
		params    map[string]any
	}{
		{"PutObject", map[string]any{"contentLength": float64(512)}},
		{"GetObject", map[string]any{"range": "bytes=0-511"}},
		{"PutBucketPolicy", map[string]any{"policy": `{"Version":"2012-10-17"}`}},
	}
	for _, tc := range cases {
		rows := ctdetail.SummarizeS3(tc.eventName, tc.params)
		for i, r := range rows {
			if r.IsNavigable {
				t.Errorf("[%s] row[%d] key=%q: IsNavigable=true; S3 residual rows must not be navigable",
					tc.eventName, i, r.Key)
			}
			if r.TargetType != "" {
				t.Errorf("[%s] row[%d] key=%q: TargetType=%q; want empty for non-navigable residual",
					tc.eventName, i, r.Key, r.TargetType)
			}
		}
	}
}

// TestCTDetailSummarizeS3_NoBucketKeyReExtraction verifies that the summarizer does NOT
// attempt to re-extract bucketName or key from params. When only non-identity fields
// are present, no bucket/object rows appear.
func TestCTDetailSummarizeS3_NoBucketKeyReExtraction(t *testing.T) {
	// Intentionally pass bucketName/key to check the summarizer ignores them
	// (TARGET extraction already handles these; if the summarizer re-emits them
	// we get duplicates in the REQUEST section).
	params := map[string]any{
		"bucketName":    "should-be-target",
		"key":           "should-be-target-too",
		"contentLength": float64(99),
	}
	rows := ctdetail.SummarizeS3("PutObject", params)
	for _, r := range rows {
		if r.Key == "Bucket" || r.Key == "Object" {
			t.Errorf("PutObject: summarizer emitted TARGET-style row Key=%q; TARGET extraction handles these upstream", r.Key)
		}
	}
}

// TestCTDetailSummarizeS3_PurityNoMutation verifies that SummarizeS3 does not mutate
// the input params map. Mutation would corrupt cleaned-params from ExtractTarget.
func TestCTDetailSummarizeS3_PurityNoMutation(t *testing.T) {
	params := map[string]any{
		"contentLength": float64(2048),
		"serverSideEncryption": "aws:kms",
		"nested": map[string]any{"x": "y"},
	}
	before := deepCopyParams(params)
	_ = ctdetail.SummarizeS3("PutObject", params)
	if !reflect.DeepEqual(params, before) {
		t.Fatalf("SummarizeS3 mutated input params: got %v, want %v", params, before)
	}
}

// TestCTDetailSummarizeS3_UnknownEvent verifies that an unrecognized S3 event name
// does not panic and returns a non-nil slice.
func TestCTDetailSummarizeS3_UnknownEvent(t *testing.T) {
	params := map[string]any{"someField": "someValue"}
	var rows []ctdetail.Row
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("SummarizeS3 panicked on unknown event: %v", r)
			}
		}()
		rows = ctdetail.SummarizeS3("CompletelyUnknownS3Event", params)
	}()
	if rows == nil {
		t.Fatal("SummarizeS3(unknown event) returned nil; want non-nil slice")
	}
}

// TestCTDetailSummarizeS3_SeverityNeverSet verifies that no row emitted by SummarizeS3
// has its Severity field populated. Severity is only set on the ACTION Event row
// by sections.go (FR-002 single-cell exception).
func TestCTDetailSummarizeS3_SeverityNeverSet(t *testing.T) {
	cases := []struct {
		eventName string
		params    map[string]any
	}{
		{"PutObject", map[string]any{"contentLength": float64(100)}},
		{"GetObject", map[string]any{"range": "bytes=0-99"}},
		{"ListBuckets", map[string]any{}},
	}
	for _, tc := range cases {
		rows := ctdetail.SummarizeS3(tc.eventName, tc.params)
		for i, r := range rows {
			if r.Severity != "" {
				t.Errorf("[%s] row[%d] key=%q: Severity=%q; summarizers must never set Severity",
					tc.eventName, i, r.Key, r.Severity)
			}
		}
	}
}

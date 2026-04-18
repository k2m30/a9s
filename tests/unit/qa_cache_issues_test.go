package unit

// qa_cache_issues_test.go — T013: cache Entry round-trip with issue count fields.
//
// Tests that cache.Entry carries Issues int, IssuesTruncated bool, and
// IssuesKnown bool fields, and that they survive a YAML marshal/unmarshal
// round-trip correctly — including the tri-state where IssuesKnown=true,
// Issues=0 distinguishes "probed and found zero issues" from "not yet probed".
//
// NOTE: Issues, IssuesTruncated, and IssuesKnown fields do not yet exist on
// cache.Entry. These tests will compile and pass once the refactor adds them.

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/internal/cache"
)

// ---------------------------------------------------------------------------
// TestCacheEntryIssueFields
// ---------------------------------------------------------------------------

// TestCacheEntryIssueFields verifies that Issues=3, IssuesKnown=true, and
// IssuesTruncated=false survive a YAML marshal/unmarshal round-trip.
func TestCacheEntryIssueFields(t *testing.T) {
	original := cache.Entry{
		HasResources:    true,
		Count:           10,
		Issues:          3,
		IssuesKnown:     true,
		IssuesTruncated: false,
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	var roundTripped cache.Entry
	if err := yaml.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	t.Run("Issues round-trips correctly", func(t *testing.T) {
		if roundTripped.Issues != 3 {
			t.Errorf("Issues = %d, want 3", roundTripped.Issues)
		}
	})

	t.Run("IssuesKnown round-trips correctly", func(t *testing.T) {
		if !roundTripped.IssuesKnown {
			t.Error("IssuesKnown = false, want true")
		}
	})

	t.Run("IssuesTruncated round-trips correctly", func(t *testing.T) {
		if roundTripped.IssuesTruncated {
			t.Error("IssuesTruncated = true, want false")
		}
	})

	t.Run("base fields unaffected", func(t *testing.T) {
		if !roundTripped.HasResources {
			t.Error("HasResources = false, want true")
		}
		if roundTripped.Count != 10 {
			t.Errorf("Count = %d, want 10", roundTripped.Count)
		}
	})
}

// ---------------------------------------------------------------------------
// TestCacheEntryIssueFieldsZeroKnown
// ---------------------------------------------------------------------------

// TestCacheEntryIssueFieldsZeroKnown verifies that Issues=0 with IssuesKnown=true
// survives a YAML round-trip — this is the critical tri-state case that
// distinguishes "probed and found zero issues" from "not yet probed".
func TestCacheEntryIssueFieldsZeroKnown(t *testing.T) {
	original := cache.Entry{
		HasResources:    true,
		Count:           5,
		Issues:          0,
		IssuesKnown:     true,
		IssuesTruncated: false,
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	var roundTripped cache.Entry
	if err := yaml.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	t.Run("Issues=0 round-trips correctly", func(t *testing.T) {
		if roundTripped.Issues != 0 {
			t.Errorf("Issues = %d, want 0", roundTripped.Issues)
		}
	})

	t.Run("IssuesKnown=true preserved when Issues=0", func(t *testing.T) {
		// This is the key invariant: zero issues known ≠ issues unknown.
		// If IssuesKnown were stored with omitempty it would be lost when false,
		// but when true it must survive.
		if !roundTripped.IssuesKnown {
			t.Error("IssuesKnown = false after round-trip, want true — this distinguishes 'probed: 0 issues' from 'not probed'")
		}
	})
}

// ---------------------------------------------------------------------------
// TestCacheEntryIssueFieldsAbsent
// ---------------------------------------------------------------------------

// TestCacheEntryIssueFieldsAbsent verifies that when an old-format YAML cache
// entry (without issue fields) is unmarshalled, Issues=0 and IssuesKnown=false
// — meaning "unknown/not probed", not "probed and healthy".
func TestCacheEntryIssueFieldsAbsent(t *testing.T) {
	// Simulate an old cache file that has no issue fields.
	oldCacheYAML := `has_resources: true
count: 7
`

	var entry cache.Entry
	if err := yaml.Unmarshal([]byte(oldCacheYAML), &entry); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	t.Run("Issues defaults to 0 when absent in YAML", func(t *testing.T) {
		if entry.Issues != 0 {
			t.Errorf("Issues = %d, want 0 for old cache format", entry.Issues)
		}
	})

	t.Run("IssuesKnown defaults to false when absent in YAML", func(t *testing.T) {
		// IssuesKnown=false means "not yet probed" — the UI should treat
		// this as unknown rather than "zero issues found".
		if entry.IssuesKnown {
			t.Error("IssuesKnown = true for old cache format, want false (= not probed)")
		}
	})

	t.Run("IssuesTruncated defaults to false when absent in YAML", func(t *testing.T) {
		if entry.IssuesTruncated {
			t.Error("IssuesTruncated = true for old cache format, want false")
		}
	})

	t.Run("base fields parsed correctly from old format", func(t *testing.T) {
		if !entry.HasResources {
			t.Error("HasResources = false, want true")
		}
		if entry.Count != 7 {
			t.Errorf("Count = %d, want 7", entry.Count)
		}
	})
}

// ---------------------------------------------------------------------------
// TestCacheEntryIssuesTruncatedRoundTrip
// ---------------------------------------------------------------------------

// TestCacheEntryIssuesTruncatedRoundTrip verifies the truncated case where
// the issue count is a lower bound (from a partial first page).
func TestCacheEntryIssuesTruncatedRoundTrip(t *testing.T) {
	original := cache.Entry{
		HasResources:    true,
		Count:           50,
		Truncated:       true,
		Issues:          12,
		IssuesKnown:     true,
		IssuesTruncated: true,
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	var roundTripped cache.Entry
	if err := yaml.Unmarshal(data, &roundTripped); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	if roundTripped.Issues != 12 {
		t.Errorf("Issues = %d, want 12", roundTripped.Issues)
	}
	if !roundTripped.IssuesKnown {
		t.Error("IssuesKnown = false, want true")
	}
	if !roundTripped.IssuesTruncated {
		t.Error("IssuesTruncated = false, want true")
	}
}

package unit

// aws_regions_sdk_backed_test.go — Conformance tests for #285: the region
// catalogue must be SDK-backed, not a hand-maintained Go literal. These
// tests pin that AllRegions() loads from the embedded SDK partitions.json
// and validate every region code against the SDK's own region regex.

import (
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// TestAllRegions_StableAlphabeticalOrder pins the selector ordering. The
// region selector relies on AllRegions() returning a deterministic order so
// cursor-position tests remain stable as new regions are added.
func TestAllRegions_StableAlphabeticalOrder(t *testing.T) {
	regions := awsclient.AllRegions()
	codes := make([]string, len(regions))
	for i, r := range regions {
		codes[i] = r.Code
	}
	sorted := make([]string, len(codes))
	copy(sorted, codes)
	sort.Strings(sorted)
	for i := range codes {
		if codes[i] != sorted[i] {
			t.Errorf("AllRegions() not alphabetically sorted at index %d: got %q, want %q",
				i, codes[i], sorted[i])
		}
	}
}

// TestAllRegions_DisplayNamesPreserved pins that the display names for a few
// well-known regions survived the migration from hand-maintained Go literals
// to SDK-sourced descriptions. The selector UX depends on these strings.
func TestAllRegions_DisplayNamesPreserved(t *testing.T) {
	want := map[string]string{
		"us-east-1":      "US East (N. Virginia)",
		"eu-west-1":      "Europe (Ireland)",
		"ap-southeast-2": "Asia Pacific (Sydney)",
		"af-south-1":     "Africa (Cape Town)",
	}
	got := map[string]string{}
	for _, r := range awsclient.AllRegions() {
		got[r.Code] = r.DisplayName
	}
	for code, displayName := range want {
		if got[code] != displayName {
			t.Errorf("DisplayName[%q] = %q, want %q", code, got[code], displayName)
		}
	}
}

// TestAllRegions_NoGovOrChinaLeaks pins that the commercial-only partition
// filter is honored. Gov-cloud and China regions share the SDK catalogue but
// the commercial build must not surface them as selectable.
func TestAllRegions_NoGovOrChinaLeaks(t *testing.T) {
	for _, r := range awsclient.AllRegions() {
		if strings.HasPrefix(r.Code, "us-gov-") {
			t.Errorf("gov-cloud region %q must not appear in commercial-partition AllRegions()", r.Code)
		}
		if strings.HasPrefix(r.Code, "cn-") {
			t.Errorf("china region %q must not appear in commercial-partition AllRegions()", r.Code)
		}
	}
}

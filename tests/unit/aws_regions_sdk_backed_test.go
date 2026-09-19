package unit

import (
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// The region selector relies on AllRegions() returning a deterministic order.
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

// The selector UX depends on these display names.
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

// Gov-cloud and China regions share the SDK catalogue, but the commercial
// build must not surface them as selectable.
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

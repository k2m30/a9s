package unit

// aws_regions_sdk_backed_test.go — Conformance tests for #285: the region
// catalogue must be SDK-backed, not a hand-maintained Go literal. These
// tests pin that AllRegions() loads from the embedded SDK partitions.json
// and validate every region code against the SDK's own region regex.

import (
	"sort"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// TestAllRegions_EveryCodeMatchesSDKRegex verifies that every region code
// returned by AllRegions() satisfies the SDK's commercial-partition region
// regex. The SDK regex lives in the same partitions.json blob we embed, so
// any future region code the SDK recognises is automatically accepted.
func TestAllRegions_EveryCodeMatchesSDKRegex(t *testing.T) {
	regions := awsclient.AllRegions()
	if len(regions) == 0 {
		t.Fatal("AllRegions() returned empty — embedded partitions.json likely unloaded")
	}
	for _, r := range regions {
		if !awsclient.ValidateRegionCode(r.Code) {
			t.Errorf("region %q does not match SDK commercial-partition regex", r.Code)
		}
	}
}

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

// TestValidateRegionCode_EdgeCases pins the regex entry point. Callers can
// use ValidateRegionCode to gate user-entered region strings before wiring
// them into NewAWSSessionContext.
func TestValidateRegionCode_EdgeCases(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{"us-east-1", true},
		{"eu-central-1", true},
		{"ap-northeast-3", true},
		{"", false},
		{"USA", false},
		{"us-east", false},    // incomplete
		{"us-east-1a", false}, // AZ, not a region
		{"not-a-region-1", false},
	}
	for _, c := range cases {
		got := awsclient.ValidateRegionCode(c.code)
		if got != c.want {
			t.Errorf("ValidateRegionCode(%q) = %v, want %v", c.code, got, c.want)
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

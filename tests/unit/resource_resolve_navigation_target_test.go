package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestResolveNavigationTarget validates that ResolveNavigationTarget correctly
// looks up both top-level resource types and child types by short name.
// These tests intentionally FAIL against the stub implementation (which always
// returns "", false, false). They pass only once the coder fills in the body.
func TestResolveNavigationTarget(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantDisplay string
		wantIsChild bool
		wantFound   bool
	}{
		{
			name:        "top-level ec2",
			input:       "ec2",
			wantDisplay: "EC2 Instances",
			wantIsChild: false,
			wantFound:   true,
		},
		{
			name:        "top-level s3",
			input:       "s3",
			wantDisplay: "S3 Buckets",
			wantIsChild: false,
			wantFound:   true,
		},
		{
			name:        "child s3_objects",
			input:       "s3_objects",
			wantDisplay: "S3 Objects",
			wantIsChild: true,
			wantFound:   true,
		},
		{
			name:        "nonexistent",
			input:       "nonexistent",
			wantDisplay: "",
			wantIsChild: false,
			wantFound:   false,
		},
		{
			name:        "empty string",
			input:       "",
			wantDisplay: "",
			wantIsChild: false,
			wantFound:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotDisplay, gotIsChild, gotFound := resource.ResolveNavigationTarget(tc.input)

			if gotFound != tc.wantFound {
				t.Errorf("ResolveNavigationTarget(%q) found=%v, want %v",
					tc.input, gotFound, tc.wantFound)
			}
			if gotIsChild != tc.wantIsChild {
				t.Errorf("ResolveNavigationTarget(%q) isChild=%v, want %v",
					tc.input, gotIsChild, tc.wantIsChild)
			}
			if gotDisplay != tc.wantDisplay {
				t.Errorf("ResolveNavigationTarget(%q) display=%q, want %q",
					tc.input, gotDisplay, tc.wantDisplay)
			}
		})
	}
}

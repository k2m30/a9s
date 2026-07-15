package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestRelatedEnter_ZeroPlusEqualsNPlus is the core pin: a truncated lower bound
// takes the IDENTICAL Enter action regardless of how many were found so far —
// "(0+)" and "(N+)" both Navigate. No site may special-case the zero bound into
// a re-dispatch/no-op (the "0+ NOT NAVIGABLE" bug).
func TestRelatedEnter_ZeroPlusEqualsNPlus(t *testing.T) {
	zeroPlus := resource.RelatedEnter(domain.RelatedResolved, 0, true)
	nPlus := resource.RelatedEnter(domain.RelatedResolved, 7, true)
	if zeroPlus != resource.RelatedEnterNavigate {
		t.Errorf("(0+) → %v, want RelatedEnterNavigate", zeroPlus)
	}
	if zeroPlus != nPlus {
		t.Errorf("(0+) action %v != (N+) action %v — the zero lower bound must not be special-cased", zeroPlus, nPlus)
	}
}

func TestRelatedEnter_AllStates(t *testing.T) {
	cases := []struct {
		name      string
		state     domain.RelatedRowState
		count     int
		truncated bool
		want      resource.RelatedEnterAction
	}{
		{"resolved exact N>0 → navigate", domain.RelatedResolved, 3, false, resource.RelatedEnterNavigate},
		{"resolved exact 0 → dead end", domain.RelatedResolved, 0, false, resource.RelatedEnterDeadEnd},
		{"resolved truncated 0+ → navigate", domain.RelatedResolved, 0, true, resource.RelatedEnterNavigate},
		{"resolved truncated N+ → navigate", domain.RelatedResolved, 5, true, resource.RelatedEnterNavigate},
		{"deferred (server filter) → navigate", domain.RelatedDeferred, 0, false, resource.RelatedEnterNavigate},
		{"unknown (blank, no scope) → resolve in place", domain.RelatedUnknown, 0, false, resource.RelatedEnterResolveInPlace},
		{"error → dead end", domain.RelatedError, 0, false, resource.RelatedEnterDeadEnd},
		{"loading → dead end", domain.RelatedLoading, 0, false, resource.RelatedEnterDeadEnd},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resource.RelatedEnter(tc.state, tc.count, tc.truncated); got != tc.want {
				t.Errorf("RelatedEnter(%v, %d, %v) = %v, want %v", tc.state, tc.count, tc.truncated, got, tc.want)
			}
		})
	}
}

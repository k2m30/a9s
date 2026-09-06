package unit

// qa_acm_validation_timed_out_test.go — ACM certificate status no longer
// colours a row on its own.
//
// INVERTED for batch w6a. This file used to hold six one-status tests pinning
// the raw-field switch in acmColor: VALIDATION_TIMED_OUT, EXPIRED, REVOKED and
// FAILED to Broken, PENDING_VALIDATION to Warning, INACTIVE to Dim. That
// switch is gone. Colour derives from findings only, so a resource carrying no
// findings is Healthy whatever its status field says, and each of these states
// is reported by the finding the ACM fetcher emits for it.
//
// The six statuses are kept as one table because the enumeration is the point:
// each one must stay uncoloured on the bare-Fields path. Do not "restore" the
// old expectations — a raw-field branch coming back is what this now catches.
//
// The original bug this file was opened for (VALIDATION_TIMED_OUT falling
// through to the default) is now impossible in the same way: every status
// falls through, and the finding carries the severity.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

func acmResource(status string) resource.Resource {
	return resource.Resource{
		ID:     "arn:aws:acm:us-east-1:123456789012:certificate/abc-123",
		Name:   "example.com",
		Fields: map[string]string{"status": status},
	}
}

func TestACMColor_StatusAloneNeverColoursARow(t *testing.T) {
	td := resource.FindResourceType("acm")
	if td == nil {
		t.Fatal("acm type not registered")
	}
	for _, status := range []string{
		"VALIDATION_TIMED_OUT",
		"ISSUED",
		"PENDING_VALIDATION",
		"EXPIRED",
		"REVOKED",
		"FAILED",
		"INACTIVE",
	} {
		t.Run(status, func(t *testing.T) {
			if got := td.Color(acmResource(status)); got != resource.ColorHealthy {
				t.Errorf("acm Color status=%s = %v, want ColorHealthy; acmColor reads the status field again", status, got)
			}
		})
	}
}

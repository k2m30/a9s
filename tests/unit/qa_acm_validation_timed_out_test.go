package unit

// qa_acm_validation_timed_out_test.go — ACM certificate status does not
// colour a row on its own.
//
// Colour derives from findings only, so a resource carrying no findings is
// Healthy whatever its status field says, and each of VALIDATION_TIMED_OUT,
// EXPIRED, REVOKED, FAILED, PENDING_VALIDATION and INACTIVE is reported by
// the finding the ACM fetcher emits for it. The six statuses are one table
// because the enumeration is the point: each one must stay uncoloured on the
// bare-Fields path.

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

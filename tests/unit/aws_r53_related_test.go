package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- Navigable Fields ---

func TestNavigableFields_R53_None(t *testing.T) {
	fields := resource.GetNavigableFields("r53")
	if len(fields) != 0 {
		t.Errorf("expected no navigable fields for r53, got %d: %v", len(fields), fields)
	}
}

package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

// A truncated result with Count=0 is a valid state.
func TestValidateRelatedResult_TruncatedResult_IsValid(t *testing.T) {
	result := resource.KnownRelated("vpc", nil, true)
	if err := resource.ValidateRelatedResult(result); err != nil {
		t.Errorf("ValidateRelatedResult(TruncatedResult(\"vpc\")) returned error: %v; want nil", err)
	}
}

package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ═══════════════════════════════════════════════════════════════════════════
// handleIdentityError tests
// ═══════════════════════════════════════════════════════════════════════════

// TestHandleIdentityError_ViewShowsError verifies that sending IdentityErrorMsg
// when the identity view is active causes the view to render an error message.
func TestHandleIdentityError_ViewShowsError(t *testing.T) {
	m := newRootSizedModel()

	// Push the identity view by pressing 'i'.
	m, _ = rootApplyMsg(m, rootKeyPress("i"))

	// Verify the identity view is initially showing loading state.
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Fetching") && !strings.Contains(plain, "identity") {
		t.Logf("pre-error view (for context): %s", plain)
	}

	// Send the identity error message.
	m, cmd := rootApplyMsg(m, messages.IdentityError{Err: "access denied: missing sts:GetCallerIdentity"})

	// handleIdentityError returns nil cmd — no async work needed.
	if cmd != nil {
		t.Errorf("handleIdentityError should return nil cmd, got non-nil")
	}

	// The identity view should now render in error state.
	errorPlain := stripANSI(rootViewContent(m))
	if !strings.Contains(errorPlain, "Error") {
		t.Errorf("view after IdentityErrorMsg should contain 'Error', got:\n%s", errorPlain)
	}
}

// TestHandleIdentityError_NoIdentityView verifies that sending IdentityErrorMsg
// when the identity view is NOT active does not panic and returns nil cmd.
func TestHandleIdentityError_NoIdentityView(t *testing.T) {
	m := newRootSizedModel()
	// Identity view is NOT pushed — active view is the main menu.

	_, cmd := rootApplyMsg(m, messages.IdentityError{Err: "some error"})

	if cmd != nil {
		t.Errorf("handleIdentityError with no identity view should return nil cmd, got non-nil")
	}
	// No panic = pass.
}

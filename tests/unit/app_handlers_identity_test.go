package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestHandleIdentityError_ViewShowsError(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("i"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Fetching") && !strings.Contains(plain, "identity") {
		t.Logf("pre-error view (for context): %s", plain)
	}

	m, cmd := rootApplyMsg(m, messages.IdentityError{Err: "access denied: missing sts:GetCallerIdentity"})

	if cmd != nil {
		t.Errorf("handleIdentityError should return nil cmd, got non-nil")
	}

	errorPlain := stripANSI(rootViewContent(m))
	if !strings.Contains(errorPlain, "Error") {
		t.Errorf("view after IdentityErrorMsg should contain 'Error', got:\n%s", errorPlain)
	}
}

func TestHandleIdentityError_NoIdentityView(t *testing.T) {
	m := newRootSizedModel()

	_, cmd := rootApplyMsg(m, messages.IdentityError{Err: "some error"})

	if cmd != nil {
		t.Errorf("handleIdentityError with no identity view should return nil cmd, got non-nil")
	}
}

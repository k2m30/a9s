package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ═══════════════════════════════════════════════════════════════════════════
// handleRelatedNavigateChild tests
// Coverage gap: child type routing from the related panel.
// ═══════════════════════════════════════════════════════════════════════════

// TestHandleRelatedNavigateChild_ValidChildType verifies that sending a
// RelatedNavigateMsg with a registered child type produces an EnterChildViewMsg.
func TestHandleRelatedNavigateChild_ValidChildType(t *testing.T) {
	m := newRootSizedModel()

	// "ecr_images" is a registered child type (ecr_images.go init).
	msg := messages.RelatedNavigateMsg{
		TargetType: "ecr_images",
	}

	newM, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("Update returned nil cmd, want a cmd that emits EnterChildViewMsg")
	}

	result := cmd()
	enterMsg, ok := result.(messages.EnterChildViewMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want messages.EnterChildViewMsg", result)
	}
	if enterMsg.ChildType != "ecr_images" {
		t.Errorf("EnterChildViewMsg.ChildType = %q, want %q", enterMsg.ChildType, "ecr_images")
	}
	// Model state is unchanged (handleRelatedNavigateChild does not mutate stack).
	_ = newM
}

// TestHandleRelatedNavigateChild_UnknownChildType verifies that sending a
// RelatedNavigateMsg with an unregistered child type produces a FlashMsg with
// IsError=true.
func TestHandleRelatedNavigateChild_UnknownChildType(t *testing.T) {
	m := newRootSizedModel()

	msg := messages.RelatedNavigateMsg{
		TargetType: "nonexistent_child_xyz",
	}

	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("Update returned nil cmd, want a cmd that emits FlashMsg")
	}

	result := cmd()
	// The root model routes unknown types through ResolveRelatedNavigate which
	// returns NavigationKindFlash for unregistered types (neither child nor top-level).
	flashMsg, ok := result.(messages.FlashMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want messages.FlashMsg", result)
	}
	if !flashMsg.IsError {
		t.Errorf("FlashMsg.IsError = false, want true for unknown child type")
	}
	if flashMsg.Text == "" {
		t.Error("FlashMsg.Text is empty, want a non-empty error message")
	}
}

// TestResolveRelatedNavigate_ChildTypeReturnsKindEnterChildView verifies the
// pure resolver returns NavigationKindEnterChildView for a registered child type,
// exercising the handleRelatedNavigateChild dispatch condition directly.
func TestResolveRelatedNavigate_ChildTypeReturnsKindEnterChildView(t *testing.T) {
	ev := runtime.RelatedNavigateEvent{
		TargetType: "ecr_images",
	}
	cache := map[string][]resource.Resource{}

	result := runtime.ResolveRelatedNavigate(ev, cache)

	if result.Kind != runtime.NavigationKindEnterChildView {
		t.Errorf("Kind = %v, want NavigationKindEnterChildView (%v)", result.Kind, runtime.NavigationKindEnterChildView)
	}
	if result.TargetType != "ecr_images" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "ecr_images")
	}
}

// TestResolveRelatedNavigate_UnknownTypeReturnsKindFlash verifies the pure
// resolver returns NavigationKindFlash for an entirely unknown type.
func TestResolveRelatedNavigate_UnknownTypeReturnsKindFlash(t *testing.T) {
	ev := runtime.RelatedNavigateEvent{
		TargetType: "nonexistent_xyz",
	}
	cache := map[string][]resource.Resource{}

	result := runtime.ResolveRelatedNavigate(ev, cache)

	if result.Kind != runtime.NavigationKindFlash {
		t.Errorf("Kind = %v, want NavigationKindFlash (%v)", result.Kind, runtime.NavigationKindFlash)
	}
	if !result.FlashIsError {
		t.Error("FlashIsError = false, want true for unknown type")
	}
}

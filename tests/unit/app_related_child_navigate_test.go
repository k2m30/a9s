package unit

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestHandleRelatedNavigateChild_ValidChildType(t *testing.T) {
	m := newRootSizedModel()

	// ecr_images is registered as a child type.
	msg := messages.RelatedNavigate{
		TargetType: "ecr_images",
	}

	newM, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("Update returned nil cmd, want a cmd that emits EnterChildViewMsg")
	}

	result := cmd()
	enterMsg, ok := result.(messages.EnterChildView)
	if !ok {
		t.Fatalf("cmd() returned %T, want messages.EnterChildView", result)
	}
	if enterMsg.ChildType != "ecr_images" {
		t.Errorf("EnterChildViewMsg.ChildType = %q, want %q", enterMsg.ChildType, "ecr_images")
	}
	_ = newM
}

func TestHandleRelatedNavigateChild_UnknownChildType(t *testing.T) {
	m := newRootSizedModel()

	msg := messages.RelatedNavigate{
		TargetType: "nonexistent_child_xyz",
	}

	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatal("Update returned nil cmd, want a cmd that emits FlashMsg")
	}

	result := cmd()
	flashMsg, ok := result.(messages.Flash)
	if !ok {
		t.Fatalf("cmd() returned %T, want messages.Flash", result)
	}
	if !flashMsg.IsError {
		t.Errorf("FlashMsg.IsError = false, want true for unknown child type")
	}
	if flashMsg.Text == "" {
		t.Error("FlashMsg.Text is empty, want a non-empty error message")
	}
}

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

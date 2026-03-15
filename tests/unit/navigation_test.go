package unit

import (
	"testing"

	"github.com/k2m30/a9s/internal/navigation"
)

func TestNavigationStack_PushAndCanGoBack(t *testing.T) {
	var ns navigation.NavigationStack

	if ns.CanGoBack() {
		t.Error("expected CanGoBack=false on empty stack")
	}
	if ns.CanGoForward() {
		t.Error("expected CanGoForward=false on empty stack")
	}

	ns.Push(navigation.ViewState{ViewType: navigation.MainMenuView, CursorPos: 0})
	ns.Push(navigation.ViewState{ViewType: navigation.ResourceListView, ResourceType: "EC2", CursorPos: 1})
	ns.Push(navigation.ViewState{ViewType: navigation.DetailView, ResourceType: "EC2", CursorPos: 2})

	if !ns.CanGoBack() {
		t.Error("expected CanGoBack=true after pushing 3 states")
	}
	if ns.CanGoForward() {
		t.Error("expected CanGoForward=false after pushes (no pops yet)")
	}
}

func TestNavigationStack_Pop(t *testing.T) {
	var ns navigation.NavigationStack

	ns.Push(navigation.ViewState{ViewType: navigation.MainMenuView, CursorPos: 0})
	ns.Push(navigation.ViewState{ViewType: navigation.ResourceListView, ResourceType: "EC2", CursorPos: 1})
	ns.Push(navigation.ViewState{ViewType: navigation.DetailView, ResourceType: "EC2", CursorPos: 2})

	// Pop most recent
	state, ok := ns.Pop()
	if !ok {
		t.Fatal("expected Pop to return ok=true")
	}
	if state.ViewType != navigation.DetailView {
		t.Errorf("expected DetailView, got %v", state.ViewType)
	}
	if state.CursorPos != 2 {
		t.Errorf("expected CursorPos=2, got %d", state.CursorPos)
	}

	// Should still be able to go back (2 items remain)
	if !ns.CanGoBack() {
		t.Error("expected CanGoBack=true with 2 items remaining")
	}

	// Pop remaining
	state, ok = ns.Pop()
	if !ok {
		t.Fatal("expected Pop to return ok=true")
	}
	if state.ViewType != navigation.ResourceListView {
		t.Errorf("expected ResourceListView, got %v", state.ViewType)
	}

	state, ok = ns.Pop()
	if !ok {
		t.Fatal("expected Pop to return ok=true")
	}
	if state.ViewType != navigation.MainMenuView {
		t.Errorf("expected MainMenuView, got %v", state.ViewType)
	}

	// Back stack should now be empty
	if ns.CanGoBack() {
		t.Error("expected CanGoBack=false after popping all items")
	}

	// Pop from empty stack
	_, ok = ns.Pop()
	if ok {
		t.Error("expected Pop to return ok=false on empty stack")
	}
}

func TestNavigationStack_Forward(t *testing.T) {
	var ns navigation.NavigationStack

	ns.Push(navigation.ViewState{ViewType: navigation.MainMenuView, CursorPos: 0})
	ns.Push(navigation.ViewState{ViewType: navigation.ResourceListView, ResourceType: "S3", CursorPos: 1})

	// Pop to create forward entry
	popped, _ := ns.Pop()

	if !ns.CanGoForward() {
		t.Error("expected CanGoForward=true after Pop")
	}

	// Forward should return the popped state
	state, ok := ns.Forward()
	if !ok {
		t.Fatal("expected Forward to return ok=true")
	}
	if state.ViewType != popped.ViewType {
		t.Errorf("expected Forward to return %v, got %v", popped.ViewType, state.ViewType)
	}

	if ns.CanGoForward() {
		t.Error("expected CanGoForward=false after Forward consumed the entry")
	}
}

func TestNavigationStack_PushClearsForward(t *testing.T) {
	var ns navigation.NavigationStack

	ns.Push(navigation.ViewState{ViewType: navigation.MainMenuView})
	ns.Push(navigation.ViewState{ViewType: navigation.ResourceListView})

	// Pop to create forward entry
	ns.Pop()

	if !ns.CanGoForward() {
		t.Error("expected CanGoForward=true after Pop")
	}

	// Push should clear forward stack
	ns.Push(navigation.ViewState{ViewType: navigation.JSONView})

	if ns.CanGoForward() {
		t.Error("expected CanGoForward=false after Push (forward stack should be cleared)")
	}
}

func TestNavigationStack_Clear(t *testing.T) {
	var ns navigation.NavigationStack

	ns.Push(navigation.ViewState{ViewType: navigation.MainMenuView})
	ns.Push(navigation.ViewState{ViewType: navigation.ResourceListView})
	ns.Pop() // create forward entry

	if !ns.CanGoBack() {
		t.Error("expected CanGoBack=true before Clear")
	}
	if !ns.CanGoForward() {
		t.Error("expected CanGoForward=true before Clear")
	}

	ns.Clear()

	if ns.CanGoBack() {
		t.Error("expected CanGoBack=false after Clear")
	}
	if ns.CanGoForward() {
		t.Error("expected CanGoForward=false after Clear")
	}
}

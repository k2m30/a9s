package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// T016: When a ResourceTypeDef has a non-nil StubCreator and autoOpenSingleDetail is true,
// a ResourcesLoadedMsg with an empty result set must emit a NavigateMsg to detail using
// the stub created by StubCreator — regardless of the type's ShortName.
//
// FAILS before fix: resourcelist.go checks `m.typeDef.ShortName == "ami"` so "test-stub"
// never matches and no NavigateMsg is returned.
func TestResourceListModel_StubCreator_NavigatesToDetail(t *testing.T) {
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "Test Stub Type",
		ShortName: "test-stub",
		Columns: []resource.Column{
			{Key: "id", Title: "ID", Width: 20},
		},
		StubCreator: func(id string) resource.Resource {
			return resource.Resource{
				ID:     id,
				Name:   id,
				Status: "-",
				Fields: map[string]string{
					"id": id,
				},
			}
		},
	}

	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	m.SetRelatedIDFilter([]string{"img-12345"})
	m.SetAutoOpenSingleDetail(true)

	var got tea.Cmd
	m, got = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "test-stub",
		Resources:    []resource.Resource{}, // empty — no match
	})

	if got == nil {
		t.Fatal("T016: expected a cmd to be returned when StubCreator is set and filtered list is empty, got nil")
	}

	rawMsg := got()
	navMsg, ok := rawMsg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("T016: expected messages.NavigateMsg, got %T: %+v", rawMsg, rawMsg)
	}
	if navMsg.ResourceType != "test-stub" {
		t.Errorf("T016: NavigateMsg.ResourceType = %q; want %q", navMsg.ResourceType, "test-stub")
	}
	if navMsg.Resource == nil {
		t.Fatal("T016: NavigateMsg.Resource is nil; expected stub resource")
	}
	if navMsg.Resource.ID != "img-12345" {
		t.Errorf("T016: NavigateMsg.Resource.ID = %q; want %q", navMsg.Resource.ID, "img-12345")
	}
	if navMsg.Target != messages.TargetDetail {
		t.Errorf("T016: NavigateMsg.Target = %v; want TargetDetail", navMsg.Target)
	}
	if !navMsg.ReplaceCurrent {
		t.Error("T016: NavigateMsg.ReplaceCurrent should be true for stub navigation")
	}
}

// T017: When a ResourceTypeDef has a nil StubCreator (no stub support),
// a ResourcesLoadedMsg with an empty result set must NOT emit a NavigateMsg.
//
// This is a regression guard: after the fix replaces `ShortName == "ami"` with
// `StubCreator != nil`, a nil StubCreator must still produce no navigation.
// PASSES both before and after the fix.
func TestResourceListModel_NoStubCreator_NoNavigation(t *testing.T) {
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:        "Test No Stub Type",
		ShortName:   "test-stub",
		StubCreator: nil, // explicitly nil
		Columns: []resource.Column{
			{Key: "id", Title: "ID", Width: 20},
		},
	}

	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	m.SetRelatedIDFilter([]string{"img-12345"})
	m.SetAutoOpenSingleDetail(true)

	var got tea.Cmd
	m, got = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "test-stub",
		Resources:    []resource.Resource{}, // empty — no match
	})

	if got == nil {
		// No cmd is the expected outcome — done.
		return
	}

	rawMsg := got()
	if _, ok := rawMsg.(messages.NavigateMsg); ok {
		t.Error("T017: NavigateMsg must NOT be emitted when StubCreator is nil")
	}
}

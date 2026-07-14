package tui

// related_navigate_stub_whitebox_test.go — OPEN INVESTIGATION white-box port
// (specs/022-codebase-cleanup/wave3-status.md §"OPEN INVESTIGATION — possible
// live regression (StubCreator / auto-open-single)").
//
// tests/unit/resourcelist_ami_stub_test.go's T016/T017 drive
// ResourceListModel.Update(messages.ResourcesLoaded) directly. That is dead
// code in production: app.go's root Update() switch routes every
// messages.ResourcesLoaded to m.handleResourcesLoaded
// (runtime_adapter_resources.go), never to the active view's own Update() —
// the same class of false-confidence gap already found in
// text_ctrl_interaction_test.go (YAMLModel.Update(), also never called by
// production). This file drives the real chain instead: root Model.Update()
// -> handleRelatedNavigate -> NavigationKindFilteredList (TargetID branch,
// Clients()==nil, matching the "no by-ID fetcher or clients not yet ready"
// fallback) -> newRelatedList(autoOpenSingleDetail:true) ->
// SetListAutoOpenSingle -> root Model.Update(empty ResourcesLoaded) ->
// handleResourcesLoaded's StubCreator branch.
//
// Uses the real "ami" ResourceTypeDef (internal/aws/catalog_compute.go),
// which has both a StubCreator and FetchByIDs registered — the same type the
// original resourcelist_ami_stub_test.go was written against, and the type
// named in the investigation's traced chain. FetchByIDs matters: with
// Clients() non-nil, handleRelatedNavigate's TargetID branch short-circuits
// straight to a KindFetchByIDDetail task and never reaches newRelatedList at
// all, so a real AWS-connected session cannot exercise this branch for "ami" —
// only the not-yet-connected window (RelatedNavigate before ClientsReady)
// reaches it, matching the "clients not yet ready" comment in
// runtime_adapter_related.go verbatim.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// newNoClientsModel builds a root Model that has never received
// messages.ClientsReady, so m.core.Clients() is nil — the exact precondition
// the investigation's traced chain needs to reach the "no by-ID fetcher or
// clients not yet ready" fallback branch instead of the direct
// KindFetchByIDDetail short-circuit.
func newNoClientsModel(t *testing.T) Model {
	t.Helper()
	m := New("demo", "us-east-1")
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	sized, ok := newM.(Model)
	if !ok {
		t.Fatalf("Model.Update(tea.WindowSizeMsg) returned %T; want tui.Model", newM)
	}
	return sized
}

// firstNavigateMsg executes cmd and returns the first messages.Navigate it
// finds, walking one tea.Batch level deep — the same one-level-deep drain the
// project already uses for auto-open-single batches
// (tests/unit/related_navigate_count_spec008_test.go's applyRelatedFollowUp),
// deep enough for handleResourcesLoaded's tea.Batch(coreCmd, navCmd) without
// recursing into unrelated async follow-ups.
func firstNavigateMsg(cmd tea.Cmd) (messages.Navigate, bool) {
	if cmd == nil {
		return messages.Navigate{}, false
	}
	msg := cmd()
	if nav, ok := msg.(messages.Navigate); ok {
		return nav, true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			if nav, ok := sub().(messages.Navigate); ok {
				return nav, true
			}
		}
	}
	return messages.Navigate{}, false
}

// amiSourceEC2 is the EC2 instance whose ImageId field drives the
// navigable-field RelatedNavigate to "ami" (internal/aws/catalog_compute.go's
// Navigable: []domain.NavigableField{{FieldPath: "ImageId", TargetType:
// "ami"}}) — the same field-navigation origin the investigation traced.
func amiSourceEC2() resource.Resource {
	return resource.Resource{
		ID:   "i-0stubwhitebox000001",
		Type: "ec2",
		Name: "i-0stubwhitebox000001",
		Fields: map[string]string{
			"ImageId": "ami-doesnotexist00001",
		},
	}
}

// TestHandleResourcesLoaded_AMIStub_AutoOpensDetail_NoClients pins the
// StubCreator auto-open-single behavior resourcelist_ami_stub_test.go's T016
// pins for the legacy dead path, driven instead through the real production
// dispatch (root Model.Update -> handleRelatedNavigate ->
// handleResourcesLoaded). An empty by-ID AMI lookup must still land on the
// StubCreator's synthetic detail — this is the whole point of a StubCreator:
// AMIs referenced by an EC2 instance's ImageId can be deregistered/foreign-
// account and thus absent from any listable page, and the UI must not strand
// the user on an empty list.
func TestHandleResourcesLoaded_AMIStub_AutoOpensDetail_NoClients(t *testing.T) {
	m := newNoClientsModel(t)
	if m.core.Clients() != nil {
		t.Fatal("precondition failed: Clients() is non-nil before ClientsReady — test no longer exercises the no-clients fallback branch")
	}

	src := amiSourceEC2()
	navResult, _ := m.Update(messages.RelatedNavigate{
		TargetType:     "ami",
		SourceResource: src,
		SourceType:     "ec2",
		TargetID:       "ami-doesnotexist00001",
		DirectDetail:   true,
	})
	m, ok := navResult.(Model)
	if !ok {
		t.Fatalf("Model.Update(messages.RelatedNavigate) returned %T; want tui.Model", navResult)
	}

	loadedResult, cmd := m.Update(messages.ResourcesLoaded{
		ResourceType: "ami",
		Resources:    []resource.Resource{},
	})
	if _, ok := loadedResult.(Model); !ok {
		t.Fatalf("Model.Update(messages.ResourcesLoaded) returned %T; want tui.Model", loadedResult)
	}

	if cmd == nil {
		t.Fatal("expected a cmd carrying messages.Navigate to the AMI StubCreator's detail; got nil — auto-open-single did not fire on the live path")
	}

	navMsg, found := firstNavigateMsg(cmd)
	if !found {
		t.Fatal("expected messages.Navigate somewhere in the returned cmd tree; none found")
	}
	if navMsg.Target != messages.TargetDetail {
		t.Errorf("Navigate.Target = %v; want TargetDetail", navMsg.Target)
	}
	if navMsg.ResourceType != "ami" {
		t.Errorf("Navigate.ResourceType = %q; want %q", navMsg.ResourceType, "ami")
	}
	if navMsg.Resource == nil {
		t.Fatal("Navigate.Resource is nil; expected the AMI StubCreator's synthetic stub")
	}
	if navMsg.Resource.ID != "ami-doesnotexist00001" {
		t.Errorf("Navigate.Resource.ID = %q; want %q", navMsg.Resource.ID, "ami-doesnotexist00001")
	}
	if !navMsg.ReplaceCurrent {
		t.Error("Navigate.ReplaceCurrent should be true for stub navigation")
	}
}

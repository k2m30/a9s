package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestDemoColdCacheS3_ListPopulates verifies that navigating to the S3 resource
// list with a cold cache produces at least one bucket from the demo transport.
func TestDemoColdCacheS3_ListPopulates(t *testing.T) {
	t.Parallel()
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{s3}, got nil")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})

	result := raw.(messages.ResourcesLoaded)

	if len(result.Resources) == 0 {
		t.Fatal("expected at least one S3 bucket in fixture data, got zero")
	}

	*m, _ = rootApplyMsg(*m, result)

	plain := stripANSI(rootViewContent(*m))
	hasName := false
	for _, r := range result.Resources {
		if strings.Contains(plain, r.ID) || strings.Contains(plain, r.Name) {
			hasName = true
			break
		}
	}
	if !hasName {
		t.Errorf("S3 list view does not contain any bucket name from fixtures; view:\n%s", plain)
	}
}

// TestDemoColdCacheS3_ObjectsChildView verifies the full child-nav flow:
// S3 list → EnterChildViewMsg{s3_objects} → fetch cmd → ResourcesLoadedMsg{s3_objects}.
//
// The test uses EnterChildViewMsg directly rather than a key press because key
// injection requires a live program loop. The message mirrors what ResourceList
// emits when Enter is pressed on a bucket row.
func TestDemoColdCacheS3_ObjectsChildView(t *testing.T) {
	t.Parallel()
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{s3}, got nil")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)

	if len(loaded.Resources) == 0 {
		t.Fatal("fixture data has zero S3 buckets; cannot drill into child view")
	}

	*m, _ = rootApplyMsg(*m, loaded)

	// Pick the first bucket and drill in. ContextKeys for s3→s3_objects:
	//   {"bucket": "ID", "prefix": ""} — bucket name is the resource ID.
	firstBucket := loaded.Resources[0]
	bucketName := firstBucket.ID

	var childCmd tea.Cmd
	*m, childCmd = rootApplyMsg(*m, messages.EnterChildView{
		ChildType:     "s3_objects",
		ParentContext: map[string]string{"bucket": bucketName, "prefix": ""},
		DisplayName:   bucketName,
	})

	if childCmd == nil {
		t.Fatal("expected a cmd after EnterChildViewMsg{s3_objects}, got nil — " +
			"is s3_objects registered as a child type?")
	}

	childRaw := extractMsg(t, childCmd, func(msg tea.Msg) bool {
		if r, ok := msg.(messages.ResourcesLoaded); ok {
			return r.ResourceType == "s3_objects"
		}
		return false
	})

	childLoaded, ok := childRaw.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected ResourcesLoadedMsg{s3_objects}; got %T", childRaw)
	}

	if childLoaded.ResourceType != "s3_objects" {
		t.Errorf("ResourcesLoadedMsg.ResourceType = %q; want %q", childLoaded.ResourceType, "s3_objects")
	}

	*m, _ = rootApplyMsg(*m, childLoaded)

	plain := stripANSI(rootViewContent(*m))
	if plain == "" {
		t.Error("view is empty after loading S3 objects into child view")
	}
}

// TestDemoColdCacheS3_UnknownBucketReturnsError: fetching s3_objects for a
// bucket that does not exist in the fixture must produce an error, not an
// empty list.
func TestDemoColdCacheS3_UnknownBucketReturnsError(t *testing.T) {
	t.Parallel()
	m := newDemoColdCacheApp(t)

	*m, _ = rootApplyMsg(*m, tea.WindowSizeMsg{Width: 120, Height: 40})

	clients := demo.NewServiceClients()
	*m, _ = rootApplyMsg(*m, messages.ClientsReady{Clients: clients, Gen: 1})

	// Navigate to S3 list first so the model has clients wired.
	var navCmd tea.Cmd
	*m, navCmd = rootApplyMsg(*m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	if navCmd == nil {
		t.Fatal("expected a cmd after NavigateMsg{s3}, got nil")
	}

	raw := extractMsg(t, navCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoaded)
		return ok
	})
	loaded := raw.(messages.ResourcesLoaded)
	*m, _ = rootApplyMsg(*m, loaded)

	var childCmd tea.Cmd
	*m, childCmd = rootApplyMsg(*m, messages.EnterChildView{
		ChildType:     "s3_objects",
		ParentContext: map[string]string{"bucket": "nonexistent-bucket-xyz-00000", "prefix": ""},
		DisplayName:   "nonexistent-bucket-xyz-00000",
	})

	if childCmd == nil {
		t.Fatal("expected a cmd after EnterChildViewMsg{s3_objects/unknown}, got nil")
	}

	childMsg := childCmd()

	if batch, ok := childMsg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			childMsg = sub()
			break
		}
	}

	switch v := childMsg.(type) {
	case messages.APIError:
		if v.Err == nil {
			t.Error("APIErrorMsg.Err must not be nil for unknown bucket")
		}
	case messages.ResourcesLoaded:
		if len(v.Resources) == 0 {
			t.Errorf("contract violation: fetching s3_objects for unknown bucket returned " +
				"empty ResourcesLoadedMsg instead of an error — " +
				"fake must return s3types.NoSuchBucket for unknown buckets (contract rule 4)")
		} else {
			t.Errorf("fetching s3_objects for unknown bucket returned %d resources — "+
				"fake should not produce objects for a nonexistent bucket", len(v.Resources))
		}
	case messages.Flash:
		if !v.IsError {
			t.Errorf("FlashMsg for unknown-bucket drill must have IsError=true; got false. Text=%q", v.Text)
		}
		// Error flash is acceptable — the error was handled and surfaced.
	default:
		t.Logf("unknown-bucket drill produced %T — acceptable if an error is surfaced upstream", childMsg)
	}
}

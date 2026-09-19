package unit

// Partial-success contract on the paginated-fetch dispatch path. A throttled
// per-item call must not empty a list whose other pages loaded:
//   - Hard failure (no Resources, err != nil)              → APIErrorMsg.
//   - Soft failure (Resources non-empty AND err != nil)    → ResourcesLoadedMsg
//     with Err set; the handler routes Err through FlashMsg → errorHistory.
//   - Success (Resources non-empty, err == nil)            → ResourcesLoadedMsg
//     with Err == nil.

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// drainAllMessages recursively walks a tea.Cmd's emitted message tree and
// returns every leaf tea.Msg. Handles arbitrarily-nested BatchMsg.
func drainAllMessages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	var out []tea.Msg
	var walk func(msg tea.Msg)
	walk = func(msg tea.Msg) {
		if msg == nil {
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				if c == nil {
					continue
				}
				walk(c())
			}
			return
		}
		out = append(out, msg)
	}
	walk(cmd())
	return out
}

// When ResourcesLoadedMsg.Err is set, the handler returns a FlashMsg routed to
// errorHistory (the `!` log) and keeps the partial Resources. Every dispatcher
// branch (fetchResources / fetchMoreResources*) funnels a non-empty partial
// result into ResourcesLoadedMsg{Err:...}, so the handler is the join point.
func TestDispatcher_PartialSuccess_HandlerEmitsFlashMsg(t *testing.T) {
	m := newBlessedModel(t, "test-profile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	partialErr := errors.New("partial: 1 of 3 IDs failed: throttled")
	_, cmd := rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "test-handler-route",
		Resources: []resource.Resource{
			{ID: "managed-001", Name: "managed-001"},
			{ID: "managed-002", Name: "managed-002"},
		},
		Err: partialErr,
	})

	if cmd == nil {
		t.Fatal("handler must emit a Cmd that yields the FlashMsg for the partial-success error")
	}
	msgs := drainAllMessages(cmd)
	var flash *messages.Flash
	for i := range msgs {
		if fm, ok := msgs[i].(messages.Flash); ok {
			flash = &fm
			break
		}
	}
	if flash == nil {
		t.Fatalf("expected FlashMsg in handler output; got %d messages", len(msgs))
	}
	if !flash.IsError {
		t.Errorf("FlashMsg.IsError = false, want true (partial-fetch err must be error-routed)")
	}
	if !strings.Contains(flash.Text, "partial: 1 of 3 IDs failed") {
		t.Errorf("FlashMsg.Text = %q, want it to wrap the composite error", flash.Text)
	}
	if !strings.Contains(flash.Text, "test-handler-route") {
		t.Errorf("FlashMsg.Text = %q, want it to name the resource type", flash.Text)
	}
}

// Compile-time pin: ResourcesLoadedMsg carries resources and an error together,
// which the partial-success contract depends on.
func TestResourcesLoadedMsg_HasErrField(t *testing.T) {
	msg := messages.ResourcesLoaded{Provenance: messages.FetchProvenanceUnknown,
		Err: errors.New("compile-time pin"),
	}
	if msg.Err == nil {
		t.Error("ResourcesLoadedMsg must carry an Err field for partial-success surfacing")
	}
}

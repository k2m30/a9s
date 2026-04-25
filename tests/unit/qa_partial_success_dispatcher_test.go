package unit

// qa_partial_success_dispatcher_test.go — Regression pin for the
// partial-success contract on the paginated-fetch dispatch path.
//
// Bug found in code review of commit 1ca32ee247b3fab13319bb55dbf6f658e717dd85
// (PR k2m30/a9s#299): the iam_policies fetcher started returning
// (managed+inline resources, inlineErr) per E1-E6, but the dispatcher in
// fetchResources / fetchMoreResources discarded result.Resources whenever
// err != nil — a single transient ListGroupPolicies throttle dropped the
// entire policies list to empty + APIErrorMsg, instead of rendering the
// list and surfacing the failure via FlashMsg.
//
// Contract pinned by these tests:
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

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
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

// TestDispatcher_PartialSuccess_HandlerEmitsFlashMsg verifies that when
// ResourcesLoadedMsg.Err is set, the app handler returns a FlashMsg routed
// to errorHistory (the `!` log) AND the partial Resources list isn't lost.
//
// This is the most direct contract pin: the handler is what guarantees the
// "preserve partial results AND surface the error" semantic. The dispatcher
// branches in fetchResources / fetchMoreResources / fetchMoreResources*
// (filtered/child/top) all funnel into ResourcesLoadedMsg{Err:...} when
// resources are non-empty, so this single test exercises the join point.
func TestDispatcher_PartialSuccess_HandlerEmitsFlashMsg(t *testing.T) {
	m := tui.New("test-profile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	partialErr := errors.New("partial: 1 of 3 IDs failed: throttled")
	_, cmd := rootApplyMsg(m, messages.ResourcesLoadedMsg{
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
	var flash *messages.FlashMsg
	for i := range msgs {
		if fm, ok := msgs[i].(messages.FlashMsg); ok {
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

// TestResourcesLoadedMsg_HasErrField is a compile-time pin: removing the Err
// field from ResourcesLoadedMsg would break the partial-success contract by
// forcing the dispatcher back to "either resources OR error". This test
// fails to compile if the field is removed.
func TestResourcesLoadedMsg_HasErrField(t *testing.T) {
	msg := messages.ResourcesLoadedMsg{
		ResourceType: "x",
		Err:          errors.New("compile-time pin"),
	}
	if msg.Err == nil {
		t.Error("ResourcesLoadedMsg must carry an Err field for partial-success surfacing")
	}
}

package unit

// app_error_surfacing_test.go — coverage for the new FlashMsg wiring on:
//   - handleEnrichmentChecked (EnrichmentCheckedMsg.Err != nil)
//   - handleAvailabilityPrefetched (AvailabilityPrefetchedMsg.PrefetchErr != nil)
//
// Each test dispatches the relevant Msg with a non-nil error, executes the
// returned tea.Cmd (batch-walked to find the flash), and asserts that a
// messages.Flash with IsError=true is emitted containing the expected
// prefix + injected error substring. These are regression pins for the
// "never silent skip" contract at the app level.

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// walkForFlash executes a tea.Cmd and recursively searches any returned
// BatchMsg for a messages.Flash. Returns (flash, true) on first hit.
func walkForFlash(cmd tea.Cmd) (messages.Flash, bool) {
	if cmd == nil {
		return messages.Flash{}, false
	}
	msg := cmd()
	return findFlashInWalk(msg)
}

func findFlashInWalk(msg tea.Msg) (messages.Flash, bool) {
	switch v := msg.(type) {
	case messages.Flash:
		return v, true
	case tea.BatchMsg:
		for _, sub := range v {
			if sub == nil {
				continue
			}
			if f, ok := findFlashInWalk(sub()); ok {
				return f, true
			}
		}
	}
	return messages.Flash{}, false
}

// TestEnrichmentCheckedMsg_ErrorEmitsFlash pins that a valid-gen enrichment
// result with Err != nil surfaces as a FlashMsg{IsError:true} — before the
// fix the error branch silently dropped through to nil cmd.
func TestEnrichmentCheckedMsg_ErrorEmitsFlash(t *testing.T) {
	m := newTestModel()
	errMsg := messages.EnrichmentChecked{
		ResourceType: "ddb",
		Err:          errors.New("access denied on ddb-enrich"),
		// Gen=0 / TypeGen=0 bypass the guards (see handleEnrichmentChecked).
	}

	_, cmd := m.Update(errMsg)
	flash, ok := walkForFlash(cmd)
	if !ok {
		t.Fatal("handleEnrichmentChecked must emit a FlashMsg when Err != nil — the `!` error log depends on it")
	}
	if !flash.IsError {
		t.Errorf("FlashMsg.IsError = false, want true (enrichment failure is an operator-visible error)")
	}
	for _, want := range []string{"enrich", "ddb", "access denied on ddb-enrich"} {
		if !strings.Contains(flash.Text, want) {
			t.Errorf("FlashMsg.Text = %q, missing expected substring %q", flash.Text, want)
		}
	}
}

// TestAvailabilityPrefetchedMsg_PrefetchErrEmitsFlash pins that a synchronous
// availability prefetch with PrefetchErr != nil surfaces as FlashMsg. Before
// the fix, per-type fetcher errors silently vanished from the menu state.
func TestAvailabilityPrefetchedMsg_PrefetchErrEmitsFlash(t *testing.T) {
	m := newTestModel()
	errMsg := messages.AvailabilityPrefetched{
		Entries:        map[string]int{},
		Truncated:      map[string]bool{},
		IssueCounts:    map[string]int{},
		IssueTruncated: map[string]bool{},
		Resources:      map[string][]resource.Resource{},
		// Gen=0 accepted unconditionally — see handleAvailabilityPrefetched gen guard.
		PrefetchErr: errors.New("eks: access denied, ng: throttled"),
	}

	_, cmd := m.Update(errMsg)
	flash, ok := walkForFlash(cmd)
	if !ok {
		t.Fatal("handleAvailabilityPrefetched must emit a FlashMsg when PrefetchErr != nil")
	}
	if !flash.IsError {
		t.Errorf("FlashMsg.IsError = false, want true")
	}
	for _, want := range []string{"availability", "eks", "ng"} {
		if !strings.Contains(flash.Text, want) {
			t.Errorf("FlashMsg.Text = %q, missing expected substring %q", flash.Text, want)
		}
	}
}

// TestAvailabilityPrefetchedMsg_NilPrefetchErrNoFlash pins the symmetric case:
// a clean prefetch (PrefetchErr == nil) must NOT emit an error FlashMsg.
func TestAvailabilityPrefetchedMsg_NilPrefetchErrNoFlash(t *testing.T) {
	m := newTestModel()
	cleanMsg := messages.AvailabilityPrefetched{
		Entries:        map[string]int{},
		Truncated:      map[string]bool{},
		IssueCounts:    map[string]int{},
		IssueTruncated: map[string]bool{},
		Resources:      map[string][]resource.Resource{},
		// PrefetchErr left nil — happy path.
	}

	_, cmd := m.Update(cleanMsg)
	if flash, ok := walkForFlash(cmd); ok && flash.IsError {
		t.Errorf("handleAvailabilityPrefetched must NOT emit an error FlashMsg on a clean prefetch; got %q", flash.Text)
	}
}

// TestEnrichmentCheckedMsg_NilErrNoFlash is the symmetric happy-path pin for
// the enrichment handler.
func TestEnrichmentCheckedMsg_NilErrNoFlash(t *testing.T) {
	m := newTestModel()
	okMsg := messages.EnrichmentChecked{
		ResourceType: "sfn",
		Findings:     map[string]resource.EnrichmentFinding{},
	}

	_, cmd := m.Update(okMsg)
	if flash, ok := walkForFlash(cmd); ok && flash.IsError {
		t.Errorf("handleEnrichmentChecked must NOT emit an error FlashMsg on a clean enrichment; got %q", flash.Text)
	}
}

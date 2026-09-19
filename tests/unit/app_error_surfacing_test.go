package unit

// handleEnrichmentChecked and handleAvailabilityPrefetched surface a non-nil
// error as a messages.Flash with IsError=true; an error is never skipped
// silently.

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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

func TestEnrichmentCheckedMsg_ErrorEmitsFlash(t *testing.T) {
	m := newTestModel(t)
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

func TestAvailabilityPrefetchedMsg_PrefetchErrEmitsFlash(t *testing.T) {
	m := newTestModel(t)
	errMsg := messages.AvailabilityPrefetched{
		Entries:        map[string]int{},
		Truncated:      map[string]bool{},
		IssueCounts:    map[string]int{},
		IssueTruncated: map[string]bool{},
		Resources:      map[string][]resource.Resource{},
		// Stamp the live AvailabilityGen — the staleness guard
		// drops zero-stamped prefetches (AcceptZeroGen=false).
		Gen:         m.Core().Session().AvailabilityGen,
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

func TestAvailabilityPrefetchedMsg_NilPrefetchErrNoFlash(t *testing.T) {
	m := newTestModel(t)
	cleanMsg := messages.AvailabilityPrefetched{
		Entries:        map[string]int{},
		Truncated:      map[string]bool{},
		IssueCounts:    map[string]int{},
		IssueTruncated: map[string]bool{},
		Resources:      map[string][]resource.Resource{},
		// Stamp the live AvailabilityGen — the staleness guard
		// drops zero-stamped prefetches (AcceptZeroGen=false).
		Gen: m.Core().Session().AvailabilityGen,
	}

	_, cmd := m.Update(cleanMsg)
	if flash, ok := walkForFlash(cmd); ok && flash.IsError {
		t.Errorf("handleAvailabilityPrefetched must NOT emit an error FlashMsg on a clean prefetch; got %q", flash.Text)
	}
}

func TestEnrichmentCheckedMsg_NilErrNoFlash(t *testing.T) {
	m := newTestModel(t)
	okMsg := messages.EnrichmentChecked{
		ResourceType: "sfn",
		Findings:     map[string][]domain.Finding{},
	}

	_, cmd := m.Update(okMsg)
	if flash, ok := walkForFlash(cmd); ok && flash.IsError {
		t.Errorf("handleEnrichmentChecked must NOT emit an error FlashMsg on a clean enrichment; got %q", flash.Text)
	}
}

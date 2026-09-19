package unit

// Core.HandleRelatedCheckResult (core/runtime/handlers_resources.go) turns a
// non-nil LazyAddError into a FlashIntent. The TUI's RelatedCheckResult case
// forwards only the returned TaskRequests, so the flash is asserted on the
// rendered view rather than on m.flash or a returned cmd.

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// A FetchByIDs error with no partial results surfaces as an error flash
// carrying "related-fetch" and the error text.
func TestLazyAddError_EmitsFlashMsg(t *testing.T) {
	const (
		srcType    = "test-flash-error-source"
		targetType = "test-flash-error-target"
	)

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Flash Error Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, []string{"id-boom-001", "id-boom-002"}, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		return nil, errors.New("simulated boom")
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-flash-001"}

	m, startCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, startCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received from opening srcType detail")
	}
	if resultMsg.LazyAddError == nil {
		t.Fatal("expected LazyAddError to be set when FetchByIDs returns an error")
	}

	m, _ = rootApplyMsg(m, resultMsg)

	view := stripANSI(rootViewContent(m))
	for _, want := range []string{"related-fetch", "simulated boom"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected view to show flash containing %q, got:\n%s", want, view)
		}
	}
}

// FetchByIDs returning (partialResults, error) both merges the partial
// results and surfaces the error flash.
func TestLazyAddError_PartialSuccess_StillEmitsFlashMsg(t *testing.T) {
	const (
		srcType    = "test-flash-partial-source"
		targetType = "test-flash-partial-target"
	)

	resource.SetRelatedForTest(srcType, []resource.RelatedDef{
		{
			TargetType:       targetType,
			DisplayName:      "Flash Partial Test Target",
			NeedsTargetCache: false,
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.KnownRelated(targetType, []string{"id-ok", "id-bad"}, false)
			},
		},
	})

	resource.SetFetchByIDsForTest(targetType, func(_ context.Context, _ any, _ []string) ([]resource.Resource, error) {
		partial := []resource.Resource{{ID: "id-ok", Name: "ok-resource"}}
		return partial, errors.New("id-bad: denied")
	})

	t.Cleanup(func() {
		resource.CleanupRelatedForTest(srcType)
		resource.CleanupFetchByIDsForTest(targetType)
	})

	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	srcRes := resource.Resource{ID: "src-partial-001"}

	m, startCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: srcType,
		Resource:     &srcRes,
	})

	resultMsg, found := collectRelatedResult(t, startCmd)
	if !found {
		t.Fatal("no RelatedCheckResultMsg received from opening srcType detail")
	}
	if resultMsg.LazyAddError == nil {
		t.Fatal("expected LazyAddError to be set for partial FetchByIDs failure")
	}

	lazySlice := resultMsg.LazyAddedResources[targetType]
	if len(lazySlice) != 1 || lazySlice[0].ID != "id-ok" {
		t.Errorf("LazyAddedResources[%s] = %v, want 1 resource with ID=id-ok", targetType, lazySlice)
	}

	m, _ = rootApplyMsg(m, resultMsg)

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "id-bad") {
		t.Errorf("expected view to show flash containing %q, got:\n%s", "id-bad", view)
	}
	if !strings.Contains(view, "related-fetch") {
		t.Errorf("expected view to show flash containing %q, got:\n%s", "related-fetch", view)
	}
}

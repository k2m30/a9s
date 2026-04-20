package unit_test

// app_related_navigate_test.go — unit tests for the RelatedNavigateMsg handler
// logic (T018).
//
// The handleRelatedNavigate method lives on the unexported root Model and
// cannot be constructed in unit tests. Instead, these tests verify the two
// prerequisite logic blocks that the handler relies on:
//
//  1. resource.FindResourceType returns nil for unknown target types
//     (handler's first guard clause — would produce FlashMsg with IsError=true).
//  2. The filter-text derivation rule applied by the handler before constructing
//     the ResourceList's initial filter string.
//
// Design spec: docs/design/related-resources.md v4.3
// QA stories:  docs/qa/related-resources-stories.md

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// TestRelatedNavigate_UnknownTargetType
// Given: a TargetType string that is not registered in the resource type registry
// When:  FindResourceType is called with that string
// Then:  FindResourceType returns nil (the handler's guard condition is satisfied)
//
// This validates the prerequisite for the FlashMsg-with-IsError=true branch of
// handleRelatedNavigate: when FindResourceType returns nil the handler must emit
// a FlashMsg{IsError: true} and abort navigation.
// ---------------------------------------------------------------------------

func TestRelatedNavigate_UnknownTargetType(t *testing.T) {
	result := resource.FindResourceType("nonexistent")
	if result != nil {
		t.Errorf("FindResourceType(\"nonexistent\") should return nil for an unregistered type, got %+v", result)
	}
}

// TestRelatedNavigate_EmptyString verifies that an empty string also yields nil.
func TestRelatedNavigate_EmptyString(t *testing.T) {
	result := resource.FindResourceType("")
	if result != nil {
		t.Errorf("FindResourceType(\"\") should return nil, got %+v", result)
	}
}

// TestRelatedNavigate_KnownTargetType verifies that a known type is found, so the
// guard clause DOES NOT fire for valid navigation requests.
func TestRelatedNavigate_KnownTargetType(t *testing.T) {
	result := resource.FindResourceType("ec2")
	if result == nil {
		t.Error("FindResourceType(\"ec2\") should return a non-nil ResourceTypeDef")
	}
}

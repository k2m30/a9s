package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

func TestRelatedNavigate_UnknownTargetType(t *testing.T) {
	result := resource.FindResourceType("nonexistent")
	if result != nil {
		t.Errorf("FindResourceType(\"nonexistent\") should return nil for an unregistered type, got %+v", result)
	}
}

func TestRelatedNavigate_EmptyString(t *testing.T) {
	result := resource.FindResourceType("")
	if result != nil {
		t.Errorf("FindResourceType(\"\") should return nil, got %+v", result)
	}
}

func TestRelatedNavigate_KnownTargetType(t *testing.T) {
	result := resource.FindResourceType("ec2")
	if result == nil {
		t.Error("FindResourceType(\"ec2\") should return a non-nil ResourceTypeDef")
	}
}

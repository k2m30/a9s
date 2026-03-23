package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ===========================================================================
// Step 3: EnterChildViewMsg and LoadResourcesMsg.ParentContext
// ===========================================================================

func TestEnterChildViewMsg_Fields(t *testing.T) {
	msg := messages.EnterChildViewMsg{
		ChildType:     "s3_objects",
		ParentContext: map[string]string{"bucket": "my-bucket", "prefix": "data/"},
		DisplayName:   "my-bucket",
	}

	if msg.ChildType != "s3_objects" {
		t.Errorf("ChildType = %q, want %q", msg.ChildType, "s3_objects")
	}
	if msg.ParentContext["bucket"] != "my-bucket" {
		t.Errorf("ParentContext[bucket] = %q, want %q", msg.ParentContext["bucket"], "my-bucket")
	}
	if msg.ParentContext["prefix"] != "data/" {
		t.Errorf("ParentContext[prefix] = %q, want %q", msg.ParentContext["prefix"], "data/")
	}
	if msg.DisplayName != "my-bucket" {
		t.Errorf("DisplayName = %q, want %q", msg.DisplayName, "my-bucket")
	}
}

func TestEnterChildViewMsg_R53(t *testing.T) {
	msg := messages.EnterChildViewMsg{
		ChildType:     "r53_records",
		ParentContext: map[string]string{"zone_id": "/hostedzone/Z123", "zone_name": "example.com."},
		DisplayName:   "example.com.",
	}

	if msg.ChildType != "r53_records" {
		t.Errorf("ChildType = %q, want %q", msg.ChildType, "r53_records")
	}
	if msg.ParentContext["zone_id"] != "/hostedzone/Z123" {
		t.Errorf("ParentContext[zone_id] = %q, want %q", msg.ParentContext["zone_id"], "/hostedzone/Z123")
	}
	if msg.DisplayName != "example.com." {
		t.Errorf("DisplayName = %q, want %q", msg.DisplayName, "example.com.")
	}
}

func TestLoadResourcesMsg_ParentContext(t *testing.T) {
	msg := messages.LoadResourcesMsg{
		ResourceType:  "s3_objects",
		ParentContext: map[string]string{"bucket": "test-bucket"},
	}

	if msg.ResourceType != "s3_objects" {
		t.Errorf("ResourceType = %q, want %q", msg.ResourceType, "s3_objects")
	}
	if msg.ParentContext["bucket"] != "test-bucket" {
		t.Errorf("ParentContext[bucket] = %q, want %q", msg.ParentContext["bucket"], "test-bucket")
	}
}

func TestLoadResourcesMsg_ParentContextOnly(t *testing.T) {
	// LoadResourcesMsg now only has ResourceType and ParentContext
	msg := messages.LoadResourcesMsg{
		ResourceType:  "s3_objects",
		ParentContext: map[string]string{"bucket": "my-bucket", "prefix": "data/"},
	}

	if msg.ResourceType != "s3_objects" {
		t.Errorf("ResourceType = %q, want %q", msg.ResourceType, "s3_objects")
	}
	if msg.ParentContext["bucket"] != "my-bucket" {
		t.Errorf("ParentContext[bucket] = %q, want %q", msg.ParentContext["bucket"], "my-bucket")
	}
	if msg.ParentContext["prefix"] != "data/" {
		t.Errorf("ParentContext[prefix] = %q, want %q", msg.ParentContext["prefix"], "data/")
	}
}

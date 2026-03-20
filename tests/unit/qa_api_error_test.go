package unit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/smithy-go"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui"
	"github.com/k2m30/a9s/internal/tui/messages"
)

// ══════════════════════════════════════════════════════════════════════════════
// TASK-001: Handle APIErrorMsg in root Update
// ══════════════════════════════════════════════════════════════════════════════

// TestQA_APIError_FlashShown verifies that when an APIErrorMsg arrives,
// a flash error is displayed in the header.
func TestQA_APIError_FlashShown(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to EC2 resource list (loading state)
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Send APIErrorMsg — simulating a failed AWS call
	m, _ = rootApplyMsg(m, messages.APIErrorMsg{
		ResourceType: "ec2",
		Err:          fmt.Errorf("operation error EC2: DescribeInstances, access denied"),
	})

	plain := stripANSI(rootViewContent(m))

	// The flash error should contain something about the error
	if !strings.Contains(plain, "access denied") && !strings.Contains(plain, "error") && !strings.Contains(plain, "Error") {
		t.Errorf("after APIErrorMsg, header should show error flash, got: %s", plain[:min(300, len(plain))])
	}
}

// TestQA_APIError_ClearsLoading verifies that after APIErrorMsg,
// the resource list is no longer in loading state (no spinner).
func TestQA_APIError_ClearsLoading(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to RDS resource list (loading state)
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})

	// Confirm it's loading
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Loading") {
		t.Errorf("before APIErrorMsg, should show Loading, got: %s", plain[:min(200, len(plain))])
	}

	// Send APIErrorMsg
	m, _ = rootApplyMsg(m, messages.APIErrorMsg{
		ResourceType: "dbi",
		Err:          fmt.Errorf("connection timeout"),
	})

	// Should NOT show "Loading..." anymore
	plain = stripANSI(rootViewContent(m))
	if strings.Contains(plain, "Loading") {
		t.Errorf("after APIErrorMsg, should NOT show Loading, got: %s", plain[:min(200, len(plain))])
	}
}

// TestQA_APIError_ClassifyAWSError_ExpiredToken verifies user-friendly messages
// for classified AWS errors like ExpiredToken.
func TestQA_APIError_ClassifyAWSError_ExpiredToken(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	// smithy.APIError for expired token
	apiErr := &smithy.GenericAPIError{
		Code:    "ExpiredToken",
		Message: "The security token included in the request is expired",
	}
	m, _ = rootApplyMsg(m, messages.APIErrorMsg{
		ResourceType: "s3",
		Err:          apiErr,
	})

	plain := stripANSI(rootViewContent(m))

	// Should show the classified error code or message
	if !strings.Contains(plain, "ExpiredToken") && !strings.Contains(plain, "expired") {
		t.Errorf("expired token error should show classified message, got: %s", plain[:min(300, len(plain))])
	}
}

// TestQA_APIError_ClassifyAWSError_AccessDenied verifies user-friendly messages
// for AccessDenied errors.
func TestQA_APIError_ClassifyAWSError_AccessDenied(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	apiErr := &smithy.GenericAPIError{
		Code:    "AccessDenied",
		Message: "User is not authorized to perform this operation",
	}
	m, _ = rootApplyMsg(m, messages.APIErrorMsg{
		ResourceType: "ec2",
		Err:          apiErr,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "AccessDenied") && !strings.Contains(plain, "not authorized") {
		t.Errorf("access denied error should show classified message, got: %s", plain[:min(300, len(plain))])
	}
}

// TestQA_APIError_NonAWSError verifies that non-AWS errors still show a flash.
func TestQA_APIError_NonAWSError(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})

	m, _ = rootApplyMsg(m, messages.APIErrorMsg{
		ResourceType: "redis",
		Err:          fmt.Errorf("AWS clients not initialized"),
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "AWS clients not initialized") {
		t.Errorf("non-AWS error should still show message in flash, got: %s", plain[:min(300, len(plain))])
	}
}

// TestQA_APIError_AllResourceTypes verifies the handler works for all resource types.
func TestQA_APIError_AllResourceTypes(t *testing.T) {
	resourceTypes := resource.AllShortNames()

	for _, rt := range resourceTypes {
		t.Run(rt, func(t *testing.T) {
			tui.Version = "test"
			m := newRootSizedModel()

			m, _ = rootApplyMsg(m, messages.NavigateMsg{
				Target:       messages.TargetResourceList,
				ResourceType: rt,
			})

			m, _ = rootApplyMsg(m, messages.APIErrorMsg{
				ResourceType: rt,
				Err:          fmt.Errorf("test error for %s", rt),
			})

			plain := stripANSI(rootViewContent(m))

			// Should show error, not loading
			if strings.Contains(plain, "Loading") {
				t.Errorf("[%s] after APIErrorMsg, should NOT show Loading", rt)
			}
		})
	}
}

// TestQA_APIError_OnMainMenu verifies APIErrorMsg doesn't crash when
// the active view is the main menu (not a resource list).
func TestQA_APIError_OnMainMenu(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Send APIErrorMsg while on main menu — should not panic
	m, _ = rootApplyMsg(m, messages.APIErrorMsg{
		ResourceType: "ec2",
		Err:          fmt.Errorf("something went wrong"),
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "something went wrong") {
		t.Errorf("APIErrorMsg on main menu should still show flash error, got: %s", plain[:min(300, len(plain))])
	}
}

// ══════════════════════════════════════════════════════════════════════════════
// TASK-003: Fix S3 object refresh bug
// ══════════════════════════════════════════════════════════════════════════════

// TestBug_S3Refresh_InsideBucket verifies that Ctrl+R inside a bucket
// refreshes objects for the same bucket, not the bucket list.
func TestBug_S3Refresh_InsideBucket(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to S3 buckets
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	buckets := []resource.Resource{
		{ID: "my-data-bucket", Name: "my-data-bucket", Fields: map[string]string{"name": "my-data-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: buckets})

	// Enter bucket
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Load objects
	objects := []resource.Resource{
		{ID: "file1.txt", Name: "file1.txt", Status: "", Fields: map[string]string{
			"key": "file1.txt", "size": "1024", "last_modified": "2025-01-01", "storage_class": "STANDARD",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: objects})

	// Verify we're in the objects view
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "my-data-bucket") {
		t.Fatalf("should be in objects view for my-data-bucket, got: %s", plain[:min(200, len(plain))])
	}

	// Press Ctrl+R to refresh
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+R should return a command to refresh")
	}

	// Execute the returned command and check the message.
	msg := cmd()
	// The refresh command should fetch S3 objects for the bucket (type "s3" with bucket param),
	// NOT an unsupported resource type like "s3_objects" with empty bucket.
	// Since we have no real AWS clients, it returns APIErrorMsg.
	switch msg := msg.(type) {
	case messages.APIErrorMsg:
		// Must NOT get "unsupported resource type: s3_objects" -- that means bucket was lost
		if strings.Contains(msg.Err.Error(), "unsupported resource type") {
			t.Errorf("refresh inside bucket should fetch s3 objects, not unsupported type; err: %v", msg.Err)
		}
		if msg.ResourceType != "s3" {
			t.Errorf("refresh should fetch resource type 's3', got %q", msg.ResourceType)
		}
	case messages.ResourcesLoadedMsg:
		if msg.ResourceType != "s3" {
			t.Errorf("refresh should load s3, got resource type %q", msg.ResourceType)
		}
	default:
		t.Logf("refresh returned message type %T", msg)
	}

	// After refresh, verify that we still see the bucket name in the view
	// (i.e., we didn't navigate away to the bucket list)
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "my-data-bucket") {
		t.Errorf("after refresh, should still be in objects view for my-data-bucket, got: %s", plain[:min(200, len(plain))])
	}
}

// TestBug_S3Refresh_InsidePrefix verifies that Ctrl+R inside a prefix
// refreshes objects for the correct bucket+prefix, not the bucket list.
func TestBug_S3Refresh_InsidePrefix(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to S3 buckets
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	buckets := []resource.Resource{
		{ID: "deep-bucket", Name: "deep-bucket", Fields: map[string]string{"name": "deep-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: buckets})

	// Enter bucket
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Load objects including a folder
	objects := []resource.Resource{
		{ID: "data/", Name: "data/", Status: "folder", Fields: map[string]string{
			"key": "data/", "size": "", "last_modified": "", "storage_class": "",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: objects})

	// Navigate into the prefix
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Load objects inside the prefix
	prefixObjects := []resource.Resource{
		{ID: "data/file.csv", Name: "data/file.csv", Status: "", Fields: map[string]string{
			"key": "data/file.csv", "size": "2048", "last_modified": "2025-02-01", "storage_class": "STANDARD",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: prefixObjects})

	// Press Ctrl+R to refresh
	_, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+R inside prefix should return a command to refresh")
	}

	// The refresh should fetch S3 objects for the correct bucket (type "s3" with bucket param),
	// NOT "s3_objects" with empty bucket.
	msg := cmd()
	switch msg := msg.(type) {
	case messages.APIErrorMsg:
		if strings.Contains(msg.Err.Error(), "unsupported resource type") {
			t.Errorf("refresh inside prefix should fetch s3 objects, not unsupported type; err: %v", msg.Err)
		}
		if msg.ResourceType != "s3" {
			t.Errorf("refresh inside prefix should fetch resource type 's3', got %q", msg.ResourceType)
		}
	case messages.ResourcesLoadedMsg:
		if msg.ResourceType != "s3" {
			t.Errorf("refresh inside prefix should load s3, got resource type %q", msg.ResourceType)
		}
	default:
		t.Logf("refresh returned message type %T", msg)
	}
}

// TestBug_S3Refresh_BucketListLevel verifies that Ctrl+R at the bucket list
// level still refreshes buckets correctly.
func TestBug_S3Refresh_BucketListLevel(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to S3 buckets
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	buckets := []resource.Resource{
		{ID: "bucket-a", Name: "bucket-a", Fields: map[string]string{"name": "bucket-a"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: buckets})

	// Press Ctrl+R to refresh at bucket level
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+R at bucket list level should return a command")
	}

	msg := cmd()
	switch msg := msg.(type) {
	case messages.APIErrorMsg:
		if msg.ResourceType != "s3" {
			t.Errorf("refresh at bucket list should fetch s3, got %q", msg.ResourceType)
		}
	default:
		t.Logf("refresh returned message type %T", msg)
	}
}

// TestBug_S3Refresh_NonS3ResourceUnaffected verifies that refresh on
// non-S3 resource types is not broken by the S3 fix.
func TestBug_S3Refresh_NonS3ResourceUnaffected(t *testing.T) {
	for _, rt := range resource.AllShortNames() {
		if rt == "s3" {
			continue
		}
		t.Run(rt, func(t *testing.T) {
			tui.Version = "test"
			m := newRootSizedModel()

			m, _ = rootApplyMsg(m, messages.NavigateMsg{
				Target:       messages.TargetResourceList,
				ResourceType: rt,
			})
			m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
				ResourceType: rt,
				Resources: []resource.Resource{
					{ID: "test-resource", Name: "test-resource", Fields: map[string]string{}},
				},
			})

			_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
			if cmd == nil {
				t.Fatalf("[%s] Ctrl+R should return a command", rt)
			}

			msg := cmd()
			switch msg := msg.(type) {
			case messages.APIErrorMsg:
				if msg.ResourceType != rt {
					t.Errorf("[%s] refresh should fetch %s, got %q", rt, rt, msg.ResourceType)
				}
			default:
				t.Logf("[%s] refresh returned message type %T", rt, msg)
			}
		})
	}
}

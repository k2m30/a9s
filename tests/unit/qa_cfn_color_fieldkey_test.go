package unit

// CFN Color reads the stack status from
// r.Fields["status"].

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

func cfnResource(fieldKey, status string) resource.Resource {
	return resource.Resource{
		ID:     "arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack/abc123",
		Name:   "my-stack",
		Fields: map[string]string{fieldKey: status},
	}
}

// TestCFNColor_CreateFailed_IsColorBroken verifies CREATE_FAILED → ColorBroken.
func TestCFNColor_CreateFailed_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("cfn")
	r := cfnResource("status", "CREATE_FAILED")
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("cfn Color with Fields[status]=CREATE_FAILED = %v, want ColorBroken", got)
	}
}

// TestCFNColor_CreateComplete_IsColorHealthy verifies CREATE_COMPLETE → ColorHealthy.
func TestCFNColor_CreateComplete_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("cfn")
	r := cfnResource("status", "CREATE_COMPLETE")
	got := td.Color(r)
	if got != resource.ColorHealthy {
		t.Errorf("cfn Color with Fields[status]=CREATE_COMPLETE = %v, want ColorHealthy", got)
	}
}

// TestCFNColor_UpdateInProgress_IsColorWarning verifies UPDATE_IN_PROGRESS → ColorWarning.
func TestCFNColor_UpdateInProgress_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("cfn")
	r := cfnResource("status", "UPDATE_IN_PROGRESS")
	got := td.Color(r)
	if got != resource.ColorWarning {
		t.Errorf("cfn Color with Fields[status]=UPDATE_IN_PROGRESS = %v, want ColorWarning", got)
	}
}

// TestCFNColor_OldWrongKey_DoesNotProduceBroken pins that "stack_status" is
// not the field key: a CREATE_FAILED value under "stack_status" must not
// produce ColorBroken.
func TestCFNColor_OldWrongKey_DoesNotProduceBroken(t *testing.T) {
	td := resource.FindResourceType("cfn")
	r := cfnResource("stack_status", "CREATE_FAILED")
	got := td.Color(r)
	if got == resource.ColorBroken {
		t.Errorf("cfn Color with Fields[stack_status]=CREATE_FAILED = ColorBroken — this means Color is reading the wrong field key 'stack_status' instead of 'status'")
	}
}

// TestCFNColor_DeleteComplete_IsColorDim verifies DELETE_COMPLETE → ColorDim.
func TestCFNColor_DeleteComplete_IsColorDim(t *testing.T) {
	td := resource.FindResourceType("cfn")
	r := cfnResource("status", "DELETE_COMPLETE")
	got := td.Color(r)
	if got != resource.ColorDim {
		t.Errorf("cfn Color with Fields[status]=DELETE_COMPLETE = %v, want ColorDim", got)
	}
}

// TestCFNColor_RollbackComplete_IsColorBroken verifies ROLLBACK_COMPLETE → ColorBroken.
func TestCFNColor_RollbackComplete_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("cfn")
	r := cfnResource("status", "ROLLBACK_COMPLETE")
	got := td.Color(r)
	if got != resource.ColorBroken {
		t.Errorf("cfn Color with Fields[status]=ROLLBACK_COMPLETE = %v, want ColorBroken", got)
	}
}

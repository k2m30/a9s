package unit_test

import (
	"testing"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func t578Row(t *testing.T, rows []resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("demo row %s not found", id)
	return resource.Resource{}
}

func t578Has(r resource.Resource, code domain.FindingCode) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// The paused demo cluster shows its state in the Status column and renders
// Dim (docs/resources/redshift.md §5), not as a blank healthy row.
func TestT578Demo_RedshiftPausedClusterRendersDim(t *testing.T) {
	td, rows := t563Merged(t, "redshift")
	r := t578Row(t, rows, fixtures.StagingDwhID)
	if got := r.Fields["status"]; got != "paused" {
		t.Errorf("%s Fields[status] = %q, want the AWS value %q", r.ID, got, "paused")
	}
	if top := t563AssertWitnessRendered(t, td, rows, r); top.Severity != domain.SevDim {
		t.Errorf("%s top finding %s severity = %v, want Dim", r.ID, top.Code, top.Severity)
	}
}

// warn-redshift-multi carries several findings at once, so the Status cell and
// row colour must come from the top one, not whichever was appended first.
func TestT578Demo_RedshiftMultiFindingRowRendersItsTopFinding(t *testing.T) {
	td, rows := t563Merged(t, "redshift")
	r := t578Row(t, rows, fixtures.WarnRedshiftMultiID)
	if len(r.Findings) < 2 {
		t.Fatalf("%s carries %d findings, want at least two", r.ID, len(r.Findings))
	}
	t563AssertWitnessRendered(t, td, rows, r)
}

// acme-legacy-api failed its first create and rolled back; AWS lets it only be
// deleted, so its row shows the rollback and no termination-protection advice.
// A live stack beside it is still judged on protection.
func TestT578Demo_CFNRollbackCompleteStackShowsOnlyItsState(t *testing.T) {
	td, rows := t563Merged(t, "cfn")
	r := t578Row(t, rows, "acme-legacy-api")
	if top := t563AssertWitnessRendered(t, td, rows, r); top.Code != awsclient.CodeCFNStackRollback {
		t.Errorf("%s top finding = %s, want %s", r.ID, top.Code, awsclient.CodeCFNStackRollback)
	}
	for _, row := range rows {
		stack, ok := row.RawStruct.(cfntypes.Stack)
		if ok && stack.StackStatus == cfntypes.StackStatusRollbackComplete && t578Has(row, "cfn.termination-protection-off") {
			t.Errorf("%s (ROLLBACK_COMPLETE) carries cfn.termination-protection-off", row.ID)
		}
	}
}

// A file system that is creating, updating or deleting shows that phase, not
// "no mount targets"; an available one with none still shows the gap.
func TestT578Demo_EFSMountTargetGapOnlyOnAvailable(t *testing.T) {
	td, rows := t563Merged(t, "efs")
	for _, r := range rows {
		fs, ok := r.RawStruct.(efstypes.FileSystemDescription)
		if !ok {
			continue
		}
		switch fs.LifeCycleState {
		case efstypes.LifeCycleStateCreating, efstypes.LifeCycleStateUpdating, efstypes.LifeCycleStateDeleting:
			if t578Has(r, awsclient.CodeEFSNoMountTargets) {
				t.Errorf("%s (%s, %d mount targets) carries no-mount-targets", r.ID, fs.LifeCycleState, fs.NumberOfMountTargets)
			}
		}
	}
	gap := t578Row(t, rows, fixtures.BrokenEFSNoMountTargetsID)
	if top := t563AssertWitnessRendered(t, td, rows, gap); top.Code != awsclient.CodeEFSNoMountTargets {
		t.Errorf("%s top finding = %s, want %s", gap.ID, top.Code, awsclient.CodeEFSNoMountTargets)
	}
}

// Pins the unconditional
// " !N" issue-suffix contract on the CONTROLLER path (Snapshot().FrameTitle /
// Controller.ListFrameTitle()), independent of any TUI adapter call.
//
// The " !N"/" !N+" suffix is UNCONDITIONAL for any list (top-level or
// child), on any renderer, whenever the aggregated issue count N > 0 and the
// list is not in attention-only (ctrl+z) mode. No flag, patch, or adapter
// call gates it; a suffix gated behind a TUI-only flag would render on the
// TUI and never on the web UI for the same count.
//
// This test drives the controller directly (app.New + Apply + the
// ApplyResourcesLoaded test seam — no ResourceListModel, no adapter call
// anywhere) and asserts the suffix is present on Snapshot().FrameTitle,
// which is the exact field the web/headless renderer consumes.
package unit_test

import (
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// controllerIssueResources builds n ec2 rows where exactly wantIssues of them
// are "stopped" and the rest "running".
func controllerIssueResources(n, wantIssues int) []resource.Resource {
	return controllerIssueResourcesWithState("ec2", n, wantIssues, "running", "stopped")
}

// controllerIssueResourcesWithState builds n rows of shortName, the first
// wantIssues in issueState and the rest in healthyState. Each row carries the
// state finding the fetcher emits for its state, which is what its colour and
// the issue badge read.
func controllerIssueResourcesWithState(shortName string, n, wantIssues int, healthyState, issueState string) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range n {
		state := healthyState
		if i < wantIssues {
			state = issueState
		}
		var findings []domain.Finding
		if f, ok := awsclient.StateFinding(shortName, state); ok {
			findings = []domain.Finding{f}
		}
		res[i] = resource.Resource{
			ID:       "i-" + itoaPad(i),
			Name:     "controller-badge-" + itoaPad(i),
			Fields:   map[string]string{"state": state},
			Findings: findings,
		}
	}
	return res
}

// itoaPad avoids importing strconv/fmt twice across this small helper file;
// zero-padding keeps IDs stable and readable for failure messages.
func itoaPad(i int) string {
	digits := "0123456789"
	if i < 10 {
		return "00" + string(digits[i])
	}
	if i < 100 {
		return "0" + string(digits[i/10]) + string(digits[i%10])
	}
	return string(digits[i/100]) + string(digits[(i/10)%10]) + string(digits[i%10])
}

// TestController_ListFrameTitle_IssueBadge_UnconditionalNoAdapterCall opens
// an ec2 list with issue rows via the Controller directly (app.New + Apply +
// ApplyResourcesLoaded), without SetShowIssueBadge, PatchListShowIssueBadge
// or GetListShowIssueBadge. Snapshot().FrameTitle must still carry the " !N"
// suffix, because the contract is unconditional.
func TestController_ListFrameTitle_IssueBadge_UnconditionalNoAdapterCall(t *testing.T) {
	c := newListController(t, "ec2")

	resources := controllerIssueResources(10, 3)
	c.ApplyResourcesLoaded("ec2", resources, nil, false)

	snap := c.Snapshot()
	wantTitle := "ec2(10) !3"
	if snap.FrameTitle != wantTitle {
		t.Errorf("Snapshot().FrameTitle = %q, want %q — the ' !N' issue suffix must render unconditionally on the controller/web path, with no SetShowIssueBadge/Patch call anywhere in this test", snap.FrameTitle, wantTitle)
	}

	// ListFrameTitle() must agree with Snapshot().FrameTitle (single source of truth).
	got := c.ListFrameTitle()
	if got != wantTitle {
		t.Errorf("ListFrameTitle() = %q, want %q", got, wantTitle)
	}
	if !strings.Contains(got, " !3") {
		t.Errorf("ListFrameTitle() = %q; want it to contain the unconditional ' !3' issue suffix", got)
	}
}

// TestController_ListFrameTitle_IssueBadge_ChildList_Unconditional: CHILD
// lists carry the suffix too. It uses the real ScreenChildList seam
// (Controller.PushChildListScreen) rather than a top-level ActionCommand
// push, so it exercises the child-screen path through buildListFrameTitle.
func TestController_ListFrameTitle_IssueBadge_ChildList_Unconditional(t *testing.T) {
	c := newListController(t, "ec2")

	c.PushChildListScreen("ebs")
	resources := controllerIssueResourcesWithState("ebs", 5, 5, "in-use", "error")
	c.ApplyResourcesLoaded("ebs", resources, nil, false)

	snap := c.Snapshot()
	wantTitle := "ebs(5) !5"
	if snap.FrameTitle != wantTitle {
		t.Errorf("Snapshot().FrameTitle = %q, want %q — issue suffix must be unconditional on a ScreenChildList, not just top-level lists", snap.FrameTitle, wantTitle)
	}

	got := c.ListFrameTitle()
	if got != wantTitle {
		t.Errorf("ListFrameTitle() (child list) = %q, want %q", got, wantTitle)
	}
}

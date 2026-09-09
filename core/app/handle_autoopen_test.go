// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestAutoOpenSingleDetail_ByIDPlaceholderOpensDetail verifies the web/headless
// by-ID drill: a placeholder list flagged AutoOpenSingle is replaced by the
// target's detail once its single row loads through Handle (the web/headless
// entry point). Guards against field clicks on by-ID resources (AMI/KMS/policy/
// snapshot IDs) landing on a one-row list instead of the target detail.
func TestAutoOpenSingleDetail_ByIDPlaceholderOpensDetail(t *testing.T) {
	c := New(runtime.New(session.New(), nil))
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenResourceList,
		Context: runtime.ScreenContext{ResourceType: "ec2"},
	}})
	c.ensureListState()
	ls := c.topListState()
	ls.AutoOpenSingle = true
	ls.RelatedIDSet = map[string]struct{}{"i-0target00000001": {}}
	// A by-ID placeholder is never the canonical top-level list — EscPops
	// marks it as such (navigate.go's applyRelatedNavResult sets this same
	// field for every by-ID/filtered/child placeholder it pushes), which is
	// what isTopLevelCanonicalList (list_state.go) keys on to exempt it from
	// the canonical-only Provenance gate (handle.go).
	ls.EscPops = true

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Gen:          c.core.AvailabilityGen(),
		Resources: []resource.Resource{
			{ID: "i-0target00000001", Name: "target", Type: "ec2",
				Fields: map[string]string{"instance_id": "i-0target00000001"}},
		}, Provenance: messages.FetchProvenanceByID,
		ScreenID: c.GetListInstance(),
	})

	snap := c.Snapshot()
	if snap.Body.Kind != BodyKindDetail {
		t.Fatalf("expected detail screen after by-id row loaded, got %q", snap.Body.Kind)
	}
	if got := c.GetDetailResource().ID; got != "i-0target00000001" {
		t.Errorf("detail resource ID = %q, want i-0target00000001", got)
	}
}

// TestAutoOpenSingleDetail_NoFlagKeepsList verifies a normal list load (no
// AutoOpenSingle flag) stays on the list — the auto-open path must not fire for
// ordinary resource-list navigation.
func TestAutoOpenSingleDetail_NoFlagKeepsList(t *testing.T) {
	c := New(runtime.New(session.New(), nil))
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenResourceList,
		Context: runtime.ScreenContext{ResourceType: "ec2"},
	}})
	c.ensureListState()

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Gen:          c.core.AvailabilityGen(),
		Resources: []resource.Resource{
			{ID: "i-0aaa00000000001", Type: "ec2", Fields: map[string]string{"instance_id": "i-0aaa00000000001"}},
			{ID: "i-0bbb00000000002", Type: "ec2", Fields: map[string]string{"instance_id": "i-0bbb00000000002"}},
		},
	})

	if snap := c.Snapshot(); snap.Body.Kind != BodyKindList {
		t.Fatalf("expected list screen for a normal load, got %q", snap.Body.Kind)
	}
}

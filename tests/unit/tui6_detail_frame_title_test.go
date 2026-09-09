// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui6_detail_frame_title_test.go — one builder for the detail frame title,
// and it cleans both of the strings it is handed.
//
// A resource type may name its rows after their identifiers: the S3 object
// fetcher does, so an object's Name IS its key, which is operator-supplied
// text the boundary deliberately leaves raw on the identifier. The title's
// named branch paints that value a second time, in the parentheses, and a
// display call on the identifier alone leaves it there.
package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// tui6NamedObjectKey is an S3 object key carrying the single-character CSI.
// The fetcher gives such an object both its ID and its Name.
const tui6NamedObjectKey = "logs/2026/\u009b31mapp.log"

// TestDetailFrameTitle_CleansBothStringsItIsHanded drives the branch a named
// resource takes. Both halves of "detail -- <id> (<name>)" come from values
// the boundary left raw, so both have to be asked for in their displayed form.
func TestDetailFrameTitle_CleansBothStringsItIsHanded(t *testing.T) {
	title := resource.DetailFrameTitle(tui6NamedObjectKey, tui6NamedObjectKey, false)

	if !strings.Contains(title, "app.log") {
		t.Fatalf("the title does not name the object (%q), so this pin proves nothing", title)
	}
	// The composed shape, not just its parts: this is the one place the
	// "detail -- <id> (<name>)" form is asserted whole, and the parentheses
	// are the half a named resource adds.
	shown := resource.DisplayID(tui6NamedObjectKey)
	if want := "detail -- " + shown + " (" + shown + ")"; title != want {
		t.Errorf("the frame title is %q, want %q", title, want)
	}
	if ctrls := tui6Controls(title); len(ctrls) > 0 {
		t.Errorf("the frame title carries control runes %q: %q", ctrls, title)
	}
	if strings.IndexByte(title, 0x9b) >= 0 {
		t.Errorf("the frame title kept a C1 introducer byte: %q", title)
	}
	if strings.Contains(title, "31m") {
		t.Errorf("the frame title kept the C1 sequence's payload as visible text: %q", title)
	}
}

// TestDetailFrameTitle_UnnamedBranchStillCleansTheIdentifier is the
// counterpart: the branch without a name has one string to clean and still
// cleans it, so a fix to the named branch cannot be a move rather than an
// addition.
func TestDetailFrameTitle_UnnamedBranchStillCleansTheIdentifier(t *testing.T) {
	title := resource.DetailFrameTitle(tui6NamedObjectKey, "", false)
	if ctrls := tui6Controls(title); len(ctrls) > 0 {
		t.Errorf("the unnamed frame title carries control runes %q: %q", ctrls, title)
	}
	if strings.Contains(title, "31m") {
		t.Errorf("the unnamed frame title kept the C1 payload as visible text: %q", title)
	}
}

// TestDetailFrameTitle_BothLanesPaintTheBodysTitle pins that the title stayed
// one fact after row 24: the view state's own title is the body's, and the
// terminal paints that string rather than composing a second one.
func TestDetailFrameTitle_BothLanesPaintTheBodysTitle(t *testing.T) {
	page, err := awsclient.FetchEC2InstancesPage(context.Background(), demo.NewServiceClients().EC2, "")
	if err != nil {
		t.Fatalf("demo ec2 fetch: %v", err)
	}

	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(page.Resources[0], "ec2")
	c.SetDetailRelatedVisible(false, true)
	c.SetDetailViewportWidth(120)
	vs := c.Snapshot()
	if vs.Body.Detail == nil {
		t.Fatal("no detail body")
	}
	want := resource.DetailFrameTitle(page.Resources[0].ID, page.Resources[0].Name, resource.DetailTitleOmitsID("ec2"))
	if vs.Body.Detail.FrameTitle != want {
		t.Errorf("the body's title is %q; the one builder says %q", vs.Body.Detail.FrameTitle, want)
	}
	if vs.FrameTitle != vs.Body.Detail.FrameTitle {
		t.Errorf("the view state titles the screen %q while its body says %q", vs.FrameTitle, vs.Body.Detail.FrameTitle)
	}

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 36})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: page.Resources,
	})
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	m, _ = drainCmds(t, m, cmd, 5)
	if painted := ansi.Strip(rootViewContent(m)); !strings.Contains(painted, want) {
		t.Errorf("the terminal paints a frame title of its own; the body says %q\n%s", want, painted)
	}
}

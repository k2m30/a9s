// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui6_codex_boundary_test.go — the lanes an external review found still
// carrying AWS-supplied text to a painter.
//
// Each one is the same defect in a place the earlier pins did not look: a
// string read back off the SDK struct the fetcher kept, an identifier the
// boundary deliberately leaves raw, and a row written into the session store
// by a lane that does not go through the controller's doors. What they have
// in common is that a reader downstream of the boundary can still reintroduce
// raw bytes, which is what "the boundary cleans everything a painter can
// reach" has to mean to be worth anything.
package unit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// tui6OSCTitle is an operating-system-command sequence: a terminal that reads
// it retitles its own window. AWS accepts it in an alarm description, an
// object key or a rule description and hands it back verbatim.
const tui6OSCTitle = "web\x1b]0;pwned\x07-prod"

// tui6HostileObjectKey is an S3 object key carrying the single-character CSI.
// A key is operator-supplied text that arrives as a Resource.ID.
const tui6HostileObjectKey = "logs/2026/\u009b31mapp.log"

// tui6AlarmWithHostileDescription is a demo alarm whose SDK struct carries the
// hostile description, which is where an alarm's own words live — Fields holds
// its name, its namespace and its threshold, never its description.
func tui6AlarmWithHostileDescription(t *testing.T) resource.Resource {
	t.Helper()
	td := resource.FindResourceType("alarm")
	if td == nil {
		t.Fatal("alarm is not a registered resource type")
	}
	page, _ := td.Fetcher(context.Background(), demo.NewServiceClients(), "")
	if len(page.Resources) == 0 {
		t.Fatal("alarm has no demo fixtures")
	}
	res := page.Resources[0]
	ma, ok := res.RawStruct.(cwtypes.MetricAlarm)
	if !ok {
		t.Fatalf("the alarm fetcher no longer keeps a MetricAlarm, it keeps %T", res.RawStruct)
	}
	ma.AlarmDescription = aws.String("Triggers when " + tui6OSCTitle + " misbehaves")
	res.RawStruct = ma
	return res
}

// tui6AlarmDescriptionConfig is the operator's own view file asking for the
// alarm description on the detail and in a list column. A configured Path
// reads the SDK struct directly, which is the reader the boundary has to be in
// front of.
func tui6AlarmDescriptionConfig() *config.ViewsConfig {
	return &config.ViewsConfig{Views: map[string]config.ViewDef{
		"alarm": {
			List: []config.ListColumn{
				{Title: "Name", Key: "alarm_name", Width: 24},
				{Title: "Description", Path: "AlarmDescription", Width: 60},
			},
			Detail: []config.DetailField{
				{Path: "AlarmDescription", Label: "Description"},
			},
		},
	}}
}

// TestRawStructBoundary_DetailProjectionIsInert pins the generic detail
// projection: a description read out of the SDK struct is as clean as one read
// out of Fields.
func TestRawStructBoundary_DetailProjectionIsInert(t *testing.T) {
	res := tui6AlarmWithHostileDescription(t)
	c := newTestController(t)
	c.SetViewConfig(tui6AlarmDescriptionConfig())
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(res, "alarm")
	c.SetDetailRelatedVisible(false, true)
	c.SetDetailViewportWidth(120)

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("no detail body")
	}
	var shown string
	for _, f := range body.Fields {
		if strings.Contains(f.Value, "misbehaves") {
			shown = f.Value
		}
	}
	if shown == "" {
		t.Fatal("the alarm description reached no detail row, so this pin proves nothing")
	}
	tui6AssertNoControls(t, "the detail row read off the SDK struct", shown)
}

// TestRawStructBoundary_ConfiguredPathColumnIsInert pins the list
// materialisation: a column the operator pointed at an SDK field.
func TestRawStructBoundary_ConfiguredPathColumnIsInert(t *testing.T) {
	res := tui6AlarmWithHostileDescription(t)
	c := newTestController(t)
	c.SetViewConfig(tui6AlarmDescriptionConfig())
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "alarm"})
	c.ApplyResourcesLoaded("alarm", []resource.Resource{res}, nil, false)

	body := c.Snapshot().Body.List
	if body == nil || len(body.Rows) != 1 {
		t.Fatalf("want the one alarm row, got %+v", body)
	}
	var shown string
	for _, cell := range body.Rows[0].Cells {
		if strings.Contains(cell, "misbehaves") {
			shown = cell
		}
	}
	if shown == "" {
		t.Fatalf("the configured Description column showed nothing; cells: %q", body.Rows[0].Cells)
	}
	tui6AssertNoControls(t, "the configured Path column's cell", shown)
}

// TestRawStructBoundary_WebLaneStateIsInert pins what the other lane decodes:
// the serialised view state, which is the whole of what a browser has to go on.
func TestRawStructBoundary_WebLaneStateIsInert(t *testing.T) {
	res := tui6AlarmWithHostileDescription(t)
	c := newTestController(t)
	c.SetViewConfig(tui6AlarmDescriptionConfig())
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(res, "alarm")
	c.SetDetailRelatedVisible(false, true)
	c.SetDetailViewportWidth(120)

	raw, err := json.Marshal(c.Snapshot())
	if err != nil {
		t.Fatalf("serialising the view state: %v", err)
	}
	if !strings.Contains(string(raw), "misbehaves") {
		t.Fatal("the alarm description is not in the serialised state, so this pin proves nothing")
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding the view state: %v", err)
	}
	tui6AssertNoControlsDeep(t, "the serialised view state", decoded)
}

// TestDisplayedID_FrameTitleIsInert pins the identifier lane. Resource.ID is
// deliberately left raw — it is what the next AWS call and the clipboard need
// — so every place that PAINTS it has to ask for its displayed form. An S3
// object key is operator-supplied text that arrives as an ID.
func TestDisplayedID_FrameTitleIsInert(t *testing.T) {
	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(resource.Resource{
		ID: tui6HostileObjectKey, Type: "s3_objects",
		Fields: map[string]string{"key": tui6HostileObjectKey},
	}, "s3_objects")

	title := c.Snapshot().FrameTitle
	if !strings.Contains(title, "app.log") {
		t.Fatalf("the frame title does not name the object (%q), so this pin proves nothing", title)
	}
	tui6AssertNoControls(t, "the detail frame title", title)
	if strings.Contains(title, "31m") {
		t.Errorf("the frame title kept the C1 sequence's payload as visible text: %q", title)
	}
}

// TestDisplayedID_CopyStaysRaw is the counterpart, and the reason the ID is not
// simply cleaned at the boundary: what the operator pastes into a shell has to
// be the key AWS knows, byte for byte.
func TestDisplayedID_CopyStaysRaw(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	c.ApplyResourcesLoaded("s3", []resource.Resource{{
		ID: tui6HostileObjectKey, Name: "a9s-demo-healthy", Type: "s3",
		Fields: map[string]string{"name": "a9s-demo-healthy"},
	}}, nil, false)
	content, _ := c.CopyContent()
	if content != tui6HostileObjectKey {
		t.Errorf("the clipboard carries %q; the object's own key is %q, and a pasted key that is not the key names nothing", content, tui6HostileObjectKey)
	}
}

// TestRowStoreBoundary_ProbeWriterIsInert pins the lane that does not go
// through the controller's doors at all: availability probing and prefetch
// write straight into the session row store, and an exact related navigation
// seeds its visible rows from there. A rule cached by a probe is painted
// without ever having been loaded as a page.
func TestRowStoreBoundary_ProbeWriterIsInert(t *testing.T) {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)

	raw := []resource.Resource{{
		ID: "nightly-batch", Name: "nightly-batch", Type: "rule",
		Fields: map[string]string{
			"name":        "nightly-batch",
			"description": "runs the " + tui6OSCTitle + " batch",
		},
	}}
	observed, _ := core.ObserveRows("rule", raw, &resource.PaginationMeta{IsTruncated: false}, session.OriginProbe, false)
	if len(observed) != 1 {
		t.Fatalf("want the one observed rule, got %d", len(observed))
	}
	tui6AssertNoControls(t, "the row the probe writer returns", observed[0].Fields["description"])

	back, ok := core.ProbeResources("rule")
	if !ok || len(back) != 1 {
		t.Fatalf("the row store does not hold the probed rule: ok=%v rows=%d", ok, len(back))
	}
	tui6AssertNoControls(t, "the row an exact related navigation seeds from the store", back[0].Fields["description"])
}

// tui6AssertNoControls fails when s carries a control rune or a raw C1 byte.
func tui6AssertNoControls(t *testing.T, where, s string) {
	t.Helper()
	if ctrls := tui6Controls(s); len(ctrls) > 0 {
		t.Errorf("%s carries control runes %q: %q", where, ctrls, s)
	}
	if strings.IndexByte(s, 0x9b) >= 0 {
		t.Errorf("%s carries a raw C1 introducer byte: %q", where, s)
	}
}

// tui6AssertNoControlsDeep walks a decoded JSON document and applies
// tui6AssertNoControls to every string in it, so the pin covers whatever the
// other lane reads rather than the one field this test happens to name.
func tui6AssertNoControlsDeep(t *testing.T, where string, v any) {
	t.Helper()
	switch node := v.(type) {
	case string:
		tui6AssertNoControls(t, where, node)
	case []any:
		for _, item := range node {
			tui6AssertNoControlsDeep(t, where, item)
		}
	case map[string]any:
		for key, item := range node {
			tui6AssertNoControlsDeep(t, where+" ("+key+")", item)
		}
	}
}

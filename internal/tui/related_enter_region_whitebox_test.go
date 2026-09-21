// SPDX-License-Identifier: GPL-3.0-or-later

package tui

// related_enter_region_whitebox_test.go — the Region a related count was read
// in survives the keyboard.
//
// A count resolved in another Region renders as "(1)" and has to open that
// Region's list. Every hop between the rendered row and the fetch copies the
// Region by hand, so the value has to arrive from the keys an operator
// presses: Navigate to the detail, `l` to focus the related panel, Enter on
// the row — and the messages.RelatedNavigate the production chain emitted is
// what this reads.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// pressKey sends one key to the root Model.Update and returns the new Model
// and whatever command it produced.
func pressKey(t *testing.T, m Model, key tea.KeyPressMsg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(key)
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Model.Update(%v) returned %T, want tui.Model", key, next)
	}
	return got, cmd
}

// firstRelatedNavigate drains cmd one tea.Batch level deep for the
// messages.RelatedNavigate the Enter produced.
func firstRelatedNavigate(cmd tea.Cmd) (messages.RelatedNavigate, bool) {
	if cmd == nil {
		return messages.RelatedNavigate{}, false
	}
	msg := cmd()
	if nav, ok := msg.(messages.RelatedNavigate); ok {
		return nav, true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if sub == nil {
				continue
			}
			if nav, ok := sub().(messages.RelatedNavigate); ok {
				return nav, true
			}
		}
	}
	return messages.RelatedNavigate{}, false
}

// TestRelatedPanelEnter_CarriesTheRegionTheCountWasReadIn drives the chain a
// keypress drives: the related row the panel rendered, the event the Enter
// built from it, and the Region that has to reach the fetch behind it.
func TestRelatedPanelEnter_CarriesTheRegionTheCountWasReadIn(t *testing.T) {
	const (
		srcType = "trail"
		srcID   = "acme-org-trail"
		homed   = "us-west-2"
	)

	m := newNoClientsModel(t)
	src := resource.Resource{ID: srcID, Type: srcType, Name: srcID, Fields: map[string]string{"name": srcID}}
	next, _ := m.Update(messages.Navigate{Target: messages.TargetDetail, ResourceType: srcType, Resource: &src})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("Navigate returned %T, want tui.Model", next)
	}

	m, _ = pressKey(t, m, tea.KeyPressMsg{Code: 'l', Text: "l"}) // focus the related panel
	focused, ok := m.ctrl.SelectedRelatedRow()
	if !ok {
		t.Fatal("the related panel has no focused row to press Enter on")
	}

	// The answer a checker that read another Region produces, delivered onto
	// the focused row the way the adapter delivers one.
	m.ctrl.ApplyDetailRelatedResultForResource(srcType, srcID, focused.DisplayName, focused.TargetType,
		domain.RelatedResolved, 1, false, "", false, []string{"a-row-of-that-region"}, nil, homed)

	row, ok := m.ctrl.SelectedRelatedRow()
	if !ok || row.DisplayName != focused.DisplayName {
		t.Fatalf("the focused row moved to %+v, want %q", row, focused.DisplayName)
	}
	if row.Region != homed {
		t.Fatalf("the rendered row carries Region %q, want %q — the count was read there", row.Region, homed)
	}

	_, cmd := pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	nav, ok := firstRelatedNavigate(cmd)
	if !ok {
		t.Fatal("Enter on the related row emitted no RelatedNavigate")
	}
	if nav.Region != homed {
		t.Errorf("Enter built a navigation for Region %q, want %q; the list it opens would hold none of what the count named", nav.Region, homed)
	}
	if nav.TargetType != row.TargetType {
		t.Errorf("Enter navigated to %q, want %q", nav.TargetType, row.TargetType)
	}

	// The list the navigation opens has to read, page and announce that same
	// Region: the hop between the message and the runtime event is one more
	// place the value is copied by hand.
	opened, _ := m.Update(nav)
	after, ok := opened.(Model)
	if !ok {
		t.Fatalf("RelatedNavigate returned %T, want tui.Model", opened)
	}
	if got := after.ctrl.GetListRegion(); got != homed {
		t.Errorf("the opened list reads Region %q, want %q", got, homed)
	}
}

// TestFieldEnter_CarriesTheRegionTheReferenceNames drives the other key that
// opens the same drill: Enter on a navigable detail field. The field's value
// is the reference, and an ARN of another Region names a row the session's
// own does not hold.
func TestFieldEnter_CarriesTheRegionTheReferenceNames(t *testing.T) {
	const (
		homed    = "us-west-2"
		groupARN = "arn:aws:logs:" + homed + ":123456789012:log-group:acme-org-trail-events:*"
	)

	m := newNoClientsModel(t)
	src := resource.Resource{
		ID: "acme-org-trail", Type: "trail", Name: "acme-org-trail",
		Fields: map[string]string{"name": "acme-org-trail", "home_region": homed},
		RawStruct: cloudtrailtypes.Trail{
			Name:                      aws.String("acme-org-trail"),
			HomeRegion:                aws.String(homed),
			CloudWatchLogsLogGroupArn: aws.String(groupARN),
		},
	}
	next, _ := m.Update(messages.Navigate{Target: messages.TargetDetail, ResourceType: "trail", Resource: &src})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("Navigate returned %T, want tui.Model", next)
	}

	// Walk the field cursor onto the log-group field with the key that walks
	// it, so the cursor the Enter reads is the one the keyboard put there.
	idx := -1
	for step := 0; step < 200; step++ {
		body := m.ctrl.Snapshot().Body.Detail
		if body == nil {
			t.Fatal("no detail body to walk")
		}
		if fc := body.FieldCursor; fc >= 0 && fc < len(body.Fields) {
			f := body.Fields[fc]
			if f.IsNavigable && f.TargetType == "logs" {
				idx = fc
				break
			}
		}
		m, _ = pressKey(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	if idx < 0 {
		t.Fatal("the trail detail has no navigable logs field to press Enter on")
	}

	_, cmd := pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	nav, ok := firstRelatedNavigate(cmd)
	if !ok {
		t.Fatal("Enter on the navigable field emitted no RelatedNavigate")
	}
	if nav.TargetType != "logs" {
		t.Fatalf("Enter navigated to %q, want logs", nav.TargetType)
	}
	if nav.Region != homed {
		t.Errorf("Enter built a navigation for Region %q, want %q; the field names a log group there", nav.Region, homed)
	}

	opened, _ := m.Update(nav)
	after, ok := opened.(Model)
	if !ok {
		t.Fatalf("RelatedNavigate returned %T, want tui.Model", opened)
	}
	if got := after.ctrl.GetListRegion(); got != homed {
		t.Errorf("the opened list reads Region %q, want %q", got, homed)
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui5_one_humanizer_test.go — one humanizer for every surface a Field value
// reaches. A fetcher writes its own spelling into Fields ("true", an RFC3339
// timestamp); the list cell already renders the settled form ("Yes",
// "2026-01-02 15:04"). These pins put the detail row and the text filter on
// the same rule, so the fetcher's raw form never shows and the operator can
// filter for what they see.
package unit_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// humanizerAMI is one AMI row whose Fields carry the fetcher's raw spellings:
// a bool as "true" and a timestamp as RFC3339. No RawStruct, which is what a
// warm-cache replay hands every surface.
func humanizerAMI() resource.Resource {
	return resource.Resource{
		ID:   "ami-0abc123def4567890",
		Name: "acme-base-image",
		Type: "ami",
		Fields: map[string]string{
			"image_id":      "ami-0abc123def4567890",
			"name":          "acme-base-image",
			"public":        "true",
			"creation_date": "2026-01-02T15:04:05Z",
		},
	}
}

// detailRowValue returns the rendered value of the detail row whose label
// matches label (case-insensitively), or "" with a listing of what was there.
func detailRowValue(t *testing.T, c *app.Controller, label string) string {
	t.Helper()
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatalf("no detail body")
	}
	var seen []string
	for _, f := range body.Fields {
		seen = append(seen, f.Key)
		if strings.EqualFold(f.Key, label) {
			return f.Value
		}
	}
	t.Fatalf("no detail row labelled %q; rows: %v", label, seen)
	return ""
}

// TestOneHumanizer_DetailRowShowsTheSettledBool pins the detail surface: the
// Fields value the fetcher wrote as "true" reaches the row through the same
// rule the list cell uses, so both surfaces say "Yes".
func TestOneHumanizer_DetailRowShowsTheSettledBool(t *testing.T) {
	c := newAttentionCursorController(t, humanizerAMI(), "ami")
	if got := detailRowValue(t, c, "Public"); got != "Yes" {
		t.Errorf("detail Public row = %q, want %q — the fetcher's raw \"true\" reached the screen", got, "Yes")
	}
}

// TestOneHumanizer_DetailRowShowsTheSettledTimestamp pins the same rule for
// the other shape a fetcher writes by hand.
func TestOneHumanizer_DetailRowShowsTheSettledTimestamp(t *testing.T) {
	c := newAttentionCursorController(t, humanizerAMI(), "ami")
	if got := detailRowValue(t, c, "creation_date"); got != "2026-01-02 15:04" {
		t.Errorf("detail creation_date row = %q, want %q — the fetcher's raw RFC3339 reached the screen", got, "2026-01-02 15:04")
	}
}

// TestOneHumanizer_DateOnlyKeepsItsPrecision pins the boundary the settled
// timestamp shape must not cross. A fetcher that writes a date and no time —
// every secrets Last Accessed / Last Changed value is one — knows the day and
// not the hour, and rendering it as "<date> 00:00" claims a midnight AWS never
// reported. The rule settles the shapes it can and adds nothing.
func TestOneHumanizer_DateOnlyKeepsItsPrecision(t *testing.T) {
	res := resource.Resource{
		ID: "prod/app/slack-webhook", Name: "prod/app/slack-webhook", Type: "secrets",
		Fields: map[string]string{
			"name":          "prod/app/slack-webhook",
			"last_accessed": "2026-03-18",
		},
	}
	c := newAttentionCursorController(t, res, "secrets")
	if got := detailRowValue(t, c, "last_accessed"); got != "2026-03-18" {
		t.Errorf("detail last_accessed row = %q, want %q — a date-only value gained a time it never had", got, "2026-03-18")
	}
}

// TestOneHumanizer_FilterMatchesWhatTheCellShows pins the filter surface: the
// operator types what the list shows them. A row whose Fields say "true" and
// whose cell says "Yes" must be found by "Yes".
func TestOneHumanizer_FilterMatchesWhatTheCellShows(t *testing.T) {
	c := openListController(t, "ami")
	c.ApplyResourcesLoaded("ami", []resource.Resource{humanizerAMI()}, nil, false)

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "Yes"})
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatalf("no list body")
	}
	if len(body.Rows) != 1 {
		t.Errorf("filter %q matched %d rows, want 1 — the filter reads the fetcher's raw \"true\", not the cell's \"Yes\"", "Yes", len(body.Rows))
	}
}

// ---------------------------------------------------------------------------
// The conventions belong to the surface, not the lane. A value that reaches a
// cell or a detail row off the SDK struct is subject to them exactly as one
// that came out of the Fields map: the operator reads one screen and cannot
// see which lane fed which line.
// ---------------------------------------------------------------------------

// humanizerAMIRaw is the shape ami's fetcher hands over: AWS models the two
// timestamps as strings, so nothing downstream sees a time.Time.
type humanizerAMIRaw struct {
	ImageId         string
	Name            string
	CreationDate    string
	DeprecationTime string
	Public          bool
}

// humanizerSecretRaw is the shape secrets' fetcher hands over: real times,
// which AWS reports at day granularity for the access date.
type humanizerSecretRaw struct {
	Name             string
	LastAccessedDate time.Time
	LastChangedDate  time.Time
}

// detailRowsFor renders one resource's scalar detail rows as label → value.
// It uses the built-in view config, which is what decides a type has detail
// paths at all; without one the projector falls back to flat Fields rendering
// and the struct lane never runs.
func detailRowsFor(t *testing.T, res resource.Resource, shortName string) map[string]string {
	t.Helper()
	c := newAttentionCursorController(t, res, shortName)
	c.SetViewConfig(config.DefaultConfig())
	c.EnsureDetailState(res, shortName)
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatalf("no detail body for %s", shortName)
	}
	out := make(map[string]string, len(body.Fields))
	for _, f := range body.Fields {
		if f.IsSection || f.IsSpacer || f.IsHeader || f.IsSubField {
			continue
		}
		out[f.Key] = f.Value
	}
	return out
}

// detailRowMust returns the named detail row's value, failing when absent.
func detailRowMust(t *testing.T, rows map[string]string, label string) string {
	t.Helper()
	v, ok := rows[label]
	if !ok {
		t.Fatalf("no detail row labelled %q; rows: %v", label, rows)
	}
	return v
}

// TestOneHumanizer_DetailRowOffTheStructFollowsTheConventions pins the
// RawStruct lane at the detail surface. ami's timestamps are strings in the
// SDK struct, so no formatter downstream recognised them as times and the row
// showed the wire format.
func TestOneHumanizer_DetailRowOffTheStructFollowsTheConventions(t *testing.T) {
	rows := detailRowsFor(t, resource.Resource{
		ID: "ami-0abc123def4567890", Name: "acme-base-image", Type: "ami",
		RawStruct: humanizerAMIRaw{
			ImageId:      "ami-0abc123def4567890",
			Name:         "acme-base-image",
			CreationDate: "2026-02-15T10:30:00Z",
			// AWS reports a deprecation at day granularity.
			DeprecationTime: "2028-01-01T00:00:00Z",
			Public:          false,
		},
	}, "ami")

	for _, tc := range []struct{ label, want string }{
		{"CreationDate", "2026-02-15 10:30"},
		{"DeprecationTime", "2028-01-01"},
		{"Public", "No"},
	} {
		if got := detailRowMust(t, rows, tc.label); got != tc.want {
			t.Errorf("detail %s = %q, want %q — the struct lane skipped the conventions", tc.label, got, tc.want)
		}
	}
}

// TestOneHumanizer_DetailBoolStringOffTheStruct pins the other spelling AWS
// uses: a CloudTrail event's ReadOnly is the string "false", not a bool.
func TestOneHumanizer_DetailBoolStringOffTheStruct(t *testing.T) {
	td := resource.FindResourceType("ct-events")
	if td == nil {
		t.Skip("ct-events is not registered")
	}
	page, _ := td.Fetcher(context.Background(), demo.NewServiceClients(), "")
	if len(page.Resources) == 0 {
		t.Skip("ct-events has no demo fixtures")
	}

	var checked int
	for _, res := range page.Resources {
		for label, value := range detailRowsFor(t, res, "ct-events") {
			if value == "true" || value == "false" {
				t.Errorf("ct-events %s detail row %q = %q, want Yes or No", res.ID, label, value)
			}
			if label == "ReadOnly" {
				checked++
			}
		}
	}
	if checked == 0 {
		t.Error("no ct-events fixture rendered a ReadOnly row; the pin proves nothing")
	}
}

// TestOneHumanizer_DetailTimeStructLosesTheFakeMidnight pins the shape the
// secrets detail showed: a time.Time whose time of day is exactly midnight is
// a day AWS reports at day granularity, and the row says the day.
func TestOneHumanizer_DetailTimeStructLosesTheFakeMidnight(t *testing.T) {
	rows := detailRowsFor(t, resource.Resource{
		ID: "prod/app/slack-webhook", Name: "prod/app/slack-webhook", Type: "secrets",
		RawStruct: humanizerSecretRaw{
			Name:             "prod/app/slack-webhook",
			LastAccessedDate: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
			LastChangedDate:  time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		},
	}, "secrets")

	if got := detailRowMust(t, rows, "LastAccessedDate"); got != "2026-04-20" {
		t.Errorf("detail LastAccessedDate = %q, want %q — a midnight nobody measured", got, "2026-04-20")
	}
	if got := detailRowMust(t, rows, "LastChangedDate"); got != "2026-04-15 12:00" {
		t.Errorf("detail LastChangedDate = %q, want %q — a real time of day must survive", got, "2026-04-15 12:00")
	}
}

// TestOneHumanizer_ListCellOffTheStructFollowsTheConventions pins the same
// lane at the other surface, so neither can be fixed alone.
func TestOneHumanizer_ListCellOffTheStructFollowsTheConventions(t *testing.T) {
	res := resource.Resource{
		ID: "ami-0abc123def4567890", Name: "acme-base-image", Type: "ami",
		RawStruct: humanizerAMIRaw{
			CreationDate:    "2026-02-15T10:30:00Z",
			DeprecationTime: "2028-01-01T00:00:00Z",
			Public:          false,
		},
	}
	for _, tc := range []struct{ path, want string }{
		{"CreationDate", "2026-02-15 10:30"},
		{"DeprecationTime", "2028-01-01"},
		{"Public", "No"},
	} {
		got := app.ExtractCellValue(app.ColumnDef{Title: tc.path, Path: tc.path, Width: 24}, nil, res)
		if got != tc.want {
			t.Errorf("list cell for %s = %q, want %q", tc.path, got, tc.want)
		}
	}
}

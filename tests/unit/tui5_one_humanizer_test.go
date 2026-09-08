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
	"github.com/k2m30/a9s/v3/core/catalog"
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

// detailRowsFor renders one resource's scalar detail rows as label → value,
// flattened across sections.
func detailRowsFor(t *testing.T, res resource.Resource, shortName string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, rows := range detailRowsBySection(t, res, shortName) {
		for k, v := range rows {
			out[k] = v
		}
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

// detailRowsBySection renders one resource's scalar detail rows grouped by the
// section heading above them, which is what says whether a row is a9s's own
// presentation of a fact or a line of a document a9s is quoting.
//
// It uses the built-in view config, which is what decides a type has detail
// paths at all; without one the projector falls back to flat Fields rendering
// and the struct lane never runs.
func detailRowsBySection(t *testing.T, res resource.Resource, shortName string) map[string]map[string]string {
	t.Helper()
	c := newAttentionCursorController(t, res, shortName)
	c.SetViewConfig(config.DefaultConfig())
	c.EnsureDetailState(res, shortName)
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatalf("no detail body for %s", shortName)
	}
	out := map[string]map[string]string{}
	section := ""
	for _, f := range body.Fields {
		if f.IsSection {
			section = f.Key
			continue
		}
		if f.IsSpacer || f.IsHeader || f.IsSubField {
			continue
		}
		if out[section] == nil {
			out[section] = map[string]string{}
		}
		out[section][f.Key] = f.Value
	}
	return out
}

// TestOneHumanizer_DetailBoolStringOffTheStruct pins the other spelling AWS
// uses — a CloudTrail event's ReadOnly is the string "false", not a bool —
// and, in the same sweep, where the conventions stop.
//
// RAW EVENT is the event exactly as AWS sent it. Quoting a document and
// editing the words inside it are different things, and the section is what
// says which one a row is: a top-level scalar of that block is as much part of
// the quotation as a key nested three lines below it. Everywhere else, a9s is
// presenting the fact in its own words and the conventions apply.
func TestOneHumanizer_DetailBoolStringOffTheStruct(t *testing.T) {
	td := resource.FindResourceType("ct-events")
	if td == nil {
		t.Skip("ct-events is not registered")
	}
	page, _ := td.Fetcher(context.Background(), demo.NewServiceClients(), "")
	if len(page.Resources) == 0 {
		t.Skip("ct-events has no demo fixtures")
	}

	const rawSection = "RAW EVENT"
	var presented, quoted int
	for _, res := range page.Resources {
		for section, rows := range detailRowsBySection(t, res, "ct-events") {
			for label, value := range rows {
				if section == rawSection {
					// The document, verbatim.
					if value == "Yes" || value == "No" {
						t.Errorf("ct-events %s: %s row %q = %q — a quoted document is not reworded",
							res.ID, section, label, value)
					}
					if value == "true" || value == "false" {
						quoted++
					}
					continue
				}
				if value == "true" || value == "false" {
					t.Errorf("ct-events %s: %s row %q = %q, want Yes or No",
						res.ID, section, label, value)
				}
				if label == "ReadOnly" {
					presented++
				}
			}
		}
	}
	if presented == 0 {
		t.Error("no fixture rendered a presented ReadOnly row; the pin proves nothing")
	}
	if quoted == 0 {
		t.Error("no fixture rendered a raw-event bool; the quoting half proves nothing")
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

// TestOneHumanizer_DetailRowReadsTheColumnsWords pins the other direction of
// the same panel inconsistency: a column that opts its field into readable
// wording is stating something about the fact, not about the list. The detail
// row for that field says the same words, so the two panels of one screen
// cannot describe one value two ways.
func TestOneHumanizer_DetailRowReadsTheColumnsWords(t *testing.T) {
	for _, tc := range []struct {
		shortName string
		id        string
		label     string // the detail row's label, which is the column's Path
		want      string
	}{
		{"nat", "nat-0failed111111111d", "FailureCode", "insufficient free addresses in subnet"},
		{"ecs-task", "f6a1b2c3d4e5f60102030405", "StopCode", "task failed to start"},
	} {
		t.Run(tc.shortName, func(t *testing.T) {
			td := resource.FindResourceType(tc.shortName)
			if td == nil {
				t.Skipf("%s is not registered", tc.shortName)
			}
			page, _ := td.Fetcher(context.Background(), demo.NewServiceClients(), "")
			var res resource.Resource
			for _, r := range page.Resources {
				if r.ID == tc.id {
					res = r
				}
			}
			if res.ID == "" {
				t.Fatalf("%s has no fixture %q", tc.shortName, tc.id)
			}

			// The list cell is the words the operator has just read.
			cols := resource.ResolveListColumnCascade(nil, tc.shortName, td)
			var cell string
			for _, lc := range cols {
				if lc.Path != tc.label {
					continue
				}
				// INVERTED for aws6 rows 4-6: the humanize opt-in is the
				// type's declaration, not the column's.
				humanized := td.HumanizedFields()
				cell = app.ExtractCellValue(app.ColumnDef{
					Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path,
					Humanize: catalog.Humanizes(humanized, lc.Key, lc.Path),
				}, td, res)
			}
			if cell != tc.want {
				t.Fatalf("test sanity: the %s cell renders %q, not %q", tc.label, cell, tc.want)
			}

			rows := detailRowsFor(t, res, tc.shortName)
			got, ok := rows[tc.label]
			if !ok {
				t.Fatalf("no detail row labelled %q; rows: %v", tc.label, rows)
			}
			if got != cell {
				t.Errorf("detail %s = %q while the cell above reads %q — one fact, two words",
					tc.label, got, cell)
			}
		})
	}
}

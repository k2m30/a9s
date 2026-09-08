// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui5_one_humanizer_test.go — one humanizer for every surface a Field value
// reaches. A fetcher writes its own spelling into Fields ("true", an RFC3339
// timestamp); the list cell already renders the settled form ("Yes",
// "2026-01-02 15:04"). These pins put the detail row and the text filter on
// the same rule, so the fetcher's raw form never shows and the operator can
// filter for what they see.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
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

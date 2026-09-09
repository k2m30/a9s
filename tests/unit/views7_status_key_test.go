// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_status_key_test.go — one key answers for the status cell.
//
// A status column is declared with a key like every other column, and the
// cascade reads that key. Reading the lifecycle key first, then "status", then
// the column's own key, means a type that declares its status under another
// name (tg's health_summary, cb's last_build) shows whatever else happens to
// be in Fields, and the declaration it made is consulted third or not at all.
//
// The other half is the two lanes. The screen builds a cell from the row a
// fetch produced; after a restart it builds the same cell from the row the
// save lane wrote. If the key the save lane persists under is not the key the
// cascade reads, the same column shows one thing today and another tomorrow.
package unit_test

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// views7StatusVPCRaw is the SDK shape a keyless status column reads its cell
// from — the one column shape whose value lives only in the struct.
type views7StatusVPCRaw struct {
	VpcId string //nolint:revive,stylecheck // the SDK's own field name, which the column's Path names
	State string
}

// views7StatusCase is one row on one type, and the cell its Status column must
// show — on the screen the fetch fills, and on the screen a restart rebuilds
// from what was saved.
type views7StatusCase struct {
	name      string
	shortName string
	id        string
	fields    map[string]string
	raw       any
	findings  []domain.Finding
	want      string
}

// views7StatusCases covers the three shapes a status column comes in, and the
// finding that outranks all of them.
var views7StatusCases = []views7StatusCase{
	{
		// The column's key IS the lifecycle key: the value is the phrase the
		// save lane stores there, and nothing else in Fields competes.
		name:      "the key is the lifecycle key",
		shortName: "sqs",
		id:        "https://sqs.us-east-1.amazonaws.com/123456789012/order-processing-queue",
		fields: map[string]string{
			"queue_name": "order-processing-queue",
			"state":      "redrive backlog",
		},
		want: "redrive backlog",
	},
	{
		// The type declares its status under its own key and the fetcher
		// writes a lifecycle value too. The declared key is the one the column
		// names, so it is the one the cell reads.
		name:      "the type declares its own status key",
		shortName: "tg",
		id:        "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/0123456789abcdef",
		fields: map[string]string{
			"target_group_name": "acme-web-tg",
			"health_summary":    "unhealthy targets: 2/5",
			"state":             "available",
		},
		want: "unhealthy targets: 2/5",
	},
	{
		// The same shape with the competing value under "status" rather than
		// the lifecycle key — the second spelling the cascade reads before the
		// column's own.
		name:      "its own status key against a stored status",
		shortName: "cb",
		id:        "acme-api-build",
		fields: map[string]string{
			"name":       "acme-api-build",
			"last_build": "last build failed",
			"status":     "SUCCEEDED",
		},
		want: "last build failed",
	},
	{
		// No key at all: the cell comes from the struct, and the save lane is
		// what puts it in Fields for the next start.
		name:      "no key, the value is in the struct",
		shortName: "vpc",
		id:        "vpc-0123456789abcdef0",
		fields:    map[string]string{"name": "acme-prod-vpc"},
		raw:       views7StatusVPCRaw{VpcId: "vpc-0123456789abcdef0", State: "available"},
		want:      "available",
	},
	{
		// A finding outranks every stored value, on every shape: the phrase is
		// what the operator is being told about this row.
		name:      "a finding outranks the stored value",
		shortName: "tg",
		id:        "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-api-tg/0123456789abcdef",
		fields: map[string]string{
			"target_group_name": "acme-api-tg",
			"health_summary":    "healthy",
			"state":             "available",
		},
		findings: []domain.Finding{{
			Code: "tg-no-healthy-targets", Phrase: "no healthy targets",
			Severity: domain.SevBroken, Source: "wave1",
		}},
		want: "no healthy targets",
	},
}

// views7StatusColumn returns the resolved status column of a type, through the
// cascade every renderer resolves its columns with.
func views7StatusColumn(t *testing.T, td *resource.ResourceTypeDef) app.ColumnDef {
	t.Helper()
	lifecycleKey := td.LifecycleKey
	if lifecycleKey == "" {
		lifecycleKey = "state"
	}
	for _, lc := range resource.ResolveListColumnCascade(nil, td.ShortName, td) {
		if config.IsStatusColumn(lc.Key, lc.Title, lifecycleKey) {
			return app.ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path, SortKey: lc.SortKey}
		}
	}
	t.Fatalf("%s declares no status column", td.ShortName)
	return app.ColumnDef{}
}

// TestStatusCellReadsTheColumnsOwnKey drives the live lane: the row as a
// fetcher hands it over, and the cell the screen builds from it.
func TestStatusCellReadsTheColumnsOwnKey(t *testing.T) {
	for _, c := range views7StatusCases {
		t.Run(c.name, func(t *testing.T) {
			td := resource.FindResourceType(c.shortName)
			if td == nil {
				t.Fatalf("no resource type %q", c.shortName)
			}
			row := resource.Resource{ID: c.id, Name: c.id, Fields: c.fields, RawStruct: c.raw, Findings: c.findings}
			if got := app.ExtractCellValue(views7StatusColumn(t, td), td, row); got != c.want {
				t.Errorf("%s Status cell = %q, want %q — the cell reads the key the column declares, "+
					"not whichever spelling of the fact is in Fields", c.shortName, got, c.want)
			}
		})
	}
}

// TestStatusCellIsTheSameAfterARestart drives the save lane and reads the same
// cell off what it persisted. The row a restart renders has no struct and only
// the Fields the save wrote, so this is where a save that stores under one key
// and a cascade that reads another part company.
func TestStatusCellIsTheSameAfterARestart(t *testing.T) {
	for _, c := range views7StatusCases {
		t.Run(c.name, func(t *testing.T) {
			td := resource.FindResourceType(c.shortName)
			if td == nil {
				t.Fatalf("no resource type %q", c.shortName)
			}
			col := views7StatusColumn(t, td)
			live := resource.Resource{ID: c.id, Name: c.id, Fields: c.fields, RawStruct: c.raw, Findings: c.findings}

			const profile, region = "example-readonly", "us-east-1"
			ctrl := newTestControllerForProfile(t, profile, region)
			_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: c.shortName})
			_, _ = ctrl.Handle(messages.ResourcesLoaded{
				ResourceType: c.shortName,
				Resources:    []resource.Resource{live},
				Pagination:   &domain.PaginationMeta{IsTruncated: false},
				Provenance:   messages.FetchProvenanceCanonicalList,
			})
			ctrl.WaitForCacheWrites()

			saved := views7SavedRow(t, profile, region, c.shortName, c.id)
			replayed := resource.Resource{ID: saved.ID, Name: saved.Name, Fields: saved.Fields, Findings: saved.Findings}

			got := app.ExtractCellValue(col, td, replayed)
			if got != c.want {
				t.Errorf("%s Status cell after a restart = %q, want %q — the save lane writes the cell "+
					"under one key and the cascade reads another, so the same column says two things "+
					"(saved Fields: %v)", c.shortName, got, c.want, saved.Fields)
			}
		})
	}
}

// views7SavedRow reads back the row the save lane persisted for one resource.
func views7SavedRow(t *testing.T, profile, region, shortName, id string) cache.Row {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if tf, ok := cache.LoadDirForTest(profile, region).Type(shortName); ok {
			for _, r := range tf.Rows {
				if r.ID == id {
					return r
				}
			}
			t.Fatalf("the %s type file holds %d rows, none of them %q", shortName, len(tf.Rows), id)
		}
		if time.Now().After(deadline) {
			t.Fatalf("no %s type file under %s/%s after 5s", shortName, profile, region)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

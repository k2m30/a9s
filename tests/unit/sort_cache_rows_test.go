// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestSort_CacheRowsSortLikeLiveRows sorts the same rows twice on a column
// whose displayed text does not sort the way the value does: once with the
// AWS struct attached (the frame right after a fetch) and once with only the
// stored Fields (the warm-cache frame the list opens on). The operator sees
// one list, so the two frames must be in the same order — otherwise the rows
// re-order under the cursor the moment the fetch lands.
func TestSort_CacheRowsSortLikeLiveRows(t *testing.T) {
	for _, tc := range []struct {
		shortName string
		colTitle  string
		live      []resource.Resource
		wantFirst string
	}{
		{
			shortName: "ddb",
			colTitle:  "Size",
			live: []resource.Resource{
				sortDDBRow("table-big", 2048, "2.0 KB"),
				sortDDBRow("table-small", 900, "900 B"),
			},
			wantFirst: "table-small",
		},
		{
			shortName: "dbi",
			colTitle:  "Status",
			// An instance that is "available" to AWS but publicly reachable
			// shows the finding phrase, not the raw status, so the two sort
			// the other way round from each other.
			live: []resource.Resource{
				sortDBIRow("db-creating", "creating", ""),
				sortDBIRow("db-exposed", "available", "publicly accessible"),
			},
			wantFirst: "db-exposed",
		},
	} {
		t.Run(tc.shortName, func(t *testing.T) {
			key := sortColKeyByTitle(t, tc.shortName, tc.colTitle)

			live := sortRowOrder(t, tc.shortName, key, tc.live)
			cached := sortRowOrder(t, tc.shortName, key, sortStripRawStruct(tc.live))

			if live[0] != tc.wantFirst {
				t.Errorf("live rows sorted by %q start with %q, want %q", tc.colTitle, live[0], tc.wantFirst)
			}
			if cached[0] != live[0] {
				t.Errorf("the cached frame sorted by %q starts with %q but the live frame starts with %q — the list re-orders under the operator when the fetch lands",
					tc.colTitle, cached[0], live[0])
			}
		})
	}
}

// TestSort_EverySortKeyColumnGetsAValueFromItsFetcher fetches each type that
// declares a sort_key from its demo fixtures and asserts every row carries a
// non-empty value under that key. A sort_key nothing writes compares empty
// strings, so pressing the column's number does nothing at all.
func TestSort_EverySortKeyColumnGetsAValueFromItsFetcher(t *testing.T) {
	checked := 0
	for _, td := range resource.AllResourceTypes() {
		keys := sortKeysOf(td.ShortName)
		if len(keys) == 0 || td.Fetcher == nil {
			continue
		}
		rows := sortDemoRows(t, td.ShortName)
		for _, pair := range keys {
			checked++
			sortCheckSortKeyValues(t, td.ShortName, pair, rows)
		}
	}
	for _, tc := range []struct {
		shortName string
		rows      []resource.Resource
	}{
		{"ecr_images", sortDemoECRImages(t)},
		{"s3_objects", sortDemoS3Objects(t)},
	} {
		for _, pair := range sortKeysOf(tc.shortName) {
			checked++
			sortCheckSortKeyValues(t, tc.shortName, pair, tc.rows)
		}
	}
	if checked == 0 {
		t.Fatal("no type declares a sort key — the gate would pass vacuously")
	}
}

// sortKeysOf pairs each sort_key declared on shortName's built-in view with
// the Fields key the same column displays.
func sortKeysOf(shortName string) [][2]string {
	var out [][2]string
	for _, col := range config.GetViewDef(nil, shortName).List {
		if col.SortKey != "" {
			out = append(out, [2]string{col.Key, col.SortKey})
		}
	}
	return out
}

// sortCheckSortKeyValues asserts the rows carry the sort key wherever they
// carry the value the column shows. A row the fetcher could not describe (the
// demo plants one per type) shows nothing and sorts by nothing, which is
// correct; a row that shows a value and sorts by nothing is the defect.
func sortCheckSortKeyValues(t *testing.T, shortName string, pair [2]string, rows []resource.Resource) {
	t.Helper()
	displayKey, sortKey := pair[0], pair[1]
	carriers := 0
	for _, r := range rows {
		if r.Fields[sortKey] != "" {
			carriers++
			continue
		}
		if displayKey != "" && r.Fields[displayKey] != "" {
			t.Errorf("%s: row %q shows %q under %q but has nothing under sort key %q",
				shortName, r.ID, r.Fields[displayKey], displayKey, sortKey)
		}
	}
	if carriers == 0 {
		t.Errorf("%s: no row carries sort key %q — sorting on that column does nothing", shortName, sortKey)
	}
}

func sortColKeyByTitle(t *testing.T, shortName, title string) string {
	t.Helper()
	ctrl := newTestController(t)
	for _, c := range ctrl.ResolveColumnsForType(shortName) {
		if c.Title == title {
			return c.SortColKey()
		}
	}
	t.Fatalf("%s has no column titled %q", shortName, title)
	return ""
}

// sortRowOrder opens a list of shortName, sorts it on key and returns the row
// IDs in rendered order.
func sortRowOrder(t *testing.T, shortName, key string, rows []resource.Resource) []string {
	t.Helper()
	ctrl := newTestController(t)
	sortOpenList(ctrl, shortName)
	ctrl.ApplyResourcesLoaded(shortName, rows, nil, false)
	ctrl.Apply(app.Action{Kind: app.ActionSort, Arg: key})
	body := ctrl.Snapshot().Body.List
	if body == nil {
		t.Fatalf("%s: no list body", shortName)
	}
	ids := make([]string, 0, len(body.Rows))
	for _, row := range body.Rows {
		ids = append(ids, row.Cells[body.MarkerCol])
	}
	if len(ids) != len(rows) {
		t.Fatalf("%s: sorted %d rows, want %d", shortName, len(ids), len(rows))
	}
	return ids
}

// sortStripRawStruct is the warm-cache shape of the same rows: the stored
// Fields survive a restart, the AWS struct does not.
func sortStripRawStruct(rows []resource.Resource) []resource.Resource {
	out := make([]resource.Resource, len(rows))
	for i, r := range rows {
		r.RawStruct = nil
		out[i] = r
	}
	return out
}

func sortDDBRow(name string, sizeBytes int64, shown string) resource.Resource {
	return resource.Resource{
		ID: name, Name: name,
		Fields: map[string]string{
			"table_name": name, "status": "active", "size_bytes": shown,
			"size_bytes_raw": strconv.FormatInt(sizeBytes, 10),
		},
		RawStruct: &ddbtypes.TableDescription{
			TableName: aws.String(name), TableSizeBytes: aws.Int64(sizeBytes),
		},
	}
}

func sortDBIRow(name, status, phrase string) resource.Resource {
	var findings []domain.Finding
	shown := status
	if phrase != "" {
		findings = []domain.Finding{{Code: "TEST", Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"}}
		shown = phrase
	}
	return resource.Resource{
		ID: name, Name: name, Findings: findings,
		Fields: map[string]string{
			"db_identifier": name, "status": shown, "status_raw": status,
		},
		RawStruct: &rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String(name), DBInstanceStatus: aws.String(status),
		},
	}
}

func sortDemoRows(t *testing.T, shortName string) []resource.Resource {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	// A partial-failure fixture (a row the demo denies DescribeTable on) is a
	// fetch error with rows attached; the rows it did return are the subject.
	res, err := td.Fetcher(context.Background(), demo.NewServiceClients(), "")
	if len(res.Resources) == 0 {
		t.Fatalf("%s demo fetch returned no rows: %v", shortName, err)
	}
	return res.Resources
}

func sortDemoECRImages(t *testing.T) []resource.Resource {
	t.Helper()
	repos, err := awsclient.FetchECRRepositoriesPage(context.Background(), fakes.NewECR(), "")
	if err != nil || len(repos.Resources) == 0 {
		t.Fatalf("ecr demo fetch: %v (%d rows)", err, len(repos.Resources))
	}
	var out []resource.Resource
	for _, repo := range repos.Resources {
		res, err := awsclient.FetchECRImages(context.Background(), fakes.NewECR(), map[string]string{
			"repository_name": repo.ID,
			"repository_uri":  repo.Fields["uri"],
		}, "")
		if err != nil {
			t.Fatalf("ecr_images demo fetch for %q: %v", repo.ID, err)
		}
		out = append(out, res.Resources...)
	}
	if len(out) == 0 {
		t.Fatal("ecr_images demo fetch returned no rows")
	}
	return out
}

func sortDemoS3Objects(t *testing.T) []resource.Resource {
	t.Helper()
	buckets, err := awsclient.FetchS3BucketsPageWithNotifications(context.Background(), fakes.NewS3(), nil, "")
	if err != nil || len(buckets.Resources) == 0 {
		t.Fatalf("s3 demo fetch: %v (%d rows)", err, len(buckets.Resources))
	}
	var out []resource.Resource
	for _, b := range buckets.Resources {
		res, err := awsclient.FetchS3Objects(context.Background(), fakes.NewS3(), b.ID, "", "")
		if err != nil {
			t.Fatalf("s3_objects demo fetch for %q: %v", b.ID, err)
		}
		for _, r := range res.Resources {
			if r.Fields["kind"] == "file" {
				out = append(out, r)
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("s3_objects demo fetch returned no file rows")
	}
	return out
}

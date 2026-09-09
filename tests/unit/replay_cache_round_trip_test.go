// replay_cache_round_trip_test.go — a row restored from the disk cache
// renders the cells the fetch rendered.
//
// Three lanes resolve a type's columns: the render lane, the cache save lane,
// and the render lane again on the replayed row. The save lane decides which
// values are worth persisting; the render lane decides how to show them. The
// live frame resolves a cell through the whole extraction cascade, RawStruct
// included; the replayed frame has no RawStruct at all, so every value it can
// still answer with has to have been persisted under the key the cascade will
// read on the way back. Where the save lane persists something other than the
// value the live cascade chose — or persists nothing — the list a user sees
// after restarting is not the list they saw before it.
//
// The live frame is canonical here: it is what users see and what every
// golden pins. So these tests assert the REPLAYED cell moves to the live
// word, never the other way round.
package unit_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// colsReplayRows is the shape a list has after a restart: what the disk cache
// kept (ID, Name, Fields, Findings) and nothing else. RawStruct is never
// persisted, so every column that can only answer from the SDK struct has to
// answer from the value the save lane materialized into Fields instead.
func colsReplayRows(shortName string, rows []cache.Row) []resource.Resource {
	out := make([]resource.Resource, len(rows))
	for i, r := range rows {
		out[i] = resource.Resource{
			ID:       r.ID,
			Name:     r.Name,
			Type:     shortName,
			Fields:   r.Fields,
			Findings: r.Findings,
		}
	}
	return out
}

// colsRowsByID indexes a rendered list body by resource ID, so the live frame
// and the replayed frame can be compared row for row regardless of the order
// each was sorted into. The whole row, not just its cells: colour, severity
// and the decorator are derived from Fields and Findings too, so a save that
// changes what Fields carries can move a row's colour without moving a cell.
func colsRowsByID(lb *app.ListBody) map[string]app.ListRow {
	out := make(map[string]app.ListRow, len(lb.Rows))
	for _, r := range lb.Rows {
		out[r.ResourceID] = r
	}
	return out
}

// colsRoundTrip drains a type's demo rows through its own Wave-1 fetcher,
// renders them, saves them through the controller's real list-open save lane,
// reads the type file back off disk and renders that. It hands back the two
// rendered frames indexed by resource ID plus the column titles they share.
func colsRoundTrip(t *testing.T, td resource.ResourceTypeDef, profilePrefix string) (live, replay map[string]app.ListRow, titles []string) {
	t.Helper()
	rows, ok := unit.DrainFixtures(t, td, demo.NewServiceClients())
	if !ok || len(rows) == 0 {
		t.Skipf("%s: no demo rows to drain", td.ShortName)
	}

	profile, region := profilePrefix+td.ShortName, "us-east-1"
	ctrl := newTestControllerForProfile(t, profile, region)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})

	ctrl.ApplyResourcesLoaded(td.ShortName, rows, nil, false)
	liveBody := ctrl.Snapshot().Body.List
	if liveBody == nil || len(liveBody.Rows) == 0 {
		t.Fatalf("%s: the live fetch rendered no rows", td.ShortName)
	}

	ctrl.WaitForCacheWrites()
	store := cache.LoadDirForTest(profile, region)
	tf, found := store.Type(td.ShortName)
	if !found || len(tf.Rows) == 0 {
		t.Fatalf("%s: the list-open save persisted nothing", td.ShortName)
	}

	ctrl.ApplyResourcesLoaded(td.ShortName, colsReplayRows(td.ShortName, tf.Rows), nil, false)
	replayBody := ctrl.Snapshot().Body.List
	if replayBody == nil {
		t.Fatalf("%s: the cache replay rendered no list body", td.ShortName)
	}
	for _, col := range replayBody.Columns {
		titles = append(titles, col.Title)
	}
	return colsRowsByID(liveBody), colsRowsByID(replayBody), titles
}

// TestCols_CacheReplayRendersTheSameCellsAsTheLiveFetch closes the loop on the
// third lane that resolves a type's columns: the cache save. That lane
// resolves its own column set to decide which values are worth persisting,
// and the render lane resolves another to decide how to show them. When the
// two sets disagree about a column's Path or its Humanize flag, a value is
// persisted the renderer will not humanize, or a value the renderer needs is
// never persisted at all — and the list a user sees after restarting is not
// the list they saw before it.
//
// Real demo rows are drained through each type's own Wave-1 fetcher, saved
// through the controller's real list-open save lane, read back off disk, and
// rendered again through the same controller. Every cell must match, and so
// must the colour, the severity and the decorator: those are derived from
// Fields and Findings as well, so a save lane that changes which key holds a
// column's value can repaint a row without moving one of its cells.
func TestCols_CacheReplayRendersTheSameCellsAsTheLiveFetch(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		t.Run(td.ShortName, func(t *testing.T) {
			liveRows, replayRows, titles := colsRoundTrip(t, td, "cols-replay-")

			for id, want := range liveRows {
				got, present := replayRows[id]
				if !present {
					t.Errorf("%s: row %q vanished on the cache replay", td.ShortName, id)
					continue
				}
				if got.Color != want.Color {
					t.Errorf("%s: row %q is coloured %q live and %q after a cache round trip", td.ShortName, id, want.Color, got.Color)
				}
				if got.Severity != want.Severity {
					t.Errorf("%s: row %q has severity %q live and %q after a cache round trip", td.ShortName, id, want.Severity, got.Severity)
				}
				if len(got.Cells) != len(want.Cells) {
					t.Errorf("%s: row %q renders %d cells live and %d on replay", td.ShortName, id, len(want.Cells), len(got.Cells))
					continue
				}
				for i := range want.Cells {
					if got.Cells[i] != want.Cells[i] {
						t.Errorf("%s: row %q col[%d] %q renders %q live and %q after a cache round trip",
							td.ShortName, id, i, titles[i], want.Cells[i], got.Cells[i])
					}
				}
			}
		})
	}
}

// replayShadowedCell names one column of one demo row and the exact string
// both frames have to show for it.
type replayShadowedCell struct {
	shortName string
	id        string
	column    string
	want      string
}

// replayShadowedCells are the columns where the live cascade reads the SDK
// struct and the replayed row, having no struct, reads whatever the fetcher
// happened to leave in Fields under the column's lowercased title. The two
// disagree, so a restart silently rewrites the cell.
//
// want is the LIVE value in every row below, deliberately: the equality gate
// above is satisfied by moving either frame, and moving the live frame would
// change what a fetched list shows without a single golden noticing. These
// rows pin the direction — the replayed cell moves to the live word.
//
// Every id is a fixture with hardcoded values; rows whose fixture dates are
// computed from time.Now are deliberately not used.
//
// A row whose column title is more than one word can pass on a lucky run while
// the defect is still there: its saved row answers to that title under two
// spellings at once, and the loser of the coin flip is sometimes the live word.
// TestReplay_SavedRowLeavesOneAnswerForAColumnTitle is the deterministic half
// of that pair, and both have to be green together.
var replayShadowedCells = []replayShadowedCell{
	// bool: the cascade's FormatValue says Yes/No, the fetcher wrote the Go
	// literal. Both polarities, so a fix that hardcodes one is still red.
	{"ami", "ami-0public00000000001", "Public", "Yes"},
	{"ami", "ami-0a1b2c3d4e5f60001", "Public", "No"},
	{"acm", "arn:aws:acm:us-east-1:123456789012:certificate/os-acme-logs-cert-0001", "In Use", "Yes"},
	{"acm", "arn:aws:acm:us-east-1:123456789012:certificate/c9d0e1f2-3456-78ab-cdef-999999999999", "In Use", "No"},
	{"cf", "E1A2B3C4D5E6F7", "Enabled", "Yes"},
	{"cf", "E3C4D5E6F7G8H9", "Enabled", "No"},
	// float64: the defaults read the fetcher's own value for this column, so
	// both frames show the precision CloudWatch reports the threshold at
	// rather than the %g the reflect formatter leaves.
	{"alarm", "cf-e1a2b3c4d5e6f7-error-rate", "Threshold", "5.00"},
	// int over a phrase: the fetcher's Fields value is not a retention at all.
	// The retention policy is one field carrying words: "30 days".
	{"logs", "/aws/lambda/process-orders", "Retention", "30 days"},
	// timestamp: the fetcher writes a date and no time of day, and the cell
	// says exactly that, with no "00:00" the fetcher never reported.
	{"secrets", "prod/app/long-lived-signing-key", "Last Changed", "2024-12-01"},
	{"secrets", "prod/payments/stripe-webhook-secret", "Last Accessed", "2026-04-28"},
	// The identifier column: the defaults read the fetcher's task_id, so a
	// column headed "Task ID" and 38 wide shows the ID rather than a
	// 78-character ARN cut in half.
	{"ecs-task", "a1b2c3d4e5f6a1b2c3d4e5f6", "Task ID", "a1b2c3d4e5f6a1b2c3d4e5f6"},
	// The status column, which the save lane excludes outright: the fetcher
	// left no lifecycle key behind, so the replayed cell is empty.
	{"kinesis", "clickstream-ingest", "Status", "active"},
}

// TestReplay_ShadowedColumnKeepsItsLiveWordAfterARestart pins the exact string
// each of the shadowed columns owes, on both frames. The equality gate is
// direction-blind by construction; this is the half that says which value is
// the right one to converge on.
func TestReplay_ShadowedColumnKeepsItsLiveWordAfterARestart(t *testing.T) {
	byType := map[string][]replayShadowedCell{}
	for _, c := range replayShadowedCells {
		byType[c.shortName] = append(byType[c.shortName], c)
	}

	for shortName, cells := range byType {
		t.Run(shortName, func(t *testing.T) {
			td := resource.FindResourceType(shortName)
			if td == nil {
				t.Fatalf("%s is not a registered resource type", shortName)
			}
			liveRows, replayRows, titles := colsRoundTrip(t, *td, "replay-shadow-")

			for _, c := range cells {
				idx := replayColumnIndex(titles, c.column)
				if idx < 0 {
					t.Errorf("%s: no column titled %q in the rendered list", shortName, c.column)
					continue
				}
				if row, ok := liveRows[c.id]; !ok || idx >= len(row.Cells) {
					t.Errorf("%s: the live frame has no row %q", shortName, c.id)
				} else if row.Cells[idx] != c.want {
					t.Errorf("%s row %q col %q: the live fetch renders %q, want %q",
						shortName, c.id, c.column, row.Cells[idx], c.want)
				}
				if row, ok := replayRows[c.id]; !ok || idx >= len(row.Cells) {
					t.Errorf("%s: the replayed frame has no row %q", shortName, c.id)
				} else if row.Cells[idx] != c.want {
					t.Errorf("%s row %q col %q: after a restart it renders %q, want %q",
						shortName, c.id, c.column, row.Cells[idx], c.want)
				}
			}
		})
	}
}

func replayColumnIndex(titles []string, title string) int {
	for i, t := range titles {
		if t == title {
			return i
		}
	}
	return -1
}

// TestReplay_SavedRowLeavesOneAnswerForAColumnTitle pins that a replayed cell
// is deterministic. A replayed row has no RawStruct, so a column that cannot
// answer from its own Key falls to the title-match loop in
// app.ExtractCellValue, which scans the Fields map for a key equal to the
// lowercased title or to that title with underscores for spaces. That scan is
// a `range` over a map: when both spellings are present with different values,
// which one reaches the screen is decided by Go's randomized map order, and
// the same cache file renders two different lists on two consecutive starts.
//
// The save lane is what creates the collision — it materializes under the
// spaced spelling while the fetcher already wrote the underscored one — so the
// save is where the ambiguity has to be resolved: at most one value may answer
// to a column's title.
func TestReplay_SavedRowLeavesOneAnswerForAColumnTitle(t *testing.T) {
	clients := demo.NewServiceClients()

	for _, td := range resource.AllResourceTypes() {
		t.Run(td.ShortName, func(t *testing.T) {
			rows, ok := unit.DrainFixtures(t, td, clients)
			if !ok || len(rows) == 0 {
				t.Skipf("%s: no demo rows to drain", td.ShortName)
			}

			profile, region := "replay-title-"+td.ShortName, "us-east-1"
			ctrl := newTestControllerForProfile(t, profile, region)
			ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})
			ctrl.ApplyResourcesLoaded(td.ShortName, rows, nil, false)

			ctrl.WaitForCacheWrites()
			store := cache.LoadDirForTest(profile, region)
			tf, found := store.Type(td.ShortName)
			if !found || len(tf.Rows) == 0 {
				t.Fatalf("%s: the list-open save persisted nothing", td.ShortName)
			}

			columns := resource.ResolveListColumnCascade(nil, td.ShortName, &td)
			for _, saved := range tf.Rows {
				for _, col := range columns {
					if col.Key != "" && saved.Fields[col.Key] != "" {
						continue
					}
					titleLower := strings.ToLower(col.Title)
					titleUnder := strings.ReplaceAll(titleLower, " ", "_")
					answers := map[string]string{}
					for k, v := range saved.Fields {
						kl := strings.ToLower(k)
						if kl == titleLower || kl == titleUnder {
							answers[v] = k
						}
					}
					if len(answers) > 1 {
						t.Errorf("%s row %q col %q: the saved row answers to its title %d ways %v; the replayed cell is whichever the map hands back first",
							td.ShortName, saved.ID, col.Title, len(answers), answers)
					}
				}
			}
		})
	}
}

// replayKeyedPathColumn reports a type's first non-status column that declares
// both a Key and a Path — the shape where Fields already holds a value the SDK
// struct cannot see (a Wave-2 override), so the save lane must not overwrite it
// with the struct scalar.
func replayKeyedPathColumn(td resource.ResourceTypeDef) (config.ListColumn, bool) {
	lifecycleKey := td.LifecycleKey
	if lifecycleKey == "" {
		lifecycleKey = "state"
	}
	for _, col := range resource.ResolveListColumnCascade(nil, td.ShortName, &td) {
		if col.Key == "" || col.Path == "" {
			continue
		}
		if col.Key == "status" || col.Key == lifecycleKey {
			continue
		}
		return col, true
	}
	return config.ListColumn{}, false
}

// TestReplay_KeyedColumnValueSurvivesTheSave pins the boundary of the save
// lane's write. A keyed column's Fields entry can hold a value no SDK struct
// carries — a Wave-2 enrichment result is the everyday case — and the live
// frame renders that value, not the struct's. If the save lane overwrites a
// populated key with the struct scalar on its way to disk, the restart drops
// the enrichment and shows the raw field instead.
func TestReplay_KeyedColumnValueSurvivesTheSave(t *testing.T) {
	clients := demo.NewServiceClients()
	const override = "overridden-by-wave-2"

	covered := 0
	for _, td := range resource.AllResourceTypes() {
		col, ok := replayKeyedPathColumn(td)
		if !ok {
			continue
		}
		covered++
		t.Run(td.ShortName, func(t *testing.T) {
			rows, ok := unit.DrainFixtures(t, td, clients)
			if !ok || len(rows) == 0 {
				t.Skipf("%s: no demo rows to drain", td.ShortName)
			}
			fields := make(map[string]string, len(rows[0].Fields)+1)
			maps.Copy(fields, rows[0].Fields)
			fields[col.Key] = override
			rows[0].Fields = fields
			id := rows[0].ID

			profile, region := "replay-keyed-"+td.ShortName, "us-east-1"
			ctrl := newTestControllerForProfile(t, profile, region)
			ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})
			ctrl.ApplyResourcesLoaded(td.ShortName, rows, nil, false)

			ctrl.WaitForCacheWrites()
			store := cache.LoadDirForTest(profile, region)
			tf, found := store.Type(td.ShortName)
			if !found {
				t.Fatalf("%s: the list-open save persisted nothing", td.ShortName)
			}
			for _, saved := range tf.Rows {
				if saved.ID != id {
					continue
				}
				if got := saved.Fields[col.Key]; got != override {
					t.Fatalf("%s row %q: column %q was saved as %q, want the value the live frame rendered (%q)",
						td.ShortName, id, col.Title, got, override)
				}
				return
			}
			t.Fatalf("%s: row %q was not persisted at all", td.ShortName, id)
		})
	}
	if covered == 0 {
		t.Fatal("no registered type resolves a non-status column with both a Key and a Path; the save lane's overwrite boundary is untested")
	}
}

// TestReplay_CacheFileWrittenByThePreviousVersionStillRendersItsWord pins the
// upgrade path. The disk schema version did not change, so the first start
// after this fix reads type files the PREVIOUS build wrote, and those rows
// carry both spellings of a multi-word column's title: the spaced one its save
// lane materialized (holding the value that build's screen showed) and the
// underscored one the fetcher wrote (holding the raw scalar).
//
// The cold-start paint renders those rows before any fetch lands, so the word
// on screen has to be the one the previous run showed. Whichever spelling the
// extraction cascade prefers, it has to prefer the one that survives on both
// shapes of file: a file this build writes carries the underscored key only,
// so looking for the spaced key first costs nothing there and is the only
// reading that is right on a file the last build wrote.
func TestReplay_CacheFileWrittenByThePreviousVersionStillRendersItsWord(t *testing.T) {
	const shortName = "secrets"
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s is not a registered resource type", shortName)
	}

	ctrl := newTestControllerForProfile(t, "replay-legacy", "us-east-1")
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	ctrl.ApplyResourcesLoaded(shortName, []resource.Resource{{
		ID:   "prod/app/legacy-cache-row",
		Name: "prod/app/legacy-cache-row",
		Type: shortName,
		Fields: map[string]string{
			"secret_name": "prod/app/legacy-cache-row",
			// What the previous build's save lane left behind.
			"last accessed": "2026-04-28 00:00",
			// What its fetcher left beside it.
			"last_accessed": "2026-04-28",
		},
	}}, nil, false)

	body := ctrl.Snapshot().Body.List
	if body == nil || len(body.Rows) != 1 {
		t.Fatalf("the legacy row rendered no list body")
	}
	titles := make([]string, 0, len(body.Columns))
	for _, col := range body.Columns {
		titles = append(titles, col.Title)
	}
	idx := replayColumnIndex(titles, "Last Accessed")
	if idx < 0 {
		t.Fatal("no column titled \"Last Accessed\" in the rendered list")
	}
	// The subject is which of the two keys wins, and the fetcher's own does.
	// The expected string carries no midnight the fetcher never reported.
	if got := body.Rows[0].Cells[idx]; got != "2026-04-28" {
		t.Errorf("a row from the previous build's cache renders Last Accessed as %q, want %q — the fetcher's own key still wins",
			got, "2026-04-28")
	}
}

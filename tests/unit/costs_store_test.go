package unit_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/costs"
)

// costs.Store is not given exact method signatures in data-model.md (only
// the on-disk YAML shape and the closed/open/merge/anomaly-TTL policy
// prose). The surface exercised below mirrors the existing
// internal/cache.Store convention (LoadDir/Put/SaveType) adapted to the
// single-file-per-profile layout in data-model.md:
//
//	costs.CachePath(profile string) string
//	costs.LoadStore(profile string) *costs.Store
//	(*costs.Store).Recovered() bool
//	(*costs.Store).Lookup(q costs.Query, window []costs.Period, now time.Time) (records []costs.Record, missing []costs.Period)
//	(*costs.Store).Merge(q costs.Query, recs []costs.Record, now time.Time)
//	(*costs.Store).Attrs() map[string]string
//	(*costs.Store).MergeAttrs(attrs map[string]string)
//	(*costs.Store).Anomalies(now time.Time) (marks []costs.AnomalyMark, ok bool)
//	(*costs.Store).PutAnomalies(marks []costs.AnomalyMark, now time.Time, covered costs.Period)
//	(*costs.Store).Save() error

func ec2MonthlyQuery() costs.Query {
	return costs.Query{
		Granularity: "MONTHLY",
		GroupBy:     []costs.Dimension{costs.Dimension("SERVICE")},
		Range:       costs.Period{Start: "2026-01-01", End: "2027-01-01"},
	}
}

func sampleAnomalyMark() costs.AnomalyMark {
	return costs.AnomalyMark{
		ID:        "anomaly-1a2b3c",
		Score:     92.0,
		Impact:    costs.Amount{Value: 452.10, Unit: "USD"},
		Period:    costs.Period{Start: "2026-07-01", End: "2026-07-08"},
		RootCause: "SERVICE=Amazon Elastic Compute Cloud - Compute, REGION=us-east-1, USAGE_TYPE=BoxUsage:m5.2xlarge",
		Dimension: map[costs.Dimension]string{
			costs.Dimension("SERVICE"):    "Amazon Elastic Compute Cloud - Compute",
			costs.Dimension("REGION"):     "us-east-1",
			costs.Dimension("USAGE_TYPE"): "BoxUsage:m5.2xlarge",
		},
	}
}

func TestStore_SaveLoadRoundTrip_PreservesRecordsAttrsAnomalies(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	q := ec2MonthlyQuery()
	period := costs.Period{Start: "2026-01-01", End: "2026-02-01"}
	recs := []costs.Record{
		{
			Period: period,
			Keys:   []string{"Amazon Elastic Compute Cloud - Compute"},
			Metrics: map[costs.Metric]costs.Amount{
				costs.Metric("invoice"): {Value: 123.45, Unit: "USD"},
			},
		},
	}

	s := costs.LoadStore("prod-account")
	s.Merge(q, recs, now)
	s.MergeAttrs(map[string]string{"123456789012": "prod-account"})
	s.PutAnomalies([]costs.AnomalyMark{sampleAnomalyMark()}, now, q.Range)
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded := costs.LoadStore("prod-account")
	gotRecs, missing := reloaded.Lookup(q, []costs.Period{period}, now)
	if len(missing) != 0 {
		t.Errorf("Lookup() missing = %v, want none", missing)
	}
	if len(gotRecs) != 1 {
		t.Fatalf("Lookup() records = %d, want 1", len(gotRecs))
	}
	if got := gotRecs[0].Metrics[costs.Metric("invoice")].Value; got != 123.45 {
		t.Errorf("reloaded record invoice value = %v, want 123.45", got)
	}

	if got := reloaded.Attrs()["123456789012"]; got != "prod-account" {
		t.Errorf("Attrs()[123456789012] = %q, want %q", got, "prod-account")
	}

	marks, ok := reloaded.Anomalies(now)
	if !ok {
		t.Fatal("Anomalies() ok = false, want true (just saved, within TTL)")
	}
	if len(marks) != 1 || marks[0].ID != "anomaly-1a2b3c" {
		t.Errorf("Anomalies() = %+v, want the persisted mark", marks)
	}
}

// TestStore_Lookup_ImmutabilityKeysOnFetchedAfterClosure supersedes the
// original TestStore_Lookup_ClosedPeriodReturnedRegardlessOfFetchedAtAge,
// which pinned the OLD rule (a period is immutable once it is closed
// RELATIVE TO THE LOOKUP CALL's own now, regardless of when the cached
// bucket was actually fetched). Round 8 finding 1 identified that rule as
// a bug: a bucket cached WHILE its period was still open (a normal,
// necessarily-partial open-period fetch) would wrongly become immutable
// the instant a LATER Lookup call's own clock rolled past the period's
// end — even though that bucket was never re-fetched after the period
// actually closed.
//
// The new single predicate: immutability keys on fetched-AFTER-closure
// (bucket.FetchedAt on or after the period's own End), not on
// "period.Closed(relative to THIS lookup's now)".
//
// Reconciled (self-review C5): immutability now additionally requires a
// 72h settlement lag past End (CE revises data 24-72h post-close). This
// test's "post-closure fetch" subtest fetches 4 days (96h) after End,
// already past that lag, so its "immutable" assertion stays valid
// unchanged — see costs_selfreview_test.go's C5a/C5b tests for the
// narrower boundary (a fetch within the 72h lag) this test does not probe.
func TestStore_Lookup_ImmutabilityKeysOnFetchedAfterClosure(t *testing.T) {
	period := costs.Period{Start: "2025-01-01", End: "2025-02-01"}
	q := ec2MonthlyQuery()
	lookupNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	t.Run("mid-period fetch is refetchable once the period closes", func(t *testing.T) {
		t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
		midPeriodFetchTime := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC) // before period.End (2025-02-01) — fetched while still open
		recs := []costs.Record{
			{Period: period, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{costs.Metric("invoice"): {Value: 88, Unit: "USD"}}},
		}

		s := costs.LoadStore("prod-account")
		s.Merge(q, recs, midPeriodFetchTime)

		_, missing := s.Lookup(q, []costs.Period{period}, lookupNow)
		if len(missing) == 0 {
			t.Error("Lookup() reports no missing periods — a bucket fetched WHILE its period was still open must be refetchable once the period closes, not treated as immutable just because the lookup's own now has since passed the period")
		}
	})

	t.Run("post-closure fetch is returned regardless of fetched-at age", func(t *testing.T) {
		t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
		postClosureFetchTime := time.Date(2025, 2, 5, 0, 0, 0, 0, time.UTC) // after period.End (2025-02-01) — fetched after closure
		recs := []costs.Record{
			{Period: period, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{costs.Metric("invoice"): {Value: 88, Unit: "USD"}}},
		}

		s := costs.LoadStore("prod-account")
		s.Merge(q, recs, postClosureFetchTime)

		got, missing := s.Lookup(q, []costs.Period{period}, lookupNow)
		if len(missing) != 0 {
			t.Errorf("Lookup() missing = %v, want none (fetched after closure, so immutable regardless of how old the fetch is)", missing)
		}
		if len(got) != 1 {
			t.Fatalf("Lookup() records = %d, want 1", len(got))
		}
	})
}

func TestStore_Lookup_OpenPeriodOnlyWithinTTL(t *testing.T) {
	openPeriod := costs.Period{Start: "2026-07-01", End: "2026-08-01"}
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	q := ec2MonthlyQuery()
	recs := []costs.Record{
		{Period: openPeriod, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{costs.Metric("invoice"): {Value: 42, Unit: "USD"}}},
	}

	t.Run("fresh fetch within 24h is usable", func(t *testing.T) {
		t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
		s := costs.LoadStore("prod-account")
		s.Merge(q, recs, now.Add(-1*time.Hour))

		got, missing := s.Lookup(q, []costs.Period{openPeriod}, now)
		if len(missing) != 0 {
			t.Errorf("Lookup() missing = %v, want none", missing)
		}
		if len(got) != 1 {
			t.Fatalf("Lookup() records = %d, want 1", len(got))
		}
	})

	t.Run("stale fetch older than 24h is treated as missing", func(t *testing.T) {
		t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
		s := costs.LoadStore("prod-account")
		s.Merge(q, recs, now.Add(-25*time.Hour))

		got, missing := s.Lookup(q, []costs.Period{openPeriod}, now)
		if len(missing) != 1 {
			t.Errorf("Lookup() missing = %v, want [%v] (stale open period)", missing, openPeriod)
		}
		if len(got) != 0 {
			t.Errorf("Lookup() records = %v, want none for a stale open period", got)
		}
	})
}

func TestStore_Merge_OpenOverwritesClosedNeverReenters(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	q := ec2MonthlyQuery()
	key := []string{"Amazon Elastic Compute Cloud - Compute"}

	t.Run("closed period never re-enters once present", func(t *testing.T) {
		s := costs.LoadStore("prod-account")
		closedPeriod := costs.Period{Start: "2025-01-01", End: "2025-02-01"}
		mergeTime := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)

		s.Merge(q, []costs.Record{{Period: closedPeriod, Keys: key, Metrics: map[costs.Metric]costs.Amount{costs.Metric("invoice"): {Value: 100, Unit: "USD"}}}}, mergeTime)
		s.Merge(q, []costs.Record{{Period: closedPeriod, Keys: key, Metrics: map[costs.Metric]costs.Amount{costs.Metric("invoice"): {Value: 999, Unit: "USD"}}}}, mergeTime)

		got, _ := s.Lookup(q, []costs.Period{closedPeriod}, mergeTime)
		if len(got) != 1 {
			t.Fatalf("Lookup() records = %d, want 1", len(got))
		}
		if v := got[0].Metrics[costs.Metric("invoice")].Value; v != 100 {
			t.Errorf("closed-period value after re-merge = %v, want 100 (first write wins, closed never re-enters)", v)
		}
	})

	t.Run("open period is overwritten by a later merge", func(t *testing.T) {
		s := costs.LoadStore("prod-account")
		openPeriod := costs.Period{Start: "2026-07-01", End: "2026-08-01"}
		t1 := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
		t2 := time.Date(2026, 7, 10, 20, 0, 0, 0, time.UTC)

		s.Merge(q, []costs.Record{{Period: openPeriod, Keys: key, Metrics: map[costs.Metric]costs.Amount{costs.Metric("invoice"): {Value: 50, Unit: "USD"}}}}, t1)
		s.Merge(q, []costs.Record{{Period: openPeriod, Keys: key, Metrics: map[costs.Metric]costs.Amount{costs.Metric("invoice"): {Value: 75, Unit: "USD"}}}}, t2)

		got, _ := s.Lookup(q, []costs.Period{openPeriod}, t2)
		if len(got) != 1 {
			t.Fatalf("Lookup() records = %d, want 1", len(got))
		}
		if v := got[0].Metrics[costs.Metric("invoice")].Value; v != 75 {
			t.Errorf("open-period value after re-merge = %v, want 75 (open periods overwrite)", v)
		}
	})
}

func TestStore_CorruptYAML_RenamedToBakFreshStoreNoPanic(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile := "corrupt-account"
	path := costs.CachePath(profile)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	corrupt := []byte("version: 1\nqueries: [this is not, valid: yaml\n")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s := costs.LoadStore(profile)

	if !s.Recovered() {
		t.Error("Recovered() = false, want true after loading a corrupt cache file")
	}
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	if got := s.Attrs(); len(got) != 0 {
		t.Errorf("Attrs() = %v, want empty on a freshly recovered store", got)
	}
	if _, ok := s.Anomalies(now); ok {
		t.Error("Anomalies() ok = true, want false on a freshly recovered store")
	}

	if _, err := os.Stat(path); err == nil {
		t.Error("original corrupt cache file still present at the primary path, want it renamed away")
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("reading .bak: %v", err)
	}
	if string(bak) != string(corrupt) {
		t.Errorf(".bak content = %q, want the original corrupt bytes %q", bak, corrupt)
	}
}

func TestStore_AlienVersion_RenamedToBakFreshStoreNoPanic(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile := "alien-version-account"
	path := costs.CachePath(profile)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	alien := []byte("version: 999\nqueries: {}\n")
	if err := os.WriteFile(path, alien, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s := costs.LoadStore(profile)

	if !s.Recovered() {
		t.Error("Recovered() = false, want true after loading an alien-version cache file")
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("reading .bak: %v", err)
	}
	if string(bak) != string(alien) {
		t.Errorf(".bak content = %q, want the original alien-version bytes %q", bak, alien)
	}
}

func TestStore_Anomalies_24hTTL(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := costs.LoadStore("prod-account")
	fetchedAt := time.Date(2026, 7, 10, 8, 0, 0, 0, time.UTC)
	mark := sampleAnomalyMark()

	if _, ok := s.Anomalies(fetchedAt); ok {
		t.Error("Anomalies() ok = true on an empty store, want false")
	}

	s.PutAnomalies([]costs.AnomalyMark{mark}, fetchedAt, mark.Period)

	if _, ok := s.Anomalies(fetchedAt.Add(23 * time.Hour)); !ok {
		t.Error("Anomalies() ok = false at 23h old, want true (within 24h TTL)")
	}
	if _, ok := s.Anomalies(fetchedAt.Add(25 * time.Hour)); ok {
		t.Error("Anomalies() ok = true at 25h old, want false (past 24h TTL)")
	}
}

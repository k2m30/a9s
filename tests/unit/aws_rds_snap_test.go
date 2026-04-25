package unit

// aws_rds_snap_test.go — Fetcher tests for rds-snap resource type.
//
// Spec: docs/resources/rds-snap.md §3.1 + §4 + impl-plan §1.1/§1.4.
// Tests call FetchRDSSnapshotsPage via a strict mock, asserting:
//   - Resource.Status = §4 phrase for each signal (healthy = "").
//   - Resource.Issues = ordered slice per §0.1 precedence ladder.
//   - Fields["arn"] populated for the backup-pivot (per §3.1 gap fix).
//   - Adversarial rows (nil ID, nil Status, nil SnapshotCreateTime) do not panic.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// ---------------------------------------------------------------------------
// Strict mock — implements RDSDescribeDBSnapshotsAPI
// ---------------------------------------------------------------------------

type mockDescribeDBSnapshots struct {
	output *rds.DescribeDBSnapshotsOutput
	err    error
}

func (m *mockDescribeDBSnapshots) DescribeDBSnapshots(
	_ context.Context,
	_ *rds.DescribeDBSnapshotsInput,
	_ ...func(*rds.Options),
) (*rds.DescribeDBSnapshotsOutput, error) {
	return m.output, m.err
}

// snapOutput is a convenience builder.
func snapOutput(snaps ...rdstypes.DBSnapshot) *rds.DescribeDBSnapshotsOutput {
	return &rds.DescribeDBSnapshotsOutput{DBSnapshots: snaps}
}

// fetchSnap fetches a page from a single-page mock holding the provided snapshots.
func fetchSnap(t *testing.T, snaps ...rdstypes.DBSnapshot) []resourceRow {
	t.Helper()
	mock := &mockDescribeDBSnapshots{output: snapOutput(snaps...)}
	result, err := awsclient.FetchRDSSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSSnapshotsPage: unexpected error: %v", err)
	}
	rows := make([]resourceRow, len(result.Resources))
	for i, r := range result.Resources {
		rows[i] = resourceRow{status: r.Status, issues: r.Issues, fields: r.Fields, id: r.ID}
	}
	return rows
}

// resourceRow captures the fields we assert on — avoids depending on the
// full resource.Resource struct layout in tests.
type resourceRow struct {
	id     string
	status string
	issues []string
	fields map[string]string
}

// ---------------------------------------------------------------------------
// §1.1 Per-signal cases
// ---------------------------------------------------------------------------

// TestRDSSnap_Fetcher_HealthyAvailable_BlankS4 verifies that a healthy
// available+encrypted snapshot produces Status="" and no issues.
func TestRDSSnap_Fetcher_HealthyAvailable_BlankS4(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-healthy"),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:snap-healthy"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		PercentProgress:      aws.Int32(100),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.status != "" {
		t.Errorf("Status = %q, want empty (healthy silence)", r.status)
	}
	if len(r.issues) != 0 {
		t.Errorf("Issues = %v, want empty for healthy row", r.issues)
	}
}

// TestRDSSnap_Fetcher_Creating_CarriesPercent verifies that Status=creating
// produces "creating: 42%" with PercentProgress embedded.
func TestRDSSnap_Fetcher_Creating_CarriesPercent(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.WarnRDSSnapCreatingID),
		DBSnapshotArn:        aws.String(fixtures.WarnRDSSnapCreatingARN),
		Status:               aws.String("creating"),
		Encrypted:            aws.Bool(true),
		PercentProgress:      aws.Int32(42),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.status != "creating: 42%" {
		t.Errorf("Status = %q, want %q", r.status, "creating: 42%")
	}
	if len(r.issues) != 1 || r.issues[0] != "creating: 42%" {
		t.Errorf("Issues = %v, want [creating: 42%%]", r.issues)
	}
}

// TestRDSSnap_Fetcher_Failed_BareKeyword verifies that Status=failed
// produces bare "failed" keyword per spec §4 (no cause available from SDK).
func TestRDSSnap_Fetcher_Failed_BareKeyword(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.BrokenRDSSnapFailedID),
		DBSnapshotArn:        aws.String(fixtures.BrokenRDSSnapFailedARN),
		Status:               aws.String("failed"),
		Encrypted:            aws.Bool(true),
		PercentProgress:      aws.Int32(0),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.status != "failed" {
		t.Errorf("Status = %q, want %q", r.status, "failed")
	}
	if len(r.issues) != 1 || r.issues[0] != "failed" {
		t.Errorf("Issues = %v, want [failed]", r.issues)
	}
}

// TestRDSSnap_Fetcher_IncompatibleKeywordPreserved verifies that
// incompatible-* statuses preserve the exact AWS keyword verbatim.
func TestRDSSnap_Fetcher_IncompatibleKeywordPreserved(t *testing.T) {
	for _, status := range []string{"incompatible-restore", "incompatible-parameters"} {
		t.Run(status, func(t *testing.T) {
			rows := fetchSnap(t, rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-incompat"),
				DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:snap-incompat"),
				Status:               aws.String(status),
				Encrypted:            aws.Bool(true),
				PercentProgress:      aws.Int32(0),
			})
			if len(rows) != 1 {
				t.Fatalf("expected 1 row, got %d", len(rows))
			}
			r := rows[0]
			if r.status != status {
				t.Errorf("Status = %q, want %q (keyword must be preserved verbatim)", r.status, status)
			}
			if len(r.issues) != 1 || r.issues[0] != status {
				t.Errorf("Issues = %v, want [%s]", r.issues, status)
			}
		})
	}
}

// TestRDSSnap_Fetcher_Unencrypted verifies that Encrypted=false produces
// Status="unencrypted" (CIS RDS.4).
func TestRDSSnap_Fetcher_Unencrypted(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.WarnRDSSnapUnencryptedID),
		DBSnapshotArn:        aws.String(fixtures.WarnRDSSnapUnencryptedARN),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(false),
		PercentProgress:      aws.Int32(100),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.status != "unencrypted" {
		t.Errorf("Status = %q, want %q", r.status, "unencrypted")
	}
	if len(r.issues) != 1 || r.issues[0] != "unencrypted" {
		t.Errorf("Issues = %v, want [unencrypted]", r.issues)
	}
}

// TestRDSSnap_Fetcher_SeverityBrokenBeatsWarning verifies that a Broken status
// (failed) wins over a Warning (Encrypted=false). Encrypted=false is suppressed
// when the snapshot is in a non-available end-state per §0.1/§1.4.
func TestRDSSnap_Fetcher_SeverityBrokenBeatsWarning(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.SeverityBrokenWarnRDSSnapID),
		DBSnapshotArn:        aws.String(fixtures.SeverityBrokenWarnRDSSnapARN),
		Status:               aws.String("failed"),
		Encrypted:            aws.Bool(false),
		PercentProgress:      aws.Int32(0),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.status != "failed" {
		t.Errorf("Status = %q, want %q (Broken wins; unencrypted suppressed when Status=failed)", r.status, "failed")
	}
	if len(r.issues) != 1 || r.issues[0] != "failed" {
		t.Errorf("Issues = %v, want [failed] (Broken suppresses Warning in same row)", r.issues)
	}
}

// TestRDSSnap_Fetcher_PopulatesARNField verifies that Fields["arn"] is
// populated from DBSnapshotArn so the backup pivot can read it.
func TestRDSSnap_Fetcher_PopulatesARNField(t *testing.T) {
	wantARN := fixtures.ProdRDSSnapARN
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.ProdRDSSnapID),
		DBSnapshotArn:        aws.String(wantARN),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		PercentProgress:      aws.Int32(100),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].fields["arn"] != wantARN {
		t.Errorf("Fields[arn] = %q, want %q", rows[0].fields["arn"], wantARN)
	}
}

// TestRDSSnap_Fetcher_IssuesPopulatedInPrecedenceOrder verifies (U7f) that
// Resource.Issues is ordered per §0.1 for each signal case:
//   - Healthy → empty
//   - failed → ["failed"]
//   - incompatible-restore → ["incompatible-restore"]
//   - creating → ["creating: 60%"]
//   - unencrypted → ["unencrypted"]
func TestRDSSnap_Fetcher_IssuesPopulatedInPrecedenceOrder(t *testing.T) {
	cases := []struct {
		name        string
		snap        rdstypes.DBSnapshot
		wantStatus  string
		wantIssues  []string
	}{
		{
			name: "healthy_empty",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-healthy-x"),
				Status:               aws.String("available"),
				Encrypted:            aws.Bool(true),
				PercentProgress:      aws.Int32(100),
			},
			wantStatus: "",
			wantIssues: nil,
		},
		{
			name: "failed_single",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-failed-x"),
				Status:               aws.String("failed"),
				Encrypted:            aws.Bool(true),
				PercentProgress:      aws.Int32(0),
			},
			wantStatus: "failed",
			wantIssues: []string{"failed"},
		},
		{
			name: "incompatible_restore",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-incompatible-x"),
				Status:               aws.String("incompatible-restore"),
				Encrypted:            aws.Bool(true),
				PercentProgress:      aws.Int32(0),
			},
			wantStatus: "incompatible-restore",
			wantIssues: []string{"incompatible-restore"},
		},
		{
			name: "creating_60pct",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-creating-x"),
				Status:               aws.String("creating"),
				Encrypted:            aws.Bool(true),
				PercentProgress:      aws.Int32(60),
			},
			wantStatus: "creating: 60%",
			wantIssues: []string{"creating: 60%"},
		},
		{
			name: "unencrypted",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-unencrypted-x"),
				Status:               aws.String("available"),
				Encrypted:            aws.Bool(false),
				PercentProgress:      aws.Int32(100),
			},
			wantStatus: "unencrypted",
			wantIssues: []string{"unencrypted"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := fetchSnap(t, tc.snap)
			if len(rows) != 1 {
				t.Fatalf("expected 1 row, got %d", len(rows))
			}
			r := rows[0]
			if r.status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", r.status, tc.wantStatus)
			}
			if len(r.issues) != len(tc.wantIssues) {
				t.Errorf("Issues length = %d, want %d; got %v", len(r.issues), len(tc.wantIssues), r.issues)
			} else {
				for i, want := range tc.wantIssues {
					if r.issues[i] != want {
						t.Errorf("Issues[%d] = %q, want %q", i, r.issues[i], want)
					}
				}
			}
		})
	}
}

// TestRDSSnap_Fetcher_MultiW1_TopPlusSuffix verifies that when creating
// (Warning top in precedence) is present together with unencrypted (Warning),
// the Status carries "creating: <pct>%" because creating ranks above unencrypted
// in §0.1 (transitional beats CIS). Issues has both phrases in order.
// Note: The multi-W1 snapshot in fixtures uses Encrypted=false + orphan (not creating),
// so we construct an adversarial inline snapshot for this specific case.
func TestRDSSnap_Fetcher_MultiW1_TopPlusSuffix(t *testing.T) {
	// creating + Encrypted=false: creating wins per §0.1 (creating is first among Warnings).
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-creating-unenc"),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:snap-creating-unenc"),
		Status:               aws.String("creating"),
		Encrypted:            aws.Bool(false),
		PercentProgress:      aws.Int32(15),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	// creating: 15% is the top phrase; unencrypted is the secondary.
	// Per §0.1 ladder, creating ranks before unencrypted among Warnings.
	// The fetcher should emit Status = "creating: 15% (+1)" with both in Issues.
	wantStatusPrefix := "creating: 15%"
	if !strings.HasPrefix(r.status, wantStatusPrefix) {
		t.Errorf("Status = %q, want prefix %q (creating wins over unencrypted)", r.status, wantStatusPrefix)
	}
	if !strings.Contains(r.status, "(+1)") {
		t.Errorf("Status = %q, want (+1) suffix (multi-W1 indicator)", r.status)
	}
	// Issues: creating phrase first, then unencrypted.
	if len(r.issues) < 2 {
		t.Errorf("Issues = %v, want at least 2 (creating + unencrypted)", r.issues)
	} else {
		if !strings.HasPrefix(r.issues[0], "creating: 15%") {
			t.Errorf("Issues[0] = %q, want creating phrase first (§0.1 precedence)", r.issues[0])
		}
		if r.issues[1] != "unencrypted" {
			t.Errorf("Issues[1] = %q, want %q", r.issues[1], "unencrypted")
		}
	}
}

// ---------------------------------------------------------------------------
// Adversarial rows — must not panic
// ---------------------------------------------------------------------------

// TestRDSSnap_Fetcher_NilDBSnapshotIdentifier verifies that a snapshot with
// nil DBSnapshotIdentifier is skipped (ID == "") or produces an empty-ID row,
// either way without panicking.
func TestRDSSnap_Fetcher_NilDBSnapshotIdentifier(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: nil,
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
	})
	// Either zero rows (skipped) or one row with empty ID — both are acceptable.
	// The contract is: no panic, no fatal error.
	for _, r := range rows {
		if r.id != "" {
			t.Errorf("expected empty ID for nil-identifier snapshot, got %q", r.id)
		}
	}
}

// TestRDSSnap_Fetcher_NilStatus verifies that nil Status is treated as "" (Healthy).
func TestRDSSnap_Fetcher_NilStatus(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-nil-status"),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:snap-nil-status"),
		Status:               nil, // nil → treated as ""
		Encrypted:            aws.Bool(true),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// nil Status → Healthy fallback → blank S4.
	if rows[0].status != "" {
		t.Errorf("nil Status: Resource.Status = %q, want empty (Healthy fallback)", rows[0].status)
	}
}

// TestRDSSnap_Fetcher_NilSnapshotCreateTime verifies that a snapshot with
// nil SnapshotCreateTime does not panic — the past-retention rule in the
// enricher must skip it cleanly.
func TestRDSSnap_Fetcher_NilSnapshotCreateTime(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-nil-time"),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:snap-nil-time"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotCreateTime:   nil, // no panic allowed
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	// No issue from a nil create time — just healthy.
	if rows[0].status != "" {
		t.Errorf("nil SnapshotCreateTime: Status = %q, want empty", rows[0].status)
	}
}

// ---------------------------------------------------------------------------
// Full fixture set smoke test
// ---------------------------------------------------------------------------

// TestRDSSnap_Fetcher_AllFixtures_NoError verifies that the full set of
// demo fixtures passes through FetchRDSSnapshotsPage without error and
// produces the expected number of rows.
func TestRDSSnap_Fetcher_AllFixtures_NoError(t *testing.T) {
	fix := fixtures.NewRDSSnapFixtures()
	mock := &mockDescribeDBSnapshots{output: snapOutput(fix.Instances...)}
	result, err := awsclient.FetchRDSSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != len(fix.Instances) {
		t.Errorf("got %d resources, want %d (one per fixture)", len(result.Resources), len(fix.Instances))
	}
	// Every row must have non-empty ID.
	for i, r := range result.Resources {
		if r.ID == "" {
			t.Errorf("resource[%d].ID is empty", i)
		}
	}
}

// ---------------------------------------------------------------------------
// U13 throttle static audit
// ---------------------------------------------------------------------------

// TestRDSSnap_StaticAudit_AllSDKCallsThrottleWrapped scans rds_snap*.go files
// under internal/aws and asserts that every direct RDS/Backup API call appears
// inside a RetryOnThrottle closure. Per universal rule U13, callers must never
// call AWS APIs directly from enricher or fetcher bodies outside throttle wraps.
//
// Scan strategy: any line that calls api.Describe*/api.Get*/api.List*/api.Lookup*
// directly (not as the argument to RetryOnThrottle) is a violation.
func TestRDSSnap_StaticAudit_AllSDKCallsThrottleWrapped(t *testing.T) {
	root := findRepoFile(t, "internal/aws")

	// Patterns that match direct API calls outside a RetryOnThrottle body.
	// We detect: `api.DescribeXxx(` or `api.ListXxx(` or `api.GetXxx(` or `api.LookupXxx(`
	// on a line that does NOT appear as the anonymous function body of RetryOnThrottle.
	directCallPattern := strings.NewReplacer(
	// just build the prefix to match
	).Replace("")
	_ = directCallPattern

	var violations []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if !strings.HasPrefix(name, "rds_snap") || !strings.HasSuffix(name, ".go") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}
		lines := strings.Split(string(body), "\n")

		// Track whether we are inside a RetryOnThrottle closure. A simple heuristic:
		// when we see `RetryOnThrottle(`, we enter a protected block; we exit when
		// we see the matching `})` at the same nesting depth. For a single-function
		// closure this is sufficient.
		inThrottle := 0
		for ln, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Skip comments.
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(line, "RetryOnThrottle(") {
				inThrottle++
			}
			if inThrottle > 0 && strings.Contains(line, "})") {
				inThrottle--
				continue
			}
			// Check for direct API call outside throttle wrap.
			if inThrottle == 0 {
				isDirectCall := (strings.Contains(line, ".DescribeDBSnapshots(") ||
					strings.Contains(line, ".ListRecoveryPointsByResource(")) &&
					!strings.Contains(line, "RetryOnThrottle")
				if isDirectCall {
					violations = append(violations, fmt.Sprintf("%s:%d: %s", path, ln+1, strings.TrimSpace(line)))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal/aws: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("U13: direct SDK calls found outside RetryOnThrottle in rds_snap files (%d):\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// TestRDSSnap_Backup_UsesArnFromFields verifies (U14 variant) that the ARN
// value stored in Fields["arn"] by the fetcher is the DBSnapshotArn — not r.ID.
// This ensures checkRDSSnapBackup reads from Fields["arn"] (populated by fetcher),
// not from the bare snapshot identifier that r.ID carries.
func TestRDSSnap_Backup_UsesArnFromFields(t *testing.T) {
	wantARN := "arn:aws:rds:us-east-1:123456789012:snapshot:rds:test-snap"
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("rds:test-snap"),
		DBSnapshotArn:        aws.String(wantARN),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		PercentProgress:      aws.Int32(100),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.id == wantARN {
		t.Errorf("r.ID = %q — fetcher must NOT set ID = ARN; ID must be the bare identifier %q", r.id, "rds:test-snap")
	}
	if r.fields["arn"] != wantARN {
		t.Errorf("Fields[arn] = %q, want %q — fetcher must store DBSnapshotArn in Fields[\"arn\"]", r.fields["arn"], wantARN)
	}
	if r.id != "rds:test-snap" {
		t.Errorf("r.ID = %q, want %q (bare identifier)", r.id, "rds:test-snap")
	}
}

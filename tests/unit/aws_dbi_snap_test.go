package unit

// aws_rds_snap_test.go — Fetcher tests for dbi-snap resource type.
// // Spec: docs/resources/dbi-snap.md §3.1 + §4 + impl-plan §1.1/§1.4.
// Tests call FetchDBISnapshotsPage via a strict mock, asserting:
// - Resource.Status = "".
// - Resource.Fields["status"] = §4 phrase for each signal (healthy = "").
// - Resource.Findings = ordered slice per §0.1 precedence ladder (source: "wave1").
// - Fields["arn"] populated for the backup-pivot (per §3.1 gap fix).
// - Adversarial rows (nil ID, nil Status, nil SnapshotCreateTime) do not panic.

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
	"github.com/k2m30/a9s/v3/internal/domain"
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
// resourceRow.status is populated from r.Fields["status"] (not r.Fields["status"]).
func fetchSnap(t *testing.T, snaps ...rdstypes.DBSnapshot) []resourceRow {
	t.Helper()
	mock := &mockDescribeDBSnapshots{output: snapOutput(snaps...)}
	result, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchDBISnapshotsPage: unexpected error: %v", err)
	}
	rows := make([]resourceRow, len(result.Resources))
	for i, r := range result.Resources {
		rows[i] = resourceRow{
			status:   r.Fields["status"],
			findings: r.Findings,
			fields:   r.Fields,
			id:       r.ID,
		}
	}
	return rows
}

// resourceRow captures the fields we assert on — avoids depending on the
// full resource.Resource struct layout in tests.
// status is now r.Fields["status"]; issues replaced by findings.
type resourceRow struct {
	id       string
	status   string
	findings []domain.Finding
	fields   map[string]string
}

// ---------------------------------------------------------------------------
// §1.1 Per-signal cases
// ---------------------------------------------------------------------------

// TestDBISnap_Fetcher_HealthyAvailable_BlankS4 verifies that a healthy
// available+encrypted snapshot produces Status="" and no issues.
func TestDBISnap_Fetcher_HealthyAvailable_BlankS4(t *testing.T) {
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
	if len(r.findings) != 0 {
		phrases := make([]string, len(r.findings))
		for i, f := range r.findings {
			phrases[i] = f.Phrase
		}
		t.Errorf("Findings = %v, want empty for healthy row", phrases)
	}
}

// TestDBISnap_Fetcher_Creating_CarriesPercent verifies that Status=creating
// produces "creating: 42%" with PercentProgress embedded.
func TestDBISnap_Fetcher_Creating_CarriesPercent(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.WarnDBISnapCreatingID),
		DBSnapshotArn:        aws.String(fixtures.WarnDBISnapCreatingARN),
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
	if len(r.findings) != 1 || r.findings[0].Phrase != "creating: 42%" {
		phrases := make([]string, len(r.findings))
		for i, f := range r.findings {
			phrases[i] = f.Phrase
		}
		t.Errorf("Findings = %v, want [creating: 42%%]", phrases)
	}
}

// TestDBISnap_Fetcher_Failed_BareKeyword verifies that Status=failed
// produces bare "failed" keyword per spec §4 (no cause available from SDK).
func TestDBISnap_Fetcher_Failed_BareKeyword(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.BrokenDBISnapFailedID),
		DBSnapshotArn:        aws.String(fixtures.BrokenDBISnapFailedARN),
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
	if len(r.findings) != 1 || r.findings[0].Phrase != "failed" {
		phrases := make([]string, len(r.findings))
		for i, f := range r.findings {
			phrases[i] = f.Phrase
		}
		t.Errorf("Findings = %v, want [failed]", phrases)
	}
}

// TestDBISnap_Fetcher_IncompatibleKeywordPreserved verifies that
// incompatible-* statuses preserve the exact AWS keyword verbatim.
func TestDBISnap_Fetcher_IncompatibleKeywordPreserved(t *testing.T) {
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
			if len(r.findings) != 1 || r.findings[0].Phrase != status {
				phrases := make([]string, len(r.findings))
				for i, f := range r.findings {
					phrases[i] = f.Phrase
				}
				t.Errorf("Findings = %v, want [%s]", phrases, status)
			}
		})
	}
}

// TestDBISnap_Fetcher_Unencrypted verifies that Encrypted=false produces
// Status="unencrypted" (CIS RDS.4).
func TestDBISnap_Fetcher_Unencrypted(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.WarnDBISnapUnencryptedID),
		DBSnapshotArn:        aws.String(fixtures.WarnDBISnapUnencryptedARN),
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
	if len(r.findings) != 1 || r.findings[0].Phrase != "unencrypted" {
		phrases := make([]string, len(r.findings))
		for i, f := range r.findings {
			phrases[i] = f.Phrase
		}
		t.Errorf("Findings = %v, want [unencrypted]", phrases)
	}
}

// TestDBISnap_Fetcher_SeverityBrokenBeatsWarning verifies that a Broken status
// (failed) wins over a Warning (Encrypted=false). Encrypted=false is suppressed
// when the snapshot is in a non-available end-state per §0.1/§1.4.
func TestDBISnap_Fetcher_SeverityBrokenBeatsWarning(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.SeverityBrokenWarnDBISnapID),
		DBSnapshotArn:        aws.String(fixtures.SeverityBrokenWarnDBISnapARN),
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
	if len(r.findings) != 1 || r.findings[0].Phrase != "failed" {
		phrases := make([]string, len(r.findings))
		for i, f := range r.findings {
			phrases[i] = f.Phrase
		}
		t.Errorf("Findings = %v, want [failed] (Broken suppresses Warning in same row)", phrases)
	}
}

// TestDBISnap_Fetcher_PopulatesARNField verifies that Fields["arn"] is
// populated from DBSnapshotArn so the backup pivot can read it.
func TestDBISnap_Fetcher_PopulatesARNField(t *testing.T) {
	wantARN := fixtures.ProdDBISnapARN
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String(fixtures.ProdDBISnapID),
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

// TestDBISnap_Fetcher_FindingsPopulatedInPrecedenceOrder verifies (U7f) that
// Resource.Findings is ordered per §0.1 for each signal case ():
// - Healthy → empty
// - failed → ["failed"]
// - incompatible-restore → ["incompatible-restore"]
// - creating → ["creating: 60%"]
// - unencrypted → ["unencrypted"]
func TestDBISnap_Fetcher_FindingsPopulatedInPrecedenceOrder(t *testing.T) {
	cases := []struct {
		name         string
		snap         rdstypes.DBSnapshot
		wantPhrase   string
		wantFindings []string
	}{
		{
			name: "healthy_empty",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-healthy-x"),
				Status:               aws.String("available"),
				Encrypted:            aws.Bool(true),
				PercentProgress:      aws.Int32(100),
			},
			wantPhrase:   "",
			wantFindings: nil,
		},
		{
			name: "failed_single",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-failed-x"),
				Status:               aws.String("failed"),
				Encrypted:            aws.Bool(true),
				PercentProgress:      aws.Int32(0),
			},
			wantPhrase:   "failed",
			wantFindings: []string{"failed"},
		},
		{
			name: "incompatible_restore",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-incompatible-x"),
				Status:               aws.String("incompatible-restore"),
				Encrypted:            aws.Bool(true),
				PercentProgress:      aws.Int32(0),
			},
			wantPhrase:   "incompatible-restore",
			wantFindings: []string{"incompatible-restore"},
		},
		{
			name: "creating_60pct",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-creating-x"),
				Status:               aws.String("creating"),
				Encrypted:            aws.Bool(true),
				PercentProgress:      aws.Int32(60),
			},
			wantPhrase:   "creating: 60%",
			wantFindings: []string{"creating: 60%"},
		},
		{
			name: "unencrypted",
			snap: rdstypes.DBSnapshot{
				DBSnapshotIdentifier: aws.String("snap-unencrypted-x"),
				Status:               aws.String("available"),
				Encrypted:            aws.Bool(false),
				PercentProgress:      aws.Int32(100),
			},
			wantPhrase:   "unencrypted",
			wantFindings: []string{"unencrypted"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := fetchSnap(t, tc.snap)
			if len(rows) != 1 {
				t.Fatalf("expected 1 row, got %d", len(rows))
			}
			r := rows[0]
			if r.status != tc.wantPhrase {
				t.Errorf("Fields[status] = %q, want %q", r.status, tc.wantPhrase)
			}
			gotPhrases := make([]string, len(r.findings))
			for i, f := range r.findings {
				gotPhrases[i] = f.Phrase
			}
			if len(gotPhrases) == 0 {
				gotPhrases = nil
			}
			wantF := tc.wantFindings
			if len(wantF) == 0 {
				wantF = nil
			}
			if len(gotPhrases) != len(wantF) {
				t.Errorf("Findings length = %d, want %d; got %v", len(gotPhrases), len(wantF), gotPhrases)
			} else {
				for i, want := range wantF {
					if gotPhrases[i] != want {
						t.Errorf("Findings[%d].Phrase = %q, want %q", i, gotPhrases[i], want)
					}
				}
			}
		})
	}
}

// TestDBISnap_Fetcher_MultiW1_TopPlusSuffix verifies that when creating
// (Warning top in precedence) is present together with unencrypted (Warning),
// the Status carries "creating: <pct>%" because creating ranks above unencrypted
// in §0.1 (transitional beats CIS). Issues has both phrases in order.
// Note: The multi-W1 snapshot in fixtures uses Encrypted=false + orphan (not creating),
// so we construct an adversarial inline snapshot for this specific case.
func TestDBISnap_Fetcher_MultiW1_TopPlusSuffix(t *testing.T) {
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
	// Findings: creating phrase first, then unencrypted.
	if len(r.findings) < 2 {
		phrases := make([]string, len(r.findings))
		for i, f := range r.findings {
			phrases[i] = f.Phrase
		}
		t.Errorf("Findings = %v, want at least 2 (creating + unencrypted)", phrases)
	} else {
		if !strings.HasPrefix(r.findings[0].Phrase, "creating: 15%") {
			t.Errorf("Findings[0].Phrase = %q, want creating phrase first (§0.1 precedence)", r.findings[0].Phrase)
		}
		if r.findings[1].Phrase != "unencrypted" {
			t.Errorf("Findings[1].Phrase = %q, want %q", r.findings[1].Phrase, "unencrypted")
		}
	}
}

// ---------------------------------------------------------------------------
// Adversarial rows — must not panic
// ---------------------------------------------------------------------------

// TestDBISnap_Fetcher_NilDBSnapshotIdentifier verifies that a snapshot with
// nil DBSnapshotIdentifier is skipped (ID == "") or produces an empty-ID row,
// either way without panicking.
func TestDBISnap_Fetcher_NilDBSnapshotIdentifier(t *testing.T) {
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

// TestDBISnap_Fetcher_NilStatus verifies that nil Status is treated as "" (Healthy).
func TestDBISnap_Fetcher_NilStatus(t *testing.T) {
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

// TestDBISnap_Fetcher_NilSnapshotCreateTime verifies that a snapshot with
// nil SnapshotCreateTime does not panic — the past-retention rule in the
// enricher must skip it cleanly.
func TestDBISnap_Fetcher_NilSnapshotCreateTime(t *testing.T) {
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

// TestDBISnap_Fetcher_AllFixtures_NoError verifies that the full set of
// demo fixtures passes through FetchDBISnapshotsPage without error and
// produces the expected number of rows.
func TestDBISnap_Fetcher_AllFixtures_NoError(t *testing.T) {
	fix := fixtures.NewDBISnapFixtures()
	mock := &mockDescribeDBSnapshots{output: snapOutput(fix.Instances...)}
	result, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
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

// TestDBISnap_StaticAudit_AllSDKCallsThrottleWrapped scans dbi_snap*.go files
// under internal/aws and asserts that every direct RDS/Backup API call appears
// inside a RetryOnThrottle closure. Per universal rule U13, callers must never
// call AWS APIs directly from enricher or fetcher bodies outside throttle wraps.
// // Scan strategy: any line that calls api.Describe*/api.Get*/api.List*/api.Lookup*
// directly (not as the argument to RetryOnThrottle) is a violation.
func TestDBISnap_StaticAudit_AllSDKCallsThrottleWrapped(t *testing.T) {
	root := findRepoFile(t, "internal/aws")

	var violations []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if !strings.HasPrefix(name, "dbi_snap") || !strings.HasSuffix(name, ".go") {
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
		t.Errorf("U13: direct SDK calls found outside RetryOnThrottle in dbi_snap files (%d):\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// TestDBISnap_Backup_UsesArnFromFields verifies (U14 variant) that the ARN
// value stored in Fields["arn"] by the fetcher is the DBSnapshotArn — not r.ID.
// This ensures checkDBISnapBackup reads from Fields["arn"] (populated by fetcher),
// not from the bare snapshot identifier that r.ID carries.
func TestDBISnap_Backup_UsesArnFromFields(t *testing.T) {
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

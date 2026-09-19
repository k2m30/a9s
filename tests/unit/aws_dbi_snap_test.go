package unit

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

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
)

func snapOutput(snaps ...rdstypes.DBSnapshot) *rds.DescribeDBSnapshotsOutput {
	return &rds.DescribeDBSnapshotsOutput{DBSnapshots: snaps}
}

func fetchSnap(t *testing.T, snaps ...rdstypes.DBSnapshot) []resourceRow {
	t.Helper()
	mock := &fakeRDSDescribeDBSnapshots{Output: snapOutput(snaps...)}
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

type resourceRow struct {
	id       string
	status   string
	findings []domain.Finding
	fields   map[string]string
}

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

// DBSnapshot carries no failure cause, so "failed" is the bare keyword.
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

// creating ranks above unencrypted among warnings. The multi-warning fixture
// pairs Encrypted=false with an orphan parent, so this case is built inline.
func TestDBISnap_Fetcher_MultiW1_TopPlusSuffix(t *testing.T) {
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
	wantStatusPrefix := "creating: 15%"
	if !strings.HasPrefix(r.status, wantStatusPrefix) {
		t.Errorf("Status = %q, want prefix %q (creating wins over unencrypted)", r.status, wantStatusPrefix)
	}
	if !strings.Contains(r.status, "(+1)") {
		t.Errorf("Status = %q, want (+1) suffix (multi-W1 indicator)", r.status)
	}
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

func TestDBISnap_Fetcher_NilStatus(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-nil-status"),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:snap-nil-status"),
		Status:               nil,
		Encrypted:            aws.Bool(true),
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].status != "" {
		t.Errorf("nil Status: Resource.Status = %q, want empty (Healthy fallback)", rows[0].status)
	}
}

func TestDBISnap_Fetcher_NilSnapshotCreateTime(t *testing.T) {
	rows := fetchSnap(t, rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("snap-nil-time"),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:snap-nil-time"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotCreateTime:   nil,
	})
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].status != "" {
		t.Errorf("nil SnapshotCreateTime: Status = %q, want empty", rows[0].status)
	}
}

func TestDBISnap_Fetcher_AllFixtures_NoError(t *testing.T) {
	fix := fixtures.NewDBISnapFixtures()
	mock := &fakeRDSDescribeDBSnapshots{Output: snapOutput(fix.Instances...)}
	result, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != len(fix.Instances) {
		t.Errorf("got %d resources, want %d (one per fixture)", len(result.Resources), len(fix.Instances))
	}
	for i, r := range result.Resources {
		if r.ID == "" {
			t.Errorf("resource[%d].ID is empty", i)
		}
	}
}

func TestDBISnap_StaticAudit_AllSDKCallsThrottleWrapped(t *testing.T) {
	root := findRepoFile(t, "core/aws")

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
		t.Fatalf("walking core/aws: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("U13: direct SDK calls found outside RetryOnThrottle in dbi_snap files (%d):\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// checkDBISnapBackup reads Fields["arn"]; r.ID carries the bare snapshot
// identifier.
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

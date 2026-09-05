package unit

// qa_wave1_supporting_rows_test.go — the wave-1 supporting rows on dbi, dbc
// and efs.
//
// These rows are the second line of the detail view's Attention block: the
// phrase says what is wrong, the row says what the setting actually is. They
// are built from data only the fetcher holds, so nothing downstream can
// recompute a row that stops being emitted or starts carrying the wrong value.
// Until now nothing asserted either, and a row whose value silently became a
// Go bool literal or an empty string would have rendered exactly as blank as
// no row at all.
//
// Asserted on the fetcher's own output, because the runtime drops wave-1
// AttentionDetails before the rendered surface; pinning them there would hide
// these rows regressing behind a gap that belongs to another batch.

import (
	"context"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w1SupportRow finds the wave-1 finding whose phrase is wantPhrase on any
// demo row of shortName and returns its supporting rows.
func w1SupportRow(t *testing.T, shortName, wantPhrase string) (string, []domain.DetailRow) {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	clients := demo.NewServiceClients()
	rows := DrainPages(t, shortName, func(token string) (resource.FetchResult, error) {
		return td.Fetcher(context.Background(), clients, token)
	})
	for _, r := range rows {
		for _, f := range r.Findings {
			if f.Source != "wave1" || f.Phrase != wantPhrase {
				continue
			}
			ad, ok := r.AttentionDetails[f.Code]
			if !ok {
				t.Fatalf("%s/%s: %q carries no supporting rows at all", shortName, r.ID, wantPhrase)
			}
			return r.ID, ad.Rows
		}
	}
	t.Fatalf("no demo %s row carries a wave-1 finding phrased %q, so its supporting row is unwitnessed",
		shortName, wantPhrase)
	return "", nil
}

func assertOneRow(t *testing.T, where string, rows []domain.DetailRow, wantLabel, wantValue string) {
	t.Helper()
	if len(rows) != 1 {
		t.Fatalf("%s: %d supporting rows, want exactly 1: %+v", where, len(rows), rows)
	}
	if rows[0].Label != wantLabel {
		t.Errorf("%s: label = %q, want %q", where, rows[0].Label, wantLabel)
	}
	if rows[0].Value != wantValue {
		t.Errorf("%s: value = %q, want %q", where, rows[0].Value, wantValue)
	}
}

// TestWave1SupportingRow_SingleAZ pins the row that answers "single-AZ
// compared to what": the phrase names the risk, the row names the setting.
// Both dbi and dbc build it from the same posture pass, so both are checked —
// a row proved on one is not proved on the other.
func TestWave1SupportingRow_SingleAZ(t *testing.T) {
	for _, short := range []string{"dbi", "dbc"} {
		t.Run(short, func(t *testing.T) {
			id, rows := w1SupportRow(t, short, "single-AZ")
			assertOneRow(t, short+"/"+id+" single-AZ", rows, "Multi-AZ", "no")
		})
	}
}

// TestWave1SupportingRow_IAMDatabaseAuth pins the row for IAM database
// authentication. "identity-based, off" is words rather than the SDK's
// boolean, which is the whole reason the row exists next to the phrase.
func TestWave1SupportingRow_IAMDatabaseAuth(t *testing.T) {
	id, rows := w1SupportRow(t, "dbi", "IAM database authentication off")
	assertOneRow(t, "dbi/"+id+" iam-auth", rows, "Database authentication", "identity-based, off")
}

// TestWave1SupportingRow_DefaultMasterUsername pins that the row names the
// username actually configured rather than repeating the phrase. The operator
// needs to know which default it is before they can rename it.
func TestWave1SupportingRow_DefaultMasterUsername(t *testing.T) {
	id, rows := w1SupportRow(t, "dbi", "default master username")
	if len(rows) != 1 {
		t.Fatalf("dbi/%s: %d supporting rows, want exactly 1: %+v", id, len(rows), rows)
	}
	if rows[0].Label != "Master username" {
		t.Errorf("label = %q, want %q", rows[0].Label, "Master username")
	}
	defaults := []string{"admin", "postgres", "root", "mysql", "master", "awsuser"}
	found := false
	for _, d := range defaults {
		if rows[0].Value == d {
			found = true
		}
	}
	if !found {
		t.Errorf("value = %q, want one of the vendor-default names %v — the row must carry the configured "+
			"username, not a constant", rows[0].Value, defaults)
	}
}

// TestWave1SupportingRow_CertificateExpiry pins the two rows behind the
// certificate-expiry finding. The date is rendered ISO-8601 rather than as an
// SDK timestamp so it can be compared against a maintenance window at a
// glance.
func TestWave1SupportingRow_CertificateExpiry(t *testing.T) {
	var id string
	var rows []domain.DetailRow
	for _, short := range []string{"dbi", "dbc"} {
		td := resource.FindResourceType(short)
		if td == nil {
			t.Fatalf("%s not registered", short)
		}
		clients := demo.NewServiceClients()
		fetched := DrainPages(t, short, func(token string) (resource.FetchResult, error) {
			return td.Fetcher(context.Background(), clients, token)
		})
		for _, r := range fetched {
			for _, f := range r.Findings {
				if f.Source == "wave1" && strings.HasPrefix(f.Phrase, "server certificate expires in ") {
					id = short + "/" + r.ID
					rows = r.AttentionDetails[f.Code].Rows
				}
			}
		}
		if rows != nil {
			break
		}
	}
	if rows == nil {
		t.Fatal("no demo dbi or dbc row carries a server-certificate-expiry finding, so its supporting rows are unwitnessed")
	}
	if len(rows) != 2 {
		t.Fatalf("%s: %d supporting rows, want Certificate authority and Valid till: %+v", id, len(rows), rows)
	}
	if rows[0].Label != "Certificate authority" {
		t.Errorf("%s: first label = %q, want %q", id, rows[0].Label, "Certificate authority")
	}
	if rows[0].Value == "" {
		t.Errorf("%s: certificate authority is blank; the operator cannot tell which CA to renew against", id)
	}
	if rows[1].Label != "Valid till" {
		t.Errorf("%s: second label = %q, want %q", id, rows[1].Label, "Valid till")
	}
	if len(rows[1].Value) != len("2006-01-02") {
		t.Errorf("%s: valid-till = %q, want an ISO-8601 date", id, rows[1].Value)
	}
}

// TestWave1SupportingRow_EFSUnencrypted pins the efs row. "off" rather than
// "false" is the same rule the networking sweep enforces everywhere else.
func TestWave1SupportingRow_EFSUnencrypted(t *testing.T) {
	id, rows := w1SupportRow(t, "efs", "not encrypted")
	assertOneRow(t, "efs/"+id, rows, "Encryption at rest", "off")
}

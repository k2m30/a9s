package unit

// The catch-all request-parameter scan picks the same candidate on every
// render of the same event. Two renders of one event that disagree about the
// TARGET row make the detail view untrustworthy, and a range over a Go map is
// randomised per iteration.
//
// Ranking: an ARN is the most specific reference an event can carry, an id the
// next, a name the least — so *Arn outranks *Id outranks *Name. Ascending byte
// order inside a rank is only a tie-break that makes the choice stable.
// The key that wins is the one removed from the parameters the REQUEST section
// then summarises.

import (
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

// ct0916ScanRuns is high enough that a map-order-dependent pick shows up:
// with two eligible keys a random pick survives 50 runs with probability 2^-49.
const ct0916ScanRuns = 50

// ct0916UnmappedEvent is an event name with no per-event-name extractor, so
// ExtractTarget reaches the catch-all scan.
const ct0916UnmappedEvent = "TagResource"

func TestCT_0916_CatchAllScan_IsDeterministicAcrossRenders(t *testing.T) {
	cases := []struct {
		name        string
		params      map[string]any
		wantValue   string
		wantCleaned map[string]any
	}{
		{
			name: "Arn outranks Name on the same subject",
			params: map[string]any{
				"roleName":   "Deployer",
				"roleArn":    "arn:aws:iam::123456789012:role/Deployer",
				"policyName": "p",
			},
			wantValue: "role/Deployer",
			wantCleaned: map[string]any{
				"roleName":   "Deployer",
				"policyName": "p",
			},
		},
		{
			name: "Id outranks Name",
			params: map[string]any{
				"bucketName": "acme-artifacts",
				"instanceId": "i-0abc123456def0000",
			},
			wantValue:   "i-0abc123456def0000",
			wantCleaned: map[string]any{"bucketName": "acme-artifacts"},
		},
		{
			name: "Arn outranks Id even when it sorts last",
			params: map[string]any{
				"aId":  "i-0abc123456def0000",
				"zArn": "arn:aws:iam::123456789012:role/Deployer",
			},
			wantValue:   "role/Deployer",
			wantCleaned: map[string]any{"aId": "i-0abc123456def0000"},
		},
		{
			name:        "byte order breaks a tie inside one rank",
			params:      map[string]any{"zName": "z", "aName": "a"},
			wantValue:   "a",
			wantCleaned: map[string]any{"zName": "z"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for run := range ct0916ScanRuns {
				rows, cleaned := ctevent.ExtractTarget(ct0916UnmappedEvent, "ram.amazonaws.com", "123456789012", nil, tc.params)
				if len(rows) != 1 {
					t.Fatalf("run %d: got %d TARGET rows, want 1: %+v", run, len(rows), rows)
				}
				if rows[0].Key != "Resource" {
					t.Fatalf("run %d: TARGET row key = %q, want %q", run, rows[0].Key, "Resource")
				}
				if rows[0].Value != tc.wantValue {
					t.Fatalf("run %d: TARGET row value = %q, want %q — the catch-all scan picked a different key than on an earlier run",
						run, rows[0].Value, tc.wantValue)
				}
				if !reflect.DeepEqual(cleaned, tc.wantCleaned) {
					t.Fatalf("run %d: cleaned params = %v, want %v — exactly the chosen key is removed",
						run, cleaned, tc.wantCleaned)
				}
			}
		})
	}
}

// A candidate must be a non-empty top-level string under an *Arn / *Id / *Name
// key: nothing else is a reference to a resource.
func TestCT_0916_CatchAllScan_NoEligibleKeyYieldsNoTargetRow(t *testing.T) {
	params := map[string]any{
		"maxResults": float64(10),
		"tagKeys":    []any{"env"},
		"instanceId": "",
		"filter":     map[string]any{"nameArn": "arn:aws:iam::123456789012:role/Deployer"},
	}
	rows, cleaned := ctevent.ExtractTarget(ct0916UnmappedEvent, "ram.amazonaws.com", "123456789012", nil, params)
	if len(rows) != 0 {
		t.Errorf("got %d TARGET rows, want none — no top-level non-empty string under an *Arn/*Id/*Name key: %+v", len(rows), rows)
	}
	if len(cleaned) != len(params) {
		t.Errorf("cleaned params dropped %d key(s); nothing was lifted into TARGET so nothing may be removed", len(params)-len(cleaned))
	}
}

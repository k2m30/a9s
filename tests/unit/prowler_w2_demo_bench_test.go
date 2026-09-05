package unit_test

// prowler_w2_demo_bench_test.go — the demo-bench half of the w2 Prowler batch.
//
// The per-type tests assert an enricher emits the right finding for the input
// it is given. This one asserts the demo account actually contains such an
// input: every new code is carried by exactly one row after the real fetcher
// and enricher path, so the finding is visible when someone opens the app and
// no other row is quietly tripping the same condition.
//
// Exactly one matters in both directions. Zero means the witness fixture never
// reaches the condition and every screenshot of this signal is empty. More
// than one means a fixture that was supposed to be healthy for this row is not,
// which is how a demo bench stops being a bench and starts being noise.

import (
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w2BenchWitness names one finding code and the demo row that must carry it.
var w2BenchWitnesses = []struct {
	shortName string
	code      string
}{
	{"s3", "s3.public"},
	{"s3", "s3.versioning-off"},
	{"s3", "s3.mfa-delete-off"},
	{"s3", "s3.access-logging-off"},
	{"s3", "s3.no-lifecycle"},
	{"s3", "s3.no-object-lock"},

	{"redis", "redis.encryption-at-rest-off"},
	{"redis", "redis.encryption-in-transit-off"},
	{"redis", "redis.no-auth"},
	{"redis", "redis.no-backup"},

	{"dbi", "dbi.single-az"},
	{"dbi", "dbi.minor-upgrade-off"},
	{"dbi", "dbi.iam-auth-off"},
	{"dbi", "dbi.default-master-user"},
	{"dbi", "dbi.ca-cert-expiring"},
	{"dbi", "dbi.engine-deprecated"},

	{"dbc", "dbc.single-az"},
	{"dbc", "dbc.minor-upgrade-off"},
	{"dbc", "dbc.iam-auth-off"},
	{"dbc", "dbc.default-master-user"},

	{"dbi-snap", "dbi-snap.public"},
	{"dbc-snap", "dbc-snap.public"},

	{"ddb", "ddb.deletion-protection-off"},
	{"ddb", "ddb.cross-account-policy"},
	{"ddb", "ddb.public-policy"},

	{"opensearch", "opensearch.public"},
	{"opensearch", "opensearch.https-not-enforced"},
	{"opensearch", "opensearch.node-to-node-tls-off"},

	{"redshift", "redshift.audit-logging-off"},
	{"redshift", "redshift.require-ssl-off"},

	{"efs", "efs.unencrypted"},
	{"efs", "efs.public-policy"},
	{"efs", "efs.no-backup-policy"},
}

func TestW2DemoBenchOneWitnessPerFinding(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	merged := make(map[string][]resource.Resource)
	for _, td := range resource.AllResourceTypes() {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}
		merged[td.ShortName] = mergeWave2Findings(t, td, fixtures, cache, clients)
	}

	for _, w := range w2BenchWitnesses {
		t.Run(w.shortName+"/"+w.code, func(t *testing.T) {
			rows := merged[w.shortName]
			if len(rows) == 0 {
				t.Fatalf("no demo rows for type %q", w.shortName)
			}
			var carriers []string
			for _, r := range rows {
				for _, f := range r.Findings {
					if string(f.Code) == w.code {
						carriers = append(carriers, r.ID)
						break
					}
				}
			}
			sort.Strings(carriers)
			if len(carriers) != 1 {
				t.Errorf("%s: %d demo rows carry %q (%v); want exactly 1 witness",
					w.shortName, len(carriers), w.code, carriers)
			}
		})
	}
}

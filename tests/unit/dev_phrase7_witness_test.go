package unit_test

// dev_phrase7_witness_test.go — the demo bench half of the six premise fixes.
// Each check was narrowed so it no longer fires on a resource AWS documents as
// fine; these pin that the demo account actually contains such a resource, so
// the corrected behaviour is visible on the bench and any rewrite that widens a
// check back shows up as a red row here.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// devPhrase7Witnesses names, per finding, the demo row that must NOT carry it
// because the resource is fine, alongside the requirement that some other row
// still does.
var devPhrase7Witnesses = []struct {
	shortName string
	code      string
	healthy   string
	why       string
}{
	{"sqs", "sqs.no-kms", fixtures.SQSManagedSSE,
		"a queue with SqsManagedSseEnabled true is encrypted with an AWS-owned key"},
	{"eks", "eks.secrets-not-kms", fixtures.EKSDefaultSecretsEncryption,
		"from Kubernetes 1.28 AWS envelope-encrypts secrets without being asked"},
	{"vpc", "vpc.no-flow-logs", fixtures.VPCSubnetScopedFlowLog,
		"a flow log on a subnet writes the same records as one on the VPC"},
	{"cf", "cf.insecure-protocol", fixtures.CFS3WebsiteOrigin,
		"an S3 website endpoint serves HTTP only, so http-only is the only policy it accepts"},
	{"ecs-task", "ecs-task.stop-code.failed", fixtures.ECSTaskSpotReclaimed,
		"Spot reclaiming capacity it warned about is the platform working"},
}

// devPhrase7Rows folds one type's demo rows the way the app folds them: the
// real fetcher, then the type's registered wave-2 enricher through
// runtime.ApplyWave2ToRow.
func devPhrase7Rows(t *testing.T, shortName string) []resource.Resource {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s not registered", shortName)
	}
	byType, cache := buildVisibilityTypeCache(t)
	rows := byType[shortName]
	if len(rows) == 0 {
		t.Fatalf("no demo rows for %s", shortName)
	}
	return mergeWave2Findings(t, *td, rows, cache, demo.NewServiceClients())
}

func TestDevPhrase7HealthyWitnessCarriesNoFinding(t *testing.T) {
	for _, w := range devPhrase7Witnesses {
		t.Run(w.shortName+"/"+w.code, func(t *testing.T) {
			rows := devPhrase7Rows(t, w.shortName)

			var witness *resource.Resource
			var otherCarriers []string
			for i := range rows {
				r := &rows[i]
				carries := false
				for _, f := range r.Findings {
					if string(f.Code) == w.code {
						carries = true
					}
				}
				if r.ID == w.healthy || r.Name == w.healthy || strings.Contains(r.ID, w.healthy) {
					witness = r
					if carries {
						t.Errorf("%s carries %q, but %s", w.healthy, w.code, w.why)
					}
					continue
				}
				if carries {
					otherCarriers = append(otherCarriers, r.ID)
				}
			}
			if witness == nil {
				t.Fatalf("no demo %s row named %q; the witness fixture is missing", w.shortName, w.healthy)
			}
			if len(otherCarriers) == 0 {
				t.Errorf("no demo %s row carries %q at all; narrowing the check must not retire the finding",
					w.shortName, w.code)
			}
		})
	}
}

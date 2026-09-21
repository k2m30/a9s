// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"slices"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// The Wave-2 enricher stops at EnrichmentCap rows, so a witness past it shows
// its verdict only to a reader who opens that one row's detail — and the demo
// list, which is what the signal is demonstrated on, shows nothing.
func TestDemoDBIWave2WitnessesAreWithinTheEnrichmentCap(t *testing.T) {
	clients := demo.NewServiceClients()
	td := resource.FindResourceType("dbi")
	if td == nil {
		t.Fatal("no dbi resource type")
	}
	rows, ok := DrainFixtures(t, *td, clients)
	if !ok {
		t.Fatal("dbi declares no Fetcher")
	}

	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}

	if len(ids) <= awsclient.EnrichmentCap {
		t.Fatalf("demo dbi list is %d rows, at or under EnrichmentCap=%d — nothing demonstrates the cap",
			len(ids), awsclient.EnrichmentCap)
	}

	// Both carry dbi.not-in-backup-plan's two answers: the instance no plan
	// selects, and the cluster member a plan covers through its cluster.
	witnesses := []string{fixtures.DBINotInBackupPlan, fixtures.DBIDocDBMember}
	for _, id := range witnesses {
		i := slices.Index(ids, id)
		if i < 0 {
			t.Errorf("%s is not in the demo dbi list", id)
			continue
		}
		if i >= awsclient.EnrichmentCap {
			t.Errorf("%s is dbi row %d, past EnrichmentCap=%d — Wave 2 never reaches it, so its "+
				"verdict is missing from the list and only its own detail view states it",
				id, i+1, awsclient.EnrichmentCap)
		}
	}
}

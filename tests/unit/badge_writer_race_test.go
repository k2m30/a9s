// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// The cache writer saves the menu badges on its own goroutine while the
// handler loop lands sweep results; the writer must read nothing the loop
// writes bare. Red under -race when it does.
func TestBadgeWriter_RunsBesideTheHandlerLoop(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	core, _ := newLiveWebStyleController(t, "pilot-prof", "us-east-1")
	rows := uninspectedEC2Rows()
	core.ObserveRows("ec2", rows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	pair := core.Pair()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			_ = core.SaveAvailabilityFromRows(pair)
		}
	}()
	for range 200 {
		core.HandleEvent(messages.EnrichmentChecked{
			ResourceType: "ec2",
			TruncatedIDs: map[string]string{rows[0].ID: awsclient.CheckCap},
		})
	}
	<-done
}

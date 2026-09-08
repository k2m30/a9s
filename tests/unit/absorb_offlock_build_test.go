// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// absorb_offlock_build_test.go pins what makes the off-lock list-body build
// safe: a row's contents are replaced, never written into. The build reads a
// shallow copy of the screen's rows while other writers keep working, so a
// writer that reaches into a row's Fields map — rather than swapping the map —
// is a data race no lock protects against.
package unit_test

import (
	"strconv"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestFieldUpdateDuringAbsorb_DoesNotRaceTheOffLockBuild fails under -race if
// a Wave-2 field update mutates a row's Fields map in place while a result is
// absorbed: the absorb builds the list body outside the controller lock, and
// the rows it reads share those maps with the screen. Run it with -race; it
// passes vacuously without.
func TestFieldUpdateDuringAbsorb_DoesNotRaceTheOffLockBuild(t *testing.T) {
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := absorbProbeRows(400)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			c.ApplyListFieldUpdates("ec2", map[string]map[string]string{
				rows[i%len(rows)].ID: {"state": "stopping"},
			})
		}
	}()
	for range 40 {
		_, _ = c.Handle(messages.ResourcesLoaded{
			ResourceType: "ec2", Resources: absorbProbeRows(400),
			Pagination: &domain.PaginationMeta{IsTruncated: false},
			Provenance: messages.FetchProvenanceCanonicalList,
		})
	}
	close(stop)
	wg.Wait()

	if b := c.Snapshot().Body.List; b == nil || len(b.Rows) != 400 {
		got := 0
		if b != nil {
			got = len(b.Rows)
		}
		t.Errorf("the list holds %d rows after the absorbs, want 400 — moving the build off "+
			"the lock must not lose rows", got)
	}
}

// absorbProbeRows mirrors the shape the ec2 fetcher writes.
func absorbProbeRows(n int) []resource.Resource {
	rows := make([]resource.Resource, n)
	for i := range rows {
		id := "i-" + strconv.Itoa(1000000000000000+i)
		rows[i] = resource.Resource{
			ID:   id,
			Name: "example-instance-" + strconv.Itoa(i),
			Fields: map[string]string{
				"instance_id": id, "state": "running", "instance_type": "t3.micro",
				"az": "us-east-1a", "private_ip": "10.0.1.10", "vpc_id": "vpc-0123456789abcdef0",
			},
		}
	}
	return rows
}

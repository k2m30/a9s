// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// The ecs-svc→logs count comes from the awslogs-group each service's own task
// definition declares. With one family's group in the demo, every other
// service reads (0) and the pivot looks like a property of the panel.
func TestDemoMoreThanOneECSServiceResolvesLogs(t *testing.T) {
	clients := demo.NewServiceClients()
	td := resource.FindResourceType("ecs-svc")
	if td == nil {
		t.Fatal("no ecs-svc resource type")
	}

	var def *resource.RelatedDef
	for i, d := range resource.GetRelated("ecs-svc") {
		if d.TargetType == "logs" {
			def = &resource.GetRelated("ecs-svc")[i]
			break
		}
	}
	if def == nil || def.Checker == nil {
		t.Fatal("ecs-svc declares no logs pivot with a checker")
	}

	rows, ok := DrainFixtures(t, *td, clients)
	if !ok {
		t.Fatal("ecs-svc declares no Fetcher")
	}

	var resolved []string
	for _, row := range rows {
		r := def.Checker(context.Background(), clients, row, resource.ResourceCache{})
		if r.EffectiveState() == domain.RelatedResolved && r.Count() > 0 {
			resolved = append(resolved, row.ID)
		}
	}

	if len(resolved) < 2 {
		t.Errorf("only %d of %d demo ecs-svc rows resolve a non-zero logs count (%v) — one is "+
			"indistinguishable from a pivot that always answers for the same service",
			len(resolved), len(rows), resolved)
	}
}

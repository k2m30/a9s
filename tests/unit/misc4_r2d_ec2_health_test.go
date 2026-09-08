// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// misc4_r2d_ec2_health_test.go — misc4 round 2, item (d).
//
// The Health cell is read off DescribeInstanceStatus. A running instance the
// fixture leaves out of that response renders a blank Health cell, which reads
// as "a9s did not check" rather than "AWS says nothing is wrong" — and nothing
// in the fixture file says a row was left out on purpose.
//
// This gate covers the whole demo ec2 list rather than the one row round 1
// added, so the next instance appended cannot reintroduce it.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
)

// TestEveryRunningDemoInstanceHasAHealthCheck pins that DescribeInstanceStatus
// answers for every running instance the demo fixture ships.
func TestEveryRunningDemoInstanceHasAHealthCheck(t *testing.T) {
	fake := fakes.NewEC2()
	out, err := awsclient.FetchEC2InstancesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchEC2InstancesPage: %v", err)
	}

	statuses, err := fake.DescribeInstanceStatus(context.Background(), &ec2.DescribeInstanceStatusInput{})
	if err != nil {
		t.Fatalf("DescribeInstanceStatus: %v", err)
	}
	checked := make(map[string]bool, len(statuses.InstanceStatuses))
	for _, s := range statuses.InstanceStatuses {
		if s.InstanceId != nil {
			checked[*s.InstanceId] = true
		}
	}

	running := 0
	for _, r := range out.Resources {
		if r.Fields["state"] != "running" {
			continue
		}
		running++
		if !checked[r.ID] {
			t.Errorf("running instance %s (%s) has no status check, so its Health "+
				"cell renders blank — add it to buildInstanceStatuses", r.Name, r.ID)
		}
	}
	if running < 20 {
		t.Fatalf("only %d running demo instances; the gate is not seeing the fixture", running)
	}
}

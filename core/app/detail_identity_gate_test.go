// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// detail_identity_gate_test.go — every row of every demo fixture, through the
// real detail projection, has an identity of its own.
//
// The detail cursor is relocated across a rebuild by the identity
// buildDetailFieldItems assigns each row, so two rows answering to one
// identity send the cursor to the wrong one silently. The gate walks the
// fixtures rather than a handful of synthetic resources: a RawStruct-heavy
// projection is where a repeated label is likely (two targets of one
// CloudTrail event, two identical lines of a raw YAML document), and no
// hand-written pin would have found those.
//
// It lives in the package rather than in tests/unit because the identity is
// internal and must stay that way: making the RENDERED label unique to satisfy
// a key would distort what the operator reads, and a raw document is allowed
// to repeat a line.
package app

import (
	"context"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

func TestDetailRowIdentities_AreUniqueAcrossEveryDemoFixture(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = demo.DemoProfile
	s.Region = demo.DemoRegion
	c := New(runtime.New(s, nil))
	t.Cleanup(c.Close)

	clients := demo.NewServiceClients()
	ctx := context.Background()
	walked, rows := 0, 0
	for _, short := range resource.AllShortNames() {
		td := resource.FindResourceType(short)
		if td == nil || td.Fetcher == nil {
			continue
		}
		// A partial page is the point of several fixtures (a row the demo
		// denies, a row it cannot find), so the resources are walked whether
		// or not the fetcher also reports an error.
		page, err := td.Fetcher(ctx, clients, "")
		if len(page.Resources) == 0 {
			t.Logf("%s: no demo fixtures to walk (err=%v)", short, err)
			continue
		}
		for _, res := range page.Resources {
			walked++
			ds := &DetailState{
				Resource:         res,
				ResourceType:     short,
				Findings:         res.Findings,
				AttentionDetails: res.AttentionDetails,
				ViewportWidth:    120,
			}
			built := c.buildDetailFieldItems(ds)
			rows += len(built.items)
			seen := map[string]int{}
			for i, key := range built.keys {
				if built.items[i].IsSpacer {
					// A spacer is a blank line the cursor never rests on:
					// every move skips it and the body's clamp walks back off
					// it, so two spacers sharing an identity is unreachable.
					continue
				}
				if prev, dup := seen[key]; dup {
					t.Errorf(
						"%s/%s: rows %d and %d answer to one identity %q\n"+
							"  first:  Key=%q Value=%q\n"+
							"  second: Key=%q Value=%q\n"+
							"the detail cursor is relocated by this identity, so it would land on the wrong row.",
						short, res.ID, prev, i, key,
						built.items[prev].Key, built.items[prev].Value,
						built.items[i].Key, built.items[i].Value,
					)
					break
				}
				seen[key] = i
			}
		}
	}
	if walked == 0 {
		t.Fatal("no demo resource was walked — the gate would pass vacuously")
	}
	t.Logf("walked %d demo resources, %d detail rows", walked, rows)
}

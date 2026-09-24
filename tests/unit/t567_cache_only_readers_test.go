package unit_test

// A pivot that reads its target only from the cache has no list when the
// cache holds rows added one by one for a detail, or rows restored from disk
// without their SDK structs: it reads unknown, as it does with no entry at all.

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

func TestT567CacheOnlyPivots_LazyOrFieldsOnlyEntryIsNoList(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	eni := t567Typed(b.byType["eni"][0], "eni")
	for _, r := range b.byType["eni"] {
		if refChecker(t, "eni", "ec2")(context.Background(), clients, t567Typed(r, "eni"), resource.ResourceCache{}).Count() > 0 {
			eni = t567Typed(r, "eni")
			break
		}
	}

	pivots := []struct{ source, target, list string }{
		{"ng", "ec2", "ec2"},
		{"ng", "ebs", "ec2"},
		{"lt", "ec2", "ec2"},
		{"ebs-snap", "ec2", "ec2"},
		{"lt", "asg", "asg"},
		{"lt", "ng", "ng"},
		{"kinesis", "ddb", "ddb"},
		{"vpc-peer", "rtb", "rtb"},
		{"vpc-peer", "vpc", "vpc"},
	}
	for _, p := range pivots {
		t.Run(p.source+"→"+p.target, func(t *testing.T) {
			check := refChecker(t, p.source, p.target)
			// A source that relates to a row of the loaded list, so no answer
			// over another entry can be a zero the source row itself decides.
			var src resource.Resource
			for _, r := range b.byType[p.source] {
				if check(context.Background(), clients, r, b.cache).Count() > 0 {
					src = t567Typed(r, p.source)
					break
				}
			}
			if src.ID == "" {
				t.Fatalf("no demo %s row relates to a %s over the loaded %s list", p.source, p.target, p.list)
			}

			lazy := t567Core(t, clients)
			if p.list == "ec2" {
				t567LazyAdd(t, lazy, clients, eni, "ec2")
			} else {
				lazy.ObservePartialRows(p.list, b.byType[p.list][:1])
			}
			entry := lazy.BuildResourceCacheSnapshot()[p.list]
			if !entry.Partial {
				t.Fatalf("lazily added %s rows snapshot as partial=%v", p.list, entry.Partial)
			}
			if got := check(context.Background(), clients, src, lazy.BuildResourceCacheSnapshot()); got.State() != domain.RelatedUnknown {
				t.Errorf("%s %s → %s with only lazily added %s rows cached = %q (state %s), want unknown", p.source, src.ID, p.target, p.list, t567Badge(got), got.State())
			}

			disk := t567Core(t, clients)
			restored := t567FieldsOnly(b.byType[p.list]).Resources
			disk.ObserveRows(p.list, restored, &resource.PaginationMeta{}, session.OriginDisk, false)
			if !disk.BuildResourceCacheSnapshot()[p.list].FieldsOnly {
				t.Fatalf("disk-restored %s rows did not snapshot as fields-only", p.list)
			}
			if got := check(context.Background(), clients, src, disk.BuildResourceCacheSnapshot()); got.State() != domain.RelatedUnknown {
				t.Errorf("%s %s → %s with the %s list restored from disk = %q (state %s), want unknown", p.source, src.ID, p.target, p.list, t567Badge(got), got.State())
			}
		})
	}
}

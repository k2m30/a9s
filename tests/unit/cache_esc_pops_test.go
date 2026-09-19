package unit

// A related-navigation list has EscPops set and no ParentContext; it holds a
// filtered subset, so its load must not replace the top-level cache.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

func TestCachePoison_RelatedNavigate_DoesNotOverwriteTopLevelCache(t *testing.T) {
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	clients := demo.NewServiceClients()
	ec2Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), clients.EC2, token)
	})
	if err != nil || len(ec2Res) < 2 {
		t.Fatalf("demo ec2 fixtures need at least 2 resources for cache-poisoning test: err=%v len=%d", err, len(ec2Res))
	}
	fullCount := len(ec2Res)

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    ec2Res,
	})

	m, enterCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	m, _ = drainCmds(t, m, enterCmd, 5)

	navMsg := messages.RelatedNavigate{
		TargetType: "ec2",
		SourceResource: resource.Resource{
			ID:   "vpc-demo-001",
			Name: "demo-vpc",
		},
		SourceType: "vpc",
	}
	m, _ = rootApplyMsg(m, navMsg)

	partialList := ec2Res[0:1]
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceFilteredList,
		ResourceType: "ec2",
		Resources:    partialList,
	})

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Leaving the list makes the next navigation restore it from cache.

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	view := stripANSI(rootViewContent(m))

	// The frame title carries the count prefix, e.g. "ec2(5" or "ec2(5/N issues)".
	expectedCountStr := fmt.Sprintf("(%d", fullCount)
	poisonedCountStr := fmt.Sprintf("(%d", len(partialList))

	if strings.Contains(view, poisonedCountStr) && !strings.Contains(view, expectedCountStr) {
		t.Fatalf(
			"BUG: cache was poisoned by related-navigation list — EC2 list shows count %s "+
				"instead of full count %s.\n"+
				"Fix: add `&& !rl.EscPops()` to the guard at app.go:246, "+
				"and `|| rl.EscPops()` to the guard at app.go:454.\nView:\n%s",
			poisonedCountStr, expectedCountStr, view,
		)
	}

	if !strings.Contains(view, expectedCountStr) {
		t.Fatalf(
			"EC2 list after cache restore must show full count %s but got unexpected view.\n"+
				"full count=%d, poisoned count=%d\nView:\n%s",
			expectedCountStr, fullCount, len(partialList), view,
		)
	}
}

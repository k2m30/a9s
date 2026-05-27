package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

func TestQA_CacheStories_WarmReentryRestoresListState(t *testing.T) {
	// Isolate from the user's ~/.a9s config so the built-in 9-column
	// EC2 defaults apply deterministically on all platforms.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    ec2TestResources(30),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    30,
			TotalHint:   -1,
		},
	})

	// Press 8 twice: sort by Instance ID (column 8, absolute) ascending then descending.
	// EC2 layout is 1:Name 2:State 3:Health 4:Lifecycle 5:Type 6:Private IP
	// 7:Public IP 8:Instance ID 9:Launch Time. Column 8 may be off-screen at
	// width 80 — that's fine, absolute keys still work.
	m, _ = rootApplyMsg(m, rootKeyPress("8"))
	m, _ = rootApplyMsg(m, rootKeyPress("8"))
	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	if cmd != nil {
		t.Fatal("warm re-entry should be served from cache without a fresh fetch")
	}

	m, cmd = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter on a warm cached list should still open detail for the selected row")
	}
	if follow := cmd(); follow != nil {
		m, _ = rootApplyMsg(m, follow)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "detail --") || !strings.Contains(plain, "i-00028") {
		t.Fatalf("warm cache should restore sort order and cursor selection, got:\n%s", plain)
	}
}

func TestQA_CacheStories_LoadMoreUpdatesWarmCache(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
	})

	m, _ = rootApplyMsg(m, rootKeyPress("M"))
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources2(50, 50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			NextToken:   "",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: true,
	})

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	m, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	if cmd != nil {
		t.Fatal("re-entering a paginated list after load-more should be a cache hit")
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(100") {
		t.Fatalf("warm cache should retain the merged page set, got:\n%s", plain)
	}
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "DeleteObject-99" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(1/100)") || !strings.Contains(plain, "DeleteObject-99") {
		t.Fatalf("warm cache should include resources from later pages, got:\n%s", plain)
	}
}

func TestQA_CacheStories_RelatedNavigationUsesTargetDataCachedFromBackgroundLoad(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	src := resource.Resource{
		ID:     "i-cache-001",
		Name:   "cache-source",
		Fields: map[string]string{"instance_id": "i-cache-001"},
	}
	// tg registers Children[Key="enter"] → tg_health with ContextKeys
	// {"target_group_arn":"target_group_arn"}. Under the rule that
	// single-result auto-drill mirrors manual Enter, the cache-hit path
	// must enter that child view rather than push the plain TG detail.
	tg1 := resource.Resource{
		ID:     "tg-cache-1",
		Name:   "frontend-tg",
		Fields: map[string]string{"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/frontend-tg/abc123"},
	}
	tg2 := resource.Resource{
		ID:     "tg-cache-2",
		Name:   "backend-tg",
		Fields: map[string]string{"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/backend-tg/def456"},
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &src,
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "tg",
		Resources:    []resource.Resource{tg1, tg2},
	})

	m, cmd := rootApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "tg",
		SourceResource: src,
		TargetID:       tg1.ID,
	})
	// Drain the EnterChildViewMsg command so the view reflects the child-view push.
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	plain := stripANSI(rootViewContent(m))
	// tg has Children[Key="enter"] → tg_health. The cache-hit fast path
	// must enter that child view rather than push the plain TG detail.
	if !strings.Contains(plain, "tg_health") {
		t.Fatalf("related cache hit on tg (enter-child registered) should enter tg_health child view, got:\n%s", plain)
	}
	// Must NOT show an intermediate filtered list.
	if strings.Contains(plain, "tg(1)") {
		t.Fatalf("related cache hit should not show an intermediate target list, got:\n%s", plain)
	}
	// Must NOT show the plain tg detail — fast path must enter child view.
	if strings.Contains(plain, "detail -- "+tg1.ID) {
		t.Fatalf("tg has Children[Key=\"enter\"] → tg_health; related cache hit must enter the child view, not push plain detail, got:\n%s", plain)
	}
}

func TestQA_CacheStories_RelatedMultiIDUsesWarmCacheSubset(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	src := resource.Resource{
		ID:     "i-cache-002",
		Name:   "cache-source",
		Fields: map[string]string{"instance_id": "i-cache-002"},
	}
	alarm1 := resource.Resource{ID: "alarm-cache-1", Name: "cpu-high"}
	alarm2 := resource.Resource{ID: "alarm-cache-2", Name: "status-check"}
	alarm3 := resource.Resource{ID: "alarm-cache-3", Name: "unrelated"}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "alarm",
		Resources:    []resource.Resource{alarm1, alarm2, alarm3},
	})

	m, cmd := rootApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "alarm",
		SourceResource: src,
		RelatedIDs:     []string{alarm1.ID, alarm2.ID},
	})
	if cmd != nil {
		t.Fatal("multi-related cache hit should not trigger a fetch")
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "alarms(2)") {
		t.Fatalf("filtered related cache hit should show only matching resources, got:\n%s", plain)
	}
	if !strings.Contains(plain, alarm1.Name) || !strings.Contains(plain, alarm2.Name) {
		t.Fatalf("filtered related cache hit should show the matching resources, got:\n%s", plain)
	}
	if strings.Contains(plain, alarm3.Name) {
		t.Fatalf("filtered related cache hit should exclude unrelated cached resources, got:\n%s", plain)
	}
}

func TestQA_CacheStories_ChildViewLoadsDoNotCreateTopLevelWarmCache(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	stack := resource.Resource{
		ID:     "stack-001",
		Name:   "payments-stack",
		Fields: map[string]string{"stack_name": "payments-stack"},
	}
	childResources := []resource.Resource{
		{ID: "evt-1", Name: "Stack create started"},
		{ID: "evt-2", Name: "Stack create done"},
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "cfn",
		Resource:     &stack,
	})
	m, _ = rootApplyMsg(m, messages.EnterChildView{
		ChildType:     "cfn_events",
		ParentContext: map[string]string{"stack_name": stack.ID, "Name": stack.Name},
		DisplayName:   stack.Name,
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "cfn_events",
		Resources:    childResources,
	})

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	_, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "cfn_events",
	})
	if cmd == nil {
		t.Fatal("child-view loads must not create a top-level warm cache entry")
	}
}

func TestQA_CacheStories_RefreshingChildViewDoesNotEvictTopLevelCache(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	// New schema: Status is verb-based, not ReadOnly. _ct.actor is set to the resource
	// Name so it renders in the ACTOR column and can be used as an assertion target
	// (the Name is not rendered in any default column without a RawStruct).
	topLevelEvents := []resource.Resource{
		{ID: "evt-top-1", Name: "LookupEvents top-level", Fields: map[string]string{
			"_ct.verb": "R", "_ct.actor": "LookupEvents top-level", "_ct.origin": "CLI",
			"_ct.target": "(none)", "_ct.outcome": "OK", "event_time": "2026-03-28 14:30:15",
		}},
		{ID: "evt-top-2", Name: "AssumeRole top-level", Fields: map[string]string{
			"_ct.verb": "R", "_ct.actor": "AssumeRole top-level", "_ct.origin": "CLI",
			"_ct.target": "(none)", "_ct.outcome": "OK", "event_time": "2026-03-28 14:30:15",
		}},
	}
	stack := resource.Resource{
		ID:     "stack-002",
		Name:   "edge-stack",
		Fields: map[string]string{"stack_name": "edge-stack"},
	}
	childResources := []resource.Resource{
		{ID: "evt-child-1", Name: "child event 1"},
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    topLevelEvents,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    2,
			TotalHint:   -1,
		},
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "cfn",
		Resource:     &stack,
	})
	m, _ = rootApplyMsg(m, messages.EnterChildView{
		ChildType:     "cfn_events",
		ParentContext: map[string]string{"stack_name": stack.ID, "Name": stack.Name},
		DisplayName:   stack.Name,
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "cfn_events",
		Resources:    childResources,
	})

	_, refreshCmd := rootApplyMsg(m, rootSpecialKey(0x12))
	if refreshCmd == nil {
		t.Fatal("refreshing a child view should still issue a child fetch")
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	if cmd != nil {
		t.Fatal("refreshing a child view must not evict an unrelated top-level cache entry")
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(2)") || !strings.Contains(plain, "LookupEvents top-level") {
		t.Fatalf("top-level cached list should still be restored after child refresh, got:\n%s", plain)
	}
}

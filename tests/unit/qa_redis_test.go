package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/viewport"

	"github.com/aws/aws-sdk-go-v2/aws"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// multiStatusRedisFixtures returns Redis replication groups with different statuses for color tests.
// Post-phase-7: RawStruct is ReplicationGroup (DescribeReplicationGroups).
// Fields["status"] carries the §4 phrase (Healthy silence = empty string) per
// docs/resources/redis.md §4, matching what the fetcher emits in production.
// RawStruct.Status still holds the raw AWS enum value because that is what the
// AWS SDK reports and any direct-RawStruct consumer will see.
func multiStatusRedisFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID: "redis-available", Name: "redis-available",
			Fields: map[string]string{
				"cluster_id": "redis-available",
				"node_type":  "cache.t2.micro", "status": "",
				"nodes": "1", "endpoint": "",
			},
			RawStruct: elasticachetypes.ReplicationGroup{
				ReplicationGroupId: aws.String("redis-available"),
				Status:             aws.String("available"),
				CacheNodeType:      aws.String("cache.t2.micro"),
				MemberClusters:     []string{"redis-available-001"},
			},
		},
		{
			ID: "redis-creating", Name: "redis-creating",
			Fields: map[string]string{
				"cluster_id": "redis-creating",
				"node_type":  "cache.t2.micro", "status": "creating — new group",
				"nodes": "1", "endpoint": "",
			},
			RawStruct: elasticachetypes.ReplicationGroup{
				ReplicationGroupId: aws.String("redis-creating"),
				Status:             aws.String("creating"),
				CacheNodeType:      aws.String("cache.t2.micro"),
				MemberClusters:     []string{"redis-creating-001"},
			},
		},
		{
			ID: "redis-deleting", Name: "redis-deleting",
			Fields: map[string]string{
				"cluster_id": "redis-deleting",
				"node_type":  "cache.t2.micro", "status": "deleting — teardown",
				"nodes": "1", "endpoint": "",
			},
			RawStruct: elasticachetypes.ReplicationGroup{
				ReplicationGroupId: aws.String("redis-deleting"),
				Status:             aws.String("deleting"),
				CacheNodeType:      aws.String("cache.t2.micro"),
				MemberClusters:     []string{"redis-deleting-001"},
			},
		},
	}
}

// ===========================================================================
// REDIS-DETAIL-01 / REDIS-DETAIL-02: Redis detail view
// ===========================================================================

// TestQA_Redis_DetailView is the live-seam replacement for the retired
// views.NewDetail(...).View() call (DetailModel.View is dead; see
// specs/022-codebase-cleanup/wave3-map-detail.md) — drives
// Controller.EnsureDetailState + NewTransientDetail.RenderDetail instead.
func TestQA_Redis_DetailView(t *testing.T) {
	fixtures := fixtureRedisClusters()
	res := fixtures[0]
	c := newDetailControllerUnit(t, res, "redis")

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil")
	}
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(20))
	m := views.NewTransientDetail(80, 20, vp)
	out := stripANSI(m.RenderDetail(*body))

	if out == "" || out == "Initializing..." {
		t.Fatal("Redis detail view returned empty or initializing")
	}

	// Detail view should contain field keys and values from the resource's Fields map
	for key, val := range res.Fields {
		if val == "" {
			continue
		}
		if !strings.Contains(out, val) {
			t.Errorf("Redis detail view missing field value for %q: %q", key, val)
		}
	}
}

// TestQA_Redis_DetailFrameTitle is the live-seam replacement for the retired
// views.NewDetail(...).FrameTitle() call — drives Snapshot().FrameTitle
// instead (detailFrameTitleLocked mirrors the legacy Name-else-ID semantics).
func TestQA_Redis_DetailFrameTitle(t *testing.T) {
	fixtures := fixtureRedisClusters()
	res := fixtures[0]
	c := newDetailControllerUnit(t, res, "redis")
	title := c.Snapshot().FrameTitle

	// FrameTitle should be the resource Name (or ID if Name is empty)
	expected := res.Name
	if expected == "" {
		expected = res.ID
	}
	if title != expected {
		t.Errorf("Redis detail FrameTitle = %q, want %q", title, expected)
	}
}

// ===========================================================================
// REDIS-DETAIL-04: Redis detail status coloring (per-type Color func)
// ===========================================================================

func TestQA_Redis_DetailStatusColoring(t *testing.T) {
	tuitest.ForceColor(t)

	// Redis Color func reads Fields["status"].
	td := resource.FindResourceType("redis")
	if td == nil {
		t.Fatal("redis resource type not found")
	}
	if td.Color == nil {
		t.Fatal("redis Color func is nil")
	}

	redisRes := func(status string) resource.Resource {
		return resource.Resource{
			ID:     "cache-001",
			Fields: map[string]string{"status": status},
		}
	}

	// Post-migration (2026-04-23): Fields["status"] carries §4 PHRASES, not
	// bare keywords. Healthy = empty string.
	availableStyle := styles.ColorStyle(td.Color(redisRes("")))
	if availableStyle.GetForeground() != styles.ColRunning {
		t.Errorf("redis healthy (blank): expected ColRunning (#9ece6a), got %v", availableStyle.GetForeground())
	}

	creatingStyle := styles.ColorStyle(td.Color(redisRes("creating — new group")))
	if creatingStyle.GetForeground() != styles.ColPending {
		t.Errorf("redis 'creating — new group': expected ColPending (#e0af68), got %v", creatingStyle.GetForeground())
	}

	// Per spec: redis deleting → Warning (not Broken).
	deletingStyle := styles.ColorStyle(td.Color(redisRes("deleting — teardown")))
	if deletingStyle.GetForeground() != styles.ColPending {
		t.Errorf("redis 'deleting — teardown': expected ColPending (Warning per spec), got %v", deletingStyle.GetForeground())
	}
}

// ===========================================================================
// REDIS-YAML-01 / REDIS-YAML-03: Redis YAML view
// ===========================================================================

func TestQA_Redis_YAMLView(t *testing.T) {
	fixtures := fixtureRedisClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(80, 30)
	out := strings.Join(m.ContentLines(), "\n")

	if out == "" || out == "Initializing..." {
		t.Fatal("Redis YAML view returned empty or initializing")
	}

	// YAML view renders from RawStruct (SDK struct field names) when RawStruct is set
	// Post-phase-7: RawStruct is ReplicationGroup, so field names changed.
	// MemberClusters is omitted from fixture to keep YAML output as key:value pairs only.
	expectedKeys := []string{"ReplicationGroupId", "Description", "Status", "CacheNodeType"}
	for _, key := range expectedKeys {
		if !strings.Contains(out, key) {
			t.Errorf("Redis YAML view missing SDK struct key %q", key)
		}
	}
	// Values from the RawStruct should appear
	expectedValues := []string{"test-redis-1", "cache.t2.micro", "available"}
	for _, val := range expectedValues {
		if !strings.Contains(out, val) {
			t.Errorf("Redis YAML view missing value %q", val)
		}
	}
}

// TestQA_Redis_YAMLFrameTitle retired: YAMLModel.FrameTitle() is DEAD per
// specs/022-codebase-cleanup/wave3-map-text.md (no production caller), and
// title-string behavior is not resource-type-specific.

// ===========================================================================
// REDIS-YAML-06: Redis YAML raw content for copy
// ===========================================================================

func TestQA_Redis_YAMLRawContent(t *testing.T) {
	fixtures := fixtureRedisClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	raw := stripANSI(strings.Join(m.ContentLines(), "\n"))

	if raw == "" {
		t.Fatal("Redis YAML RawContent() returned empty string")
	}

	// RawContent renders from RawStruct (SDK struct field names)
	// Post-phase-7: RawStruct is ReplicationGroup, so field names changed.
	// MemberClusters is omitted from fixture to keep YAML output as key:value pairs only.
	expectedKeys := []string{"ReplicationGroupId", "Description", "Status", "CacheNodeType"}
	for _, key := range expectedKeys {
		if !strings.Contains(raw, key) {
			t.Errorf("Redis YAML RawContent missing SDK struct key %q", key)
		}
	}
}

// ===========================================================================
// Redis integration: full root model navigation
// ===========================================================================

func TestQA_Redis_NavigateFromMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to Redis list
	m, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	_ = cmd

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Errorf("after navigate to Redis, frame should contain 'redis', got: %s", plain)
	}
}

func TestQA_Redis_LoadAndDisplayList(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to Redis
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})

	// Load fixtures
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "redis",
		Resources:    fixtures, Provenance: messages.FetchProvenanceCanonicalList,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis(1)") {
		t.Errorf("after loading Redis, frame title should contain 'redis(1)', got: %s", plain)
	}
	// Note: At root model level (80-char width), config-driven columns use Path-based
	// extraction (CacheClusterId, etc.) which requires RawStruct. Fixtures use Fields
	// maps instead, so cell data appears via ResourceListModel directly (unit-level
	// tests above), not through the root model integration path.
	// Verify column headers are present instead.
	if !strings.Contains(plain, "Cluster ID") {
		t.Errorf("Redis list should contain 'Cluster ID' column header, got: %s", plain)
	}
}

func TestQA_Redis_NavigateToDetail(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to Redis and load data
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "redis",
		Resources:    fixtures,
	})

	// Navigate to detail
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	plain := stripANSI(rootViewContent(m))
	expected := res.Name
	if expected == "" {
		expected = res.ID
	}
	if !strings.Contains(plain, expected) {
		t.Errorf("Redis detail frame should contain %q, got: %s", expected, plain)
	}
}

func TestQA_Redis_NavigateToYAML(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to Redis and load data
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "redis",
		Resources:    fixtures,
	})

	// Navigate to YAML
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: &res,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Errorf("Redis YAML frame should contain 'yaml', got: %s", plain)
	}
}

func TestQA_Redis_DetailBackNavigation(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to Redis list
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "redis",
		Resources:    fixtures,
	})

	// Navigate to detail
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	// Pop back
	m, _ = rootApplyMsg(m, messages.PopView{})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Errorf("after pop from Redis detail, should return to Redis list, got: %s", plain)
	}
}

// ===========================================================================
// REDIS command mode: :redis navigates correctly
// ===========================================================================

func TestQA_Redis_CommandNavigation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	// Type "redis"
	for _, r := range "redis" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	// Press enter to execute
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Error("executeCommand('redis') should return a command (NavigateMsg)")
	}
}

// ===========================================================================
// REDIS-YAML-07: YAML back navigation via root model
// ===========================================================================

func TestQA_Redis_YAMLBackNavigation(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate: Redis list -> YAML -> pop
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "redis",
		Resources:    fixtures,
	})
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: &res,
	})

	// Verify YAML view
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Fatalf("should be on YAML view, got: %s", plain)
	}

	// Pop back to list
	m, _ = rootApplyMsg(m, messages.PopView{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Errorf("after pop from Redis YAML, should return to Redis list, got: %s", plain)
	}
}

// ===========================================================================
// Redis: Full round-trip list -> detail -> yaml -> pop -> pop -> pop
// ===========================================================================

func TestQA_Redis_FullNavigationRoundTrip(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Main menu -> Redis list
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "redis",
		Resources:    fixtures,
	})

	// Redis list -> detail
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	// Detail -> YAML
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: &res,
	})

	// Pop YAML -> detail
	m, _ = rootApplyMsg(m, messages.PopView{})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, res.Name) && !strings.Contains(plain, res.ID) {
		t.Errorf("pop from YAML should return to detail, got: %s", plain)
	}

	// Pop detail -> list
	m, _ = rootApplyMsg(m, messages.PopView{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Errorf("pop from detail should return to Redis list, got: %s", plain)
	}

	// Pop list -> main menu
	m, _ = rootApplyMsg(m, messages.PopView{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("pop from Redis list should return to main menu, got: %s", plain)
	}
}

// ===========================================================================
// Redis: Filter via root model header display
// ===========================================================================

func TestQA_Redis_FilterHeaderDisplay(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to Redis
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "redis",
		Resources:    multiStatusRedisFixtures(),
	})

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "prod" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/prod") {
		t.Errorf("header should show filter text '/prod' during filter mode, got: %s", plain)
	}
}

// ===========================================================================
// CROSS-HELP-01: Help accessible from Redis view
// ===========================================================================

func TestQA_Redis_HelpOverlay(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})

	// Open help
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Errorf("help overlay should be visible from Redis view, got: %s", plain)
	}
}

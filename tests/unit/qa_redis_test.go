package unit

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// Helpers for Redis tests
// ===========================================================================

func redisTypeDef() resource.ResourceTypeDef {
	for _, rt := range resource.AllResourceTypes() {
		if rt.ShortName == "redis" {
			return rt
		}
	}
	panic("redis resource type not found")
}

func loadedRedisModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	td := redisTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    fixtureRedisClusters(),
	})
	return m
}

// multiStatusRedisFixtures returns Redis clusters with different statuses for color tests.
func multiStatusRedisFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID: "redis-available", Name: "redis-available", Status: "available",
			Fields: map[string]string{
				"cluster_id": "redis-available", "engine_version": "7.0.7",
				"node_type": "cache.t2.micro", "status": "available",
				"nodes": "1", "endpoint": "",
			},
			RawStruct: elasticachetypes.CacheCluster{
				CacheClusterId:     aws.String("redis-available"),
				EngineVersion:      aws.String("7.0.7"),
				CacheNodeType:      aws.String("cache.t2.micro"),
				CacheClusterStatus: aws.String("available"),
				NumCacheNodes:      aws.Int32(1),
			},
		},
		{
			ID: "redis-creating", Name: "redis-creating", Status: "creating",
			Fields: map[string]string{
				"cluster_id": "redis-creating", "engine_version": "7.0.7",
				"node_type": "cache.t2.micro", "status": "creating",
				"nodes": "1", "endpoint": "",
			},
			RawStruct: elasticachetypes.CacheCluster{
				CacheClusterId:     aws.String("redis-creating"),
				EngineVersion:      aws.String("7.0.7"),
				CacheNodeType:      aws.String("cache.t2.micro"),
				CacheClusterStatus: aws.String("creating"),
				NumCacheNodes:      aws.Int32(1),
			},
		},
		{
			ID: "redis-deleting", Name: "redis-deleting", Status: "deleting",
			Fields: map[string]string{
				"cluster_id": "redis-deleting", "engine_version": "7.0.7",
				"node_type": "cache.t2.micro", "status": "deleting",
				"nodes": "1", "endpoint": "",
			},
			RawStruct: elasticachetypes.CacheCluster{
				CacheClusterId:     aws.String("redis-deleting"),
				EngineVersion:      aws.String("7.0.7"),
				CacheNodeType:      aws.String("cache.t2.micro"),
				CacheClusterStatus: aws.String("deleting"),
				NumCacheNodes:      aws.Int32(1),
			},
		},
	}
}

// ===========================================================================
// REDIS-LIST-02: Redis list displays correct columns
// ===========================================================================

func TestQA_Redis_ListColumns(t *testing.T) {
	m := loadedRedisModel(t)
	out := m.View()

	expectedHeaders := []string{"Cluster ID", "Version", "Node Type", "Status", "Nodes", "Endpoint"}
	for _, header := range expectedHeaders {
		if !strings.Contains(out, header) {
			t.Errorf("Redis list view missing column header %q", header)
		}
	}
}

// ===========================================================================
// REDIS-LIST-03: Redis list populates column data from correct fields
// ===========================================================================

func TestQA_Redis_ListColumnData(t *testing.T) {
	fixtures := fixtureRedisClusters()
	m := loadedRedisModel(t)
	out := m.View()

	// Verify fixture data appears in the rendered output
	r := fixtures[0]
	expectedValues := []string{
		r.Fields["cluster_id"],
		r.Fields["engine_version"],
		r.Fields["node_type"],
		r.Fields["status"],
		r.Fields["nodes"],
	}
	for _, val := range expectedValues {
		if val == "" {
			continue
		}
		if !strings.Contains(out, val) {
			t.Errorf("Redis list view missing field value %q", val)
		}
	}
}

// ===========================================================================
// REDIS-LIST-04: Redis list row count appears in frame title
// ===========================================================================

func TestQA_Redis_FrameTitle(t *testing.T) {
	m := loadedRedisModel(t)
	title := m.FrameTitle()

	expected := "redis(1)"
	if title != expected {
		t.Errorf("expected FrameTitle() = %q, got %q", expected, title)
	}
}

// ===========================================================================
// REDIS-LIST-05: Redis list row coloring by status
// ===========================================================================

func TestQA_Redis_StatusColoring(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := redisTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    multiStatusRedisFixtures(),
	})

	out := m.View()

	// All three statuses must be visible in the rendered output
	for _, status := range []string{"available", "creating", "deleting"} {
		if !strings.Contains(out, status) {
			t.Errorf("Redis list missing status %q in rendered output", status)
		}
	}

	// With NO_COLOR unset, the output should contain ANSI escape sequences
	// (color codes) -- verify the output is styled
	if !strings.Contains(out, "\x1b[") {
		t.Error("Redis list with status colors should contain ANSI escape sequences")
	}
}

// ===========================================================================
// REDIS-LIST-06: Redis list cursor navigation
// ===========================================================================

func TestQA_Redis_CursorNavigation(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := redisTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    multiStatusRedisFixtures(),
	})

	// Initial selection is row 0
	sel := m.SelectedResource()
	if sel == nil || sel.ID != "redis-available" {
		t.Fatalf("expected initial selection to be redis-available, got %v", sel)
	}

	// Move down with 'j'
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	sel = m.SelectedResource()
	if sel == nil || sel.ID != "redis-creating" {
		t.Errorf("after 'j', expected redis-creating, got %v", sel)
	}

	// Move up with 'k'
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
	sel = m.SelectedResource()
	if sel == nil || sel.ID != "redis-available" {
		t.Errorf("after 'k', expected redis-available, got %v", sel)
	}

	// Jump to bottom with 'G'
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "G"})
	sel = m.SelectedResource()
	if sel == nil || sel.ID != "redis-deleting" {
		t.Errorf("after 'G', expected redis-deleting, got %v", sel)
	}

	// Jump to top with 'g'
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "g"})
	sel = m.SelectedResource()
	if sel == nil || sel.ID != "redis-available" {
		t.Errorf("after 'g', expected redis-available, got %v", sel)
	}
}

// ===========================================================================
// REDIS-LIST-08: Redis list filter
// ===========================================================================

func TestQA_Redis_ListFilter(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := redisTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    multiStatusRedisFixtures(),
	})

	// Apply a filter that matches only one resource
	m.SetFilter("creating")
	out := m.View()

	if !strings.Contains(out, "redis-creating") {
		t.Error("filtered Redis list should contain 'redis-creating'")
	}
	if strings.Contains(out, "redis-available") {
		t.Error("filtered Redis list should NOT contain 'redis-available'")
	}

	title := m.FrameTitle()
	if title != "redis(1/3)" {
		t.Errorf("expected filtered FrameTitle = %q, got %q", "redis(1/3)", title)
	}

	// Clear filter
	m.SetFilter("")
	title = m.FrameTitle()
	if title != "redis(3)" {
		t.Errorf("expected unfiltered FrameTitle = %q, got %q", "redis(3)", title)
	}
}

// ===========================================================================
// REDIS-LIST-09: Redis list sorting
// ===========================================================================

func TestQA_Redis_ListSort(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := redisTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    multiStatusRedisFixtures(),
	})

	// Sort by column 0 ('1') -- Cluster ID column (key "cluster_id", index 0, 1-indexed key "1")
	// so the sort indicator should appear on that column.
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "1"})
	out := m.View()
	if !strings.Contains(out, "\u2191") && !strings.Contains(out, "\u2193") {
		t.Error("expected sort indicator arrow in Cluster ID column header after pressing 1")
	}

	// Press 1 again to toggle sort direction
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "1"})
	out2 := m.View()
	// Sort should still be active (indicator present)
	if !strings.Contains(out2, "\u2191") && !strings.Contains(out2, "\u2193") {
		t.Error("expected sort indicator to remain after toggling sort direction")
	}

	// Sort by column 1 ('2') -- "Version" column; verify sort happens without crashing.
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "2"})
	sel := m.SelectedResource()
	if sel == nil {
		t.Error("after sort by column 2, should still have a selected resource")
	}
}

// ===========================================================================
// REDIS-LIST-11: Redis list with no clusters
// ===========================================================================

func TestQA_Redis_EmptyList(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := redisTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    []resource.Resource{},
	})

	out := m.View()
	if !strings.Contains(out, "No resources found") {
		t.Errorf("empty Redis list should show 'No resources found', got: %q", out)
	}

	title := m.FrameTitle()
	if title != "redis(0)" {
		t.Errorf("expected empty FrameTitle = %q, got %q", "redis(0)", title)
	}
}

// ===========================================================================
// REDIS-LIST-12: Redis list with null ConfigurationEndpoint
// ===========================================================================

func TestQA_Redis_NullEndpoint(t *testing.T) {
	m := loadedRedisModel(t)
	out := m.View()

	// The fixture has an empty endpoint. It should NOT show "null" or "<nil>".
	if strings.Contains(out, "null") || strings.Contains(out, "<nil>") {
		t.Error("Redis list should not display 'null' or '<nil>' for empty endpoint")
	}
}

// ===========================================================================
// REDIS-DETAIL-01 / REDIS-DETAIL-02: Redis detail view
// ===========================================================================

func TestQA_Redis_DetailView(t *testing.T) {
	fixtures := fixtureRedisClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewDetail(res, "redis", nil, k)
	m.SetSize(80, 20)
	out := m.View()

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

func TestQA_Redis_DetailFrameTitle(t *testing.T) {
	fixtures := fixtureRedisClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewDetail(res, "redis", nil, k)
	title := m.FrameTitle()

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
// REDIS-DETAIL-04: Redis detail status coloring
// ===========================================================================

func TestQA_Redis_DetailStatusColoring(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	// Verify the style system maps "available" -> green (ColRunning)
	availableStyle := styles.RowColorStyle("available")
	if availableStyle.GetForeground() != styles.ColRunning {
		t.Errorf("expected 'available' to map to ColRunning (#9ece6a), got %v", availableStyle.GetForeground())
	}

	creatingStyle := styles.RowColorStyle("creating")
	if creatingStyle.GetForeground() != styles.ColPending {
		t.Errorf("expected 'creating' to map to ColPending (#e0af68), got %v", creatingStyle.GetForeground())
	}

	deletingStyle := styles.RowColorStyle("deleting")
	if deletingStyle.GetForeground() != styles.ColStopped {
		t.Errorf("expected 'deleting' to map to ColStopped (#f7768e), got %v", deletingStyle.GetForeground())
	}
}

// ===========================================================================
// REDIS-YAML-01 / REDIS-YAML-03: Redis YAML view
// ===========================================================================

func TestQA_Redis_YAMLView(t *testing.T) {
	fixtures := fixtureRedisClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAML(res, "", k)
	m.SetSize(80, 30)
	out := m.View()

	if out == "" || out == "Initializing..." {
		t.Fatal("Redis YAML view returned empty or initializing")
	}

	// YAML view renders from RawStruct (SDK struct field names) when RawStruct is set
	expectedKeys := []string{"CacheClusterId", "EngineVersion", "CacheNodeType", "CacheClusterStatus", "NumCacheNodes"}
	for _, key := range expectedKeys {
		if !strings.Contains(out, key) {
			t.Errorf("Redis YAML view missing SDK struct key %q", key)
		}
	}
	// Values from the RawStruct should appear
	expectedValues := []string{"test-redis-1", "7.0.7", "cache.t2.micro", "available"}
	for _, val := range expectedValues {
		if !strings.Contains(out, val) {
			t.Errorf("Redis YAML view missing value %q", val)
		}
	}
}

func TestQA_Redis_YAMLFrameTitle(t *testing.T) {
	fixtures := fixtureRedisClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAML(res, "", k)
	title := m.FrameTitle()

	expected := res.Name + " yaml"
	if res.Name == "" {
		expected = res.ID + " yaml"
	}
	if title != expected {
		t.Errorf("Redis YAML FrameTitle = %q, want %q", title, expected)
	}
}

// ===========================================================================
// REDIS-YAML-06: Redis YAML raw content for copy
// ===========================================================================

func TestQA_Redis_YAMLRawContent(t *testing.T) {
	fixtures := fixtureRedisClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAML(res, "", k)
	raw := m.RawContent()

	if raw == "" {
		t.Fatal("Redis YAML RawContent() returned empty string")
	}

	// RawContent renders from RawStruct (SDK struct field names)
	expectedKeys := []string{"CacheClusterId", "EngineVersion", "CacheNodeType", "CacheClusterStatus", "NumCacheNodes"}
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
	m, cmd := rootApplyMsg(m, messages.NavigateMsg{
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
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})

	// Load fixtures
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    fixtures,
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
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    fixtures,
	})

	// Navigate to detail
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
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
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    fixtures,
	})

	// Navigate to YAML
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
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
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    fixtures,
	})

	// Navigate to detail
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	// Pop back
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})
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
// REDIS: Horizontal scroll
// ===========================================================================

func TestQA_Redis_HorizontalScroll(t *testing.T) {
	fixtures := fixtureRedisClusters()

	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := redisTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(50, 20) // very narrow to force scrolling
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    fixtures,
	})

	outBefore := m.View()

	// Scroll right
	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
	outAfter := m.View()

	if outBefore == outAfter {
		t.Error("expected horizontal scroll to change Redis list output")
	}
}

// ===========================================================================
// REDIS: Loading shows spinner
// ===========================================================================

func TestQA_Redis_LoadingSpinner(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := redisTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(140, 20)
	m, _ = m.Init()

	out := m.View()
	if !strings.Contains(out, "Loading") {
		t.Error("Redis list in loading state should show 'Loading'")
	}

	title := m.FrameTitle()
	if title != "redis" {
		t.Errorf("Redis loading FrameTitle = %q, want %q", title, "redis")
	}
}

// ===========================================================================
// REDIS: No separator row below headers
// ===========================================================================

func TestQA_Redis_NoSeparatorBelowHeaders(t *testing.T) {
	m := loadedRedisModel(t)
	out := m.View()

	lines := strings.SplitSeq(out, "\n")
	for line := range lines {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		allDash := true
		for _, ch := range stripped {
			if ch != '-' && ch != '_' && ch != '=' && ch != ' ' {
				allDash = false
				break
			}
		}
		if allDash && len(stripped) > 5 {
			t.Errorf("found what looks like a separator row in Redis list: %q", stripped)
		}
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
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    fixtures,
	})
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: &res,
	})

	// Verify YAML view
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Fatalf("should be on YAML view, got: %s", plain)
	}

	// Pop back to list
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})
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
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "redis",
		Resources:    fixtures,
	})

	// Redis list -> detail
	res := fixtures[0]
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	// Detail -> YAML
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: &res,
	})

	// Pop YAML -> detail
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, res.Name) && !strings.Contains(plain, res.ID) {
		t.Errorf("pop from YAML should return to detail, got: %s", plain)
	}

	// Pop detail -> list
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Errorf("pop from detail should return to Redis list, got: %s", plain)
	}

	// Pop list -> main menu
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})
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
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
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

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})

	// Open help
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetHelp})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Errorf("help overlay should be visible from Redis view, got: %s", plain)
	}
}

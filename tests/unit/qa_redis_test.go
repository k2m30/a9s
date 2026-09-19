package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"charm.land/bubbles/v2/viewport"

	"github.com/aws/aws-sdk-go-v2/aws"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// multiStatusRedisFixtures returns Redis replication groups with different
// statuses. RawStruct is the ReplicationGroup (DescribeReplicationGroups).
// Fields["status"] carries the status phrase from docs/resources/redis.md
// (empty for a healthy group), as the fetcher emits it; RawStruct.Status holds
// the raw AWS enum.
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
	res := fixtures[0]
	c := newDetailControllerUnit(t, res, "redis")
	title := c.Snapshot().FrameTitle

	// The snapshot's title is the one builder's output, not a second Name-else-ID
	// answer of the controller's own; the builder is pinned in
	// tui6_detail_frame_title_test.go.
	expected := resource.DetailFrameTitle(res.ID, res.Name, resource.DetailTitleOmitsID("redis"))
	if title != expected {
		t.Errorf("Redis detail FrameTitle = %q, want %q", title, expected)
	}
}

func TestQA_Redis_DetailStatusColoring(t *testing.T) {
	tuitest.ForceColor(t)

	td := resource.FindResourceType("redis")
	if td == nil {
		t.Fatal("redis resource type not found")
	}
	if td.Color == nil {
		t.Fatal("redis Color func is nil")
	}

	// Same shape as the dbc probe: the finding carries the severity, so the
	// phrase alone cannot decide a colour.
	redisRes := func(status string, sev domain.Severity) resource.Resource {
		r := resource.Resource{
			ID:     "cache-001",
			Fields: map[string]string{"status": status},
		}
		if status != "" {
			r.Findings = []domain.Finding{{
				Code: "redis.probe", Phrase: status, Severity: sev, Source: "wave1",
			}}
		}
		return r
	}

	// Fields["status"] carries status phrases, not bare keywords; a healthy group
	// is the empty string.
	availableStyle := styles.ColorStyle(td.Color(redisRes("", domain.SevOK)))
	if availableStyle.GetForeground() != styles.ColRunning {
		t.Errorf("redis healthy (blank): expected ColRunning (#9ece6a), got %v", availableStyle.GetForeground())
	}

	creatingStyle := styles.ColorStyle(td.Color(redisRes("creating — new group", domain.SevWarn)))
	if creatingStyle.GetForeground() != styles.ColPending {
		t.Errorf("redis 'creating — new group': expected ColPending (#e0af68), got %v", creatingStyle.GetForeground())
	}

	// A deleting group is Warning, not Broken.
	deletingStyle := styles.ColorStyle(td.Color(redisRes("deleting — teardown", domain.SevWarn)))
	if deletingStyle.GetForeground() != styles.ColPending {
		t.Errorf("redis 'deleting — teardown': expected ColPending (Warning per spec), got %v", deletingStyle.GetForeground())
	}
}

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

	// YAML renders RawStruct's SDK field names. The fixture leaves MemberClusters
	// empty so the output stays key:value pairs.
	expectedKeys := []string{"ReplicationGroupId", "Description", "Status", "CacheNodeType"}
	for _, key := range expectedKeys {
		if !strings.Contains(out, key) {
			t.Errorf("Redis YAML view missing SDK struct key %q", key)
		}
	}
	expectedValues := []string{"test-redis-1", "cache.t2.micro", "available"}
	for _, val := range expectedValues {
		if !strings.Contains(out, val) {
			t.Errorf("Redis YAML view missing value %q", val)
		}
	}
}

func TestQA_Redis_YAMLRawContent(t *testing.T) {
	fixtures := fixtureRedisClusters()
	k := keys.Default()
	res := fixtures[0]
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	raw := stripANSI(strings.Join(m.ContentLines(), "\n"))

	if raw == "" {
		t.Fatal("Redis YAML RawContent() returned empty string")
	}

	// RawContent renders RawStruct's SDK field names. The fixture leaves
	// MemberClusters empty so the output stays key:value pairs.
	expectedKeys := []string{"ReplicationGroupId", "Description", "Status", "CacheNodeType"}
	for _, key := range expectedKeys {
		if !strings.Contains(raw, key) {
			t.Errorf("Redis YAML RawContent missing SDK struct key %q", key)
		}
	}
}

func TestQA_Redis_NavigateFromMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

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

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "redis",
		Resources:    fixtures, Provenance: messages.FetchProvenanceCanonicalList,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis(1)") {
		t.Errorf("after loading Redis, frame title should contain 'redis(1)', got: %s", plain)
	}
	// Config-driven columns extract by Path from RawStruct, and these fixtures
	// carry Fields only, so the root-level check is on the column headers.
	if !strings.Contains(plain, "Cluster ID") {
		t.Errorf("Redis list should contain 'Cluster ID' column header, got: %s", plain)
	}
}

func TestQA_Redis_NavigateToDetail(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

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

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Errorf("Redis YAML frame should contain 'yaml', got: %s", plain)
	}
}

func TestQA_Redis_DetailBackNavigation(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

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
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	m, _ = rootApplyMsg(m, messages.PopView{})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Errorf("after pop from Redis detail, should return to Redis list, got: %s", plain)
	}
}

func TestQA_Redis_CommandNavigation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	for _, r := range "redis" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Error("executeCommand('redis') should return a command (NavigateMsg)")
	}
}

func TestQA_Redis_YAMLBackNavigation(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

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

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Fatalf("should be on YAML view, got: %s", plain)
	}

	m, _ = rootApplyMsg(m, messages.PopView{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Errorf("after pop from Redis YAML, should return to Redis list, got: %s", plain)
	}
}

func TestQA_Redis_FullNavigationRoundTrip(t *testing.T) {
	fixtures := fixtureRedisClusters()

	tui.Version = "0.6.0"
	m := newRootSizedModel()

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
		Target:   messages.TargetDetail,
		Resource: &res,
	})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: &res,
	})

	m, _ = rootApplyMsg(m, messages.PopView{})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, res.Name) && !strings.Contains(plain, res.ID) {
		t.Errorf("pop from YAML should return to detail, got: %s", plain)
	}

	m, _ = rootApplyMsg(m, messages.PopView{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "redis") {
		t.Errorf("pop from detail should return to Redis list, got: %s", plain)
	}

	m, _ = rootApplyMsg(m, messages.PopView{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("pop from Redis list should return to main menu, got: %s", plain)
	}
}

func TestQA_Redis_FilterHeaderDisplay(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "redis",
		Resources:    multiStatusRedisFixtures(),
	})

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "prod" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/prod") {
		t.Errorf("header should show filter text '/prod' during filter mode, got: %s", plain)
	}
}

func TestQA_Redis_HelpOverlay(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Errorf("help overlay should be visible from Redis view, got: %s", plain)
	}
}

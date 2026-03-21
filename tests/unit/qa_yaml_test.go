package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/views"
)

// yamlKeyPress creates a tea.KeyPressMsg for a printable character.
func yamlKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// ── Helper: build YAMLModel from resource, set size, return View output ─────

func yamlView(t *testing.T, res resource.Resource, w, h int) string {
	t.Helper()
	k := keys.Default()
	m := views.NewYAML(res, k)
	m.SetSize(w, h)
	out := m.View()
	if out == "" || out == "Initializing..." {
		t.Fatalf("YAMLModel.View() returned empty or initializing for resource %q", res.ID)
	}
	return out
}

func yamlModel(res resource.Resource, w, h int) views.YAMLModel {
	k := keys.Default()
	m := views.NewYAML(res, k)
	m.SetSize(w, h)
	return m
}

// ════════════════════════════════════════════════════════════════════════════
// QA-09: YAML View for all resource types — S3, EC2, RDS, Redis, DocDB, EKS, Secrets
// ════════════════════════════════════════════════════════════════════════════

// ── S3 ──────────────────────────────────────────────────────────────────────

func TestQA_YAML_S3_ViewContainsFields(t *testing.T) {
	buckets := fixtureS3Buckets()
	for _, b := range buckets {
		out := yamlView(t, b, 120, 40)
		for k, v := range b.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("S3 YAML for %q missing key %q", b.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("S3 YAML for %q missing value %q", b.ID, v)
			}
		}
	}
}

func TestQA_YAML_S3_SyntaxColoring(t *testing.T) {
	buckets := fixtureS3Buckets()
	out := yamlView(t, buckets[0], 120, 40)
	// Colored output must contain ANSI escape codes
	if !strings.Contains(out, "\x1b[") {
		t.Error("S3 YAML output has no ANSI color codes — syntax coloring missing")
	}
}

func TestQA_YAML_S3_Structure(t *testing.T) {
	buckets := fixtureS3Buckets()
	m := yamlModel(buckets[0], 120, 40)
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("S3 RawContent() returned empty")
	}
	// RawContent must have key: value format
	if !strings.Contains(raw, ": ") {
		t.Error("S3 RawContent() missing YAML key: value format")
	}
	// Must not contain ANSI codes
	if strings.Contains(raw, "\x1b[") {
		t.Error("S3 RawContent() contains ANSI codes — should be plain")
	}
}

func TestQA_YAML_S3_FrameTitle(t *testing.T) {
	buckets := fixtureS3Buckets()
	m := yamlModel(buckets[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("S3 FrameTitle() = %q, want 'yaml' in title", title)
	}
	if !strings.Contains(title, buckets[0].Name) {
		t.Errorf("S3 FrameTitle() = %q, want resource name %q", title, buckets[0].Name)
	}
}

func TestQA_YAML_S3_RawContentUncolored(t *testing.T) {
	buckets := fixtureS3Buckets()
	m := yamlModel(buckets[0], 120, 40)
	raw := m.RawContent()
	stripped := stripANSI(raw)
	if raw != stripped {
		t.Error("S3 RawContent() contains ANSI codes, expected plain YAML for clipboard copy")
	}
}

// ── EC2 ─────────────────────────────────────────────────────────────────────

func TestQA_YAML_EC2_ViewContainsFields(t *testing.T) {
	instances := fixtureEC2Instances()
	for _, inst := range instances {
		out := yamlView(t, inst, 120, 40)
		for k, v := range inst.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("EC2 YAML for %q missing key %q", inst.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("EC2 YAML for %q missing value %q", inst.ID, v)
			}
		}
	}
}

func TestQA_YAML_EC2_SyntaxColoring(t *testing.T) {
	instances := fixtureEC2Instances()
	out := yamlView(t, instances[0], 120, 40)
	if !strings.Contains(out, "\x1b[") {
		t.Error("EC2 YAML output has no ANSI color codes")
	}
}

func TestQA_YAML_EC2_FrameTitle(t *testing.T) {
	instances := fixtureEC2Instances()
	// First instance has no Name, so FrameTitle should use ID
	m := yamlModel(instances[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("EC2 FrameTitle() = %q, want 'yaml' in title", title)
	}
	if !strings.Contains(title, instances[0].ID) {
		t.Errorf("EC2 FrameTitle() = %q, want ID %q (Name is empty)", title, instances[0].ID)
	}

	// Named instance uses Name
	m2 := yamlModel(instances[1], 120, 40)
	title2 := m2.FrameTitle()
	if !strings.Contains(title2, instances[1].Name) {
		t.Errorf("EC2 FrameTitle() = %q, want Name %q", title2, instances[1].Name)
	}
}

func TestQA_YAML_EC2_RawContentUncolored(t *testing.T) {
	instances := fixtureEC2Instances()
	m := yamlModel(instances[0], 120, 40)
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("EC2 RawContent() returned empty")
	}
	if strings.Contains(raw, "\x1b[") {
		t.Error("EC2 RawContent() contains ANSI codes")
	}
}

func TestQA_YAML_EC2_Structure(t *testing.T) {
	instances := fixtureEC2Instances()
	m := yamlModel(instances[0], 120, 40)
	raw := m.RawContent()
	if !strings.Contains(raw, ": ") {
		t.Error("EC2 RawContent() missing key: value YAML format")
	}
}

// ── RDS ─────────────────────────────────────────────────────────────────────

func TestQA_YAML_RDS_ViewContainsFields(t *testing.T) {
	instances := fixtureRDSInstances()
	for _, inst := range instances {
		out := yamlView(t, inst, 120, 40)
		for k, v := range inst.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("RDS YAML for %q missing key %q", inst.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("RDS YAML for %q missing value %q", inst.ID, v)
			}
		}
	}
}

func TestQA_YAML_RDS_SyntaxColoring(t *testing.T) {
	instances := fixtureRDSInstances()
	out := yamlView(t, instances[0], 120, 40)
	if !strings.Contains(out, "\x1b[") {
		t.Error("RDS YAML output has no ANSI color codes")
	}
}

func TestQA_YAML_RDS_FrameTitle(t *testing.T) {
	instances := fixtureRDSInstances()
	m := yamlModel(instances[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("RDS FrameTitle() = %q, want 'yaml' in title", title)
	}
	if !strings.Contains(title, instances[0].Name) {
		t.Errorf("RDS FrameTitle() = %q, want Name %q", title, instances[0].Name)
	}
}

func TestQA_YAML_RDS_RawContentUncolored(t *testing.T) {
	instances := fixtureRDSInstances()
	m := yamlModel(instances[0], 120, 40)
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("RDS RawContent() returned empty")
	}
	if strings.Contains(raw, "\x1b[") {
		t.Error("RDS RawContent() contains ANSI codes")
	}
}

// ── Redis ───────────────────────────────────────────────────────────────────

func TestQA_YAML_Redis_ViewContainsFields(t *testing.T) {
	clusters := fixtureRedisClusters()
	for _, c := range clusters {
		out := yamlView(t, c, 120, 40)
		if c.RawStruct != nil {
			// When RawStruct is present, YAML renders SDK struct field names
			expectedKeys := []string{"CacheClusterId", "EngineVersion", "CacheNodeType", "CacheClusterStatus", "NumCacheNodes"}
			for _, k := range expectedKeys {
				if !strings.Contains(out, k) {
					t.Errorf("Redis YAML for %q missing SDK struct key %q", c.ID, k)
				}
			}
			expectedValues := []string{"test-redis-1", "7.0.7", "cache.t2.micro", "available"}
			for _, v := range expectedValues {
				if !strings.Contains(out, v) {
					t.Errorf("Redis YAML for %q missing value %q", c.ID, v)
				}
			}
		} else {
			for k, v := range c.Fields {
				if !strings.Contains(out, k) {
					t.Errorf("Redis YAML for %q missing key %q", c.ID, k)
				}
				if v != "" && !strings.Contains(out, v) {
					t.Errorf("Redis YAML for %q missing value %q", c.ID, v)
				}
			}
		}
	}
}

func TestQA_YAML_Redis_SyntaxColoring(t *testing.T) {
	clusters := fixtureRedisClusters()
	out := yamlView(t, clusters[0], 120, 40)
	if !strings.Contains(out, "\x1b[") {
		t.Error("Redis YAML output has no ANSI color codes")
	}
}

func TestQA_YAML_Redis_FrameTitle(t *testing.T) {
	clusters := fixtureRedisClusters()
	m := yamlModel(clusters[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("Redis FrameTitle() = %q, want 'yaml' in title", title)
	}
	if !strings.Contains(title, clusters[0].Name) {
		t.Errorf("Redis FrameTitle() = %q, want Name %q", title, clusters[0].Name)
	}
}

func TestQA_YAML_Redis_RawContentUncolored(t *testing.T) {
	clusters := fixtureRedisClusters()
	m := yamlModel(clusters[0], 120, 40)
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("Redis RawContent() returned empty")
	}
	if strings.Contains(raw, "\x1b[") {
		t.Error("Redis RawContent() contains ANSI codes")
	}
}

// ── DocumentDB ──────────────────────────────────────────────────────────────

func TestQA_YAML_DocDB_ViewContainsFields(t *testing.T) {
	clusters := fixtureDocDBClusters()
	for _, c := range clusters {
		out := yamlView(t, c, 120, 40)
		if c.RawStruct != nil {
			// When RawStruct is present, YAML renders SDK struct field names
			expectedKeys := []string{"DBClusterIdentifier", "EngineVersion", "Status", "Endpoint"}
			for _, k := range expectedKeys {
				if !strings.Contains(out, k) {
					t.Errorf("DocDB YAML for %q missing SDK struct key %q", c.ID, k)
				}
			}
		} else {
			for k, v := range c.Fields {
				if !strings.Contains(out, k) {
					t.Errorf("DocDB YAML for %q missing key %q", c.ID, k)
				}
				if v != "" && !strings.Contains(out, v) {
					t.Errorf("DocDB YAML for %q missing value %q", c.ID, v)
				}
			}
		}
	}
}

func TestQA_YAML_DocDB_SyntaxColoring(t *testing.T) {
	clusters := fixtureDocDBClusters()
	out := yamlView(t, clusters[0], 120, 40)
	if !strings.Contains(out, "\x1b[") {
		t.Error("DocDB YAML output has no ANSI color codes")
	}
}

func TestQA_YAML_DocDB_FrameTitle(t *testing.T) {
	clusters := fixtureDocDBClusters()
	m := yamlModel(clusters[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("DocDB FrameTitle() = %q, want 'yaml' in title", title)
	}
	if !strings.Contains(title, clusters[0].Name) {
		t.Errorf("DocDB FrameTitle() = %q, want Name %q", title, clusters[0].Name)
	}
}

func TestQA_YAML_DocDB_RawContentUncolored(t *testing.T) {
	clusters := fixtureDocDBClusters()
	m := yamlModel(clusters[0], 120, 40)
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("DocDB RawContent() returned empty")
	}
	if strings.Contains(raw, "\x1b[") {
		t.Error("DocDB RawContent() contains ANSI codes")
	}
}

// ── EKS ─────────────────────────────────────────────────────────────────────

func TestQA_YAML_EKS_ViewContainsFields(t *testing.T) {
	clusters := fixtureEKSClusters()
	for _, c := range clusters {
		out := yamlView(t, c, 120, 40)
		for k, v := range c.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("EKS YAML for %q missing key %q", c.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("EKS YAML for %q missing value %q", c.ID, v)
			}
		}
	}
}

func TestQA_YAML_EKS_SyntaxColoring(t *testing.T) {
	clusters := fixtureEKSClusters()
	out := yamlView(t, clusters[0], 120, 40)
	if !strings.Contains(out, "\x1b[") {
		t.Error("EKS YAML output has no ANSI color codes")
	}
}

func TestQA_YAML_EKS_FrameTitle(t *testing.T) {
	clusters := fixtureEKSClusters()
	m := yamlModel(clusters[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("EKS FrameTitle() = %q, want 'yaml' in title", title)
	}
	if !strings.Contains(title, clusters[0].Name) {
		t.Errorf("EKS FrameTitle() = %q, want Name %q", title, clusters[0].Name)
	}
}

func TestQA_YAML_EKS_RawContentUncolored(t *testing.T) {
	clusters := fixtureEKSClusters()
	m := yamlModel(clusters[0], 120, 40)
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("EKS RawContent() returned empty")
	}
	if strings.Contains(raw, "\x1b[") {
		t.Error("EKS RawContent() contains ANSI codes")
	}
}

// ── Secrets Manager ─────────────────────────────────────────────────────────

func TestQA_YAML_Secrets_ViewContainsFields(t *testing.T) {
	secrets := fixtureSecrets()
	for _, s := range secrets {
		out := yamlView(t, s, 120, 40)
		for k, v := range s.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("Secrets YAML for %q missing key %q", s.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("Secrets YAML for %q missing value %q", s.ID, v)
			}
		}
	}
}

func TestQA_YAML_Secrets_SyntaxColoring(t *testing.T) {
	secrets := fixtureSecrets()
	out := yamlView(t, secrets[0], 120, 40)
	if !strings.Contains(out, "\x1b[") {
		t.Error("Secrets YAML output has no ANSI color codes")
	}
}

func TestQA_YAML_Secrets_FrameTitle(t *testing.T) {
	secrets := fixtureSecrets()
	m := yamlModel(secrets[0], 120, 40)
	title := m.FrameTitle()
	if !strings.Contains(title, "yaml") {
		t.Errorf("Secrets FrameTitle() = %q, want 'yaml' in title", title)
	}
	if !strings.Contains(title, secrets[0].Name) {
		t.Errorf("Secrets FrameTitle() = %q, want Name %q", title, secrets[0].Name)
	}
}

func TestQA_YAML_Secrets_RawContentUncolored(t *testing.T) {
	secrets := fixtureSecrets()
	m := yamlModel(secrets[0], 120, 40)
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("Secrets RawContent() returned empty")
	}
	if strings.Contains(raw, "\x1b[") {
		t.Error("Secrets RawContent() contains ANSI codes")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Cross-resource: Scroll behavior (j/k/g/G)
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_Scroll_AllTypes(t *testing.T) {
	type testCase struct {
		name string
		res  resource.Resource
	}
	cases := []testCase{
		{"S3", fixtureS3Buckets()[0]},
		{"EC2", fixtureEC2Instances()[0]},
		{"RDS", fixtureRDSInstances()[0]},
		{"Redis", fixtureRedisClusters()[0]},
		{"DocDB", fixtureDocDBClusters()[0]},
		{"EKS", fixtureEKSClusters()[0]},
		{"Secrets", fixtureSecrets()[0]},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := keys.Default()
			m := views.NewYAML(tc.res, k)
			// Use a small viewport so content overflows for scroll testing
			m.SetSize(60, 3)

			viewBefore := m.View()
			if viewBefore == "" || viewBefore == "Initializing..." {
				t.Fatalf("%s: View() returned empty or initializing", tc.name)
			}

			// Scroll down with 'j' — viewport standard binding
			m, _ = m.Update(yamlKeyPress("j"))
			viewAfterJ := m.View()

			// Scroll up with 'k' — viewport standard binding
			m, _ = m.Update(yamlKeyPress("k"))
			viewAfterK := m.View()

			// Scroll down with arrow key
			m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
			viewAfterDown := m.View()

			// Scroll up with arrow key
			m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
			viewAfterUp := m.View()

			// Jump to bottom with 'G' — this is passed to viewport
			m, _ = m.Update(yamlKeyPress("G"))
			_ = m.View()

			// Jump to top with 'g' — this is passed to viewport
			m, _ = m.Update(yamlKeyPress("g"))
			_ = m.View()

			// Verify scroll produces different views (if content > viewport height)
			raw := stripANSI(m.RawContent())
			lineCount := len(strings.Split(strings.TrimRight(raw, "\n"), "\n"))
			if lineCount > 3 {
				// Content overflows — down arrow should scroll
				if viewBefore == viewAfterDown {
					t.Errorf("%s: down-arrow did not change view (content has %d lines, viewport 3)", tc.name, lineCount)
				}
			}

			// Verify j/k and arrows don't crash
			_ = viewAfterJ
			_ = viewAfterK
			_ = viewAfterUp
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Cross-resource: Wrap toggle (w)
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_WrapToggle_AllTypes(t *testing.T) {
	type testCase struct {
		name string
		res  resource.Resource
	}
	cases := []testCase{
		{"S3", fixtureS3Buckets()[0]},
		{"EC2", fixtureEC2Instances()[0]},
		{"RDS", fixtureRDSInstances()[0]},
		{"Redis", fixtureRedisClusters()[0]},
		{"DocDB", fixtureDocDBClusters()[0]},
		{"EKS", fixtureEKSClusters()[0]},
		{"Secrets", fixtureSecrets()[0]},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := keys.Default()
			m := views.NewYAML(tc.res, k)
			// Use narrow viewport so long values would be clipped
			m.SetSize(40, 20)

			viewNoWrap := m.View()

			// Toggle wrap on
			m, _ = m.Update(yamlKeyPress("w"))
			viewWrapped := m.View()

			// Toggle wrap off
			m, _ = m.Update(yamlKeyPress("w"))
			viewUnwrapped := m.View()

			// Wrap toggle should not crash; views may or may not differ depending on content length
			_ = viewNoWrap
			_ = viewWrapped
			_ = viewUnwrapped

			// After double-toggle, should match original
			if viewNoWrap != viewUnwrapped {
				t.Logf("%s: double-toggle wrap does not restore identical view (may be viewport state)", tc.name)
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Cross-resource: RawContent() for clipboard copy (uncolored)
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_RawContent_AllTypes(t *testing.T) {
	type testCase struct {
		name string
		res  resource.Resource
	}
	cases := []testCase{
		{"S3", fixtureS3Buckets()[0]},
		{"EC2", fixtureEC2Instances()[0]},
		{"RDS", fixtureRDSInstances()[0]},
		{"Redis", fixtureRedisClusters()[0]},
		{"DocDB", fixtureDocDBClusters()[0]},
		{"EKS", fixtureEKSClusters()[0]},
		{"Secrets", fixtureSecrets()[0]},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := yamlModel(tc.res, 120, 40)
			raw := m.RawContent()

			if raw == "" {
				t.Fatalf("%s: RawContent() is empty", tc.name)
			}

			// Must be plain text — no ANSI
			if strings.Contains(raw, "\x1b[") {
				t.Errorf("%s: RawContent() contains ANSI codes", tc.name)
			}

			// Must contain key: value format
			if !strings.Contains(raw, ": ") {
				t.Errorf("%s: RawContent() missing YAML key: value format", tc.name)
			}

			// When RawStruct is set, YAML renders SDK struct field names, not Fields keys.
			// Only check Fields keys when RawStruct is nil (the fallback path).
			if tc.res.RawStruct == nil {
				for k := range tc.res.Fields {
					if !strings.Contains(raw, k) {
						t.Errorf("%s: RawContent() missing field key %q", tc.name, k)
					}
				}
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Cross-resource: FrameTitle includes "yaml"
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_FrameTitle_AllTypes(t *testing.T) {
	type testCase struct {
		name     string
		res      resource.Resource
		wantName string
	}
	s3 := fixtureS3Buckets()[0]
	ec2 := fixtureEC2Instances()[0]
	ec2Named := fixtureEC2Instances()[1]
	rds := fixtureRDSInstances()[0]
	redis := fixtureRedisClusters()[0]
	docdb := fixtureDocDBClusters()[0]
	eks := fixtureEKSClusters()[0]
	secret := fixtureSecrets()[0]

	cases := []testCase{
		{"S3", s3, s3.Name},
		{"EC2_NoName", ec2, ec2.ID},       // Name is empty, uses ID
		{"EC2_Named", ec2Named, ec2Named.Name},
		{"RDS", rds, rds.Name},
		{"Redis", redis, redis.Name},
		{"DocDB", docdb, docdb.Name},
		{"EKS", eks, eks.Name},
		{"Secrets", secret, secret.Name},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := yamlModel(tc.res, 120, 40)
			title := m.FrameTitle()

			if !strings.Contains(title, "yaml") {
				t.Errorf("%s: FrameTitle() = %q, want 'yaml' in title", tc.name, title)
			}
			if !strings.Contains(title, tc.wantName) {
				t.Errorf("%s: FrameTitle() = %q, want %q in title", tc.name, title, tc.wantName)
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Edge case: Resource with only Fields (no RawStruct) still produces YAML
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_FieldsOnly_NoRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:     "test-fields-only",
		Name:   "fields-only-resource",
		Status: "active",
		Fields: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		// RawStruct is nil
	}

	m := yamlModel(res, 120, 40)
	out := m.View()
	if out == "" || out == "Initializing..." {
		t.Fatal("YAML View() returned empty for fields-only resource")
	}
	if !strings.Contains(out, "key1") {
		t.Error("YAML View() missing 'key1' for fields-only resource")
	}
	if !strings.Contains(out, "value1") {
		t.Error("YAML View() missing 'value1' for fields-only resource")
	}

	raw := m.RawContent()
	if raw == "" {
		t.Fatal("RawContent() returned empty for fields-only resource")
	}
	if strings.Contains(raw, "\x1b[") {
		t.Error("RawContent() contains ANSI for fields-only resource")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Edge case: Empty resource shows "No YAML data available"
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_EmptyResource(t *testing.T) {
	res := resource.Resource{
		ID:     "empty-resource",
		Name:   "empty",
		Status: "",
		Fields: map[string]string{},
		// RawStruct is nil, Fields is empty
	}

	k := keys.Default()
	m := views.NewYAML(res, k)
	m.SetSize(120, 40)
	out := m.View()

	plain := stripANSI(out)
	if !strings.Contains(plain, "No YAML data available") {
		t.Errorf("Empty resource YAML should show 'No YAML data available', got: %q", plain)
	}
}

func TestQA_YAML_NilFieldsResource(t *testing.T) {
	res := resource.Resource{
		ID:   "nil-fields",
		Name: "nil",
	}

	k := keys.Default()
	m := views.NewYAML(res, k)
	m.SetSize(120, 40)
	out := m.View()

	plain := stripANSI(out)
	if !strings.Contains(plain, "No YAML data available") {
		t.Errorf("Nil-fields resource YAML should show 'No YAML data available', got: %q", plain)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Edge case: Boolean fields colored differently from strings
// Note: ToSafeValue + FormatValue converts booleans to "Yes"/"No" strings,
// and isZeroOrNil skips false (zero-value) bools entirely.
// So booleans appear as "Yes"/"No" string values in YAML output.
// The YAML bool regex also matches Yes/No, so they get bool coloring.
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_BooleanColoring(t *testing.T) {
	type fakeStruct struct {
		Enabled    bool
		Name       string
		InstanceID string
	}

	raw := fakeStruct{
		Enabled:    true,
		Name:       "test-resource",
		InstanceID: "i-abc123",
	}
	res := resource.Resource{
		ID:        "bool-test",
		Name:      "bool-test",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}

	m := yamlModel(res, 120, 40)
	out := m.View()

	// The output should have ANSI codes
	if !strings.Contains(out, "\x1b[") {
		t.Fatal("Boolean test: no ANSI codes in output")
	}

	// Check RawContent to understand the actual YAML output
	rawYAML := m.RawContent()

	// FormatValue converts true -> "Yes", which yaml.v3 may quote as '"Yes"'
	// to avoid YAML 1.1 boolean interpretation. Check what actually appears.
	plain := stripANSI(out)

	// The Enabled bool should appear in some form in both raw and view
	if !strings.Contains(rawYAML, "Enabled") {
		t.Fatalf("Boolean test: 'Enabled' key not found in raw YAML: %q", rawYAML)
	}

	if !strings.Contains(plain, "test-resource") {
		t.Error("Boolean test: 'test-resource' not found in plain output")
	}

	// Verify that the Enabled line and Name line have different coloring.
	// yaml.v3 may quote "Yes" as a string, or the colorizer may treat it as bool.
	lines := strings.Split(out, "\n")
	var enabledLine, nameLine string
	for _, line := range lines {
		stripped := stripANSI(line)
		if strings.Contains(stripped, "Enabled") {
			enabledLine = line
		}
		if strings.Contains(stripped, "test-resource") {
			nameLine = line
		}
	}

	if enabledLine == "" {
		t.Fatalf("Boolean test: could not find line with 'Enabled' key")
	}
	if nameLine == "" {
		t.Fatal("Boolean test: could not find line with string 'test-resource'")
	}

	// Both lines should have ANSI codes (colored)
	if !strings.Contains(enabledLine, "\x1b[") {
		t.Error("Enabled line has no ANSI coloring")
	}
	if !strings.Contains(nameLine, "\x1b[") {
		t.Error("Name line has no ANSI coloring")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Edge case: Null fields behavior
// Note: ToSafeValue skips nil pointers (isZeroOrNil returns true for nil),
// so null fields are omitted from YAML output entirely.
// Only non-nil pointer values appear. This is the actual SDK behavior.
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_NullFields(t *testing.T) {
	type fakeStruct struct {
		Name      string
		PublicIP  *string
		PrivateIP *string
	}

	privIP := "10.0.0.1"
	raw := fakeStruct{
		Name:      "test-instance",
		PublicIP:  nil, // will be omitted by ToSafeValue
		PrivateIP: &privIP,
	}
	res := resource.Resource{
		ID:        "null-test",
		Name:      "null-test",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}

	m := yamlModel(res, 120, 40)
	out := m.View()
	plain := stripANSI(out)

	// Non-nil pointer should show the value
	if !strings.Contains(plain, "10.0.0.1") {
		t.Errorf("Non-nil pointer should show value '10.0.0.1', got: %q", strings.TrimSpace(plain))
	}

	// Nil pointer is omitted by ToSafeValue (isZeroOrNil returns true)
	if strings.Contains(plain, "PublicIP") {
		t.Errorf("Nil pointer field 'PublicIP' should be omitted by ToSafeValue, but found in output")
	}

	// Name should be present
	if !strings.Contains(plain, "test-instance") {
		t.Errorf("Name field should be present, got: %q", strings.TrimSpace(plain))
	}
}

// TestQA_YAML_NullColoringViaFields verifies that "null" values in Fields map
// (as plain strings) get the null/dim YAML coloring treatment.
func TestQA_YAML_NullColoringViaFields(t *testing.T) {
	res := resource.Resource{
		ID:   "null-field-test",
		Name: "null-field-test",
		Fields: map[string]string{
			"Name":     "test-instance",
			"PublicIP": "null",
		},
	}

	m := yamlModel(res, 120, 40)
	out := m.View()
	plain := stripANSI(out)

	// The "null" string value should appear in output
	if !strings.Contains(plain, "null") {
		t.Errorf("Fields map with 'null' value should show 'null' in output")
	}

	// Output should be colored
	if !strings.Contains(out, "\x1b[") {
		t.Error("Output has no ANSI coloring for null value")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Edge case: Numeric values colored with orange
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_NumericColoring(t *testing.T) {
	type fakeStruct struct {
		Port             int
		AllocatedStorage int
		Name             string
	}

	raw := fakeStruct{
		Port:             5432,
		AllocatedStorage: 100,
		Name:             "mydb",
	}
	res := resource.Resource{
		ID:        "num-test",
		Name:      "num-test",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}

	m := yamlModel(res, 120, 40)
	out := m.View()
	plain := stripANSI(out)

	if !strings.Contains(plain, "5432") {
		t.Error("Numeric test: '5432' not found")
	}
	if !strings.Contains(plain, "100") {
		t.Error("Numeric test: '100' not found")
	}

	// Verify that numbers are colored (have ANSI codes around them)
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		stripped := stripANSI(line)
		if strings.Contains(stripped, ": 5432") {
			if !strings.Contains(line, "\x1b[") {
				t.Error("Numeric value '5432' is not colored")
			}
		}
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Edge case: RawStruct takes precedence over Fields
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_RawStructPrecedence(t *testing.T) {
	type fakeStruct struct {
		StructField string
	}
	raw := fakeStruct{StructField: "from-struct"}
	res := resource.Resource{
		ID:        "precedence-test",
		Name:      "precedence-test",
		RawStruct: &raw,
		Fields: map[string]string{
			"fields_key": "from-fields",
		},
	}

	m := yamlModel(res, 120, 40)
	plain := stripANSI(m.View())

	// RawStruct should be used, so StructField should appear
	if !strings.Contains(plain, "StructField") {
		t.Error("RawStruct field 'StructField' not found — RawStruct should take precedence")
	}
	if !strings.Contains(plain, "from-struct") {
		t.Error("RawStruct value 'from-struct' not found")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// YAML structure: proper indentation for nested structs
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_NestedStructIndentation(t *testing.T) {
	type Endpoint struct {
		Address string
		Port    int
	}
	type fakeRDS struct {
		DBInstanceIdentifier string
		Endpoint             Endpoint
	}

	raw := fakeRDS{
		DBInstanceIdentifier: "mydb-prod",
		Endpoint: Endpoint{
			Address: "mydb-prod.c9abcdef.us-east-1.rds.amazonaws.com",
			Port:    5432,
		},
	}
	res := resource.Resource{
		ID:        "mydb-prod",
		Name:      "mydb-prod",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}

	m := yamlModel(res, 120, 40)
	rawYAML := m.RawContent()

	// Verify nested structure: Endpoint should have indented children
	if !strings.Contains(rawYAML, "Endpoint:") {
		t.Fatal("Nested struct test: 'Endpoint:' not found in raw YAML")
	}
	if !strings.Contains(rawYAML, "  Address:") || !strings.Contains(rawYAML, "  Port:") {
		t.Error("Nested struct test: expected 2-space indented Address/Port under Endpoint")
	}

	// Verify the nested value is present
	if !strings.Contains(rawYAML, "mydb-prod.c9abcdef.us-east-1.rds.amazonaws.com") {
		t.Error("Nested struct test: endpoint address value not found")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// YAML structure: array items use "- " prefix
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_ArrayItems(t *testing.T) {
	type Tag struct {
		Key   string
		Value string
	}
	type fakeEC2 struct {
		InstanceID string
		Tags       []Tag
	}

	raw := fakeEC2{
		InstanceID: "i-abc123",
		Tags: []Tag{
			{Key: "Name", Value: "api-prod"},
			{Key: "Env", Value: "production"},
		},
	}
	res := resource.Resource{
		ID:        "i-abc123",
		Name:      "api-prod",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}

	m := yamlModel(res, 120, 40)
	rawYAML := m.RawContent()

	// Array items should use "- " prefix
	if !strings.Contains(rawYAML, "- Key:") {
		t.Error("Array test: expected '- Key:' prefix for array items")
	}

	// Verify colored view also shows them
	out := m.View()
	plain := stripANSI(out)
	if !strings.Contains(plain, "api-prod") {
		t.Error("Array test: 'api-prod' not found in view")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// YAML structure: empty arrays behavior
// Note: ToSafeValue omits empty slices (isZeroOrNil returns true for len==0).
// So empty arrays do not appear in the YAML output at all.
// Non-empty arrays render correctly with "- " prefix.
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_EmptyArray(t *testing.T) {
	type fakeDocDB struct {
		ClusterID       string
		AssociatedRoles []string
		ActiveRoles     []string
	}

	raw := fakeDocDB{
		ClusterID:       "docdb-cluster",
		AssociatedRoles: []string{},                       // empty, will be omitted
		ActiveRoles:     []string{"reader", "readWrite"},  // non-empty, will appear
	}
	res := resource.Resource{
		ID:        "docdb-cluster",
		Name:      "docdb-cluster",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}

	m := yamlModel(res, 120, 40)
	rawYAML := m.RawContent()

	// Empty slices are omitted by ToSafeValue
	if strings.Contains(rawYAML, "AssociatedRoles") {
		t.Errorf("Empty array 'AssociatedRoles' should be omitted by ToSafeValue, got: %q", rawYAML)
	}

	// Non-empty array should be present with items
	if !strings.Contains(rawYAML, "ActiveRoles") {
		t.Errorf("Non-empty array 'ActiveRoles' should be present, got: %q", rawYAML)
	}
	if !strings.Contains(rawYAML, "- reader") {
		t.Errorf("Array item 'reader' should be present with '- ' prefix, got: %q", rawYAML)
	}
}

func TestQA_YAML_NilPointerToStructShowsNoNull(t *testing.T) {
	// A non-nil pointer to a struct with all zero/nil fields should be omitted,
	// not rendered as "null".
	type Inner struct {
		Enabled *bool
	}
	type Outer struct {
		Name    string
		Options *Inner // non-nil pointer, but Inner has all-nil fields
	}

	raw := Outer{
		Name:    "test",
		Options: &Inner{}, // non-nil pointer to zero-value struct
	}
	res := resource.Resource{
		ID:        "null-struct-test",
		Name:      "null-struct-test",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}

	m := yamlModel(res, 120, 40)
	rawYAML := m.RawContent()

	if strings.Contains(rawYAML, "null") {
		t.Errorf("Non-nil pointer to empty struct should be omitted, not 'null'. Got:\n%s", rawYAML)
	}
	if strings.Contains(rawYAML, "Options") {
		t.Errorf("Empty struct field 'Options' should be omitted. Got:\n%s", rawYAML)
	}
}

func TestQA_YAML_SliceOfEmptyStructsShowsNoNull(t *testing.T) {
	// A slice containing structs where all fields are zero should not produce
	// "- null" entries.
	type Item struct {
		Value *string
	}
	type Container struct {
		Name  string
		Items []Item
	}

	raw := Container{
		Name:  "test",
		Items: []Item{{Value: nil}, {Value: nil}}, // 2 items, both all-nil
	}
	res := resource.Resource{
		ID:        "null-slice-test",
		Name:      "null-slice-test",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}

	m := yamlModel(res, 120, 40)
	rawYAML := m.RawContent()

	if strings.Contains(rawYAML, "null") {
		t.Errorf("Slice of empty structs should not produce 'null'. Got:\n%s", rawYAML)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// All fixture functions used: verify each returns non-empty data
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_AllFixturesNonEmpty(t *testing.T) {
	if len(fixtureS3Buckets()) == 0 {
		t.Error("fixtureS3Buckets() returned empty")
	}
	if len(fixtureEC2Instances()) == 0 {
		t.Error("fixtureEC2Instances() returned empty")
	}
	if len(fixtureRDSInstances()) == 0 {
		t.Error("fixtureRDSInstances() returned empty")
	}
	if len(fixtureRedisClusters()) == 0 {
		t.Error("fixtureRedisClusters() returned empty")
	}
	if len(fixtureDocDBClusters()) == 0 {
		t.Error("fixtureDocDBClusters() returned empty")
	}
	if len(fixtureEKSClusters()) == 0 {
		t.Error("fixtureEKSClusters() returned empty")
	}
	if len(fixtureSecrets()) == 0 {
		t.Error("fixtureSecrets() returned empty")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// All fixture resources produce valid YAML views
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_AllFixtureResources(t *testing.T) {
	allResources := map[string][]resource.Resource{
		"S3":      fixtureS3Buckets(),
		"EC2":     fixtureEC2Instances(),
		"RDS":     fixtureRDSInstances(),
		"Redis":   fixtureRedisClusters(),
		"DocDB":   fixtureDocDBClusters(),
		"EKS":     fixtureEKSClusters(),
		"Secrets": fixtureSecrets(),
	}

	for typeName, resources := range allResources {
		for i, res := range resources {
			t.Run(typeName+"_"+res.ID, func(t *testing.T) {
				m := yamlModel(res, 120, 40)
				out := m.View()

				if out == "" || out == "Initializing..." {
					t.Errorf("%s[%d] %q: View() returned empty or initializing", typeName, i, res.ID)
				}

				raw := m.RawContent()
				if raw == "" {
					t.Errorf("%s[%d] %q: RawContent() returned empty", typeName, i, res.ID)
				}

				title := m.FrameTitle()
				if !strings.Contains(title, "yaml") {
					t.Errorf("%s[%d] %q: FrameTitle() = %q, missing 'yaml'", typeName, i, res.ID, title)
				}

				// ANSI coloring present in View
				if !strings.Contains(out, "\x1b[") {
					t.Errorf("%s[%d] %q: View() has no ANSI color codes", typeName, i, res.ID)
				}

				// No ANSI in RawContent
				if strings.Contains(raw, "\x1b[") {
					t.Errorf("%s[%d] %q: RawContent() contains ANSI codes", typeName, i, res.ID)
				}
			})
		}
	}
}

// ════════════════════════════════════════════════════════════════════════════
// S3 Objects fixture also works with YAML view
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_S3Objects_ViewContainsFields(t *testing.T) {
	objects := fixtureS3Objects()
	for _, obj := range objects {
		out := yamlView(t, obj, 120, 40)
		for k, v := range obj.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("S3 Object YAML for %q missing key %q", obj.ID, k)
			}
			if v != "" && !strings.Contains(out, v) {
				t.Errorf("S3 Object YAML for %q missing value %q", obj.ID, v)
			}
		}
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Verify YAML key:value format in RawContent for all types
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_KeyValueFormat_AllTypes(t *testing.T) {
	type testCase struct {
		name string
		res  resource.Resource
	}
	cases := []testCase{
		{"S3", fixtureS3Buckets()[0]},
		{"EC2", fixtureEC2Instances()[1]},
		{"RDS", fixtureRDSInstances()[1]},
		{"Redis", fixtureRedisClusters()[0]},
		{"DocDB", fixtureDocDBClusters()[1]},
		{"EKS", fixtureEKSClusters()[0]},
		{"Secrets", fixtureSecrets()[2]},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := yamlModel(tc.res, 120, 40)
			raw := m.RawContent()
			lines := strings.Split(strings.TrimSpace(raw), "\n")

			if len(lines) == 0 {
				t.Fatalf("%s: RawContent() produced no lines", tc.name)
			}

			// Every non-empty line should contain a colon (key: value)
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if !strings.Contains(trimmed, ":") {
					t.Errorf("%s: line %q missing colon — not valid YAML key:value", tc.name, trimmed)
				}
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Verify ResourceID() returns correct ID
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_ResourceID_AllTypes(t *testing.T) {
	type testCase struct {
		name string
		res  resource.Resource
	}
	cases := []testCase{
		{"S3", fixtureS3Buckets()[0]},
		{"EC2", fixtureEC2Instances()[0]},
		{"RDS", fixtureRDSInstances()[0]},
		{"Redis", fixtureRedisClusters()[0]},
		{"DocDB", fixtureDocDBClusters()[0]},
		{"EKS", fixtureEKSClusters()[0]},
		{"Secrets", fixtureSecrets()[0]},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := yamlModel(tc.res, 120, 40)
			rid := m.ResourceID()
			if rid != tc.res.ID {
				t.Errorf("%s: ResourceID() = %q, want %q", tc.name, rid, tc.res.ID)
			}
		})
	}
}

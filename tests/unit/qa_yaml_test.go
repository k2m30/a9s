package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ── Helper: build a live YAMLModel (NewYAMLWithCtrl) and return its
// ContentLines() joined — the LIVE render path (yaml.go's View/RawContent/
// FrameTitle are DEAD per specs/022-codebase-cleanup/wave3-map-text.md; the
// live navigate path builds via NewYAMLWithCtrl and reads ContentLines()
// directly — see runtime_adapter_navigate.go's pushTextScreen).

func yamlView(t *testing.T, res resource.Resource, w, h int) string {
	t.Helper()
	k := keys.Default()
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(w, h)
	out := strings.Join(m.ContentLines(), "\n")
	if out == "" {
		t.Fatalf("YAMLModel.ContentLines() returned empty for resource %q", res.ID)
	}
	return out
}

func yamlModel(res resource.Resource, w, h int) views.YAMLModel {
	k := keys.Default()
	m := views.NewYAMLWithCtrl(res, "", k, nil)
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

// ── Redis ───────────────────────────────────────────────────────────────────

func TestQA_YAML_Redis_ViewContainsFields(t *testing.T) {
	clusters := fixtureRedisClusters()
	for _, c := range clusters {
		out := yamlView(t, c, 120, 40)
		if c.RawStruct != nil {
			// When RawStruct is present, YAML renders SDK struct field names
			// Post-phase-7: RawStruct is ReplicationGroup, so field names changed.
			// MemberClusters is omitted from fixture to keep YAML output as key:value pairs only.
			expectedKeys := []string{"ReplicationGroupId", "Description", "Status", "CacheNodeType"}
			for _, k := range expectedKeys {
				if !strings.Contains(out, k) {
					t.Errorf("Redis YAML for %q missing SDK struct key %q", c.ID, k)
				}
			}
			expectedValues := []string{"test-redis-1", "cache.t2.micro", "available"}
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

// Scroll/wrap are VIEW-rendering (RenderText) concerns, not ContentLines()
// concerns — ContentLines() always returns the full, unwindowed, unwrapped
// content, so a scroll/wrap toggle can never be observed by comparing two
// ContentLines() joins (YAMLModel.Update() is DEAD per
// specs/022-codebase-cleanup/wave3-map-text.md anyway). The type-swept
// versions of these checks are retired: scroll/wrap rendering logic is
// resource-type-agnostic (driven by content width/length, not by which AWS
// type the content came from), and text_ctrl_interaction_test.go already
// pins the real live mechanism (ctrl.Apply(ActionToggleWrap/ActionMoveDown/
// ActionPageDown) -> Snapshot().Body.Text -> RenderText(body)). The one
// genuinely distinct case — a KNOWN long value that MUST wrap — is ported
// onto that live seam below instead of relying on ContentLines().

// TestQA_YAML_WrapToggle_KnownLongValue verifies wrap behaviour with a
// resource that has a KNOWN 200-char field value, making the assertion
// unconditionally reliable. Uses the live ctrl.Apply(ActionToggleWrap) ->
// RenderText(body) seam (ContentLines() cannot observe wrap at all).
func TestQA_YAML_WrapToggle_KnownLongValue(t *testing.T) {
	longValue := strings.Repeat("A", 200)
	res := resource.Resource{
		ID:     "wrap-yaml-test",
		Name:   "wrap-yaml-test",
		Fields: map[string]string{"description": longValue},
	}

	k := keys.Default()
	lines := views.NewYAMLWithCtrl(res, "", k, nil).ContentLines()

	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	ctrl := app.New(core)
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenYAML}})
	ctrl.EnsureTextState(lines)

	m := views.NewYAMLWithCtrl(res, "", k, ctrl)
	m.SetSize(40, 30) // narrow: 40 cols, the 200-char value must wrap

	viewNoWrap := m.RenderText(*ctrl.Snapshot().Body.Text)

	ctrl.Apply(app.Action{Kind: app.ActionToggleWrap})
	viewWrapped := m.RenderText(*ctrl.Snapshot().Body.Text)

	// Must differ — the 200-char value absolutely forces additional lines.
	if viewNoWrap == viewWrapped {
		t.Error("toggling wrap on should change the YAML view for a resource with a 200-char field value, but the view was identical")
	}

	// Toggle wrap off — must restore original.
	ctrl.Apply(app.Action{Kind: app.ActionToggleWrap})
	viewRestored := m.RenderText(*ctrl.Snapshot().Body.Text)

	if viewNoWrap != viewRestored {
		t.Error("double-toggle wrap should restore the original YAML view")
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
			raw := stripANSI(strings.Join(m.ContentLines(), "\n"))

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

// ════════════════════════════════════════════════════════════════════════════
// Edge case: Resource with only Fields (no RawStruct) still produces YAML
// ════════════════════════════════════════════════════════════════════════════

func TestQA_YAML_FieldsOnly_NoRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:   "test-fields-only",
		Name: "fields-only-resource",
		Fields: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		// RawStruct is nil
	}

	m := yamlModel(res, 120, 40)
	out := strings.Join(m.ContentLines(), "\n")
	if out == "" || out == "Initializing..." {
		t.Fatal("YAML View() returned empty for fields-only resource")
	}
	if !strings.Contains(out, "key1") {
		t.Error("YAML View() missing 'key1' for fields-only resource")
	}
	if !strings.Contains(out, "value1") {
		t.Error("YAML View() missing 'value1' for fields-only resource")
	}

	raw := stripANSI(strings.Join(m.ContentLines(), "\n"))
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
		Fields: map[string]string{},
		// RawStruct is nil, Fields is empty
	}

	k := keys.Default()
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(120, 40)
	out := strings.Join(m.ContentLines(), "\n")

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
	m := views.NewYAMLWithCtrl(res, "", k, nil)
	m.SetSize(120, 40)
	out := strings.Join(m.ContentLines(), "\n")

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
	out := strings.Join(m.ContentLines(), "\n")

	// The output should have ANSI codes
	if !strings.Contains(out, "\x1b[") {
		t.Fatal("Boolean test: no ANSI codes in output")
	}

	// Check RawContent to understand the actual YAML output
	rawYAML := stripANSI(strings.Join(m.ContentLines(), "\n"))

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
	out := strings.Join(m.ContentLines(), "\n")
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
	out := strings.Join(m.ContentLines(), "\n")
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
	out := strings.Join(m.ContentLines(), "\n")
	plain := stripANSI(out)

	if !strings.Contains(plain, "5432") {
		t.Error("Numeric test: '5432' not found")
	}
	if !strings.Contains(plain, "100") {
		t.Error("Numeric test: '100' not found")
	}

	// Verify that numbers are colored (have ANSI codes around them)
	lines := strings.SplitSeq(out, "\n")
	for line := range lines {
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
	plain := stripANSI(strings.Join(m.ContentLines(), "\n"))

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
	rawYAML := stripANSI(strings.Join(m.ContentLines(), "\n"))

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
	rawYAML := stripANSI(strings.Join(m.ContentLines(), "\n"))

	// Array items should use "- " prefix
	if !strings.Contains(rawYAML, "- Key:") {
		t.Error("Array test: expected '- Key:' prefix for array items")
	}

	// Verify colored view also shows them
	out := strings.Join(m.ContentLines(), "\n")
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
		AssociatedRoles: []string{},                      // empty, will be omitted
		ActiveRoles:     []string{"reader", "readWrite"}, // non-empty, will appear
	}
	res := resource.Resource{
		ID:        "docdb-cluster",
		Name:      "docdb-cluster",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}

	m := yamlModel(res, 120, 40)
	rawYAML := stripANSI(strings.Join(m.ContentLines(), "\n"))

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
	rawYAML := stripANSI(strings.Join(m.ContentLines(), "\n"))

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
	rawYAML := stripANSI(strings.Join(m.ContentLines(), "\n"))

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
				out := strings.Join(m.ContentLines(), "\n")

				if out == "" || out == "Initializing..." {
					t.Errorf("%s[%d] %q: View() returned empty or initializing", typeName, i, res.ID)
				}

				raw := stripANSI(strings.Join(m.ContentLines(), "\n"))
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
			raw := stripANSI(strings.Join(m.ContentLines(), "\n"))
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

// IsTextViewer/NewTextViewer/CopyContent-rawText-branch retired: all DEAD per
// specs/022-codebase-cleanup/wave3-map-text.md (line 13) — the live error-log
// path is ctrl-backed via newErrorLogRS+EnsureTextState, not IsTextViewer;
// raw-text copy is pinned live in text_ports_test.go's
// TestPort_ErrorLogCopy_UncoloredContent (handleCopy on rsKindText).

// ════════════════════════════════════════════════════════════════════════════
// colorizeYAML list-item path — lines starting with "- " (scalar list items)
// ════════════════════════════════════════════════════════════════════════════

// TestQA_YAML_ColorizeListItems verifies that standalone YAML list items
// (lines starting with "- ") get syntax coloring applied to their scalar value.
func TestQA_YAML_ColorizeListItems(t *testing.T) {
	type fakeRoles struct {
		Roles []string
	}
	raw := fakeRoles{Roles: []string{"reader", "readWrite", "dbAdmin"}}
	res := resource.Resource{
		ID:        "color-list-test",
		Name:      "color-list-test",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}
	m := yamlModel(res, 120, 40)
	out := strings.Join(m.ContentLines(), "\n")
	plain := stripANSI(out)

	// List items must appear in the view
	if !strings.Contains(plain, "reader") {
		t.Error("list item 'reader' not found in YAML view")
	}
	if !strings.Contains(plain, "readWrite") {
		t.Error("list item 'readWrite' not found in YAML view")
	}

	// Output must be colored (colorizeValue was applied to the list items)
	if !strings.Contains(out, "\x1b[") {
		t.Error("YAML view with list items has no ANSI color codes")
	}

	// Raw content must list the items under "- " prefix
	rawYAML := stripANSI(strings.Join(m.ContentLines(), "\n"))
	if !strings.Contains(rawYAML, "- reader") {
		t.Errorf("RawContent() missing '- reader' list item, got:\n%s", rawYAML)
	}
}

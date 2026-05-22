package unit_test

// qa_bottom_hints_test.go — tests for BottomHints() on MainMenuModel,
// ResourceListModel, DetailModel, and YAMLModel.
//
// Covers spec #197: bottom border key hints for all 4 view models.

import (
	"context"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// hintKeys extracts the Key field from each hint.
func hintKeys(hints []layout.KeyHint) []string {
	out := make([]string, len(hints))
	for i, h := range hints {
		out[i] = h.Key
	}
	return out
}

// hintDescs extracts the Desc field from each hint.
func hintDescs(hints []layout.KeyHint) []string {
	out := make([]string, len(hints))
	for i, h := range hints {
		out[i] = h.Desc
	}
	return out
}

// hasHint returns true if any hint in the slice has the given key.
func hasHint(hints []layout.KeyHint, key string) bool {
	for _, h := range hints {
		if h.Key == key {
			return true
		}
	}
	return false
}

// hintsEqual returns true if two KeyHint slices have identical Key+Desc pairs
// in the same order.
func hintsEqual(a, b []layout.KeyHint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Key != b[i].Key || a[i].Desc != b[i].Desc {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// A. MainMenu tests
// ---------------------------------------------------------------------------

func TestBottomHints_MainMenu_Normal(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 24)
	hints := m.BottomHints()

	want := []layout.KeyHint{
		{Key: "ctrl+z", Desc: "Issues only"},
		{Key: "ctrl+r", Desc: "Refresh"},
	}
	if !hintsEqual(hints, want) {
		t.Errorf("MainMenu BottomHints = %v, want %v", hints, want)
	}
}

// ---------------------------------------------------------------------------
// B. ResourceList tests
// ---------------------------------------------------------------------------

func TestBottomHints_ResourceList_NoEnterChild(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "hints_test_ec2_no_child",
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)

	hints := m.BottomHints()
	// "hints_test_ec2_no_child" has no CloudTrailKey — t hint suppressed
	wantKeys := []string{"y", "J", "ctrl+r", "ctrl+z"}
	if got := hintKeys(hints); !stringSliceEqual(got, wantKeys) {
		t.Errorf("BottomHints keys = %v, want %v", got, wantKeys)
	}
}

func TestBottomHints_ResourceList_WithEnterChild(t *testing.T) {
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects_hints_test",
	})
	t.Cleanup(func() { resource.CleanupChildTypeForTest("s3_objects_hints_test") })

	td := resource.ResourceTypeDef{
		Name:      "S3 Buckets",
		ShortName: "s3_test_hints",
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
		Children: []resource.ChildViewDef{
			{ChildType: "s3_objects_hints_test", Key: "enter"},
		},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)

	hints := m.BottomHints()
	// "s3_test_hints" has no CloudTrailKey — t hint suppressed
	wantKeys := []string{"enter", "d", "y", "J", "ctrl+r", "ctrl+z"}
	if got := hintKeys(hints); !stringSliceEqual(got, wantKeys) {
		t.Errorf("BottomHints keys = %v, want %v", got, wantKeys)
	}

	descs := hintDescs(hints)
	if descs[0] != "S3 Objects" {
		t.Errorf("enter hint Desc = %q, want %q", descs[0], "S3 Objects")
	}
	if descs[1] != "Detail" {
		t.Errorf("d hint Desc = %q, want %q", descs[1], "Detail")
	}
}

func TestBottomHints_ResourceList_WithReveal(t *testing.T) {
	resource.SetRevealFetcherForTest("secrets_test_hints", func(_ context.Context, _ any, _ string) (string, error) {
		return "", nil
	})
	t.Cleanup(func() { resource.CleanupRevealFetcherForTest("secrets_test_hints") })

	td := resource.ResourceTypeDef{
		Name:      "Secrets",
		ShortName: "secrets_test_hints",
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)

	hints := m.BottomHints()
	// "secrets_test_hints" has no CloudTrailKey — t hint suppressed
	wantKeys := []string{"x", "y", "J", "ctrl+r", "ctrl+z"}
	if got := hintKeys(hints); !stringSliceEqual(got, wantKeys) {
		t.Errorf("BottomHints keys = %v, want %v", got, wantKeys)
	}
	if !hasHint(hints, "x") {
		t.Error("expected hint with key 'x' for Reveal")
	}
	for _, h := range hints {
		if h.Key == "x" && h.Desc != "Reveal" {
			t.Errorf("reveal hint Desc = %q, want %q", h.Desc, "Reveal")
		}
	}
}

func TestBottomHints_ResourceList_EscPops(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "hints_test_ec2_escp",
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetEscPops(true)

	hints := m.BottomHints()
	if len(hints) == 0 {
		t.Fatal("BottomHints returned empty slice")
	}
	if hints[0].Key != "esc" || hints[0].Desc != "Back" {
		t.Errorf("first hint = {%q, %q}, want {esc, Back}", hints[0].Key, hints[0].Desc)
	}
}

func TestBottomHints_ResourceList_MultipleChildKeys(t *testing.T) {
	resource.SetChildTypeForTest(resource.ResourceTypeDef{Name: "ECS Tasks", ShortName: "ecs_tasks_hints_test"})
	resource.SetChildTypeForTest(resource.ResourceTypeDef{Name: "ECS Events", ShortName: "ecs_events_hints_test"})
	resource.SetChildTypeForTest(resource.ResourceTypeDef{Name: "ECS Logs", ShortName: "ecs_logs_hints_test"})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest("ecs_tasks_hints_test")
		resource.CleanupChildTypeForTest("ecs_events_hints_test")
		resource.CleanupChildTypeForTest("ecs_logs_hints_test")
	})

	td := resource.ResourceTypeDef{
		Name:      "ECS Services",
		ShortName: "ecs_test_hints",
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
		Children: []resource.ChildViewDef{
			{ChildType: "ecs_tasks_hints_test", Key: "enter"},
			{ChildType: "ecs_events_hints_test", Key: "e"},
			{ChildType: "ecs_logs_hints_test", Key: "L"},
		},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)

	hints := m.BottomHints()
	// "ecs_test_hints" has no CloudTrailKey — t hint suppressed
	wantKeys := []string{"enter", "d", "y", "J", "e", "L", "ctrl+r", "ctrl+z"}
	if got := hintKeys(hints); !stringSliceEqual(got, wantKeys) {
		t.Errorf("BottomHints keys = %v, want %v", got, wantKeys)
	}

	// Verify non-enter child descs resolve to registered names
	for _, h := range hints {
		switch h.Key {
		case "e":
			if h.Desc != "ECS Events" {
				t.Errorf("e hint Desc = %q, want %q", h.Desc, "ECS Events")
			}
		case "L":
			if h.Desc != "ECS Logs" {
				t.Errorf("L hint Desc = %q, want %q", h.Desc, "ECS Logs")
			}
		}
	}
}

func TestBottomHints_ResourceList_Pagination_WithMore(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "hints_test_ec2_paged",
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	}
	k := keys.Default()
	pagination := &resource.PaginationMeta{IsTruncated: true, NextToken: "tok"}
	m := views.NewResourceListFromCache(td, nil, k, nil, pagination, "", views.SortColNone, true, 0, 0, false)

	hints := m.BottomHints()
	if !hasHint(hints, "m") {
		t.Error("expected hint with key 'm' for More when pagination is truncated")
	}
	// "m" should be last
	last := hints[len(hints)-1]
	if last.Key != "m" || last.Desc != "More" {
		t.Errorf("last hint = {%q, %q}, want {m, More}", last.Key, last.Desc)
	}
}

func TestBottomHints_ResourceList_Pagination_NoMore(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "hints_test_ec2_nopaged",
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	}
	k := keys.Default()
	m := views.NewResourceListFromCache(td, nil, k, nil, nil, "", views.SortColNone, true, 0, 0, false)

	hints := m.BottomHints()
	if hasHint(hints, "m") {
		t.Error("unexpected 'm' hint when pagination is nil")
	}
}

// ---------------------------------------------------------------------------
// C. Detail tests
// ---------------------------------------------------------------------------

func TestBottomHints_Detail_PlainField_NoRelated(t *testing.T) {
	res := resource.Resource{
		ID:   "test-id",
		Name: "test-resource",
	}
	m := views.NewDetail(res, "hints_test_no_related", nil, keys.Default())

	hints := m.BottomHints()
	// Unknown resource type "hints_test_no_related" has no CloudTrailKey — no t hint
	want := []layout.KeyHint{
		{Key: "y", Desc: "YAML"},
		{Key: "J", Desc: "JSON"},
		{Key: "ctrl+r", Desc: "Refresh"},
		{Key: "w", Desc: "Wrap"},
	}
	if !hintsEqual(hints, want) {
		t.Errorf("Detail BottomHints = %v, want %v", hints, want)
	}
}

func TestBottomHints_Detail_PlainField_WithRelated(t *testing.T) {
	resource.SetRelatedForTest("hints_test_with_related", []resource.RelatedDef{
		{
			TargetType:  "vpc",
			DisplayName: "VPC",
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{}
			},
		},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest("hints_test_with_related") })

	res := resource.Resource{
		ID:   "test-id",
		Name: "test-resource",
	}
	m := views.NewDetail(res, "hints_test_with_related", nil, keys.Default())

	hints := m.BottomHints()
	// Unknown resource type "hints_test_with_related" has no CloudTrailKey — no t hint
	want := []layout.KeyHint{
		{Key: "y", Desc: "YAML"},
		{Key: "J", Desc: "JSON"},
		{Key: "r", Desc: "Related"},
		{Key: "ctrl+r", Desc: "Refresh"},
		{Key: "w", Desc: "Wrap"},
	}
	if !hintsEqual(hints, want) {
		t.Errorf("Detail BottomHints = %v, want %v", hints, want)
	}
}

// testNavDetailEC2 is a minimal stand-in for EC2 used in navigable hints tests.
type testNavDetailEC2 struct {
	VpcId        *string
	InstanceType string
}

func TestBottomHints_Detail_NavigableField(t *testing.T) {
	replaceEC2NavigableFields(t, []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})

	vpcID := "vpc-test456"
	res := resource.Resource{
		ID:   "i-test456",
		Name: "test-ec2",
		Fields: map[string]string{
			"vpc_id": "vpc-test456",
		},
		RawStruct: &testNavDetailEC2{
			VpcId:        &vpcID,
			InstanceType: "t3.medium",
		},
	}

	vc := navViewConfig()
	m := views.NewDetail(res, "ec2", vc, keys.Default())
	m.SetSize(140, 30)
	// Trigger fieldList build
	_ = m.View()

	hints := m.BottomHints()

	if len(hints) == 0 {
		t.Fatal("BottomHints returned empty slice")
	}
	first := hints[0]
	if first.Key != "enter" {
		t.Errorf("first hint Key = %q, want %q", first.Key, "enter")
	}
	// Desc should resolve "vpc" to its registered resource type name
	rt := resource.FindResourceType("vpc")
	wantDesc := "vpc"
	if rt != nil {
		wantDesc = rt.Name
	}
	if first.Desc != wantDesc {
		t.Errorf("navigable enter hint Desc = %q, want %q", first.Desc, wantDesc)
	}
}

// TestBottomHints_Detail_RightColVisible_TabCols verifies that "tab/Cols" appears
// when the right column is explicitly toggled visible (rightColVisible=true) and
// related defs are registered.
// NOTE: Auto-shown right column (rightColAutoShown) does NOT produce the tab hint;
// only explicit toggle does (per implementation: checks m.rightColVisible).
// We skip the auto-show path and use a narrow terminal to avoid auto-show,
// then check that without explicit toggle there is no "tab" hint.
func TestBottomHints_Detail_RightColVisible_NoTabWhenAutoShow(t *testing.T) {
	resource.SetRelatedForTest("hints_test_right_col", []resource.RelatedDef{
		{
			TargetType:  "vpc",
			DisplayName: "VPC",
			Checker: func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
				return resource.RelatedCheckResult{}
			},
		},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest("hints_test_right_col") })

	res := resource.Resource{
		ID:   "test-id",
		Name: "test-resource",
	}
	// Use a narrow terminal (< 60) so right column does NOT auto-show.
	m := views.NewDetail(res, "hints_test_right_col", nil, keys.Default())
	m.SetSize(50, 24)

	hints := m.BottomHints()
	// "r" should be present (related defs registered) but "tab" should NOT
	// (right column not explicitly toggled on).
	if !hasHint(hints, "r") {
		t.Error("expected hint with key 'r' for Related")
	}
	if hasHint(hints, "tab") {
		t.Error("unexpected 'tab' hint when right column is not explicitly shown")
	}
}

// ---------------------------------------------------------------------------
// D. YAML tests
// ---------------------------------------------------------------------------

func TestBottomHints_YAML(t *testing.T) {
	res := resource.Resource{
		ID:   "yaml-test-id",
		Name: "yaml-test-resource",
	}
	// No resource type → no CloudTrailKey → t hint suppressed
	m := views.NewYAML(res, "", keys.Default())

	hints := m.BottomHints()
	want := []layout.KeyHint{
		{Key: "w", Desc: "Wrap"},
		{Key: "c", Desc: "Copy"},
	}
	if !hintsEqual(hints, want) {
		t.Errorf("YAML BottomHints = %v, want %v", hints, want)
	}
}

// ---------------------------------------------------------------------------
// E. CloudTrail "t" key hint tests (#247)
// ---------------------------------------------------------------------------

func TestBottomHints_ResourceList_ShowsCloudTrail(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("resource type 'ec2' not registered")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, nil, k)
	m.SetSize(80, 24)

	hints := m.BottomHints()
	found := false
	for _, h := range hints {
		if h.Key == "t" && h.Desc == "CloudTrail" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceList BottomHints missing {t, CloudTrail}; got %v", hints)
	}
}

func TestBottomHints_Detail_ShowsCloudTrail(t *testing.T) {
	res := resource.Resource{
		ID:   "i-test",
		Name: "test-resource",
		Fields: map[string]string{
			"arn": "arn:aws:ec2:us-east-1:000000000000:instance/i-test",
		},
	}
	m := views.NewDetail(res, "ec2", nil, keys.Default())
	m.SetSize(80, 24)

	hints := m.BottomHints()
	found := false
	for _, h := range hints {
		if h.Key == "t" && h.Desc == "CloudTrail" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Detail BottomHints missing {t, CloudTrail}; got %v", hints)
	}
}

func TestBottomHints_YAML_ShowsCloudTrail(t *testing.T) {
	res := resource.Resource{
		ID:   "yaml-test-id",
		Name: "yaml-test-resource",
		Fields: map[string]string{
			"arn": "arn:aws:ec2:us-east-1:000000000000:instance/i-test",
		},
	}
	m := views.NewYAML(res, "ec2", keys.Default())
	m.SetSize(80, 24)

	hints := m.BottomHints()
	found := false
	for _, h := range hints {
		if h.Key == "t" && h.Desc == "CloudTrail" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("YAML BottomHints missing {t, CloudTrail}; got %v", hints)
	}
}

// TestBottomHints_Detail_RightColFocused_ShowsCloudTrail verifies that the "t"
// (CloudTrail) hint is present in BottomHints() even when the right column is
// focused. Regression: right-col focus must not suppress the t hint.
func TestBottomHints_Detail_RightColFocused_ShowsCloudTrail(t *testing.T) {
	res := resource.Resource{
		ID:   "i-rhs-focus",
		Name: "rhs-focus-instance",
		Fields: map[string]string{
			"arn": "arn:aws:ec2:us-east-1:000000000000:instance/i-rhs-focus",
		},
	}
	m := views.NewDetail(res, "ec2", nil, keys.Default())
	m.SetSize(120, 40)

	if !strings.Contains(m.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=120; skipping right-col focus hint test")
	}

	// r → r → Tab: explicit-visible transition then focus right column.
	m = focusRightColumn(m)

	hints := m.BottomHints()
	found := false
	for _, h := range hints {
		if h.Key == "t" && h.Desc == "CloudTrail" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Detail BottomHints missing {t, CloudTrail} when right col focused; got %v", hints)
	}
}

// ---------------------------------------------------------------------------
// Helpers shared across hint tests
// ---------------------------------------------------------------------------

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

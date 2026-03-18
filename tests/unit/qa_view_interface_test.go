package unit

import (
	"testing"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/views"
)

// ── Compile-time interface satisfaction checks ──────────────────────────────

var (
	_ views.View = (*views.MainMenuModel)(nil)
	_ views.View = (*views.ResourceListModel)(nil)
	_ views.View = (*views.DetailModel)(nil)
	_ views.View = (*views.YAMLModel)(nil)
	_ views.View = (*views.RevealModel)(nil)
	_ views.View = (*views.ProfileModel)(nil)
	_ views.View = (*views.RegionModel)(nil)
	_ views.View = (*views.HelpModel)(nil)
)

var (
	_ views.Filterable = (*views.MainMenuModel)(nil)
	_ views.Filterable = (*views.ResourceListModel)(nil)
	_ views.Filterable = (*views.ProfileModel)(nil)
	_ views.Filterable = (*views.RegionModel)(nil)
)

// ── Test: each view satisfies View interface ────────────────────────────────

func TestViewInterface_MainMenuSatisfiesView(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	var v views.View = &m
	if v.FrameTitle() == "" {
		t.Error("MainMenuModel.FrameTitle() should return non-empty string")
	}
	_ = v.View()
}

func TestViewInterface_ResourceListSatisfiesView(t *testing.T) {
	k := keys.Default()
	rt := resource.ResourceTypeDef{Name: "Test", ShortName: "test"}
	m := views.NewResourceList(rt, nil, k)
	var v views.View = &m
	if v.FrameTitle() == "" {
		t.Error("ResourceListModel.FrameTitle() should return non-empty string")
	}
}

func TestViewInterface_DetailSatisfiesView(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{ID: "test-id", Name: "test-name"}
	m := views.NewDetail(res, "ec2", nil, k)
	var v views.View = &m
	if v.FrameTitle() == "" {
		t.Error("DetailModel.FrameTitle() should return non-empty string")
	}
}

func TestViewInterface_YAMLSatisfiesView(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{ID: "test-id", Name: "test-name"}
	m := views.NewYAML(res, k)
	var v views.View = &m
	if v.FrameTitle() == "" {
		t.Error("YAMLModel.FrameTitle() should return non-empty string")
	}
}

func TestViewInterface_RevealSatisfiesView(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("my-secret", "secret-value", k)
	var v views.View = &m
	if v.FrameTitle() == "" {
		t.Error("RevealModel.FrameTitle() should return non-empty string")
	}
}

func TestViewInterface_ProfileSatisfiesView(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"default", "staging"}, "default", k)
	var v views.View = &m
	if v.FrameTitle() == "" {
		t.Error("ProfileModel.FrameTitle() should return non-empty string")
	}
}

func TestViewInterface_RegionSatisfiesView(t *testing.T) {
	k := keys.Default()
	m := views.NewRegion([]string{"us-east-1", "eu-west-1"}, "us-east-1", k)
	var v views.View = &m
	if v.FrameTitle() == "" {
		t.Error("RegionModel.FrameTitle() should return non-empty string")
	}
}

func TestViewInterface_HelpSatisfiesView(t *testing.T) {
	k := keys.Default()
	m := views.NewHelp(k, views.HelpFromMainMenu)
	var v views.View = &m
	if v.FrameTitle() != "help" {
		t.Errorf("HelpModel.FrameTitle() should return 'help', got %q", v.FrameTitle())
	}
}

// ── Test: Filterable views ──────────────────────────────────────────────────

func TestFilterable_MainMenuSetFilter(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	var f views.Filterable = &m
	f.SetFilter("ec2")
}

func TestFilterable_ResourceListSetFilter(t *testing.T) {
	k := keys.Default()
	rt := resource.ResourceTypeDef{Name: "Test", ShortName: "test"}
	m := views.NewResourceList(rt, nil, k)
	var f views.Filterable = &m
	f.SetFilter("something")
}

func TestFilterable_ProfileSetFilter(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"default"}, "default", k)
	var f views.Filterable = &m
	f.SetFilter("def")
}

func TestFilterable_RegionSetFilter(t *testing.T) {
	k := keys.Default()
	m := views.NewRegion([]string{"us-east-1"}, "us-east-1", k)
	var f views.Filterable = &m
	f.SetFilter("us")
}

// ── Test: Non-filterable views do NOT satisfy Filterable ─────────────────────

func TestNonFilterable_DetailDoesNotSatisfyFilterable(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{ID: "test-id"}
	m := views.NewDetail(res, "ec2", nil, k)
	var v interface{} = &m
	if _, ok := v.(views.Filterable); ok {
		t.Error("DetailModel should NOT satisfy Filterable interface")
	}
}

func TestNonFilterable_YAMLDoesNotSatisfyFilterable(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{ID: "test-id"}
	m := views.NewYAML(res, k)
	var v interface{} = &m
	if _, ok := v.(views.Filterable); ok {
		t.Error("YAMLModel should NOT satisfy Filterable interface")
	}
}

func TestNonFilterable_HelpDoesNotSatisfyFilterable(t *testing.T) {
	k := keys.Default()
	m := views.NewHelp(k, views.HelpFromMainMenu)
	var v interface{} = &m
	if _, ok := v.(views.Filterable); ok {
		t.Error("HelpModel should NOT satisfy Filterable interface")
	}
}

func TestNonFilterable_RevealDoesNotSatisfyFilterable(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("s", "v", k)
	var v interface{} = &m
	if _, ok := v.(views.Filterable); ok {
		t.Error("RevealModel should NOT satisfy Filterable interface")
	}
}

// ── Test: SetSize works through the View interface ──────────────────────────

func TestViewInterface_SetSizePropagates(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	var v views.View = &m
	v.SetSize(80, 24)
	output := v.View()
	if output == "" {
		t.Error("MainMenuModel.View() should return non-empty after SetSize")
	}
}

// ── Test: stack operations with View interface ──────────────────────────────

func TestViewStack_PushAndPop(t *testing.T) {
	k := keys.Default()

	stack := make([]views.View, 0)

	menu := views.NewMainMenu(k)
	stack = append(stack, &menu)

	rt := resource.ResourceTypeDef{Name: "EC2", ShortName: "ec2"}
	rl := views.NewResourceList(rt, nil, k)
	stack = append(stack, &rl)

	res := resource.Resource{ID: "i-123", Name: "my-instance"}
	detail := views.NewDetail(res, "ec2", nil, k)
	stack = append(stack, &detail)

	if len(stack) != 3 {
		t.Fatalf("expected stack length 3, got %d", len(stack))
	}

	top := stack[len(stack)-1]
	if top.FrameTitle() != "my-instance" {
		t.Errorf("top of stack should be detail with title 'my-instance', got %q", top.FrameTitle())
	}

	stack = stack[:len(stack)-1]
	if len(stack) != 2 {
		t.Fatalf("after pop, expected stack length 2, got %d", len(stack))
	}

	top = stack[len(stack)-1]
	if top.FrameTitle() != "ec2" {
		t.Errorf("after pop, top should be resource list with title 'ec2', got %q", top.FrameTitle())
	}

	stack = stack[:len(stack)-1]
	top = stack[len(stack)-1]
	title := top.FrameTitle()
	if title == "" {
		t.Error("main menu FrameTitle should not be empty")
	}
}

func TestViewStack_SetSizeAllViews(t *testing.T) {
	k := keys.Default()

	stack := make([]views.View, 0)
	menu := views.NewMainMenu(k)
	stack = append(stack, &menu)

	help := views.NewHelp(k, views.HelpFromMainMenu)
	stack = append(stack, &help)

	for _, v := range stack {
		v.SetSize(100, 30)
	}

	for i, v := range stack {
		_ = v.View()
		title := v.FrameTitle()
		if title == "" {
			t.Errorf("view at index %d has empty FrameTitle after SetSize", i)
		}
	}
}

func TestViewStack_FilterableFromStack(t *testing.T) {
	k := keys.Default()

	stack := make([]views.View, 0)
	menu := views.NewMainMenu(k)
	stack = append(stack, &menu)

	top := stack[len(stack)-1]
	if f, ok := top.(views.Filterable); ok {
		f.SetFilter("ec2")
	} else {
		t.Error("MainMenuModel should satisfy Filterable when accessed as View")
	}

	res := resource.Resource{ID: "test"}
	detail := views.NewDetail(res, "ec2", nil, k)
	stack = append(stack, &detail)

	top = stack[len(stack)-1]
	if _, ok := top.(views.Filterable); ok {
		t.Error("DetailModel should NOT satisfy Filterable")
	}
}

// ── Test: app.go stack uses View interface (integration) ────────────────────

func TestRootModel_StackUsesViewInterface(t *testing.T) {
	m := newRootSizedModel()

	content := rootViewContent(m)
	if content == "" {
		t.Error("initial view should render non-empty content")
	}

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("help view should render non-empty content via View interface")
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(0x1b))
	plain = stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("after pop, main menu should render via View interface")
	}
}

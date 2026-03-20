package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/views"
)

// ── MainMenuModel.View() ────────────────────────────────────────────────────

func TestContentMainMenu_ViewNonEmpty(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	m.SetSize(80, 20)
	out := m.View()
	if out == "" {
		t.Error("MainMenuModel.View() returned empty string")
	}
}

func TestContentMainMenu_ViewContainsAllResourceNames(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	m.SetSize(80, 80)
	out := m.View()
	for _, rt := range resource.AllResourceTypes() {
		if !strings.Contains(out, rt.Name) {
			t.Errorf("MainMenu.View() missing resource name %q", rt.Name)
		}
	}
}

func TestContentMainMenu_ViewContainsShortNames(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	m.SetSize(120, 80)
	out := m.View()
	for _, rt := range resource.AllResourceTypes() {
		alias := ":" + rt.ShortName
		// The alias column is 13 chars wide; aliases longer than that get truncated.
		// Check for the prefix that fits within the column.
		if len(alias) > 13 {
			alias = alias[:12] // check prefix that fits
		}
		if !strings.Contains(out, alias) {
			t.Errorf("MainMenu.View() missing short name prefix %q for type %q", alias, rt.ShortName)
		}
	}
}

func TestContentMainMenu_ViewHasCorrectLineCount(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	m.SetSize(80, 80)
	out := m.View()
	lines := strings.Split(out, "\n")
	expectedTypes := len(resource.AllResourceTypes())
	if len(lines) < expectedTypes {
		t.Errorf("MainMenu.View() expected at least %d lines, got %d", expectedTypes, len(lines))
	}
}

// ── DetailModel.renderContent() via SetSize → View() ────────────────────────

func TestContentDetail_ViewWithFields(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "i-abc123",
		Name:   "test-instance",
		Status: "running",
		Fields: map[string]string{
			"InstanceId":   "i-abc123",
			"InstanceType": "t3.medium",
			"State":        "running",
		},
	}
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(80, 20)
	out := m.View()
	if out == "" || out == "Initializing..." {
		t.Error("DetailModel.View() returned empty or initializing")
	}
}

func TestContentDetail_ViewContainsFieldKeys(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "i-abc123",
		Name:   "test-instance",
		Status: "running",
		Fields: map[string]string{
			"InstanceId":   "i-abc123",
			"InstanceType": "t3.medium",
		},
	}
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(80, 20)
	out := m.View()
	if !strings.Contains(out, "InstanceId") {
		t.Error("Detail.View() missing key 'InstanceId'")
	}
	if !strings.Contains(out, "InstanceType") {
		t.Error("Detail.View() missing key 'InstanceType'")
	}
}

func TestContentDetail_ViewContainsFieldValues(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "i-abc123",
		Name:   "test-instance",
		Status: "running",
		Fields: map[string]string{
			"InstanceId":   "i-abc123",
			"InstanceType": "t3.medium",
		},
	}
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(80, 20)
	out := m.View()
	if !strings.Contains(out, "i-abc123") {
		t.Error("Detail.View() missing value 'i-abc123'")
	}
	if !strings.Contains(out, "t3.medium") {
		t.Error("Detail.View() missing value 't3.medium'")
	}
}

func TestContentDetail_ViewWithRawStructAndConfig(t *testing.T) {
	k := keys.Default()
	type fakeEC2 struct {
		InstanceId   *string
		InstanceType string
	}
	instID := "i-struct123"
	rawStruct := fakeEC2{
		InstanceId:   &instID,
		InstanceType: "t3.large",
	}
	viewCfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []string{"InstanceId", "InstanceType"},
			},
		},
	}
	res := resource.Resource{
		ID:        "i-struct123",
		Name:      "struct-instance",
		Status:    "running",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}
	m := views.NewDetail(res, "ec2", viewCfg, k)
	m.SetSize(80, 20)
	out := m.View()
	if !strings.Contains(out, "i-struct123") {
		t.Errorf("Detail.View() with RawStruct missing value 'i-struct123', got: %s", out)
	}
	if !strings.Contains(out, "t3.large") {
		t.Errorf("Detail.View() with RawStruct missing value 't3.large', got: %s", out)
	}
}

// ── YAMLModel.renderContent() via SetSize → View() ─────────────────────────

func TestContentYAML_ViewWithFields(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "i-abc123",
		Name:   "test-instance",
		Status: "running",
		Fields: map[string]string{
			"InstanceId":   "i-abc123",
			"InstanceType": "t3.medium",
		},
	}
	m := views.NewYAML(res, k)
	m.SetSize(80, 20)
	out := m.View()
	if out == "" || out == "Initializing..." {
		t.Error("YAMLModel.View() returned empty or initializing")
	}
}

func TestContentYAML_ViewContainsYAMLKeys(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "i-abc123",
		Name:   "test-instance",
		Status: "running",
		Fields: map[string]string{
			"InstanceId":   "i-abc123",
			"InstanceType": "t3.medium",
		},
	}
	m := views.NewYAML(res, k)
	m.SetSize(80, 20)
	out := m.View()
	if !strings.Contains(out, "InstanceId") {
		t.Error("YAML.View() missing key 'InstanceId'")
	}
	if !strings.Contains(out, "InstanceType") {
		t.Error("YAML.View() missing key 'InstanceType'")
	}
}

func TestContentYAML_ViewContainsYAMLValues(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "i-abc123",
		Name:   "test-instance",
		Status: "running",
		Fields: map[string]string{
			"InstanceId":   "i-abc123",
			"InstanceType": "t3.medium",
		},
	}
	m := views.NewYAML(res, k)
	m.SetSize(80, 20)
	out := m.View()
	if !strings.Contains(out, "i-abc123") {
		t.Error("YAML.View() missing value 'i-abc123'")
	}
	if !strings.Contains(out, "t3.medium") {
		t.Error("YAML.View() missing value 't3.medium'")
	}
}

func TestContentYAML_ViewWithRawStruct(t *testing.T) {
	k := keys.Default()
	type fakeEC2 struct {
		InstanceId   string
		InstanceType string
	}
	rawStruct := fakeEC2{
		InstanceId:   "i-struct456",
		InstanceType: "t3.large",
	}
	res := resource.Resource{
		ID:        "i-struct456",
		Name:      "struct-instance",
		Status:    "running",
		RawStruct: &rawStruct,
		Fields:    map[string]string{},
	}
	m := views.NewYAML(res, k)
	m.SetSize(80, 20)
	out := m.View()
	if !strings.Contains(out, "i-struct456") {
		t.Errorf("YAML.View() with RawStruct missing 'i-struct456', got: %s", out)
	}
	if !strings.Contains(out, "t3.large") {
		t.Errorf("YAML.View() with RawStruct missing 't3.large', got: %s", out)
	}
}

// ── HelpModel.View() ───────────────────────────────────────────────────────

func TestContentHelp_ViewNonEmpty(t *testing.T) {
	k := keys.Default()
	m := views.NewHelp(k, views.HelpFromResourceList)
	m.SetSize(84, 20)
	out := m.View()
	if out == "" {
		t.Error("HelpModel.View() returned empty string")
	}
}

func TestContentHelp_ViewContainsCategories(t *testing.T) {
	k := keys.Default()
	m := views.NewHelp(k, views.HelpFromResourceList)
	m.SetSize(84, 20)
	out := m.View()
	categories := []string{"NAVIGATION", "ACTIONS", "SORT", "OTHER"}
	for _, cat := range categories {
		if !strings.Contains(out, cat) {
			t.Errorf("Help.View() missing category %q", cat)
		}
	}
}

func TestContentHelp_ViewContainsKeyBindings(t *testing.T) {
	k := keys.Default()
	m := views.NewHelp(k, views.HelpFromResourceList)
	m.SetSize(84, 20)
	out := m.View()
	bindings := []string{"esc", "j", "k", "?"}
	for _, b := range bindings {
		if !strings.Contains(out, b) {
			t.Errorf("Help.View() missing key binding %q", b)
		}
	}
}

func TestContentHelp_ViewContainsCloseHint(t *testing.T) {
	k := keys.Default()
	m := views.NewHelp(k, views.HelpFromMainMenu)
	m.SetSize(84, 20)
	out := m.View()
	if !strings.Contains(out, "Press any key to close") {
		t.Error("Help.View() missing 'Press any key to close' hint")
	}
}

// ── ProfileModel.View() ────────────────────────────────────────────────────

func TestContentProfile_ViewNonEmpty(t *testing.T) {
	k := keys.Default()
	profiles := []string{"default", "prod", "staging"}
	m := views.NewProfile(profiles, "default", k)
	m.SetSize(60, 20)
	out := m.View()
	if out == "" {
		t.Error("ProfileModel.View() returned empty string")
	}
}

func TestContentProfile_ViewContainsAllProfiles(t *testing.T) {
	k := keys.Default()
	profiles := []string{"default", "prod", "staging"}
	m := views.NewProfile(profiles, "default", k)
	m.SetSize(60, 20)
	out := m.View()
	for _, p := range profiles {
		if !strings.Contains(out, p) {
			t.Errorf("Profile.View() missing profile %q", p)
		}
	}
}

func TestContentProfile_ViewShowsCurrentAnnotation(t *testing.T) {
	k := keys.Default()
	profiles := []string{"default", "prod", "staging"}
	m := views.NewProfile(profiles, "prod", k)
	m.SetSize(60, 20)
	out := m.View()
	if !strings.Contains(out, "(current)") {
		t.Error("Profile.View() missing '(current)' annotation for active profile")
	}
}

func TestContentProfile_ViewCorrectLineCount(t *testing.T) {
	k := keys.Default()
	profiles := []string{"default", "prod", "staging"}
	m := views.NewProfile(profiles, "default", k)
	m.SetSize(60, 20)
	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) < len(profiles) {
		t.Errorf("Profile.View() expected at least %d lines, got %d", len(profiles), len(lines))
	}
}

// ── RegionModel.View() ─────────────────────────────────────────────────────

func TestContentRegion_ViewNonEmpty(t *testing.T) {
	k := keys.Default()
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	m := views.NewRegion(regions, "us-east-1", k)
	m.SetSize(60, 20)
	out := m.View()
	if out == "" {
		t.Error("RegionModel.View() returned empty string")
	}
}

func TestContentRegion_ViewContainsAllRegions(t *testing.T) {
	k := keys.Default()
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	m := views.NewRegion(regions, "us-east-1", k)
	m.SetSize(60, 20)
	out := m.View()
	for _, r := range regions {
		if !strings.Contains(out, r) {
			t.Errorf("Region.View() missing region %q", r)
		}
	}
}

func TestContentRegion_ViewShowsCurrentAnnotation(t *testing.T) {
	k := keys.Default()
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	m := views.NewRegion(regions, "us-west-2", k)
	m.SetSize(60, 20)
	out := m.View()
	if !strings.Contains(out, "(current)") {
		t.Error("Region.View() missing '(current)' annotation for active region")
	}
}

func TestContentRegion_ViewCorrectLineCount(t *testing.T) {
	k := keys.Default()
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	m := views.NewRegion(regions, "us-east-1", k)
	m.SetSize(60, 20)
	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) < len(regions) {
		t.Errorf("Region.View() expected at least %d lines, got %d", len(regions), len(lines))
	}
}

// ── RevealModel ─────────────────────────────────────────────────────────────

func TestContentReveal_SetSizePopulatesViewport(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("my-secret", "super-secret-value-123", k)
	m.SetSize(80, 20)
	out := m.View()
	if !strings.Contains(out, "super-secret-value-123") {
		t.Errorf("Reveal.View() missing secret value, got: %s", out)
	}
}

func TestContentReveal_HeaderWarningNonEmpty(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("my-secret", "value", k)
	warning := m.HeaderWarning()
	if warning == "" {
		t.Error("Reveal.HeaderWarning() returned empty string")
	}
	if !strings.Contains(warning, "esc") {
		t.Error("Reveal.HeaderWarning() missing 'esc' reference")
	}
}

func TestContentReveal_FrameTitle(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("my-secret", "value", k)
	title := m.FrameTitle()
	if title != "my-secret" {
		t.Errorf("Reveal.FrameTitle() = %q, want %q", title, "my-secret")
	}
}

// ── Cross-cutting: all 10 resource types in MainMenu ────────────────────────

func TestContentMainMenu_AllSevenResourceTypes(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	// Use a tall viewport so all 62 resource types + category headers fit
	m.SetSize(80, 100)
	out := m.View()

	allTypes := resource.AllResourceTypes()
	for _, rt := range allTypes {
		if !strings.Contains(out, rt.Name) {
			t.Errorf("MainMenu.View() missing resource type %q", rt.Name)
		}
	}
}

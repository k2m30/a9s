package unit

import (
	"testing"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/views"
)

// makeResourcesLoadedMsg creates a ResourcesLoadedMsg for testing view updates.
func makeResourcesLoadedMsg(resourceType string, resources []resource.Resource) messages.ResourcesLoadedMsg {
	return messages.ResourcesLoadedMsg{
		ResourceType: resourceType,
		Resources:    resources,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// CopyContent tests — each view returns appropriate content and label
// ═══════════════════════════════════════════════════════════════════════════

func TestCopyContent_MainMenu_ReturnsEmpty(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	var v views.View = &m
	content, label := v.CopyContent()
	if content != "" {
		t.Errorf("MainMenu.CopyContent() should return empty content, got %q", content)
	}
	if label != "" {
		t.Errorf("MainMenu.CopyContent() should return empty label, got %q", label)
	}
}

func TestCopyContent_ResourceList_ReturnsSelectedID(t *testing.T) {
	k := keys.Default()
	rt := resource.ResourceTypeDef{Name: "EC2 Instances", ShortName: "ec2"}
	m := views.NewResourceList(rt, nil, k)
	m.SetSize(120, 40)
	// Simulate loading resources
	m, _ = m.Update(makeResourcesLoadedMsg("ec2", []resource.Resource{
		{ID: "i-0abc123", Name: "web-server", Status: "running", Fields: map[string]string{"instance_id": "i-0abc123"}},
	}))
	var v views.View = &m
	content, label := v.CopyContent()
	if content != "i-0abc123" {
		t.Errorf("ResourceList.CopyContent() should return selected resource ID, got %q", content)
	}
	if label == "" {
		t.Error("ResourceList.CopyContent() should return non-empty label")
	}
}

func TestCopyContent_ResourceList_EmptyWhenNoResources(t *testing.T) {
	k := keys.Default()
	rt := resource.ResourceTypeDef{Name: "EC2 Instances", ShortName: "ec2"}
	m := views.NewResourceList(rt, nil, k)
	var v views.View = &m
	content, label := v.CopyContent()
	if content != "" {
		t.Errorf("ResourceList.CopyContent() with no resources should return empty content, got %q", content)
	}
	if label != "" {
		t.Errorf("ResourceList.CopyContent() with no resources should return empty label, got %q", label)
	}
}

func TestCopyContent_Detail_ReturnsYAML(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:   "i-detail123",
		Name: "detail-server",
		Fields: map[string]string{
			"instance_id": "i-detail123",
			"state":       "running",
		},
	}
	m := views.NewDetail(res, "ec2", nil, k)
	m.SetSize(120, 40)
	var v views.View = &m
	content, label := v.CopyContent()
	if content == "" {
		t.Error("Detail.CopyContent() should return non-empty YAML content")
	}
	if label == "" {
		t.Error("Detail.CopyContent() should return non-empty label")
	}
}

func TestCopyContent_YAML_ReturnsRawContent(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:   "i-yaml123",
		Name: "yaml-server",
		Fields: map[string]string{
			"instance_id": "i-yaml123",
			"state":       "running",
		},
	}
	m := views.NewYAML(res, k)
	m.SetSize(120, 40)
	var v views.View = &m
	content, label := v.CopyContent()
	if content == "" {
		t.Error("YAML.CopyContent() should return non-empty content")
	}
	if label == "" {
		t.Error("YAML.CopyContent() should return non-empty label")
	}
}

func TestCopyContent_Reveal_ReturnsSecretValue(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("my-secret", "super-secret-value", k)
	var v views.View = &m
	content, label := v.CopyContent()
	if content != "super-secret-value" {
		t.Errorf("Reveal.CopyContent() should return secret value, got %q", content)
	}
	if label == "" {
		t.Error("Reveal.CopyContent() should return non-empty label")
	}
}

func TestCopyContent_Profile_ReturnsEmpty(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"default", "staging"}, "default", k)
	var v views.View = &m
	content, label := v.CopyContent()
	if content != "" {
		t.Errorf("Profile.CopyContent() should return empty content, got %q", content)
	}
	if label != "" {
		t.Errorf("Profile.CopyContent() should return empty label, got %q", label)
	}
}

func TestCopyContent_Region_ReturnsEmpty(t *testing.T) {
	k := keys.Default()
	m := views.NewRegion([]string{"us-east-1", "eu-west-1"}, "us-east-1", k)
	var v views.View = &m
	content, label := v.CopyContent()
	if content != "" {
		t.Errorf("Region.CopyContent() should return empty content, got %q", content)
	}
	if label != "" {
		t.Errorf("Region.CopyContent() should return empty label, got %q", label)
	}
}

func TestCopyContent_Help_ReturnsEmpty(t *testing.T) {
	k := keys.Default()
	m := views.NewHelp(k, views.HelpFromMainMenu)
	var v views.View = &m
	content, label := v.CopyContent()
	if content != "" {
		t.Errorf("Help.CopyContent() should return empty content, got %q", content)
	}
	if label != "" {
		t.Errorf("Help.CopyContent() should return empty label, got %q", label)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// GetHelpContext tests — each view returns the correct HelpContext
// ═══════════════════════════════════════════════════════════════════════════

func TestGetHelpContext_MainMenu(t *testing.T) {
	k := keys.Default()
	m := views.NewMainMenu(k)
	var v views.View = &m
	ctx := v.GetHelpContext()
	if ctx != views.HelpFromMainMenu {
		t.Errorf("MainMenu.GetHelpContext() should return HelpFromMainMenu, got %v", ctx)
	}
}

func TestGetHelpContext_ResourceList_NonSecrets(t *testing.T) {
	k := keys.Default()
	rt := resource.ResourceTypeDef{Name: "EC2 Instances", ShortName: "ec2"}
	m := views.NewResourceList(rt, nil, k)
	var v views.View = &m
	ctx := v.GetHelpContext()
	if ctx != views.HelpFromResourceList {
		t.Errorf("ResourceList(ec2).GetHelpContext() should return HelpFromResourceList, got %v", ctx)
	}
}

func TestGetHelpContext_ResourceList_Secrets(t *testing.T) {
	k := keys.Default()
	rt := resource.ResourceTypeDef{Name: "Secrets Manager", ShortName: "secrets"}
	m := views.NewResourceList(rt, nil, k)
	var v views.View = &m
	ctx := v.GetHelpContext()
	if ctx != views.HelpFromSecretsList {
		t.Errorf("ResourceList(secrets).GetHelpContext() should return HelpFromSecretsList, got %v", ctx)
	}
}

func TestGetHelpContext_Detail(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{ID: "test-id", Name: "test-name"}
	m := views.NewDetail(res, "ec2", nil, k)
	var v views.View = &m
	ctx := v.GetHelpContext()
	if ctx != views.HelpFromDetail {
		t.Errorf("Detail.GetHelpContext() should return HelpFromDetail, got %v", ctx)
	}
}

func TestGetHelpContext_YAML(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{ID: "test-id", Name: "test-name"}
	m := views.NewYAML(res, k)
	var v views.View = &m
	ctx := v.GetHelpContext()
	if ctx != views.HelpFromYAML {
		t.Errorf("YAML.GetHelpContext() should return HelpFromYAML, got %v", ctx)
	}
}

func TestGetHelpContext_Profile(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{"default"}, "default", k)
	var v views.View = &m
	ctx := v.GetHelpContext()
	if ctx != views.HelpFromSelector {
		t.Errorf("Profile.GetHelpContext() should return HelpFromSelector, got %v", ctx)
	}
}

func TestGetHelpContext_Region(t *testing.T) {
	k := keys.Default()
	m := views.NewRegion([]string{"us-east-1"}, "us-east-1", k)
	var v views.View = &m
	ctx := v.GetHelpContext()
	if ctx != views.HelpFromSelector {
		t.Errorf("Region.GetHelpContext() should return HelpFromSelector, got %v", ctx)
	}
}

func TestGetHelpContext_Reveal(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("my-secret", "value", k)
	var v views.View = &m
	ctx := v.GetHelpContext()
	if ctx != views.HelpFromReveal {
		t.Errorf("Reveal.GetHelpContext() should return HelpFromReveal, got %v", ctx)
	}
}

func TestGetHelpContext_Help(t *testing.T) {
	k := keys.Default()
	m := views.NewHelp(k, views.HelpFromMainMenu)
	var v views.View = &m
	ctx := v.GetHelpContext()
	if ctx != views.HelpFromMainMenu {
		t.Errorf("Help.GetHelpContext() should return HelpFromMainMenu, got %v", ctx)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// All 10 resource types: CopyContent returns correct ID on resource list
// ═══════════════════════════════════════════════════════════════════════════

func TestCopyContent_AllResourceTypes(t *testing.T) {
	types := []struct {
		name string
		id   string
	}{
		{"s3", "my-bucket"},
		{"ec2", "i-0abc"},
		{"dbi", "mydb"},
		{"redis", "redis-001"},
		{"dbc", "docdb-cluster"},
		{"eks", "my-cluster"},
		{"secrets", "prod/api-key"},
		{"vpc", "vpc-0001"},
		{"sg", "sg-0001"},
		{"ng", "ng-web"},
	}
	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			k := keys.Default()
			rt := resource.ResourceTypeDef{Name: tt.name, ShortName: tt.name}
			m := views.NewResourceList(rt, nil, k)
			m.SetSize(120, 40)
			m, _ = m.Update(makeResourcesLoadedMsg(tt.name, []resource.Resource{
				{ID: tt.id, Name: tt.id, Fields: map[string]string{"name": tt.id}},
			}))
			var v views.View = &m
			content, label := v.CopyContent()
			if content != tt.id {
				t.Errorf("CopyContent() for %s should return %q, got %q", tt.name, tt.id, content)
			}
			if label == "" {
				t.Errorf("CopyContent() for %s should return non-empty label", tt.name)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// All 10 resource types: GetHelpContext returns correct context
// ═══════════════════════════════════════════════════════════════════════════

func TestGetHelpContext_AllResourceTypes(t *testing.T) {
	types := []struct {
		name     string
		expected views.HelpContext
	}{
		{"s3", views.HelpFromResourceList},
		{"ec2", views.HelpFromResourceList},
		{"dbi", views.HelpFromResourceList},
		{"redis", views.HelpFromResourceList},
		{"dbc", views.HelpFromResourceList},
		{"eks", views.HelpFromResourceList},
		{"secrets", views.HelpFromSecretsList},
		{"vpc", views.HelpFromResourceList},
		{"sg", views.HelpFromResourceList},
		{"ng", views.HelpFromResourceList},
	}
	for _, tt := range types {
		t.Run(tt.name, func(t *testing.T) {
			k := keys.Default()
			rt := resource.ResourceTypeDef{Name: tt.name, ShortName: tt.name}
			m := views.NewResourceList(rt, nil, k)
			var v views.View = &m
			ctx := v.GetHelpContext()
			if ctx != tt.expected {
				t.Errorf("GetHelpContext() for %s should return %v, got %v", tt.name, tt.expected, ctx)
			}
		})
	}
}

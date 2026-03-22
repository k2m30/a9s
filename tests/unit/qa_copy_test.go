package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// --- Resource list: c copies resource ID ---

func TestQA_Copy_ResourceList_CopiesID(t *testing.T) {
	tui.Version = "test"
	m := tui.New("test", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: []resource.Resource{
		{ID: "i-0abc123", Name: "web-server", Status: "running", Fields: map[string]string{"instance_id": "i-0abc123"}},
	}})

	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'c'})
	if cmd == nil {
		t.Fatal("c on resource list should return a copy command")
	}
	// Execute the cmd to get the FlashMsg
	msg := cmd()
	flash, ok := msg.(messages.FlashMsg)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	if !strings.Contains(flash.Text, "i-0abc123") {
		t.Errorf("flash should mention copied ID, got: %s", flash.Text)
	}
}

// --- Detail view: c copies full detail content, not just ID ---

func TestQA_Copy_Detail_CopiesYAML(t *testing.T) {
	tui.Version = "test"
	m := tui.New("test", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})
	res := resource.Resource{
		ID: "i-detail123", Name: "detail-server", Status: "running",
		Fields: map[string]string{
			"instance_id": "i-detail123", "name": "detail-server",
			"state": "running", "type": "t3.micro", "private_ip": "10.0.1.5",
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: []resource.Resource{res}})

	// Navigate to detail
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'd'})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Press c on detail view — should copy YAML, same as YAML view
	_, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: 'c'})
	if cmd == nil {
		t.Fatal("c on detail view should return a copy command")
	}
	msg := cmd()
	flash, ok := msg.(messages.FlashMsg)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	// Should NOT be just the resource ID
	if flash.Text == "Copied: i-detail123" {
		t.Error("detail copy should copy YAML content, not just resource ID")
	}
	// Flash should indicate YAML was copied
	if !strings.Contains(flash.Text, "YAML") && !strings.Contains(flash.Text, "yaml") && !strings.Contains(flash.Text, "detail") {
		t.Errorf("flash should mention YAML or detail, got: %s", flash.Text)
	}
}

// --- YAML view: c copies full YAML ---

func TestQA_Copy_YAML_CopiesFullYAML(t *testing.T) {
	tui.Version = "test"
	m := tui.New("test", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})
	res := resource.Resource{
		ID: "i-yaml123", Name: "yaml-server", Status: "running",
		Fields: map[string]string{"instance_id": "i-yaml123", "state": "running"},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: []resource.Resource{res}})

	// Navigate to YAML via y key
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'y'})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	_, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: 'c'})
	if cmd == nil {
		t.Fatal("c on YAML view should return a copy command")
	}
	msg := cmd()
	flash, ok := msg.(messages.FlashMsg)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	if !strings.Contains(flash.Text, "YAML") {
		t.Errorf("YAML copy flash should mention YAML, got: %s", flash.Text)
	}
}

// --- Main menu: c is no-op ---

func TestQA_Copy_MainMenu_NoOp(t *testing.T) {
	m := newRootSizedModel()
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'c'})
	if cmd != nil {
		t.Error("c on main menu should be no-op (nil cmd)")
	}
}

// --- All 10 resource types: c works on resource list ---

func TestQA_Copy_AllResourceTypes(t *testing.T) {
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
			tui.Version = "test"
			m := tui.New("test", "us-east-1")
			m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
			m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: tt.name})
			m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
				ResourceType: tt.name,
				Resources:    []resource.Resource{{ID: tt.id, Name: tt.id, Fields: map[string]string{"name": tt.id}}},
			})
			_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'c'})
			if cmd == nil {
				t.Errorf("c on %s list should return a command", tt.name)
				return
			}
			msg := cmd()
			if flash, ok := msg.(messages.FlashMsg); ok {
				if !strings.Contains(flash.Text, tt.id) {
					t.Errorf("flash for %s should contain %q, got: %s", tt.name, tt.id, flash.Text)
				}
			}
		})
	}
}

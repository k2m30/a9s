package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ===========================================================================
// Log Streams YAML view fixtures
// ===========================================================================

func fixtureLogStreams() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "2024/03/22/[$LATEST]abcdef1234567890",
			Name:   "2024/03/22/[$LATEST]abcdef1234567890",
			Status: "",
			Fields: map[string]string{
				"stream_name": "2024/03/22/[$LATEST]abcdef1234567890",
				"last_event":  "2024-03-23 00:00",
				"first_event": "2024-03-22 00:00",
			},
		},
	}
}

// ===========================================================================
// Log Events YAML view fixtures
// ===========================================================================

func fixtureLogEvents() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "evt-1711065600000-0",
			Name:   "ERROR NullPointerException in com.example.App.main",
			Status: "ERROR",
			Fields: map[string]string{
				"timestamp":      "2024-03-22 00:00",
				"message":        "ERROR NullPointerException in com.example.App.main",
				"ingestion_time": "2024-03-22 00:00",
			},
		},
	}
}

// ===========================================================================
// Log Streams YAML tests
// ===========================================================================

func TestQA_YAML_LogStreams_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureLogStreams() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("LogStreams YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("LogStreams YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_LogStreams_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureLogStreams()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("LogStreams FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_LogStreams_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureLogStreams()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("LogStreams RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// Log Events YAML tests
// ===========================================================================

func TestQA_YAML_LogEvents_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureLogEvents() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("LogEvents YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("LogEvents YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_LogEvents_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureLogEvents()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("LogEvents FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_LogEvents_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureLogEvents()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("LogEvents RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// Target Health YAML view fixtures
// ===========================================================================

func fixtureTargetHealth() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "i-0abc1234def56789a",
			Name:   "i-0abc1234def56789a",
			Status: "unhealthy",
			Fields: map[string]string{
				"target_id":   "i-0abc1234def56789a",
				"port":        "8080",
				"az":          "us-east-1a",
				"health":      "unhealthy",
				"reason":      "Target.FailedHealthChecks",
				"description": "Health checks failed with 503",
			},
		},
	}
}

// ===========================================================================
// Target Health YAML tests
// ===========================================================================

func TestQA_YAML_TargetHealth_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureTargetHealth() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("TargetHealth YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("TargetHealth YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_TargetHealth_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureTargetHealth()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("TargetHealth FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_TargetHealth_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureTargetHealth()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("TargetHealth RawContent() contains ANSI codes, expected plain YAML")
	}
}

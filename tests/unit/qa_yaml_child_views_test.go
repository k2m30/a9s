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

// ===========================================================================
// Lambda Invocations YAML view fixtures
// ===========================================================================

func fixtureLambdaInvocations() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "12345678-1234-1234-1234-123456789012",
			Name:   "12345678-1234-1234-1234-123456789012",
			Status: "OK",
			Fields: map[string]string{
				"request_id":  "12345678-1234-1234-1234-123456789012",
				"timestamp":   "2024-03-22 00:00",
				"status":      "OK",
				"duration_ms": "2103 ms",
				"memory_used": "128/256 MB",
				"cold_start":  "no",
			},
		},
	}
}

// ===========================================================================
// Lambda Invocations YAML tests
// ===========================================================================

func TestLambdaInvocationsYAMLViewContains(t *testing.T) {
	for _, r := range fixtureLambdaInvocations() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("LambdaInvocations YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("LambdaInvocations YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestLambdaInvocationsYAMLFrameTitle(t *testing.T) {
	m := yamlModel(fixtureLambdaInvocations()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("LambdaInvocations FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestLambdaInvocationsYAMLNoANSI(t *testing.T) {
	m := yamlModel(fixtureLambdaInvocations()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("LambdaInvocations RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// Lambda Invocation Logs YAML view fixtures
// ===========================================================================

func fixtureLambdaInvocationLogs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "log-001",
			Name:   "INFO Processing request for user abc-123",
			Status: "",
			Fields: map[string]string{
				"timestamp": "2024-03-22 00:00",
				"message":   "INFO Processing request for user abc-123",
			},
		},
	}
}

// ===========================================================================
// Lambda Invocation Logs YAML tests
// ===========================================================================

func TestLambdaInvocationLogsYAMLViewContains(t *testing.T) {
	for _, r := range fixtureLambdaInvocationLogs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("LambdaInvocationLogs YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("LambdaInvocationLogs YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestLambdaInvocationLogsYAMLFrameTitle(t *testing.T) {
	m := yamlModel(fixtureLambdaInvocationLogs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("LambdaInvocationLogs FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestLambdaInvocationLogsYAMLNoANSI(t *testing.T) {
	m := yamlModel(fixtureLambdaInvocationLogs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("LambdaInvocationLogs RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// ECS Service Events YAML view fixtures
// ===========================================================================

func fixtureEcsSvcEvents() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "evt-yaml-001",
			Name:   "(service web-service) has reached a steady state.",
			Status: "",
			Fields: map[string]string{
				"timestamp": "2024-03-22 10:00",
				"message":   "(service web-service) has reached a steady state.",
			},
		},
	}
}

// ===========================================================================
// ECS Service Events YAML tests
// ===========================================================================

func TestQA_YAML_EcsSvcEvents_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureEcsSvcEvents() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("EcsSvcEvents YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("EcsSvcEvents YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_EcsSvcEvents_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureEcsSvcEvents()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("EcsSvcEvents FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_EcsSvcEvents_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureEcsSvcEvents()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("EcsSvcEvents RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// ECS Service Tasks YAML view fixtures
// ===========================================================================

func fixtureEcsSvcTasks() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "abc123def456",
			Name:   "abc123def456",
			Status: "RUNNING",
			Fields: map[string]string{
				"task_id_short":  "abc123def456",
				"status":         "RUNNING",
				"health":         "HEALTHY",
				"task_def_short": "web-app:5",
				"started_at":     "2024-03-22 10:00",
				"stopped_reason": "",
			},
		},
	}
}

// ===========================================================================
// ECS Service Tasks YAML tests
// ===========================================================================

func TestQA_YAML_EcsSvcTasks_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureEcsSvcTasks() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("EcsSvcTasks YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("EcsSvcTasks YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_EcsSvcTasks_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureEcsSvcTasks()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("EcsSvcTasks FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_EcsSvcTasks_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureEcsSvcTasks()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("EcsSvcTasks RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// ECS Service Logs YAML view fixtures
// ===========================================================================

func fixtureEcsSvcLogs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "evt-svc-log-001",
			Name:   "INFO Starting application server on port 8080",
			Status: "",
			Fields: map[string]string{
				"timestamp":    "2024-03-21 16:00",
				"stream_short": "web/abc123de",
				"message":      "INFO Starting application server on port 8080",
			},
		},
	}
}

// ===========================================================================
// ECS Service Logs YAML tests
// ===========================================================================

func TestQA_YAML_EcsSvcLogs_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureEcsSvcLogs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("EcsSvcLogs YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("EcsSvcLogs YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_EcsSvcLogs_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureEcsSvcLogs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("EcsSvcLogs FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_EcsSvcLogs_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureEcsSvcLogs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("EcsSvcLogs RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// CFN Stack Events YAML view fixtures
// ===========================================================================

func fixtureCfnEvents() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "evt-yaml-cfn-001",
			Name:   "2024-03-22 10:00:00",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"timestamp":              "2024-03-22 10:00:00",
				"logical_resource_id":    "MyBucket",
				"resource_type":          "AWS::S3::Bucket",
				"resource_status":        "CREATE_COMPLETE",
				"resource_status_reason": "Resource creation complete",
			},
		},
	}
}

// ===========================================================================
// CFN Stack Events YAML tests
// ===========================================================================

func TestQA_YAML_CfnEvents_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureCfnEvents() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("CfnEvents YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("CfnEvents YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_CfnEvents_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureCfnEvents()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("CfnEvents FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_CfnEvents_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureCfnEvents()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("CfnEvents RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// CFN Stack Resources YAML view fixtures
// ===========================================================================

func fixtureCfnResources() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "MyBucket",
			Name:   "MyBucket",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"logical_resource_id":  "MyBucket",
				"physical_resource_id": "my-stack-mybucket-abc123",
				"resource_type":        "AWS::S3::Bucket",
				"resource_status":      "CREATE_COMPLETE",
				"drift_status":         "IN_SYNC",
				"last_updated":         "2024-03-22 10:00:00",
			},
		},
	}
}

// ===========================================================================
// CFN Stack Resources YAML tests
// ===========================================================================

func TestQA_YAML_CfnResources_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureCfnResources() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("CfnResources YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("CfnResources YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_CfnResources_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureCfnResources()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("CfnResources FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_CfnResources_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureCfnResources()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("CfnResources RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// ASG Scaling Activities YAML view fixtures
// ===========================================================================

func fixtureAsgActivities() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "act-yaml-001",
			Name:   "2024-03-22 10:00:00",
			Status: "Successful",
			Fields: map[string]string{
				"start_time":  "2024-03-22 10:00:00",
				"status_code": "Successful",
				"description": "Launching a new EC2 instance: i-0abc1234",
				"cause":       "At 2024-03-22T10:00:00Z an instance was started",
			},
		},
		{
			ID:     "act-yaml-002",
			Name:   "2024-03-22 10:05:00",
			Status: "Failed",
			Fields: map[string]string{
				"start_time":  "2024-03-22 10:05:00",
				"status_code": "Failed",
				"description": "Terminating EC2 instance: i-0def5678",
				"cause":       "At 2024-03-22T10:05:00Z an instance was terminated due to health check failure",
			},
		},
	}
}

// ===========================================================================
// ASG Scaling Activities YAML tests
// ===========================================================================

func TestQA_YAML_AsgActivities_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureAsgActivities() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("AsgActivities YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("AsgActivities YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_AsgActivities_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureAsgActivities()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("AsgActivities FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_AsgActivities_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureAsgActivities()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("AsgActivities RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// Alarm History YAML view fixtures
// ===========================================================================

func fixtureAlarmHistory() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "2024-03-22 10:00:00",
			Name:   "2024-03-22 10:00:00",
			Status: "StateUpdate",
			Fields: map[string]string{
				"timestamp":         "2024-03-22 10:00:00",
				"history_item_type": "StateUpdate",
				"history_summary":   "Alarm updated from OK to ALARM",
			},
		},
		{
			ID:     "2024-03-22 10:05:00",
			Name:   "2024-03-22 10:05:00",
			Status: "ConfigurationUpdate",
			Fields: map[string]string{
				"timestamp":         "2024-03-22 10:05:00",
				"history_item_type": "ConfigurationUpdate",
				"history_summary":   "Alarm threshold changed from 80 to 90",
			},
		},
	}
}

// ===========================================================================
// Alarm History YAML tests
// ===========================================================================

func TestQA_YAML_AlarmHistory_ViewContainsFields(t *testing.T) {
	for _, r := range fixtureAlarmHistory() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("AlarmHistory YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("AlarmHistory YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_AlarmHistory_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureAlarmHistory()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("AlarmHistory FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_AlarmHistory_RawContentUncolored(t *testing.T) {
	m := yamlModel(fixtureAlarmHistory()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("AlarmHistory RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// ELB Listeners YAML view fixtures
// ===========================================================================

func fixtureELBListeners() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456",
			Name:   "443",
			Status: "",
			Fields: map[string]string{
				"port":                  "443",
				"protocol":              "HTTPS",
				"default_action_type":   "forward",
				"default_action_target": "api-prod-tg",
				"ssl_policy":            "ELBSecurityPolicy-TLS13-1-2-2021-06",
				"certificate_short":     "abc-def-123",
			},
		},
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/ghi789",
			Name:   "80",
			Status: "",
			Fields: map[string]string{
				"port":                  "80",
				"protocol":              "HTTP",
				"default_action_type":   "redirect",
				"default_action_target": "HTTPS:443",
				"ssl_policy":            "",
				"certificate_short":     "",
			},
		},
	}
}

// ===========================================================================
// ELB Listeners YAML tests
// ===========================================================================

func TestQA_YAML_ELBListeners_ContainsFields(t *testing.T) {
	for _, r := range fixtureELBListeners() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ELBListeners YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("ELBListeners YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_ELBListeners_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureELBListeners()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("ELBListeners FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ELBListeners_NoANSI(t *testing.T) {
	m := yamlModel(fixtureELBListeners()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ELBListeners RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// CodeBuild Builds YAML view fixtures
// ===========================================================================

func fixtureCBBuilds() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "my-project:build-id-001",
			Name:   "#142",
			Status: "SUCCEEDED",
			Fields: map[string]string{
				"build_number":            "142",
				"build_status":            "SUCCEEDED",
				"start_time":              "2024-06-15 10:00:00",
				"end_time":                "2024-06-15 10:04:12",
				"duration":                "4m 12s",
				"source_version_short":    "abc123de",
				"initiator":               "codepipeline/my-pipeline",
				"build_id":                "my-project:build-id-001",
				"build_arn":               "arn:aws:codebuild:us-east-1:123456789012:build/my-project:build-id-001",
				"current_phase":           "COMPLETED",
				"source_version":          "abc123def456789012345678901234567890abcd",
				"resolved_source_version": "abc123def456789012345678901234567890abcd",
				"log_group_name":          "/aws/codebuild/my-project",
				"log_stream_name":         "build-id-001",
			},
		},
	}
}

// ===========================================================================
// CodeBuild Builds YAML tests
// ===========================================================================

func TestQA_YAML_CBBuilds_ContainsFields(t *testing.T) {
	for _, r := range fixtureCBBuilds() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("CBBuilds YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("CBBuilds YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_CBBuilds_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureCBBuilds()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("CBBuilds FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_CBBuilds_NoANSI(t *testing.T) {
	m := yamlModel(fixtureCBBuilds()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("CBBuilds RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// CodeBuild Build Logs YAML view fixtures
// ===========================================================================

func fixtureCBBuildLogs() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "evt-1718445600000-0",
			Name:   "[Container] Running command echo hello",
			Status: "IN_PROGRESS",
			Fields: map[string]string{
				"timestamp":      "2024-06-15 10:00:00",
				"message":        "[Container] Running command echo hello",
				"ingestion_time": "2024-06-15 10:00:01",
				"event_id":       "evt-1718445600000-0",
			},
		},
	}
}

// ===========================================================================
// CodeBuild Build Logs YAML tests
// ===========================================================================

func TestQA_YAML_CBBuildLogs_ContainsFields(t *testing.T) {
	for _, r := range fixtureCBBuildLogs() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("CBBuildLogs YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("CBBuildLogs YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_CBBuildLogs_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureCBBuildLogs()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("CBBuildLogs FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_CBBuildLogs_NoANSI(t *testing.T) {
	m := yamlModel(fixtureCBBuildLogs()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("CBBuildLogs RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// ECR Images YAML view fixtures
// ===========================================================================

func fixtureECRImages() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			Name:   "latest, v1.0.0",
			Status: "",
			Fields: map[string]string{
				"image_tags":     "latest, v1.0.0",
				"digest_short":   "abcdef123456",
				"pushed_at":      "2024-06-15 10:00:00",
				"image_size":     "50.0 MB",
				"scan_status":    "COMPLETE",
				"finding_counts": "3H 5M",
				"image_uri":      "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:latest",
			},
		},
	}
}

// ===========================================================================
// ECR Images YAML tests
// ===========================================================================

func TestQA_YAML_ECRImages_ContainsFields(t *testing.T) {
	for _, r := range fixtureECRImages() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("ECRImages YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("ECRImages YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_ECRImages_FrameTitle(t *testing.T) {
	m := yamlModel(fixtureECRImages()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("ECRImages FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_ECRImages_NoANSI(t *testing.T) {
	m := yamlModel(fixtureECRImages()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("ECRImages RawContent() contains ANSI codes, expected plain YAML")
	}
}

// ===========================================================================
// Pipeline Stages YAML view fixtures
// ===========================================================================

func fixturePipelineStages() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "Source/GitHub",
			Name:   "GitHub",
			Status: "running",
			Fields: map[string]string{
				"stage_name":           "Source",
				"stage_status":         "Succeeded",
				"action_name":          "GitHub",
				"action_status":        "Succeeded",
				"last_change_time":     "2024-06-15 10:00:00",
				"external_url":         "https://github.com/org/repo/commit/abc123",
				"action_token":         "approval-token-xyz",
				"action_error_details": "",
				"revision_id":          "abc123def456",
				"revision_summary":     "commit-sha-abc",
			},
		},
	}
}

// ===========================================================================
// Pipeline Stages YAML tests
// ===========================================================================

func TestQA_YAML_PipelineStages_ContainsFields(t *testing.T) {
	for _, r := range fixturePipelineStages() {
		out := yamlView(t, r, 120, 40)
		for k, val := range r.Fields {
			if !strings.Contains(out, k) {
				t.Errorf("PipelineStages YAML for %q missing key %q", r.ID, k)
			}
			if val != "" && !strings.Contains(out, val) {
				t.Errorf("PipelineStages YAML for %q missing value %q", r.ID, val)
			}
		}
	}
}

func TestQA_YAML_PipelineStages_FrameTitle(t *testing.T) {
	m := yamlModel(fixturePipelineStages()[0], 120, 40)
	if title := m.FrameTitle(); !strings.Contains(title, "yaml") {
		t.Errorf("PipelineStages FrameTitle() = %q, want 'yaml' in title", title)
	}
}

func TestQA_YAML_PipelineStages_NoANSI(t *testing.T) {
	m := yamlModel(fixturePipelineStages()[0], 120, 40)
	raw := m.RawContent()
	if raw != stripANSI(raw) {
		t.Error("PipelineStages RawContent() contains ANSI codes, expected plain YAML")
	}
}

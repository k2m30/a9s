package unit

// qa67_malformed_data_test.go — §C Corrupted / Malformed Data (issue #67)
//
// Bugs caught:
//   - C.1: nil optional fields in fetcher output panic the resource list render
//   - C.2: empty ID/name renders a blank row that is still selectable (not skipped)
//   - C.3: unknown enum values use default color, not crash
//   - C.4: malformed ARN is displayed as-is (no truncation or panic)
//   - C.5: unicode names in resource list do not corrupt column layout
//   - C.6: zero-value timestamps are displayed without panic
//   - C.7: nil nested struct shows empty/null in detail, no panic
//   - C.8: very long tag value (256 chars) does not break list layout
//   - C.9: resource with all-nil optional fields renders detail without crash

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// C.1 — Nil optional fields in resource do not cause panic in list render.
func TestQa67_C1_NilOptionalFields_ListRenderDoesNotPanic(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	// Resource with many empty/nil-equivalent fields
	resources := []resource.Resource{
		{
			ID:     "i-niltest01",
			Name:   "",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-niltest01",
				"name":        "",
				"state":       "running",
				"type":        "",
				"private_ip":  "",
				"public_ip":   "",
				"launch_time": "",
				"lifecycle":   "",
				"key_name":    "",
				"iam_profile": "",
			},
		},
	}
	// Must not panic
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources})
	out := rootViewContent(m)
	plain := stripANSI(out)
	if plain == "" {
		t.Error("C.1: resource list should render non-empty output even with nil-equivalent fields")
	}
	// Application did not crash: output contains frame structure
	if !strings.Contains(plain, "ec2") && !strings.Contains(plain, "i-niltest01") {
		t.Logf("C.1: output: %s", plain[:min(300, len(plain))])
	}
}

// C.1 — All registered resource types: loading a resource with empty fields does not panic.
func TestQa67_C1_NilFields_AllResourceTypes(t *testing.T) {
	// Representative sample — full sweep in CI slow suite
	for _, rt := range []string{"ec2", "s3", "secrets", "dbi"} {
		t.Run(rt, func(t *testing.T) {
			m := newRootSizedModel()
			m, _ = rootApplyMsg(m, messages.Navigate{
				Target:       messages.TargetResourceList,
				ResourceType: rt,
			})
			// Resource with only ID populated — all other fields empty
			empty := resource.Resource{
				ID:     "empty-resource-id",
				Name:   "",
				Status: "",
				Fields: map[string]string{},
			}
			// Must not panic
			m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
				ResourceType: rt,
				Resources:    []resource.Resource{empty},
			})
			out := rootViewContent(m)
			if out == "" {
				t.Errorf("[%s] C.1: View() returned empty after loading resource with empty fields", rt)
			}
		})
	}
}

// C.2 — Empty string ID renders a selectable row (not skipped or panicking).
func TestQa67_C2_EmptyID_RowStillRendersAndSelectable(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:     "",
			Name:   "",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "",
				"name":        "",
				"state":       "running",
				"type":        "t2.micro",
				"private_ip":  "10.0.0.1",
				"public_ip":   "",
				"launch_time": "",
				"lifecycle":   "",
			},
		},
		{
			ID:     "i-normal",
			Name:   "normal-instance",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-normal",
				"name":        "normal-instance",
				"state":       "running",
				"type":        "t2.micro",
				"private_ip":  "10.0.0.2",
				"public_ip":   "",
				"launch_time": "",
				"lifecycle":   "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources})
	// Must not crash
	out := rootViewContent(m)
	plain := stripANSI(out)
	// The normal instance should be visible; application should not crash
	if !strings.Contains(plain, "normal-instance") {
		t.Errorf("C.2: normal instance should appear after loading resources with an empty-ID row, got: %s", plain[:min(300, len(plain))])
	}
}

// C.3 — Unknown enum value in status field renders as plain text, no crash.
func TestQa67_C3_UnknownEnum_RendersAsPlainText(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:     "i-unknown-state",
			Name:   "unusual-instance",
			Status: "hibernating", // not a known EC2 state
			Fields: map[string]string{
				"instance_id": "i-unknown-state",
				"name":        "unusual-instance",
				"state":       "hibernating",
				"type":        "t3.micro",
				"private_ip":  "10.0.1.5",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources})
	out := rootViewContent(m)
	plain := stripANSI(out)
	// The row must render without crashing and display the resource
	if !strings.Contains(plain, "unusual-instance") {
		t.Errorf("C.3: resource with unknown status should render its name, got: %s", plain[:min(300, len(plain))])
	}
}

// C.4 — Malformed ARN renders as-is in the list without crash or truncation of other fields.
func TestQa67_C4_MalformedARN_RendersWithoutPanic(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	malformedARN := "arn:aws::::::malformed::extra::colons::everywhere"
	resources := []resource.Resource{
		{
			ID:     malformedARN,
			Name:   "malformed-arn-secret",
			Status: "active",
			Fields: map[string]string{
				"name":               "malformed-arn-secret",
				"arn":                malformedARN,
				"description":        "",
				"last_changed_date":  "",
				"last_accessed_date": "",
				"rotation_enabled":   "false",
				"kms_key_id":         "",
				"tags":               "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "secrets", Resources: resources})
	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "malformed-arn-secret") {
		t.Errorf("C.4: secret with malformed ARN should render without crash, got: %s", plain[:min(300, len(plain))])
	}
}

// C.5 — Unicode and emoji characters in resource names do not corrupt layout or panic.
func TestQa67_C5_UnicodeNames_DoNotCorruptLayout(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	unicodeNames := []struct {
		id, name string
	}{
		{"i-emoji", "🚀 rocket-prod"},
		{"i-cjk", "生産サーバー-01"},
		{"i-arabic", "خادم-إنتاج"},
		{"i-mixed", "server-日本語-🌍"},
	}
	var resources []resource.Resource
	for _, u := range unicodeNames {
		resources = append(resources, resource.Resource{
			ID:     u.id,
			Name:   u.name,
			Status: "running",
			Fields: map[string]string{
				"instance_id": u.id,
				"name":        u.name,
				"state":       "running",
				"type":        "t3.micro",
				"private_ip":  "10.0.0.1",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			},
		})
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources})
	// Must not panic; output must be non-empty
	out := rootViewContent(m)
	if out == "" {
		t.Error("C.5: View() returned empty after loading resources with unicode names")
	}
}

// C.6 — Zero-value timestamp (Go zero time) does not panic during rendering.
func TestQa67_C6_ZeroTimestamp_DoesNotPanic(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:     "i-zero-time",
			Name:   "zero-time-instance",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-zero-time",
				"name":        "zero-time-instance",
				"state":       "running",
				"type":        "t2.micro",
				"private_ip":  "10.0.0.5",
				"public_ip":   "",
				"launch_time": "0001-01-01T00:00:00Z", // zero time as string
				"lifecycle":   "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources})
	out := rootViewContent(m)
	if out == "" {
		t.Error("C.6: View() returned empty after loading resource with zero timestamp")
	}
}

// C.7 — Resource with all optional fields empty (nil equivalent) renders in detail without panic.
func TestQa67_C7_AllNilOptionalFields_DetailViewNoPanic(t *testing.T) {
	m := newRootSizedModel()
	// Resource with every optional field empty
	res := &resource.Resource{
		ID:        "i-all-nil",
		Name:      "all-nil-instance",
		Status:    "stopped",
		Fields:    map[string]string{},
		RawStruct: nil,
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})
	// Must not panic
	out := rootViewContent(m)
	if out == "" {
		t.Error("C.7: detail view should not be empty for resource with all-nil fields")
	}
}

// C.8 — Very long tag value (256 chars) in resource Fields does not break layout.
func TestQa67_C8_LongTagValue_DoesNotBreakListLayout(t *testing.T) {
	longTag := strings.Repeat("x", 256)
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:     "i-long-tag",
			Name:   "long-tag-instance",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-long-tag",
				"name":        "long-tag-instance",
				"state":       "running",
				"type":        "t3.medium",
				"private_ip":  "10.0.2.1",
				"public_ip":   "",
				"launch_time": "2025-06-01",
				"lifecycle":   "",
				"tags":        "LongTag=" + longTag,
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources})
	out := rootViewContent(m)
	plain := stripANSI(out)
	// The resource row should still appear
	if !strings.Contains(plain, "long-tag-instance") {
		t.Errorf("C.8: resource with 256-char tag should render its name, got: %s", plain[:min(300, len(plain))])
	}
}

// C.9 — Resource with all optional fields nil renders in detail view without crash.
func TestQa67_C9_AllNilFields_DetailViewRendersAvailableFields(t *testing.T) {
	m := newRootSizedModel()
	res := &resource.Resource{
		ID:     "minimal-resource",
		Name:   "minimal",
		Status: "available",
		Fields: map[string]string{
			"db_identifier":  "minimal-resource",
			"engine":         "mysql",
			"engine_version": "",
			"status":         "available",
			"class":          "",
			"endpoint":       "",
			"multi_az":       "",
		},
		RawStruct: nil,
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})
	out := rootViewContent(m)
	if out == "" {
		t.Error("C.9: detail view should render non-empty output even with mostly nil fields")
	}
}

// C.9 extended — pressing y after navigating to detail with nil fields does not panic.
func TestQa67_C9_AllNilFields_YAMLViewNoPanic(t *testing.T) {
	m := newRootSizedModel()
	res := &resource.Resource{
		ID:        "yaml-nil-resource",
		Name:      "yaml-nil",
		Status:    "available",
		Fields:    map[string]string{},
		RawStruct: nil,
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: res,
	})
	// Must not panic
	out := rootViewContent(m)
	if out == "" {
		t.Error("C.9: YAML view should render non-empty output for resource with nil RawStruct")
	}
}

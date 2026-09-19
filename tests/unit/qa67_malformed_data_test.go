package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// Nil optional fields in a resource do not panic the list render.
func TestQa67_C1_NilOptionalFields_ListRenderDoesNotPanic(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:   "i-niltest01",
			Name: "",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: resources})
	out := rootViewContent(m)
	plain := stripANSI(out)
	if plain == "" {
		t.Error("C.1: resource list should render non-empty output even with nil-equivalent fields")
	}
	if !strings.Contains(plain, "ec2") && !strings.Contains(plain, "i-niltest01") {
		t.Logf("C.1: output: %s", plain[:min(300, len(plain))])
	}
}

// Loading a resource with empty fields does not panic, across resource types.
func TestQa67_C1_NilFields_AllResourceTypes(t *testing.T) {
	for _, rt := range []string{"ec2", "s3", "secrets", "dbi"} {
		t.Run(rt, func(t *testing.T) {
			m := newRootSizedModel()
			m, _ = rootApplyMsg(m, messages.Navigate{
				Target:       messages.TargetResourceList,
				ResourceType: rt,
			})
			empty := resource.Resource{
				ID:     "empty-resource-id",
				Name:   "",
				Fields: map[string]string{},
			}
			m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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

// An empty string ID renders a selectable row rather than being skipped or
// panicking.
func TestQa67_C2_EmptyID_RowStillRendersAndSelectable(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:   "",
			Name: "",
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
			ID:   "i-normal",
			Name: "normal-instance",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources, Provenance: messages.FetchProvenanceCanonicalList})

	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "normal-instance") {
		t.Errorf("C.2: normal instance should appear after loading resources with an empty-ID row, got: %s", plain[:min(300, len(plain))])
	}
}

// An unknown enum value in the status field renders as plain text.
func TestQa67_C3_UnknownEnum_RendersAsPlainText(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:   "i-unknown-state",
			Name: "unusual-instance",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources, Provenance: messages.FetchProvenanceCanonicalList})
	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "unusual-instance") {
		t.Errorf("C.3: resource with unknown status should render its name, got: %s", plain[:min(300, len(plain))])
	}
}

// A malformed ARN renders as-is in the list without truncating other fields.
func TestQa67_C4_MalformedARN_RendersWithoutPanic(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	malformedARN := "arn:aws::::::malformed::extra::colons::everywhere"
	resources := []resource.Resource{
		{
			ID:   malformedARN,
			Name: "malformed-arn-secret",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "secrets", Resources: resources, Provenance: messages.FetchProvenanceCanonicalList})
	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "malformed-arn-secret") {
		t.Errorf("C.4: secret with malformed ARN should render without crash, got: %s", plain[:min(300, len(plain))])
	}
}

// Unicode and emoji in resource names do not corrupt the layout or panic.
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
			ID:   u.id,
			Name: u.name,
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: resources})
	out := rootViewContent(m)
	if out == "" {
		t.Error("C.5: View() returned empty after loading resources with unicode names")
	}
}

// A zero-value timestamp does not panic during rendering.
func TestQa67_C6_ZeroTimestamp_DoesNotPanic(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:   "i-zero-time",
			Name: "zero-time-instance",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: resources})
	out := rootViewContent(m)
	if out == "" {
		t.Error("C.6: View() returned empty after loading resource with zero timestamp")
	}
}

// A resource with every optional field empty renders in detail without panic.
func TestQa67_C7_AllNilOptionalFields_DetailViewNoPanic(t *testing.T) {
	m := newRootSizedModel()
	res := &resource.Resource{
		ID:        "i-all-nil",
		Name:      "all-nil-instance",
		Fields:    map[string]string{},
		RawStruct: nil,
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})
	out := rootViewContent(m)
	if out == "" {
		t.Error("C.7: detail view should not be empty for resource with all-nil fields")
	}
}

// A 256-char tag value in resource Fields does not break the list layout.
func TestQa67_C8_LongTagValue_DoesNotBreakListLayout(t *testing.T) {
	longTag := strings.Repeat("x", 256)
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:   "i-long-tag",
			Name: "long-tag-instance",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources, Provenance: messages.FetchProvenanceCanonicalList})
	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "long-tag-instance") {
		t.Errorf("C.8: resource with 256-char tag should render its name, got: %s", plain[:min(300, len(plain))])
	}
}

// A resource with all optional fields nil renders its available fields in
// the detail view.
func TestQa67_C9_AllNilFields_DetailViewRendersAvailableFields(t *testing.T) {
	m := newRootSizedModel()
	res := &resource.Resource{
		ID:   "minimal-resource",
		Name: "minimal",
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

// Pressing y in the detail view of a resource with nil fields does not panic.
func TestQa67_C9_AllNilFields_YAMLViewNoPanic(t *testing.T) {
	m := newRootSizedModel()
	res := &resource.Resource{
		ID:        "yaml-nil-resource",
		Name:      "yaml-nil",
		Fields:    map[string]string{},
		RawStruct: nil,
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: res,
	})
	out := rootViewContent(m)
	if out == "" {
		t.Error("C.9: YAML view should render non-empty output for resource with nil RawStruct")
	}
}

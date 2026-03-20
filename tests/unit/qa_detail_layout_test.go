package unit_test

import (
	"strings"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/views"
)

func TestDetailLayout_KeysHaveColon(t *testing.T) {
	ensureNoColor(t)

	res := resource.Resource{
		ID:   "i-test",
		Name: "test-server",
		Fields: map[string]string{
			"instance_id": "i-test",
			"state":       "running",
			"type":        "t3.micro",
		},
	}
	k := keys.Default()
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(80, 30)
	view := m.View()
	plain := stripAnsi(view)

	// Every key-value line should have a colon after the key name
	for _, line := range strings.Split(plain, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip section headers (indented sub-lines)
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "Code:") || strings.HasPrefix(trimmed, "Name:") {
			continue
		}
		// Lines with field names should contain ":"
		if strings.Contains(trimmed, "instance_id") || strings.Contains(trimmed, "state") || strings.Contains(trimmed, "type") {
			if !strings.Contains(trimmed, ":") {
				t.Errorf("key-value line missing colon separator: %q", trimmed)
			}
		}
	}
}

func TestDetailLayout_ScalarFieldsInline(t *testing.T) {
	ensureNoColor(t)

	cfg, err := config.LoadFrom([]string{"../../.a9s/views.yaml"})
	if err != nil {
		t.Skipf("views.yaml not found: %v", err)
	}

	// Use a simple struct where State is a scalar string, not a nested object
	res := resource.Resource{
		ID:   "i-test",
		Name: "test",
		Fields: map[string]string{
			"instance_id": "i-test",
			"state":       "running",
			"type":        "t3.micro",
			"private_ip":  "10.0.1.5",
		},
	}
	k := keys.Default()
	m := views.NewDetail(res, "ec2", cfg, k)
	m.SetSize(80, 30)
	view := m.View()
	plain := stripAnsi(view)

	// "InstanceId" should be on the same line as its value with a colon
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "InstanceId") {
			if !strings.Contains(line, ":") {
				t.Errorf("InstanceId line should have colon: %q", line)
			}
			if !strings.Contains(line, "i-test") {
				t.Errorf("InstanceId line should have value on same line: %q", line)
			}
		}
	}
}

func TestDetailLayout_SectionHeadersAlignedWithScalars(t *testing.T) {
	ensureNoColor(t)

	// Build a resource with RawStruct that produces BOTH scalar fields and section fields.
	// EC2 Instance with a nested State struct is ideal: InstanceId is scalar, State is a section.
	inst := ec2types.Instance{
		InstanceId:       ptrString("i-test123"),
		InstanceType:     ec2types.InstanceTypeT3Large,
		PrivateIpAddress: ptrString("10.0.1.5"),
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: ptrInt32(16),
		},
	}
	cfg, err := config.LoadFrom([]string{"../../.a9s/views.yaml"})
	if err != nil {
		t.Fatalf("views.yaml not found: %v", err)
	}
	res := resource.Resource{
		ID:        "i-test123",
		Name:      "test-server",
		RawStruct: inst,
	}

	k := keys.Default()
	m := views.NewDetail(res, "ec2", cfg, k)
	m.SetSize(100, 40)
	view := m.View()
	plain := stripAnsi(view)

	// Count leading spaces on every non-empty line.
	// Verify:
	// 1. ALL top-level key lines start with exactly 1 space
	// 2. Sub-field lines start with 5 spaces
	// 3. NO line starts with exactly 3 spaces (old broken indent)

	foundScalar := false
	foundSection := false
	foundSubField := false

	for _, line := range strings.Split(plain, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) == 0 || line[0] != ' ' {
			continue // skip non-indented lines (border, title, etc.)
		}

		spaces := 0
		for _, ch := range line {
			if ch == ' ' {
				spaces++
			} else {
				break
			}
		}

		trimmed := strings.TrimSpace(line)

		// 3 spaces is the old broken indent — must never appear
		if spaces == 3 {
			t.Errorf("line starts with exactly 3 spaces (old broken indent): %q", line)
		}

		// Top-level keys should have exactly 1 space
		if spaces == 1 {
			if !strings.Contains(trimmed, ":") {
				t.Errorf("top-level line at 1-space indent should contain colon: %q", line)
			}
			// Track whether we see both scalars and sections
			if strings.HasSuffix(trimmed, ":") {
				foundSection = true // section header like "State:"
			} else {
				foundScalar = true // scalar like "InstanceId: i-test123"
			}
		}

		// Sub-fields should have exactly 5 spaces
		if spaces == 5 {
			foundSubField = true
		}

		// Only valid indentation levels are 1, 5, or 9+
		if spaces != 1 && spaces != 5 && spaces < 9 && spaces != 0 {
			// Allow 2 spaces for "No detail data" dim text, but nothing else
			if spaces != 2 || !strings.Contains(trimmed, "No detail") {
				t.Errorf("unexpected indentation of %d spaces: %q", spaces, line)
			}
		}
	}

	if !foundScalar {
		t.Error("expected at least one scalar field line at 1-space indent, found none")
	}
	if !foundSection {
		t.Error("expected at least one section header line at 1-space indent, found none")
	}
	if !foundSubField {
		t.Error("expected at least one sub-field line at 5-space indent, found none")
	}
}

func TestDetailLayout_EmptySliceShowsDashNotNull(t *testing.T) {
	ensureNoColor(t)

	// Bug: EC2 instance with empty SecurityGroups slice renders as "null" in detail view.
	// Expected: "-" (empty) or omitted, never "null".
	inst := ec2types.Instance{
		InstanceId:   ptrString("i-0abc123def456789a"),
		InstanceType: ec2types.InstanceTypeT3Large,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameTerminated,
			Code: ptrInt32(48),
		},
		SecurityGroups: []ec2types.GroupIdentifier{}, // empty slice — triggers the bug
		Tags:           []ec2types.Tag{},             // another empty slice
	}
	cfg, err := config.LoadFrom([]string{"../../.a9s/views.yaml"})
	if err != nil {
		t.Fatalf("views.yaml not found: %v", err)
	}
	res := resource.Resource{
		ID:        "i-0abc123def456789a",
		Name:      "test-terminated",
		RawStruct: inst,
	}

	k := keys.Default()
	m := views.NewDetail(res, "ec2", cfg, k)
	m.SetSize(100, 40)
	view := m.View()
	plain := stripAnsi(view)

	// "null" must never appear in detail view output
	if strings.Contains(plain, "null") {
		t.Errorf("Detail view must not display 'null' for empty lists. Got:\n%s", plain)
	}

	// SecurityGroups should show "-" (empty/no data), not "null"
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "SecurityGroups") {
			if strings.Contains(line, "null") {
				t.Errorf("SecurityGroups line should show '-', not 'null': %q", line)
			}
		}
		if strings.Contains(line, "Tags") {
			if strings.Contains(line, "null") {
				t.Errorf("Tags line should show '-', not 'null': %q", line)
			}
		}
	}
}

func TestDetailLayout_NilSliceShowsDashNotNull(t *testing.T) {
	ensureNoColor(t)

	// Nil slices (not just empty) should also show "-", not "null"
	inst := ec2types.Instance{
		InstanceId:   ptrString("i-0abc123def456789a"),
		InstanceType: ec2types.InstanceTypeT3Large,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameTerminated,
			Code: ptrInt32(48),
		},
		// SecurityGroups and Tags are nil (zero value for slices)
	}
	cfg, err := config.LoadFrom([]string{"../../.a9s/views.yaml"})
	if err != nil {
		t.Fatalf("views.yaml not found: %v", err)
	}
	res := resource.Resource{
		ID:        "i-0abc123def456789a",
		Name:      "test-nil-slices",
		RawStruct: inst,
	}

	k := keys.Default()
	m := views.NewDetail(res, "ec2", cfg, k)
	m.SetSize(100, 40)
	view := m.View()
	plain := stripAnsi(view)

	if strings.Contains(plain, "null") {
		t.Errorf("Detail view must not display 'null' for nil slices. Got:\n%s", plain)
	}
}

func TestDetailLayout_SectionHeadersHaveColon(t *testing.T) {
	ensureNoColor(t)

	// When a field is multi-line (struct/array), the section header should also use colon
	// e.g., "State:" not just "State"
	// This can't easily be tested without RawStruct, but we verify the format
	res := resource.Resource{
		ID:   "test",
		Name: "test",
		Fields: map[string]string{
			"key1": "value1",
		},
	}
	k := keys.Default()
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(80, 20)
	view := m.View()
	plain := stripAnsi(view)

	// With simple fields, every non-empty line should have ":"
	for _, line := range strings.Split(plain, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "Initializing..." {
			continue
		}
		if !strings.Contains(trimmed, ":") && !strings.Contains(trimmed, "-") {
			t.Errorf("line should contain colon: %q", trimmed)
		}
	}
}

package unit

import (
	"testing"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/views"
)

// ---------------------------------------------------------------------------
// T051 - Test FilterResources
// ---------------------------------------------------------------------------

func makeTestResources() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "i-001",
			Name:   "prod-web-1",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-001",
				"name":        "prod-web-1",
				"state":       "running",
				"type":        "t3.medium",
				"env":         "production",
			},
		},
		{
			ID:     "i-002",
			Name:   "prod-api-1",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-002",
				"name":        "prod-api-1",
				"state":       "running",
				"type":        "t3.large",
				"env":         "production",
			},
		},
		{
			ID:     "i-003",
			Name:   "staging-web-1",
			Status: "stopped",
			Fields: map[string]string{
				"instance_id": "i-003",
				"name":        "staging-web-1",
				"state":       "stopped",
				"type":        "t3.small",
				"env":         "staging",
			},
		},
		{
			ID:     "i-004",
			Name:   "dev-worker-1",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-004",
				"name":        "dev-worker-1",
				"state":       "running",
				"type":        "t3.micro",
				"env":         "development",
			},
		},
		{
			ID:     "i-005",
			Name:   "test-batch",
			Status: "running",
			Fields: map[string]string{
				"instance_id": "i-005",
				"name":        "test-batch",
				"state":       "running",
				"type":        "t3.medium",
				"env":         "test",
			},
		},
	}
}

func TestFilterResources_ProdFilter(t *testing.T) {
	resources := makeTestResources()
	result := views.FilterResources("prod", resources)

	// Should match "prod-web-1" and "prod-api-1" by name,
	// and also anything with "production" in fields.
	if len(result) != 2 {
		t.Errorf("expected 2 resources matching 'prod', got %d", len(result))
		for _, r := range result {
			t.Logf("  matched: %s (%s)", r.Name, r.ID)
		}
	}

	for _, r := range result {
		if r.Name != "prod-web-1" && r.Name != "prod-api-1" {
			t.Errorf("unexpected resource in result: %s", r.Name)
		}
	}
}

func TestFilterResources_CaseInsensitive(t *testing.T) {
	resources := makeTestResources()

	resultLower := views.FilterResources("prod", resources)
	resultUpper := views.FilterResources("PROD", resources)
	resultMixed := views.FilterResources("Prod", resources)

	if len(resultLower) != len(resultUpper) {
		t.Errorf("case-insensitive mismatch: lower=%d, upper=%d", len(resultLower), len(resultUpper))
	}
	if len(resultLower) != len(resultMixed) {
		t.Errorf("case-insensitive mismatch: lower=%d, mixed=%d", len(resultLower), len(resultMixed))
	}
}

func TestFilterResources_EmptyFilter(t *testing.T) {
	resources := makeTestResources()
	result := views.FilterResources("", resources)

	if len(result) != len(resources) {
		t.Errorf("expected all %d resources for empty filter, got %d", len(resources), len(result))
	}
}

func TestFilterResources_NoMatch(t *testing.T) {
	resources := makeTestResources()
	result := views.FilterResources("nonexistent-xyz", resources)

	if len(result) != 0 {
		t.Errorf("expected 0 resources for non-matching filter, got %d", len(result))
	}
}

func TestFilterResources_MatchesFieldValues(t *testing.T) {
	resources := makeTestResources()
	// "t3.medium" appears in fields of i-001 and i-005
	result := views.FilterResources("t3.medium", resources)

	if len(result) != 2 {
		t.Errorf("expected 2 resources matching 't3.medium' in fields, got %d", len(result))
	}
}

func TestFilterResources_MatchesStatus(t *testing.T) {
	resources := makeTestResources()
	result := views.FilterResources("stopped", resources)

	if len(result) != 1 {
		t.Errorf("expected 1 resource matching 'stopped', got %d", len(result))
	}
	if len(result) > 0 && result[0].Name != "staging-web-1" {
		t.Errorf("expected staging-web-1, got %s", result[0].Name)
	}
}

func TestFilterResources_MatchesID(t *testing.T) {
	resources := makeTestResources()
	result := views.FilterResources("i-003", resources)

	if len(result) != 1 {
		t.Errorf("expected 1 resource matching 'i-003', got %d", len(result))
	}
	if len(result) > 0 && result[0].ID != "i-003" {
		t.Errorf("expected i-003, got %s", result[0].ID)
	}
}

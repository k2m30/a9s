package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestSfnColor_TriviallyHealthy(t *testing.T) {
	td := resource.FindResourceType("sfn")
	if td == nil {
		t.Fatal("sfn not registered")
	}

	cases := []map[string]string{
		nil,
		{"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:my-sm"},
		{"type": "STANDARD", "name": "order-processor"},
		{"type": "EXPRESS", "name": "fast-handler", "creation_date": "2024-01-15T10:00:00Z"},
	}

	for i, fields := range cases {
		got := td.Color(resource.Resource{Fields: fields})
		if got != resource.ColorHealthy {
			t.Errorf("case %d: Color = %v, want ColorHealthy (Wave 1 None per doc; Wave 2 enricher carries findings)", i, got)
		}
	}
}

package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestGlueColor_TriviallyHealthy verifies that Glue Jobs always return ColorHealthy
// regardless of fields. Per docs/attention-signals.md, GetJobs returns definitions
// only (Wave 1 is None); run-state signals require Wave 2 (GetJobRuns per job).
func TestGlueColor_TriviallyHealthy(t *testing.T) {
	td := resource.FindResourceType("glue")
	if td == nil {
		t.Fatal("glue not registered")
	}

	cases := []map[string]string{
		nil,
		{"job_name": "my-etl-job"},
		{"job_name": "transform-orders", "glue_version": "4.0", "worker_type": "G.1X", "num_workers": "10"},
		{"job_name": "archive-logs", "last_modified": "2024-01-15 09:00"},
		{},
	}

	for i, fields := range cases {
		got := td.Color(resource.Resource{Fields: fields})
		if got != resource.ColorHealthy {
			t.Errorf("case %d: Color = %v, want ColorHealthy (Wave 1 None per attention-signals.md; run-state requires Wave 2 GetJobRuns)", i, got)
		}
	}
}

package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestLambdaColor_StateAndOverrides(t *testing.T) {
	td := resource.FindResourceType("lambda")
	if td == nil {
		t.Fatal("lambda not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "state=Active",
			fields: map[string]string{"state": "Active", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=Pending",
			fields: map[string]string{"state": "Pending", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			want:   resource.ColorWarning,
		},
		{
			name:   "state=Inactive",
			fields: map[string]string{"state": "Inactive", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			want:   resource.ColorDim,
		},
		{
			name:   "state=Failed",
			fields: map[string]string{"state": "Failed", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state=Active+last_update_status=Failed",
			fields: map[string]string{"state": "Active", "last_update_status": "Failed", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state=Active+deprecated_runtime=python3.7",
			fields: map[string]string{"state": "Active", "runtime": "python3.7", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			want:   resource.ColorBroken,
		},
		{
			name:   "state=Active+current_runtime=python3.12",
			fields: map[string]string{"state": "Active", "runtime": "python3.12", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=Active+no_dlq",
			fields: map[string]string{"state": "Active"},
			want:   resource.ColorWarning,
		},
		{
			name:   "state=Active+dlq_present",
			fields: map[string]string{"state": "Active", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-dlq"},
			want:   resource.ColorHealthy,
		},
		{
			name: "all_signals_broken_wins_over_warning",
			fields: map[string]string{
				"state":              "Active",
				"last_update_status": "Failed",
				"runtime":            "python3.7",
				"dlq_target_arn":     "",
			},
			want: resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != tc.want {
				t.Errorf("Color(%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}

package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestTrailColor(t *testing.T) {
	td := resource.FindResourceType("trail")
	if td == nil {
		t.Fatal("trail not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "logging_no_errors",
			fields: map[string]string{"is_logging": "true", "latest_delivery_error": "-"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "not_logging",
			fields: map[string]string{"is_logging": "false"},
			want:   resource.ColorBroken,
		},
		{
			name:   "delivery_error",
			fields: map[string]string{"is_logging": "true", "latest_delivery_error": "AccessDenied to bucket xyz"},
			want:   resource.ColorBroken,
		},
		{
			name:   "status_failed",
			fields: map[string]string{"status": "failed"},
			want:   resource.ColorBroken,
		},
		{
			name: "log_validation_disabled",
			fields: map[string]string{
				"is_logging":                  "true",
				"latest_delivery_error":       "-",
				"log_file_validation_enabled": "false",
			},
			want: resource.ColorWarning,
		},
		{
			name: "log_validation_enabled",
			fields: map[string]string{
				"is_logging":                  "true",
				"latest_delivery_error":       "-",
				"log_file_validation_enabled": "true",
			},
			want: resource.ColorHealthy,
		},
		{
			name: "broken_overrides_warning",
			fields: map[string]string{
				"is_logging":                  "false",
				"log_file_validation_enabled": "false",
			},
			want: resource.ColorBroken,
		},
		{
			name:   "empty",
			fields: map[string]string{},
			want:   resource.ColorHealthy,
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

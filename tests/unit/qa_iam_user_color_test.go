package unit

// qa_iam_user_color_test.go — Behavioral tests for the iam-user Color function.
//
// Contract assertions:
//   - No console access (has_console_password=false) → ColorHealthy.
//   - Console access with recent password use (≤90d) → ColorHealthy.
//   - Console access with dormant password use (>90d) → ColorWarning.
//   - Empty fields (no has_console_password key) → ColorHealthy.

import (
	"fmt"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestIamUserColor(t *testing.T) {
	td := resource.FindResourceType("iam-user")
	if td == nil {
		t.Fatal("iam-user not registered")
	}

	now := time.Now()
	recentDate := now.Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04")
	dormantDate := now.Add(-100 * 24 * time.Hour).Format("2006-01-02 15:04")

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "no_console",
			fields: map[string]string{"has_console_password": "false"},
			want:   resource.ColorHealthy,
		},
		{
			name: "recent_password",
			fields: map[string]string{
				"has_console_password": "true",
				"password_last_used":   recentDate,
			},
			want: resource.ColorHealthy,
		},
		{
			name: "dormant",
			fields: map[string]string{
				"has_console_password": "true",
				"password_last_used":   dormantDate,
			},
			want: resource.ColorWarning,
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
				t.Errorf("Color(%v) = %v, want %v", formatFields(tc.fields), got, tc.want)
			}
		})
	}
}

// formatFields renders a map[string]string for error messages.
func formatFields(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	s := "{"
	for k, v := range m {
		s += fmt.Sprintf("%s=%q ", k, v)
	}
	return s[:len(s)-1] + "}"
}

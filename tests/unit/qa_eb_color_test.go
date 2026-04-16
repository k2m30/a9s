package unit

// qa_eb_color_test.go — Regression tests for Elastic Beanstalk Color mapping.
//
// EB Color uses "health" field first (Red/Yellow/Grey/Green), falling back to
// "status" field. Tests pin each branch so regressions are caught.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func ebResource(health, status string) resource.Resource {
	return resource.Resource{
		ID:     "arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/MyApp/MyEnv",
		Name:   "MyEnv",
		Fields: map[string]string{"health": health, "status": status},
	}
}

// TestEBColor_HealthRed_IsColorBroken verifies health=Red → ColorBroken.
func TestEBColor_HealthRed_IsColorBroken(t *testing.T) {
	td := resource.FindResourceType("eb")
	if got := td.Color(ebResource("Red", "Ready")); got != resource.ColorBroken {
		t.Errorf("eb Color health=Red = %v, want ColorBroken", got)
	}
}

// TestEBColor_HealthYellow_IsColorWarning verifies health=Yellow → ColorWarning.
func TestEBColor_HealthYellow_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("eb")
	if got := td.Color(ebResource("Yellow", "Ready")); got != resource.ColorWarning {
		t.Errorf("eb Color health=Yellow = %v, want ColorWarning", got)
	}
}

// TestEBColor_HealthGrey_IsColorWarning verifies health=Grey → ColorWarning.
// Grey means health unknown/degraded — an attention signal, not just dimmed.
func TestEBColor_HealthGrey_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("eb")
	if got := td.Color(ebResource("Grey", "Ready")); got != resource.ColorWarning {
		t.Errorf("eb Color health=Grey = %v, want ColorWarning", got)
	}
}

// TestEBColor_HealthGreen_IsColorHealthy verifies health=Green → ColorHealthy.
func TestEBColor_HealthGreen_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("eb")
	if got := td.Color(ebResource("Green", "Ready")); got != resource.ColorHealthy {
		t.Errorf("eb Color health=Green = %v, want ColorHealthy", got)
	}
}

// TestEBColor_HealthTakesPrecedenceOverStatus verifies health field wins over status.
// health=Red with status=Ready must produce ColorBroken (not ColorHealthy from status).
func TestEBColor_HealthTakesPrecedenceOverStatus(t *testing.T) {
	td := resource.FindResourceType("eb")
	r := ebResource("Red", "Ready") // health says broken, status says healthy
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("eb Color health=Red/status=Ready = %v, want ColorBroken (health takes precedence)", got)
	}
}

// TestEBColor_NoHealth_StatusReady_IsColorHealthy verifies status=Ready → ColorHealthy
// when health is not set.
func TestEBColor_NoHealth_StatusReady_IsColorHealthy(t *testing.T) {
	td := resource.FindResourceType("eb")
	r := ebResource("", "Ready")
	if got := td.Color(r); got != resource.ColorHealthy {
		t.Errorf("eb Color health=''/status=Ready = %v, want ColorHealthy", got)
	}
}

// TestEBColor_NoHealth_StatusLaunching_IsColorWarning verifies status=Launching → ColorWarning
// when health is not set.
func TestEBColor_NoHealth_StatusLaunching_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("eb")
	r := ebResource("", "Launching")
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("eb Color health=''/status=Launching = %v, want ColorWarning", got)
	}
}

// TestEBColor_NoHealth_StatusUpdating_IsColorWarning verifies status=Updating → ColorWarning
// when health is not set.
func TestEBColor_NoHealth_StatusUpdating_IsColorWarning(t *testing.T) {
	td := resource.FindResourceType("eb")
	r := ebResource("", "Updating")
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("eb Color health=''/status=Updating = %v, want ColorWarning", got)
	}
}

// TestEBColor_NoHealth_StatusTerminating_IsColorDim verifies status=Terminating → ColorDim
// when health is not set.
func TestEBColor_NoHealth_StatusTerminating_IsColorDim(t *testing.T) {
	td := resource.FindResourceType("eb")
	r := ebResource("", "Terminating")
	if got := td.Color(r); got != resource.ColorDim {
		t.Errorf("eb Color health=''/status=Terminating = %v, want ColorDim", got)
	}
}

// TestEBColor_NoHealth_StatusTerminated_IsColorDim verifies status=Terminated → ColorDim
// when health is not set.
func TestEBColor_NoHealth_StatusTerminated_IsColorDim(t *testing.T) {
	td := resource.FindResourceType("eb")
	r := ebResource("", "Terminated")
	if got := td.Color(r); got != resource.ColorDim {
		t.Errorf("eb Color health=''/status=Terminated = %v, want ColorDim", got)
	}
}

// TestEbColor covers additional table-driven cases including precedence with Terminated status
// and empty-fields fallback.
func TestEbColor(t *testing.T) {
	td := resource.FindResourceType("eb")
	if td == nil {
		t.Fatal("eb not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name:   "terminated_red_priority",
			fields: map[string]string{"health": "Red", "status": "Terminated"},
			want:   resource.ColorBroken,
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
				t.Errorf("Color(fields=%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}

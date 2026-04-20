package unit

import (
	"testing"

	demo "github.com/k2m30/a9s/v3/internal/demo"
)

func TestDemoConstants(t *testing.T) {
	if demo.DemoProfile != "demo" {
		t.Errorf("DemoProfile: got %q, want %q", demo.DemoProfile, "demo")
	}
	if demo.DemoRegion != "us-east-1" {
		t.Errorf("DemoRegion: got %q, want %q", demo.DemoRegion, "us-east-1")
	}
}

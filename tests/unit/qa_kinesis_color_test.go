package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestKinesisColor(t *testing.T) {
	td := resource.FindResourceType("kinesis")
	if td == nil {
		t.Fatal("kinesis not registered")
	}

	streamStatusCases := []struct {
		name         string
		streamStatus string
		want         resource.Color
	}{
		{name: "active", streamStatus: "ACTIVE", want: resource.ColorHealthy},
		{name: "creating", streamStatus: "CREATING", want: resource.ColorWarning},
		{name: "updating", streamStatus: "UPDATING", want: resource.ColorWarning},
		{name: "deleting", streamStatus: "DELETING", want: resource.ColorWarning},
		{name: "empty", streamStatus: "", want: resource.ColorHealthy},
	}

	for _, tc := range streamStatusCases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: map[string]string{"stream_status": tc.streamStatus}})
			if got != tc.want {
				t.Errorf("Color(stream_status=%q) = %v, want %v", tc.streamStatus, got, tc.want)
			}
		})
	}

	// Fallback: when stream_status is absent, Color should read Fields["status"].
	t.Run("status_active_fallback", func(t *testing.T) {
		got := td.Color(resource.Resource{Fields: map[string]string{"status": "ACTIVE"}})
		if got != resource.ColorHealthy {
			t.Errorf("Color(status=%q) = %v, want %v", "ACTIVE", got, resource.ColorHealthy)
		}
	})
}

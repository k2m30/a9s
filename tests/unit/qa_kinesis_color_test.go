package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
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

	// Fields["status"] holds the rendered phrase, which is lowercase, so the
	// switch that used to read it as a raw enum could never match and is gone.
	// The classifier follows the enum: a stream whose phrase disagrees with its
	// stream_status is coloured by the status.
	t.Run("rendered_phrase_is_not_an_input", func(t *testing.T) {
		got := td.Color(resource.Resource{Fields: map[string]string{
			"stream_status": "ACTIVE",
			"status":        "creating",
		}})
		if got != resource.ColorHealthy {
			t.Errorf("active stream whose phrase says creating = %v, want %v", got, resource.ColorHealthy)
		}
	})
}

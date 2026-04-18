package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestSnsSubColor(t *testing.T) {
	td := resource.FindResourceType("sns-sub")
	if td == nil {
		t.Fatal("sns-sub not registered")
	}

	cases := []struct {
		name            string
		subscriptionARN string
		want            resource.Color
	}{
		{
			name:            "pending_confirmation",
			subscriptionARN: "PendingConfirmation",
			want:            resource.ColorWarning,
		},
		{
			name:            "deleted",
			subscriptionARN: "Deleted",
			want:            resource.ColorDim,
		},
		{
			name:            "confirmed",
			subscriptionARN: "arn:aws:sns:us-east-1:111122223333:topic:abc-123",
			want:            resource.ColorHealthy,
		},
		{
			name:            "empty",
			subscriptionARN: "",
			want:            resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: map[string]string{
				"subscription_arn": tc.subscriptionARN,
			}})
			if got != tc.want {
				t.Errorf("Color(subscription_arn=%q) = %v, want %v", tc.subscriptionARN, got, tc.want)
			}
		})
	}
}

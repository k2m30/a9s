package aws

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{1536, "1.5 KB"},
		{10240, "10 KB"},
		{1048576, "1 MB"},
		{1572864, "1.5 MB"},
		{1073741824, "1 GB"},
		{1610612736, "1.5 GB"},
		{1099511627776, "1 TB"},
		{1649267441664, "1.5 TB"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatBytes(tc.input)
			if got != tc.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{1.0, "1"},
		{1.5, "1.5"},
		{2.0, "2"},
		{10.0, "10"},
		{3.7, "3.7"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := formatFloat(tc.input)
			if got != tc.want {
				t.Errorf("formatFloat(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

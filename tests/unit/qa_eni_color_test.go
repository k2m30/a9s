package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEniColor(t *testing.T) {
	td := resource.FindResourceType("eni")
	if td == nil {
		t.Fatal("eni not registered")
	}

	cases := []struct {
		name   string
		status string
		typ    string
		want   resource.Color
	}{
		{name: "in_use", status: "in-use", typ: "interface", want: resource.ColorHealthy},
		{name: "orphan_interface", status: "available", typ: "interface", want: resource.ColorWarning},
		{name: "aws_managed_idle", status: "available", typ: "requester-managed", want: resource.ColorHealthy},
		{name: "attaching", status: "attaching", typ: "interface", want: resource.ColorWarning},
		{name: "detaching", status: "detaching", typ: "interface", want: resource.ColorWarning},
		{name: "unknown", status: "", typ: "", want: resource.ColorHealthy},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := resource.Resource{Fields: map[string]string{
				"status": tc.status,
				"type":   tc.typ,
			}}
			got := td.Color(r)
			if got != tc.want {
				t.Errorf("Color(status=%q, type=%q) = %v, want %v", tc.status, tc.typ, got, tc.want)
			}
		})
	}
}

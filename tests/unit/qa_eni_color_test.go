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
		name              string
		status            string
		typ               string
		requesterManaged  string
		want              resource.Color
	}{
		{name: "in_use", status: "in-use", typ: "interface", want: resource.ColorHealthy},
		{name: "orphan_interface", status: "available", typ: "interface", want: resource.ColorWarning},
		// Post-cleanup: "requester-managed" is NOT a valid NetworkInterfaceType value.
		// The correct mechanism is the RequesterManaged boolean field (Fields["requester_managed"]="true").
		// AWS-managed ENIs (VPC endpoints, ELB NICs) that are "available" are exempt from the
		// cost-waste warning — they are controlled by AWS services, not idle customer resources.
		{name: "aws_managed_idle", status: "available", typ: "interface", requesterManaged: "true", want: resource.ColorHealthy},
		{name: "attaching", status: "attaching", typ: "interface", want: resource.ColorWarning},
		{name: "detaching", status: "detaching", typ: "interface", want: resource.ColorWarning},
		{name: "unknown", status: "", typ: "", want: resource.ColorHealthy},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := resource.Resource{Fields: map[string]string{
				"status":            tc.status,
				"type":              tc.typ,
				"requester_managed": tc.requesterManaged,
			}}
			got := td.Color(r)
			if got != tc.want {
				t.Errorf("Color(status=%q, type=%q, requester_managed=%q) = %v, want %v", tc.status, tc.typ, tc.requesterManaged, got, tc.want)
			}
		})
	}
}

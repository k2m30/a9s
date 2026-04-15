package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestTgColor_TriviallyHealthy(t *testing.T) {
	td := resource.FindResourceType("tg")
	if td == nil {
		t.Fatal("tg not registered")
	}

	cases := []map[string]string{
		nil,
		{"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abc123"},
		{"protocol": "HTTP", "vpc_id": "vpc-0a1b2c3d4e5f"},
		{"target_group_name": "my-tg", "port": "443", "protocol": "HTTPS", "target_type": "instance"},
	}

	for i, fields := range cases {
		got := td.Color(resource.Resource{Fields: fields})
		if got != resource.ColorHealthy {
			t.Errorf("case %d: Color = %v, want ColorHealthy (Wave 1 None per doc; Wave 2 enricher carries findings)", i, got)
		}
	}
}

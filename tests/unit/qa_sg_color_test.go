package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// TestSgColor pins that a security group's row colour is derived from the
// findings the fetcher emitted, never re-derived from the raw
// dangerous_open_count / wide_open fields. Those fields still exist for the
// list's Risk column and for ec2's exposure composite, but a second
// classifier reading them would be a second source of truth that can disagree
// with the Attention block the operator opens next.
//
// The cases therefore go in as security groups and come out through the real
// fetcher, exactly as they do in the app.
func TestSgColor(t *testing.T) {
	td := resource.FindResourceType("sg")
	if td == nil {
		t.Fatal("sg not registered")
	}

	allProtocolsOpen := ec2types.IpPermission{
		IpProtocol: aws.String("-1"),
		IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("0.0.0.0/0")}},
	}

	cases := []struct {
		name  string
		perms []ec2types.IpPermission
		want  resource.Color
	}{
		{name: "safe", perms: nil, want: resource.ColorHealthy},
		{
			name:  "ssh_open",
			perms: []ec2types.IpPermission{w3TCPFromInternet(22)},
			want:  resource.ColorBroken,
		},
		{
			name:  "db_open",
			perms: []ec2types.IpPermission{w3TCPFromInternet(3306), w3TCPFromInternet(5432)},
			want:  resource.ColorBroken,
		},
		{
			name:  "all_protocols_open",
			perms: []ec2types.IpPermission{allProtocolsOpen},
			want:  resource.ColorBroken,
		},
		{
			name:  "both",
			perms: []ec2types.IpPermission{allProtocolsOpen, w3TCPFromInternet(22)},
			want:  resource.ColorBroken,
		},
		{
			name: "internal_only",
			perms: []ec2types.IpPermission{{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpRanges:   []ec2types.IpRange{{CidrIp: aws.String("10.0.0.0/8")}},
			}},
			want: resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := w3FetchSGs(t, ec2types.SecurityGroup{
				GroupId:             aws.String("sg-0aa11bb22cc33dd44"),
				GroupName:           aws.String("acme-app"),
				VpcId:               aws.String("vpc-0abc123"),
				IpPermissions:       tc.perms,
				IpPermissionsEgress: []ec2types.IpPermission{w3DefaultEgress()},
			})
			if got := td.Color(rows[0]); got != tc.want {
				t.Errorf("Color = %v, want %v (findings=%+v, dangerous_open_count=%q, wide_open=%q)",
					got, tc.want, rows[0].Findings,
					rows[0].Fields["dangerous_open_count"], rows[0].Fields["wide_open"])
			}
		})
	}
}

// TestSgColor_RawFieldsAloneDoNotColour is the other half of the same
// contract: a row carrying the risk fields but no findings must stay healthy.
// If this ever goes red, a raw-field branch has been reintroduced into the sg
// classifier and the colour can now disagree with the Attention block.
func TestSgColor_RawFieldsAloneDoNotColour(t *testing.T) {
	td := resource.FindResourceType("sg")
	if td == nil {
		t.Fatal("sg not registered")
	}
	r := resource.Resource{Fields: map[string]string{
		"dangerous_open_count": "3",
		"wide_open":            "true",
	}}
	if got := td.Color(r); got != resource.ColorHealthy {
		t.Errorf("Color = %v for a row with risk fields but no findings, want ColorHealthy — the classifier must read findings only", got)
	}
}

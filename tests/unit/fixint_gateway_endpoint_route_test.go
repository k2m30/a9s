package unit

// A gateway endpoint (S3, DynamoDB) is routed by its prefix list
// (DestinationPrefixListId) with the vpce- ID in GatewayId; it carries no
// internet traffic. Beside an internet-gateway default route it changes
// nothing: the instance is exposed. The endpoint route sits first so a check
// that ignored the destination would stop on it.

import (
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestEC2Exposure_GatewayEndpointPrefixListRouteKeepsIGWVerdict(t *testing.T) {
	s3Endpoint := ec2types.Route{DestinationPrefixListId: aws.String("pl-63a5400a"), GatewayId: aws.String("vpce-0f1e2d3c4b5a69788"), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute}
	cases := []struct {
		name string
		igw  ec2types.Route
		inst ec2types.Instance
	}{
		{
			name: "IPv4",
			igw:  ec2types.Route{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String(t562IGW), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			inst: t562Instance("i-0gwep4igw000aaa1", t562SubnetA, "203.0.113.52", "", false, inspSSHv4SG),
		},
		{
			name: "IPv6",
			igw:  ec2types.Route{DestinationIpv6CidrBlock: aws.String("::/0"), GatewayId: aws.String(t562IGW), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			inst: t562IPv6Instance("i-0gwep6igw000aaa1", t562SubnetA, t562IPv6B, true, t562SSHv6SG),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, reason, marked := inspEnrich(t, inspRouteTable(t562RTBA, t562SubnetA, s3Endpoint, tc.igw), tc.inst)
			if marked {
				t.Errorf("marked not inspected (%q) by a gateway endpoint's prefix-list route", reason)
			}
			if !slices.Contains(findings, string(pw1EC2CodeInternetExposed)) && !slices.Contains(findings, string(t562ExpAllCd)) {
				t.Errorf("findings %v, want an internet-exposed verdict through the internet gateway", findings)
			}
		})
	}
}

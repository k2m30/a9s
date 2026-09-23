package unit

// An instance whose subnet sends its default route to a Gateway Load Balancer
// endpoint (a third-party appliance or an AWS Network Firewall endpoint) is
// reached from the internet only through that appliance, whose rules the
// security-group and route-table lists cannot read. Its exposure is not
// decided: the row reads not inspected, neither clean nor exposed.
//
// The application subnet's route table routes 0.0.0.0/0 and ::/0 to the
// endpoint by its VPC endpoint ID (create-route --vpc-endpoint-id vpce-…):
// docs.aws.amazon.com/elasticloadbalancing/latest/gateway/getting-started-cli.html#configure-routing-aws-cli,
// and a Network Firewall deployment points the customer subnet's
// internet-bound route at its firewall endpoint the same way:
// docs.aws.amazon.com/network-firewall/latest/developerguide/vpc-config-route-tables.html.
// DescribeRouteTables' Route (docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Route.html)
// has no VPC-endpoint field; the endpoint ID is reported in GatewayId.

import (
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	inspSSHv4SG = "sg-0ssh4open0aaaaaa1"
	inspGWLBE   = "vpce-0a1b2c3d4e5f60718"
	inspNAT     = "nat-0a1b2c3d4e5f60718"
)

func inspRouteTable(id, subnet string, routes ...ec2types.Route) ec2types.RouteTable {
	table := t562RouteTable(id, subnet, ec2types.Route{})
	table.Routes = append(table.Routes[:2], routes...)
	return table
}

func inspEnrich(t *testing.T, table ec2types.RouteTable, inst ec2types.Instance) (findings []string, reason string, marked bool) {
	t.Helper()
	rtb := t562RTBEntry(t, false, t562MainTable(), table)
	cache := pw1SGCache(t, pw1SG(inspSSHv4SG, 22, 22, false), t562SSHv6Only(t562SSHv6SG))
	cache["rtb"] = rtb
	res := t562EnrichEC2(t, cache, inst)
	id := aws.ToString(inst.InstanceId)
	for _, f := range res.Findings[id] {
		findings = append(findings, string(f.Code))
	}
	reason, marked = res.TruncatedIDs[id]
	return findings, reason, marked
}

func TestEC2Exposure_InspectionEndpointRouteReadsNotInspected(t *testing.T) {
	cases := []struct {
		name  string
		route ec2types.Route
		inst  ec2types.Instance
	}{
		{
			name:  "IPv4 default route to the endpoint",
			route: ec2types.Route{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String(inspGWLBE), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			inst:  t562Instance("i-0insp4gwlbe00aaa1", t562SubnetA, "203.0.113.50", "", false, inspSSHv4SG),
		},
		{
			name:  "IPv6 default route to the endpoint",
			route: ec2types.Route{DestinationIpv6CidrBlock: aws.String("::/0"), GatewayId: aws.String(inspGWLBE), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute},
			inst:  t562IPv6Instance("i-0insp6gwlbe00aaa1", t562SubnetA, t562IPv6B, true, t562SSHv6SG),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, _, marked := inspEnrich(t, inspRouteTable(t562RTBA, t562SubnetA, tc.route), tc.inst)
			if !marked {
				t.Errorf("%s: not marked not inspected; the row reads clean although its inbound path runs through an appliance whose rules were not read (findings %v)",
					aws.ToString(tc.inst.InstanceId), findings)
			}
			for _, code := range findings {
				if code == string(pw1EC2CodeInternetExposed) || code == string(t562ExpAllCd) {
					t.Errorf("%s: carries %s; an inspected path is not a decided exposure", aws.ToString(tc.inst.InstanceId), code)
				}
			}
		})
	}
}

// A NAT gateway admits nothing inbound: the same instance and group behind
// one reads clean, with no marker. Behind an internet gateway the same pair is
// exposed, so the table under test is the one the verdict reads.
func TestEC2Exposure_NATGatewayRouteStaysClean(t *testing.T) {
	inst := t562Instance("i-0inspnat000aaaa1", t562SubnetA, "203.0.113.51", "", false, inspSSHv4SG)
	route := ec2types.Route{DestinationCidrBlock: aws.String("0.0.0.0/0"), NatGatewayId: aws.String(inspNAT), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute}

	findings, reason, marked := inspEnrich(t, inspRouteTable(t562RTBA, t562SubnetA, route), inst)
	if marked {
		t.Errorf("marked not inspected (%q) behind a NAT gateway, whose verdict is decided", reason)
	}
	for _, code := range findings {
		if code == string(pw1EC2CodeInternetExposed) || code == string(t562ExpAllCd) {
			t.Errorf("carries %s behind a NAT gateway, which admits nothing inbound", code)
		}
	}

	igw := ec2types.Route{DestinationCidrBlock: aws.String("0.0.0.0/0"), GatewayId: aws.String(t562IGW), State: ec2types.RouteStateActive, Origin: ec2types.RouteOriginCreateRoute}
	findings, _, _ = inspEnrich(t, inspRouteTable(t562RTBA, t562SubnetA, igw), inst)
	if !slices.Contains(findings, string(pw1EC2CodeInternetExposed)) {
		t.Errorf("behind an internet gateway: findings %v, want %s", findings, pw1EC2CodeInternetExposed)
	}
}

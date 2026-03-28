package demo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

// registerNetworkHandlers registers all networking handlers (EC2 networking + ELBv2).
func registerNetworkHandlers(t *Transport) {
	registerEC2NetworkHandlers(t)
	registerELBv2Handlers(t)
}

// ---------------------------------------------------------------------------
// EC2 networking resources (ec2query XML, service "ec2")
// ---------------------------------------------------------------------------

func registerEC2NetworkHandlers(t *Transport) {
	// DescribeVpcs
	t.Handle("ec2", "DescribeVpcs", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["vpc"]()
		vpcs := ExtractSDK[ec2types.Vpc](resources)

		var items strings.Builder
		for _, v := range vpcs {
			vpcID := aws.ToString(v.VpcId)
			state := string(v.State)
			cidr := aws.ToString(v.CidrBlock)
			isDefault := "false"
			if v.IsDefault != nil && *v.IsDefault {
				isDefault = "true"
			}
			ownerID := aws.ToString(v.OwnerId)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<vpcId>%s</vpcId>`, xmlEscape(vpcID))
			fmt.Fprintf(&items, `<state>%s</state>`, xmlEscape(state))
			fmt.Fprintf(&items, `<cidrBlock>%s</cidrBlock>`, xmlEscape(cidr))
			fmt.Fprintf(&items, `<isDefault>%s</isDefault>`, isDefault)
			fmt.Fprintf(&items, `<ownerId>%s</ownerId>`, xmlEscape(ownerID))
			items.WriteString(buildTagSetXML(v.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeVpcs", "vpcSet", items.String())), nil
	})

	// DescribeSecurityGroups
	t.Handle("ec2", "DescribeSecurityGroups", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["sg"]()
		sgs := ExtractSDK[ec2types.SecurityGroup](resources)

		var items strings.Builder
		for _, sg := range sgs {
			groupID := aws.ToString(sg.GroupId)
			groupName := aws.ToString(sg.GroupName)
			vpcID := aws.ToString(sg.VpcId)
			desc := aws.ToString(sg.Description)
			ownerID := aws.ToString(sg.OwnerId)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<groupId>%s</groupId>`, xmlEscape(groupID))
			fmt.Fprintf(&items, `<groupName>%s</groupName>`, xmlEscape(groupName))
			fmt.Fprintf(&items, `<vpcId>%s</vpcId>`, xmlEscape(vpcID))
			fmt.Fprintf(&items, `<groupDescription>%s</groupDescription>`, xmlEscape(desc))
			fmt.Fprintf(&items, `<ownerId>%s</ownerId>`, xmlEscape(ownerID))
			items.WriteString(buildTagSetXML(sg.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeSecurityGroups", "securityGroupInfo", items.String())), nil
	})

	// DescribeSubnets
	t.Handle("ec2", "DescribeSubnets", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["subnet"]()
		subnets := ExtractSDK[ec2types.Subnet](resources)

		var items strings.Builder
		for _, s := range subnets {
			subnetID := aws.ToString(s.SubnetId)
			state := string(s.State)
			vpcID := aws.ToString(s.VpcId)
			cidr := aws.ToString(s.CidrBlock)
			az := aws.ToString(s.AvailabilityZone)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<subnetId>%s</subnetId>`, xmlEscape(subnetID))
			fmt.Fprintf(&items, `<state>%s</state>`, xmlEscape(state))
			fmt.Fprintf(&items, `<vpcId>%s</vpcId>`, xmlEscape(vpcID))
			fmt.Fprintf(&items, `<cidrBlock>%s</cidrBlock>`, xmlEscape(cidr))
			fmt.Fprintf(&items, `<availabilityZone>%s</availabilityZone>`, xmlEscape(az))
			items.WriteString(buildTagSetXML(s.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeSubnets", "subnetSet", items.String())), nil
	})

	// DescribeNatGateways
	t.Handle("ec2", "DescribeNatGateways", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["nat"]()
		nats := ExtractSDK[ec2types.NatGateway](resources)

		var items strings.Builder
		for _, n := range nats {
			natID := aws.ToString(n.NatGatewayId)
			state := string(n.State)
			subnetID := aws.ToString(n.SubnetId)
			vpcID := aws.ToString(n.VpcId)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<natGatewayId>%s</natGatewayId>`, xmlEscape(natID))
			fmt.Fprintf(&items, `<state>%s</state>`, xmlEscape(state))
			fmt.Fprintf(&items, `<subnetId>%s</subnetId>`, xmlEscape(subnetID))
			fmt.Fprintf(&items, `<vpcId>%s</vpcId>`, xmlEscape(vpcID))
			items.WriteString(buildTagSetXML(n.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeNatGateways", "natGatewaySet", items.String())), nil
	})

	// DescribeInternetGateways
	t.Handle("ec2", "DescribeInternetGateways", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["igw"]()
		igws := ExtractSDK[ec2types.InternetGateway](resources)

		var items strings.Builder
		for _, igw := range igws {
			igwID := aws.ToString(igw.InternetGatewayId)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<internetGatewayId>%s</internetGatewayId>`, xmlEscape(igwID))
			// Attachments
			items.WriteString(`<attachmentSet>`)
			for _, att := range igw.Attachments {
				fmt.Fprintf(&items, `<item><vpcId>%s</vpcId><state>%s</state></item>`,
					xmlEscape(aws.ToString(att.VpcId)), xmlEscape(string(att.State)))
			}
			items.WriteString(`</attachmentSet>`)
			items.WriteString(buildTagSetXML(igw.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeInternetGateways", "internetGatewaySet", items.String())), nil
	})

	// DescribeAddresses (EIPs)
	t.Handle("ec2", "DescribeAddresses", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["eip"]()
		eips := ExtractSDK[ec2types.Address](resources)

		var items strings.Builder
		for _, eip := range eips {
			publicIP := aws.ToString(eip.PublicIp)
			allocID := aws.ToString(eip.AllocationId)
			instanceID := aws.ToString(eip.InstanceId)
			assocID := aws.ToString(eip.AssociationId)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<publicIp>%s</publicIp>`, xmlEscape(publicIP))
			if allocID != "" {
				fmt.Fprintf(&items, `<allocationId>%s</allocationId>`, xmlEscape(allocID))
			}
			if instanceID != "" {
				fmt.Fprintf(&items, `<instanceId>%s</instanceId>`, xmlEscape(instanceID))
			}
			if assocID != "" {
				fmt.Fprintf(&items, `<associationId>%s</associationId>`, xmlEscape(assocID))
			}
			items.WriteString(buildTagSetXML(eip.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeAddresses", "addressesSet", items.String())), nil
	})

	// DescribeNetworkInterfaces (ENIs)
	t.Handle("ec2", "DescribeNetworkInterfaces", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["eni"]()
		enis := ExtractSDK[ec2types.NetworkInterface](resources)

		var items strings.Builder
		for _, eni := range enis {
			eniID := aws.ToString(eni.NetworkInterfaceId)
			subnetID := aws.ToString(eni.SubnetId)
			vpcID := aws.ToString(eni.VpcId)
			status := string(eni.Status)
			desc := aws.ToString(eni.Description)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<networkInterfaceId>%s</networkInterfaceId>`, xmlEscape(eniID))
			fmt.Fprintf(&items, `<subnetId>%s</subnetId>`, xmlEscape(subnetID))
			fmt.Fprintf(&items, `<vpcId>%s</vpcId>`, xmlEscape(vpcID))
			fmt.Fprintf(&items, `<status>%s</status>`, xmlEscape(status))
			fmt.Fprintf(&items, `<description>%s</description>`, xmlEscape(desc))
			items.WriteString(buildTagSetXML(eni.TagSet))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeNetworkInterfaces", "networkInterfaceSet", items.String())), nil
	})

	// DescribeRouteTables
	t.Handle("ec2", "DescribeRouteTables", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["rtb"]()
		rtbs := ExtractSDK[ec2types.RouteTable](resources)

		var items strings.Builder
		for _, rtb := range rtbs {
			rtbID := aws.ToString(rtb.RouteTableId)
			vpcID := aws.ToString(rtb.VpcId)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<routeTableId>%s</routeTableId>`, xmlEscape(rtbID))
			fmt.Fprintf(&items, `<vpcId>%s</vpcId>`, xmlEscape(vpcID))
			items.WriteString(buildTagSetXML(rtb.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeRouteTables", "routeTableSet", items.String())), nil
	})

	// DescribeTransitGateways
	t.Handle("ec2", "DescribeTransitGateways", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["tgw"]()
		tgws := ExtractSDK[ec2types.TransitGateway](resources)

		var items strings.Builder
		for _, tgw := range tgws {
			tgwID := aws.ToString(tgw.TransitGatewayId)
			state := string(tgw.State)
			ownerID := aws.ToString(tgw.OwnerId)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<transitGatewayId>%s</transitGatewayId>`, xmlEscape(tgwID))
			fmt.Fprintf(&items, `<state>%s</state>`, xmlEscape(state))
			fmt.Fprintf(&items, `<ownerId>%s</ownerId>`, xmlEscape(ownerID))
			items.WriteString(buildTagSetXML(tgw.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeTransitGateways", "transitGatewaySet", items.String())), nil
	})

	// DescribeVpcEndpoints
	t.Handle("ec2", "DescribeVpcEndpoints", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["vpce"]()
		endpoints := ExtractSDK[ec2types.VpcEndpoint](resources)

		var items strings.Builder
		for _, ep := range endpoints {
			epID := aws.ToString(ep.VpcEndpointId)
			vpcID := aws.ToString(ep.VpcId)
			svcName := aws.ToString(ep.ServiceName)
			state := string(ep.State)

			fmt.Fprintf(&items, `<item>`)
			fmt.Fprintf(&items, `<vpcEndpointId>%s</vpcEndpointId>`, xmlEscape(epID))
			fmt.Fprintf(&items, `<vpcId>%s</vpcId>`, xmlEscape(vpcID))
			fmt.Fprintf(&items, `<serviceName>%s</serviceName>`, xmlEscape(svcName))
			fmt.Fprintf(&items, `<state>%s</state>`, xmlEscape(state))
			items.WriteString(buildTagSetXML(ep.Tags))
			fmt.Fprintf(&items, `</item>`)
		}

		return XMLResponse(ec2QueryXML("DescribeVpcEndpoints", "vpcEndpointSet", items.String())), nil
	})
}

// buildTagSetXML renders a tagSet XML element from a slice of ec2types.Tag.
func buildTagSetXML(tags []ec2types.Tag) string {
	if len(tags) == 0 {
		return `<tagSet/>`
	}
	var sb strings.Builder
	sb.WriteString(`<tagSet>`)
	for _, tag := range tags {
		fmt.Fprintf(&sb, `<item><key>%s</key><value>%s</value></item>`,
			xmlEscape(aws.ToString(tag.Key)),
			xmlEscape(aws.ToString(tag.Value)),
		)
	}
	sb.WriteString(`</tagSet>`)
	return sb.String()
}

// ---------------------------------------------------------------------------
// ELBv2 (awsquery XML, service "elasticloadbalancing")
// ---------------------------------------------------------------------------

func registerELBv2Handlers(t *Transport) {
	t.Handle("elasticloadbalancing", "DescribeLoadBalancers", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["elb"]()
		elbs := ExtractSDK[elbv2types.LoadBalancer](resources)

		var sb strings.Builder
		sb.WriteString(`<LoadBalancers>`)
		for _, elb := range elbs {
			name := aws.ToString(elb.LoadBalancerName)
			arn := aws.ToString(elb.LoadBalancerArn)
			dns := aws.ToString(elb.DNSName)
			lbType := string(elb.Type)
			scheme := string(elb.Scheme)
			state := ""
			if elb.State != nil {
				state = string(elb.State.Code)
			}
			vpcID := aws.ToString(elb.VpcId)

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<LoadBalancerName>%s</LoadBalancerName>`, xmlEscape(name))
			fmt.Fprintf(&sb, `<LoadBalancerArn>%s</LoadBalancerArn>`, xmlEscape(arn))
			fmt.Fprintf(&sb, `<DNSName>%s</DNSName>`, xmlEscape(dns))
			fmt.Fprintf(&sb, `<Type>%s</Type>`, xmlEscape(lbType))
			fmt.Fprintf(&sb, `<Scheme>%s</Scheme>`, xmlEscape(scheme))
			fmt.Fprintf(&sb, `<State><Code>%s</Code></State>`, xmlEscape(state))
			fmt.Fprintf(&sb, `<VpcId>%s</VpcId>`, xmlEscape(vpcID))
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</LoadBalancers>`)

		body := awsQueryXML("DescribeLoadBalancers", "https://elasticloadbalancing.amazonaws.com/doc/2015-12-01/", sb.String())
		return XMLResponse(body), nil
	})

	t.Handle("elasticloadbalancing", "DescribeTargetGroups", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["tg"]()
		tgs := ExtractSDK[elbv2types.TargetGroup](resources)

		var sb strings.Builder
		sb.WriteString(`<TargetGroups>`)
		for _, tg := range tgs {
			name := aws.ToString(tg.TargetGroupName)
			arn := aws.ToString(tg.TargetGroupArn)
			protocol := string(tg.Protocol)
			port := int32(0)
			if tg.Port != nil {
				port = *tg.Port
			}
			vpcID := aws.ToString(tg.VpcId)
			targetType := string(tg.TargetType)

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<TargetGroupName>%s</TargetGroupName>`, xmlEscape(name))
			fmt.Fprintf(&sb, `<TargetGroupArn>%s</TargetGroupArn>`, xmlEscape(arn))
			fmt.Fprintf(&sb, `<Protocol>%s</Protocol>`, xmlEscape(protocol))
			fmt.Fprintf(&sb, `<Port>%d</Port>`, port)
			fmt.Fprintf(&sb, `<VpcId>%s</VpcId>`, xmlEscape(vpcID))
			fmt.Fprintf(&sb, `<TargetType>%s</TargetType>`, xmlEscape(targetType))
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</TargetGroups>`)

		body := awsQueryXML("DescribeTargetGroups", "https://elasticloadbalancing.amazonaws.com/doc/2015-12-01/", sb.String())
		return XMLResponse(body), nil
	})
}

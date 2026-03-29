# VPCs (vpc) — Related Resources

## Real-World Use Cases

**1. "What's running in this VPC?"** During infrastructure inventory or incident response, you need a complete picture: how many EC2 instances, RDS databases, ELBs, and other resources exist in this VPC? The VPC's own API response has none of this — just CIDR blocks and DHCP options.

**2. "Is this VPC connected to other VPCs or on-prem?"** You need to find VPC peering connections, Transit Gateway attachments, and VPN connections. None of these appear in the VPC's DescribeVpcs response.

**3. "Can I delete this VPC?"** Before deletion, you need to verify nothing is running in it — no instances, no ELBs, no ENIs. The VPC API doesn't tell you this; you must check each resource type that lives in VPCs.

**4. "Which subnets have available IPs?"** During capacity planning or incident ("pods can't get IPs"), you need to see subnets and their available IP counts.

## Reverse Relationships

VPCs are the foundational container for almost all network-attached resources. The reverse relationship list is enormous.

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| EC2 Instances (ec2) | `ec2:DescribeInstances` with `Filters=[{Name=vpc-id, Values=[vpc-xxx]}]`. | "What instances are running in this VPC?" Inventory and blast radius assessment. | P0 |
| Subnets (subnet) | `ec2:DescribeSubnets` with `Filters=[{Name=vpc-id, Values=[vpc-xxx]}]`. | "What's the network layout? Which subnets are public vs private?" Foundation for all network troubleshooting. | P0 |
| Security Groups (sg) | `ec2:DescribeSecurityGroups` with `Filters=[{Name=vpc-id, Values=[vpc-xxx]}]`. | "What SGs exist in this VPC?" Security audit. | P0 |
| Load Balancers (elb) | `elbv2:DescribeLoadBalancers` and filter by `VpcId`. If a9s has ELB data cached, filter in-memory. | "Which ELBs route traffic in this VPC?" | P1 |
| RDS Instances (dbi) | RDS instances are in VPCs via subnet groups. `rds:DescribeDBInstances` and match on `DBSubnetGroup.VpcId`. If a9s has RDS data cached, filter in-memory. | "Which databases are in this VPC?" | P1 |
| Lambda Functions (lambda) | `lambda:ListFunctions` and filter by `VpcConfig.VpcId`. If a9s has Lambda data cached, filter in-memory. | "Which Lambdas run in this VPC?" VPC Lambdas consume ENIs and IPs. | P1 |
| EKS Clusters (eks) | Search EKS clusters where `resourcesVpcConfig.vpcId` matches. If a9s has EKS data cached, filter in-memory. | "Which EKS clusters use this VPC?" | P1 |
| NAT Gateways (nat) | `ec2:DescribeNatGateways` with `Filters=[{Name=vpc-id, Values=[vpc-xxx]}]`. | "How does private subnet traffic egress?" Cost-relevant — NAT GW is expensive. | P1 |
| Internet Gateways (igw) | `ec2:DescribeInternetGateways` with `Filters=[{Name=attachment.vpc-id, Values=[vpc-xxx]}]`. | "Does this VPC have internet access?" | P1 |
| Route Tables (rtb) | `ec2:DescribeRouteTables` with `Filters=[{Name=vpc-id, Values=[vpc-xxx]}]`. | "How is traffic routed?" | P1 |
| VPC Endpoints (vpce) | `ec2:DescribeVpcEndpoints` with `Filters=[{Name=vpc-id, Values=[vpc-xxx]}]`. | "Which AWS services have private endpoints in this VPC?" | P2 |
| Transit Gateway Attachments (tgw) | `ec2:DescribeTransitGatewayAttachments` with `Filters=[{Name=resource-id, Values=[vpc-xxx]}]`. | "Is this VPC connected to a Transit Gateway?" Cross-VPC and hybrid connectivity. | P1 |
| VPC Peering Connections (not in a9s) | `ec2:DescribeVpcPeeringConnections` with filter `requester-vpc-info.vpc-id` OR `accepter-vpc-info.vpc-id`. | "Is this VPC peered with other VPCs?" | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this VPC?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| VPC Flow Logs (not in a9s) | `ec2:DescribeFlowLogs` with `Filters=[{Name=resource-id, Values=[vpc-xxx]}]`. Returns flow log configuration including the destination (CloudWatch Log Group or S3 bucket). | "Are flow logs enabled? Where do they go?" Security audit — flow logs should be enabled on production VPCs. | P1 |
| DHCP Options Set (not in a9s) | VPC response has `DhcpOptionsId` — FORWARD. `ec2:DescribeDhcpOptions` shows DNS servers and domain name. | "What DNS settings does this VPC use?" Debugging DNS resolution issues. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteVpc | "Who deleted this VPC?" VPC deletion requires all resources to be removed first — this is a deliberate decommission. |
| ModifyVpcAttribute | "Who changed enableDnsSupport or enableDnsHostnames?" Disabling DNS support breaks service discovery and private hosted zone resolution. |
| CreateVpc | "Who created this VPC and with what CIDR?" New VPCs should follow the IP addressing plan — overlapping CIDRs break peering and TGW. |

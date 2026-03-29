# Transit Gateways (tgw) — Related Resources

## Real-World Use Cases

**1. "Which VPCs are connected through this Transit Gateway?"** TGW is the hub in a hub-and-spoke network topology. You need to see all VPC attachments to understand the network graph.

**2. "Why can't VPC A talk to VPC B through the TGW?"** Check TGW route tables for routes between the VPCs' CIDRs. Missing or blackhole routes are the most common cause.

**3. "Is on-prem connected?"** Check for VPN or Direct Connect Gateway attachments on the TGW.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| VPCs (vpc) | `ec2:DescribeTransitGatewayAttachments` with `Filters=[{Name=transit-gateway-id, Values=[tgw-xxx]}, {Name=resource-type, Values=[vpc]}]`. Returns VPC attachment details. | "Which VPCs are connected?" The primary question for TGWs. | P0 |
| Route Tables (rtb) | Routes in VPC route tables may target this TGW. `ec2:DescribeRouteTables` with `Filters=[{Name=route.transit-gateway-id, Values=[tgw-xxx]}]`. | "Which VPC route tables send traffic through this TGW?" | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| TGW Route Tables (not in a9s) | `ec2:DescribeTransitGatewayRouteTables` with `Filters=[{Name=transit-gateway-id, Values=[tgw-xxx]}]`. Then `ec2:SearchTransitGatewayRoutes` for each. | "How is cross-VPC traffic routed?" Route tables determine which VPCs can communicate through the TGW. | P0 |
| TGW Attachments (not in a9s) | `ec2:DescribeTransitGatewayAttachments` with TGW filter. Shows VPC, VPN, Direct Connect, and peering attachments with their state. | "What's connected to this TGW?" The complete picture of all network connections. | P0 |
| VPN Connections (not in a9s) | `ec2:DescribeTransitGatewayAttachments` filtered on `resource-type=vpn`. | "Is on-prem connected through this TGW?" | P1 |
| Direct Connect Gateway (not in a9s) | `ec2:DescribeTransitGatewayAttachments` filtered on `resource-type=direct-connect-gateway`. | "Is there a dedicated network connection?" | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteTransitGateway | "Who deleted the TGW?" Breaks all cross-VPC and hybrid connectivity. |
| CreateTransitGatewayVpcAttachment / DeleteTransitGatewayVpcAttachment | "Who connected or disconnected a VPC?" Network topology change. |
| CreateTransitGatewayRoute / DeleteTransitGatewayRoute | "Who changed the routing?" Routing changes in TGW affect cross-VPC and on-prem reachability. |

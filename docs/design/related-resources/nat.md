# NAT Gateways (nat) — Related Resources

## Real-World Use Cases

**1. "Which subnets lose internet if this NAT GW fails?"** NAT GWs are single-AZ. If one fails, all private subnets routing through it lose internet access. You need to find which route tables point to this NAT GW, then which subnets use those route tables.

**2. "Why is our NAT GW bill so high?"** NAT GW charges per GB processed. You need CloudWatch metrics (BytesOutToDestination, BytesOutToSource) to understand traffic volume, and you need to find which instances/services in the routed subnets generate the most traffic.

**3. "Private subnet instances can't reach the internet — is the NAT GW healthy?"** Check the NAT GW state, then verify the route table has the correct route to this NAT GW.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Route Tables (rtb) | `ec2:DescribeRouteTables` with `Filters=[{Name=route.nat-gateway-id, Values=[nat-xxx]}]`. Returns all route tables that have routes pointing to this NAT GW. | "Which route tables depend on this NAT GW?" This is the blast radius — every subnet associated with these route tables will lose internet if the NAT GW fails. | P0 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Subnets (subnet) — dependent | Multi-hop: Route tables with routes to this NAT GW → each route table's `Associations[].SubnetId` → those subnets. Shows which subnets lose internet if this NAT GW fails. | "What's the blast radius of a NAT GW failure?" Critical for understanding AZ redundancy. | P0 |
| Subnet (subnet) — location | NAT response has `SubnetId` — FORWARD. The NAT GW's own subnet must be a public subnet (with an IGW route). | "Which subnet hosts this NAT GW?" NAT GWs must be in public subnets. | P1 |
| Elastic IP (eip) | NAT response has `NatGatewayAddresses[].AllocationId` — FORWARD. The EIP is the NAT GW's public IP. | "What's the public IP?" Useful for whitelisting with external services. | P1 |
| VPC (vpc) | NAT response has `VpcId` — FORWARD. | "Which VPC is this NAT GW in?" | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteNatGateway | "Who deleted this NAT GW?" Immediate loss of internet access for all dependent private subnets. |
| CreateNatGateway | "Who created it and in which subnet/AZ?" For cost tracking — NAT GWs are expensive ($0.045/hr + data processing). |

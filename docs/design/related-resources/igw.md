# Internet Gateways (igw) — Related Resources

## Real-World Use Cases

**1. "Does this VPC have internet access?"** An IGW must be attached to the VPC, and at least one route table must have a route to it. Both conditions are required — an attached IGW with no route does nothing.

**2. "Which subnets are public?"** Public subnets are those whose route table has a route to this IGW. Find the route tables referencing this IGW, then their associated subnets.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Route Tables (rtb) | `ec2:DescribeRouteTables` with `Filters=[{Name=route.gateway-id, Values=[igw-xxx]}]`. | "Which route tables use this IGW?" Determines which subnets are public. | P0 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| VPC (vpc) | IGW response has `Attachments[].VpcId` — FORWARD. | "Which VPC has this IGW?" An IGW can only be attached to one VPC. | P0 |
| Public Subnets (subnet) | Multi-hop: Route tables with routes to this IGW → their associated subnet IDs. These are the VPC's public subnets. | "Which subnets have direct internet access via this IGW?" | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DetachInternetGateway | "Who detached the IGW?" Immediately breaks all internet access for the VPC's public subnets. Catastrophic if unintentional. |
| DeleteInternetGateway | "Who deleted the IGW?" Must be detached first, so this is a deliberate decommission. |
| AttachInternetGateway | "Who attached an IGW to this VPC?" Security-relevant — adding an IGW is the first step toward making resources internet-accessible. |

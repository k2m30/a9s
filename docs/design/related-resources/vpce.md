# VPC Endpoints (vpce) — Related Resources

## Real-World Use Cases

**1. "Why can't my Lambda reach S3 even though there's a VPC endpoint?"** The VPC endpoint exists, but is it in the right route table (gateway type) or the right subnet (interface type)? And does the endpoint policy allow access to the specific bucket?

**2. "Which route tables have the S3 gateway endpoint?"** Gateway endpoints inject routes into specified route tables. If a new subnet doesn't have access, its route table might not be in the endpoint's route table list.

**3. "Is this interface endpoint accessible from my subnet?"** Interface endpoints create ENIs in specific subnets with specific security groups. If your resource is in a different subnet, it needs DNS resolution and network connectivity to the endpoint's ENI.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Route Tables (rtb) — gateway type | For gateway endpoints: `ec2:DescribeRouteTables` with `Filters=[{Name=route.gateway-id, Values=[vpce-xxx]}]`. Or the VPCE response has `RouteTableIds[]` — FORWARD. | "Which route tables have this endpoint's route?" | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Subnets (subnet) — interface type | VPCE response has `SubnetIds[]` — FORWARD. Navigate to subnets to verify the endpoint ENIs are in the right AZs. | "Which subnets have this endpoint's ENIs?" Interface endpoints are AZ-scoped. | P1 |
| Security Groups (sg) — interface type | VPCE response has `Groups[].GroupId` — FORWARD. Navigate to SGs to verify inbound rules allow traffic on the service port (443 for most AWS services). | "Why can't my resource connect through this endpoint?" SG must allow inbound HTTPS from the source resources. | P1 |
| VPC (vpc) | VPCE response has `VpcId` — FORWARD. | "Which VPC has this endpoint?" | P1 |
| ENIs (eni) — interface type | VPCE response has `NetworkInterfaceIds[]` — FORWARD. The ENIs show the actual private IPs that resolve when using the endpoint. | "What are the endpoint's IP addresses?" Useful for debugging DNS resolution to endpoint IPs. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteVpcEndpoints | "Who deleted this endpoint?" Removing a VPC endpoint breaks private connectivity to the AWS service for all resources in the VPC. |
| ModifyVpcEndpoint | "Who changed the endpoint policy, route tables, or subnets?" A restricted endpoint policy can block access to specific resources. |
| CreateVpcEndpoint | "Who created this endpoint?" Audit trail for network architecture changes. |

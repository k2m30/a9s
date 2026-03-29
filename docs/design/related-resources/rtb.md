# Route Tables (rtb) — Related Resources

## Real-World Use Cases

**1. "Why can't this subnet reach the internet?"** You check the route table associated with the subnet. Is there a route to a NAT Gateway (private subnet) or Internet Gateway (public subnet)? A missing route is the most common cause of connectivity failures.

**2. "Which subnets use this route table?"** Before modifying a route, you need to know the blast radius. The route table's `Associations[]` field shows associated subnets — this is forward, but understanding which subnets means understanding which resources are affected.

**3. "Someone changed routing and broke connectivity."** You need CloudTrail to find who added, deleted, or modified routes.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal reverse relationships) | Route tables are referenced BY subnets through associations, but the association data is already in the route table's own API response (`Associations[]`). Other resources don't reference route tables directly. | — | — |

## Algorithmic Relationships

Route tables are unusual: most of their important relationships are FORWARD (in their own API response), because the route table itself contains references to other resources in its `Routes[]` array.

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Subnets (subnet) | Route table response has `Associations[].SubnetId` — FORWARD. Navigate to subnets to see what resources are affected by routes in this table. | "Which subnets use this route table?" Must check before modifying routes. | P0 |
| NAT Gateway (nat) | Routes with `NatGatewayId` target — FORWARD. Navigate to the NAT GW to check its state (a failed NAT GW breaks all private subnet internet access). | "Which NAT GW handles internet-bound traffic?" If the NAT GW is unhealthy, all private subnets using this route table lose internet. | P0 |
| Internet Gateway (igw) | Routes with `GatewayId` starting with `igw-` — FORWARD. | "Does this route table have internet access?" The presence of an IGW route makes associated subnets public. | P1 |
| Transit Gateway (tgw) | Routes with `TransitGatewayId` — FORWARD. | "How does cross-VPC traffic route?" | P1 |
| VPC Peering (not in a9s) | Routes with `VpcPeeringConnectionId` — FORWARD. | "Which peered VPC does this route point to?" | P1 |
| VPC Endpoints (vpce) | Gateway VPC endpoints inject routes automatically. Routes with `GatewayId` starting with `vpce-` — FORWARD. | "Are S3/DynamoDB gateway endpoints in this route table?" | P2 |
| VPC (vpc) | Route table response has `VpcId` — FORWARD. | "Which VPC owns this route table?" | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| CreateRoute / ReplaceRoute / DeleteRoute | "Who changed the routing?" The most dangerous route table change — a deleted or wrong route breaks connectivity for all associated subnets. |
| AssociateRouteTable / DisassociateRouteTable | "Who changed which route table a subnet uses?" Swapping a route table can instantly make a private subnet public or vice versa. |
| DeleteRouteTable | "Who deleted this route table?" Fails if still associated, but if successful, means it was fully detached first. |

# Network Interfaces (eni) — Related Resources

## Real-World Use Cases

**1. "What AWS resource owns this ENI?"** ENIs are created by many services — EC2, RDS, ELB, Lambda, ECS, VPC Endpoints, NAT Gateways. The ENI's `Description` and `InterfaceType` fields reveal the owner, but you need to parse these to navigate to the actual resource.

**2. "Why does this ENI exist and can I delete it?"** Orphaned ENIs (leftover from deleted resources) consume private IPs and clutter the network. But some ENIs are managed by services and will break things if deleted manually.

**3. "What security groups are on this ENI?"** ENIs are the actual bearer of security group assignments. Even though you "assign SGs to an EC2 instance," the SGs are actually on the ENIs attached to that instance.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | ENIs are themselves the reverse lookup mechanism — other resources are found THROUGH ENIs, not the other way. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Owning Resource | Parse `Description` and `InterfaceType` to identify the creating service and resource: `InterfaceType=interface` + `Description="ELB app/..."` → Load Balancer. `InterfaceType=lambda` → Lambda function. `InterfaceType=nat_gateway` → NAT Gateway. `Description="RDSNetworkInterface"` → RDS instance. `Description` containing `arn:aws:ecs:` → ECS Task. `Description="VPC Endpoint Interface..."` → VPC Endpoint. `InterfaceType=interface` + `Attachment.InstanceId` → EC2 instance. | "What service created this ENI?" THE primary question. Navigate to the owning resource. | P0 |
| EC2 Instance (ec2) | ENI response has `Attachment.InstanceId` — FORWARD (if attached to an EC2 instance). | "Which instance uses this ENI?" | P0 |
| Security Groups (sg) | ENI response has `Groups[].GroupId` — FORWARD. | "What SGs control this ENI's traffic?" | P0 |
| Subnet (subnet) | ENI response has `SubnetId` — FORWARD. | "Which subnet is this ENI in?" | P1 |
| Elastic IP (eip) | ENI response has `Association.AllocationId` — FORWARD (if an EIP is associated). | "Does this ENI have a public IP?" | P1 |
| VPC (vpc) | ENI response has `VpcId` — FORWARD. | "Which VPC?" | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteNetworkInterface | "Who deleted this ENI?" If it was managed by a service, manual deletion causes that service to malfunction. |
| ModifyNetworkInterfaceAttribute | "Who changed SGs on this ENI?" SG changes on an ENI affect whatever resource owns it. |
| AttachNetworkInterface / DetachNetworkInterface | "Who moved this ENI between instances?" ENI migration is used for failover patterns. |

# Elastic IPs (eip) — Related Resources

## Real-World Use Cases

**1. "What is this EIP attached to?"** EIPs cost money when unattached. During cost cleanup, find unassociated EIPs. For associated ones, verify they're attached to the right instance or NAT GW.

**2. "Will stopping this instance change its public IP?"** If the instance has an EIP, the IP survives stop/start. If not, the auto-assigned public IP changes. Check the EIP associations for this instance.

**3. "Which DNS records point to this IP?"** Before releasing an EIP, find Route 53 A records pointing to its address — releasing it without updating DNS causes outages.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Route 53 Records (r53) | Search R53 hosted zones for A records with value matching this EIP's public IP address. If a9s has R53 data cached, search in-memory. | "What DNS names resolve to this IP?" Must update DNS before releasing the EIP. | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| EC2 Instance (ec2) | EIP response has `InstanceId` — FORWARD (if associated with an instance). | "Which instance uses this EIP?" | P0 |
| ENI (eni) | EIP response has `NetworkInterfaceId` — FORWARD (if associated with an ENI). | "Which network interface has this EIP?" For non-EC2 associations (ELB, NAT GW). | P1 |
| NAT Gateway (nat) | `ec2:DescribeNatGateways` with `Filters=[{Name=nat-gateway-address.allocation-id, Values=[eipalloc-xxx]}]`. Alternatively, check if the EIP's associated ENI has `InterfaceType=natGateway`. | "Is this EIP used by a NAT Gateway?" NAT GWs require EIPs — releasing the EIP breaks the NAT GW. | P0 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| ReleaseAddress | "Who released this EIP?" If it was in use, this causes an immediate connectivity loss for whatever was using it. Released EIPs are gone — the same IP is not recoverable. |
| DisassociateAddress | "Who detached this EIP from the instance?" The instance loses its stable public IP. |
| AssociateAddress | "Who attached this EIP?" Shows when and to what resource it was assigned. |

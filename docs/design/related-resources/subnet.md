# Subnets (subnet) — Related Resources

## Real-World Use Cases

**1. "What's consuming IPs in this subnet?"** The subnet shows `AvailableIpAddressCount` is 3 out of 4091. Something is eating all the IPs — probably EKS pods or Lambda ENIs. You need to see which ENIs exist in this subnet and what service owns them.

**2. "Is this a public or private subnet?"** The subnet itself doesn't have a "public/private" flag. You need to check its route table: if there's a route to an Internet Gateway, it's public. If traffic goes through a NAT Gateway, it's private.

**3. "Why can't instances in this subnet reach the internet?"** Check the route table association — is there a route to a NAT GW or IGW? Is the NAT GW healthy? Is the route table even associated?

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| EC2 Instances (ec2) | `ec2:DescribeInstances` with `Filters=[{Name=subnet-id, Values=[subnet-xxx]}]`. | "What instances run in this subnet?" | P0 |
| ENIs (eni) | `ec2:DescribeNetworkInterfaces` with `Filters=[{Name=subnet-id, Values=[subnet-xxx]}]`. The universal query — shows ALL resources with network interfaces in this subnet (EC2, RDS, ELB, Lambda, ECS, VPC Endpoints). | "What's consuming IPs here?" Parse ENI descriptions to identify the owning service. | P0 |
| NAT Gateways (nat) | `ec2:DescribeNatGateways` with `Filters=[{Name=subnet-id, Values=[subnet-xxx]}]`. | "Is there a NAT GW in this subnet?" NAT GWs must be in public subnets. | P1 |
| ELB (elb) | ELBs are multi-AZ and span subnets. Check ELB `AvailabilityZones[].SubnetId` for this subnet. If a9s has ELB data cached, filter in-memory. | "Which load balancers have endpoints in this subnet?" | P1 |
| RDS Subnet Groups (not in a9s) | `rds:DescribeDBSubnetGroups` and check `Subnets[].SubnetIdentifier` for this subnet. | "Which RDS subnet groups include this subnet?" Determines which databases could be placed here. | P2 |
| EKS Node Groups (ng) | Check NG `subnets[]` for this subnet ID. If a9s has NG data cached, filter in-memory. | "Which EKS node groups launch nodes here?" | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this subnet?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Route Table (rtb) | `ec2:DescribeRouteTables` with `Filters=[{Name=association.subnet-id, Values=[subnet-xxx]}]`. If no explicit association, the VPC's main route table applies (filter by `association.main=true` and matching VPC). | "Is this subnet public or private?" The route table determines internet reachability. A route to an IGW = public; route to NAT GW = private; no route = isolated. | P0 |
| VPC (vpc) | Subnet response has `VpcId` — FORWARD. | "Which VPC contains this subnet?" | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteSubnet | "Who deleted this subnet?" Deletion fails if resources still exist in it, so this means all resources were removed first. |
| ModifySubnetAttribute | "Who changed auto-assign public IP or IPv6 settings?" Enabling auto-assign on a private subnet is a security misconfiguration. |
| CreateSubnet | "Who created this subnet and with what CIDR?" CIDR allocation should follow the VPC addressing plan. |

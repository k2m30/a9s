# QA User Stories: Cross-Resource Navigation (Issue #64)

Scope: navigating from any resource to its related resources -- peer-to-peer and
upward references between resources that link to each other (e.g., EC2 to VPC,
EC2 to Security Groups, VPC to Subnets). This is distinct from parent-to-child
drill-down (which shows owned sub-resources like S3 Objects inside a Bucket).

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration files. AWS CLI equivalents are cited so testers can
verify data parity.

> **Dependency:** Blocked by #65 (AMI resource type) and #66 (EBS Volumes/Snapshots)
> which must exist before cross-resource navigation can reference them.

---

## A. Related Resources Picker

### A.1 Opening the Related Resources Picker

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I am viewing the EC2 instance list with an instance selected. I press `R`. | A picker overlay or sub-view appears showing all related resources for this EC2 instance. The picker lists entries such as VPC, Subnet, Security Groups, EIP, ENI, IAM Role, ASG, and Target Groups. Each entry shows the resource type, name/ID, and relationship type (e.g., "attached", "associated"). |
| A.1.2 | I am viewing the EC2 instance list. The selected instance has VPC "vpc-0abc123", Subnet "subnet-0def456", and 2 Security Groups "sg-0111" and "sg-0222". I press `R`. | The picker shows at minimum: "VPC: vpc-0abc123 (associated)", "Subnet: subnet-0def456 (associated)", "Security Group: sg-0111 (associated)", "Security Group: sg-0222 (associated)". The entries are derived from fields visible in the EC2 detail (VpcId, SubnetId, SecurityGroups). |
| A.1.3 | I am viewing the EC2 instance list. The selected instance has no Public IP, no EIP, and no IAM Instance Profile. I press `R`. | The picker shows only the related resources that actually exist for this instance (VPC, Subnet, Security Groups). Entries for EIP and IAM Role do not appear (or appear greyed out / marked as "none"). |
| A.1.4 | I am viewing a resource list for a type that has zero related resources defined (hypothetical edge case). I press `R`. | Nothing happens, or a brief flash message appears indicating "No related resources" in the header right side. |
| A.1.5 | I press `R` while in the main menu (not a resource list). | Nothing happens. The `R` key has no effect on the main menu. |

**AWS comparison:**
```
aws ec2 describe-instances --instance-ids i-0abc123 --query 'Reservations[0].Instances[0].{VpcId:VpcId,SubnetId:SubnetId,SecurityGroups:SecurityGroups[*].GroupId,IamInstanceProfile:IamInstanceProfile.Arn}'
```
Expected fields used to derive related resources: VpcId, SubnetId, SecurityGroups, IamInstanceProfile, PublicIpAddress (for EIP correlation)

### A.2 Picker Display and Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | The related resources picker is open with 6 entries. | I can navigate between entries using `j`/down-arrow and `k`/up-arrow. The selected entry is highlighted with blue background (#7aa2f7) and dark foreground (#1a1b26), bold. |
| A.2.2 | I look at each entry in the picker. | Each entry shows three pieces of information: (1) the resource type display name (e.g., "VPC", "Security Group"), (2) the resource name or ID (e.g., "vpc-0abc123", "sg-0111"), and (3) the relationship type (e.g., "associated", "attached"). |
| A.2.3 | The picker has more entries than fit on screen. | The picker scrolls vertically. The frame title shows the count of related resources, e.g., "related(8)". |
| A.2.4 | I press `Esc` on the picker. | The picker closes and I return to the resource list with the same row selected. |
| A.2.5 | I press `g` in the picker. | Selection jumps to the first related resource entry. |
| A.2.6 | I press `G` in the picker. | Selection jumps to the last related resource entry. |

### A.3 Selecting a Related Resource

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | The picker is open for EC2 instance "api-prod-01". I select the "VPC: vpc-0abc123 (associated)" entry and press Enter. | The application navigates to the VPC resource list view. The list is pre-filtered or pre-focused to show VPC "vpc-0abc123". I can see the VPC in its native list view with columns Name, VPC ID (width 24), CIDR Block (width 18), State (width 12), Default (width 9). |
| A.3.2 | The picker is open for EC2 instance "api-prod-01". I select "Security Group: sg-0111 (associated)" and press Enter. | The application navigates to the Security Groups list view, pre-filtered to show security group "sg-0111". Columns visible: Group Name (width 28), Group ID (width 24), VPC ID (width 24), Description (width 36). |
| A.3.3 | The picker is open for EC2 instance "api-prod-01". I select "Subnet: subnet-0def456 (associated)" and press Enter. | The application navigates to the Subnets list view, pre-filtered or focused on subnet "subnet-0def456". Columns visible: Name (width 28), Subnet ID (width 26), VPC ID (width 24), CIDR Block (width 18), AZ (width 14), State (width 12), Available IPs (width 14). |
| A.3.4 | The picker is open for an EC2 instance with an IAM Instance Profile. I select the IAM Role entry and press Enter. | The application navigates to the IAM Roles list view, pre-filtered to the role associated with the instance profile. Columns visible: Role Name (width 36), Last Used (width 22), Path (width 20), Created (width 22), Description (width 30). |

**AWS comparison:**
```
aws ec2 describe-vpcs --vpc-ids vpc-0abc123
aws ec2 describe-security-groups --group-ids sg-0111
aws ec2 describe-subnets --subnet-ids subnet-0def456
aws iam get-instance-profile --instance-profile-name PROFILE_NAME
```

---

## B. EC2 Instance Related Resources

### B.1 Forward Navigation from EC2

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | EC2 instance has VpcId "vpc-0abc123". I press `R` and select VPC. | I navigate to VPCs list, filtered to "vpc-0abc123". |
| B.1.2 | EC2 instance has SubnetId "subnet-0def456". I press `R` and select Subnet. | I navigate to Subnets list, filtered to "subnet-0def456". |
| B.1.3 | EC2 instance has SecurityGroups ["sg-0111", "sg-0222"]. I press `R`. | The picker shows two separate Security Group entries: "sg-0111" and "sg-0222". Selecting either navigates to SG list, filtered to that group. |
| B.1.4 | EC2 instance has a PublicIpAddress "54.123.45.67" associated with an Elastic IP. I press `R` and select EIP. | I navigate to EIP list, filtered to the Elastic IP with Public IP "54.123.45.67". EIP columns visible: Name (width 24), Allocation ID (width 26), Public IP (width 16), Association (width 26), Instance (width 20), Domain (width 8). |
| B.1.5 | EC2 instance has IamInstanceProfile set. I press `R`. | IAM Role does NOT appear in the related picker — instance profile ARNs are not role ARNs, so direct navigation is not possible. The EC2-to-Role relationship requires an algorithmic lookup (iam:GetInstanceProfile). |
| B.1.6 | EC2 instance has associated ENIs. I press `R` and select ENI. | I navigate to ENI list, filtered to show ENIs attached to this instance. ENI columns visible: Name (width 24), ENI ID (width 26), Status (width 12), Type (width 14), VPC ID (width 24), Private IP (width 16). |

**AWS comparison:**
```
aws ec2 describe-instances --instance-ids i-0abc123 --query 'Reservations[0].Instances[0].{VpcId:VpcId,SubnetId:SubnetId,SecurityGroups:SecurityGroups,PublicIpAddress:PublicIpAddress,IamInstanceProfile:IamInstanceProfile,NetworkInterfaces:NetworkInterfaces}'
```
Expected EC2 detail fields providing navigation data: VpcId, SubnetId, SecurityGroups, PublicIpAddress, IamInstanceProfile, NetworkInterfaces

---

## C. VPC Related Resources

### C.1 Forward Navigation from VPC

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I am viewing the VPC list. I select VPC "vpc-0abc123" and press `R`. | The picker shows related resources: Subnets, Route Tables, NAT Gateways, Internet Gateways, Security Groups, VPC Endpoints, Transit Gateways. Each entry shows the relationship as "contains" or "associated". |
| C.1.2 | I select "Subnets" and press Enter. | I navigate to the Subnets list, filtered to show subnets in VPC "vpc-0abc123". |
| C.1.3 | I select "Route Tables" and press Enter. | I navigate to the Route Tables list, filtered to VPC "vpc-0abc123". Columns visible: Name (width 28), Route Table ID (width 26), VPC ID (width 24), Routes (width 8), Assoc. (width 8). |
| C.1.4 | I select "NAT Gateways" and press Enter. | I navigate to the NAT Gateways list, filtered to VPC "vpc-0abc123". Columns visible: Name (width 24), NAT Gateway ID (width 26), VPC ID (width 24), Subnet ID (width 26), State (width 12), Public IP (width 16). |
| C.1.5 | I select "Internet Gateways" and press Enter. | I navigate to the IGW list, filtered to IGWs attached to VPC "vpc-0abc123". Columns visible: Name (width 28), IGW ID (width 26), VPC ID (width 24), State (width 12). |
| C.1.6 | I select "Security Groups" and press Enter. | I navigate to the Security Groups list, filtered to VPC "vpc-0abc123". |
| C.1.7 | I select "VPC Endpoints" and press Enter. | I navigate to the VPC Endpoints list, filtered to VPC "vpc-0abc123". Columns visible: Service Name (width 40), Endpoint ID (width 26), Type (width 12), State (width 12), VPC ID (width 24). |

**AWS comparison:**
```
aws ec2 describe-subnets --filters "Name=vpc-id,Values=vpc-0abc123"
aws ec2 describe-route-tables --filters "Name=vpc-id,Values=vpc-0abc123"
aws ec2 describe-nat-gateways --filter "Name=vpc-id,Values=vpc-0abc123"
aws ec2 describe-internet-gateways --filters "Name=attachment.vpc-id,Values=vpc-0abc123"
aws ec2 describe-security-groups --filters "Name=vpc-id,Values=vpc-0abc123"
aws ec2 describe-vpc-endpoints --filters "Name=vpc-id,Values=vpc-0abc123"
```

---

## D. ECS Service Related Resources

### D.1 Forward Navigation from ECS Service

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I am viewing the ECS Services list. I select a service "api-service" and press `R`. | The picker shows related resources: Cluster, Target Group, Load Balancer, Security Groups, Subnets, IAM Role. Each shows the relationship as "configured". |
| D.1.2 | I select "Target Group" and press Enter. | I navigate to the Target Groups list, filtered to the target group referenced by the ECS service's load balancer configuration. Columns visible: Target Group (width 32), Port (width 8), Protocol (width 10), VPC ID (width 24), Target Type (width 12), Health Check (width 24). |
| D.1.3 | I select "Load Balancer" and press Enter. | I navigate to the Load Balancers list, filtered to the ELB referenced by the ECS service. Columns visible: Name (width 32), Type (width 12), Scheme (width 14), State (width 12), DNS Name (width 48), VPC ID (width 24). |
| D.1.4 | I select "IAM Role" and press Enter. | I navigate to IAM Roles, filtered to the execution role ARN from the ECS service configuration. |

**AWS comparison:**
```
aws ecs describe-services --cluster CLUSTER --services api-service --query 'services[0].{LoadBalancers:loadBalancers,NetworkConfig:networkConfiguration,RoleArn:roleArn}'
```
Expected ECS service detail fields providing navigation data: ClusterArn, LoadBalancers, NetworkConfiguration, RoleArn

---

## E. Lambda Related Resources

### E.1 Forward Navigation from Lambda

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | I am viewing the Lambda list. I select function "data-processor" which has a VPC configuration. I press `R`. | The picker shows: VPC, Subnets (may show multiple), Security Groups, IAM Role, Log Group. Each shows the relationship as "configured". |
| E.1.2 | I select "VPC" and press Enter. | I navigate to the VPCs list, filtered to the VPC ID from the Lambda's VpcConfig. |
| E.1.3 | I select "IAM Role" and press Enter. | I navigate to IAM Roles, filtered to the role extracted from the Lambda's Role ARN. |
| E.1.4 | I select "Log Group" and press Enter. | I navigate to Log Groups, filtered to the log group name "/aws/lambda/data-processor". Columns visible: Log Group Name (width 48), Size (width 14), Retention (width 10), Metric Filters (width 8), Created (width 16). |
| E.1.5 | Lambda function "simple-handler" has NO VPC configuration. I press `R`. | The picker shows only: IAM Role, Log Group. No VPC, Subnet, or Security Group entries appear. |

**AWS comparison:**
```
aws lambda get-function-configuration --function-name data-processor --query '{VpcConfig:VpcConfig,Role:Role,LoggingConfig:LoggingConfig}'
```
Expected Lambda detail fields providing navigation data: VpcConfig, Role, LoggingConfig

---

## F. RDS Related Resources

### F.1 Forward Navigation from RDS Instance

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | I am viewing the RDS instances list. I select "prod-database" and press `R`. | The picker shows: VPC, Subnets (from DB Subnet Group), Security Groups. Each shows the relationship as "associated". |
| F.1.2 | I select "Security Groups" and press Enter. | I navigate to the Security Groups list, filtered to the VPC security group IDs from the RDS instance's VpcSecurityGroups field. |
| F.1.3 | I select "VPC" and press Enter. | I navigate to VPCs, filtered to the VPC ID from the RDS instance's DBSubnetGroup. |

**AWS comparison:**
```
aws rds describe-db-instances --db-instance-identifier prod-database --query 'DBInstances[0].{VpcSecurityGroups:VpcSecurityGroups,DBSubnetGroup:DBSubnetGroup}'
```
Expected RDS detail fields providing navigation data: VpcSecurityGroups, DBSubnetGroup

---

## G. EKS Cluster Related Resources

### G.1 Forward Navigation from EKS

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | I am viewing the EKS clusters list. I select cluster "prod-cluster" and press `R`. | The picker shows: VPC, Subnets, Security Groups, Node Groups, IAM Role. |
| G.1.2 | I select "VPC" and press Enter. | I navigate to VPCs, filtered to the VPC ID from the EKS cluster's ResourcesVpcConfig. |
| G.1.3 | I select "Node Groups" and press Enter. | I navigate to the EKS Node Groups list, filtered to node groups belonging to cluster "prod-cluster". Columns visible: Node Group (width 28), Cluster (width 24), Status (width 14), Instance Types (width 20), Desired (width 9). |
| G.1.4 | I select "IAM Role" and press Enter. | I navigate to IAM Roles, filtered to the role from the EKS cluster's RoleArn. |

**AWS comparison:**
```
aws eks describe-cluster --name prod-cluster --query 'cluster.{ResourcesVpcConfig:resourcesVpcConfig,RoleArn:roleArn}'
aws eks list-nodegroups --cluster-name prod-cluster
```
Expected EKS detail fields providing navigation data: ResourcesVpcConfig, RoleArn

---

## H. Load Balancer Related Resources

### H.1 Forward Navigation from Load Balancer

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | I am viewing the Load Balancers list. I select ELB "api-prod-alb" and press `R`. | The picker shows: VPC, Security Groups, Subnets, Target Groups, ACM Certificate. |
| H.1.2 | I select "VPC" and press Enter. | I navigate to VPCs, filtered to the ELB's VpcId. |
| H.1.3 | I select "Target Groups" and press Enter. | I navigate to Target Groups list, filtered to target groups associated with this ELB. |
| H.1.4 | I select "Security Groups" and press Enter. | I navigate to Security Groups, filtered to the SGs listed in the ELB's SecurityGroups field. |

**AWS comparison:**
```
aws elbv2 describe-load-balancers --names api-prod-alb --query 'LoadBalancers[0].{VpcId:VpcId,SecurityGroups:SecurityGroups,AvailabilityZones:AvailabilityZones}'
aws elbv2 describe-target-groups --load-balancer-arn ARN
```
Expected ELB detail fields providing navigation data: VpcId, SecurityGroups, AvailabilityZones

---

## I. Target Group Related Resources

### I.1 Forward Navigation from Target Group

| ID | Story | Expected |
|----|-------|----------|
| I.1.1 | I am viewing the Target Groups list. I select "api-prod-tg" and press `R`. | The picker shows: Load Balancer, VPC. |
| I.1.2 | I select "Load Balancer" and press Enter. | I navigate to the Load Balancers list, filtered to the ELB ARN from the target group's LoadBalancerArns field. |
| I.1.3 | I select "VPC" and press Enter. | I navigate to VPCs, filtered to the target group's VpcId. |

**AWS comparison:**
```
aws elbv2 describe-target-groups --names api-prod-tg --query 'TargetGroups[0].{LoadBalancerArns:LoadBalancerArns,VpcId:VpcId}'
```
Expected TG detail fields providing navigation data: LoadBalancerArns, VpcId

---

## J. Security Group Related Resources (Phase 2 -- Reverse Navigation)

### J.1 Cross-Reference Navigation from Security Group

| ID | Story | Expected |
|----|-------|----------|
| J.1.1 | I am viewing the Security Groups list. I select "web-sg" (sg-0111) and press `R`. | The picker shows: VPC, and (Phase 2) referencing resources -- EC2 instances, RDS instances, ELBs, and Lambda functions that use this security group. Each referencing resource shows "referenced-by" as the relationship. |
| J.1.2 | I select "VPC: vpc-0abc123 (associated)" and press Enter. | I navigate to VPCs, filtered to "vpc-0abc123". |
| J.1.3 | I select "EC2 Instance: api-prod-01 (referenced-by)" and press Enter. | I navigate to the EC2 list, filtered to show instance "api-prod-01". The EC2 list columns are visible: Name (width 24), State (width 12), Lifecycle (width 12), Type (width 14), Private IP (width 16), Public IP (width 16), Instance ID (width 20), Launch Time (width 22). |
| J.1.4 | I select "RDS Instance: prod-database (referenced-by)" and press Enter. | I navigate to the RDS instances list, filtered to "prod-database". Columns visible: DB Identifier (width 28), Engine (width 12), Version (width 10), Status (width 14), Class (width 16), Endpoint (width 40), Multi-AZ (width 10). |

**AWS comparison:**
```
aws ec2 describe-instances --filters "Name=instance.group-id,Values=sg-0111"
aws rds describe-db-instances --query 'DBInstances[?VpcSecurityGroups[?VpcSecurityGroupId==`sg-0111`]]'
aws elbv2 describe-load-balancers --query 'LoadBalancers[?SecurityGroups[?contains(@,`sg-0111`)]]'
```

---

## K. Subnet Related Resources

### K.1 Forward and Upward Navigation from Subnet

| ID | Story | Expected |
|----|-------|----------|
| K.1.1 | I am viewing the Subnets list. I select "private-subnet-1a" and press `R`. | The picker shows: VPC, Route Table, NAT Gateway (if associated), ENIs. |
| K.1.2 | I select "VPC" and press Enter. | I navigate to VPCs, filtered to the subnet's VpcId. |
| K.1.3 | I select "Route Table" and press Enter. | I navigate to Route Tables, filtered to the route table associated with this subnet. |

**AWS comparison:**
```
aws ec2 describe-subnets --subnet-ids subnet-0def456 --query 'Subnets[0].VpcId'
aws ec2 describe-route-tables --filters "Name=association.subnet-id,Values=subnet-0def456"
```

---

## L. S3, CloudFront, Route53, IAM Role Related Resources

### L.1 S3 Bucket Related Resources

| ID | Story | Expected |
|----|-------|----------|
| L.1.1 | I am viewing the S3 list. I select bucket "static-assets-prod" and press `R`. | The picker shows related resources such as CloudFront Distribution (if the bucket is configured as an origin) and any Lambda notification targets. |

**AWS comparison:**
```
aws s3api get-bucket-notification-configuration --bucket static-assets-prod
aws cloudfront list-distributions --query 'DistributionList.Items[?Origins.Items[?DomainName==`static-assets-prod.s3.amazonaws.com`]]'
```

### L.2 CloudFront Related Resources

| ID | Story | Expected |
|----|-------|----------|
| L.2.1 | I am viewing the CloudFront list. I select a distribution and press `R`. | The picker shows: S3 Bucket (origin), ACM Certificate, WAF Web ACL, Route53 Record. |
| L.2.2 | I select "S3 Bucket" and press Enter. | I navigate to S3 list, filtered to the origin bucket. Columns visible: Bucket Name (width 36), Region (width 14), Creation Date (width 22). |
| L.2.3 | I select "ACM Certificate" and press Enter. | I navigate to ACM Certificates list, filtered to the certificate. Columns visible: Domain Name (width 40), Status (width 14), Type (width 14), Expires (width 22), In Use (width 8). |

**AWS comparison:**
```
aws cloudfront get-distribution --id DIST_ID --query 'Distribution.DistributionConfig.{Origins:Origins,ViewerCertificate:ViewerCertificate,WebACLId:WebACLId}'
```

### L.3 Route53 Hosted Zone Related Resources

| ID | Story | Expected |
|----|-------|----------|
| L.3.1 | I am viewing the Route53 list. I select a hosted zone and press `R`. | The picker shows alias targets that point to ELBs, CloudFront, EIPs, or S3 endpoints. The relationship is "points-to". |

**AWS comparison:**
```
aws route53 list-resource-record-sets --hosted-zone-id ZONE_ID --query 'ResourceRecordSets[?AliasTarget]'
```

### L.4 IAM Role Related Resources

| ID | Story | Expected |
|----|-------|----------|
| L.4.1 | I am viewing the IAM Roles list. I select "payment-service-role" and press `R`. | The picker shows resources that assume this role: Lambda functions, ECS services, EC2 instances (Phase 2 reverse lookup). Also shows Attached Policies. |
| L.4.2 | I select "Attached Policies" and press Enter. | I navigate to the Role Policies child view (which already exists as a parent-child drill-down). |

**AWS comparison:**
```
aws iam list-attached-role-policies --role-name payment-service-role
aws lambda list-functions --query 'Functions[?Role==`arn:aws:iam::123456789012:role/payment-service-role`]'
```

---

## M. ASG Related Resources

### M.1 Forward Navigation from ASG

| ID | Story | Expected |
|----|-------|----------|
| M.1.1 | I am viewing the ASG list. I select "api-prod-asg" and press `R`. | The picker shows: Target Group, Subnets, EC2 Instances (managed). |
| M.1.2 | I select "Target Group" and press Enter. | I navigate to Target Groups, filtered to the target group ARN from the ASG's TargetGroupARNs field. |
| M.1.3 | I select "EC2 Instances" and press Enter. | I navigate to EC2 list, filtered to show instances belonging to this ASG. |

**AWS comparison:**
```
aws autoscaling describe-auto-scaling-groups --auto-scaling-group-names api-prod-asg --query 'AutoScalingGroups[0].{TargetGroupARNs:TargetGroupARNs,VPCZoneIdentifier:VPCZoneIdentifier,Instances:Instances}'
```
Expected ASG detail fields providing navigation data: TargetGroupARNs, VPCZoneIdentifier, Instances

---

## N. Breadcrumb Navigation (Phase 3)

### N.1 Multi-Hop Navigation with Breadcrumb History

| ID | Story | Expected |
|----|-------|----------|
| N.1.1 | I navigate from EC2 "api-prod-01" to VPC "vpc-0abc123" via `R`. Then from VPC I press `R` and navigate to Subnet "subnet-0def456". | Each hop is pushed onto the view stack. The frame title or breadcrumb indicator shows the navigation chain. |
| N.1.2 | I press `Esc` from the Subnet view. | I return to the VPC list view where "vpc-0abc123" is focused. |
| N.1.3 | I press `Esc` again. | I return to the EC2 list view where "api-prod-01" is focused. |
| N.1.4 | I perform a 4-hop navigation: EC2 -> VPC -> Subnet -> Route Table. I press `Esc` 3 times. | Each Esc pops one level. After 3 Esc presses, I am back at the EC2 list. The view stack unwinds correctly without skipping levels or returning to the wrong view. |
| N.1.5 | I navigate EC2 -> VPC via `R`, then from the VPC view I press `R` again and navigate to an IGW. The breadcrumb shows a multi-hop path. | The breadcrumb or view stack reflects: EC2 -> VPC -> IGW. Pressing Esc from IGW returns to VPC, not to EC2. |

---

## O. Help Screen Shows Related Key Binding

### O.1 Help Screen Integration

| ID | Story | Expected |
|----|-------|----------|
| O.1.1 | I am on any resource list. I press `?` to open help. | The help screen shows `<R>` Related (or similar) in the key bindings, under the RESOURCE or NAVIGATION column. The key is rendered in green bold (#9ece6a) with the description in plain white (#c0caf5). |
| O.1.2 | I am on the main menu. I press `?`. | The `R` key binding does NOT appear in the help screen for the main menu (since cross-resource navigation is not applicable there). |

---

## P. Filter and Command Mode Interaction

### P.1 Related Resources Picker and Input Modes

| ID | Story | Expected |
|----|-------|----------|
| P.1.1 | I press `/` in the related resources picker. | Filter mode activates. Typing narrows the list of related resources by substring match (e.g., typing "vpc" shows only VPC-related entries). Header right shows "/vpc" in amber (#e0af68) bold. |
| P.1.2 | I press `Esc` while filter is active in the picker. | The filter clears and all related resources reappear. |
| P.1.3 | I am in the related resources picker and press `:`. | Command mode activates. I can type a command like `:ec2` to navigate directly to the EC2 list, leaving the picker. |
| P.1.4 | I press `R` while filter mode is active on the resource list. | The `R` character is appended to the filter text. The related resources picker does not open. |
| P.1.5 | I press `R` while command mode is active on the resource list. | The `R` character is appended to the command text. The related resources picker does not open. |

---

## Q. Error Handling

### Q.1 Navigation Target Not Found

| ID | Story | Expected |
|----|-------|----------|
| Q.1.1 | I navigate from EC2 to VPC "vpc-0abc123" via `R`, but the VPC has been deleted since the EC2 data was fetched. | The VPC list loads but the filter finds no matching VPC. The frame title shows "vpc(0)" or the list is empty. No crash occurs. |
| Q.1.2 | The related resource target is in a different region than the current session. | The navigation attempt shows the target list but the resource is not found (since it belongs to a different region). A hint or error message may indicate the resource is not in the current region. |
| Q.1.3 | The user lacks IAM permissions to describe the related resource type (e.g., can view EC2 but not VPCs). | After navigating to the VPC list, a red error flash appears in the header (e.g., "Error: AccessDenied"). The EC2 list remains accessible via Esc. |

---

## R. Terminal Resize

### R.1 Resize During Related Resources Picker

| ID | Story | Expected |
|----|-------|----------|
| R.1.1 | The related resources picker is open. I resize the terminal. | The picker re-renders at the new size. All entries remain visible (scrolling if needed). |
| R.1.2 | I resize below minimum dimensions (< 60 cols or < 7 rows) while the picker is open. | The "Terminal too narrow/short" error appears. Resizing back above minimum restores the picker. |

---

## S. Key Binding Coverage Summary

Every key binding related to cross-resource navigation appears in at least one story:

| Key | Stories |
|-----|---------|
| `R` (open related picker) | A.1.1, A.1.5, B.1.1-B.1.6, C.1.1, D.1.1, E.1.1, F.1.1, G.1.1, H.1.1, I.1.1, J.1.1, K.1.1 |
| `j`/down (navigate picker) | A.2.1 |
| `k`/up (navigate picker) | A.2.1 |
| `g` (jump to top in picker) | A.2.5 |
| `G` (jump to bottom in picker) | A.2.6 |
| `Enter` (select related resource) | A.3.1-A.3.4, B.1.1-B.1.6 |
| `Esc` (close picker / back) | A.2.4, N.1.2-N.1.4 |
| `/` (filter in picker) | P.1.1 |
| `:` (command in picker) | P.1.3 |
| `?` (help shows R key) | O.1.1, O.1.2 |

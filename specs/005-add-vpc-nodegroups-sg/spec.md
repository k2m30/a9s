# Feature Specification: Add VPC, EKS Node Groups, and Security Groups

**Feature Branch**: `005-add-vpc-nodegroups-sg`
**Created**: 2026-03-18
**Status**: Draft
**Input**: User description: "I want to add VPC, Node Groups, and Security Groups to list of resources available."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse VPCs (Priority: P1)

As a DevOps engineer, I want to see all VPCs in the current AWS region so I can quickly identify networking boundaries, CIDR blocks, and VPC state without switching to the AWS Console.

**Why this priority**: VPCs are the foundational networking primitive. Security Groups and Node Groups both exist within VPCs, so VPC visibility is prerequisite context for the other two resource types.

**Independent Test**: Can be fully tested by selecting "VPC" from the main menu, viewing a list of VPCs with key columns, and drilling into detail view for a specific VPC.

**Acceptance Scenarios**:

1. **Given** the user is on the main menu, **When** they select the VPC resource type, **Then** all VPCs in the current region are listed with columns: VPC ID, Name, CIDR Block, State, and Is Default.
2. **Given** the user is viewing the VPC list, **When** they select a VPC, **Then** the detail view shows all VPC attributes including tags, DHCP options, tenancy, and DNS settings.
3. **Given** the user is viewing the VPC list, **When** they press `y`, **Then** the full VPC YAML representation is displayed.

---

### User Story 2 - Browse Security Groups (Priority: P1)

As a DevOps engineer, I want to see all Security Groups in the current region so I can audit network access rules, identify overly permissive groups, and verify security posture without the AWS Console.

**Why this priority**: Security Groups are critical for network security auditing and are one of the most frequently inspected resources. Co-equal priority with VPCs since they are independently valuable.

**Independent Test**: Can be fully tested by selecting "Security Groups" from the main menu, viewing a list with key columns, and drilling into detail view for a specific Security Group.

**Acceptance Scenarios**:

1. **Given** the user is on the main menu, **When** they select the Security Groups resource type, **Then** all Security Groups in the current region are listed with columns: Group ID, Group Name, VPC ID, and Description.
2. **Given** the user is viewing the Security Group list, **When** they select a Security Group, **Then** the detail view shows all attributes including inbound rules, outbound rules, tags, and associated VPC.
3. **Given** the user is viewing the Security Group list, **When** they press `y`, **Then** the full Security Group YAML representation is displayed including all ingress/egress rules.

---

### User Story 3 - Browse EKS Node Groups (Priority: P2)

As a DevOps engineer managing Kubernetes infrastructure, I want to see all EKS Managed Node Groups so I can monitor node group health, scaling configuration, and instance types across my clusters.

**Why this priority**: Node Groups depend on EKS clusters (already supported) and are relevant to a narrower audience (only users running EKS). Still high value for Kubernetes operators.

**Independent Test**: Can be fully tested by selecting "Node Groups" from the main menu, viewing a list with key columns, and drilling into detail view for a specific Node Group.

**Acceptance Scenarios**:

1. **Given** the user is on the main menu, **When** they select the Node Groups resource type, **Then** all managed Node Groups across all EKS clusters in the current region are listed with columns: Node Group Name, Cluster Name, Status, Instance Types, and Desired Size.
2. **Given** the user is viewing the Node Group list, **When** they select a Node Group, **Then** the detail view shows all attributes including scaling config, AMI type, disk size, subnets, labels, and taints.
3. **Given** there are multiple EKS clusters, **When** the Node Groups list loads, **Then** Node Groups from all clusters are aggregated into a single list.
4. **Given** the user is viewing the Node Group list, **When** they press `y`, **Then** the full Node Group YAML representation is displayed.

---

### Edge Cases

- What happens when there are no VPCs in the region? Display an empty list with a "No resources found" message (consistent with existing resource types).
- What happens when there are no EKS clusters (and therefore no Node Groups)? Display an empty list — do not show an error.
- What happens when an EKS cluster exists but has no managed Node Groups? Display an empty list.
- What happens when the user lacks IAM permissions for one of the new resource types? Display the appropriate AWS permission error, consistent with how existing resources handle permission errors.
- How does the system handle Security Groups with hundreds of ingress/egress rules? The YAML and detail views should display all rules; the list view shows summary columns only.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST display VPCs as a selectable resource type in the main menu with short name "vpc" and aliases ["vpc", "vpcs"].
- **FR-002**: System MUST display Security Groups as a selectable resource type in the main menu with short name "sg" and aliases ["sg", "securitygroups", "security-groups"].
- **FR-003**: System MUST display EKS Node Groups as a selectable resource type in the main menu with short name "nodegroups" and aliases ["nodegroups", "ng", "node-groups"].
- **FR-004**: System MUST fetch VPCs using the AWS DescribeVpcs API and display VPC ID, Name (from tags), CIDR Block, State, and Is Default as list columns.
- **FR-005**: System MUST fetch Security Groups using the AWS DescribeSecurityGroups API and display Group ID, Group Name, VPC ID, and Description as list columns.
- **FR-006**: System MUST fetch EKS Node Groups by first listing all EKS clusters, then calling DescribeNodegroup for each cluster's node groups, and display Node Group Name, Cluster Name, Status, Instance Types, and Desired Size as list columns.
- **FR-007**: System MUST provide detail views for all three resource types showing all available AWS attributes, consistent with existing resource type detail views.
- **FR-008**: System MUST provide YAML views for all three resource types showing the full AWS SDK struct representation.
- **FR-009**: System MUST support filtering and sorting on all new resource type list views, consistent with existing resource types.
- **FR-010**: System MUST register all three new resource types using the existing resource registry pattern (init() + resource.Register()).

### Key Entities

- **VPC**: A Virtual Private Cloud network — identified by VPC ID, contains CIDR block, state, tenancy, DNS settings, and tags.
- **Security Group**: A virtual firewall for controlling inbound and outbound traffic — identified by Group ID, associated with a VPC, contains ingress and egress rules.
- **EKS Node Group**: A managed group of EC2 instances within an EKS cluster — identified by Node Group name + Cluster name, contains scaling configuration, instance types, AMI type, and labels/taints.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can browse VPCs, Security Groups, and Node Groups from the main menu with the same interaction patterns as existing resource types (select, detail, YAML views).
- **SC-002**: All three new resource types appear in the main menu and are accessible via their short names and aliases.
- **SC-003**: List views for all three resource types load and display data within the same timeframe as existing resource types under equivalent conditions.
- **SC-004**: All existing resource types continue to function identically after the addition (no regressions).
- **SC-005**: Unit test coverage for the three new resource types matches the coverage standard of existing resource types (fetchers, list rendering, detail views).

## Assumptions

- **Node Groups scope**: Only EKS Managed Node Groups are in scope. Self-managed node groups and Fargate profiles are excluded.
- **VPC Name column**: The "Name" column for VPCs is extracted from the `Name` tag (standard AWS convention), not a native VPC field.
- **Security Group rules in list**: Individual ingress/egress rules are shown only in detail/YAML views, not in the list columns. The list view focuses on identification columns.
- **Existing patterns**: All three resource types follow the identical architectural patterns as the existing 7 resource types (registry, fetcher, views.yaml config, etc.).
- **No cross-resource navigation**: This feature does not add the ability to navigate from a VPC to its Security Groups or from an EKS cluster to its Node Groups. Each resource type is browsed independently.

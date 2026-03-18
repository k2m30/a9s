# Sensitive Data Cleanup Plan

**Date:** 2026-03-18
**Priority:** URGENT / SECURITY
**Status:** Plan — not yet executed

---

## 1. Problem Statement

Real AWS infrastructure data from the `gobubble-dev` account (872515270585, eu-west-2) has been hardcoded into Go source files that are tracked by git and will be pushed to a public repository. This includes real account IDs, VPC IDs, Security Group IDs, subnet IDs, ARNs, IP ranges, cluster names, endpoint URLs, launch template IDs, and organizational resource names.

The project already has a pattern of `.gitignored` JSON fixture files in `tests/testdata/fixtures/` for raw AWS data. However, the Go fixture files (`fixtures_vpc.go`, `fixtures_sg.go`, `fixtures_nodegroups.go`) and their corresponding test files bypassed this pattern by embedding real data directly in committed Go code.

Additionally, the ORIGINAL fixture data in `tests/unit/fixtures_test.go` — which predates the VPC/SG/NodeGroup work — also contains real gobubble-dev data (real instance IDs, bucket names, IP addresses, endpoints, secret names). This file was never sanitized either.

---

## 2. Audit Results — Files Containing Real AWS Data

### CRITICAL — Go fixture files with raw AWS SDK structs (committed to git)

| File | Sensitive Data Types | Severity |
|------|---------------------|----------|
| `tests/testdata/fixtures_vpc.go` | Account ID (872515270585), 2 real VPC IDs, DHCP option ID, CIDR 10.10.0.0/16, CIDR assoc IDs, tag "vpc-dev" | CRITICAL |
| `tests/testdata/fixtures_sg.go` | Account ID (872515270585) x50+, 21 real SG IDs, 21 real SG ARNs, real VPC IDs, real IP CIDRs (10.10.x.x), real org resource names (emo-ai-dev, multimedia-mod-dev, gh-ci-ubuntu), SSM config paths, karpenter tags | CRITICAL |
| `tests/testdata/fixtures_nodegroups.go` | Account ID (872515270585), cluster name eks-eu-west-2-dev, 3 real node group ARNs, 3 real IAM role ARNs, 3 real subnet IDs, 3 real launch template IDs, real ASG names | CRITICAL |

### CRITICAL — Test files asserting on real AWS values

| File | Sensitive Data Types | Severity |
|------|---------------------|----------|
| `tests/unit/aws_vpc_test.go` | Asserts on real VPC IDs (vpc-05a7ebc57fd26ae33, vpc-09f63b3acf1049f1d), account ID 872515270585, real DHCP option ID, real CIDR 10.10.0.0/16 | CRITICAL |
| `tests/unit/aws_sg_test.go` | Lists all 21 real SG IDs with comments naming real resources, asserts on real ARNs, account ID, real resource names (eks-eu-west-2-dev-node), real tags | CRITICAL |
| `tests/unit/aws_nodegroups_test.go` | Asserts on real cluster name eks-eu-west-2-dev, real node group ARN, real subnet list, real tag values | CRITICAL |
| `tests/unit/fixtures_test.go` | Real S3 bucket names (cdn-cloudfront-logs.gobubble.cloud, gobubble-dev-fileshare, etc.), real EC2 instance IDs (6x), real IPs (10.10.x.x, 35.176.64.234, 13.43.142.76), real RDS/DocDB endpoints with cluster hash (cziaoicgy5um), real EKS endpoint hash (9F74DCF3EA0BB96E...), real secret names | CRITICAL |

### HIGH — Test files referencing real data from fixtures_test.go

| File | Sensitive Data Types |
|------|---------------------|
| `tests/unit/qa_ec2_test.go` | Real instance IDs (i-095a865e6d3afffa0, i-0974bcb534dc7245d), real IPs (10.10.48.175, 35.176.64.234, 13.43.142.76) |
| `tests/unit/qa_s3_test.go` | Real bucket names (auth-service-dev-state, gobubble-dev-fileshare) |
| `tests/unit/qa_rds_test.go` | Real DB identifiers (docdb-docdb-dev, rds-eu-west-2-dev-instance) |
| `tests/unit/qa_eks_secrets_test.go` | Real cluster name (eks-eu-west-2-dev), real secret names |
| `tests/unit/qa_detail_paths_test.go` | Comment referencing "gobubble-dev AWS account" |
| `tests/unit/qa_profile_switch_test.go` | Uses "gobubble-dev" and "gobubble-prod" as profile names (lower risk — profile names are not infrastructure data, but do reveal org naming) |

### MEDIUM — Design/preview files with real data baked into wireframes

| File | Sensitive Data Types |
|------|---------------------|
| `docs/design/detail-view.md` | Real instance ID (i-095a865e6d3afffa0), real VPC ID, real subnet ID, real AMI ID, real SG ID, real IPs (10.10.48.175, 35.176.64.234) |
| `cmd/preview_detail/main.go` | Same real values as detail-view.md — this is the visual preview binary |

### LOW — Documentation with real data in examples

| File | Sensitive Data Types |
|------|---------------------|
| `docs/qa/qa-test-tasks.md` | References real instance IDs, bucket names, DB identifiers, cluster names (used as "expected assertion values") |

### SAFE — Already using dummy/generic data

| File | Status |
|------|--------|
| `cmd/preview/main.go` | Uses dummy data (vpc-0123456789abcdef0, i-0abc123def456789a, etc.) — CLEAN |
| `specs/005-add-vpc-nodegroups-sg/qa-stories.md` | Uses dummy patterns (vpc-0123456789abcdef0, sg-0abc123def456789a) — CLEAN |
| `docs/qa/02-s3-views.md` through `docs/qa/09-yaml-all-types.md` | Uses generic dummy data — CLEAN |
| `tests/unit/qa_detail_test.go` | Uses dummy patterns (i-0abcdef1234567890) — CLEAN |
| `tests/unit/qa_configurable_views_test.go` | Uses dummy patterns — CLEAN |

### NOT TRACKED (safe) — JSON fixtures

| File | Status |
|------|--------|
| `tests/testdata/fixtures/*.json` | Contains real data BUT is .gitignored — SAFE |

---

## 3. Sanitization Mapping

All real values must be replaced with structurally equivalent dummy values. The dummy values must:
- Be obviously fake (use well-known test patterns)
- Preserve the same format/length so rendering tests still pass
- Be internally consistent (same VPC ID referenced across fixtures)

### Account & Owner IDs

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `872515270585` | `123456789012` |

### VPC IDs

| Real Value | Dummy Replacement | Context |
|-----------|-------------------|---------|
| `vpc-05a7ebc57fd26ae33` | `vpc-0aaa1111bbb2222cc` | Non-default VPC ("vpc-dev") |
| `vpc-09f63b3acf1049f1d` | `vpc-0ddd3333eee4444ff` | Default VPC |

### Security Group IDs

| Real Value | Dummy Replacement | Context |
|-----------|-------------------|---------|
| `sg-0d36c01884a154c6f` | `sg-0aa0000000000001a` | docdb-sg |
| `sg-03b92880991b76838` | `sg-0aa0000000000002b` | allow-node-to-node |
| `sg-06cde63f293698711` | `sg-0aa0000000000003c` | msk-sg |
| `sg-0276e3bec4d38cee8` | `sg-0aa0000000000004d` | efs-sg |
| `sg-0a6989528278ab20f` | `sg-0aa0000000000005e` | eks-node |
| `sg-021c912c3b2dd02a8` | `sg-0aa0000000000006f` | ci-runner-ubuntu |
| `sg-0cc5b576a311dcfb3` | `sg-0aa0000000000007a` | elasticache |
| `sg-01080ee6437bc8c8d` | `sg-0aa0000000000008b` | allow-http-https-ssh |
| `sg-0cc3780e3224356bf` | `sg-0aa0000000000009c` | eks-cluster-sg |
| `sg-0a93883c5f8857ac4` | `sg-0aa000000000000ad` | vpc-endpoints |
| `sg-0a92fddc651866279` | `sg-0aa000000000000be` | efs-media |
| `sg-09d186c4b5d043a40` | `sg-0aa000000000000cf` | ingress-external |
| `sg-0deb3b3b23c156330` | `sg-0aa000000000000d0` | shared-backend |
| `sg-0080ac62258abcbe4` | `sg-0aa000000000000e1` | ingress-internal |
| `sg-0a8a0463c3e109c58` | `sg-0aa000000000000f2` | vpn-sg |
| `sg-0035d69559ce47014` | `sg-0aa0000000000010a` | ci-runner |
| `sg-0481daf9bccc8c0d6` | `sg-0aa0000000000011b` | default-vpc-default |
| `sg-0095316d8a4f3d1ee` | `sg-0aa0000000000012c` | eks-cluster |
| `sg-0c8f36991a9fcaa21` | `sg-0aa0000000000013d` | launch-wizard |
| `sg-00cef816993723496` | `sg-0aa0000000000014e` | vpc-dev-default |
| `sg-083aab8496b428d22` | `sg-0aa0000000000015f` | rds |

### Subnet IDs

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `subnet-0ae38cf9d0176f297` | `subnet-0aaa111111111111a` |
| `subnet-0e89f41b0eba1e56a` | `subnet-0bbb222222222222b` |
| `subnet-022b2e78940e0c9e5` | `subnet-0ccc333333333333c` |
| `subnet-09cab89cf4911785f` | `subnet-0ddd444444444444d` |

### DHCP Option IDs

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `dopt-08b8d47ce2a8c5cd4` | `dopt-0aaa111111111111a` |

### VPC CIDR Association IDs

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `vpc-cidr-assoc-098b0904f4cd77015` | `vpc-cidr-assoc-0aaa11111111111a` |
| `vpc-cidr-assoc-03115ad21bece7b28` | `vpc-cidr-assoc-0bbb22222222222b` |

### IP Addresses & CIDRs

| Real Value | Dummy Replacement | Notes |
|-----------|-------------------|-------|
| `10.10.0.0/16` | `10.0.0.0/16` | VPC CIDR |
| `10.10.0.0/20` | `10.0.0.0/20` | Subnet CIDR |
| `10.10.16.0/20` | `10.0.16.0/20` | Subnet CIDR |
| `10.10.32.0/20` | `10.0.32.0/20` | Subnet CIDR |
| `10.10.48.0/24` | `10.0.48.0/24` | Subnet CIDR |
| `10.10.49.0/24` | `10.0.49.0/24` | Subnet CIDR |
| `10.10.50.0/24` | `10.0.50.0/24` | Subnet CIDR |
| `10.10.48.175` | `10.0.48.175` | EC2 private IP |
| `10.10.48.186` | `10.0.48.186` | EC2 private IP |
| `10.10.12.47` | `10.0.12.47` | EC2 private IP |
| `10.10.0.32` | `10.0.0.32` | EC2 private IP |
| `10.10.3.140` | `10.0.3.140` | EC2 private IP |
| `35.176.64.234` | `203.0.113.10` | EC2 public IP (use RFC 5737 TEST-NET-3) |
| `13.43.142.76` | `203.0.113.20` | EC2 public IP |

### EC2 Instance IDs

| Real Value | Dummy Replacement | Context |
|-----------|-------------------|---------|
| `i-0974bcb534dc7245d` | `i-0aaa111111111111a` | unnamed GPU instance |
| `i-095a865e6d3afffa0` | `i-0bbb222222222222b` | VPN |
| `i-02a94d4e56f0c10bc` | `i-0ccc333333333333c` | kafka |
| `i-08231635bfaf16ad9` | `i-0ddd444444444444d` | monitoring |
| `i-0dce06b25e765504c` | `i-0eee555555555555e` | apps-on-demand |
| `i-08d92d2b43e1a56bc` | `i-0fff666666666666f` | apps (terminated) |

### AMI IDs

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `ami-08f79bee58074adeb` | `ami-0aaa111111111111a` |

### EKS Cluster Names

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `eks-eu-west-2-dev` | `test-cluster-1` |

### Node Group Names & ARNs

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `gpu-20250529125537419200000001` | `gpu-20250101120000000000000001` |
| `kafka-20250606103349821200000005` | `kafka-20250101120000000000000002` |
| `kube-system-20250606080408742900000019` | `system-20250101120000000000000003` |
| All node group ARNs | Reconstruct with `arn:aws:eks:us-east-1:123456789012:nodegroup/test-cluster-1/{name}/{uuid}` |
| All IAM role ARNs | `arn:aws:iam::123456789012:role/{name}-role` |

### Launch Template IDs

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `lt-0882bfd79436d9164` | `lt-0aaa111111111111a` |
| `lt-0957718588c814e99` | `lt-0bbb222222222222b` |
| `lt-056db7a53acc1974f` | `lt-0ccc333333333333c` |

### ASG Names

Replace all real ASG names (`eks-gpu-...`, `eks-kafka-...`, `eks-kube-system-...`) with corresponding dummy names using the sanitized node group names.

### S3 Bucket Names

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `auth-service-dev-state` | `test-app-state` |
| `cdn-cloudfront-logs.gobubble.cloud` | `cdn-logs.example.com` |
| `cdn-test-website.gobubble.cloud` | `cdn-website.example.com` |
| `gobubble-dev-fileshare` | `dev-fileshare` |
| `gobubble-dev-loki-chunks` | `dev-loki-chunks` |

### RDS/DocDB Endpoints

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `docdb-docdb-dev.cziaoicgy5um.eu-west-2.docdb.amazonaws.com` | `test-docdb-1.cluster-abc123def.us-east-1.docdb.amazonaws.com` |
| `rds-eu-west-2-dev-instance.cziaoicgy5um.eu-west-2.rds.amazonaws.com` | `test-rds-1.cluster-abc123def.us-east-1.rds.amazonaws.com` |
| `docdb-cluster-dev.cluster-cziaoicgy5um.eu-west-2.docdb.amazonaws.com` | `test-docdb-cluster.cluster-abc123def.us-east-1.docdb.amazonaws.com` |
| `rds-eu-west-2-dev.cluster-cziaoicgy5um.eu-west-2.rds.amazonaws.com` | `test-rds-cluster.cluster-abc123def.us-east-1.rds.amazonaws.com` |

### EKS Endpoint

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `https://9F74DCF3EA0BB96E39BC82293834A8BB.gr7.eu-west-2.eks.amazonaws.com` | `https://ABCDEF0123456789ABCDEF0123456789.gr7.us-east-1.eks.amazonaws.com` |

### RDS/DocDB/Redis Identifiers

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `docdb-docdb-dev` | `test-docdb-1` |
| `rds-eu-west-2-dev-instance` | `test-rds-1` |
| `docdb-cluster-dev` | `test-docdb-cluster` |
| `rds-eu-west-2-dev` | `test-rds-cluster` |
| `elasticache-dev` | `test-redis-1` |

### Secret Names

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `integration_test` | `test/integration` |
| `dev-gihub-app-ubuntu` | `test/github-app` |
| `docdb-dev-credentials` | `test/docdb-credentials` |
| `elasticache-dev-creds` | `test/redis-credentials` |
| `rds-dev-creds` | `test/rds-credentials` |

### SG Group Names (org-identifying)

| Real Value | Dummy Replacement |
|-----------|-------------------|
| `Migration-security-group` | `migration-sg` |
| `allow-node-to-node-traffic-20250605...` | `node-to-node-traffic` |
| `emo-ai-dev-efs` | `app-efs` |
| `msk-dev-sg` | `msk-sg` |
| `gh-ci-ubuntu-ubuntu-x64-github-actions-runner-sg...` | `ci-runner-ubuntu-sg` |
| `gh-ci-github-actions-runner-sg...` | `ci-runner-sg` |
| `multimedia-mod-dev-efs` | `media-efs` |
| `vpn-allow-http-https-ssh` | `vpn-sg` |
| `k8s-ingress-external-816e71c642` | `k8s-ingress-external` |
| `k8s-traffic-ekseuwest2dev-79e10bb328` | `k8s-traffic-shared` |
| `k8s-ingress-internal-179e0a770c` | `k8s-ingress-internal` |
| `launch-wizard-1` | `launch-wizard-1` (generic enough to keep) |
| `eks-eu-west-2-dev-node-...` | `test-cluster-1-node` |
| `eks-eu-west-2-dev-cluster-...` | `test-cluster-1-cluster` |
| `eks-cluster-sg-eks-eu-west-2-dev-...` | `eks-cluster-sg-test-cluster-1` |
| `vpc-dev-vpc-endpoints` | `vpc-endpoints` |

### Tag Values (org-identifying)

| Real Pattern | Dummy Replacement |
|-------------|-------------------|
| `kubernetes.io/cluster/eks-eu-west-2-dev` | `kubernetes.io/cluster/test-cluster-1` |
| `karpenter.sh/discovery: eks-eu-west-2-dev` | `karpenter.sh/discovery: test-cluster-1` |
| `elbv2.k8s.aws/cluster: eks-eu-west-2-dev` | `elbv2.k8s.aws/cluster: test-cluster-1` |
| `aws:eks:cluster-name: eks-eu-west-2-dev` | `aws:eks:cluster-name: test-cluster-1` |
| `ghr:ssm_config_path: /github-action-runners/...` | `ghr:ssm_config_path: /ci-runners/default/config` |
| `ghr:environment: gh-ci-ubuntu-ubuntu-x64` | `ghr:environment: ci-runner-ubuntu` |
| `ghr:environment: gh-ci` | `ghr:environment: ci-runner` |
| `Name: docdb-sg` | `Name: docdb-sg` (generic enough) |
| `Name: vpc-dev` | `Name: dev-vpc` |

### Profile Names

| Real Value | Dummy Replacement | Notes |
|-----------|-------------------|-------|
| `gobubble-dev` | Keep as-is in agent config | Only sanitize in test assertions and comments |
| `gobubble-prod` | `staging-profile` | In test assertions |

---

## 4. Decision: Go Fixtures vs JSON

### Recommendation: Sanitize the Go fixtures in-place

The three new Go fixture files (`fixtures_vpc.go`, `fixtures_sg.go`, `fixtures_nodegroups.go`) return typed AWS SDK structs (`ec2types.Vpc`, `ec2types.SecurityGroup`, `ekstypes.Nodegroup`). Converting these to JSON-loaded fixtures would require writing JSON deserialization code for complex AWS SDK types with enums, nested structs, and pointers. This is unnecessary complexity.

**Decision:** Replace all real values in the Go fixture files with sanitized dummy values from the mapping above. The Go fixtures stay as committed Go code, but with zero real infrastructure data.

The existing `tests/unit/fixtures_test.go` returns `resource.Resource` structs (not raw AWS SDK types), so the same approach applies — sanitize in-place.

The `.gitignored` JSON files in `tests/testdata/fixtures/` can continue to hold real data for manual debugging — they are never committed.

---

## 5. File-by-File Cleanup Instructions

### Phase 1: Core fixture files (CRITICAL)

#### 5.1 `tests/testdata/fixtures_vpc.go`
- Replace account ID `872515270585` -> `123456789012`
- Replace VPC IDs per mapping
- Replace DHCP option ID per mapping
- Replace CIDR `10.10.0.0/16` -> `10.0.0.0/16`
- Replace CIDR assoc IDs per mapping
- Replace tag `vpc-dev` -> `dev-vpc`
- Update comment header: remove "Account: 872515270585", change to "Account: 123456789012 (sanitized)"
- Remove "gobubble-dev" from source comment

#### 5.2 `tests/testdata/fixtures_sg.go`
- Replace ALL instances of `872515270585` -> `123456789012`
- Replace ALL 21 SG IDs per mapping
- Replace ALL SG ARNs (reconstruct with dummy account + region + SG ID)
- Replace both VPC IDs per mapping
- Replace ALL CIDRs (10.10.x.x -> 10.0.x.x)
- Replace ALL group names per mapping
- Replace ALL tag keys/values per mapping
- Update comment header

#### 5.3 `tests/testdata/fixtures_nodegroups.go`
- Replace account ID
- Replace cluster name `eks-eu-west-2-dev` -> `test-cluster-1`
- Replace all node group names per mapping
- Replace all ARNs (nodegroup ARNs, IAM role ARNs)
- Replace all subnet IDs per mapping
- Replace all launch template IDs per mapping
- Replace all ASG names
- Replace all tag values
- Update comment header

### Phase 2: Test files asserting on real values (CRITICAL)

#### 5.4 `tests/unit/aws_vpc_test.go`
- Update all assertions to use sanitized VPC IDs, account ID, DHCP option ID, CIDR
- Update test function name/comment: remove "gobubble-dev" reference

#### 5.5 `tests/unit/aws_sg_test.go`
- Update the full SG ID list (21 entries) to use sanitized IDs
- Update all field assertions (Name, ARN, VPC ID, Owner ID, tags)
- Update test name/comments

#### 5.6 `tests/unit/aws_nodegroups_test.go`
- Update cluster name assertions
- Update ARN assertions
- Update subnet assertions
- Update tag assertions
- Update test name/comments
- Note: Lines ~200-280 use already-sanitized dummy data (arn:aws:eks:us-east-1:123456789012:...) — leave these alone

#### 5.7 `tests/unit/fixtures_test.go`
- Replace ALL S3 bucket names per mapping
- Replace ALL EC2 instance IDs per mapping
- Replace ALL private/public IPs per mapping
- Replace ALL RDS/DocDB/Redis identifiers and endpoints per mapping
- Replace EKS cluster name and endpoint per mapping
- Replace ALL secret names per mapping
- Update comments: remove "gobubble-dev" references

### Phase 3: Test files with transitive references (HIGH)

#### 5.8 `tests/unit/qa_ec2_test.go`
- Replace instance IDs (i-0974bcb534dc7245d, i-095a865e6d3afffa0)
- Replace IPs (10.10.48.175, 35.176.64.234, 13.43.142.76, 10.10.48)
- These values must match the sanitized `fixtures_test.go`

#### 5.9 `tests/unit/qa_s3_test.go`
- Replace bucket names (auth-service-dev-state, gobubble patterns)
- Must match sanitized `fixtures_test.go`

#### 5.10 `tests/unit/qa_rds_test.go`
- Replace DB identifiers (docdb-docdb-dev)
- Must match sanitized `fixtures_test.go`

#### 5.11 `tests/unit/qa_eks_secrets_test.go`
- Replace cluster name (eks-eu-west-2-dev)
- Replace secret names
- Must match sanitized `fixtures_test.go`

#### 5.12 `tests/unit/qa_detail_paths_test.go`
- Remove "gobubble-dev" from comment on line 17

### Phase 4: Design/preview files (MEDIUM)

#### 5.13 `docs/design/detail-view.md`
- Replace real instance ID `i-095a865e6d3afffa0` -> `i-0bbb222222222222b` (or use generic `i-0abc123def456789a`)
- Replace real VPC ID -> `vpc-0aaa1111bbb2222cc`
- Replace real subnet ID -> `subnet-0ddd444444444444d`
- Replace real AMI ID -> `ami-0aaa111111111111a`
- Replace real SG ID -> `sg-0aa000000000000f2`
- Replace real IPs -> 10.0.48.175, 203.0.113.10

NOTE: Since this is a design wireframe, using the canonical dummy patterns (vpc-0123456789abcdef0, etc.) that already appear in `cmd/preview/main.go` and the QA stories is preferred for consistency.

#### 5.14 `cmd/preview_detail/main.go`
- Same replacements as detail-view.md — this file renders the wireframe

#### 5.15 `docs/qa/qa-test-tasks.md`
- Update all referenced instance IDs, bucket names, DB identifiers, cluster names to match sanitized fixture values

---

## 6. Updated a9s-fixtures Agent Instructions

The `a9s-fixtures` agent instructions at `.claude/agents/a9s-fixtures.md` must be updated. Currently line 118 says:

> gobubble-dev is a test account. Keep all real IDs, names, ARNs, IPs, endpoints, timestamps.

This must be replaced with sanitization instructions.

### New Section: "Post-Fetch Sanitization" (replaces "No Sanitization")

```markdown
## Post-Fetch Sanitization

After fetching data, the JSON files in `tests/testdata/fixtures/` are .gitignored
and may contain real data for local debugging. However, ANY Go fixture files or
test files that reference this data MUST be sanitized before commit.

When creating Go fixture files from fetched data, apply these replacements:

| Pattern | Replacement |
|---------|-------------|
| Real AWS account IDs (12-digit) | `123456789012` |
| Real VPC IDs (`vpc-xxx`) | `vpc-0aaa1111bbb2222cc` (numbered sequentially) |
| Real SG IDs (`sg-xxx`) | `sg-0aa000000000000Na` (numbered sequentially) |
| Real subnet IDs (`subnet-xxx`) | `subnet-0aaa111111111111a` (numbered) |
| Real ARNs | Reconstruct with `123456789012`, `us-east-1`, and generic resource names |
| Real IP addresses (10.x.x.x) | `10.0.x.x` (keep last two octets) |
| Real public IPs | `203.0.113.x` (RFC 5737 TEST-NET-3) |
| Real cluster/resource names with org prefixes | Generic names (test-cluster-1, etc.) |
| Real DHCP option IDs | `dopt-0aaa111111111111a` |
| Real launch template IDs | `lt-0aaa111111111111a` (numbered) |
| Real endpoints with cluster hashes | Replace hash with `abc123def` |
| Organization-specific names (gobubble, emo-ai, etc.) | Generic equivalents |

NEVER commit real AWS infrastructure data to tracked Go files.
```

---

## 7. Prevention: Going Forward

### Rule 1: No real data in committed code
Any Go file (`.go`) that is tracked by git must use sanitized dummy values. Real data lives only in `.gitignored` JSON files.

### Rule 2: a9s-fixtures sanitizes on output
When the a9s-fixtures agent creates Go fixture data from AWS API responses, it must apply the sanitization mapping before writing Go files. The raw JSON stays real (for debugging); the Go representation is sanitized.

### Rule 3: Pre-commit verification
Consider adding a CI check or pre-commit hook that scans for the known real account ID (`872515270585`) in any `.go`, `.md`, or `.yaml` file. If found, fail the check.

### Rule 4: Agent review gate
The a9s-tui-reviewer agent must check for real AWS data patterns during code review. Any PR containing real VPC IDs, SG IDs, account IDs, or org-specific names (gobubble, emo-ai, etc.) must be rejected.

---

## 8. Execution Order

1. **Phase 1** first — sanitize the three `testdata/fixtures_*.go` files
2. **Phase 2** — update test files that assert on real values from those fixtures
3. **Phase 3** — update transitive test files that reference fixture data
4. **Phase 4** — update design docs and preview binaries
5. **Update a9s-fixtures agent** — change `.claude/agents/a9s-fixtures.md`
6. **Run full test suite** — `go test ./tests/unit/ -count=1 -timeout 120s`
7. **Verify** — grep the entire repo for `872515270585` and other sentinel values to confirm zero leaks
8. **Commit** — single commit with message: "security: sanitize all real AWS data from committed files"

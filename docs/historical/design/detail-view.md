# Detail View Indentation Fix

Version: 1.3
Scope: `internal/tui/views/detail.go` — `renderFromConfig` method only.

---

## 1. Problem

Scalar fields use a 3-space left margin. Section headers (structs and slices)
use a 1-space left margin. Top-level keys are misaligned.

```
   InstanceId:           i-0bbb222222222222b   ← col 4 (scalar)
 State:                                         ← col 2 (section)
     Code: "16"
     Name: running
   InstanceType:         t3.large               ← col 4 (scalar)
```

---

## 2. Fix

Change scalar field prefix in `renderFromConfig` from `"   "` (3 spaces) to
`" "` (1 space). Section headers already use `" "` — leave them unchanged.
Sub-field lines stay at `"     "` (5 spaces). No other changes.

---

## 3. Indentation Rules

| Level | Spaces | Applies to                                          | Example prefix  |
|-------|--------|-----------------------------------------------------|-----------------|
| 0     | 1      | All top-level keys: scalars and section headers     | `" "`           |
| 1     | 5      | First-level sub-fields; array item openers (`- …`) | `"     "`       |
| 2     | 9      | Second-level sub-fields (nested struct in array)    | `"         "`   |

---

## 4. Wireframes

### BEFORE (broken)

```
┌──────────────── i-0bbb222222222222b ────────────────────────────────────────┐
│   InstanceId:           i-0bbb222222222222b                                 │
│ State:                                                                       │
│     Code: "16"                                                               │
│     Name: running                                                            │
│   InstanceType:         t3.large                                             │
│   VpcId:                vpc-0aaa1111bbb2222cc                                │
│ SecurityGroups:                                                              │
│     - GroupId: sg-0aa000000000000f2                                          │
│       GroupName: vpn-sg                                    │
│   LaunchTime:           2025-07-25 12:26:50                                  │
│ Tags:                                                                        │
│     - Key: Name                                                              │
│       Value: VPN                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### AFTER (fixed) — EC2 with all nested cases

```
┌──────────────── i-0bbb222222222222b ────────────────────────────────────────┐
│ InstanceId:           i-0bbb222222222222b                                   │
│ State:                                                                       │
│     Code: "16"                                                               │
│     Name: running                                                            │
│ InstanceType:         t3.large                                               │
│ ImageId:              ami-0aaa111111111111a                                  │
│ VpcId:                vpc-0aaa1111bbb2222cc                                  │
│ SubnetId:             subnet-0ddd444444444444d                               │
│ PrivateIpAddress:     10.0.48.175                                           │
│ PublicIpAddress:      203.0.113.10                                          │
│ Placement:                                                                   │
│     AvailabilityZone: eu-west-2a                                             │
│     GroupName: ""                                                            │
│     Tenancy: default                                                         │
│ NetworkInterfaces:                                                           │
│     - NetworkInterfaceId: eni-0abc123                                        │
│       PrivateIpAddress: 10.0.1.5                                             │
│       SubnetId: subnet-0ddd444                                               │
│     - NetworkInterfaceId: eni-0def456                                        │
│       PrivateIpAddress: 10.0.2.10                                            │
│       SubnetId: subnet-0abc456                                               │
│ BlockDeviceMappings:                                                         │
│     - DeviceName: /dev/xvda                                                  │
│       Ebs:                                                                   │
│         VolumeId: vol-0abc123                                                │
│         Status: attached                                                     │
│ LaunchTime:           2025-07-25 12:26:50                                    │
│ Architecture:         x86_64                                                 │
│ Platform:             -                                                      │
│ Tags:                                                                        │
│     - Key: Name                                                              │
│       Value: web-server                                                      │
│     - Key: Environment                                                       │
│       Value: production                                                      │
└─────────────────────────────────────────────────────────────────────────────┘
```

Notes on EC2 wireframe:
- `State`, `Placement` — struct sections, sub-fields at level 1 (5 spaces)
- `NetworkInterfaces` — slice section, each `- …` opener at level 1, continuation fields at level 1 with `  ` prefix
- `BlockDeviceMappings` — slice where each item has a nested struct (`Ebs`); `Ebs:` opener at level 1, its fields at level 2 (9 spaces)
- `Tags` — simple key/value slice at level 1

### AFTER (fixed) — RDS with nested Endpoint and VpcSecurityGroups

```
┌──────────────────────── mydb-prod ──────────────────────────────────────────┐
│ DBInstanceIdentifier: mydb-prod                                             │
│ Engine:               postgres                                              │
│ EngineVersion:        15.4                                                  │
│ DBInstanceStatus:     available                                             │
│ DBInstanceClass:      db.t3.medium                                          │
│ Endpoint:                                                                   │
│     Address: mydb-prod.cluster-abc123.eu-west-2.rds.amazonaws.com          │
│     Port: 5432                                                              │
│     HostedZoneId: Z1TTGA775OQIAX                                            │
│ MultiAZ:              true                                                  │
│ VpcSecurityGroups:                                                          │
│     - VpcSecurityGroupId: sg-0abc123                                        │
│       Status: active                                                        │
└─────────────────────────────────────────────────────────────────────────────┘
```

Notes on RDS wireframe:
- `Endpoint` — nested struct section, sub-fields at level 1
- `VpcSecurityGroups` — slice section, array items at level 1

---

## 5. Implementation Note

In `renderFromConfig`, the `kv` helper (scalar field renderer) currently
prepends `"   "` (3 spaces). Change it to `" "` (1 space):

```go
// Before
return "   " + keyStyle.Render(padRight(key+":", keyColW)) + valStyle.Render(val)

// After
return " " + keyStyle.Render(padRight(key+":", keyColW)) + valStyle.Render(val)
```

Section header lines already use `" "` — no change needed there.
Sub-field lines already use `"     "` — no change needed there.
`subLine2` (second-level, 9 spaces) uses `"         "` — no change needed.

---

## 6. Color Palette (Tokyo Night Dark)

| Element            | Foreground | Background | Style  |
|--------------------|------------|------------|--------|
| Key (any level 0)  | `#7aa2f7`  | —          | normal |
| Value (scalar)     | `#c0caf5`  | —          | normal |
| Value (status OK)  | `#9ece6a`  | —          | normal |
| Value (status ERR) | `#f7768e`  | —          | normal |
| Value (status WRN) | `#e0af68`  | —          | normal |
| Sub-field text     | `#565f89`  | —          | dim    |
| Border             | `#414868`  | —          | normal |
| Title              | `#c0caf5`  | —          | bold   |

---

## 7. Layout

Single `lipgloss.RoundedBorder()` panel, 100% terminal width, scrollable via
`bubbles/viewport`. Content is a `strings.Builder` with one line per field —
no `JoinVertical`.

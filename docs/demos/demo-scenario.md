# a9s Demo Recording Scenario

Target: 40-45 seconds, loopable GIF + MP4 output via VHS.
Runs in `--demo` mode with synthetic fixture data. No real AWS credentials.
The GIF starts and ends on the main menu so it loops seamlessly.
The shell/launch command is never shown — recording begins inside the app.

---

## Terminal Setup

- Terminal size: 120x35 (wide enough for tables, tall enough for context)
- Font: SF Mono or JetBrains Mono, 14pt
- Theme: dark background (matches Tokyo Night)
- VHS `Hide` used during launch, recording starts after app is ready

---

## Script (timed)

### Act 1: Main Menu (0-4s)

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 0.0s | (already in app) | Main menu visible: header `a9s vX.Y.Z  demo:us-east-1`, categorized resource list |
| 1.5s | Pause | Viewer reads the categories (Compute, Networking, Databases, etc.) |
| 2.5s | Press `Down` x3 | Cursor moves through categories |

**Overlay:** "66 AWS resource types with live counts" / "↑ ↓  navigate"

### Act 2: EC2 Instances — List View (4-10s)

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 4.0s | Press `Enter` on EC2 | EC2 instance list — table with Name, Instance ID, Type, State, AZ |
| 5.5s | Pause | Status colors: green (running), red (stopped), yellow (pending) |
| 6.5s | Press `Down` x2 | Cursor moves down rows |
| 7.5s | Type `/web` | Filter narrows to instances matching "web" |
| 9.5s | Press `Esc` | Clear filter, full list back |

**Overlays:**
- "EC2 Instances" / "Enter  open resource list" (4-7.5s)
- "Filter resources instantly" / "/  search · Esc  clear" (8-10s)

### Act 3: EC2 Detail + YAML (10-20s)

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 10.5s | Press `d` | Detail view — key-value pairs for selected instance |
| 12.0s | Pause | InstanceId, InstanceType, State, PublicIP, VpcId, Tags |
| 13.0s | Press `Down` x3 | Scroll through detail fields |
| 14.5s | Press `y` | YAML view — syntax-colored full API response |
| 16.5s | Pause | Colored YAML: strings green, numbers orange, booleans purple |
| 18.5s | Press `Esc` x3 | Back: YAML → detail → list → menu |

**Overlays:**
- "Detail View — all instance fields" / "d  detail · ↓  scroll" (10.5-14.5s)
- "Full YAML — raw AWS API response" / "y  yaml view" (15-18.5s)

### Act 4: Related Views (20-30s) — NEW

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 20.0s | Press `Enter` on EC2 | Back into EC2 list |
| 21.5s | Press `Down`, then `d` | Open detail for an instance |
| 23.0s | Press `r` | Right column shows RELATED panel — Target Groups, ASGs, Alarms, EBS, EIPs, etc. with counts |
| 26.0s | Press `Down` x3, `Up` | Browse related items |
| 27.5s | Press `Enter` | Navigate to related resource (e.g., Target Group or EBS volume) |
| 29.5s | Press `Esc` | Back to EC2 detail |
| 30.0s | Press `r`, `Esc` x2 | Close related, back to menu |

**Overlays:**
- "Related Resources — cross-service connections" / "r  toggle related panel" (23-27s)
- "Navigate to any related resource" / "Enter  jump · ↑ ↓  browse" (27.5-30s)

### Act 5: S3 Drill-Down (30-37s)

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 31.0s | Type `:s3` + Enter | Command mode jump — bucket list |
| 33.0s | Pause | S3 buckets with Region, Creation Date |
| 34.0s | Press `Enter` | Drill into bucket — object list (folders + files) |
| 35.5s | Pause | Objects: Key, Size, Last Modified, Storage Class |
| 37.0s | Press `Esc` x2 | Back to buckets, back to main menu |

**Overlays:**
- "S3 Buckets" / ":s3  jump to any service" (31-34s)
- "Drill into bucket objects" / "Enter  child view" (34.5-37s)

### Act 6: Quick Tour + Return to Menu (37-44s)

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 38.0s | Type `:lambda` + Enter | Lambda functions — Runtime, Memory, Last Modified |
| 39.5s | Pause | Lambda list |
| 41.0s | Type `:rds` + Enter | RDS — DB instances with Engine, Status, Size |
| 42.5s | Pause | RDS instances with status colors |
| 43.0s | Press `Esc` | Back to main menu |
| 43.5s | Pause 2s | Main menu visible — same frame as start, loop point |

**Overlays:**
- "Lambda Functions" / ":lambda  jump to service" (38-40.5s)
- "RDS Databases" / ":rds  jump to service" (41-44s)

---

## Key Features Demonstrated

1. **Categorized main menu** — organized by AWS service category
2. **Arrow key navigation** — Down/Up/Left/Right + Enter/Esc (vim keys also work but not shown)
3. **Status colors** — green/red/yellow for resource states
4. **Filtering** — `/web` narrows results in real-time
5. **Detail view** — structured key-value display
6. **YAML view** — syntax-colored full API response
7. **Related views** — cross-service resource connections with navigation
8. **S3 drill-down** — bucket → objects hierarchy
9. **Command mode** — `:s3`, `:lambda`, `:rds` to jump anywhere
10. **Multi-resource** — EC2, S3, Lambda, RDS in one session

## Resources Shown

| Resource | View | Why |
|----------|------|-----|
| EC2 | List + Detail + YAML + Related | Core resource, shows all four views |
| S3 | List + Drill-down | Unique bucket → object navigation |
| Lambda | List | Shows serverless/compute category |
| RDS | List | Shows database category with status colors |

## Overlay Captions (10 total, two lines each)

| Act | Description line (white, 22pt) | Key hint line (blue, 16pt) | Timing |
|-----|-------------------------------|---------------------------|--------|
| 1 | 66 AWS resource types with live counts | ↑ ↓  navigate | 0.5-4s |
| 2a | EC2 Instances | Enter  open resource list | 5-7.5s |
| 2b | Filter resources instantly | /  search · Esc  clear | 8-10s |
| 3a | Detail View — all instance fields | d  detail · ↓  scroll | 10.5-14.5s |
| 3b | Full YAML — raw AWS API response | y  yaml view | 15-18.5s |
| 4a | Related Resources — cross-service connections | r  toggle related panel | 23-27s |
| 4b | Navigate to any related resource | Enter  jump · ↑ ↓  browse | 27.5-30s |
| 5a | S3 Buckets | :s3  jump to any service | 31-34s |
| 5b | Drill into bucket objects | Enter  child view | 34.5-37s |
| 6a | Lambda Functions | :lambda  jump to service | 38-40.5s |
| 6b | RDS Databases | :rds  jump to service | 41-44s |

## Fixture Requirements

Each resource shown needs:
- 5-8 realistic entries per resource type
- Mix of statuses (running/stopped/pending for EC2, available/creating for RDS)
- `RawStruct` populated for EC2 (detail + YAML views need it)
- Related demo fixtures registered for EC2 (66 resource types have related demos)
- Realistic but fake names (e.g., `web-prod-01`, `api-staging-02`, `data-pipeline-logs`)
- Fake but plausible IDs (e.g., `i-0a1b2c3d4e5f`, `sg-0abc123def`)

## Fixture Coverage

All 66 resource types have fixture functions with RawStruct.
All 66 resource types have RegisterRelatedDemo fixtures.
EC2 related-view demo shows: Target Groups (1), ASGs (1), Alarms (2), CFN (0), EIPs (1), EBS Snapshots (2), EBS Volumes (2), Node Groups (1), CloudTrail Events (1).

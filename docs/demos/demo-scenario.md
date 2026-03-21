# a9s Demo Recording Scenario

Target: 30-35 seconds, loopable GIF + MP4 output via VHS.
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
| 0.0s | (already in app) | Main menu visible: header `a9s v3.0.0  demo:us-east-1`, categorized resource list |
| 1.5s | Pause | Viewer reads the categories (Compute, Networking, Databases, etc.) |
| 2.5s | Press `Down` x3 | Cursor moves through categories |

### Act 2: EC2 Instances — List View (4-13s)

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 4.0s | Press `Enter` on EC2 | EC2 instance list — table with Name, Instance ID, Type, State, AZ |
| 5.5s | Pause | Status colors: green (running), red (stopped), yellow (pending) |
| 7.0s | Press `Down` x2 | Cursor moves down rows |
| 8.0s | Type `/web` | Filter narrows to instances matching "web" |
| 9.5s | Press `Esc` | Clear filter, full list back |

### Act 3: EC2 Detail + YAML (13-20s)

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 13.0s | Press `d` | Detail view — key-value pairs for selected instance |
| 14.5s | Pause | InstanceId, InstanceType, State, PublicIP, VpcId, Tags |
| 16.0s | Press `Down` x3 | Scroll through detail fields |
| 17.0s | Press `y` | YAML view — syntax-colored full API response |
| 18.5s | Pause | Colored YAML: strings green, numbers orange, booleans purple |
| 19.5s | Press `Esc` x2 | Back to list, then back to main menu |

### Act 4: S3 Drill-Down (20-26s)

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 20.0s | Type `:s3` + Enter | Command mode jump — bucket list |
| 21.5s | Pause | S3 buckets with Region, Creation Date |
| 22.5s | Press `Enter` | Drill into bucket — object list (folders + files) |
| 24.0s | Pause | Objects: Key, Size, Last Modified, Storage Class |
| 25.0s | Press `Esc` x2 | Back to buckets, back to main menu |

### Act 5: Quick Tour + Return to Menu (26-33s)

| Time | Action | What viewer sees |
|------|--------|-----------------|
| 26.0s | Type `:lambda` + Enter | Lambda functions — Runtime, Memory, Last Modified |
| 27.5s | Pause | Lambda list |
| 28.5s | Type `:rds` + Enter | RDS — DB instances with Engine, Status, Size |
| 30.0s | Pause | RDS instances with status colors |
| 31.0s | Press `Esc` | Back to main menu |
| 31.5s | Pause 1.5s | Main menu visible — same frame as start, loop point |

---

## Key Features Demonstrated

1. **Categorized main menu** — organized by AWS service category
2. **Arrow key navigation** — Down/Up/Left/Right + Enter/Esc (vim keys also work but not shown)
3. **Status colors** — green/red/yellow for resource states
4. **Filtering** — `/web` narrows results in real-time
5. **Detail view** — structured key-value display
6. **YAML view** — syntax-colored full API response
7. **S3 drill-down** — bucket → objects hierarchy
8. **Command mode** — `:s3`, `:lambda`, `:rds` to jump anywhere
9. **Multi-resource** — EC2, S3, Lambda, RDS in one session

## Resources Shown

| Resource | View | Why |
|----------|------|-----|
| EC2 | List + Detail + YAML | Core resource, shows all three views |
| S3 | List + Drill-down | Unique bucket → object navigation |
| Lambda | List | Shows serverless/compute category |
| RDS | List | Shows database category with status colors |

## Fixture Requirements

Each resource shown needs:
- 5-8 realistic entries per resource type
- Mix of statuses (running/stopped/pending for EC2, available/creating for RDS)
- `RawStruct` populated for EC2 (detail + YAML views need it)
- Realistic but fake names (e.g., `web-prod-01`, `api-staging-02`, `data-pipeline-logs`)
- Fake but plausible IDs (e.g., `i-0a1b2c3d4e5f`, `sg-0abc123def`)

## Fixture Coverage

All 62 resource types have fixture functions (Fields-only, no RawStruct).
For the demo, only EC2 needs RawStruct added (it's the only type shown in detail + YAML).
The other 4 demo resources (S3, Lambda, RDS, SG) only appear in list view — Fields-only is sufficient.

| Fixture | Status | Needed for demo |
|---------|--------|-----------------|
| `fixtureEC2Instances()` | Fields only | Needs RawStruct added |
| `fixtureS3Buckets()` | Fields only | OK (list view only) |
| `fixtureS3Objects()` | Fields only | OK (list view only) |
| `fixtureLambdaForYAML()` | Fields only | OK (list view only) |
| `fixtureRDSInstances()` | Fields only | OK (list view only) |
| `fixtureSGs()` | Fields only | OK (list view only) |

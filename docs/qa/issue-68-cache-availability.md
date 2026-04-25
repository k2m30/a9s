# QA User Stories: Cache Resource Availability (Issue #68)

Scope: caching which AWS resource types have resources in the current
account/profile so that the main menu can grey out empty types and launch
faster. The cache is per-profile, per-region, and TTL-based.

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration files. AWS CLI equivalents are cited so testers can
verify data parity.

---

## A. Initial Launch with No Cache

### A.1 First Launch (Cold Start)

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I launch a9s for the first time (no cache file exists at `~/.a9s/cache/`). The main menu displays. | The main menu appears immediately with all resource types listed in their normal style (white text #c0caf5). No resource types are greyed out yet. All rows are interactive and selectable. |
| A.1.2 | The main menu is displayed. Within a few seconds, background availability checks complete. | Resource types that have zero resources in the current account/region transition to a dimmed/greyed-out visual style (e.g., dim text #565f89). Resource types that have resources remain in their normal style. |
| A.1.3 | I observe the main menu during the background check period. | The menu is fully interactive while checks run. I can navigate with `j`/`k`, use filter (`/`), command (`:`) mode, press Enter to open any resource type, or press `?` for help. There is no blocking spinner on the main menu. |
| A.1.4 | The background checks complete for all resource types. | The cache file is written to `~/.a9s/cache/<profile>.yaml` (where `<profile>` is the current AWS profile name). The file contains the profile name, region, timestamp, and per-resource-type boolean availability. |

**AWS comparison:**

```
# Equivalent of lightweight availability checks:
aws ec2 describe-instances --max-items 1 --query 'Reservations[0].Instances[0].InstanceId'
aws s3api list-buckets --query 'Buckets[0].Name'
aws rds describe-db-instances --max-records 1 --query 'DBInstances[0].DBInstanceIdentifier'
aws lambda list-functions --max-items 1 --query 'Functions[0].FunctionName'
# etc. for each resource type -- lightweight calls with MaxResults=1
```

---

## B. Subsequent Launch with Cached Data

### B.1 Warm Start

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I launch a9s with a valid cache file at `~/.a9s/cache/prod.yaml` for profile "prod" and region "us-east-1". The cache indicates EC2=true, S3=true, RDS=false, Lambda=true, VPC=true, others=false. | The main menu appears instantly. Resource types marked `false` in the cache (e.g., RDS) are immediately displayed in greyed-out/dimmed style (#565f89). Resource types marked `true` (EC2, S3, Lambda, VPC) are displayed in normal white (#c0caf5). |
| B.1.2 | After the menu appears, background refresh checks start. | The background checks run silently in the background. If any availability has changed (e.g., an RDS instance was created since the cache was written), the menu updates: RDS transitions from greyed-out to normal style. The cache file is updated. |
| B.1.3 | I open the app and the cache is only 5 minutes old (within the TTL). | Cached state is used immediately. Background checks may still run to refresh, but the initial display is instant and accurate based on recent cache. |
| B.1.4 | The cache is 3 hours old (beyond the default 1-hour TTL). | Cached state is still used for the initial display (better than nothing), but background checks start immediately. As results come in, greyed-out states are updated. |

---

## C. Visual Appearance of Greyed-Out Resource Types

### C.1 Greyed-Out Style

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | RDS has no instances in the current account/region. I look at the "RDS Instances" row. | The row text is rendered in dim grey (#565f89), visually distinct from active resource types in white (#c0caf5). The shortname alias (`:rds`) is also dimmed. |
| C.1.2 | S3 has buckets in the current account. I look at the "S3 Buckets" row. | The row text is rendered in normal white (#c0caf5). The shortname alias (`:s3`) is in its standard dimmed style. |
| C.1.3 | I navigate to the greyed-out "RDS Instances" row with `j`/`k`. The cursor lands on it. | The greyed-out row becomes selected: it receives the standard blue background (#7aa2f7) with dark foreground (#1a1b26), bold. The greyed-out styling does not prevent selection. |
| C.1.4 | I press Enter on the greyed-out "RDS Instances" row. | The application navigates to the RDS Instances list view. The list loads (possibly showing zero instances). Greyed-out does NOT mean disabled -- it is a visual hint, not a restriction. |
| C.1.5 | Multiple resource types are greyed out (e.g., RDS, DocumentDB, Redshift, OpenSearch). | Each greyed-out row uses the same dim style. The visual effect makes populated resource types stand out, directing attention to what exists in this account. |

### C.2 Frame Title with Availability Info

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | The main menu is displayed with 55 total resource types and 18 have resources. | The frame title shows the total count as usual, e.g., "resource-types(55)". The count reflects ALL types, not just available ones. |
| C.2.2 | A filter is active (e.g., typing "ec") and narrows to 3 visible rows, 2 of which are greyed out. | The frame title shows "resource-types(3/55)". Greyed-out types ARE included in the visible count (they match the filter and are shown, just dimmed). |

---

## D. Background Availability Check Behavior

### D.1 Asynchronous Checks

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I launch a9s. I observe the main menu while background checks are running. | The menu does not freeze or stutter. Availability results arrive incrementally -- individual resource types update their greyed-out status one by one (or in batches) as each check completes. |
| D.1.2 | A background check for Lambda completes, changing it from unknown to "has resources". | The Lambda row transitions smoothly from its initial state to normal white (#c0caf5). No full-screen re-render flicker occurs. |
| D.1.3 | A background check for Redshift completes, confirming zero resources. | The Redshift row transitions to greyed-out dim style (#565f89). |
| D.1.4 | All background checks complete. | The cache file at `~/.a9s/cache/<profile>.yaml` is written (or updated) with the current timestamp and all resource availability booleans. |

### D.2 Check Timing

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | I launch a9s and wait. Background checks complete. | All resource types have been checked within a reasonable time (seconds, not minutes). Checks are parallelized. |
| D.2.2 | Some resource types take longer to check (e.g., a slow API call). | Faster checks update the menu first. Slower ones update when they complete. The overall experience is progressive -- I see results appear over a few seconds. |

---

## E. Cache File Structure and Persistence

### E.1 Cache File

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | Background checks complete. I inspect `~/.a9s/cache/prod.yaml`. | The file contains a valid YAML structure with: `profile: prod`, `region: us-east-1`, `checked_at: <timestamp>`, and a `resources:` map with boolean values for each resource type. |
| E.1.2 | The resources map in the cache file. | Each entry uses the resource shortname as key and a boolean as value, e.g., `s3: true`, `ec2: true`, `rds: false`, `lambda: true`. |
| E.1.3 | I switch to profile "staging" and the checks complete. | A separate cache file exists: `~/.a9s/cache/staging.yaml`. The "prod" cache file is unmodified. |
| E.1.4 | I switch regions from us-east-1 to eu-west-1 under the same profile. | The cache file is updated (or a separate region-specific cache is created). The cache reflects the resources in eu-west-1, not us-east-1. |

### E.2 Cache Corruption and Missing Files

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | The cache file `~/.a9s/cache/prod.yaml` is corrupted (invalid YAML, truncated, zero bytes). | The application launches normally. The corrupted cache is ignored. All resource types start in their default (not-greyed-out) style. Background checks run and rebuild the cache. |
| E.2.2 | The cache directory `~/.a9s/cache/` does not exist. | The application creates the directory automatically and writes the cache file after checks complete. No error is shown to the user. |
| E.2.3 | The cache file exists but the `checked_at` field is missing or unparseable. | The cache is treated as expired. Background checks run immediately. |
| E.2.4 | The cache file has resource types that no longer exist in the application (e.g., from an older version). | Unknown resource types in the cache are ignored. Known resource types are used. |
| E.2.5 | The cache file is missing resource types that exist in the current version (e.g., new types added in an update). | Missing resource types are treated as unknown (not greyed out). Background checks will determine their availability and update the cache. |

---

## F. Profile Switch Triggers Re-Check

### F.1 Profile Switch

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | I am on the main menu using profile "prod". I press `:`, type "ctx", press Enter to open the profile selector. I select profile "staging". | The main menu reappears. If a cache exists for "staging", it is loaded instantly -- some resource types may be greyed out based on the staging cache. Background checks start for "staging". |
| F.1.2 | Profile "staging" has no cache file. | The main menu shows all resource types in normal style (no greying). Background checks run and gradually grey out empty types. |
| F.1.3 | I switch from "prod" (which has EC2, S3, RDS) to "dev" (which only has Lambda, S3). | After the switch and background checks, the main menu shows Lambda and S3 in normal style. EC2, RDS, and other types are greyed out. The header left side updates to show "dev:us-east-1". |
| F.1.4 | I switch profiles rapidly (prod -> staging -> dev within seconds). | Each switch cancels the previous background checks and starts new ones for the current profile. No stale results from a previous profile's check appear. |

**AWS comparison:**

```
# Each profile switch is equivalent to:
export AWS_PROFILE=staging
aws ec2 describe-instances --max-items 1
aws s3api list-buckets --query 'Buckets[0]'
# etc.
```

---

## G. Region Switch Triggers Re-Check

### G.1 Region Switch

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | I am on the main menu using region "us-east-1". I press `:`, type "region", press Enter. I select "eu-west-1". | The main menu reappears. If a cache exists for the current profile + eu-west-1, it is loaded. Background checks start for eu-west-1. |
| G.1.2 | The account has resources only in us-east-1. I switch to ap-southeast-1. | After background checks complete, most or all resource types are greyed out in ap-southeast-1 (since no resources exist there). |
| G.1.3 | I switch back to us-east-1. | The cached state for us-east-1 is restored instantly. Resource types that were available before are shown in normal style. |

**AWS comparison:**

```
aws ec2 describe-instances --region eu-west-1 --max-items 1
aws s3api list-buckets  # S3 is global but bucket region is checked
```

---

## H. Permission Errors During Availability Checks

### H.1 AccessDenied Handling

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | The IAM identity does not have permission to list EC2 instances (`ec2:DescribeInstances`). The background check for EC2 returns AccessDenied. | The EC2 row is NOT greyed out. An AccessDenied error means we cannot determine availability -- the conservative approach is to assume the resource type might have instances. |
| H.1.2 | Multiple resource types return AccessDenied. | Each resource type that returns a permission error remains in its normal (not greyed-out) style. Only types that successfully confirm zero resources are greyed out. |
| H.1.3 | The cache file records the AccessDenied status. | The cache may store an "unknown" or "denied" state (not `false`) for resources where the check failed. On next launch, these are treated as "might have resources" (not greyed out). |
| H.1.4 | The IAM identity has read access to all services. The background checks succeed for all types. | All resource types are correctly categorized as available or empty. No false negatives (incorrectly greying out a type that has resources). |

---

## I. Filter Interaction with Greyed-Out Types

### I.1 Filter Mode

| ID | Story | Expected |
|----|-------|----------|
| I.1.1 | Three resource types are greyed out: RDS, DocumentDB, Redshift. I press `/` and type "rds". | The filter shows "RDS Instances" (the only match). The row is displayed in its greyed-out dim style. The filter works on greyed-out types -- they are filtered/shown like any other row. |
| I.1.2 | I type "ec" which matches EC2 (available), ElastiCache (available), and Secrets Manager (available). | All three matching rows are shown in normal style. The frame title shows "resource-types(3/N)". |
| I.1.3 | I type a filter that matches only greyed-out types (e.g., "redshift" when only Redshift exists and it is greyed out). | The single greyed-out Redshift row is shown. I can still select it and press Enter to navigate to it. |

---

## J. Command Mode Interaction

### J.1 Command Navigation to Greyed-Out Types

| ID | Story | Expected |
|----|-------|----------|
| J.1.1 | RDS is greyed out (no instances). I press `:`, type "rds", press Enter. | The application navigates directly to the RDS Instances list view. The list loads (showing zero instances). Greyed-out status does not prevent command-mode navigation. |
| J.1.2 | I use `:ec2` to navigate to EC2 (which is not greyed out). | Normal navigation occurs. The EC2 list loads with instances. |

---

## K. Detail/YAML Views and Cache

### K.1 Cache Does Not Affect Sub-Views

| ID | Story | Expected |
|----|-------|----------|
| K.1.1 | I navigate into a resource type that is NOT greyed out (EC2). I view details (`d`), YAML (`y`). | All sub-views work normally. The cache has no effect on detail, YAML, or child views. |
| K.1.2 | I navigate into a greyed-out resource type (RDS -- empty). The list shows zero instances. | The empty list message appears as usual. Pressing `d`, `y`, or `c` on an empty list has no effect (no resource selected). |

---

## L. Ctrl+R Refresh and Cache

### L.1 Manual Refresh

| ID | Story | Expected |
|----|-------|----------|
| L.1.1 | I am on the main menu. I press `ctrl+r`. | Background availability checks restart. The cache is refreshed. If any availability has changed, the greyed-out state updates. |
| L.1.2 | I am on a resource list (e.g., EC2). I press `ctrl+r`. | The resource list refreshes from AWS (normal behavior). This does NOT trigger a main menu cache refresh -- the cache is only refreshed on the main menu or during profile/region switches. |

---

## M. Edge Cases

### M.1 All Resource Types Empty

| ID | Story | Expected |
|----|-------|----------|
| M.1.1 | The current account/region has zero resources of any type. Background checks complete. | Every resource type in the main menu is greyed out. The menu remains fully interactive -- I can still navigate and open any resource type. |
| M.1.2 | I press Enter on any greyed-out type. | The resource list loads normally, showing zero resources with an empty state message. |

### M.2 All Resource Types Have Resources

| ID | Story | Expected |
|----|-------|----------|
| M.2.1 | The current account/region has resources for every type. Background checks complete. | No resource types are greyed out. The visual appearance is identical to the pre-cache behavior (all normal style). |

### M.3 Resource Created After Cache

| ID | Story | Expected |
|----|-------|----------|
| M.3.1 | The cache says RDS is empty (false). I create an RDS instance outside of a9s. I launch a9s. | Initially, RDS appears greyed out (from cache). After background checks complete, RDS transitions to normal style (the new instance is detected). |

### M.4 Resource Deleted After Cache

| ID | Story | Expected |
|----|-------|----------|
| M.4.1 | The cache says EC2 is available (true). I terminate all EC2 instances outside of a9s. I launch a9s. | Initially, EC2 appears in normal style (from cache). After background checks complete, EC2 transitions to greyed-out (no instances found). |

---

## N. Demo Mode

### N.1 Cache with Demo/Offline Mode

| ID | Story | Expected |
|----|-------|----------|
| N.1.1 | I launch a9s in demo mode (using synthetic fixture data, no real AWS calls). | The main menu shows all resource types in normal style (not greyed out). Demo mode does not use cache -- all fixture types are assumed to have data. |
| N.1.2 | No background availability checks are made in demo mode. | No cache file is created or updated. No AWS API calls are attempted. |

---

## O. Terminal Resize

### O.1 Resize During Background Checks

| ID | Story | Expected |
|----|-------|----------|
| O.1.1 | Background checks are in progress. I resize the terminal. | The main menu re-renders at the new size. Greyed-out states are preserved for types already checked. Checks continue uninterrupted. |
| O.1.2 | I resize below minimum dimensions while checks are running. | The "Terminal too narrow/short" error appears. Resizing back above minimum restores the menu with current greyed-out states. Background checks are not cancelled by resize. |

---

## P. Help Screen

### P.1 Help Screen and Cache

| ID | Story | Expected |
|----|-------|----------|
| P.1.1 | I am on the main menu with some types greyed out. I press `?`. | The help screen displays normally. It does not mention greyed-out types or cache functionality. |
| P.1.2 | I close the help screen. | The main menu reappears with the same greyed-out states preserved. |

---

## Q. Startup Performance

### Q.1 Launch Time

| ID | Story | Expected |
|----|-------|----------|
| Q.1.1 | I launch a9s with a warm cache. | The main menu appears within sub-second time, with greyed-out states immediately applied from cache. The experience is noticeably faster for identifying which resource types are relevant. |
| Q.1.2 | I launch a9s with no cache (cold start). | The main menu still appears within sub-second time. All types are in normal style initially. Background checks begin immediately after render. |
| Q.1.3 | I compare launch with cache (warm) vs. without (cold). | Both launches are equally fast to first render. The difference is only in the initial greyed-out state accuracy. |

# QA User Stories: Configurable Menu -- Show Only Selected Resource Types (Issue #81)

Scope: the ability to limit the main menu to a subset of resource types via `config.yaml`, with per-profile configuration, include/exclude semantics, category-level filtering, and per-project overrides. All stories treat a9s as a black box.

Resource short names referenced below match `views.yaml` keys (e.g., `ec2`, `s3`, `dbi`, `redis`, `lambda`, `ecs`, `elb`, `vpc`, `sg`, `cfn`, `logs`, `alarm`, `sqs`, `sns`, `glue`, `athena`, `redshift`, `kinesis`, `secrets`, `ssm`, `kms`, `r53`, `cf`, `acm`, `apigw`, `role`, `policy`, `eks`, `ng`, `nat`, `igw`, `eip`, `eni`, `subnet`, `rtb`, `tg`, `ddb`, `opensearch`, `efs`, `ecr`, `cb`, `pipeline`, `codeartifact`, `eb`, `sfn`, `msk`, `trail`, `waf`, `ses`, `backup`, `tgw`, `vpce`, `asg`) and colon-command aliases (`:lambda`, `:ec2`, etc.).

---

## A. Default Behavior (No Config)

### A.1 Backward Compatibility

| # | Story | Expected |
|---|-------|----------|
| A.1.1 | No `~/.a9s/config.yaml` exists. I launch a9s. | The main menu shows all resource types under all category headings. Behavior is identical to previous versions. |
| A.1.2 | `~/.a9s/config.yaml` exists but contains no `profiles` or `default` key. I launch a9s. | The main menu shows all resource types. The config file is parsed without error; unrelated keys are ignored. |
| A.1.3 | `~/.a9s/config.yaml` exists with `default: menu: all`. I launch a9s. | The main menu shows all resource types. The `all` keyword explicitly requests no filtering. |
| A.1.4 | `~/.a9s/config.yaml` has profiles defined but none match my current AWS profile, and no `default` section exists. I launch a9s. | The main menu shows all resource types (fall-through to show-all). |

**AWS comparison:**

```
aws configure list-profiles
```

Expected: Full menu with all categories and resource types visible, same as launching without any config.yaml.

---

## B. Include Filter (Per Profile)

### B.1 Basic Include

| # | Story | Expected |
|---|-------|----------|
| B.1.1 | `~/.a9s/config.yaml` has `profiles: prod-account: menu: include: [lambda, iam_roles, iam_policies]`. I launch a9s with `AWS_PROFILE=prod-account`. | The main menu shows only Lambda Functions, IAM Roles, and IAM Policies. All other resource types are hidden. |
| B.1.2 | Same config as B.1.1. I count the visible menu rows. | Exactly 3 resource types are listed. The frame title shows the filtered count (e.g., `resource-types(3)`). |
| B.1.3 | Same config as B.1.1. I look at category headings. | Only categories containing at least one included resource appear. COMPUTE shows Lambda; SECURITY & IAM shows IAM Roles and IAM Policies. Categories like DATABASES & STORAGE, NETWORKING, etc. are hidden. |
| B.1.4 | `~/.a9s/config.yaml` has `profiles: staging: menu: include: [ec2, ecs, rds, lambda, s3, sqs, alarm]`. I launch with `AWS_PROFILE=staging`. | The main menu shows exactly: EC2, ECS Services, RDS (DB Instances), Lambda, S3, SQS, and CloudWatch Alarms. Category headings adjust accordingly. |
| B.1.5 | Same config as B.1.4. I navigate with `j`/`k`. | I can navigate only among the 7 visible resource types. Cursor wraps from the last visible type to the first. Hidden types are unreachable via navigation. |
| B.1.6 | Same config as B.1.4. I press `G` (jump to bottom). | Selection jumps to the last visible resource type (CloudWatch Alarms or whichever is last in the configured order). |

**AWS comparison:**

```
aws configure list-profiles
# Verify the profile name matches
echo $AWS_PROFILE
```

Expected fields visible: Only the resource types listed in `include`. Category headings auto-adjust.

### B.2 Include with Command Mode

| # | Story | Expected |
|---|-------|----------|
| B.2.1 | Config includes only `[lambda, s3]`. I press `:` and type `lambda` then Enter. | The application navigates to the Lambda Functions list. Included resource commands work normally. |
| B.2.2 | Config includes only `[lambda, s3]`. I press `:` and type `ec2` then Enter. | The application navigates to the EC2 Instances list view. Command mode provides direct access to any resource type regardless of menu filtering -- the menu filter controls visibility, not availability. |
| B.2.3 | Config includes only `[lambda, s3]`. I press `:` and type `ec2`, load EC2 list, press `esc` to return. | I return to the main menu showing only Lambda and S3. The EC2 detour does not alter the menu configuration. |

### B.3 Include with Filter Mode

| # | Story | Expected |
|---|-------|----------|
| B.3.1 | Config includes `[ec2, rds, lambda, s3, sqs]`. I press `/` and type `s`. | Only resource types from the included set whose name contains "s" are shown (e.g., S3, SQS). Non-included types are not surfaced by the filter. |
| B.3.2 | Config includes `[ec2, rds, lambda]`. I press `/` and type `vpc`. | No results. VPC is not in the include list so it cannot appear in filter results. Frame title shows `resource-types(0/3)`. |
| B.3.3 | Config includes `[ec2, rds, lambda]`. I press `/` and type `r`. | "RDS (DB Instances)" appears (matching "r"). The filter operates over the 3 included types only. |

---

## C. Exclude Filter (Per Profile)

### C.1 Basic Exclude

| # | Story | Expected |
|---|-------|----------|
| C.1.1 | `~/.a9s/config.yaml` has `profiles: sandbox: menu: exclude: [ses, backup, codeartifact]`. I launch with `AWS_PROFILE=sandbox`. | The main menu shows all resource types except SES Identities, Backup Plans, and CodeArtifact. |
| C.1.2 | Same config as C.1.1. I count the visible menu rows. | The total is the full resource count minus 3 (the excluded types). Frame title reflects the reduced count. |
| C.1.3 | Same config as C.1.1. I look at categories. | If a category had only excluded resources and no remaining ones, that category heading disappears. Otherwise, the category heading remains with its non-excluded resources. |
| C.1.4 | `~/.a9s/config.yaml` has `profiles: minimal: menu: exclude: [ec2]`. I launch with `AWS_PROFILE=minimal`. | All resource types except EC2 Instances are shown. The COMPUTE category still appears (it has ECS, Lambda, ASG, Elastic Beanstalk). |
| C.1.5 | I navigate with `j`/`k` in the excluded menu. | Excluded resource types are not reachable. Navigation skips over them seamlessly. |

**AWS comparison:**

```
echo $AWS_PROFILE
# sandbox -> show everything except ses, backup, codeartifact
```

Expected: All resource types minus the excluded ones, with category headings auto-hiding when all resources in that category are excluded.

### C.2 Exclude All Resources in a Category

| # | Story | Expected |
|---|-------|----------|
| C.2.1 | Config excludes all resource types in the BACKUP category (`[backup, ses]`). I launch a9s. | The BACKUP category heading is completely hidden from the main menu. All other categories remain visible. |
| C.2.2 | Config excludes all resource types in CONTAINERS (`[eks, ng]`). I launch a9s. | The CONTAINERS category heading disappears. All other categories remain. |

---

## D. Include and Exclude Are Mutually Exclusive

### D.1 Conflicting Configuration

| # | Story | Expected |
|---|-------|----------|
| D.1.1 | Config has `profiles: broken: menu: include: [ec2] exclude: [s3]`. I launch with `AWS_PROFILE=broken`. | The application handles this gracefully: it either uses `include` (taking precedence), uses `exclude`, or shows an error message indicating that include and exclude cannot both be specified. The application does not crash. |
| D.1.2 | Config has `profiles: prod: menu: include: [ec2, rds]` and `profiles: staging: menu: exclude: [ses]`. I launch with `AWS_PROFILE=prod`. | Only the `prod` profile config applies. The staging config has no effect. EC2 and RDS are the only visible types. |
| D.1.3 | Same as D.1.2, but I launch with `AWS_PROFILE=staging`. | All types except SES are shown. The prod include config has no effect. |

---

## E. Default Profile Fallback

### E.1 Default Section

| # | Story | Expected |
|---|-------|----------|
| E.1.1 | Config has `default: menu: include: [ec2, s3, lambda]` and no matching profile entry. I launch with `AWS_PROFILE=unknown-profile`. | The menu shows EC2, S3, and Lambda (from the default section), since no profile-specific config matches. |
| E.1.2 | Config has `profiles: prod: menu: include: [rds]` and `default: menu: include: [ec2, s3, lambda]`. I launch with `AWS_PROFILE=prod`. | The menu shows only RDS (profile match takes precedence over default). |
| E.1.3 | Config has `profiles: prod: menu: include: [rds]` and `default: menu: include: [ec2, s3]`. I launch with `AWS_PROFILE=dev` (no matching profile). | The menu shows EC2 and S3 (from default fallback). |
| E.1.4 | Config has `default: menu: all`. I launch with any unmatched profile. | All resource types are shown (explicit `all`). |

**AWS comparison:**

```
echo $AWS_PROFILE
# Verify which profile is active to determine which config block applies
```

### E.2 No AWS_PROFILE Set

| # | Story | Expected |
|---|-------|----------|
| E.2.1 | Config has profiles defined but `$AWS_PROFILE` is not set (using the `default` AWS profile). I launch a9s. | The application resolves the effective profile name (e.g., "default") and matches against `profiles.default` if it exists, otherwise falls through to the `default` section or all resources. |
| E.2.2 | Config has `profiles: default: menu: include: [s3]`. `$AWS_PROFILE` is unset. | The menu shows only S3, since the effective AWS profile name is "default" and it matches `profiles.default`. |

---

## F. Profile Resolution Order

### F.1 Profile Flag Override

| # | Story | Expected |
|---|-------|----------|
| F.1.1 | Config has `profiles: staging: menu: include: [ec2]`. I launch with `--profile staging`. | The menu shows only EC2. The `--profile` flag correctly resolves to the `staging` config entry. |
| F.1.2 | `$AWS_PROFILE=prod` but I launch with `--profile staging`. Config has entries for both. | The `--profile` flag takes precedence over `$AWS_PROFILE`. The staging config applies. |
| F.1.3 | I switch profiles at runtime using `:ctx`, selecting a profile that has a different menu config. | After the profile switch, the main menu updates to reflect the new profile's menu configuration. Resource types appear or disappear according to the newly selected profile's include/exclude rules. |

**AWS comparison:**

```
aws configure list-profiles
aws sts get-caller-identity --profile staging
```

### F.2 Profile Switch Updates Menu

| # | Story | Expected |
|---|-------|----------|
| F.2.1 | I start with `AWS_PROFILE=prod` (include: [rds]). I use `:ctx` to switch to `staging` (include: [ec2, s3, rds, lambda]). | After the switch, the main menu shows 4 resource types instead of 1. The frame title count updates. |
| F.2.2 | I start with `AWS_PROFILE=staging` (include: [ec2, s3]). I use `:ctx` to switch to `sandbox` (exclude: [ses]). | After the switch, the menu shows all resource types except SES. The category headings adjust accordingly. |
| F.2.3 | I start with `AWS_PROFILE=restricted` (include: [lambda]). I use `:ctx` to switch to a profile with no config entry and no default. | After the switch, all resource types appear (fall-through to full menu). |

---

## G. Per-Project Override

### G.1 Project-Level Config

| # | Story | Expected |
|---|-------|----------|
| G.1.1 | A `.a9s/config.yaml` file exists in the current working directory with `default: menu: include: [lambda, iam_roles, iam_policies]`. No `~/.a9s/config.yaml` exists. I launch a9s from that directory. | The menu shows only Lambda, IAM Roles, and IAM Policies. The project-level config is used. |
| G.1.2 | Both `.a9s/config.yaml` (project: include [lambda]) and `~/.a9s/config.yaml` (default: include [ec2, s3]) exist. I launch a9s from the project directory. | The menu shows only Lambda. Project-level config takes precedence over user-level config. |
| G.1.3 | `.a9s/config.yaml` exists with `default: menu: include: [lambda]`. I launch a9s from a different directory (where no `.a9s/config.yaml` exists) with `~/.a9s/config.yaml` having `default: menu: include: [ec2]`. | The menu shows only EC2 (user-level config applies since no project-level config is found). |
| G.1.4 | `.a9s/config.yaml` exists with `profiles: prod: menu: include: [rds]` and `~/.a9s/config.yaml` has `profiles: prod: menu: include: [ec2, s3]`. I launch with `AWS_PROFILE=prod`. | The menu shows only RDS (project-level profile config takes precedence). |

**AWS comparison:**

```
ls -la .a9s/config.yaml
ls -la ~/.a9s/config.yaml
echo $AWS_PROFILE
```

### G.2 Project Config Checked Into Repo

| # | Story | Expected |
|---|-------|----------|
| G.2.1 | A team checks `.a9s/config.yaml` into their git repo with `default: menu: include: [lambda, iam_roles, iam_policies]`. A team member clones the repo and launches a9s from within it. | The menu shows Lambda, IAM Roles, and IAM Policies regardless of which AWS profile the team member uses. |
| G.2.2 | Same as G.2.1, but the team member has a `~/.a9s/config.yaml` with `default: menu: all`. | The project config still takes precedence. Only Lambda, IAM Roles, and IAM Policies are shown. |

---

## H. Category-Level Filtering

### H.1 Category Shorthand

| # | Story | Expected |
|---|-------|----------|
| H.1.1 | Config has `profiles: infra-team: menu: categories: [NETWORKING, SECURITY]`. I launch with `AWS_PROFILE=infra-team`. | The menu shows all resource types under NETWORKING (Load Balancers, Target Groups, Security Groups, VPCs, Subnets, Route Tables, NAT Gateways, Internet Gateways, Elastic IPs, VPC Endpoints, Transit Gateways, Network Interfaces) and SECURITY & IAM (IAM Roles, IAM Policies, IAM Users, IAM Groups, WAF Web ACLs). All other categories are hidden. |
| H.1.2 | Same config as H.1.1. I count visible resource types. | The total matches all resources in NETWORKING + SECURITY & IAM categories. |
| H.1.3 | Config has `profiles: data-team: menu: categories: [DATA & ANALYTICS]`. I launch with `AWS_PROFILE=data-team`. | Only Glue Jobs and Athena Workgroups are shown (the two resources under DATA & ANALYTICS). |
| H.1.4 | Config has `profiles: team: menu: categories: [CONTAINERS]`. I launch. | Only EKS Clusters and EKS Node Groups are shown. |

**AWS comparison:**

```
echo $AWS_PROFILE
# Verify category membership against docs/design/resources-groupping.md
```

### H.2 Category Combined with Include/Exclude

| # | Story | Expected |
|---|-------|----------|
| H.2.1 | Config has `profiles: custom: menu: categories: [NETWORKING] include: [lambda]`. I launch with `AWS_PROFILE=custom`. | The menu shows all NETWORKING resources plus Lambda. The category and include lists are merged (union). |
| H.2.2 | Config has `profiles: custom: menu: categories: [NETWORKING] exclude: [nat, eip]`. I launch. | The menu shows all NETWORKING resources except NAT Gateways and Elastic IPs. The exclude list is applied after category expansion. |
| H.2.3 | Config has `profiles: custom: menu: categories: [COMPUTE] include: [s3, rds]`. I launch. | The menu shows all COMPUTE resources (EC2, ECS, Lambda, ASG, Elastic Beanstalk) plus S3 and RDS. |

---

## I. Invalid Configuration Handling

### I.1 Invalid Resource Short Names

| # | Story | Expected |
|---|-------|----------|
| I.1.1 | Config has `profiles: broken: menu: include: [ec2, nonexistent, lambda]`. I launch with `AWS_PROFILE=broken`. | The menu shows EC2 and Lambda. The `nonexistent` entry is silently ignored (or a warning is shown). The application does not crash. |
| I.1.2 | Config has `profiles: broken: menu: include: [nonexistent_only]`. I launch. | The menu shows zero resource types (or falls back to all if the result would be empty). The application does not crash. |
| I.1.3 | Config has `profiles: broken: menu: exclude: [fake_service]`. I launch. | All resource types are shown (the unrecognized name is ignored). No crash. |

### I.2 Invalid Category Names

| # | Story | Expected |
|---|-------|----------|
| I.2.1 | Config has `categories: [NONEXISTENT_CATEGORY]`. I launch. | The application handles this gracefully: either no resources are shown for that category (it is ignored), or a warning is displayed. No crash. |
| I.2.2 | Config has `categories: [NETWORKING, FAKE, CONTAINERS]`. I launch. | NETWORKING and CONTAINERS resources are shown. FAKE is silently ignored. |

### I.3 Malformed Config

| # | Story | Expected |
|---|-------|----------|
| I.3.1 | `~/.a9s/config.yaml` contains malformed YAML (syntax error). I launch a9s. | The application starts with the full menu (all resource types). A warning or error flash may appear. The application does not crash. |
| I.3.2 | `~/.a9s/config.yaml` has `profiles: prod: menu: include: "ec2"` (string instead of list). I launch. | The application handles the type mismatch gracefully. It either treats the string as a single-item list `[ec2]` or falls back to the full menu with a warning. No crash. |
| I.3.3 | `~/.a9s/config.yaml` has `profiles: prod: menu: include: []` (empty list). I launch. | The menu shows zero resource types or falls back to all (showing an empty include list as "show nothing" or "show everything" -- either interpretation is valid as long as it is consistent and documented). |
| I.3.4 | `~/.a9s/config.yaml` has `profiles: prod: menu: 42` (invalid type). I launch. | The application falls back to full menu. No crash. |

---

## J. Menu Visual Behavior with Filtered Config

### J.1 Category Heading Visibility

| # | Story | Expected |
|---|-------|----------|
| J.1.1 | Config includes only `[ec2, lambda]` (both in COMPUTE). I view the main menu. | The COMPUTE category heading is visible. All other category headings are hidden. |
| J.1.2 | Config includes `[ec2, s3]` (COMPUTE and DATABASES & STORAGE). I view the main menu. | Both COMPUTE and DATABASES & STORAGE headings are visible. Other categories are hidden. |
| J.1.3 | Config includes `[ec2]` only. I view the main menu. | Only the COMPUTE category heading and EC2 Instances are shown. The frame title shows `resource-types(1)`. |
| J.1.4 | Config excludes all resources in BACKUP (`[backup, ses]`). I view the main menu. | The BACKUP category heading is gone. All other categories and their resources are visible. |

### J.2 Shortname Aliases Still Visible

| # | Story | Expected |
|---|-------|----------|
| J.2.1 | Config includes `[lambda, s3]`. I view the main menu. | Each visible resource type row shows its shortname alias (`:lambda`, `:s3`) in dimmed style, just as in the full menu. |
| J.2.2 | Config includes `[ec2]`. I view the main menu showing only EC2. | The EC2 row shows `:ec2` dimmed on the right, maintaining the same visual format. |

### J.3 Menu Count in Frame Title

| # | Story | Expected |
|---|-------|----------|
| J.3.1 | Config includes 5 resource types. No filter active. | Frame title shows `resource-types(5)`. |
| J.3.2 | Config includes 5 resource types. I press `/` and type a filter that matches 2 of them. | Frame title shows `resource-types(2/5)`. The denominator reflects the configured set, not the full catalog. |
| J.3.3 | Config includes 5 resource types. I press `/` and type a filter that matches none. | Frame title shows `resource-types(0/5)`. |

---

## K. Help Screen and Command Behavior

### K.1 Help Screen

| # | Story | Expected |
|---|-------|----------|
| K.1.1 | Config includes only 3 resource types. I press `?` to open help. | The help screen displays normally with all key binding categories (RESOURCE, GENERAL, NAVIGATION, HOTKEYS). The menu filtering does not affect help content. |

### K.2 Command Mode with Filtered Menu

| # | Story | Expected |
|---|-------|----------|
| K.2.1 | Config includes only `[s3]`. I press `:` and type `rds` then Enter. | The application navigates to the RDS list. Command mode provides direct navigation to any resource type, even those not shown in the menu. |
| K.2.2 | Config includes only `[s3]`. After navigating to RDS via command, I press `esc`. | I return to the main menu showing only S3. The command navigation does not change the menu config. |
| K.2.3 | Config includes only `[s3]`. I press `:` and type `q` then Enter. | The application quits. The `:q` command is unaffected by menu filtering. |
| K.2.4 | Config includes only `[s3]`. I press `:` and type `ctx` then Enter. | The profile selector opens. System commands are unaffected by menu filtering. |

---

## L. Config File Separate from views.yaml

### L.1 Config Independence

| # | Story | Expected |
|---|-------|----------|
| L.1.1 | I have both `~/.a9s/views.yaml` (customizing columns) and `~/.a9s/config.yaml` (filtering menu). I launch a9s. | Both configurations apply independently: the menu shows only configured resource types, and list/detail views use the custom column definitions. |
| L.1.2 | I delete `~/.a9s/config.yaml` but keep `~/.a9s/views.yaml`. | The full menu appears. Custom column definitions from views.yaml still apply. |
| L.1.3 | I delete `~/.a9s/views.yaml` but keep `~/.a9s/config.yaml` with menu filtering. | The filtered menu appears. Default column definitions apply (built-in views.yaml). |

---

## M. Edge Cases with Profile Switching

### M.1 Dynamic Menu Updates

| # | Story | Expected |
|---|-------|----------|
| M.1.1 | I start with profile A (include: [ec2, s3]) and am viewing the EC2 resource list. I press `:ctx` and switch to profile B (include: [lambda]). | After the profile switch, I return to the main menu. The menu now shows only Lambda. EC2 and S3 are no longer visible. |
| M.1.2 | I start with profile A (include: [ec2]) and have EC2 selected in the menu. I switch to profile B (include: [s3]) via `:ctx`. | The main menu updates to show only S3. The selection resets to the first visible resource type (S3). |
| M.1.3 | I am in a detail view for an EC2 instance. I switch profile via `:ctx` to one that excludes EC2. | After the profile switch, I return to the main menu with the new filter applied. The previous EC2 detail view is no longer on the navigation stack. |

### M.2 Region Switch Does Not Affect Menu Config

| # | Story | Expected |
|---|-------|----------|
| M.2.1 | Config includes `[ec2, s3]`. I switch region using `:region`. | The menu filter remains the same (EC2 and S3). Region changes do not affect which resource types are shown. Only the data within each resource list changes. |

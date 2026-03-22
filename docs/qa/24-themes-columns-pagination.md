# QA User Stories: Configurable Themes, Column Order, Pagination

Covers three GitHub issues as a combined QA document:
- **Issue #22:** Configurable color themes (11 built-in + custom YAML)
- **Issue #23:** Name column first in all default list views
- **Issue #24:** Pagination and lazy loading for large data sets

All stories are written from a black-box perspective against the design spec
(`docs/design/design.md`, `docs/design/themes.md`) and the `views.yaml` /
`views_reference.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Color Themes -- Default Behavior

### A.1 Startup Without Config

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | No `~/.a9s/config.yaml` exists. I launch a9s. | The application starts normally. All colors match the Tokyo Night Dark palette from the design spec: header accent `#7aa2f7`, dim text `#565f89`, table header blue `#7aa2f7`, selected row background `#7aa2f7` with foreground `#1a1b26`, running rows green `#9ece6a`, stopped rows red `#f7768e`, pending rows yellow `#e0af68`, terminated rows dim `#565f89`. No error or warning is displayed. |
| A.1.2 | No `~/.a9s/config.yaml` exists. I navigate through main menu, resource list (EC2), detail view, YAML view, and help screen. | Every view renders with Tokyo Night Dark colors. Header, frame borders (`#414868`), YAML syntax highlighting (keys blue, strings green, numbers orange, booleans purple, null dim), help keys green, help categories orange -- all match design spec section 1. |
| A.1.3 | `~/.a9s/config.yaml` exists but contains no `theme` key (e.g., only future settings). I launch a9s. | The application starts with Tokyo Night Dark as the default theme. No error or warning about missing theme configuration. |

**AWS comparison:**
```
aws ec2 describe-instances --query 'Reservations[].Instances[].{ID:InstanceId,State:State.Name}'
```
Expected fields visible: Name, Instance ID, State, Type, Private IP, Public IP, Launch Time (per views.yaml ec2 list).
The color of each row should match the instance state: running=green, stopped=red, pending=yellow, terminated=dim.

### A.2 Built-in Theme Selection

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | `~/.a9s/config.yaml` contains `theme: "tokyo-night"`. I launch a9s. | The application starts with the default Tokyo Night Dark palette. Behavior is identical to A.1.1 since this is the default theme by name. |
| A.2.2 | `~/.a9s/config.yaml` contains `theme: "catppuccin-mocha"`. I launch a9s and open the EC2 list. | The application starts with Catppuccin Mocha colors. All UI elements use the Catppuccin Mocha palette: warm pastels with dark background. Header, table headers, selected rows, status-colored rows, borders, and YAML syntax all reflect the Catppuccin Mocha color scheme. The data content (column values, row counts) is unchanged -- only colors differ. |
| A.2.3 | `~/.a9s/config.yaml` contains `theme: "dracula"`. I launch a9s and open the help screen (?). | The application uses Dracula theme colors: purple accents, high contrast. Help key colors, help category headers, and the 4-column layout all render in Dracula palette tones. The help content (key bindings, categories) is unchanged. |
| A.2.4 | `~/.a9s/config.yaml` contains `theme: "nord"`. I launch a9s, open S3 buckets, press d on a bucket. | The detail view renders with Nord palette: arctic blue tones for keys, muted tones for values, Nord-appropriate section header colors. Field content (BucketArn, BucketRegion, CreationDate) is unchanged. |
| A.2.5 | `~/.a9s/config.yaml` contains `theme: "gruvbox-dark"`. I launch a9s, open EC2, press y on an instance. | The YAML view renders with Gruvbox Dark palette: earthy warm tones. YAML keys, string values, numbers, booleans, null values, and tree connectors each use distinct Gruvbox colors. The YAML content itself is unchanged. |
| A.2.6 | `~/.a9s/config.yaml` contains `theme: "solarized-dark"`. I launch a9s and navigate to the Secrets Manager list. | The list renders with Solarized Dark colors: precision colors with low contrast on dark base03 background. Secret Name, Description, Last Accessed, Last Changed, and Rotation columns are visible with Solarized palette coloring. |
| A.2.7 | `~/.a9s/config.yaml` contains `theme: "tokyo-night-light"`. I launch a9s on a terminal with a light background. | The application uses Tokyo Night Light colors: warm whites with blue accents. Text is dark on light backgrounds. The header, frame borders, row selection, and all views use appropriate light-theme contrast. |
| A.2.8 | `~/.a9s/config.yaml` contains `theme: "catppuccin-latte"`. I launch a9s and open CloudFormation stacks. | The application uses Catppuccin Latte: warm pastels on a light background. Stack Name, Status, Created, Updated, Description columns render legibly. Status-colored rows (CREATE_COMPLETE, ROLLBACK_FAILED, etc.) use Latte-appropriate green/red/yellow tones. |
| A.2.9 | `~/.a9s/config.yaml` contains `theme: "nord-light"`. I launch a9s and open Lambda functions. | The application uses Nord Light (Snow Storm palette): arctic light tones. Function Name, Runtime, Memory, Timeout, State, Last Modified columns render with appropriate contrast on the light background. |
| A.2.10 | `~/.a9s/config.yaml` contains `theme: "gruvbox-light"`. I launch a9s and open the profile selector (`:ctx`). | The profile selector renders with Gruvbox Light: cream background, dark text. The current profile indicator "(current)" and profiles with "(no credentials)" dim styling both remain legible. |
| A.2.11 | `~/.a9s/config.yaml` contains `theme: "solarized-light"`. I launch a9s and open the region selector (`:region`). | The region selector renders with Solarized Light: base3 background. Region names and descriptions are legible. The selected region has visually distinct highlighting appropriate for the Solarized Light palette. |

**AWS comparison:**
```
aws s3api list-buckets --query 'Buckets[].{Name:Name,Created:CreationDate}'
aws ec2 describe-instances --output yaml
aws lambda list-functions --query 'Functions[].{Name:FunctionName,Runtime:Runtime}'
```
Expected: Data content identical regardless of theme. Only rendering colors change.

### A.3 Theme Rendering Across All View Types

For each of the 11 built-in themes, the following elements must render with visually distinct colors and sufficient contrast:

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | I set each built-in theme and view the main menu. | The resource type list renders. The selected row is visually distinct from unselected rows. The shortname aliases (`:ec2`, `:s3`, etc.) appear in a dimmed style. The frame border and title are visible. |
| A.3.2 | I set each built-in theme and view a resource list with mixed statuses (EC2 with running, stopped, pending, terminated instances). | Each status maps to a visually distinct color within that theme. Running rows are clearly distinguishable from stopped rows. The selected row always overrides status coloring with its own highlight style. |
| A.3.3 | I set each built-in theme and open the detail view (d). | Detail keys and values use visually distinct colors. Section headers are bold and colored differently from regular keys. The key-value alignment (22-char key column) is unchanged. |
| A.3.4 | I set each built-in theme and open the YAML view (y). | YAML keys, string values, number values, boolean values, null values, and tree connector lines each use a distinct color appropriate to the theme. At least 5 distinct colors are visible in a typical YAML output. |
| A.3.5 | I set each built-in theme and open the help screen (?). | The four-column layout renders. Category headers (RESOURCE, GENERAL, NAVIGATION, HOTKEYS) are bold and colored differently from key descriptions. Key names (e.g., `<esc>`, `<j>`) are colored differently from descriptions. The "Press any key to close" hint is dim. |
| A.3.6 | I set each built-in theme and trigger a flash message (press c to copy). | The "Copied!" success flash appears in the header in a visually distinct success color (green-like tone). It auto-clears after ~2 seconds. |
| A.3.7 | I set each built-in theme and trigger an error (e.g., API error from invalid credentials). | The error flash appears in the header in a visually distinct error color (red-like tone). |
| A.3.8 | I set each built-in theme and enter filter mode (/). | The filter text in the header right side renders in a visually distinct filter color (amber/yellow-like tone), bold. |
| A.3.9 | I set each built-in theme and enter command mode (:). | The command text in the header right side renders in a visually distinct command color (amber/yellow-like tone), bold. |
| A.3.10 | I set each built-in theme and view a loading spinner. | The spinner animation is visible and colored distinctly from the background. |

### A.4 Invalid Theme Configuration

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | `~/.a9s/config.yaml` contains `theme: "nonexistent"`. I launch a9s. | The application starts successfully. It falls back to the default Tokyo Night Dark theme. A warning is logged or shown (e.g., as a transient flash or log message) indicating the theme name was not recognized. The app does NOT crash. |
| A.4.2 | `~/.a9s/config.yaml` contains `theme: ""` (empty string). I launch a9s. | The application starts with Tokyo Night Dark as the default. No crash. |
| A.4.3 | `~/.a9s/config.yaml` contains `theme: 42` (non-string value). I launch a9s. | The application handles the type mismatch gracefully. It falls back to Tokyo Night Dark. No crash. |
| A.4.4 | `~/.a9s/config.yaml` is malformed YAML (syntax error, e.g., missing colon). I launch a9s. | The application starts with defaults. A warning or error flash indicates the config file could not be parsed. The app does NOT crash. |

---

## B. Custom Theme Files

### B.1 Full Custom Theme

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I create `~/.a9s/themes/my-theme.yaml` with all 33 color slots defined (valid hex values). `~/.a9s/config.yaml` contains `theme: "~/.a9s/themes/my-theme.yaml"`. I launch a9s. | The application loads the custom theme. Every UI element uses the colors from my custom theme file: header foreground, accent, dim, border, row selection, status colors, detail keys/values, YAML syntax, help colors, filter/command/flash colors, spinner, scroll indicator. |
| B.1.2 | My custom theme uses `accent: "#ff0000"` (bright red). I open the help screen. | The accent color applies to elements that use the accent slot: header "a9s" text, table column headers, etc. The bright red is visible and applied consistently across views. |
| B.1.3 | I verify all 33 color slots are respected by checking each UI element against my custom palette. | Every color slot defined in the theme palette structure (per `docs/design/themes.md`) maps to at least one visible UI element. No hardcoded colors override custom theme values. |

**AWS comparison:**
```
aws ec2 describe-instances --output table
```
Expected: Same data content. Custom theme colors applied to all rendering.

### B.2 Partial Custom Theme

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | I create a custom theme file with only `accent: "#ff0000"` defined (1 of 33 color slots). `~/.a9s/config.yaml` points to this file. I launch a9s. | The accent color is red. All other 32 colors inherit from the Tokyo Night Dark defaults. The application renders correctly with no missing or blank colors. |
| B.2.2 | I create a custom theme with only `running: "#00ff00"`, `stopped: "#ff0000"`, and `pending: "#ffff00"` defined (3 status colors). I open the EC2 list. | Running instances use my bright green, stopped instances use my bright red, pending instances use my bright yellow. All other UI elements (header, borders, YAML syntax, etc.) use Tokyo Night Dark defaults. |
| B.2.3 | I create a custom theme with only YAML syntax colors defined: `yaml_key`, `yaml_str`, `yaml_num`, `yaml_bool`, `yaml_null`. I press y on an EC2 instance. | The YAML view uses my custom syntax colors. The rest of the application (header, list view, detail view, help) uses Tokyo Night Dark defaults. |
| B.2.4 | I create a custom theme with only `name: "My Theme"` and no colors section. I launch a9s. | The application uses all Tokyo Night Dark defaults. The theme name is accepted but no colors are overridden. |

**AWS comparison:**
```
aws ec2 describe-instances --instance-ids i-abc123 --output yaml
```
Expected: YAML content matches. Only YAML syntax highlighting colors differ.

### B.3 Invalid Custom Theme Files

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | `~/.a9s/config.yaml` contains `theme: "~/.a9s/themes/my-theme.yaml"` but the file does not exist at that path. I launch a9s. | The application falls back to Tokyo Night Dark. A warning or error is shown indicating the theme file was not found. The app does NOT crash. |
| B.3.2 | The custom theme file exists but contains malformed YAML (e.g., `colors:\n  accent: [invalid`). I launch a9s. | The application falls back to Tokyo Night Dark. A clear error message indicates the theme file could not be parsed. The app does NOT crash. |
| B.3.3 | The custom theme file contains a color with an invalid hex value (e.g., `accent: "not-a-color"`). I launch a9s. | The application handles the invalid hex gracefully: either the specific invalid color falls back to the default, or the entire theme falls back. A clear error message describes which color value is invalid. |
| B.3.4 | The custom theme file contains extra unknown keys (e.g., `colors:\n  accent: "#ff0000"\n  unicorn: "#123456"`). I launch a9s. | Unknown keys are silently ignored. The recognized colors are applied. The app does NOT crash or warn about unknown keys. |
| B.3.5 | The custom theme file is empty (0 bytes). I launch a9s. | The application falls back to Tokyo Night Dark. No crash. |
| B.3.6 | The custom theme file path uses tilde expansion: `theme: "~/my-theme.yaml"`. I launch a9s. | The tilde is expanded to the user's home directory. If the file exists there, it loads. If not, the error references the expanded path. |

### B.4 Config Lookup Chain

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | Both `.a9s/config.yaml` (project-local) and `~/.a9s/config.yaml` (home) exist with different theme values. I launch a9s from the project directory. | The project-local config takes precedence. The theme from `.a9s/config.yaml` is used. |
| B.4.2 | Only `~/.a9s/config.yaml` exists (no project-local config). I launch a9s. | The home directory config is used for theme selection. |
| B.4.3 | Neither config file exists. I launch a9s. | Tokyo Night Dark is used. No errors or warnings about missing config. |

---

## C. NO_COLOR Environment Variable

| ID | Story | Expected |
|----|-------|----------|
| C.1 | `NO_COLOR=1` is set. No config.yaml exists. I launch a9s. | All colors are stripped. The application renders in plain monochrome terminal text. No colored output whatsoever: no blue headers, no green/red status rows, no colored YAML syntax. Structural elements (borders, text content, key-value pairs) remain visible and readable. |
| C.2 | `NO_COLOR=1` is set. `~/.a9s/config.yaml` contains `theme: "dracula"`. I launch a9s. | `NO_COLOR` overrides the Dracula theme. All output is monochrome. The theme setting is ignored. |
| C.3 | `NO_COLOR=1` is set. `~/.a9s/config.yaml` points to a custom theme file. I launch a9s. | `NO_COLOR` overrides the custom theme. All output is monochrome. |
| C.4 | `NO_COLOR=1` is set. I open the YAML view for an EC2 instance. | YAML renders without syntax highlighting. Keys, strings, numbers, booleans, and nulls all appear in the same plain text color. Tree connectors are still visible as text characters. |
| C.5 | `NO_COLOR=1` is set. I open the help screen. | Help screen renders in monochrome. Categories, keys, and descriptions are distinguishable by layout (columns, alignment) but not by color. Bold styling may or may not be preserved depending on terminal capability. |
| C.6 | `NO_COLOR=1` is set. I select a running EC2 instance and a stopped instance. | Both rows appear in the same color (no green/red distinction). The selected row may still appear distinct via bold or reverse video if the terminal supports it, or it relies solely on the cursor position indicator. |
| C.7 | `NO_COLOR=1` is set. I copy a resource ID (c). | The "Copied!" flash still appears in the header (as plain text) and auto-clears after ~2 seconds. Functionality is preserved; only color is removed. |
| C.8 | `NO_COLOR` is set to empty string (`NO_COLOR=`). I launch a9s with a theme configured. | Per the NO_COLOR specification, any value (including empty) means no color. All output is monochrome. |

**AWS comparison:**
```
NO_COLOR=1 aws ec2 describe-instances --output table
```
Expected: AWS CLI also respects NO_COLOR for its own formatting. a9s should behave equivalently.

---

## D. Column Order -- Name First Audit

Issue #23 requires that every resource type's first visible column is a human-readable name, not an opaque ID. This section audits every resource type defined in `views.yaml`.

### D.1 Resources Currently Name-First (Verify Unchanged)

| ID | Resource | First Column | Path | Expected |
|----|----------|-------------|------|----------|
| D.1.1 | S3 Buckets (`s3`) | Bucket Name | `Name` | First column is "Bucket Name". Verified against `aws s3api list-buckets --query 'Buckets[].Name'`. |
| D.1.2 | S3 Objects (`s3_objects`) | Key | `Key` | First column is "Key" (the object's name). Verified against `aws s3api list-objects-v2 --bucket BUCKET --query 'Contents[].Key'`. |
| D.1.3 | EC2 Instances (`ec2`) | Name | Tag:Name | First column is "Name" (derived from the Name tag). Verified against `aws ec2 describe-instances --query 'Reservations[].Instances[].Tags[?Key==\`Name\`].Value'`. |
| D.1.4 | RDS Instances (`dbi`) | DB Identifier | `DBInstanceIdentifier` | First column is "DB Identifier". This IS the human-readable name for RDS (RDS instances are identified by their identifier, not a separate Name tag). Verified against `aws rds describe-db-instances --query 'DBInstances[].DBInstanceIdentifier'`. |
| D.1.5 | ElastiCache Redis (`redis`) | Cluster ID | `CacheClusterId` | First column is "Cluster ID". This IS the human-readable name for Redis clusters. Verified against `aws elasticache describe-cache-clusters --query 'CacheClusters[].CacheClusterId'`. |
| D.1.6 | DocumentDB Clusters (`dbc`) | Cluster ID | `DBClusterIdentifier` | First column is "Cluster ID". This IS the human-readable name for DocumentDB. Verified against `aws docdb describe-db-clusters --query 'DBClusters[].DBClusterIdentifier'`. |
| D.1.7 | EKS Clusters (`eks`) | Cluster Name | `Name` | First column is "Cluster Name". Verified against `aws eks list-clusters`. |
| D.1.8 | Secrets Manager (`secrets`) | Secret Name | `Name` | First column is "Secret Name". Verified against `aws secretsmanager list-secrets --query 'SecretList[].Name'`. |
| D.1.9 | EKS Node Groups (`ng`) | Node Group | `NodegroupName` | First column is "Node Group". Verified against `aws eks list-nodegroups --cluster-name CLUSTER`. |
| D.1.10 | Lambda (`lambda`) | Function Name | `FunctionName` | First column is "Function Name". Verified against `aws lambda list-functions --query 'Functions[].FunctionName'`. |
| D.1.11 | CloudWatch Alarms (`alarm`) | Alarm Name | `AlarmName` | First column is "Alarm Name". Verified against `aws cloudwatch describe-alarms --query 'MetricAlarms[].AlarmName'`. |
| D.1.12 | SNS Topics (`sns`) | Topic Name | `TopicArn` | First column is "Topic Name" (extracted from ARN). Verified against `aws sns list-topics --query 'Topics[].TopicArn'`. |
| D.1.13 | SQS Queues (`sqs`) | Queue Name | key: `queue_name` | First column is "Queue Name" (extracted from QueueUrl). Verified against `aws sqs list-queues`. |
| D.1.14 | ELB (`elb`) | Name | `LoadBalancerName` | First column is "Name". Verified against `aws elbv2 describe-load-balancers --query 'LoadBalancers[].LoadBalancerName'`. |
| D.1.15 | Target Groups (`tg`) | Target Group | `TargetGroupName` | First column is "Target Group". Verified against `aws elbv2 describe-target-groups --query 'TargetGroups[].TargetGroupName'`. |
| D.1.16 | ECS Clusters (`ecs`) | Cluster Name | `ClusterName` | First column is "Cluster Name". Verified against `aws ecs list-clusters`. |
| D.1.17 | ECS Services (`ecs-svc`) | Service Name | `ServiceName` | First column is "Service Name". Verified against `aws ecs list-services --cluster CLUSTER`. |
| D.1.18 | CloudFormation (`cfn`) | Stack Name | `StackName` | First column is "Stack Name". Verified against `aws cloudformation list-stacks --query 'StackSummaries[].StackName'`. |
| D.1.19 | IAM Roles (`role`) | Role Name | `RoleName` | First column is "Role Name". Verified against `aws iam list-roles --query 'Roles[].RoleName'`. |
| D.1.20 | CloudWatch Logs (`logs`) | Log Group Name | `LogGroupName` | First column is "Log Group Name". Verified against `aws logs describe-log-groups --query 'logGroups[].logGroupName'`. |
| D.1.21 | SSM Parameters (`ssm`) | Name | `Name` | First column is "Name". Verified against `aws ssm describe-parameters --query 'Parameters[].Name'`. |
| D.1.22 | DynamoDB Tables (`ddb`) | Table Name | `TableName` | First column is "Table Name". Verified against `aws dynamodb list-tables`. |
| D.1.23 | ACM Certificates (`acm`) | Domain Name | `DomainName` | First column is "Domain Name". Verified against `aws acm list-certificates --query 'CertificateSummaryList[].DomainName'`. |
| D.1.24 | Auto Scaling Groups (`asg`) | ASG Name | `AutoScalingGroupName` | First column is "ASG Name". Verified against `aws autoscaling describe-auto-scaling-groups --query 'AutoScalingGroups[].AutoScalingGroupName'`. |
| D.1.25 | Route 53 Records (`r53_records`) | Name | `Name` | First column is "Name". Verified against `aws route53 list-resource-record-sets --hosted-zone-id ZONE --query 'ResourceRecordSets[].Name'`. |

### D.2 Resources Requiring Column Reorder (ID-First -> Name-First)

These resource types currently have an ID column first and a Name column second. After Issue #23, the Name column should be first.

| ID | Resource | Before (First Col) | After (First Col) | Expected |
|----|----------|--------------------|--------------------|----------|
| D.2.1 | VPCs (`vpc`) | VPC ID (`VpcId`) | Name (Tag:Name) | First column becomes "Name". "VPC ID" moves to the second column. VPC names derived from the Name tag are now the first thing users see. Verified against `aws ec2 describe-vpcs --query 'Vpcs[].{ID:VpcId,Name:Tags[?Key==\`Name\`].Value|[0]}'`. |
| D.2.2 | Security Groups (`sg`) | Group ID (`GroupId`) | Group Name (`GroupName`) | First column becomes "Group Name". "Group ID" moves to the second column. Verified against `aws ec2 describe-security-groups --query 'SecurityGroups[].{Name:GroupName,ID:GroupId}'`. |
| D.2.3 | Subnets (`subnet`) | Subnet ID (`SubnetId`) | Name (Tag:Name) | First column becomes "Name". "Subnet ID" moves to the second column. Verified against `aws ec2 describe-subnets --query 'Subnets[].{ID:SubnetId,Name:Tags[?Key==\`Name\`].Value|[0]}'`. |
| D.2.4 | Route Tables (`rtb`) | Route Table ID (`RouteTableId`) | Name (Tag:Name) | First column becomes "Name". "Route Table ID" moves to the second column. Verified against `aws ec2 describe-route-tables --query 'RouteTables[].{ID:RouteTableId,Name:Tags[?Key==\`Name\`].Value|[0]}'`. |
| D.2.5 | NAT Gateways (`nat`) | NAT Gateway ID (`NatGatewayId`) | Name (Tag:Name) | First column becomes "Name". "NAT Gateway ID" moves to the second column. Verified against `aws ec2 describe-nat-gateways --query 'NatGateways[].{ID:NatGatewayId,Name:Tags[?Key==\`Name\`].Value|[0]}'`. |
| D.2.6 | Internet Gateways (`igw`) | IGW ID (`InternetGatewayId`) | Name (Tag:Name) | First column becomes "Name". "IGW ID" moves to the second column. Verified against `aws ec2 describe-internet-gateways --query 'InternetGateways[].{ID:InternetGatewayId,Name:Tags[?Key==\`Name\`].Value|[0]}'`. |
| D.2.7 | Elastic IPs (`eip`) | Allocation ID (`AllocationId`) | Name (Tag:Name) | First column becomes "Name". "Allocation ID" moves to the second column. Verified against `aws ec2 describe-addresses --query 'Addresses[].{ID:AllocationId,Name:Tags[?Key==\`Name\`].Value|[0]}'`. |

### D.3 Name-First Column Behavior After Reorder

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | After reorder, I open VPCs. Some VPCs have a Name tag and some do not. | VPCs with a Name tag show the name in the first column. VPCs without a Name tag show a dash, empty cell, or the VPC ID as a fallback. The display is not blank or confusing -- there is always something meaningful in the first column. |
| D.3.2 | After reorder, I open Security Groups. I sort by pressing N. | Sort is applied to the first column (Group Name). Rows sort alphabetically by security group name. The sort indicator arrow appears on the "Group Name" column header. |
| D.3.3 | After reorder, I open Subnets. I enter filter mode (/) and type "prod". | Filtering matches against all visible columns including the new first column (Name). Subnets named "prod-private-1a" or similar match the filter. The frame title shows matched/total count. |
| D.3.4 | After reorder, I open VPCs. I press c to copy the resource ID. | The copy action copies the VPC ID (the resource identifier), NOT the Name tag. The "Copied!" flash appears. Pasting produces the VPC ID (e.g., `vpc-0abc123def456`). |
| D.3.5 | After reorder, I open Security Groups. I press d to view detail. | The detail view opens for the selected security group. All detail fields (GroupId, GroupName, VpcId, Description, etc.) are displayed correctly regardless of column order changes. |
| D.3.6 | After reorder, I open Route Tables. The terminal is 80 columns wide. | The Name column is visible as the first column. If not all columns fit, rightmost columns are hidden. Horizontal scroll with h/l still works. The Name column is always visible without scrolling. |
| D.3.7 | After reorder, I open Internet Gateways. No IGWs have a Name tag. | The first column (Name) shows dashes or empty values for every row. The IGW ID column (now second) still shows the ID. The list is still usable -- sorting by name sorts the dashes, and the IGW ID remains visible for identification. |
| D.3.8 | After reorder, I open Elastic IPs. I verify the column headers. | The column order is: Name, Allocation ID, Public IP, Association, Instance, Domain. The header row shows these columns in this order. |
| D.3.9 | After reorder, I open NAT Gateways. I press h/l to scroll columns. | Horizontal scrolling works correctly with the new column order. Scrolling left shows the leftmost columns (Name first). Scrolling right reveals rightmost columns. Column headers scroll in sync with data. |

### D.4 Exception Documentation

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | I review RDS Instances (`dbi`). The first column is "DB Identifier". | This is an acceptable exception. RDS instances are identified by their DB Identifier, which IS their human-readable name. There is no separate Name tag on RDS instances. This should be documented as an intentional exception. |
| D.4.2 | I review ElastiCache Redis (`redis`). The first column is "Cluster ID". | Acceptable exception. Redis clusters are identified by their Cluster ID, which IS their human-readable name. Documented. |
| D.4.3 | I review DocumentDB Clusters (`dbc`). The first column is "Cluster ID". | Acceptable exception. DocumentDB clusters are identified by their Cluster Identifier, which IS their human-readable name. Documented. |
| D.4.4 | I review SNS Topics (`sns`). The first column is "Topic Name" extracted from TopicArn. | This is correct. The topic name is extracted from the ARN since SNS topics don't have a separate Name field. The column is already name-first. |
| D.4.5 | I review SQS Queues (`sqs`). The first column is "Queue Name" extracted from QueueUrl. | This is correct. The queue name is extracted from the URL since SQS queues don't have a separate Name field. The column is already name-first. |

---

## E. Pagination -- Small Data Sets (Baseline)

### E.1 Small EC2 Set

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | My account has 5 EC2 instances. I open the EC2 list. | All 5 instances load quickly (under 2 seconds). The frame title shows "ec2-instances(5)". All 5 rows are visible without scrolling on a standard 24-line terminal. No pagination indicators, no "load more" prompt. |
| E.1.2 | I navigate with j/k through all 5 instances. | Navigation is instantaneous. No lag between keypresses and cursor movement. |
| E.1.3 | I sort by name (N), then by status (S), then by age (A). | Each sort completes instantly. No visible delay. |
| E.1.4 | I filter with "/" and type "prod". | Filtering is instant. Matched rows appear immediately as I type each character. |
| E.1.5 | I press ctrl+r to refresh. | The spinner appears briefly, the API call completes, and the table repopulates. Total refresh time depends on AWS API latency, not on rendering overhead. |

**AWS comparison:**
```
aws ec2 describe-instances --query 'Reservations[].Instances[] | length(@)'
```
Expected: Result is 5. All 5 are displayed.

### E.2 Small S3 Set

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | My account has 10 S3 buckets. I open the S3 list. | All 10 buckets load quickly. Frame title shows "s3(10)". All rows visible. |
| E.2.2 | I enter a bucket with 50 objects. | All 50 objects load. Frame title shows bucket name with count 50. Objects are listed with Key, Size, Storage Class, Last Modified columns. |

**AWS comparison:**
```
aws s3api list-buckets --query 'Buckets | length(@)'
aws s3api list-objects-v2 --bucket my-bucket --query 'KeyCount'
```

---

## F. Pagination -- Large Data Sets

### F.1 Large EC2 Instance List

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | My account has 5,000 EC2 instances across all states. I open the EC2 list. | The loading spinner appears. Data loads without the application becoming unresponsive. The frame title eventually shows the total count (e.g., "ec2-instances(5000)" or a capped count). If pagination is capped, the title indicates the cap (e.g., "ec2-instances(1000+)"). |
| F.1.2 | The 5,000 instances finish loading. I press j to move down. | Navigation responds within a frame refresh (typically under 100ms). There is no perceptible lag when moving the cursor. The UI does NOT freeze or stutter. |
| F.1.3 | I press G to jump to the last of 5,000 rows. | The cursor jumps to row 5,000 (or the capped maximum). The view scrolls to show the last rows. The jump is immediate -- no progressive scrolling through all intermediate rows. |
| F.1.4 | I press g to jump back to the first row from row 5,000. | The cursor returns to row 1 instantly. |
| F.1.5 | I press PageDown repeatedly to scroll through 5,000 rows. | Each page-down scrolls by one page of visible rows. There is no accumulating delay -- the 50th page-down is as fast as the 1st. |
| F.1.6 | I enter filter mode (/) and type "api". | The filter scans all 5,000 rows and displays only matches. The filtering completes within 1 second. The frame title updates to show matched/total (e.g., "ec2-instances(150/5000)"). Typing additional characters further narrows results without noticeable delay. |
| F.1.7 | I sort by name (N) on 5,000 rows. | The sort completes within 1 second. The sort indicator appears on the NAME column. The selected row may change position but the cursor remains on a valid row. |
| F.1.8 | I sort by status (S) on 5,000 rows, then toggle to descending (S again). | Both sort operations complete quickly. No UI freeze. |
| F.1.9 | I press ctrl+r to refresh the 5,000-instance list. | The spinner appears. Multiple paginated API calls are made behind the scenes. The total refresh may take several seconds (AWS API pagination), but the UI remains responsive (the spinner animates). The table repopulates when all data arrives. |

**AWS comparison:**
```
aws ec2 describe-instances --query 'Reservations[].Instances[] | length(@)'
# May require multiple pages: aws ec2 describe-instances --max-results 1000
```
Expected: a9s fetches all pages automatically, matching the total count from CLI.

### F.2 Large S3 Object List

| ID | Story | Expected |
|----|-------|----------|
| F.2.1 | I enter a bucket containing 100,000 objects. I press Enter on the bucket. | The loading spinner appears. The application begins fetching objects. Given the volume, fetching may take significant time. The UI remains responsive (spinner animates). |
| F.2.2 | The bucket has 100,000 objects and pagination is implemented with a cap or "load more". | Either: (a) The first batch (e.g., 1,000 objects) loads and displays with an indicator that more exist, or (b) all objects are fetched via paginated API calls before display, with the spinner showing throughout. The approach should be documented. |
| F.2.3 | If "load more" is implemented: I scroll to the bottom of the initial batch. | A visual indicator (e.g., "Load more..." row, or automatic loading) signals that additional objects are available. Pressing Enter or scrolling past the last row triggers loading the next batch. |
| F.2.4 | I filter the 100,000 objects by key prefix (/ then "logs/2024"). | Filtering applies to all loaded objects. If only a partial batch is loaded, the filter applies to the loaded set and this is indicated in the frame title (e.g., "my-bucket(42/1000)" not "42/100000"). |
| F.2.5 | I navigate into a prefix ("logs/") inside a bucket with 100,000 objects. | The prefix navigation calls `list-objects-v2` with the `Prefix` parameter, which returns only objects under that prefix. This is a new API call scoped to the prefix, not a client-side filter of all 100,000 objects. |
| F.2.6 | I press ctrl+r to refresh a bucket with 100,000 objects. | The refresh re-fetches objects. If a fetch cap is in place, it refreshes up to the cap. The spinner indicates the refresh is in progress. |

**AWS comparison:**
```
aws s3api list-objects-v2 --bucket my-bucket --query 'KeyCount'
aws s3api list-objects-v2 --bucket my-bucket --prefix "logs/" --query 'KeyCount'
aws s3api list-objects-v2 --bucket my-bucket --max-keys 1000
```

### F.3 Large CloudWatch Log Streams

| ID | Story | Expected |
|----|-------|----------|
| F.3.1 | A log group has 10,000 log streams. I open the log group. | The loading spinner appears. Log streams load via paginated API calls. The UI remains responsive during loading. |
| F.3.2 | All 10,000 log streams are loaded (or capped). I navigate with j/k. | Navigation is smooth. No lag between keypress and cursor movement, even at the bottom of the list. |
| F.3.3 | I filter the 10,000 log streams by name. | The filter completes within 1 second. Only matching streams are displayed. |

**AWS comparison:**
```
aws logs describe-log-streams --log-group-name /my/log/group --query 'logStreams | length(@)'
# Paginated: --limit 50 --next-token ...
```

### F.4 API Throttling

| ID | Story | Expected |
|----|-------|----------|
| F.4.1 | Fetching 5,000 EC2 instances triggers AWS API rate limiting (ThrottlingException). | The application handles throttling gracefully: it retries with exponential backoff. The spinner continues to animate during retries. No crash. Eventually, the data loads or a clear error message is shown if retries are exhausted. |
| F.4.2 | Fetching S3 objects triggers a rate limit. | Same behavior as F.4.1: graceful retry with backoff, spinner during retries, eventual success or clear error. |
| F.4.3 | I press ctrl+r rapidly (3 times in quick succession). | The application does not send 3 concurrent refresh requests. It either debounces (only the last refresh fires) or cancels the in-flight request before starting a new one. API rate limits are not needlessly consumed. |

### F.5 Max Items Configuration

| ID | Story | Expected |
|----|-------|----------|
| F.5.1 | `~/.a9s/config.yaml` contains `max_items: 500`. My account has 2,000 EC2 instances. I open the EC2 list. | The fetcher stops after retrieving 500 instances. The frame title indicates the cap (e.g., "ec2-instances(500+)" or "ec2-instances(500)"). Navigation, sort, and filter operate on the 500 loaded items. |
| F.5.2 | `max_items: 500` is configured. I filter the 500 loaded EC2 instances. | The filter operates on the 500 loaded items. The user understands that matches beyond the first 500 are not included. The frame title reflects the filter against the loaded set. |
| F.5.3 | `max_items: 500` is configured. I sort the 500 loaded EC2 instances by name. | Sort operates on the loaded set of 500. The sort is consistent and fast. |
| F.5.4 | No `max_items` is configured (default). My account has 2,000 EC2 instances. | All 2,000 instances are fetched (all pagination pages). The default behavior fetches all available resources unless a cap is configured. |
| F.5.5 | `max_items: 0` is configured. | This is treated as "no limit" (same as default) or produces a clear error. It does NOT mean "fetch zero items". |
| F.5.6 | `max_items: 10` is configured. My account has 5 EC2 instances. | All 5 instances load. The cap of 10 is higher than the actual count, so the full set is displayed. Frame title shows "ec2-instances(5)". |

**AWS comparison:**
```
aws ec2 describe-instances --max-results 500 --query 'Reservations[].Instances[] | length(@)'
```
Expected: a9s respects max_items similarly to `--max-results`.

---

## G. Virtual Scrolling and Rendering Performance

### G.1 Visible Row Rendering

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | 5,000 EC2 instances are loaded. The terminal has 24 lines (approximately 20 visible data rows). | Only the 20 visible rows are rendered to the terminal at any given time. The application does NOT render all 5,000 rows and then clip. Memory usage stays constant regardless of total row count. |
| G.1.2 | I scroll down through 5,000 rows continuously by holding j. | The frame rate remains smooth. There is no progressive slowdown as the cursor moves deeper into the list. Row 4,990 renders as quickly as row 10. |
| G.1.3 | I resize the terminal from 24 lines to 50 lines while viewing 5,000 rows. | The visible row count increases. More rows become visible. The rendering adjusts instantly. No re-fetch from AWS occurs. |
| G.1.4 | I resize the terminal from 50 lines back to 24 lines. | The visible row count decreases. Fewer rows are visible. If the selected row is still in view, it remains selected and highlighted. If the selected row is now off-screen, the view scrolls to keep it visible. |

### G.2 Column Rendering with Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| G.2.1 | 5,000 EC2 instances with 7 columns. Terminal is 80 columns wide (some columns hidden). | Only visible columns are rendered. Pressing l reveals the next column and hides the leftmost. The scroll is instant even with 5,000 rows because only visible cells are drawn. |
| G.2.2 | I rapidly press l to scroll through all 7 columns on a 5,000-row list. | Each horizontal scroll step completes instantly. No lag or stutter. |

---

## H. Child View Pagination (High-Cardinality Drill-Downs)

### H.1 S3 Bucket with Many Objects

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | I enter a bucket with 1 million objects. | The application does not attempt to load all 1 million objects at once. It loads a bounded first page (e.g., 1,000) and provides a mechanism to load more. The UI does not freeze. |
| H.1.2 | I navigate into a prefix with 50,000 objects under it. | Same bounded loading behavior. The prefix scopes the API call, but if the prefix itself has 50,000 objects, pagination or capping is applied. |
| H.1.3 | I navigate into a deeply nested prefix path: "a/b/c/d/e/f/" containing 10 objects. | The nested prefix loads quickly since only 10 objects exist. The full prefix path is reflected in the frame title or context. |

### H.2 ECS Tasks (High Cardinality)

| ID | Story | Expected |
|----|-------|----------|
| H.2.1 | An ECS cluster has 500 running tasks. I drill into the cluster's tasks. | Tasks load via paginated API calls. The list renders with a count in the frame title. Navigation is smooth. |

**AWS comparison:**
```
aws ecs list-tasks --cluster my-cluster --query 'taskArns | length(@)'
```

### H.3 CloudWatch Log Events

| ID | Story | Expected |
|----|-------|----------|
| H.3.1 | A log stream has millions of log events. I open the log stream. | The application loads a bounded window of recent log events (not all events since the stream's creation). The frame title indicates the loaded count. |
| H.3.2 | I scroll to the top of the loaded log events. | If "load older" is supported, a visual indicator prompts for loading earlier events. If not, the top of the loaded window is the first visible event. |

**AWS comparison:**
```
aws logs get-log-events --log-group-name /my/log --log-stream-name stream --limit 100
```

---

## I. Fetcher Pagination Audit

### I.1 Fetcher Behavior per Resource Type

Each AWS fetcher should handle pagination correctly. This section verifies that each resource type's fetcher retrieves all available resources (or respects a configured cap).

| ID | Resource | AWS API | Pagination Expected | Story |
|----|----------|---------|---------------------|-------|
| I.1.1 | EC2 (`ec2`) | `DescribeInstances` | Uses `NextToken` for pages of up to 1,000 | Fetcher follows all NextToken pages until no more remain. An account with 2,500 instances returns all 2,500 (3 API pages). |
| I.1.2 | S3 Buckets (`s3`) | `ListBuckets` | Returns all buckets (no pagination, max ~100) | All buckets load in a single call. No pagination needed. |
| I.1.3 | S3 Objects (`s3_objects`) | `ListObjectsV2` | Uses `ContinuationToken` for pages of up to 1,000 | Fetcher follows ContinuationToken. A bucket with 5,000 objects requires 5 API pages. All 5,000 are returned (or capped per config). |
| I.1.4 | RDS (`dbi`) | `DescribeDBInstances` | Uses `Marker` for pages of up to 100 | Fetcher follows Marker. An account with 250 RDS instances requires 3 API pages. |
| I.1.5 | ElastiCache (`redis`) | `DescribeCacheClusters` | Uses `Marker` for pages of up to 100 | Fetcher follows Marker. |
| I.1.6 | DocumentDB (`dbc`) | `DescribeDBClusters` | Uses `Marker` for pages of up to 100 | Fetcher follows Marker. |
| I.1.7 | EKS (`eks`) | `ListClusters` + `DescribeCluster` | `ListClusters` uses `NextToken`, max 100 per page | Fetcher follows NextToken for cluster names, then describes each. An account with 150 clusters makes 2 list calls + 150 describe calls. |
| I.1.8 | Secrets Manager (`secrets`) | `ListSecrets` | Uses `NextToken` for pages of up to 100 | Fetcher follows NextToken. |
| I.1.9 | VPCs (`vpc`) | `DescribeVpcs` | Uses `NextToken` for pages of up to 200 | Fetcher follows NextToken. |
| I.1.10 | Security Groups (`sg`) | `DescribeSecurityGroups` | Uses `NextToken` for pages of up to 1,000 | Fetcher follows NextToken. |
| I.1.11 | Subnets (`subnet`) | `DescribeSubnets` | Uses `NextToken` for pages of up to 200 | Fetcher follows NextToken. |
| I.1.12 | Lambda (`lambda`) | `ListFunctions` | Uses `Marker` for pages of up to 50 | Fetcher follows Marker. An account with 200 functions requires 4 API pages. |
| I.1.13 | CloudWatch Alarms (`alarm`) | `DescribeAlarms` | Uses `NextToken` for pages of up to 100 | Fetcher follows NextToken. |
| I.1.14 | SNS Topics (`sns`) | `ListTopics` | Uses `NextToken`, returns up to 100 per page | Fetcher follows NextToken. |
| I.1.15 | SQS Queues (`sqs`) | `ListQueues` | Uses `NextToken`, returns up to 1,000 per page | Fetcher follows NextToken. Then `GetQueueAttributes` for each. |
| I.1.16 | ELB (`elb`) | `DescribeLoadBalancers` | Uses `Marker` for pages of up to 400 | Fetcher follows Marker. |
| I.1.17 | Target Groups (`tg`) | `DescribeTargetGroups` | Uses `Marker` for pages of up to 400 | Fetcher follows Marker. |
| I.1.18 | ECS Clusters (`ecs`) | `ListClusters` + `DescribeClusters` | `ListClusters` uses `NextToken`, max 100 | Fetcher follows NextToken, then batch-describes (up to 100 per describe call). |
| I.1.19 | ECS Services (`ecs-svc`) | `ListServices` + `DescribeServices` | `ListServices` uses `NextToken`, max 100 | Fetcher follows NextToken, then batch-describes (up to 10 per describe call). |
| I.1.20 | CloudFormation (`cfn`) | `DescribeStacks` | Uses `NextToken` for pages | Fetcher follows NextToken. |
| I.1.21 | IAM Roles (`role`) | `ListRoles` | Uses `Marker` for pages of up to 100 | Fetcher follows Marker. IAM roles can number in the thousands in large accounts. |
| I.1.22 | CloudWatch Logs (`logs`) | `DescribeLogGroups` | Uses `NextToken` for pages of up to 50 | Fetcher follows NextToken. An account with 500 log groups requires 10 API pages. |
| I.1.23 | SSM Parameters (`ssm`) | `DescribeParameters` | Uses `NextToken` for pages of up to 50 | Fetcher follows NextToken. |
| I.1.24 | DynamoDB (`ddb`) | `ListTables` + `DescribeTable` | `ListTables` uses `ExclusiveStartTableName`, max 100 | Fetcher follows pagination. Then describes each table individually. |
| I.1.25 | Elastic IPs (`eip`) | `DescribeAddresses` | No pagination (returns all, typically small count) | All EIPs returned in a single call. No pagination handling needed. |
| I.1.26 | ACM (`acm`) | `ListCertificates` | Uses `NextToken` for pages of up to 1,000 | Fetcher follows NextToken. |
| I.1.27 | ASG (`asg`) | `DescribeAutoScalingGroups` | Uses `NextToken` for pages of up to 100 | Fetcher follows NextToken. |
| I.1.28 | Route Tables (`rtb`) | `DescribeRouteTables` | Uses `NextToken` for pages of up to 200 | Fetcher follows NextToken. |
| I.1.29 | NAT Gateways (`nat`) | `DescribeNatGateways` | Uses `NextToken` for pages of up to 1,000 | Fetcher follows NextToken. |
| I.1.30 | Internet Gateways (`igw`) | `DescribeInternetGateways` | Uses `NextToken` for pages of up to 200 | Fetcher follows NextToken. |
| I.1.31 | EKS Node Groups (`ng`) | `ListNodegroups` + `DescribeNodegroup` | Uses `NextToken`, max 100 per page | Fetcher follows NextToken for names, then describes each. |
| I.1.32 | Route 53 Records (`r53_records`) | `ListResourceRecordSets` | Uses `NextRecordName`/`NextRecordType` for pages of up to 300 | Fetcher follows pagination tokens. Hosted zones with 10,000 records require ~34 API pages. |

---

## J. UI Responsiveness Under Load

### J.1 No UI Freezes

| ID | Story | Expected |
|----|-------|----------|
| J.1.1 | 5,000 EC2 instances are being fetched (multiple API pages in flight). I press Escape to go back to the main menu. | The Escape key is processed immediately. I return to the main menu. The in-flight API calls are cancelled or complete in the background without blocking the UI. |
| J.1.2 | A slow API call is in progress (e.g., describing 200 EKS clusters one-by-one). I press ? to open help. | The help screen opens immediately. The spinner or loading state continues in the background. When I close help, the loading spinner reappears if data has not yet finished loading. |
| J.1.3 | 5,000 rows are loaded. I rapidly press / to filter, type "test", press Escape, press / again, type "prod". | Each action responds within a frame refresh. No accumulated lag from rapid filter toggling. |
| J.1.4 | 5,000 rows are loaded. I press : to enter command mode and type "s3" + Enter to switch to S3 view. | The command is processed immediately. The S3 list begins loading. The 5,000 EC2 rows are released from memory (not kept indefinitely). |
| J.1.5 | During a long fetch (e.g., 100,000 S3 objects), I resize the terminal. | The layout reflows immediately. The spinner continues. The fetch is not interrupted by the resize event. |

### J.2 Memory Behavior

| ID | Story | Expected |
|----|-------|----------|
| J.2.1 | I load 5,000 EC2 instances, then navigate back to the main menu, then open S3 with 1,000 buckets. | Memory from the EC2 data set is released when navigating away (or at least is eligible for garbage collection). The application does not accumulate unbounded memory from previous views. |
| J.2.2 | I repeatedly open and close a resource list with 1,000 items (10 cycles). | Memory usage remains roughly constant across cycles. There is no memory leak from repeated view creation/destruction. |

---

## K. Cross-Cutting: Theme + Column Order + Pagination Interactions

### K.1 Theme Applied to Large Lists

| ID | Story | Expected |
|----|-------|----------|
| K.1.1 | `theme: "dracula"` is configured. 5,000 EC2 instances are loaded with mixed statuses. | All 5,000 rows use Dracula palette status colors. The Dracula running/stopped/pending/terminated colors are applied. Scrolling through the list, every row's color matches its status. The selected row uses Dracula's selection highlight. |
| K.1.2 | `theme: "solarized-light"` is configured. 5,000 EC2 instances are loaded. I scroll rapidly. | Light theme colors render correctly at high scroll speed. No flickering between default and themed colors. The palette is applied consistently to every visible row at every scroll position. |

### K.2 Column Order with Large Lists

| ID | Story | Expected |
|----|-------|----------|
| K.2.1 | After column reorder (Name first), 5,000 VPCs are loaded. I sort by Name (N). | The sort operates on the Name column (first column). With 5,000 VPCs, the sort completes within 1 second. VPCs are alphabetically ordered by their Name tag value. VPCs without a Name tag sort consistently (e.g., all at the top or all at the bottom). |
| K.2.2 | After column reorder, 3,000 security groups are loaded. I filter by name. | The filter matches against the Group Name (first column) and all other visible columns. Filtering 3,000 rows completes quickly. |

### K.3 NO_COLOR with Large Lists

| ID | Story | Expected |
|----|-------|----------|
| K.3.1 | `NO_COLOR=1` is set. 5,000 EC2 instances are loaded. | All rows render in monochrome. The lack of color does not affect performance. Navigation, sorting, and filtering all work identically to the colored version. |

### K.4 Theme Change with Pagination Config

| ID | Story | Expected |
|----|-------|----------|
| K.4.1 | `theme: "nord"` and `max_items: 500` are both configured. My account has 2,000 instances. | The Nord theme applies to the 500 loaded instances. The frame title shows the capped count. Both features work together without conflict. |

---

## L. Edge Cases and Error Recovery

### L.1 Theme Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| L.1.1 | A light theme is configured but my terminal has a light background already (white on white risk). | The light theme should use dark text on light backgrounds. If the terminal's own background matches the theme's row background, text should still be readable because the theme defines both foreground and background colors for key elements (selected row, alternating rows). |
| L.1.2 | I configure theme A, start a9s, then change config.yaml to theme B while a9s is running. | The theme does not hot-reload. The running instance continues with theme A. I must restart a9s to see theme B. This is expected behavior per the design doc ("theme is set at startup via config"). |
| L.1.3 | A custom theme defines `accent` and `detail_key` as the same color. | The theme loads without error. The visual result may be suboptimal (accent and detail keys are indistinguishable), but it is the user's choice. No validation prevents same-color assignments. |

### L.2 Column Order Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| L.2.1 | A user has a custom `views.yaml` that explicitly sets VPC ID as the first column (overriding the default). | The user's custom `views.yaml` takes precedence over built-in defaults. The VPC ID remains first per user configuration. Issue #23 changes only the built-in defaults, not user overrides. |
| L.2.2 | A resource type has no concept of "name" at all (hypothetical). | The resource should document this exception. The first column should be the most human-recognizable identifier available (e.g., ARN-derived name, or the primary identifier). |

### L.3 Pagination Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| L.3.1 | AWS API returns a NextToken but the next page is empty (0 items). | The fetcher handles the empty page gracefully and stops pagination. No infinite loop. |
| L.3.2 | Network connection drops mid-pagination (page 3 of 5). | The application shows an error for the partial failure. Either: (a) it displays the data loaded so far (pages 1-2) with an error indicator, or (b) it shows an error and allows retry with ctrl+r. It does NOT silently show partial data without indication. |
| L.3.3 | An AWS account has exactly 0 resources of a type. Pagination is configured. | The empty state displays correctly: "No [resources] found" with a hint to refresh or change region. No pagination indicators appear. Frame title shows count 0. |
| L.3.4 | `max_items: 1` is configured. Account has 100 instances. | Exactly 1 instance is fetched and displayed. The frame title shows the count (1 or "1+"). This is a degenerate but valid configuration. |
| L.3.5 | The paginated API call returns duplicate items across pages (theoretical edge case). | Duplicates are either deduplicated before display or shown as-is. The application does not crash on duplicate resource IDs. |

# QA User Stories: Bottom Border Key Hints (#197, closes #190)

Covers GitHub issue #197: embed contextual key hints directly into the bottom frame border line, breaking it the same way the resource title breaks the top border. Also satisfies #190 (feedforward navigation hints in detail view).

All stories are written from a black-box perspective. AWS CLI equivalents are cited where applicable. Key bindings, color values, and layout rules reference only the design spec (`docs/design/design.md`) and the feature spec (`docs/197-bottom-border-hints.md`).

---

## A. Bottom Border Rendering

### A.1 Plain Border (No Hints)

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I open the Help view. | The bottom border is a plain line: `└` followed by dashes filling the width, ending with `┘`. No key hints appear. |
| A.1.2 | I open the Profile Selector (`:ctx`). | The bottom border is a plain `└───...───┘` line with no embedded hints. |
| A.1.3 | I open the Region Selector (`:region`). | The bottom border is a plain `└───...───┘` line with no embedded hints. |
| A.1.4 | I press `x` on a secret to open the Reveal view. | The bottom border is a plain `└───...───┘` line with no embedded hints. The view has its own inline close hint. |
| A.1.5 | I open the Identity view. | The bottom border is a plain `└───...───┘` line with no embedded hints. The view has its own inline key hints. |

### A.2 Hint Visual Styling

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | I open the main menu and inspect the bottom border. | Key characters (e.g., `ctrl+r`) appear in accent blue (`#7aa2f7`) bold. Description text (e.g., `Refresh`) appears in dim (`#565f89`). Dashes and corner characters (`└`, `┘`) appear in border color (`#414868`). |
| A.2.2 | I open a resource list (e.g., EC2) and inspect the bottom border. | Each hint follows the pattern: `──key desc──`, where `key` is accent bold and `desc` is dim, separated by border-colored dashes. |
| A.2.3 | I compare the bottom border hint styling against the top border title. | The bottom border uses the same `└` and `┘` corner characters as the current plain border, and the same dash character `─` in the same border color (`#414868`). |
| A.2.4 | Multiple hints appear in the bottom border. | Hints are separated by at least two dash characters (`──`) between each hint. Remaining width after the last hint is filled with dashes to reach `┘`. |

### A.3 Corner Invariants

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | I view the bottom border at any terminal width (60, 80, 120, 200 cols). | The border always starts with `└` and ends with `┘`. The total visual width of the bottom border equals the terminal width exactly. |
| A.3.2 | I view the bottom border with zero hints (Help, Selector views). | The border is `└` + dashes filling `w-2` characters + `┘`, identical to the pre-feature plain border. |
| A.3.3 | I view the bottom border with hints that fill almost the entire width. | The border still ends with `┘` at the correct column. No overflow, no wrapping, no missing corner. |

---

## B. Main Menu Hints

### B.1 Normal State

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I launch a9s and view the main menu. | The bottom border shows: `└──ctrl+r Refresh──...──┘`. The `ctrl+r` text is accent blue bold; `Refresh` is dim. |
| B.1.2 | I navigate the main menu with `j`/`k`. | The bottom border hints remain unchanged regardless of which resource type is highlighted. |
| B.1.3 | I activate filter mode with `/` on the main menu. | The bottom border hints remain visible and unchanged. The filter input appears in the header, not the border. |

**AWS comparison:**
No direct CLI equivalent. This is a navigation aid for the resource type selection screen.

---

## C. Resource List Hints

### C.1 Resource List Without Enter-Child Override

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I open the EC2 Instances list. | The bottom border shows: `└──y YAML──ctrl+r Refresh──...──┘`. No `enter` or `d Detail` hint appears because `enter` already means detail (the default, obvious behavior). |
| C.1.2 | I open the VPC list. | The bottom border shows: `└──y YAML──ctrl+r Refresh──...──┘`. Same as EC2 -- no enter-child override. |
| C.1.3 | I open the EKS Clusters list. | The bottom border shows: `└──y YAML──ctrl+r Refresh──...──┘`. |

**AWS comparison:**

```
aws ec2 describe-instances
aws ec2 describe-vpcs
aws eks list-clusters
```

Expected hints visible: `y YAML`, `ctrl+r Refresh`

### C.2 Resource List With Enter-Child Override

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | I open the S3 Buckets list. | The bottom border shows: `└──enter Objects──d Detail──y YAML──ctrl+r Refresh──...──┘`. Because `enter` navigates to S3 Objects (a child view), `d Detail` is shown as the alternative path to the detail view. |
| C.2.2 | I open the ELB list. | The bottom border shows: `└──enter Listeners──d Detail──y YAML──ctrl+r Refresh──...──┘`. |
| C.2.3 | I open the RDS Instances list. | The bottom border shows: `└──enter Events──d Detail──y YAML──ctrl+r Refresh──...──┘`. |
| C.2.4 | I open the SNS Topics list. | The bottom border shows `enter Subscriptions` and `d Detail` among the hints, reflecting the enter-child override for SNS subscriptions. |

**AWS comparison:**

```
aws s3api list-buckets
aws elbv2 describe-load-balancers
aws rds describe-db-instances
aws sns list-topics
```

Expected hints visible: `enter {ChildName}`, `d Detail`, `y YAML`, `ctrl+r Refresh`

### C.3 Resource List With Additional Child Keys

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | I open the ECS Services list. | The bottom border shows: `└──enter Tasks──d Detail──y YAML──e Events──L Logs──ctrl+r Refresh──...──┘`. The `e` and `L` child keys appear after `y YAML` but before `ctrl+r Refresh`. |
| C.3.2 | I open the CloudFormation Stacks list. | The bottom border shows: `└──enter Events──d Detail──y YAML──R Resources──ctrl+r Refresh──...──┘`. The `R` child key for stack resources appears. |
| C.3.3 | I open the CodeBuild Projects list. | The bottom border includes `enter Builds` plus any additional child keys (e.g., `L Logs`) defined for this resource type. |

**AWS comparison:**

```
aws ecs list-services --cluster <cluster>
aws cloudformation list-stacks
aws codebuild list-projects
```

Expected hints visible: `enter {ChildName}`, `d Detail`, `y YAML`, `{childKey} {childLabel}`, `ctrl+r Refresh`

### C.4 Resource List With Reveal (Secrets)

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | I open the Secrets Manager list. | The bottom border shows: `└──x Reveal──y YAML──ctrl+r Refresh──...──┘`. The `x Reveal` hint appears because this resource type supports secret reveal. No `enter` or `d Detail` disambiguation is needed (enter=detail is the default). |
| C.4.2 | I open the SSM Parameters list. | The bottom border shows: `└──x Reveal──y YAML──ctrl+r Refresh──...──┘`. Same pattern as Secrets Manager. |

**AWS comparison:**

```
aws secretsmanager list-secrets
aws ssm describe-parameters
```

Expected hints visible: `x Reveal`, `y YAML`, `ctrl+r Refresh`

### C.5 Child/Related List (escPops=true)

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | I navigate into S3 Objects (press `enter` on an S3 bucket). | The bottom border shows: `└──esc Back──y YAML──ctrl+r Refresh──...──┘`. The `esc Back` hint appears first because this is a child list where `esc` pops back to the parent. |
| C.5.2 | I navigate into ECS Tasks (press `enter` on an ECS service). | The bottom border starts with `esc Back`, followed by child-appropriate hints. |
| C.5.3 | I navigate into CloudFormation Events. | The bottom border starts with `esc Back`. |
| C.5.4 | I navigate into Log Streams from a Log Group. | The bottom border starts with `esc Back`, followed by remaining applicable hints. |
| C.5.5 | I navigate into R53 Records from a Hosted Zone. | The bottom border starts with `esc Back`. |

**AWS comparison:**

```
aws s3api list-objects-v2 --bucket <bucket>
aws ecs list-tasks --service <service> --cluster <cluster>
aws cloudformation describe-stack-events --stack-name <stack>
```

Expected hints visible: `esc Back`, then view-appropriate hints

### C.6 Resource List With Pagination

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | I open a resource list where results are truncated by pagination. | The bottom border includes `m More` as the rightmost hint, after `ctrl+r Refresh`. |
| C.6.2 | I press `m` to load more results and all results are now loaded. | The `m More` hint disappears from the bottom border. |
| C.6.3 | I open a resource list where all results fit in one page. | The `m More` hint does not appear in the bottom border. |

**AWS comparison:**

```
aws ec2 describe-instances --max-items 50
```

Expected additional hint when paginated: `m More`

---

## D. Detail View Hints

### D.1 Plain Field, No Related Resources

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I press `d` on a VPC that has no related resource definitions. | The bottom border shows: `└──y YAML──ctrl+r Refresh──w Wrap──...──┘`. No `enter` hint appears because the cursor is on a plain (non-navigable) field. No `r Related` hint appears because this type has no related definitions. |
| D.1.2 | I scroll down through the detail fields with `j`. | The hints remain `y YAML`, `ctrl+r Refresh`, `w Wrap` as long as the cursor is on non-navigable fields. |

**AWS comparison:**

```
aws ec2 describe-vpcs --vpc-ids <vpc-id>
```

Expected hints visible: `y YAML`, `ctrl+r Refresh`, `w Wrap`

### D.2 Navigable Field

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | I open the detail view for an EC2 instance and navigate the cursor to the `VpcId` field. | The bottom border shows: `└──enter VPC──y YAML──ctrl+r Refresh──w Wrap──...──┘`. The `enter VPC` hint appears because `VpcId` is a navigable field pointing to the VPC resource type. |
| D.2.2 | I move the cursor from `VpcId` to a plain field like `InstanceType`. | The `enter VPC` hint disappears from the bottom border. The hints revert to the plain-field set. |
| D.2.3 | I navigate the cursor to the `SubnetId` field. | The bottom border shows `enter Subnet` (or the display name for the subnet resource type) as the first hint. |
| D.2.4 | I navigate the cursor to the `ImageId` field. | The bottom border shows `enter AMI` (or the display name for the AMI resource type) as the first hint. |
| D.2.5 | I navigate the cursor to a `SecurityGroups` field entry. | The bottom border shows `enter Security Group` (or appropriate display name) as the first hint. |

**AWS comparison:**

```
aws ec2 describe-instances --instance-ids <id> --query 'Reservations[].Instances[].{VpcId:VpcId,SubnetId:SubnetId}'
```

Expected hints visible (navigable): `enter {TargetType}`, `y YAML`, `ctrl+r Refresh`, `w Wrap`

### D.3 Detail With Related Resources

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | I open the detail view for an EC2 instance (which has related resource definitions). | The bottom border includes `r Related` after `y YAML`. |
| D.3.2 | I press `r` to open the related panel. The left column (fields) remains focused. | The bottom border now includes `tab Cols` in addition to `r Related`, indicating I can switch focus to the right column. |
| D.3.3 | I am on a navigable field with the related panel open. | The bottom border shows: `enter {Target}`, `y YAML`, `r Related`, `tab Cols`, `ctrl+r Refresh`, `w Wrap`. |
| D.3.4 | I am on a plain field with the related panel open. | The bottom border shows: `y YAML`, `r Related`, `tab Cols`, `ctrl+r Refresh`, `w Wrap`. No `enter` hint. |

**AWS comparison:**

```
aws ec2 describe-instances --instance-ids <id>
```

Expected hints visible (has related, panel open): `y YAML`, `r Related`, `tab Cols`, `ctrl+r Refresh`, `w Wrap`

### D.4 Right Column Focused

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | I press `tab` to focus the right column in the detail view. A related type is highlighted. | The bottom border shows: `└──enter {SelectedType}──tab Fields──y YAML──ctrl+r Refresh──...──┘`. The `enter` hint shows the display name of the currently selected related type. `tab Fields` replaces `tab Cols` to indicate switching back to the left column. |
| D.4.2 | I navigate the right column cursor to a different related type (e.g., from "VPC" to "Subnet"). | The `enter` hint updates to show the new target type name (`enter Subnet`). |
| D.4.3 | The right column is focused but no related type is selected (empty related list). | The bottom border shows: `└──tab Fields──y YAML──ctrl+r Refresh──...──┘`. No `enter` hint because there is nothing to navigate to. |
| D.4.4 | I press `tab` again to return focus to the left (fields) column. | The bottom border reverts to the left-column hint set (with `tab Cols` instead of `tab Fields`). |

**AWS comparison:**

```
aws ec2 describe-instances --instance-ids <id>
```

Expected hints visible (right col focused, selection): `enter {SelectedType}`, `tab Fields`, `y YAML`, `ctrl+r Refresh`

---

## E. YAML View Hints

### E.1 Normal State

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | I press `y` on any resource to open the YAML view. | The bottom border shows: `└──w Wrap──c Copy──...──┘`. |
| E.1.2 | I scroll through the YAML content with `j`/`k`. | The bottom border hints remain `w Wrap` and `c Copy` regardless of scroll position. |
| E.1.3 | I press `w` to toggle word wrap in the YAML view. | The bottom border hints remain `w Wrap` and `c Copy`. The hint does not change to indicate wrap state (it is a toggle). |
| E.1.4 | I press `c` to copy the YAML content. | The header shows a "Copied!" flash message. The bottom border hints remain unchanged. |

**AWS comparison:**

```
aws ec2 describe-instances --instance-ids <id> --output yaml
```

Expected hints visible: `w Wrap`, `c Copy`

---

## F. Width Truncation Behavior

### F.1 Hints Dropped Right-to-Left

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | I view the S3 resource list at 120+ columns. | All hints are visible: `enter Objects`, `d Detail`, `y YAML`, `ctrl+r Refresh`. |
| F.1.2 | I resize the terminal to 80 columns while viewing the S3 resource list. | Some rightmost hints may be dropped. The leftmost hints (`enter Objects`, `d Detail`) remain visible because they are most important for disambiguation. The border still starts with `└` and ends with `┘`. |
| F.1.3 | I resize the terminal to 60 columns (minimum supported width). | Only the first 1-2 hints fit. Remaining hints are dropped. The border is well-formed with corners intact. |
| F.1.4 | I view a detail view with many hints at 80 columns. | The priority-ordered hints (escape route, enter disambiguation, YAML) survive truncation; auxiliary hints (wrap, copy) are dropped first. |

### F.2 Extreme Widths

| ID | Story | Expected |
|----|-------|----------|
| F.2.1 | The terminal is exactly 60 columns wide. | The bottom border renders correctly. At least the `└` and `┘` corners are present. If even one hint does not fit, a plain border (dashes only) is shown. |
| F.2.2 | The terminal is very wide (200+ columns). | All hints are visible. The remaining space is filled with dashes to reach `┘`. No extra characters, no double-rendering. |
| F.2.3 | No hints are provided (empty slice). | The border renders as a plain `└──...──┘` line, identical to the pre-feature border. Zero visual regression. |

### F.3 Single Hint Edge Case

| ID | Story | Expected |
|----|-------|----------|
| F.3.1 | Only one hint exists (e.g., main menu: `ctrl+r Refresh`). | The border shows `└──ctrl+r Refresh──...──┘` with dashes filling the rest. |
| F.3.2 | The single hint is too wide for the terminal. | The hint is dropped entirely, producing a plain `└──...──┘` border. No partial hint rendering. |

---

## G. Interaction with Existing Features

### G.1 Filter Active State

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | I activate filter mode (`/`) on the EC2 resource list. | The header right side shows `/search-text` in amber. The bottom border hints remain unchanged -- filter input is in the header, not the border. |
| G.1.2 | I type a filter term that narrows results to zero rows. | The bottom border hints remain visible even when the table content area shows an empty state. |
| G.1.3 | I press `esc` to clear the filter. | The bottom border hints remain unchanged throughout the filter lifecycle. |

### G.2 Command Mode Active

| ID | Story | Expected |
|----|-------|----------|
| G.2.1 | I activate command mode (`:`) on any view. | The header right side shows `:cmd` in amber. The bottom border hints remain unchanged during command input. |

### G.3 Loading State

| ID | Story | Expected |
|----|-------|----------|
| G.3.1 | A resource list is loading (spinner visible). | The bottom border shows hints appropriate for the resource type even while loading. The hints do not disappear during the loading state. |
| G.3.2 | The resource list finishes loading. | The bottom border hints may update if the loaded state reveals new information (e.g., pagination `m More`), but the core hints remain stable. |

### G.4 Error State

| ID | Story | Expected |
|----|-------|----------|
| G.4.1 | An API error occurs while fetching resources. | The header shows the error flash. The bottom border hints remain visible and unchanged -- they reflect available key actions, not data state. |

### G.5 Flash Messages

| ID | Story | Expected |
|----|-------|----------|
| G.5.1 | I press `c` to copy a resource ID, triggering a "Copied!" flash in the header. | The bottom border hints are unaffected by the flash message. The flash appears in the header right side, not the border. |

---

## H. Terminal Resize Behavior

### H.1 Dynamic Resize

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | I resize the terminal from 120 to 80 columns while viewing a resource list. | The bottom border re-renders. Hints that no longer fit are dropped from the right. The border width matches the new terminal width exactly. |
| H.1.2 | I resize the terminal from 80 to 120 columns. | Previously truncated hints reappear as space becomes available. |
| H.1.3 | I resize the terminal below the minimum width (< 60 columns). | The application shows the "Terminal too narrow" error message. No bottom border is rendered. |
| H.1.4 | I resize the terminal below the minimum height (< 7 lines). | The application shows the "Terminal too short" error message. No bottom border is rendered. |
| H.1.5 | I resize from below-minimum back to a valid size. | The bottom border with hints re-renders correctly for the new dimensions. |

---

## I. NO_COLOR Environment Variable

### I.1 Monochrome Mode

| ID | Story | Expected |
|----|-------|----------|
| I.1.1 | I launch a9s with `NO_COLOR=1` set in the environment and open a resource list. | The bottom border hints are still present and readable. Key and description text appear without color styling (no accent blue, no dim). The structural layout (corners, dashes, spacing) is identical to the colored version. |
| I.1.2 | I launch a9s with `NO_COLOR=1` and verify hint key text is distinguishable from description text. | Even without color, the key text should be visually distinct from the description (e.g., bold attribute may still apply if the terminal supports it, or the key-space-description pattern provides sufficient separation). |

---

## J. View Transition Consistency

### J.1 Hints Update on View Change

| ID | Story | Expected |
|----|-------|----------|
| J.1.1 | I am on the main menu (hints: `ctrl+r Refresh`), then press `enter` to open EC2 list. | The bottom border immediately updates to the resource list hints (`y YAML`, `ctrl+r Refresh`). |
| J.1.2 | I press `d` on an EC2 instance to open the detail view. | The bottom border immediately updates to the detail view hints (`y YAML`, `ctrl+r Refresh`, `w Wrap`). |
| J.1.3 | I press `y` to open the YAML view from the detail. | The bottom border immediately updates to `w Wrap`, `c Copy`. |
| J.1.4 | I press `esc` from the YAML view to return to detail. | The bottom border reverts to the detail view hints. |
| J.1.5 | I press `esc` from the detail view to return to the resource list. | The bottom border reverts to the resource list hints. |
| J.1.6 | I press `esc` from the resource list to return to the main menu. | The bottom border reverts to `ctrl+r Refresh`. |
| J.1.7 | I press `?` to open the Help view from any hintable view. | The bottom border switches to a plain border (no hints). |
| J.1.8 | I press any key to close the Help view. | The bottom border reverts to the hints for the view that was active before Help was opened. |

### J.2 Hints Update on Detail Cursor Movement

| ID | Story | Expected |
|----|-------|----------|
| J.2.1 | I move the detail cursor from a plain field to a navigable field (`VpcId`). | The bottom border updates in real-time to add `enter VPC` as the first hint. |
| J.2.2 | I move the detail cursor from one navigable field (`VpcId`) to another (`SubnetId`). | The `enter` hint updates from `enter VPC` to `enter Subnet` in real-time. |
| J.2.3 | I move the detail cursor from a navigable field back to a plain field. | The `enter` hint disappears from the bottom border in real-time. |

### J.3 Hints Update on Focus Toggle

| ID | Story | Expected |
|----|-------|----------|
| J.3.1 | I press `tab` to move focus from the left column to the right column in the detail view. | The bottom border updates to the right-column hint set (`enter {Type}`, `tab Fields`, `y YAML`, `ctrl+r Refresh`). |
| J.3.2 | I press `tab` to move focus back to the left column. | The bottom border updates to the left-column hint set (with `tab Cols`). |
| J.3.3 | I press `r` to toggle the related panel visibility. | The hints update to reflect whether `tab Cols` should appear (panel visible) or not (panel hidden). |

---

## K. Backward Compatibility

### K.1 Views Without Hints

| ID | Story | Expected |
|----|-------|----------|
| K.1.1 | I open the Help view. | The bottom border is a plain `└───...───┘` line, visually identical to what it was before this feature. |
| K.1.2 | I open the Profile Selector. | Plain bottom border, no regression. |
| K.1.3 | I open the Region Selector. | Plain bottom border, no regression. |
| K.1.4 | I open the Reveal view. | Plain bottom border, no regression. The view's own inline hint (`esc` to close) is unaffected. |
| K.1.5 | I open the Identity view. | Plain bottom border, no regression. |

### K.2 Frame Structure

| ID | Story | Expected |
|----|-------|----------|
| K.2.1 | I verify the top border of any view. | The top border is unchanged: `┌──── title ────┐` with centered title. This feature only modifies the bottom border. |
| K.2.2 | I verify the side borders of any view. | Side borders (`│`) are unchanged. Content rows are unaffected. |
| K.2.3 | I verify the total line budget. | Header: 1 line. Frame top border: 1 line. Content: `termHeight - 3` lines. Frame bottom border: 1 line. No extra lines consumed by hints. |

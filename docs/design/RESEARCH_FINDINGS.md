# Research Findings

## 1. TUI Best Practices Summary

### 1.1 Layout Architecture

The most successful TUI applications (k9s, lazygit, htop, btop, ranger) share a three-zone layout:

```
+--[ HEADER / CONTEXT BAR ]------------------------------------------+
|  Persistent context: what am I looking at? Profile, cluster, etc.  |
+--[ CONTENT AREA ]--------------------------------------------------+
|                                                                     |
|  Primary interactive area. Tables, trees, previews, or panels.     |
|  This is where 80%+ of the terminal real estate goes.              |
|                                                                     |
+--[ STATUS / COMMAND BAR ]------------------------------------------+
|  Mode indicator, key hints, command input, error messages           |
+--------------------------------------------------------------------+
```

**Key principle**: The header and status bar are fixed anchors. Users build spatial memory around them. The content area is the only zone that changes between views.

**Evidence**: k9s uses exactly this pattern. So does lazygit (with additional panel splits within the content area). htop/btop dedicate the full screen to content but still anchor a header showing system info.

### 1.2 Navigation Paradigms

Three navigation models dominate successful TUI applications:

| Pattern | Examples | Best For |
|---------|----------|----------|
| **Hierarchical drill-down** | k9s (namespace > resource > pod > logs), ranger (dir > dir > file) | Tree-structured data like AWS resources |
| **Panel switching** | lazygit (files / branches / commits / stash), htop (process tree vs flat) | Multiple views of the same dataset |
| **Command palette** | k9s (`:` command mode), Sublime/VS Code | Power users who know what they want |

**a9s should combine hierarchical drill-down with command palette**, matching k9s. The current implementation already does this but needs refinement.

### 1.3 Keybinding Conventions

Established conventions across successful TUIs:

| Key | Convention | Used By |
|-----|-----------|---------|
| `j/k` | Up/Down navigation | k9s, lazygit, vim, tig |
| `g/G` | Top/Bottom | k9s, lazygit, vim |
| `/` | Search/Filter | k9s, lazygit, vim, less |
| `:` | Command mode | k9s, vim |
| `?` | Help overlay | k9s, lazygit, less |
| `Esc` | Back/Cancel | Universal |
| `Enter` | Select/Confirm | Universal |
| `d` | Describe/Delete (context-dependent) | k9s, lazygit |
| `y` | YAML view | k9s |
| `l` | Logs (in k9s context) | k9s |
| `Ctrl-R` | Refresh | k9s, many CLI tools |
| `q` | Quit (from root only) | k9s, lazygit, htop |
| `Tab` | Switch panels/sections | lazygit, many TUIs |

**Critical finding**: a9s correctly uses `h/l` for horizontal scroll, but this conflicts with `h` as "left" in vim-mode navigation. k9s avoids this by not having horizontal scroll on tables (it auto-fits columns). This is a design tension point that needs resolution.

### 1.4 Color and Theming

Best practices from btop, k9s, lazygit:

- **Status coloring**: Green = healthy/running, Red = error/stopped, Yellow = pending/warning, Cyan = informational
- **Dimmed text**: Use faint/dim for secondary information, separators, disabled items
- **Bold**: Headers, active selections, important labels
- **Reverse video**: Current cursor/selection row (universal convention)
- **Border colors**: Cyan or blue for focused panels, gray for unfocused
- **Minimal palette**: 6-8 colors maximum. More causes cognitive overload.
- **NO_COLOR support**: Respect the `NO_COLOR` environment variable (a9s already does this correctly)

### 1.5 Information Density

Power user TUIs optimize for information density:

- **k9s**: Shows 7-8 columns for pods, uses the full terminal width
- **htop**: Every pixel is used; no blank space
- **lazygit**: Splits screen into panels, each showing different data simultaneously

**Anti-patterns to avoid**:
- Large padding/margins in the content area
- Single-column layouts when tabular data is available
- Fixed-width content that does not expand to fill the terminal

### 1.6 Responsive Design

Terminal sizes vary from 80x24 (minimum reasonable) to 300x80+ (ultrawide monitors):

- **Minimum viable width**: 80 columns. Below this, truncate gracefully.
- **Column prioritization**: Hide low-priority columns on narrow terminals rather than squeezing all columns.
- **k9s approach**: Auto-sizes columns proportionally; drops columns that do not fit.
- **Height management**: Use viewport/pagination; show visible row count vs total.

---

## 2. k9s Design Pattern Analysis

### 2.1 Information Architecture

k9s organizes Kubernetes resources in a flat hierarchy:

```
Context (cluster) > Namespace > Resource Type > Individual Resource
```

The user selects a context (cluster), optionally filters by namespace, then navigates resource types via `:` commands. Each resource type has:
- **List view**: Table with resource-type-specific columns
- **Describe view**: Full kubectl describe output (scrollable)
- **YAML view**: Raw YAML manifest
- **Logs view**: Streaming logs (for pods)
- **Shell view**: Interactive exec into containers

**Mapping to AWS**: This maps directly:
```
Profile (account) > Region > Service > Individual Resource
```

### 2.2 Key Design Decisions in k9s

1. **No main menu**: k9s starts directly on a resource list (pods by default). Users switch resource types via `:` commands. This is faster than a menu for experienced users.

2. **Breadcrumb bar**: Shows `context > namespace > resource-type` at all times, so the user never loses orientation.

3. **Resource count in breadcrumbs**: Shows count of displayed items, e.g., "pods (47)" -- a9s has adopted this pattern.

4. **Instant filtering**: `/` opens a filter bar at the bottom. Results update as you type. Match count shown. This is the expected standard.

5. **Column auto-sizing**: k9s adapts column widths to terminal width. No horizontal scrolling needed for most cases.

6. **Status indicators**: Colored bullets/text for resource status (Running = green, CrashLoopBackOff = red, etc.)

7. **Hotkey bar**: Bottom bar shows available actions for the current context, like `<d>describe <y>yaml <l>logs <ctrl-d>delete`.

8. **Command autocomplete**: `:` mode shows suggestions as you type.

9. **Shortcut overlay**: `?` shows a categorized list of all keybindings.

10. **XRay mode**: Deep view showing relationships between resources. No direct a9s equivalent yet, but could map to "related resources" (e.g., EC2 instance > its security groups, VPC, etc.).

### 2.3 What Makes k9s Successful

- **Zero learning curve for Kubernetes users**: Mental model matches `kubectl` exactly.
- **Eliminates repetitive typing**: Common `kubectl` sequences become single keystrokes.
- **Real-time updates**: Resource list auto-refreshes. Status changes appear instantly.
- **Progressive disclosure**: List > Describe > YAML > Logs, each deeper level of detail.
- **Muscle memory formation**: Consistent shortcuts across all resource types.
- **Context awareness**: Actions change based on resource type (logs only for pods, not for services).

---

## 3. AWS User Workflow Analysis by Persona

### 3.1 DevOps / SRE Engineer

**Primary tasks**:
- Monitor running infrastructure: EC2 status, RDS health, EKS cluster state
- Troubleshoot incidents: Check instance states, review security groups, examine endpoints
- Manage secrets: Rotate, view, update secrets in Secrets Manager
- S3 operations: Browse buckets, check object metadata
- Cross-account operations: Switch between staging/production profiles frequently

**Pain points with current tools**:
- AWS Console: 15+ seconds to load a page, multiple clicks to reach a resource, losing context when switching services
- AWS CLI: Verbose commands (`aws ec2 describe-instances --filters "Name=tag:Name,Values=my-server" --query 'Reservations[].Instances[].{ID:InstanceId,State:State.Name}' --output table`), difficult to browse
- Both: No unified view across services, no quick cross-service navigation

**What they need from a9s**:
- Fast profile/region switching (they manage 3-10 accounts)
- Instant filtering across hundreds of resources
- Quick copy of resource IDs, ARNs, endpoints for pasting into CLI commands or Terraform
- View resource details without losing list position
- Keyboard-driven everything

### 3.2 Senior Developer

**Primary tasks**:
- Look up endpoints for services they connect to (RDS endpoints, Redis endpoints, S3 bucket names)
- Check EC2 instance types and states for cost awareness
- Retrieve secrets for local development configuration
- Verify deployment state (EKS cluster version, EC2 instance counts)

**Pain points**:
- Having to context-switch to browser for simple lookups
- Forgetting which profile/region a resource is in
- AWS CLI output is hard to parse visually

**What they need from a9s**:
- Quick lookup: type `:rds`, find the database, copy the endpoint
- Secret reveal: find the secret, press `x`, see the value or copy it
- Minimal learning curve: vim-like navigation they already know

### 3.3 CTO / Engineering Manager

**Primary tasks**:
- Overview of infrastructure: how many instances, what types, what regions
- Cost visibility: instance types, storage sizes, multi-AZ configurations
- Security posture: are secrets rotated? Are endpoints public?
- Compliance: what regions are we running in?

**What they need from a9s**:
- Summary statistics (not yet implemented)
- Export capability for reports
- Quick overview without deep diving
- Clear visual indicators of health/problems

---

## 4. Competitive Landscape

### 4.1 Existing AWS TUI/CLI Tools

| Tool | Type | Strengths | Weaknesses |
|------|------|-----------|------------|
| **AWS CLI** | CLI | Complete coverage, scriptable | Verbose, no visual browsing |
| **AWS Console** | Web | Full feature set, visual | Slow, requires browser, click-heavy |
| **aws-shell** | Interactive CLI | Autocomplete, inline docs | Still text-based output, no TUI |
| **former2** | Web | Generates IaC from existing resources | Narrow use case |
| **steampipe** | SQL-based | Query AWS with SQL | Learning curve, not real-time browsing |
| **iamlive** | CLI | Records IAM usage | Single-purpose |
| **awsls** | CLI | Lists resources across types | Text output, not interactive |
| **granted** | CLI | Fast profile switching | Profile switching only |
| **leapp** | Desktop | Visual credential management | Not a TUI, limited scope |

### 4.2 Gap Analysis

No existing tool provides what a9s targets: **a k9s-like interactive TUI for browsing and managing AWS resources with keyboard-driven navigation, real-time updates, and progressive detail disclosure.**

The closest conceptual match is k9s for Kubernetes. The opportunity is to be "the k9s for AWS."

### 4.3 Differentiation Opportunities

1. **Cross-service navigation**: Jump from EC2 instance to its security group to its VPC -- no tool does this well in a TUI.
2. **Multi-profile management**: Built-in profile switcher with SSO support -- faster than `granted` for quick lookups.
3. **Resource relationships**: Show related resources (EC2 > SG > VPC, RDS > subnet group > VPC).
4. **Cost annotations**: Show estimated monthly cost per resource based on type/size.
5. **Action shortcuts**: SSH to EC2, connect to RDS, open console URL for a specific resource.

---

## 5. Current a9s Architecture Assessment

### 5.1 Technology Stack

- **Go 1.22+**: Solid choice for TUI. Fast compilation, good concurrency model for async data loading.
- **Bubble Tea v2**: The standard Go TUI framework. Well-maintained, good ecosystem (lipgloss, bubbles).
- **lipgloss v2**: For styling. Good color support, composable styles.
- **evertras/bubble-table**: Referenced in `resourcelist.go` but actually superseded by custom table rendering in `app.go`. The custom renderer in `renderResourceList()` builds tables manually with `padOrTruncate` and horizontal scrolling. This is more flexible but also more bug-prone.

### 5.2 Architecture Pattern

The app uses a **monolithic model** pattern: `AppState` is a single large struct (~30 fields) that holds all application state. The `Update()` method dispatches to view-specific handlers. The `View()` method renders by checking `CurrentView`.

**Comparison with k9s**: k9s uses a component-based architecture where each view is an independent component with its own model/update/view cycle. This scales better as feature count grows.

### 5.3 Supported AWS Services (v0.4.5)

1. S3 (buckets + object browsing)
2. EC2 (instances)
3. RDS (instances)
4. ElastiCache Redis (clusters)
5. DocumentDB (clusters)
6. EKS (clusters)
7. Secrets Manager (secrets + reveal)

### 5.4 View Types

1. **MainMenuView**: Static list of resource types
2. **ResourceListView**: Tabular list with custom rendering
3. **DetailView**: Key-value pairs with config-driven field extraction
4. **JSONView**: Raw YAML/JSON scrollable text
5. **RevealView**: Secret value display
6. **ProfileSelectView**: AWS profile picker
7. **RegionSelectView**: AWS region picker

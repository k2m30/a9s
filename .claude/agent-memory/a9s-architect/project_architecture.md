---
name: a9s Architecture Snapshot
description: Core architecture patterns, file layout, and resource type system for the a9s AWS TUI manager as of v0.4.5
type: project
---

a9s is a Go TUI AWS resource manager using Bubble Tea v2 (charm.land/bubbletea/v2).

**Why:** This captures the architectural state after features 001 (MVP) and 002 (configurable views) landed, providing context for all future architectural discussions.

**How to apply:** Reference this when reviewing any new feature work, resource type additions, or refactoring proposals.

## Key Architectural Facts (as of 003-fix-ui-bugs branch)

### Resource Type System (Dual-Path)
- `internal/resource/types.go`: Hardcoded `ResourceTypeDef` with old `Column` structs (key-based: "instance_id", "name", etc.) for 7 resource types + S3 objects
- `internal/config/defaults.go`: Parallel config-driven `ViewDef` definitions using dot-notation paths into AWS SDK structs ("InstanceId", "State.Name", etc.)
- Runtime renders via config path when `RawStruct != nil`, falls back to `Fields` map otherwise
- Both systems must stay in sync -- this is a maintenance burden

### God Object: app.go (1895 lines)
- Contains ALL view rendering (custom table renderer, not bubble-table), ALL key handling, navigation, fetching, sorting, filtering
- `renderResourceList()` is ~200 lines of custom table rendering (header, separator, rows, hscroll, viewport)
- `fetchResources()` is a giant switch statement mapping resource type strings to AWS API calls
- No separation between controller/model/view concerns

### Resource Registration Pattern
- Resources are defined in `resource/types.go` as a static slice `resourceTypes`
- Adding a resource requires changes in: types.go, config/defaults.go, views.yaml, aws/{service}.go, aws/interfaces.go, aws/client.go (ServiceClients struct), app.go (fetchResources switch, knownCommands)
- Minimum 7 files must be touched to add a new resource type

### Navigation Stack
- `internal/navigation/history.go`: Simple back/forward stack storing `ViewState` (ViewType, ResourceType, CursorPos, Filter, S3Bucket, S3Prefix)
- Preserves selected index on back navigation
- S3 context changes trigger re-fetch on back navigation

### bubble-table (views/resourcelist.go)
- Full bubble-table integration exists but appears UNUSED in production rendering
- app.go uses custom rendering instead, presumably for more control over horizontal scrolling and styling

### Reflection Engine (internal/fieldpath/)
- `extract.go`: Navigates structs via dot-notation paths, case-insensitive field matching
- `format.go`: Auto-formats time.Time, bool, int, float, string
- `enumerate.go`: Walks struct types to list all available paths (used by cmd/refgen)
- `ToSafeValue()`: Recursively converts AWS SDK structs to map[string]interface{} safe for YAML marshaling

### AWS Service Layer
- Per-service files in internal/aws/ (ec2.go, s3.go, rds.go, redis.go, docdb.go, eks.go, secrets.go)
- Each fetcher returns []resource.Resource with both Fields map AND RawStruct
- Narrow interfaces in interfaces.go enable testing
- ServiceClients struct in client.go holds all typed AWS clients

### Test Suite
- ~21K lines of unit tests in tests/unit/
- Many large QA test files (qa_state_consistency_test.go: 2440 lines)
- Tests reference both legacy Fields-based and config-driven rendering paths

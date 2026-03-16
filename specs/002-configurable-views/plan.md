# Implementation Plan: Configurable Views

**Branch**: `002-configurable-views` | **Date**: 2026-03-15 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-configurable-views/spec.md`

## Summary

Replace hardcoded view definitions with YAML-driven configuration. List view columns and detail view fields are defined in `views.yaml` using dot-notation paths into AWS Go SDK structs. Values are extracted at runtime via Go reflection with auto-formatting by type. A development-time reference generator enumerates all available paths via `reflect.TypeOf()` on SDK response structs.

## Technical Context

**Language/Version**: Go 1.25+ (go.mod)
**Primary Dependencies**: Bubble Tea v2, bubble-table, AWS SDK Go v2, `gopkg.in/yaml.v3` (new)
**Storage**: YAML config files (filesystem)
**Testing**: Standard Go `testing` package, 676 unit tests, 26 integration tests
**Target Platform**: macOS/Linux terminal
**Project Type**: CLI/TUI application
**Performance Goals**: Config loading + reflection overhead imperceptible at startup
**Constraints**: No per-resource-type display logic; all field extraction via reflection
**Scale/Scope**: 8 view types (7 resource types + S3 objects), ~50 fields per resource type

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. TDD (NON-NEGOTIABLE) | PASS | All new packages (config, fieldpath) developed test-first. Existing tests updated for new code paths. |
| II. Code Quality | PASS | New packages follow single responsibility. Reflection logic isolated in dedicated package. No magic values — paths and defaults from config/constants. |
| III. Testing Standards | PASS | Unit tests for config loading, path resolution, reflection extraction, auto-formatting. Integration tests for full config→render pipeline. |
| IV. UX Consistency | PASS | Existing view rendering preserved. Detail view enhanced with YAML subtree rendering using consistent styling. Error messages for invalid config are clear and actionable. |
| V. Performance | PASS | Reflection is one-time per resource fetch, not per-frame. Config loaded once at startup. Benchmark tests added for reflection extraction. |

## Project Structure

### Documentation (this feature)

```text
specs/002-configurable-views/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── config/
│   ├── config.go           # YAML config loading, lookup chain, parsing
│   └── defaults.go         # Built-in default view definitions
├── reflect/
│   ├── extract.go          # Dot-path resolution via reflect.Value
│   ├── enumerate.go        # Path enumeration via reflect.Type (for reference generator)
│   └── format.go           # Auto-formatting by Go type (time.Time, bool, etc.)
├── resource/
│   ├── types.go            # Updated: Column gets Path field, loads from config
│   └── resource.go         # Updated: Resource carries raw SDK struct reference
├── aws/
│   ├── ec2.go              # Updated: stores raw SDK struct in Resource
│   ├── s3.go               # Updated: stores raw SDK struct in Resource
│   ├── rds.go              # Updated: stores raw SDK struct in Resource
│   ├── redis.go            # Updated: stores raw SDK struct in Resource
│   ├── docdb.go            # Updated: stores raw SDK struct in Resource
│   ├── eks.go              # Updated: stores raw SDK struct in Resource
│   └── secrets.go          # Updated: stores raw SDK struct in Resource
├── views/
│   ├── resourcelist.go     # Updated: builds columns from config, extracts via reflection
│   └── detail.go           # Updated: renders ordered fields with YAML subtree support
└── app/
    └── app.go              # Updated: loads config at startup

cmd/
└── refgen/
    └── main.go             # Development-time reference generator binary

views.yaml                  # Default config shipped with project
views_reference.yaml        # Generated reference artifact

tests/
├── unit/
│   ├── config_test.go      # Config loading, lookup chain, parsing, fallback
│   ├── fieldpath_test.go     # Path resolution, enumeration, formatting
│   ├── views_config_test.go # View rendering with config-driven columns
│   └── detail_config_test.go # Detail view with YAML subtree rendering
└── testdata/
    ├── views_valid.yaml    # Test fixture: valid config
    ├── views_invalid.yaml  # Test fixture: invalid YAML
    └── views_partial.yaml  # Test fixture: partial config (some views only)
```

**Structure Decision**: Two new internal packages (`config`, `fieldpath`) keep concerns separated. The reference generator is a standalone `cmd/refgen` binary, not part of the runtime app. Existing packages are modified minimally — AWS fetchers add raw struct storage, views read from config instead of hardcoded types.

## Architecture

### Data Flow

```
Startup:
  config.Load() → lookup chain → parse YAML → ViewConfig per resource type
                                            ↘ fallback to defaults.go if missing

Resource Fetch:
  aws.FetchEC2Instances() → []Resource (now carries raw SDK struct via interface{})

List View Render:
  For each Resource:
    For each configured column:
      fieldpath.ExtractValue(resource.RawStruct, column.Path) → string
      fieldpath.AutoFormat(value, goType) → formatted string
    → table row

Detail View Render:
  For each configured path:
    fieldpath.ExtractSubtree(resource.RawStruct, path) → scalar or nested value
    If scalar: render as "key : value"
    If nested: render as indented YAML
```

### Key Design Decisions

1. **Resource struct gets `RawStruct interface{}`** — carries the original AWS SDK typed struct (e.g., `ec2types.Instance`) alongside existing Fields/DetailData maps. Existing code continues to work; new config-driven path uses RawStruct.

2. **Reflection package is pure** — `internal/fieldpath` has no AWS dependencies. It works on any Go struct. Testable with simple test structs.

3. **Config package handles all I/O** — loading, parsing, validation, lookup chain, fallback. Returns typed `ViewConfig` structs.

4. **Gradual migration** — Built-in defaults in `defaults.go` match current hardcoded values exactly. Zero behavior change when no `views.yaml` exists. Each resource type can be migrated independently.

5. **Reference generator is `cmd/refgen`** — standalone binary that imports SDK types and `internal/fieldpath/enumerate.go`. Run during development, output committed to repo.

### Reflection Path Resolution

```
Path: "state.name"
Struct: ec2types.Instance

1. Split path by "." → ["state", "name"]
2. reflect.ValueOf(instance)
3. Find field with json tag "state" → Instance.State (*InstanceState)
4. Dereference pointer
5. Find field with json tag "name" → InstanceState.Name (InstanceStateName)
6. Return string representation
```

For arrays in detail view:
```
Path: "securityGroups"
1. Find field with json tag "securityGroups" → []GroupIdentifier
2. Detect slice type → marshal to YAML for display
```

### Auto-Formatting Rules

| Go Type | Format | Example |
|---------|--------|---------|
| `string`, `*string` | As-is (deref pointer) | `"i-0abc123"` |
| `time.Time`, `*time.Time` | `2006-01-02 15:04:05` | `"2024-12-10 16:20:39"` |
| `bool`, `*bool` | `Yes` / `No` | `"Yes"` |
| `int32`, `*int32`, `int64`, etc. | `fmt.Sprintf("%d", v)` | `"8080"` |
| `float32`, `float64` | `fmt.Sprintf("%g", v)` | `"3.14"` |
| Enum types (e.g., `InstanceStateName`) | `.String()` or underlying string | `"running"` |
| Slice, Map, Struct | YAML marshal (detail only) | indented subtree |

## Complexity Tracking

No constitution violations. No complexity justifications needed.

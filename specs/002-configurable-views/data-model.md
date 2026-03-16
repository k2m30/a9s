# Data Model: Configurable Views

## Entities

### ViewsConfig (YAML root)

Top-level YAML structure parsed from `views.yaml`.

```
ViewsConfig
├── Views: map[string]ViewDef   # key = resource short name (s3, ec2, rds, etc.)
```

### ViewDef

Configuration for a single resource type's views.

```
ViewDef
├── List: map[string]ListColumnDef   # key = display name, order-preserving (YAML map)
└── Detail: []string                  # ordered list of dot-notation paths
```

### ListColumnDef

A single column in a list view table.

```
ListColumnDef
├── Path: string    # dot-notation path into AWS SDK struct (e.g., "state.name")
└── Width: int      # column width in characters (0 = flexible)
```

### Resource (updated)

Existing `internal/resource.Resource` with new field.

```
Resource (existing + new)
├── ID: string                     # (existing)
├── Name: string                   # (existing)
├── Status: string                 # (existing)
├── Fields: map[string]string      # (existing, kept for backward compat during migration)
├── RawJSON: string                # (existing)
├── DetailData: map[string]string  # (existing, kept for backward compat during migration)
└── RawStruct: interface{}         # NEW: original AWS SDK typed struct for reflection
```

## Relationships

```
ViewsConfig 1──* ViewDef         (one config contains many view definitions)
ViewDef     1──* ListColumnDef   (one view has many list columns)
ViewDef     1──* string          (one view has many detail paths)
Resource    1──1 RawStruct       (each resource carries its SDK struct)
```

## Config Lookup Chain

```
Priority 1: ./views.yaml                    (current working directory)
Priority 2: $A9S_CONFIG_FOLDER/views.yaml   (environment variable)
Priority 3: ~/.a9s/views.yaml               (user home directory)
Fallback:   internal/config/defaults.go      (compiled-in defaults)
```

First file found wins. No merging across locations.

## Auto-Format Type Mapping

```
Go Type              → Display Format
─────────────────────────────────────
string, *string      → as-is (deref pointer)
time.Time, *time.Time → "2006-01-02 15:04:05"
bool, *bool          → "Yes" / "No"
int*, uint*, *int*   → decimal string
float32, float64     → %g format
Named string types   → underlying string value
[]T, map, struct     → YAML marshal (detail view only, empty in list view)
nil pointer          → "" (empty string)
```

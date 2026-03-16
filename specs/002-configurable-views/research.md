# Research: Configurable Views

## R1: Go Reflection for Dot-Path Extraction on AWS SDK Structs

**Decision**: Use `reflect.Value` / `reflect.Type` to navigate AWS SDK v2 struct fields by their JSON tags.

**Rationale**: AWS SDK Go v2 structs consistently use `json:"fieldName"` tags that match the API JSON field names. Reflection allows generic field extraction without per-type code. The `reflect` package is part of Go stdlib — no external dependency.

**Alternatives considered**:
- **JSON marshal + jq-style parsing**: Marshal struct to JSON, then use a JSON path library. Higher overhead (serialize → parse → query), adds dependency. Rejected.
- **Code generation**: Generate accessor functions per SDK type at build time. Fast at runtime but complex build process, must regenerate on SDK updates. Rejected.
- **Manual field maps**: Current approach — per-resource Go code mapping SDK fields to flat string maps. Works but defeats the purpose of configurable views. Rejected.

**Key findings**:
- AWS SDK v2 uses pointer types (`*string`, `*int32`, `*time.Time`) extensively — reflection must handle pointer dereferencing.
- Enum types (e.g., `ec2types.InstanceStateName`) are named string types — `.String()` or `reflect.Value.String()` works.
- Slice fields need detection and YAML marshaling for detail view.
- JSON tags can be parsed with `strings.Split(tag.Get("json"), ",")[0]` to get the field name.

## R2: YAML Config Loading with Lookup Chain

**Decision**: Use `gopkg.in/yaml.v3` for YAML parsing. Implement a simple file-exists lookup chain.

**Rationale**: `yaml.v3` is the standard Go YAML library, well-maintained, and handles the config structure naturally. The lookup chain is just `os.Stat` + `os.ReadFile` on 3 paths in order.

**Alternatives considered**:
- **Viper**: Full config framework with env var merging, multiple formats, etc. Overkill for a single YAML file with a simple priority chain. Rejected.
- **Built-in encoding/json**: JSON lacks comments and is less user-friendly for hand-edited config. Rejected.
- **TOML**: Less common in Go ecosystem, no clear advantage over YAML for this use case. Rejected.

## R3: YAML Subtree Rendering for Detail View

**Decision**: Use `gopkg.in/yaml.v3` to marshal nested objects/arrays from SDK structs into indented YAML text for the detail view.

**Rationale**: The detail view already renders key:value pairs. For nested data, marshaling to YAML produces human-readable indented output that fits naturally. The same library used for config parsing handles marshaling.

**Key findings**:
- `yaml.Marshal()` on a `reflect.Value.Interface()` produces clean YAML output.
- Need to strip the leading YAML document separator (`---`) if present.
- Indentation can be controlled or post-processed to align with the detail view's existing format.

## R4: Reference Generator via `reflect.TypeOf`

**Decision**: Walk AWS SDK struct types recursively using `reflect.Type` to enumerate all fields and their dot-notation paths.

**Rationale**: `reflect.TypeOf()` works on zero-value types — no instantiation or API calls needed. Combined with JSON tag parsing, it produces the exact paths users need.

**Key findings**:
- Recursive walk handles nested structs: `reflect.Type.Field(i).Type.Kind() == reflect.Struct`.
- Pointer types: `type.Elem()` to unwrap.
- Slice types: `type.Elem()` to get element type, append `[]` to path.
- Stop recursion at primitive types (`string`, `int`, `bool`, `time.Time`).
- AWS SDK types to enumerate: `ec2types.Instance`, `s3types.Bucket`, `s3types.Object`, `rdstypes.DBInstance`, `elasticachetypes.CacheCluster`, `docdbtypes.DBCluster`, `ekstypes.Cluster`, `smtypes.SecretListEntry`.

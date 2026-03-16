# Quickstart: Configurable Views

## For End Users

### 1. Customize your views

Create a `views.yaml` in your working directory (or `~/.a9s/views.yaml` for global config):

```yaml
views:
  ec2:
    list:
      Instance ID:
        path: instanceId
        width: 20
      Name:
        path: tags[0].value
        width: 30
      State:
        path: state.name
        width: 12
    detail:
      - instanceId
      - state
      - instanceType
      - placement
      - securityGroups
      - tags
```

### 2. Discover available fields

Consult `views_reference.yaml` (shipped with the project) to see all available paths per resource type.

### 3. Run a9s

Launch `a9s` as usual. Your configured columns and detail fields take effect automatically.

### Config file lookup order

1. `./views.yaml` (current directory — highest priority)
2. `$A9S_CONFIG_FOLDER/views.yaml` (environment variable)
3. `~/.a9s/views.yaml` (home directory — lowest priority)

If no config file found, built-in defaults are used.

## For Developers

### Run the reference generator

```bash
go run ./cmd/refgen > views_reference.yaml
```

This reflects on AWS SDK Go v2 structs and outputs all available field paths. No AWS credentials needed.

### Run tests

```bash
make test
```

### Key packages

- `internal/config` — YAML loading, lookup chain, defaults
- `internal/fieldpath` — dot-path extraction, type enumeration, auto-formatting
- `cmd/refgen` — reference generator binary

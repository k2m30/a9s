# Contract: views.yaml Schema

## Format

```yaml
views:
  <resource_short_name>:    # s3, ec2, rds, redis, docdb, eks, secrets, s3_objects
    list:                    # optional — list view columns
      <Display Name>:       # column header text (free-form string)
        path: <dot.path>    # dot-notation into AWS SDK struct (required)
        width: <int>        # column width in chars (required, 0 = flexible)
    detail:                  # optional — detail view fields
      - <dot.path>          # ordered list of paths to display
      - <dot.path>
```

## Path Syntax

- Dot-separated field names matching AWS SDK struct JSON tags
- Examples: `instanceId`, `state.name`, `placement.availabilityZone`
- Array notation: `securityGroups[].groupId` (in reference file for discovery; detail view uses parent path like `securityGroups` to render full subtree)

## Validation Rules

- `views` key must be present at root
- Each resource key must be a known short name or is silently ignored
- `list` columns: `path` and `width` are required per column
- `detail` entries: each must be a non-empty string
- Invalid YAML → error message + fallback to defaults
- Missing resource → fallback to defaults for that resource
- Missing `list` or `detail` subsection → fallback to defaults for that subsection

## Resource Short Names

| Short Name   | AWS Service        |
|--------------|--------------------|
| `s3`         | S3 Buckets         |
| `s3_objects` | S3 Objects/Prefixes|
| `ec2`        | EC2 Instances      |
| `rds`        | RDS Instances      |
| `redis`      | ElastiCache Redis  |
| `docdb`      | DocumentDB Clusters|
| `eks`        | EKS Clusters       |
| `secrets`    | Secrets Manager    |

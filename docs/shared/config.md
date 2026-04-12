a9s stores view configuration in `~/.a9s/views/` as per-resource YAML files (e.g., `ec2.yaml`, `s3.yaml`) — optional, sensible defaults are built-in. AWS profiles and regions are read from `~/.aws/config`. a9s never reads `~/.aws/credentials` — authentication is delegated to the AWS SDK credential chain.

## View Customization

Default view config files are auto-created in `~/.a9s/views/` on first launch (one YAML file per resource type). These control which columns appear in list views and which fields show in detail views. Edit any file to customize — a9s never overwrites user-edited files. Delete a file to restore its defaults on next launch.

### File Structure

Each file (e.g., `ec2.yaml`) has two optional sections:

```yaml
list:
  Name:
    width: 24
  State:
    path: State.Name
    width: 12
  Lifecycle:
    key: lifecycle
    width: 12

detail:
  - InstanceId
  - State
  - InstanceType
  - LaunchTime
  - Tags
```

**`list:`** — Ordered map of columns. Each column has:
- **`path:`** — Dot-separated field path into the AWS SDK struct (e.g., `State.Name`)
- **`key:`** — Special computed key (e.g., `lifecycle`, `age`, `status`) — use instead of `path` for derived values
- **`width:`** — Column width in characters

If neither `path` nor `key` is specified, the column title is used as the field name.

**`detail:`** — List of field paths shown in the detail view (press `Enter` on a resource).

### Finding Available Fields

A complete field reference is maintained at `~/.a9s/views_reference.yaml`, automatically updated on each launch. It lists every available field path for each resource type, generated from AWS SDK struct definitions:

```yaml
ec2:  # ec2types.Instance
  - Architecture
  - BlockDeviceMappings[].DeviceName
  - BlockDeviceMappings[].Ebs.VolumeId
  - InstanceId
  - InstanceType
  - State.Code
  - State.Name
  ...
```

Use this file to discover paths you can add to your view configs.

### Examples

**Hide a column:** Remove it from the `list:` section.

**Reorder columns:** Reorder the entries under `list:` — YAML map order is preserved.

**Add a new column:**

```yaml
list:
  AZ:
    path: Placement.AvailabilityZone
    width: 16
```

**Change column width:**

```yaml
list:
  Name:
    width: 40
```

### Lookup Chain

View configs are loaded from two directories in order:
1. `~/.a9s/views/` — global defaults (auto-created on first run)
2. `.a9s/views/` in the current directory — per-project overrides

Per-project files overlay global ones on a per-resource basis.

## Color Themes

a9s ships with 11 built-in color themes, extracted to `~/.a9s/themes/` on first run. Set a theme in `~/.a9s/config.yaml`:

```yaml
theme: "dracula.yaml"
```

Built-in dark themes: `tokyo-night` (default), `catppuccin-mocha`, `dracula`, `nord`, `gruvbox-dark`, `solarized-dark`.
Built-in light themes: `tokyo-night-light`, `catppuccin-latte`, `nord-light`, `gruvbox-light`, `solarized-light`.

> **Note:** Dark themes are designed for dark terminal backgrounds; light themes for light terminal backgrounds. Match your theme to your terminal for best results.

To switch themes at runtime, press `:` and type `theme`. Custom themes: copy any built-in file, edit the colors, and point your config at it. Partial themes inherit missing colors from the default (Tokyo Night Dark). The `NO_COLOR` environment variable always forces monochrome, regardless of theme.

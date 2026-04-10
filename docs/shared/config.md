a9s stores view configuration in `~/.a9s/views/` as per-resource YAML files (e.g., `ec2.yaml`, `s3.yaml`) — optional, sensible defaults are built-in. AWS profiles and regions are read from `~/.aws/config`. a9s never reads `~/.aws/credentials` — authentication is delegated to the AWS SDK credential chain.

## Color Themes

a9s ships with 11 built-in color themes, extracted to `~/.a9s/themes/` on first run. Set a theme in `~/.a9s/config.yaml`:

```yaml
theme: "dracula.yaml"
```

Built-in themes: `tokyo-night` (default), `tokyo-night-light`, `catppuccin-mocha`, `catppuccin-latte`, `dracula`, `nord`, `nord-light`, `gruvbox-dark`, `gruvbox-light`, `solarized-dark`, `solarized-light`.

To switch themes at runtime, press `:` and type `theme`. Custom themes: copy any built-in file, edit the colors, and point your config at it. Partial themes inherit missing colors from the default (Tokyo Night Dark). The `NO_COLOR` environment variable always forces monochrome, regardless of theme.

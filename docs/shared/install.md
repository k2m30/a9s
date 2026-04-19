### Homebrew (macOS and Linux)

```sh
brew install k2m30/a9s/a9s
```

### Scoop (Windows)

```powershell
scoop bucket add a9s https://github.com/k2m30/scoop-a9s.git
scoop install a9s
```

### Go install

```sh
go install github.com/k2m30/a9s/v3/cmd/a9s@latest
```

### Download binary

Download the latest release for your platform from [GitHub Releases](https://github.com/k2m30/a9s/releases/latest).

Available platforms:
- **macOS**: Intel (amd64) and Apple Silicon (arm64)
- **Linux**: amd64 and arm64
- **Windows**: amd64 and arm64

> **Windows note:** Downloaded binaries may trigger a Microsoft Defender SmartScreen warning because they are not code-signed. Click "More info" → "Run anyway" to proceed, or install via Scoop to avoid this. Windows support is new and has been verified via cross-compilation and CI only — the maintainer does not have a Windows machine. If you encounter any issues, please [open an issue](https://github.com/k2m30/a9s/issues/new).

### Docker

```sh
# Demo mode (no AWS credentials needed)
docker run --rm -it ghcr.io/k2m30/a9s:latest --demo

# Real AWS access — mount the whole ~/.aws directory so SSO tokens and
# static credentials resolve; the image defaults to AWS_EC2_METADATA_DISABLED=true
# so missing creds fail fast instead of timing out against IMDS.
docker run --rm -it \
  -v ~/.aws:/home/a9s/.aws:ro \
  ghcr.io/k2m30/a9s:latest
```

For SSO profiles, run `aws sso login --profile <name>` on the host before
starting the container so the cached token exists in `~/.aws/sso/cache`.

To persist per-user view / theme customization across runs, also mount
`~/.a9s`: `-v ~/.a9s:/home/a9s/.a9s`. Without that mount the container
ships with the built-in defaults only.

### Build from source

Requires Go 1.26+.

```sh
git clone https://github.com/k2m30/a9s.git
cd a9s
make build
./a9s
```

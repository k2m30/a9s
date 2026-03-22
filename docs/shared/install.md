### Homebrew (macOS and Linux)

```sh
brew install k2m30/a9s/a9s
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

### Docker

```sh
# Demo mode (no AWS credentials needed)
docker run --rm -it ghcr.io/k2m30/a9s:latest --demo

# Real AWS access
docker run --rm -it -v ~/.aws/config:/home/a9s/.aws/config:ro ghcr.io/k2m30/a9s:latest
```

### Build from source

Requires Go 1.26+.

```sh
git clone https://github.com/k2m30/a9s.git
cd a9s
make build
./a9s
```

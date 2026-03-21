---
title: "Installation"
---

## Homebrew (macOS and Linux)

```sh
brew install k2m30/a9s/a9s
```

## Go install

```sh
go install github.com/k2m30/a9s/cmd/a9s@latest
```

## Download Binary

Download the latest release for your platform from [GitHub Releases](https://github.com/k2m30/a9s/releases/latest).

Available platforms:
- **macOS**: Intel (amd64) and Apple Silicon (arm64)
- **Linux**: amd64 and arm64, plus .deb, .rpm, and .apk packages
- **Windows**: amd64 and arm64

Verify the signature (optional):

```sh
cosign verify-blob --signature checksums.txt.sig checksums.txt
```

## Docker (demo)

Try a9s without installing — runs in demo mode with synthetic data:

```sh
docker run --rm -it ghcr.io/k2m30/a9s:latest --demo
```

## Build from Source

Requires Go 1.26+.

```sh
git clone https://github.com/k2m30/a9s.git
cd a9s
make build
./a9s
```

## Quick Start

a9s uses the standard [AWS credential chain](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html). Any of these work:
- Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
- AWS config files (`~/.aws/config`, `~/.aws/credentials`)
- EC2 instance metadata / ECS task role / SSO

```sh
a9s                       # use default profile
a9s -p production         # use a specific profile
a9s -r eu-west-1          # override region
a9s --version             # print version
```

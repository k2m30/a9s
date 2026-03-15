# Quickstart: a9s

## Prerequisites

- Go 1.22 or later
- AWS CLI configured with at least one profile
  (`~/.aws/config` and/or `~/.aws/credentials`)
- Valid AWS credentials for the profile you want to use
- Terminal with 256-color support

## Build and Run

```bash
# Clone and build
git clone <repo-url>
cd a9s
go build -o a9s ./cmd/a9s

# Run with default profile
./a9s

# Run with specific profile and region
./a9s --profile prod --region eu-west-1
```

## First Steps

1. **Launch**: Run `./a9s`. The main screen shows supported
   resource types (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets).

2. **Browse resources**: Use `j`/`k` to navigate the list, press
   `Enter` to view resources of that type. Or type `:ec2` to
   jump directly to EC2 instances.

3. **Switch profile**: Type `:ctx` to see all AWS profiles.
   Select one and press `Enter`.

4. **Switch region**: Type `:region` to see available regions.
   Select one and press `Enter`.

5. **Inspect a resource**: From any resource list, select a
   resource and press `d` to see all its attributes.

6. **Filter**: Press `/` and type to filter the current list.
   Press `Escape` to clear the filter.

7. **Copy**: Press `c` on any resource to copy its ID to
   your clipboard.

8. **Navigate back**: Press `Escape` to go back, or `[`/`]`
   for history navigation.

9. **Get help**: Press `?` to see all available commands and
   keybindings.

10. **Exit**: Type `:q` or press `Ctrl-C`.

## Smoke Test Checklist

After building, verify these work:

- [ ] App launches and shows resource type list
- [ ] Header displays profile name and region
- [ ] `:ec2` navigates to EC2 instance list
- [ ] `j`/`k` moves cursor up/down in lists
- [ ] `d` on a resource shows describe view
- [ ] `Escape` returns to previous view
- [ ] `:ctx` shows profile list
- [ ] `:region` shows region list
- [ ] `/` activates filter mode
- [ ] `?` shows help overlay
- [ ] `:q` exits the application
- [ ] `Ctrl-R` refreshes the current view

# Step 1 — Raw-facts collector (`cmd/snapshot`)

Downloads raw resource facts for **all 66 resource types** a9s can display,
from a real read-only AWS account, into one big JSON file. This is the
comparison baseline the checklist generator ([step 2](step2-checklist.md)) turns into expected
screen checklists. Part of the
[real-account web-e2e suite](snapshot-web-e2e.md).

The collector is **fully standalone**: it uses the AWS SDK directly — no a9s
packages, no a9s binary involvement. That independence is deliberate: the
baseline must not be derived through the code under test.

## What it produces

```text
tests/e2e/testdata/snapshot/<profile>--<region>/
└── snapshot.json    # one big file: {region, collected_at, types: {<type>: raw facts…}, errors}
```

The whole directory is **gitignored** (root `.gitignore` →
`tests/e2e/testdata/snapshot/`) because it contains real resource names.
Re-run the collector any time to refresh the baseline.

## Running

```sh
# everything (all 66 types)
go run ./cmd/snapshot --profile my-real-profile --region eu-west-2

# refresh just a few types — merged into the existing snapshot.json
go run ./cmd/snapshot --profile my-real-profile --region eu-west-2 --types s3,ec2
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--profile` | *(required)* | AWS profile to read. **Never stored in code or committed artifacts** — a runtime argument only. |
| `--region` | profile's region | Region to capture (single region per run). |
| `--out` | `tests/e2e/testdata/snapshot` | Output root. |
| `--types` | `all` | Comma-separated resource short names, or `all`. Partial runs merge into the existing file. |

Requires a working read-only profile (`aws sts get-caller-identity --profile <p>`
should succeed). A type failing (missing permission, unavailable service) does
not sink the rest — it lands in the file's `errors` section.

## What it captures

Per type, exactly the raw facts the checklist rules need, defined by
`docs/resources/<type>.md` — §1 Identity (list API + fields) and §3.2 (Wave-2
per-resource describe calls). Examples: s3 = `ListBuckets` + per-bucket
`GetPublicAccessBlock` outcome; ec2 = `DescribeInstances` +
`DescribeInstanceStatus` (incl. scheduled events); trail = `DescribeTrails` +
`GetTrailStatus`. Per-item describe failures are recorded as
`{outcome: "error", error_code}` — never classified.

Everything is captured with full pagination — display caps (page size,
enrichment cap) are screen behavior and belong to the checklist generator, not the data.
One exception: event-log-shaped types (`ct-events`) cap at the 200 newest
events with a `truncated` marker.

## Adding / adjusting a resource type

Each type is a `capture<Type>(ctx, cfg) (any, error)` function in a per-family
file (`ec2_network.go`, `databases.go`, `compute.go`, `messaging.go`,
`security.go`, `ops.go`, `s3.go`), registered in `registry.go`. Use plain SDK
clients (`<svc>.NewFromConfig(cfg)`); which operations matter is defined by
`docs/resources/<type>.md` §1 and §3.2.

Guidelines:

- Emit **raw facts**, not classifications — record what AWS returned (states,
  flags, error codes), never "is this a finding". The checklist generator decides findings
  from the documented rules.
- Capture everything; let the checklist generator apply caps and paging.
- Keep the JSON shape readable and stable — the checklist generator and human diffs depend
  on it.

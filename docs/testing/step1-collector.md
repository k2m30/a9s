# Step 1 — Raw-facts collector (`cmd/snapshot`)

Downloads raw resource facts for **all 70 top-level resource types** a9s can display,
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
# everything (all 70 types)
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

Per type, the collector sends the requests of the type's fetcher, its detail
enricher and its Wave-2 enricher that answer a fact the snapshot records,
with the same operation and the same input.
`TestT11a_CollectorSendsEveryRequestTheAppSendsForEveryTopLevelType` runs the
app and the capture for every top-level type against synthetic answers and
enforces this:

- An operation the app sends and the collector does not is declared in
  `notCaptured` with its reason (today: `cmd/checklist` compares no fact it
  answers). An undeclared one fails, and so does a declared one the collector
  sends or the app does not.
- An operation both send is compared request by request, input included. A
  request that differs by design is declared in `declaredRequestDifferences`
  by its exact input, with its reason; only that request leaves the
  comparison.

s3, the type `cmd/checklist` compares, has no `notCaptured` entry: per bucket
the collector sends `GetBucketLocation` and, to the bucket's own region,
`GetBucketNotificationConfiguration`, `GetPublicAccessBlock`,
`GetBucketPolicyStatus`, `GetBucketAcl`, `GetBucketVersioning`,
`GetBucketLogging`, `GetBucketLifecycleConfiguration` and
`GetObjectLockConfiguration`. Other examples: ec2 = `DescribeInstances` +
`DescribeInstanceStatus` (incl. scheduled events); trail = `DescribeTrails` +
`GetTrailStatus`. Related-panel reads are not the collector's.

Each answer is recorded in the form the app shows it:

- A type whose list names the items and whose describe fills them
  (`describeListed` in `sources.go`) has one row per listed item. A described
  item has outcome `ok`, an item the answer names nothing for has
  `not_found`, and an item of a refused call has `error` with the code. One
  item's failure never fails the type. An item the service reports gone
  (deleted between the list and the describe) has no row.
- Every other read of an item records its outcome through `outcomeOf`: a
  refused read is `error` with the code AWS returned, never an empty answer,
  and a code by which AWS states the configuration does not exist (no public
  access block, no console password, no lifecycle rules) is `none`. A read
  that answers for one item through `readOne` is `not_found` when the answer
  names nothing or the item is gone.
- A fact a read answers is recorded only when the read answered it: after a
  failed or cut-short read it is absent, with the read's outcome beside it,
  never `false` or `0`. Absent beside an `ok` outcome is the zero value.
- Only the list a type (or one of its parts) is made of fails it. A read
  that adds to the listed rows records its outcome and never fails the type.
- A type listed from more than one call (`composed` in `sources.go`) keeps the
  rows of the calls that answered and lists the failed calls in
  `partial_errors`. The run reports such a type as an error: its stderr line
  names the failure and the summary counts it.
- A list walk (`pageAll` in `pagination.go`) that fails on a later page keeps
  the rows of the pages before it and returns the error.

Everything is captured with full pagination — display caps (page size,
enrichment cap) are screen behavior and belong to the checklist generator, not the data.
The exceptions are declared in `TestT11a_EveryBoundedReadIsDeclared`: reads
that answer newest first, where the app reads the same first page. Event-log-shaped
types (`ct-events`) cap at the 200 newest events, with `truncated` set only
when CloudTrail has more.

## Adding / adjusting a resource type

Each type is a `capture<Type>(ctx, cfg) (any, error)` function in a per-family
file (`ec2_network.go`, `databases.go`, `compute.go`, `messaging.go`,
`security.go`, `ops.go`, `platform.go`, `s3.go`), registered in `registry.go`.
Use plain SDK clients (`<svc>.NewFromConfig(cfg)`); which operations matter is
what the app's fetcher and enrichers send for the type.

Guidelines:

- Emit **raw facts**, not classifications — record what AWS returned (states,
  flags, error codes), never "is this a finding". The checklist generator decides findings
  from the documented rules.
- Capture everything; let the checklist generator apply caps and paging.
- Keep the JSON shape readable and stable — the checklist generator and human diffs depend
  on it.
- Never record a value that can carry a secret. Environment variables, job and action
  arguments, call payloads, and URLs that can hold userinfo or a token in their path or query
  are recorded as keys, a reference to where the secret lives, or an origin — never the value.
  The file sits on disk and gets copied around.

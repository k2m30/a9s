# v3.31.0 — Pagination Fixes & Code Modernization

Three pagination bugs fixed, plus a sweep of Go modernization across 95 files.

## Bug Fixes

**SNS Subscriptions false "(0+)" count** — The AWS `ListSubscriptions` API has a documented quirk where it returns a `NextToken` even when there are zero results. a9s now treats empty-result pages as non-truncated, so the menu correctly shows "(0)" instead of "(0+)". The same guard is applied to SNS Topics.

**Secrets Manager & SSM Parameters tiny page size** — Both fetchers were relying on the AWS API default page size, which turned out to be ~10 items. They now explicitly request 50 items per page (`DefaultPageSize`), matching all other paginated fetchers. The menu will show accurate counts instead of "10+".

**Lowercase `m` for load-more** — The load-more key now accepts both `m` and `M`. The hint bar at the bottom of paginated lists has been updated to show the lowercase variant.

## Code Quality

- `interface{}` replaced with `any` across 95 files in `internal/aws/`
- Manual for-loop contains checks replaced with `slices.Contains` (3 files)
- `strings.Index` replaced with `strings.Cut` (2 files)
- `strP` helper replaced with `aws.String` in Transit Gateway related checker

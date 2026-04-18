# Implementation Agent Prompt — Related-Panel Checkers

You are implementing AWS related-panel checkers for the a9s project. Working directory: `/Users/k2m30/projects/a9s`.

## Source inputs (READ THESE FIRST)

1. `docs/historical/019-related-panel/related-panel-missing.md` — the full list of 114 parent→related pairs currently missing implementations. Every row is your work queue.
2. `docs/historical/019-related-panel/related-panel-devops-consensus.md` — 5 blind DevOps reviewers' independent answers for each pair (possible? + AWS API sequence). **This is your source of truth for what each pair means in AWS and how to resolve it.**
3. `docs/related-resources.md` — the golden contract.

## Absolute rules

1. **NO STUBS.** A function with signature `func check...(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache)` that ignores its inputs is banned. Every checker MUST inspect at least one parameter and produce an output that depends on what it found. A function returning the same `Count: 0` or `Count: -1` regardless of input is a stub — unacceptable.
2. **Budget: N+1 throttled AWS API calls.** You can iterate. You can make one call per item across a list. You can do list+describe chains. Wrap every call with `RetryOnThrottle(ctx, c.Config, func() ... { ... })`. There is NO 1-call limit — the original prompt's wording was "bounded API call wrapped in RetryOnThrottle", not "one call".
3. **Use the cache when it helps, but do NOT limit yourself to cache-only.** If data isn't in cached list responses, make the describe / get calls needed. `NeedsTargetCache: true` is appropriate when your algorithm scans cached target resources. When your algorithm starts from the source and fans out via AWS APIs, you don't need target cache at all.
4. **Consult the DevOps consensus doc for every pair.** The 5 reviewers already mapped out AWS API sequences. Use the agreed sequence. When they disagreed, pick the approach most consistent with AWS documentation and note the choice in a comment.
5. **For unanimous `no` pairs** (all 5 reviewers said no, listed in consensus doc under "Unanimous `no`"):
   - Remove the row from `docs/related-resources.md` (the parent's row in the per-type contract table — drop this related type from the comma-separated list).
   - Remove the corresponding `RegisterRelated` entry from the parent's `.go` file.
   - Do NOT create a checker function.
   - Add a short note in the commit-less changeset about what was dropped and why (the consensus reasoning).
6. **No commits.** The user commits themselves.

## Implementation pattern per pair (when NOT unanimous `no`)

Each pair needs 4 things:

### (a) Checker function

Add to the appropriate `internal/aws/<parent>_related.go` file (create `<parent>_related_extra.go` if the main file is getting crowded).

- Signature: `func check<Parent><Target>(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult`
- Start by asserting `res.RawStruct` to the expected type (e.g. `assertStruct[apigatewayv2types.Api](res.RawStruct)`). Return `Count: -1` if assertion fails (defensive).
- If your algorithm needs AWS API calls: `c, ok := clients.(*ServiceClients); if !ok || c == nil { return {..., Count: -1} }`. Then call the API via `RetryOnThrottle`.
- Use `relatedResult(targetType, ids)` helper (defined in `ec2_related.go`) to build the result — it handles dedup and Count computation.
- Count semantics:
  - `Count: 0` = definitively no related resources found (e.g. the field is empty, the API returned nothing). The function inspected its inputs and found nothing.
  - `Count: -1` = could not determine (wrong struct type, nil clients, API error). Rare.
  - `Count: N` where N ≥ 1 = real matches.

### (b) `RegisterRelated` entry

Ensure the parent's `init()` in `<parent>.go` or `<parent>_related.go` contains `{TargetType: "<target>", DisplayName: "<human-readable>", Checker: check<Parent><Target>, NeedsTargetCache: <true|false>}`.

- `NeedsTargetCache: true` if your checker scans the target's cached list (e.g. scanning the `cfn` cache looking for stacks that reference this resource).
- `NeedsTargetCache: false` if your checker only reads from `res.RawStruct`, `res.Fields`, or makes AWS API calls independent of cached target data.

### (c) Interface + fake (when a new AWS SDK operation is used)

If you add a new SDK call:
1. Add a narrow interface to `internal/aws/interfaces.go` at the end of the file:
   ```go
   // XyzAbcAPI defines the interface for Xyz:Abc.
   type XyzAbcAPI interface {
       Abc(ctx context.Context, params *xyz.AbcInput, optFns ...func(*xyz.Options)) (*xyz.AbcOutput, error)
   }
   ```
2. Add method receiver on the aggregate `XyzAPI` interface (also in `interfaces.go`) so `*xyz.Client` satisfies it.
3. Add a fake implementation in `internal/demo/fakes/<service>.go` so demo mode builds — return a minimal empty-but-non-nil output. Demo mode won't exercise the new behavior; this is purely to keep the build green.

### (d) Test

Add to `tests/unit/aws_<parent>_related_test.go`:

- `TestRelated_<Parent>_<Target>_Match` — positive case: constructs a `resource.Resource` where `RawStruct` (or `Fields` / `cache`) contains a value that links to the target. For checkers that use the cache, populate `resource.ResourceCache` with the target entries. For checkers that make AWS API calls, inject a fake `*ServiceClients` via `resource.Resource{...}` + a local `ServiceClients` whose relevant API field is a fake that returns the expected response.
- `TestRelated_<Parent>_<Target>_Empty` — negative case: the source has no linkage → assert `Count == 0`.
- `TestRelated_<Parent>_<Target>_WrongRawStruct` — assert the checker returns `Count: -1` when `RawStruct` is the wrong type.

For checkers using AWS API calls, you can construct an inline fake inside the test using an anonymous struct or a minimal fake type that satisfies the narrow interface. Look for existing patterns in `tests/unit/aws_ec2_related_test.go` or similar `_test.go` files for how API-call tests are structured in this codebase.

### (e) Fixture (only when needed for `--demo` mode quality)

Demo mode uses `internal/demo/fixtures/<service>.go`. If the checker relies on a specific field in the raw struct and the existing fixture doesn't populate it, update the fixture to include a plausible value so the related panel shows meaningful data in demo mode. This is lowest priority — tests come first.

## For each pair, your checklist

- [ ] Read the consensus doc entry for this pair.
- [ ] If unanimous `no`: remove registration + golden-doc row. Done.
- [ ] Otherwise: implement checker, register, test, optionally update fixture.
- [ ] After each pair, run `/opt/homebrew/bin/go build ./...` to keep the tree green.

## Final verification

After all pairs are processed:
- `/opt/homebrew/bin/go build ./...` — must succeed
- `cd /Users/k2m30/projects/a9s && /opt/homebrew/bin/go test ./tests/unit/ -count=1` — must pass
- `cd /Users/k2m30/projects/a9s && /opt/homebrew/bin/go test ./tests/unit/ -run TestRelatedPanel_ContractMatchesGoldenDoc -count=1` — must pass (golden contract)
- Grep must return ZERO matches: `grep -rn "func check.*_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache" /Users/k2m30/projects/a9s/internal/aws/`

## Bash rules

- Never chain commands with `&&`, `;`, `|`, `cd`. Each Bash call is one standalone command with absolute paths.
- Use `rtk` wrappers (e.g. `rtk go test ...`) if the shell provides them.

## Reporting

After finishing, report:
1. Which pairs were implemented (count + brief list)
2. Which pairs were dropped as unanimous `no` (list)
3. Any pair where you diverged from the consensus doc's recommended API sequence, and why
4. Build + test output
5. Result of the stub grep (must be 0)

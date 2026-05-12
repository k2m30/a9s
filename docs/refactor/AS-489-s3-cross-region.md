# AS-489 — S3 related-defs hit PermanentRedirect for cross-region buckets

**Stage:** 2 (Architect spec)
**Size:** S (after Pattern A; CTO sized M to leave room for Pattern B)
**Owner of next stage:** QA (Stage 3) → Coder (Stage 4)
**Stage 5 reviewers:** CodeReviewer + CodexReviewer (size S → no Architect re-review required, but I will read the diff)
**Stage 6.5:** Mandatory — touches `internal/aws/`

This spec is the contract. The CTO Stage 1 triage comment on AS-489 listed both Pattern A (soft-truncate) and Pattern B (region-pin per bucket) as legal options. This document picks **Pattern A** and locks the file-level scope.

---

## 1. Decision — Pattern A (soft-truncate to ApproximateZero)

For each of the four related-def checkers in `internal/aws/s3_related.go` that today make a per-bucket S3 API call and return `Count:-1, Err:err` on any non-recognised error, detect the cross-region rejection pair `(PermanentRedirect | IllegalLocationConstraintException)` and return `resource.ApproximateZero(targetType)` instead — i.e. `Count:0, Approximate:true`, which renders as `0+`.

### Why Pattern A (not B)

- **Literal AC match.** AS-489's AC2 says "no `<unknown>` fallout caused by 301." `ApproximateZero` renders as `0+`, never `<unknown>`. A region-pinned `s3:GetBucketLocation` + per-bucket client (Pattern B) would also satisfy the AC, but at 3–5× the LOC and a brand-new operational surface (per-bucket region cache + invalidation on profile/region switch).
- **Consistency with existing precedent.** `internal/aws/s3_issue_enrichment.go:94–107` already treats this exact error pair as "operational, not a bug." We extend the same pattern to the related-defs and extract the detection into one helper.
- **a9s is region-scoped by design.** The product stance documented in `docs/architecture.md` is that every per-resource view is region-scoped. `ListBuckets` is the one global outlier; soft-truncating per-bucket calls when the bucket lives outside the active region is the consistent stance.
- **Reversibility.** Pattern A is ~80 LOC and one helper. If we later decide to do region-pinning (Pattern B), Pattern A doesn't get in the way — the helper just becomes one branch among several.

### Why the rendered token must be `0+` (Approximate), not `?` (UnknownRelated)

- The bucket exists and is reachable — only the per-bucket call failed for an environmental reason. We have not "failed to scan"; we have scanned and the answer is "we cannot see across regions." That semantic maps cleanly to `ApproximateZero("…")`: Count=0, Approximate=true. See `internal/resource/related.go:202–215`.
- `UnknownRelated()` (Count=-1, renders `?`) would re-introduce exactly the `<unknown>` token AC2 forbids.

---

## 2. File scope

### 2.1 NEW: `internal/aws/s3_cross_region.go` (~30 LOC)

```go
// s3_cross_region.go — Shared detection of S3 cross-region API rejections.
//
// ListBuckets returns ALL buckets globally regardless of the configured
// client region, but per-bucket calls require the bucket's own regional
// endpoint. AWS rejects with PermanentRedirect (301) or
// IllegalLocationConstraintException (400) when the configured client
// region differs from the bucket's region. These are legitimate environmental
// conditions, not bugs — related-defs return ApproximateZero ("0+") and
// the issue enricher marks TruncatedIDs ("?" row marker).
//
// Precedent / first user: EnrichS3PublicAccessBlock in s3_issue_enrichment.go.
package aws

import (
    "errors"

    smithy "github.com/aws/smithy-go"
)

// isS3CrossRegionErr reports whether err is the S3 cross-region rejection
// pair: PermanentRedirect (301) or IllegalLocationConstraintException (400).
// Both indicate the configured S3 client's region does not match the
// target bucket's region — not a bug, just multi-region account topology.
func isS3CrossRegionErr(err error) bool {
    if err == nil {
        return false
    }
    var apiErr smithy.APIError
    if !errors.As(err, &apiErr) {
        return false
    }
    code := apiErr.ErrorCode()
    return code == "PermanentRedirect" || code == "IllegalLocationConstraintException"
}
```

### 2.2 MODIFY: `internal/aws/s3_related.go`

Four call sites. Each currently has the shape:

```go
if err != nil {
    if strings.Contains(err.Error(), "<HappyEmptySentinel>") {
        return resource.RelatedCheckResult{TargetType: "<t>", Count: 0}
    }
    return resource.RelatedCheckResult{TargetType: "<t>", Count: -1, Err: err}
}
```

Insert one branch between the happy-empty sentinel and the bare `-1` return:

```go
if isS3CrossRegionErr(err) {
    return resource.ApproximateZero("<t>")
}
```

Exact sites (line numbers anchor on current `phase-05-pr-05b-msg-taxonomy-AS-74` HEAD):

| Function | Line | API call | TargetType arg to `ApproximateZero` |
|---|---|---|---|
| `checkS3CFN` | after the `NoSuchTagSet` branch (~line 102) | `GetBucketTagging` | `"cfn"` |
| `checkS3KMS` | after the `ServerSideEncryptionConfigurationNotFoundError` branch (~line 163) | `GetBucketEncryption` | `"kms"` |
| `checkS3Logs` | inside the existing `if err != nil` block (~line 212) | `GetBucketLogging` | `"s3"` |
| `checkS3Role` | after the `NoSuchBucketPolicy` branch (~line 416) | `GetBucketPolicy` | `"role"` |

Note: `checkS3Logs` has no happy-empty sentinel today, so the cross-region branch is added as the first (and only) classifier inside the `err != nil` block, before the fall-through `-1` return.

### 2.3 MODIFY: `internal/aws/s3_issue_enrichment.go`

Replace the inline cross-region detection at lines 101–107 with a call to the new helper. Behaviour MUST stay byte-identical: `truncated = true`, `truncatedIDs[r.ID] = true`, `continue`. No failure-aggregate entry. Keep the existing 10-line code comment block (lines 94–100) — it documents the contract and is referenced from the new helper file. Verify by re-running `TestEnrichS3PublicAccessBlock_CrossRegionDoesNotSpamErrorLog` and `TestEnrichS3PublicAccessBlock_IllegalLocationConstraintNotSpammed` in `tests/unit/qa_s3_cross_region_test.go`.

### 2.4 NO CHANGE: `internal/aws/s3.go` (list-path `firstS3NotificationTargets`)

Line 196–228 already swallows ALL errors silently (`// Best effort enrichment: keep list results even if this lookup fails.`). That includes PermanentRedirect. Result: cross-region buckets land in the list with empty `Fields["notification_lambda" | "_sns" | "_sqs"]`, and the three forward-lookup pivots (`checkS3Lambda`, `checkS3SNS`, `checkS3SQS`) honestly return `Count:0` — no `<unknown>` fallout. The CTO's call-site map listed this line for completeness; the spec confirms no change is required. Adding logging here would scope-creep AS-489.

### 2.5 OUT OF SCOPE for AS-489 (will NOT be touched)

- Any per-bucket `s3:GetBucketLocation` lookup, bucket→region cache, or region-pinned client construction (this is Pattern B; if we later need it, file a new issue).
- Any change to `firstS3NotificationTargets` error handling.
- Any change to `RelatedCheckResult` semantics or to the `ApproximateZero` / `UnknownRelated` helpers.
- Any change to the integration test `TestLiveFullIntegration_AllResourcesBaseline/s3` itself — it is the failing AC repro; Coder must make it pass, not modify it. The test file currently lives at `tests/integration/full_integration_test.go` (or `_helpers_test.go`); do not touch.

---

## 3. Test scope (QA owns Stage 3)

### 3.1 NEW file: `tests/unit/qa_s3_related_cross_region_test.go` (~180 LOC)

`package unit_test` (same as `aws_s3_related_test.go` — reuses `s3CheckerByTarget`, `s3CheckerByDisplayName`, `emptyBucketResource`).

**One `s3Fake` per affected API**, each returning `&smithy.GenericAPIError{Code: <code>, Message: …}` from the relevant `GetBucket*` method. Two error codes per checker → 8 sub-tests, table-driven is fine. Example shape:

```go
type s3GetBucketTaggingErrFake struct{ code string }
func (f *s3GetBucketTaggingErrFake) GetBucketTagging(_ context.Context, _ *s3.GetBucketTaggingInput, _ ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error) {
    return nil, &smithy.GenericAPIError{Code: f.code, Message: "test cross-region"}
}
```

For each `(checker, errorCode)` pair, assert:

```go
got := checker(ctx, &awsclient.ServiceClients{S3: fake}, emptyBucketResource("xregion"), nil)
if got.Count != 0 || !got.Approximate || got.Err != nil {
    t.Fatalf("want Count=0 Approximate=true Err=nil; got Count=%d Approximate=%v Err=%v", got.Count, got.Approximate, got.Err)
}
if got.TargetType != "<expected target>" { t.Fatalf("...") }
```

**Test matrix — 4 checkers × 2 error codes = 8 sub-tests minimum:**

| Checker (`s3CheckerByTarget` / `s3CheckerByDisplayName`) | API errored | Error codes |
|---|---|---|
| `s3CheckerByTarget(t, "cfn")` | `GetBucketTagging` | `PermanentRedirect`, `IllegalLocationConstraintException` |
| `s3CheckerByTarget(t, "kms")` | `GetBucketEncryption` | `PermanentRedirect`, `IllegalLocationConstraintException` |
| `s3CheckerByDisplayName(t, "Access Log Bucket")` | `GetBucketLogging` | `PermanentRedirect`, `IllegalLocationConstraintException` |
| `s3CheckerByTarget(t, "role")` | `GetBucketPolicy` | `PermanentRedirect`, `IllegalLocationConstraintException` |

**Sanity-test contract preservation (1 test):** verify that a NON cross-region error (e.g. `Code:"AccessDenied"`) still produces `Count:-1, Err:non-nil` for one of the four checkers — guards against the new branch swallowing real failures. This is the negative test that proves we haven't broken the existing `-1` semantics.

**Helper-level test (1 test):** call `isS3CrossRegionErr` directly with `(nil)`, `(errors.New("plain"))`, `(&smithy.GenericAPIError{Code:"PermanentRedirect"})`, `(&smithy.GenericAPIError{Code:"IllegalLocationConstraintException"})`, `(&smithy.GenericAPIError{Code:"AccessDenied"})`. Expect `false, false, true, true, false` respectively. Note: `isS3CrossRegionErr` is unexported — this test must live in `package unit` (NOT `unit_test`) and import the package as a same-package test, OR be skipped if Coder chooses to keep the helper unexported. **Architect decision: keep the helper unexported (internal detail) and rely on the 8 checker-level tests plus the existing `qa_s3_cross_region_test.go` enricher tests to cover the behaviour. Skip the direct helper test.**

Final scope: **9 tests** (8 checker × error-code combos + 1 contract-preservation guard).

### 3.2 NO CHANGE to existing tests

- `tests/unit/qa_s3_cross_region_test.go` — already covers the enricher path. Coder's refactor of `s3_issue_enrichment.go` to use the new helper MUST keep these green.
- `tests/unit/aws_s3_related_test.go` — existing pass/fail cases for the 4 checkers MUST stay green; the new branch is only entered on the cross-region error code, never on the happy path.

### 3.3 Integration test (not a QA deliverable, but the AC repro)

`TestLiveFullIntegration_AllResourcesBaseline/s3` is the failing live test. Stage 6.5 by E2ETester reruns it against the multi-region account; no Stage 3 work is required there. If QA discovers a unit-level gap that would have caught the regression in CI, file a follow-up issue rather than expand AS-489's scope.

---

## 4. Acceptance criteria (refined from CTO triage)

1. The four affected checkers (`checkS3CFN`, `checkS3KMS`, `checkS3Logs`, `checkS3Role`) return `RelatedCheckResult{Count:0, Approximate:true, Err:nil, TargetType:<their target>}` when the underlying `GetBucket*` call returns `PermanentRedirect` or `IllegalLocationConstraintException`. (Verified by §3.1 tests.)
2. For non-cross-region errors (e.g. `AccessDenied`, transient throttle that survives retries, anything else), the four checkers still return `Count:-1, Err:err` — the existing contract is preserved. (Verified by §3.1 guard test.)
3. `EnrichS3PublicAccessBlock` still passes its existing cross-region tests in `tests/unit/qa_s3_cross_region_test.go` after the refactor to use `isS3CrossRegionErr`. (Verified by re-running those tests.)
4. `TestLiveFullIntegration_AllResourcesBaseline/s3` passes against an account that contains buckets in multiple regions. (Verified by E2ETester at Stage 6.5.)
5. The chosen pattern is documented inline at `internal/aws/s3_cross_region.go` (the helper file's package-level comment), so the next reader doesn't reintroduce the bug.
6. Read-only invariant preserved — no new mutating S3 calls introduced. (Verified by `make security` and CodeReviewer.)
7. `make ready-to-push` passes locally before the PR is opened (Stage 6 gate).

---

## 5. Sizing & risk

**Size:** S — one new file (~30 LOC), four 3-line patches to `s3_related.go`, one 6-line refactor in `s3_issue_enrichment.go`, plus ~180 LOC of new tests. Total diff ~250 LOC.

**Risk:** Low. The new branch is only entered on two well-known AWS error codes; the old fall-through `-1` behaviour is preserved for everything else. The helper's only edge case is `err == nil`, which is guarded at the top of the function. No new dependencies; `smithy.APIError` and `errors.As` are already used in the same package (`s3_issue_enrichment.go`).

**Stage 5 reviewers:** size S → CodeReviewer + CodexReviewer per `docs/development-process.md`. Architect re-review is optional for S; I will read the diff. CTO does the final sign-off.

**Stage 6.5:** Mandatory because `internal/aws/` is touched. E2ETester reruns `TestLiveFullIntegration_AllResourcesBaseline/s3` against the multi-region dev account.

---

## 6. Cross-references

- AS-489 (this spec's parent issue)
- AS-481 (Stage 6.5 verdict that surfaced the bug — closed)
- AS-431 (prior Stage 6.5 sign-off that missed it — gap acknowledged in AS-489 filing)
- `internal/aws/s3_issue_enrichment.go:75–158` (Pattern A precedent, first user of `isS3CrossRegionErr`)
- `internal/resource/related.go:202–215` (`ApproximateZero` definition and rendering contract)
- `docs/development-process.md` §"Stage 5 — Review" and §"Stage 6 — Pre-push Validation"

// Generic helpers shared by every *_related.go checker (moved out of ec2_related.go — they are not EC2-specific).
package aws

import (
	"context"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// assertStruct extracts a value of type T from an interface that may hold
// either T or *T. Used for RawStruct type assertions across related checkers.
func assertStruct[T any](v any) (T, bool) {
	if val, ok := v.(T); ok {
		return val, true
	}
	if p, ok := v.(*T); ok && p != nil {
		return *p, true
	}
	var zero T
	return zero, false
}

// kmsKeyIDFromField extracts a DescribeKey-compatible identifier from an AWS
// field that references a KMS key, for a checker whose source resource has
// type srcType. AWS returns this reference in one of four shapes — key ARN
// (arn:aws:kms:region:account:key/UUID), bare key UUID, alias ARN
// (arn:aws:kms:region:account:alias/NAME, where NAME frequently contains its
// own "/" as in the AWS-managed "aws/s3", "aws/rds", etc.), or bare alias
// name ("alias/NAME"). Every *_related.go KMS checker used to split on the
// LAST "/" — correct for the key/UUID shape but wrong for any alias, since it
// discards the "alias/aws/" prefix and returns just the trailing
// service-name segment (e.g. "s3"), which DescribeKey/FetchByIDs then
// rejects as an invalid key ID. This is the single extraction seam: it
// strips only a recognized "key/" or "alias/" ARN prefix, preserving the
// full alias name (including embedded slashes) instead of trimming to the
// last path segment.
//
// It also guards against the degenerate case a live incident surfaced: a
// thin, cache-seeded source resource with no real encryption data can hand a
// checker a bare, non-ARN field value that happens to collide with the
// source's own resource-type short name (e.g. an s3 bucket's "KMSMasterKeyID"
// resolving to the literal string "s3"). No AWS-assigned key ID, key ARN,
// alias name, or alias ARN is ever equal to a bare a9s type short name, so
// any extracted value matching srcType is treated as fabricated/garbage and
// dropped (returns "") rather than handed to DescribeKey/FetchByIDs.
//
// Returns "" for an empty input, or when the extracted value equals srcType.
// Returns the input unchanged if it matches neither ARN shape (already-bare
// UUID or alias).
func kmsKeyIDFromField(raw, srcType string) string {
	if raw == "" {
		return ""
	}
	keyID := raw
	if idx := strings.Index(keyID, ":key/"); idx >= 0 {
		keyID = keyID[idx+len(":key/"):]
	} else if idx := strings.Index(keyID, ":alias/"); idx >= 0 {
		keyID = keyID[idx+len(":"):]
	}
	if srcType != "" && keyID == srcType {
		return ""
	}
	return keyID
}

// backupSelectionTagsMatch parses a comma-joined "k=v" selection-tags string
// (as emitted by the backup fetcher's Fields["selection_tags"], one entry per
// BackupSelection.ListOfTags condition) and reports whether any condition
// matches one of the given resource tags. AWS Backup's ListOfTags conditions
// are OR'd together (a resource is selected if ANY condition matches), so a
// single match is sufficient.
func backupSelectionTagsMatch(selectionTagsCSV string, resourceTags map[string]string) bool {
	if selectionTagsCSV == "" || len(resourceTags) == 0 {
		return false
	}
	for cond := range strings.SplitSeq(selectionTagsCSV, ",") {
		key, value, ok := strings.Cut(cond, "=")
		if !ok {
			continue
		}
		if v, present := resourceTags[key]; present && v == value {
			return true
		}
	}
	return false
}

func relatedResult(target string, ids []string) resource.RelatedCheckResult {
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: target, Count: 0}
	}
	set := make(map[string]struct{}, len(ids))
	uniq := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := set[id]; ok {
			continue
		}
		set[id] = struct{}{}
		uniq = append(uniq, id)
	}
	sort.Strings(uniq)
	return resource.RelatedCheckResult{
		TargetType:  target,
		Count:       len(uniq),
		ResourceIDs: uniq,
	}
}

// relatedResultTrunc is relatedResult with the truncation flag carried through
// UNIFORMLY for any count. A truncated scan renders "(N+)" — the "+" means the
// target list is truncated, so navigating shows "m for more". 0+ and 10+ are the
// same case (N found so far, list truncated), not two: there is no special
// zero-truncated result.
func relatedResultTrunc(target string, ids []string, truncated bool) resource.RelatedCheckResult {
	r := relatedResult(target, ids)
	r.Truncated = truncated
	return r
}

// typedRow pairs a cached Resource's ID with its RawStruct already asserted
// to T, so callers of cachedTypedRows never re-assert.
type typedRow[T any] struct {
	ID  string
	Raw T
}

// cachedTypedRows reads the shortName entry directly from cache — it never
// fetches. Tri-state contract (the ng_related.go original): cache absent →
// (nil, false, false) = unknown; entry present but zero rows assert to T
// (disk-seeded, no RawStruct) → (nil, false, false) = unknown; entry present
// and typed → the asserting rows only (non-asserting rows are dropped, as
// every caller previously did itself).
func cachedTypedRows[T any](cache resource.ResourceCache, shortName string) (rows []typedRow[T], truncated bool, ok bool) {
	entry, present := cache[shortName]
	if !present {
		return nil, false, false
	}
	if len(entry.Resources) == 0 {
		return nil, entry.IsTruncated, true
	}
	for _, r := range entry.Resources {
		if raw, asserted := assertStruct[T](r.RawStruct); asserted {
			rows = append(rows, typedRow[T]{ID: r.ID, Raw: raw})
		}
	}
	if len(rows) == 0 {
		return nil, false, false
	}
	return rows, entry.IsTruncated, true
}

// lambdaEventSourceMappingLambdaCheck is shared by checkKinesisLambda and
// checkMSKLambda. Both pivots need the same mechanism: a stream/cluster ARN
// is the Lambda event source, and lambda:ListEventSourceMappings filtered by
// EventSourceArn (one call per open resource — budget rule 7 in
// docs/related-resources.md) is itself the authoritative mechanism per
// kinesis.md/msk.md §2 — its FunctionArn values are the definitive answer,
// and every FunctionArn in the response resolves to exactly one counted ID.
// The already-loaded lambda cache, when present, only enriches: for a
// FunctionArn the cache has, it resolves to the cached Resource.ID (the bare
// function name the lambda drill/detail view navigates by — see
// FetchLambdaFunctionsPageWithEventSources); for any FunctionArn the cache
// lacks (stale/incomplete cache — a cache miss must never drop a
// API-confirmed mapping), the bare name is instead parsed out of the
// FunctionArn (arn:aws:lambda:region:account:function:name[:qualifier]). The
// two resolutions are unioned per-arn, so cache presence never changes the
// count — only which ID string represents an already-cached function.
func lambdaEventSourceMappingLambdaCheck(ctx context.Context, clients any, eventSourceArn string, cache resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Lambda == nil {
		return resource.UnknownRelated("lambda")
	}
	api, ok := c.Lambda.(LambdaListEventSourceMappingsAPI)
	if !ok {
		return resource.UnknownRelated("lambda")
	}

	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*lambda.ListEventSourceMappingsOutput, error) {
		return api.ListEventSourceMappings(ctx, &lambda.ListEventSourceMappingsInput{
			EventSourceArn: aws.String(eventSourceArn),
		})
	})
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}

	functionArns := make(map[string]struct{}, len(out.EventSourceMappings))
	for _, m := range out.EventSourceMappings {
		if m.FunctionArn != nil && *m.FunctionArn != "" {
			functionArns[*m.FunctionArn] = struct{}{}
		}
	}
	if len(functionArns) == 0 {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	arnToID := make(map[string]string, len(functionArns))
	if entry, cacheOK := cache["lambda"]; cacheOK {
		for _, fn := range entry.Resources {
			if arn := fn.Fields["arn"]; arn != "" && fn.ID != "" {
				arnToID[arn] = fn.ID
			}
		}
	}

	ids := make([]string, 0, len(functionArns))
	for arn := range functionArns {
		if id, matched := arnToID[arn]; matched {
			ids = append(ids, id)
			continue
		}
		if name := resource.LambdaNameFromARN(arn); name != "" {
			ids = append(ids, name)
		}
	}
	return relatedResult("lambda", ids)
}

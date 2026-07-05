// Generic helpers shared by every *_related.go checker (moved out of ec2_related.go — they are not EC2-specific).
package aws

import (
	"sort"
	"strings"

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

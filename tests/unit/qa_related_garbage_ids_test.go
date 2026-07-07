package unit_test

// qa_related_garbage_ids_test.go — regression pins for a live-reported
// related-panel bug: opening the detail view over a CACHE-SEEDED s3 row
// (thin fields — bucket_name/creation_date/name/notification_*/region,
// RawStruct nil) fired the s3→kms related check, which returned a bare
// (non-ARN) KMSMasterKeyID value equal to the SOURCE resource's own type
// ("s3") as the navigation key ID. The downstream by-ID fetch then called
// the live KMS API with keyId="s3", which AWS rejected with
// NotFoundException "Invalid keyId 's3'".
//
// The fix (internal/aws/related_common.go's kmsKeyIDFromField(raw, srcType))
// now guards every *_related.go KMS checker registry-wide: an extracted
// keyID equal to the source resource's own type short name is treated as
// fabricated/garbage and dropped rather than handed to DescribeKey/FetchByIDs.
//
// Pin 1 drives checkS3KMS directly against a thin, cache-seeded-shaped s3
// resource with a fake S3 client whose GetBucketEncryption response carries
// exactly this degenerate KMSMasterKeyID shape, and asserts the checker must
// never surface "s3" (the source Resource.Type) as a related ResourceID.
// Passes now that the fix has landed; guards against regression.
//
// Pin 2 generalizes the contract across the FULL related-checker registry
// (resource.GetRelated over every resource.AllShortNames() entry): for every
// registered RelatedDef with a non-nil Checker, driving it against a thin
// seeded-shape resource of that type — with a poisoned cache seeded under
// every target type — must never yield a ResourceID equal to the source
// type's own short name. See the COVERAGE BOUNDARY doc comment on
// TestRelatedRegistry_ThinSeededRow_NeverFabricatesSourceTypeAsID for the
// honest limits of what this sweep can hermetically exercise.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// s3EncryptionGarbageKeyFake returns a KMSMasterKeyID that is NOT an ARN —
// a bare string equal to the value under test. This is the exact live-bug
// shape: no "/" for checkS3KMS's inline ARN-strip to act on, so the bare
// value passes straight through as the returned ResourceID.
type s3EncryptionGarbageKeyFake struct {
	s3NoopAPI
	keyID string
}

func (f *s3EncryptionGarbageKeyFake) GetBucketEncryption(
	_ context.Context, _ *s3.GetBucketEncryptionInput, _ ...func(*s3.Options),
) (*s3.GetBucketEncryptionOutput, error) {
	return &s3.GetBucketEncryptionOutput{
		ServerSideEncryptionConfiguration: &s3types.ServerSideEncryptionConfiguration{
			Rules: []s3types.ServerSideEncryptionRule{
				{
					ApplyServerSideEncryptionByDefault: &s3types.ServerSideEncryptionByDefault{
						SSEAlgorithm:   s3types.ServerSideEncryptionAwsKms,
						KMSMasterKeyID: aws.String(f.keyID),
					},
				},
			},
		},
	}, nil
}

// thinCacheSeededS3Resource mirrors the live-bug shape: a cache-seeded s3 row
// carries only the list-view fields (bucket_name/creation_date/name/
// notification_*/region) with RawStruct nil — no encryption-related field is
// ever populated by the s3 fetcher, so a related check over this row has NO
// legitimate source data to extract a KMS key ID from at all.
func thinCacheSeededS3Resource(bucket string) resource.Resource {
	return resource.Resource{
		ID:   bucket,
		Name: bucket,
		Type: "s3",
		Fields: map[string]string{
			"bucket_name":         bucket,
			"name":                bucket,
			"creation_date":       "2026-01-01T00:00:00Z",
			"notification_lambda": "",
			"notification_sns":    "",
			"notification_sqs":    "",
			"region":              "us-east-1",
		},
		RawStruct: nil,
	}
}

// TestSeededRow_MissingSourceField_NeverFabricatesIDs pins the reported bug:
// driving checkS3KMS against a thin, cache-seeded s3 resource whose
// GetBucketEncryption response carries a bare (non-ARN) KMSMasterKeyID equal
// to the source resource's own Type ("s3") must NOT surface "s3" as a related
// ResourceID. This is the exact seam that fed keyId="s3" into the live KMS
// DescribeKey/GetKeyPolicy call and produced the AWS NotFoundException
// "Invalid keyId 's3'" before kmsKeyIDFromField's srcType guard landed.
func TestSeededRow_MissingSourceField_NeverFabricatesIDs(t *testing.T) {
	const bucket = "thin-seeded-bucket"
	src := thinCacheSeededS3Resource(bucket)

	clients := &awsclient.ServiceClients{S3: &s3EncryptionGarbageKeyFake{keyID: src.Type}}
	checker := s3CheckerByTarget(t, "kms")

	got := checker(context.Background(), clients, src, nil)

	for _, id := range got.ResourceIDs {
		if id == src.Type {
			t.Fatalf(
				"checkS3KMS(bucket=%q) returned ResourceIDs=%v containing %q — "+
					"the SOURCE resource's own Type fabricated as a KMS key ID; "+
					"this is the live-reported bug (AWS NotFoundException "+
					"\"Invalid keyId 's3'\"). RelatedCheckResult=%+v",
				bucket, got.ResourceIDs, id, got,
			)
		}
	}
}

// thinSeededResourceOf builds a minimal, cache-seeded-shaped resource.Resource
// for shortName: ID/Name set to a placeholder identifier, Type set to
// shortName, Fields populated with only a "name" key (the one field every
// resource type's fetcher unconditionally sets), RawStruct nil. This
// reproduces "cache-seeded thin row" for ANY resource type, not just s3.
func thinSeededResourceOf(shortName string) resource.Resource {
	placeholderID := "seeded-" + shortName
	return resource.Resource{
		ID:   placeholderID,
		Name: placeholderID,
		Type: shortName,
		Fields: map[string]string{
			"name": placeholderID,
		},
		RawStruct: nil,
	}
}

// poisonedTargetCache builds a resource.ResourceCache with one synthetic
// candidate resource under EVERY registered target type across the whole
// registry. Each candidate's ID equals its own target type's short name, and
// its Fields carry the same value under every common cache-scan match-key
// used across internal/aws/*_related.go (resource_arn, resources,
// event_pattern, s3website_alias_names, result_output_location, stack_name)
// PLUS the thin source resource's own bare ID/placeholder string — so any
// cache-scan checker whose string-matching logic degrades to a loose/
// unguarded comparison against source data has a real candidate to match
// against, without needing any live AWS client. This lets pure cache-scan
// checkers (checkS3Backup, checkS3Athena, checkS3Glue, checkS3EBRule,
// checkS3R53, checkS3Role's post-policy-parse cache lookup, etc.) actually
// exercise their extraction logic under clients=nil.
func poisonedTargetCache(sourceShortName string, sourceID string) resource.ResourceCache {
	cache := make(resource.ResourceCache, len(resource.AllShortNames()))
	for _, targetType := range resource.AllShortNames() {
		candidateID := targetType
		cache[targetType] = resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:   candidateID,
					Name: candidateID,
					Type: targetType,
					Fields: map[string]string{
						"name":                   candidateID,
						"resource_arn":           "arn:aws:s3:::" + sourceID,
						"resources":              "arn:aws:s3:::" + sourceID,
						"not_resources":          "",
						"event_pattern":          `{"source":["aws.` + sourceShortName + `"],"detail":{"bucket":{"name":["` + sourceID + `"]}}}`,
						"s3website_alias_names":  sourceID,
						"result_output_location": "s3://" + sourceID + "/",
						"stack_name":             sourceID,
					},
				},
			},
			IsTruncated: false,
		}
	}
	return cache
}

// TestRelatedRegistry_ThinSeededRow_NeverFabricatesSourceTypeAsID sweeps
// EVERY registered RelatedDef across the full resource-type registry
// (resource.AllShortNames() x resource.GetRelated) and asserts that driving
// each Checker against a thin, cache-seeded-shaped resource of its OWN source
// type — with a poisonedTargetCache seeded under every target type — never
// yields a ResourceID equal to that source type's own short name.
//
// COVERAGE BOUNDARY (honest disclosure): clients=nil is hermetic — every
// AWS-calling checker in internal/aws guards with
// `c, ok := clients.(*ServiceClients); if !ok || c == nil`, so with nil
// clients those checkers short-circuit to State: RelatedUnknown (or a resolved 0) BEFORE reaching their
// vulnerable ID-extraction logic, regardless of the cache. Of the 605
// registered defs at time of writing, only ~5 pure cache-scan checkers
// (e.g. checkS3Backup/Athena/Glue/EBRule/R53-shaped defs) actually run their
// extraction logic against poisonedTargetCache and get checked for
// fabrication here — the checked/exercisedWithIDs counts logged below report
// the exact number for the current registry. This sweep does NOT re-prove
// the live-API bug class (s3->kms and siblings) that pin 1 pins directly
// with a real fake client — providing per-service fakes for every
// registered checker's AWS client interface is out of scope for this pin.
func TestRelatedRegistry_ThinSeededRow_NeverFabricatesSourceTypeAsID(t *testing.T) {
	checked := 0
	skippedNilChecker := 0
	skippedNoIDs := 0
	exercisedWithIDs := 0

	for _, shortName := range resource.AllShortNames() {
		defs := resource.GetRelated(shortName)
		if len(defs) == 0 {
			continue
		}
		src := thinSeededResourceOf(shortName)
		cache := poisonedTargetCache(shortName, src.ID)

		for _, def := range defs {
			if def.Checker == nil {
				skippedNilChecker++
				continue
			}
			checked++

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf(
							"GetRelated(%q) def TargetType=%q Checker panicked on thin seeded row (ID=%q): %v",
							shortName, def.TargetType, src.ID, r,
						)
					}
				}()

				got := def.Checker(context.Background(), nil, src, cache)

				if len(got.ResourceIDs) == 0 {
					skippedNoIDs++
					return
				}
				exercisedWithIDs++

				for _, id := range got.ResourceIDs {
					if id == shortName {
						t.Errorf(
							"GetRelated(%q) def TargetType=%q DisplayName=%q returned "+
								"ResourceIDs=%v containing %q — the SOURCE resource's own "+
								"short name fabricated as a related-target ID on a thin "+
								"seeded row (no RawStruct, minimal Fields). "+
								"RelatedCheckResult=%+v",
							shortName, def.TargetType, def.DisplayName, got.ResourceIDs, id, got,
						)
					}
				}
			}()
		}
	}

	if checked == 0 {
		t.Fatal("swept zero RelatedDef checkers — resource.AllShortNames()/GetRelated() registry appears empty; Install() may not have run")
	}

	t.Logf(
		"swept %d related checkers across %d resource types (skipped %d nil-Checker defs; "+
			"%d produced no ResourceIDs on the thin seeded row — mostly clients=nil short-circuits "+
			"for live-API checkers; %d actually exercised ID-extraction logic and were checked "+
			"for fabrication — see COVERAGE BOUNDARY doc comment)",
		checked, len(resource.AllShortNames()), skippedNilChecker, skippedNoIDs, exercisedWithIDs,
	)
}

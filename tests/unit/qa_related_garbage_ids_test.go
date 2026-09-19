package unit_test

// A cache-seeded s3 row carries thin fields (name/creation_date/
// notification_*/region, RawStruct nil). A related check over such a row must
// never return the source type's short name ("s3") as a target ID: handed to
// DescribeKey it becomes keyId="s3", which KMS rejects with NotFoundException
// "Invalid keyId 's3'". kmsKeyIDFromField(raw, srcType) in
// core/aws/related_common.go drops such an ID for every KMS checker.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// s3EncryptionGarbageKeyFake returns a KMSMasterKeyID that is not an ARN: with
// no "/" for checkS3KMS's ARN-strip to act on, the bare value passes through as
// the returned ResourceID.
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

// thinCacheSeededS3Resource is a cache-seeded s3 row: only the list-view
// fields (name/creation_date/notification_*/region), RawStruct nil. The s3
// fetcher populates no encryption field, so a related check over this row has
// no source data to extract a KMS key ID from.
func thinCacheSeededS3Resource(bucket string) resource.Resource {
	return resource.Resource{
		ID:   bucket,
		Name: bucket,
		Type: "s3",
		Fields: map[string]string{
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

// checkS3KMS over a thin cache-seeded s3 row whose GetBucketEncryption
// response carries a bare KMSMasterKeyID equal to the source Type ("s3") must
// not surface "s3" as a related ResourceID; KMS rejects keyId="s3" with
// NotFoundException "Invalid keyId 's3'".
func TestSeededRow_MissingSourceField_NeverFabricatesIDs(t *testing.T) {
	const bucket = "thin-seeded-bucket"
	src := thinCacheSeededS3Resource(bucket)

	clients := &awsclient.ServiceClients{S3: &s3EncryptionGarbageKeyFake{keyID: src.Type}}
	checker := s3CheckerByTarget(t, "kms")

	got := checker(context.Background(), clients, src, nil)

	for _, id := range got.ResourceIDs() {
		if id == src.Type {
			t.Fatalf(
				"checkS3KMS(bucket=%q) returned ResourceIDs=%v containing %q — "+
					"the SOURCE resource's own Type fabricated as a KMS key ID; "+
					"this is the live-reported bug (AWS NotFoundException "+
					"\"Invalid keyId 's3'\"). RelatedCheckResult=%+v",
				bucket, got.ResourceIDs(), id, got,
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
// used across core/aws/*_related.go (resource_arn, resources,
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
// every registered RelatedDef (resource.AllShortNames() x resource.GetRelated)
// against a thin, cache-seeded-shaped resource of its own source type, with a
// poisonedTargetCache seeded under every target type, and asserts no
// ResourceID equals that source type's short name.
//
// Coverage boundary: clients=nil is hermetic — every AWS-calling checker in
// core/aws guards with `c, ok := clients.(*ServiceClients); if !ok || c == nil`,
// so it returns State: RelatedUnknown (or a resolved 0) before its ID
// extraction. Only the pure cache-scan checkers (checkS3Backup/Athena/Glue/
// EBRule/R53-shaped defs) run extraction against poisonedTargetCache; the
// logged checked/exercisedWithIDs counts give the number for the current
// registry. An AWS-calling checker needs its own fake client, as
// TestSeededRow_MissingSourceField_NeverFabricatesIDs has for s3→kms.
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

				if len(got.ResourceIDs()) == 0 {
					skippedNoIDs++
					return
				}
				exercisedWithIDs++

				for _, id := range got.ResourceIDs() {
					if id == shortName {
						t.Errorf(
							"GetRelated(%q) def TargetType=%q DisplayName=%q returned "+
								"ResourceIDs=%v containing %q — the SOURCE resource's own "+
								"short name fabricated as a related-target ID on a thin "+
								"seeded row (no RawStruct, minimal Fields). "+
								"RelatedCheckResult=%+v",
							shortName, def.TargetType, def.DisplayName, got.ResourceIDs(), id, got,
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

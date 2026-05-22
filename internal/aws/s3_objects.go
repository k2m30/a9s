// s3_objects.go — fetcher helpers for the S3 object child view. The
// s3_objects child-type catalog entry (Columns / ChildFetcher / Children /
// RelatedContextFromIDs / FieldKeys) lives in catalog_data.go's
// dataChildTypes slice; the init() body that previously lived here was
// migrated in AS-947 / PR #TBD as part of the Wave 2.5 init-leakage sweep.
package aws

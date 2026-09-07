## Fixed

- A sort you set on a resource list now survives leaving the list and coming back. For many column layouts the sort was quietly discarded on re-entry, and the list came back in its default order.
- Sorting a list on its Size or Status column now gives the same order whether the list is showing cached rows or freshly fetched ones. The order used to change under the cursor the moment a fetch landed. This affects RDS instances, DocumentDB and Aurora clusters, ElastiCache, Redshift, DynamoDB tables, log groups, ECR images and S3 objects.
- CloudTrail events sort by their real timestamp again in the column header, and the header arrow now appears on the column being sorted.
- A sorted column keeps its arrow at any width. A column just wide enough for its title, or squeezed by a narrow terminal, used to lose the arrow to the ellipsis, so the header read as unsorted while the rows below it were sorted.

## Changed

- View files in `~/.a9s/views/` carry a `generated:` stamp. When a9s ships new columns for a resource type, an existing file gets them added on the next launch, in place: the columns, widths, paths and keys you set are kept, though the file is rewritten from its parsed form, so YAML comments do not survive. Files already at the current stamp are never touched.

## Fixed

- A CloudTrail event list whose rows came from the on-disk cache no longer
  leaves the column the attention marker sits on blank, and sorting by that
  column now orders those rows instead of leaving them in arrival order.
- Sorting a child list by one of its columns now works. Child types resolved
  no columns at all through the controller, so the column a sort named was
  never found and the sort was silently dropped.
- Sorting by a date or a size column now orders by the value rather than by
  the text on screen, so April no longer sorts before March and 9 KB no
  longer sorts after 10 MB. Affects the event time, status, size, duration
  and memory columns of CloudTrail events, RDS clusters and instances,
  Redis, Redshift, DynamoDB, ECR images, log groups, S3 objects and Lambda
  invocations. Existing view files in `~/.a9s/views/` are never overwritten;
  delete the file for a resource type to pick up its corrected column.

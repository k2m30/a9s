## Fixed

- A CloudTrail event list whose rows came from the on-disk cache no longer
  leaves the column the attention marker sits on blank, and sorting by that
  column now orders those rows instead of leaving them in arrival order.
- Sorting a child list by one of its columns now works. Child types resolved
  no columns at all through the controller, so the column a sort named was
  never found and the sort was silently dropped.

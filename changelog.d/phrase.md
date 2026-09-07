## Fixed

- A resource with two failed pipeline stages, two access keys past rotation, or
  two containers that exited non-zero now names all of them. The status column
  reads the condition once and every offending item is a supporting row, where
  before the first item's wording stood for the whole resource and the rest were
  dropped.
- A transit gateway with several failed attachments lists them all instead of
  only the worst one, and a node group or EKS cluster lists every health issue
  AWS reports instead of promoting the first into the status column.
- The status column, the detail view and the attention-signals page now agree on
  what a finding says: the wording is registered once with the finding and the
  emitters read it from there.

## Added

- An expired certificate, an EventBridge rule with no targets, and a target
  group whose targets are all unhealthy each carry their own finding now, so the
  status column says which of them it is and the row's colour matches.
- A CloudTrail event says why it is flagged under its own finding: a failed
  call, a destructive call, a modifying call, root activity, cross-account
  access, or a read of sensitive data.

## Changed

- Sixteen detail rows now read as words instead of an AWS constant: a
  certificate's renewal eligibility, an alarm's comparison operator, an auto
  scaling group's health check type, a DynamoDB table's status, an ECS
  service's scheduling strategy, an ECS task's connectivity, five KMS key
  fields (manager, spec, state, usage and origin), a log group's data
  protection status and class, an Airflow environment's status, a pipeline's
  execution mode, and a target group's protocol version. A key waiting to be
  deleted reads `pending deletion`.

- The columns a resource type shows are written down once. They used to be
  written twice, and the two copies had drifted apart, so which one a change
  reached depended on which file it was made in. Every list looks exactly as
  it did: same columns, same order, same widths. The view files under
  `.a9s/views/` now also record which field each cell reads, including every
  status column, which used to be matched to a field by its heading alone. A
  view file written by an earlier version picks all of that up on the next
  start, and anything you changed in it stays yours.

## Fixed

- A view file you edited keeps working when you rename its status column. The
  key `status`, which older versions accepted on any type, is migrated to the
  key that type actually uses, so the column still shows the finding phrase and
  still colours the row. A key that names nothing at all is now reported when
  a9s starts, once, naming the file and the key — and the rest of your file is
  used as written. The web server used to discard the whole file over one such
  line; it keeps it now, like the terminal app always did.

- A resource whose status the list computes now shows that status, not another
  value stored beside it. A target group whose own column said `unhealthy
  targets: 2/5` displayed `available`, and a build project reading `last build
  failed` displayed `succeeded`. The cell now reads the field its column names
  and nothing else, live and after a restart, so it says the same thing on both.

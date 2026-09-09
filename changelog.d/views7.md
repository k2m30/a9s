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
  used as written. The web UI shows that report on the page, where the terminal
  shows its own; it used to discard your whole file over one such line and
  write a message only to the log. The message no longer claims defaults are in
  use when your file is.

- Five more ways an edited view file was not read the way it was written. A key
  you typed on a column the built-in view leaves blank is kept instead of being
  taken for one the upgrade moved, and the rest of that file is no longer
  replaced along with it. A status column you renamed keeps working: the upgrade
  maps it to the field the type now uses. Files for the views you drill into,
  like a target group's health checks, are upgraded like any other. A file named
  `EC2.yaml` is used for EC2, and one named after nothing at all is reported at
  startup instead of being ignored in silence. And a column of your own that
  reads an AWS field by path is no longer reported as unfillable — it renders,
  live and after a restart.

- Two view files in one directory naming the same resource type are reported at
  startup, saying which of them is the one on screen. One of them is used whole
  rather than half of each.

- A resource whose status the list computes now shows that status, not another
  value stored beside it. A target group whose own column said `unhealthy
  targets: 2/5` displayed `available`, and a build project reading `last build
  failed` displayed `succeeded`. The cell now reads the field its column names
  and nothing else, live and after a restart, so it says the same thing on both.

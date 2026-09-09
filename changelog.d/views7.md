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
  `.a9s/views/` now also record which field each cell reads, including the
  fifteen status columns that used to be matched to a field by their heading
  alone. A view file written by an earlier version picks all of that up on the
  next start, and anything you changed in it stays yours.

## Added

- A web ACL attached to nothing now says `not associated with any resource`
  instead of borrowing the logging finding's wording, so the two conditions
  read as the two problems they are.
- Target-health rows in a target group's child view now carry a finding and a
  colour: an unhealthy or unreachable target no longer renders green with its
  reason in a plain cell.

## Changed

- A stopped ECS task now reads its stop code in plain English —
  `stopped: essential container exited` rather than
  `stopped: EssentialContainerExited` — and the raw identifier is stated once,
  in the finding's supporting row.
- A disabled AMI now reads `disabled` and a deregistered one `deregistered`
  under one signal, rather than the doc promising only the second.
- An Aurora or DocumentDB cluster in a status a9s does not recognise now reads
  `<status>: in progress` like every other transitional status, instead of the
  bare keyword.

## Fixed

- Every Wave-1 finding is now built in one place and reads its wording from
  the signal catalog, so the text on a row, the sentence in its detail view
  and the generated signals page can no longer drift apart. A value AWS
  supplies that contains an angle bracket no longer swallows the rest of the
  phrase.

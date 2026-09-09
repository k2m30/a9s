## Fixed

- A row the attention checks could not inspect now names the check that did not answer: its detail view reads "not inspected: DescribeInstanceStatus" instead of only saying that something refused.
- A drill and the list it was opened from now order their own refreshes separately. They shared one counter, so each superseded the other's requests and a refreshed drill could revert to the rows the refresh replaced.
- A child list is drawn from the same resource definition as the rest of its screen: it shows its own title instead of an internal short name, and a finding on a child row now colours it.
- The main menu no longer stalls behind a cache write. A large type file was encoded while holding a lock the screen needs, so one key press per write waited out the whole encode.
- `--demo` no longer forces the disk cache off. The demo now runs the same cache-load-then-background-sweep path an installation runs, which is how the menu's issue badges are written.

## Removed

- The new-since-last-scan finding counts, which were computed and stored on every cache write and read by nothing. The first-seen timestamps themselves are unchanged and still persisted.

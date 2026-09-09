## Fixed

- A row the attention checks could not inspect now names the check that did not answer: its detail view reads "not inspected: DescribeInstanceStatus" instead of only saying that something refused.
- A drill and the list it was opened from now order their own refreshes separately. They shared one counter, so each superseded the other's requests and a refreshed drill could revert to the rows the refresh replaced.
- A child list is drawn from the same resource definition as the rest of its screen: it shows its own title instead of an internal short name, and a finding on a child row now colours it.
- The main menu no longer stalls behind a cache write. A large type file was encoded while holding a lock the screen needs, so one key press per write waited out the whole encode.
- A failed page now reaches the list that asked for it. It carried neither the screen nor the request number its successful twin carried, so it was applied to whichever list of that type was on top and a failure a later refresh had already replaced could not be recognised as stale.
- An ECS task whose definition came back without its definition now says which call answered short, instead of reporting "no reason given" about a call a9s can name.
- A filtered drill and a child list now receive only their own pages. Like the failed page above, they answered without naming the screen that asked, so a page for the drill beneath landed on the one on top.
- A page that names no screen now reaches none, instead of being applied to whichever list of that type was on top. Every fetch carries the identity of the screen that dispatched it, including the one a navigation builds for the list it is opening, so nothing lands on a screen that did not ask for it.
- A refresh that a newer one has already superseded no longer clears the newer one's spinner. Pressing Ctrl+R twice reported that loading was over while the second fetch was still running; each list now records which request raised its own marker.
- A snapshot whose share attributes came back without the attributes result no longer reads as "checked, not shared publicly". The same silence is closed for a node group whose launch template could not be read, a hosted zone whose record sets were refused, an AS2 agreement whose partner profile did not resolve, and the Auto Scaling and Secrets Manager panels that reported a confident count from a call that did not answer.
- `--demo` no longer forces the disk cache off. The demo now runs the same cache-load-then-background-sweep path an installation runs, which is how the menu's issue badges are written.

## Removed

- The new-since-last-scan finding counts, which were computed and stored on every cache write and read by nothing. The first-seen timestamps themselves are unchanged and still persisted.

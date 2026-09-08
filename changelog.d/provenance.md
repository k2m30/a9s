## Fixed

- Saving a list to the on-disk cache no longer happens in the latency of the
  fetch that produced it: absorbing 6000 rows returns in about 20 ms instead
  of 65.
- Two browser tabs reading the same session no longer take turns: a page view
  that changes nothing takes a read lock.
- A large list no longer freezes the interface while it lands. Absorbing a
  6000-row result blocked every key press for about 50 ms and a 12000-row one
  for about 80; the wait is now a few milliseconds and no longer grows with
  the number of rows.
- A list that could not finish loading no longer records its rows as the
  complete set of that type, and no longer offers "more" when there is none.
- A page loaded on one list no longer lands on another list of the same type
  opened on top of it. Each list screen now owns the fetches it starts.
- A refresh started while an earlier one is still running no longer clears the
  "refreshing" marker when the earlier one returns.
- A failed fetch for a type opened under an alias now reaches its own screen
  instead of leaving it loading for ever, and a failure that a newer request
  has already replaced no longer marks the screen.
- A list whose first page came back incomplete is no longer recorded as the
  type's complete population when a later page reports there are no more.
- Switching away from a profile and back no longer lets a save prepared before
  the switch write the other profile's numbers into this one's cache.
- A background sweep's saved snapshot no longer overwrites newer rows that
  arrived while it was queued.
- Cost anomaly marks now follow the range on screen: zooming out past the
  cached range shows the marks that cover it, with the partial warning,
  instead of showing none.
- A closed month whose costs came from a capped fetch is now repaired by the
  next complete fetch instead of being re-fetched every time the screen opens.
- A Lambda whose permission check could not be completed is reported as
  unchecked rather than healthy.
- The cached row count for a type no longer flips between "exact" and
  "at least" depending on which of two background saves happened to finish
  first. A restart now reads the same number the session ended with.
- A cached type file is no longer briefly written without its rows, so a
  restart during a refresh no longer opens the list empty.
- Load-more and Ctrl+R on a related drill now act on the drill, not on the
  list underneath it. A refreshed drill keeps showing what it found instead of
  the type's whole list, and its title counts the drill, not the account. The
  issue count beside that title counts the drill too, so a refreshed drill no
  longer reports the account's issue total over the rows it is showing.
- A page cap inside one check no longer hides the findings the same check
  already established: the capped number is marked with a "+", and the
  resource keeps its other answers. Affects SNS subscriptions, CodeArtifact
  packages, API Gateway stages, EFS mount targets and EventBridge rule
  targets.
- The API Gateway list and detail now show the API ID, protocol and endpoint
  for REST APIs, which showed a dash before.
- The main menu's issue badge no longer restores a confident "0 issues" for a
  type whose deeper checks never ran in the previous session.
- Two failures on one related check now read as one sentence in the status
  line instead of overwriting each other.
- A failed profile read is phrased the same way wherever it happens.
- Cost anomaly marks and cost totals now say when the data behind them is
  partial rather than presenting a gap as a number.
- Two profiles whose names differ only by a slash or a space no longer share
  one cost cache file.
- A Lambda whose function was deleted mid-check is no longer reported as a
  function with no resource policy.
- ECS task rows show the short task id instead of the full ARN.

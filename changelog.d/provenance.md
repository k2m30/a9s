## Fixed

- A list that could not finish loading no longer records its rows as the
  complete set of that type, and no longer offers "more" when there is none.
- Load-more and Ctrl+R on a related drill now act on the drill, not on the
  list underneath it. A refreshed drill keeps showing what it found instead of
  the type's whole list, and its title counts the drill, not the account.
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

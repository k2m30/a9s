### Fixed

- A cached main menu loaded for one profile could briefly seed the screen of another. On startup the app resolves a region from the local AWS config so the cache can render before the connection settles, but it kept that resolution to itself, so until the connection landed there was nothing to compare an incoming cache read against. The resolved profile and region are now the session's from the moment the cache is read, and a read that answers for any other pair is discarded.
- The main menu's saved counts and issue badges could go backwards. Two places wrote them: the background scan, from the rows the session had actually observed, and the menu itself, from a copy of what it was showing when the save was queued. The menu's copy could land last and put the older numbers back. Both now write the same answer, derived from the observed rows.
- A scan failure for a type reachable under two names (`rds` and `dbi` are the same type) could raise its banner and log its error twice in one sweep. One type is now one entry per sweep whichever name the result arrives under.

### Changed

- `make test-race` allows 900 seconds instead of 300. Under the race detector the unit tests take around 200 seconds on an idle machine, so the old ceiling reported a timeout whenever the machine was busy.

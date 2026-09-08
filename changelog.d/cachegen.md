## Fixed

- Cached counts and rows loaded for one profile or region no longer land on another's menu when you switch while the load is still in flight, and a cache write prepared for one pair is never written into the other's directory.
- Two profiles whose names differ only in a slash, a backslash or a space no longer share one cache directory and overwrite each other's answers.
- A Wave-2 check that was denied, timed out, or could not inspect a row no longer wipes the findings already on screen or persists those rows as clean; it now replaces only what it actually answered, and the row keeps its finding across a later list refresh too.
- A late cache load no longer regresses a count, an issue badge or a list the current session already verified, including a live answer of "none".
- Two cache writes for the same resource type can no longer land out of order: the older one is skipped instead of overwriting the newer state on the next start.
- Loading a resource list no longer holds the app while its cache file is written, so keys and web requests stay responsive on a slow filesystem.
- An issue badge now drops to zero once every issue for that type is fixed and re-verified, instead of showing and persisting the old count for the rest of the session.
- The issue badge on the menu and the one saved for the next launch are now the same number, so restarting no longer changes a badge that nothing in the account answered for.
- A Wave-2 warning is no longer saved as an issue, so a restart cannot show an issue badge the previous screen never had.
- A warm list of a type with more resources than the stored page now re-verifies to the full known total instead of stopping at the stored rows and falling back to "50+".
- Re-entering a profile or region you already visited this session verifies it again, instead of showing the values on disk as if they had just been checked.
- A cache file written under a resource type's alias name now supplies that type's rows, not only its count.
- A cached type known to be empty renders as an empty list and can still be opened; only a type verified empty this session is dimmed shut.

## Fixed

- Errors now reach the `!` log wherever they happen. A failure that raised a banner in the terminal but left no trace in the web session log is recorded once for both, and the web error log shows the same entries.
- A list no longer re-orders itself when a fetch lands over cached rows. A column sorts the same whether the rows came from the cache or from AWS.
- The detail cursor stays on the finding you are reading when a new one arrives above it, instead of jumping back to the Attention heading.

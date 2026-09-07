## Fixed

- Switching profile or region no longer leaves menu rows claiming they were verified under the previous account: every per-type fact the old pair established is dropped with the counts.
- A failed enrichment now flashes what went wrong instead of the raw AWS error, so a request id, a host id and an encoded authorization message no longer reach the status bar.
- A service the selected region does not offer now says `no service` on its menu row instead of reading like a network failure.
- A menu row whose probe timed out now says `timeout`, and one that could not reach the service says `transport`, instead of both reading a bare `error` while the log said otherwise.
- A timed-out or unreachable-endpoint failure reads as its cause; the `operation error <Service>: <Op>` preamble is gone from every error class, not just from API errors.

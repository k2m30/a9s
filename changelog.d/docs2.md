## Changed

- The per-resource design docs now quote only Status text a9s actually shows. The "At 3am" paragraph in each doc named phrases no finding produces (`issuance failed` for certificates, `no data` for alarms, `CRITICAL CVEs in latest` for repositories); every one is now the phrase the list renders, under the colour the row actually takes.
- The related-panel section of each design doc now lists exactly the pivots the panel builds. Blocks describing pivots a9s does not register — target groups to backups, endpoints to certificates, buckets to WAF — have been removed, and the reason each is excluded is kept where the other exclusions live.
- References to the related-resources contract now name a heading instead of a line number, so a reader following one lands on the section rather than on whatever moved into that line.

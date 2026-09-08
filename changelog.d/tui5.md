## Fixed

- A value a fetcher writes by hand now reads the same everywhere it appears. A boolean shows as Yes or No and a timestamp as a plain date and time in the detail view, not only in the list, and typing what you see into the filter finds the row.
- The ECS task Stop Code and the NAT gateway Failure columns show a readable cause instead of the raw AWS constant.
- Upgrading now brings a corrected column order to an installation you have already run, not only to a fresh one. A view file you have edited keeps the order you gave it.
- The profile selector explains a failure to read the local AWS config instead of showing the raw error text.

## Removed

- The unused row-marker glyph plumbing in the list. A row's colour already carries its worst finding, so no marker was ever produced.

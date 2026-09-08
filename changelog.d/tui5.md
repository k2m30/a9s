## Fixed

- A value now reads the same everywhere it appears. A boolean shows as Yes or No and a timestamp as a date and time, in the detail view as well as the list, whether AWS reported it as a real bool and time or as text. A date AWS reports without a time of day, such as when a secret was last accessed or when an AMI is deprecated, no longer gains a midnight that was never measured.
- The filter matches what is on screen. Typing the words a column shows finds the row, and a raw AWS constant that appears nowhere finds nothing.
- The ECS task Stop Code and the NAT gateway Failure show a readable cause instead of the raw AWS constant, in the detail view as well as the list.
- A CloudTrail event's RAW EVENT block is the event exactly as AWS sent it, down to the last line. The sections above it read in a9s's own words.
- Upgrading now brings a corrected column order, and corrected columns, to an installation you have already run rather than only to a fresh one. A view file you have edited keeps the order you gave it, and a column you changed keeps what you set while the rest of the correction still reaches you.
- The profile selector explains a failure to read the local AWS config instead of showing the raw error text.

## Removed

- The unused row-marker glyph plumbing in the list. A row's colour already carries its worst finding, so no marker was ever produced.

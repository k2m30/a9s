## Added

- An EC2 instance behind a security group that admits every protocol from the internet now says "every port reachable from the internet" as its own signal, instead of listing a port called "all".

## Fixed

- An internet-exposed EC2 instance now reads "port 22" or "ports 22, 3389" instead of the literal "port(s)".
- Deleted and unconfirmed subscriptions under one SNS topic are now separate rows. They shared one identity before, so paging a topic's subscriptions kept only the first of them.
- A detail field name wider than the terminal no longer pushes its value off the right edge; the label column now takes at most two fifths of the width.
- An error in the header no longer takes the profile and region off it. The message is cut to the room that remains.
- A demo Redshift cluster named for an expired maintenance window, which reports nothing, is now named for what it shows: a lapsed deferral.

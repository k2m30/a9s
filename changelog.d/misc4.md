## Added

- An EC2 instance behind a security group that admits every protocol from the internet now says "every port reachable from the internet" as its own signal, instead of listing a port called "all".

## Changed

- The CloudTrail events list shows TIME before Status, and the load balancer list shows DNS Name second, matching the order the built-in lists already used.

## Fixed

- An internet-exposed EC2 instance now reads "port 22" or "ports 22, 3389" instead of the literal "port(s)".
- Deleted and unconfirmed SNS subscriptions are now separate rows on both the topic's subscription list and the account-wide one. They shared one identity before, so paging a list kept only the first of them.
- The Subscription ARN column is now empty for a subscription that has none, instead of showing the word "PendingConfirmation" under a heading that promises an identifier.
- A list cell now reads the same whether it came from a live fetch or a warm cache. A yes/no value said "true" down one path and "Yes" down the other, and a timestamp showed the raw stamp or the readable one, depending on which.
- The CloudWatch alarm Threshold, ECS task Task ID, CloudWatch Logs Retention, EKS Node Group name and Secrets Manager timestamps now show the same value the rest of a9s shows for them. The Task ID column shows the ID rather than the full task ARN, and the secret timestamps show the date Secrets Manager reports without a time of day it does not.
- A detail field name wider than the terminal no longer pushes its value off the right edge; the label column now takes at most two fifths of the width.
- An error in the header no longer takes the profile and region off it. The message is cut to the room that remains.
- A demo Redshift cluster named for an expired maintenance window, which reports nothing, is now named for what it shows: a lapsed deferral.

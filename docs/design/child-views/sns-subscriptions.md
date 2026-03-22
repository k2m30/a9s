# Child View: SNS Topics --> Subscriptions

**Status:** Planned
**Tier:** SHOULD-HAVE

---

## Navigation

- **Entry:** Press Enter on an SNS topic in the SNS Topics list
- **Frame title:** `sns-subs(5) — critical-alerts-prod`
- **View stack:** SNS Topics --> Subscriptions --> (detail/YAML via d/y)
- **Esc** returns to SNS Topics list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
sns_subscriptions:
  list:
    Protocol:
      path: Protocol
      width: 10
    Endpoint:
      path: Endpoint
      width: 48
    Status:
      key: confirmation_status
      width: 18
    Owner:
      path: Owner
      width: 14
  detail:
    - SubscriptionArn
    - Protocol
    - Endpoint
    - Owner
    - TopicArn
```

Note on computed fields:
- `confirmation_status`: "Confirmed" if `SubscriptionArn` is a real ARN, "PendingConfirmation" if `SubscriptionArn` equals `"PendingConfirmation"`. This is how AWS signals unconfirmed subscriptions — through the ARN field itself.

Source struct: `snstypes.Subscription`

## AWS API

- `sns:ListSubscriptionsByTopic` with `TopicArn`
- Paginated via `NextToken`
- **Latency:** Fast (<1 second). Topics typically have 1-20 subscriptions.
- **Note:** Endpoint values may be partially obscured by AWS for security (email addresses show as `***@example.com` unless the caller owns the subscription). This is an AWS-side behavior, not something a9s controls.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────── sns-subs(5) — critical-alerts-prod ──────────────────────────────┐
│ PROTOCOL   ENDPOINT                                         STATUS             … │
│ email      oncall-team@company.com                           Confirmed           │
│ email      platform-lead@company.com                         Confirmed           │
│ https      https://hooks.slack.com/services/T01/B02/xyz      Confirmed           │
│ lambda     arn:aws:lambda:us-east-1:123456:function:alert…   Confirmed           │
│ sqs        arn:aws:sqs:us-east-1:123456:alert-dead-letter    PendingConfirmation │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by confirmation status (entire row):
- `Confirmed`: GREEN `#9ece6a`
- `PendingConfirmation`: YELLOW `#e0af68` — this is the "why isn't Slack getting alerts?" answer

Selected row: full-width blue background overrides status coloring.

## Copy Behavior

`c` copies the Endpoint value — the email address, URL, or ARN of the subscription target. This is what you need to verify or cross-reference.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ SUBSCRIPTIONS         GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Endpoint                      <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

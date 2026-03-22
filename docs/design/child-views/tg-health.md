# Child View: Target Groups --> Target Health

**Status:** Planned
**Tier:** MUST-HAVE

---

## Navigation

- **Entry:** Press Enter on a target group in the Target Groups list
- **Frame title:** `tg-health(4) — api-prod-tg`
- **View stack:** Target Groups --> Target Health --> (detail/YAML via d/y)
- **Esc** returns to Target Groups list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
tg_health:
  list:
    Target ID:
      path: Target.Id
      width: 24
    Port:
      path: Target.Port
      width: 8
    AZ:
      path: Target.AvailabilityZone
      width: 14
    Health:
      path: TargetHealth.State
      width: 14
    Reason:
      path: TargetHealth.Reason
      width: 28
    Description:
      path: TargetHealth.Description
      width: 36
  detail:
    - Target.Id
    - Target.Port
    - Target.AvailabilityZone
    - TargetHealth.State
    - TargetHealth.Reason
    - TargetHealth.Description
    - HealthCheckPort
    - AnomalyDetection
```

Source struct: `elbtypes.TargetHealthDescription`

## AWS API

- `elasticloadbalancingv2:DescribeTargetHealth` with `TargetGroupArn`
- **No pagination** — returns all targets at once (typically < 100)
- **Latency:** Fast (<1 second). Single API call with immediate response.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────────────── tg-health(4) — api-prod-tg ──────────────────────────────┐
│ TARGET ID                PORT     AZ             HEALTH         REASON           │
│ i-0a1b2c3d4e5f67890      8080     us-east-1a     healthy        —                │
│ i-0f9e8d7c6b5a43210      8080     us-east-1b     healthy        —                │
│ i-0112233445566778a      8080     us-east-1a     unhealthy      Target.Timeout…  │
│ i-0aabbccddeeff0011      8080     us-east-1c     draining       Target.Deregis…  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

With Description column scrolled into view (via `l` key):
```
│ HEALTH         REASON                       DESCRIPTION                          │
│ healthy        —                            —                                    │
│ healthy        —                            —                                    │
│ unhealthy      Target.FailedHealthChecks    Health checks failed                 │
│ draining       Target.DeregistrationInProg  Target deregistration is in progre…  │
```

IP-type target group example:
```
│ TARGET ID                PORT     AZ             HEALTH         REASON           │
│ 10.0.1.47                8080     us-east-1a     healthy        —                │
│ 10.0.2.83                8080     us-east-1b     healthy        —                │
│ 10.0.1.122               8080     us-east-1a     initial        Elb.Registratio… │
│ 10.0.3.91                8080     us-east-1c     unavailable    Target.InvalidS… │
```

Row coloring by health state (entire row):
- `healthy`: GREEN `#9ece6a`
- `unhealthy`: RED `#f7768e`
- `draining`: YELLOW `#e0af68`
- `initial`: YELLOW `#e0af68`
- `unavailable`: RED `#f7768e`
- `unused`: DIM `#565f89`

Selected row: full-width blue background overrides health-state coloring.

## Copy Behavior

`c` copies the Target ID (instance ID or IP address) — the identifier you need to cross-reference with EC2, ECS, or other services.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ TARGET HEALTH         GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Target                        <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

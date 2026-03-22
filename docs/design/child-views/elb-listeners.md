# Child View: Load Balancers --> Listeners --> Listener Rules

**Status:** Planned
**Tier:** MUST-HAVE (Listeners) + Structural (Rules)

---

## Level 1: Load Balancers --> Listeners

### Navigation

- **Entry:** Press Enter on a load balancer in the ELB list
- **Frame title:** `elb-listeners(3) — api-prod-alb`
- **View stack:** ELB --> Listeners --> (Rules via Enter, detail/YAML via d/y)
- **Esc** returns to ELB list
- **No new key bindings** beyond the standard set

### views.yaml

```yaml
elb_listeners:
  list:
    Port:
      path: Port
      width: 8
    Protocol:
      path: Protocol
      width: 10
    Action:
      key: default_action_type
      width: 16
    Target:
      key: default_action_target
      width: 32
    SSL Policy:
      path: SslPolicy
      width: 24
    Certificate:
      key: certificate_short
      width: 32
  detail:
    - ListenerArn
    - Port
    - Protocol
    - DefaultActions
    - SslPolicy
    - Certificates
    - AlpnPolicy
    - MutualAuthentication
```

Note on computed fields:
- `default_action_type`: extracted from `DefaultActions[0].Type` (forward, redirect, fixed-response, authenticate-cognito, authenticate-oidc)
- `default_action_target`: for forward actions, the target group name extracted from `DefaultActions[0].ForwardConfig.TargetGroups[0].TargetGroupArn` or `DefaultActions[0].TargetGroupArn`; for redirect actions, the redirect URL; for fixed-response, the status code
- `certificate_short`: the ACM certificate domain name extracted from `Certificates[0].CertificateArn` (e.g., `*.example.com` from the full ARN, requires an additional `acm:DescribeCertificate` call or cross-reference with parent cert data)

Source struct: `elbtypes.Listener`

### AWS API

- `elasticloadbalancingv2:DescribeListeners` with `LoadBalancerArn`
- **No pagination needed** — most ALBs have 2-5 listeners (HTTP, HTTPS, maybe a few more)
- **Latency:** Fast (<1 second)

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────────── elb-listeners(3) — api-prod-alb ─────────────────────────────┐
│ PORT     PROTOCOL   ACTION           TARGET                           SSL POLI… │
│ 443      HTTPS      forward          api-prod-tg                      ELBSecur… │
│ 80       HTTP       redirect         https://#{host}:443/#{path}?#{…  —         │
│ 8443     HTTPS      forward          api-internal-tg                  ELBSecur… │
└─────────────────────────────────────────────────────────────────────────────────┘
```

No status-based row coloring — listeners do not have health/status semantics. All rows are PLAIN `#c0caf5`.

Selected row: full-width blue background.

### Copy Behavior

`c` copies the Listener ARN.

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ LISTENERS             GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <enter> View Rules    <q>      Quit        <k>       Up         <:>   Command   │
│ <d>     Detail        </>      Filter      <g>       Top                        │
│ <y>     YAML          <:>      Command     <G>       Bottom                     │
│ <c>     Copy ARN                           <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Level 2: Listeners --> Listener Rules

### Navigation

- **Entry:** Press Enter on a listener in the Listeners list
- **Frame title:** `listener-rules(5) — :443 HTTPS`
- **View stack:** ELB --> Listeners --> Rules --> (detail/YAML via d/y)
- **Esc** returns to Listeners list
- **No new key bindings** beyond the standard set

### views.yaml

```yaml
elb_listener_rules:
  list:
    Priority:
      path: Priority
      width: 10
    Conditions:
      key: conditions_summary
      width: 36
    Action:
      key: action_type
      width: 16
    Target:
      key: action_target
      width: 32
  detail:
    - RuleArn
    - Priority
    - Conditions
    - Actions
    - IsDefault
```

Note on computed fields:
- `conditions_summary`: human-readable summary of `Conditions[]` (e.g., `path: /api/v2/*` or `host: api.example.com` or `path: /health AND header: X-Custom=true`)
- `action_type`: extracted from `Actions[0].Type`
- `action_target`: for forward actions, the target group name; for redirect actions, the redirect URL; for fixed-response, the status code + content type

Source struct: `elbtypes.Rule`

### AWS API

- `elasticloadbalancingv2:DescribeRules` with `ListenerArn`
- **No pagination needed** — max 100 rules per listener (AWS limit), typically 5-20
- **Latency:** Fast (<1 second)

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────────── listener-rules(5) — :443 HTTPS ──────────────────────────────┐
│ PRIORITY   CONDITIONS                           ACTION           TARGET         │
│ 1          path: /api/v1/*                       forward          api-v1-tg     │
│ 2          path: /api/v2/*                       forward          api-v2-tg     │
│ 3          host: admin.example.com               forward          admin-tg      │
│ 4          path: /health                         fixed-response   200 text/pl…  │
│ default    —                                     forward          api-prod-tg   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

No status-based row coloring. All rows PLAIN `#c0caf5`. The `default` rule row is DIM `#565f89` to visually distinguish it.

Selected row: full-width blue background.

### Copy Behavior

`c` copies the conditions summary (the routing rule) — what you want to share when debugging routing issues.

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ LISTENER RULES        GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Rule                          <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

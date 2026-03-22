# Child View: Route 53 Hosted Zones → Records

**Status:** Implemented
**Tier:** MUST-HAVE

## Navigation

- **Entry:** Press Enter on a hosted zone in the Route 53 zone list
- **Frame title:** `r53-records(N) — example.com.`
- **View stack:** Route 53 Zones → R53 Records → (detail/YAML via d/y)
- **No special behavior:** flat list, no further drill-down.

## views.yaml

```yaml
r53_records:
  list:
    Name:
      path: Name
      width: 40
    Type:
      path: Type
      width: 8
    TTL:
      path: TTL
      width: 8
    Values:
      key: values
      width: 50
  detail:
    - Name
    - Type
    - TTL
    - ResourceRecords
    - AliasTarget
    - SetIdentifier
    - Weight
    - Region
    - Failover
    - GeoLocation
    - HealthCheckId
    - MultiValueAnswer
```

## views_reference.yaml

Source struct: `r53types.ResourceRecordSet`

```
- Name
- Type
- AliasTarget.DNSName
- AliasTarget.EvaluateTargetHealth
- AliasTarget.HostedZoneId
- CidrRoutingConfig.CollectionId
- CidrRoutingConfig.LocationName
- Failover
- GeoLocation.ContinentCode
- GeoLocation.CountryCode
- GeoLocation.SubdivisionCode
- GeoProximityLocation.AWSRegion
- GeoProximityLocation.Bias
- GeoProximityLocation.Coordinates.Latitude
- GeoProximityLocation.Coordinates.Longitude
- GeoProximityLocation.LocalZoneGroup
- HealthCheckId
- MultiValueAnswer
- Region
- ResourceRecords[].Value
- SetIdentifier
- TTL
- TrafficPolicyInstanceId
- Weight
```

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────────── r53-records(24) — example.com. ───────────────────────────────┐
│ NAME                                    TYPE     TTL      VALUES                   │
│ example.com.                            A        300      203.0.113.10             │
│ example.com.                            MX       3600     10 mail.example.com.     │
│ api.example.com.                        CNAME    300      alb-prod-123.elb.ama...  │
│ *.example.com.                          A        —        ALIAS d111111.cf.net.    │
│ _acme-challenge.example.com.            TXT      60       "gBz1p..."              │
│   · · · (19 more)                                                                 │
└────────────────────────────────────────────────────────────────────────────────────┘
```

ALIAS records show `—` for TTL and `ALIAS <target>` in Values.

## AWS API

- `route53:ListResourceRecordSets` — paginated via `NextRecordName` + `NextRecordType`
- Called with `HostedZoneId` from the parent zone

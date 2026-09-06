# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Volumes, databases, clusters and tables that no backup plan selects now say
  so. The check joins each resource's ARN against the backup plans already
  loaded, and it only answers from a plan list read to the end: a list nobody
  fetched, or one cut short, reports nothing rather than calling a resource
  unprotected on the strength of a page nobody read. A plan that selects by tag
  is matched too: volumes carry their tags in the list already, and for
  databases, clusters and tables the tags are read one resource at a time, only
  where a plan's ARNs have not already covered them. A resource whose tags
  cannot be read is left alone rather than called unprotected. An attached
  volume with no snapshot behind it is flagged too. Closes Prowler `ec2_ebs_volume_protected_by_backup_plan`,
  `ec2_ebs_volume_snapshots_exists`, `rds_instance_protected_by_backup_plan`,
  `rds_cluster_protected_by_backup_plan` and
  `dynamodb_table_protected_by_backup_plan`.

- Listing backup plans without a Backup client configured returns an empty page
  instead of crashing.
- CloudFormation stacks now flag termination protection being off, and a
  credential pasted into a stack output. Stack status also reads in words
  (`update rollback complete`) instead of the raw status keyword.
- CodeBuild projects now flag build results being publicly visible, a buildspec
  taken from the source repository, a credential embedded in the source
  repository address, and a credential in a plaintext environment variable.
  Closes Prowler codebuild_project_not_publicly_accessible,
  codebuild_project_user_controlled_buildspec,
  codebuild_project_source_repo_url_no_sensitive_credentials and
  codebuild_project_no_secrets_in_variables.
- ECR repositories now flag scan-on-push being off, mutable tags, a repository
  policy open to anyone, and the absence of a lifecycle policy. Closes Prowler
  ecr_repositories_scan_images_on_push_enabled,
  ecr_repositories_tag_immutability,
  ecr_repositories_not_publicly_accessible and
  ecr_repositories_lifecycle_policy_enabled.
- Glue jobs now flag having no security configuration, continuous logging being
  off, and a credential in the job arguments. Closes Prowler
  glue_etl_jobs_amazon_s3_encryption_enabled,
  glue_etl_jobs_cloudwatch_logs_encryption_enabled,
  glue_etl_jobs_job_bookmark_encryption_enabled, glue_etl_jobs_logging_enabled
  and glue_etl_jobs_no_secrets_in_arguments.
- EKS clusters now flag an endpoint reachable from the internet, incomplete
  control-plane logging, secrets without their own KMS key, and a Kubernetes
  version out of standard support. The support state is read from AWS rather
  than a version constant compiled into the binary, so it stays correct when
  the calendar moves. Closes Prowler eks_cluster_not_publicly_accessible,
  eks_cluster_private_nodes_enabled,
  eks_control_plane_logging_all_types_enabled,
  eks_cluster_kms_cmk_encryption_in_secrets_enabled and
  eks_cluster_uses_a_supported_version.
- The Health column on an ECS service's task list now reads `healthy` /
  `unhealthy` / `unknown` instead of the API's uppercase spelling.

- A database instance being deleted now reads as a warning instead of green.
- A snapshot in a state neither ready nor failed, such as one still copying,
  now reads as a warning instead of green, on both RDS and DocumentDB
  snapshots. Only three states were named and everything else fell through as
  ready.
- ECS services now say why they are short of capacity: a service that wants
  tasks and is running none reads `no tasks running`, and one running fewer
  than it asks for reads `running below desired count`. Both used to colour
  the row with nothing in the Status column to explain it.
- IAM users whose console sign-in has gone unused for over 90 days are now
  flagged, alongside the existing never-used check.
- ECS service task lists now work in demo mode — the demo backend matched
  clusters by ARN only, so no service ever listed its tasks.

- `make check-deps` reports outdated direct Go modules, a newer Go toolchain
  patch, and newer releases of pinned GitHub Actions, and fails the pre-push
  gate the same way `make security` does — dependency drift that was only
  visible in Dependabot on the remote is now visible before the push.
- Security groups now flag more of the ports that should never face the
  internet — FTP, Telnet, SMTP, SMB, Oracle, Kafka, Cassandra,
  Memcached and Kibana join SSH, RDP and the databases already covered.
  A VPC's default group is flagged when it still carries rules, since
  AWS attaches it to anything launched without an explicit group, and a
  group no network interface references is flagged as dead
  configuration. Closes Prowler
  `ec2_securitygroup_allow_ingress_from_internet_to_tcp_port_ftp_20_21`,
  `_telnet_23`, `_memcached_11211`, `_cassandra_7199_9160_8888`,
  `_kafka_9092`, `_oracle_1521_2483`, `_to_high_risk_tcp_ports`,
  `ec2_securitygroup_default_restrict_traffic` and
  `ec2_securitygroup_not_used`.

- Subnets that hand every instance a public IP address on launch are now
  flagged. Closes Prowler `vpc_subnet_no_public_ip_by_default`.

- Load balancers now report four more posture problems: HTTP desync
  mitigation left in monitor-only mode, invalid HTTP headers forwarded
  to targets instead of dropped, a listener carrying traffic in the
  clear, and a listener on a TLS policy that predates TLS 1.2. Closes
  Prowler `elbv2_desync_mitigation_mode`,
  `elbv2_alb_drop_invalid_header_fields_enabled`, `elbv2_ssl_listeners`,
  `elbv2_nlb_tls_termination_enabled` and `elbv2_insecure_ssl_ciphers`.

- Transit gateways that accept shared VPC attachments without review are
  now flagged. Closes Prowler
  `ec2_transitgateway_auto_accept_vpc_attachments`.

- VPC endpoints whose policy grants every action to every principal —
  the policy AWS attaches when none is supplied — are now flagged.
  Closes Prowler `vpc_endpoint_connections_trust_boundaries`.
- IAM roles now report three trust and permission problems the list was
  silent about: a trust policy any AWS account can use, an AWS service
  trusted with nothing scoping which caller it acts for, and an inline
  policy whose action set adds up to full administrator. A role carrying
  AdministratorAccess or PowerUserAccess is flagged too. The wildcard-trust
  check moved off string matching onto the shared policy engine, so a
  wildcard principal paired with an external ID no longer reads as
  wide open. Closes Prowler `iam_role_cross_service_confused_deputy_prevention`,
  `iam_inline_policy_allows_privilege_escalation`,
  `iam_role_administratoraccess_policy` and part of
  `iam_role_cross_account_readonlyaccess_policy`.

- IAM policies now report a customer-managed policy whose actions combine
  into a path to full administrator, even when no single action looks
  privileged. A policy already reported as wildcard admin is not reported
  twice. Closes Prowler `iam_policy_allows_privilege_escalation`.

- IAM users now report a console password that has never been used, an
  access key that has gone 90 days without signing a request, both
  access-key slots active at once, and AdministratorAccess attached
  directly to the user. Access keys are named by their last four
  characters only. Each condition is now its own entry, so a user with a
  stale key and no MFA no longer has one problem hidden behind the other.
  Closes Prowler `iam_user_console_access_unused`,
  `iam_user_accesskey_unused`, `iam_user_two_active_access_key` and
  `iam_user_administrator_access_policy`.

- IAM groups now report AdministratorAccess attached to the group, which
  hands administrator access to every member. Closes Prowler
  `iam_group_administrator_access_policy`.

- WAF web ACLs now report an ACL with no rules at all, which lets every
  request through while still reading as protection. Closes Prowler
  `wafv2_webacl_with_rules`.

- Secrets Manager secrets now report a resource policy any AWS account can
  read the secret through, and one that names a principal in another
  account. Closes Prowler `secretsmanager_not_publicly_accessible` and
  `secretsmanager_has_restrictive_resource_policy`.

- KMS keys now report a key policy that lets any AWS principal decrypt with
  the key. Closes Prowler `kms_key_not_publicly_accessible`.
- EC2 instances now flag three security-posture problems on the list row
  itself: instance metadata that still answers without a session token, a
  routable public address, and a credential pasted into user data. A running
  instance with a public address whose security groups leave a sensitive port
  open to the world is reported as reachable from the internet, with the
  ports named — read entirely from data a9s already holds, at no extra API
  cost. Closes Prowler `ec2_instance_imdsv2_enabled`,
  `ec2_instance_public_ip`, `ec2_instance_secrets_user_data` and
  `ec2_instance_port_ssh_exposed_to_internet` with its sixteen sibling port
  checks.
- AMIs owned by the account and shared with every AWS account are now
  reported. Closes Prowler `ec2_ami_public`.
- ECS services that hand their tasks routable public addresses are now
  flagged. Closes Prowler `ecs_service_no_assign_public_ip`.
- ECS tasks now report what their task definition bakes in: a privileged
  container, the host network or process namespace, a container that can
  write its own root filesystem, a container with no log driver, and a
  credential stored as a plaintext environment variable. Closes Prowler
  `ecs_task_definitions_no_privileged_containers`,
  `ecs_task_definitions_host_namespace_not_shared`,
  `ecs_task_definitions_containers_readonly_access`,
  `ecs_task_definitions_logging_enabled` and
  `ecs_task_definitions_no_environment_secrets`.
- Lambda functions now report a credential in their environment variables, a
  resource policy any caller can invoke, and a function URL that requires no
  authentication. Closes Prowler
  `awslambda_function_no_secrets_in_variables`,
  `awslambda_function_not_publicly_accessible`, `awslambda_function_url_public`
  and `awslambda_function_url_cors_policy`.
- Auto Scaling groups now report a legacy launch configuration, a single
  availability zone, and a group behind a load balancer that still decides
  health from EC2 status checks. For groups that still reference a launch
  configuration, one batched call reports whether it permits IMDSv1, assigns
  public addresses, or carries a credential in its user data. Closes Prowler
  `autoscaling_group_using_ec2_launch_template`,
  `autoscaling_group_multiple_az`,
  `autoscaling_group_elb_health_check_enabled`,
  `autoscaling_group_launch_configuration_requires_imdsv2`,
  `autoscaling_group_launch_configuration_no_public_ip` and
  `autoscaling_find_secrets_ec2_launch_configuration`.
- EBS snapshots restorable by every AWS account are now reported, from one
  account-wide call rather than a per-snapshot query. Closes Prowler
  `ec2_ebs_public_snapshot`.
- Launch templates now report a credential pasted into the default version's
  user data. Closes Prowler `ec2_launch_template_no_secrets`.
- Databases and storage now report the security posture an auditor asks
  about, not just the lifecycle state. Every signal below is read-only
  and shows up in the list status column, the row color, and the detail
  view's attention section. Supporting rows read as words (`on`/`off`,
  `yes`/`no`), and a row that only restated its finding was dropped.
- **S3 buckets**: a bucket AWS reports as public by policy, versioning
  never enabled, MFA delete off on a versioned bucket, server access
  logging off, no enabled lifecycle rule, and object lock off. Closes
  Prowler `s3_bucket_public_access`,
  `s3_bucket_policy_public_write_access`, `s3_bucket_object_versioning`,
  `s3_bucket_no_mfa_delete`, `s3_bucket_server_access_logging_enabled`,
  `s3_bucket_lifecycle_enabled`, `s3_bucket_object_lock`.
- **ElastiCache Redis**: encryption at rest off, encryption in transit
  off, no authentication token on a group that does encrypt in transit, and
  automatic backups off. Closes Prowler
  `elasticache_redis_cluster_rest_encryption_enabled`,
  `elasticache_redis_cluster_in_transit_encryption_enabled`,
  `elasticache_redis_replication_group_auth_enabled`,
  `elasticache_redis_cluster_backup_enabled`.
- **RDS DB instances**: single-AZ, auto minor version upgrade off, IAM
  database authentication off, a vendor-default master username, a CA
  certificate inside 90 days of expiry, and an engine version AWS no
  longer supports. Closes Prowler `rds_instance_multi_az`,
  `rds_instance_minor_version_upgrade_enabled`,
  `rds_instance_iam_authentication_enabled`, `rds_instance_default_admin`,
  `rds_instance_certificate_expiration`,
  `rds_instance_deprecated_engine_version`,
  `rds_instance_extended_support`.
- **DB clusters** (Aurora and DocumentDB): the same four configuration
  checks on the cluster shape. Closes Prowler `rds_cluster_multi_az`,
  `rds_cluster_minor_version_upgrade_enabled`,
  `rds_cluster_iam_authentication_enabled`, `rds_cluster_default_admin`.
- **DB snapshots**, instance and cluster alike: a snapshot whose restore
  attribute is shared with every AWS account. Closes Prowler
  `rds_snapshots_public_access`, `documentdb_cluster_public_snapshot`.
- **DynamoDB tables**: deletion protection off, and a resource policy
  that grants another account or any principal at all. Closes Prowler
  `dynamodb_table_deletion_protection_enabled`,
  `dynamodb_table_cross_account_access`.
- **OpenSearch domains**: reachable outside a VPC behind an open access
  policy, HTTPS not enforced, and node-to-node encryption off. Closes
  Prowler `opensearch_service_domains_not_publicly_accessible`,
  `opensearch_service_domains_https_communications_enforced`,
  `opensearch_service_domains_node_to_node_encryption_enabled`.
- **Redshift clusters**: audit logging off and a parameter group that
  does not require SSL. Closes Prowler `redshift_cluster_audit_logging`,
  `redshift_cluster_in_transit_encryption_enabled`.
- **EFS file systems**: not encrypted at rest, a file system policy open
  to anyone, and AWS Backup's automatic backups off. Closes Prowler
  `efs_encryption_at_rest_enabled`, `efs_not_publicly_accessible`,
  `efs_have_backup_enabled`.

### Changed

- Row colour and the Status column now come from one selection for every
  resource type, EventBridge rules, Kinesis streams and MSK clusters
  included. A row that carries several findings shows the worst one and
  counts the rest, instead of showing whichever finding happened to be first.
  Route tables, SNS subscriptions, SSM parameters, CloudTrail trails,
  CloudWatch alarms, CloudTrail events, SES identities, IAM users and
  policies, auto scaling groups, ECS clusters and services, RDS and DocumentDB
  instances, clusters and snapshots, Redis, DynamoDB, Redshift, OpenSearch and
  EFS all classified from raw AWS fields or from the rendered Status text
  beside the findings that described the same thing; they no longer do.
- SES identities now colour when the account's 24-hour sending quota is over
  80% consumed, instead of showing the warning glyph on a green row.
- An OpenSearch domain with a forced software update pending now reads as a
  warning rather than as broken: the update is scheduled, not a failure.
- Every OpenSearch signal, including the internet-reachable, plaintext-HTTP and
  node-to-node-encryption checks, is now on screen the moment the list loads.
  They were reported a moment later by a second pass that made no AWS call and
  only re-read what the list load had already fetched.
- OpenSearch domains report the software-update and encryption-at-rest checks
  as soon as the list loads rather than a moment later, and the "+N" in the
  Status column now matches the number of findings the detail view shows.
- ECS tasks listed under a service now carry the same unhealthy and
  stop-reason findings the top-level task list has always shown.

- Redis and Redshift rows now take their color from the worst finding on
  the row rather than the first Wave 1 one, so a Broken signal can no
  longer hide behind a Warning that happened to be evaluated earlier.

### Fixed (security)

- Switching profile or region could leave the previous account's data on
  screen: its caller identity in the header, a revealed secret's
  decrypted value, or its Cost Explorer figures — all attributed to the
  newly-selected account. Work started before the switch carried a
  freshness stamp that still looked current afterwards, because one of
  the four generation counters started at zero instead of one. A failed
  identity fetch on the new account also left the old ARN in the header
  indefinitely. Both are closed, with regression coverage for identity,
  revealed values and costs.

### Fixed

- A hosted zone no longer reports load balancers, CloudFront distributions or
  other targets that do not exist. Its alias records name a DNS target, and the
  panel used to turn that name straight into a count, so a zone whose alias
  pointed at a deleted load balancer showed one anyway and dead-ended on Enter.
  Every alias pivot now reports only what the target list confirms, and reads
  as unknown when no list has been read.

- A related row no longer reads as a confident zero when nothing behind it was
  read at all. Twenty-nine pivots across S3, Redis, Document DB, RDS, DynamoDB,
  EC2, ECS, EKS, EFS, Lambda, SES and WAF now read as unknown when the list they
  answer from was never fetched, instead of a zero, or a zero with a "more to
  come" marker that implied a list had been seen. A cluster snapshot no longer
  reports the parent cluster it names as present without checking. A list that
  was fetched but came back partial still reports what it found, marked as a
  lower bound.

- The role a CloudTrail event names is now reported only once the role list
  confirms it still exists. An event body records what was there at the time,
  so an event naming a since-deleted role used to answer with a count that
  went nowhere on Enter. With no role list read yet, the row now reads as
  unknown rather than guessing, and a role filed under a path is matched by
  its whole name instead of a trailing fragment that could belong to a
  different role. Where only part of the list was read, every CloudTrail event
  pivot now reports what those pages confirmed, marked as a lower bound, which
  is the same reading the rest of the related panel uses.

- A load balancer with several listeners in the clear, or several on a weak
  TLS policy, now names every affected port rather than the first one, and its
  listeners are read to the end instead of one page deep, so a listener no
  longer hides behind whichever listeners AWS returned first. Each supporting
  row leads with its port, so a balancer with three of them says which three.

- An IAM user whose access-key last use could not be read now reads as
  unknown rather than clean. The key that could not be read is exactly the
  one that might be idle.

- A role opened from another view now reports the same problems as the same
  role in its list. The drill skipped inline policies, so a privilege-
  escalation policy was visible one way and invisible the other.

- Pending-maintenance results survive a failed page. A later page failing
  used to discard the instances the earlier pages had already named.

- CodeArtifact repository policies are read by parsing them, not by matching
  text, so a pretty-printed policy is no longer missed and one scoped to an
  organisation is no longer called public.

- A VPC endpoint that grants every action of its own service to every
  principal now reports as open. The endpoint only ever fronts that service,
  so the narrower wildcard withholds nothing.

- Policy documents decode one way everywhere. A literal "+" inside a policy
  survived at one site and became a space at another, so the same document
  read differently depending on which view asked.

- Redshift parameter groups are read once each and concurrently, instead of
  serialising every cluster in the pass behind one lock.

- An ECS service event newer than the window is no longer missed because an
  older event preceded it in the list.
- Demo mode showed sixteen database instances with deletion protection
  turned off, burying the one instance that exists to demonstrate the
  signal. The bulk filler pool left the setting unstated, so every filler
  row inherited the warning. One instance carries it now.

- Demo mode stated two different public addresses for the same instance:
  the instance said one and its Elastic IP said another. They agree now,
  on both the public and the private address.

- Demo mode had no network load balancer carrying a cleartext listener, so
  half of that signal went unshown. One network balancer now serves TCP on
  port 443, where the port promises TLS and the protocol never terminates
  it.

- Demo mode had no resource whose related panel drilled to an IAM role with
  a finding on it, so the role's issues could only be seen from the role
  list. One build project now uses the role whose inline policy allows
  privilege escalation.

- Detail rows say what is true of the account rather than how the SDK spells
  it. A public image and a public snapshot read `yes` instead of `true`, an
  auto-scaling group's health check reads `ec2` instead of `EC2`, its public
  address assignment reads `enabled`, Redshift's encrypted-connection
  parameter and the four S3 public-access-block flags read `off` beside the
  parameter name they name, and an Athena workgroup's unenforced settings
  read `no` with its unencrypted results reading `off` rather than `nil`.

- Demo mode flagged four Athena workgroups for unencrypted query results and
  none for unenforced settings, so the signal had no single carrier and the
  type never appeared in the sweeps that check every other type's rendered
  rows. One workgroup now carries both halves and the rest encrypt their
  results.

- An Athena workgroup's governance problems now read as sentences. The list
  cell said `Workgroup settings enforced (2 findings)`, which was the label of
  the row underneath it and left the reader to work out which way "enforced"
  pointed. Unenforced settings and unencrypted results are two independent
  settings, so they are two signals: `settings can be overridden per query`
  and `query results stored unencrypted`, each with a sentence saying what it
  exposes and what to change, and the second naming where the results land.

- A load balancer with one listener in the clear now says `port 443`, not
  `ports 443`, and its explanation says "this listener" rather than "these
  listeners". The same on weak TLS policies. A single port under a plural
  heading reads as a list that got truncated.

- An Elastic IP attached to a NAT gateway no longer shows as idle. The colour
  was decided a second time from two of the three attachment fields, and a NAT
  gateway's address — which has an interface but no association and no
  instance — looked unattached to it. The one place that decides attachment is
  the finding the list already carries.

- Demo mode said three different things about one instance's addresses: its
  Elastic IP, its network interface and the instance list each named a
  different pair, and the address was a NAT gateway's allocation, which cannot
  belong to an instance. All three now agree, and the allocation belongs to
  the NAT gateway alone.

- Every demo address now has one owner. Two NAT gateway allocations were also
  serving as instance Elastic IPs, one of them advertised on two interfaces
  with two different public addresses, and a staging allocation named a
  production instance's interface. The instances keep their own allocations,
  the NAT gateways keep theirs, and each address appears on one interface and
  one instance.

- Demo mode listed every KMS key twice, in a different order on each run.
  The list was being built from the lookup table that deliberately holds each
  key under both its bare ID and its full ARN, rather than from the account's
  key list. Each key now appears once, in a stable order.

- Drilling into a related resource no longer overwrites the full list it
  came from. After loading all 200 instances of a type, opening a pivot
  that matched 3 of them replaced the cached list with just those 3 —
  the main menu then reported `3` as an exact count, and the wrong count
  was written to the on-disk cache, so it survived a restart. Lookups by
  ID and child lists corrupted the same way. Each fetch now states which
  kind it is, and only a full list of a type may replace that type's
  cached rows. The same corruption was reachable by a second route — a
  late drill or by-ID result arriving while that type's full list was
  open on screen — which is closed too; such a result now finds the view
  it actually belongs to instead of overwriting the list it merely
  shares a type with.

- A failed "load more" no longer leaves a resource list stuck. In the
  web UI, when loading the next page failed, the list kept showing its
  loading indicator indefinitely with no way to retry — only that one
  failure path forgot to switch it off. Every path that ends a fetch now
  clears all of its indicators together.

- A fetch that partly failed no longer erases its own error message.
  When a result came back with some rows and an error, the rows landing
  cleared the error marker, so the failure went unreported.

- Fixed a crash and occasional wrong rows in resource lists. A list
  screen and the shared cache held the same rows in memory while
  guarding them with separate locks, so loading another page could
  overwrite what the cache held without it noticing, and a background
  refresh landing at the wrong moment could take the app down. Each now
  keeps its own copy.

- A related-resource pivot no longer reports a confident `0` when the
  AWS call behind it failed or was denied. Fifteen checkers turned an
  error into a proven dead end — a log group actively streaming to
  Kinesis showed `(0)` if `logs:DescribeSubscriptionFilters` was not
  granted, a Redis replication group showed `(0)` across four pivots
  when one lookup was throttled, an IAM group with three inline
  policies showed `(0)` when only one of two list calls was permitted.
  These now render as unknown, which is the truth: we could not look.
  Where several calls back one pivot and only some succeed, the count
  renders as `N+` instead of a false exact total. A throttled
  EventBridge enrichment also no longer invents a critical "enabled
  rule has no targets" alarm for a healthy rule.

  The pipeline pivots on CodeBuild projects and ECR repositories were
  the last two holdouts: they inspect each pipeline in turn, and a
  lookup that failed was quietly skipped, so a project used by five
  pipelines reported an exact `2` when three lookups were throttled.
  The ECR one was worse — without permission to read pipelines at all
  it reported an exact `0`, a definitive "nothing uses this
  repository" arrived at without a single successful call.

### Added

- Detail views fetch what the list APIs don't return (#261). Opening a
  detail, YAML, or JSON view now enriches on demand: Step Functions
  show the full ASL definition, live status, and role; CloudFormation
  stacks show the template body (session-cached, version-keyed so an
  updated stack never serves its pre-update template); Lambda functions
  show state, code metadata, and reserved concurrency; EC2 instances
  show decoded user data (base64 and gzip handled); SNS topics show
  their full attribute set including effective delivery policy; S3
  buckets show bucket policy, CORS rules, and lifecycle configuration.
  These six and the two IAM document enrichers run on one generic
  engine (Transfer agreements keep their own resolver), and a dedicated
  demo smoke gate walks all nine of them on every push.
- Detail open/refresh runs under a single operation identity: the
  enrichment and every related-panel check created by one user action
  share one generation, one set of AWS clients, and one call-coalescing
  namespace, and their results are accepted only while that operation
  is still the active one. A refresh supersedes in-flight work instead
  of racing it; concurrent calls within one operation collapse to a
  single AWS request per API (an SFN refresh performs one
  `DescribeStateMachine` in total); rotation invalidates everything
  in flight.
- `--trace <path>` writes a structured, JSON-lines diagnostic stream of
  the detail-operation lifecycle — an operation beginning, each AWS
  call as executed or served without a new request, an enrichment
  cache hit/miss/write, and a result fold's accept/reject decision
  (#488). Off by default; writes only to the given file, never stdout.

### Fixed

- Web mode: detail enrichment now works end to end — results fold into
  the shared controller state and directly opened YAML/JSON views
  dispatch enrichment, so the web view shows what the terminal shows.
- Web mode: related-panel drill-ins find targets outside the cached
  list (lazy-add previously ran only in the terminal), and background
  task execution no longer reads session state unlocked while request
  handlers mutate it.
- Enrichment and related-panel errors surface as an error flash in the
  terminal again; both previously rendered no feedback at all.
- Documents keep numeric fidelity: integers above 2^53 in policy
  documents, templates, ASL definitions, and topic attributes no longer
  silently round (9007199254740993 stayed ...992 before), and a
  malformed document with a stray trailing brace still displays
  verbatim rather than being reshaped into structure.
- YAML and JSON views render the same shape for the same resource: the
  YAML view promotes embedded struct fields exactly like the JSON view
  (`encoding/json` semantics) instead of nesting them under the
  embedded type's name.
- Related-panel counts over truncated target populations render "N+"
  instead of a misleading exact "N", and an empty truncated page no
  longer erases the truncation marker from the cached row store.

## [3.56.0] - 2026-07-21

### Added

- `o` opens the selected resource in the AWS console; `O` copies the
  console URL (#477). Every top-level type (70) and child-view type (29)
  resolves a deep link — each URL shape verified against AWS's own
  `/go/view` ARN resolver (78 live captures, including the GovCloud and
  China partition domains) and official documentation. Works on lists,
  child lists, detail, and the focused related-panel row; region,
  partition, and per-service quirks (global consoles, WAF scope,
  DocumentDB vs Aurora vs Neptune engines, account-in-path consoles)
  are handled per type. Demo mode shows a notice instead of opening —
  `O` still copies.
- `$BROWSER` is honored as the opener command (whitespace-split into
  argv, so `BROWSER="firefox --new-tab"` works; never passed through a
  shell); unset, the platform opener is used (`open`/`xdg-open`/
  `rundll32`). Opened URLs must pass an https + console-domain
  allow-list guard before any process is spawned.
- Web mode parity: the same keys work in `--web`, via `window.open` and
  the async clipboard API inside the keydown handler — the server never
  executes anything.
- Footer `o Open` hints on list and detail; help-overlay entries; the
  keybindings reference documents both keys.
- New wiki page: Environment Variables — every variable a9s honors,
  including three that were previously documented nowhere
  (`A9S_MODE`, `A9S_LOG_FILE`, `A9S_CONFIG_FOLDER`); README and website
  now point there.

### Fixed

- Cache-restored rows keep their console links: `trail`, `msk`, and
  `codeartifact` wrote fields at fetch time that were missing from
  their `FieldKeys` lists and silently dropped on the cache round-trip;
  builders that needed `RawStruct` (which never survives the cache)
  were migrated to Wave-1 `Fields`.
- Neptune clusters open the Neptune console (the generic ARN resolver
  lands them on the RDS console).

## [3.55.2] - 2026-07-20

### Fixed

- Web mode's `c` copy key works (#478): it was a silent no-op — the
  client POSTed the action to a server that treats copy as
  renderer-only, and the browser half was never implemented. Copy
  resolution now lives once in the controller
  (`Controller.CopyContent`), exposed through
  `ViewState.CopyText/CopyLabel` and rendered as `data-copy-*`
  attributes; the web client writes the clipboard in the keydown
  handler and shows a client-side flash. The TUI's `handleCopy`
  delegates to the same resolution, deleting its duplicated logic.
- `CopyField` actually resolves (#478): every `CopyField` in the
  catalog is registered on a child type, but the copy path consulted
  only the top-level registry — so copy always fell back to the row ID.
  The shared lookup consults both registries; on an
  `sns_subscriptions` child list, `c` now copies the endpoint.
- `Controller.CopyContent` takes the write lock: the detail branch
  reaches the mutating detail-body builder, and the read lock allowed
  concurrent map writes (crash) under concurrent web requests.
- Identity no longer survives a profile/region rotation stale: a new
  `ClearIdentityIntent` (emitted by `HandleProfileSelected` /
  `HandleRegionSelected`) clears the controller's cached identity and
  the TUI identity screen's renderer state, so neither renderer shows
  or copies the previous pair's ARN. Repopulation is owned exclusively
  by the post-connect identity fetch — rotation itself never fetches
  (the retained pre-rotation clients would re-land the old identity
  stamped with the new generation). On `--no-cache` rotations (demo
  included), where no post-connect refetch exists, the identity screen
  resets to blank instead of a never-resolving loading state.

### Added

- `core/` exports: `app.Controller.CopyContent`,
  `app.ViewState.CopyText/CopyLabel`, `runtime.ClearIdentityIntent`.

## [3.55.1] - 2026-07-20

### Fixed

- Deleted-resource 404s no longer surface in the `!` error log as
  enrichment failures (#456): a resource deleted between the list call
  and a per-ID describe (`NoSuchBucket`, bare `NotFound`,
  `NoSuchHostedZone`) is classified as an operational race — the row is
  marked data-incomplete (`?`), no finding is emitted, and the failure
  aggregate stays clean. The S3 public-access-block and Route 53
  enrichers adopt the shared classifier; the four S3 related-panel
  checkers switch from message-substring to error-code matching and
  render a deleted bucket as an honest zero instead of an error cell.
- Stale disk-cache rows can no longer outlive a deletion indefinitely
  (#457): an exact observation is now authoritative at the per-type disk
  reconcile chokepoint and shrinks stored rows to the live population
  (truncated first pages still never lose deeper rows), and a type whose
  population went to zero persists its emptiness. This ends the
  startup-loop where a deleted resource's cached row re-seeded every
  session and fed Wave-2 describes against a deleted ID. Wave-2 finding
  carry and `FindingFirstSeen` survive the shrink
  (`docs/design/cache-requirements.md` C6a amended, defect D18).
- S3 throttling (`SlowDown`, 503) is retried like every other throttle
  code instead of failing the call on the first attempt.

### Added

- `core/aws.IsNotFoundErr` — the canonical deleted-between-list-and-describe
  classifier for fetchers, checkers, and enrichers (`core/` importers).

## [3.55.0] - 2026-07-20

### Added

- Opt-in diagnostics: `--log-file` / `A9S_LOG_FILE` install a JSON file
  logger (off by default); cache skip reports route through it, probe
  failures/timeouts appear in the `!` error log (one entry per type per
  sweep), and web mode logs requests and render errors when enabled.
- New pre-push gates: `verify-renderer-free` (core/ compiles with zero
  renderer or internal/ deps), `verify-hooks` (git hooks installed),
  `check-catalogen` (generated doc blocks match catalog declarations);
  `verify-readonly` reimplemented as an AST checker (`cmd/readonlycheck`)
  immune to comment/string/formatting bypasses; a dead-export baseline in
  arch-review; drift-guard tests pinning the process doc's gate list to
  the Makefile and attention-signals prose to its generated tables.

### Changed

- List rendering memoized: 7.3 ms → 72 µs per frame on a 3,000-row list
  (105.7k → 16 allocs); invalidation covers filter, sort, attention,
  row-store generations, and enrichment state.
- Cache saves skip byte-identical type files and perform disk I/O outside
  the session-wide pair lock (prepare-under-lock, commit-outside split).
- Wave-2 enrichment dispatches through a bounded 4-probe window with live
  refills instead of ~50 concurrent probes per sweep.
- ACM expiry/orphan findings compute in wave 1 from the list call —
  the per-certificate wave-2 sweep and its >50-cert blind spot are gone.
- Severities aligned with the attention-signals contract: tg graduated
  (partial-unhealthy warns, all-unhealthy breaks; initial/draining/unused
  don't count), s3 public-access-block findings warn, acm graduated
  30d/7d, ec2's four instance-status conditions split into four codes.
- **BREAKING (`core/`)**: `IssueEnricherResult.IssueCount` and
  `messages.EnrichmentChecked.Issues` deleted; 24 dead exports removed
  (superseded whole-page fetchers, `projection.Generic` — use
  `GenericWithConfig` — and others); 17 test-only exports renamed to
  `*ForTest`; `scripts/verify-readonly.sh` removed in favor of
  `cmd/readonlycheck`.

### Fixed

- Fatal `concurrent map read and map write` crash when opening a list
  during the wave-2 enrichment sweep.
- Eternal spinner on stalled networks: all interactive fetch lanes,
  including server-side filtered drills, carry a 30s deadline; profile or
  region switches cancel in-flight background tasks.
- EKS node-group pagination lost every page after the first for clusters
  spanning multiple `ListNodegroups` pages, and a result-capped fetch
  returned an unresumable empty continuation token — both fixed.
- Related-checker panics surfaced nowhere (permanent `?`); they now reach
  the error flash and log. Four related checkers (dbc, dbi, ddb, redis)
  reported a false proven-zero on an unloaded alarm cache; they now
  report unknown per the engine contract.
- RELATED panel filter mode: Up/Down moved a dead widget cursor and Enter
  never navigated; movement now drives the real cursor (arrow keys only —
  `j`/`k` still type into the filter), Enter confirms the filter and
  stays, and the next Enter navigates, with the cursor surviving the
  confirm.
- `SaveTheme` silently destroyed `config.yaml` on a corrupt parse and
  wrote non-atomically; child-view fetches dropped partial results on
  composite errors in both live paths.
- Enrichment dispatcher races: stale completions starved the queue,
  list-open probes stole refills or triggered duplicate dispatches, and a
  rerun-invalidated final completion stranded the progress badge.
- Stale list rows when a background availability update replaced the
  row store under a screen that had not adopted its own rows yet.

## [3.54.1] - 2026-07-19

### Changed

- Replaced the retired Go Report Card badge (goreportcard.com globally
  retired its hosted badge service) with a golangci-lint badge in the README.
- **BREAKING (`core/resource`)**: `FormatApproximate` and
  `CellKindApproximate` are renamed to `FormatTruncated` and
  `CellKindTruncated`. External importers of the now-public `core/resource`
  package must update references. AWS wire field names
  (`ApproximateNumberOfMessages`, ...) and the SQS `approx_messages` /
  `approx_not_visible` domain keys are unchanged — they are AWS's own
  semantics, not the a9s related-count truncation concept.

### Fixed

- The per-resource docs' related-panel **"Truncated?"** column was derived
  from `NeedsTargetCache` — a prefetch flag that does not imply whether a
  count can be a truncated lower bound — so it carried stale yes/no values
  across every `docs/resources/*.md`. It is now sourced from a new
  `domain.RelatedDef.Truncated` flag set from each checker's actual
  behavior, and the affected docs are corrected.
- 13 reverse-scan / count related checkers dropped the truncation flag
  (returned an exact `relatedResult` where the honest answer is a truncated
  lower bound), so a partially-scanned cache rendered a misleading exact
  count instead of `N+`. They now carry the flag via `relatedResultTrunc`;
  identity-resolvers that miss on a truncated parent return `?` (unknown)
  rather than a false `0`. The 8 corresponding catalog `Truncated` flags are
  flipped to match.
- Six Cost Explorer resource-drill tests began failing once the calendar
  passed the 14th of the month: they drilled into a first-of-month cell that
  the (correct) 14-day resource-retention guard refuses. The tests now drill
  into the current day and share one clock across their headless and TUI
  lanes; application behavior is unchanged.
- A CodeQL path-injection finding on the cache path is closed by an
  `IsLocal` containment guard, and a controller close-discipline test gate
  fixes 8 leaked controllers surfaced under `-shuffle=on` test ordering.
- The four newest resource types (`mwaa`, `transfer`, `vpc-peer`, `lt`)
  omitted `LifecycleKey: "status"` in their catalog definitions, so their
  Status column skipped the lifecycle-aware decoration sibling types get and
  the generated docs mislabeled the lifecycle key as "none". All four now
  declare it.
- Cost Explorer no longer overflows its pane at a one-row height:
  `RenderCosts` appended a blank line plus footer (2 lines) even at height
  1; it now renders a single pinned footer row.

### Removed

- ~70 unreachable view-model functions left by the headless-controller
  migration (old stateful `DetailModel`/`MainMenuModel`/`ResourceListModel`
  lifecycles and accessors). Pure internal cleanup, no behavior change;
  `deadcode` reports zero dead functions in `internal/tui`, and unit-test
  coverage of `internal/tui` rose from 76% to 90.7%.

## [3.54.0] - 2026-07-15

### Changed

- The platform-agnostic core moved from `internal/*` to `core/*` (16
  packages; `internal/tui` stays), making it importable by external Go
  modules. Pure move + import rewrite — zero behavior change; a9s remains
  read-only toward AWS.
- LICENSE now carries the full GPL-3.0 text (previously a pointer). The
  `core/` packages are additionally offered under a commercial license
  (COMMERCIAL-LICENSE.md); contributions to `core/` require a CLA
  (CONTRIBUTING.md). SPDX headers added across `core/` and `internal/tui/`.
- `make test-race` now runs with `-shuffle=on`, matching CI's test-order
  shuffling so order-dependent leaks fail locally before any push.

### Fixed

- A test-order-dependent flake (macOS CI, `-shuffle=on`): the session's
  on-disk cache root was resolved lazily at first write, so a background
  cache writer leaked by a never-closed controller could follow a
  since-changed `A9S_CONFIG_FOLDER` into an unrelated test's temp directory
  mid-cleanup. The cache root is now pinned once at session construction.
- `cache.Store.SaveType` now enforces an explicit path-containment check:
  a resolved cache file path outside the profile--region pair directory is
  rejected with an error instead of relying on `os.CreateTemp`'s incidental
  separator rejection.
- The web session cookie sets `Secure` when the request arrives over TLS.
  It stays unset on plain loopback HTTP because WebKit does not treat
  `http://localhost` as trustworthy for Secure cookies (webkit.org/b/281149)
  and an unconditional flag would break `--web` in Safari.

## [3.53.2] - 2026-07-15

### Fixed

- Degraded "details denied" rows now distinguish a genuine authorization
  denial from a non-auth failure. A nil/empty describe response, a throttle,
  a cancellation, or a row absent from a batch response renders the neutral
  "details unavailable" instead of falsely reporting an IAM denial; EC2's
  `UnauthorizedOperation` code (used by Launch Templates) is now correctly
  classified as a denial. Applies to MWAA, Transfer, Launch Templates,
  DynamoDB, OpenSearch, EKS clusters, and node groups. Row color, menu
  counts, and severity are unchanged.
- EKS node-group related panels no longer risk associating EC2 instances or
  EBS volumes from another cluster: an instance matches only when its
  `eks:cluster-name` tag exactly equals the node group's cluster (an absent
  tag previously slipped through, so two clusters with a same-named node
  group could cross-contaminate).

## [3.53.1] - 2026-07-15

### Changed

- Internal duplication audit across the mwaa/transfer/lt/vpc-peer series
  removed ~340 lines with no behavior change: a shared details-denied finding
  builder, per-state finding lookup tables, merged launch-template/vpc-peer/
  Lambda helpers, one shared row-color function, and de-duplicated demo
  fixtures/fakes. All status phrases, related-panel counts, and findings are
  unchanged.

## [3.53.0] - 2026-07-15

### Added

- New resource type: **VPC Peering (`vpc-peer`, aliases `pcx`, `peering`)**
  — the 70th type (main menu 71), closing the four-type series started in
  v3.50.0. One paginated call carries the whole story. Status phrases per
  lifecycle state, including the actionable countdown `pending acceptance:
  expires in <N>d` (AWS expires unaccepted requests at 7 days — the number
  IS the escalation) and verbatim `Status.Message` causes for rejected /
  failed connections. An active-only IPv4 CIDR-overlap warning names the
  clashing ranges; CIDR fields are nil on every non-active state per the
  API contract and render nil-safe.
- **Two zero-API route checks** ride the route-table cache as `~`
  background annotations: `no local route to peer` (an accepted tunnel
  nobody routes into) and `route to peer blackholed` (the other side tore
  down; your route eats packets). An absent or truncated route-table list
  produces no verdict — never a guess.
- **Related panel**: the route tables that actually route to the peer
  (truncated lists render the honest `N+` lower bound), the local VPC
  through a symmetric membership gate — the cross-account remote side
  renders as plain OwnerId/VpcId facts, a9s never pretends to see the
  other account — and the CloudTrail audit trail ("who accepted this").

### Fixed

- The web-UI e2e error-log expectation caught up with the v3.51.0
  honest-degradation contract: the demo session legitimately carries the
  denied-witness composite errors, so `!` opens the error-log screen.

## [3.52.0] - 2026-07-15

### Added

- New resource type: **EC2 Launch Templates (`lt`, aliases `launch-template`,
  `launchtemplate`, `launch-templates`, `lts`)** — the 69th type (main menu
  70). One extra call per template reads the `$Default` version — what ASGs,
  node groups and instances actually resolve at launch — funding every pivot
  and every warning. Two security warnings a fleet owner actually chases:
  `IMDSv1 allowed` (HttpTokens not `required` — and unset DEFAULTS to
  optional; templates with the metadata endpoint explicitly disabled are
  exempt) and `EBS encryption disabled` (explicit `Encrypted=false` only —
  nil is unknown in default-encryption accounts, never flagged). A `~`
  background check marks templates whose default version references a
  deprecated AMI, cross-referenced against the already-loaded AMI list with
  zero extra API calls.
- **Blast-radius related panel for templates**: which Auto Scaling Groups
  (plain, mixed-instances policy, and per-Overrides), which EKS node groups,
  and which running instances (via the `aws:ec2launchtemplate:id` auto-tag)
  launch from this template — plus the template's own AMI, KMS key, security
  groups (ids ∪ per-ENI groups) and pinned subnets. Truncated sibling lists
  answer `?`, never a fake zero. A template whose `$Default` version is
  denied keeps every listed field plus `details denied`.

### Fixed

- Transfer agreement details now show the RESOLVED AS2 IDs for both trading
  partners (previously the bare profile ids), and certificate findings —
  expired AND expiring — all reach the Attention section (previously at
  most one survived the fold). Detail enrichment now carries every finding
  for all types.
- OpenSearch domains listed by a role denied `es:DescribeDomains` degrade
  to `details denied` rows instead of vanishing with a hard error.
- Humanize-flagged columns render humanized on warm-cache rows too — a
  cached list no longer shows raw enums (`AMAZON_ISSUED`) where the live
  render shows `Amazon Issued`.
- Zooming out on a resource-level Cost Explorer drill no longer fires a
  multi-month resource query past the 14-day retention (which replaced the
  grid with a validation error) — the zoom stops at the retention boundary.

### Changed

- `make ready-to-release` is additive on top of `make ready-to-push`
  (integration + live read-only smokes only) instead of re-running the
  whole push gate.

## [3.51.0] - 2026-07-14

### Added

- New resource type: **Transfer Family (`transfer`, aliases `sftp`, `as2`,
  `ftps`)** — the 68th type (main menu 69). Lists servers and describes each
  in the same pass; status phrases per lifecycle state (`offline: not
  accepting transfers`, `start failed`, …) plus two operator warnings: a
  legacy security policy (weak ciphers / old TLS, denylist — FIPS/restricted
  variants never false-positive) and no activity logging at all. Nine
  related pivots — ACM certificate, Elastic IPs (internet-facing endpoints),
  Lambda authorizer, log groups, logging role, subnets, VPC, VPC endpoint,
  CloudTrail events — and seven navigable detail fields, including the
  lambda authorizer ARN via a new central lambda name extractor.
- **Agreements child view** (`e` on a server) — AS2 agreements with local /
  partner profiles and base directory; an INACTIVE agreement warns
  `inactive: partner traffic rejected`, and the agreement detail resolves
  each profile's `As2Id` and certificate expiry on demand (expired → Broken,
  under 30 days → Warning).
- **Rich degraded rows**: a server whose `DescribeServer` is denied keeps
  every field the list already returned — state finding included — and adds
  `details denied`, extending the v3.50.0 honest-degradation contract.
- **The availability sweep runs once per session per profile–region pair.**
  Switching profiles back and forth no longer re-probes every type and
  re-runs every background check — a revisited pair seeds instantly from the
  session's own disk cache. An interrupted sweep finishes on revisit, and
  `Ctrl+R` on the main menu always forces a full re-sweep.

### Fixed

- `y` (YAML), `J` (JSON) and `t` (CloudTrail) now work while the related
  column holds the cursor; `h` correctly returns focus to the detail body.
  The related-panel `/` filter is unaffected.

## [3.50.0] - 2026-07-14

### Added

- New resource type: **Managed Airflow (`mwaa`, alias `airflow`)** — the 67th
  type (main menu 68). Lists environments and describes each one in the same
  pass (the API returns names only), with status phrases per state, warnings
  for a silently failed last update and an internet-reachable webserver, and
  related pivots to CloudWatch alarms (by `EnvironmentName` dimension), KMS,
  the five per-component log groups, the execution role, the DAG source
  bucket, security groups, subnets, and CloudTrail events. 17 demo fixtures
  including a fully connected showroom environment.

### Changed

- **Honest degradation is now a fleet-wide contract.** A resource the list
  API names but the per-item describe cannot deliver (an IAM denial, a nil
  body) is KEPT as a name-only row with a `details denied` warning instead of
  silently vanishing — adopted by mwaa, EKS clusters, node groups, DynamoDB
  tables, and OpenSearch domains, with a single shared implementation so the
  behavior cannot diverge per type.
- Partial fetch success (rows plus a per-item composite error) no longer
  raises a blocking startup banner: the rows render with their degraded-state
  findings and the detail goes to the `!` error log. A row-less failure still
  banners. Related-panel target caches keep partially fetched rows instead of
  answering `?`.
- A service endpoint that does not resolve in the selected region (e.g.
  CodeArtifact in eu-central-2) now logs `service not available in region X`
  instead of raw DNS transport jargon.
- MWAA list pagination respects the API's `MaxResults` cap of 25 (the shared
  default of 50 was rejected with a ValidationException).
- The live smoke reads the `!` error log after its full walk: request-level
  errors fail the smoke, IAM denials print as loud warnings — previously a
  type whose fetch errored rendered no count and was silently skipped by the
  sweep.

## [3.49.1] - 2026-07-14

### Fixed

- `-c events` / `:events` no longer relies on registration order to resolve:
  the alias was registered on both CloudTrail events and EventBridge rules.
  It now belongs to CloudTrail events only (the documented behavior);
  EventBridge rules keep `eb-rule` and `eventbridge`. A catalog-wide gate now
  asserts every command name — alias or short name — maps to exactly one
  resource type.

## [3.49.0] - 2026-07-14

### Fixed

- Text screens (YAML/JSON views) regained four documented keys that had been
  silently dead since the controller migration: `y`/`J` (YAML↔JSON toggle),
  `t` (CloudTrail events for the resource), and `d` (open detail view). Only
  legacy tests had pinned them; they are now pinned on the live key path.
- YAML syntax coloring no longer corrupts keys containing colons (e.g. the
  EC2 tag `aws:autoscaling:groupName`): the colorizer split every line on the
  first colon, producing display text that was no longer valid YAML. The
  key/value splitter is now quote- and escape-aware, and the invariant
  "stripping colors yields the original text" is pinned by a regression test.

### Changed

- Codebase cleanup release: net ~37,700 lines removed with behavior pinned
  before every deletion. The legacy pre-controller view-model layer (dead
  self-render paths, constructors, and accessors superseded by the
  controller/ViewState architecture) is gone, along with its orphaned tests —
  every unique behavior those tests pinned was first ported onto the live
  seams (44+ new controller-path tests landed in the process).
- One truth source for list-column resolution: the identity/marker-column
  cascade previously existed in three near-identical copies (app list lane,
  runtime save lane, dead views mirror); all callers now delegate to a single
  shared `resource.ResolveListColumnCascade`.
- README generation consolidated on `cmd/readmegen` (the duplicate shell
  script is gone); `cmd/snapshot` uses SDK paginators instead of ~44
  hand-rolled pagination loops; 100+ copy-pasted client-assertion preludes
  and 50+ byte-identical related-resource wrappers collapsed into shared
  helpers; 110+ test-only all-pages fetch wrappers replaced by one test
  helper.
- Dependencies dropped: `testify` (with `go-spew`, `go-difflib`) and
  `gopkg.in/ini.v1` (AWS profile/region parsing now uses a minimal built-in
  scanner; behavior pinned by existing tests).
- Homebrew formula publishing no longer uses GoReleaser's deprecated `brews:`
  generator (#455): the release workflow renders `Formula/a9s.rb` from the
  release checksums and pushes it to the tap directly, validating the
  generation path before anything publishes. `brew install k2m30/a9s/a9s`
  stays a formula — no cask, no Gatekeeper quarantine dialog.

### Removed

- Dead one-shot design-mockup binaries (`cmd/preview-pagination`,
  `cmd/preview_detail`, `cmd/preview-policy-doc`, `cmd/preview/ct_event`),
  the never-wired Lambda code-download module, and assorted zero-caller
  functions verified individually before deletion.

## [3.48.0] - 2026-07-12

### Added

- Cost Explorer (`:costs` / `:ce`, `-c costs`, or the main-menu entry): a
  spend grid — services (or region / account / usage type / purchase option /
  charge category pivots via `1`-`6`) in rows, periods in columns, the
  invoice-mode amount in cells so column totals match the AWS invoice.
  `+`/`-` zoom between years, months, weeks, and days anchored at the
  cursor; `b` cycles cost metrics; Enter drills a cell down to usage types
  and (last 14 days, EC2) individual instances, and once more into the
  standard resource detail view. Cost anomalies mark their cells with the
  root cause and dollar impact in the footer; cells color by
  period-over-period change with a neutral band so spikes stand out from
  noise. Closed months are fetched once and cached on disk per profile
  (surviving restarts and rendering offline); only the current month
  refreshes, and `Ctrl+R` forces it. Available in the web UI with the same
  keys. Demo mode ships a planted growth story end to end.
- EC2 instances resolve by ID (`FetchByIDs`), so exact-ID drills from the
  related panel and the Cost Explorer land on the instance detail directly.
- IAM policy availability probes on group-heavy accounts no longer time out:
  the probe counts managed policies only, the inline group-policy sweep runs
  with bounded parallelism, and partial-failure logs summarize instead of
  enumerating every group.

### Changed

- Cost-screen decisions live in an explicit domain state machine
  (`internal/costs/screen`): fetch planning, drill transitions, window
  construction, and the computed view are pure functions with typed
  outcomes; fetch results carry their own authority (a skipped fetch can
  never masquerade as authoritative data), and money keeps its currency
  unit until a total is proven single-currency — mixed-currency grids
  suppress the total with an explanation instead of summing units.
- Failed by-ID drills travel as one typed message in both the terminal
  and web lanes; the placeholder pops only on an exact target match
  (previously any error while an empty list was on top could navigate
  the user away).
- Region-less charges ("NoRegion") drill correctly: Cost Explorer
  filters them by empty string while reporting them as "NoRegion" — the
  translation now happens at one seam, and a drill whose finer
  granularity has no records falls back once to the parent granularity
  instead of rendering silent zeros.

### Fixed

- A cost delivery landing during a background cache save could crash the
  process (concurrent map access) — the store now owns its own lock.
- `--no-cache` and demo sessions keep the cost cache in memory only.
- Cost rows rank by the visible window; day-level drills cover exactly
  the selected day; year drills cover the selected year's months;
  drilling before AWS connect completes recovers automatically when
  clients arrive.

## [3.47.0] - 2026-07-10

### Added

- Wave-2 enrichers emit a slice of findings per resource; each problem is its
  own Attention entry (OpenSearch surfaces "update forced" and "encryption at
  rest off" separately), keyed by finding code.
- Child views join the findings-color doctrine — unhealthy targets color
  themselves and name the cause in lowercase; web child lists match the TUI and
  Enter opens child views from the browser.
- The menu issue badge shows `+` when the count is truncated.
- Demo mode gained dozens of witnesses (ecs-task/MSK/API Gateway/CloudFormation
  lifecycles, execution-history failures, 34+ finding codes) so every color
  bucket and most related-panel pivots have a live demo witness.

### Changed

- Related navigation resolves through one shared path for the TUI and web
  renderers — the same rows and the same fetch on both surfaces.
- CloudTrail-event, networking, RDS instance→cluster, and five forward
  IAM-role pivots resolve their counts from event data with no extra list
  fetch.
- The related-panel four-state row contract (`(N)` / `(N+)`·`(0+)` / `(0)` /
  blank drill-in) is enforced everywhere; the ambiguous `(?)` row is gone.

### Fixed

- Truncated related rows (`(N+)`/`(0+)`) are navigable — Enter opens the scoped
  list and keeps discovering matches as pages load; `(0+)` behaves identically
  to `(N+)`.
- A truncated single-match pivot opens the scoped scan list, not one detail.
- A cache-hit filtered related list renders lazily-fetched (by-ID) rows instead
  of appearing empty; the web no longer navigates a related row into an empty,
  stuck list.
- A popped related list no longer leaks its scan filter onto later lists of the
  same type.
- ACM certificates are keyed by ARN — multiple certificates sharing one domain
  no longer collide in the related panel.
- CloudTrail-event pivots resolve named targets by identity (never a scoreless
  `(0+)`) and drill into the correct target; blackhole route targets are
  skipped; secrets/ECR role pivots emit role names, not ARNs; IAM policy
  lazy-add resolves by name.
- Issue counts are consistent — a list title counts issues like the menu badge
  (per resource); `~`-only enrichers no longer lower-bound the badge; failed
  batches surface a per-row `?` or error.
- Detail: Tab focus lands on the first drillable pivot; the viewport follows
  the field cursor; nested CloudTrail REQUEST/RESPONSE render as compact JSON.
- The availability sweep waits for AWS client readiness; a profile/region
  switch can no longer race the availability save loop.

## [3.46.0] - 2026-07-07

### Changed

- **One Status column everywhere** — every list view has exactly one status
  column, titled `Status`; the duplicate `State`/`Issues`/`Risk`/`State
  Reason`/`Logging` columns are gone. The cell renders the row's finding
  phrase, else its humanized lifecycle state; no raw AWS enum reaches any
  rendered cell.
- **Row color derives from findings** — classifiers resolve color
  findings-first (worst severity wins); color, status text and the detail
  Attention block share one source, so a colored row always explains
  itself. Enforced by an empty-allowlist conformance gate over the demo
  bench.
- **Finding phrases are operator sentences** — cause, not severity or
  benchmark citations ("reads sensitive data (ListSecrets)", "deletion
  protection disabled", "access key >90d old").
- **One row store** — the five in-memory copies of per-type list rows
  (probe buffer, session caches, controller mirror, screen rows) collapse
  into a single session-scoped `RowStore`; screens adopt what the store's
  reconciler accepts, both save lanes share one materializer with the
  user's column config honored, and a conformance test bans new shadow
  row maps at the source level.
- **Related-panel contract hardened** — 24 pivots that returned zero or
  garbage on any data now implement their documented mechanisms (five new
  AWS API integrations); 21 structurally-uncomputable pivots were removed
  from the registry under the one-extra-call policy rule; checkers return
  IDs the drill-down can actually resolve.

### Added

- Demo smoke joins `make ready-to-push`; a live readonly smoke
  (`make smoke-live`) sweeps every non-empty type in the account for raw
  enums and vacuous statuses.
- Dynamic witness gate: a registered finding that never fires on a demo
  row is a coverage failure.
- **Persistent cache survives restarts in both UIs** — every list column,
  status text and `!`/`~` issue glyph renders in the first frame from the
  per-type cache files and is silently re-verified in the background; a
  Wave-1 refresh can no longer strip previously learned findings from the
  file or the screen (Wave-2 carry rule C6b).
- **Machine findings registry** — every resource type declares its complete
  finding inventory (code, phrase, severity, wave) on the catalog; the
  findings tables in `docs/resources/*.md` and `docs/attention-signals.md`
  are generated from it.
- **Demo mode is a full verification bench** — background checks run in
  demo; every registered related-panel pivot has a live witness; every
  witnessed ID resolves on drill-down; issue-carrying fixtures exist for
  every issue-capable type; per-type color-state coverage is inventoried
  by ratchet tests that only shrink.
- **Filtered related drill-downs cache per session** — re-entering e.g.
  CloudTrail Events from a key renders instantly from the session cache
  with a background refresh instead of a bare full-screen Loading.

### Fixed

- ebs orphan/unencrypted volumes, ebs-snap and dbc-snap unencrypted/orphan/
  aged snapshots emit findings — yellow rows carry a cause in Status and
  Attention.
- elb: warn-level misconfiguration no longer renders red.
- opensearch: the epoch zero date no longer counts as a forced-update
  deadline; encryption-at-rest has its own sentence.
- ec2 → IAM Role related pivot resolves through `GetInstanceProfile`
  (profile name is not role name).
- Availability sweep waits for AWS client readiness; the menu issue badge
  counts findings from a single source.
- EC2 status-check glyphs restored on the Status column.
- KMS color states never worked (classifier read a field the fetcher never
  wrote); console users without MFA now classify Broken as documented;
  Lambda state precedence follows the spec (Inactive dims before the
  missing-DLQ warning); CloudTrail emits findings for all four documented
  conditions including stale delivery.
- Related panel: all zero-count rows dim consistently; Tab lands on the
  first actionable row; cursor movement skips dead ends; count badges
  render on the legacy TUI lane; results without display names bind to
  their row instead of appending phantom rows.
- Sort order, filter and cursor position survive list re-entry.
- Render tests are hermetic: one deterministic color profile for the
  whole suite regardless of the invoking terminal.

## [3.45.0] - 2026-05-12

### Changed (BREAKING — resource type renames, no aliases)

- **`rds-snap` → `dbi-snap`** — the RDS DBSnapshot resource type was
  renamed to match its parent shortName (`dbi`). The shortName is the
  primary identifier for views, themes, scripts, etc; users who scripted
  against the old name must update. No backward-compat alias is provided.
- **`docdb-snap` → `dbc-snap`** — the cluster-snapshot resource type was
  renamed to match its parent shortName (`dbc`). This resource type covers
  BOTH DocumentDB cluster snapshots AND Aurora cluster snapshots — they
  share the AWS API (`DescribeDBClusterSnapshots`). Display name is now
  `DB Cluster Snapshots`. No backward-compat alias.
- **Display names** — `RDS Snapshots` → `DB Instance Snapshots`,
  `DocDB Snapshots` → `DB Cluster Snapshots`. Aligns with parent display
  names (`DB Instances` / `DB Clusters`).

### Added

- **Generic `SnapshotCrossRef` enricher helper**
  (`internal/aws/snapshot_cross_ref.go`) parameterized by parent shortName,
  parent-ID extractor, parent-retention extractor, and a retention-rule
  flag. Covers the orphan + past-retention pattern shared across snapshot
  types. Both `dbi-snap` and `dbc-snap` enrichers are now thin
  configuration wrappers (~60 lines each); the future `ebs-snap` consumer
  can disable the retention rule (`ec2.Volume` has no
  `BackupRetentionPeriod`).
- **`dbc-snap` cross-ref enricher activated** — was `NoOpIssueEnricher`.
  Orphan and past-retention signals now fire for both DocumentDB and
  Aurora cluster snapshots whose parent cluster is missing or past its
  declared retention period.
- **`dbc-snap` drill-through coverage** —
  `tests/integration/scenario_related_drill_through_test.go` now includes
  Aurora and DocumentDB graph-roots; the `dbc-snap → backup` pivot was
  rewritten to use cache scan + plan-IDs (was returning recovery-point
  ARNs that didn't drill).

### Removed (no aliases)

- The `rds-snap → dbc` (now `dbi-snap → dbc`) related-pivot registration
  and its checker are gone. Aurora cluster snapshots are not stored as
  `DBSnapshot`s in real AWS (`CreateDBSnapshot` is rejected on Aurora
  cluster members), so the pivot had no realistic non-zero case. Aurora
  cluster snapshots live in `dbc-snap`.
- The bogus `ProdRDSSnapAuroraID` / `ARN` demo fixture and its associated
  backup recovery points. CodeRabbit flagged this in a prior review; we
  waved it away with a "legal at SDK type level" comment. The fixture
  shape is rejected by real AWS — drop it. The `dbi-snap` graph-root is
  now `ProdDBISnapID`.

### Fixed

- **Retention threshold reconciled to 1.0×** for both `dbi-snap` and
  `dbc-snap`. The previous `dbc-snap` 1.5× multiplier was authoring drift —
  `BackupRetentionPeriod` IS the operator's declared retention policy; any
  snapshot kept past it is policy drift regardless of engine. The N4
  "intentional divergence" note in the spec doc has been deleted.
- **`dbc-snap → backup` pivot** now resolves plan IDs via cache scan
  instead of recovery-point ARNs via API call. Drilling into the pivot
  lands on a non-empty backup-plan list (was empty).
- **`dbc-snap` fetcher now populates `Resource.Issues` and computes the
  §4 phrase via `ComputeDBCSnapStatusAndIssues`** — mirroring the
  `dbi-snap` pattern. Adds `failed`, `incompatible-*` (Broken),
  `creating` (Warning), and `manual age > 365d` (Warning) signals.
  Previously the fetcher set `Status` to a raw AWS keyword passthrough
  and left `Issues=nil`, breaking universal rule U7f and the detail-view
  Attention section.
- **`computeMergedStatus` reverted to honor the fetcher's §4 phrase** —
  removed a defensive gate added in ab1d7c1 that masked the missing-Issues
  bug above by silently letting cross-ref phrases override
  fetcher-emitted Broken phrases (a `failed + orphan` row would render
  as just `orphan: source cluster deleted`, losing the Broken signal).
  The fetcher contract (Issues populated for every active Wave-1 phrase)
  is now load-bearing; the helper trusts it.
- **`SnapshotCrossRefConfig.OrphanRowLabel` renamed to `ParentRowLabel`** —
  the field is used both for the orphan citation row AND the
  past-retention parent-cite row; the original name implied
  orphan-specific use.
- **dbc-snap fetcher now calls both DocDB and RDS SDKs and merges results** —
  reverses an earlier wrong claim that the DocDB-side
  `DescribeDBClusterSnapshots` returned Aurora rows via a "shared backend".
  AWS SDK docstrings are explicit: docdb-side is DocDB-scoped
  (docdb@v1.48.12/api_op_DescribeDBClusterSnapshots.go:14); rds-side
  covers Aurora + Multi-AZ
  (rds@v1.116.3/api_op_DescribeDBClusterSnapshots.go:19-25). Without the
  merge, every Aurora cluster snapshot would (a) be invisible to a9s
  entirely, or (b) once visible via demo fixtures only, register as orphan
  in real accounts because the dbc parent cache was DocDB-only too. Same
  merge applied to the dbc parent fetcher. `dbc_snap_issue_enrichment.go`
  and `dbc_snap_related.go` regain the `rdstypes.DBClusterSnapshot`
  extractor branches that were wrongly deleted in commit a07331e.
- **`checkDbcSnapBackup` truncation handling** — when the dbc cache is
  truncated AND the parent ARN cannot be resolved from the visible
  window, the checker now returns `UnknownRelated("backup")` instead
  of `Count: 0`. Previously the pivot lied with a definitive zero
  for snapshots whose parent cluster fell past page 1 of `dbc`.
- **`dbc → subnet` and `dbc → vpc` pivots now resolve correctly for Aurora
  clusters** — the subnet-group lookup now dispatches by RawStruct shape:
  Aurora rows (`rdstypes.DBCluster`) call `c.RDS.DescribeDBSubnetGroups`,
  DocDB rows (`docdbtypes.DBCluster`) call `c.DocDB.DescribeDBSubnetGroups`.
  Previously the DocDB-only call returned empty for Aurora rows.

### Spec

- `docs/resources/dbc-snap.md` §3.1 + §4 now list the `incompatible-*`
  Broken signal (defensive parity with the documented `DBSnapshot`
  status family).

### Internal (Phase-05 architecture refactor)

- **`internal/runtime` package** — platform-agnostic app core (`Core`,
  handlers, probes, fetchers, orchestrator) extracted from `internal/tui`.
  `tui.Model` shrunk from ~1200 LOC to ~420 LOC; all behavior preserved.
- **`session.Session` owns all session-scoped mutable state** — resource
  cache, related cache, enrichment findings, generation counters, identity.
  Accessed via `m.core.Session()`; `Rotate()` invalidates in-flight gens
  on profile/region switch. Replaces the former `sessionRuntime` embed.
- **`domain.Gen` unified generation counters** — `AvailabilityGen`,
  `EnrichmentGen`, `ConnectGen` are now typed `domain.Gen` values with
  shared `IsStale` / `Stamp` helpers (AS-73).
- **Typed Cmd/Event message taxonomy** — `runtime/messages/cmd.go` (user
  intent) and `event.go` (async results) with `GenStamped` embed for
  compile-time gen-guard guarantees (AS-74).
- **`KindFetchMore` carries token via `FetchMorePayload`** — pagination
  token bundled with type + gen to eliminate token-mismatch races (AS-270).
- **Wave-2 enrichers migrated to Findings API** — database enrichers no
  longer write `FieldUpdates["status"]`; they emit `Findings` with
  `Source="wave2"`. The status column applies a two-layer priority rule:
  Wave-1 issue phrases first, Wave-2 findings appended only when no Wave-1
  issue is present (AS-140).
- **Glyphs only on `ColorHealthy` rows** — `"! "` and `"~ "` decorators
  are applied only when `ResolveColor(r) == ColorHealthy`; non-Healthy
  rows are colored instead.
- **S3 cross-region related-defs soft-truncate to 0+** — out-of-region
  buckets show empty count instead of error flash (AS-489).
- **Related-panel literal `(N)` count suffix** — avoids ANSI-width
  miscalculation when Lipgloss measures badge glyphs (AS-378).
- **`verify-readonly` Makefile grep widened** to cover `internal/runtime/`
  (AS-236).
- **DBC/DBC-snap dual-API dedup** — rows appearing in both standard and
  Aurora-specific API responses are deduplicated (AS-145).

## [3.44.0] - 2026-04-25

### Changed

- **Partial-success contract end-to-end** — paginated fetchers, the synchronous demo prefetch, the per-type availability probe, and Wave-2 enrichers all now preserve partial results when an error accompanies them. `ResourcesLoadedMsg`, `AvailabilityCheckedMsg`, and `EnrichmentCheckedMsg` carry the partial slice + composite error simultaneously; the app surfaces the error via `FlashMsg` while still applying the partial state. Previously a single per-item failure inside any of these layers blanked the menu badge or list view for that resource type.
- **Never-silent-skip rule** — every AWS SDK call across the codebase now wraps in `RetryOnThrottle` and surfaces per-item failures through the operator-visible error channel. Previously, ~50 files silently swallowed per-item describe/get errors, so a resource list could look complete while permission or throttling issues silently degraded the view. Operators now see aggregated composite errors via the `!` error log.
- New shared helper `internal/aws/partial_errors.go` (`AggregateFailures`, `AggregateMissing`) standardizes composite-error formatting across fetchers, related checkers, and Wave-2 enrichers.
- Wave-2 enrichers now set `Truncated`/`TruncatedIDs` (per-row `?` marker) AND return an aggregated top-level error (feeds `EnrichmentCheckedMsg.Err` → FlashMsg) — both channels fire on per-item failure.
- Related-panel checkers (`*_related.go`) set `RelatedCheckResult.Err` to the aggregated composite; the app layer converts it to `FlashMsg{IsError:true}` so the error log captures the reason the pivot shows `?`.
- Top-level fetchers (`backup.go`, `dbc.go`, `ddb.go`, `efs.go`, `eks.go`, `kms.go`, `ng.go`, `opensearch.go`, `redis.go`, `redshift.go`, `s3.go`, `sg.go`, `sqs.go`, plus the lazy-add fetchers) aggregate per-item describe failures; `ResourcesLoadedMsg.Err` already surfaces as a flash.
- ECS-task fetcher decouples join-failure signaling from `Pagination.IsTruncated` — per-task `Fields["task_def_join_error"]="true"` marks affected tasks; `checkEFSECSTask` reports `Approximate=true` without the fetcher advertising a bogus "m: load more" footer.
- `a9s-implement-resource` skill: new **Error handling and throttle rules** section (E1–E6) with banned-pattern list, category/surface-channel table, and enforcement hooks in phases 5, 7.5, 8, 9. Coverage matrix gains U12 (partial-failure FlashMsg) and U13 (throttle-wrap static audit).
- Lazy-add cache snapshot now marks lazy-only entries as `IsTruncated:true` (sparse, not authoritative). The related-panel prefetch decision uses a `mainCacheKeys` set built from `resourceCache` only — a lazy-only entry no longer suppresses the real first-page fetch when a `NeedsTargetCache:true` checker scans it.
- IAM policy `FetchIAMPoliciesByIDsFull` builds also index entries by ARN alongside `PolicyName` so checkers that emit ARN form (e.g. role → managed policy) drill into the cached row.

### Added

- `tui` package split into smaller files to keep each under the 500-line file-size budget: `app_probes.go` (availability + enrichment probes), `app_handlers_availability.go` (their handlers + `unifiedIssueCount`), `app_handlers_related_navigate.go` (`handleRelatedNavigate` and child variant), `related_cache.go` (LRU + replay helpers).
- `EnrichmentCheckedMsg.Err` (existing) and new `ResourcesLoadedMsg.Err` carry partial-success errors alongside resources.
- Lazy-only fast path in related navigation now requires ALL requested IDs to short-circuit (`len(filtered) == len(result.RelatedIDs)`); partial coverage falls through to a full fetch instead of rendering only the lazily-cached subset.
- Lazy-add path for related-panel drill-through: when a checker emits IDs outside the top-level fetcher's scope filter (KMS customer-managed, AMI `Owners=self`, EBS snapshot `OwnerIds=self`, IAM Policy `Scope=Local`), the orchestrator calls `FetchByIDs` to resolve them and populates `lazyResourceCache`. Drilling into a KMS pivot for an `aws/rds` key, an AMI pivot for a public marketplace AMI, or an IAM role's `AdministratorAccess` now lands on a real entry instead of an empty list.
- `lazyResourceCache` session map (separate from `resourceCache`) consulted only by related-navigation, never by main-menu top-level list. Prevents lazy-added out-of-scope entries from polluting the scope-filtered top-level view.
- `ResetIAMPoliciesCache()` exported and wired into `sessionRuntime.resetForSessionSwitch` — IAM policy memoization now clears on profile/region switch so account-A policies never leak into account-B drills.
- `RelatedCheckResultMsg.LazyAddError` field routes FetchByIDs failures into FlashMsg without masking partial successes.
- `AvailabilityPrefetchedMsg.PrefetchErr` surfaces per-type failures from the synchronous availability prefetch in no-cache / demo mode.
- User-story set `tests/stories/lazy_add.md` — 44 given/when/then stories across 9 sections covering cross-scope drill-through, session lifecycle, idempotence, race/timing, and size extremes.

### Fixed

- `probeEnrichment` no longer drops the `IssueEnricherResult` when the enricher returns a composite error alongside partial findings — Wave-2 menu badges, FieldUpdates, and per-row `?` markers now survive across per-item failures.
- `probeResourceAvailability` no longer drops the partial resource list when the fetcher returns a composite error — the menu count + `Issues` badge update from the partial slice and a FlashMsg surfaces the error.
- `handleEnrichmentChecked` and `handleAvailabilityChecked` now apply state changes when the message carries partial success (Err+Resources/Findings populated). Pure failure (Err alone) still leaves the menu entry as "unknown".
- `FetchAMIsByIDs` now sets `IncludeDeprecated: true` to match the single-ID `FetchAMIByID` path; deprecated AMIs referenced from EC2 / ASG / EKS pivots no longer silently vanish from batch drills.
- `efs_issue_enrichment` uses the typed `efstypes.LifeCycleStateAvailable` constant instead of the string literal `"available"` for safer future SDK upgrades.
- `EnrichEC2InstanceStatus` no longer panics when called with a nil `*ServiceClients` — `probeEnrichment` re-instates the nil-clients guard and returns a clear "AWS clients not initialized" error.
- `handleEnrichmentChecked` no longer silently swallows `EnrichmentCheckedMsg.Err` — failures now emit `FlashMsg{IsError:true}` so the error log (`!` key) captures them.
- `handleRelatedNavigate` cache-hit path now consults `lazyResourceCache` alongside `resourceCache`, so a drill through a cache-hit pivot finds lazy-added out-of-scope targets.
- ENI list no longer renders `m: load more` on a fully-resolved exact-ID filter — `handleRelatedNavigate` strips `IsTruncated` on the pagination passed to the list view when every `RelatedIDs` entry matched.
- `FetchKMSKeysPage` `DescribeKey` and `ListAliases` per-item failures now aggregate into a composite returned error instead of silently skipping keys or stopping the alias-page loop.
- `FetchDynamoDBTablesPage` per-table `DescribeTable` failures aggregate into a composite error (previously: silent `continue`).
- `sfnDescribe` signature returns `(out, err)` — `checkSFNRole`, `checkSFNKMS`, `checkSFNLambda`, and `ecs-svc` cross-references now set `Result.Err` on API failure instead of collapsing to bare `Count=-1`.
- `redshiftLoggingStatus` signature returns `(status, err)` — `checkRedshiftLogs` and `checkRedshiftS3` surface the underlying error via `Result.Err`.
- `checkEbTG` per-LB `DescribeLoadBalancers`/`DescribeListeners` failures aggregate into `Result.Err` (previously silent `continue`, undercounting TG relationships).
- `secrets-related: DescribeTaskDefinition` treats ECS `ClientException` ("task definition does not exist") as definitive absence rather than a real error — mirrors the `ecs_task.go` carve-out so demo fixtures and real environments with soft-deleted task definitions don't spuriously flash.
- Enricher tests pinning the old `err == nil` silent-skip contract updated to accept the new aggregated-error contract across ASG, CFN, ECR, ECS-svc, EFS, Logs, MSK, R53, SFN, SQS, TG, TGW, WAF, plus EKS top-level fetcher.

## [3.43.0] - 2026-04-24

### Changed

- Redis, DynamoDB, DBI (RDS instance), DBC (RDS cluster), S3, SES, and Backup resources re-implemented from spec (#295, #296) — spec-compliant §4 phrases, Wave-2 findings, graph-root pivot accuracy
- Detail view: unified `Attention (N)` section replaces per-type issue sections (Pending Maintenance, Target Health, Latest Build, etc.) — one consistent format for every resource type
- Redis fetcher now lists `ReplicationGroup` (was `CacheCluster`); engine filter excludes Valkey/Memcached
- Backup enricher surfaces Wave-2 job-state findings via the unified Status column instead of a parallel column

### Added

- Shard-level Wave-1 phrases for cluster-mode-enabled Redis (e.g. `shard ng-1: modifying (+2)`)
- Table-driven drill-through integration tests pin every registered related-panel pivot + navigable field across nine graph-root fixtures
- Central `NavIDFromValue` registry normalizes ARNs to bare IDs at navigation time for KMS, IAM role, ECS cluster, CloudWatch Logs, S3, and IAM user
- `ApproximateZero` / `Approximate=true` contract for truncated reverse-scan related checkers (DDB, S3, SES, Redis)
- Backup plan coverage detection honors `arn:aws:<service>:*:*:<type>/*` wildcards, `NotResources` exclusions, and tag-based selections (#296)
- API Gateway REST v1 APIs now appear in the `apigw` list alongside HTTP/WebSocket APIs
- API Gateway detail view resolves ACM certificates attached via custom domain mappings

### Fixed

- Navigable fields in detail view now land on the actual resource — highlighting a KMS ARN, role ARN, bucket name, or ECS cluster ARN and pressing enter previously produced empty landing lists
- SES `DescribeActiveReceiptRuleSet` cache clears on profile/region switch — previously stale Lambda/S3 related-panel counts persisted after `Ctrl+R` or profile switch
- SES related pivots: lambda/eb-rule ID-format mismatch, kinesis type mismatch, Ctrl+R staleness (#295)
- S3 related-panel joins now use boundary-safe resource-ID matching (no more substring collisions)
- Attention entry color capped at row's S2 color bucket (no more green glyphs on red rows)
- Redis color function matches §4 phrases via `StripFindingSuffix` so the Status column paints correctly under multi-finding rows
- EKS clusters whose `DescribeCluster` call fails now surface as a `DescribeFailed` row instead of silently vanishing from the list
- KMS paginated fetcher fully paginates `ListAliases` so customer-managed keys on later alias pages no longer render with a blank alias
- API Gateway enricher emits the documented "no deployed stages" warning when `GetStages` returns empty, matching `docs/attention-signals.md`

## [3.42.0] - 2026-04-14

### Added

- Inter-navigation between detail, JSON, and YAML views — press `d`/`y`/`J` to switch freely between them without going back first (#269)
- Sort by column position with `1`–`0` keys — pressing the same key toggles sort direction (#267)
- Error log view with `!` key — shows session errors with timestamps (#268)

### Fixed

- View-switch keys (`d`/`J`) are blocked in raw-text viewer mode (error log) to prevent navigation with empty resource context (#269)

### Changed

- Architecture guide updated with all recent features: full message catalog, ReplaceCurrent navigation pattern, sorting, error log, resource type categories

## [3.41.0] - 2026-04-13

### Added

- `:root` / `:main` colon command to navigate back to the main menu from any view depth (#258)
- COMMANDS section in all help screens showing available colon commands (`:q`, `:ctx`, `:profile`, `:region`, `:theme`, `:help`, `:root`, `:main`, `:<resource>`) (#258)
- Tab completion for `:root` and `:main` commands (#258)
- Auto-detect and pretty-print JSON in detail view field values — top-level and sub-field JSON strings expand as indented YAML sub-fields (#262)
- Syntax-colored JSON pretty-printing in secret/parameter reveal view (`x` key) with raw copy preserved (#262)
- AWS tags render as flat `Key: Value` pairs in detail views instead of verbose `Key/Value` struct fields; rich tag structs (e.g., ASG with PropagateAtLaunch) are left unflattened (#210)

## [3.40.0] - 2026-04-13

### Fixed

- Detail view nested structures (arrays, maps, sub-objects) now render with proper YAML hierarchy instead of flat 5-space indentation (#265)
- Bare YAML list items like `- '*'` no longer misrendered as key:value pairs in detail view
- Fields-map and RawStruct data sources now produce identical YAML list format for array sections (e.g., SecurityGroups)
- Detail search matches are cursor-position-independent — canonical `": "` spacing on all sub-field types
- EC2 status-check sub-fields store raw values; cursor row no longer leaks ANSI color escapes

### Changed

- `make test` runs without `-race` for fast local iteration (4s); `make test-race` added for pre-push race detection
- Unit test suite optimized: removed real timer waits, replaced full-app journeys with direct model setup, sampled representative resource types in exhaustive sweeps (86.3% coverage maintained)

### Added

- Shared YAML line tokenizer (`yaml_line.go`) ensuring markers and spacing stay identical across cursor states and views
- `"impaired"` and `"initializing"` status colors in row color cache for EC2 status checks

## [3.39.0] - 2026-04-12

### Added

- Auto-create default view config files (`~/.a9s/views/*.yaml`) on first launch — no source checkout needed to customize views
- Auto-create and auto-refresh `~/.a9s/views_reference.yaml` field reference on each launch — always up-to-date with the binary version
- `--reset-views` flag to delete all view configs and regenerate defaults on next launch (with confirmation prompt)
- `--reset-themes` flag to delete all theme files and regenerate defaults on next launch (with confirmation prompt)
- Synthetic child view entries (`lambda_invocations`, `lambda_invocation_logs`, `pipeline_stages`) in `views_reference.yaml`
- View Customization and Color Themes wiki pages

## [3.38.0] - 2026-04-12

### Added

- JSON view for raw resource data (`J` key). Opens from resource list and detail views, complementing the existing YAML view (`y`). Marshals the AWS SDK response struct directly via `json.MarshalIndent` with 2-space indentation, preserving native JSON types (booleans, numbers, nulls) for direct use in AWS CLI `--cli-input-json`, jq, and other tooling. Supports search (`/`), scroll, line wrap (`w`), copy (`c`), and CloudTrail jump (`t`). Help screen (`?`) lists the new binding.

### Fixed

- Lambda invocation logs fetcher now paginates through empty CloudWatch `FilterLogEvents` pages. Previously made a single API call, returning zero results whenever matching events lived in a later log stream page.

## [3.37.0] - 2026-04-12

### Added

- `-c` / `--command` CLI flag to open a resource list directly on startup, skipping the main menu (k9s-style). Accepts any resource short name or alias (e.g. `a9s -c ec2`, `a9s -c events`). Combinable with `-p`, `-r`, `--demo`, `--no-cache`. Resolves via `resource.FindResourceType`; unknown names fail fast with exit 1 before the TUI starts.

### Fixed

- Auto-navigation from `-c` is guarded to startup-only: if the initial AWS connection is slow and the user navigates away from the main menu before `ClientsReadyMsg` arrives, the flag is silently consumed without pushing a view on top of the user's current screen.

## [3.36.1] - 2026-04-11

### Fixed

- `cache.Save` is now crash-atomic and concurrent-safe. Previously wrote directly to the target path with `os.WriteFile`, which truncated the file mid-write — concurrent readers could observe a zero-length cache, and two a9s processes sharing the same `~/.a9s/cache/<profile>--<region>.yaml` raced on a deterministic `.tmp` name and silently lost updates. Save now uses `os.CreateTemp` for a unique per-invocation temp file, followed by `os.Rename`.
- `cache.Path` now sanitizes backslash in addition to `/` and space. On Windows, profile names containing `\` could previously create unintended subdirectories.
- App context cancellation now fires reliably on exit. `defer model.Cancel()` was added around `tea.Program.Run` via a `main` → `runProgram` refactor, so in-flight AWS fetcher goroutines are cancelled on normal return, error path, and panic — previously only normal `QuitMsg` dispatched the cancel.
- `ClientsReadyMsg` with an unexpected `Clients` type now surfaces an internal error via `APIErrorMsg` instead of silently falling back. The demo-mode `nil` path still correctly routes to pre-supplied clients.

## [3.36.0] - 2026-04-10

### Added

- Configurable color themes: 11 built-in themes (Tokyo Night Dark/Light, Catppuccin Mocha/Latte, Dracula, Nord/Nord Light, Gruvbox Dark/Light, Solarized Dark/Light) shipped as embedded YAML files, auto-extracted to `~/.a9s/themes/` on first run.
- `:theme` command opens a selector overlay (same UX as `:region`) for runtime theme switching with immediate re-render and config persistence.
- `~/.a9s/config.yaml` configuration file for app-level settings, starting with `theme:` key.
- Custom themes: copy any built-in YAML file, edit colors, point config at it. Partial themes inherit missing colors from the Tokyo Night Dark default.
- Active theme name displayed in help view (`?`).
- Path traversal protection on theme filename validation (rejects `../`, absolute paths, path separators).

### Changed

- Extracted 33 hardcoded Tokyo Night Dark palette colors into a `Theme` struct with `DefaultTheme()`, `ApplyTheme()`, `ThemeFromYAML()`, and `ActiveTheme()` API.
- Migrated 13 package-level style captures from 4 view files (help, identity, yaml, search) into centralized `styles.go` composed styles, rebuilt on every `ApplyTheme()` call.
- `NoColorActive()` now correctly returns `true` for `NO_COLOR=` (empty value) per the NO_COLOR spec. Previously required a non-empty value.
- `:theme` command persists before applying — if config save fails, the change is aborted and the user sees an error flash instead of a silent partial state.
- Theme file operations guard against missing config directory (`$HOME` unset, no `$A9S_CONFIG_FOLDER`) — no accidental writes to the working directory.
- `:theme` selector always marks the active theme as "(current)", including the default Tokyo Night Dark on first run with no config.

## [3.35.0] - 2026-04-10

### Added

- CloudTrail Events related view extended to all 66 resource types. Every resource now shows "CloudTrail Events" in the right-column related panel with correct per-service filters.
- `t` keyboard shortcut for direct CloudTrail Events navigation from resource list, detail, and YAML views.
- Deterministic `CloudTrailKey` per-type configuration on `ResourceTypeDef` — no heuristics, no reflection. Each resource type explicitly declares its CloudTrail lookup attribute and value source.
- ARN-based CloudTrail filters for Lambda, RDS, EKS, Secrets Manager, DocumentDB, and SQS (types where CloudTrail indexes by ARN, not name).
- SQS resources now expose `Fields["arn"]` extracted from queue attributes.
- 9 new demo CloudTrail fixture events covering Lambda, RDS, ECS, DynamoDB, Secrets Manager, EKS, and CloudFormation.
- Demo CloudTrail fake now supports suffix matching on ResourceName (bare name matches ARN-prefixed events).
- `t` / `cloudtrail` added to help overlay (`?`) for resource list, detail, and YAML views.
- `TestFullRelatedViewValidation` comprehensive integration test: validates all related-resource entries and navigable fields across all resource types in both demo and live AWS modes (1,133 demo subtests, 610 live subtests).
- 15 live integration scenario tests for CloudTrail `t` key against real AWS.
- 40+ new unit tests covering filter logic, key handlers, hints, help overlay, coverage gaps.

### Changed

- File splits: `resourcelist.go`, `detail.go`, `app_handlers.go`, `help.go` each split into two files to stay under 500 lines per the codebase checklist.
- YAML view (`NewYAML`) now accepts `resourceType` parameter for child-type detection.
- `t` hint and key suppressed on ct-events lists (no self-reference), child resource lists, child detail/YAML views, and types with no CloudTrail support.
- Pre-push rules now require `TestFullRelatedViewValidation` against a real AWS profile before any push.
- Test count updated from 4,400+ to 4,500+ in README and website.
- `website/content/resources.md`: CloudTrail Events short name corrected to `ct-events`.

### Removed

- Reflection-based ARN extraction (`extractARNFromRawStruct`) replaced by deterministic `CloudTrailKey` config.

## [3.34.0] - 2026-04-10

### Changed

- Demo mode (`--demo`) fully migrated from legacy HTTP transport to per-service typed fakes. All 66 resource types now served by 42 typed fakes in `internal/demo/fakes/` backed by SDK-typed fixture data in `internal/demo/fixtures/`. Deleted the legacy fixture stores (`demoData`, `childDemoData`, `RegisterRelatedDemo`), all `fixtures_*.go` category files, all `handlers_*.go` transport handlers, and the parallel demo-related registry. Only the STS handler remains for `GetCallerIdentity` probes.
- IAM Policies list now includes inline group policies alongside managed policies. Replaced "Policy ID" column with "Type" (managed/inline). Related panel for inline policies shows the parent group.
- CloudTrail navigable fields fixed: AWSService events no longer pollute `Fields["user"]` and `Fields["role_name"]` with service principals; AssumedRole events no longer copy role names into the user navigation field.
- Demo main menu now shows resource counts on startup (availability probes run in noCache mode).
- Main menu command hint shows first alias (e.g., `:event`) instead of canonical short name (`:ct-events`).
- Go toolchain bumped from 1.26.1 to 1.26.2 (fixes 4 crypto/x509 + crypto/tls stdlib vulnerabilities).

## [3.33.0] - 2026-04-07

### Changed

- CloudTrail events list view redesigned (v2): one row one color, severity-based tinting (`ct-info` dim / `ct-attention` yellow / `ct-danger` red) replacing the per-cell ANSI composition pipeline.
- Verb classification table updated: `BatchGet*`, `Decrypt`, `Encrypt`, `Sign`, `GenerateDataKey*` now correctly classified as read (`R`).
- `AssumeRole` and `AssumeRoleWithSAML` reclassified from write (`W`) to read (`R`) — STS session vending is identity exchange, not a state mutation. Ordinary AssumeRole events now render as `ct-info` instead of `ct-attention`; cross-account, Root, and error paths still escalate.
- Cross-account events in ct-events now show the counterparty account ID inline in the ACTOR column (`999988887777/alice`) and TARGET column (for ARNs from other accounts).
- TIME column now renders as `Apr 07 17:00:59` (15 chars) instead of ISO timestamp.
- TARGET column strips ARN prefix to show just the resource portion.
- Sort indicator glyph now bound to exactly one column per sort mode (fixes double-glyph on ct-events TIME/EVENT columns).
- IAM `ListEntitiesForPolicy` consolidated from three filtered calls per policy to one unfiltered call partitioned client-side, with a 5-second per-policy cache. Reduces API quota use on policy detail views by 3×.
- Help screen ct-events legend now shows the three real row-tint colors (dim/yellow/red) instead of seven decorative palette colors that contradicted the design spec.
- `cache.Load` now validates resource keys against the registry and skips unknown keys instead of silently retaining them.

### Added

- `ctrl+z` — global "show only attention-worthy rows" filter on every resource list view. Hides dim/neutral rows (e.g. ct-info events, terminated EC2 instances).
- CloudTrail target fallback table for management events with empty `resources[]` (`DescribeInstances`, `GetParameter`, `GetSecretValue`, `AssumeRole`, etc.).
- Sensitive-reads allowlist: events reading secret material (Secrets Manager, SSM Parameters, STS AssumeRole, IAM credential reports, ACM exports) escalate to `ct-attention` severity.
- App-wide cancellation context. `tui.Model` now owns an `appCtx` created in `New()` and cancelled on quit; fetchers and IAM related-checkers use it instead of `context.Background()`, so navigating away or quitting actually cancels in-flight AWS calls.

### Fixed

- Demo-mode related counters now match the navigation target lists. Previously a `policy` detail could show "5 Roles" but pressing Enter opened an empty list; affected `policy → role/iam-user/iam-group` and `role → lambda/glue` fixtures.
- Demo handlers no longer swallow `Sscanf`/`Unmarshal` errors — malformed pagination tokens or request bodies now produce HTTP 400 instead of zero-offset silent success.
- Demo fixture `mustParseTime` panic on bad RFC3339 literal replaced with `ParseTime` returning an error.
- Removed deprecated `SecretName` field from `ValueRevealedMsg`; all callers use `ResourceID`.

### Removed

- `ListColumn.Color` field — per-cell color classifiers are no longer supported.
- Per-cell ANSI composition helpers (`ApplyCellColor`, `applyVerbColor`, `applyActorColor`, `applyOutcomeColor`, `applyOriginColor`, etc.) and their tests.
- Legacy `ct-write`/`ct-read` status values and `[cross]` actor prefix.

## [3.32.3] - 2026-04-07

### Fixed

- CloudTrail Events `User` column now shows `invokedBy` (e.g., `ec2.amazonaws.com`) for `AWSService` identity events (EC2 instance profile credential refresh via STS)

## [3.32.2] - 2026-04-07

### Fixed

- Cmd+V and ctrl+V paste now work in filter mode (`/`) — pasted text is applied as a filter immediately
- Cmd+V and ctrl+V paste now work in search mode (`/` in detail/YAML views) — pasted text is appended to the search query
- CloudTrail Events `User` column is no longer empty for `AssumeRole` events — the role name from `userIdentity.sessionContext.sessionIssuer.userName` is shown as a fallback when `Username` is nil

## [3.32.1] - 2026-04-07

### Fixed

- IAM User → CloudTrail Events counter in the related panel no longer shows a misleading partial count (e.g., `(4)`) when the event cache is truncated — the panel now shows the entry without a count and navigates to the full server-side filtered result
- EC2 → CloudTrail Events has the same fix — truncated cache no longer produces a wrong counter
- CloudTrail AssumedRole events now correctly identify the associated IAM Role via `userIdentity.sessionContext.sessionIssuer.userName` in the raw event JSON; the role name is stored as `Fields["role_name"]` and the `IAM Roles` related panel now shows a non-zero count
- `ct-events` detail view now registers `user` → `iam-user` and `role_name` → `role` as navigable fields, making usernames and role names clickable
- IAM User → IAM Policies, IAM Role → Policies, IAM Group → Policies were navigating to empty lists because the policy ID comparison was using `PolicyArn` while the policy fetcher stores `PolicyName` as the resource ID

## [3.32.0] - 2026-04-06

### Added

- EC2 status check indicators in list view — `! running` for impaired, `~ running` for initializing (#188)
- EC2 Status Checks section in detail view with color-coded System/Instance values (#188)
- `DescribeInstanceStatus` API call integrated into EC2 fetcher with graceful degradation
- Demo mode fixtures with impaired and initializing EC2 instances

### Fixed

- Pre-existing lint warning in `cmd/preview/main.go` (if-else chain → switch)

[3.46.0]: https://github.com/k2m30/a9s/compare/v3.45.0...v3.46.0
[3.45.0]: https://github.com/k2m30/a9s/compare/v3.44.0...v3.45.0
[3.44.0]: https://github.com/k2m30/a9s/compare/v3.43.0...v3.44.0
[3.43.0]: https://github.com/k2m30/a9s/compare/v3.42.0...v3.43.0
[3.42.0]: https://github.com/k2m30/a9s/compare/v3.41.0...v3.42.0
[3.41.0]: https://github.com/k2m30/a9s/compare/v3.40.0...v3.41.0
[3.40.0]: https://github.com/k2m30/a9s/compare/v3.39.0...v3.40.0
[3.39.0]: https://github.com/k2m30/a9s/compare/v3.38.0...v3.39.0
[3.38.0]: https://github.com/k2m30/a9s/compare/v3.37.0...v3.38.0
[3.37.0]: https://github.com/k2m30/a9s/compare/v3.36.1...v3.37.0
[3.36.1]: https://github.com/k2m30/a9s/compare/v3.36.0...v3.36.1
[3.36.0]: https://github.com/k2m30/a9s/compare/v3.35.0...v3.36.0
[3.35.0]: https://github.com/k2m30/a9s/compare/v3.34.0...v3.35.0
[3.34.0]: https://github.com/k2m30/a9s/compare/v3.33.0...v3.34.0
[3.33.0]: https://github.com/k2m30/a9s/compare/v3.32.3...v3.33.0
[3.32.3]: https://github.com/k2m30/a9s/compare/v3.32.2...v3.32.3
[3.32.2]: https://github.com/k2m30/a9s/compare/v3.32.1...v3.32.2
[3.32.1]: https://github.com/k2m30/a9s/compare/v3.32.0...v3.32.1
[3.32.0]: https://github.com/k2m30/a9s/releases/tag/v3.32.0

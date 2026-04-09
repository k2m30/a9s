| File | Score | Reasoning |
|------|-------|-----------|
| tests/unit/aws_backup_related_test.go | 28 | Only one target type (role), checker always returns 0 (undeterminable); tests are registration guards and a stub-return assertion with low bug-catching value. |
| tests/unit/aws_pipeline_related_test.go | 28 | Two checkers that always return Count=0 (undeterminable); mostly verifies registration and demo checker presence; low bug-catching value. |
| tests/unit/version_test.go | 28 | Three trivial tests: non-empty string in → same string out; empty → non-empty; dev → non-empty. No interesting branching. |
| tests/unit/child_view_messages_test.go | 30 | Field-access tests on plain structs; no behavior tested — pure struct construction guards. |
| tests/unit/aws_iam_groups_related_test.go | 32 | Only nil-client and empty-ID guards; no real IAM group membership logic tested; 3 of 4 tests are trivial -1 guards. |
| tests/integration/clipboard_test.go | 40 | Clipboard write/read smoke test; environment-dependent (skipped headless); low coverage of production logic. |
| tests/unit/child_view_keys_test.go | 40 | Four tests verify "e"/"L"/"r"/"s" match their key bindings — trivial key.Matches checks, no state or logic tested. |
| tests/unit/child_view_registry_test.go | 40 | Tests struct field assignment and registry round-trips for ChildViewDef; mostly exercises Go struct/map mechanics rather than real product logic. |
| tests/unit/qa_yaml_ec2_family_test.go | 42 | Repetitive 3-assertion pattern (fields/title/ANSI) for 15 resource types; real value is field-presence check, but pattern is boilerplate-heavy with low incremental signal per type. |
| tests/integration/aws_test.go | 45 | Integration tests requiring real AWS config; invalid profile no-panic and region-no-service smoke tests; valuable but environment-dependent and skipped in CI without credentials. |
| tests/unit/mocks_services_test.go | 45 | Infrastructure file (mock structs only, no test functions); value is indirect via tests that use these mocks. |
| tests/unit/aws_ebrule_related_test.go | 48 | Registration + demo-checker only; no checker logic test since the single role checker is tested via nil-client path. |
| tests/unit/qa_view_interface_test.go | 48 | Compile-time interface checks + non-empty FrameTitle; value is type-safety enforcement, not logic coverage. |
| tests/unit/app_related_navigate_test.go | 50 | Tests FindResourceType nil guards and filter-text derivation; prerequisite logic only, not handler itself. |
| tests/unit/aws_athena_related_test.go | 52 | Checkers always return Count=0 (undeterminable); mostly verifies registration, minimal logic coverage. |
| tests/unit/aws_codeartifact_related_test.go | 52 | Only one target (cb) registered; checker returns constant 0 so the logic test is trivial; value mainly in registration and demo-checker coverage. |
| tests/unit/aws_iam_user_related_test.go | 52 | API-call checkers (iam-group, policy); only nil-client path testable without live IAM; low logic coverage beyond nil guard. |
| tests/unit/cache_test.go | 52 | Dir/Path construction with env override — simple path logic, moderate value. |
| tests/integration/cli_test.go | 55 | Binary build + CLI flag smoke tests; catches build regressions and flag parsing; useful but broad and environment-coupled. |
| tests/unit/aws_eb_related_test.go | 55 | Verifies registration + demo coverage for EB; low logic depth but catches missing registrations. |
| tests/unit/aws_ec2_related_cold_cache_live_test.go | 55 | Cold-cache live-fetch path for demo clients; catches checker contract bugs but requires demo AWS config. |
| tests/unit/aws_iam_group_related_test.go | 55 | API-call checkers (iam-user, policy) can only test nil-client → -1 path without a live client; limited logic coverage. |
| tests/unit/aws_iam_policies_related_test.go | 55 | Nil-client guard and basic RawStruct-based policy→role lookup; partial view of the file only shows boilerplate; moderate value. |
| tests/unit/aws_iam_users_related_test.go | 55 | Only three tests: nil-clients returns -1, EmptyID nil-clients returns -1 — mostly nil-client guards with little mapping logic tested. |
| tests/unit/aws_igw_test.go | 55 | Field mapping for IGW; standard pattern, catches Name-tag/status mapping. |
| tests/unit/aws_policy_related_test.go | 55 | Nil-client guards for policy related checkers; mostly cache-miss smoke tests. |
| tests/unit/aws_r53_related_test.go | 55 | Mostly zero/undeterminable-count stubs; navigable fields assertion is the only substantive check. |
| tests/unit/aws_ssm_test.go | 55 | Field mapping + error/empty; standard pattern, low novelty beyond SSM-specific field keys. |
| tests/unit/aws_waf_related_test.go | 55 | Nil-client guard for WAF→ELB; limited view; likely has more tests but only nil-client pattern visible. |
| tests/unit/demo_all66_test.go | 55 | Catch-all completeness guard; catches new resource types missing fixtures but no behavioral logic. |
| tests/unit/qa_r53_records_config_test.go | 55 | Config column widths and detail paths for r53_records; hardcoded assertions that break on intentional config changes. |
| tests/unit/s3_perf_test.go | 55 | Single timing guard ensuring no GetBucketLocation call; catches the N+1 perf bug but fragile if machines are slow. |
| tests/unit/tui_content_views_test.go | 55 | View non-empty and contains-known-text assertions across 7 view types; useful completeness checks but mostly surface-level and unlikely to catch deep rendering bugs. |
| tests/unit/filter_bench_test.go | 58 | Benchmark + 200ms performance assertion for filter on 1000 rows; useful regression guard but threshold may be flaky on slow CI. |
| tests/unit/qa_glue_test.go | 58 | Basic field mapping + nil fields + error; worker_type string conversion is the only non-trivial assertion. |
| tests/unit/qa_pipeline_test.go | 58 | CodePipeline field mapping; standard pattern. |
| tests/integration/ec2_nav_chain_spec008_test.go | 60 | Full navigation chain spec under build tag; requires exported FieldCursor getter; catches navigation regressions that unit tests miss, but gated behind spec008 tag limits routine coverage. |
| tests/unit/aws_cloudformation_test.go | 60 | Standard 3-test pattern; field mapping and status string are verified but no edge cases beyond nil/empty. |
| tests/unit/aws_docdb_snap_related_test.go | 60 | Registration presence and display names for docdb-snap related defs; demo checker smoke test. |
| tests/unit/aws_iam_roles_test.go | 60 | IAM role field mapping; standard pattern. |
| tests/unit/aws_profile_test.go | 60 | ListProfiles parsing from testdata config file; simple and low bug risk but exercises real file parsing. |
| tests/unit/aws_sns_test.go | 60 | Tests SNS topic ARN parsing for name extraction and subscription pagination; ARN-segment extraction is a regression risk but tests are fairly standard. |
| tests/unit/aws_targetgroups_test.go | 60 | Target group field mapping; standard pattern. |
| tests/unit/demo_child_fixtures_test.go | 60 | Coverage exercise for all 22 child demo generators; prevents silent registration drift. |
| tests/unit/qa_athena_test.go | 60 | Tests Athena workgroup fetcher with status, engine version, and nil engine version; standard fetcher parsing test. |
| tests/unit/qa_codeartifact_test.go | 60 | Tests CodeArtifact repository fetcher field mapping; exact assertions on domain, ARN, description fields catch mapping bugs. |
| tests/unit/qa_detail_layout_test.go | 60 | Detail key-value colon separator, path rendering for EC2 family types. |
| tests/unit/qa_kinesis_test.go | 60 | Tests Kinesis stream fetcher field mapping and status string; basic parsing test with exact assertions but limited edge case coverage. |
| tests/unit/qa_yaml_child_views_test.go | 60 | YAML field presence and raw content tests for child views; useful completeness checks but low logic density. |
| tests/unit/qa_yaml_services_test.go | 60 | YAML view field rendering across many services; fixture-heavy but catches field-display regressions. |
| tests/unit/scroll_state_test.go | 60 | ScrollState cursor mechanics (up/down, boundaries, SetCursor, Top/Bottom); foundational but low bug risk in practice. |
| tests/unit/aws_eip_test.go | 62 | Field mapping + unassociated EIP edge case; error/empty guards are standard. |
| tests/unit/demo_app_test.go | 62 | Demo Init and EC2 fetch via root model; mostly integration smoke; ClientsReadyMsg wire-up. |
| tests/unit/demo_related_test.go | 62 | EC2 demo checker returns 9 results with correct counts and IDs; hardcoded count catches fixture drift. |
| tests/unit/qa_backup_test.go | 62 | Fetcher field mapping (plan_name, last_execution) with exact datetime formatting assertion. |
| tests/unit/qa_copy_test.go | 62 | Tests clipboard copy in resource list and detail views via FlashMsg; gracefully skips when clipboard unavailable — functional but environment-sensitive. |
| tests/unit/qa_eni_test.go | 62 | Fetcher field-mapping + TypeDef column verification; solid but limited edge-case coverage. |
| tests/unit/qa_iam_policies_test.go | 62 | Field mapping for IAM policies including AttachmentCount; catches ARN/path parsing. |
| tests/unit/qa_redshift_test.go | 62 | Standard field mapping (node_type, endpoint, nil endpoint); no pagination or related-view coverage. |
| tests/unit/qa_s3_object_detail_test.go | 62 | Verifies detail paths exist and don't panic on s3types.Object and CommonPrefix; limited assertions but catches missing config bugs. |
| tests/unit/qa_ses_test.go | 62 | SES field mapping and status coloring (DOMAIN/sending_enabled); straightforward with solid exact-value assertions. |
| tests/unit/qa_sfn_test.go | 62 | SFN fetcher field mapping (arn, type enum, creation_date) with exact values; standard fetcher test. |
| tests/unit/qa_trail_test.go | 62 | Tests CloudTrail trail fetcher field mapping (multi-region, log validation, org trail booleans); boolean field assertions catch inversion bugs. |
| tests/unit/qa_waf_test.go | 62 | Scope-capture test (ScopeRegional) catches a real misconfiguration bug; field mapping is well-asserted. |
| tests/unit/qa_yaml_v220_test.go | 62 | YAML view fixture builders and resource type registration checks for v2.2.0 types — mostly setup shared by other tests, low standalone value. |
| tests/unit/resource_pagination_test.go | 62 | PaginationMeta round-trip through registry + PaginationMsg wiring; catches NextToken field bugs. |
| tests/unit/tui_identity_test.go | 62 | Identity view account/ARN/role rendering; moderate value for session display. |
| tests/unit/aws_ecs_clusters_test.go | 63 | Two-step ListClusters+DescribeClusters parsing, field verification, error propagation; useful but straightforward. |
| tests/unit/aws_elb_test.go | 63 | Multi-field mapping for ALB/NLB with exact scheme/type/state values; catches enum-to-string mapping bugs. |
| tests/unit/aws_natgateways_test.go | 63 | Tests NAT gateway fetcher with tag extraction, multiple addresses, empty address list, and no-tag fallback; exact field assertions catch mapping regressions. |
| tests/unit/aws_s3_test.go | 63 | S3 bucket listing field mapping; datetime format assertion; standard fetcher coverage. |
| tests/unit/config_test.go | 63 | Tests YAML parsing, column ordering, path/width validation, and merge behavior; covers real config loading bugs but is more integration than unit. |
| tests/unit/qa_apigw_test.go | 63 | Field mapping for HTTP/WebSocket protocol types; error and empty cases; no pagination or related coverage. |
| tests/unit/qa_cf_test.go | 63 | CloudFront fetcher: aliases string join, enabled bool, status, domain mapping with exact values. |
| tests/unit/qa_r53_test.go | 63 | Field mapping (zone_id, record_count, private_zone bool) with exact values; RawStruct type assertion. |
| tests/unit/qa_sns_subscriptions_test.go | 63 | Field mapping (topic name from ARN, protocol, endpoint), ID=ARN, empty/error — slightly above routine due to ARN parsing. |
| tests/unit/reveal_registry_test.go | 63 | Registry register/get/unregister/nil contract for reveal fetchers; some tests designed to fail before coder ships the API. |
| tests/unit/aws_lambda_test.go | 64 | Standard fetcher: field extraction, RawStruct, error/empty — runtime/architecture mapping slightly non-trivial but mostly routine. |
| tests/unit/filter_test.go | 64 | FilterResources: match by ID/Name/Status/Fields, case-insensitive, empty filter, no match — solid but simple logic. |
| tests/unit/qa_codebuild_test.go | 64 | Two-step list+batch fetch, field mapping (source_type, description), RawStruct type assertion. |
| tests/unit/aws_autoscaling_test.go | 65 | ASG field mapping, instance count, status, column verification; standard but catches key field bugs. |
| tests/unit/aws_cloudwatch_test.go | 65 | CW Alarm field mapping (threshold, comparison operator, statistic), RawStruct, error/empty cases. |
| tests/unit/aws_cwlogs_test.go | 65 | Standard fetcher: field extraction, RawStruct, empty/error — well-written but no unusual edge cases. |
| tests/unit/aws_dbi_related_test.go | 65 | Registration completeness and per-checker correctness for 6 RDS related types; alarm CW dimension and snapshot matching are non-trivial. |
| tests/unit/aws_docdb_test.go | 65 | DocDB field mapping, member count, status coloring, and TUI rendering; catches field-key mismatches and color regressions. |
| tests/unit/aws_ecr_images_test.go | 65 | Multi-page pagination + scan finding summary mapping for ECR image child view. |
| tests/unit/aws_elb_listener_rules_test.go | 65 | Rule condition summary + action-type mapping for listener rules child view. |
| tests/unit/aws_elb_listeners_test.go | 65 | ELB listener field mapping including certificate ARN; standard child-view pattern. |
| tests/unit/aws_kinesis_related_test.go | 65 | Tests CloudWatch alarm dimension matching via StreamName; cache-miss and two stub-return tests add modest coverage beyond the alarm checker itself. |
| tests/unit/aws_rds_test.go | 65 | RDS field mapping (engine, version, endpoint, multi-az) with exact values; standard fetcher test. |
| tests/unit/aws_secrets_test.go | 65 | SecretsManager fetcher field mapping, rotation boolean, date formatting; straightforward but catches format regressions. |
| tests/unit/aws_ses_related_test.go | 65 | Pattern N (naming convention) R53 domain matching; domain-strip and trailing-dot normalization are easy to get wrong. |
| tests/unit/aws_sns_topic_subscriptions_test.go | 65 | SNS subscriptions with PendingConfirmation edge case + pagination. |
| tests/unit/aws_subnets_test.go | 65 | Subnet fetcher field mapping (AZ, CIDR, available-IPs, public-on-launch bool) with exact values. |
| tests/unit/layout_twocolumn_test.go | 65 | Layout pixel math for two-column frame; catches off-by-one width bugs and empty-right-panel edge case; moderately regression-prone. |
| tests/unit/qa_bottom_hints_test.go | 65 | BottomHints() presence and content for all four view models; catches missing or mislabeled hints. |
| tests/unit/qa_docdb_snapshots_test.go | 65 | Standard fetcher coverage: field mapping, status, error/empty — no unusual edge cases beyond storage type variation. |
| tests/unit/qa_eb_test.go | 65 | EB fetcher uses Health not Status for row color (#61); catches the specific design decision regression. |
| tests/unit/qa_ecr_test.go | 65 | Standard fetcher coverage: field mapping, RawStruct, empty/error — no unusual logic; scan_on_push nil→false default is the one non-trivial case. |
| tests/unit/qa_efs_test.go | 65 | Field mapping for EFS (mount_targets, performance_mode, size) with exact value assertions; RawStruct type check. |
| tests/unit/qa_iam_groups_test.go | 65 | Tests IAM group fetcher with path extraction, ARN mapping, and status; empty group, error propagation, and type-def column checks add breadth. |
| tests/unit/qa_msk_test.go | 65 | Column-def assertions (widths, keys, titles) + field mapping; MSK cluster type enum checked exactly. |
| tests/unit/qa_reveal_test.go | 65 | RevealModel view content, FrameTitle, SetSize, scroll, scroll-up; overlaps significantly with qa_profile_update_test.go RevealModel section. |
| tests/unit/qa_transit_gateways_test.go | 65 | Solid field-mapping and empty/error coverage; catches Name tag extraction and column order bugs. |
| tests/unit/related_registry_test.go | 65 | RegisterRelated replace-existing + navigable field round-trip; prevents silent registry mutation bugs. |
| tests/unit/tui_root_test.go | 65 | Root model View, header, navigation message handling, frame rendering helpers used by all root-model tests. |
| tests/unit/views_resourcelist_ctrlz_hint_test.go | 65 | Catches forgetting to register ctrl+z in BottomHints() for both ec2 and ct-events. |
| tests/unit/aws_cf_related_test.go | 66 | CF-related registration (5 targets), DisplayName strings, checker nil/non-nil; catches registration/naming bugs. |
| tests/unit/qa_view_methods_test.go | 66 | CopyContent contract on all view types (main menu, resource list, detail, yaml) — catches interface regressions on each view. |
| tests/unit/aws_ecs_svc_tasks_test.go | 67 | ECS service tasks child fetcher: task_id_short extraction, health status, task_def_short — catches truncation logic. |
| tests/unit/qa_ecs_tasks_test.go | 67 | ECS task fetch with multi-cluster pagination and field verification; covers two-step API pattern. |
| tests/unit/app_related_token_test.go | 68 | Verifies truncated-cache related navigate initiates a fetch cmd; Append assertion limited by nil-client constraint but cmd non-nil is meaningful. |
| tests/unit/aws_cb_builds_test.go | 68 | Duration formatting, status mapping, pagination for CodeBuild builds child view. |
| tests/unit/aws_ecs_services_test.go | 68 | Three-step fetch (List clusters→List services→Describe), cluster extraction from ARN, error propagation — above-average due to multi-step flow. |
| tests/unit/aws_ecs_svc_events_test.go | 68 | Tests ECS service events child fetcher with timestamp formatting, status classification, nil fields, and pagination; status classification catches regex/logic bugs. |
| tests/unit/aws_efs_related_test.go | 68 | KMS (Pattern F), CFN tag lookup (Pattern C), lambda stub, demo registration; good RawStruct-based coverage; cache-miss -1 guard correct. |
| tests/unit/aws_iam_group_members_test.go | 68 | Tests IAM group member fetcher with pagination, field mapping, nil fields, and error propagation; exact field assertions catch mapping regressions. |
| tests/unit/aws_iam_roles_related_test.go | 68 | Role→Lambda/Glue/EKS/CodeBuild reverse-lookup checkers with RawStruct matching; ARN last-segment extraction is subtle. |
| tests/unit/aws_redis_related_test.go | 68 | Tests Redis→alarm CacheClusterId dimension matching and Redis→VPC cache lookup; cache-miss and no-match edge cases catch dimension-name typos. |
| tests/unit/aws_redshift_related_test.go | 68 | Alarm dimension checker with found/not-found/cache-miss/empty-ID and demo registration; standard pattern, solid coverage. |
| tests/unit/aws_s3_related_test.go | 68 | S3→trail (S3BucketName), CF, lambda/sqs notification checkers with cache lookups; realistic ARN matching. |
| tests/unit/aws_sfn_executions_test.go | 68 | Tests SFN execution child fetcher with all optional fields populated, nil fields, pagination, and status mapping; exact assertions on 10+ fields catch mapping bugs. |
| tests/unit/aws_sqs_related_test.go | 68 | SNS subscription, alarm, and Lambda reverse-lookup checkers for SQS; ARN-based matching with QueueArn from RawStruct; edge cases (empty ARN, no match) present. |
| tests/unit/aws_sqs_test.go | 68 | Fetcher field-mapping tests with mock clients; RawStruct type assertion catches type-mismatch bugs; queue-name extraction from URL is easy to break. |
| tests/unit/aws_ssm_reveal_test.go | 68 | RevealSSMParameter: value extraction, WithDecryption flag, nil parameter guard, API error — designed to fail before coder ships. |
| tests/unit/demo_rawstruct_test.go | 68 | RawStruct type assertion + fieldpath extraction + yaml round-trip for S3/Lambda/RDS fixtures; catches type-mismatch bugs. |
| tests/unit/demo_test.go | 68 | RawStruct population + field completeness for EC2; gateway test for demo fixture quality. |
| tests/unit/fieldpath_format_test.go | 68 | FormatValue covers nil pointer, bool, time, int, named-type — catches formatting regressions across all detail views. |
| tests/unit/qa_availability_test.go | 68 | Main menu availability rendering + cursor navigation across all resource types with dim states and section headers. |
| tests/unit/qa_coverage_gaps_test.go | 68 | PropagateSize + colorizeValue branches + SelectedItem — covers previously untested app.go delegation paths. |
| tests/unit/qa_ebrule_test.go | 68 | Field mapping (schedule/event_bus, ENABLED/DISABLED status), RawStruct type assertion, error/empty cases — catches mapping bugs. |
| tests/unit/qa_field_alias_test.go | 68 | Alias registration, no-overwrite, original-key-preserved — catches aliasing logic bugs. |
| tests/unit/qa_mainmenu_nav_test.go | 68 | Filter mode, Esc, Enter navigation on main menu; catches real TUI state-machine bugs. |
| tests/unit/qa_opensearch_test.go | 68 | Two-step fetch (List→Describe), field mapping, error paths, column def, alias lookup — mostly standard, alias test adds value. |
| tests/unit/qa_profile_switch_test.go | 68 | Profile switch triggers reconnect and loading state; moderate coverage of ClientsReadyMsg handling. |
| tests/unit/qa_rds_test.go | 68 | RDS list rendering with RawStruct, status color, field alignment, child-view trigger key via root model. |
| tests/unit/qa_registry_test.go | 68 | GetPaginatedFetcher nil-guard + 10 spot-checks + mock registration round-trip; prevents silent fetcher gaps. |
| tests/unit/qa_vpc_endpoints_test.go | 68 | Tests field mapping for VPC endpoints (vpce_id, type enum string, vpc_id) with exact value assertions; column-definition contract test catches future column reordering. |
| tests/unit/qa_yaml_test.go | 68 | YAML view rendering across multiple resource types: field presence, RawContent uncolored, FrameTitle; multi-type coverage is good. |
| tests/unit/aws_toplevel_pagination_batch5_test.go | 69 | ECS cluster and Kinesis stream pagination (NextToken follow-through); catches single-page-only fetcher bugs. |
| tests/unit/aws_acm_related_test.go | 70 | 4 ACM related checkers (ELB/CF/APIGW/R53) registration + ARN-based cache matching. |
| tests/unit/aws_alarm_related_test.go | 70 | Tests alarm→SNS action extraction across AlarmActions, OKActions, InsufficientDataActions; dedup test catches double-counting bugs. |
| tests/unit/aws_apigw_related_test.go | 70 | APIGW related: lambda/logs/waf checkers with regex naming, dimension, tag patterns and demo completeness. |
| tests/unit/aws_cfn_resources_test.go | 70 | Tests CFN stack resource field mapping including drift status, timestamps, and status reason; exact assertions catch field-path mismatches. |
| tests/unit/aws_ct_events_test.go | 70 | CloudTrail event field mapping, status classification, and pagination; catches verb→status mapping regressions. |
| tests/unit/aws_dbc_related_test.go | 70 | DBC related: registration check, sg/alarm/secrets/logs checkers with found/not-found/cache-miss; standard but complete. |
| tests/unit/aws_ddb_related_test.go | 70 | Tests DynamoDB→KMS ARN extraction, lambda dimension matching, and alarm dimension matching; three distinct checker patterns with cache-miss and nil-key edge cases. |
| tests/unit/aws_dynamodb_test.go | 70 | Two-step list+describe pattern; verifies item_count, size_bytes (MB formatting), billing_mode mapping — real format bugs. |
| tests/unit/aws_nodegroups_test.go | 70 | Three-step fetch (ListClusters→ListNodegroups→Describe) with multi-cluster multi-group scenarios. |
| tests/unit/aws_rds_snap_related_test.go | 70 | DBI and KMS cache-based checkers for RDS snapshots; ARN suffix extraction, not-found, and cache-miss paths. |
| tests/unit/aws_rtb_related_test.go | 70 | Navigable field registration + fixture RawStruct resolution for route tables. |
| tests/unit/aws_sfn_execution_history_test.go | 70 | Multi-event parsing, timestamp formatting, RawStruct preservation for SFN child view; catches history event mapping bugs. |
| tests/unit/aws_sns_related_test.go | 70 | Tests reverse-lookup alarm checker across AlarmActions/OKActions/InsufficientDataActions; multi-alarm and empty-ARN edge cases cover realistic bugs. |
| tests/unit/aws_vpc_related_test.go | 70 | VPC→subnet/sg/ec2/nat reverse cache checkers; vpc_id field matching, found/not-found/cache-miss. |
| tests/unit/demo_render_test.go | 70 | End-to-end list rendering with production views.yaml config for ec2/s3/lambda/rds; catches column config mismatches. |
| tests/unit/ec2_detail_design_contract_test.go | 70 | Design contract test: frame title context, first selected row is InstanceId — catches column ordering regressions in the detail view. |
| tests/unit/qa_configurable_views_test.go | 70 | Tests realistic SDK struct→field extraction pipeline for 7+ resource types via configurable views; realistic struct builders exercise the full fieldpath chain. |
| tests/unit/qa_detail_v220_test.go | 70 | Realistic SDK struct builders for 20+ resource types used in detail rendering tests; catches fieldpath extraction regressions at the struct level. |
| tests/unit/qa_ec2_test.go | 70 | Column headers, state colors, filter, sort, cursor nav for EC2 list; real list-view behavior coverage. |
| tests/unit/views_help_resource_wiring_test.go | 70 | Documents bug where NewHelp call site loses ct-events short name; regression guard for NewHelpWithResource path. |
| tests/unit/bug_cfn_key_collision_test.go | 71 | Single-assertion test for CFN key collision ("r" vs "R") — low complexity but directly catches a real silent-shadowing bug. |
| tests/unit/golden_research_related_test.go | 71 | Parses P0 relationships from design docs and cross-checks registry; catches docs/code drift for mandatory relationships. |
| tests/unit/qa_docdb_test.go | 71 | DocDB list rendering, status color, full app navigation to detail + yaml + back; end-to-end view stack coverage. |
| tests/unit/tui_styles_test.go | 71 | Status→color mapping exhaustive test: all green/red/yellow/neutral statuses verified against exact ColRunning/ColStopped palette values. |
| tests/unit/tui_wiring_test.go | 71 | Copy (c key) in list/detail returns FlashMsg or CopiedMsg; checks message type and content, exercises clipboard wiring end-to-end. |
| tests/unit/aws_ami_related_test.go | 72 | Tests Pattern C cache lookup and Pattern F RawStruct extraction for AMI→EC2 and AMI→EBS-snap checkers; invalid RawStruct edge case catches real bugs. |
| tests/unit/aws_asg_activities_test.go | 72 | Child-view fetcher: timestamp formatting, status codes, nil fields, all activity states; catches field-mapping regressions in a time-sensitive fetcher. |
| tests/unit/aws_ct_events_format_test.go | 72 | TDD tests for new FormatCTTimestamp/FormatCTTarget exports; §3 length invariant and §5 cross-account logic are non-trivial and easy to get wrong. |
| tests/unit/aws_ebs_snap_test.go | 72 | Field extraction, status parsing, RawStruct type, no-Name-tag edge case — standard fetcher coverage with exact value assertions. |
| tests/unit/aws_ebs_test.go | 72 | Solid field-mapping coverage (attached_to, created, encrypted) with exact-value assertions; catches real fetcher bugs. |
| tests/unit/aws_ecs_tasks_related_test.go | 72 | Group-field parsing (service: prefix extraction) and ARN cluster-name extraction; precise boundary conditions. |
| tests/unit/aws_elb_related_test.go | 72 | Navigable field registration + CloudWatch alarm cache checker for ELB. |
| tests/unit/aws_igw_related_test.go | 72 | Tests VPC attachment extraction from RawStruct and route-table GatewayId scan; detached IGW and mis-matched gateway ID edge cases catch realistic mapping bugs. |
| tests/unit/aws_kms_related_test.go | 72 | KMS→EBS/RDS/Secrets cache checkers with ARN suffix extraction; found/not-found/cache-miss for all three targets. |
| tests/unit/aws_lambda_code_test.go | 72 | Container image rejection, too-large guard, nil configuration safety — non-trivial branching in code fetcher. |
| tests/unit/aws_lambda_invocation_logs_test.go | 72 | Level-2 child fetcher with RequestId filtering, status classification from log prefixes, and error propagation. |
| tests/unit/aws_msk_related_test.go | 72 | "Cluster Name" dimension matching for CloudWatch, demo checker; solid Pattern-C test but single target. |
| tests/unit/aws_routetables_test.go | 72 | Verifies main-RTB flag, route/association counts, no-name fallback; exact field assertions catch mapping regressions. |
| tests/unit/aws_sns_sub_related_test.go | 72 | Protocol-gated checkers (sns/lambda/sqs) with match/no-match/wrong-protocol/cache-miss; catches protocol guard bugs. |
| tests/unit/aws_tg_related_test.go | 72 | Pattern F + Pattern C checkers for TG→ELB/ECS/ASG/Alarm; dimension suffix matching for alarm is subtle and regression-prone. |
| tests/unit/aws_tgw_related_test.go | 72 | RTB route reverse-lookup, VPC attachment match, VPN attachment match, cache-miss handling — nested RawStruct field traversal is non-trivial. |
| tests/unit/aws_toplevel_pagination_batch4_test.go | 72 | Multi-service pagination (CFN, ACM, ASG, CW, ECR, EB, R53, SM, SFN, SSM) — catches off-by-one token handling. |
| tests/unit/aws_vpc_test.go | 72 | Field mapping, RawStruct type assertion, empty/error response, and real sanitized fixture data with CidrBlockAssociationSet; catches tag-name extraction bugs. |
| tests/unit/cache_registry_validation_test.go | 72 | Documents known gap (unknown key silently accepted) + round-trip for all registered types; catches yaml tag omissions on new types. |
| tests/unit/demo_ec2_fixtures_spec008_test.go | 72 | Documents 5 known missing RawStruct fields with intentional-fail tests; regression guards on VpcId/SubnetId cross-refs are solid. |
| tests/unit/detail_boundary_spec008_test.go | 72 | j/k boundary clamping in DetailModel fieldCursor — requires build tag until getter is added; catches off-by-one at list ends. |
| tests/unit/detail_related_test.go | 72 | Toggle state machine cmd dispatch; catches RelatedCheckStartedMsg not being emitted on second show. |
| tests/unit/ec2_related_view_golden_test.go | 72 | Golden file snapshot (text + ANSI) for EC2 related view; prevents visual regressions but brittle to cosmetic changes. |
| tests/unit/fieldpath_enumerate_test.go | 72 | Covers pointer-dereference, time.Time leaf treatment, named-string-type leaf, and slice notation; fieldpath bugs silently produce empty detail rows. |
| tests/unit/issue234_fetcher_pagination_honesty_test.go | 72 | Verifies eks/kms/ng are registered as paginated fetchers (prevents regression to unbounded internal loop). |
| tests/unit/qa_26_search_core_test.go | 72 | Search activation, typing, confirm/cancel, match counter, empty/no-match states via root model; real key routing. |
| tests/unit/qa_detail_ec2_family_test.go | 72 | Detail view rendering for EC2 family + RawStruct path extraction across multiple types. |
| tests/unit/qa_iam_users_test.go | 72 | IAM user field mapping (path, PasswordLastUsed nil handling), error/empty paths; exact field assertions. |
| tests/unit/qa_mainmenu_test.go | 72 | Verifies all resource type names and aliases are visible in main menu; catches registry omissions. |
| tests/unit/qa_pagination_monitoring_test.go | 72 | Covers alarm, log groups, DynamoDB, SQS, SNS pagination; first/last/empty/error for each is repetitive but catches IsTruncated and NextToken wiring bugs per fetcher. |
| tests/unit/qa_profile_update_test.go | 72 | Full cursor navigation + boundary + Enter + marker tests on SelectorModel and RevealModel; solid state-machine coverage. |
| tests/unit/qa_rds_snapshots_test.go | 72 | RDS snapshot field mapping (type, engine, status), error/empty paths; exact field assertions. |
| tests/unit/qa_retry_probe_test.go | 72 | Tests RetryOnThrottle contract for probeResourceAvailability; throttle-retry, non-retryable-fail, config parameters — pins the retry wiring before the coder changes it. |
| tests/unit/s3_object_pagination_test.go | 72 | Single-call pagination contract for S3 objects with continuation-token flow; catches next-token wiring bugs. |
| tests/unit/styles_is_dim_row_color_test.go | 72 | IsDimRowColor truth table for ct-events severity + EC2 lifecycle + CFN suffix patterns. |
| tests/unit/tui_resourcelist_test.go | 72 | ResourceList cursor movement, filter, sort, column rendering across multiple terminal sizes; foundational list view behavior. |
| tests/unit/aws_acm_test.go | 73 | Field mapping (in_use bool, status, type enums), nil optional date; exact value assertions; standard happy/error/empty trio. |
| tests/unit/aws_ssm_related_test.go | 73 | Tests SSM→KMS alias/ARN matching with multiple match strategies; alias vs ARN vs key-ID resolution covers a fragile multi-path lookup that is easy to break. |
| tests/unit/aws_subnet_related_test.go | 73 | EC2, ELB, TG, NAT reverse-cache checkers for subnet; fixture resolution, match/no-match/cache-miss for each. |
| tests/unit/cache_snapshot_test.go | 73 | IsTruncated propagation from cache entry to related checker; verifies the cache snapshot path correctly forwards the flag. |
| tests/unit/left_column_preview_regressions_test.go | 73 | Regression guards: InstanceId first row, no YAML bullets for SecurityGroups sub-fields, IAM ARN on indented sub-field line — prevents rendering regressions. |
| tests/unit/qa67_terminal_size_test.go | 73 | Boundary terminal sizes (60×24, 80×7), resize during detail/YAML/child views, extreme dimensions (300 wide, 200 tall). |
| tests/unit/qa_kms_test.go | 73 | Multi-step (ListKeys→DescribeKey), filters AWS-managed vs customer-managed keys, alias lookup — filtering logic is the key regression risk. |
| tests/unit/qa_name_column_first_test.go | 73 | 14 resource types × config + TypeDef × first/second column; directly enforces issue-23 schema fix. |
| tests/unit/aws_ami_test.go | 74 | AMI field mapping (architecture, state, public flag, owner filter), self-owned filter logic, capturing input assertions. |
| tests/unit/aws_asg_related_test.go | 74 | Tests 5 ASG related checkers (EC2 tag, TG VPC, subnet, alarm, node group); RawStruct extraction and cache-based dimension matching catch mapping regressions. |
| tests/unit/aws_cb_related_test.go | 74 | Tests two distinct checker patterns (ARN segment match for role, explicit+convention match for logs); naming-convention fallback and nil-ServiceRole edge cases catch mapping bugs. |
| tests/unit/aws_eks_related_test.go | 74 | EKS→ng/alarm/cfn checkers; cluster name dimension matching, CFN tag lookup, empty cluster name edge case. |
| tests/unit/aws_nat_related_test.go | 74 | Three navigable fields (VpcId, SubnetId, AllocationId), related checkers with RawStruct inspection — solid but typical pattern. |
| tests/unit/aws_opensearch_related_test.go | 74 | Alarm dimension (DomainName) match, CFN undeterminable, demo checker completeness — similar pattern to other related tests but OpenSearch-specific. |
| tests/unit/aws_secrets_related_test.go | 74 | KMS ARN→UUID extraction, Lambda ARN→name extraction, no-navigable-fields assertion; exact ID checks. |
| tests/unit/config_split_test.go | 74 | YAML config parsing (list cols + detail paths), round-trip, error cases, LoadFromDir — config correctness catches silent misconfiguration. |
| tests/unit/demo_fixture_load_test.go | 74 | Pan-fixture panic guard across all child types and related checkers; ParseTime error-vs-panic contract catches a real API design bug. |
| tests/unit/ec2_fixture_crossref_test.go | 74 | Cross-references EC2 RawStruct fields against demo fixture IDs; catches fixture inconsistencies that break navigable-field tests. |
| tests/unit/layout_frame_test.go | 74 | PadOrTrunc with ANSI/unicode/negative-width edge cases catches real rendering regressions in column layout. |
| tests/unit/qa_architect_bugs_test.go | 74 | S3 folder enter-vs-detail bug, and other architect-identified edge cases; catches data-driven nav bugs. |
| tests/unit/qa_demo_completeness_test.go | 74 | RawStruct field completeness for 50+ types via fieldpath.ExtractSubtree — catches fixtures that only populate Fields map. |
| tests/unit/qa_eks_secrets_test.go | 74 | Covers EKS list/detail/YAML and Secrets reveal UX across multiple non-secret resource types; x-key guard across 6 types prevents accidental reveal leakage. |
| tests/unit/qa_list_rawstruct_test.go | 74 | All resource types load into ResourceListModel without panic; checks RawStruct round-trip. |
| tests/unit/qa_pagination_compute_test.go | 74 | Paginated fetchers for EC2/Lambda/S3 — catches token propagation and page-boundary bugs. |
| tests/unit/aws_cb_build_logs_test.go | 75 | Status classification table + nil-field safety + RawStruct preservation for cross-service CW Logs child view. |
| tests/unit/aws_ecr_related_test.go | 75 | Cache-based Pattern C checkers for Lambda/CodeBuild/CFN with concrete RawStruct matching; catches URI prefix and tag-lookup bugs; good edge-case coverage (empty URI, no tag). |
| tests/unit/aws_ecs_services_related_test.go | 75 | ECS service related checkers: cluster (Fields-based), alarm (dimension), CFN (tag), logs (naming), demo completeness — good pattern coverage. |
| tests/unit/aws_eks_test.go | 75 | Two-step ListClusters+DescribeCluster fetch, field mapping, error handling; verifies the two-call pattern is correct. |
| tests/unit/aws_errors_test.go | 75 | ClassifyAWSError with smithy APIError codes (expired, throttling, access denied, unknown, plain) — retryable/non-retryable classification logic. |
| tests/unit/aws_identity_test.go | 75 | Tests ARN parsing logic for assumed-role, IAM user, federated-user, no-alias, IAM-error-graceful-degradation, and STS-error — realistic parser edge cases with exact field assertions. |
| tests/unit/aws_logs_related_test.go | 75 | Pattern C cache lookup for lambda/ECS log group name parsing, alarm dimension match, navigable fields — solid related-checker coverage. |
| tests/unit/aws_rds_events_test.go | 75 | Child-view fetcher for RDS events: multi-event, error, empty, all states, nil fields; realistic child-view coverage for a timestamp-heavy fetcher. |
| tests/unit/aws_trail_related_test.go | 75 | ARN parsing for CloudWatch logs group name + SNS/KMS/S3 cache checkers; nil-ARN guards. |
| tests/unit/config_ct_events_layout_test.go | 75 | Locks exact column widths from §8 spec; catches silent width drift in defaults_monitoring.go. |
| tests/unit/detail_render_unit_test.go | 75 | Covers RawYAML, PlainContent, renderFromConfig snake_case fallback, computeKeyWidth — was 0% coverage; real rendering logic. |
| tests/unit/issue119_140_regressions_test.go | 75 | Issue #119 (stacked width r toggle) and #140 (related list title includes source context) — pinned regression tests. |
| tests/unit/qa67_concurrency_test.go | 75 | Deleted-resource, rapid-Esc, late-LoadedMsg, and refresh-state-change scenarios; catches real timing bugs. |
| tests/unit/qa_filtering_test.go | 75 | End-to-end filter journey tests through root model; tests slash activation, text filtering, escape, regex, and multi-type coverage — catches integration-level filter regressions. |
| tests/unit/qa_help_profile_region_test.go | 75 | Full TUI integration: help open/close from all views, region selector flow, flash message lifecycle, resize recovery — real UX regressions caught. |
| tests/unit/qa_list_rawstruct_child_views_test.go | 75 | List rendering with RawStruct for multiple child types (log_streams, log_events, and others); validates column key mapping. |
| tests/unit/qa_pagination_root_test.go | 75 | Root-model pagination wiring (IsTruncated→"50+" title) and ct-events truncation probe; documents real bugs. |
| tests/unit/related_escape_regression_test.go | 75 | Regression: Esc from related-filtered list must return to source detail (not clear filter first); easy to break during navigation stack refactors. |
| tests/unit/aws_cfn_related_test.go | 76 | Role ARN last-segment extraction (Pattern F), nil RoleARN guard, cache-miss path; catches ARN parsing regressions. |
| tests/unit/aws_ebs_related_test.go | 76 | Tests three distinct checker patterns (field lookup, cache scan, KMS ARN extraction); multiple snapshots count, nil KmsKeyId, bad RawStruct — solid edge-case coverage. |
| tests/unit/aws_ecs_svc_logs_test.go | 76 | Cross-service child fetcher (DescribeTaskDefinition→FilterLogEvents), awslogs driver, stream_short extraction, pagination, no-awslogs fallback. |
| tests/unit/aws_sg_test.go | 76 | Real AWS data fixture (21 SGs), VPC distribution counts, IPv6 egress, nil fields, RawStruct type assertion — realistic data coverage. |
| tests/unit/detail_focus_test.go | 76 | Tab/focus toggle, Esc unfocuses without popping view stack, Enter emits RelatedNavigateMsg — covers non-obvious state-machine behavior. |
| tests/unit/ec2_status_checks_test.go | 76 | Status check glyph prefixes in list view + fetcher merge of system/instance status; issue-188 regression guard. |
| tests/unit/fieldpath_fieldlist_test.go | 76 | ExtractFieldList with nil obj, unknown path, fields-map precedence, struct expansion; covers FROZEN package. |
| tests/unit/fieldpath_unexported_test.go | 76 | Panic safety for ExtractSubtree/ExtractScalar across 7 AWS SDK types with unexported fields. |
| tests/unit/qa_fetch_test.go | 76 | Full app-level fetch integration: every AWS service client wired, nil-client guard across all resource types — catches wiring regressions. |
| tests/unit/qa_pagination_hint_test.go | 76 | Pagination hint text variants (standard, filter-aware, loading state); T012 TDD test explicitly expected to fail until feature lands. |
| tests/unit/qa_pagination_stories_test.go | 76 | Large-count pagination (D), Ctrl+R reset (F), nav-across-views (G), demo mode (H), sort-preserve (I). |
| tests/unit/qa_status_colors_test.go | 76 | Status→color mapping for ct-events severity ladder + EC2 lifecycle; catches visual regression bugs. |
| tests/unit/tui_selector_test.go | 76 | Full SelectorModel contract: nav, boundaries, g/G, page up/down, filter, frame title, current marker, empty state, compile-time interface check. |
| tests/unit/aws_alarm_history_test.go | 77 | Child fetcher field mapping (HistoryItemType, timestamp format, JSON HistoryData), pagination, RawStruct, nil-field safety. |
| tests/unit/aws_ec2_related_test.go | 77 | Tests 8+ EC2 related checkers (TG VPC match, EBS attachment, CFN stack tag, ASG tag, etc.) with RawStruct extraction and navigable field path resolution against demo fixtures. |
| tests/unit/aws_ng_related_test.go | 77 | Three related checkers (EKS cluster, IAM role ARN extraction, ASG name from nested Resources), navigable field resolution from fixtures. |
| tests/unit/aws_redis_test.go | 77 | Client-side engine filtering (redis vs memcached), endpoint nil-safety, status mapping; catches filter regression. |
| tests/unit/bug_reveal_related_column_narrow_width_test.go | 77 | Reproduces specific screenshot bug (RELATED disappears at width=76); pinpoints layout threshold regression. |
| tests/unit/qa67_cross_cutting_test.go | 77 | 13 error-resilience stories: no-panic on arbitrary keys, view stack integrity after error, filter/copy/sort on empty list — good crash regression coverage. |
| tests/unit/qa_redis_test.go | 77 | TUI-level Redis list rendering: status colors, column layout, hscroll — complements the fetcher unit test. |
| tests/unit/aws_eip_related_test.go | 78 | Three checker patterns (F, F, C) with associated/unassociated/cache-miss cases; exact ResourceID assertions. |
| tests/unit/aws_glue_related_test.go | 78 | ARN→name extraction, cache-miss sentinel, alarm dimension match, demo checker completeness — solid related-checker coverage. |
| tests/unit/aws_r53_records_test.go | 78 | Multi-value record field extraction, type→string mapping, RawStruct, pagination, child type registration — standard but covers A/CNAME/MX/Alias. |
| tests/unit/aws_sfn_related_test.go | 78 | Full SFN related coverage: logs (naming convention), alarm (dimension), role/cfn (undeterminable), demo completeness — thorough. |
| tests/unit/aws_tg_health_test.go | 78 | All 6 health states, nil-field safety, IP targets, RawStruct fidelity, and column schema; comprehensive coverage of a child-view fetcher. |
| tests/unit/aws_toplevel_pagination_batch2_test.go | 78 | Second batch: SG, DocDB, ElastiCache, ELB, RDS, Redshift multi-page pagination; broad fetcher coverage. |
| tests/unit/aws_vpce_related_test.go | 78 | Pattern-F struct-based checkers for Interface vs Gateway endpoint types; bad-RawStruct panic guard; demo completeness. |
| tests/unit/bug_reveal_related_column_medium_width_test.go | 78 | Regression test revealing a real UI bug (RELATED panel missing at 95-col width); directly exercises layout math at a specific terminal width that triggered a prod defect. |
| tests/unit/connect_aws_region_test.go | 78 | Pins bug #82 (empty region → Missing Region error); tests GetDefaultRegion fallback and InitConnectMsg/ProfileSelectedMsg/RegionSelectedMsg flows end-to-end. |
| tests/unit/demo_no_real_webhooks_test.go | 78 | Scans all demo/*.go files for real Slack/Discord/PagerDuty webhook URLs; security regression guard with placeholder validation. |
| tests/unit/demo_transport_test.go | 78 | Tests that demo mock transport intercepts all 40+ AWS SDK service calls without real credentials; catches any newly added service missing from the mock transport dispatch. |
| tests/unit/ec2_stories_cursor_enter_test.go | 78 | EC2 detail cursor navigation, navigable-field Enter emit, non-navigable no-op, g/G jump — several tests designed to fail until coder ships FieldCursor. |
| tests/unit/preview_design_regression_test.go | 78 | Full-model integration tests for right-column filter, scroll, focus, help — catches real UI wiring bugs. |
| tests/unit/qa67_view_errors_test.go | 78 | Crash-safety across clipboard, deleted secret, no-version, x-key on non-secret — real regression guard. |
| tests/unit/qa_api_error_test.go | 78 | APIErrorMsg clears loading state, shows flash, multi-type coverage — catches error propagation gaps in root Update handler. |
| tests/unit/qa_cache_invalidation_test.go | 78 | Tests cache-clear on profile/region switch and selective refresh; multi-step state machine tests that catch subtle cache-persistence bugs after navigation events. |
| tests/unit/qa_cache_user_stories_test.go | 78 | Warm re-entry, load-more cache update, ctrl+R invalidation user stories; catches subtle cache consistency bugs that only appear on navigation round-trips. |
| tests/unit/qa_child_pagination_test.go | 78 | Tests single-page contract, IsTruncated, NextToken forwarding, and continuation token passthrough — realistic pagination bugs. |
| tests/unit/qa_help_context_test.go | 78 | Exhaustively verifies context-sensitive help inclusion/exclusion per view type and resource type; catches misrouted bindings and stale labels (sort age vs date). |
| tests/unit/related_fetch_test.go | 78 | Tests cache-hit, cache-miss, truncation, and error-propagation paths of FetchRelatedTarget — real logic branches, not trivial guards. |
| tests/unit/rightcolumn_test.go | 78 | Toggle on/off, result delivery, count display, error state, width thresholds for related column layout. |
| tests/unit/views_help_ct_events_legend_test.go | 78 | TDD tests for CT events legend gating (ct-events only, not other resource types, not main menu); catches incorrect legend display in wrong context. |
| tests/unit/column_key_mismatch_test.go | 79 | Global invariant: every column Key must have a registered field key — catches copy-paste errors between types.go and fetchers across all resource types. |
| tests/unit/detail_resize_test.go | 79 | Narrow→wide resize sets pendingRelatedDispatch, first-paint no double-dispatch — specific contract between DetailModel and app.go. |
| tests/unit/qa_26_search_integration_test.go | 79 | Root-model integration of search: Esc clears highlights, K scroll-to-match, resize, other-key bindings during search — broad TUI integration coverage. |
| tests/unit/qa_pagination_child_audit_test.go | 79 | Two-call Load More flow for 10+ child fetchers with real mock pagination; verifies NextToken threading end-to-end, catching token-loss bugs invisible to single-page tests. |
| tests/unit/qa_pagination_services_test.go | 79 | Pagination contract for 15+ service fetchers: first-page/last-page IsTruncated, NextToken forwarding, Marker-based pagination — cross-service coverage. |
| tests/unit/related_smoketest_table_test.go | 79 | Table-driven S01-S06 across many resource types; consolidates 19 files and catches label/nav regressions. |
| tests/unit/aws_ct_events_redesign_test.go | 80 | Original redesign TDD suite: verb classifier, status mapping, _ct.* field presence, cross-account, is_root, outcome, order; foundational for the entire CT pipeline. |
| tests/unit/aws_ec2_test.go | 80 | Verifies multi-reservation flattening, lifecycle field mapping (spot/on-demand/scheduled), nil public IP — catches real field-mapping regressions. |
| tests/unit/aws_error_wrapping_test.go | 80 | Broad cross-fetcher contract: every fetcher must wrap errors with descriptive context; catches missing `%w` or wrong message strings. |
| tests/unit/aws_lambda_related_test.go | 80 | ARN-to-role-name extraction, CloudWatch alarm dimension matching, cache-miss/not-found paths — real parsing logic under test. |
| tests/unit/aws_log_events_test.go | 80 | Tests status classification heuristics (ERROR/WARN/META), truncation, nil fields no-panic, StartFromHead=false ordering, and RawStruct preservation — all catching realistic bugs. |
| tests/unit/aws_log_streams_test.go | 80 | Pagination contract, IsTruncated, continuation token forwarding, timestamp formatting, nil fields, RawStruct — strong child-view coverage. |
| tests/unit/aws_retry_test.go | 80 | Tests throttle/non-throttle/context-cancel/max-retries paths of the generic retry helper; catches wrong error classification and context leak; no other test covers this. |
| tests/unit/aws_toplevel_pagination_test.go | 80 | Multi-page pagination across EC2, CWLogs, DynamoDB, IAM, KMS, Lambda, RDS, SNS, SQS; catches page-loop bugs. |
| tests/unit/bug_reveal_related_column_journey_test.go | 80 | End-to-end journey test (menu→filter→list→detail) revealing RELATED column must be visible at 170-col terminal; catches view-routing regressions invisible to unit-level tests. |
| tests/unit/bug_reveal_related_column_resize_test.go | 80 | Catches specific resize bug: RELATED column missing after narrow→wide resize; plus explicit-hide persistence across resize. |
| tests/unit/detail_navigable_test.go | 80 | Underline rendering + Enter emits RelatedNavigateMsg for navigable fields; covers a complete UI feature path. |
| tests/unit/detail_selected_row_visibility_test.go | 80 | ANSI escape code assertions for background highlight and underline removal on selected rows; catches subtle rendering bugs that are invisible without escape-code inspection. |
| tests/unit/first_screen_journey_regression_test.go | 80 | Full journey smoke tests: EC2→Enter→detail→RELATED, Enter-on-ImageId→related, missing-ResourceType fallback, external-ImageId no-empty-list; end-to-end regression against real navigation bugs. |
| tests/unit/golden_demo_related_test.go | 80 | Cross-layer consistency: demo checker vs RegisterRelated vs fixture IDs; catches silent demo-mode navigation failures. |
| tests/unit/qa67_demo_test.go | 80 | Demo mode completeness (all types have fixtures), error-state rows exercise coloring, navigation features in demo — catches fixture gaps. |
| tests/unit/qa_horizontal_scroll_test.go | 80 | Targets a specific fitColumns bug (#105) where shrunk columns block scroll; tests scroll-right, round-trip, bounded-stop, and header alignment — all covering real rendering defects. |
| tests/unit/qa_pagination_security_test.go | 80 | Pagination for sg, iam-role, iam-policy, secrets, ssm; NextToken and Marker variants with error propagation — catches missing pagination in security fetchers. |
| tests/unit/qa_s3_test.go | 80 | Full S3 TUI flow including bucket→object drill-down, filter, sort, and child-view navigation with real root model. |
| tests/unit/qa_search_views_test.go | 80 | Root-level n/N routing to Detail/YAML search — catches key-swallow bug that keeps counter stuck at 1/N. |
| tests/unit/views_resourcelist_sort_indicator_test.go | 80 | Catches the exact-one-sort-glyph contract for ct-events (EVENT column also matching "event" keyword); pinpoints a known indicator-duplication bug. |
| tests/unit/bug_report_regressions_test.go | 81 | Right-column slash-filter shows matching/hides non-matching rows, Escape clears filter; all-zero rows block focus — real UX regression guards. |
| tests/unit/detail_source_id_guard_test.go | 81 | Catches the stale async result poisoning bug: wrong-SourceResourceID results must be ignored even when ResourceType matches. |
| tests/unit/qa_pagination_remaining_test.go | 81 | EventBridge, Kinesis, MSK, SFN, SNS-sub, Glue, Athena, Redshift, Backup, SES, WAF pagination — 11 fetchers, catches missing NextToken loops. |
| tests/unit/app_cancellation_test.go | 82 | Catches real architectural bug: context.Background() in fetchers ignoring cancellation; static source-text pin + mock-context threading test reveal latent resource leaks. |
| tests/unit/aws_ct_events_status_test.go | 82 | Severity model regression: every verb→status mapping tested against the §1.1 three-tier model; catches wrong verb→status wiring that would miscolor rows. |
| tests/unit/aws_eni_related_test.go | 82 | Pattern-C cache checkers with nil-attachment/nil-association guards; navigable field fixture resolution. |
| tests/unit/aws_pipeline_stages_test.go | 82 | Stage+action flattening, status mapping, RawStruct preservation, nil optional fields no-panic — complex child-fetcher logic. |
| tests/unit/aws_toplevel_pagination_batch3_test.go | 82 | Covers IAM Marker pagination, MSK/SES/WAF/EKS/CodeBuild/Backup NextToken patterns across 14 fetchers — high breadth, catches missing pagination loops. |
| tests/unit/demo_fixture_integrity_issue189_test.go | 82 | Deep cross-reference integrity across all 66 types — ECS/TG/ELB, R53 alias→CF/ELB, CloudTrail→role, EKS→nodegroup. High regression value. |
| tests/unit/ec2_stories_nav_chains_test.go | 82 | Full navigation chain tests (count=1 right-col, filtered drill-down, Esc chain) covering bugs not exercised by unit tests; explicitly regression-targeted. |
| tests/unit/ec2_stories_rightcol_misc_test.go | 82 | Covers 20+ EC2 detail/right-column stories; many currently-failing tests catch unimplemented features and real behavioral regressions. |
| tests/unit/issue119_scenarios_golden_test.go | 82 | Golden snapshots for issue #119 (stacked layout, search, right-column filter); high regression value. |
| tests/unit/issue140_scenarios_golden_test.go | 82 | Golden snapshot suite across 12 EC2 detail scenarios; catches layout regressions including stacked/side-by-side. |
| tests/unit/qa_detail_test.go | 82 | Multi-resource-type detail view rendering against realistic SDK structs; catches wrong field path and nil-field rendering regressions. |
| tests/unit/qa_pagination_infra_test.go | 82 | ECS two-step list+describe, ASG, EB, VPC, ELB, TG pagination; catches multi-step pagination gaps across 15 infra fetchers. |
| tests/unit/qa_r53_records_test.go | 82 | Multi-value record parsing, pagination contract, RawStruct preservation, fieldpath extraction, and full TUI drill-down flow. |
| tests/unit/related_navigate_partial_cache_test.go | 82 | Targets a specific partial-cache/truncated-pagination bug in handleRelatedNavigate's multi-ID branch; regression test designed to fail before fix. |
| tests/unit/rightcolumn_root_filter_regression_test.go | 82 | Regression test for filter state in the focused related pane; verifies header text, row filtering, and Esc-clears-confirmed-filter; realistic user flow with non-obvious state. |
| tests/unit/aws_glue_runs_test.go | 83 | Glue job run flattening, status mapping, nil fields no-panic, DPU-seconds, error message field, pagination, RawStruct preservation. |
| tests/unit/detail_review_fixes_test.go | 83 | Six review-issue fixes tested: Tab on auto-shown panel, independent field cursor, Enter blocked on unavailable rows, Count=-1 rendering. |
| tests/unit/qa_detail_services_test.go | 83 | Realistic SDK struct builders and detail view field-presence checks for Lambda, IAM, ELB, SNS, SSM and others — cross-type field mapping. |
| tests/unit/qa_search_component_test.go | 83 | Exercises SearchModel state machine: match cycling, ANSI content isolation, zero-match no-ops, case-insensitive matching, activate/deactivate — solid component tests. |
| tests/unit/qa_sort_order_test.go | 83 | End-to-end sort: asc/desc toggle, field switch resets direction, deterministic multi-time-field column selection, ListTitle fallback — catches map-iteration non-determinism bug. |
| tests/unit/aws_ebs_snap_related_test.go | 84 | Four checkers (ami cache, ebs field, ec2 description regex, kms ARN suffix) with found/not-found/edge cases — real regex and struct-traversal logic. |
| tests/unit/aws_role_policies_test.go | 84 | Covers managed/inline ordering, admin highlight, nil fields, pagination IsTruncated, RawStruct type assertion, and column widths — very thorough. |
| tests/unit/fieldpath_extract_test.go | 84 | Tests core reflection engine used by every resource type; nil pointers, missing fields, JSON-in-string detection, YAML subtree, bool/time formatting — high regression risk if broken. |
| tests/unit/registry_init_order_test.go | 84 | Catches ChildViewDef referencing unregistered child type and resource types missing fetchers — real runtime panic prevention. |
| tests/unit/s3_pagination_test.go | 84 | Targeted regression test for a specific known bug: ContinuationToken pagination missing from FetchS3Buckets; verifies 2-page traversal. |
| tests/unit/aws_ct_events_verb_classification_test.go | 85 | Table covers §2.1 exactly including bug-fix cases (BatchGet*, Decrypt, AssumeRole*); directly drives TDD fixes. |
| tests/unit/aws_ecs_clusters_related_test.go | 85 | Three Pattern-C checkers (ecs-svc, alarm, cfn) each with found/not-found/cache-miss/empty-source-ID cases; catches ARN suffix and tag matching bugs. |
| tests/unit/aws_sg_related_test.go | 85 | Six distinct checker patterns (VPC/EC2/ENI/ELB/CFN/SG→SG) with egress, self-skip, empty-ID, and cache-miss branches — high regression value. |
| tests/unit/bug_detail_refresh_resets_rightcol_test.go | 85 | Targeted regression test for a specific documented bug (Ctrl+R stale right-column counts); explicitly designed to fail before fix and pass after. |
| tests/unit/detail_width_after_refresh_test.go | 85 | Pinpoints two precise call sites (detail.go:189, detail.go:639) using wrong constant vs computed width; overflow measurement with lipgloss.Width catches invisible layout overflow. |
| tests/unit/issue140_story_render_contract_test.go | 85 | Story-level render contracts for detail+related: first-row, underline visibility, right-column type set, count rendering, tab focus, dim-row skip, filtered list title; catches subtle rendering bugs. |
| tests/unit/qa_26_search_highlight_test.go | 85 | ANSI-aware search highlighting, match navigation, wrap, boundary, rapid-nav — covers novel search logic thoroughly. |
| tests/unit/qa_detail_child_views_test.go | 85 | Detail view rendering for 20+ resource types (log streams, IAM, ECS, ELB, etc.) with realistic SDK structs — broad regression surface. |
| tests/unit/related_cache_bug_test.go | 85 | Two intentionally failing tests revealing the RelatedCheck re-dispatch bug; guards cache-miss and invalidation-on-profile-switch. |
| tests/unit/resourcelist_ami_stub_test.go | 85 | Tests the StubCreator dispatch path (T016 fails before fix); regression guard that StubCreator=nil suppresses navigation. |
| tests/unit/aws_ct_events_related_test.go | 86 | FetchFilter propagation on truncated cache, AssumedRole JSON extraction, cross-resource (EC2→ct-events) truncation regression. |
| tests/unit/issue237_239_240_241_related_fixes_test.go | 86 | Four regression tests for critical related-view infrastructure bugs (token loss, stale-result discard, unnecessary cold-fetch, concurrency cap); catches subtle async timing bugs. |
| tests/unit/qa67_malformed_data_test.go | 86 | Nine distinct malformed-data scenarios (nil fields, empty ID, unknown enum, malformed ARN, unicode, zero timestamp) across all resource types — high-value crash guards. |
| tests/unit/qa_pagination_view_test.go | 86 | FrameTitle state machine (non-truncated/truncated/loading/filtered), append vs replace, cursor preservation across pages, M-key noop guards. |
| tests/unit/aws_ct_events_invariants_test.go | 87 | Cross-cutting property tests: every sensitive-read allowlist entry must be verb=R, and service matching must be exact (not substring); these catch structural bugs that per-entry tests miss. |
| tests/unit/aws_lambda_invocations_test.go | 87 | REPORT line parsing, cold-start detection, pagination, continuation token, log-group-not-found → empty, nil fields no-panic; high-value fetcher tests. |
| tests/unit/aws_s3_n1_test.go | 87 | Directly catches N+1 API call bug (#220) using a counting HTTP transport; real regression test that would fail silently without this explicit call-count assertion. |
| tests/unit/issue230_ctrl_r_cache_clear_test.go | 87 | Regression guard for dead Ctrl+R handler in detail.go; tests cache clear, right-column loading state, and RelatedCheckStartedMsg dispatch. |
| tests/unit/related_navigate_count_spec008_test.go | 87 | Spec-008 bugs: single-ID must open detail not list, multi-ID must filter exactly, LoadMore must stay constrained. |
| tests/unit/aws_cfn_events_test.go | 88 | Full field-mapping, pagination contract, nil-field panic guard, newline-stripping, and timestamp format — catches real parsing bugs. |
| tests/unit/aws_ct_events_target_fallback_test.go | 88 | Bug-revealing tests (TF1/TF2 expected to fail) plus §4 per-event-name fallback table; directly drives new CT feature implementation. |
| tests/unit/aws_eb_rule_targets_test.go | 88 | ArnToResourceName table-driven (7 ARN patterns), ComputeInputSummary priority order, nil field no-panic, RawStruct preservation — comprehensive helper and fetcher tests. |
| tests/unit/aws_lambda_n1_test.go | 88 | Transport-level assertion that ListEventSourceMappings is never called; directly catches the documented N+1 bug #221 with two test cases. |
| tests/unit/child_view_resourcelist_test.go | 88 | Covers drill condition routing, @parent context resolution, non-enter keys, DrillCondition true/false branching — central regression surface. |
| tests/unit/detail_rendering_spec007_test.go | 88 | Five precisely-specified rendering divergences (column separator, sub-field key styling, cursor highlight, navigable underline, stacked layout) — specification-driven TDD. |
| tests/unit/qa_correctness_bugs_test.go | 88 | Pins 4 distinct real bugs (#191–#194) in config merge, availability probes, profile switch, and region env-var handling; regression-prone paths. |
| tests/unit/related_navigate_cache_detail_init_test.go | 88 | Regression tests documenting a specific bug: cache-hit branch drops RelatedCheckStartedMsg, causing permanent loading state. |
| tests/unit/views_resourcelist_dim_filter_test.go | 88 | Tests new ctrl+z attention filter feature end-to-end including cache round-trip, per-view isolation, status indicator, and cursor reset — all novel TDD coverage. |
| tests/unit/cache_esc_pops_test.go | 89 | Catches the cache-poisoning bug: related-navigation list with EscPops=true overwrites top-level cache; complex scenario that would be hard to find manually. |
| tests/unit/issue235_related_check_race_test.go | 90 | Direct race-condition regression for #235; catches both correctness and data-race under `-race`; no existing test covers this. |
| tests/unit/issue236_truncated_zero_nav_test.go | 90 | Regression guard for a specific bug: truncated-zero row must allow cursor, Enter, and show "(0+)" — all three assertions target regression-prone UI state. |
| tests/unit/qa_detail_paths_test.go | 90 | Loads real views.yaml and verifies every configured field path resolves to non-empty output for every resource type — the highest-value field-mapping regression test. |
| tests/unit/aws_ct_events_cross_account_actor_test.go | 91 | TDD regression tests for §1.4 cross-account actor format change from "[cross] " prefix to "accountID/actor" — will fail until implementation lands. |
| tests/unit/aws_ct_events_review_fixes_test.go | 92 | Five named regression bugs with exact error messages and ASCII-sort vs RFC3339 root cause; catches real classification errors (BatchDeleteAttributes→W vs D) and cross-account ROOT prefix; high signal. |
| tests/unit/aws_ct_events_severity_test.go | 92 | Full §1.2 severity ladder + §1.3 allowlist property test; catches any verb/errorCode/Root/cross-account mapping bug. |
| tests/unit/demo_infrastructure_integrity_test.go | 92 | Five-part fixture contract: non-nil RawStructs, field path resolution, cross-reference ID consistency, related-demo Count>0 — the highest-value fixture integrity test in the suite. |
| tests/unit/issue232_truncation_contract_test.go | 92 | Directly pins the truncation-contract bug (Count=-1 vs 0) across 4 buggy checkers plus 4 regression guards; was written to fail before fix. |
| tests/unit/issue233_empty_truncated_cache_test.go | 92 | Tests three-step cache corruption bug (empty+truncated page drops pagination metadata); FAILS with current code — high regression value. |

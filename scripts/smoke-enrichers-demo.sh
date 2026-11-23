#!/bin/sh
# smoke-enrichers-demo.sh — deterministic TUI smoke over every catalog-
# registered on-demand DETAIL enricher (issue #261 + follow-ups), demo
# fixtures only.
#
# Drives the real binary in a headless tmux session and asserts the FETCHED
# enrichment payload actually renders — not just that the view opens.
# Covers all nine catalog-registered detail enrichers: sfn, cfn, lambda,
# ec2, sns, s3, policy (top-level IAM), and the two child-view enrichers,
# role_policies (IAM Roles' "enter" child) and transfer_agreements
# (Transfer servers' "e" child).
#
# Scope: rendered enrichment payloads in demo mode only. Session-cache HIT
# behavior (sfn/cfn's DetailDocCache) is unit-tested
# (tests/unit/*detail_doc_cache*, *session*) and deliberately NOT asserted
# here — re-opening a detail fast enough to still be within the same
# session's warm cache is timing-flaky under a tmux capture.
#
# Every detail and YAML marker is found with page_until: an enriched field's
# screen position depends on how many rows and findings render above it.
#
# Requires tmux. Fails fast with the offending capture on any miss.
set -u

. "$(dirname "$0")/smoke-lib.sh"

smoke_start smoke-enrichers

back_to_menu() {
	keys Escape
	sleep 1
	keys Escape
	sleep 1
}

open_detail() {
	keys ":$1" Enter
	sleep 2
	select_row "$2"
	keys 'd'
}

open_detail sfn user-onboarding-flow
page_until sfn_detail.txt "ACTIVE" "sfn detail (curated view, not just yaml) shows the enriched Status row"
keys 'y'
page_until sfn_yaml.txt "StartAt" "sfn yaml carries the ASL StartAt key"
page_until sfn_yaml.txt "CreateProfile" "sfn yaml renders the user-onboarding-flow ASL definition"

back_to_menu
open_detail cfn acme-vpc-stack
sleep 2
keys 'y'
page_until cfn_yaml.txt "TemplateBody" "cfn yaml carries the fetched TemplateBody field"
page_until cfn_yaml.txt "Resources" "cfn yaml carries real template content"

back_to_menu
open_detail lambda api-gateway-authorizer
page_until lambda_detail.txt "successful" "lambda detail shows the GetFunction deploy status"
expect_re lambda_detail.txt 'State: +Active' "lambda detail state populated by enrichment"
keys 'y'
page_until lambda_yaml.txt "LastUpdateStatus" "lambda yaml also carries LastUpdateStatus"

# process-orders is the one fixture function with reserved concurrency
# (core/demo/fixtures/lambda.go ReservedConcurrency).
back_to_menu
open_detail lambda process-orders
page_until lambda_concurrency_detail.txt "ReservedConcurrentExecutions" "lambda detail shows the fetched reserved Concurrency"

back_to_menu
open_detail ec2 web-prod-01
page_until ec2_detail.txt '#!/bin/bash' "ec2 detail (curated view, not just yaml) shows the decoded bootstrap script"
keys 'y'
sleep 1
page_until ec2_yaml.txt '#!/bin/bash' "ec2 yaml renders the decoded bootstrap script"
forbid ec2_yaml.txt "H4sIA" "ec2 yaml shows no gzip/base64 mojibake"

back_to_menu
open_detail sns alarm-notifications
page_until sns_detail.txt "DisplayName" "sns detail shows the fetched Attributes DisplayName"
page_until sns_detail.txt "SubscriptionsConfirmed" "sns detail shows SubscriptionsConfirmed"
keys 'y'
page_until sns_yaml.txt "EffectiveDeliveryPolicy" "sns yaml carries EffectiveDeliveryPolicy"
page_until sns_yaml.txt "backoffFunction" "sns yaml's EffectiveDeliveryPolicy renders structured content"

back_to_menu
open_detail s3 a9s-demo-healthy
page_until s3_detail.txt "s3:GetObject" "s3 detail (curated view, not just yaml) shows the fetched bucket policy"
keys 'y'
page_until s3_yaml.txt "LifecycleRules" "s3 yaml carries the fetched lifecycle rules"
page_until s3_yaml.txt "Policy" "s3 yaml carries the fetched bucket policy"
page_until s3_yaml.txt "CORSRules" "s3 yaml carries the fetched CORS rules"
page_until s3_yaml.txt "s3:GetObject" "s3 yaml's bucket policy content is real, not opaque"

back_to_menu
open_detail policy acme-s3-read-only
sleep 2
keys 'y'
page_until policy_yaml.txt "Document" "iam policy yaml carries the fetched Document field"
page_until policy_yaml.txt "Statement" "iam policy yaml's Document content is real, not opaque"

back_to_menu
keys ':role' Enter
sleep 2
select_row acme-eks-node-role
keys Enter
sleep 2
select_row trust-policy
keys 'd'
sleep 2
keys 'y'
page_until role_policies_yaml.txt "Document" "role_policies yaml carries the fetched Document field"
page_until role_policies_yaml.txt "Statement" "role_policies yaml's Document content is real, not opaque"

back_to_menu
keys Escape
sleep 1
keys ':transfer' Enter
sleep 2
select_row prod-as2-gateway
keys 'e'
sleep 2
select_row agr-prod-partner
keys 'd'
page_until transfer_agreement_detail.txt "ACME-LOCAL" "transfer agreement detail shows the resolved local profile As2Id"
page_until transfer_agreement_detail.txt "PARTNER-CO" "transfer agreement detail shows the resolved partner profile As2Id"

smoke_finish

#!/bin/sh
# smoke-enrichers-demo.sh — deterministic TUI smoke over every catalog-
# registered on-demand DETAIL enricher (issue #261 + follow-ups), demo
# fixtures only.
#
# Drives the real binary in a headless tmux session and asserts the FETCHED
# enrichment payload actually renders — not just that the view opens.
# Covers all nine catalog-registered detail enrichers: sfn, cfn, lambda,
# ec2, sns, s3, policy (top-level IAM, #87 — never smoke-covered before
# this script), and the two child-view enrichers, role_policies (IAM
# Roles' "enter" child) and transfer_agreements (Transfer servers' "e"
# child).
#
# Scope: rendered enrichment payloads in demo mode only. Session-cache HIT
# behavior (sfn/cfn's DetailDocCache) is unit-tested
# (tests/unit/*detail_doc_cache*, *session*) and deliberately NOT asserted
# here — re-opening a detail fast enough to still be within the same
# session's warm cache is timing-flaky under a tmux capture.
#
# Requires tmux. Fails fast with the offending capture on any miss.
set -u

SESSION="a9s-smoke-enrichers-$$"
# The smoke builds and drives the REPO binary (./a9s), not a temp copy —
# every smoke/gate pass leaves the user's binary fresh by construction.
BIN="./a9s"
CAPDIR="$(mktemp -d "${TMPDIR:-/tmp}/a9s-smoke-enrichers.XXXXXX")"
FAILURES=0

command -v tmux >/dev/null 2>&1 || { echo "smoke-enrichers: tmux is required"; exit 1; }

echo "smoke-enrichers: make build (refreshes ./a9s)"
make build || exit 1

cleanup() {
	tmux kill-session -t "$SESSION" 2>/dev/null
}
trap cleanup EXIT

tmux new-session -d -s "$SESSION" -x 220 -y 50 "$BIN --demo"
sleep 4

wait_for() {
	# $1 = capture file, $2 = marker substring to poll for, $3 = human label.
	# Re-captures the pane up to 10x1s until $2 appears, instead of a fixed
	# sleep racing AWS-call-then-render latency behind an on-demand detail
	# enrichment. Fails loudly (and keeps the last capture) on timeout.
	i=0
	while [ "$i" -lt 10 ]; do
		tmux capture-pane -t "$SESSION" -p > "$CAPDIR/$1"
		if grep -qF -- "$2" "$CAPDIR/$1"; then
			return 0
		fi
		i=$((i + 1))
		sleep 1
	done
	echo "FAIL  $3 — timed out waiting for \"$2\" in $CAPDIR/$1"
	FAILURES=$((FAILURES + 1))
}

page_until() {
	# $1 = capture file, $2 = marker substring, $3 = human label, $4 = max
	# NPage presses (default 12). Some enriched fields render below the
	# fold (ec2's embedded Instance struct is large before UserData) — page
	# down between polls instead of just waiting in place.
	max="${4:-12}"
	i=0
	while [ "$i" -lt "$max" ]; do
		tmux capture-pane -t "$SESSION" -p > "$CAPDIR/$1"
		if grep -qF -- "$2" "$CAPDIR/$1"; then
			return 0
		fi
		tmux send-keys -t "$SESSION" NPage
		sleep 1
		i=$((i + 1))
	done
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/$1"
	if grep -qF -- "$2" "$CAPDIR/$1"; then
		return 0
	fi
	echo "FAIL  $3 — timed out paging (>$max pages) waiting for \"$2\" in $CAPDIR/$1"
	FAILURES=$((FAILURES + 1))
}

expect() {
	# $1 = capture file, $2 = required substring, $3 = human label
	if grep -qF -- "$2" "$CAPDIR/$1"; then
		echo "PASS  $3"
	else
		echo "FAIL  $3 — missing \"$2\" in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	fi
}

expect_re() {
	# $1 = capture file, $2 = required ERE, $3 = human label
	if grep -qE -- "$2" "$CAPDIR/$1"; then
		echo "PASS  $3"
	else
		echo "FAIL  $3 — no match for /$2/ in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	fi
}

forbid() {
	# $1 = capture file, $2 = forbidden substring, $3 = human label
	if grep -qF -- "$2" "$CAPDIR/$1"; then
		echo "FAIL  $3 — forbidden \"$2\" present in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	else
		echo "PASS  $3"
	fi
}

# --- sfn: parsed ASL definition renders (user-onboarding-flow fixture) -----
tmux send-keys -t "$SESSION" ':sfn' Enter
sleep 2
tmux send-keys -t "$SESSION" '/user-onboarding-flow' Enter
sleep 1
tmux send-keys -t "$SESSION" 'd'
wait_for sfn_detail.txt "ACTIVE" "sfn detail (curated view, not just yaml) shows the enriched Status row"
tmux send-keys -t "$SESSION" 'y'
wait_for sfn_yaml.txt "CreateProfile" "sfn yaml renders the user-onboarding-flow ASL definition"
expect sfn_yaml.txt "StartAt" "sfn yaml carries the ASL StartAt key"

# --- cfn: fetched template body renders (acme-vpc-stack, first row) -------
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':cfn' Enter
sleep 2
tmux send-keys -t "$SESSION" 'd'
sleep 2
tmux send-keys -t "$SESSION" 'y'
wait_for cfn_yaml.txt "TemplateBody" "cfn yaml carries the fetched TemplateBody field"
expect cfn_yaml.txt "Resources" "cfn yaml carries real template content"

# --- lambda: GetFunction deploy-state fields render ------------------------
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':lambda' Enter
sleep 2
tmux send-keys -t "$SESSION" 'd'
wait_for lambda_detail.txt "LastUpdateStatus" "lambda detail shows the GetFunction deploy status"
expect_re lambda_detail.txt 'State: +Active' "lambda detail state populated by enrichment"
tmux send-keys -t "$SESSION" 'y'
wait_for lambda_yaml.txt "LastUpdateStatus" "lambda yaml also carries LastUpdateStatus"

# process-orders is the one fixture function with reserved concurrency
# (core/demo/fixtures/lambda.go ReservedConcurrency) — reusing the same
# "/name" filter idiom already used for sfn/s3/role/transfer above, since
# the default (unfiltered) first row never lands on it.
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':lambda' Enter
sleep 2
tmux send-keys -t "$SESSION" '/process-orders' Enter
sleep 1
tmux send-keys -t "$SESSION" 'd'
wait_for lambda_concurrency_detail.txt "ReservedConcurrentExecutions" "lambda detail shows the fetched reserved Concurrency"

# --- ec2: decoded (not base64/gzip) bootstrap script renders ---------------
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':ec2' Enter
sleep 2
tmux send-keys -t "$SESSION" 'd'
wait_for ec2_detail.txt '#!/bin/bash' "ec2 detail (curated view, not just yaml) shows the decoded bootstrap script"
tmux send-keys -t "$SESSION" 'y'
sleep 1
page_until ec2_yaml.txt '#!/bin/bash' "ec2 yaml renders the decoded bootstrap script" 12
forbid ec2_yaml.txt "H4sIA" "ec2 yaml shows no gzip/base64 mojibake"

# --- sns: fetched topic attributes render ----------------------------------
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':sns' Enter
sleep 2
tmux send-keys -t "$SESSION" 'd'
wait_for sns_detail.txt "DisplayName" "sns detail shows the fetched Attributes DisplayName"
expect sns_detail.txt "SubscriptionsConfirmed" "sns detail shows SubscriptionsConfirmed"
tmux send-keys -t "$SESSION" 'y'
wait_for sns_yaml.txt "EffectiveDeliveryPolicy" "sns yaml carries EffectiveDeliveryPolicy"
expect sns_yaml.txt "backoffFunction" "sns yaml's EffectiveDeliveryPolicy renders structured content"

# --- s3: bucket policy/CORS/lifecycle render (reference bucket) ------------
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':s3' Enter
sleep 2
tmux send-keys -t "$SESSION" '/a9s-demo-healthy' Enter
sleep 1
tmux send-keys -t "$SESSION" 'd'
wait_for s3_detail.txt "s3:GetObject" "s3 detail (curated view, not just yaml) shows the fetched bucket policy"
tmux send-keys -t "$SESSION" 'y'
wait_for s3_yaml.txt "LifecycleRules" "s3 yaml carries the fetched lifecycle rules"
expect s3_yaml.txt "Policy" "s3 yaml carries the fetched bucket policy"
expect s3_yaml.txt "CORSRules" "s3 yaml carries the fetched CORS rules"
expect s3_yaml.txt "s3:GetObject" "s3 yaml's bucket policy content is real, not opaque"

# --- policy (IAM, #87 — never smoke-covered before this script) -----------
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':policy' Enter
sleep 2
tmux send-keys -t "$SESSION" 'd'
sleep 2
tmux send-keys -t "$SESSION" 'y'
wait_for policy_yaml.txt "Document" "iam policy yaml carries the fetched Document field"
expect policy_yaml.txt "Statement" "iam policy yaml's Document content is real, not opaque"

# --- role_policies (child view, "enter" off IAM Roles) ---------------------
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':role' Enter
sleep 2
tmux send-keys -t "$SESSION" '/acme-eks-node-role' Enter
sleep 1
tmux send-keys -t "$SESSION" Enter
sleep 2
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/role_policies_list.txt"
tmux send-keys -t "$SESSION" 'd'
sleep 2
tmux send-keys -t "$SESSION" 'y'
wait_for role_policies_yaml.txt "Document" "role_policies yaml carries the fetched Document field"
expect role_policies_yaml.txt "Statement" "role_policies yaml's Document content is real, not opaque"

# --- transfer_agreements (child view, "e" off Transfer servers) ------------
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':transfer' Enter
sleep 2
tmux send-keys -t "$SESSION" '/prod-as2-gateway' Enter
sleep 1
tmux send-keys -t "$SESSION" 'e'
sleep 2
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/transfer_agreements_list.txt"
tmux send-keys -t "$SESSION" 'd'
wait_for transfer_agreement_detail.txt "ACME-LOCAL" "transfer agreement detail shows the resolved local profile As2Id"
expect transfer_agreement_detail.txt "PARTNER-CO" "transfer agreement detail shows the resolved partner profile As2Id"

if [ "$FAILURES" -gt 0 ]; then
	echo "smoke-enrichers: $FAILURES failure(s); captures kept in $CAPDIR"
	exit 1
fi
rm -rf "$CAPDIR"
echo "smoke-enrichers: all checks green"

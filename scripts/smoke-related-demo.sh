#!/bin/sh
# smoke-related-demo.sh — deterministic TUI smoke over the RELATED panel,
# demo fixtures only.
#
# Drives the real binary in a headless tmux session and asserts the
# rendered-surface contracts the unit suites cannot see end to end: exact
# fixture witness badges, the zero-count-row cursor skip, count-1 drills
# landing on the target DETAIL (not a child view), a circular drill re-showing
# cached counts instantly with the depth badge intact, Esc unwinding back to
# the same detail, and the ec2 IAM Role pivot (instance-profile -> role
# resolution). Runs in ~30s; part of `make ready-to-push` (Stage 6), alongside
# `scripts/smoke-demo.sh`.
#
# Requires tmux. Fails fast with the offending capture on any miss.
set -u

SESSION="a9s-smoke-related-$$"
# The smoke builds and drives the REPO binary (./a9s), not a temp copy —
# every smoke/gate pass leaves the user's binary fresh by construction.
BIN="./a9s"
CAPDIR="$(mktemp -d "${TMPDIR:-/tmp}/a9s-smoke-related.XXXXXX")"
FAILURES=0

command -v tmux >/dev/null 2>&1 || { echo "smoke-related: tmux is required"; exit 1; }

echo "smoke-related: make build (refreshes ./a9s)"
make build || exit 1

cleanup() {
	tmux kill-session -t "$SESSION" 2>/dev/null
	[ -n "${A9S_CONFIG_FOLDER:-}" ] && rm -rf "$A9S_CONFIG_FOLDER"
}
trap cleanup EXIT

# A cache root of the run's own: the demo now keeps a cache like any other
# session, and a smoke must neither read the operator's nor leave anything in
# it. Fresh per run, so every run starts cold and asserts the same thing.
A9S_CONFIG_FOLDER="$(mktemp -d "${TMPDIR:-/tmp}/a9s-smoke-home.XXXXXX")"
export A9S_CONFIG_FOLDER

tmux new-session -d -s "$SESSION" -x 220 -y 50 "$BIN --demo"

# A keystroke sent before the menu accepts input is dropped silently. Boot is
# slowest exactly when the gate runs the smokes back to back, so poll for
# probe data rather than assuming a duration.
i=0
ready=0
while [ "$i" -lt 30 ]; do
	if tmux capture-pane -t "$SESSION" -p 2>/dev/null | grep -qE 'issues:[0-9]+'; then
		ready=1
		break
	fi
	sleep 1
	i=$((i + 1))
done
if [ "$ready" -ne 1 ]; then
	echo "smoke-related: app did not reach a populated menu within 30s" >&2
	exit 1
fi

open_and_capture() {
	# $1 = command alias, $2 = capture file, $3 = settle seconds
	tmux send-keys -t "$SESSION" ":$1" Enter
	sleep "$3"
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/$2"
}

# --- Witness 1/2/8: reference bucket detail, exact fixture badges ----------
open_and_capture s3 s3.txt 3
tmux send-keys -t "$SESSION" '/a9s-demo-healthy' Enter
sleep 1
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_detail.txt"

# --- Witness 2: zero-count rows are dimmed and the cursor skips them -------
# a9s-demo-nopab carries several (0) pivots (Lambda/SNS/SQS notifications,
# Athena, Glue, Backup, EventBridge, Route 53, IAM Roles) between two
# actionable rows (CloudTrail Trails, CloudFront) and the bare CloudTrail
# Events pivot — an ideal witness for the cursor-skip walk.
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" '/a9s-demo-nopab' Enter
sleep 1
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_nopab_detail.txt"
tmux send-keys -t "$SESSION" Tab
sleep 1
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_nopab_tab1.txt"
i=0
while [ $i -lt 11 ]; do
	tmux send-keys -t "$SESSION" Down
	sleep 0.3
	i=$((i + 1))
done
sleep 1
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_nopab_walked.txt"

# --- Witness 3: count-1 drill opens the target DETAIL, not a child view ----
# From the top of the panel (CloudTrail Trails (1), a count-1 pivot).
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" '/a9s-demo-healthy' Enter
sleep 1
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux send-keys -t "$SESSION" Tab
sleep 1
tmux send-keys -t "$SESSION" Enter
sleep 2
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_drill_trail.txt"

# --- Witness 4/6: circular drill (bucket -> trail -> bucket) --------------
# Re-entered detail re-shows cached related counts instantly; header keeps
# "a9s v" AND the depth badge; Esc unwinds one level at a time back to the
# same trail detail, then back to the bucket detail.
tmux send-keys -t "$SESSION" Tab
sleep 1
tmux send-keys -t "$SESSION" Enter
sleep 2
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_circular.txt"
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_esc_from_circular.txt"

# --- Witness 5: transient (?) contract (#38) -------------------------------
# ng -> EBS Volumes is the canonical cold-cache pair (ng detail's EBS Volumes
# pivot fans out over ec2 -> ebs, both cold on a fresh session). Empirically,
# the demo's synchronous checker dispatch resolves the sibling caches too
# fast for a headless capture to ever land mid-flight — even a settle of 0.3s
# already shows a resolved "(0)"/"(N)" badge, never "(?)". We therefore do not
# assert the "(?)" glyph itself (it would be flaky-by-construction here);
# instead we assert the actionable-drill path indirectly: a fresh ng detail's
# RELATED panel settles to real badges with no dangling "(?)" and no fetch
# errors, which is the only externally observable half of the #38 contract
# the demo can exercise deterministically.
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
open_and_capture ng ng.txt 2
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/ng_detail_settled.txt"

# --- Witness 7: ec2 detail, IAM Role pivot (instance-profile -> role) ------
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
open_and_capture ec2 ec2.txt 3
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/ec2_detail.txt"

# --- Witness 9: the lower bound on a partial list -------------------------
# The log-group fixtures are the one demo list served in two pages, so the
# related panel's "+" marker has somewhere to come from. A cluster whose
# groups are on the first page reads (2+); one with none there reads (0+).
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
open_and_capture logs logs_list.txt 3
tmux send-keys -t "$SESSION" Escape
sleep 1
open_and_capture docdb docdb.txt 3
tmux send-keys -t "$SESSION" '/acme-docdb-prod' Enter
sleep 1
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/docdb_prod_detail.txt"
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" '/warn-dbc-unenc' Enter
sleep 1
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/docdb_unenc_detail.txt"

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

forbid_re() {
	# $1 = capture file, $2 = forbidden ERE, $3 = human label
	if grep -qE -- "$2" "$CAPDIR/$1"; then
		echo "FAIL  $3 — forbidden /$2/ present in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	else
		echo "PASS  $3"
	fi
}

# 1. Reference bucket detail: exact fixture witnesses, bare pivot.
expect s3_detail.txt "RELATED" "s3 detail renders the related panel"
expect s3_detail.txt "CloudTrail Trails (1)" "s3 detail related shows the trail witness"
expect s3_detail.txt "KMS Key (1)" "s3 detail related shows the kms witness"
expect s3_detail.txt "CloudTrail Events" "s3 detail related shows the CloudTrail Events pivot"
forbid_re s3_detail.txt 'CloudTrail Events[[:space:]]*\([0-9?]' "CloudTrail Events renders bare, no count badge"

# 2. Zero-count rows render dimmed AND the cursor skips them: after Tab in,
# selection starts on the first row (CloudTrail Trails); after 11 Downs
# through every intervening (0) row, the footer never names a (0) pivot as
# selected and lands on the next actionable/bare row instead.
expect s3_nopab_detail.txt "Lambda (notifications) (0)" "nopab bucket carries zero-count pivots to walk through"
expect s3_nopab_tab1.txt "enter CloudTrail Trails" "tab into the panel selects the first actionable row"
expect s3_nopab_walked.txt "enter CloudTrail Events" "cursor walk skips every (0) row and lands on the next actionable pivot"
forbid_re s3_nopab_walked.txt 'enter (Lambda \(notifications\)|SNS \(notifications\)|SQS \(notifications\)|Athena WorkGroups|Glue Jobs|Backup|EventBridge Rules|Route 53|IAM Roles)($| )' "selection marker never lands on a (0) row"

# 3. Count-1 drill opens the target DETAIL, not a child view.
expect s3_drill_trail.txt "detail -- a9s-demo-s3-trail" "count-1 drill opens the target detail, not a list"
forbid s3_drill_trail.txt "resource-types(" "count-1 drill did not land on a list/menu frame"

# 4/6. Circular drill re-shows cached counts instantly; header keeps version
# + depth badge; Esc returns to the same detail.
expect s3_circular.txt "detail -- a9s-demo-healthy" "circular drill lands back on the bucket detail"
expect s3_circular.txt "CloudTrail Trails (1)" "circular re-entry re-shows cached related counts"
expect s3_circular.txt "a9s v" "header keeps the version at depth"
expect_re s3_circular.txt '\[[0-9]+\]' "header shows the depth badge"
expect s3_esc_from_circular.txt "detail -- a9s-demo-s3-trail" "esc from the drilled detail returns to the same trail detail"
expect s3_esc_from_circular.txt "S3 Bucket (1)" "esc-returned trail detail keeps its counts"

# 5. Transient (?) contract (#38): documented limitation above — the demo's
# synchronous dispatch resolves too fast to ever capture a live "(?)" here,
# so we assert the settled half of the contract instead: no dangling
# unresolved badges and no fetch errors on a fresh ng detail.
expect ng_detail_settled.txt "RELATED" "ng detail renders the related panel"
forbid ng_detail_settled.txt "(?)" "ng related panel carries no dangling (?) badge once settled"

# 7. ec2 detail: IAM Role pivot (instance-profile -> role resolution).
expect ec2_detail.txt "RELATED" "ec2 detail renders the related panel"
expect ec2_detail.txt "IAM Role (1)" "ec2 detail shows the IAM Role pivot resolved via the instance profile"

# 8. Forbid on every related capture: no fetch errors, no frozen (?) after
# settling, no raw UPPER_SNAKE tokens.
for cap in s3_detail.txt s3_nopab_detail.txt s3_drill_trail.txt s3_circular.txt s3_esc_from_circular.txt ng_detail_settled.txt ec2_detail.txt; do
	forbid "$cap" "FetchByIDs failed" "no FetchByIDs failure in $cap"
	forbid "$cap" "cannot be found" "no unresolved-reference error in $cap"
	forbid_re "$cap" ' [A-Z][A-Z0-9]*(_[A-Z0-9]+)+ ' "no raw UPPER_SNAKE cell in $cap"
done
forbid s3_circular.txt "(?)" "re-entered warm detail carries no (?) for cache-computable pivots"

# 9. The lower bound renders. Without these the "+" has no witness on the
# bench and a regression to a bare count, or back to Unknown, is invisible.
expect logs_list.txt "logs(41+)" "the paginated log-group list reports its count as a lower bound"
expect docdb_prod_detail.txt "Log Groups (2+)" "a pivot matching inside a partial list reads (N+), not (N)"
expect docdb_unenc_detail.txt "Log Groups (0+)" "a pivot matching nothing in a partial list reads (0+), not (0) or (?)"

if [ "$FAILURES" -gt 0 ]; then
	echo "smoke-related: $FAILURES failure(s); captures kept in $CAPDIR"
	exit 1
fi
rm -rf "$CAPDIR"
echo "smoke-related: all checks green"

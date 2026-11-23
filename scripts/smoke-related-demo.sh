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
# resolution). Runs in ~30s; part of `make ready-to-push` (Stage 6),
# alongside `scripts/smoke-demo.sh`.
#
# The "(1)" / "(0)" / "(2+)" badges asserted below are properties of the
# named witness rows (what each one is related to), not population counts.
#
# Requires tmux. Fails fast with the offending capture on any miss.
set -u

. "$(dirname "$0")/smoke-lib.sh"

smoke_start smoke-related

ZERO_ROW_SELECTED='enter (Lambda \(notifications\)|SNS \(notifications\)|SQS \(notifications\)|Athena WorkGroups|Glue Jobs|Backup|EventBridge Rules|Route 53|IAM Roles)($| )'

# --- Witness 1/2/8: reference bucket detail, exact fixture badges ----------
open_and_capture s3 s3.txt 3
select_row a9s-demo-healthy
keys 'd'
wait_for s3_detail.txt "CloudTrail Trails (1)" "s3 detail related shows the trail witness"

# --- Witness 2: zero-count rows are dimmed and the cursor skips them -------
# a9s-demo-nopab carries several (0) pivots (Lambda/SNS/SQS notifications,
# Athena, Glue, Backup, EventBridge, Route 53, IAM Roles) between two
# actionable rows (CloudTrail Trails, CloudFront) and the bare CloudTrail
# Events pivot — an ideal witness for the cursor-skip walk.
keys Escape
sleep 1
keys Escape
sleep 1
select_row a9s-demo-nopab
keys 'd'
wait_for s3_nopab_detail.txt "Lambda (notifications) (0)" "nopab bucket carries zero-count pivots to walk through"
keys Tab
sleep 1
capture s3_nopab_tab1.txt
touch "$CAPDIR/s3_nopab_zero_stop.txt"
i=0
while [ "$i" -lt 30 ]; do
	keys Down
	sleep 0.3
	capture s3_nopab_walked.txt
	if grep -qE -- "$ZERO_ROW_SELECTED" "$CAPDIR/s3_nopab_walked.txt"; then
		cp "$CAPDIR/s3_nopab_walked.txt" "$CAPDIR/s3_nopab_zero_stop.txt"
	fi
	grep -qF -- "enter CloudTrail Events" "$CAPDIR/s3_nopab_walked.txt" && break
	i=$((i + 1))
done

# --- Witness 3: count-1 drill opens the target DETAIL, not a child view ----
# From the top of the panel (CloudTrail Trails (1), a count-1 pivot).
keys Escape
sleep 1
keys Escape
sleep 1
select_row a9s-demo-healthy
keys 'd'
wait_for s3_healthy_again.txt "CloudTrail Trails (1)" "the reference bucket detail settles its related counts"
keys Tab
sleep 1
keys Enter
sleep 2
capture s3_drill_trail.txt

# --- Witness 4/6: circular drill (bucket -> trail -> bucket) --------------
# Re-entered detail re-shows cached related counts instantly; header keeps
# "a9s v" AND the depth badge; Esc unwinds one level at a time back to the
# same trail detail, then back to the bucket detail.
keys Tab
sleep 1
keys Enter
sleep 2
capture s3_circular.txt
keys Escape
sleep 1
capture s3_esc_from_circular.txt

# --- Witness 5: transient (?) contract -------------------------------------
# ng -> EBS Volumes is the canonical cold-cache pair (ng detail's EBS Volumes
# pivot fans out over ec2 -> ebs, both cold on a fresh session). The demo's
# checker dispatch is synchronous, so a headless capture always sees the
# settled panel: real badges, no dangling "(?)", no fetch errors.
keys Escape
sleep 1
keys Escape
sleep 1
keys Escape
sleep 1
open_and_capture ng ng.txt 2
select_row staging-degraded-net-pool
keys 'd'
sleep 3
capture ng_detail_settled.txt

# --- Witness 7: ec2 detail, IAM Role pivot (instance-profile -> role) ------
keys Escape
sleep 1
keys Escape
sleep 1
open_and_capture ec2 ec2.txt 3
select_row web-prod-01
keys 'd'
wait_for ec2_detail.txt "IAM Role (1)" "ec2 detail shows the IAM Role pivot resolved via the instance profile"

# --- Witness 9: the lower bound on a partial list -------------------------
# The log-group fixtures are the one demo list served in two pages, so the
# related panel's "+" marker has somewhere to come from. A cluster whose
# groups are on the first page reads (2+); one with none there reads (0+).
keys Escape
sleep 1
keys Escape
sleep 1
open_and_capture logs logs_list.txt 3
keys Escape
sleep 1
open_and_capture docdb docdb.txt 3
select_row acme-docdb-prod
keys 'd'
wait_for docdb_prod_detail.txt "Log Groups (2+)" "a pivot matching inside a partial list reads (N+), not (N)"
keys Escape
sleep 1
select_row warn-dbc-unenc
keys 'd'
wait_for docdb_unenc_detail.txt "Log Groups (0+)" "a pivot matching nothing in a partial list reads (0+), not (0) or (?)"

# 1. Reference bucket detail: exact fixture witnesses, bare pivot.
expect s3_detail.txt "RELATED" "s3 detail renders the related panel"
expect s3_detail.txt "KMS Key (1)" "s3 detail related shows the kms witness"
expect s3_detail.txt "CloudTrail Events" "s3 detail related shows the CloudTrail Events pivot"
forbid_re s3_detail.txt 'CloudTrail Events[[:space:]]*\([0-9?]' "CloudTrail Events renders bare, no count badge"

# 2. Zero-count rows render dimmed AND the cursor skips them: after Tab in,
# selection starts on the first row (CloudTrail Trails); walking down, the
# footer never names a (0) pivot as selected and lands on the next
# actionable/bare row instead.
expect s3_nopab_tab1.txt "enter CloudTrail Trails" "tab into the panel selects the first actionable row"
expect s3_nopab_walked.txt "enter CloudTrail Events" "cursor walk skips every (0) row and lands on the next actionable pivot"
forbid_re s3_nopab_zero_stop.txt "$ZERO_ROW_SELECTED" "selection marker never lands on a (0) row"

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

expect ng_detail_settled.txt "RELATED" "ng detail renders the related panel"
forbid ng_detail_settled.txt "(?)" "ng related panel carries no dangling (?) badge once settled"

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
expect logs_list.txt "logs($(demo_count logs))" "the paginated log-group list reports its count as a lower bound"

smoke_finish

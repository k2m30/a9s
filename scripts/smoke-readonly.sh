#!/bin/sh
# smoke-readonly.sh — live TUI smoke against a real read-only AWS profile.
#
#   PROFILE=<readonly-profile> REGION=<region> ./scripts/smoke-readonly.sh
#
# Asserts data-independent invariants the demo cannot exercise: the in-session
# availability sweep reaching "verified" origins, humanized statuses on real
# enum values, issue titles explained per row, related drills landing on
# details without fetch errors, and a full-catalog sweep of every resource
# type the account actually returns rows for. Assertions are pattern-based —
# no fixture counts — so any account works. Read-only by construction (a9s
# never writes).
#
# Not part of the push gate (needs credentials); it is the Stage 6 live
# sub-rule companion for changes touching core/aws/ or rendering.
set -u

: "${PROFILE:?smoke-readonly: set PROFILE=<a *readonly* AWS profile>}"
: "${REGION:?smoke-readonly: set REGION=<an AWS region>}"
case "$PROFILE" in
*readonly*) ;;
*) echo "smoke-readonly: refusing profile \"$PROFILE\" — live smoke runs only on *readonly* profiles"; exit 1 ;;
esac

SESSION="a9s-livesmoke-$$"
# Builds and drives the REPO binary (./a9s) so the user's binary is always
# fresh after any smoke pass.
BIN="./a9s"
CAPDIR="$(mktemp -d "${TMPDIR:-/tmp}/a9s-livesmoke.XXXXXX")"
FAILURES=0

command -v tmux >/dev/null 2>&1 || { echo "smoke-readonly: tmux is required"; exit 1; }

echo "smoke-readonly: make build (refreshes ./a9s)"
make build || exit 1

cleanup() {
	tmux kill-session -t "$SESSION" 2>/dev/null
}
trap cleanup EXIT

# -y 90: tall enough that every registered resource-type row (66 today) is
# captured on a single pane, so the full-catalog sweep below never has to
# scroll the menu to see a type past the fold.
tmux new-session -d -s "$SESSION" -x 220 -y 90 \
	"$BIN --profile $PROFILE --region $REGION"

# The first screen must say what it is doing while it does it: catch the
# title's sweep counter before the sweep settles. Polled fast (0.5s) because
# on a small account the whole sweep can finish inside one 5s step.
SAW_VERIFYING=0
i=0
while [ $i -lt 60 ]; do
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/menu_sweep.txt"
	if grep -qE 'verifying [0-9]+/[0-9]+' "$CAPDIR/menu_sweep.txt"; then
		SAW_VERIFYING=1
		break
	fi
	sleep 0.5
	i=$((i + 1))
done

# The sweep needs real time: poll the menu until most rows leave the dim
# "cache" origin, up to 90s.
i=0
while [ $i -lt 18 ]; do
	sleep 5
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/menu.txt"
	if grep -qE 'issues:[0-9]+' "$CAPDIR/menu.txt"; then
		break
	fi
	i=$((i + 1))
done

open_and_capture() {
	tmux send-keys -t "$SESSION" ":$1" Enter
	sleep "$3"
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/$2"
}

open_and_capture ec2 ec2.txt 8
open_and_capture sg sg.txt 8
open_and_capture lambda lambda.txt 10

# EC2 detail + related: pick the first row, describe, let checks settle,
# then drill the IAM Roles pivot (the instance-profile→role resolution).
tmux send-keys -t "$SESSION" ':ec2' Enter
sleep 5
tmux send-keys -t "$SESSION" 'd'
sleep 8
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/ec2_detail.txt"

expect() {
	if grep -qE -- "$2" "$CAPDIR/$1"; then
		echo "PASS  $3"
	else
		echo "FAIL  $3 — no match for /$2/ in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	fi
}

forbid() {
	if grep -qE -- "$2" "$CAPDIR/$1"; then
		echo "FAIL  $3 — forbidden /$2/ present in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	else
		echo "PASS  $3"
	fi
}

# Menu: sweep produced issue badges in-session (not only cached counts).
# 71 = 70 resource types + the Cost Explorer entry.
expect menu.txt 'resource-types\(71\)' "menu shows the full catalog"
expect menu.txt 'issues:[0-9]+' "sweep produced at least one issue badge"

# First screen: the sweep announces itself while it runs and stops saying so
# when it is done; a healthy read-only role leaves no row marked with a cause
# and no account-wide failure in the title.
if [ "$SAW_VERIFYING" = 1 ]; then
	echo "PASS  menu title carried the sweep counter while the sweep ran"
else
	echo "FAIL  menu title never showed /verifying N\/M/ during the sweep — see $CAPDIR/menu_sweep.txt"
	FAILURES=$((FAILURES + 1))
fi
forbid menu.txt 'verifying [0-9]+/[0-9]+' "sweep counter clears once the sweep is done"
forbid menu.txt '[[:space:]](denied|expired|throttled)[[:space:]]' "no row carries a failure cause on a healthy read-only role"
forbid menu.txt 'sweep: access denied|session expired' "no account-wide probe failure on a healthy read-only role"

# Whole-cell raw enums must not survive rendering anywhere we look.
for cap in ec2.txt sg.txt lambda.txt menu.txt; do
	forbid "$cap" ' [A-Z][A-Z0-9]*(_[A-Z0-9]+)+ ' "no raw UPPER_SNAKE cell in $cap"
done
forbid sg.txt 'WIDE_OPEN|PORTS:' "sg risk column carries no raw tokens"

# Issue-titled lists must carry per-row causes: any list titled !N needs at
# least one non-empty phrase-looking status (lowercase words) in its body.
if grep -qE '\) !\d' "$CAPDIR/lambda.txt" 2>/dev/null || grep -qE '\) ![0-9]' "$CAPDIR/lambda.txt"; then
	expect lambda.txt ' [a-z][a-z-]+( [a-z0-9./-]+)+ ' "lambda issue rows carry a cause phrase"
fi

# EC2 detail renders the related panel with settled checks. An account with
# no instances has no detail to open — skip honestly rather than fail the
# gate on fleet size (the demo smoke pins the detail path deterministically).
if grep -qE 'ec2\(0\)' "$CAPDIR/ec2.txt"; then
	echo "SKIP  ec2 detail checks — live account has no EC2 instances"
else
	expect ec2_detail.txt 'RELATED' "ec2 detail renders the related panel"
	expect ec2_detail.txt '\([0-9]+\)' "ec2 related checks settled to counts"
	forbid ec2_detail.txt 'FetchByIDs failed|cannot be found' "no fetch errors on the detail"
fi

# Full-catalog sweep: every resource type the menu shows with a non-zero
# count gets its own list capture and the same two forbids the targeted
# captures above already carry — a raw UPPER_SNAKE cell, or a vacuous
# whole-cell status word standing in for a real cause. Data-independent: the
# set of types and their counts come from the live menu capture, never a
# hardcoded list.
catalog_line=0
while IFS= read -r line; do
	catalog_line=$((catalog_line + 1))
	count=$(printf '%s' "$line" | grep -oE '\([0-9]+' | head -1 | tr -d '(')
	shortname=$(printf '%s' "$line" | grep -oE ':[a-z0-9-]+ *│' | head -1 | tr -d ': │')
	[ -n "$count" ] || continue
	[ -n "$shortname" ] || continue
	[ "$count" -gt 0 ] || continue

	cap="catalog_${shortname}.txt"
	open_and_capture "$shortname" "$cap" 5

	forbid "$cap" ' [A-Z][A-Z0-9]*(_[A-Z0-9]+)+ ' "no raw UPPER_SNAKE cell in $shortname list"
	forbid "$cap" ' (Attention|Danger|Warning|Issue|Problem) ' "no vacuous whole-word status in $shortname list"
done < "$CAPDIR/menu.txt"
echo "smoke-readonly: full-catalog sweep covered $catalog_line menu line(s)"

# The error log is the sweep's blind spot: a type whose fetch ERRORS renders
# no (N) count in the menu, so the count-driven loop above silently skips it
# — a brand-new broken type would sail through (live regression 2026-07-14:
# mwaa's ListEnvironments ValidationException was invisible to this walk).
# Capture the `!` log, always print it, and gate on it: request-level errors
# (ValidationException, serialization, 4xx/5xx plumbing) FAIL the smoke;
# pure IAM denials are printed as WARN — a partially-denied readonly role is
# a legitimate account state, but it must be seen, never silent.
tmux send-keys -t "$SESSION" "!"
sleep 2
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/errorlog.txt"
tmux send-keys -t "$SESSION" Escape
errlines=$(grep -E 'availability |fetch|operation error|Exception|error:' "$CAPDIR/errorlog.txt" | grep -vE '^\s*$' || true)
if [ -n "$errlines" ]; then
	echo "smoke-readonly: error log after full walk:"
	printf '%s\n' "$errlines" | sed 's/^/    /'
	# StatusCode: 403 counts as a denial: the pane truncates long log lines
	# at the frame border, so the trailing "AccessDeniedException: … not
	# authorized" tail may be cut off while the HTTP status survives.
	nondenial=$(printf '%s\n' "$errlines" | grep -vE 'AccessDenied|not authorized|UnauthorizedOperation|AuthorizationError|StatusCode: 403' || true)
	if [ -n "$nondenial" ]; then
		echo "FAIL  error log carries non-denial errors after the walk"
		FAILURES=$((FAILURES + 1))
	else
		echo "WARN  error log carries IAM denials only (partially-denied readonly role) — review above"
	fi
else
	echo "PASS  error log is empty after the full walk"
fi

if [ "$FAILURES" -gt 0 ]; then
	echo "smoke-readonly: $FAILURES failure(s); captures kept in $CAPDIR"
	exit 1
fi
rm -rf "$CAPDIR"
echo "smoke-readonly: all checks green"

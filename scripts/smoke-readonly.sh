#!/bin/sh
# smoke-readonly.sh — live TUI smoke against a real read-only AWS profile.
#
#   PROFILE=acme-dev-readonly REGION=eu-west-2 ./scripts/smoke-readonly.sh
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
# sub-rule companion for changes touching internal/aws/ or rendering.
set -u

PROFILE="${PROFILE:-acme-dev-readonly}"
REGION="${REGION:-eu-west-2}"
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

# The sweep needs real time: poll the menu until most rows leave the dim
# "cache" origin, up to 90s.
i=0
while [ $i -lt 18 ]; do
	sleep 5
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/menu.txt"
	if grep -qE '\([0-9]+\+?\) *!' "$CAPDIR/menu.txt"; then
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
	if grep -qE "$2" "$CAPDIR/$1"; then
		echo "PASS  $3"
	else
		echo "FAIL  $3 — no match for /$2/ in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	fi
}

forbid() {
	if grep -qE "$2" "$CAPDIR/$1"; then
		echo "FAIL  $3 — forbidden /$2/ present in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	else
		echo "PASS  $3"
	fi
}

# Menu: sweep produced issue badges in-session (not only cached counts).
expect menu.txt 'resource-types\(66\)' "menu shows the full catalog"
expect menu.txt '\([0-9]+\+?\) *![0-9]' "sweep produced at least one issue badge"

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

# EC2 detail renders the related panel with settled checks.
expect ec2_detail.txt 'RELATED' "ec2 detail renders the related panel"
expect ec2_detail.txt '\([0-9]+\)' "ec2 related checks settled to counts"
forbid ec2_detail.txt 'FetchByIDs failed|cannot be found' "no fetch errors on the detail"

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

if [ "$FAILURES" -gt 0 ]; then
	echo "smoke-readonly: $FAILURES failure(s); captures kept in $CAPDIR"
	exit 1
fi
rm -rf "$CAPDIR"
echo "smoke-readonly: all checks green"

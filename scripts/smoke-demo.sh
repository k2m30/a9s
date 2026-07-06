#!/bin/sh
# smoke-demo.sh — deterministic TUI smoke over the demo fixture store.
#
# Drives the real binary in a headless tmux session and asserts the
# user-visible contracts that unit suites cannot see rendered end to end:
# menu availability, humanized status texts (no raw AWS enums), the
# owner-worded security-group risk phrases, per-row issue causes, and the
# related panel of the reference bucket. Runs in ~30s; part of
# `make ready-to-push` (Stage 6).
#
# Requires tmux. Fails fast with the offending capture on any miss.
set -u

SESSION="a9s-smoke-$$"
# The smoke builds and drives the REPO binary (./a9s), not a temp copy —
# every smoke/gate pass leaves the user's binary fresh by construction.
BIN="./a9s"
CAPDIR="$(mktemp -d "${TMPDIR:-/tmp}/a9s-smoke.XXXXXX")"
FAILURES=0

command -v tmux >/dev/null 2>&1 || { echo "smoke: tmux is required"; exit 1; }

echo "smoke: make build (refreshes ./a9s)"
make build || exit 1

cleanup() {
	tmux kill-session -t "$SESSION" 2>/dev/null
}
trap cleanup EXIT

tmux new-session -d -s "$SESSION" -x 220 -y 50 "$BIN --demo"
sleep 4
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/menu.txt"

open_and_capture() {
	# $1 = command alias, $2 = capture file, $3 = settle seconds
	tmux send-keys -t "$SESSION" ":$1" Enter
	sleep "$3"
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/$2"
}

open_and_capture sg sg.txt 3
open_and_capture ng ng.txt 3
open_and_capture lambda lambda.txt 3
open_and_capture s3 s3.txt 3

# Reference-bucket walk: filter narrows the list, describe opens the detail
# with the RELATED panel, YAML view renders, a circular related drill
# (bucket → trail → bucket) re-shows cached badges with the depth-badged
# header, and escape unwinds back to the list.
tmux send-keys -t "$SESSION" '/a9s-demo-healthy' Enter
sleep 1
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_filtered.txt"
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_detail.txt"
tmux send-keys -t "$SESSION" 'y'
sleep 2
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_yaml.txt"
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Tab
sleep 1
tmux send-keys -t "$SESSION" Enter
sleep 3
tmux send-keys -t "$SESSION" Tab
sleep 1
tmux send-keys -t "$SESSION" Enter
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_circular.txt"

# A flagged bucket's detail must open with the Attention block on top.
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" Escape
sleep 1
tmux send-keys -t "$SESSION" ':s3' Enter
sleep 2
tmux send-keys -t "$SESSION" '/a9s-demo-nopab' Enter
sleep 1
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/s3_attention.txt"

expect() {
	# $1 = capture file, $2 = required substring, $3 = human label
	if grep -qF "$2" "$CAPDIR/$1"; then
		echo "PASS  $3"
	else
		echo "FAIL  $3 — missing \"$2\" in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	fi
}

expect_re() {
	# $1 = capture file, $2 = required ERE, $3 = human label
	if grep -qE "$2" "$CAPDIR/$1"; then
		echo "PASS  $3"
	else
		echo "FAIL  $3 — no match for /$2/ in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	fi
}

forbid() {
	# $1 = capture file, $2 = forbidden substring, $3 = human label
	if grep -qF "$2" "$CAPDIR/$1"; then
		echo "FAIL  $3 — forbidden \"$2\" present in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	else
		echo "PASS  $3"
	fi
}

# Menu: full catalog present.
expect menu.txt "resource-types(66)" "menu shows the full catalog"

# Security groups: owner-worded risk phrases, no raw classifier tokens.
expect sg.txt "all ports open" "sg wide-open row uses the owner wording"
expect sg.txt "ports 22 open" "sg dangerous-ports row uses the owner wording"
forbid sg.txt "WIDE_OPEN" "sg shows no raw WIDE_OPEN token"
forbid sg.txt "PORTS:" "sg shows no raw PORTS: token"

# Node groups: humanized states and issue causes.
expect ng.txt "create failed" "ng failed pool status is humanized"
forbid ng.txt "CREATE_FAILED" "ng shows no raw CREATE_FAILED enum"
forbid ng.txt "ACTIVE" "ng healthy status is lowercased"
expect ng.txt "insufficient free addresses" "ng degraded pool names its cause"

# Lambda: the issue-colored rows explain themselves in the State column.
expect lambda.txt "lambda(36) !" "lambda title carries the issue count"
expect lambda.txt "runtime is end-of-life" "lambda deprecated runtime explains itself"

# S3: reference type baseline — count, issues, the PAB cause visible.
expect s3.txt "s3(36)" "s3 list shows the fixture count"
expect s3.txt "public access block incomplete" "s3 flagged bucket names its cause"

# Reference bucket walk.
expect s3_filtered.txt "s3(1/36)" "filter narrows the list to one row"
expect s3_detail.txt "RELATED" "s3 detail renders the related panel"
expect s3_detail.txt "CloudTrail Trails (1)" "s3 detail related shows the trail witness"
expect s3_detail.txt "KMS Key (1)" "s3 detail related shows the kms witness"
expect s3_yaml.txt "BucketArn" "yaml view renders the resource document"

# Circular drill (bucket → trail → bucket): cached badges reappear at once
# and the header keeps the version next to the depth badge.
expect s3_circular.txt "detail -- a9s-demo-healthy" "circular drill lands back on the bucket detail"
expect s3_circular.txt "CloudTrail Trails (1)" "circular re-entry re-shows cached related counts"
expect s3_circular.txt "a9s v" "header keeps the version at depth"
expect_re s3_circular.txt '\[[0-9]+\]' "header shows the depth badge"

# A flagged resource explains itself at the top of its detail.
expect s3_attention.txt "Attention (" "flagged bucket detail opens with the attention block"
expect_re s3_attention.txt '[Pp]ublic access block incomplete' "attention names the cause"

if [ "$FAILURES" -gt 0 ]; then
	echo "smoke: $FAILURES failure(s); captures kept in $CAPDIR"
	exit 1
fi
rm -rf "$CAPDIR"
echo "smoke: all checks green"

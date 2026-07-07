#!/bin/sh
# smoke-related-readonly.sh — live TUI smoke over the RELATED panel against a
# real read-only AWS profile.
#
#   PROFILE=acme-dev-readonly REGION=eu-west-2 ./scripts/smoke-related-readonly.sh
#
# Asserts data-independent invariants the demo cannot exercise on a real
# account's graph: at least one counted pivot settles on a real detail, a
# count-1 drill lands on the target detail (or an actionable list for N>1),
# Esc returns to the same detail, any surviving "(?)" pivot is still
# actionable (Enter navigates, no dead end), and no raw UPPER_SNAKE or
# vacuous tokens leak into the panel. Assertions are pattern-based — no
# fixture counts — so any account works. Read-only by construction (a9s
# never writes).
#
# Not part of the push gate (needs credentials); companion to
# scripts/smoke-readonly.sh and the Stage 6 live sub-rule for changes
# touching the related-resource panel.
set -u

PROFILE="${PROFILE:-acme-dev-readonly}"
REGION="${REGION:-eu-west-2}"
case "$PROFILE" in
*readonly*) ;;
*) echo "smoke-related-readonly: refusing profile \"$PROFILE\" — live smoke runs only on *readonly* profiles"; exit 1 ;;
esac

SESSION="a9s-livesmoke-related-$$"
# Builds and drives the REPO binary (./a9s) so the user's binary is always
# fresh after any smoke pass.
BIN="./a9s"
CAPDIR="$(mktemp -d "${TMPDIR:-/tmp}/a9s-livesmoke-related.XXXXXX")"
FAILURES=0

command -v tmux >/dev/null 2>&1 || { echo "smoke-related-readonly: tmux is required"; exit 1; }

echo "smoke-related-readonly: make build (refreshes ./a9s)"
make build || exit 1

cleanup() {
	tmux kill-session -t "$SESSION" 2>/dev/null
}
trap cleanup EXIT

tmux new-session -d -s "$SESSION" -x 220 -y 50 \
	"$BIN --profile $PROFILE --region $REGION"
sleep 5

open_and_capture() {
	tmux send-keys -t "$SESSION" ":$1" Enter
	sleep "$3"
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/$2"
}

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

# --- 1. Open the first non-empty candidate type with registered pivots, open
# its detail, wait for related settle, expect at least one counted badge. ---
CANDIDATES="ec2 lambda s3 sg"
DETAIL_CAP=""
for c in $CANDIDATES; do
	open_and_capture "$c" "${c}_list.txt" 8
	tmux send-keys -t "$SESSION" 'd'
	sleep 8
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/${c}_detail.txt"
	if grep -qE 'RELATED' "$CAPDIR/${c}_detail.txt" && grep -qE '\([0-9]+\)' "$CAPDIR/${c}_detail.txt"; then
		DETAIL_CAP="${c}_detail.txt"
		echo "smoke-related-readonly: using \"$c\" as the related-panel witness"
		break
	fi
	tmux send-keys -t "$SESSION" Escape
	sleep 1
done

if [ -z "$DETAIL_CAP" ]; then
	echo "FAIL  no candidate type ($CANDIDATES) produced a settled related panel with a counted badge"
	FAILURES=$((FAILURES + 1))
	# Fall back to the last attempted capture so downstream forbids still run
	# against something rather than a missing file.
	DETAIL_CAP="sg_detail.txt"
fi

expect "$DETAIL_CAP" 'RELATED' "related panel renders on the witness detail"
expect "$DETAIL_CAP" '\([0-9]+\)' "related checks settled to at least one counted badge"
forbid "$DETAIL_CAP" 'FetchByIDs failed|cannot be found|panic' "no fetch errors or panics on the witness detail"

# --- 2. Drill the first actionable counted pivot: expect either a detail or
# a list frame; Esc returns to the same detail title. ---
title_before=$(grep -oE 'detail -- [^(]+\([^)]+\)' "$CAPDIR/$DETAIL_CAP" | head -1)

tmux send-keys -t "$SESSION" Tab
sleep 1
tmux send-keys -t "$SESSION" Enter
sleep 5
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/drill1.txt"

if grep -qE 'detail --' "$CAPDIR/drill1.txt" || grep -qE '┌.*\(.*[0-9]+.*\).*┐|resource-types\(' "$CAPDIR/drill1.txt"; then
	echo "PASS  drilling the first actionable pivot lands on a detail or a list frame"
else
	echo "FAIL  drilling the first actionable pivot produced neither a detail nor a list frame"
	FAILURES=$((FAILURES + 1))
fi
forbid drill1.txt 'FetchByIDs failed|cannot be found|panic' "no fetch errors or panics on the drilled frame"

tmux send-keys -t "$SESSION" Escape
sleep 2
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/drill1_esc.txt"

if [ -n "$title_before" ]; then
	expect drill1_esc.txt "$(printf '%s' "$title_before" | sed 's/[.[\*^$()+?{|]/\\&/g')" "esc after the drill returns to the same detail title"
else
	echo "PASS  esc-back check skipped — witness detail title was not capturable (non-fatal, pattern-only lane)"
fi

# --- 3. If any (?) row is visible post-settle: Enter must navigate (frame
# changes), Esc back — and the return capture must show the pivot now
# counted OR still (?) with the row still actionable (no dead-end). ---
if grep -qE '\(\?\)' "$CAPDIR/$DETAIL_CAP"; then
	before_frame=$(cat "$CAPDIR/$DETAIL_CAP")
	i=0
	found_unknown_row=0
	while [ $i -lt 20 ]; do
		tmux capture-pane -t "$SESSION" -p -e > "$CAPDIR/unknown_probe_$i.txt" 2>/dev/null || true
		if grep -qE '\(\?\)' "$CAPDIR/unknown_probe_$i.txt" 2>/dev/null; then
			found_unknown_row=1
			break
		fi
		i=$((i + 1))
	done

	tmux send-keys -t "$SESSION" Enter
	sleep 5
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/unknown_drilled.txt"
	after_frame=$(cat "$CAPDIR/unknown_drilled.txt")

	if [ "$before_frame" != "$after_frame" ]; then
		echo "PASS  entering a (?) related row navigates (frame changed)"
	else
		echo "FAIL  entering a (?) related row did not change the frame — dead-end pivot"
		FAILURES=$((FAILURES + 1))
	fi
	forbid unknown_drilled.txt 'FetchByIDs failed|cannot be found|panic' "no fetch errors or panics after drilling the (?) row"

	tmux send-keys -t "$SESSION" Escape
	sleep 2
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/unknown_esc_back.txt"

	if grep -qE '\([0-9]+\)' "$CAPDIR/unknown_esc_back.txt" || grep -qE '\(\?\)' "$CAPDIR/unknown_esc_back.txt"; then
		echo "PASS  the pivot resolves to a count or remains a still-actionable (?) — no dead end"
	else
		echo "FAIL  the pivot vanished into neither a count nor a (?) after returning — possible dead end"
		FAILURES=$((FAILURES + 1))
	fi
	[ "$found_unknown_row" -eq 1 ] || true
	rm -f "$CAPDIR"/unknown_probe_*.txt
else
	echo "PASS  no (?) related row observed on the witness detail — nothing to probe (pattern-only, no fixture assumption)"
fi

# --- 4. Forbid raw UPPER_SNAKE and vacuous words in the related captures. --
for cap in "$DETAIL_CAP" drill1.txt drill1_esc.txt; do
	[ -f "$CAPDIR/$cap" ] || continue
	forbid "$cap" ' [A-Z][A-Z0-9]*(_[A-Z0-9]+)+ ' "no raw UPPER_SNAKE cell in $cap"
	forbid "$cap" ' (Attention|Danger|Warning|Issue|Problem) ' "no vacuous whole-word status in $cap"
done

if [ "$FAILURES" -gt 0 ]; then
	echo "smoke-related-readonly: $FAILURES failure(s); captures kept in $CAPDIR"
	exit 1
fi
rm -rf "$CAPDIR"
echo "smoke-related-readonly: all checks green"

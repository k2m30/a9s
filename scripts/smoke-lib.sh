# smoke-lib.sh — sourced by the demo smokes (scripts/smoke-*-demo.sh,
# scripts/smoke-demo.sh). One copy of the session setup, the checks, the
# paging wait and the demo counts, so a fix to any of them reaches every
# script.
#
# A smoke never types a number or a row position the demo data decides:
# counts come from demo_count, which reads the fixture store `a9s --demo`
# loads, and a row is reached by filtering on its name (select_row).

smoke_start() {
	SMOKE="$1"
	shift
	SESSION="a9s-$SMOKE-$$"
	# The smoke drives the REPO binary (./a9s), not a temp copy — every
	# smoke/gate pass leaves the user's binary fresh by construction.
	BIN="./a9s"
	CAPDIR="$(mktemp -d "${TMPDIR:-/tmp}/a9s-$SMOKE.XXXXXX")"
	FAILURES=0

	command -v tmux >/dev/null 2>&1 || { echo "$SMOKE: tmux is required"; exit 1; }

	echo "$SMOKE: make build (refreshes ./a9s)"
	make build || exit 1
	go run ./cmd/democount > "$CAPDIR/demo_counts.txt" || { echo "$SMOKE: go run ./cmd/democount failed"; exit 1; }

	trap smoke_cleanup EXIT

	# A cache root of the run's own: the demo keeps a cache like any other
	# session, and a smoke must neither read the operator's nor leave anything
	# in it. Fresh per run, so every run starts cold and asserts the same thing.
	A9S_CONFIG_FOLDER="$(mktemp -d "${TMPDIR:-/tmp}/a9s-smoke-home.XXXXXX")"
	export A9S_CONFIG_FOLDER

	tmux new-session -d -s "$SESSION" -x 220 -y 50 "$BIN --demo $*"
	wait_boot
}

smoke_cleanup() {
	tmux kill-session -t "$SESSION" 2>/dev/null
	[ -n "${A9S_CONFIG_FOLDER:-}" ] && rm -rf "$A9S_CONFIG_FOLDER"
}

# wait_boot polls for probe data on the menu: a keystroke sent before the menu
# accepts input is dropped silently, and boot is slowest exactly when the gate
# runs the smokes back to back.
wait_boot() {
	i=0
	while [ "$i" -lt 30 ]; do
		tmux capture-pane -t "$SESSION" -p > "$CAPDIR/menu.txt" 2>/dev/null
		if grep -qE 'issues:[0-9]+' "$CAPDIR/menu.txt"; then
			return 0
		fi
		sleep 1
		i=$((i + 1))
	done
	echo "$SMOKE: app did not reach a populated menu within 30s; capture in $CAPDIR/menu.txt" >&2
	exit 1
}

# demo_count TYPE prints the count TYPE's demo list title shows ("44", or
# "50+" for a list served in pages); "resource-types" is the menu's.
demo_count() {
	n="$(awk -v t="$1" '$1 == t { print $2 }' "$CAPDIR/demo_counts.txt")"
	if [ -z "$n" ]; then
		echo "$SMOKE: democount has no entry for $1" >&2
		exit 1
	fi
	echo "$n"
}

keys() {
	tmux send-keys -t "$SESSION" "$@"
}

capture() {
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/$1"
}

snap() {
	sleep "$2"
	capture "$1"
}

open_and_capture() {
	keys ":$1" Enter
	snap "$2" "$3"
}

# select_row NAME filters the open list down to the row named NAME, so the
# next key acts on that row whatever the fixtures put first.
select_row() {
	keys "/$1" Enter
	sleep 1
}

expect() {
	if grep -qF -- "$2" "$CAPDIR/$1"; then
		echo "PASS  $3"
	else
		echo "FAIL  $3 — missing \"$2\" in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	fi
}

expect_re() {
	if grep -qE -- "$2" "$CAPDIR/$1"; then
		echo "PASS  $3"
	else
		echo "FAIL  $3 — no match for /$2/ in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	fi
}

forbid() {
	if grep -qF -- "$2" "$CAPDIR/$1"; then
		echo "FAIL  $3 — forbidden \"$2\" present in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	else
		echo "PASS  $3"
	fi
}

forbid_re() {
	if grep -qE -- "$2" "$CAPDIR/$1"; then
		echo "FAIL  $3 — forbidden /$2/ present in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	else
		echo "PASS  $3"
	fi
}

# wait_for re-captures the pane up to 10x1s until the marker appears, instead
# of a fixed sleep racing call-then-render latency.
wait_for() {
	i=0
	while [ "$i" -lt 10 ]; do
		capture "$1"
		if grep -qF -- "$2" "$CAPDIR/$1"; then
			echo "PASS  $3"
			return 0
		fi
		i=$((i + 1))
		sleep 1
	done
	echo "FAIL  $3 — timed out waiting for \"$2\" in $CAPDIR/$1"
	FAILURES=$((FAILURES + 1))
}

# page_until is wait_for for a view that scrolls: where the marker lands
# depends on how much renders above it, so it pages down between polls. It
# starts from the top, so the order of two calls on one view does not matter.
page_until() {
	max="${4:-12}"
	keys g
	# The first capture must see the top page: a capture taken before the
	# jump renders is followed by NPage, and the top page is never checked.
	sleep 1
	i=0
	while [ "$i" -le "$max" ]; do
		capture "$1"
		if grep -qF -- "$2" "$CAPDIR/$1"; then
			echo "PASS  $3"
			return 0
		fi
		[ "$i" -lt "$max" ] && keys NPage
		sleep 1
		i=$((i + 1))
	done
	echo "FAIL  $3 — timed out paging (>$max pages) waiting for \"$2\" in $CAPDIR/$1"
	FAILURES=$((FAILURES + 1))
}

smoke_finish() {
	if [ "$FAILURES" -gt 0 ]; then
		echo "$SMOKE: $FAILURES failure(s); captures kept in $CAPDIR"
		exit 1
	fi
	rm -rf "$CAPDIR"
	echo "$SMOKE: all checks green"
}

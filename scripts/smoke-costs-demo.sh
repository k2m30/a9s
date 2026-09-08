#!/bin/sh
# smoke-costs-demo.sh — deterministic Cost Explorer smoke over the demo
# fixture store.
#
# Drives the real binary in a headless tmux session through the cost walks
# users actually exercised: open at the current month, zoom month→week→day
# and back out past year without a CE validation error, cycle metrics,
# pivot by account, drill the planted growth story down to usage types,
# hit the 14-day resource boundary message on old cells, reach synthetic
# resource rows on the current month, and start straight into the screen
# via `-c costs`. The demo cost fixtures are anchored to the current month
# at process start, so every assertion here stays valid as wall-clock time
# moves. Runs in ~60s; part of `make ready-to-push` (Stage 6).
#
# Requires tmux. Fails fast with the offending capture on any miss.
set -u

SESSION="a9s-smoke-costs-$$"
BIN="./a9s"
CAPDIR="$(mktemp -d "${TMPDIR:-/tmp}/a9s-smoke-costs.XXXXXX")"
FAILURES=0

command -v tmux >/dev/null 2>&1 || { echo "smoke-costs: tmux is required"; exit 1; }

echo "smoke-costs: make build (refreshes ./a9s)"
make build || exit 1

cleanup() {
	tmux kill-session -t "$SESSION" 2>/dev/null
}
trap cleanup EXIT

# Current-month column header, e.g. "Jul'26*" (the open-period marker).
# BSD date (darwin) first, GNU date fallback.
CUR_LABEL="$(date "+%b'%y")*"
# Drilled columns render day ranges ("Jan 1–4"), so match the month name only.
GROWTH_LABEL="$(date -v-6m "+%b" 2>/dev/null || date -d '-6 months' "+%b")"

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

expect_absent() {
	# $1 = capture file, $2 = forbidden substring, $3 = human label
	if grep -qF -- "$2" "$CAPDIR/$1"; then
		echo "FAIL  $3 — forbidden \"$2\" present in $CAPDIR/$1"
		FAILURES=$((FAILURES + 1))
	else
		echo "PASS  $3"
	fi
}

snap() {
	# $1 = capture file, $2 = settle seconds
	sleep "$2"
	tmux capture-pane -t "$SESSION" -p > "$CAPDIR/$1"
}

keys() {
	tmux send-keys -t "$SESSION" "$@"
}

# The demo profile persists its cost cache like any profile; a stale file
# from an earlier run (older fixture anchor or names) would poison every
# assertion below. Start cold.
rm -f "$HOME/.a9s/cache/demo--costs.yaml"

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
	echo "smoke-costs: app did not reach a populated menu within 30s" >&2
	exit 1
fi

# --- open the grid ---------------------------------------------------------
keys ':costs' Enter
snap grid.txt 3
expect     grid.txt "Costs: by service · invoice · month" "grid opens with the by-service invoice title"
expect     grid.txt "$CUR_LABEL"                          "current month visible with the open-period marker"
expect     grid.txt "Elastic Compute Cloud - Compute"     "growth-story service row present"
expect_re  grid.txt "^│Relational Database Service"       "vendor prefix stripped from service labels"
expect_absent grid.txt "Amazon "                          "no raw Amazon-prefixed labels"
expect_absent grid.txt "CE calls"                         "no API-call counter in the footer"
expect_absent grid.txt "… others"                         "no others-fold row"
expect_absent grid.txt "-0.0"                             "no negative-zero amounts"
expect     grid.txt "TOTAL"                               "pinned TOTAL row"
expect     grid.txt "b Metric"                            "key-hint bar rendered"

# --- zoom chain on the current month: month → week → day → back out --------
keys '+'
snap week.txt 3
expect     week.txt "· week"  "zoom-in lands on week granularity"
expect_re  week.txt "TOTAL +[0-9,]*[1-9]" "week view carries real data (TOTAL non-zero)"
keys '+'
snap day.txt 3
expect     day.txt "· day"   "second zoom-in lands on day granularity"
expect_re  day.txt "TOTAL +[0-9,]*[1-9]" "day view carries real data (TOTAL non-zero)"
keys '-'
keys '-'
snap backout.txt 2
expect     backout.txt "· month" "zoom-out returns to month granularity"
expect_absent backout.txt "operation error" "no CE error after zoom round-trip"
keys '-'
snap year.txt 3
expect     year.txt "· year" "third zoom-out lands on year granularity"
expect_absent year.txt "operation error"        "no CE validation error at year zoom"
expect_absent year.txt "ValidationException"    "no raw ValidationException at year zoom"
keys '+'
snap remonth.txt 2

# --- metric cycle: unblended drops the Tax row ------------------------------
keys 'b'
snap unblended.txt 3
expect     unblended.txt "· unblended" "b cycles to the unblended metric"
expect_absent unblended.txt "Tax" "unblended view excludes the Tax row"
keys 'b'
keys 'b'
keys 'b'
keys 'b'
snap invoice_again.txt 2
expect     invoice_again.txt "· invoice" "metric cycle returns to invoice"

# --- account pivot -----------------------------------------------------------
keys '3'
snap account.txt 3
expect     account.txt "by linked_account" "digit 3 pivots to accounts"
expect     account.txt "123456789012"      "account row present"
# '0' resets to the default view (service pivot, drills popped, cursor on
# the newest period) — the deterministic anchor for the walks below.
keys '0'
snap byservice.txt 2

# --- growth-story drill: service × growth month → usage types ---------------
# The growth service (Elastic Compute Cloud - Compute) is the largest-total
# row, so it already sits at row 0 after digit-0's reset — no row navigation
# needed, only the column scroll back to the planted growth month.
keys Left Left Left Left Left Left
snap growth_cell.txt 1
keys Enter
snap drill.txt 3
expect     drill.txt "Elastic Compute Cloud - Compute" "drill narrows to the growth service"
expect     drill.txt "by usage_type"                   "drill pivots rows to usage types"
expect     drill.txt "g5.xlarge"                        "planted g5.xlarge usage type visible"
expect     drill.txt "$GROWTH_LABEL"                    "drill breadcrumb names the growth month"

# --- resource boundary: old cell refuses with the honest reason -------------
keys Enter
snap refused.txt 2
expect     refused.txt "14 days" "resource drill outside the window explains itself"
keys Escape
snap popped.txt 2
expect     popped.txt "by service" "Esc pops the drill back to the service grid"

# --- resource rows inside the window (current month) ------------------------
# '0' is the deterministic anchor: reset to the default view (service pivot,
# drills popped, cursor on the newest period).
keys '0'
snap reset.txt 2
expect     reset.txt "by service · invoice · month" "digit 0 resets to the default view"
keys Enter
snap cur_drill.txt 3
expect     cur_drill.txt "by usage_type" "current-month drill pivots to usage types"
keys Enter
snap resources.txt 3
expect_re  resources.txt "i-[0-9a-f]" "resource drill reaches synthetic instance IDs"
# The final hop of the "why" chain: Enter on a resource row opens the real
# EC2 detail view; Esc returns to the costs grid where the drill left off.
keys Enter
snap jump.txt 4
expect     jump.txt "detail --"          "Enter on a resource row opens the resource detail view"
expect_re  jump.txt "i-[0-9a-f]"         "detail view shows the drilled instance"
expect_absent jump.txt "by resource_id"  "the costs grid is no longer the active screen"
keys Escape
snap back_from_jump.txt 2
expect     back_from_jump.txt "by resource_id" "Esc returns from the detail to the costs resource level"
keys '0'
snap unwound.txt 2

# --- help section ------------------------------------------------------------
keys '?'
snap help.txt 2
expect     help.txt "COST EXPLORER" "help shows the Cost Explorer section"
expect_re  help.txt "[Ss]ort"       "help documents the sort rule"
# Escape closes the overlay; 'q' would quit the whole app (app-wide help
# behavior, tracked separately).
keys Escape
sleep 1

# --- Esc returns to the menu -------------------------------------------------
# The '0' before the help section left the screen at the drill root, so a
# single Esc pops the whole costs screen.
keys Escape
snap menu.txt 2
expect     menu.txt "resource-types(" "Esc leaves the costs screen back to the menu"

# --- direct start via -c ------------------------------------------------------
tmux kill-session -t "$SESSION" 2>/dev/null
tmux new-session -d -s "$SESSION" -x 220 -y 50 "$BIN --demo -c costs"
snap direct.txt 5
expect     direct.txt "Costs: by service · invoice · month" "-c costs starts straight into the grid"

echo
if [ "$FAILURES" -gt 0 ]; then
	echo "smoke-costs: $FAILURES failure(s); captures in $CAPDIR"
	exit 1
fi
echo "smoke-costs: all checks passed; captures in $CAPDIR"

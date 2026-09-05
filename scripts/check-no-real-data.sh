#!/bin/sh
# check-no-real-data.sh — block real AWS/environment identifiers from the repo.
#
# NOTHING sensitive is stored in this (public) repo:
#   * The committed check is a GENERIC account-ID-in-ARN heuristic — a pattern,
#     not any real value. Synthetic/example IDs are allow-listed PER account ID
#     (a synthetic ARN never masks a real one on the same line); bare 12-digit
#     numbers (timestamps) are ignored — only ARN-embedded ones are accounts.
#   * Exact forbidden TERMS live ONLY in .githooks/sensitive_patterns.txt —
#     local, never committed, not even hashed (a 12-digit ID or short name is
#     brute-forced from a hash in seconds). That file must NEVER be tracked;
#     this scanner hard-fails if it ever is.
#
# Scope: term enforcement is on NEW content only (prevent new leaks); a
# pre-existing occurrence never blocks unrelated work. Use --audit to hunt
# existing leaks across the whole tree.
#
# Modes: (default) tree | --staged | --diff (added lines on stdin) | --audit
set -u

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || echo .)
cd "$ROOT" || exit 2

MODE=tree
case "${1:-}" in
	--staged) MODE=staged ;;
	--diff) MODE=diff ;;
	--audit) MODE=audit ;;
esac

ALLOW_ACCT='123456789012|111122223333|111222333444|210987654321|987654321098|123456789000|000000000000|111111111111|222222222222|333333333333|444444444444|555555555555|666666666666|777777777777|888888888888|999999999999|999988887777'
PATTERNS="$ROOT/.githooks/sensitive_patterns.txt"
DENYLIST_PATH='.githooks/sensitive_patterns.txt'
PATHSPEC=". :!:scripts/check-no-real-data.sh :!:.githooks/sensitive_patterns.txt :!:docs/historical :!:.a9s/views_reference.yaml :!:graphify-out"

EFF=""
if [ -f "$PATTERNS" ]; then
	EFF=$(grep -vE '^[[:space:]]*(#|$)' "$PATTERNS" 2>/dev/null | paste -sd '|' -)
fi

# arn_check: stdin -> "lineno:line" for each line carrying an ARN whose account
# ID is NOT allow-listed. The allow-list is applied PER account ID, so a
# synthetic ARN on the same line as a real one does not hide the real one.
arn_check() {
	perl -e '
		my %allow = map { ($_ => 1) } split /\|/, shift @ARGV;
		while (my $l = <STDIN>) {
			my @a = $l =~ /arn:aws[a-z-]*:[a-zA-Z0-9*-]*:[a-z0-9-]*:([0-9]{12}):/g;
			print "$.:$l" if grep { !$allow{$_} } @a;
		}
	' "$ALLOW_ACCT"
}
# term_check: stdin -> matching lines against the local term list (if any).
term_check() { [ -n "$EFF" ] && grep -niE "$EFF" 2>/dev/null; }
# key_check: stdin -> lines carrying an AWS access-key-ID-shaped token that is
# not visibly synthetic. GitHub push protection rejects the whole push for a
# key-shaped literal, so a synthetic key in a test must carry EXAMPL or XMP
# inside the token (the AWS documentation examples do).
key_check() {
	perl -ne 'print "$.:$_" if grep { !/EXAMPL|XMP/ } /\b(?:AKIA|ASIA)[0-9A-Z]{16}\b/g'
}

fail=0

# Structural invariant (all modes): the local deny-list must never be tracked or
# staged, whatever its contents.
if git ls-files --error-unmatch "$DENYLIST_PATH" >/dev/null 2>&1 \
	|| git diff --cached --name-only -- "$DENYLIST_PATH" 2>/dev/null | grep -q .; then
	echo "BLOCKED: $DENYLIST_PATH is tracked/staged — it is the local deny-list and must NEVER be committed."
	fail=1
fi

case "$MODE" in
diff|staged)
	if [ "$MODE" = diff ]; then
		added=$(grep -E '^\+' | grep -vE '^\+\+\+' | sed 's/^+//')
	else
		added=$(git diff --cached -U0 --no-color -- $PATHSPEC | grep -E '^\+' | grep -vE '^\+\+\+' | sed 's/^+//')
	fi
	a=$(printf '%s\n' "$added" | arn_check)
	t=$(printf '%s\n' "$added" | term_check)
	k=$(printf '%s\n' "$added" | key_check)
	if [ -n "$a" ] || [ -n "$t" ] || [ -n "$k" ]; then
		echo "BLOCKED: real identifier in the ${MODE} changes —"
		[ -n "$a" ] && { echo "  account ID inside an ARN:"; printf '%s\n' "$a" | sed 's/^/    /'; }
		[ -n "$k" ] && { echo "  access-key-shaped token without EXAMPL/XMP (GitHub push protection rejects it):"; printf '%s\n' "$k" | sed 's/^/    /'; }
		[ -n "$t" ] && { echo "  forbidden term (local .githooks/sensitive_patterns.txt):"; printf '%s\n' "$t" | sed 's/^/    /'; }
		fail=1
	fi
	;;
tree|audit)
	report=$(git ls-files -- $PATHSPEC | while IFS= read -r f; do
		[ -f "$f" ] || continue
		a=$(arn_check < "$f")
		[ -n "$a" ] && printf '%s\n' "$a" | sed "s|^|$f:|"
		if [ "$MODE" = audit ]; then
			t=$(term_check < "$f")
			[ -n "$t" ] && printf '%s\n' "$t" | sed "s|^|$f:|"
		fi
	done)
	if [ -n "$report" ]; then
		echo "BLOCKED: real identifier(s) in tracked file(s) —"
		printf '%s\n' "$report" | sed 's/^/  /'
		fail=1
	fi
	# Tree mode also term-scans NEW content (worktree vs merge-base with
	# origin/main) so the gate enforces the term list even when git hooks are
	# not installed. Whole-tree term scanning stays audit-only by design.
	if [ "$MODE" = tree ]; then
		base=$(git merge-base origin/main HEAD 2>/dev/null)
		if [ -n "$base" ]; then
			new_content=$(git diff "$base" -U0 --no-color -- $PATHSPEC | grep -E '^\+' | grep -vE '^\+\+\+' | sed 's/^+//')
			nt=$(printf '%s\n' "$new_content" | term_check)
			if [ -n "$nt" ]; then
				echo "BLOCKED: forbidden term in new content (vs merge-base with origin/main) —"
				printf '%s\n' "$nt" | sed 's/^/    /'
				fail=1
			fi
			nk=$(printf '%s\n' "$new_content" | key_check)
			if [ -n "$nk" ]; then
				echo "BLOCKED: access-key-shaped token in new content (GitHub push protection rejects it) —"
				printf '%s\n' "$nk" | sed 's/^/    /'
				fail=1
			fi
		fi
	fi
	;;
esac

if [ "$fail" -ne 0 ]; then
	echo ""
	echo "check-no-real-data: real environment identifiers must never be committed or pushed."
	echo "Use synthetic values (123456789012, example-readonly) or <placeholders>."
	echo "Exact terms are maintained locally in .githooks/sensitive_patterns.txt (never committed)."
	exit 1
fi
echo "check-no-real-data: clean"
exit 0

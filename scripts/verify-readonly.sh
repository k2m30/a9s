#!/bin/sh
# verify-readonly.sh — a9s must never issue write API calls to AWS.
#
# Token-based scan. Per line, in order: string/rune literals are blanked
# (so "bucket/*" cannot open a fake comment), // and /* */ comments are
# stripped (multi-line block comments and multi-line raw strings carry
# state across lines), then every method CALL `.<WriteVerb>Xxx(` is
# flagged — tolerating whitespace around the dot and before the call
# paren, Go's legal line break after the dot (`c.` newline `Delete(`),
# and a block comment splitting the name from its paren
# (`c.PutObject /*x*/ (in)`). Exemptions are exact method-name matches,
# never line-level substrings.
#
# File-level exclusions: *_test.go, interface declaration files (SDK method
# signatures look like calls), client.go (constructs SDK clients), errors.go,
# profile.go, regions.go.
#
# Token exemptions:
#   CreateServiceClients — local helper in core/aws/client.go constructing SDK client structs
#   ExecuteTaskAt        — local runtime executor helper, not an API call
#
# The durable answer is a go/analysis checker over real ASTs; this scan is
# the fast gate.
set -u

VERBS='Create|Delete|Update|Put|Modify|Terminate|Stop|Reboot|Execute|Send|Publish|Remove|Start|Cancel|Attach|Detach|Associate|Disassociate|Register|Deregister|Enable|Disable|Restore|Invoke|Revoke|Authorize'
EXEMPT='CreateServiceClients|ExecuteTaskAt'

echo "Checking for write API calls in core/aws/ and core/runtime/..."
hits=$(perl -ne '
	BEGIN { $prev_dot = 0; $in_block = 0; $in_raw = 0; $pending = ""; }
	sub reset_state { $prev_dot = 0; $in_block = 0; $in_raw = 0; $pending = ""; }
	if ($ARGV =~ m{(_test\.go|interfaces\.go|/errors\.go|/client\.go|/profile\.go|/regions\.go)$}) {
		if (eof) { close ARGV; reset_state(); } next;
	}
	if ($in_raw) {
		if (s{^[^`]*`}{``}) { $in_raw = 0; } else { if (eof) { close ARGV; reset_state(); } next; }
	}
	if ($in_block) {
		if (s{^.*?\*/}{ }) { $in_block = 0; } else { if (eof) { close ARGV; reset_state(); } next; }
	}
	s{\x27(?:[^\x27\\]|\\.)*\x27}{R}g;
	s{`[^`]*`}{``}g;
	$in_raw = 1 if s{`[^`]*$}{``};
	s{"(?:[^"\\]|\\.)*"}{""}g;
	s{/\*.*?\*/}{ }g;
	my $opened = s{/\*.*$}{ };
	s{//.*}{};
	$in_block = 1 if $opened;
	my $call = qr/(?:RunInstances|(?:'"$VERBS"')[A-Z][A-Za-z0-9]*)/;
	my $exempt = qr/^(?:'"$EXEMPT"')$/;
	if ($pending ne "" && /^\s*\(/) {
		print "$ARGV:$.:$pending\n" unless $pending =~ $exempt;
	}
	$pending = "";
	if ($prev_dot && /^\s*($call)\s*\(/) {
		print "$ARGV:$.:$1\n" unless $1 =~ $exempt;
	}
	while (/\.\s*($call)\s*\(/g) {
		print "$ARGV:$.:$1\n" unless $1 =~ $exempt;
	}
	if ($opened) {
		if (/\.\s*($call)\s*$/) { $pending = $1; }
		elsif ($prev_dot && /^\s*($call)\s*$/) { $pending = $1; }
	}
	$prev_dot = (/\.\s*$/) ? 1 : 0;
	if (eof) { close ARGV; reset_state(); }
' core/aws/*.go core/runtime/*.go)

if [ -n "$hits" ]; then
	printf '%s\n' "$hits"
	echo "FAIL: Write API calls detected!"
	exit 1
fi
echo "PASS: All API calls are read-only"

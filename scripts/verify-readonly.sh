#!/bin/sh
# verify-readonly.sh — a9s must never issue write API calls to AWS.
#
# Token-based scan: strips // comments (so a trailing comment cannot hide a
# call on the same line), then flags every method CALL `.<WriteVerb>Xxx(`.
# Exemptions are exact method-name matches, never line-level substrings — a
# line mixing a read call and a write call still fails on the write token.
#
# File-level exclusions: *_test.go, interface declaration files (SDK method
# signatures look like calls), client.go (constructs SDK clients), errors.go,
# profile.go, regions.go.
#
# Token exemptions:
#   CreateServiceClients — local helper in core/aws/client.go constructing SDK client structs
#   ExecuteTaskAt        — local runtime executor helper, not an API call
set -u

VERBS='Create|Delete|Update|Put|Modify|Terminate|Stop|Reboot|Execute|Send|Publish|Remove|Start|Cancel|Attach|Detach|Associate|Disassociate|Register|Deregister|Enable|Disable|Restore|Invoke|Revoke|Authorize'
EXEMPT='CreateServiceClients|ExecuteTaskAt'

echo "Checking for write API calls in core/aws/ and core/runtime/..."
hits=$(perl -sne '
	next if $ARGV =~ m{(_test\.go|interfaces\.go|/errors\.go|/client\.go|/profile\.go|/regions\.go)$};
	s|//.*||;
	while (/\.((?:RunInstances|(?:'"$VERBS"')[A-Z][A-Za-z0-9]*))\(/g) {
		my $m = $1;
		next if $m =~ /^(?:'"$EXEMPT"')$/;
		print "$ARGV:$.:$m\n";
	}
	close ARGV if eof;
' core/aws/*.go core/runtime/*.go)

if [ -n "$hits" ]; then
	printf '%s\n' "$hits"
	echo "FAIL: Write API calls detected!"
	exit 1
fi
echo "PASS: All API calls are read-only"

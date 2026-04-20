package unit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// webhookPattern holds a compiled regex and a function that decides whether
// the captured groups represent a known-safe placeholder.
type webhookPattern struct {
	name          string
	re            *regexp.Regexp
	isPlaceholder func(groups []string) bool
}

var webhookPatterns = []webhookPattern{
	{
		name: "Slack",
		// Full URL: https://hooks.slack.com/services/<workspace>/<channel>/<token>
		re: regexp.MustCompile(`https://hooks\.slack\.com/services/([A-Z0-9]+)/([A-Z0-9]+)/([A-Za-z0-9]+)`),
		// Allowed placeholder: workspace is T followed by all zeros, channel is B followed by all zeros,
		// token is all X (case-insensitive match on token per observed fixture value).
		// Reference fixture: T00000000/B00000000/XXXXXXXX (fixtures_messaging.go lines 266, 272)
		isPlaceholder: func(groups []string) bool {
			workspace := groups[1]
			channel := groups[2]
			token := groups[3]
			return isAllZeroWithPrefix(workspace, 'T') &&
				isAllZeroWithPrefix(channel, 'B') &&
				isAllChar(token, 'X')
		},
	},
	{
		name: "Discord",
		// Full URL: https://discord.com/api/webhooks/<id>/<token>
		//        or https://discordapp.com/api/webhooks/<id>/<token>
		re: regexp.MustCompile(`https://discord(?:app)?\.com/api/webhooks/(\d+)/([A-Za-z0-9_-]+)`),
		// Allowed placeholder: id is all zeros, token is all X or all zeros.
		isPlaceholder: func(groups []string) bool {
			id := groups[1]
			token := groups[2]
			return isAllChar(id, '0') &&
				(isAllChar(token, 'X') || isAllChar(token, '0'))
		},
	},
	{
		name: "PagerDuty",
		// Full URL: https://events.pagerduty.com/integration/<key>/enqueue
		re: regexp.MustCompile(`https://events\.pagerduty\.com/integration/([a-f0-9]{32})/enqueue`),
		// Allowed placeholder: integration key is all zeros or all 'f'.
		isPlaceholder: func(groups []string) bool {
			key := groups[1]
			return isAllChar(key, '0') || isAllChar(key, 'f')
		},
	},
}

// isAllZeroWithPrefix returns true when s starts with prefix and the rest of
// the string consists entirely of '0' characters.
func isAllZeroWithPrefix(s string, prefix byte) bool {
	if len(s) < 2 || s[0] != prefix {
		return false
	}
	return strings.Count(s[1:], "0") == len(s)-1
}

// isAllChar returns true when every byte in s equals ch.
func isAllChar(s string, ch byte) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != ch {
			return false
		}
	}
	return true
}

// TestDemo_NoRealWebhookURLs walks every .go file under internal/demo/ and
// verifies that any webhook URL found matches an approved placeholder shape.
// It fails with the file path and line number of any non-placeholder match,
// acting as a regression guard against real credentials being committed.
//
// go test sets the working directory to the package directory (tests/unit/),
// so the path to internal/demo/ is two levels up.
func TestDemo_NoRealWebhookURLs(t *testing.T) {
	demoDir := filepath.Join("..", "..", "internal", "demo")

	err := filepath.WalkDir(demoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("could not read %s: %v", path, readErr)
			return nil
		}

		for _, wp := range webhookPatterns {
			allMatches := wp.re.FindAllSubmatchIndex(contents, -1)
			for _, loc := range allMatches {
				// loc[0]:loc[1] is the full match; loc[2n]:loc[2n+1] are group n pairs.
				groups := make([]string, len(loc)/2)
				for i := range groups {
					start, end := loc[i*2], loc[i*2+1]
					if start < 0 {
						groups[i] = ""
					} else {
						groups[i] = string(contents[start:end])
					}
				}

				if !wp.isPlaceholder(groups) {
					// Compute a human-readable line number for the match offset.
					matchStart := loc[0]
					lineNum := 1 + strings.Count(string(contents[:matchStart]), "\n")
					t.Errorf("%s: real %s webhook URL found at line %d: %s",
						path, wp.name, lineNum, groups[0])
				}
			}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("WalkDir(%s): %v", demoDir, err)
	}
}

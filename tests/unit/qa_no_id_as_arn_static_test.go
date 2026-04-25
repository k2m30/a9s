package unit

// qa_no_id_as_arn_static_test.go — Static guard against the recurring
// "fetcher emits ID = bare name, enricher passes r.ID as *Arn" bug.
//
// First found in tg_issue_enrichment.go (TestEnrichTargetGroupHealth_UsesARNFromFields),
// then sfn and elb the same day. All three are the same anti-pattern:
//
//   <Field>Arn: aws.String(r.ID),
//
// when the corresponding fetcher actually sets `ID: <bare-name>` and stores
// the ARN in Fields["<key>_arn"]. Real AWS rejects with InvalidArn /
// ValidationError; demo mode silently returned empty for forgiving fakes.
//
// This static test walks every Wave-2 enricher and related-checker file under
// internal/aws/ and asserts no occurrence of the anti-pattern. If a future
// enricher legitimately needs r.ID as an ARN (because its fetcher does emit
// ID = ARN), assign through a clearly-named local first:
//
//   resourceARN := r.ID  // <fetcher>.go sets ID = ARN intentionally
//   ... <Field>Arn: aws.String(resourceARN), ...
//
// — that variable name + comment make the intent reviewable; an audit can
// then confirm the fetcher really does emit the ARN as ID.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoIDAsARN_StaticGuard fails when any enricher / related-checker file
// passes r.ID directly as a *Arn or *ARN parameter via aws.String / & literal.
func TestNoIDAsARN_StaticGuard(t *testing.T) {
	// Match field literals like:
	//   FooArn: aws.String(r.ID)
	//   FooARN: aws.String(r.ID)
	//   FooArn: &r.ID
	patAWSString := regexp.MustCompile(`A[Rr][Nn]:\s*aws\.String\(r\.ID\)`)
	patAddr := regexp.MustCompile(`A[Rr][Nn]:\s*&r\.ID\b`)
	// Indirect: local := r.ID (or res.ID). Captures the local name so we
	// can then look for that same name being passed to an *Arn: field.
	patLocalFromID := regexp.MustCompile(`^\s*(\w+)\s*:=\s*(?:r|res|rsrc|resource)\.ID\s*$`)

	roots := []string{
		findRepoFile(t, "internal/aws"),
	}
	t.Logf("scanning roots: %v", roots)

	var hits []string
	enricherFilesScanned := 0
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			name := info.Name()
			// Restrict to enricher and related files where this pattern is meaningful.
			isEnricher := strings.HasSuffix(name, "_issue_enrichment.go") ||
				strings.HasSuffix(name, "_detail_enrichment.go") ||
				strings.HasSuffix(name, "_related.go") ||
				strings.HasSuffix(name, "_related_extra.go")
			if !isEnricher {
				return nil
			}
			enricherFilesScanned++
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			lines := strings.Split(string(body), "\n")
			hasEscapeHatch := func(ln int) bool {
				for i := max(0, ln-3); i <= ln+3 && i < len(lines); i++ {
					if strings.Contains(lines[i], "fetcher emits ID=ARN") {
						return true
					}
				}
				return false
			}
			// Pass 1: collect `<local> := r.ID` assignments with line numbers.
			locals := map[string]int{} // local name → line of the assignment
			for ln, line := range lines {
				trimmed := strings.TrimLeft(line, " \t")
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if m := patLocalFromID.FindStringSubmatch(line); m != nil {
					locals[m[1]] = ln
				}
			}
			// Pass 2: scan for direct patterns and `*Arn: aws.String(<local>)`
			// or `*Arn: &<local>` where `<local>` was assigned from r.ID.
			for ln, line := range lines {
				trimmed := strings.TrimLeft(line, " \t")
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if patAWSString.FindStringIndex(line) != nil || patAddr.FindStringIndex(line) != nil {
					hits = append(hits, path+":"+itoa(ln+1)+": "+strings.TrimSpace(line))
					continue
				}
				for local, defLn := range locals {
					if hasEscapeHatch(defLn) {
						continue
					}
					viaAWSString := "A" + "rn: aws.String(" + local + ")"
					viaAWSStringU := "A" + "RN: aws.String(" + local + ")"
					viaAddr := "A" + "rn: &" + local
					viaAddrU := "A" + "RN: &" + local
					if strings.Contains(line, viaAWSString) || strings.Contains(line, viaAWSStringU) ||
						strings.Contains(line, viaAddr) || strings.Contains(line, viaAddrU) {
						hits = append(hits, path+":"+itoa(ln+1)+": "+strings.TrimSpace(line)+
							"  (indirect — local "+local+" was set from r.ID at line "+itoa(defLn+1)+
							"; if fetcher really emits ID=ARN, add `// fetcher emits ID=ARN` comment near the assignment)")
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %q: %v", root, err)
		}
	}

	t.Logf("scanned %d enricher / related files; %d hits", enricherFilesScanned, len(hits))
	if enricherFilesScanned == 0 {
		t.Fatalf("file walk found no enricher / related files under %q — regex would never fire (false-pass guard)",
			roots)
	}

	if len(hits) > 0 {
		t.Fatalf("anti-pattern detected — *Arn parameter receiving r.ID directly. "+
			"If the fetcher really emits ID=ARN, assign via a named local with a comment "+
			"(e.g. `resourceARN := r.ID  // fetcher emits ID=ARN`) and re-run.\n"+
			"Found %d occurrence(s):\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// findRepoFile resolves a path relative to the repository root by walking up
// from cwd until a go.mod is found. Tests run from tests/unit so the relative
// path "internal/aws" is two levels up.
func findRepoFile(t *testing.T, rel string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, rel)
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("go.mod not found above %q", cwd)
	return ""
}

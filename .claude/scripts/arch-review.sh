#!/usr/bin/env bash
# Architecture Review Script for a9s
# Runs deterministic checks from docs/go-codebase-checklist.md
# Output: structured PASS/FAIL report to stdout
#
# Usage: bash .claude/scripts/arch-review.sh [project_root]

set -euo pipefail

ROOT="${1:-$(pwd)}"
cd "$ROOT"

PASS=0
FAIL=0
WARN=0
REPORT=""

pass() { PASS=$((PASS + 1)); REPORT+="  PASS  $1"$'\n'; }
fail() { FAIL=$((FAIL + 1)); REPORT+="  FAIL  $1"$'\n'; }
warn() { WARN=$((WARN + 1)); REPORT+="  WARN  $1"$'\n'; }
section() { REPORT+=$'\n'"== $1 =="$'\n'; }
detail() { REPORT+="        $1"$'\n'; }

# ============================================================================
section "FILE SIZE (>500 lines, excluding tests and demo fixtures)"
# ============================================================================

large_files=$(find internal cmd -name "*.go" \
  ! -name "*_test.go" \
  ! -path "*/demo/fixtures_*" \
  ! -path "cmd/preview*" \
  ! -path "cmd/refgen/*" \
  -exec wc -l {} + 2>/dev/null \
  | grep -v ' total$' \
  | awk '$1 > 500 {print $0}' \
  | sort -rn || true)

if [ -z "$large_files" ]; then
  pass "No source files exceed 500 lines"
else
  fail "Source files exceed 500 lines"
  while IFS= read -r line; do
    detail "$line"
  done <<< "$large_files"
fi

# ============================================================================
section "FUNCTION SIZE (>50 lines, approximate)"
# ============================================================================

# Use awk to find functions >50 lines in non-test, non-demo-fixture files
long_funcs=$(find internal cmd -name "*.go" \
  ! -name "*_test.go" \
  ! -path "*/demo/fixtures_*" \
  ! -path "cmd/preview*" \
  ! -path "cmd/refgen/*" \
  -print0 2>/dev/null \
  | xargs -0 awk '
    /^func / {
      fname = FILENAME ":" NR ": " $0
      start = NR
      depth = 0
      in_func = 1
    }
    in_func && /{/ { depth++ }
    in_func && /}/ {
      depth--
      if (depth <= 0) {
        lines = NR - start + 1
        if (lines > 50) {
          # Truncate function signature for display
          sig = fname
          if (length(sig) > 100) sig = substr(sig, 1, 97) "..."
          printf "%d lines  %s\n", lines, sig
        }
        in_func = 0
      }
    }
  ' 2>/dev/null | sort -rn || true)

if [ -z "$long_funcs" ]; then
  pass "No functions exceed 50 lines"
else
  warn "Functions exceed 50 lines (review needed -- Update/View handlers may be acceptable)"
  while IFS= read -r line; do
    detail "$line"
  done <<< "$long_funcs"
fi

# ============================================================================
section "PACKAGE DEPENDENCY VIOLATIONS"
# ============================================================================

# views must not import layout
views_layout=$(grep -rn '".*tui/layout"' internal/tui/views/ 2>/dev/null || true)
if [ -z "$views_layout" ]; then
  pass "views/ does not import layout/"
else
  fail "views/ imports layout/ (forbidden)"
  while IFS= read -r line; do detail "$line"; done <<< "$views_layout"
fi

# views must not import app
views_app=$(grep -rn '".*tui/app"' internal/tui/views/ 2>/dev/null || true)
if [ -z "$views_app" ]; then
  pass "views/ does not import app"
else
  fail "views/ imports app (forbidden)"
  while IFS= read -r line; do detail "$line"; done <<< "$views_app"
fi

# messages must not import tui/
msgs_tui=$(grep -rn '".*tui/' internal/runtime/messages/ 2>/dev/null | grep -v '_test.go' || true)
if [ -z "$msgs_tui" ]; then
  pass "messages/ does not import tui/"
else
  fail "messages/ imports tui/ (forbidden)"
  while IFS= read -r line; do detail "$line"; done <<< "$msgs_tui"
fi

# layout must only import styles (within tui/)
layout_bad=$(grep -rn '".*internal/tui/' internal/tui/layout/ 2>/dev/null | grep -v 'tui/styles' | grep -v '_test.go' || true)
if [ -z "$layout_bad" ]; then
  pass "layout/ only imports styles/ (within tui/)"
else
  fail "layout/ imports non-styles tui/ packages"
  while IFS= read -r line; do detail "$line"; done <<< "$layout_bad"
fi

# styles must not import other internal/ packages
styles_bad=$(grep -rn '".*internal/' internal/tui/styles/ 2>/dev/null | grep -v '_test.go' || true)
if [ -z "$styles_bad" ]; then
  pass "styles/ does not import other internal/ packages"
else
  fail "styles/ imports internal/ packages (should be stdlib + lipgloss only)"
  while IFS= read -r line; do detail "$line"; done <<< "$styles_bad"
fi

# ============================================================================
section "KEY BINDING ANTI-PATTERNS"
# ============================================================================

# Raw string comparison instead of key.Matches
raw_key_cmp=$(grep -rn 'msg\.String().*==' internal/tui/ 2>/dev/null | grep -v '_test.go' || true)
if [ -z "$raw_key_cmp" ]; then
  pass "No raw msg.String() == comparisons found"
else
  fail "Raw string key comparisons (use key.Matches instead)"
  while IFS= read -r line; do detail "$line"; done <<< "$raw_key_cmp"
fi

# Inline key.NewBinding (should be in keys/keys.go only)
inline_bindings=$(grep -rn 'key\.NewBinding' internal/tui/ 2>/dev/null | grep -v 'keys/keys.go' | grep -v '_test.go' || true)
if [ -z "$inline_bindings" ]; then
  pass "All key.NewBinding calls are in keys/keys.go"
else
  fail "Inline key.NewBinding outside keys/keys.go"
  while IFS= read -r line; do detail "$line"; done <<< "$inline_bindings"
fi

# ============================================================================
section "init() LOCATION VIOLATIONS"
# ============================================================================

# init() allowed in: internal/aws/, internal/demo/, internal/tui/styles/
bad_init=$(grep -rn 'func init()' internal/ 2>/dev/null \
  | grep -v 'internal/aws/' \
  | grep -v 'internal/demo/' \
  | grep -v 'internal/tui/styles/' \
  | grep -v '_test.go' \
  || true)

if [ -z "$bad_init" ]; then
  pass "init() functions only in allowed locations (aws/, demo/, styles/)"
else
  fail "init() found in forbidden locations"
  while IFS= read -r line; do detail "$line"; done <<< "$bad_init"
fi

# ============================================================================
section "ERROR HANDLING"
# ============================================================================

# Bare error returns in fetchers (no fmt.Errorf wrapping)
bare_err=$(grep -rn 'return nil, err$' internal/aws/*.go 2>/dev/null || true)
bare_fetch_err=$(grep -rn 'return resource\.FetchResult{}, err$' internal/aws/*.go 2>/dev/null || true)
bare_all="$bare_err"$'\n'"$bare_fetch_err"
bare_all=$(echo "$bare_all" | grep -v '^$' || true)

if [ -z "$bare_all" ]; then
  pass "No bare error returns in fetchers"
else
  warn "Bare error returns in fetchers (consider fmt.Errorf wrapping)"
  while IFS= read -r line; do detail "$line"; done <<< "$bare_all"
fi

# Ignored errors without justifying comment
ignored_errs=$(grep -rn '_ =' internal/ 2>/dev/null \
  | grep -v '_test.go' \
  | grep -v '// ' \
  | grep -v 'nolint' \
  || true)

if [ -z "$ignored_errs" ]; then
  pass "No unjustified ignored errors (_ =)"
else
  warn "Ignored errors without justifying comment"
  while IFS= read -r line; do detail "$line"; done <<< "$ignored_errs"
fi

# ============================================================================
section "CONCURRENCY"
# ============================================================================

# Manual goroutines in tui (should use tea.Cmd)
manual_goroutines=$(grep -rn 'go func' internal/tui/ 2>/dev/null | grep -v '_test.go' || true)
if [ -z "$manual_goroutines" ]; then
  pass "No manual goroutines in tui/ (tea.Cmd used correctly)"
else
  fail "Manual goroutines in tui/ (should use tea.Cmd)"
  while IFS= read -r line; do detail "$line"; done <<< "$manual_goroutines"
fi

# context.Context stored in structs (should be parameter only)
ctx_in_struct=$(grep -rn 'context\.Context' internal/tui/ 2>/dev/null \
  | grep -v '_test.go' \
  | grep -v 'func ' \
  | grep -v '//' \
  || true)

if [ -z "$ctx_in_struct" ]; then
  pass "No context.Context stored in structs"
else
  fail "context.Context appears to be stored in structs (should be function parameter only)"
  while IFS= read -r line; do detail "$line"; done <<< "$ctx_in_struct"
fi

# ============================================================================
section "INTERFACE CHECKS"
# ============================================================================

# AWS interfaces should be single-method
multi_method_aws=$(awk '
  /^type .*API interface/ {
    iface = $2
    methods = 0
    in_iface = 1
    next
  }
  in_iface && /^}/ {
    if (methods > 1) print FILENAME ": " iface " has " methods " methods"
    in_iface = 0
  }
  in_iface && /^\t[A-Z]/ { methods++ }
' internal/aws/*_interfaces.go 2>/dev/null || true)

if [ -z "$multi_method_aws" ]; then
  pass "All AWS interfaces are single-method"
else
  fail "AWS interfaces with >1 method"
  while IFS= read -r line; do detail "$line"; done <<< "$multi_method_aws"
fi

# View interface should have exactly 5 methods
view_methods=$(awk '
  /^type View interface/ { in_iface = 1; methods = 0; next }
  in_iface && /^}/ { print methods; in_iface = 0 }
  in_iface && /^\t[A-Z]/ { methods++ }
' internal/tui/views/view.go 2>/dev/null || true)

if [ "$view_methods" = "5" ]; then
  pass "View interface has exactly 5 methods"
else
  fail "View interface has $view_methods methods (expected 5)"
fi

# ============================================================================
section "NOLINT DIRECTIVES"
# ============================================================================

# Every //nolint must have an explanatory comment
bare_nolint=$(grep -rn '//nolint' internal/ cmd/ 2>/dev/null \
  | grep -v '_test.go' \
  | grep -v '// ' \
  || true)
# The above is tricky -- nolint comments that DO have explanations contain "// " after the nolint
# Better check: nolint without any text after it on the same line
bare_nolint2=$(grep -rn '//nolint:[a-z]*$' internal/ cmd/ 2>/dev/null | grep -v '_test.go' || true)

if [ -z "$bare_nolint2" ]; then
  pass "All //nolint directives have explanatory comments"
else
  fail "//nolint directives without explanatory comments"
  while IFS= read -r line; do detail "$line"; done <<< "$bare_nolint2"
fi

# ============================================================================
section "RENDERING"
# ============================================================================

# len() used for string width instead of lipgloss.Width()
# This is heuristic -- look for len(someString) in rendering contexts
len_in_render=$(grep -rn 'len(' internal/tui/layout/ internal/tui/views/ 2>/dev/null \
  | grep -v '_test.go' \
  | grep -v '// ' \
  | grep -v 'len(m\.' \
  | grep -v 'len(items' \
  | grep -v 'len(rows' \
  | grep -v 'len(cols' \
  | grep -v 'len(lines' \
  | grep -v 'len(result' \
  | grep -v 'len(filtered' \
  | grep -v 'len(resources' \
  | grep -v 'len(fields' \
  | grep -v 'len(sections' \
  | grep -v 'len(content' \
  | grep -v 'len(help' \
  | grep -v 'len(path' \
  | grep -v 'len(parts' \
  | grep -v 'len(categories' \
  | grep -v 'len(matches' \
  | grep -v 'len(entries' \
  | grep -v 'len(args' \
  || true)
# This check is too noisy for automated pass/fail -- make it informational
if [ -n "$len_in_render" ]; then
  warn "len() calls in view/layout code (verify none are for string width -- use lipgloss.Width)"
  count=$(echo "$len_in_render" | wc -l | tr -d ' ')
  detail "$count occurrences found (manual review needed)"
fi

# ============================================================================
section "BUBBLE TEA v2 PATTERNS"
# ============================================================================

# Check for BT v1 Init signature: Init() (tea.Model, tea.Cmd)
btv1_init=$(grep -rn 'Init().*tea\.Model' internal/tui/ 2>/dev/null | grep -v '_test.go' || true)
if [ -z "$btv1_init" ]; then
  pass "No BT v1 Init() signatures found"
else
  fail "BT v1 Init() signature found (should be Init() tea.Cmd)"
  while IFS= read -r line; do detail "$line"; done <<< "$btv1_init"
fi

# ============================================================================
section "PROJECT STRUCTURE"
# ============================================================================

# All domain code under internal/
non_internal=$(find . -name "*.go" \
  -not -path "./internal/*" \
  -not -path "./cmd/*" \
  -not -path "./tests/*" \
  -not -path "./.specify/*" \
  -not -path "./.claude/*" \
  -not -path "./vendor/*" \
  -not -path "./website/*" \
  2>/dev/null || true)

if [ -z "$non_internal" ]; then
  pass "All domain code is under internal/, cmd/, or tests/"
else
  warn "Go files outside expected directories"
  while IFS= read -r line; do detail "$line"; done <<< "$non_internal"
fi

# Single go.mod at root (exclude worktrees, vendor, website)
gomod_count=$(find . -name "go.mod" \
  -not -path "./vendor/*" \
  -not -path "./website/*" \
  -not -path "./.claude/worktrees/*" \
  2>/dev/null | wc -l | tr -d ' ')
if [ "$gomod_count" = "1" ]; then
  pass "Single go.mod at project root"
else
  warn "Found $gomod_count go.mod files (expected 1)"
fi

# ============================================================================
section "STYLE CONSTANTS"
# ============================================================================

# Inline hex color strings outside styles/ (should use palette constants)
inline_hex=$(grep -rn '"#[0-9a-fA-F]\{6\}"' internal/tui/ 2>/dev/null \
  | grep -v 'styles/' \
  | grep -v '_test.go' \
  || true)

if [ -z "$inline_hex" ]; then
  pass "No inline hex color strings outside styles/"
else
  fail "Inline hex color strings found outside styles/"
  while IFS= read -r line; do detail "$line"; done <<< "$inline_hex"
fi

# ============================================================================
section "GOD STRUCT MONITORING"
# ============================================================================

# Count fields on the root Model struct
model_fields=$(awk '
  /^type Model struct/ { in_struct = 1; fields = 0; next }
  in_struct && /^}/ { print fields; in_struct = 0 }
  in_struct && /^\t[a-zA-Z]/ { fields++ }
' internal/tui/app.go 2>/dev/null || true)

if [ -n "$model_fields" ]; then
  if [ "$model_fields" -le 20 ]; then
    pass "Root Model has $model_fields fields (<=20)"
  elif [ "$model_fields" -le 30 ]; then
    warn "Root Model has $model_fields fields (20-30 range, monitor growth)"
  else
    fail "Root Model has $model_fields fields (>30, consider refactoring)"
  fi
fi

# ============================================================================
section "PACKAGE EXPORT COUNT (>15 symbols, excluding resource and messages)"
# ============================================================================

high_export_pkgs=""
for pkg_dir in internal/tui/keys internal/tui/layout internal/tui/styles internal/tui/views internal/config internal/fieldpath internal/buildinfo; do
  if [ -d "$pkg_dir" ]; then
    exports=$(grep -rh '^func [A-Z]\|^type [A-Z]\|^var [A-Z]\|^const [A-Z]' "$pkg_dir"/*.go 2>/dev/null \
      | grep -v '_test.go' \
      | wc -l | tr -d ' ')
    if [ "$exports" -gt 15 ]; then
      high_export_pkgs+="$pkg_dir: $exports exports"$'\n'
    fi
  fi
done

if [ -z "$high_export_pkgs" ]; then
  pass "No packages export >15 symbols (excluding resource, messages, aws)"
else
  warn "Packages with >15 exported symbols"
  while IFS= read -r line; do
    [ -n "$line" ] && detail "$line"
  done <<< "$high_export_pkgs"
fi

# ============================================================================
section "CIRCULAR IMPORT CHECK"
# ============================================================================

circular=$(go build ./... 2>&1 | grep -i 'import cycle' || true)
if [ -z "$circular" ]; then
  pass "No circular imports"
else
  fail "Circular imports detected"
  while IFS= read -r line; do detail "$line"; done <<< "$circular"
fi

# ============================================================================
section "NAKED RETURNS"
# ============================================================================

# Find functions with named return values that use bare return
# Awk approach: track functions with named returns, look for bare return inside them
naked_returns=$(find internal cmd -name "*.go" ! -name "*_test.go" -print0 2>/dev/null \
  | xargs -0 awk '
    /^func.*\).*\(.*\) \{/ {
      # Function with named return values (has parens around return types)
      in_named_func = 1
      fname = FILENAME ":" NR
    }
    /^func/ && !/\).*\(.*\) \{/ {
      in_named_func = 0
    }
    in_named_func && /^[[:space:]]*return$/ {
      print FILENAME ":" NR ": bare return in function with named returns"
    }
    /^\}/ && in_named_func { in_named_func = 0 }
  ' 2>/dev/null || true)

if [ -z "$naked_returns" ]; then
  pass "No naked returns in functions with named return values"
else
  warn "Potential naked returns (manual review needed)"
  count=$(echo "$naked_returns" | wc -l | tr -d ' ')
  detail "$count occurrences found"
  while IFS= read -r line; do detail "$line"; done <<< "$naked_returns"
fi

# ============================================================================
section "CHARM.LAND IMPORTS (not github.com/charmbracelet)"
# ============================================================================

# github.com/charmbracelet/x/ansi is a legitimate utility (not the old BT/lipgloss path)
old_charm=$(grep -rn 'github.com/charmbracelet/' internal/ cmd/ 2>/dev/null \
  | grep -v '_test.go' \
  | grep -v 'go.mod' \
  | grep -v 'go.sum' \
  | grep -v 'charmbracelet/x/' \
  || true)

if [ -z "$old_charm" ]; then
  pass "All Bubble Tea imports use charm.land/ (not github.com/charmbracelet/)"
else
  fail "Old github.com/charmbracelet/ imports found (should be charm.land/)"
  while IFS= read -r line; do detail "$line"; done <<< "$old_charm"
fi

# ============================================================================
section "RESOURCE TYPE CONSISTENCY"
# ============================================================================

# Count registered fetchers (resource.Register is called from internal/aws/*.go init() funcs)
reg_count=$(grep -rch 'resource\.Register(' internal/aws/*.go 2>/dev/null | awk '{s+=$1} END {print s+0}')
# Count default view definition map keys across all defaults files
def_count=$(grep -rch '^\t\t"[a-z]' internal/config/defaults_*.go 2>/dev/null | awk '{s+=$1} END {print s+0}')

detail "Registered fetchers (resource.Register calls in aws/): $reg_count"
detail "Default view definitions (map keys in defaults_*.go): $def_count"

if [ "$reg_count" -le "$def_count" ]; then
  pass "All $reg_count registered fetchers have default view definitions ($def_count defs, includes child views)"
else
  fail "Registered fetchers ($reg_count) exceed default view definitions ($def_count) -- missing view defs"
fi

# ============================================================================
# SUMMARY
# ============================================================================

REPORT+=$'\n'"========================================"$'\n'
REPORT+="SUMMARY: $PASS passed, $FAIL failed, $WARN warnings"$'\n'
REPORT+="========================================"$'\n'

if [ "$FAIL" -gt 0 ]; then
  REPORT+="RESULT: NEEDS ATTENTION"$'\n'
elif [ "$WARN" -gt 3 ]; then
  REPORT+="RESULT: MOSTLY CLEAN (review warnings)"$'\n'
else
  REPORT+="RESULT: CLEAN"$'\n'
fi

echo "$REPORT"

# Exit with non-zero if any failures
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0

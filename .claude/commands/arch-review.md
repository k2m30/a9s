---
description: Run the architecture review checklist — automated checks + agent judgment scoring
---

Run the full architecture review using the `a9s-arch-review` skill.

## Step 1: Run automated checks

Run the architecture review script and save output:

```bash
bash .claude/scripts/arch-review.sh > /tmp/arch-review.txt 2>&1
```

Read `/tmp/arch-review.txt` for the structured PASS/FAIL report.

## Step 2: Run toolchain checks

Run these three checks and save their output:

```bash
golangci-lint run ./... > /tmp/arch-lint.txt 2>&1
```

```bash
go test -race ./tests/unit/ -count=1 -timeout 120s > /tmp/arch-tests.txt 2>&1
```

```bash
govulncheck ./... > /tmp/arch-vulncheck.txt 2>&1
```

Read all three output files.

## Step 3: Interpret and score

Using the `a9s-arch-review` skill's judgment guidelines, produce the final scored report. Apply known acceptable exceptions before marking anything as a real issue.

Output the report in the format specified by the skill.

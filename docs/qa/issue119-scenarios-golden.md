# Issue #119 Scenario Goldens

This workflow auto-generates and verifies deterministic related-view snapshots for Issue #119 behavior (wide two-column and 80-99 stacked layouts).

Test harness: `tests/unit/issue119_scenarios_golden_test.go`.
Golden output directory: `tests/testdata/golden/issue119/`.

## Commands

Generate or refresh all golden snapshots:

```bash
UPDATE_GOLDEN=1 go test ./tests/unit -run TestGenerateIssue119Scenarios -v
```

Verify current app rendering against saved goldens:

```bash
go test ./tests/unit -run TestIssue119ScenarioGoldens -v
```

List available scenarios:

```bash
go test ./tests/unit -run TestIssue119ScenarioCatalog -v
```

## What gets generated

For each scenario, two files are produced:

- `<scenario>.golden.txt`: plain-text snapshot (ANSI stripped)
- `<scenario>.ansi.golden`: ANSI-preserving snapshot (styles/focus/highlight included)

## Scenario map

- `stacked_090_default`: stacked layout visible at width 90
- `stacked_090_toggle_hidden`: stacked related section hidden after `r`
- `stacked_090_toggle_restore`: stacked related section restored after second `r`
- `wide_120_two_column`: baseline two-column detail layout
- `wide_120_right_focus`: right column focused via `Tab`
- `wide_120_right_filter_cloud`: right column filtered by `/cloud`

## Notes

- Scenarios run with fixed demo fixtures and explicit EC2 related definitions for deterministic output.
- Use `UPDATE_GOLDEN=1` only for intentional UI changes.
- In CI, run verification command only.

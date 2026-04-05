# Issue #140 Scenario Goldens

This workflow auto-generates and verifies deterministic EC2 related-view snapshots for Issue #140 QA stories.

Source stories: `docs/qa/ec2-related-navigation-stories.md`.

Test harness: `tests/unit/issue140_scenarios_golden_test.go`.
Golden output directory: `tests/testdata/golden/issue140/`.

## Commands

Generate or refresh all golden snapshots:

```bash
UPDATE_GOLDEN=1 go test ./tests/unit -run TestGenerateIssue140Scenarios -v
```

Verify current app rendering against saved goldens:

```bash
go test ./tests/unit -run TestIssue140ScenarioGoldens -v
```

List available scenarios:

```bash
go test ./tests/unit -run TestIssue140ScenarioCatalog -v
```

## What gets generated

For each scenario, two files are produced:

- `<scenario>.golden.txt`: plain-text snapshot (ANSI stripped)
- `<scenario>.ansi.golden`: ANSI-preserving snapshot (styles/focus/highlight included)

## Scenario map

- `ec2_001_initial_detail`: EC2 initial detail render contract
- `ec2_017_vpcid_selected`: VpcId row selected (underline/selection interaction)
- `ec2_018_right_column_types`: right-column related type set render
- `ec2_020_counts_arrived`: right-column count updates
- `ec2_021_right_focus_after_tab`: focus state after Tab to right column
- `ec2_023_right_hidden_after_toggle`: render after `r` toggle hide
- `ec2_029_filtered_alarms_list`: filtered alarm list opened from related navigation
- `ec2_033_only_alarm_available_focus`: right-column focus when only alarm is actionable

## Notes

- Snapshots are deterministic: fixed demo fixtures, fixed terminal sizes, deterministic key/message sequences.
- Use `UPDATE_GOLDEN=1` only for intentional UI changes or design-approved updates.
- In CI, run verification command only.

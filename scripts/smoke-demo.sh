#!/bin/sh
# smoke-demo.sh — deterministic TUI smoke over the demo fixture store.
#
# Drives the real binary in a headless tmux session and asserts the
# user-visible contracts that unit suites cannot see rendered end to end:
# menu availability, humanized status texts (no raw AWS enums), the
# owner-worded security-group risk phrases, per-row issue causes, and the
# related panel of the reference bucket. Runs in ~45s; part of
# `make ready-to-push` (Stage 6). On-demand DETAIL enrichment payloads are
# covered separately by `scripts/smoke-enrichers-demo.sh`.
#
# Requires tmux. Fails fast with the offending capture on any miss.
set -u

. "$(dirname "$0")/smoke-lib.sh"

smoke_start smoke

open_and_capture sg sg.txt 3
open_and_capture ng ng.txt 3
open_and_capture lambda lambda.txt 3
open_and_capture s3 s3.txt 3

# Reference-bucket walk: filter narrows the list, describe opens the detail
# with the RELATED panel, YAML view renders, a circular related drill
# (bucket → trail → bucket) re-shows cached badges with the depth-badged
# header, and escape unwinds back to the list.
select_row a9s-demo-healthy
capture s3_filtered.txt
keys 'd'
wait_for s3_detail.txt "CloudTrail Trails (1)" "s3 detail related shows the trail witness"
keys 'y'
page_until s3_yaml.txt "BucketArn" "yaml view renders the resource document"
keys Escape
sleep 1
keys Tab
sleep 1
keys Enter
sleep 3
keys Tab
sleep 1
keys Enter
sleep 3
capture s3_circular.txt

# A flagged bucket's detail must open with the Attention block on top.
keys Escape
sleep 1
keys Escape
sleep 1
keys Escape
sleep 1
keys ':s3' Enter
sleep 2
select_row a9s-demo-nopab
keys 'd'
wait_for s3_attention.txt "Attention (" "flagged bucket detail opens with the attention block"

# The node-group health issue is a supporting row, not a state, so the detail
# body is the only place it can be read.
keys Escape
sleep 1
keys ':ng' Enter
sleep 2
select_row staging-degraded-net-pool
# The Attention block opens on top of a flagged detail by contract, so its
# first screen is asserted as captured; the rows under it are paged for.
tmux send-keys -t "$SESSION" 'd'
sleep 3
tmux capture-pane -t "$SESSION" -p > "$CAPDIR/ng_detail.txt"
expect ng_detail.txt "health issue" "ng health issue is readable in the detail body"
page_until ng_detail.txt "~ Health issue" "ng detail carries the health issue as its own row"
page_until ng_detail.txt "Issue: insufficient free addresses" "ng detail carries the reported code as a row under the finding"

# Menu: full catalog present, the synthetic Cost Explorer entry included.
expect menu.txt "resource-types($(demo_count resource-types))" "menu shows the full catalog"

# Security groups: owner-worded risk phrases, no raw classifier tokens.
expect sg.txt "all ports open" "sg wide-open row uses the owner wording"
expect sg.txt "port 22 open" "sg dangerous-ports row uses the owner wording"
forbid sg.txt "WIDE_OPEN" "sg shows no raw WIDE_OPEN token"
forbid sg.txt "PORTS:" "sg shows no raw PORTS: token"

# Node groups: humanized states and issue causes.
expect ng.txt "create failed" "ng failed pool status is humanized"
forbid ng.txt "CREATE_FAILED" "ng shows no raw CREATE_FAILED enum"
forbid ng.txt "ACTIVE" "ng healthy status is lowercased"
expect ng.txt "health issue" "ng pool with a health issue says so"
expect ng.txt "degraded" "ng degraded pool says so"
expect ng.txt "details denied" "ng keeps the node group whose describe was denied"
expect ng.txt "details unavailable" "ng keeps the node group whose describe found nothing"

# Lambda: the issue-colored rows explain themselves in the State column.
expect lambda.txt "lambda($(demo_count lambda)) !" "lambda title carries the issue count"
expect lambda.txt "runtime is end-of-life" "lambda deprecated runtime explains itself"

# S3: reference type baseline — count, issues, the PAB cause visible.
expect s3.txt "s3($(demo_count s3))" "s3 list shows the fixture count"
expect s3.txt "public access block incomplete" "s3 flagged bucket names its cause"

# Reference bucket walk.
expect s3_filtered.txt "s3(1/$(demo_count s3))" "filter narrows the list to one row"
expect s3_detail.txt "RELATED" "s3 detail renders the related panel"
expect s3_detail.txt "KMS Key (1)" "s3 detail related shows the kms witness"

# Circular drill (bucket → trail → bucket): cached badges reappear at once
# and the header keeps the version next to the depth badge.
expect s3_circular.txt "detail -- a9s-demo-healthy" "circular drill lands back on the bucket detail"
expect s3_circular.txt "CloudTrail Trails (1)" "circular re-entry re-shows cached related counts"
expect s3_circular.txt "a9s v" "header keeps the version at depth"
expect_re s3_circular.txt '\[[0-9]+\]' "header shows the depth badge"

# A flagged resource explains itself at the top of its detail.
expect_re s3_attention.txt '[Pp]ublic access block incomplete' "attention names the cause"

smoke_finish

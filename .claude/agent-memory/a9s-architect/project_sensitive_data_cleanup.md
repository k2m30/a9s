---
name: sensitive-data-cleanup
description: Real dev-account AWS data (account 999000111222, VPC IDs, SG IDs, etc.) was hardcoded in committed Go files — cleanup plan written 2026-03-18
type: project
---

Real AWS infrastructure data from dev-account (account 999000111222, eu-west-2) was found hardcoded in committed Go fixture and test files. A comprehensive cleanup plan was written to `specs/005-add-vpc-nodegroups-sg/sensitive-data-cleanup.md`.

**Why:** The repo is intended to be public. Real VPC IDs, SG IDs, account IDs, subnet IDs, ARNs, IP ranges, cluster names, and org-identifying resource names were embedded directly in Go source files that would be committed, bypassing the existing .gitignored JSON fixture pattern.

**How to apply:**
- Any new Go fixture code must use sanitized dummy values (123456789012 for account, vpc-0aaa1111bbb2222cc pattern for VPCs, etc.)
- The a9s-fixtures agent instructions must include post-fetch sanitization
- During code review, reject any PR containing real AWS infrastructure identifiers
- The `.gitignored` JSON fixtures can keep real data for local debugging
- Sentinel check: grep for `999000111222` as a smoke test for leaks

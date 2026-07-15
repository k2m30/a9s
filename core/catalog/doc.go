// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

//go:generate go run ../../cmd/catalogen

// Package catalog is the declarative registry of all a9s resource types.
// It is the single source of truth for resource metadata (identity, columns,
// fetchers, enrichers, related-panel definitions, finding codes, etc.).
//
// Boundary rule: core/catalog imports ONLY core/domain — never
// core/resource, core/aws, or internal/tui.
//
// cmd/catalogen reads the installed catalog and emits markdown documentation
// only (no generated Go code). Run "make generate" to regenerate.
package catalog

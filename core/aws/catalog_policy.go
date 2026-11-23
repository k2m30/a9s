// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"slices"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

// policyFindingDefs are the FindingDefs a type's policy declaration raises,
// in the one wording every policy finding shares: the document named by its
// noun, and a detail sentence that states the grant without a consequence
// the verdict did not check.
func policyFindingDefs(d catalog.PolicyDecl) []catalog.FindingDef {
	var defs []catalog.FindingDef
	if d.Public != "" {
		defs = append(defs, catalog.FindingDef{
			Code: d.Public, Phrase: d.Noun + " policy open to anyone", Severity: d.PublicSeverity, Source: d.Source, NotKinds: d.NotKinds,
			Detail: "The " + d.Noun + " policy allows any principal, so anyone can use this " + d.Subject +
				", depending on the actions allowed. Replace the wildcard principal with the accounts or roles that need it, " +
				"or add a condition that requires the caller's account or ARN to equal one you expect; " +
				"a condition that only says whether a key is set scopes nothing.",
		})
	}
	if d.Cross != "" {
		defs = append(defs, catalog.FindingDef{
			Code: d.Cross, Phrase: d.Noun + " policy grants another account", Severity: domain.SevWarn, Source: d.Source, NotKinds: d.NotKinds,
			Detail: "The " + d.Noun + " policy admits a caller from another AWS account, so that account can use this " + d.Subject +
				", depending on the actions allowed. Confirm each account belongs to a partner you meant to share with, " +
				"and remove the rest from the " + d.Noun + " policy.",
		})
	}
	if d.Unread != "" {
		defs = append(defs, catalog.FindingDef{
			Code: d.Unread, Phrase: d.Noun + " policy not inspected", Severity: domain.SevWarn, Source: d.Source,
			Detail: "The " + d.Noun + " policy document AWS returned does not parse, so a9s cannot say whether it opens this " + d.Subject +
				" to anyone. Read the document in the detail view and judge its grants by hand.",
		})
	}
	return defs
}

func withPolicyFindings(types []catalog.ResourceTypeDef) []catalog.ResourceTypeDef {
	for i := range types {
		if p := types[i].Policy; p != nil {
			types[i].Findings = slices.Concat(types[i].Findings, policyFindingDefs(*p))
		}
	}
	return types
}

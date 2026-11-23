package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
)

// A policy that grants everyone and names another account is reported once,
// as the grant to everyone: the share is part of what "anyone" already says.
func TestT3BDev_PublicGrantBesideAForeignAccountRaisesOnlyThePublicFinding(t *testing.T) {
	for _, ty := range t3bCrossTypes() {
		t.Run(ty.short, func(t *testing.T) {
			doc := t3bDoc(
				ty.stmt("Allow", "*", "Action", ty.actions, nil),
				ty.stmt("Allow", map[string]any{"AWS": "arn:aws:iam::" + t3bForeign + ":root"}, "Action", ty.actions, nil),
			)
			out := ty.run(t, doc, t3bOwn, t3bOwn)
			if _, ok := t3bFind(out.findings, ty.public); !ok {
				t.Errorf("no %s; got %v", ty.public, t3bCodes(out.findings))
			}
			if _, ok := t3bFind(out.findings, ty.cross); ok {
				t.Errorf("%s raised beside %s", ty.cross, ty.public)
			}
		})
	}
}

// A document with no statements grants nobody: no finding, no mark, and a
// Wave 1 row that does not read "not inspected".
func TestT3BDev_PolicyWithNoStatementRaisesNothing(t *testing.T) {
	for _, ty := range t3bTypes {
		t.Run(ty.short, func(t *testing.T) {
			out := ty.run(t, t3bDoc(), t3bOwn, t3bOwn)
			for _, code := range []domain.FindingCode{ty.public, ty.cross, t3bDevDecl(t, ty.short).Unread} {
				if _, ok := t3bFind(out.findings, code); code != "" && ok {
					t.Errorf("%s raised from a policy with no statement", code)
				}
			}
			if out.marked {
				t.Errorf("row marked %q for a policy that parsed", out.mark)
			}
		})
	}
}

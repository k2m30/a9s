package unit

// partial_round4_attack_test.go — attacks on row 10's predicate from the
// directions its own table does not cover: the shapes the flat list DOES
// represent must keep deciding, and the fold the related panel reads must
// survive a selection the coverage join abstains on.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// partialR4Selection builds one selection carrying both tag shapes, so a case
// can hold ListOfTags and Conditions at once the way a plan edited twice does.
func partialR4Selection(listOfTags []backuptypes.Condition, conditions *backuptypes.Conditions) backuptypes.BackupSelection {
	sel := partialBackupSelection(nil, conditions)
	sel.ListOfTags = listOfTags
	return sel
}

// partialR4Tag is one ListOfTags entry. ListOfTags carries the older Condition
// shape, which supports StringEquals alone.
func partialR4Tag(key, value string) backuptypes.Condition {
	return backuptypes.Condition{
		ConditionType:  backuptypes.ConditionTypeStringequals,
		ConditionKey:   aws.String("aws:ResourceTag/" + key),
		ConditionValue: aws.String(value),
	}
}

// TestPartialR4Backup_WhatTheFlatListDoesRepresentStillDecides is the other
// half of row 10. The abstention buys correctness by giving up an answer, so
// it has to stay narrow: ListOfTags is an OR of its entries and the flat list
// is an OR, so any number of them folds exactly and must keep deciding. Only
// the Conditions block — an AND — can force the abstention.
func TestPartialR4Backup_WhatTheFlatListDoesRepresentStillDecides(t *testing.T) {
	for _, tc := range []struct {
		name        string
		listOfTags  []backuptypes.Condition
		conditions  *backuptypes.Conditions
		tags        map[string]string
		wantPartial bool
		wantWarning bool
	}{
		{
			// Two ListOfTags entries are two chances to match, which is what
			// the flat list already means. A resource matching either is
			// covered and the plan is fully known.
			name: "two ListOfTags entries cover either one",
			listOfTags: []backuptypes.Condition{
				partialR4Tag("backup", "nightly"),
				partialR4Tag("tier", "critical"),
			},
			tags:        map[string]string{"tier": "critical"},
			wantPartial: false,
			wantWarning: false,
		},
		{
			name: "two ListOfTags entries still report what neither names",
			listOfTags: []backuptypes.Condition{
				partialR4Tag("backup", "nightly"),
				partialR4Tag("tier", "critical"),
			},
			tags:        map[string]string{"environment": "production"},
			wantPartial: false,
			wantWarning: true,
		},
		{
			// An empty block asks for nothing, so it is representable and the
			// plan decides on its other selections — here, none.
			name:        "an empty Conditions block is representable",
			conditions:  &backuptypes.Conditions{},
			tags:        map[string]string{"environment": "production"},
			wantPartial: false,
			wantWarning: true,
		},
		{
			// The two shapes on one selection: the OR half folds exactly, and
			// the AND half is what forces the abstention.
			name: "an unrepresentable block taints a selection that also uses ListOfTags",
			listOfTags: []backuptypes.Condition{
				partialR4Tag("backup", "nightly"),
			},
			conditions: &backuptypes.Conditions{
				StringEquals: []backuptypes.ConditionParameter{
					partialRow10Param("tier", "critical"),
					partialRow10Param("environment", "production"),
				},
			},
			tags:        map[string]string{"owner": "platform"},
			wantPartial: true,
			wantWarning: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, plans := partialSelectionPlan(t, partialR4Selection(tc.listOfTags, tc.conditions))

			if gotPartial := plan.Fields["selections_partial"] != ""; gotPartial != tc.wantPartial {
				t.Errorf("plan row selections_partial = %q, want partial = %v; selection_tags folded to %q",
					plan.Fields["selections_partial"], tc.wantPartial, plan.Fields["selection_tags"])
			}

			res := w7EnrichEBS(t, []resource.Resource{partialRow10Volume(tc.tags)}, plans)
			if tc.wantWarning {
				w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan,
					"not covered by a backup plan", domain.SevWarn, "wave2:ebs")
				return
			}
			w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
		})
	}
}

// TestPartialR4Backup_AbstainingStillFeedsTheRelatedPivot pins the half of
// row 10 that is not the coverage join. selection_tags is read twice: by the
// join, which now abstains on an unrepresentable block, and by the ebs and
// ec2 related panels, which list the plans a resource's tags match. The
// abstention is about not asserting "not covered"; it is not a reason to stop
// showing an operator the plan their volume is tagged for, so the positive
// parameters keep folding and the pivot keeps its match.
func TestPartialR4Backup_AbstainingStillFeedsTheRelatedPivot(t *testing.T) {
	sel := partialBackupSelection(nil, &backuptypes.Conditions{
		StringEquals: []backuptypes.ConditionParameter{
			partialRow10Param("backup", "nightly"),
			partialRow10Param("tier", "critical"),
		},
	})
	plan, plans := partialSelectionPlan(t, sel)

	if plan.Fields["selections_partial"] == "" {
		t.Fatal("precondition failed: the plan is not marked partial, so this proves nothing about an abstaining plan")
	}

	volume := partialRow10Volume(map[string]string{"backup": "nightly"})
	got := partialR4EBSBackupPivot(t, volume, plans)
	if len(got) != 1 || got[0] != plan.ID {
		t.Errorf("the ebs→backup panel lists %v, want [%s]: the volume carries the tag the plan selects on",
			got, plan.ID)
	}
}

// partialR4EBSBackupPivot runs the registered ebs→backup related checker, the
// one the detail panel calls.
func partialR4EBSBackupPivot(t *testing.T, volume resource.Resource, cache resource.ResourceCache) []string {
	t.Helper()
	for _, def := range resource.GetRelated("ebs") {
		if def.TargetType != "backup" {
			continue
		}
		if def.Checker == nil {
			t.Fatal("the ebs→backup related checker is registered but nil")
		}
		return def.Checker(context.Background(), &awsclient.ServiceClients{}, volume, cache).ResourceIDs()
	}
	t.Fatal("no ebs→backup related checker in the registry")
	return nil
}

// TestPartialR4Backup_AParameterAWSDidNotFillIsNotAnEmptySelection attacks the
// one shape the predicate counts but the fold drops. A ConditionParameter is
// counted whether or not its key and value arrived, so a block holding one
// half-filled parameter is called representable while folding to nothing —
// and a plan that folded to nothing is a plan that covers nothing, which is
// the confident wrong answer row 10 exists to remove.
func TestPartialR4Backup_AParameterAWSDidNotFillIsNotAnEmptySelection(t *testing.T) {
	plan, plans := partialSelectionPlan(t, partialBackupSelection(nil, &backuptypes.Conditions{
		StringEquals: []backuptypes.ConditionParameter{{
			ConditionKey: aws.String("aws:ResourceTag/backup"),
		}},
	}))

	if plan.Fields["selections_partial"] == "" {
		t.Errorf("plan row selections_partial = %q, want it marked partial; the selection named a condition a9s could not read, and selection_tags folded to %q",
			plan.Fields["selections_partial"], plan.Fields["selection_tags"])
	}

	res := w7EnrichEBS(t, []resource.Resource{partialRow10Volume(map[string]string{"backup": "nightly"})}, plans)
	w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
}

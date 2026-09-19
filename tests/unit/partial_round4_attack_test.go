package unit

// The Conditions-block abstention predicate:
// the shapes the flat list DOES represent keep deciding, and the fold the
// related panel reads survives a selection the coverage join abstains on.

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

// TestPartialR4Backup_WhatTheFlatListDoesRepresentStillDecides pins that
// ListOfTags is an OR of its entries and keeps deciding, that a clause AWS
// returned half-filled cannot be evaluated and abstains, and that a Conditions
// block is evaluated whole as the AND it is.
func TestPartialR4Backup_WhatTheFlatListDoesRepresentStillDecides(t *testing.T) {
	for _, tc := range []struct {
		name        string
		listOfTags  []backuptypes.Condition
		conditions  *backuptypes.Conditions
		tags        map[string]string
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
			wantWarning: false,
		},
		{
			name: "two ListOfTags entries still report what neither names",
			listOfTags: []backuptypes.Condition{
				partialR4Tag("backup", "nightly"),
				partialR4Tag("tier", "critical"),
			},
			tags:        map[string]string{"environment": "production"},
			wantWarning: true,
		},
		{
			// The same half-filled condition in the older ListOfTags shape,
			// where no Conditions block exists to reach the predicate at all.
			// A dropped entry narrows the OR, so the plan reads as protecting
			// less than it does — the direction that invents a finding.
			name: "a ListOfTags entry AWS did not fill",
			listOfTags: []backuptypes.Condition{
				{ConditionType: backuptypes.ConditionTypeStringequals, ConditionKey: aws.String("aws:ResourceTag/backup")},
			},
			tags:        map[string]string{"backup": "nightly"},
			wantWarning: false,
		},
		{
			// An empty block asks for nothing, and a selection with no
			// Resources and no ListOfTags takes every resource.
			name:       "an empty Conditions block is representable",
			conditions: &backuptypes.Conditions{},
			tags:       map[string]string{"environment": "production"},
		},
		{
			// The two shapes on one selection: the volume matches neither the
			// ListOfTags entry nor the AND.
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
			wantWarning: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, plans := partialSelectionPlan(t, partialR4Selection(tc.listOfTags, tc.conditions))
			if _, complete := awsclient.BackupPlanSelections(plan); !complete {
				t.Error("plan row marked incomplete, want complete: its one selection was read")
			}

			res := w7EnrichEBS(t, []resource.Resource{partialRow10Volume(tc.tags)}, plans)
			if tc.wantWarning {
				w4AssertFinding(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan,
					"not covered by a backup plan", domain.SevWarn, "wave2")
				return
			}
			w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
		})
	}
}

// TestPartialR4Backup_AbstainingStillFeedsTheRelatedPivot pins that the ebs
// related panel applies a Conditions block the way the coverage join does: the
// block is an AND, so the plan is listed only when the volume carries both tags.
func TestPartialR4Backup_AbstainingStillFeedsTheRelatedPivot(t *testing.T) {
	sel := partialBackupSelection(nil, &backuptypes.Conditions{
		StringEquals: []backuptypes.ConditionParameter{
			partialRow10Param("backup", "nightly"),
			partialRow10Param("tier", "critical"),
		},
	})
	plan, plans := partialSelectionPlan(t, sel)

	if got := partialR4EBSBackupPivot(t, partialRow10Volume(map[string]string{"backup": "nightly"}), plans); len(got) != 0 {
		t.Errorf("the ebs→backup panel lists %v, want none: the volume lacks tier=critical", got)
	}
	both := partialRow10Volume(map[string]string{"backup": "nightly", "tier": "critical"})
	if got := partialR4EBSBackupPivot(t, both, plans); len(got) != 1 || got[0] != plan.ID {
		t.Errorf("the ebs→backup panel lists %v, want [%s]: the volume carries both tags the plan selects on",
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
		return def.Checker(context.Background(), bk551Clients(), volume, cache).ResourceIDs()
	}
	t.Fatal("no ebs→backup related checker in the registry")
	return nil
}

// TestPartialR4Backup_AParameterAWSDidNotFillIsNotAnEmptySelection pins the
// one shape the predicate counts but the fold drops. A ConditionParameter is
// counted whether or not its key and value arrived, so a block holding one
// half-filled parameter is called representable while folding to nothing —
// and a plan that folded to nothing is a plan that covers nothing, a
// confident wrong answer.
func TestPartialR4Backup_AParameterAWSDidNotFillIsNotAnEmptySelection(t *testing.T) {
	plan, plans := partialSelectionPlan(t, partialBackupSelection(nil, &backuptypes.Conditions{
		StringEquals: []backuptypes.ConditionParameter{{
			ConditionKey: aws.String("aws:ResourceTag/backup"),
		}},
	}))

	// The evaluator abstains on a clause with no value; the plan row stays complete.
	if _, complete := awsclient.BackupPlanSelections(plan); !complete {
		t.Error("plan row marked incomplete, want complete: its one selection was read")
	}

	res := w7EnrichEBS(t, []resource.Resource{partialRow10Volume(map[string]string{"backup": "nightly"})}, plans)
	w4AssertNoCode(t, res.Findings[w7VolumeID], awsclient.CodeEBSNotInBackupPlan)
}

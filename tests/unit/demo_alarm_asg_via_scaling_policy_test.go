// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// An alarm reaches an Auto Scaling group two ways: by an AutoScalingGroupName
// dimension, or by a scaling-policy action whose ARN carries the group after
// "autoScalingGroupName/". Scaling on any metric other than the group's own —
// queue depth, request count — takes the second, and a demo that only shows
// the first leaves the action path unexercised.
func TestDemoAlarmNamesASGThroughAScalingPolicyAction(t *testing.T) {
	const asg = "acme-web-prod-asg"
	const viaAction = "acme-web-prod-asg-scale-out-on-backlog"

	clients := demo.NewServiceClients()
	td := resource.FindResourceType("alarm")
	if td == nil {
		t.Fatal("no alarm resource type")
	}

	var def *resource.RelatedDef
	defs := resource.GetRelated("alarm")
	for i := range defs {
		if defs[i].TargetType == "asg" {
			def = &defs[i]
			break
		}
	}
	if def == nil || def.Checker == nil {
		t.Fatal("alarm declares no asg pivot with a checker")
	}

	rows, ok := DrainFixtures(t, *td, clients)
	if !ok {
		t.Fatal("alarm declares no Fetcher")
	}

	var matched resource.Resource
	for _, row := range rows {
		if row.ID == viaAction {
			matched = row
		}
	}
	if matched.ID == "" {
		t.Fatalf("%s is not in the demo alarm list", viaAction)
	}

	raw, ok := matched.RawStruct.(cwtypes.MetricAlarm)
	if !ok {
		t.Fatalf("%s carries no MetricAlarm RawStruct", viaAction)
	}
	for _, d := range raw.Dimensions {
		if aws.ToString(d.Name) == "AutoScalingGroupName" {
			t.Fatalf("%s carries an AutoScalingGroupName dimension; the dimension path would answer "+
				"whether or not the action one does", viaAction)
		}
	}

	r := def.Checker(context.Background(), clients, matched, resource.ResourceCache{})
	if r.EffectiveState() != domain.RelatedResolved || !slices.Contains(r.ResourceIDs(), asg) {
		t.Errorf("%s -> asg resolved %v with ids %v, want %s — the scaling policy ARN in its "+
			"AlarmActions is the only place the group is named",
			viaAction, r.EffectiveState(), r.ResourceIDs(), asg)
	}
}

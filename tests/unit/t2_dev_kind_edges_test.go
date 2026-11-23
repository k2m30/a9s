package unit_test

import (
	"context"
	"maps"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// A row the disk cache replays carries Fields and no SDK struct, and its kind
// is the value the fetcher stored; a kind AWS adds later is excluded by an
// OnlyKinds declaration and included by a NotKinds one.
func TestT2Dev_KindFromStoredFieldsAndUnknownValues(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]string
		code   domain.FindingCode
		want   bool
	}{
		{"logs, no class stored", map[string]string{"log_group_class": ""}, t2LogsNoMetricFilters, true},
		{"logs, standard", map[string]string{"log_group_class": "STANDARD"}, t2LogsNoMetricFilters, true},
		{"logs, infrequent access", map[string]string{"log_group_class": "INFREQUENT_ACCESS"}, t2LogsNoMetricFilters, false},
		{"logs, a class AWS adds later", map[string]string{"log_group_class": "ARCHIVE"}, t2LogsNoMetricFilters, false},
		{"logs, delivery, retention", map[string]string{"log_group_class": "DELIVERY"}, "logs.retention-never-expire", false},
		{"logs, standard, retention", map[string]string{"log_group_class": "STANDARD"}, "logs.retention-never-expire", true},
		{"glue, ray", map[string]string{"command": "glueray"}, t2GlueNoSecConfig, false},
		{"glue, python shell", map[string]string{"command": "pythonshell"}, t2GlueNoSecConfig, true},
		{"glue, a kind AWS adds later, NotKinds", map[string]string{"command": "gluefuture"}, t2GlueNoSecConfig, true},
		{"glue, a kind AWS adds later, OnlyKinds", map[string]string{"command": "gluefuture"}, t2GlueLoggingOff, false},
		{"ses, domain word", map[string]string{"identity_type": "domain"}, "ses.dkim-off", true},
		{"ses, email address word", map[string]string{"identity_type": "email address"}, "ses.dkim-off", false},
		{"ssm, secure string", map[string]string{"type": "SecureString"}, t2SSMStale, true},
		{"ssm, string", map[string]string{"type": "String"}, t2SSMStale, false},
		{"role, service-linked path", map[string]string{"path": "/aws-service-role/ecs.amazonaws.com/"}, "iam-role.dormant", false},
		{"role, root path", map[string]string{"path": "/"}, "iam-role.dormant", true},
		{"dbi-snap, manual", map[string]string{"snapshot_type": "manual"}, "dbi-snap.past-retention", false},
		{"dbi-snap, automated", map[string]string{"snapshot_type": "automated"}, "dbi-snap.past-retention", true},
	}
	for _, tc := range cases {
		short, _, _ := strings.Cut(tc.name, ",")
		if got := catalog.AppliesToRow(short, tc.code, domain.Resource{ID: "row", Fields: tc.fields}); got != tc.want {
			t.Errorf("%s: AppliesToRow(%s) = %v, want %v", tc.name, tc.code, got, tc.want)
		}
	}
}

// AWS fixes a Delivery log group's retention, so the demo Delivery group,
// which reports no retentionInDays, carries no never-expire finding.
func TestT2Dev_DeliveryLogGroupCarriesNoRetentionFinding(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	for _, r := range byType["logs"] {
		if r.ID != demofixtures.LogGroupDelivery {
			continue
		}
		if t1aHas(r.Findings, "logs.retention-never-expire") {
			t.Errorf("%s: retention-never-expire stands; findings %v", r.ID, t2Codes(r.Findings))
		}
		return
	}
	t.Fatal("the demo Delivery log group is not on the bench")
}

// The state finding a row's lifecycle value raises stands whatever its class,
// and the posture beside it follows the class: a key pending deletion shows
// that and nothing about its rotation.
func TestT2Dev_StateFindingStandsAndPostureFollowsTheClass(t *testing.T) {
	row := domain.Resource{ID: "key", Fields: map[string]string{"status": "PendingDeletion"}}
	if !catalog.AppliesToRow("kms", awsclient.CodeKMSStatePendingDeletion, row) {
		t.Error("a key pending deletion: its state finding is withheld")
	}
	if catalog.AppliesToRow("kms", "kms.rotation-disabled", row) {
		t.Error("a key pending deletion: rotation-disabled stands beside its state finding")
	}
}

// The Wave 2 door holds for an enricher that reads every kind: a Network Load
// Balancer whose attributes report the HTTP desync and header settings an
// Application Load Balancer is judged on carries neither finding, and the
// Application Load Balancer that reports them carries both.
func TestT2Dev_Wave2DoorWithholdsAFindingForAnExcludedKind(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	var alb, nlb resource.Resource
	for _, r := range byType["elb"] {
		switch r.ID {
		case demofixtures.ELBDesyncMonitor:
			alb = r
		case demofixtures.ELBKeepsInvalidHeaders:
			nlb = r
		}
	}
	if alb.ID == "" || nlb.ID == "" {
		t.Fatal("the demo elb witnesses are not on the bench")
	}
	lb, ok := nlb.RawStruct.(elbv2types.LoadBalancer)
	if !ok {
		t.Fatalf("%s RawStruct is %T", nlb.ID, nlb.RawStruct)
	}
	lb.Type = elbv2types.LoadBalancerTypeEnumNetwork
	nlb.RawStruct = lb
	nlb.Fields = maps.Clone(nlb.Fields)
	nlb.Fields["type"] = string(elbv2types.LoadBalancerTypeEnumNetwork)

	res := unit.Wave2Result(t, "elb", demo.NewServiceClients(), []resource.Resource{alb, nlb})
	if !t1aHas(res.Findings[alb.ID], "elb.desync-mitigation-off") {
		t.Errorf("%s (application): desync-mitigation-off missing; findings %v", alb.ID, t2Codes(res.Findings[alb.ID]))
	}
	if t1aHas(res.Findings[nlb.ID], "elb.invalid-headers-kept") {
		t.Errorf("%s (network): invalid-headers-kept stands; findings %v", nlb.ID, t2Codes(res.Findings[nlb.ID]))
	}
}

// A finding the emit door withholds takes its supporting rows with it, so the
// detail of a cluster member names no single-AZ row.
func TestT2Dev_WithheldFindingLeavesNoSupportingRows(t *testing.T) {
	db := t2DBInstance("acme-orders-maz-instance-2", "mysql", "8.0.39", "acme-orders-maz")
	db.MultiAZ = aws.Bool(false)
	res, err := awsclient.FetchRDSInstancesPage(context.Background(), &t2RDSFake{instances: []rdstypes.DBInstance{db}}, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage: %v", err)
	}
	r := res.Resources[0]
	if _, ok := r.AttentionDetails[t2DBISingleAZ]; ok {
		t.Errorf("%s: withheld %s still carries supporting rows %v", r.ID, t2DBISingleAZ, r.AttentionDetails[t2DBISingleAZ])
	}
}

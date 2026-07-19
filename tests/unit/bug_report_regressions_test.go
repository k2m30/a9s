package unit_test

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// defaultEC2RelatedDefs returns a snapshot of the production ec2 RelatedDefs
// at call time. Implemented as a function (not a package-level var) so the
// catalog.Find lookup it triggers happens after TestMain has called
// aws.Install — package-level var initializers run before TestMain.
func defaultEC2RelatedDefs() []resource.RelatedDef {
	return append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
}

// TestBug_AllZeroRelatedRows_DoNotAllowRightColumnFocus and
// TestBug_FirstToggleRelated_HidesAutoShownColumn (dead views.DetailModel
// Update/View) removed — 022-codebase-cleanup wave 3, DetailModel cluster.
// No port needed: right-column focus/toggle mechanics are covered on the
// live controller path by app_related_cursor_skip_test.go and
// app_related_focus_entry_test.go (cursor-skip/Tab-focus-entry), and the
// actionability decision itself by TestIsRelatedActionable_Table
// (rightcolumn_actionable_test.go) against resource.IsRelatedActionable —
// isActionableRow in rightcolumn.go is a thin wrapper over that same table.
//
// TestBug_RightColumnFilter_SlashFiltersAndEscapeClears WAS also dropped in
// that same cleanup with the same "covered elsewhere" claim, but nothing
// else in the suite actually drives ActionSetFilter against a detail
// screen's related panel end to end — app_related_cursor_skip_test.go only
// covers dimmed-row cursor-skip, not filtering. Ported below directly
// against the live Controller + RenderDetail seam (rightcolumn_test.go's
// helpers, same package).

// TestBug_RightColumnFilter_SlashFiltersAndEscapeClears verifies that
// ActionSetFilter narrows the related panel to matching DisplayNames (the
// production effect of the '/' key while the panel is focused — see
// core/app/detail_cursor.go's ActionSetFilter case), and that clearing the
// filter (Arg="", the effect of Escape via rs.rightCol.Update in
// internal/tui/app_stack.go) restores every row.
func TestBug_RightColumnFilter_SlashFiltersAndEscapeClears(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: noopChecker},
	})

	c := newRightColController(t, "ec2")
	showRightColPanel(t, c, 140, 30)

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "target"})
	filtered := renderRightCol(t, c, 140, 30)
	if !strings.Contains(filtered, "Target Groups") {
		t.Errorf("filter %q should keep matching row \"Target Groups\"; got:\n%s", "target", filtered)
	}
	if strings.Contains(filtered, "Auto Scaling Groups") || strings.Contains(filtered, "Security Groups") {
		t.Errorf("filter %q should hide non-matching rows; got:\n%s", "target", filtered)
	}

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: ""})
	cleared := renderRightCol(t, c, 140, 30)
	for _, want := range []string{"Target Groups", "Auto Scaling Groups", "Security Groups"} {
		if !strings.Contains(cleared, want) {
			t.Errorf("clearing the filter (Escape) should restore %q; got:\n%s", want, cleared)
		}
	}
}

// TestBug_EC2DefaultDetail_ShowsAttachedEBSVolumeIDs is the live-seam
// replacement for the dead-View()-driven original: verifies the live
// RenderDetail path extracts a nested BlockDeviceMappings[].Ebs.VolumeId
// from RawStruct via the ec2 view config.
func TestBug_EC2DefaultDetail_ShowsAttachedEBSVolumeIDs(t *testing.T) {
	ensureNoColor(t)

	inst := ec2types.Instance{
		InstanceId: aws.String("i-0abc123456def7890"),
		BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &ec2types.EbsInstanceBlockDevice{
					VolumeId: aws.String("vol-0abc123456def7890"),
				},
			},
		},
	}

	res := buildResource("i-0abc123456def7890", "ec2-with-volume", inst)
	plain := stripAnsi(wave3RenderDetailFor(t, res, "ec2", 120, 40))
	if !strings.Contains(plain, "vol-0abc123456def7890") {
		t.Fatalf("default EC2 detail should show attached EBS volume IDs, got:\n%s", plain)
	}
}

func TestBug_EC2DefaultRelatedDefinitions_IncludeEBSVolumes(t *testing.T) {
	for _, def := range defaultEC2RelatedDefs() {
		if def.TargetType == "ebs" {
			return
		}
	}
	t.Fatal("EC2 related definitions should include EBS volumes")
}

// TestBug_AMIDetail_ShowsUsefulImageMetadata is the live-seam replacement for
// the dead-View()-driven original: verifies the live RenderDetail path
// extracts AMI's uncommon nested fields (Hypervisor, SriovNetSupport,
// ImageOwnerAlias, BlockDeviceMappings[].Ebs.SnapshotId) via the ami view
// config.
func TestBug_AMIDetail_ShowsUsefulImageMetadata(t *testing.T) {
	ensureNoColor(t)

	img := ec2types.Image{
		ImageId:            aws.String("ami-0aaa111111111111a"),
		Name:               aws.String("ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-20250712"),
		Description:        aws.String("Canonical, Ubuntu, 22.04, amd64 jammy image"),
		State:              ec2types.ImageStateAvailable,
		Architecture:       ec2types.ArchitectureValuesX8664,
		PlatformDetails:    aws.String("Linux/UNIX"),
		UsageOperation:     aws.String("RunInstances"),
		Hypervisor:         ec2types.HypervisorTypeXen,
		ImageOwnerAlias:    aws.String("amazon"),
		RootDeviceName:     aws.String("/dev/sda1"),
		RootDeviceType:     ec2types.DeviceTypeEbs,
		SriovNetSupport:    aws.String("simple"),
		VirtualizationType: ec2types.VirtualizationTypeHvm,
		BootMode:           ec2types.BootModeValuesUefiPreferred,
		ImageLocation:      aws.String("amazon/ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-20250712"),
		OwnerId:            aws.String("099720109477"),
		CreationDate:       aws.String("2025-07-12T06:57:02.000Z"),
		DeprecationTime:    aws.String("2026-02-09T23:47:00.000Z"),
		Public:             aws.Bool(true),
		BlockDeviceMappings: []ec2types.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &ec2types.EbsBlockDevice{
					SnapshotId:          aws.String("snap-05e8f994aa6314465"),
					VolumeSize:          aws.Int32(8),
					VolumeType:          ec2types.VolumeTypeGp2,
					DeleteOnTermination: aws.Bool(true),
					Encrypted:           aws.Bool(false),
				},
			},
		},
	}

	res := buildResource("ami-0aaa111111111111a", "ami-0aaa111111111111a", img)
	plain := stripAnsi(wave3RenderDetailFor(t, res, "ami", 120, 40))
	for _, want := range []string{
		"RunInstances",
		"/dev/sda1",
		"xen",
		"amazon",
		"simple",
		"snap-05e8f994aa6314465",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("AMI detail should include %q, got:\n%s", want, plain)
		}
	}
}

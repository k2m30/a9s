package unit_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

var defaultEC2RelatedDefs = append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)

func TestBug_RightColumnFilter_SlashFiltersAndEscapeClears(t *testing.T) {
	ensureNoColor(t)

	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: resource.NoopChecker},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: resource.NoopChecker},
	})
	t.Cleanup(func() { resource.UnregisterRelated("ec2") })

	d := makeDetailForFocusTest(t, 140)
	d = makeExplicitlyVisible(d)
	d, _ = d.Update(tabKeyMsg())

	d, _ = d.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, ch := range "trail" {
		d, _ = d.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}

	filtered := stripAnsi(d.View())
	if !strings.Contains(filtered, "CloudTrail Events") {
		t.Fatalf("right-column filter should keep matching rows visible, got:\n%s", filtered)
	}
	if strings.Contains(filtered, "CloudWatch Alarms") {
		t.Fatalf("right-column filter should hide non-matching rows, got:\n%s", filtered)
	}

	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	cleared := stripAnsi(d.View())
	if !strings.Contains(cleared, "CloudWatch Alarms") {
		t.Fatalf("escaping right-column filter should restore hidden rows, got:\n%s", cleared)
	}
}

func TestBug_AllZeroRelatedRows_DoNotAllowRightColumnFocus(t *testing.T) {
	ensureNoColor(t)

	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: resource.NoopChecker},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: resource.NoopChecker},
	})
	t.Cleanup(func() { resource.UnregisterRelated("ec2") })

	d := makeDetailForFocusTest(t, 140)
	d = makeExplicitlyVisible(d)
	d, _ = d.Update(messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result:       resource.RelatedCheckResult{TargetType: "alarm", Count: 0},
	})
	d, _ = d.Update(messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result:       resource.RelatedCheckResult{TargetType: "ct-events", Count: 0},
	})

	beforeTab := stripAnsi(d.View())
	d, _ = d.Update(tabKeyMsg())
	afterTab := stripAnsi(d.View())
	if beforeTab != afterTab {
		t.Fatalf("when every related row resolves to zero, Tab should not move focus into the right column")
	}

	d, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		if _, ok := cmd().(messages.RelatedNavigateMsg); ok {
			t.Fatal("when every related row resolves to zero, Enter should not navigate from the right column")
		}
	}
}

func TestBug_FirstToggleRelated_HidesAutoShownColumn(t *testing.T) {
	ensureNoColor(t)

	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: resource.NoopChecker},
	})
	t.Cleanup(func() { resource.UnregisterRelated("ec2") })

	d := makeDetailForFocusTest(t, 140)
	if !strings.Contains(stripAnsi(d.View()), "RELATED") {
		t.Fatal("precondition failed: related column should be auto-shown on wide EC2 detail")
	}

	d, firstCmd := d.Update(detailKeyPress("r"))
	if strings.Contains(stripAnsi(d.View()), "RELATED") {
		t.Fatalf("first press of r should hide the auto-shown related column")
	}
	if firstCmd != nil {
		t.Fatalf("first press of r should hide the column without refreshing related rows")
	}

	d, secondCmd := d.Update(detailKeyPress("r"))
	if !strings.Contains(stripAnsi(d.View()), "RELATED") {
		t.Fatalf("second press of r should show the related column again")
	}
	if secondCmd == nil {
		t.Fatalf("second press of r should re-open and refresh the related column")
	}
}

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

	m := newDetailModel(buildResource("i-0abc123456def7890", "ec2-with-volume", inst), "ec2", configForType("ec2"))
	plain := stripAnsi(m.View())
	if !strings.Contains(plain, "vol-0abc123456def7890") {
		t.Fatalf("default EC2 detail should show attached EBS volume IDs, got:\n%s", plain)
	}
}

func TestBug_EC2DefaultRelatedDefinitions_IncludeEBSVolumes(t *testing.T) {
	for _, def := range defaultEC2RelatedDefs {
		if def.TargetType == "ebs" {
			return
		}
	}
	t.Fatal("EC2 related definitions should include EBS volumes")
}

func TestBug_AMIDetail_ShowsUsefulImageMetadata(t *testing.T) {
	ensureNoColor(t)

	img := ec2types.Image{
		ImageId:            aws.String("ami-08f79bee58074adeb"),
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

	m := newDetailModel(buildResource("ami-08f79bee58074adeb", "ami-08f79bee58074adeb", img), "ami", configForType("ami"))
	plain := stripAnsi(m.View())
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

package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// Once the registry has scanning rules only the rules decide: under enhanced
// scanning a repository no rule matches is off, and under basic scanning it
// is set to manual frequency, whatever its own ScanOnPush says. In a basic
// registry with no rules the repository's own ScanOnPush scans it.
func TestT581ECR_ScanTypeDecidesWhetherTheRepositorySettingCounts(t *testing.T) {
	const (
		ruleCovered = "acme/api-service"
		ownOn       = "acme/frontend"
		ownOff      = fixtures.ECRRegistryScanned
	)
	t.Run("enhanced", func(t *testing.T) {
		rows := t581ECRRun(t, &t581ECRWrapper{scanning: &ecrtypes.RegistryScanningConfiguration{
			ScanType: ecrtypes.ScanTypeEnhanced,
			Rules:    []ecrtypes.RegistryScanningRule{t581ScanRule(ecrtypes.ScanFrequencyScanOnPush, "acme/api-*")},
		}})
		if r := t581Row(t, rows, ruleCovered); hasCode(r.Findings, t581CodeECRScanOnPushOff) {
			t.Errorf("%s: matched by an enhanced rule, raised %s", ruleCovered, t581CodeECRScanOnPushOff)
		}
		for _, name := range []string{ownOn, ownOff} {
			r := t581Row(t, rows, name)
			if !hasCode(r.Findings, t581CodeECRScanOnPushOff) {
				t.Errorf("%s: no enhanced rule matches it, raised no %s", name, t581CodeECRScanOnPushOff)
				continue
			}
			w2AssertRow(t, r.AttentionDetails[t581CodeECRScanOnPushOff].Rows, "Registry scanning", "enhanced, no rule matches this repository")
		}
	})
	t.Run("basic with a rule", func(t *testing.T) {
		rows := t581ECRRun(t, &t581ECRWrapper{scanning: &ecrtypes.RegistryScanningConfiguration{
			ScanType: ecrtypes.ScanTypeBasic,
			Rules:    []ecrtypes.RegistryScanningRule{t581ScanRule(ecrtypes.ScanFrequencyScanOnPush, "acme/api-*")},
		}})
		if r := t581Row(t, rows, ruleCovered); hasCode(r.Findings, t581CodeECRScanOnPushOff) {
			t.Errorf("%s: matched by a basic scan-on-push filter, raised %s", ruleCovered, t581CodeECRScanOnPushOff)
		}
		for _, name := range []string{ownOn, ownOff} {
			r := t581Row(t, rows, name)
			if !hasCode(r.Findings, t581CodeECRScanOnPushOff) {
				t.Errorf("%s: no basic filter matches it, raised no %s", name, t581CodeECRScanOnPushOff)
				continue
			}
			w2AssertRow(t, r.AttentionDetails[t581CodeECRScanOnPushOff].Rows, "Registry scanning", "basic, no scan-on-push filter matches this repository")
		}
	})
	t.Run("basic without rules", func(t *testing.T) {
		rows := t581ECRRun(t, &t581ECRWrapper{scanning: &ecrtypes.RegistryScanningConfiguration{ScanType: ecrtypes.ScanTypeBasic}})
		if r := t581Row(t, rows, ownOn); hasCode(r.Findings, t581CodeECRScanOnPushOff) {
			t.Errorf("%s: its own scan on push is on under basic scanning, raised %s", ownOn, t581CodeECRScanOnPushOff)
		}
		if r := t581Row(t, rows, ownOff); !hasCode(r.Findings, t581CodeECRScanOnPushOff) {
			t.Errorf("%s: scan on push off and no rule under basic scanning, raised no %s", ownOff, t581CodeECRScanOnPushOff)
		}
	})
}

type t581ECRRegistryRefused struct{ *t581ECRWrapper }

func (t581ECRRegistryRefused) GetRegistryScanningConfiguration(context.Context, *ecr.GetRegistryScanningConfigurationInput, ...func(*ecr.Options)) (*ecr.GetRegistryScanningConfigurationOutput, error) {
	return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform: ecr:GetRegistryScanningConfiguration"}
}

// A refused registry read does not say whether enhanced scanning ignores the
// repository setting, so a repository with its own scan on push on is as
// unknown as one with it off.
func TestT581ECR_RefusedRegistryReadLeavesEveryRepositoryNotInspected(t *testing.T) {
	clients := demo.NewServiceClients()
	w := &t581ECRWrapper{ECRAPI: clients.ECR}
	clients.ECR = t581ECRRegistryRefused{w}
	rows, result := t581LoadAndEnrich(t, "ecr", clients, nil)
	for _, name := range []string{"acme/frontend", fixtures.ECRRegistryScanned} {
		if hasCode(t581Row(t, rows, name).Findings, t581CodeECRScanOnPushOff) {
			t.Errorf("%s: a refused registry read raised %s", name, t581CodeECRScanOnPushOff)
		}
		if _, marked := result.TruncatedIDs[name]; !marked {
			t.Errorf("%s: a refused registry read left it inspected", name)
		}
	}
}

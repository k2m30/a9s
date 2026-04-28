package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeELBStateProvisioning   domain.FindingCode = "elb.state.provisioning"
	CodeELBStateActiveImpaired domain.FindingCode = "elb.state.active_impaired"
	CodeELBStateFailed         domain.FindingCode = "elb.state.failed"
)

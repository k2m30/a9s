package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeTGHealthUnhealthy   domain.FindingCode = "tg_health.state.unhealthy"
	CodeTGHealthDraining    domain.FindingCode = "tg_health.state.draining"
	CodeTGHealthUnavailable domain.FindingCode = "tg_health.state.unavailable"
	CodeTGHealthInitial     domain.FindingCode = "tg_health.state.initial"
)

// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures — costs.go provides the synthetic Cost Explorer dataset
// served by the demo transport's ce:* handlers (core/demo/handlers.go).
// Evergreen: CostsAnchorMonth is resolved once, from the real wall clock,
// when the process starts — not per-render, so a single run stays
// internally consistent, and every demo session opens on a genuinely
// current (never stale) trailing window.
package fixtures

import (
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
)

// CostsAnchorMonth is the last (open) month in the demo dataset — 13
// trailing months end here — resolved once at process start from the real
// current month.
var CostsAnchorMonth = currentMonthStart(time.Now())

func currentMonthStart(t time.Time) string {
	return costs.FormatDate(time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC))
}

// Shared dimension values every synthetic row carries for REGION/
// PURCHASE_TYPE group-bys, plus the two synthetic LINKED_ACCOUNT values
// every row's amount is deterministically split across (costsAccountSplit)
// — account IDs are canonical synthetic placeholders used throughout the
// demo fixtures (never a real account).
const (
	CostsDemoAccountID      = "123456789012"
	CostsDemoAccountName    = "demo-prod"
	CostsDemoAccountIDAlt   = "210987654321"
	CostsDemoAccountNameAlt = "demo-staging"
	CostsDemoRegion         = "us-east-1"
	CostsPurchaseOnDemand   = "On-Demand"
)

// CostsAccountName resolves a LINKED_ACCOUNT id to its display name, "" for
// an unrecognized id.
func CostsAccountName(id string) string {
	switch id {
	case CostsDemoAccountID:
		return CostsDemoAccountName
	case CostsDemoAccountIDAlt:
		return CostsDemoAccountNameAlt
	default:
		return ""
	}
}

// costsProdAccountShare is the deterministic fraction of every row's amount
// attributed to CostsDemoAccountID; the remainder (an exact float
// complement, not an independent multiplication) goes to
// CostsDemoAccountIDAlt so the two always sum back to the original amount.
const costsProdAccountShare = 0.6

// Planted growth story (spec.md US1 independent test): a GPU-inference
// fleet under CostsGrowthService ramps up starting CostsGrowthMonth, driven
// by usage type CostsGrowthUsageType. The anomaly overlay (spec.md US1,
// FR-014) names the same service + usage type so the "why did my bill jump"
// flow resolves end to end. CostsGrowthService is the same string as
// awsclient.CostExplorerServiceNameEC2 (a registered a9s detail-view
// mapping, resolved by costsResourceRowTargetType) so the spike's own
// resource-level drill (SERVICE -> USAGE_TYPE -> RESOURCE_ID) actually
// reaches a real EC2 detail view instead of being refused for an
// unsupported service.
const (
	CostsGrowthService   = awsclient.CostExplorerServiceNameEC2
	CostsGrowthUsageType = "USE1-BoxUsage:g5.xlarge"
)

// CostsGrowthMonth is the growth-story month: index 6 of the 13-month
// window costsMonthsEndingAt builds (matching growthMonthIdx below), i.e.
// 6 months before CostsAnchorMonth.
var CostsGrowthMonth = costsMonthsEndingAt(CostsAnchorMonth, 13)[6]

// CostsResourceRow is one (resource ID, daily amount) fact for the
// GetCostAndUsageWithResources synthetic dataset — resource IDs reference
// real fixture entities (core/demo/fixtures/ec2.go's EC2 instances) so
// a future resource-detail jump lands on an entity that actually exists.
type CostsResourceRow struct {
	ResourceID  string
	DailyAmount float64
}

// CostsResourceRowsByService is the resource-level drill dataset, keyed by
// the CE canonical SERVICE dimension value GetCostAndUsageWithResources is
// ever filtered on. Only services with a registered a9s detail-view mapping
// (a catalog entry whose CostExplorerServiceName is set, resolved by
// core/app/costs_state.go's costsResourceRowTargetType) carry entries.
// The three g5.xlarge instances (ec2.go's ml-inference-0{1,2,3}) are the
// growth story's own resource-drill target — their combined daily amounts
// plausibly attribute CostsGrowthMonth's spike; the other three (web-prod-0{1,2},
// api-staging-01) round out the service's ordinary, non-story spend.
var CostsResourceRowsByService = map[string][]CostsResourceRow{
	awsclient.CostExplorerServiceNameEC2: {
		{ResourceID: "i-0a1b2c3d4e5f60001", DailyAmount: 22.5},
		{ResourceID: "i-0a1b2c3d4e5f60002", DailyAmount: 14.0},
		{ResourceID: "i-0a1b2c3d4e5f60003", DailyAmount: 6.75},
		{ResourceID: "i-0a1b2c3d4e5f60040", DailyAmount: 2.8},
		{ResourceID: "i-0a1b2c3d4e5f60041", DailyAmount: 2.5},
		{ResourceID: "i-0a1b2c3d4e5f60042", DailyAmount: 2.1},
	},
}

// CostsRow is one (month, service, usage type, account) fact.
type CostsRow struct {
	Month      string // "YYYY-MM-01"
	Service    string
	UsageType  string
	RecordType string // "Usage" | "Tax" — CE's RECORD_TYPE dimension
	Account    string // CostsDemoAccountID | CostsDemoAccountIDAlt
	Amount     float64
}

// CostsAnomaly is the single planted GetAnomalies result.
type CostsAnomaly struct {
	ID        string
	Month     string
	Service   string
	UsageType string
	Impact    float64
	Score     float64
}

// CostsFixtures holds the synthetic 13-month dataset.
type CostsFixtures struct {
	Months  []string
	Rows    []CostsRow
	Anomaly CostsAnomaly
}

// ec2OtherUsageTypes are the three usage types "EC2 - Other" breaks into —
// an ordinary, non-story service kept flat/seasonal across the window (its
// own multi-usage-type breakdown is what keeps the demo grid's
// multi-service realism, distinct from the planted growth story below).
var (
	ec2OtherNatGatewayMonthly = [13]float64{150, 152, 148, 155, 151, 153, 149, 154, 150, 152, 148, 151, 150}
	ec2OtherBoxUsageMonthly   = [13]float64{110, 112, 109, 113, 111, 112, 118, 116, 115, 114, 113, 112, 95}
	ec2OtherDataXferMonthly   = [13]float64{40, 41, 39, 42, 40, 41, 43, 42, 41, 41, 40, 40, 20}
)

// ec2ComputeG5xlargeMonthly is the planted growth story (spec.md US1
// independent test): a GPU-inference fleet (ec2.go's ml-inference-0{1,2,3},
// instance type g5.xlarge) scales up starting CostsGrowthMonth and stays
// elevated through the anchor month (a step change, not a spike that
// reverts) — the last entry is lower because the anchor month is the open,
// partial current month.
var ec2ComputeG5xlargeMonthly = [13]float64{42, 44, 41, 45, 43, 44, 205, 208, 206, 204, 207, 205, 90}

// seasonalMultipliers gives every other service's flat baseline a small,
// deterministic month-to-month variation so the demo grid doesn't look
// artificially flat.
var seasonalMultipliers = [13]float64{1.00, 0.995, 1.02, 0.99, 1.03, 0.985, 1.01, 0.995, 1.015, 0.99, 1.02, 0.995, 1.00}

type costsServiceSpec struct {
	service    string
	usageType  string
	base       float64
	recordType string
}

// costsServiceSpecs is every service's ordinary (non-growth-story) baseline
// usage type — ~11 realistic services plus Tax so invoice totals include
// tax (FR-003), plus "EC2 - Other" built separately below. The first entry
// (Elastic Compute Cloud - Compute's own BoxUsage:m5.2xlarge baseline) sits
// alongside the growth story's g5.xlarge usage type (also built separately
// below) under the SAME service — a service can carry both an ordinary and
// a growth-story usage type, same as any real account. Every service name
// is the real AWS Cost Explorer SERVICE dimension value (GetDimensionValues),
// not a shorthand/marketing name — a CE-naming quirk each label is pinned
// against: "Amazon Relational Database Service" (not "Amazon RDS"),
// "Amazon Simple Storage Service" (not "Amazon S3"), "AmazonCloudWatch"
// (no space, not "Amazon CloudWatch"), "Amazon Elastic Kubernetes Service"
// (not "Amazon EKS").
var costsServiceSpecs = []costsServiceSpec{
	{"Amazon Elastic Compute Cloud - Compute", "BoxUsage:m5.2xlarge", 1200, "Usage"},
	{"Amazon Relational Database Service", "InstanceUsage:db.r5.large", 890, "Usage"},
	{"Amazon Simple Storage Service", "TimedStorage-ByteHrs", 140, "Usage"},
	{"AmazonCloudWatch", "MetricMonitorUsage", 88, "Usage"},
	{"Amazon DynamoDB", "ReadCapacityUnit-Hrs", 65, "Usage"},
	{"AWS Lambda", "Lambda-GB-Second", 42, "Usage"},
	{"Amazon ElastiCache", "NodeUsage:cache.r6g.large", 130, "Usage"},
	{"Amazon Elastic Kubernetes Service", "AmazonEKS-Hours", 220, "Usage"},
	{"Elastic Load Balancing", "LoadBalancerUsage", 95, "Usage"},
	{"Amazon Route 53", "HostedZone", 18, "Usage"},
	{"Tax", "Tax", 263, "Tax"},
}

// NewCostsFixtures builds the 13-month demo dataset.
func NewCostsFixtures() *CostsFixtures {
	months := costsMonthsEndingAt(CostsAnchorMonth, 13)

	var rows []CostsRow
	for _, sp := range costsServiceSpecs {
		for i, month := range months {
			rows = append(rows, CostsRow{
				Month:      month,
				Service:    sp.service,
				UsageType:  sp.usageType,
				RecordType: sp.recordType,
				Amount:     sp.base * seasonalMultipliers[i],
			})
		}
	}

	const ec2OtherService = "EC2 - Other"
	for i, month := range months {
		rows = append(rows,
			// EC2 - Other: an ordinary, non-story service (flat/seasonal
			// across the window) — kept for multi-service realism.
			CostsRow{Month: month, Service: ec2OtherService, UsageType: "USE1-NatGateway-Bytes", RecordType: "Usage", Amount: ec2OtherNatGatewayMonthly[i]},
			CostsRow{Month: month, Service: ec2OtherService, UsageType: "USE1-BoxUsage:m5.large", RecordType: "Usage", Amount: ec2OtherBoxUsageMonthly[i]},
			CostsRow{Month: month, Service: ec2OtherService, UsageType: "USE1-DataTransfer-Out-Bytes", RecordType: "Usage", Amount: ec2OtherDataXferMonthly[i]},
			// The planted growth story, under the SAME service as
			// costsServiceSpecs' own EC2-Compute baseline row.
			CostsRow{Month: month, Service: CostsGrowthService, UsageType: CostsGrowthUsageType, RecordType: "Usage", Amount: ec2ComputeG5xlargeMonthly[i]},
		)
	}

	growthMonthIdx, prevMonthIdx := 6, 5 // CostsGrowthMonth vs the month before it

	return &CostsFixtures{
		Months: months,
		Rows:   splitRowsAcrossAccounts(rows),
		Anomaly: CostsAnomaly{
			ID:        "anomaly-ec2compute-g5xlarge",
			Month:     CostsGrowthMonth,
			Service:   CostsGrowthService,
			UsageType: CostsGrowthUsageType,
			Impact:    ec2ComputeG5xlargeMonthly[growthMonthIdx] - ec2ComputeG5xlargeMonthly[prevMonthIdx],
			Score:     0.92,
		},
	}
}

// splitRowsAcrossAccounts deterministically splits every row's amount
// across the two synthetic LINKED_ACCOUNT values (costsProdAccountShare):
// a LINKED_ACCOUNT-grouped query sees two distinct rows; any other grouping
// re-sums them (row.Amount's remainder-based complement, not two
// independent multiplications) back to the exact original total.
func splitRowsAcrossAccounts(rows []CostsRow) []CostsRow {
	out := make([]CostsRow, 0, len(rows)*2)
	for _, r := range rows {
		prodAmt := r.Amount * costsProdAccountShare
		stagingAmt := r.Amount - prodAmt

		prod := r
		prod.Account = CostsDemoAccountID
		prod.Amount = prodAmt
		out = append(out, prod)

		staging := r
		staging.Account = CostsDemoAccountIDAlt
		staging.Amount = stagingAmt
		out = append(out, staging)
	}
	return out
}

// costsMonthsEndingAt returns n months ("YYYY-MM-01") trailing and ending
// at anchor, inclusive.
func costsMonthsEndingAt(anchor string, n int) []string {
	end, err := costs.ParseDate(anchor)
	if err != nil {
		return nil
	}
	out := make([]string, n)
	for i := range n {
		out[i] = costs.FormatDate(end.AddDate(0, -(n - 1 - i), 0))
	}
	return out
}

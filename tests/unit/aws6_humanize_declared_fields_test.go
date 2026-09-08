package unit_test

// aws6_humanize_declared_fields_test.go — the fields whose types declare them
// readable go on rendering as words.
//
// These twenty-nine were the last raw SDK constants on a demo detail screen.
// Each is now declared on its own type (ResourceTypeDef.HumanizeFields) and
// renders as words, so the sweep in aws6_humanize_owner_test.go never reaches
// them and they need no entry there — verbatimDetailPaths is for values that
// must stay as AWS wrote them, and none of these is one.
//
// The list survives as a ratchet, not as an exemption. Nothing here is
// excused, deferred or waiting: every key is asserted to render as words, and
// the day a declaration is dropped or a type stops reading it, this test says
// which field went back to showing the constant. A plain deletion of the list
// would leave that unwatched.

import (
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// humanizedDeclaredFields are the detail rows that MUST render as words: one
// per fact a type declares readable, named by the path the detail projector
// actually renders it under.
//
// That path is the SDK's, not the fetcher's key. A declaration names a FACT and
// the fact reaches the detail off RawStruct, so "state" is watched as
// StateValue and "status" as StackStatus. Watching the fetcher spelling instead
// watched rows the configured detail does not render at all, which is how
// INSUFFICIENT_DATA sat on the alarm screen while this ratchet reported the
// field readable.
//
// An entry is removed only when the field itself is gone; the second half of
// the test below fails if one stops rendering, so the list cannot rot.
var humanizedDeclaredFields = []string{
	"acm/Type",
	"alarm/StateValue",
	"apigw/protocol",
	"athena/State",
	"cf/Status",
	"cfn/StackStatus",
	"eb-rule/State",
	"ecr/ImageTagMutability",
	"ecs-svc/LaunchType",
	"ecs-svc/Status",
	"ecs-task/DesiredStatus",
	"ecs-task/LastStatus",
	"ecs-task/LaunchType",
	"ecs/Status",
	"eks/Status",
	"kinesis/StreamModeDetails.StreamMode",
	"kinesis/StreamStatus",
	"lambda/LastUpdateStatus",
	"msk/ClusterType",
	"msk/State",
	"mwaa/EndpointManagement",
	"mwaa/LastUpdate",
	"mwaa/WebserverAccessMode",
	"ng/Status",
	"pipeline/PipelineType",
	"ses/VerificationStatus",
	"sfn/Type",
	"ssm/Type",
	"tg/Protocol",
	"transfer/Domain",
	"transfer/EndpointType",
	"transfer/IdentityProviderType",
	"vpce/State",
}

func TestDeclaredFieldsGoOnRenderingAsWords(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	watched := make(map[string]bool, len(humanizedDeclaredFields))
	for _, k := range humanizedDeclaredFields {
		watched[k] = true
	}

	// One example per key: a constant on a field shows on every row of its
	// type, and one field is one thing to fix.
	example := map[string]string{}
	seen := map[string]bool{}
	for _, td := range resource.AllResourceTypes() {
		rows := mergeWave2Findings(t, td, byType[td.ShortName], cache, clients)
		for _, r := range rows {
			for _, f := range aws6DetailRows(t, r, td.ShortName) {
				key := td.ShortName + "/" + f.Path
				if !watched[key] || f.Value == "" {
					continue
				}
				seen[key] = true
				if _, already := example[key]; already {
					continue
				}
				if rawEnumCellPattern.MatchString(f.Value) ||
					rawCamelEnumCellPattern.MatchString(f.Value) ||
					rawEnumCellPattern.MatchString(scalarAfterLabel(f.Value)) {
					example[key] = "(row " + r.ID + ") = " + f.Value
				}
			}
		}
	}

	keys := make([]string, 0, len(example))
	for k := range example {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Errorf("%s %s renders an SDK constant — a declaration names a fact, and this is the row the "+
			"projector renders that fact under; declare the spelling on the type "+
			"(ResourceTypeDef.HumanizeFields) rather than recording the field anywhere",
			k, example[k])
	}

	// The ratchet cannot pass by not looking: every watched key has to be a
	// detail row the demo bench actually renders. A key that stops appearing
	// is a field that was renamed or removed, and the entry goes with it.
	var missing []string
	for _, k := range humanizedDeclaredFields {
		if !seen[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	for _, k := range missing {
		t.Errorf("no demo detail row renders %q, so this entry watches nothing — if the field is gone, "+
			"drop the entry with it; if it is only missing from the bench, the fixture that showed it went "+
			"away", k)
	}
}

// scalarAfterLabel returns the value part of a nested subtree row, which the
// projector renders as "Label: VALUE" on one line. Empty for a plain row.
func scalarAfterLabel(v string) string {
	_, after, found := strings.Cut(v, ": ")
	if !found {
		return ""
	}
	return strings.TrimSpace(after)
}

// declaredHumanizedPaths is humanizedDeclaredFields as a set, so the sweep in
// aws6_humanize_owner_test.go can leave these to the ratchet above rather than
// reporting the same field twice.
var declaredHumanizedPaths = func() map[string]bool {
	out := make(map[string]bool, len(humanizedDeclaredFields))
	for _, k := range humanizedDeclaredFields {
		out[k] = true
	}
	return out
}()

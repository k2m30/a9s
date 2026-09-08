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
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// humanizedDeclaredFields are (type, detail path) pairs that MUST render as
// words. A key is removed from here only when the field itself is gone.
var humanizedDeclaredFields = []string{
	"alarm/state",
	"athena/state",
	"cb/source_type",
	"cf/status",
	"cfn/status",
	"eb-rule/state",
	"ecr/tag_mutability",
	"ecs/status",
	"ecs-task/launch_type",
	"ecs-task/status",
	"eip/status",
	"eks/status",
	"kinesis/stream_mode",
	"kinesis/stream_status",
	"kms/status",
	"lambda/last_update_status",
	"msk/cluster_type",
	"msk/state",
	"opensearch/domain_processing_status",
	"pipeline/pipeline_type",
	"role/trust_summary",
	"secrets/status",
	"ses/verification_status",
	"sfn/type",
	"ssm/type",
	"tg/protocol",
	"transfer/domain",
	"vpce/state",
	"waf/scope",
}

func TestDeclaredFieldsGoOnRenderingAsWords(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	watched := make(map[string]bool, len(humanizedDeclaredFields))
	for _, k := range humanizedDeclaredFields {
		watched[k] = true
	}

	var raw []string
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
				if rawEnumCellPattern.MatchString(f.Value) || rawCamelEnumCellPattern.MatchString(f.Value) {
					raw = append(raw, key+" (row "+r.ID+") = "+f.Value)
				}
			}
		}
	}

	sort.Strings(raw)
	for i, s := range raw {
		if i > 0 && s == raw[i-1] {
			continue
		}
		t.Errorf("%s renders an SDK constant again — its type declared it readable and something dropped "+
			"that; restore the declaration on the type (ResourceTypeDef.HumanizeFields) rather than "+
			"recording the field anywhere", s)
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

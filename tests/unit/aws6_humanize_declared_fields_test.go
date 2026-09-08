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

	awsclient "github.com/k2m30/a9s/v3/core/aws"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// humanizedDeclaredFields are (type, detail path) pairs that MUST render as
// words. A key is removed from here only when the field itself is gone.
//
// A fact reaches the detail under two spellings — the fetcher's key and the
// SDK path the projector reads off RawStruct — and they are not always the
// same word (the alarm's state is Fields["state"] and RawStruct's StateValue).
// Watching only the fetcher spelling let INSUFFICIENT_DATA sit on the alarm
// screen while this ratchet reported the field readable, so the RawStruct
// spellings are listed alongside it and rendered through the view config the
// app loads at startup.
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

// humanizedDeclaredRawStructFields are the SDK paths under which the same
// declared facts reach the detail. They are listed separately because they are
// rendered by a different projector arm and only appear once the view config
// is loaded.
var humanizedDeclaredRawStructFields = []string{
	"alarm/StateValue",
	"cfn/StackStatus",
	"ecs-task/LastStatus",
	"kinesis/StreamModeDetails",
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
	missing = append(missing, rawStructRatchetViolations(t, byType, cache, clients, seen)...)
	sort.Strings(missing)
	for _, k := range missing {
		t.Errorf("no demo detail row renders %q, so this entry watches nothing — if the field is gone, "+
			"drop the entry with it; if it is only missing from the bench, the fixture that showed it went "+
			"away", k)
	}
}

// rawStructRatchetViolations runs the same ratchet over the RawStruct detail
// rows, which the flat pass above never reaches: without the view config the
// controller projects Fields only, so a constant on an SDK path is invisible
// to it. Reports a raw value directly and returns the keys that watch nothing.
func rawStructRatchetViolations(
	t *testing.T,
	byType map[string][]resource.Resource,
	cache resource.ResourceCache,
	clients *awsclient.ServiceClients,
	_ map[string]bool,
) []string {
	t.Helper()
	watched := make(map[string]bool, len(humanizedDeclaredRawStructFields))
	for _, k := range humanizedDeclaredRawStructFields {
		watched[k] = true
	}

	seen := map[string]bool{}
	// One example per key: a constant on a field shows on every row of its
	// type, and one field is one thing to fix.
	example := map[string]string{}
	for _, td := range resource.AllResourceTypes() {
		rows := mergeWave2Findings(t, td, byType[td.ShortName], cache, clients)
		for _, r := range rows {
			for _, f := range aws6DetailRowsWithViews(t, r, td.ShortName) {
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
		t.Errorf("%s %s renders an SDK constant on the row the projector reads off RawStruct — the type "+
			"declares this fact readable under its fetcher key, and a declaration names a fact, not a "+
			"spelling", k, example[k])
	}

	var missing []string
	for _, k := range humanizedDeclaredRawStructFields {
		if !seen[k] {
			missing = append(missing, k)
		}
	}
	return missing
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

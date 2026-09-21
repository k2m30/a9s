package unit_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

// relatedPairFacts records, for every pair read against the AWS API, the field
// each direction reads and whether the two directions read one fact (the same
// relationship from its two ends, Mirror) or two facts that happen to join the
// same two types (Distinct). Field names are the ones the AWS API Reference
// uses, and each is what the corresponding docs/related-resources.md line and,
// for a Distinct direction, the Distinct phrase must name.
var relatedPairFacts = []struct {
	a, b             string
	oneFact          bool
	fieldAB, fieldBA string
}{
	{a: "ec2", b: "ebs", oneFact: true, fieldAB: "BlockDeviceMappings", fieldBA: "Volume.Attachments"},
	{a: "rtb", b: "subnet", oneFact: true, fieldAB: "Associations", fieldBA: "Associations"},
	{a: "tgw", b: "vpc", oneFact: true, fieldAB: "TransitGateway", fieldBA: "TransitGateway"},
	{a: "subnet", b: "vpc", oneFact: true, fieldAB: "Subnet.VpcId", fieldBA: "Subnet.VpcId"},
	{a: "ec2", b: "eni", oneFact: true, fieldAB: "NetworkInterfaces", fieldBA: "Attachment.InstanceId"},
	{a: "iam-group", b: "iam-user", oneFact: true, fieldAB: "GetGroup", fieldBA: "ListGroupsForUser"},
	{a: "acm", b: "elb", oneFact: true, fieldAB: "InUseBy", fieldBA: "Certificates"},
	{a: "eip", b: "eni", oneFact: true, fieldAB: "NetworkInterfaceId", fieldBA: "AllocationId"},
	{a: "iam-group", b: "policy", oneFact: true, fieldAB: "ListAttachedGroupPolicies", fieldBA: "ListEntitiesForPolicy"},
	{a: "role", b: "policy", oneFact: true, fieldAB: "ListAttachedRolePolicies", fieldBA: "ListEntitiesForPolicy"},

	{a: "ec2", b: "ebs-snap", fieldAB: "Snapshot.VolumeId", fieldBA: "Snapshot.Description"},
	{a: "ec2", b: "tg", fieldAB: "TargetGroup.VpcId", fieldBA: "DescribeTargetHealth"},
	{a: "cf", b: "s3", fieldAB: "Logging.Bucket", fieldBA: "Origins.Items"},
	{a: "cfn", b: "s3", fieldAB: "ListStackResources", fieldBA: "aws:cloudformation:stack-name"},
	{a: "acm", b: "apigw", fieldAB: "InUseBy", fieldBA: "DomainNameConfigurations"},
	{a: "ecs-task", b: "logs", fieldAB: "awslogs-group", fieldBA: "family"},
	{a: "acm", b: "r53", fieldAB: "DomainValidationOptions", fieldBA: "acm-validations.aws"},
}

// oneFactPairs are the pairs of relatedPairFacts whose two directions read one
// AWS fact, in the shape the demo symmetry test holds Mirror pairs to.
func oneFactPairs() [][2]string {
	var out [][2]string
	for _, p := range relatedPairFacts {
		if p.oneFact {
			out = append(out, [2]string{p.a, p.b})
		}
	}
	return out
}

// relatedDirectionDecision names what a direction declares about its reverse.
func relatedDirectionDecision(def resource.RelatedDef) string {
	switch {
	case def.Mirror && def.Distinct != "":
		return "Mirror and Distinct"
	case def.Mirror:
		return "Mirror"
	case def.Distinct != "":
		return "Distinct"
	}
	return "neither"
}

// relatedPairUndecided returns "" when both directions of a pair make the same
// single declaration, and otherwise the sentence naming what each side
// declares. lookup answers with the RelatedDef registered for a direction.
func relatedPairUndecided(a, b string, lookup func(src, target string) (resource.RelatedDef, bool)) string {
	defAB, okAB := lookup(a, b)
	defBA, okBA := lookup(b, a)
	if !okAB || !okBA {
		return fmt.Sprintf("%s <-> %s: not registered in both directions", a, b)
	}
	dAB, dBA := relatedDirectionDecision(defAB), relatedDirectionDecision(defBA)
	if dAB == dBA && (dAB == "Mirror" || dAB == "Distinct") {
		return ""
	}
	return fmt.Sprintf("%s <-> %s: %s -> %s declares %s, %s -> %s declares %s", a, b, a, b, dAB, b, a, dBA)
}

// mutuallyRegisteredPairs lists, once per unordered pair, the types that each
// register a pivot at the other, excluding the pairs symmetryOutOfScope covers.
func mutuallyRegisteredPairs() [][2]string {
	registered := map[[2]string]bool{}
	for _, td := range resource.AllResourceTypes() {
		for _, def := range resource.GetRelated(td.ShortName) {
			registered[[2]string{td.ShortName, def.TargetType}] = true
		}
	}
	seen := map[[2]string]bool{}
	var out [][2]string
	for dir := range registered {
		a, b := dir[0], dir[1]
		if a == b || symmetryOutOfScope[a] || symmetryOutOfScope[b] || !registered[[2]string{b, a}] {
			continue
		}
		if a > b {
			a, b = b, a
		}
		if seen[[2]string{a, b}] {
			continue
		}
		seen[[2]string{a, b}] = true
		out = append(out, [2]string{a, b})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// TestRelatedPairDecision_EveryMutuallyRegisteredPairIsDecided pins that a
// pair whose two types each register a pivot at the other says which it is:
// one relationship read from two ends, where a link one side shows and the
// other denies is a defect, or two relationships, where the difference is the
// answer. Undeclared, the two are indistinguishable.
func TestRelatedPairDecision_EveryMutuallyRegisteredPairIsDecided(t *testing.T) {
	pairs := mutuallyRegisteredPairs()
	if len(pairs) < 100 {
		t.Fatalf("mutuallyRegisteredPairs found %d pairs; the registry holds far more, so the enumeration is not reading it", len(pairs))
	}

	var undecided []string
	for _, p := range pairs {
		if msg := relatedPairUndecided(p[0], p[1], relatedDefFor); msg != "" {
			undecided = append(undecided, msg)
		}
	}
	if len(undecided) > 0 {
		t.Errorf("%d of %d mutually registered pairs are undecided:\n  %s",
			len(undecided), len(pairs), strings.Join(undecided, "\n  "))
	}
}

// TestRelatedPairDecision_GateNamesTheUndeclaredSide pins what the gate says
// when a pair stops being decided, so the message points at the direction to
// change rather than at the pair alone.
func TestRelatedPairDecision_GateNamesTheUndeclaredSide(t *testing.T) {
	const a, b = "cf", "r53"
	if msg := relatedPairUndecided(a, b, relatedDefFor); msg != "" {
		t.Fatalf("%s <-> %s is decided today: %s", a, b, msg)
	}

	silenced := func(src, target string) (resource.RelatedDef, bool) {
		def, ok := relatedDefFor(src, target)
		if src == a && target == b {
			def.Mirror, def.Distinct = false, ""
		}
		return def, ok
	}
	const wantSilenced = "cf <-> r53: cf -> r53 declares neither, r53 -> cf declares Mirror"
	if got := relatedPairUndecided(a, b, silenced); got != wantSilenced {
		t.Errorf("one side undeclared:\n got %q\nwant %q", got, wantSilenced)
	}

	contradictory := func(src, target string) (resource.RelatedDef, bool) {
		def, ok := relatedDefFor(src, target)
		if src == a && target == b {
			def.Distinct = "the distribution's origin domain names"
		}
		return def, ok
	}
	const wantContradictory = "cf <-> r53: cf -> r53 declares Mirror and Distinct, r53 -> cf declares Mirror"
	if got := relatedPairUndecided(a, b, contradictory); got != wantContradictory {
		t.Errorf("one direction declaring both:\n got %q\nwant %q", got, wantContradictory)
	}
}

// TestRelatedPairDecision_ReadPairsCarryTheirVerdict pins the verdict read off
// the AWS API for each pair in relatedPairFacts, and that a Distinct direction
// names its own field where a Mirror direction has nothing of its own to name.
func TestRelatedPairDecision_ReadPairsCarryTheirVerdict(t *testing.T) {
	for _, p := range relatedPairFacts {
		for _, dir := range [][3]string{{p.a, p.b, p.fieldAB}, {p.b, p.a, p.fieldBA}} {
			def, ok := relatedDefFor(dir[0], dir[1])
			if !ok {
				t.Errorf("%s registers no %s pivot", dir[0], dir[1])
				continue
			}
			if p.oneFact {
				if !def.Mirror {
					t.Errorf("%s -> %s is not Mirror; both directions read one AWS fact", dir[0], dir[1])
				}
				if def.Distinct != "" {
					t.Errorf("%s -> %s is Mirror and also names a Distinct fact %q", dir[0], dir[1], def.Distinct)
				}
				continue
			}
			if def.Mirror {
				t.Errorf("%s -> %s is Mirror; the two directions read different AWS fields", dir[0], dir[1])
			}
			if def.Distinct == "" {
				t.Errorf("%s -> %s names no Distinct fact; it reads %s", dir[0], dir[1], dir[2])
				continue
			}
			if !strings.Contains(def.Distinct, dir[2]) {
				t.Errorf("%s -> %s Distinct = %q, which does not name %s", dir[0], dir[1], def.Distinct, dir[2])
			}
		}
	}
}

// relatedDocBullets parses docs/related-resources.md "Per-target reasoning"
// and returns, per source type, the reasoning line keyed by target type.
func relatedDocBullets(t *testing.T) map[string]map[string]string {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "related-resources.md")
	raw, err := os.ReadFile(path) //nolint:gosec // fixed path inside the repo
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	out := map[string]map[string]string{}
	source := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(line, "### `"); ok {
			source, _, _ = strings.Cut(rest, "`")
			out[source] = map[string]string{}
			continue
		}
		if source == "" {
			continue
		}
		rest, ok := strings.CutPrefix(line, "- **`")
		if !ok {
			continue
		}
		target, body, found := strings.Cut(rest, "`**")
		if !found {
			continue
		}
		out[source][target] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(body), "—"))
	}
	if len(out) == 0 {
		t.Fatalf("%s holds no per-target reasoning sections", path)
	}
	return out
}

// TestRelatedPairDecision_DocNamesTheFieldEachDirectionReads pins that the
// contract in docs/related-resources.md names, for each direction of every
// pair in relatedPairFacts, the AWS field that direction reads — the same
// field the Distinct phrase names, so the doc and the registration cannot
// drift into describing two different relationships.
func TestRelatedPairDecision_DocNamesTheFieldEachDirectionReads(t *testing.T) {
	bullets := relatedDocBullets(t)
	for _, p := range relatedPairFacts {
		for _, dir := range [][3]string{{p.a, p.b, p.fieldAB}, {p.b, p.a, p.fieldBA}} {
			line, ok := bullets[dir[0]][dir[1]]
			if !ok {
				t.Errorf("docs/related-resources.md carries no `%s` line under ### `%s`", dir[1], dir[0])
				continue
			}
			if !strings.Contains(line, dir[2]) {
				t.Errorf("docs/related-resources.md `%s` -> `%s` reads %s but its line does not name it: %q",
					dir[0], dir[1], dir[2], line)
			}
		}
	}
}

package unit_test

// aws6_rawstruct_detail_reads_as_words_test.go — the detail rows projected off
// the SDK struct read as words too.
//
// A type's declaration names a fact, not a spelling. The fetcher writes that
// fact under a snake_case key and the detail projector also renders it off
// RawStruct under the SDK's own path, and those two spellings are not always
// the same word: the CloudWatch alarm's state is Fields["state"] and
// RawStruct's StateValue. A declaration that names only the fetcher's key
// therefore reaches one row and not the other, and the operator reads
// INSUFFICIENT_DATA on the screen the declaration was added for.
//
// A nested scalar is the same problem one level down. The projector composes a
// path for a subfield — StreamModeDetails.StreamMode — but the item it emits
// keeps the parent's path and carries the whole rendered line as its value, so
// a dotted declaration matches nothing and humanizing the line would reword the
// label with it.
//
// Both rows come off RawStruct, which the shared detail helper renders because
// the controller it builds carries the view config production carries.

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	kintypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rawStructDetailWitness is one resource carrying only the SDK struct, so the
// row under test can only come from the RawStruct projection.
var rawStructDetailWitnesses = []struct {
	shortName string
	label     string // the detail row's label as the projector writes it
	raw       any
	constant  string // what the screen shows today
	want      string // the words it must show
}{
	{
		shortName: "alarm",
		label:     "StateValue",
		raw: cwtypes.MetricAlarm{
			AlarmName:  aws.String("acme-probe-alarm"),
			StateValue: cwtypes.StateValueInsufficientData,
		},
		constant: "INSUFFICIENT_DATA",
		want:     "insufficient data",
	},
	{
		shortName: "cfn",
		label:     "StackStatus",
		raw: cfntypes.Stack{
			StackName:   aws.String("acme-probe-stack"),
			StackStatus: cfntypes.StackStatusRollbackComplete,
		},
		constant: "ROLLBACK_COMPLETE",
		want:     "rollback complete",
	},
	{
		shortName: "ecs-task",
		label:     "LastStatus",
		raw: ecstypes.Task{
			TaskArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-probe/aaaabbbbccccdddd"),
			LastStatus: aws.String("DEACTIVATING"),
		},
		constant: "DEACTIVATING",
		want:     "deactivating",
	},
}

// TestRawStructDetailRowReadsAsWords pins row 24's three named screens.
func TestRawStructDetailRowReadsAsWords(t *testing.T) {
	for _, w := range rawStructDetailWitnesses {
		t.Run(w.shortName+"/"+w.label, func(t *testing.T) {
			res := resource.Resource{
				ID: "acme-probe", Name: "acme-probe", Type: w.shortName, RawStruct: w.raw,
			}
			rows := aws6DetailRows(t, res, w.shortName)
			if len(rows) == 0 {
				t.Fatalf("%s: the detail rendered no rows at all", w.shortName)
			}

			got, ok := detailRowValueByLabel(rows, w.label)
			if !ok {
				t.Fatalf("%s: no detail row labelled %q; the projector no longer renders this "+
					"RawStruct path, so the scenario has moved", w.shortName, w.label)
			}
			if got == w.constant {
				t.Errorf("%s: the %s row shows the SDK constant %q; the type declares this fact readable "+
					"under its fetcher key, and a declaration names a FACT, so every spelling of it — the "+
					"fetcher's key and the RawStruct path — reads the same words. Want %q",
					w.shortName, w.label, got, w.want)
				return
			}
			if got != w.want {
				t.Errorf("%s: the %s row = %q, want %q", w.shortName, w.label, got, w.want)
			}
		})
	}
}

// TestDottedDeclarationHumanizesTheNestedScalar pins row 23 for every dotted
// declaration the catalog carries, so a second one added later is covered
// without this test changing.
func TestDottedDeclarationHumanizesTheNestedScalar(t *testing.T) {
	dotted := dottedDeclarations(t)
	if len(dotted) == 0 {
		t.Fatal("no type declares a dotted humanize field, so nothing exercises the nested-scalar path")
	}

	// kinesis is the witness; a dotted declaration on another type is reported
	// so it gets a witness of its own rather than riding on this one.
	const kinesisShortName = "kinesis"
	for _, d := range dotted {
		if d.shortName != kinesisShortName {
			t.Errorf("%s declares the dotted field %q and has no witness here; the nested-scalar path "+
				"needs one resource per dotted declaration", d.shortName, d.field)
		}
	}

	res := resource.Resource{
		ID: "acme-probe-stream", Name: "acme-probe-stream", Type: kinesisShortName,
		RawStruct: kintypes.StreamSummary{
			StreamName:        aws.String("acme-probe-stream"),
			StreamModeDetails: &kintypes.StreamModeDetails{StreamMode: kintypes.StreamModeOnDemand},
		},
	}
	rows := aws6DetailRows(t, res, kinesisShortName)
	if len(rows) == 0 {
		t.Fatal("kinesis: the detail rendered no rows at all")
	}

	var line string
	for _, f := range rows {
		if strings.Contains(f.Value, "StreamMode") || strings.Contains(f.Key, "StreamMode") {
			line = f.Value
		}
	}
	if line == "" {
		t.Fatal("kinesis: no detail row renders the nested StreamMode; the scenario has moved")
	}
	if strings.Contains(line, "ON_DEMAND") {
		t.Errorf("kinesis: the nested StreamMode row reads %q; the dotted declaration "+
			"%q humanizes the list cell and must reach this row too — the projector composes the "+
			"path but the item keeps the parent's, so the declaration matches nothing",
			line, "StreamModeDetails.StreamMode")
	}
	if !strings.Contains(line, "on demand") {
		t.Errorf("kinesis: the nested StreamMode row reads %q, want the readable mode in it", line)
	}
	// The label is not the value: humanizing the whole rendered line would
	// lowercase "StreamMode" along with the constant.
	if !strings.Contains(line, "StreamMode") {
		t.Errorf("kinesis: the nested row reads %q and no longer names the subfield; the scalar is "+
			"humanized, the label is not", line)
	}
}

// detailRowValueByLabel returns the value of the detail row whose label or path
// matches name.
func detailRowValueByLabel(rows []app.FieldRow, name string) (string, bool) {
	for _, f := range rows {
		if strings.EqualFold(f.Key, name) || strings.EqualFold(f.Path, name) {
			return f.Value, true
		}
	}
	return "", false
}

// dottedDeclarations lists every (type, field) where a type declares a nested
// path rather than a flat key.
func dottedDeclarations(t *testing.T) []struct{ shortName, field string } {
	t.Helper()
	var out []struct{ shortName, field string }
	for _, td := range resource.AllResourceTypes() {
		for _, f := range td.HumanizeFields {
			if strings.Contains(f, ".") {
				out = append(out, struct{ shortName, field string }{td.ShortName, f})
			}
		}
	}
	return out
}

package unit_test

// aws6_fake_models_only_real_errors_test.go — a fake refuses only where the
// real operation refuses.
//
// The rule that every fake is honest about a key it does not hold is per
// OPERATION, not per service. A read whose API models a not-found error answers
// one for an unregistered key; a read that has no such error answers what AWS
// answers, which for a listing is an empty list.
//
// CloudWatch's DescribeAlarmHistory is the second kind. Its API Reference lists
// InvalidNextToken as the only error it returns, and its own doc comment says
// CloudWatch retains the history of an alarm even after the alarm is deleted —
// an unknown name is simply a query that matches nothing. types.ResourceNotFound
// ("The named resource does not exist",
// service/cloudwatch/types/errors.go:784) belongs to the dashboard and
// anomaly-detector operations, not to this one. A fake answering it turns an
// empty screen into a fetch error that AWS would never produce, which is the
// same class of lie as answering empty where AWS refuses — just pointing the
// other way.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
)

// TestCloudWatchFakeAnswersEmptyHistoryForAnUnknownAlarm pins the operation's
// real behaviour.
func TestCloudWatchFakeAnswersEmptyHistoryForAnUnknownAlarm(t *testing.T) {
	f := fakes.NewCloudWatch()

	out, err := f.DescribeAlarmHistory(context.Background(), &cloudwatch.DescribeAlarmHistoryInput{
		AlarmName: aws.String(unregisteredKey),
	})
	if err != nil {
		t.Fatalf("DescribeAlarmHistory for the unregistered alarm %q returned %v; the operation models no "+
			"not-found error and an unknown name matches no history, so demo mode shows a fetch error "+
			"where AWS shows an empty list", unregisteredKey, err)
	}
	if out == nil {
		t.Fatal("DescribeAlarmHistory returned no output and no error")
	}
	if len(out.AlarmHistoryItems) != 0 {
		t.Errorf("DescribeAlarmHistory for an unknown alarm returned %d item(s), want none",
			len(out.AlarmHistoryItems))
	}
}

// TestRegisteredAlarmStillAnswersItsHistory is the negative case: answering
// empty for an unknown name must not become answering empty for every name.
func TestRegisteredAlarmStillAnswersItsHistory(t *testing.T) {
	f := fakes.NewCloudWatch()
	alarms, err := f.DescribeAlarms(context.Background(), &cloudwatch.DescribeAlarmsInput{})
	if err != nil {
		t.Fatalf("DescribeAlarms: %v", err)
	}

	withHistory := 0
	for _, a := range alarms.MetricAlarms {
		out, err := f.DescribeAlarmHistory(context.Background(), &cloudwatch.DescribeAlarmHistoryInput{
			AlarmName: a.AlarmName,
		})
		if err != nil {
			t.Fatalf("DescribeAlarmHistory(%q): %v", aws.ToString(a.AlarmName), err)
		}
		if len(out.AlarmHistoryItems) > 0 {
			withHistory++
		}
	}
	if withHistory == 0 {
		t.Error("no demo alarm answers any history, so an empty answer for an unknown alarm proves " +
			"nothing about the registered ones")
	}
}

// TestNoFakeRefusalUsesCloudWatchResourceNotFound is the class check. Once the
// history call stops answering it, no fake refusal spells not-found that way,
// and the code has no reader left to keep it in the one error table for.
func TestNoFakeRefusalUsesCloudWatchResourceNotFound(t *testing.T) {
	for _, pin := range fakeNotFoundPins() {
		if pin.wantCode != "ResourceNotFound" {
			continue
		}
		t.Errorf("the %s pin still expects the refusal code %q; CloudWatch models that error on its "+
			"dashboard and anomaly-detector operations, and no read a9s makes is one of them",
			pin.name, pin.wantCode)
	}

	// And the table entry goes with its last reader: a code no operation
	// answers is a branch nothing reaches.
	if awsclient.ErrCodeIs(cloudWatchResourceNotFoundProbe(), "ResourceNotFound") &&
		awsclient.IsNotFoundErr(cloudWatchResourceNotFoundProbe()) {
		t.Error("\"ResourceNotFound\" is still classified as not-found while no a9s read receives it; " +
			"the one error table lists codes its readers actually see, and an entry with no reader is " +
			"a rule nobody can check")
	}
}

// cloudWatchResourceNotFoundProbe is the error the fake used to answer, kept
// only so the check above has something to classify.
func cloudWatchResourceNotFoundProbe() error {
	return &smithy.GenericAPIError{Code: "ResourceNotFound", Message: "The named resource does not exist."}
}

package demo

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	smithycbor "github.com/aws/smithy-go/encoding/cbor"
)

// registerMonitoringHandlers registers CloudWatch, CW Logs, and CloudTrail handlers.
func registerMonitoringHandlers(t *Transport) {
	registerCWAlarmHandlers(t)
	registerCWLogsHandlers(t)
	registerCloudTrailHandlers(t)
}

// ---------------------------------------------------------------------------
// CloudWatch Alarms (Smithy RPCv2 CBOR, service "monitoring")
// The modern CloudWatch SDK v2 uses the Smithy RPCv2 CBOR wire protocol.
// Requests use Content-Type: application/cbor and URL path
// /service/CloudWatch/operation/DescribeAlarms.
// ---------------------------------------------------------------------------

func registerCWAlarmHandlers(t *Transport) {
	t.Handle("monitoring", "DescribeAlarms", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["alarm"]()
		alarms := ExtractSDK[cwtypes.MetricAlarm](resources)

		alarmList := make(smithycbor.List, 0, len(alarms))
		for _, alarm := range alarms {
			m := smithycbor.Map{
				"AlarmName":  smithycbor.String(aws.ToString(alarm.AlarmName)),
				"AlarmArn":   smithycbor.String(aws.ToString(alarm.AlarmArn)),
				"StateValue": smithycbor.String(string(alarm.StateValue)),
				"MetricName": smithycbor.String(aws.ToString(alarm.MetricName)),
				"Namespace":  smithycbor.String(aws.ToString(alarm.Namespace)),
			}
			if alarm.Threshold != nil {
				m["Threshold"] = smithycbor.Float64(*alarm.Threshold)
			}
			if alarm.ActionsEnabled != nil {
				m["ActionsEnabled"] = smithycbor.Bool(*alarm.ActionsEnabled)
			}
			alarmList = append(alarmList, m)
		}

		response := smithycbor.Map{
			"MetricAlarms":    alarmList,
			"CompositeAlarms": smithycbor.List{},
		}
		return CBORResponse(response), nil
	})
}

// ---------------------------------------------------------------------------
// CloudWatch Logs (awsjson11, service "logs")
// The deserializer expects lowercase camelCase keys: "logGroups".
// ---------------------------------------------------------------------------

func registerCWLogsHandlers(t *Transport) {
	t.Handle("logs", "DescribeLogGroups", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["logs"]()
		logGroups := ExtractSDK[cwlogstypes.LogGroup](resources)

		groups := make([]map[string]interface{}, 0, len(logGroups))
		for _, lg := range logGroups {
			m := map[string]interface{}{
				"logGroupName": aws.ToString(lg.LogGroupName),
			}
			if lg.StoredBytes != nil {
				m["storedBytes"] = *lg.StoredBytes
			}
			if lg.RetentionInDays != nil {
				m["retentionInDays"] = *lg.RetentionInDays
			}
			if lg.Arn != nil {
				m["arn"] = *lg.Arn
			}
			groups = append(groups, m)
		}

		return JSONResponse(map[string]interface{}{"logGroups": groups})
	})
}

// ---------------------------------------------------------------------------
// CloudTrail (awsjson11, service "cloudtrail")
// The deserializer expects lowercase camelCase: "trailList".
// ---------------------------------------------------------------------------

func registerCloudTrailHandlers(t *Transport) {
	t.Handle("cloudtrail", "DescribeTrails", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["trail"]()
		trails := ExtractSDK[cloudtrailtypes.Trail](resources)

		list := make([]map[string]interface{}, 0, len(trails))
		for _, tr := range trails {
			m := map[string]interface{}{
				"Name":        aws.ToString(tr.Name),
				"TrailARN":    aws.ToString(tr.TrailARN),
				"HomeRegion":  aws.ToString(tr.HomeRegion),
				"IsMultiRegionTrail": tr.IsMultiRegionTrail != nil && *tr.IsMultiRegionTrail,
			}
			if tr.S3BucketName != nil {
				m["S3BucketName"] = *tr.S3BucketName
			}
			list = append(list, m)
		}

		return JSONResponse(map[string]interface{}{"trailList": list})
	})

	t.Handle("cloudtrail", "LookupEvents", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["ct-events"]()
		events := ExtractSDK[cloudtrailtypes.Event](resources)

		eventList := make([]map[string]interface{}, 0, len(events))
		for _, ev := range events {
			m := map[string]interface{}{
				"EventId":     aws.ToString(ev.EventId),
				"EventName":   aws.ToString(ev.EventName),
				"EventSource": aws.ToString(ev.EventSource),
				"ReadOnly":    aws.ToString(ev.ReadOnly),
				"Username":    aws.ToString(ev.Username),
			}
			if ev.EventTime != nil {
				m["EventTime"] = float64(ev.EventTime.Unix())
			}
			if ev.CloudTrailEvent != nil {
				m["CloudTrailEvent"] = *ev.CloudTrailEvent
			}
			if len(ev.Resources) > 0 {
				resList := make([]map[string]interface{}, 0, len(ev.Resources))
				for _, r := range ev.Resources {
					resList = append(resList, map[string]interface{}{
						"ResourceType": aws.ToString(r.ResourceType),
						"ResourceName": aws.ToString(r.ResourceName),
					})
				}
				m["Resources"] = resList
			}
			eventList = append(eventList, m)
		}

		return JSONResponse(map[string]interface{}{"Events": eventList})
	})
}

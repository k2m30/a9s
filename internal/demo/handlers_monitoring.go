package demo

import (
	"encoding/json"
	"io"
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

		groups := make([]map[string]any, 0, len(logGroups))
		for _, lg := range logGroups {
			m := map[string]any{
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

		return JSONResponse(map[string]any{"logGroups": groups})
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

		list := make([]map[string]any, 0, len(trails))
		for _, tr := range trails {
			m := map[string]any{
				"Name":               aws.ToString(tr.Name),
				"TrailARN":           aws.ToString(tr.TrailARN),
				"HomeRegion":         aws.ToString(tr.HomeRegion),
				"IsMultiRegionTrail": tr.IsMultiRegionTrail != nil && *tr.IsMultiRegionTrail,
			}
			if tr.S3BucketName != nil {
				m["S3BucketName"] = *tr.S3BucketName
			}
			list = append(list, m)
		}

		return JSONResponse(map[string]any{"trailList": list})
	})

	t.Handle("cloudtrail", "LookupEvents", func(r *http.Request) (*http.Response, error) {
		resources := demoData["ct-events"]()
		events := ExtractSDK[cloudtrailtypes.Event](resources)

		attrs := parseLookupAttributes(r)
		if len(attrs) > 0 {
			filtered := make([]cloudtrailtypes.Event, 0, len(events))
			for _, ev := range events {
				if lookupEventMatches(ev, attrs) {
					filtered = append(filtered, ev)
				}
			}
			events = filtered
		}

		eventList := make([]map[string]any, 0, len(events))
		for _, ev := range events {
			m := map[string]any{
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
				resList := make([]map[string]any, 0, len(ev.Resources))
				for _, r := range ev.Resources {
					resList = append(resList, map[string]any{
						"ResourceType": aws.ToString(r.ResourceType),
						"ResourceName": aws.ToString(r.ResourceName),
					})
				}
				m["Resources"] = resList
			}
			eventList = append(eventList, m)
		}

		return JSONResponse(map[string]any{"Events": eventList})
	})
}

// parseLookupAttributes reads the JSON request body and returns the
// LookupAttributes slice as a key→value map. Returns nil on any parse error
// so the caller falls back to the unfiltered path.
func parseLookupAttributes(r *http.Request) map[string]string {
	if r == nil || r.Body == nil {
		return nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return nil
	}
	var req struct {
		LookupAttributes []struct {
			AttributeKey   string `json:"AttributeKey"`
			AttributeValue string `json:"AttributeValue"`
		} `json:"LookupAttributes"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}
	if len(req.LookupAttributes) == 0 {
		return nil
	}
	out := make(map[string]string, len(req.LookupAttributes))
	for _, a := range req.LookupAttributes {
		if a.AttributeKey != "" {
			out[a.AttributeKey] = a.AttributeValue
		}
	}
	return out
}

// lookupEventMatches reports whether a fixture event matches all requested
// LookupAttributes (AND semantics). Keys not handled here return false so
// the event is excluded — prefer strictness over permissiveness.
func lookupEventMatches(ev cloudtrailtypes.Event, attrs map[string]string) bool {
	for key, want := range attrs {
		switch key {
		case "Username":
			if aws.ToString(ev.Username) != want {
				return false
			}
		case "EventName":
			if aws.ToString(ev.EventName) != want {
				return false
			}
		case "EventSource":
			if aws.ToString(ev.EventSource) != want {
				return false
			}
		case "ResourceType":
			if !anyResourceMatchesField(ev.Resources, "type", want) {
				return false
			}
		case "ResourceName":
			if !anyResourceMatchesField(ev.Resources, "name", want) {
				return false
			}
		case "AccessKeyId":
			if extractAccessKeyIdFromCTEvent(ev) != want {
				return false
			}
		default:
			// Unknown attribute key — be strict, exclude the event.
			return false
		}
	}
	return true
}

func anyResourceMatchesField(resources []cloudtrailtypes.Resource, field, want string) bool {
	for _, r := range resources {
		switch field {
		case "type":
			if aws.ToString(r.ResourceType) == want {
				return true
			}
		case "name":
			if aws.ToString(r.ResourceName) == want {
				return true
			}
		}
	}
	return false
}

func extractAccessKeyIdFromCTEvent(ev cloudtrailtypes.Event) string {
	if ev.CloudTrailEvent == nil {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(*ev.CloudTrailEvent), &parsed); err != nil {
		return ""
	}
	ui, _ := parsed["userIdentity"].(map[string]any)
	s, _ := ui["accessKeyId"].(string)
	return s
}

package demo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// registerMessagingHandlers registers SQS, SNS, EventBridge, Kinesis, SFN, and MSK handlers.
func registerMessagingHandlers(t *Transport) {
	registerSQSHandlers(t)
	registerSNSHandlers(t)
	registerEventBridgeHandlers(t)
	registerKinesisHandlers(t)
	registerSFNHandlers(t)
	registerMSKHandlers(t)
}

// ---------------------------------------------------------------------------
// SQS (awsjson10)
// ---------------------------------------------------------------------------

func registerSQSHandlers(t *Transport) {
	t.Handle("sqs", "ListQueues", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["sqs"]()

		queueURLs := make([]string, 0, len(resources))
		for _, r := range resources {
			if url, ok := r.Fields["queue_url"]; ok && url != "" {
				queueURLs = append(queueURLs, url)
			} else {
				queueURLs = append(queueURLs, fmt.Sprintf("https://sqs.us-east-1.amazonaws.com/123456789012/%s", r.ID))
			}
		}

		out := &sqs.ListQueuesOutput{
			QueueUrls: queueURLs,
		}
		return JSONResponse(out)
	})

	t.Handle("sqs", "GetQueueAttributes", func(_ *http.Request) (*http.Response, error) {
		// Return a generic set of attributes
		attrs := map[string]string{
			"ApproximateNumberOfMessages":           "0",
			"ApproximateNumberOfMessagesNotVisible":  "0",
			"DelaySeconds":                          "0",
			"VisibilityTimeout":                     "30",
			"MessageRetentionPeriod":                "345600",
			"MaximumMessageSize":                    "262144",
			"ReceiveMessageWaitTimeSeconds":          "0",
			"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:demo-queue",
		}

		out := &sqs.GetQueueAttributesOutput{
			Attributes: attrs,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// SNS (awsquery — XML)
// ---------------------------------------------------------------------------

func registerSNSHandlers(t *Transport) {
	t.Handle("sns", "ListTopics", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["sns"]()
		topics := ExtractSDK[snstypes.Topic](resources)

		var sb strings.Builder
		sb.WriteString(`<Topics>`)
		for _, topic := range topics {
			topicArn := aws.ToString(topic.TopicArn)
			fmt.Fprintf(&sb, `<member><TopicArn>%s</TopicArn></member>`, xmlEscape(topicArn))
		}
		sb.WriteString(`</Topics>`)

		body := awsQueryXML("ListTopics", "http://sns.amazonaws.com/doc/2010-03-31/", sb.String())
		return XMLResponse(body), nil
	})

	t.Handle("sns", "ListSubscriptions", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["sns-sub"]()
		subs := ExtractSDK[snstypes.Subscription](resources)

		var sb strings.Builder
		sb.WriteString(`<Subscriptions>`)
		for _, sub := range subs {
			topicArn := aws.ToString(sub.TopicArn)
			protocol := aws.ToString(sub.Protocol)
			endpoint := aws.ToString(sub.Endpoint)
			subArn := aws.ToString(sub.SubscriptionArn)
			owner := aws.ToString(sub.Owner)

			fmt.Fprintf(&sb, `<member>`)
			fmt.Fprintf(&sb, `<TopicArn>%s</TopicArn>`, xmlEscape(topicArn))
			fmt.Fprintf(&sb, `<Protocol>%s</Protocol>`, xmlEscape(protocol))
			fmt.Fprintf(&sb, `<Endpoint>%s</Endpoint>`, xmlEscape(endpoint))
			fmt.Fprintf(&sb, `<SubscriptionArn>%s</SubscriptionArn>`, xmlEscape(subArn))
			fmt.Fprintf(&sb, `<Owner>%s</Owner>`, xmlEscape(owner))
			fmt.Fprintf(&sb, `</member>`)
		}
		sb.WriteString(`</Subscriptions>`)

		body := awsQueryXML("ListSubscriptions", "http://sns.amazonaws.com/doc/2010-03-31/", sb.String())
		return XMLResponse(body), nil
	})
}

// ---------------------------------------------------------------------------
// EventBridge (awsjson11)
// ---------------------------------------------------------------------------

func registerEventBridgeHandlers(t *Transport) {
	t.Handle("events", "ListRules", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["eb-rule"]()
		rules := ExtractSDK[eventbridgetypes.Rule](resources)

		out := &eventbridge.ListRulesOutput{
			Rules: rules,
		}
		return JSONResponse(out)
	})
}

// ---------------------------------------------------------------------------
// Kinesis (awsjson11)
// The deserializer expects "StreamSummaries" (capital) but also "StreamNames" (capital).
// The test checks StreamNames, so we must populate both.
// ---------------------------------------------------------------------------

func registerKinesisHandlers(t *Transport) {
	t.Handle("kinesis", "ListStreams", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["kinesis"]()
		summaries := ExtractSDK[kinesistypes.StreamSummary](resources)

		streamNames := make([]string, 0, len(summaries))
		for _, s := range summaries {
			if s.StreamName != nil {
				streamNames = append(streamNames, *s.StreamName)
			}
		}

		// Kinesis deserializer uses capital-case keys ("StreamSummaries", "StreamNames", "HasMoreStreams").
		return JSONResponse(map[string]interface{}{
			"StreamNames":     streamNames,
			"StreamSummaries": summaries,
			"HasMoreStreams":   false,
		})
	})
}

// ---------------------------------------------------------------------------
// Step Functions (awsjson10, service "states")
// The deserializer expects lowercase camelCase "stateMachines".
// ---------------------------------------------------------------------------

func registerSFNHandlers(t *Transport) {
	t.Handle("states", "ListStateMachines", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["sfn"]()
		sms := ExtractSDK[sfntypes.StateMachineListItem](resources)

		smMaps := make([]map[string]interface{}, 0, len(sms))
		for _, sm := range sms {
			m := map[string]interface{}{
				"name":            aws.ToString(sm.Name),
				"stateMachineArn": aws.ToString(sm.StateMachineArn),
				"type":            string(sm.Type),
			}
			if sm.CreationDate != nil {
				m["creationDate"] = sm.CreationDate.Unix()
			}
			smMaps = append(smMaps, m)
		}

		return JSONResponse(map[string]interface{}{"stateMachines": smMaps})
	})
}

// ---------------------------------------------------------------------------
// MSK (restjson1 — routed by URL path, service "kafka")
// The deserializer expects "clusterInfoList" (lowercase camelCase).
// ---------------------------------------------------------------------------

func registerMSKHandlers(t *Transport) {
	t.Handle("kafka", "ListClustersV2", func(_ *http.Request) (*http.Response, error) {
		resources := demoData["msk"]()
		clusters := ExtractSDK[kafkatypes.Cluster](resources)

		clusterMaps := make([]map[string]interface{}, 0, len(clusters))
		for _, c := range clusters {
			m := map[string]interface{}{
				"clusterName": aws.ToString(c.ClusterName),
				"clusterArn":  aws.ToString(c.ClusterArn),
				"state":       string(c.State),
				"clusterType": string(c.ClusterType),
			}
			clusterMaps = append(clusterMaps, m)
		}

		return JSONResponse(map[string]interface{}{"clusterInfoList": clusterMaps})
	})
}

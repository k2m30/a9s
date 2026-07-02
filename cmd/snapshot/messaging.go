package main

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// ---------------------------------------------------------------------
// sns — SNS Topics
// ---------------------------------------------------------------------

type snsData struct {
	Topics []snsTopic `json:"topics"`
}

type snsTopic struct {
	TopicArn   string            `json:"topic_arn"`
	Attributes snsTopicAttrs     `json:"attributes"`
	RawAttrs   map[string]string `json:"raw_attributes,omitempty"`
}

// snsTopicAttrs captures the GetTopicAttributes outcome for a topic.
//
//	Outcome "ok":    attribute map populated.
//	Outcome "error": call failed.
type snsTopicAttrs struct {
	Outcome                string `json:"outcome"`
	ErrorCode              string `json:"error_code,omitempty"`
	SubscriptionsConfirmed string `json:"subscriptions_confirmed,omitempty"`
	SubscriptionsPending   string `json:"subscriptions_pending,omitempty"`
	KmsMasterKeyId         string `json:"kms_master_key_id,omitempty"`
	Policy                 string `json:"policy,omitempty"`
}

func captureSNS(ctx context.Context, cfg aws.Config) (any, error) {
	client := sns.NewFromConfig(cfg)

	var topics []snsTopic
	var nextToken *string
	for {
		out, err := client.ListTopics(ctx, &sns.ListTopicsInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, t := range out.Topics {
			topics = append(topics, snsTopic{TopicArn: aws.ToString(t.TopicArn)})
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	for i := range topics {
		topics[i].Attributes, topics[i].RawAttrs = captureSNSTopicAttributes(ctx, client, topics[i].TopicArn)
	}

	return snsData{Topics: topics}, nil
}

func captureSNSTopicAttributes(ctx context.Context, client *sns.Client, topicArn string) (snsTopicAttrs, map[string]string) {
	out, err := client.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: aws.String(topicArn)})
	if err != nil {
		return snsTopicAttrs{Outcome: "error", ErrorCode: err.Error()}, nil
	}
	attrs := snsTopicAttrs{
		Outcome:                "ok",
		SubscriptionsConfirmed: out.Attributes["SubscriptionsConfirmed"],
		SubscriptionsPending:   out.Attributes["SubscriptionsPending"],
		KmsMasterKeyId:         out.Attributes["KmsMasterKeyId"],
		Policy:                 out.Attributes["Policy"],
	}
	return attrs, out.Attributes
}

// ---------------------------------------------------------------------
// sns-sub — SNS Subscriptions
// ---------------------------------------------------------------------

type snsSubData struct {
	Subscriptions []snsSubscription `json:"subscriptions"`
}

type snsSubscription struct {
	SubscriptionArn string `json:"subscription_arn"`
	TopicArn        string `json:"topic_arn"`
	Protocol        string `json:"protocol"`
	Endpoint        string `json:"endpoint"`
	Owner           string `json:"owner"`
}

func captureSNSSub(ctx context.Context, cfg aws.Config) (any, error) {
	client := sns.NewFromConfig(cfg)

	var subs []snsSubscription
	var nextToken *string
	for {
		out, err := client.ListSubscriptions(ctx, &sns.ListSubscriptionsInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, s := range out.Subscriptions {
			subs = append(subs, snsSubscription{
				SubscriptionArn: aws.ToString(s.SubscriptionArn),
				TopicArn:        aws.ToString(s.TopicArn),
				Protocol:        aws.ToString(s.Protocol),
				Endpoint:        aws.ToString(s.Endpoint),
				Owner:           aws.ToString(s.Owner),
			})
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	return snsSubData{Subscriptions: subs}, nil
}

// ---------------------------------------------------------------------
// sqs — SQS Queues
// ---------------------------------------------------------------------

type sqsData struct {
	Queues []sqsQueue `json:"queues"`
}

type sqsQueue struct {
	QueueUrl   string            `json:"queue_url"`
	Attributes sqsQueueAttrs     `json:"attributes"`
	RawAttrs   map[string]string `json:"raw_attributes,omitempty"`
}

// sqsQueueAttrs captures the GetQueueAttributes(AttributeNames=[All]) outcome.
//
//	Outcome "ok":    attribute map populated.
//	Outcome "error": call failed.
type sqsQueueAttrs struct {
	Outcome                       string `json:"outcome"`
	ErrorCode                     string `json:"error_code,omitempty"`
	QueueArn                      string `json:"queue_arn,omitempty"`
	ApproximateNumberOfMessages   string `json:"approximate_number_of_messages,omitempty"`
	ApproximateAgeOfOldestMessage string `json:"approximate_age_of_oldest_message,omitempty"`
	VisibilityTimeout             string `json:"visibility_timeout,omitempty"`
	RedrivePolicy                 string `json:"redrive_policy,omitempty"`
	RedriveAllowPolicy            string `json:"redrive_allow_policy,omitempty"`
	KmsMasterKeyId                string `json:"kms_master_key_id,omitempty"`
	Policy                        string `json:"policy,omitempty"`
}

func captureSQS(ctx context.Context, cfg aws.Config) (any, error) {
	client := sqs.NewFromConfig(cfg)

	var queues []sqsQueue
	var nextToken *string
	for {
		out, err := client.ListQueues(ctx, &sqs.ListQueuesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, url := range out.QueueUrls {
			queues = append(queues, sqsQueue{QueueUrl: url})
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	for i := range queues {
		queues[i].Attributes, queues[i].RawAttrs = captureSQSQueueAttributes(ctx, client, queues[i].QueueUrl)
	}

	return sqsData{Queues: queues}, nil
}

func captureSQSQueueAttributes(ctx context.Context, client *sqs.Client, queueURL string) (sqsQueueAttrs, map[string]string) {
	out, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameAll},
	})
	if err != nil {
		return sqsQueueAttrs{Outcome: "error", ErrorCode: err.Error()}, nil
	}
	attrs := sqsQueueAttrs{
		Outcome:                       "ok",
		QueueArn:                      out.Attributes["QueueArn"],
		ApproximateNumberOfMessages:   out.Attributes["ApproximateNumberOfMessages"],
		ApproximateAgeOfOldestMessage: out.Attributes["ApproximateAgeOfOldestMessage"],
		VisibilityTimeout:             out.Attributes["VisibilityTimeout"],
		RedrivePolicy:                 out.Attributes["RedrivePolicy"],
		RedriveAllowPolicy:            out.Attributes["RedriveAllowPolicy"],
		KmsMasterKeyId:                out.Attributes["KmsMasterKeyId"],
		Policy:                        out.Attributes["Policy"],
	}
	return attrs, out.Attributes
}

// ---------------------------------------------------------------------
// eb-rule — EventBridge Rules
// ---------------------------------------------------------------------

type ebRuleData struct {
	Rules []ebRule `json:"rules"`
}

type ebRule struct {
	Name               string        `json:"name"`
	Arn                string        `json:"arn"`
	State              string        `json:"state"`
	EventBusName       string        `json:"event_bus_name,omitempty"`
	ScheduleExpression string        `json:"schedule_expression,omitempty"`
	RoleArn            string        `json:"role_arn,omitempty"`
	Description        string        `json:"description,omitempty"`
	Targets            ebRuleTargets `json:"targets"`
}

// ebRuleTargets captures the ListTargetsByRule outcome for a rule.
//
//	Outcome "ok":    target list populated (possibly empty).
//	Outcome "error": call failed.
type ebRuleTargets struct {
	Outcome   string         `json:"outcome"`
	ErrorCode string         `json:"error_code,omitempty"`
	Items     []ebRuleTarget `json:"items,omitempty"`
}

type ebRuleTarget struct {
	Arn                 string `json:"arn"`
	RoleArn             string `json:"role_arn,omitempty"`
	DeadLetterConfigArn string `json:"dead_letter_config_arn,omitempty"`
}

func captureEBRule(ctx context.Context, cfg aws.Config) (any, error) {
	client := eventbridge.NewFromConfig(cfg)

	var rules []ebRule
	var nextToken *string
	for {
		out, err := client.ListRules(ctx, &eventbridge.ListRulesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, r := range out.Rules {
			rules = append(rules, ebRule{
				Name:               aws.ToString(r.Name),
				Arn:                aws.ToString(r.Arn),
				State:              string(r.State),
				EventBusName:       aws.ToString(r.EventBusName),
				ScheduleExpression: aws.ToString(r.ScheduleExpression),
				RoleArn:            aws.ToString(r.RoleArn),
				Description:        aws.ToString(r.Description),
			})
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	for i := range rules {
		rules[i].Targets = captureEBRuleTargets(ctx, client, rules[i].Name, rules[i].EventBusName)
	}

	return ebRuleData{Rules: rules}, nil
}

func captureEBRuleTargets(ctx context.Context, client *eventbridge.Client, ruleName, eventBusName string) ebRuleTargets {
	in := &eventbridge.ListTargetsByRuleInput{Rule: aws.String(ruleName)}
	if eventBusName != "" {
		in.EventBusName = aws.String(eventBusName)
	}

	var items []ebRuleTarget
	var nextToken *string
	for {
		in.NextToken = nextToken
		out, err := client.ListTargetsByRule(ctx, in)
		if err != nil {
			return ebRuleTargets{Outcome: "error", ErrorCode: err.Error()}
		}
		for _, t := range out.Targets {
			items = append(items, ebRuleTarget{
				Arn:                 aws.ToString(t.Arn),
				RoleArn:             aws.ToString(t.RoleArn),
				DeadLetterConfigArn: deadLetterArn(t.DeadLetterConfig),
			})
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	return ebRuleTargets{Outcome: "ok", Items: items}
}

func deadLetterArn(dlq *ebtypes.DeadLetterConfig) string {
	if dlq == nil {
		return ""
	}
	return aws.ToString(dlq.Arn)
}

// ---------------------------------------------------------------------
// kinesis — Kinesis Streams
// ---------------------------------------------------------------------

type kinesisData struct {
	Streams []kinesisStream `json:"streams"`
}

type kinesisStream struct {
	StreamARN               string               `json:"stream_arn"`
	StreamName              string               `json:"stream_name"`
	StreamStatus            string               `json:"stream_status"`
	StreamCreationTimestamp string               `json:"stream_creation_timestamp,omitempty"`
	Summary                 kinesisStreamSummary `json:"describe_stream_summary"`
}

// kinesisStreamSummary captures the DescribeStreamSummary outcome for a stream.
//
//	Outcome "ok":    KeyId/EncryptionType populated.
//	Outcome "error": call failed.
type kinesisStreamSummary struct {
	Outcome        string `json:"outcome"`
	ErrorCode      string `json:"error_code,omitempty"`
	KeyId          string `json:"key_id,omitempty"`
	EncryptionType string `json:"encryption_type,omitempty"`
}

func captureKinesis(ctx context.Context, cfg aws.Config) (any, error) {
	client := kinesis.NewFromConfig(cfg)

	var streams []kinesisStream
	var exclusiveStartStreamName *string
	for {
		in := &kinesis.ListStreamsInput{}
		if exclusiveStartStreamName != nil {
			in.ExclusiveStartStreamName = exclusiveStartStreamName
		}
		out, err := client.ListStreams(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, s := range out.StreamSummaries {
			streams = append(streams, kinesisStream{
				StreamARN:               aws.ToString(s.StreamARN),
				StreamName:              aws.ToString(s.StreamName),
				StreamStatus:            string(s.StreamStatus),
				StreamCreationTimestamp: msgFormatTime(s.StreamCreationTimestamp),
			})
		}
		if out.HasMoreStreams == nil || !*out.HasMoreStreams || len(out.StreamSummaries) == 0 {
			break
		}
		last := out.StreamSummaries[len(out.StreamSummaries)-1].StreamName
		exclusiveStartStreamName = last
	}

	for i := range streams {
		streams[i].Summary = captureKinesisStreamSummary(ctx, client, streams[i].StreamName)
	}

	return kinesisData{Streams: streams}, nil
}

func captureKinesisStreamSummary(ctx context.Context, client *kinesis.Client, streamName string) kinesisStreamSummary {
	out, err := client.DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{StreamName: aws.String(streamName)})
	if err != nil {
		return kinesisStreamSummary{Outcome: "error", ErrorCode: err.Error()}
	}
	if out.StreamDescriptionSummary == nil {
		return kinesisStreamSummary{Outcome: "ok"}
	}
	return kinesisStreamSummary{
		Outcome:        "ok",
		KeyId:          aws.ToString(out.StreamDescriptionSummary.KeyId),
		EncryptionType: string(out.StreamDescriptionSummary.EncryptionType),
	}
}

// ---------------------------------------------------------------------
// msk — MSK Clusters
// ---------------------------------------------------------------------

type mskData struct {
	Clusters []mskCluster `json:"clusters"`
}

type mskCluster struct {
	ClusterArn     string       `json:"cluster_arn"`
	ClusterName    string       `json:"cluster_name"`
	State          string       `json:"state"`
	StateInfo      mskStateInfo `json:"state_info,omitzero"`
	ClusterType    string       `json:"cluster_type,omitempty"`
	CreationTime   string       `json:"creation_time,omitempty"`
	CurrentVersion string       `json:"current_version,omitempty"`
}

type mskStateInfo struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func captureMSK(ctx context.Context, cfg aws.Config) (any, error) {
	client := kafka.NewFromConfig(cfg)

	var clusters []mskCluster
	var nextToken *string
	for {
		out, err := client.ListClustersV2(ctx, &kafka.ListClustersV2Input{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, c := range out.ClusterInfoList {
			mc := mskCluster{
				ClusterArn:     aws.ToString(c.ClusterArn),
				ClusterName:    aws.ToString(c.ClusterName),
				State:          string(c.State),
				ClusterType:    string(c.ClusterType),
				CreationTime:   msgFormatTime(c.CreationTime),
				CurrentVersion: aws.ToString(c.CurrentVersion),
			}
			if c.StateInfo != nil {
				mc.StateInfo = mskStateInfo{
					Code:    aws.ToString(c.StateInfo.Code),
					Message: aws.ToString(c.StateInfo.Message),
				}
			}
			clusters = append(clusters, mc)
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	return mskData{Clusters: clusters}, nil
}

// ---------------------------------------------------------------------
// ses — SES Identities
// ---------------------------------------------------------------------

type sesData struct {
	Identities []sesIdentity `json:"identities"`
	Account    sesAccount    `json:"account"`
}

type sesIdentity struct {
	IdentityName       string `json:"identity_name"`
	IdentityType       string `json:"identity_type"`
	SendingEnabled     bool   `json:"sending_enabled"`
	VerificationStatus string `json:"verification_status"`
}

// sesAccount captures the account-wide GetAccount outcome (SESv2), one call.
//
//	Outcome "ok":    fields populated.
//	Outcome "error": call failed.
type sesAccount struct {
	Outcome           string  `json:"outcome"`
	ErrorCode         string  `json:"error_code,omitempty"`
	EnforcementStatus string  `json:"enforcement_status,omitempty"`
	Max24HourSend     float64 `json:"max_24_hour_send,omitempty"`
	SentLast24Hours   float64 `json:"sent_last_24_hours,omitempty"`
}

func captureSES(ctx context.Context, cfg aws.Config) (any, error) {
	client := sesv2.NewFromConfig(cfg)

	var identities []sesIdentity
	var nextToken *string
	for {
		out, err := client.ListEmailIdentities(ctx, &sesv2.ListEmailIdentitiesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, id := range out.EmailIdentities {
			identities = append(identities, sesIdentity{
				IdentityName:       aws.ToString(id.IdentityName),
				IdentityType:       string(id.IdentityType),
				SendingEnabled:     id.SendingEnabled,
				VerificationStatus: string(id.VerificationStatus),
			})
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	account := captureSESAccount(ctx, client)

	return sesData{Identities: identities, Account: account}, nil
}

func captureSESAccount(ctx context.Context, client *sesv2.Client) sesAccount {
	out, err := client.GetAccount(ctx, &sesv2.GetAccountInput{})
	if err != nil {
		return sesAccount{Outcome: "error", ErrorCode: err.Error()}
	}
	acct := sesAccount{
		Outcome:           "ok",
		EnforcementStatus: aws.ToString(out.EnforcementStatus),
	}
	if out.SendQuota != nil {
		acct.Max24HourSend = out.SendQuota.Max24HourSend
		acct.SentLast24Hours = out.SendQuota.SentLast24Hours
	}
	return acct
}

// ---------------------------------------------------------------------
// sfn — Step Functions
// ---------------------------------------------------------------------

type sfnData struct {
	StateMachines []sfnStateMachine `json:"state_machines"`
}

type sfnStateMachine struct {
	Name            string            `json:"name"`
	StateMachineArn string            `json:"state_machine_arn"`
	Type            string            `json:"type"`
	CreationDate    string            `json:"creation_date,omitempty"`
	LastFailed      sfnExecutionProbe `json:"last_failed_execution"`
	LastSucceeded   sfnExecutionProbe `json:"last_succeeded_execution"`
}

// sfnExecutionProbe captures a single ListExecutions(statusFilter=X, maxResults=1)
// outcome — the most recent execution in that status, used by the checklist generator to
// detect a failure loop (most recent FAILED newer than most recent SUCCEEDED).
//
//	Outcome "found":    an execution in that status exists; fields populated.
//	Outcome "none":     no execution in that status.
//	Outcome "error":    call failed.
type sfnExecutionProbe struct {
	Outcome      string `json:"outcome"`
	ErrorCode    string `json:"error_code,omitempty"`
	ExecutionArn string `json:"execution_arn,omitempty"`
	StartDate    string `json:"start_date,omitempty"`
	StopDate     string `json:"stop_date,omitempty"`
}

func captureSFN(ctx context.Context, cfg aws.Config) (any, error) {
	client := sfn.NewFromConfig(cfg)

	var machines []sfnStateMachine
	var nextToken *string
	for {
		out, err := client.ListStateMachines(ctx, &sfn.ListStateMachinesInput{NextToken: nextToken})
		if err != nil {
			return nil, err
		}
		for _, m := range out.StateMachines {
			machines = append(machines, sfnStateMachine{
				Name:            aws.ToString(m.Name),
				StateMachineArn: aws.ToString(m.StateMachineArn),
				Type:            string(m.Type),
				CreationDate:    msgFormatTime(m.CreationDate),
			})
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		nextToken = out.NextToken
	}

	for i := range machines {
		machines[i].LastFailed = captureSFNLastExecution(ctx, client, machines[i].StateMachineArn, sfntypes.ExecutionStatusFailed)
		machines[i].LastSucceeded = captureSFNLastExecution(ctx, client, machines[i].StateMachineArn, sfntypes.ExecutionStatusSucceeded)
	}

	return sfnData{StateMachines: machines}, nil
}

func captureSFNLastExecution(ctx context.Context, client *sfn.Client, stateMachineArn string, status sfntypes.ExecutionStatus) sfnExecutionProbe {
	out, err := client.ListExecutions(ctx, &sfn.ListExecutionsInput{
		StateMachineArn: aws.String(stateMachineArn),
		StatusFilter:    status,
		MaxResults:      1,
	})
	if err != nil {
		return sfnExecutionProbe{Outcome: "error", ErrorCode: err.Error()}
	}
	if len(out.Executions) == 0 {
		return sfnExecutionProbe{Outcome: "none"}
	}
	e := out.Executions[0]
	return sfnExecutionProbe{
		Outcome:      "found",
		ExecutionArn: aws.ToString(e.ExecutionArn),
		StartDate:    msgFormatTime(e.StartDate),
		StopDate:     msgFormatTime(e.StopDate),
	}
}

// ---------------------------------------------------------------------
// apigw — API Gateways (HTTP/WebSocket v2 + REST v1)
// ---------------------------------------------------------------------

type apigwData struct {
	V2APIs []apigwV2API `json:"v2_apis"`
	V1APIs []apigwV1API `json:"v1_apis"`
}

type apigwV2API struct {
	ApiId        string        `json:"api_id"`
	Name         string        `json:"name"`
	ProtocolType string        `json:"protocol_type"`
	CreatedDate  string        `json:"created_date,omitempty"`
	Stages       apigwV2Stages `json:"stages"`
}

// apigwV2Stages captures the GetStages outcome for one HTTP/WebSocket API.
//
//	Outcome "ok":    stage list populated (possibly empty — "no deployed stage").
//	Outcome "error": call failed.
type apigwV2Stages struct {
	Outcome   string         `json:"outcome"`
	ErrorCode string         `json:"error_code,omitempty"`
	Items     []apigwV2Stage `json:"items,omitempty"`
}

type apigwV2Stage struct {
	StageName    string `json:"stage_name"`
	DeploymentId string `json:"deployment_id,omitempty"`
}

type apigwV1API struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	CreatedDate string `json:"created_date,omitempty"`
}

func captureAPIGW(ctx context.Context, cfg aws.Config) (any, error) {
	v2Client := apigatewayv2.NewFromConfig(cfg)
	v1Client := apigateway.NewFromConfig(cfg)

	var v2APIs []apigwV2API
	var v2NextToken *string
	for {
		out, err := v2Client.GetApis(ctx, &apigatewayv2.GetApisInput{NextToken: v2NextToken})
		if err != nil {
			return nil, err
		}
		for _, a := range out.Items {
			v2APIs = append(v2APIs, apigwV2API{
				ApiId:        aws.ToString(a.ApiId),
				Name:         aws.ToString(a.Name),
				ProtocolType: string(a.ProtocolType),
				CreatedDate:  msgFormatTime(a.CreatedDate),
			})
		}
		if out.NextToken == nil || *out.NextToken == "" {
			break
		}
		v2NextToken = out.NextToken
	}

	for i := range v2APIs {
		v2APIs[i].Stages = captureAPIGWStages(ctx, v2Client, v2APIs[i].ApiId)
	}

	var v1APIs []apigwV1API
	var position *string
	for {
		out, err := v1Client.GetRestApis(ctx, &apigateway.GetRestApisInput{Position: position})
		if err != nil {
			return nil, err
		}
		for _, a := range out.Items {
			v1APIs = append(v1APIs, apigwV1API{
				Id:          aws.ToString(a.Id),
				Name:        aws.ToString(a.Name),
				CreatedDate: msgFormatTime(a.CreatedDate),
			})
		}
		if out.Position == nil || *out.Position == "" {
			break
		}
		position = out.Position
	}

	return apigwData{V2APIs: v2APIs, V1APIs: v1APIs}, nil
}

func captureAPIGWStages(ctx context.Context, client *apigatewayv2.Client, apiId string) apigwV2Stages {
	out, err := client.GetStages(ctx, &apigatewayv2.GetStagesInput{ApiId: aws.String(apiId)})
	if err != nil {
		return apigwV2Stages{Outcome: "error", ErrorCode: err.Error()}
	}
	items := make([]apigwV2Stage, 0, len(out.Items))
	for _, s := range out.Items {
		items = append(items, apigwV2Stage{
			StageName:    aws.ToString(s.StageName),
			DeploymentId: aws.ToString(s.DeploymentId),
		})
	}
	return apigwV2Stages{Outcome: "ok", Items: items}
}

// ---------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------

func msgFormatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

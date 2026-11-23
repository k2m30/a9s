package main

import (
	"cmp"
	"context"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	v2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
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
	paginator := sns.NewListTopicsPaginator(client, &sns.ListTopicsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, t := range out.Topics {
			topics = append(topics, snsTopic{TopicArn: aws.ToString(t.TopicArn)})
		}
	}

	for i := range topics {
		topics[i].Attributes, topics[i].RawAttrs = captureSNSTopicAttributes(ctx, client, topics[i].TopicArn)
	}

	return snsData{Topics: topics}, nil
}

func captureSNSTopicAttributes(ctx context.Context, client *sns.Client, topicArn string) (snsTopicAttrs, map[string]string) {
	out, err := client.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: aws.String(topicArn)})
	if err != nil {
		o, code := outcomeOf(err)
		return snsTopicAttrs{Outcome: o, ErrorCode: code}, nil
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

// An HTTP(S) endpoint's path and query are routinely the webhook's shared
// secret and its userinfo is SNS's basic-auth form, so only scheme and host
// are kept; an unparseable URL keeps nothing. The other protocols' endpoints
// are ARNs, email addresses and phone numbers.
func snsSubEndpointOf(protocol, endpoint string) string {
	if protocol != "http" && protocol != "https" {
		return endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func captureSNSSub(ctx context.Context, cfg aws.Config) (any, error) {
	client := sns.NewFromConfig(cfg)

	var subs []snsSubscription
	paginator := sns.NewListSubscriptionsPaginator(client, &sns.ListSubscriptionsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range out.Subscriptions {
			subs = append(subs, snsSubscription{
				SubscriptionArn: aws.ToString(s.SubscriptionArn),
				TopicArn:        aws.ToString(s.TopicArn),
				Protocol:        aws.ToString(s.Protocol),
				Endpoint:        snsSubEndpointOf(aws.ToString(s.Protocol), aws.ToString(s.Endpoint)),
				Owner:           aws.ToString(s.Owner),
			})
		}
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

	var urls []string
	paginator := sqs.NewListQueuesPaginator(client, &sqs.ListQueuesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		urls = append(urls, out.QueueUrls...)
	}

	rows := describeListed(urls, 1, func(batch []string) (map[string]map[string]string, []string, error) {
		out, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String(batch[0]),
			AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameAll},
		})
		if err != nil {
			return nil, nil, err
		}
		if len(out.Attributes) == 0 {
			return nil, nil, nil
		}
		return map[string]map[string]string{batch[0]: out.Attributes}, nil, nil
	})

	queues := make([]sqsQueue, 0, len(rows))
	for _, row := range rows {
		q := sqsQueue{QueueUrl: row.id, Attributes: sqsQueueAttrs{Outcome: row.read.Outcome, ErrorCode: row.read.ErrorCode}}
		if row.item != nil {
			a := *row.item
			q.RawAttrs = a
			q.Attributes.QueueArn = a["QueueArn"]
			q.Attributes.ApproximateNumberOfMessages = a["ApproximateNumberOfMessages"]
			q.Attributes.ApproximateAgeOfOldestMessage = a["ApproximateAgeOfOldestMessage"]
			q.Attributes.VisibilityTimeout = a["VisibilityTimeout"]
			q.Attributes.RedrivePolicy = a["RedrivePolicy"]
			q.Attributes.RedriveAllowPolicy = a["RedriveAllowPolicy"]
			q.Attributes.KmsMasterKeyId = a["KmsMasterKeyId"]
			q.Attributes.Policy = a["Policy"]
		}
		queues = append(queues, q)
	}

	return sqsData{Queues: queues}, nil
}

// ---------------------------------------------------------------------
// eb-rule — EventBridge Rules
// ---------------------------------------------------------------------

type ebRuleData struct {
	Rules []ebRule `json:"rules"`
	partialRead
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

// ebRuleParts plans one part per event bus: ListRules answers for one bus,
// the default when none is named. An answer naming no bus has answered for
// nothing, since every account has a default one, so that bus is read.
func ebRuleParts(ctx context.Context, cfg aws.Config) ([]part[ebRuleData], error) {
	client := eventbridge.NewFromConfig(cfg)
	buses, err := pageAll(ctx, &eventbridge.ListEventBusesInput{},
		func(ctx context.Context, in *eventbridge.ListEventBusesInput) (*eventbridge.ListEventBusesOutput, error) {
			return client.ListEventBuses(ctx, in)
		},
		func(out *eventbridge.ListEventBusesOutput) []ebtypes.EventBus { return out.EventBuses }, allPages)
	var names []string
	for _, b := range buses {
		if name := aws.ToString(b.Name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		if err != nil {
			return nil, err
		}
		names = []string{""}
	}
	parts := make([]part[ebRuleData], 0, len(names))
	for _, bus := range names {
		parts = append(parts, part[ebRuleData]{name: cmp.Or(bus, "default"), run: func(ctx context.Context, cfg aws.Config, d *ebRuleData) error {
			in := &eventbridge.ListRulesInput{}
			if bus != "" {
				in.EventBusName = aws.String(bus)
			}
			rules, listErr := pageAll(ctx, in,
				func(ctx context.Context, in *eventbridge.ListRulesInput) (*eventbridge.ListRulesOutput, error) {
					return client.ListRules(ctx, in)
				},
				func(out *eventbridge.ListRulesOutput) []ebtypes.Rule { return out.Rules }, allPages)
			for _, r := range rules {
				d.Rules = append(d.Rules, ebRule{
					Name:               aws.ToString(r.Name),
					Arn:                aws.ToString(r.Arn),
					State:              string(r.State),
					EventBusName:       aws.ToString(r.EventBusName),
					ScheduleExpression: aws.ToString(r.ScheduleExpression),
					RoleArn:            aws.ToString(r.RoleArn),
					Description:        aws.ToString(r.Description),
				})
			}
			if isGone(listErr) {
				return nil
			}
			return listErr
		}})
	}
	return parts, err
}

func ebRuleTargetsFinish(ctx context.Context, cfg aws.Config, d *ebRuleData) error {
	client := eventbridge.NewFromConfig(cfg)
	for i := range d.Rules {
		d.Rules[i].Targets = captureEBRuleTargets(ctx, client, d.Rules[i].Name, d.Rules[i].EventBusName)
	}
	return nil
}

func captureEBRuleTargets(ctx context.Context, client *eventbridge.Client, ruleName, eventBusName string) ebRuleTargets {
	in := &eventbridge.ListTargetsByRuleInput{Rule: aws.String(ruleName)}
	if eventBusName != "" {
		in.EventBusName = aws.String(eventBusName)
	}

	targets, err := pageAll(ctx, in,
		func(ctx context.Context, in *eventbridge.ListTargetsByRuleInput) (*eventbridge.ListTargetsByRuleOutput, error) {
			return client.ListTargetsByRule(ctx, in)
		},
		func(out *eventbridge.ListTargetsByRuleOutput) []ebtypes.Target { return out.Targets }, allPages)
	result := ebRuleTargets{}
	result.Outcome, result.ErrorCode = outcomeOf(err)
	for _, t := range targets {
		result.Items = append(result.Items, ebRuleTarget{
			Arn:                 aws.ToString(t.Arn),
			RoleArn:             aws.ToString(t.RoleArn),
			DeadLetterConfigArn: deadLetterArn(t.DeadLetterConfig),
		})
	}
	return result
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
	paginator := kinesis.NewListStreamsPaginator(client, &kinesis.ListStreamsInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range out.StreamSummaries {
			streams = append(streams, kinesisStream{
				StreamARN:               aws.ToString(s.StreamARN),
				StreamName:              aws.ToString(s.StreamName),
				StreamStatus:            string(s.StreamStatus),
				StreamCreationTimestamp: snapFormatTime(s.StreamCreationTimestamp),
			})
		}
	}

	for i := range streams {
		streams[i].Summary = captureKinesisStreamSummary(ctx, client, streams[i].StreamName)
	}

	return kinesisData{Streams: streams}, nil
}

func captureKinesisStreamSummary(ctx context.Context, client *kinesis.Client, streamName string) kinesisStreamSummary {
	got, read := readOne(func() (*kinesistypes.StreamDescriptionSummary, error) {
		out, err := client.DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{StreamName: aws.String(streamName)})
		if err != nil {
			return nil, err
		}
		return out.StreamDescriptionSummary, nil
	})
	summary := kinesisStreamSummary{Outcome: read.Outcome, ErrorCode: read.ErrorCode}
	if got != nil {
		summary.KeyId = aws.ToString(got.KeyId)
		summary.EncryptionType = string(got.EncryptionType)
	}
	return summary
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
	paginator := kafka.NewListClustersV2Paginator(client, &kafka.ListClustersV2Input{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, c := range out.ClusterInfoList {
			mc := mskCluster{
				ClusterArn:     aws.ToString(c.ClusterArn),
				ClusterName:    aws.ToString(c.ClusterName),
				State:          string(c.State),
				ClusterType:    string(c.ClusterType),
				CreationTime:   snapFormatTime(c.CreationTime),
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
	paginator := sesv2.NewListEmailIdentitiesPaginator(client, &sesv2.ListEmailIdentitiesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
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
	}

	account := captureSESAccount(ctx, client)

	return sesData{Identities: identities, Account: account}, nil
}

func captureSESAccount(ctx context.Context, client *sesv2.Client) sesAccount {
	out, err := client.GetAccount(ctx, &sesv2.GetAccountInput{})
	if err != nil {
		o, code := outcomeOf(err)
		return sesAccount{Outcome: o, ErrorCode: code}
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
	Name            string `json:"name"`
	StateMachineArn string `json:"state_machine_arn"`
	Type            string `json:"type"`
	CreationDate    string `json:"creation_date,omitempty"`
	// LastExecution is absent for an EXPRESS machine: Step Functions does not
	// list its executions (ListExecutions answers StateMachineTypeNotSupported).
	LastExecution *sfnExecutionProbe `json:"last_execution,omitempty"`
}

// sfnExecutionProbe is the ListExecutions(maxResults=1) answer: the newest
// execution, whatever its status.
//
//	Outcome "found":    an execution exists; fields populated.
//	Outcome "none":     the machine never ran.
//	Outcome "error":    call failed.
type sfnExecutionProbe struct {
	Outcome      string `json:"outcome"`
	ErrorCode    string `json:"error_code,omitempty"`
	ExecutionArn string `json:"execution_arn,omitempty"`
	Status       string `json:"status,omitempty"`
	StartDate    string `json:"start_date,omitempty"`
	StopDate     string `json:"stop_date,omitempty"`
}

func captureSFN(ctx context.Context, cfg aws.Config) (any, error) {
	client := sfn.NewFromConfig(cfg)

	var machines []sfnStateMachine
	paginator := sfn.NewListStateMachinesPaginator(client, &sfn.ListStateMachinesInput{})
	for paginator.HasMorePages() {
		out, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, m := range out.StateMachines {
			machines = append(machines, sfnStateMachine{
				Name:            aws.ToString(m.Name),
				StateMachineArn: aws.ToString(m.StateMachineArn),
				Type:            string(m.Type),
				CreationDate:    snapFormatTime(m.CreationDate),
			})
		}
	}

	for i := range machines {
		if machines[i].Type != string(sfntypes.StateMachineTypeExpress) {
			machines[i].LastExecution = captureSFNLastExecution(ctx, client, machines[i].StateMachineArn)
		}
	}

	return sfnData{StateMachines: machines}, nil
}

func captureSFNLastExecution(ctx context.Context, client *sfn.Client, stateMachineArn string) *sfnExecutionProbe {
	executions, err := pageAll(ctx, &sfn.ListExecutionsInput{
		StateMachineArn: aws.String(stateMachineArn),
		MaxResults:      1,
	},
		func(ctx context.Context, in *sfn.ListExecutionsInput) (*sfn.ListExecutionsOutput, error) {
			return client.ListExecutions(ctx, in)
		},
		func(out *sfn.ListExecutionsOutput) []sfntypes.ExecutionListItem { return out.Executions }, onePage)
	if err != nil {
		probe := &sfnExecutionProbe{}
		probe.Outcome, probe.ErrorCode = outcomeOf(err)
		return probe
	}
	if len(executions) == 0 {
		return &sfnExecutionProbe{Outcome: "none"}
	}
	e := executions[0]
	return &sfnExecutionProbe{
		Outcome:      "found",
		ExecutionArn: aws.ToString(e.ExecutionArn),
		Status:       string(e.Status),
		StartDate:    snapFormatTime(e.StartDate),
		StopDate:     snapFormatTime(e.StopDate),
	}
}

// ---------------------------------------------------------------------
// apigw — API Gateways (HTTP/WebSocket v2 + REST v1)
// ---------------------------------------------------------------------

type apigwData struct {
	V2APIs []apigwV2API `json:"v2_apis"`
	V1APIs []apigwV1API `json:"v1_apis"`
	partialRead
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

func apigwRESTPart() part[apigwData] {
	return part[apigwData]{name: "rest", run: func(ctx context.Context, cfg aws.Config, d *apigwData) error {
		paginator := apigateway.NewGetRestApisPaginator(apigateway.NewFromConfig(cfg), &apigateway.GetRestApisInput{})
		for paginator.HasMorePages() {
			out, err := paginator.NextPage(ctx)
			if err != nil {
				return err
			}
			for _, a := range out.Items {
				d.V1APIs = append(d.V1APIs, apigwV1API{
					Id:          aws.ToString(a.Id),
					Name:        aws.ToString(a.Name),
					CreatedDate: snapFormatTime(a.CreatedDate),
				})
			}
		}
		return nil
	}}
}

func apigwHTTPPart() part[apigwData] {
	return part[apigwData]{name: "http", run: func(ctx context.Context, cfg aws.Config, d *apigwData) error {
		client := apigatewayv2.NewFromConfig(cfg)
		apis, err := pageAll(ctx, &apigatewayv2.GetApisInput{},
			func(ctx context.Context, in *apigatewayv2.GetApisInput) (*apigatewayv2.GetApisOutput, error) {
				return client.GetApis(ctx, in)
			},
			func(out *apigatewayv2.GetApisOutput) []v2types.Api { return out.Items }, allPages)
		for _, a := range apis {
			d.V2APIs = append(d.V2APIs, apigwV2API{
				ApiId:        aws.ToString(a.ApiId),
				Name:         aws.ToString(a.Name),
				ProtocolType: string(a.ProtocolType),
				CreatedDate:  snapFormatTime(a.CreatedDate),
			})
		}
		for i := range d.V2APIs {
			d.V2APIs[i].Stages = captureAPIGWStages(ctx, client, d.V2APIs[i].ApiId)
		}
		return err
	}}
}

func captureAPIGWStages(ctx context.Context, client *apigatewayv2.Client, apiId string) apigwV2Stages {
	stages, err := pageAll(ctx, &apigatewayv2.GetStagesInput{ApiId: aws.String(apiId)},
		func(ctx context.Context, in *apigatewayv2.GetStagesInput) (*apigatewayv2.GetStagesOutput, error) {
			return client.GetStages(ctx, in)
		},
		func(out *apigatewayv2.GetStagesOutput) []v2types.Stage { return out.Items }, allPages)
	result := apigwV2Stages{}
	result.Outcome, result.ErrorCode = outcomeOf(err)
	for _, s := range stages {
		result.Items = append(result.Items, apigwV2Stage{
			StageName:    aws.ToString(s.StageName),
			DeploymentId: aws.ToString(s.DeploymentId),
		})
	}
	return result
}

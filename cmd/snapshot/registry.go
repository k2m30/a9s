package main

// captures maps every a9s resource short name to its capture function.
// captureOrder fixes the execution order (and the stderr progress log);
// the JSON output itself is a map, so file order is alphabetical.
var captures = map[string]captureFunc{
	// compute & containers (compute.go)
	"ec2":      captureEC2,
	"ecs":      captureECS,
	"ecs-svc":  captureECSSvc,
	"ecs-task": captureECSTask,
	"lambda":   captureLambda,
	"asg":      captureASG,
	"ebs":      captureEBS,
	"ebs-snap": captureEBSSnap,
	"ami":      captureAMI,
	"eks":      captureEKS,
	"ng":       captureNG,
	"ecr":      captureECR,
	"eb":       captureEB,
	"elb":      captureELB,
	"tg":       captureTG,

	// networking (ec2_network.go)
	"vpc":    captureVPC,
	"subnet": captureSubnet,
	"sg":     captureSG,
	"eni":    captureENI,
	"eip":    captureEIP,
	"rtb":    captureRTB,
	"igw":    captureIGW,
	"nat":    captureNAT,
	"tgw":    captureTGW,
	"vpce":   captureVPCE,
	"r53":    captureR53,
	"cf":     captureCF,
	"apigw":  captureAPIGW,

	// databases & storage (databases.go, s3.go)
	"s3":         captureS3,
	"dbi":        captureDBI,
	"dbc":        captureDBC,
	"dbi-snap":   captureDBISnap,
	"dbc-snap":   captureDBCSnap,
	"ddb":        captureDDB,
	"redis":      captureRedis,
	"redshift":   captureRedshift,
	"efs":        captureEFS,
	"backup":     captureBackup,
	"athena":     captureAthena,
	"glue":       captureGlue,
	"opensearch": captureOpenSearch,

	// messaging & integration (messaging.go)
	"sns":     captureSNS,
	"sns-sub": captureSNSSub,
	"sqs":     captureSQS,
	"eb-rule": captureEBRule,
	"kinesis": captureKinesis,
	"msk":     captureMSK,
	"ses":     captureSES,
	"sfn":     captureSFN,

	// security & identity (security.go)
	"iam-user":  captureIAMUser,
	"iam-group": captureIAMGroup,
	"role":      captureRole,
	"policy":    capturePolicy,
	"kms":       captureKMS,
	"secrets":   captureSecrets,
	"acm":       captureACM,
	"waf":       captureWAF,
	"ssm":       captureSSM,

	// devops & monitoring (ops.go)
	"cb":           captureCB,
	"pipeline":     capturePipeline,
	"codeartifact": captureCodeArtifact,
	"cfn":          captureCFN,
	"alarm":        captureAlarm,
	"logs":         captureLogs,
	"trail":        captureTrail,
	"ct-events":    captureCTEvents,
}

var captureOrder = []string{
	"ec2", "ecs", "ecs-svc", "ecs-task", "lambda", "asg", "ebs", "ebs-snap", "ami",
	"eks", "ng", "ecr", "eb", "elb", "tg",
	"vpc", "subnet", "sg", "eni", "eip", "rtb", "igw", "nat", "tgw", "vpce",
	"r53", "cf", "apigw",
	"s3", "dbi", "dbc", "dbi-snap", "dbc-snap", "ddb", "redis", "redshift", "efs",
	"backup", "athena", "glue", "opensearch",
	"sns", "sns-sub", "sqs", "eb-rule", "kinesis", "msk", "ses", "sfn",
	"iam-user", "iam-group", "role", "policy", "kms", "secrets", "acm", "waf", "ssm",
	"cb", "pipeline", "codeartifact", "cfn", "alarm", "logs", "trail", "ct-events",
}

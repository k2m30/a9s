package demo

import (
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/session"
)

// DemoRegion is the synthetic region displayed in demo mode.
const DemoRegion = "us-east-1"

// DemoProfile is the synthetic profile displayed in demo mode.
const DemoProfile = "demo"

// NewServiceClients assembles a *awsclient.ServiceClients backed entirely by
// typed in-process fakes — no HTTP transport required.
// STS is the only service without a typed fake; it uses the demo transport so
// that availability probes (GetCallerIdentity) succeed without real AWS credentials.
func NewServiceClients() *awsclient.ServiceClients {
	clients := awsclient.CreateServiceClients(NewDemoAWSConfig())
	clients.EC2 = fakes.NewEC2()
	clients.S3 = fakes.NewS3()
	clients.RDS = fakes.NewRDS()
	clients.ElastiCache = fakes.NewElastiCache()
	clients.DocDB = fakes.NewDocDB()
	clients.EKS = fakes.NewEKS()
	clients.SecretsManager = fakes.NewSecrets()
	clients.Lambda = fakes.NewLambda()
	clients.CloudWatch = fakes.NewCloudWatch()
	clients.SNS = fakes.NewSNS()
	clients.SQS = fakes.NewSQS()
	clients.ELBv2 = fakes.NewELB()
	clients.ECS = fakes.NewECS()
	clients.CloudFormation = fakes.NewCFN()
	clients.IAM = fakes.NewIAM()
	clients.CloudWatchLogs = fakes.NewCWLogs()
	clients.SSM = fakes.NewSSM()
	clients.DynamoDB = fakes.NewDynamoDB()
	clients.ACM = fakes.NewACM()
	clients.AutoScaling = fakes.NewASG()
	clients.CloudFront = fakes.NewCloudFront()
	clients.Route53 = fakes.NewR53()
	clients.APIGatewayV2 = fakes.NewAPIGW()
	clients.APIGatewayV1 = fakes.NewAPIGWV1()
	clients.ECR = fakes.NewECR()
	clients.EFS = fakes.NewEFS()
	clients.EventBridge = fakes.NewEventBridge()
	clients.SFN = fakes.NewSFN()
	clients.CodePipeline = fakes.NewCodePipeline()
	clients.Kinesis = fakes.NewKinesis()
	clients.WAFv2 = fakes.NewWAF()
	clients.Glue = fakes.NewGlue()
	clients.ElasticBeanstalk = fakes.NewEB()
	clients.SES = fakes.NewSESV1()
	clients.SESv2 = fakes.NewSES()
	clients.Redshift = fakes.NewRedshift()
	clients.CloudTrail = fakes.NewCloudTrail()
	clients.Athena = fakes.NewAthena()
	clients.CodeArtifact = fakes.NewCodeArtifact()
	clients.CodeBuild = fakes.NewCodeBuild()
	clients.OpenSearch = fakes.NewOpenSearch()
	clients.KMS = fakes.NewKMS()
	clients.MSK = fakes.NewMSK()
	clients.Backup = fakes.NewBackup()
	clients.IAMPolicies = session.NewPolicyStore()
	clients.IdentityStore = session.NewIdentityStore()
	return clients
}

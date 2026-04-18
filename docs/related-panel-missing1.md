# Related-Panel Missing Checkers

**97 registrations with deleted function bodies** across 31 parent resource types.

Source of truth: `docs/related-resources.md`. Each row is a `{parent} → {target}` relationship declared in the golden doc and registered in code, but the checker function body was deleted because it was a stub (`_ _ _ _` signature, ignored all inputs).

| Parent | Target | Stub function name | Registration file |
|--------|--------|---------------------|--------------------|
| `alarm` | `role` | `checkAlarmRole` | `internal/aws/cloudwatch.go` |
| `backup` | `eb-rule` | `checkBackupEBRule` | `internal/aws/backup_related.go` |
| `backup` | `logs` | `checkBackupLogs` | `internal/aws/backup_related.go` |
| `cb` | `pipeline` | `checkCbPipeline` | `internal/aws/codebuild.go` |
| `codeartifact` | `acm` | `checkCodeartifactAcm` | `internal/aws/codeartifact.go` |
| `codeartifact` | `cb` | `checkCodeartifactCB` | `internal/aws/codeartifact.go` |
| `codeartifact` | `kinesis` | `checkCodeartifactKinesis` | `internal/aws/codeartifact.go` |
| `codeartifact` | `kms` | `checkCodeartifactKMS` | `internal/aws/codeartifact.go` |
| `codeartifact` | `lambda` | `checkCodeartifactLambda` | `internal/aws/codeartifact.go` |
| `codeartifact` | `logs` | `checkCodeartifactLogs` | `internal/aws/codeartifact.go` |
| `codeartifact` | `r53` | `checkCodeartifactR53` | `internal/aws/codeartifact.go` |
| `codeartifact` | `role` | `checkCodeartifactRole` | `internal/aws/codeartifact.go` |
| `codeartifact` | `waf` | `checkCodeartifactWaf` | `internal/aws/codeartifact.go` |
| `ddb` | `backup` | `checkDdbBackup` | `internal/aws/dynamodb.go` |
| `ddb` | `kinesis` | `checkDdbKinesis` | `internal/aws/dynamodb.go` |
| `ddb` | `secrets` | `checkDdbSecrets` | `internal/aws/dynamodb.go` |
| `ddb` | `sns` | `checkDdbSNS` | `internal/aws/dynamodb.go` |
| `docdb-snap` | `backup` | `checkDocdbSnapBackup` | `internal/aws/docdb_snapshots.go` |
| `eb` | `elb` | `checkEbELB` | `internal/aws/eb_related.go` |
| `eb` | `role` | `checkEbRole` | `internal/aws/eb_related.go` |
| `eb` | `s3` | `checkEbS3` | `internal/aws/eb_related.go` |
| `eb` | `sg` | `checkEbSG` | `internal/aws/eb_related.go` |
| `eb` | `tg` | `checkEbTG` | `internal/aws/eb_related.go` |
| `ec2` | `ssm` | `checkEC2SSM` | `internal/aws/ec2.go` |
| `ecr` | `eb-rule` | `checkECREbRule` | `internal/aws/ecr.go` |
| `ecr` | `ecs` | `checkECRECS` | `internal/aws/ecr.go` |
| `ecr` | `eks` | `checkECREKS` | `internal/aws/ecr.go` |
| `ecr` | `pipeline` | `checkECRPipeline` | `internal/aws/ecr.go` |
| `ecr` | `role` | `checkECRRole` | `internal/aws/ecr.go` |
| `ecs-svc` | `cfn` | `checkECSSvcCF` | `internal/aws/ecs_services_related.go` |
| `ecs-svc` | `eb-rule` | `checkECSSvcEbRule` | `internal/aws/ecs_services_related.go` |
| `ecs-svc` | `ecr` | `checkECSSvcECR` | `internal/aws/ecs_services_related.go` |
| `ecs-svc` | `r53` | `checkECSSvcR53` | `internal/aws/ecs_services_related.go` |
| `ecs-svc` | `secrets` | `checkECSSvcSecrets` | `internal/aws/ecs_services_related.go` |
| `ecs-svc` | `sfn` | `checkECSSvcSFN` | `internal/aws/ecs_services_related.go` |
| `ecs-task` | `kms` | `checkECSTaskKMS` | `internal/aws/ecs_tasks_related.go` |
| `ecs-task` | `secrets` | `checkECSTaskSecrets` | `internal/aws/ecs_tasks_related.go` |
| `ecs-task` | `ssm` | `checkECSTaskSSM` | `internal/aws/ecs_tasks_related.go` |
| `efs` | `backup` | `checkEFSBackup` | `internal/aws/efs_related.go` |
| `efs` | `ecs-task` | `checkEFSECSTask` | `internal/aws/efs_related.go` |
| `eip` | `kms` | `checkEIPKMS` | `internal/aws/eip_related.go` |
| `eks` | `acm` | `checkEKSACM` | `internal/aws/eks_related.go` |
| `eks` | `ami` | `checkEKSAMI` | `internal/aws/eks_related.go` |
| `eks` | `ec2` | `checkEKSEC2` | `internal/aws/eks_related.go` |
| `eks` | `ecr` | `checkEKSECR` | `internal/aws/eks_related.go` |
| `eks` | `iam-user` | `checkEKSIAMUser` | `internal/aws/eks_related.go` |
| `elb` | `logs` | `checkELBLogs` | `internal/aws/elb.go` |
| `iam-user` | `kms` | `checkIAMUserKMS` | `internal/aws/iam_users.go` |
| `iam-user` | `role` | `checkIAMUserRole` | `internal/aws/iam_users.go` |
| `kinesis` | `ddb` | `checkKinesisDDB` | `internal/aws/kinesis_related.go` |
| `kinesis` | `eb-rule` | `checkKinesisEBRule` | `internal/aws/kinesis_related.go` |
| `kinesis` | `logs` | `checkKinesisLogs` | `internal/aws/kinesis_related.go` |
| `kms` | `role` | `checkKMSRole` | `internal/aws/kms.go` |
| `lambda` | `asg` | `checkLambdaASG` | `internal/aws/lambda.go` |
| `lambda` | `ec2` | `checkLambdaEC2` | `internal/aws/lambda.go` |
| `lambda` | `ecr` | `checkLambdaECR` | `internal/aws/lambda.go` |
| `lambda` | `elb` | `checkLambdaELB` | `internal/aws/lambda.go` |
| `lambda` | `r53` | `checkLambdaR53` | `internal/aws/lambda.go` |
| `lambda` | `sfn` | `checkLambdaSFN` | `internal/aws/lambda.go` |
| `opensearch` | `role` | `checkOpenSearchRole` | `internal/aws/opensearch_related.go` |
| `pipeline` | `eb-rule` | `checkPipelineEbRule` | `internal/aws/codepipeline.go` |
| `pipeline` | `logs` | `checkPipelineLogs` | `internal/aws/codepipeline.go` |
| `rds-snap` | `backup` | `checkRDSSnapBackup` | `internal/aws/rds_snapshots.go` |
| `role` | `kms` | `checkRoleKMS` | `internal/aws/iam_roles_related.go` |
| `secrets` | `codeartifact` | `checkSecretsCodeArtifact` | `internal/aws/secrets_related.go` |
| `secrets` | `eb` | `checkSecretsEB` | `internal/aws/secrets_related.go` |
| `secrets` | `ecr` | `checkSecretsECR` | `internal/aws/secrets_related.go` |
| `secrets` | `ecs-task` | `checkSecretsECSTask` | `internal/aws/secrets_related.go` |
| `secrets` | `logs` | `checkSecretsLogs` | `internal/aws/secrets_related.go` |
| `secrets` | `pipeline` | `checkSecretsPipeline` | `internal/aws/secrets_related.go` |
| `secrets` | `role` | `checkSecretsRole` | `internal/aws/secrets_related.go` |
| `secrets` | `s3` | `checkSecretsS3` | `internal/aws/secrets_related.go` |
| `secrets` | `sns` | `checkSecretsSNS` | `internal/aws/secrets_related.go` |
| `ses` | `acm` | `checkSESAcm` | `internal/aws/ses.go` |
| `ses` | `alarm` | `checkSESAlarm` | `internal/aws/ses.go` |
| `ses` | `cfn` | `checkSESCFN` | `internal/aws/ses.go` |
| `ses` | `eb-rule` | `checkSESEbRule` | `internal/aws/ses.go` |
| `ses` | `kinesis` | `checkSESKinesis` | `internal/aws/ses.go` |
| `ses` | `kms` | `checkSESKMS` | `internal/aws/ses.go` |
| `ses` | `lambda` | `checkSESLambda` | `internal/aws/ses.go` |
| `ses` | `logs` | `checkSESLogs` | `internal/aws/ses.go` |
| `ses` | `role` | `checkSESRole` | `internal/aws/ses.go` |
| `ses` | `s3` | `checkSESS3` | `internal/aws/ses.go` |
| `ses` | `sns` | `checkSESSns` | `internal/aws/ses.go` |
| `ses` | `trail` | `checkSESTrail` | `internal/aws/ses.go` |
| `sfn` | `cfn` | `checkSFNCFN` | `internal/aws/sfn.go` |
| `sfn` | `eb-rule` | `checkSFNEbRule` | `internal/aws/sfn.go` |
| `sns` | `cfn` | `checkSNSCFN` | `internal/aws/sns_related.go` |
| `sns-sub` | `ecs` | `checkSNSSubECS` | `internal/aws/sns_sub_related.go` |
| `sns-sub` | `kms` | `checkSNSSubKMS` | `internal/aws/sns_sub_related.go` |
| `sns-sub` | `policy` | `checkSNSSubPolicy` | `internal/aws/sns_sub_related.go` |
| `sqs` | `eb-rule` | `checkSQSEbRule` | `internal/aws/sqs_related.go` |
| `sqs` | `role` | `checkSQSRole` | `internal/aws/sqs_related.go` |
| `tg` | `kms` | `checkTGKMS` | `internal/aws/tg_related.go` |
| `tg` | `role` | `checkTGRole` | `internal/aws/tg_related.go` |
| `tg` | `secrets` | `checkTGSecrets` | `internal/aws/tg_related.go` |
| `waf` | `role` | `checkWAFRole` | `internal/aws/waf_related.go` |

## Summary by parent

| Parent | Missing relationships |
|--------|-----------------------|
| `alarm` | `role` |
| `backup` | `eb-rule`, `logs` |
| `cb` | `pipeline` |
| `codeartifact` | `acm`, `cb`, `kinesis`, `kms`, `lambda`, `logs`, `r53`, `role`, `waf` |
| `ddb` | `backup`, `kinesis`, `secrets`, `sns` |
| `docdb-snap` | `backup` |
| `eb` | `elb`, `role`, `s3`, `sg`, `tg` |
| `ec2` | `ssm` |
| `ecr` | `eb-rule`, `ecs`, `eks`, `pipeline`, `role` |
| `ecs-svc` | `cfn`, `eb-rule`, `ecr`, `r53`, `secrets`, `sfn` |
| `ecs-task` | `kms`, `secrets`, `ssm` |
| `efs` | `backup`, `ecs-task` |
| `eip` | `kms` |
| `eks` | `acm`, `ami`, `ec2`, `ecr`, `iam-user` |
| `elb` | `logs` |
| `iam-user` | `kms`, `role` |
| `kinesis` | `ddb`, `eb-rule`, `logs` |
| `kms` | `role` |
| `lambda` | `asg`, `ec2`, `ecr`, `elb`, `r53`, `sfn` |
| `opensearch` | `role` |
| `pipeline` | `eb-rule`, `logs` |
| `rds-snap` | `backup` |
| `role` | `kms` |
| `secrets` | `codeartifact`, `eb`, `ecr`, `ecs-task`, `logs`, `pipeline`, `role`, `s3`, `sns` |
| `ses` | `acm`, `alarm`, `cfn`, `eb-rule`, `kinesis`, `kms`, `lambda`, `logs`, `role`, `s3`, `sns`, `trail` |
| `sfn` | `cfn`, `eb-rule` |
| `sns` | `cfn` |
| `sns-sub` | `ecs`, `kms`, `policy` |
| `sqs` | `eb-rule`, `role` |
| `tg` | `kms`, `role`, `secrets` |
| `waf` | `role` |

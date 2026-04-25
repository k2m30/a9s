# Related-Panel Missing Checkers

**114 missing parent→related pairs** across 35 parent resource types.

Source: `docs/related-resources.md` (golden contract).
Status: either no `RegisterRelated` entry exists, or the entry points to a deleted stub function.

| Parent | Related | Status | Deleted stub |
|--------|---------|--------|--------------|
| `alarm` | `role` | registered but function deleted | `checkAlarmRole` |
| `apigw` | `kms` | no registration | — |
| `asg` | `ami` | no registration | — |
| `asg` | `cfn` | no registration | — |
| `asg` | `elb` | no registration | — |
| `asg` | `role` | no registration | — |
| `asg` | `sg` | no registration | — |
| `asg` | `sns` | no registration | — |
| `asg` | `vpc` | no registration | — |
| `backup` | `eb-rule` | registered but function deleted | `checkBackupEBRule` |
| `backup` | `logs` | registered but function deleted | `checkBackupLogs` |
| `cb` | `pipeline` | registered but function deleted | `checkCbPipeline` |
| `codeartifact` | `acm` | registered but function deleted | `checkCodeartifactAcm` |
| `codeartifact` | `cb` | registered but function deleted | `checkCodeartifactCB` |
| `codeartifact` | `kinesis` | registered but function deleted | `checkCodeartifactKinesis` |
| `codeartifact` | `kms` | registered but function deleted | `checkCodeartifactKMS` |
| `codeartifact` | `lambda` | registered but function deleted | `checkCodeartifactLambda` |
| `codeartifact` | `logs` | registered but function deleted | `checkCodeartifactLogs` |
| `codeartifact` | `r53` | registered but function deleted | `checkCodeartifactR53` |
| `codeartifact` | `role` | registered but function deleted | `checkCodeartifactRole` |
| `codeartifact` | `waf` | registered but function deleted | `checkCodeartifactWaf` |
| `ddb` | `backup` | registered but function deleted | `checkDdbBackup` |
| `ddb` | `kinesis` | registered but function deleted | `checkDdbKinesis` |
| `ddb` | `secrets` | registered but function deleted | `checkDdbSecrets` |
| `ddb` | `sns` | registered but function deleted | `checkDdbSNS` |
| `docdb-snap` | `backup` | registered but function deleted | `checkDocdbSnapBackup` |
| `eb` | `elb` | registered but function deleted | `checkEbELB` |
| `eb` | `role` | registered but function deleted | `checkEbRole` |
| `eb` | `s3` | registered but function deleted | `checkEbS3` |
| `eb` | `sg` | registered but function deleted | `checkEbSG` |
| `eb` | `tg` | registered but function deleted | `checkEbTG` |
| `ec2` | `ssm` | registered but function deleted | `checkEC2SSM` |
| `ecr` | `eb-rule` | registered but function deleted | `checkECREbRule` |
| `ecr` | `ecs` | registered but function deleted | `checkECRECS` |
| `ecr` | `eks` | registered but function deleted | `checkECREKS` |
| `ecr` | `pipeline` | registered but function deleted | `checkECRPipeline` |
| `ecr` | `role` | registered but function deleted | `checkECRRole` |
| `ecs-svc` | `cf` | registered but function deleted | `checkECSSvcCF` |
| `ecs-svc` | `eb-rule` | registered but function deleted | `checkECSSvcEbRule` |
| `ecs-svc` | `ecr` | registered but function deleted | `checkECSSvcECR` |
| `ecs-svc` | `r53` | registered but function deleted | `checkECSSvcR53` |
| `ecs-svc` | `secrets` | registered but function deleted | `checkECSSvcSecrets` |
| `ecs-svc` | `sfn` | registered but function deleted | `checkECSSvcSFN` |
| `ecs-task` | `kms` | registered but function deleted | `checkECSTaskKMS` |
| `ecs-task` | `secrets` | registered but function deleted | `checkECSTaskSecrets` |
| `ecs-task` | `ssm` | registered but function deleted | `checkECSTaskSSM` |
| `efs` | `backup` | registered but function deleted | `checkEFSBackup` |
| `efs` | `ecs-task` | registered but function deleted | `checkEFSECSTask` |
| `eip` | `kms` | registered but function deleted | `checkEIPKMS` |
| `eks` | `acm` | registered but function deleted | `checkEKSACM` |
| `eks` | `ami` | registered but function deleted | `checkEKSAMI` |
| `eks` | `ec2` | registered but function deleted | `checkEKSEC2` |
| `eks` | `ecr` | registered but function deleted | `checkEKSECR` |
| `eks` | `iam-user` | registered but function deleted | `checkEKSIAMUser` |
| `elb` | `logs` | registered but function deleted | `checkELBLogs` |
| `iam-user` | `kms` | registered but function deleted | `checkIAMUserKMS` |
| `iam-user` | `role` | registered but function deleted | `checkIAMUserRole` |
| `kinesis` | `ddb` | registered but function deleted | `checkKinesisDDB` |
| `kinesis` | `eb-rule` | registered but function deleted | `checkKinesisEBRule` |
| `kinesis` | `logs` | registered but function deleted | `checkKinesisLogs` |
| `kms` | `role` | registered but function deleted | `checkKMSRole` |
| `lambda` | `asg` | registered but function deleted | `checkLambdaASG` |
| `lambda` | `ec2` | registered but function deleted | `checkLambdaEC2` |
| `lambda` | `ecr` | registered but function deleted | `checkLambdaECR` |
| `lambda` | `elb` | registered but function deleted | `checkLambdaELB` |
| `lambda` | `r53` | registered but function deleted | `checkLambdaR53` |
| `lambda` | `sfn` | registered but function deleted | `checkLambdaSFN` |
| `ng` | `ami` | no registration | — |
| `ng` | `ebs` | no registration | — |
| `ng` | `kms` | no registration | — |
| `ng` | `subnet` | no registration | — |
| `opensearch` | `role` | registered but function deleted | `checkOpenSearchRole` |
| `pipeline` | `eb-rule` | registered but function deleted | `checkPipelineEbRule` |
| `pipeline` | `logs` | registered but function deleted | `checkPipelineLogs` |
| `rds-snap` | `backup` | registered but function deleted | `checkRDSSnapBackup` |
| `role` | `kms` | registered but function deleted | `checkRoleKMS` |
| `secrets` | `codeartifact` | registered but function deleted | `checkSecretsCodeArtifact` |
| `secrets` | `eb` | registered but function deleted | `checkSecretsEB` |
| `secrets` | `ecr` | registered but function deleted | `checkSecretsECR` |
| `secrets` | `ecs-task` | registered but function deleted | `checkSecretsECSTask` |
| `secrets` | `logs` | registered but function deleted | `checkSecretsLogs` |
| `secrets` | `pipeline` | registered but function deleted | `checkSecretsPipeline` |
| `secrets` | `role` | registered but function deleted | `checkSecretsRole` |
| `secrets` | `s3` | registered but function deleted | `checkSecretsS3` |
| `secrets` | `sns` | registered but function deleted | `checkSecretsSNS` |
| `ses` | `acm` | registered but function deleted | `checkSESAcm` |
| `ses` | `alarm` | registered but function deleted | `checkSESAlarm` |
| `ses` | `cfn` | registered but function deleted | `checkSESCFN` |
| `ses` | `eb-rule` | registered but function deleted | `checkSESEbRule` |
| `ses` | `kinesis` | registered but function deleted | `checkSESKinesis` |
| `ses` | `kms` | registered but function deleted | `checkSESKMS` |
| `ses` | `lambda` | registered but function deleted | `checkSESLambda` |
| `ses` | `logs` | registered but function deleted | `checkSESLogs` |
| `ses` | `role` | registered but function deleted | `checkSESRole` |
| `ses` | `s3` | registered but function deleted | `checkSESS3` |
| `ses` | `sns` | registered but function deleted | `checkSESSns` |
| `ses` | `trail` | registered but function deleted | `checkSESTrail` |
| `sfn` | `cfn` | registered but function deleted | `checkSFNCFN` |
| `sfn` | `eb-rule` | registered but function deleted | `checkSFNEbRule` |
| `sns` | `cfn` | registered but function deleted | `checkSNSCFN` |
| `sns-sub` | `ecs` | registered but function deleted | `checkSNSSubECS` |
| `sns-sub` | `kms` | registered but function deleted | `checkSNSSubKMS` |
| `sns-sub` | `policy` | registered but function deleted | `checkSNSSubPolicy` |
| `sqs` | `eb-rule` | registered but function deleted | `checkSQSEbRule` |
| `sqs` | `role` | registered but function deleted | `checkSQSRole` |
| `tg` | `kms` | registered but function deleted | `checkTGKMS` |
| `tg` | `role` | registered but function deleted | `checkTGRole` |
| `tg` | `secrets` | registered but function deleted | `checkTGSecrets` |
| `tgw` | `cfn` | no registration | — |
| `tgw` | `role` | no registration | — |
| `tgw` | `rtb` | no registration | — |
| `tgw` | `subnet` | no registration | — |
| `tgw` | `vpc` | no registration | — |
| `waf` | `role` | registered but function deleted | `checkWAFRole` |

## For each (parent, related) pair, the DevOps reviewer must answer

**(a) Can the parent resource even have this related resource?** (yes / no / sometimes — explain)

**(b) If yes, what sequence of AWS API calls gets it for us?** (from the parent resource, given only its ARN/ID)

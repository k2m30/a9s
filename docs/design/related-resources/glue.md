# Glue Jobs (glue) — Related Resources

## Real-World Use Cases

**1. "Did the ETL job run last night?"** The job config shows schedule triggers and script location, but the actual run history (success/failure/duration) requires the Job Runs child view. Cross-resource navigation connects to the logs and the data sources.

**2. "Where does this job read from and write to?"** Glue jobs typically read from S3, databases (via JDBC connections), or Glue Data Catalog tables, and write to S3, Redshift, or databases. The connections and script reveal the data lineage.

**3. "Why did the job fail?"** The error message in the job run gives the Glue-level reason, but the detailed error is in CloudWatch Logs. Navigate to the log group for the full stack trace.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Step Functions (sfn) | Parse state machine definitions for tasks that invoke Glue jobs (resource type `arn:aws:states:::glue:startJobRun`). If a9s has SFN data cached, scan definitions. | "Is this job part of a workflow?" Glue jobs are often orchestrated by Step Functions. | P1 |
| EventBridge Rules (eb-rule) | Search EventBridge rules for targets that match Glue or search for Glue triggers via `glue:GetTriggers` with job name. | "What triggers this job?" Scheduled or event-driven. | P1 |
| CloudFormation Stacks (cfn) | Check tags. | "Which stack manages this job?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Log Groups (logs) | Glue jobs write to `/aws-glue/jobs/output` (stdout) and `/aws-glue/jobs/error` (stderr) by default. Custom log groups can be configured via `DefaultArguments["--continuous-log-logGroup"]`. | "Where are the job logs?" Navigate to the log group for the actual error messages and stack traces. | P0 |
| S3 Bucket (s3) — Script | Job has `Command.ScriptLocation` — FORWARD. The S3 path to the ETL script. Navigate to the bucket to see the script or check versioning. | "Where is the ETL code?" | P1 |
| S3 Bucket (s3) — Data | Job `DefaultArguments` often contain S3 paths for input (`--source_path`) and output (`--output_path`). Also, Glue connections reference JDBC endpoints. Heuristic — requires parsing arguments. | "What data does this job process?" | P1 |
| IAM Role (role) | Job has `Role` — FORWARD. Navigate to the role for permissions (S3 access, Glue catalog access, JDBC connectivity). | "Why is the job getting AccessDenied?" | P1 |
| Glue Triggers (not in a9s) | `glue:GetTriggers` — shows schedule, conditional (after another job), and on-demand triggers for this job. | "When and how is this job triggered?" | P1 |
| Glue Connections (not in a9s) | Job has `Connections.Connections[]` — FORWARD. Connections provide JDBC endpoints, VPC config, and credentials for database access. Navigate via connection to see the target database. | "Which databases does this job connect to?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteJob | "Who deleted this ETL job?" Scheduled data processing stops. |
| UpdateJob | "Who changed the job configuration?" Script, role, or resource allocation changes. |
| StartJobRun | "Who triggered a job run?" Manual runs or unexpected triggers. |

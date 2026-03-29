# API Gateways (apigw) — Related Resources

## Real-World Use Cases

**1. "Which Lambda functions does this API invoke?"** API Gateway routes map to backend integrations — usually Lambda functions. The API config has integration details, but navigating to the actual Lambda to check its health, logs, and recent errors requires cross-resource navigation.

**2. "What DNS name resolves to this API?"** Custom domains use ACM certificates and Route 53 alias records. The API's default endpoint (`{id}.execute-api.{region}.amazonaws.com`) is always available, but production traffic usually goes through a custom domain.

**3. "Why is this API returning 500s?"** Check the integration target (Lambda, ALB, Step Functions) for errors. Also check the API's CloudWatch access logs and execution logs.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Route 53 Records (r53) | Search R53 for alias or CNAME records pointing to this API's custom domain name or default endpoint. | "What domains point to this API?" | P1 |
| CloudFront Distributions (cf) | Search CF distributions with origins pointing to this API's endpoint. | "Is this API behind CloudFront?" | P1 |
| WAF Web ACLs (waf) | `wafv2:GetWebACLForResource` with this API's stage ARN (for REST APIs) or API ARN (for HTTP APIs). | "Is this API protected by WAF?" | P1 |
| CloudWatch Alarms (alarm) | Search alarms with `ApiId` or `ApiName` dimension. | "What monitoring watches this API?" | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Lambda Functions (lambda) | For HTTP APIs: `apigatewayv2:GetIntegrations` with `ApiId` — each integration with `IntegrationType=AWS_PROXY` has `IntegrationUri` containing the Lambda ARN. For REST APIs: `apigateway:GetResources` then `apigateway:GetIntegration` for each resource/method — integration URI contains the Lambda ARN. | "Which Lambdas does this API call?" THE core relationship. Navigate to Lambda for logs and error details. | P0 |
| CloudWatch Log Group (logs) | API stage has `AccessLogSettings.DestinationArn` — FORWARD. Also, execution logging goes to `API-Gateway-Execution-Logs_{api-id}/{stage-name}`. | "Where are API access and execution logs?" Access logs show request patterns; execution logs show integration details and errors. | P0 |
| ACM Certificate (acm) | Custom domain configuration references ACM certificate ARN — FORWARD. | "Which certificate does the custom domain use?" Check for expiration. | P1 |
| VPC Link (not in a9s) | For private integrations, the integration references a VPC link which connects to an ALB/NLB in a VPC. `apigatewayv2:GetVpcLinks` or the integration's `ConnectionId`. | "How does this API reach private resources?" VPC links bridge API GW to VPC-internal services. | P2 |
| Step Functions (sfn) | Some API integrations invoke Step Functions directly. Check integrations for `IntegrationSubtype=StepFunctions-StartExecution` or URI containing `states:action/StartExecution`. | "Does this API trigger a Step Functions workflow?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteApi | "Who deleted this API?" All endpoints stop responding. |
| UpdateApi | "Who changed API settings?" Changes to CORS, throttling, or protocol can break clients. |
| UpdateStage / CreateDeployment | "Who deployed a new version?" API GW deployments activate route/integration changes. A bad deployment breaks all API consumers. |

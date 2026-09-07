## Fixed

- A node group flagged for a health issue now shows the issue AWS reported
  under the finding in its detail view. The code was reaching the screen only
  as a plain field further down, while the finding promised to list it.
- The per-resource docs now point at the signals page as it is written today:
  every reference names a category and a row, or a section heading, instead of
  a line number or a table cell that no longer exists.
- The EKS cluster, CloudTrail event, CodeArtifact and RDS snapshot docs now
  describe the wordings, severities and checks the app actually ships, and the
  signals page no longer lists a Secrets Manager check that already ships as
  not implemented.

## Fixed

- A credential in an environment variable, a stage variable, a job argument, a
  stack output, user data or a state-machine definition is now listed once. A
  generated value under a credential-named key satisfied two of the scanner's
  rules at once and was listed twice, one line per reason.
- A credential written with a quoted key and a bare value, `"password":
  hunter2`, is now reported. It is valid YAML and a real shape in task
  parameters and CloudFormation templates, and the scanner skipped it unless
  the value was quoted too.
- An API Gateway stage with more than one leaking variable now lists them all.
  The stage reported one issue per leaking variable, and each report replaced
  the previous one's supporting rows, so only the last variable was named.
- An issue found on more than one part of a resource now names every part it
  was found on, and is stated once. An API with two stages missing access logs
  raised the issue twice and named only the last stage, leaving nothing to say
  the first had been inspected.

## Changed

- A check a9s could not run reads the same way everywhere: one line naming what was refused, how many resources it covered and one example, with no request ids or SDK preamble.
- A batch refused for one reason is recognised as a refusal on the main menu, instead of showing as an unclassified error.
- A failed check now names the call it was making, so a batch that makes two different calls says which one refused.
- A DynamoDB table whose backup setting could not be read says so in the error log, instead of only showing "?" with no reason.

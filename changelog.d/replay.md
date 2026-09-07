## Fixed

- A list restored from the cache after a restart now shows the same cells the
  fetch showed. Nine columns across eight types used to change wording on the
  way back from disk: AMIs and certificates and distributions flipped `Yes` to
  `true`, alarm thresholds grew trailing zeros, secret timestamps lost their
  time of day, log group retention turned into a word that was not a retention,
  an ECS task's identifier shrank to a bare id, and a Kinesis stream's status
  went blank.
- A cached row could render two different lists on two consecutive starts. The
  same column answered to its title under two spellings at once and whichever
  one came back first decided the cell.

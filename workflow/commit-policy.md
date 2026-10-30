# Commit Policy

Commits are concise historical records of meaningful changes, not implementation
reports. Use a short message describing the outcome or intent of the change.

Put durable system context in relevant files under `features/`.
Put reusable workflow lessons in relevant files under `workflow/`.
Keep task-specific findings with the work item unless they establish knowledge
that will be useful beyond that task. Do not duplicate documentation in commits.

## Completing tasks

- Run `sh workflow/lint.sh` and the relevant tests and checks when the
  environment permits.
- Commit completed task changes after relevant verification.
- Push completed task commits when a suitable configured remote is available
  and the current branch can be pushed safely.
- Inspect the working tree, current branch, remotes, and upstream as needed.
- Inspect the diff before staging and stage only the intended changes.
- Verify the staged diff before committing.
- Do not commit unrelated changes or overwrite another contributor's work.
- If existing changes have unclear ownership or purpose, do not stage, amend,
  discard, or include them. Continue only if your changes can be safely isolated;
  otherwise, report the blocker.
- Investigate and fix failures relevant to the task. Distinguish task-related
  failures from pre-existing failures and environmental limitations.
- Do not pursue unrelated fixes just to make all checks pass.
- If verification is incomplete, state that limitation accurately.
- If the push fails, report the reason. Do not force-push, guess the remote,
  or change remote settings. Retry only when the cause is understood and the
  corrective action is clearly safe and within scope.
- Never claim a commit or push succeeded without verifying the result.

Task lifecycle and Git history are separate. Move a task to `tasks/closed/`
only when its acceptance criteria are met. Record outstanding limitations or
follow-up work explicitly.

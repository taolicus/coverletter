# Execution and Stopping Rules

## Start with relevant context

- Start from the requested outcome.
- Read project instructions, the relevant work item, directly relevant feature
  documentation, and declared dependencies.
- Investigate further only to resolve a concrete implementation or verification
  question.
- Use the best available evidence. Do not repeatedly revisit settled decisions
  without new evidence.
- Do not inspect unrelated files or history without a specific reason.

## Plan proportionately

- Use a lightweight plan for routine, low-risk changes; do not create a separate
  planning phase unless it adds value.
- For substantial changes, establish scope, acceptance criteria, and a practical
  implementation sequence.
- Split tasks only according to `incremental-task-scoping.md`.
- If a material uncertainty remains after reasonable, targeted investigation,
  record what is known and the next action. Ask for clarification when the
  uncertainty prevents a safe or correct decision.

## Control scope

- Implement the requested outcome without unrelated cleanup or speculative
  improvements.
- Record material, actionable discoveries outside the current scope as follow-up
  issues or tasks.
- Update documentation only when it adds useful durable system knowledge or a
  reusable workflow lesson.
- Do not repeat checks, recreate artifacts, or reconsider settled decisions
  unless new evidence justifies it.

## Verify and stop

- Verify against the task's acceptance criteria using the most relevant available
  checks. Do not try to prove the absence of every possible defect.
- Run `sh workflow/lint.sh` when the repository has work items; it catches the
  bookkeeping errors that a passing build does not.
- If a relevant check fails, make targeted attempts to diagnose and fix it
  within scope.
- If blocked, record the evidence, limitation, and next concrete action rather
  than repeating unsuccessful attempts or claiming completion.
- Stop when the acceptance criteria are met, relevant verification is complete
  to the extent possible, necessary documentation is updated, and the required
  commit/push steps have been attempted.
- Report material limitations, failed checks, and follow-up work explicitly.

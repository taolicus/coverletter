# Workflow

Read the rules relevant to the task. Apply them proportionately; do not turn
routine changes into unnecessary planning, investigation, or administration.

## Work item lifecycle

- `issues/`: unresolved observations, problems, questions, and investigations.
- `tasks/open/`: actionable work sufficiently understood to implement.
- `tasks/closed/`: completed tasks retained for history.
- `features/`: durable knowledge about implemented system behaviour and design.

Start a new item from `task-template.md` or `issue-template.md`.

Move an issue to `tasks/open/` when it becomes actionable.
Move a completed task to `tasks/closed/`, preserving its filename and context.
Record the outcome under `## Outcome`; `lint.sh` requires it for a closed task.
Update relevant feature documentation when work establishes useful durable
knowledge. Do not duplicate information already documented.

Create follow-up issues for material, actionable work outside the current scope.
Do not create issues for trivial observations, speculative improvements, or work
already tracked elsewhere.

The split matters more than it looks: an agent that cannot tell an unresolved
observation from an actionable task will start implementing the observation.

## Workflow rules

- `execution-and-stopping.md`
- `incremental-task-scoping.md`
- `work-item-metadata.md`
- `device-aware-development.md`
- `commit-policy.md`
- `verify-what-the-user-sees.md`

## Templates

- `task-template.md`
- `issue-template.md`

## Checks

`sh workflow/lint.sh` verifies the invariants these rules assume and exits
nonzero when one does not hold, so it can gate a commit or a CI job.

## Ruleset version

`workflow/VERSION` records the ruleset revision this project received.
`workflow/.bootstrap-manifest` records the hash of every installed file, which
is what makes an update safe. Re-run the bootstrap script with `--upgrade` to
receive a later revision: files you have not touched are replaced, and files you
have edited are kept with the new version beside them as `<file>.new`.

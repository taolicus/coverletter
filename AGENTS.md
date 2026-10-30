# Agent Instructions

Read `workflow/README.md` before working on this project.

Follow the relevant workflow rules. Inspect only the context needed for the
task, preserve existing work, avoid unrelated changes, and keep the repository
in a coherent state.

Use:
- `issues/` for unresolved observations, problems, and questions.
- `tasks/open/` for actionable work.
- `tasks/closed/` for completed tasks retained for history.
- `features/` for durable knowledge about the implemented system.

Start new work items from `workflow/issue-template.md` and
`workflow/task-template.md`. Delete the sections that do not apply rather than
filling them in.

Run `sh workflow/lint.sh` before declaring a task complete. It checks the
invariants the rules assume but cannot enforce: dependencies that resolve,
closed tasks that record an outcome, and unique work-item filenames.

Put reusable workflow lessons in `workflow/`. Do not use commit messages or
conversation history as substitutes for durable documentation.

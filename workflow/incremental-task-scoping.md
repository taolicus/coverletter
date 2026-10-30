# Incremental Task Scoping

Break substantial work into the smallest practical set of coherent tasks that
materially improves reviewability, reduces risk, resolves uncertainty, or
enables independent progress.

Prefer the simplest workable breakdown. Avoid both oversized tasks and excessive
fragmentation.

## When to split

Split work when:
- A meaningful outcome can be implemented and verified independently.
- Distinct outcomes have different acceptance criteria.
- A step resolves significant uncertainty before further implementation.
- Separation materially reduces regression risk or simplifies diagnosis.
- Dependencies require a clear execution order.

Do not split merely because work touches multiple files, layers, or technical
steps. Keep tightly coupled changes together when splitting adds artificial
boundaries, scaffolding, or extra coordination without meaningful benefit.

## Size and acceptance criteria

A small, routine change should usually remain one task. Substantial work may need
a short plan and several tasks. Use a detailed breakdown only when complexity,
risk, or dependencies justify it.

Each task should define:
- One coherent outcome.
- Scope and important constraints.
- Observable acceptance criteria.
- Relevant verification.
- Genuine dependencies, if any.

`task-template.md` has that shape. Copy it rather than inventing one; an
inconsistent format costs more to read than it saves to write.

Make each task small enough for a focused review but large enough to deliver
meaningful value. Do not create tasks for trivial edits, routine substeps, or
every file touched.

## Preserve repository coherence

A completed task should leave the repository in a coherent, reviewable state.
Where applicable, it should build and pass relevant tests.

Order dependent work so intermediate states remain valid. Use compatibility
layers, feature flags, adapters, or incremental migrations only when needed to
keep intermediate states safe and the added complexity is justified.

If changes cannot be separated without breaking the repository or adding
excessive scaffolding, keep the coupled work together. Do not label an unsafe
intermediate state as independently complete.

## Stop decomposing

Once each task has a coherent outcome, clear acceptance criteria, and a safe
implementation path, stop decomposing and begin work.

Reassess the breakdown only when new information materially changes the scope,
risk, dependencies, or feasibility. If implementation reveals independent work
outside scope, record it separately instead of silently expanding the task.

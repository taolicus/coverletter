# Work Item Metadata

Metadata is optional. Use it only when it adds practical value.

## Area

- `area` is an optional, single slug for grouping or filtering work.
- Omit it when it provides no useful distinction.

## Dependencies

- `dependencies` is an optional list of work item filenames.
- Each reference must resolve to an item in `issues/`, `tasks/open/`,
  or `tasks/closed/`.
- Filenames must be unique across these directories.
- Preserve filenames when moving items between lifecycle directories.
- Update references if a referenced file is renamed or removed.
- Dependencies represent genuine execution constraints, not merely related work.
- Check declared dependencies before implementation.
- For completed dependencies, inspect their recorded outcome and current code
  only as needed to determine whether the requirement is satisfied.
- If an unresolved dependency blocks the task, record the blocker and determine
  the appropriate next step. Do not silently work around it.
- Do not investigate unrelated dependency history.
- Omit the field when no dependencies apply.

`sh workflow/lint.sh` fails when a declared dependency does not resolve, and
when two work items share a filename.

Do not add a priority field. Decide what to work on during planning based on
the task, constraints, dependencies, and available time.

# Device-Aware Development

Choose tasks based on the current device's capabilities, actual availability,
task risk, and the ability to verify the result.

## Phone

Prefer bounded tasks that can be performed and reviewed effectively on a phone:
- Review code, diffs, documentation, and investigation results.
- Refine plans, acceptance criteria, and issue descriptions.
- Make small, low-risk edits when suitable tools are available.

Do not assume every coding task must wait for a laptop.

## Laptop

Defer work when it materially depends on capabilities unavailable on the phone,
for example:
- Local builds, substantial test runs, or debugging.
- Simulator or desktop-tool workflows unavailable on the phone.
- Changes that cannot be reliably reviewed or verified on mobile.

Do not defer work merely because a laptop is generally preferable for coding.

## Decision and handoff

Assess device suitability once at the start using required tools, environment,
risk, and verification needs. Reassess only when requirements change or a
concrete limitation is encountered.

Record device constraints in a task only when they materially affect execution
or verification. Do not require device metadata on every task.

For cross-device handoffs, record:
- Findings and decisions.
- Changes made and unresolved questions.
- The next concrete action.
- Which step requires the other device.

Preserve uncommitted work. Never assume changes are available on another device
unless repository synchronisation has been verified.

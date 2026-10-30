# Verify What The User Sees

A change is not verified by the diff that produces it. It is verified by the
result someone can observe. When the outcome is rendered, served, or deployed,
check that result, not the code that should cause it.

## A change that does nothing

- A style change must change a computed value. Read the current cascade and the
  property's default first: a rule that restates a default, such as
  `justify-content: flex-start` on a flex column, deploys cleanly, passes every
  check, and changes nothing on screen.
- Layout and styling that exist in one state only — playing, lobby, error — can
  only be checked in that state. A rule scoped to one state is invisible from
  every other.

## The artefact may be stale, or the rule may be a no-op

When a change is reported as not visible, compare what is served with what is in
the repository before blaming the client: fetch the file with a cache-busting
query and compare it against the working tree. If the bytes match, the rule was
a no-op or applies in the wrong state.

## Verify on the device that will see it

A phone, a desktop, and a CI browser are different clients. Prefer verifying on
the one that reported the problem. If that device cannot run the check, say so
in the task outcome rather than implying the check passed.

## Do not keep rules that do nothing

Rules added while diagnosing and then left behind read as if they matter, and
they hide the one rule that does. Remove them when the diagnosis is done.

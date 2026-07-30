---
ledger-id: rename-threshold
---

# Follow renames at 25%, not git's default 50%

## Context

A file that both moves *and* grows scores low on git's similarity metric. A
real case during development: `retry.go` moved into `internal/auth/` and gained
twelve lines of documentation in the same range of history. Git scored it 36%,
below the default 50% rename threshold, so the anchor reported broken even
though the code was plainly still there.

## Decision

Pass `-M25%` when asking git to detect renames.

## The code this governs

```ledger
id: rename-threshold
```

## Consequences

A looser threshold risks following a rename to the wrong file. That failure is
self-correcting: the anchor's stored body will not match the candidate, so it
resolves as broken — exactly what would have happened without the rename
detection. So the downside of being permissive here is bounded, while the
upside is catching the common move-and-edit case.

Do not "fix" this back to the default because it looks unusual.

---
ledger-id: break-loudly
---

# Break loudly rather than relocate silently

## Context

When the governed code has been edited, the resolver relocates it by
similarity. Any threshold is a trade: too low and we confidently point at code
that is no longer the decision's subject; too high and everything reports as
broken and people stop reading the colours.

## Decision

`driftThreshold = 0.5`. Below half similarity we report **broken** rather than
guessing a location.

## The code this governs

```ledger
id: break-loudly
```

## Consequences

A wrong relocation is worse than an admitted failure. If we silently move an
anchor onto unrelated code, the note now describes something it never governed
and nobody finds out. A loud break asks the one question worth asking: is this
decision still valid?

If you raise this number, expect false "tracked" reports. If you lower it,
expect noise. Neither is obviously wrong, but changing it needs a reason
better than a hunch.

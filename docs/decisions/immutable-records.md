---
ledger-id: immutable-records
---

# Records are immutable after bind

## Context

An early version cached the last resolution inside the record file. It seemed
harmless — a convenience for tooling.

It silently defeated the CI gate. `git commit -a` staged the rewritten record,
and the gate reads a touched record as "the rationale was revisited". A pull
request that changed governed code without revisiting anything would pass,
because merely *looking* at the decision had modified it.

## Decision

Nothing writes to `.ledger/*.json` outside of an explicit `bind`. Resolution is
computed on demand, every time.

## The code this governs

```ledger
id: immutable-records
```

## Consequences

Resolution costs a few git invocations per query instead of reading a cache.
That is cheap, and it is the price of the gate meaning what it says.

`TestResolveDoesNotMutateRecord` pins this behaviour. If you are adding a cache
for performance, put it outside the repo — anything inside `.ledger/` is read
by the gate as human intent.

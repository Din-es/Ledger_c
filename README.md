# ledger

Decisions that can't silently rot. Bind a design decision to the exact code it
governs; `ledger` keeps the link anchored as the code moves, and fails CI when
the code changes without the decision being revisited.

![Anchoring a decision, following it through a refactor, and blocking a pull
request that changes the code without revisiting the
rationale](docs/media/demo.gif)

**New here? Read [GETTING_STARTED.md](GETTING_STARTED.md)** — plain-language
explanation, first decision in five minutes, and setup for Obsidian, VS Code,
and CI.

## Install

```bash
go install github.com/Din-es/Ledger_c/cmd/ledger@latest
```

Or grab a prebuilt binary for Windows, macOS or Linux from
[Releases](https://github.com/Din-es/Ledger_c/releases). Then, in any repo:

```bash
ledger init
```

## Why

Docs drift because nothing forces them to keep up. `ledger` turns a decision
into a checked invariant: the link records live in the repo (`.ledger/*.json`),
travel with every clone, and a CI gate breaks the build when governed code
changes but its rationale doesn't.

## Commands

```
ledger init
ledger bind <file>:<start>-<end> --note <id> [--title "..."] [--add] [--note-file <path>]
ledger unbind <id>
ledger resolve <id> [--json]
ledger why <file>[:<line>[-<end>]] [--json]
ledger list [--json]
ledger verify [--since <base>] [--strict]
```

- **init** — scaffold `.ledger/`, `docs/decisions/`, and a CI workflow.
- **bind** — capture a decision's code span at the current commit (commit SHA +
  content fingerprint + surrounding context). Re-binding an id re-anchors it,
  which is how you clear a drift; `--add` appends another anchor so one
  decision can govern several places.
- **unbind** — retire a decision record while preserving its rationale document.
- **resolve** — find where that span lives now. Reports `fresh` (tracked),
  `drifted` (relocated by similarity, content changed), or `broken` (the code
  is gone).
- **verify** — CI gate. Plain form fails on `broken` (`--strict` also fails on
  `drifted`). `--since <base>` fails when a decision's code changed between
  `<base>` and `HEAD` but its note/record was not touched in the same range.
- **list** — resolve every decision at once. `--json` is the IPC surface the
  editor integrations consume.
- **why** — the reverse of bind: which decisions govern this line or range?

## Demo

```bash
./demo.sh
```

Builds a throwaway repo and walks the whole lifecycle: bind, refactor, rename,
a PR blocked for not revisiting the rationale, the same PR passing once it
does, `why` from the code side, and the decision breaking when its code is
deleted.

## How anchoring works

An anchor is `{commit, range, fingerprint, before/after context, body}`.
Resolution replays `git diff <boundCommit>..<target>`: hunks above the span
shift its line numbers; if the span itself was touched, a windowed fuzzy
relocate (LCS line similarity) finds its new home and scores confidence. When
the file itself is gone, git's rename detection follows it to its new path. The
bias is to break loudly rather than relocate silently.

## Editor surfaces

- **[obsidian-decision-ledger](https://github.com/Din-es/obsidian-decision-ledger)**
  — authoring side, in its own repository. A ```` ```ledger ```` codeblock
  renders the live code a decision governs, plus a staleness sidebar.
- `vscode-extension/` — reading side. Gutter dot, CodeLens and hover on
  governed spans, and a "why does this code exist?" command.
  Build with `npm install && npx tsc -p ./`, then load the folder as an
  unpacked extension (F5 in VS Code, or symlink into `~/.vscode/extensions`).

## This repo uses itself

Four of the non-obvious choices in this codebase are bound with `ledger`, and
`.github/workflows/decisions.yml` runs the gate on every pull request here. If
you change the drift threshold, the rename threshold, the tie-break, or the
record shape without touching the matching note in `docs/decisions/`, CI fails.

```bash
ledger list
```

```
[fresh] break-loudly       internal/ledger/resolve.go:15-17     conf 1.00
[fresh] break-loudly       internal/ledger/resolve.go:229-236   conf 1.00
[fresh] immutable-records  internal/ledger/record.go:42-55      conf 1.00
[fresh] rename-threshold   internal/ledger/git.go:59-63         conf 1.00
```

Those notes are worth reading before changing the resolver — each records a bug
that was fixed the hard way.

## A worked example

[**Din-es/stepclaim**](https://github.com/Din-es/stepclaim) is a separate
project — a step-claiming game — evolved across five versions with a bug in
each, used to test what this tool does when code genuinely moves. It carries a
hand-written change log beside the ledger's own output at every version, so the
two can be compared.

It is worth reading for two things: the ledger flagged a money bug that the
project's own test suite passed, and evolving it exposed a real bug in the
ledger, fixed in v1.2.1.

## Testing

```bash
go test ./...
```

Tests build real temp git repos and exercise shift, drift, deletion, rename,
and the CI gate.

## Contributing

This is a young project and most contributions are genuinely useful rather than
cosmetic. There are
[good first issues](https://github.com/Din-es/Ledger_c/labels/good%20first%20issue)
if you want something small and self-contained, and
[help wanted](https://github.com/Din-es/Ledger_c/labels/help%20wanted) for
bigger pieces — a Neovim client, an HTML report, or the open design problem of
anchoring to symbols instead of line ranges.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the build, the test approach, and a
map of the codebase.

## License

MIT — see [LICENSE](LICENSE).

**Dependencies.** The engine has none: `go.mod` requires nothing beyond the
standard library. The VS Code extension uses TypeScript and type packages at
build time only (MIT / Apache-2.0); the shipped bundle vendors no third-party
code, requiring only `child_process`, `path`, and the VS Code API.

**git.** `ledger` runs `git` as a separate process. Invoking a program is not
linking to it, so git's GPL-2.0 does not reach this codebase — the same basis
on which any tool that shells out to git stays permissively licensed. Nothing
from git is copied, bundled, or redistributed here.

## Status

**v1.2.0.** Anchor engine (shift, fuzzy relocate, rename-follow), CI gate,
multi-anchor decisions, JSON IPC, and both editor integrations — all working
and verified in their real hosts.

Possible next: an in-process go-git engine instead of shelling out, and
binding by symbol name rather than line range. See the
[open issues](https://github.com/Din-es/Ledger_c/issues).

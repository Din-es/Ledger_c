# Contributing

Thanks for looking. This project is young and small, which means most
contributions are genuinely useful rather than cosmetic.

## Getting set up

You need **Go 1.21+** and **git**. Nothing else for the engine.

```bash
git clone https://github.com/Din-es/Ledger_c.git
cd Ledger_c
go build -o ledger ./cmd/ledger
go test ./...
```

For the editor integrations you also need Node 18+:

```bash
cd obsidian-plugin  && npm install && npm run build
cd vscode-extension && npm install && npx tsc -p ./
```

## Before you open a pull request

```bash
gofmt -l .        # must print nothing
go vet ./...
go test ./...
```

CI runs these on Linux, macOS and Windows. **Please check Linux if you develop
on Windows** — a bug slipped through exactly this way once, because backslashes
are path separators on Windows and ordinary filename characters everywhere
else. If you have Docker:

```bash
docker run --rm -v "$PWD:/src" -w /src golang:1.26 sh -c "gofmt -l . && go vet ./... && go test ./..."
```

## How the thing works

Read [GETTING_STARTED.md](GETTING_STARTED.md) first for the user-facing model,
then:

| File | Responsibility |
|---|---|
| `internal/ledger/record.go` | The anchor and record types, JSON on disk |
| `internal/ledger/git.go` | Every call to `git` lives here |
| `internal/ledger/resolve.go` | The interesting part: finding code that moved |
| `cmd/ledger/main.go` | CLI parsing and output |

`resolve.go` is where most work happens. The flow is: diff the bound commit
against now → shift the line range by the hunks above it → follow a rename if
the file moved → if the governed lines themselves changed, search nearby for
the best match and score it. Statuses are `fresh`, `drifted`, `broken`.

## Read the decision notes

This repo uses its own tool. Four choices are anchored in `.ledger/` with
rationale in `docs/decisions/`. Each one records a bug that was fixed the hard
way, so please read the relevant note before changing:

- the drift threshold or the tie-break in `resolve.go`
- the rename threshold in `git.go`
- the shape of `LinkRecord` in `record.go`

If you change that code without touching the note, CI will stop you. That is
the tool working, not a mistake. Update the note, or say in the PR why the
decision no longer holds.

## Tests

Tests build real temporary git repos rather than mocking git — see
`newRepo()` in `internal/ledger/resolve_test.go`. If you fix a bug, add a case
that fails without your fix. Every bug found so far came from a realistic
scenario, not from unit tests written alongside the code.

## Style

- Comments explain *why*, not *what*. If a constant looks odd, say why it is
  that value.
- No new dependencies in the engine without discussion. It is currently pure
  standard library and that is worth protecting.
- Keep error messages actionable: name the file, say what to do next.

## Good first issues

Look for the [`good first issue`](https://github.com/Din-es/Ledger_c/labels/good%20first%20issue)
label. If something is unclear, comment on the issue — a question is a useful
contribution too, because it usually means the docs are wrong.

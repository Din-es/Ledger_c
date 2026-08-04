# Getting started

A guide for someone who has never seen this tool before. Read part 1, then
follow whichever setup you need.

---

## 1. What problem this solves

You make a non-obvious decision in code — retry with jitter, a specific lock
order, a magic number like 500. You write it down in a doc. Six months later
the code has moved, been refactored, or deleted, and the doc still confidently
describes something that no longer exists. Nobody notices, because nothing
ever checks.

**Decision Ledger ties a written decision to the exact lines of code it
governs, follows those lines as the code changes, and tells you the moment the
two fall out of sync.**

---

## 2. How it works (the mental model)

Three ideas. That's the whole tool.

### Idea 1 — An anchor is a fingerprint, not a line number

When you bind a decision, we don't just save "file X, line 40". Line numbers
break the instant someone adds an import. We save:

| We store | Why |
|---|---|
| The commit SHA | The ground truth of what the code looked like |
| The line range | Where it was *at that commit* |
| A fingerprint (hash of those lines) | To detect if the content changed |
| The lines themselves + surrounding context | To find the code again if it moves |

### Idea 2 — Resolving replays history to find the code now

When you ask "where is this code today?", the engine:

1. Diffs the bound commit against now.
2. **Shifts** the line range by whatever was added or removed above it.
   (This alone handles most drift — someone adding lines elsewhere in the file.)
3. If the file was **renamed**, follows it to the new path.
4. If the governed lines themselves were touched, **searches nearby** for the
   best match and scores how similar it is.

### Idea 3 — Everything reduces to three states

| State | Meaning | What you should do |
|---|---|---|
| 🟢 **tracked** | Found it, content unchanged | Nothing |
| 🟡 **drifted 75%** | Found it, but the code changed | Re-read the decision; still true? |
| 🔴 **code gone** | Can't find it at all | The decision is probably stale — fix or delete it |

That red state is the point of the whole tool. When code disappears, the doc
describing it is almost certainly lying, and now something tells you.

### Where things live

Link records are JSON files in `.ledger/` **committed to your repo**. That
matters: they travel with every clone, so anyone can go from code to decision,
and CI can check them. Your prose lives in normal markdown notes.

```
your-repo/
├── .ledger/
│   └── jitter-backoff.json      ← the anchor (committed)
├── docs/decisions/
│   └── jitter-backoff.md        ← the prose you actually write
└── src/auth/retry.go            ← the code it governs
```

> **Records are immutable after bind.** Nothing writes back to them during
> normal use. If something did, `git commit -a` would stage it and the CI gate
> would misread that as "the rationale was revisited" — silently defeating
> itself.

---

## 3. Setup — build the engine

You need **Go 1.21+** and **git**. Everything else is optional.

```bash
cd decision-ledger
go build -o ledger.exe ./cmd/ledger
```

On macOS/Linux drop the `.exe`. Put it on your `PATH`, or note the full path —
the editor integrations need it either way.

Check it works:

```bash
./ledger.exe --help
```

---

## 4. Your first decision (5 minutes)

Work inside any git repo. Scaffold it once:

```bash
ledger init
```

That creates `.ledger/`, `docs/decisions/`, and a ready-to-use GitHub Actions
workflow. It's safe to re-run.

**Step 1 — write the decision.** Create `docs/decisions/batch-cap.md`:

```markdown
---
ledger-id: batch-cap
---

# Sync batches capped at 500 records

Upstream rejects bodies over 1 MiB. Our widest row is ~2 KB, so 500 is the
largest batch that reliably fits.

## The code this governs

```ledger
id: batch-cap
```
```

**Step 2 — bind it to the code.** Find the lines that implement it, then:

```bash
ledger bind src/sync/batch.go:4-7 --note batch-cap --title "Sync batches capped at 500 records"
```

This writes `.ledger/batch-cap.json`. **Commit it alongside your code change.**

**Step 3 — check it.**

```bash
ledger resolve batch-cap
```

```
[fresh]  fresh     src/sync/batch.go:4-7  conf 1.00
```

**Step 4 — see it survive a refactor.** Add ten lines at the top of the file,
commit, and resolve again. The range follows the code:

```
[fresh]  fresh     src/sync/batch.go:14-17  conf 1.00
```

**Step 5 — closing the loop when a decision changes.** Eventually you'll
*change* the decision, not just the code. Say the cap moves from 500 to 800:

1. Edit the code.
2. Update the note to explain why it changed.
3. **Re-bind** to re-anchor at the new value:

```bash
ledger bind src/sync/batch.go:4-7 --note batch-cap --title "Sync batches capped at 800 records"
```

Binding an existing id overwrites its record — that's how you clear a drift.
Commit the updated `.ledger/*.json` with your change. Without step 3 the
decision stays amber forever, and people learn to ignore the colour.

**If your notes don't live in `docs/decisions/`.** That's the default, but the
gate watches whatever path the record stores, so point it at yours:

```bash
ledger bind src/sync/batch.go:4-7 --note batch-cap --note-file adr/0007-batch-cap.md
```

Get this wrong and the gate watches a file nobody edits, so every change looks
stale. The path is echoed on every bind — check it says what you expect. Once
set it survives re-binding, so you only pass it the first time.

**One decision, several places.** A rationale often governs code in more than
one spot — a constant and the check that uses it. Add anchors with `--add`:

```bash
ledger bind src/sync/client.kt:88-90 --note batch-cap --add
```

Each anchor resolves independently, and a change to any of them flags the
decision.

That's the core loop. Everything below is surfacing it where you already work.

---

## 5. See it in Obsidian (the writing side)

**Install:**

1. Build the plugin from its
   [own repository](https://github.com/Din-es/obsidian-decision-ledger):
   ```bash
   git clone https://github.com/Din-es/obsidian-decision-ledger
   cd obsidian-decision-ledger
   npm install && npm run build
   ```
   Or download `main.js`, `manifest.json` and `styles.css` straight from its
   [releases](https://github.com/Din-es/obsidian-decision-ledger/releases).
2. Copy `main.js`, `manifest.json`, and `styles.css` into
   `<your-vault>/.obsidian/plugins/decision-ledger/`
3. In Obsidian: **Settings → Community plugins** → turn off Restricted mode →
   enable **Decision Ledger**
4. In the plugin's settings, set:
   - **Ledger binary** — full path to `ledger.exe`
   - **Repository path** — full path to your repo

**Use it:** put a `ledger` block in any decision note:

````markdown
```ledger
id: batch-cap
```
````

It renders the live code that decision governs, with a status pill. Open the
sidebar via the command palette → **Open decision ledger** to see every
decision sorted worst-first.

> **Tip:** the cleanest setup is notes *inside* the repo — then vault paths and
> repo paths are the same thing and everything lines up. A separate vault
> works, but you'll be mapping paths by hand.

---

## 6. Host it in VS Code (the reading side)

This is the direction that matters most day to day: you're reading unfamiliar
code and want to know *why it's like that*.

### Option A — install it for real (what you want most of the time)

The extension has **no third-party dependencies**, so installing is just a
file copy.

```bash
cd vscode-extension
npm install          # dev-only: TypeScript + @types/vscode
npx tsc -p ./        # compiles src/ -> out/
```

Then copy `package.json`, `out/`, and `media/` into a folder under your VS Code
extensions directory:

| OS | Path |
|---|---|
| Windows | `%USERPROFILE%\.vscode\extensions\local.decision-ledger-0.1.0` |
| macOS / Linux | `~/.vscode/extensions/local.decision-ledger-0.1.0` |

```bash
DST=~/.vscode/extensions/local.decision-ledger-0.1.0
mkdir -p "$DST" && cp -r package.json out media "$DST/"
```

Restart VS Code.

### Option B — run it in the Extension Development Host (for hacking on it)

Open the `vscode-extension/` folder in VS Code and press **F5**. A second VS
Code window launches with the extension loaded, and you can set breakpoints in
`src/extension.ts`. Use this when changing the extension itself.

### Point it at the engine

Add to your workspace `.vscode/settings.json` (or your user settings):

```json
{
  "decisionLedger.binaryPath": "C:/Users/you/tools/ledger.exe",
  "decisionLedger.showCodeLens": true
}
```

Use forward slashes even on Windows — they work fine and avoid JSON escaping
mistakes.

### What you get

**Reading** — open a file that has a bound decision:

- **Gutter dot** next to every governed line — green tracked, amber drifted,
  red gone
- **CodeLens** above the block: `why: Jittered backoff avoids thundering herd`
  — click to open the decision note
- **Hover** over the lines for the status, confidence, and a link to the note
- **Command palette → "Why does this code exist?"** for the decision at your
  cursor

**Writing** — select the lines that embody a decision, right-click, and choose
**"Bind selection to a decision"**. Pick *New decision…* or an existing one to
add another anchor. If the note doesn't exist yet it's created from a template
and opened, so you can write the rationale while it's still in your head.

That last part matters more than it sounds. If capturing a decision costs more
than a right-click, nobody does it and the ledger stays empty.

The extension shells out to `ledger list --json` in your workspace root and
refreshes on save. If nothing appears, that command is the thing to debug —
run it yourself in the same folder.

---

## 7. Turn on the CI gate (the part that makes it stick)

Docs rot because nothing enforces them. This is the enforcement.

```bash
ledger verify --since <base-branch-sha>
```

It fails the build when a decision's governed code changed between the base
and your branch **but the decision note wasn't touched**. You either update the
rationale or explicitly confirm the move — the build won't go green otherwise.

A ready-made workflow lives at `.github/workflows/decisions.yml`:

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0     # REQUIRED — the gate needs history
- run: go build -o ledger ./cmd/ledger
- run: ./ledger verify --since "${{ github.event.pull_request.base.sha }}"
```

Plain `ledger verify` (no `--since`) is a health check: it fails if any
decision's code is *gone*. Add `--strict` to also fail on drift.

---

## 8. Command cheat sheet

| Command | What it does |
|---|---|
| `ledger init` | Scaffold `.ledger/`, `docs/decisions/`, CI workflow |
| `ledger bind <file>:<start>-<end> --note <id>` | Anchor a decision to code (re-run to re-anchor) |
| `ledger bind ... --note <id> --add` | Add another anchor to the same decision |
| `ledger bind ... --note-file <path>` | Where the rationale lives (default `docs/decisions/<id>.md`) |
| `ledger resolve <id>` | Where is this decision's code now? |
| `ledger why <file>[:<line>[-<end>]]` | Which decisions govern this file, line, or range? |
| `ledger list` | Status of every decision |
| `ledger verify` | Fail if any decision's code is gone |
| `ledger verify --since <sha>` | Fail if code changed without revisiting the rationale |

Add `--json` to `resolve`, `list`, or `why` for machine-readable output — it's
the same interface the editor integrations use.

Want to see all of it at once? Run `./demo.sh`. It builds a throwaway repo and
walks the whole lifecycle: bind, refactor, rename, a blocked PR, a passing PR,
and a decision breaking when its code is deleted.

---

## 9. Troubleshooting

**`spawn ledger ENOENT`**
The integration can't find the binary. Set the full path in settings, or put
`ledger` on your `PATH`.

**Settings seem ignored / it falls back to defaults**
Your settings JSON is probably malformed — a common one on Windows is
single-backslash paths, which are invalid JSON escapes. Use forward slashes:
`"C:/Users/you/ledger.exe"`.

**Everything says "code gone" right after cloning**
The gate needs real history. In CI, set `fetch-depth: 0`; a shallow clone has
no commits to diff against.

**Every change is flagged stale even though you updated the note**
The record is watching a different path than the note you edited. Run
`ledger resolve <id>` — the gate watches the `note` field in
`.ledger/<id>.json`. Re-bind with `--note-file <your/path.md>` to correct it.

**A decision shows "drifted" but the code looks fine**
Whitespace-only changes are ignored, but any real content edit inside the span
counts. That's intended — re-read it and re-bind if the decision still holds.

**Everything reads "drifted" all the time**
You're binding spans that are too large or too churn-heavy. Anchor the few
lines that actually embody the decision, not the whole function.

**The Obsidian plugin doesn't appear**
Restricted mode is on. Settings → Community plugins → turn it off, then enable
the plugin.

---

## 10. How to use it well

- **Bind at commit time.** If capturing the decision isn't near-zero effort in
  the moment, nobody does it. Make it part of writing the code.
- **Anchor small.** The 3–5 lines that *are* the decision, not the file.
- **Not everything needs a decision.** Reserve it for choices a smart teammate
  would otherwise undo by accident.
- **Treat red as a prompt, not a chore.** "Code gone" is the tool asking a
  real question: is this decision still true?

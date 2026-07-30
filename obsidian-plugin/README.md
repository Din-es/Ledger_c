# Decision Ledger — Obsidian plugin

Authoring surface for the `ledger` engine. Notes become windows onto the code
they govern instead of stale snapshots.

## Build

```bash
npm install
npm run build      # tsc typecheck + esbuild bundle -> main.js
npm run dev        # watch mode
```

Then copy `main.js`, `manifest.json`, and `styles.css` into
`<vault>/.obsidian/plugins/decision-ledger/` and enable it in Obsidian.

## Setup

In plugin settings, point **Ledger binary** at the compiled `ledger`
executable and **Repository path** at the git repo holding `.ledger/` records.

## Use

Put a block in any decision note:

````markdown
```ledger
id: jitter-backoff
```
````

It renders the live code that decision governs, resolved at read time, with a
status pill:

- **tracked** — the anchor still matches its code
- **drifted NN%** — relocated by similarity; the code changed since binding
- **code gone** — the governed code no longer exists, so the decision is
  probably stale too

The id can also come from the note's frontmatter (`ledger-id: jitter-backoff`),
in which case the block body can be empty.

## Commands

- **Open decision ledger** — staleness sidebar, worst first
- **Resolve all decisions** — re-resolve against the working tree
- **Show stale decisions** — notice listing anything not `fresh`

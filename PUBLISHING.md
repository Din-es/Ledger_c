# Publishing

Everything is wired for `github.com/Din-es/Ledger_c`. This is the order to do
it in — each step stands alone, so you can stop after step 1.

## 1. GitHub (do this first)

Create the repo at <https://github.com/Din-es/Ledger_c>, then:

```bash
git add -A
git commit -m "ledger v1.0.0"
git branch -M main
git remote add origin https://github.com/Din-es/Ledger_c.git
git push -u origin main
```

`.github/workflows/ci.yml` then runs tests on Linux, macOS and Windows for
every push and PR.

## 2. Releases and `go install`

Tag a version and push the tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

`.github/workflows/release.yml` builds binaries for linux/amd64, linux/arm64,
darwin/amd64, darwin/arm64 and windows/amd64, publishes them with checksums,
attaches the Obsidian plugin files, and uploads the packaged `.vsix`.

`go install` works the moment the tag is public — no registry, no account:

```bash
go install github.com/Din-es/Ledger_c/cmd/ledger@latest
```

> The module path has capitals (`Din-es`, `Ledger_c`). That is legal; the Go
> proxy case-encodes it internally. Users still type the path exactly as
> above.

## 3. VS Code Marketplace

One-time setup:

1. Create a free Azure DevOps organisation at <https://dev.azure.com>.
2. Generate a Personal Access Token with **Marketplace → Manage** scope.
3. Create the publisher `Din-es` at
   <https://marketplace.visualstudio.com/manage>.

Then, from `vscode-extension/`:

```bash
npx @vscode/vsce login Din-es
npx @vscode/vsce publish
```

Before the first publish, add a 128×128 PNG icon and set `"icon"` in
`package.json` — listings without one look abandoned.

## 4. Obsidian community plugins

The store is a curated list, reviewed by hand, and it takes a few weeks.

1. Make sure a GitHub release exists whose tag exactly matches the
   `version` in `manifest.json` (`1.0.0`), with `main.js`, `manifest.json`
   and `styles.css` attached as loose files. The release workflow already
   does this.
2. Fork <https://github.com/obsidianmd/obsidian-releases>.
3. Add an entry to `community-plugins.json`:

```json
{
  "id": "decision-ledger",
  "name": "Decision Ledger",
  "author": "Dinesh",
  "description": "Bind decisions to the code they govern and see when that code drifts.",
  "repo": "Din-es/Ledger_c"
}
```

4. Open a PR and answer the reviewer's comments.

Note the plugin needs the `ledger` binary installed separately and is
desktop-only. Say so plainly in the plugin README — reviewers check, and
users will otherwise file bugs about the `spawn ledger ENOENT` error.

## 5. Package managers (later)

Once releases exist with stable checksums:

- **Scoop** (Windows): a manifest JSON pointing at the release zip.
- **Homebrew** (macOS/Linux): a formula in a `homebrew-tap` repo.
- **winget**: a manifest PR to `microsoft/winget-pkgs`.

None of these are worth doing until people are actually asking for them.

## Before you announce it

- Add a short demo GIF to the README — this tool is much easier to show
  than to explain.
- The honest pitch is the CI gate, not the note-taking. Lead with
  *"your build fails when code changes without its decision being
  revisited."*
- It is one release old and has only run on small repos. Say that. It buys
  goodwill and sets up the bug reports you actually want.

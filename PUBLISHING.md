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
2. Create a Personal Access Token at
   <https://go.microsoft.com/fwlink/?LinkId=307137> — set **Organization** to
   *All accessible organizations* and **Scopes** to *Custom defined → Show all
   scopes → Marketplace (Manage)*. Anything narrower fails at publish time.
3. Create the publisher `Din-es` at
   <https://marketplace.visualstudio.com/manage>.

Then, from `vscode-extension/`:

```bash
npx @vscode/vsce login Din-es
npx @vscode/vsce publish
```

Docs: [Publishing an extension](https://code.visualstudio.com/api/working-with-extensions/publishing-extension)

Nothing is blocking this one — `package.json` already has every required field
(`name`, `version`, `publisher`, `engines.vscode`) and every recommended one
(`description`, `icon`, `license`, `repository`). The account is the only
missing piece.

**No account needed to test or share.** `npx @vscode/vsce package` produces a
`.vsix` that anyone can install with
`code --install-extension decision-ledger-1.1.0.vsix`. Worth doing before the
public listing.

## 4. Obsidian community plugins

**The submission process changed.** It is no longer a pull request to
`obsidianmd/obsidian-releases`. You now submit through the community directory:

1. Sign in at <https://community.obsidian.md> with your Obsidian account.
2. Link your GitHub account so it can verify you own the repo.
3. Choose **New plugin**, give it the repository URL, accept the developer
   policies.
4. Answer review feedback by pushing fixes and cutting new releases.

Docs: [Submit your plugin](https://docs.obsidian.md/Plugins/Releasing/Submit+your+plugin)
· [Plugin guidelines](https://docs.obsidian.md/Plugins/Releasing/Plugin+guidelines)

### Two things block us today

**a. The tag must not have a `v` prefix.** Obsidian requires a GitHub release
whose tag *exactly* matches `version` in `manifest.json` — so `1.1.0`, not
`v1.1.0`. Our release tags are `v`-prefixed, which is right for Go but wrong
for Obsidian. Either publish an additional un-prefixed tag for the plugin, or
release the plugin from its own repo with its own tagging scheme.

**b. Obsidian expects the repo root to *be* the plugin.** The directory reads
`manifest.json` from the default branch HEAD, and the required root files are
`README.md`, `LICENSE` and `manifest.json`. Ours live in `obsidian-plugin/`,
because this repo is primarily a Go tool.

**Recommendation: give the plugin its own repository** (e.g.
`Din-es/obsidian-decision-ledger`) with the plugin at its root. That is the
shape every community plugin has, it sidesteps both problems, and it keeps the
Go release cadence independent of the plugin's. Mirror or submodule the source
if you would rather keep one place to edit.

### Before submitting

The plugin needs the `ledger` binary installed separately and is desktop-only.
Say so in the first paragraph of the plugin README — reviewers check, and
users will otherwise file bugs about `spawn ledger ENOENT`.

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

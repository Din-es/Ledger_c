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
and uploads the packaged `.vsix`. The Obsidian plugin releases from its own
repository (see step 4).

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

The plugin lives in its own repository:
**<https://github.com/Din-es/obsidian-decision-ledger>**

It was split out because Obsidian's community directory reads `manifest.json`
from the repository root and requires a release tag matching it exactly with no
`v` prefix — neither of which works from inside this Go monorepo. That repo
already tags `1.1.0` (un-prefixed) and attaches `main.js`, `manifest.json` and
`styles.css` loose on the release, which is what the directory expects.

To submit:

1. Sign in at <https://community.obsidian.md> with your Obsidian account.
2. Link your GitHub account so it can verify you own the repo.
3. Choose **New plugin**, give it
   `https://github.com/Din-es/obsidian-decision-ledger`, accept the developer
   policies.
4. Answer review feedback by pushing fixes and cutting new releases.

Docs: [Submit your plugin](https://docs.obsidian.md/Plugins/Releasing/Submit+your+plugin)
· [Plugin guidelines](https://docs.obsidian.md/Plugins/Releasing/Plugin+guidelines)

Reviewers check that the README is upfront about limitations. Its first
paragraph already says the plugin needs the `ledger` binary installed
separately and is desktop-only.

## 5. Package managers (later)

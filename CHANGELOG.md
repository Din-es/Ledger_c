# Changelog

## v1.1.0

**Added**

- `ledger why` accepts a line range: `ledger why file.go:10-40` reports every
  decision whose span overlaps those lines, which is what you want when
  reviewing a diff. The whole-file and single-line forms are unchanged.
  Thanks to [@JasonColapietro](https://github.com/JasonColapietro) for the
  contribution (#3, #11).

## v1.0.1

Use this instead of v1.0.0.

**Fixed**

- `bind` now stores anchor paths git-style with forward slashes. Records are
  committed and shared, so a Windows developer binding `src\f.go` previously
  wrote a path that no teammate on Linux or macOS could resolve.
- CI passes on Linux and macOS. A test asserted that `.\retry.go` normalises,
  which only holds on Windows — on Unix a backslash is an ordinary filename
  character. That failure also blocked the release job, so v1.0.0 shipped
  without any binaries.

**Changed**

- All sources gofmt-clean; CI now enforces it.
- CI matrix uses `fail-fast: false`, so one platform failing no longer hides
  the others.

## v1.0.0

Withdrawn. The release carried no binaries, and anchors bound on Windows were
unusable on other platforms. The Go module proxy caches versions immutably, so
this tag cannot be corrected in place — hence v1.0.1.

First cut of everything: anchor engine (diff-shift, fuzzy relocate,
rename-follow), the `verify --since` CI gate, multi-anchor decisions, an
Obsidian plugin, and a VS Code extension.

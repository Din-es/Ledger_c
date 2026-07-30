package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/Din-es/Ledger_c/internal/ledger"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "bind":
		err = cmdBind(os.Args[2:])
	case "resolve":
		err = cmdResolve(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "list":
		err = cmdList(os.Args[2:])
	case "why":
		err = cmdWhy(os.Args[2:])
	case "init":
		err = cmdInit(os.Args[2:])
	case "-v", "--version", "version":
		fmt.Println("ledger", version)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const version = "1.0.1"

func usage() {
	fmt.Fprint(os.Stderr, `ledger — decisions that can't silently rot

usage:
  ledger init                                       scaffold a repo for decisions
  ledger bind <file>:<start>-<end> --note <id>      anchor a decision to code
       [--title "..."] [--add]                      --add appends another anchor
       [--note-file <path>]                          where the rationale lives
                                                    (default docs/decisions/<id>.md)
  ledger resolve <id> [--json]                      where is this code now?
  ledger why <file>[:<line>] [--json]               which decision governs this?
  ledger list [--json]                              status of every decision
  ledger verify [--since <base>] [--strict]         CI gate

exit codes:
  0  all good        1  gate failed or error        2  bad usage
`)
}

var locRe = regexp.MustCompile(`^(.+):(\d+)-(\d+)$`)

func cmdBind(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("bind needs a <file>:<start>-<end> location")
	}
	m := locRe.FindStringSubmatch(args[0])
	if m == nil {
		return fmt.Errorf("bad location %q, want file:start-end", args[0])
	}
	// Records are committed and shared, so store the path the way git does —
	// forward slashes. A Windows dev binding "src\f.go" would otherwise write
	// a path no teammate on Linux or macOS can resolve.
	file := filepath.ToSlash(filepath.Clean(m[1]))
	start, _ := strconv.Atoi(m[2])
	end, _ := strconv.Atoi(m[3])
	if start < 1 || end < start {
		return fmt.Errorf("bad range %d-%d", start, end)
	}

	id, title, notePath := "", "", ""
	add := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--note":
			i++
			if i < len(args) {
				id = args[i]
			}
		case "--title":
			i++
			if i < len(args) {
				title = args[i]
			}
		case "--note-file":
			i++
			if i < len(args) {
				notePath = filepath.ToSlash(filepath.Clean(args[i]))
			}
		case "--add":
			add = true
		}
	}
	if id == "" {
		return fmt.Errorf("bind needs --note <id>")
	}

	head, err := ledger.HeadSHA()
	if err != nil {
		return err
	}
	lines, err := ledger.FileAtWorktree(file)
	if err != nil {
		return err
	}
	if end > len(lines) {
		return fmt.Errorf("range end %d past end of file (%d lines)", end, len(lines))
	}

	body := lines[start-1 : end]
	before := lines[max(0, start-1-ledger.ContextLines) : start-1]
	after := lines[end:min(len(lines), end+ledger.ContextLines)]

	anchor := ledger.Anchor{
		File:        file,
		Range:       [2]int{start, end},
		Commit:      head,
		Fingerprint: ledger.Fingerprint(body),
		Before:      before,
		After:       after,
		Body:        body,
	}

	// The note path is what the CI gate watches for "was the rationale
	// revisited?". Default to the conventional location, but keep whatever a
	// re-bind already had so a custom path survives re-anchoring — otherwise
	// the gate silently starts watching a file that does not exist.
	existing, existErr := ledger.Load(id)
	if notePath == "" {
		if existErr == nil && existing.Note != "" {
			notePath = existing.Note
		} else {
			notePath = "docs/decisions/" + id + ".md"
		}
	}

	// --add appends to an existing decision so one rationale can govern code
	// in several places. Without it, re-binding an id replaces the record —
	// that is the re-anchor path used to clear a drift.
	rec := &ledger.LinkRecord{
		ID:      id,
		Title:   title,
		Note:    notePath,
		Created: time.Now().UTC().Format(time.RFC3339),
		Anchors: []ledger.Anchor{anchor},
	}
	replaced := false
	if add {
		if existErr != nil {
			return fmt.Errorf("--add needs an existing decision: %w", existErr)
		}
		if title != "" {
			existing.Title = title
		}
		existing.Note = notePath
		// Re-adding the same span re-anchors that site rather than stacking a
		// duplicate, which would double-count it in every report.
		for i, a := range existing.Anchors {
			if a.File == anchor.File && a.Range == anchor.Range {
				existing.Anchors[i] = anchor
				replaced = true
				break
			}
		}
		if !replaced {
			existing.Anchors = append(existing.Anchors, anchor)
		}
		rec = existing
	}
	if err := rec.Save(); err != nil {
		return err
	}
	verb := "bound"
	switch {
	case replaced:
		verb = "re-anchored"
	case add:
		verb = fmt.Sprintf("added anchor %d to", len(rec.Anchors))
	}
	fmt.Printf("%s %s → %s:%d-%d @ %s\n", verb, id, file, start, end, short(head))
	fmt.Printf("   rationale: %s\n", rec.Note)
	return nil
}

// cmdInit scaffolds a repo so a team can start recording decisions.
func cmdInit(args []string) error {
	if _, err := ledger.HeadSHA(); err != nil {
		return fmt.Errorf("not a git repository (or no commits yet)")
	}
	for _, dir := range []string{ledger.LedgerDir, "docs/decisions"} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	// Keep .ledger/ in git even before the first decision exists.
	keep := ledger.LedgerDir + "/.gitkeep"
	if _, err := os.Stat(keep); os.IsNotExist(err) {
		if err := os.WriteFile(keep, nil, 0o644); err != nil {
			return err
		}
	}

	wf := ".github/workflows/decisions.yml"
	if _, err := os.Stat(wf); os.IsNotExist(err) {
		if err := os.MkdirAll(".github/workflows", 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(wf, []byte(workflowYAML), 0o644); err != nil {
			return err
		}
		fmt.Println("created", wf)
	} else {
		fmt.Println("kept existing", wf)
	}

	fmt.Printf(`created %s/ and docs/decisions/

next:
  1. write docs/decisions/<id>.md explaining a decision
  2. ledger bind <file>:<start>-<end> --note <id> --title "..."
  3. commit the .ledger/<id>.json alongside your code
`, ledger.LedgerDir)
	return nil
}

const workflowYAML = `name: decision ledger

on:
  pull_request:

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
        with:
          fetch-depth: 0            # full history so the base commit is available

      - uses: actions/setup-go@v6
        with:
          go-version: "1.26"

      - name: build ledger
        run: go build -o ledger ./cmd/ledger

      - name: verify decisions vs. their code
        run: ./ledger verify --since "${{ github.event.pull_request.base.sha }}"
`

func cmdResolve(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("resolve needs a decision id")
	}
	rec, err := ledger.Load(args[0])
	if err != nil {
		return err
	}
	if hasFlag(args, "--json") {
		return emitJSON(rec.Reports())
	}

	for _, a := range rec.Anchors {
		res := ledger.Resolve(a)
		loc := "—"
		if res.Range[1] > 0 {
			loc = fmt.Sprintf("%s:%d-%d", res.File, res.Range[0], res.Range[1])
		}
		fmt.Printf("%s  %-8s  %s  conf %.2f\n",
			icon(res.Status), res.Status, loc, res.Confidence)
		if res.Renamed {
			fmt.Printf("   followed rename: %s → %s\n", a.File, res.File)
		}
		if res.Status == ledger.StatusBroken {
			fmt.Printf("   was %s:%d-%d @ %s — the code this decision governed is gone.\n",
				a.File, a.Range[0], a.Range[1], short(a.Commit))
		}
	}
	return nil
}

func cmdList(args []string) error {
	recs, skipped, err := ledger.AllRecords()
	if err != nil {
		return err
	}
	warnSkipped(skipped)
	var reports []ledger.Report
	for _, r := range recs {
		reports = append(reports, r.Reports()...)
	}
	if hasFlag(args, "--json") {
		if reports == nil {
			reports = []ledger.Report{}
		}
		return emitJSON(reports)
	}
	if len(reports) == 0 {
		fmt.Println("no decisions recorded")
		return nil
	}
	for _, rp := range reports {
		fmt.Printf("%s %-24s %s:%d-%d  conf %.2f\n",
			icon(rp.Status), rp.ID, rp.File, rp.Range[0], rp.Range[1], rp.Confidence)
	}
	return nil
}

var whyRe = regexp.MustCompile(`^(.+?)(?::(\d+))?$`)

// cmdWhy answers "why does this code exist?" — the reverse of bind.
func cmdWhy(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("why needs a <file>[:<line>]")
	}
	m := whyRe.FindStringSubmatch(args[0])
	if m == nil {
		return fmt.Errorf("bad location %q, want file[:line]", args[0])
	}
	file := m[1]
	line := 0
	if m[2] != "" {
		line, _ = strconv.Atoi(m[2])
	}

	recs, skipped, err := ledger.AllRecords()
	if err != nil {
		return err
	}
	warnSkipped(skipped)
	var all []ledger.Report
	for _, r := range recs {
		all = append(all, r.Reports()...)
	}
	hits := ledger.Governing(all, file, line)

	if hasFlag(args, "--json") {
		if hits == nil {
			hits = []ledger.Report{}
		}
		return emitJSON(hits)
	}
	if len(hits) == 0 {
		fmt.Printf("no decision governs %s\n", args[0])
		return nil
	}
	for _, rp := range hits {
		fmt.Printf("%s %s\n", icon(rp.Status), rp.Title)
		fmt.Printf("   %s:%d-%d · bound @ %s · %s\n",
			rp.File, rp.Range[0], rp.Range[1], short(rp.BoundAt), rp.Note)
	}
	return nil
}

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func cmdVerify(args []string) error {
	var since string
	var strict bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--since":
			i++
			if i < len(args) {
				since = args[i]
			}
		case "--strict":
			strict = true
		}
	}

	recs, skipped, err := ledger.AllRecords()
	if err != nil {
		return err
	}
	warnSkipped(skipped)
	if len(recs) == 0 {
		fmt.Println("no decisions recorded")
		return nil
	}

	// --since is the PR gate; the health check is what plain verify does.
	// Asking for both (--since --strict) runs both and fails if either does.
	ok := true
	if since != "" {
		if !verifySince(recs, since) {
			ok = false
		}
	}
	if since == "" || strict {
		if since != "" {
			fmt.Println()
		}
		if !verifyHealth(recs, strict) {
			ok = false
		}
	}
	// A corrupt record is a failure too — silently ignoring it would let a
	// decision disappear from the gate by being broken rather than deleted.
	if len(skipped) > 0 || !ok {
		os.Exit(1)
	}
	return nil
}

func warnSkipped(skipped []error) {
	for _, e := range skipped {
		fmt.Fprintln(os.Stderr, "warning: skipping unreadable record:", e)
	}
}

// verifyHealth resolves every anchor against the working tree. It reports ok
// = false when any decision governs code that has broken (or, with --strict,
// merely drifted).
func verifyHealth(recs []*ledger.LinkRecord, strict bool) (ok bool) {
	var broken, drifted int
	for _, rec := range recs {
		for _, a := range rec.Anchors {
			res := ledger.Resolve(a)
			fmt.Printf("%s %-24s %s:%d-%d  conf %.2f\n",
				icon(res.Status), rec.ID, a.File, res.Range[0], res.Range[1], res.Confidence)
			switch res.Status {
			case ledger.StatusBroken:
				broken++
			case ledger.StatusDrifted:
				drifted++
			}
		}
	}
	fmt.Printf("\n%d decision(s): %d broken, %d drifted\n", len(recs), broken, drifted)
	if broken > 0 || (strict && drifted > 0) {
		fmt.Fprintln(os.Stderr, "verify failed: decisions no longer match their code")
		return false
	}
	return true
}

// verifySince is the CI gate: for changes between <base> and HEAD, fail when a
// decision's governed code was modified but its rationale (the note or the
// record itself) was not touched in the same range of history.
func verifySince(recs []*ledger.LinkRecord, base string) (ok bool) {
	changed, err := ledger.ChangedFiles(base, "HEAD")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return false
	}
	var stale, touched int
	for _, rec := range recs {
		recordPath := ".ledger/" + rec.ID + ".json"
		rationaleTouched := changed[rec.Note] || changed[recordPath]
		for _, a := range rec.Anchors {
			codeChanged, brokenAtBase := ledger.CodeChanged(a, base, "HEAD")
			if brokenAtBase || !codeChanged {
				continue
			}
			touched++
			if rationaleTouched {
				fmt.Printf("[ok]    %-24s code changed, rationale revisited\n", rec.ID)
			} else {
				stale++
				fmt.Printf("[STALE] %-24s %s changed since %s, but %s was not\n",
					rec.ID, a.File, short(base), rec.Note)
			}
		}
	}
	if stale == 0 {
		if touched == 0 {
			fmt.Printf("no governed code changed since %s\n", short(base))
		} else {
			fmt.Printf("\n%d touched decision(s) had their rationale revisited\n", touched)
		}
		return true
	}
	fmt.Fprintf(os.Stderr, "\nverify failed: %d decision(s) changed without revisiting the rationale\n", stale)
	return false
}

func icon(status string) string {
	switch status {
	case ledger.StatusFresh:
		return "[fresh]"
	case ledger.StatusDrifted:
		return "[drift]"
	default:
		return "[BROKE]"
	}
}

func short(sha string) string {
	if len(sha) >= 7 {
		return sha[:7]
	}
	return sha
}

package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const sample = `package auth

import "time"

func retry(fn func() error) error {
	var err error
	for i := 0; i < 5; i++ {
		if err = fn(); err == nil {
			return nil
		}
		// jittered backoff: base*2^i +/- 25% to avoid thundering herd
		d := backoff(i)
		time.Sleep(d)
	}
	return err
}
`

// anchoredRange is the jitter block in sample: lines 11-13.
var anchoredRange = [2]int{11, 13}

// newRepo creates a temp git repo with sample content committed, chdirs into
// it for the duration of the test, and returns a commit helper.
func newRepo(t *testing.T) func(name string) {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t.co"},
		{"config", "user.name", "t"},
		{"config", "core.autocrlf", "false"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	write(t, "retry.go", sample)
	commit := func(name string) {
		t.Helper()
		if out, err := exec.Command("git", "add", "-A").CombinedOutput(); err != nil {
			t.Fatalf("git add: %v: %s", err, out)
		}
		if out, err := exec.Command("git", "commit", "-qm", name).CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v: %s", err, out)
		}
	}
	commit("init")
	return commit
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bindSample anchors the jitter block at the current HEAD.
func bindSample(t *testing.T) Anchor {
	t.Helper()
	head, err := HeadSHA()
	if err != nil {
		t.Fatal(err)
	}
	lines, err := FileAtWorktree("retry.go")
	if err != nil {
		t.Fatal(err)
	}
	body := lines[anchoredRange[0]-1 : anchoredRange[1]]
	return Anchor{
		File:        "retry.go",
		Range:       anchoredRange,
		Commit:      head,
		Fingerprint: Fingerprint(body),
		Body:        body,
	}
}

func TestResolveUnchanged(t *testing.T) {
	newRepo(t)
	a := bindSample(t)

	res := Resolve(a)
	if res.Status != StatusFresh || res.Range != anchoredRange {
		t.Fatalf("want fresh at %v, got %s at %v", anchoredRange, res.Status, res.Range)
	}
}

func TestResolveShiftsPastInsertionAbove(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)

	write(t, "retry.go", "// header\n// header\n// header\n"+sample)
	commit("insert above")

	res := Resolve(a)
	if res.Status != StatusFresh {
		t.Fatalf("want fresh, got %s (conf %.2f)", res.Status, res.Confidence)
	}
	want := [2]int{anchoredRange[0] + 3, anchoredRange[1] + 3}
	if res.Range != want {
		t.Fatalf("want range %v, got %v", want, res.Range)
	}
	if res.Confidence != 1.0 {
		t.Fatalf("want confidence 1.0, got %.2f", res.Confidence)
	}
}

func TestResolveShiftsPastDeletionAbove(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)

	// Drop the import block (lines 3-4), shifting the anchor up by 2.
	lines := strings.Split(sample, "\n")
	write(t, "retry.go", strings.Join(append(append([]string{}, lines[:2]...), lines[4:]...), "\n"))
	commit("drop import")

	res := Resolve(a)
	want := [2]int{anchoredRange[0] - 2, anchoredRange[1] - 2}
	if res.Status != StatusFresh || res.Range != want {
		t.Fatalf("want fresh at %v, got %s at %v", want, res.Status, res.Range)
	}
}

func TestResolveDetectsDriftWhenBodyEdited(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)

	write(t, "retry.go", strings.Replace(sample, "25%", "50%", 1))
	commit("widen jitter")

	res := Resolve(a)
	if res.Status != StatusDrifted {
		t.Fatalf("want drifted, got %s (conf %.2f)", res.Status, res.Confidence)
	}
	if res.Range != anchoredRange {
		t.Fatalf("want relocate to %v, got %v", anchoredRange, res.Range)
	}
	if res.Confidence <= 0 || res.Confidence >= 1 {
		t.Fatalf("want partial confidence, got %.2f", res.Confidence)
	}
}

func TestResolveBreaksWhenBodyDeleted(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)

	lines := strings.Split(sample, "\n")
	write(t, "retry.go", strings.Join(append(append([]string{}, lines[:10]...), lines[13:]...), "\n"))
	commit("remove jitter")

	res := Resolve(a)
	if res.Status != StatusBroken {
		t.Fatalf("want broken, got %s (conf %.2f)", res.Status, res.Confidence)
	}
}

func TestResolveBreaksWhenFileDeleted(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)

	if err := os.Remove("retry.go"); err != nil {
		t.Fatal(err)
	}
	commit("delete file")

	if res := Resolve(a); res.Status != StatusBroken {
		t.Fatalf("want broken, got %s", res.Status)
	}
}

func TestResolveFollowsRename(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)

	write(t, "internal/auth/retry.go", sample)
	if err := os.Remove("retry.go"); err != nil {
		t.Fatal(err)
	}
	commit("move retry.go into internal/auth")

	res := Resolve(a)
	if res.Status != StatusFresh {
		t.Fatalf("want fresh after rename, got %s (conf %.2f)", res.Status, res.Confidence)
	}
	if !res.Renamed {
		t.Error("want Renamed=true")
	}
	want := filepath.ToSlash("internal/auth/retry.go")
	if filepath.ToSlash(res.File) != want {
		t.Fatalf("want file %s, got %s", want, res.File)
	}
	if res.Range != anchoredRange {
		t.Fatalf("want range %v, got %v", anchoredRange, res.Range)
	}
}

func TestCodeChangedGate(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)
	base, err := HeadSHA()
	if err != nil {
		t.Fatal(err)
	}

	// An unrelated change must not trip the gate.
	write(t, "README.md", "# unrelated\n")
	commit("add readme")
	if changed, broken := CodeChanged(a, base, "HEAD"); changed || broken {
		t.Fatalf("unrelated change tripped the gate (changed=%v broken=%v)", changed, broken)
	}

	// Editing the governed span must trip it.
	write(t, "retry.go", strings.Replace(sample, "25%", "50%", 1))
	commit("widen jitter")
	if changed, _ := CodeChanged(a, base, "HEAD"); !changed {
		t.Fatal("editing the governed span did not trip the gate")
	}
}

func TestCodeChangedIgnoresEditsElsewhereInFile(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)
	base, err := HeadSHA()
	if err != nil {
		t.Fatal(err)
	}

	write(t, "retry.go", strings.Replace(sample, "package auth", "package auth // edited", 1))
	commit("touch unrelated line")

	if changed, broken := CodeChanged(a, base, "HEAD"); changed || broken {
		t.Fatalf("edit outside the span tripped the gate (changed=%v broken=%v)", changed, broken)
	}
}

// Resolving must never mutate a record. An earlier version cached the last
// resolution in the record file, which `git commit -a` then staged — making
// the CI gate read it as "the rationale was revisited" and pass a PR it should
// have blocked.
func TestResolveDoesNotMutateRecord(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)
	rec := &LinkRecord{ID: "jitter", Note: "docs/jitter.md", Anchors: []Anchor{a}}
	if err := rec.Save(); err != nil {
		t.Fatal(err)
	}
	commit("record decision")

	before, err := os.ReadFile(filepath.Join(LedgerDir, "jitter.json"))
	if err != nil {
		t.Fatal(err)
	}

	write(t, "retry.go", strings.Replace(sample, "25%", "50%", 1))
	commit("widen jitter")
	rec.Reports() // the read path the CLI and plugin both use

	after, err := os.ReadFile(filepath.Join(LedgerDir, "jitter.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("resolving rewrote the record file; the CI gate would misread this as a revisited rationale")
	}

	out, err := exec.Command("git", "status", "--porcelain", LedgerDir).Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("resolving dirtied the ledger dir: %s", out)
	}
}

// A file that both moves and grows scores below git's default 50% rename
// threshold; the anchor must still follow it.
func TestResolveFollowsRenameWithHeavyEdit(t *testing.T) {
	commit := newRepo(t)
	a := bindSample(t)

	header := strings.Repeat("// boilerplate\n", 12)
	write(t, "internal/auth/retry.go", header+sample)
	if err := os.Remove("retry.go"); err != nil {
		t.Fatal(err)
	}
	commit("move and document")

	res := Resolve(a)
	if res.Status != StatusFresh {
		t.Fatalf("want fresh, got %s (conf %.2f)", res.Status, res.Confidence)
	}
	if !res.Renamed {
		t.Error("want Renamed=true")
	}
	want := [2]int{anchoredRange[0] + 12, anchoredRange[1] + 12}
	if res.Range != want {
		t.Fatalf("want range %v, got %v", want, res.Range)
	}
}

// When two candidate windows score the same similarity, the one nearest the
// expected position must win. Taking the first tie instead reports a span
// shifted off the real lines — which misplaces gutter icons and shows the
// wrong code in the note.
func TestRelocatePrefersNearestOnTie(t *testing.T) {
	body := []string{"alpha", "beta", "gamma", "delta"}
	// Windows at 1 and 2 both share exactly "beta" and "gamma" with body, so
	// both score 0.5. The anchor was at line 2, so line 2 must win.
	cur := []string{"filler", "ALPHA", "beta", "gamma", "DELTA", "trailing"}

	start, end, sim := relocate(cur, body, 2)
	if sim != 0.5 {
		t.Fatalf("expected a 0.5 tie between the two windows, got %.2f", sim)
	}
	if start != 2 || end != 5 {
		t.Fatalf("want the window nearest the anchor (2-5), got %d-%d", start, end)
	}
}

// The same tie, approached from the other side: when the anchor sits below
// both candidates, the lower-numbered window is the nearer one.
func TestRelocateTieBreakIsSymmetric(t *testing.T) {
	body := []string{"alpha", "beta", "gamma", "delta"}
	cur := []string{"filler", "ALPHA", "beta", "gamma", "DELTA", "trailing"}

	if start, _, _ := relocate(cur, body, 1); start != 1 {
		t.Fatalf("with the anchor at 1, want window 1, got %d", start)
	}
}

// One decision often governs code in more than one place. Every anchor must
// resolve independently, and a change to any of them must be visible.
func TestMultiAnchorRecord(t *testing.T) {
	commit := newRepo(t)
	first := bindSample(t)

	head, err := HeadSHA()
	if err != nil {
		t.Fatal(err)
	}
	lines, err := FileAtWorktree("retry.go")
	if err != nil {
		t.Fatal(err)
	}
	secondBody := lines[4:6] // the func signature and its first line
	second := Anchor{
		File: "retry.go", Range: [2]int{5, 6}, Commit: head,
		Fingerprint: Fingerprint(secondBody), Body: secondBody,
	}

	rec := &LinkRecord{ID: "multi", Note: "docs/multi.md", Anchors: []Anchor{first, second}}
	if err := rec.Save(); err != nil {
		t.Fatal(err)
	}
	commit("record multi-anchor decision")

	reports := rec.Reports()
	if len(reports) != 2 {
		t.Fatalf("want 2 reports, got %d", len(reports))
	}
	for i, rp := range reports {
		if rp.Status != StatusFresh {
			t.Fatalf("anchor %d: want fresh, got %s", i, rp.Status)
		}
	}

	// Breaking only the second anchor must leave the first tracked.
	write(t, "retry.go", strings.Replace(sample, "func retry(fn func() error) error {", "func retry(f func() error) (err error) {", 1))
	commit("change the signature")

	reports = rec.Reports()
	if reports[0].Status != StatusFresh {
		t.Errorf("first anchor should be unaffected, got %s", reports[0].Status)
	}
	if reports[1].Status == StatusFresh {
		t.Errorf("second anchor should have changed, got fresh")
	}
}

// One corrupt record must not blind the tool to every other decision — that
// is exactly when you most need it working. It is reported, not swallowed.
func TestAllRecordsSkipsCorruptFiles(t *testing.T) {
	newRepo(t)
	good := &LinkRecord{ID: "good", Note: "docs/good.md", Anchors: []Anchor{bindSample(t)}}
	if err := good.Save(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(LedgerDir, "bad.json"), []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	recs, skipped, err := AllRecords()
	if err != nil {
		t.Fatalf("a corrupt record should not fail the whole call: %v", err)
	}
	if len(recs) != 1 || recs[0].ID != "good" {
		t.Fatalf("want the good record returned, got %d records", len(recs))
	}
	if len(skipped) != 1 {
		t.Fatalf("want 1 skipped record reported, got %d", len(skipped))
	}
	if !strings.Contains(skipped[0].Error(), "bad.json") {
		t.Errorf("skip error should name the file, got: %v", skipped[0])
	}
}

func TestGoverning(t *testing.T) {
	reports := []Report{
		{ID: "jitter", File: "retry.go", Range: [2]int{11, 13}, Status: StatusFresh},
		{ID: "loop", File: "retry.go", Range: [2]int{7, 14}, Status: StatusDrifted},
		{ID: "other", File: "other.go", Range: [2]int{1, 99}, Status: StatusFresh},
		{ID: "gone", File: "retry.go", Range: [2]int{0, 0}, Status: StatusBroken},
	}

	ids := func(rs []Report) []string {
		out := []string{}
		for _, r := range rs {
			out = append(out, r.ID)
		}
		return out
	}

	got := ids(Governing(reports, "retry.go", 12))
	if len(got) != 2 || got[0] != "jitter" || got[1] != "loop" {
		t.Fatalf("want [jitter loop] covering line 12, got %v", got)
	}
	if got := ids(Governing(reports, "retry.go", 8)); len(got) != 1 || got[0] != "loop" {
		t.Fatalf("want [loop] covering line 8, got %v", got)
	}
	if got := ids(Governing(reports, "retry.go", 50)); len(got) != 0 {
		t.Fatalf("want none covering line 50, got %v", got)
	}
	// Line 0 means "everything in this file"; broken anchors never match.
	if got := ids(Governing(reports, "retry.go", 0)); len(got) != 2 {
		t.Fatalf("want 2 for whole file (broken excluded), got %v", got)
	}
	// A "./" prefix must not matter anywhere.
	if got := ids(Governing(reports, "./retry.go", 12)); len(got) != 2 {
		t.Fatalf("leading ./ broke matching, got %v", got)
	}
	// Backslashes only separate paths on Windows; on Unix they are ordinary
	// filename characters, so normalising them there would be wrong.
	if runtime.GOOS == "windows" {
		if got := ids(Governing(reports, ".\\retry.go", 12)); len(got) != 2 {
			t.Fatalf("windows separators broke matching, got %v", got)
		}
	}
}

// Records are committed and shared between machines, so an anchor path must
// be stored git-style with forward slashes regardless of who bound it.
func TestAnchorPathsAreStoredPortable(t *testing.T) {
	newRepo(t)
	rec := &LinkRecord{ID: "p", Anchors: []Anchor{{File: "src/a/b.go", Range: [2]int{1, 2}}}}
	if err := rec.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load("p")
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Anchors[0].File; strings.Contains(got, `\`) {
		t.Fatalf("anchor path must use forward slashes, got %q", got)
	}
}

func TestFingerprintIgnoresTrailingWhitespace(t *testing.T) {
	a := Fingerprint([]string{"foo", "bar"})
	b := Fingerprint([]string{"foo  ", "bar\t"})
	if a != b {
		t.Fatal("trailing whitespace changed the fingerprint")
	}
	if a == Fingerprint([]string{"foo", "baz"}) {
		t.Fatal("different content produced the same fingerprint")
	}
}

func TestParseHunks(t *testing.T) {
	diff := "@@ -1 +1,3 @@\n@@ -10,4 +12,2 @@\n@@ -20,0 +21,5 @@\n"
	hunks := parseHunks(diff)
	if len(hunks) != 3 {
		t.Fatalf("want 3 hunks, got %d", len(hunks))
	}
	if hunks[0].oldCount != 1 || hunks[0].newCount != 3 {
		t.Errorf("omitted count should default to 1, got %+v", hunks[0])
	}
	if hunks[2].oldStart != 20 || hunks[2].oldCount != 0 {
		t.Errorf("pure insertion mis-parsed: %+v", hunks[2])
	}
}

func TestSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want float64
	}{
		{"identical", []string{"a", "b"}, []string{"a", "b"}, 1},
		{"disjoint", []string{"a", "b"}, []string{"x", "y"}, 0},
		{"half", []string{"a", "b"}, []string{"a", "z"}, 0.5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := similarity(tc.a, tc.b); got != tc.want {
				t.Fatalf("want %.2f, got %.2f", tc.want, got)
			}
		})
	}
}

func TestSliceLinesClampsBounds(t *testing.T) {
	lines := []string{"a", "b", "c"}
	if got := sliceLines(lines, 0, 99); len(got) != 3 {
		t.Fatalf("want clamped to 3 lines, got %d", len(got))
	}
	if got := sliceLines(lines, 3, 1); len(got) != 0 {
		t.Fatalf("want empty for inverted range, got %d", len(got))
	}
}

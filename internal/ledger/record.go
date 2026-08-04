package ledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LedgerDir is the repo-relative directory where link records are committed so
// they travel with every clone.
const LedgerDir = ".ledger"

// Anchor pins a decision to a span of code at a specific point in history.
// It is deliberately more than a line number: the commit is the ground truth,
// the fingerprint detects content change, and the context lets us relocate the
// span when the surrounding code drifts.
type Anchor struct {
	File        string   `json:"file"`        // repo-relative path
	Range       [2]int   `json:"range"`       // 1-based inclusive [start,end]
	Commit      string   `json:"commit"`      // SHA at bind time
	Fingerprint string   `json:"fingerprint"` // sha256 of normalized span
	Before      []string `json:"before"`      // up to N lines before the span
	After       []string `json:"after"`       // up to N lines after the span
	Body        []string `json:"body"`        // the span itself, verbatim
}

// Resolution is the last computed location of an anchor at some HEAD.
type Resolution struct {
	Commit     string  `json:"commit"`
	File       string  `json:"file"` // current path (differs if renamed)
	Range      [2]int  `json:"range"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"` // fresh | drifted | broken
	Renamed    bool    `json:"renamed,omitempty"`
}

// LinkRecord is one decision: prose lives in Note, the code it governs in
// Anchors.
//
// Records are deliberately immutable after bind. An earlier version cached the
// last resolution here, but writing to the record made `git commit -a` stage
// it — which the CI gate then read as "the rationale was revisited", silently
// defeating itself. Resolution is computed on demand instead.
type LinkRecord struct {
	ID      string   `json:"id"`
	Title   string   `json:"title,omitempty"`
	Note    string   `json:"note,omitempty"` // path to the decision prose
	Anchors []Anchor `json:"anchors"`
	Created string   `json:"created"`
}

// ContextLines is how many lines of surrounding code we capture on each side.
const ContextLines = 3

// Report is a resolved view of one decision, shaped for machine consumers
// (the Obsidian plugin) rather than the terminal.
type Report struct {
	ID         string   `json:"id"`
	Title      string   `json:"title,omitempty"`
	Note       string   `json:"note,omitempty"`
	File       string   `json:"file"`
	Range      [2]int   `json:"range"`
	Status     string   `json:"status"`
	Confidence float64  `json:"confidence"`
	Renamed    bool     `json:"renamed,omitempty"`
	BoundAt    string   `json:"boundAt"`
	Code       []string `json:"code,omitempty"` // live code at the resolved span
}

// Governing returns the reports whose resolved span covers the given file and
// line. A line of 0 matches every decision in the file. This is the code→note
// direction: "why does this code exist?"
func Governing(reports []Report, file string, line int) []Report {
	if line == 0 {
		return GoverningRange(reports, file, 0, 0)
	}
	return GoverningRange(reports, file, line, line)
}

// GoverningRange returns the reports whose resolved spans overlap the inclusive
// requested range. A 0-0 range matches every decision in the file.
func GoverningRange(reports []Report, file string, start, end int) []Report {
	want := filepath.ToSlash(filepath.Clean(file))
	var out []Report
	for _, rp := range reports {
		if rp.Status == StatusBroken {
			continue
		}
		if filepath.ToSlash(filepath.Clean(rp.File)) != want {
			continue
		}
		if start == 0 || (start <= rp.Range[1] && end >= rp.Range[0]) {
			out = append(out, rp)
		}
	}
	return out
}

// Reports resolves every anchor of a record against the working tree and
// returns one Report per anchor, including the current code when locatable.
func (r *LinkRecord) Reports() []Report {
	var out []Report
	for _, a := range r.Anchors {
		res := Resolve(a)
		rep := Report{
			ID: r.ID, Title: r.Title, Note: r.Note,
			File: res.File, Range: res.Range, Status: res.Status,
			Confidence: res.Confidence, Renamed: res.Renamed, BoundAt: a.Commit,
		}
		if res.Status != StatusBroken {
			if lines, err := FileAtWorktree(res.File); err == nil {
				rep.Code = sliceLines(lines, res.Range[0], res.Range[1])
			}
		}
		out = append(out, rep)
	}
	return out
}

// Fingerprint normalizes a span (trim trailing whitespace per line, join with
// \n) and hashes it. Normalization makes the fingerprint stable against
// whitespace-only churn while still catching real content changes.
func Fingerprint(lines []string) string {
	norm := make([]string, len(lines))
	for i, l := range lines {
		norm[i] = strings.TrimRight(l, " \t\r")
	}
	sum := sha256.Sum256([]byte(strings.Join(norm, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func recordPath(id string) string {
	return filepath.Join(LedgerDir, id+".json")
}

// Save writes a record as pretty JSON under .ledger/.
func (r *LinkRecord) Save() error {
	if err := os.MkdirAll(LedgerDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(recordPath(r.ID), data, 0o644)
}

// AllRecords loads every link record under .ledger/, sorted by id. A missing
// directory is not an error — it just means no decisions yet.
//
// A single unreadable record is reported in skipped rather than failing the
// whole call: one corrupt file must not blind the tool to every other
// decision, which is exactly when you most need it working.
func AllRecords() (recs []*LinkRecord, skipped []error, err error) {
	entries, err := os.ReadDir(LedgerDir)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r, err := Load(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			skipped = append(skipped, err)
			continue
		}
		recs = append(recs, r)
	}
	sort.Slice(recs, func(i, j int) bool { return recs[i].ID < recs[j].ID })
	return recs, skipped, nil
}

// Load reads a record by id from .ledger/.
func Load(id string) (*LinkRecord, error) {
	data, err := os.ReadFile(recordPath(id))
	if err != nil {
		return nil, fmt.Errorf("no such decision %q: %w", id, err)
	}
	var r LinkRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", recordPath(id), err)
	}
	return &r, nil
}

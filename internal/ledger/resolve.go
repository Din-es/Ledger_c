package ledger

import (
	"regexp"
	"strconv"
)

// Status values for a resolved anchor.
const (
	StatusFresh   = "fresh"   // located with full confidence, content identical
	StatusDrifted = "drifted" // relocated by similarity; content changed
	StatusBroken  = "broken"  // the governed code is gone or unrecognizable
)

// driftThreshold is the similarity below which a relocate is called broken
// rather than drifted. Bias toward breaking loudly over relocating silently.
const driftThreshold = 0.5

// searchWindow bounds the fuzzy relocate to a neighborhood around the mapped
// position so cost stays linear on real files.
const searchWindow = 60

type hunk struct {
	oldStart, oldCount int
	newStart, newCount int
}

var hunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func parseHunks(diff string) []hunk {
	var hunks []hunk
	for _, line := range splitLines(diff) {
		m := hunkRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		h := hunk{
			oldStart: atoi(m[1]),
			oldCount: atoiDefault(m[2], 1),
			newStart: atoi(m[3]),
			newCount: atoiDefault(m[4], 1),
		}
		hunks = append(hunks, h)
	}
	return hunks
}

// afterFrom is the first old-line number that sits strictly after a hunk's
// consumed region (an insertion consumes nothing, so it is oldStart+1).
func (h hunk) afterFrom() int {
	if h.oldCount > 0 {
		return h.oldStart + h.oldCount
	}
	return h.oldStart + 1
}

func (h hunk) touches(line int) bool {
	return h.oldCount > 0 && line >= h.oldStart && line <= h.oldStart+h.oldCount-1
}

// mapLine maps an old line number to its new position, reporting whether the
// line fell inside a changed region (touched).
func mapLine(line int, hunks []hunk) (newLine int, touched bool) {
	offset := 0
	for _, h := range hunks {
		if h.touches(line) {
			return line + offset, true
		}
		if line >= h.afterFrom() {
			offset += h.newCount - h.oldCount
		}
	}
	return line + offset, false
}

func rangeTouched(start, end int, hunks []hunk) bool {
	for _, h := range hunks {
		if h.oldCount == 0 {
			continue
		}
		hs, he := h.oldStart, h.oldStart+h.oldCount-1
		if hs <= end && he >= start { // intervals overlap
			return true
		}
	}
	return false
}

// Resolve computes the current location and status of an anchor by replaying
// history from its bound commit to the working tree.
func Resolve(a Anchor) Resolution { return resolveTarget(a, "") }

// ResolveAt computes an anchor's location and status at an arbitrary ref
// (commit/branch) instead of the working tree. Used by the CI gate to find
// where a decision's code sat on the base branch.
func ResolveAt(a Anchor, ref string) Resolution { return resolveTarget(a, ref) }

// resolveTarget locates an anchor at ref ("" means the working tree),
// following the file across renames when it is no longer at its bound path.
func resolveTarget(a Anchor, ref string) Resolution {
	path, renamed := currentPath(a, ref)
	if path == "" {
		return Resolution{File: a.File, Status: StatusBroken}
	}

	cur, err := readFile(path, ref)
	if err != nil {
		return Resolution{File: a.File, Status: StatusBroken}
	}

	var diff string
	if renamed {
		diff, err = DiffPaths(a.Commit, ref, a.File, path)
	} else {
		diff, err = Diff(a.Commit, ref, path)
	}
	if err != nil {
		return Resolution{File: a.File, Status: StatusBroken}
	}

	res := resolveWith(a, cur, diff)
	res.File = path
	res.Renamed = renamed
	return res
}

// currentPath returns where the anchored file lives at ref, following a rename
// if the original path is gone. An empty result means the file is unreachable.
func currentPath(a Anchor, ref string) (path string, renamed bool) {
	if fileExists(a.File, ref) {
		return a.File, false
	}
	target := ref
	if target == "" {
		target = "HEAD"
	}
	if np, ok := FollowRename(a.Commit, target, a.File); ok {
		return np, true
	}
	return "", false
}

func readFile(path, ref string) ([]string, error) {
	if ref == "" {
		return FileAtWorktree(path)
	}
	return FileAtCommit(ref, path)
}

func fileExists(path, ref string) bool {
	if ref == "" {
		return Exists(path)
	}
	_, err := FileAtCommit(ref, path)
	return err == nil
}

// resolveWith is the shared core: given the target file contents and the diff
// from the bound commit to that target, locate the anchor and classify it.
func resolveWith(a Anchor, cur []string, diff string) Resolution {
	hunks := parseHunks(diff)

	start, end := a.Range[0], a.Range[1]

	if !rangeTouched(start, end, hunks) {
		// Clean shift: nothing inside the span changed. Map both ends and
		// confirm the fingerprint still matches at the new location.
		ns, _ := mapLine(start, hunks)
		ne, _ := mapLine(end, hunks)
		body := sliceLines(cur, ns, ne)
		if Fingerprint(body) == a.Fingerprint {
			return Resolution{Range: [2]int{ns, ne}, Confidence: 1.0, Status: StatusFresh}
		}
		// Fingerprint disagrees despite a clean map — fall through to fuzzy.
	}

	// The governed code was touched (or the clean map failed): relocate the
	// original body by similarity within a window around the mapped position.
	approx, _ := mapLine(start, hunks)
	ns, ne, sim := relocate(cur, a.Body, approx)
	switch {
	case sim >= 0.999:
		return Resolution{Range: [2]int{ns, ne}, Confidence: sim, Status: StatusFresh}
	case sim >= driftThreshold:
		return Resolution{Range: [2]int{ns, ne}, Confidence: sim, Status: StatusDrifted}
	default:
		return Resolution{Confidence: sim, Status: StatusBroken}
	}
}

// CodeChanged reports whether an anchor's governed span was modified between
// base and head. It first locates the span on the base ref, then checks
// whether the base..head diff touches that region. brokenAtBase is true when
// the anchor can't even be found on the base ref (its file is absent there).
func CodeChanged(a Anchor, base, head string) (changed, brokenAtBase bool) {
	resBase := ResolveAt(a, base)
	if resBase.Status == StatusBroken {
		return false, true
	}
	// Moving the file counts as changing the governed code — the decision
	// should be revisited either way.
	if _, ok := FollowRename(base, head, resBase.File); ok {
		return true, false
	}
	diff, err := Diff(base, head, resBase.File)
	if err != nil {
		return false, false
	}
	return rangeTouched(resBase.Range[0], resBase.Range[1], parseHunks(diff)), false
}

// relocate slides a window the size of body across cur (bounded to a
// neighborhood of approx) and returns the best-matching span and its
// similarity in [0,1].
func relocate(cur, body []string, approx int) (start, end int, sim float64) {
	n := len(body)
	if n == 0 || len(cur) == 0 {
		return 0, 0, 0
	}
	lo := approx - searchWindow
	if lo < 1 {
		lo = 1
	}
	hi := approx + searchWindow
	if hi > len(cur) {
		hi = len(cur)
	}
	// On equal similarity, prefer the candidate nearest the expected position.
	// Edits are local, so a tie between two windows almost always means the
	// nearer one is the real home of the code; taking the first match instead
	// silently reports a span shifted off the actual lines.
	best, bestStart := -1.0, lo
	for i := lo; i+n-1 <= len(cur) && i <= hi; i++ {
		s := similarity(body, sliceLines(cur, i, i+n-1))
		if s > best || (s == best && abs(i-approx) < abs(bestStart-approx)) {
			best, bestStart = s, i
		}
	}
	if best < 0 {
		return 0, 0, 0
	}
	return bestStart, bestStart + n - 1, best
}

// similarity is an LCS-based ratio over lines: 2*LCS / (len(a)+len(b)).
func similarity(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	l := lcs(a, b)
	return 2 * float64(l) / float64(len(a)+len(b))
}

func lcs(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// slice returns lines [start,end] (1-based inclusive), clamped to bounds.
func sliceLines(lines []string, start, end int) []string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return []string{}
	}
	return lines[start-1 : end]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	return atoi(s)
}

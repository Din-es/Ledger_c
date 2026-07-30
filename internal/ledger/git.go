package ledger

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// git runs a git command in the current working directory and returns stdout.
func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// HeadSHA returns the current HEAD commit.
func HeadSHA() (string, error) {
	out, err := git("rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

// FileAtWorktree reads a file's current working-tree contents split into lines.
// The trailing empty element from a final newline is dropped.
func FileAtWorktree(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return splitLines(string(data)), nil
}

// FileAtCommit reads a file's contents at a specific commit via `git show`.
func FileAtCommit(commit, path string) ([]string, error) {
	out, err := git("show", commit+":"+path)
	if err != nil {
		return nil, err
	}
	return splitLines(out), nil
}

// DiffHunks returns the unified-diff hunk headers between two commits (or a
// commit and the worktree when to == "") for a single file.
func Diff(from, to, path string) (string, error) {
	args := []string{"diff", "--unified=0", "--no-color", from}
	if to != "" {
		args = append(args, to)
	}
	args = append(args, "--", path)
	return git(args...)
}

// renameThreshold is deliberately looser than git's 50% default: a file that
// both moves and gains a lot of code still scores low, and following a wrong
// candidate is harmless — the anchor body won't match there, so it resolves
// broken anyway.
const renameThreshold = "-M25%"

// FollowRename reports a file's new path if it was renamed between commit and
// ref. Git's rename detection is similarity-based, so a file rewritten beyond
// the threshold while moving may still go unreported.
func FollowRename(commit, ref, path string) (string, bool) {
	out, err := git("diff", renameThreshold, "--name-status", commit, ref)
	if err != nil {
		return "", false
	}
	for _, line := range splitLines(out) {
		if !strings.HasPrefix(line, "R") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) == 3 && parts[1] == path {
			return parts[2], true
		}
	}
	return "", false
}

// DiffPaths diffs a rename pair, letting git line up the old and new files so
// hunk line numbers map across the move.
func DiffPaths(from, to, oldPath, newPath string) (string, error) {
	args := []string{"diff", renameThreshold, "--unified=0", "--no-color", from}
	if to != "" {
		args = append(args, to)
	}
	args = append(args, "--", oldPath, newPath)
	return git(args...)
}

// Exists reports whether a path is present in the working tree.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ChangedFiles returns repo-relative paths changed between base and head.
func ChangedFiles(base, head string) (map[string]bool, error) {
	out, err := git("diff", "--name-only", base, head)
	if err != nil {
		return nil, err
	}
	changed := map[string]bool{}
	for _, p := range splitLines(out) {
		if p != "" {
			changed[p] = true
		}
	}
	return changed, nil
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

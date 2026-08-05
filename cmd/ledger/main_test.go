package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Din-es/Ledger_c/internal/ledger"
)

func TestParseWhyLocation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		location, file string
		start, end     int
		wantErr        bool
	}{
		{name: "whole file", location: "file.go", file: "file.go"},
		{name: "single line", location: "file.go:12", file: "file.go", start: 12, end: 12},
		{name: "line range", location: "file.go:10-40", file: "file.go", start: 10, end: 40},
		{name: "descending range", location: "file.go:40-10", wantErr: true},
		{name: "zero line", location: "file.go:0", wantErr: true},
		{name: "missing range end", location: "file.go:10-", wantErr: true},
		{name: "empty location", location: "", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, start, end, err := parseWhyLocation(tc.location)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseWhyLocation(%q) unexpectedly succeeded", tc.location)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWhyLocation(%q): %v", tc.location, err)
			}
			if file != tc.file || start != tc.start || end != tc.end {
				t.Fatalf("parseWhyLocation(%q) = %q, %d, %d; want %q, %d, %d", tc.location, file, start, end, tc.file, tc.start, tc.end)
			}
		})
	}
}

func TestCmdUnbindRemovesOnlyRecord(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	rec := &ledger.LinkRecord{ID: "retired", Note: "docs/decisions/retired.md"}
	if err := rec.Save(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(rec.Note), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rec.Note, []byte("keep this history\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := cmdUnbind([]string{"retired"}); err != nil {
		t.Fatalf("cmdUnbind: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ledger.LedgerDir, "retired.json")); !os.IsNotExist(err) {
		t.Fatalf("record remains after unbind: %v", err)
	}
	if _, err := os.Stat(rec.Note); err != nil {
		t.Fatalf("unbind removed rationale: %v", err)
	}
}

func TestCmdUnbindUnknownDecision(t *testing.T) {
	if err := cmdUnbind([]string{"missing"}); err == nil {
		t.Fatal("cmdUnbind succeeded for a missing decision")
	}
}

func TestCmdUnbindRejectsPathLikeID(t *testing.T) {
	for _, id := range []string{"", ".", "..", "../outside", `..\\outside`} {
		if err := cmdUnbind([]string{id}); err == nil {
			t.Fatalf("cmdUnbind(%q) accepted a path-like id", id)
		}
	}
}

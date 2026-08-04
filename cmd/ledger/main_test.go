package main

import "testing"

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

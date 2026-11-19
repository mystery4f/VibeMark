package main

import "testing"

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
		wantDir  string
		wantErr  bool
	}{
		{name: "no arguments", args: nil},
		{name: "one file", args: []string{"note.md"}, wantPath: "note.md"},
		{name: "dir flag", args: []string{"--dir", "."}, wantDir: "."},
		{name: "dir flag before file", args: []string{"-dir=docs"}, wantDir: "docs"},
		{name: "file and dir", args: []string{"note.md", "--dir", "."}, wantErr: true},
		{name: "unknown flag", args: []string{"--nope"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path, dir, err := parseArgs(test.args)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseArgs(%q) = %q, %q, nil; want an error", test.args, path, dir)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseArgs(%q) returned %v", test.args, err)
			}
			if path != test.wantPath || dir != test.wantDir {
				t.Fatalf("parseArgs(%q) = %q, %q; want %q, %q", test.args, path, dir, test.wantPath, test.wantDir)
			}
		})
	}
}

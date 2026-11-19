package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanReturnsMarkdownTree(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "# a")
	writeFile(t, filepath.Join(dir, "sub", "b.md"), "# b")
	writeFile(t, filepath.Join(dir, "sub", "skip.txt"), "no")
	writeFile(t, filepath.Join(dir, ".hidden", "c.md"), "# c")
	writeFile(t, filepath.Join(dir, "node_modules", "d.md"), "# d")

	roots, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 {
		t.Fatalf("roots = %d, want a file and one directory: %+v", len(roots), roots)
	}
	if roots[0].Label != "sub" || !roots[0].Dir {
		t.Fatalf("first root is not the directory: %+v", roots[0])
	}
	if len(roots[0].Children) != 1 || roots[0].Children[0].Label != "b.md" {
		t.Fatalf("nested file missing: %+v", roots[0].Children)
	}
	if roots[1].Label != "a.md" || roots[1].Dir {
		t.Fatalf("top level file missing: %+v", roots[1])
	}
}

func TestScanRejectsAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.md")
	writeFile(t, path, "x")
	if _, err := Scan(path); err == nil {
		t.Fatal("Scan of a file must fail")
	}
}

func TestWriteAtomicKeepsBytesAndLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.md")
	content := "行一\r\n行二\n无结尾换行"
	if err := WriteAtomic(path, content); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != content {
		t.Fatalf("content changed: %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestEnsureCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.md")
	if err := Ensure(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("new file is not empty: %d bytes", info.Size())
	}
}

func TestEnsureRejectsMissingParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "new.md")
	if err := Ensure(path); err == nil {
		t.Fatal("missing parent directory must fail")
	}
}

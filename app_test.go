package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/dyike/keel/ui/el"
	"github.com/dyike/keel/ui/window"
	"github.com/dyike/keel/ui/window/windowtest"
)

// writeNote puts one markdown file in a fresh directory and returns its path.
func writeNote(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	path = filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("# 标题\n\n正文"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, path
}

// The single-file window shows the file it opened and switches to the preview.
func TestSingleFileWindowShowsPathAndTogglesPreview(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Want("note.md")
	app.Want("已保存")

	app.Press("ctrl+p")
	app.Want("预览")

	app.Press("ctrl+p")
	app.WantNot("预览")
}

// The working-directory window lists the markdown files it found.
func TestWorkspaceWindowListsFiles(t *testing.T) {
	dir, _ := writeNote(t)
	a := newApp("", dir)
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Want("note.md")
	app.Want("已保存")
}

// A missing file is created empty, which is what git hands an editor.
func TestMissingFileIsCreated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")
	a := newApp(path, "")
	windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 0 {
		t.Fatalf("created file is not empty: %d bytes", info.Size())
	}
}

// An empty working directory says so instead of showing a blank editor.
func TestEmptyWorkspaceShowsHint(t *testing.T) {
	a := newApp("", t.TempDir())
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Want(emptyHint)
}

// A filesystem failure reaches the status bar as a sentence, not a raw error.
func TestUserErrorIsReadable(t *testing.T) {
	if got, want := userError("打开", "note.md", fs.ErrNotExist), "打开 note.md 失败：文件或目录不存在"; got != want {
		t.Fatalf("userError = %q, want %q", got, want)
	}
	if got, want := userError("保存", "note.md", fs.ErrPermission), "保存 note.md 失败：没有访问权限"; got != want {
		t.Fatalf("userError = %q, want %q", got, want)
	}
}

// The editor names the open file, so an assistive reader hears the file name.
func TestEditorIsNamedAfterTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(path, []byte("# 标题"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Want("note.md")
}

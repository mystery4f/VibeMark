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

// A rendered block swaps to its own source when it is pressed.
func TestPressingABlockShowsItsSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(path, []byte("# 标题\n\n- 项目\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Want("标题")
	app.WantNot("# 标题")

	app.Click("项目")
	app.Want("- 项目")
}

// Leaving a block writes its source back into the document, so the whole file
// keeps the edit.
func TestCommittingABlockWritesBack(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.md")
	if err := os.WriteFile(path, []byte("# 标题\n\n正文\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(path, "")
	a.once()

	a.activate(1)
	if got := a.editor.Value(); got != "正文" {
		t.Fatalf("active block source = %q, want 正文", got)
	}
	a.editor.SetValue("改过的正文")
	a.commit()

	if want := "# 标题\n\n改过的正文\n"; a.doc != want {
		t.Fatalf("document = %q, want %q", a.doc, want)
	}
	a.save()
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "# 标题\n\n改过的正文\n"; string(onDisk) != want {
		t.Fatalf("file = %q, want %q", onDisk, want)
	}
}

// An empty working directory is not a dead end: Ctrl+N creates a file there and
// opens it, and a second press does not overwrite the first one.
func TestNewFileFillsAnEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	a := newApp("", dir)
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})
	app.Want("这个目录里还没有 Markdown 文件")

	app.Press("ctrl+n")
	app.Want("untitled.md")

	app.Press("ctrl+n")
	app.Want("untitled-2.md")

	for _, name := range []string{"untitled.md", "untitled-2.md"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

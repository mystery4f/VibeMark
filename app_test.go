package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	// Block 0 is open for typing on a single file, so only the other blocks
	// still render as markdown.
	app.Want("项目")
	app.WantNot("- 项目")

	app.Click("项目")
	app.Want("- 项目")
	if got := a.editor.Value(); !strings.Contains(got, "- 项目") {
		t.Fatalf("editor = %q, want the pressed block source", got)
	}
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

// git opens the editor and expects the caret to be usable without a click: the
// first keystroke has to land somewhere.
func TestSingleFileAcceptsTypingWithoutAClick(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Type("追加一句")
	if got := a.text(); !strings.Contains(got, "追加一句") {
		t.Fatalf("typed text did not land; document = %q", got)
	}
}

// A commit message file can be empty, and an empty file must still accept text.
func TestEmptyFileAcceptsTyping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Type("fix: something")
	if got := a.text(); !strings.Contains(got, "fix: something") {
		t.Fatalf("typed text did not land; document = %q", got)
	}
}

// Ctrl+S after typing must write what was typed, without pressing Esc first.
func TestSaveKeepsTextTypedInTheActiveBlock(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Type("尾巴")
	app.Press("ctrl+s")
	if got := mustRead(t, path); !strings.Contains(got, "尾巴") {
		t.Fatalf("file = %q, want it to hold the typed text", got)
	}
	if a.dirty {
		t.Fatal("save left the document dirty")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Moving to another block keeps what was typed in the previous one.
func TestSwitchingBlocksKeepsTypedText(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	a.editor.SetValue("# 改过的标题")
	a.activate(1)
	if got := a.text(); !strings.Contains(got, "# 改过的标题") {
		t.Fatalf("document = %q, want the edited heading", got)
	}
}

// Enter inside a block starts a new block, and both keep rendering afterwards.
func TestNewParagraphBecomesItsOwnBlock(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	a.editor.SetValue("# 标题\n\n新的一段")
	a.commit()
	// Two paragraphs typed into block 0 plus the untouched one further down.
	if blocks := len(a.blocks); blocks != 3 {
		t.Fatalf("blocks = %d, want 3 for %q", blocks, a.doc)
	}
}

// Preview shows the active block's text too, not the stale document.
func TestPreviewSeesTheActiveBlock(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	a.editor.SetValue("# 标题\n\n预览里要看见这句")
	app.Press("ctrl+p")
	app.Want("预览里要看见这句")
}

// Ctrl+B wraps the selection of the block under the caret.
func TestBoldWrapsTheSelection(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	a.editor.SetValue("加粗这句")
	a.editor.SetCursor(0, 0)
	a.wrap("**", "**")
	if got := a.editor.Value(); got != "****加粗这句" {
		t.Fatalf("value = %q, want the markers around the caret", got)
	}
}

// The working directory saves by itself once typing pauses.
func TestWorkspaceAutosaves(t *testing.T) {
	dir, path := writeNote(t)
	a := newApp("", dir)
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	a.activate(1)
	a.editor.SetValue("自动保存的正文")
	a.changed("自动保存的正文")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(mustRead(t, path), "自动保存的正文") {
			return
		}
		// The save is queued with core.Update, which runs in the frame loop.
		app.Render()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("file = %q, want the automatic save", mustRead(t, path))
}

// Enter finishes the line above: it becomes its own rendered block while the
// caret continues on the line below.
func TestEnterCommitsTheLineAbove(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	a.editor.SetValue("第一行\n第二行")
	a.changed(a.editor.Value())
	a.editor.SetCursor(0, 3)
	app.Press("enter")

	if got := a.text(); !strings.Contains(got, "第一行\n\n第二行") {
		t.Fatalf("document = %q, want the two lines split into blocks", got)
	}
	if a.active < 0 {
		t.Fatal("the caret left the editor; the second line must stay editable")
	}
	if got := a.editor.Value(); got != "第二行" {
		t.Fatalf("editor = %q, want only the line below the caret", got)
	}
}

// Enter on the last line keeps an empty block open for the next paragraph.
func TestEnterAtTheEndStartsANewBlock(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	a.editor.SetValue("只有一行")
	a.changed(a.editor.Value())
	a.editor.SetCursor(0, 4)
	before := len(a.blocks)
	app.Press("enter")

	if a.active < 0 {
		t.Fatal("no block is open after Enter")
	}
	if got := a.editor.Value(); got != "" {
		t.Fatalf("editor = %q, want an empty block", got)
	}
	if len(a.blocks) != before+1 {
		t.Fatalf("blocks = %d, want %d", len(a.blocks), before+1)
	}
}

// Heading and paragraph shortcuts follow Typora: Ctrl+3 styles the line, Ctrl+0
// puts the plain paragraph back.
func TestHeadingAndParagraphShortcuts(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Press("ctrl+3")
	if got := a.editor.Value(); got != "### 标题" {
		t.Fatalf("editor = %q, want a third-level heading", got)
	}

	app.Press("ctrl+0")
	if got := a.editor.Value(); got != "标题" {
		t.Fatalf("editor = %q, want the plain paragraph back", got)
	}
}

// Ctrl+Shift+K wraps the block in a code fence and takes it off again; the
// inline chords of Typora keep working the same way.
func TestFenceAndInlineShortcuts(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	app.Press("ctrl+shift+k")
	if got := a.editor.Value(); got != "```\n# 标题\n```" {
		t.Fatalf("editor = %q, want a fenced block", got)
	}
	app.Press("ctrl+shift+k")
	if got := a.editor.Value(); got != "# 标题" {
		t.Fatalf("editor = %q, want the fences gone", got)
	}

	a.editor.SetCursor(0, 5)
	a.changed(a.editor.Value())
	app.Press("alt+shift+5")
	if got := a.editor.Value(); !strings.Contains(got, "~~~~") {
		t.Fatalf("editor = %q, want an empty strike pair", got)
	}
}

// Ctrl+/ shows the whole document as one source block, the way Typora's source
// code mode does, and Ctrl+/ again gives the blocks back.
func TestSourceModeShortcut(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	blocks := len(a.blocks)
	app.Press("ctrl+/")
	if !a.sourceMode || len(a.blocks) != 1 {
		t.Fatalf("sourceMode = %v, blocks = %d, want one source block", a.sourceMode, len(a.blocks))
	}
	if got := a.editor.Value(); !strings.Contains(got, "# 标题\n\n正文") {
		t.Fatalf("editor = %q, want the whole document", got)
	}

	app.Press("ctrl+/")
	if a.sourceMode || len(a.blocks) != blocks {
		t.Fatalf("sourceMode = %v, blocks = %d, want %d blocks back", a.sourceMode, len(a.blocks), blocks)
	}
}

// Enter inside a list item carries the marker to the next line and keeps the
// list in one block, the way Typora does.
func TestEnterContinuesTheList(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	a.editor.SetValue("- 项目")
	a.changed("- 项目")
	a.editor.SetCursor(0, 4)
	blocks := len(a.blocks)

	app.Press("enter")
	if got, want := a.editor.Value(), "- 项目\n- "; got != want {
		t.Fatalf("editor = %q, want %q", got, want)
	}
	if len(a.blocks) != blocks {
		t.Fatalf("blocks = %d, want the list to stay in %d", len(a.blocks), blocks)
	}
}

// Enter on an item with no text leaves the list instead of nesting another one.
func TestEnterOnAnEmptyItemLeavesTheList(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	a.editor.SetValue("- ")
	a.changed("- ")
	a.editor.SetCursor(0, 2)

	app.Press("enter")
	if got := a.editor.Value(); got != "" {
		t.Fatalf("editor = %q, want the marker gone", got)
	}
}

// Shift+Enter breaks the line without ending the block.
func TestShiftEnterStaysInTheBlock(t *testing.T) {
	_, path := writeNote(t)
	a := newApp(path, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	blocks := len(a.blocks)
	a.editor.SetCursor(0, 4)

	app.Press("shift+enter")
	got := a.editor.Value()
	if !strings.HasPrefix(got, "# 标题\n") {
		t.Fatalf("editor = %q, want a break after the heading", got)
	}
	if len(a.blocks) != blocks {
		t.Fatalf("blocks = %d, want %d", len(a.blocks), blocks)
	}
}

// Enter inside a fence is code, not a block boundary.
func TestEnterInsideAFenceKeepsTheBlock(t *testing.T) {
	dir, _ := writeNote(t)
	fence := filepath.Join(dir, "fence.md")
	if err := os.WriteFile(fence, []byte("```\ncode\n```"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newApp(fence, "")
	app := windowtest.New(t, window.Options{Content: el.Root(a), KeepState: &a.state, Shortcuts: a.shortcuts()})

	blocks := len(a.blocks)
	a.editor.SetCursor(1, 4)

	app.Press("enter")
	if len(a.blocks) != blocks {
		t.Fatalf("blocks = %d, want %d", len(a.blocks), blocks)
	}
	if got := a.editor.Value(); !strings.HasPrefix(got, "```\ncode\n") {
		t.Fatalf("editor = %q, want the fence kept", got)
	}
}

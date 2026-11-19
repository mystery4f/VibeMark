package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	nativeclip "github.com/dyike/keel/native/clipboard"
	"github.com/dyike/keel/ui/core"
	"github.com/dyike/keel/ui/el"
	"github.com/dyike/keel/ui/kit"
	"github.com/dyike/keel/ui/markdown"
	"github.com/dyike/keel/ui/theme"
	"github.com/mystery4f/vibemark/internal/mddoc"
	"github.com/mystery4f/vibemark/internal/media"
	"github.com/mystery4f/vibemark/internal/workspace"
)

type appMode int

const (
	modeWorkspace appMode = iota // a directory tree plus the editor
	modeSingle                   // one file, no tree
)

const (
	// emptyHint is the zero-data text a workspace shows before a file is open.
	emptyHint = "这个目录里还没有 Markdown 文件"
	// lineHeight is one editor row in dp: kit.CodeEditor draws TextMd (13sp)
	// at 1.5 line height.
	lineHeight = 20
	// maxEditorHeight bounds the editor: past a few hundred lines a frame costs
	// tens of milliseconds, so a long block scrolls inside the box instead.
	maxEditorHeight = 600
	// blockIndent is the width kit.CodeEditor reserves for its gutter, so the
	// rendered blocks start where the editor's text starts and switching blocks
	// does not shift the text sideways.
	blockIndent = 56
	// blockGuard is a zero-width character on both ends of the active block's
	// buffer. keel's editor swallows Backspace and Delete on the buffer's first
	// and last cell, so a guard going away is the only signal that the key
	// arrived at a block edge, which is what joins two blocks.
	blockGuard = "​"
	// saveDelay is how long typing rests before a working directory writes.
	saveDelay = 300 * time.Millisecond
)

// appState is what keel persists between runs through KeepState.
type appState struct {
	Dir  string
	Path string
}

// app is the whole window: a column of blocks, one of which is the editor, an
// optional tree, and a status bar.
type app struct {
	state appState

	mode     appMode
	dir      string
	path     string
	mediaDir string // where a pasted picture is written, never beside the note

	editor       *kit.CodeEditorView
	decor        *kit.CodeDecorationCollection
	tree         *kit.TreeView
	bar          *kit.StatusBarView
	preview      *markdown.Doc
	previewStale bool // the preview has to parse the document again

	hasFiles bool // the working directory holds at least one markdown file

	// The document is shown as top-level blocks: a block renders as markdown
	// until the caret enters it, and then as its own source.
	doc    string
	blocks []mddoc.Block
	docs   []*markdown.Doc

	// lastValue is what the editor held after the previous change, so a bare
	// newline can be told apart from pasted text or a programmatic SetValue.
	lastValue string
	splitting bool

	active        int    // index of the block being edited, -1 when none
	before, after string // the text before and after the active block

	saved       string
	dirty       bool
	previewing  bool
	pending     bool        // decorations are stale and must be rebuilt
	saveArmed   atomic.Bool // a debounce timer is running
	saveDue     atomic.Bool // the debounce elapsed; the write runs in the next frame
	status      string
	statusColor *color.NRGBA
	treeHidden  bool // the sidebar is folded away
	sourceMode  bool // the whole document is shown as one source block
	allSelected bool // Ctrl+A picked the whole document, not just one block

	ready bool
}

// newApp builds the app for one file (path) or one directory (dir). Exactly one
// of them is non-empty when the command line named it.
func newApp(path, dir string) *app {
	a := &app{path: path, dir: dir, bar: kit.StatusBar(), preview: markdown.New(""), active: -1, mediaDir: media.Root()}
	if path != "" {
		a.mode = modeSingle
	} else {
		a.mode = modeWorkspace
	}
	return a
}

// once loads the document and builds the widgets on the first frame, after keel
// has restored the persisted state.
func (a *app) once() {
	if a.ready {
		return
	}
	a.ready = true
	a.preview = a.newDoc("")

	a.editor = kit.CodeEditor("").
		Language("markdown").
		SoftWrap(true).
		AutoClose(true).
		OnChange(a.changed)
	a.decor = a.editor.Decorations()

	if a.mode == modeWorkspace && a.dir == "" {
		a.dir = a.state.Dir
	}
	if a.mode == modeSingle && a.path == "" {
		a.path = a.state.Path
	}

	switch a.mode {
	case modeSingle:
		a.openFile()
	case modeWorkspace:
		if a.dir != "" {
			a.loadDir()
		}
	}
}

// openFile creates the file when it does not exist yet and loads it. A failure
// leaves the window empty and reports through the status bar.
func (a *app) openFile() {
	name := filepath.Base(a.path)
	if err := workspace.Ensure(a.path); err != nil {
		a.fail(userError("打开", name, err))
		return
	}
	content, err := workspace.Read(a.path)
	if err != nil {
		a.fail(userError("读取", name, err))
		return
	}
	a.load(content)
	a.state.Path = a.path
	a.status = ""
}

// load makes content the saved text of the window.
func (a *app) load(content string) {
	a.saved = content
	a.dirty = false
	a.editor.Name(filepath.Base(a.path))
	a.editor.SetValue("")
	a.split(content)
	// A file opens ready to edit: git hands the editor a file and expects the
	// next keystroke to land in it, and the workspace needs no click before a
	// caret exists. An empty file gets one empty block to write into.
	if len(a.blocks) == 0 {
		a.appendBlock()
		return
	}
	a.activate(0)
}

// split refills the block list and drops the rendered markdown of the old one.
func (a *app) split(content string) {
	a.previewStale = true
	a.active = -1
	a.before, a.after = "", ""
	a.doc = content
	a.blocks = mddoc.Blocks(content)
	a.docs = make([]*markdown.Doc, len(a.blocks))
}

// loadDir scans the directory into the tree and opens its first file.
func (a *app) loadDir() {
	roots, err := workspace.Scan(a.dir)
	if err != nil {
		a.fail(userError("扫描目录", a.dir, err))
		return
	}
	a.tree = kit.Tree(treeNodes(roots)...).OnChange(a.open)
	a.hasFiles = len(roots) > 0
	a.state.Dir = a.dir
	if first := firstFile(roots); first != "" {
		a.open(first)
	}
}

// open switches the window to the file the tree selected, saving pending edits
// of the current one first.
func (a *app) open(path string) {
	a.commit()
	content, err := workspace.Read(path)
	if err != nil {
		a.fail(userError("打开", filepath.Base(path), err))
		return
	}
	a.path = path
	a.load(content)
}

// text is the whole document as it stands, including the block under the caret.
func (a *app) text() string {
	if a.active < 0 {
		return a.doc
	}
	return a.before + a.buffer() + a.after
}

// buffer is the block text without the guards that mark its edges.
func (a *app) buffer() string {
	return strings.TrimSuffix(strings.TrimPrefix(a.editor.Value(), blockGuard), blockGuard)
}

// caret returns the caret in the coordinates mddoc works in: keel counts
// columns in runes, and the guard takes the first one of a block's first line.
func (a *app) caret() mddoc.Pos {
	line, col := a.editor.Cursor()
	if line == 0 && col > 0 {
		col--
	}
	return mddoc.Pos{Line: line, Col: col}
}

// setBuffer puts text in the editor with the guards around it and the caret at
// caret, which counts runes of text rather than of the buffer.
func (a *app) setBuffer(text string, caret mddoc.Pos) {
	col := caret.Col
	if caret.Line == 0 {
		col++
	}
	a.editor.SetValue(blockGuard + text + blockGuard)
	a.editor.SetCursor(caret.Line, col)
	a.lastValue = a.editor.Value()
}

// lineAt returns one line of text, without its newline.
func lineAt(text string, line int) string {
	start := 0
	for i := 0; i < line; i++ {
		n := strings.IndexByte(text[start:], '\n')
		if n < 0 {
			return ""
		}
		start += n + 1
	}
	rest := text[start:]
	if n := strings.IndexByte(rest, '\n'); n >= 0 {
		rest = rest[:n]
	}
	return rest
}

// activate makes block i the one under the caret, writing the open block back
// first.
func (a *app) activate(i int) {
	a.commit()
	if a.allSelected {
		a.allSelected = false
		a.status = ""
		a.statusColor = nil
	}
	if i < 0 || i >= len(a.blocks) {
		return
	}
	a.active = i
	a.before, a.after = a.doc[:a.blocks[i].Start], a.doc[a.blocks[i].End:]
	a.setBuffer(a.blocks[i].Text(a.doc), mddoc.Pos{})
	a.editor.Focus()
}

// joinUp merges the block being edited with the one above it, the way deleting
// the break between two paragraphs does in Typora.
func (a *app) joinUp() {
	if a.active <= 0 {
		a.setBuffer(a.buffer(), mddoc.Pos{})
		return
	}
	prev, block := a.blocks[a.active-1], a.blocks[a.active]
	before, above := a.doc[:prev.Start], prev.Text(a.doc)
	doc := before + above + a.buffer() + a.doc[block.End:]
	a.place(doc, len(before)+len(above))
}

// joinDown merges the block below into the one being edited, the way Delete at
// the end of a paragraph does.
func (a *app) joinDown() {
	if a.active < 0 || a.active+1 >= len(a.blocks) {
		a.setBuffer(a.buffer(), mddoc.Pos{})
		return
	}
	block, next := a.blocks[a.active], a.blocks[a.active+1]
	text := a.buffer()
	doc := a.doc[:block.Start] + text + next.Text(a.doc) + a.doc[next.End:]
	a.place(doc, block.Start+len(text))
}

// place re-splits the document and puts the caret at offset, inside the block
// that holds it.
func (a *app) place(doc string, offset int) {
	a.doc = doc
	a.split(doc)
	index := mddoc.BlockAt(a.blocks, offset)
	if index < 0 {
		index = len(a.blocks) - 1
	}
	if index < 0 {
		return
	}
	at := mddoc.PosOf(doc, offset)
	a.activate(index)
	if a.active != index {
		return
	}
	start := mddoc.PosOf(doc, a.blocks[index].Start)
	a.setBuffer(a.blocks[index].Text(a.doc), mddoc.Pos{Line: at.Line - start.Line, Col: at.Col})
	a.editor.Focus()
	a.touch(a.buffer())
}

// commit writes the edited block back into the document and re-splits it.
func (a *app) commit() {
	if a.active < 0 {
		return
	}
	a.doc = a.before + a.buffer() + a.after
	a.active = -1
	a.allSelected = false
	a.before, a.after = "", ""
	a.split(a.doc)
}

// appendBlock adds an empty paragraph at the end of the document and starts
// editing it, so a reader can keep writing without hunting for a spot.
func (a *app) appendBlock() {
	a.commit()
	if a.doc != "" && !strings.HasSuffix(a.doc, "\n\n") {
		if strings.HasSuffix(a.doc, "\n") {
			a.doc += "\n"
		} else {
			a.doc += "\n\n"
		}
	}
	a.split(a.doc)
	a.blocks = append(a.blocks, mddoc.Block{Start: len(a.doc), End: len(a.doc)})
	a.docs = append(a.docs, nil)
	a.active = len(a.blocks) - 1
	a.before, a.after = a.doc, ""
	a.setBuffer("", mddoc.Pos{})
	a.editor.Focus()
	a.changed(a.lastValue)
}

// changed runs on every editor change. Decorations wait for the next frame so a
// burst of keystrokes costs one parse.
func (a *app) changed(raw string) {
	if a.splitting {
		a.lastValue = raw
		return
	}
	// The whole document is picked, so this edit replaces all of it.
	if a.allSelected {
		a.allSelected = false
		a.status = ""
		a.statusColor = nil
		a.replaceDocument(a.buffer())
		return
	}
	// An empty block is nothing but its two guards, so a Backspace leaves one
	// behind and keeps both of the edge checks above quiet: it means the same
	// thing as deleting the leading guard.
	if raw == blockGuard {
		a.lastValue = raw
		a.joinUp()
		return
	}
	// A click on a rendered block leaves the caret at column zero, in front of
	// the leading guard, so typing there would push the guard into the text and
	// eventually into the file. Put it back and keep the caret after the typing.
	if inner := strings.TrimSuffix(strings.TrimPrefix(raw, blockGuard), blockGuard); strings.Contains(inner, blockGuard) {
		off := strings.Index(raw, blockGuard)
		clean := strings.ReplaceAll(raw, blockGuard, "")
		// Every guard before the cut disappears from that offset's left side.
		off -= strings.Count(raw[:off], blockGuard) * len(blockGuard)
		if off > len(clean) {
			off = len(clean)
		}
		a.setBuffer(clean, mddoc.PosOf(clean, off))
		a.touch(clean)
		return
	}
	// keel swallows Backspace on the buffer's first cell, so the guard going
	// away is the trace of that key: it joins the block above instead, the way
	// deleting a paragraph break does in Typora.
	if strings.HasSuffix(a.lastValue, blockGuard) && !strings.HasSuffix(raw, blockGuard) {
		a.lastValue = raw
		a.joinDown()
		return
	}
	if strings.HasPrefix(a.lastValue, blockGuard) && !strings.HasPrefix(raw, blockGuard) {
		a.lastValue = raw
		a.joinUp()
		return
	}
	text := a.buffer()
	// Source mode edits the document as one block, so Enter stays a plain
	// newline there instead of starting a new block.
	if a.sourceMode {
		a.lastValue = raw
		a.dirty = text != a.saved
		a.pending = true
		if a.mode == modeWorkspace {
			a.scheduleSave()
		}
		return
	}
	if off, ok := newlineInsertion(a.lastValue, raw); ok {
		a.lastValue = raw
		a.splitAt(off - len(blockGuard))
		return
	}
	a.lastValue = raw
	full := a.text()
	a.dirty = full != a.saved
	a.pending = true
	if a.mode == modeWorkspace {
		a.scheduleSave()
	}
}

// newlineInsertion reports whether next is prev with exactly one newline added,
// and where the caret sits after it. Typing Enter must finish the line above and
// start a new block instead of keeping the whole paragraph as source.
func newlineInsertion(prev, next string) (int, bool) {
	if len(next) != len(prev)+1 {
		return 0, false
	}
	at := 0
	for at < len(prev) && prev[at] == next[at] {
		at++
	}
	if next[at] != '\n' || next[at+1:] != prev[at:] {
		return 0, false
	}
	return at + 1, true
}

// splitAtCaret turns the text above off as its own block and keeps editing the
// text below it, so everything already written renders again.
func (a *app) splitAt(off int) {
	if a.active < 0 {
		return
	}
	value := a.buffer()
	if off <= 0 || off > len(value) {
		return
	}
	head := strings.TrimRight(value[:off], "\n")
	tail := strings.TrimLeft(value[off:], "\n")
	line := head
	if i := strings.LastIndexByte(head, '\n'); i >= 0 {
		line = head[i+1:]
	}
	// A newline inside a fence is content, so the block keeps it as it is.
	if mddoc.InsideFence(value, strings.Count(head, "\n")) {
		a.touch(value)
		return
	}
	// Enter in a list item continues the list the way Typora does.
	if next, ok := mddoc.ListNext(line); ok {
		a.continueList(value, off, line, next)
		return
	}
	if tail == "" {
		a.appendBlock()
		return
	}
	// Enter at the start of a block adds an empty line above the text, the way
	// it does in Typora; an empty block cannot exist, so the break stays here.
	if head == "" {
		a.touch(value)
		return
	}
	gap := "\n\n"
	doc := a.before + head + gap + tail + a.after
	start := len(a.before) + len(head) + len(gap)
	a.splitting = true
	a.doc = doc
	a.split(doc)
	a.splitting = false
	index := mddoc.BlockAt(a.blocks, start)
	if index < 0 {
		index = len(a.blocks) - 1
	}
	a.activate(index)
	a.dirty = a.text() != a.saved
	a.pending = true
	if a.mode == modeWorkspace {
		a.scheduleSave()
	}
}

// continueList keeps Enter inside a list: the new line carries the same marker
// and the list stays one block. Enter on an item with no text leaves the list.
func (a *app) continueList(value string, at int, line, next string) {
	if next == "" {
		// An empty item leaves the list: the marker and its space go too.
		start := at - 1 - len(line)
		if start < 0 {
			start = 0
		}
		a.setValue(value[:start], mddoc.Pos{Line: strings.Count(value[:start], "\n")})
		return
	}
	out := value[:at] + next + value[at:]
	a.setValue(out, mddoc.Pos{Line: strings.Count(value[:at], "\n"), Col: utf8.RuneCountInString(next)})
}

// newLine starts the next paragraph, or continues the list the caret is in,
// the way Enter does in Typora. The window registers it for Enter so the
// buffer guards stay the only trace of the key inside the editor.
func (a *app) newLine() {
	if a.active < 0 || a.previewing {
		return
	}
	value := a.buffer()
	at := a.caret()
	line := lineAt(value, at.Line)
	out, caret := mddoc.Insert(value, at, "\n")
	inserted := mddoc.Offset(out, caret)
	a.setValue(out, caret)
	if a.sourceMode || mddoc.InsideFence(value, at.Line) {
		return
	}
	// An empty line only gains another one: there is no text to split off,
	// and Enter has to move the caret down whether the line holds text or not.
	if strings.TrimSpace(line) == "" {
		return
	}
	if next, ok := mddoc.ListNext(line); ok {
		a.continueList(out, inserted, line, next)
		return
	}
	a.splitAt(inserted)
}

// softBreak adds a line break inside the block instead of starting a new one,
// the way Typora's Shift+Enter does.
func (a *app) softBreak() {
	if a.active < 0 || a.previewing || a.sourceMode {
		return
	}
	out, caret := mddoc.Insert(a.buffer(), a.caret(), "\n")
	a.setValue(out, caret)
}

// setValue writes the editor for a change that must not look like a fresh Enter
// to changed, so a continued list or a soft break stays in the same block.
func (a *app) setValue(src string, caret mddoc.Pos) {
	a.splitting = true
	a.setBuffer(src, caret)
	a.splitting = false
	a.touch(src)
}

// touch records an edit that keeps the block boundaries as they are.
func (a *app) touch(value string) {
	a.previewStale = true
	a.lastValue = blockGuard + value + blockGuard
	a.dirty = a.text() != a.saved
	a.pending = true
	if a.mode == modeWorkspace {
		a.scheduleSave()
	}
}

// scheduleSave writes 300ms after the last keystroke. Working-directory mode has
// no unsaved marker, so the write must follow the typing closely.
func (a *app) scheduleSave() {
	if a.mode != modeWorkspace || !a.saveArmed.CompareAndSwap(false, true) {
		return
	}
	time.AfterFunc(saveDelay, func() {
		// The write itself runs in a frame, where the editor buffer belongs to
		// this goroutine; asking for one only needs an invalidation.
		a.saveArmed.Store(false)
		a.saveDue.Store(true)
		core.Update(func() {})
	})
}

// save writes the document through a temporary file. A failure keeps the dirty
// flag so the next attempt still has data to write.
func (a *app) save() {
	if a.path == "" {
		return
	}
	full := a.text()
	if err := workspace.WriteAtomic(a.path, full); err != nil {
		a.fail(userError("保存", filepath.Base(a.path), err))
		return
	}
	a.saved = full
	a.dirty = false
	a.status = ""
	a.statusColor = nil
}

// userError turns a filesystem error into text the reader can act on.
func userError(op, name string, err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("%s %s 失败：文件或目录不存在", op, name)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Sprintf("%s %s 失败：没有访问权限", op, name)
	}
	return fmt.Sprintf("%s %s 失败：%v", op, name, err)
}

// fail records an error message for the status bar.
func (a *app) fail(msg string) {
	pal := theme.Current()
	c := pal.Danger
	a.status = msg
	a.statusColor = &c
}

// applyDecorations rebuilds the decoration set from the block being edited.
func (a *app) applyDecorations(src string) {
	if a.decor == nil {
		return
	}
	a.decor.Clear()
	if a.active < 0 {
		return
	}
	a.decor.Append(mddoc.Decorations(src)...)
}

// Render draws the window: tree, the column of blocks, then the status bar.
func (a *app) Render(cx *el.Context) el.Element {
	a.once()
	// A save that came due while the window was idle runs here, in a frame.
	if a.saveDue.Swap(false) && a.dirty {
		a.save()
	}
	if a.pending {
		a.pending = false
		a.applyDecorations(a.buffer())
	}

	pal := theme.Current()
	body := el.Div().Row().Grow().HFull()
	if a.mode == modeWorkspace && !a.previewing && !a.treeHidden && a.tree != nil && a.hasFiles {
		body.Child(el.Div().W(el.Dp(240)).HFull().Border(1, pal.Border).ScrollY().Child(a.tree.Render(cx)))
	}
	switch {
	case a.mode == modeWorkspace && a.path == "":
		body.Child(el.Div().Grow().HFull().P(24).Items(el.Start).Justify(el.Start).
			Child(el.Text(emptyHint).TextColor(pal.Muted)))
	case a.previewing:
		// Parsing the document once per change instead of once per frame keeps
		// a long file from re-parsing while the preview just sits there.
		if a.previewStale {
			a.previewStale = false
			a.preview.SetSource(a.text())
		}
		body.Child(el.Div().Grow().HFull().ScrollY().P(24).Child(a.preview.Render(cx)))
	default:
		body.Child(a.column(cx))
	}

	bar := a.bar.Left(textView{text: a.title(), color: pal.Muted})
	if state := a.stateText(); state != "" {
		c := pal.Muted
		if a.statusColor != nil {
			c = *a.statusColor
		}
		bar.Right(textView{text: state, color: c})
	}
	// The editor takes the keys while it draws, so the select-all has to be
	// noticed after the body is built, and applied in the next frame.
	a.widenSelection()

	root := el.Div().Col().WFull().HFull()
	return root.Child(body, bar.Render(cx))
}

// column draws every block in order: the active one as source, the rest as
// rendered markdown, with a trailing strip that starts a new paragraph.
func (a *app) column(cx *el.Context) el.Element {
	pal := theme.Current()
	col := el.Div().Col().Grow().HFull().ScrollY().Px(24).Py(20).Gap(10)
	for i := range a.blocks {
		if i == a.active {
			col.Child(el.Div().WFull().Child(a.activeEditor(cx)))
			continue
		}
		col.Child(a.blockView(cx, i, pal))
	}
	if len(a.blocks) == 0 {
		col.Child(el.Div().WFull().P(8).OnClick(a.appendBlock).
			Child(el.Text("点这里开始写").TextColor(pal.Muted)))
	}
	col.Child(el.Div().WFull().H(el.Dp(64)).OnClick(a.appendBlock))
	return col
}

// blockView renders one untouched block as markdown inside a press target.
func (a *app) blockView(cx *el.Context, i int, pal theme.Palette) el.Element {
	block := a.blocks[i]
	text := block.Text(a.doc)
	press := el.Div().WFull().Pl(float32(blockIndent)).OnClick(func() { a.activate(i) })
	if items, ok := taskItems(text); ok {
		return press.Child(taskList(cx, items))
	}
	if a.docs[i] == nil {
		a.docs[i] = a.newDoc(text)
	}
	return press.Child(a.docs[i].Render(cx))
}

// taskItem is one GFM task-list row read straight from the block source.
type taskItem struct {
	checked bool
	indent  int
	text    string
}

// taskItems reads a block that is nothing but a task list. The renderer keel
// ships draws its checked marker with U+2611, which the bundled fonts do not
// have, so a completed task loses its box; drawing the rows here keeps both
// states visible.
func taskItems(text string) ([]taskItem, bool) {
	var items []taskItem
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		body := line[indent:]
		if !strings.HasPrefix(body, "- [") && !strings.HasPrefix(body, "* [") {
			return nil, false
		}
		if len(body) < 6 || body[4] != ']' {
			return nil, false
		}
		state := body[3]
		if state != ' ' && state != 'x' && state != 'X' {
			return nil, false
		}
		items = append(items, taskItem{
			checked: state != ' ',
			indent:  indent,
			text:    strings.TrimPrefix(body[6:], " "),
		})
	}
	return items, len(items) > 0
}

// taskList draws the rows taskItems read, keeping each row's own markdown.
func taskList(cx *el.Context, items []taskItem) el.Element {
	list := el.Div().Col().WFull().Gap(6)
	for _, item := range items {
		row := el.Div().Row().WFull().Gap(6).Items(el.Start)
		if item.indent > 0 {
			row.Pl(float32(item.indent * 8))
		}
		row.Child(el.Div().W(el.Dp(16)).NoShrink().Items(el.End).Child(taskBox(item.checked)))
		row.Child(markdown.New(item.text).Render(cx))
		list.Child(row)
	}
	return list
}

// taskBox is the checkbox in front of a task row. It uses the check mark glyph
// because the bundled fonts have U+2713 but not U+2611.
func taskBox(checked bool) el.Element {
	pal := theme.Current()
	box := el.Div().W(el.Dp(14)).H(el.Dp(14)).Border(1, pal.Muted).
		Items(el.Center).Justify(el.Center)
	if !checked {
		return box
	}
	return box.Border(1, pal.Primary).Child(el.Text("✓").TextSize(10).TextColor(pal.Primary))
}

// activeEditor draws the block under the caret. The height follows the text
// until maxEditorHeight: keel draws every line of its buffer each frame, and a
// block past a few hundred lines costs tens of milliseconds, so a long block
// scrolls inside a bounded box instead of stretching the page.
func (a *app) activeEditor(cx *el.Context) el.Element {
	return el.Div().WFull().Child(a.editor.Height(a.editorHeight()).Render(cx))
}

// editorHeight follows the active block up to the bound, where the editor
// scrolls inside its own box instead of stretching the page.
func (a *app) editorHeight() float32 {
	lines := strings.Count(a.buffer(), "\n") + 1
	height := float32(lines*lineHeight + 24)
	if height > maxEditorHeight {
		return maxEditorHeight
	}
	return height
}

// widenSelection marks the whole document once the active block is fully
// selected, so the second Ctrl+A reaches every block. kit.CodeEditor cannot be
// handed a selection, and putting the file into one buffer to reuse its
// select-all costs tens of milliseconds per frame on a long document, so the
// document stays split and this only marks it as picked.
func (a *app) widenSelection() {
	if a.allSelected || a.sourceMode || a.previewing || a.active < 0 || len(a.blocks) < 2 {
		return
	}
	if a.editor.Selection() != a.editor.Value() {
		return
	}
	// The body is already built, so the editor has drawn and taken the key; the
	// status bar shows the change on the next frame.
	a.allSelected = true
	a.status = "整篇已选中，输入即替换"
	a.statusColor = nil
}

// replaceDocument makes typed the whole document, which is what the next edit
// means while Ctrl+A has the document selected.
func (a *app) replaceDocument(typed string) {
	a.splitting = true
	a.doc = typed
	a.split(typed)
	a.splitting = false
	if len(a.blocks) == 0 {
		a.appendBlock()
		return
	}
	a.activate(0)
	first := a.blocks[0].Text(a.doc)
	a.setBuffer(first, mddoc.PosOf(first, len(first)))
	a.touch(a.editor.Value())
}

// title is the path shown at the left of the status bar.
func (a *app) title() string {
	if a.path == "" {
		if a.dir != "" {
			return a.dir
		}
		return "未打开文件"
	}
	if a.mode == modeWorkspace && a.dir != "" {
		if rel, err := filepath.Rel(a.dir, a.path); err == nil {
			return rel
		}
	}
	return filepath.Base(a.path)
}

// stateText is the right-hand status: an error, the preview flag, or the save
// state (only single-file mode has unsaved text worth showing).
func (a *app) stateText() string {
	switch {
	case a.status != "":
		return a.status
	case a.previewing:
		return "预览"
	case a.mode == modeSingle && a.dirty:
		return "未保存"
	case a.path != "":
		return "已保存"
	}
	return ""
}

// shortcuts returns the key bindings the window registers.
func (a *app) shortcuts() map[string]func() {
	// The chords follow Typora's defaults, so muscle memory carries over.
	return map[string]func(){
		"ctrl+b":       func() { a.wrap("**", "**") },
		"ctrl+i":       func() { a.wrap("*", "*") },
		"ctrl+shift+`": func() { a.wrap("`", "`") },
		"alt+shift+5":  func() { a.wrap("~~", "~~") },
		"ctrl+k":       func() { a.wrap("[", "](url)") },
		"ctrl+v":       a.paste,
		"ctrl+s":       a.save,
		"enter":        a.newLine,
		"esc":          a.commit,
		"shift+enter":  a.softBreak,
		"ctrl+1":       func() { a.prefix("# ") },
		"ctrl+2":       func() { a.prefix("## ") },
		"ctrl+3":       func() { a.prefix("### ") },
		"ctrl+4":       func() { a.prefix("#### ") },
		"ctrl+5":       func() { a.prefix("##### ") },
		"ctrl+6":       func() { a.prefix("###### ") },
		"ctrl+0":       func() { a.prefix("") },
		"ctrl+shift+k": a.codeFence,
		"ctrl+shift+q": func() { a.prefix("> ") },
		"ctrl+shift+[": func() { a.prefix("1. ") },
		"ctrl+shift+]": func() { a.prefix("- ") },
		"ctrl+p":       a.togglePreview,
		"ctrl+/":       a.toggleSource,
		"ctrl+shift+l": a.toggleTree,
	}
}

// wrap toggles an inline marker around the selection of the block being edited.
func (a *app) wrap(open, close string) {
	if a.active < 0 {
		return
	}
	selected := strings.TrimSuffix(strings.TrimPrefix(a.editor.Selection(), blockGuard), blockGuard)
	out, caret := mddoc.WrapSel(a.buffer(), a.caret(), selected, open, close)
	a.replace(out, caret)
}

// prefix sets the block marker of the caret's line.
func (a *app) prefix(marker string) {
	if a.active < 0 {
		return
	}
	out, caret := mddoc.PrefixLine(a.buffer(), a.caret(), marker)
	a.replace(out, caret)
}

// togglePreview shows the whole document rendered and comes back.
func (a *app) togglePreview() {
	a.commit()
	a.previewing = !a.previewing
}

// toggleTree folds the sidebar away in the working-directory mode.
func (a *app) toggleTree() {
	a.treeHidden = !a.treeHidden
}

// toggleSource shows the whole document as one source block, the way Typora's
// source code mode does, and returns to blocks afterwards.
func (a *app) toggleSource() {
	a.commit()
	// One buffer for the whole file costs tens of milliseconds per frame once
	// it passes about a thousand lines, measured with go test -bench, so source
	// mode stops there instead of turning typing sluggish.
	a.sourceMode = !a.sourceMode
	if a.sourceMode {
		a.blocks = []mddoc.Block{{Start: 0, End: len(a.doc)}}
		a.docs = make([]*markdown.Doc, 1)
		a.active = -1
		a.before, a.after = "", ""
		a.activate(0)
		return
	}
	a.split(a.doc)
	if len(a.blocks) > 0 {
		a.activate(0)
	}
}

// codeFence turns the block under the caret into a fenced code block, and takes
// the fences off again when it already is one.
func (a *app) codeFence() {
	if a.active < 0 {
		return
	}
	lines := strings.Split(a.buffer(), "\n")
	if strings.HasPrefix(strings.TrimLeft(lines[0], " \t"), "```") {
		lines = lines[1:]
		if n := len(lines); n > 0 && strings.HasPrefix(strings.TrimLeft(lines[n-1], " \t"), "```") {
			lines = lines[:n-1]
		}
		a.replace(strings.Join(lines, "\n"), mddoc.Pos{Line: 0, Col: 0})
		return
	}
	a.replace("```\n"+a.buffer()+"\n```", mddoc.Pos{Line: 1, Col: 0})
}

// replace writes the edited source back and puts the caret where the command
// left it.
func (a *app) replace(src string, caret mddoc.Pos) {
	a.setValue(src, caret)
	a.pending = true
}

// newDoc builds a rendered block whose pictures load from the document's own
// folder, so a relative ![](assets/x.png) resolves next to the note.
func (a *app) newDoc(text string) *markdown.Doc {
	return markdown.New(text).ImageLoader(a.loadImage)
}

// docDir is the folder the open document lives in.
func (a *app) docDir() string {
	if a.path != "" {
		return filepath.Dir(a.path)
	}
	return a.dir
}

// loadImage reads a picture for the renderer. Remote sources stay off, so a
// note never reaches the network by accident.
func (a *app) loadImage(ctx context.Context, src string) (image.Image, error) {
	path, err := a.imagePath(src)
	if err != nil {
		a.reportImageError(src, err)
		return nil, err
	}
	img, err := markdown.DecodeImage(ctx, path)
	if err != nil {
		a.reportImageError(src, err)
		return nil, err
	}
	return img, nil
}

// imagePath turns the source written in markdown into a path on disk. A reader
// pastes all sorts: file URLs, percent escapes, <angle brackets> around spaces,
// and paths that are relative to the working directory rather than the note.
func (a *app) imagePath(src string) (string, error) {
	raw := strings.TrimSpace(src)
	raw = strings.TrimSuffix(strings.TrimPrefix(raw, "<"), ">")
	switch {
	case strings.HasPrefix(raw, "data:"):
		return "", fmt.Errorf("%s: inline data is not read", src)
	case strings.HasPrefix(raw, "file://"):
		u, err := url.Parse(raw)
		if err != nil {
			return "", err
		}
		raw = u.Path
		// file:///D:/pics/a.png keeps a slash in front of the drive letter.
		if len(raw) > 2 && raw[0] == '/' && raw[2] == ':' {
			raw = raw[1:]
		}
	case strings.Contains(raw, "://"):
		return "", fmt.Errorf("%s: only local pictures are read", src)
	}
	if unescaped, err := url.PathUnescape(raw); err == nil {
		raw = unescaped
	}
	// A path pasted from Explorer arrives with backslashes; treat those as
	// separators rather than as markdown escapes.
	path := filepath.FromSlash(strings.ReplaceAll(raw, "\\", "/"))
	if filepath.IsAbs(path) {
		return path, nil
	}
	for _, base := range []string{a.docDir(), a.dir, cwd()} {
		if base == "" {
			continue
		}
		candidate := filepath.Join(base, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	// Nothing found: decode the path the note suggests, so the message names
	// the place the reader expected the picture to be.
	return filepath.Join(a.docDir(), path), nil
}

// cwd is the directory the editor was started in. A note often names a picture
// relative to the project it lives in, which is that directory.
func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

// reportImageError says on the status bar which picture could not be read. The
// loader runs off the UI thread, so the state changes inside core.Update.
func (a *app) reportImageError(src string, err error) {
	if err == nil {
		return
	}
	core.Update(func() {
		a.status = "图片读取失败：" + media.Name(src)
		a.statusColor = nil
	})
}

// paste puts the clipboard into the open block: pictures are copied next to the
// document and shown, other files become markdown links, and plain text lands
// as it is.
func (a *app) paste() {
	if a.active < 0 || a.previewing {
		return
	}
	nativeclip.Read(func(data nativeclip.Data, err error) {
		if err != nil {
			core.Update(func() { a.fail(userError("粘贴", "剪贴板", err)) })
			return
		}
		core.Update(func() { a.insertPasted(data) })
	})
}

// insertPasted writes one clipboard snapshot at the caret.
func (a *app) insertPasted(data nativeclip.Data) {
	if a.active < 0 || a.previewing {
		return
	}
	// The window shortcut consumes the key, so the editor never runs its own
	// clipboard read: plain text has to be inserted here too.
	var parts []string
	for _, file := range data.Files {
		path, err := media.Copy(a.mediaDir, file)
		if err != nil {
			a.fail(userError("粘贴", media.Name(file), err))
			continue
		}
		if media.Image(file) {
			parts = append(parts, "!["+media.Name(file)+"]("+path+")")
			continue
		}
		parts = append(parts, "["+media.Name(file)+"]("+path+")")
	}
	// A picture held as bytes, e.g. a screenshot, arrives without a file name.
	for i, pic := range data.Images {
		name := fmt.Sprintf("pasted-%d%s", i+1, pasteExt(pic.MIME))
		path, err := media.Save(a.mediaDir, name, pic.Data)
		if err != nil {
			a.fail(userError("粘贴", name, err))
			continue
		}
		parts = append(parts, "![]("+path+")")
	}
	text := data.Text
	if len(parts) > 0 {
		text = strings.Join(parts, "\n")
	}
	if text == "" {
		return
	}
	out, caret := mddoc.Insert(a.buffer(), a.caret(), text)
	a.setValue(out, caret)
}

// pasteExt maps a clipboard media type to the extension the saved file gets.
func pasteExt(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/bmp":
		return ".bmp"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}

// treeNodes converts the scanned directory into keel tree nodes.
func treeNodes(nodes []*workspace.Node) []*kit.TreeNode {
	out := make([]*kit.TreeNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, &kit.TreeNode{ID: n.ID, Label: n.Label, Children: treeNodes(n.Children)})
	}
	return out
}

// firstFile returns the first markdown file in the tree, depth first.
func firstFile(nodes []*workspace.Node) string {
	for _, n := range nodes {
		if !n.Dir {
			return n.Path
		}
		if p := firstFile(n.Children); p != "" {
			return p
		}
	}
	return ""
}

// textView is the status bar's small text item. kit.StatusBarView.Left and
// Right take views rather than strings, so the label keeps its own token color
// and never collapses into the overflow menu.
type textView struct {
	text  string
	color color.NRGBA
}

func (t textView) Render(cx *el.Context) el.Element {
	return el.Text(t.text).TextSize(12).TextColor(t.color)
}

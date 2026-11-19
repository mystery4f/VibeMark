# VibeMark

English | [简体中文](README.zh-CN.md)

A small WYSIWYG Markdown editor for the desktop, built in Go on the
[keel](https://github.com/dyike/keel) UI framework. It is meant to be usable as
`$EDITOR`: point `git config core.editor` at it and commits open a real editor
instead of a terminal buffer.

## How it renders

Every top-level block is drawn by `ui/markdown`, so Markdown markers are gone and
headings keep their real size. Clicking a block turns that one block into its
source text; `Esc` (or clicking another block) puts it back. Only the block you
are editing is ever shown as source.

This is the deliberate trade-off recorded in
[`docs/adr/0002-block-editing-wysiwyg.md`](docs/adr/0002-block-editing-wysiwyg.md):
keel decorations cannot hide characters or change glyph width, so true WYSIWYG
needs a per-block editor rather than inline styling.

## Modes

- `vibemark <file>` — single file, blocking, for `$EDITOR`. Saves on `Ctrl+S`
  only; closing with unsaved changes exits `1` so git aborts the commit.
- `vibemark --dir <dir>` — browse a directory of Markdown files with a tree on
  the left. Saves automatically 300 ms after you stop typing.
- `vibemark` — reopens the last directory, or the current working directory.

Exit codes for the single-file mode: `0` saved, `1` cancelled, `2` could not
open or write (message on stderr).

## Shortcuts

| Chord | Action |
| --- | --- |
| `Enter` | Continue a list item, start a new block, or add an empty line |
| `Shift+Enter` | Line break inside the same block |
| `Ctrl+V` | Paste text, files or a screenshot: pictures are copied into `assets/` and shown, other files become links |
| `Backspace` / `Delete` at a block edge | Merge that block with its neighbour |
| `Esc` | Commit the active block |
| `Ctrl+S` | Save |
| `Ctrl+P` | Toggle the full-screen preview |
| `Ctrl+/` | Toggle source mode for the whole document |
| `Ctrl+Shift+L` | Toggle the file tree |
| `Ctrl+B` / `Ctrl+I` | Bold / italic around the selection |
| ``Ctrl+Shift+` `` | Inline code around the selection |
| `Alt+Shift+5` | Strikethrough around the selection |
| `Ctrl+K` | Wrap the selection as a link |
| `Ctrl+1` ... `Ctrl+6` | Heading level 1 ... 6 |
| `Ctrl+0` | Back to a plain paragraph |
| `Ctrl+Shift+K` | Toggle a fenced code block |
| `Ctrl+Shift+Q` | Blockquote |
| `Ctrl+Shift+[` / `Ctrl+Shift+]` | Ordered / bullet list |

The chords follow Typora's defaults, so muscle memory carries over.

## Pictures

A pasted or referenced picture is stored in an `assets/` folder next to the
document, so a note and its images travel together:

```markdown
![shot.png](assets/shot.png)
```

The renderer reads local files only; a remote `https://` source is ignored
instead of being fetched, so opening a note never reaches the network.

## Use it as git's editor

```sh
go build -o vibemark.exe .        # or: keel build
# put it on PATH, then:
git config --global core.editor vibemark
```

## Development

```sh
go build ./...
go test ./...       # UI tests run headless through keel's windowtest
gofmt -l .          # .githooks/pre-commit checks this, plus vet and test
```

`windowtest` synthesises keys but never touches the real keyboard or clipboard,
so the parts that depend on the OS are checked with keel's own automation
server, `keel-mcp` (installed by `go install
github.com/dyike/keel/cmd/keel-mcp@v0.4.11`, configured in
[`.pi/mcp.json`](.pi/mcp.json)):

```sh
go build -o .tmp/vibemark.exe .
# then, through the MCP tools:
#   launch  {"command": "…/.tmp/vibemark.exe --dir .tmp/demo"}
#   click   {"text": "正文"}
#   press_key {"key": "backspace"}
#   press_key {"key": "ctrl+v"}
```

It drives the real window off-screen (or with `visible: true`), so cross-block
deleting, clipboard pasting and autosave are verified end to end instead of
being assumed.

Enable the hooks once per clone: `git config core.hooksPath .githooks`.

Design records live in [`docs/design/vibemark-mvp.md`](docs/design/vibemark-mvp.md),
[`GLOSSARY.md`](GLOSSARY.md), and [`docs/adr/`](docs/adr/).

## Known limits

- The active block shows keel's line-number gutter and code background; keel
  hard-codes that gutter with no toggle, and switching to a plain text area
  would lose the cursor and selection APIs the wrap commands need.
- Undo covers the active block only.
- A click selects a block, not a character position inside it; click inside the
  opened block to place the caret.
- No system file dialog and no file drag-and-drop: keel v0.4.11 offers neither
  on desktop, so a picture arrives through the clipboard.
- Every active block carries an invisible `U+200B` guard at both ends; the
  buffer's API has no selection setter, so removing the guard is the only
  signal that Backspace crossed a block edge. The characters are stripped
  before anything is written.
- Ctrl+A selects the block under the caret, and a second Ctrl+A selects the
  whole file. In an empty paragraph it does nothing, because there is nothing
  there to select; press Ctrl+/ for source mode and Ctrl+A there instead.

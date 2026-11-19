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
| `Enter` | Continue a list item, or start a new block |
| `Shift+Enter` | Line break inside the same block |
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
- No system file dialog, no file drag-and-drop, and no images: keel v0.4.11
  offers none of them on desktop.

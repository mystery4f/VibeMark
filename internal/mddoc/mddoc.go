// Package mddoc turns markdown source into editor decorations and applies
// inline edits to a selection.
//
// The editor control (kit.CodeEditor) is the source of truth: decorations are
// rebuilt from the parsed document after every change, so deleting, moving and
// undoing never leaves a stale range behind.
package mddoc

import (
	"image/color"
	"sort"
	"strings"

	"github.com/dyike/keel/gio/font"
	"github.com/dyike/keel/ui/kit"
	"github.com/dyike/keel/ui/theme"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// Pos is a rune-based source position, matching kit.CodeRange.
type Pos struct{ Line, Col int }

// span is one decoration in byte offsets before the rune conversion.
type span struct {
	start, end int
	style      kit.CodeDecorationStyle
	color      *color.NRGBA
	weight     font.Weight
	italic     bool
}

var parser = goldmark.New(goldmark.WithExtensions(extension.GFM))

// lineOwner is implemented by blocks that carry their own text lines.
type lineOwner interface{ Lines() *text.Segments }

// blockLines returns the text lines of n, descending into the first child that
// owns them: a list item does not hold its lines, its first paragraph does.
func blockLines(n ast.Node) *text.Segments {
	if owner, ok := n.(lineOwner); ok {
		if lines := owner.Lines(); lines.Len() > 0 {
			return lines
		}
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if lines := blockLines(c); lines != nil {
			return lines
		}
	}
	return nil
}

// Decorations returns the decorations for src. Markers fade to the muted
// token, headings and links take the accent token, and emphasis keeps the
// syntax color while changing the glyph shape.
func Decorations(src string) []kit.CodeDecoration {
	if src == "" {
		return nil
	}
	raw := []byte(src)
	doc := parser.Parser().Parse(text.NewReader(raw))
	idx := newLineIndex(src)
	pal := theme.Current()

	muted := pal.Muted
	accent := pal.Primary

	var spans []span
	add := func(s, e int, st kit.CodeDecorationStyle, c *color.NRGBA, w font.Weight, it bool) {
		if s < 0 || e > len(src) || s >= e {
			return
		}
		spans = append(spans, span{start: s, end: e, style: st, color: c, weight: w, italic: it})
	}
	fade := func(s, e int) { add(s, e, kit.CodeDecorationText, &muted, 0, false) }

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.FencedCodeBlock:
			// Fence lines fade; the body keeps its syntax colors untouched.
			if lines := blockLines(node); lines != nil {
				first := idx.lineOf(lines.At(0).Start) - 1
				last := idx.lineOf(lines.At(lines.Len()-1).Stop) + 1
				for line := first; line <= last; line++ {
					idx.fadeFence(line, fade)
				}
			}
			return ast.WalkSkipChildren, nil

		case *ast.Heading:
			lines := node.Lines()
			if lines.Len() == 0 {
				return ast.WalkContinue, nil
			}
			textStart := lines.At(0).Start
			if line := idx.lineOf(textStart); line >= 0 {
				fade(idx.starts[line], textStart)
			}
			add(textStart, lines.At(lines.Len()-1).Stop, kit.CodeDecorationText, &accent, font.Bold, false)

		case *ast.Blockquote:
			if lines := blockLines(node); lines != nil {
				if line := idx.lineOf(lines.At(0).Start); line >= 0 {
					if markEnd := idx.markerEnd(line, ">"); markEnd > 0 {
						fade(idx.starts[line], markEnd)
					}
				}
			}

		case *ast.ListItem:
			if lines := blockLines(node); lines != nil {
				line := idx.lineOf(lines.At(0).Start)
				if line >= 0 {
					if markEnd := idx.markerEnd(line, ">-*+"); markEnd > 0 {
						end, lineEnd := markEnd, idx.lineEnd(line)
						for end < lineEnd && src[end] == ' ' {
							end++
						}
						// A task box fades with the marker; keel has no
						// strikethrough, so a low-contrast box is the honest mark.
						if end+2 < lineEnd && src[end] == '[' && src[end+2] == ']' {
							end += 3
						}
						fade(idx.starts[line], end)
					}
				}
			}

		case *ast.Emphasis:
			start, end, ok := inlineExtent(node, raw)
			if !ok {
				return ast.WalkContinue, nil
			}
			start, end = idx.expandDelimiters(start, end)
			if start < end {
				if node.Level >= 2 {
					add(start, end, kit.CodeDecorationText, nil, font.Bold, false)
				} else {
					add(start, end, kit.CodeDecorationText, nil, 0, true)
				}
			}

		case *ast.CodeSpan:
			start, end, ok := inlineExtent(node, raw)
			if !ok {
				return ast.WalkContinue, nil
			}
			start, end = idx.expandDelimiters(start, end)
			if start < end {
				add(start, end, kit.CodeDecorationText, &muted, 0, false)
			}

		case *ast.Link:
			if start, end, ok := linkExtent(node, raw); ok {
				// Colour alone would be the only link signal; the underline keeps
				// the distinction readable when colour is unavailable.
				add(start, end, kit.CodeDecorationText, &accent, 0, false)
				add(start, end, kit.CodeDecorationUnderline, &accent, 0, false)
			}
		}
		return ast.WalkContinue, nil
	})

	sort.SliceStable(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	out := make([]kit.CodeDecoration, 0, len(spans))
	for _, s := range spans {
		from, to := idx.pos(s.start), idx.pos(s.end)
		out = append(out, kit.CodeDecoration{
			Range:  kit.CodeRange{Line: from.Line, Col: from.Col, EndLine: to.Line, EndCol: to.Col},
			Style:  s.style,
			Color:  s.color,
			Weight: s.weight,
			Italic: s.italic,
		})
	}
	return out
}

// inlineExtent returns the byte range covered by a node's inline children.
func inlineExtent(n ast.Node, src []byte) (int, int, bool) {
	start, end := -1, -1
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch child := c.(type) {
		case *ast.Text:
			seg := child.Segment
			if start < 0 {
				start = seg.Start
			}
			end = seg.Stop
		case *ast.String:
			if start < 0 {
				start = 0
			}
			end = len(src)
		}
	}
	if start < 0 || end <= start {
		return 0, 0, false
	}
	return start, end, true
}

// linkExtent returns the range from the opening bracket to the closing paren.
func linkExtent(n *ast.Link, src []byte) (int, int, bool) {
	start, end, ok := inlineExtent(n, src)
	if !ok {
		return 0, 0, false
	}
	open := start
	for open > 0 && src[open-1] != '[' {
		open--
	}
	if open > 0 {
		open--
	}
	close := end
	for close < len(src) && src[close] != ')' {
		close++
	}
	if close < len(src) {
		close++
	}
	for ; open < close && (src[open] == ' ' || src[open] == '\t'); open++ {
	}
	if open >= close {
		return 0, 0, false
	}
	return open, close, true
}

// fadeFence fades one line when it is a code fence, so both the opening and the
// closing fence are covered whether goldmark counts the closing line or not.
func (l *lineIndex) fadeFence(line int, fade func(int, int)) {
	if line < 0 || line >= len(l.starts) {
		return
	}
	start, end := l.starts[line], l.lineEnd(line)
	trimmed := strings.TrimSpace(l.src[start:end])
	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
		fade(start, end)
	}
}

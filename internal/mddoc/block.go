package mddoc

import "strings"

// Block is one top-level chunk of a document: the unit the editor keeps
// rendered while the caret is elsewhere. Start and End are byte offsets into
// the source and End is exclusive.
type Block struct {
	Start int
	End   int
}

// Text returns the block's source.
func (b Block) Text(src string) string {
	if b.Start < 0 || b.End > len(src) || b.Start > b.End {
		return ""
	}
	return src[b.Start:b.End]
}

// Blocks splits src into top-level chunks at blank lines outside fenced code
// and display-math blocks. This is the same rule ui/markdown uses to decide
// what it can re-render on its own, so a block boundary never falls inside a
// fence.
func Blocks(src string) []Block {
	var blocks []Block
	start := -1
	fence := ""

	flush := func(end int) {
		if start < 0 {
			return
		}
		for start < end && (src[start] == '\n' || src[start] == '\r') {
			start++
		}
		for end > start && (src[end-1] == '\n' || src[end-1] == '\r' || src[end-1] == ' ' || src[end-1] == '\t') {
			end--
		}
		if end > start {
			blocks = append(blocks, Block{Start: start, End: end})
		}
		start = -1
	}

	offset := 0
	for offset <= len(src) {
		next := strings.IndexByte(src[offset:], '\n')
		var line string
		var lineEnd int
		if next < 0 {
			line, lineEnd = src[offset:], len(src)
		} else {
			line, lineEnd = src[offset:offset+next], offset+next+1
		}
		trimmed := strings.TrimSpace(line)

		marker := ""
		if fence == "" {
			marker = fenceMarker(trimmed)
		}
		switch {
		case fence != "":
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
		case marker != "":
			fence = marker
			if start < 0 {
				start = offset
			}
		case trimmed == "":
			flush(offset)
		default:
			if start < 0 {
				start = offset
			}
		}
		if next < 0 {
			break
		}
		offset = lineEnd
	}
	flush(len(src))
	return blocks
}

// fenceMarker returns the delimiter a trimmed line opens a fence with.
func fenceMarker(trimmed string) string {
	switch {
	case strings.HasPrefix(trimmed, "```"):
		return "```"
	case strings.HasPrefix(trimmed, "~~~"):
		return "~~~"
	case trimmed == "$$":
		return "$$"
	}
	return ""
}

// BlockAt returns the index of the block holding the byte offset, and -1 when
// the offset sits in the blank space between blocks.
func BlockAt(blocks []Block, offset int) int {
	for i, b := range blocks {
		if offset >= b.Start && offset < b.End {
			return i
		}
	}
	return -1
}

// Splice replaces a byte range with text and returns the new source.
func Splice(src string, start, end int, text string) string {
	if start < 0 || end > len(src) || start > end {
		return src
	}
	return src[:start] + text + src[end:]
}

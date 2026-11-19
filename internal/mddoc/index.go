package mddoc

import (
	"strings"
	"unicode/utf8"
)

// lineIndex maps byte offsets to rune positions; it is built once per parse so
// marker scanning and position conversion stay linear.
type lineIndex struct {
	src    string
	starts []int // byte offset of every line start
}

func newLineIndex(src string) *lineIndex {
	idx := &lineIndex{src: src, starts: []int{0}}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			idx.starts = append(idx.starts, i+1)
		}
	}
	return idx
}

// lineOf returns the 0-based line holding off, or -1 when off is negative.
func (l *lineIndex) lineOf(off int) int {
	if off < 0 || len(l.starts) == 0 {
		return -1
	}
	if off >= len(l.src) {
		return len(l.starts) - 1
	}
	lo, hi := 0, len(l.starts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if l.starts[mid] <= off {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// lineEnd returns the offset of the newline ending line.
func (l *lineIndex) lineEnd(line int) int {
	if line < 0 || line >= len(l.starts) {
		return len(l.src)
	}
	if line+1 < len(l.starts) {
		return l.starts[line+1] - 1
	}
	return len(l.src)
}

// pos converts a byte offset into the rune position kit.CodeRange expects.
func (l *lineIndex) pos(off int) Pos {
	line := l.lineOf(off)
	if line < 0 {
		return Pos{}
	}
	if off > len(l.src) {
		off = len(l.src)
	}
	start := l.starts[line]
	if off < start {
		off = start
	}
	return Pos{Line: line, Col: utf8.RuneCountInString(l.src[start:off])}
}

// markerEnd returns the byte offset just past the block marker that opens the
// line, or 0 when the line does not start with one. set holds marker bytes
// that count as a marker on their own; ordered markers need no entry because
// digits followed by "." or ")" are recognised directly.
func (l *lineIndex) markerEnd(line int, set string) int {
	if line < 0 || line >= len(l.starts) {
		return 0
	}
	i, end := l.starts[line], l.lineEnd(line)
	for i < end && (l.src[i] == ' ' || l.src[i] == '\t') {
		i++
	}
	if i >= end {
		return 0
	}
	j := i
	for j < end && l.src[j] >= '0' && l.src[j] <= '9' {
		j++
	}
	if j > i && j < end && (l.src[j] == '.' || l.src[j] == ')') {
		return j + 1
	}
	if strings.IndexByte(set, l.src[i]) >= 0 {
		return i + 1
	}
	return 0
}

const delimiterChars = "*_~`"

// expandDelimiters grows an inline text extent outward over the marker
// characters that wrap it, so the fade covers the markers themselves.
func (l *lineIndex) expandDelimiters(start, end int) (int, int) {
	for start > 0 && strings.IndexByte(delimiterChars, l.src[start-1]) >= 0 {
		start--
	}
	for end < len(l.src) && strings.IndexByte(delimiterChars, l.src[end]) >= 0 {
		end++
	}
	return start, end
}

// Offset converts a rune position into the byte offset that holds it.
func Offset(src string, p Pos) int {
	return offset(src, p)
}

// PosOf converts a byte offset into the rune position that holds it.
func PosOf(src string, off int) Pos {
	return newLineIndex(src).pos(off)
}

// offset converts a rune position back into a byte offset, clamped to src.
func offset(src string, p Pos) int {
	if p.Line < 0 || p.Col < 0 {
		return 0
	}
	line, start := 0, 0
	for i := 0; i < len(src) && line < p.Line; i++ {
		if src[i] == '\n' {
			line++
			start = i + 1
		}
	}
	if line < p.Line {
		return len(src)
	}
	off := start
	for col := 0; col < p.Col && off < len(src) && src[off] != '\n'; col++ {
		_, size := utf8.DecodeRuneInString(src[off:])
		off += size
	}
	return off
}

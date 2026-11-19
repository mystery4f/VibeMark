package mddoc

// Toggle wraps the selection in open and close, or removes a wrapping pair the
// selection already carries. An empty selection inserts the pair and leaves the
// caret between the two markers. The returned Pos is the caret after the edit.
func Toggle(src string, start, end Pos, open, close string) (string, Pos) {
	if open == "" && close == "" {
		return src, start
	}
	a, b := offset(src, start), offset(src, end)
	if a > b {
		a, b = b, a
	}
	switch {
	case a == b:
		out := src[:a] + open + close + src[a:]
		return out, newLineIndex(out).pos(a + len(open))
	case a >= len(open) && b+len(close) <= len(src) &&
		src[a-len(open):a] == open && src[b:b+len(close)] == close:
		out := src[:a-len(open)] + src[a:b] + src[b+len(close):]
		return out, newLineIndex(out).pos(b - len(open))
	default:
		out := src[:a] + open + src[a:b] + close + src[b:]
		return out, newLineIndex(out).pos(b + len(open) + len(close))
	}
}

// WrapSel wraps the selection the editor reports. caret is its active end and
// selected its text; the pair is located by matching that text backwards and
// then forwards from the caret. When neither matches, the call inserts the pair
// at the caret instead of guessing a range.
func WrapSel(src string, caret Pos, selected, open, close string) (string, Pos) {
	a := offset(src, caret)
	b := a
	if n := len(selected); n > 0 {
		switch {
		case a >= n && src[a-n:a] == selected:
			b, a = a, a-n
		case a+n <= len(src) && src[a:a+n] == selected:
			b = a + n
		}
	}
	idx := newLineIndex(src)
	return Toggle(src, idx.pos(a), idx.pos(b), open, close)
}

// PrefixLine sets the block marker of the caret's line, dropping an existing
// heading, quote or list marker first. Applying the marker the line already has
// removes it, so the same shortcut toggles.
func PrefixLine(src string, caret Pos, prefix string) (string, Pos) {
	idx := newLineIndex(src)
	line := caret.Line
	if line < 0 || line >= len(idx.starts) {
		return src, caret
	}
	start, end := idx.starts[line], idx.lineEnd(line)

	indent := start
	for indent < end && (src[indent] == ' ' || src[indent] == '\t') {
		indent++
	}

	// An existing heading wins over list and quote markers: "## text" keeps one
	// marker, never two.
	markEnd := indent
	if hashes := indent; hashes < end && src[hashes] == '#' {
		for hashes < end && src[hashes] == '#' {
			hashes++
		}
		if hashes < end && src[hashes] == ' ' {
			markEnd = hashes + 1
		}
	}
	if markEnd == indent {
		if found := idx.markerEnd(line, ">-*+"); found > 0 {
			markEnd = found
			for markEnd < end && src[markEnd] == ' ' {
				markEnd++
			}
		}
	}

	col := clamp(caret.Col-markEnd+indent, 0)
	if src[indent:markEnd] == prefix {
		return src[:indent] + src[markEnd:], Pos{Line: line, Col: col}
	}
	return src[:indent] + prefix + src[markEnd:], Pos{Line: line, Col: col + len(prefix)}
}

func clamp(v, low int) int {
	if v < low {
		return low
	}
	return v
}

package mddoc

import "testing"

func blockTexts(src string) []string {
	var out []string
	for _, b := range Blocks(src) {
		out = append(out, b.Text(src))
	}
	return out
}

func TestBlocksSplitsAtBlankLines(t *testing.T) {
	src := "第一段\n\n第二段\n继续第二段\n\n第三段\n"
	got := blockTexts(src)
	want := []string{"第一段", "第二段\n继续第二段", "第三段"}
	if len(got) != len(want) {
		t.Fatalf("blocks = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("block %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBlocksKeepsFencedCodeWhole(t *testing.T) {
	src := "说明\n\n```go\nline one\n\nline two\n```\n\n结尾"
	got := blockTexts(src)
	if len(got) != 3 {
		t.Fatalf("blocks = %q, want 3 blocks", got)
	}
	if got[1] != "```go\nline one\n\nline two\n```" {
		t.Fatalf("code block = %q", got[1])
	}
}

func TestBlocksKeepsMathWhole(t *testing.T) {
	src := "公式\n\n$$\na + b\n\nc + d\n$$\n\n之后"
	got := blockTexts(src)
	if len(got) != 3 {
		t.Fatalf("blocks = %q, want 3 blocks", got)
	}
	if got[1] != "$$\na + b\n\nc + d\n$$" {
		t.Fatalf("math block = %q", got[1])
	}
}

func TestBlocksTrimsBlankRun(t *testing.T) {
	src := "\n\n  第一段  \n\n\n\n第二段\n\n\n"
	got := blockTexts(src)
	// Leading indentation is source, not padding: keep it.
	want := []string{"  第一段", "第二段"}
	if len(got) != len(want) {
		t.Fatalf("blocks = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("block %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBlocksEmptySource(t *testing.T) {
	if got := Blocks(""); len(got) != 0 {
		t.Fatalf("blocks = %v, want none", got)
	}
	if got := Blocks("\n\n\n"); len(got) != 0 {
		t.Fatalf("blocks = %v, want none", got)
	}
}

func TestBlockAtFindsBlockAndGap(t *testing.T) {
	src := "甲甲\n\n乙乙"
	blocks := Blocks(src)
	if hunk := BlockAt(blocks, 0); hunk != 0 {
		t.Fatalf("offset 0 = block %d, want 0", hunk)
	}
	if hunk := BlockAt(blocks, len(src)-1); hunk != 1 {
		t.Fatalf("last offset = block %d, want 1", hunk)
	}
	if hunk := BlockAt(blocks, blocks[0].End); hunk != -1 {
		t.Fatalf("gap offset = block %d, want -1", hunk)
	}
}

func TestSpliceReplacesRange(t *testing.T) {
	src := "前缀块\n\n旧内容\n\n后缀块"
	blocks := Blocks(src)
	got := Splice(src, blocks[1].Start, blocks[1].End, "新内容")
	want := "前缀块\n\n新内容\n\n后缀块"
	if got != want {
		t.Fatalf("splice = %q, want %q", got, want)
	}
}

func TestSpliceRejectsBadRange(t *testing.T) {
	src := "abc"
	if got := Splice(src, 2, 1, "x"); got != src {
		t.Fatalf("splice = %q, want untouched %q", got, src)
	}
	if got := Splice(src, 0, 99, "x"); got != src {
		t.Fatalf("splice = %q, want untouched %q", got, src)
	}
}

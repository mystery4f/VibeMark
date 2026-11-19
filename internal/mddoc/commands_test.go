package mddoc

import "testing"

func TestInsertPutsTextAtTheCaret(t *testing.T) {
	out, caret := Insert("第一行\n第二行", Pos{Line: 0, Col: 3}, "\n")
	if out != "第一行\n\n第二行" {
		t.Fatalf("insert = %q, want a break after the first line", out)
	}
	if caret != (Pos{Line: 1, Col: 0}) {
		t.Fatalf("caret = %+v, want the start of the new line", caret)
	}
}

func TestListNextContinuesTheMarker(t *testing.T) {
	cases := []struct {
		line string
		want string
		ok   bool
	}{
		{"- 项目", "- ", true},
		{"* 项目", "* ", true},
		{"+ 项目", "+ ", true},
		{"  - 缩进项", "  - ", true},
		{"1. 第一项", "2. ", true},
		{"9) 第九项", "10) ", true},
		{"> 引用", "> ", true},
		{"- ", "", true},
		{"1. ", "", true},
		{"普通段落", "", false},
		{"# 标题", "", false},
	}
	for _, tc := range cases {
		got, ok := ListNext(tc.line)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ListNext(%q) = %q, %v, want %q, %v", tc.line, got, ok, tc.want, tc.ok)
		}
	}
}

func TestInsideFenceReportsFenceLines(t *testing.T) {
	src := "段落\n```go\n代码\n```\n段落\n$$\nx\n$$"
	// The fence counts as open once its opening line is consumed, so Enter at
	// the end of that line stays inside the block.
	cases := map[int]bool{0: false, 1: true, 2: true, 3: false, 4: false, 6: true, 7: false}
	for line, want := range cases {
		if got := InsideFence(src, line); got != want {
			t.Errorf("InsideFence(line %d) = %v, want %v", line, got, want)
		}
	}
}

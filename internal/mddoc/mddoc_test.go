package mddoc

import (
	"testing"

	"github.com/dyike/keel/gio/font"
	"github.com/dyike/keel/ui/kit"
)

// at returns the decorations whose range starts on line and covers col.
func at(decs []kit.CodeDecoration, line, col int) []kit.CodeDecoration {
	var out []kit.CodeDecoration
	for _, d := range decs {
		if d.Range.Line == line && d.Range.Col <= col && d.Range.EndCol > col {
			out = append(out, d)
		}
	}
	return out
}

func TestDecorationsEmpty(t *testing.T) {
	if got := Decorations(""); got != nil {
		t.Fatalf("Decorations(\"\") = %v, want nil", got)
	}
}

func TestHeadingFadesTheMarkerAndBoldsTheText(t *testing.T) {
	decs := Decorations("# 标题")
	if len(at(decs, 0, 0)) == 0 {
		t.Fatal("no decoration on the heading marker")
	}
	found := false
	for _, d := range at(decs, 0, 3) {
		if d.Weight == font.Bold && d.Color != nil {
			found = true
		}
	}
	if !found {
		t.Fatalf("heading text is not bold and accent colored: %+v", decs)
	}
}

func TestEmphasisCoversMarkersAndText(t *testing.T) {
	decs := Decorations("a **b** c")
	bold := false
	for _, d := range at(decs, 0, 4) {
		if d.Weight == font.Bold {
			bold = true
			if d.Range.Col != 2 || d.Range.EndCol != 7 {
				t.Fatalf("bold range = %d..%d, want 2..7", d.Range.Col, d.Range.EndCol)
			}
		}
	}
	if !bold {
		t.Fatalf("bold text carries no bold decoration: %+v", decs)
	}
}

func TestItalicSetsItalicOnly(t *testing.T) {
	decs := Decorations("_x_")
	found := false
	for _, d := range at(decs, 0, 1) {
		if d.Italic {
			found = true
			if d.Color != nil {
				t.Fatal("italic must keep the syntax color by leaving Color nil")
			}
		}
	}
	if !found {
		t.Fatalf("italic text carries no italic decoration: %+v", decs)
	}
}

func TestCodeSpanAndLinkAreDecorated(t *testing.T) {
	for _, src := range []string{"a `x` b", "a [t](u) b"} {
		decs := Decorations(src)
		if len(at(decs, 0, 3)) == 0 {
			t.Fatalf("%q: no decoration inside the span: %+v", src, decs)
		}
	}
}

func TestBlockMarkersFade(t *testing.T) {
	cases := []struct {
		src  string
		col  int
		what string
	}{
		{"- item", 0, "bullet marker"},
		{"1. item", 0, "ordered marker"},
		{"> quote", 0, "quote marker"},
		{"- [x] done", 2, "task box"},
	}
	for _, c := range cases {
		decs := Decorations(c.src)
		if len(at(decs, 0, c.col)) == 0 {
			t.Fatalf("%s at col %d is not decorated: %+v", c.what, c.col, decs)
		}
	}
}

func TestFencedCodeBlockFadesFencesOnly(t *testing.T) {
	src := "```go\ncode\n```\n"
	decs := Decorations(src)
	if len(at(decs, 0, 0)) == 0 {
		t.Fatal("opening fence is not decorated")
	}
	if len(at(decs, 2, 0)) == 0 {
		t.Fatal("closing fence is not decorated")
	}
	if len(at(decs, 1, 0)) != 0 {
		t.Fatal("code body must keep its syntax colors")
	}
}

func TestToggleInsertsPairAtCaret(t *testing.T) {
	out, caret := Toggle("ab", Pos{0, 1}, Pos{0, 1}, "**", "**")
	if out != "a****b" {
		t.Fatalf("got %q", out)
	}
	if caret != (Pos{0, 3}) {
		t.Fatalf("caret = %+v, want between the markers", caret)
	}
}

func TestToggleWrapsSelection(t *testing.T) {
	out, caret := Toggle("ab", Pos{0, 0}, Pos{0, 2}, "*", "*")
	if out != "*ab*" {
		t.Fatalf("got %q", out)
	}
	if caret.Col != 4 {
		t.Fatalf("caret = %+v, want past the closing marker", caret)
	}
}

func TestToggleRemovesExistingPair(t *testing.T) {
	out, _ := Toggle("*ab*", Pos{0, 1}, Pos{0, 3}, "*", "*")
	if out != "ab" {
		t.Fatalf("got %q", out)
	}
}

func TestWrapSelLocatesTheSelection(t *testing.T) {
	src := "one two two"
	// caret after the second "two", selection reported by the editor
	out, _ := WrapSel(src, Pos{0, 11}, "two", "**", "**")
	if out != "one two **two**" {
		t.Fatalf("got %q", out)
	}
}

func TestWrapSelFallsBackToInsert(t *testing.T) {
	out, _ := WrapSel("ab", Pos{0, 1}, "zz", "*", "*")
	if out != "a**b" {
		t.Fatalf("got %q", out)
	}
}

func TestPrefixLineSetsAndClearsHeading(t *testing.T) {
	out, caret := PrefixLine("text", Pos{0, 0}, "# ")
	if out != "# text" {
		t.Fatalf("got %q", out)
	}
	if caret.Col != 2 {
		t.Fatalf("caret = %+v", caret)
	}

	out, _ = PrefixLine("# text", Pos{0, 2}, "## ")
	if out != "## text" {
		t.Fatalf("replacing a heading gave %q", out)
	}

	out, _ = PrefixLine("## text", Pos{0, 3}, "## ")
	if out != "text" {
		t.Fatalf("clearing the same marker gave %q", out)
	}
}

func TestPrefixLineReplacesListMarker(t *testing.T) {
	out, _ := PrefixLine("- item", Pos{0, 2}, "> ")
	if out != "> item" {
		t.Fatalf("got %q", out)
	}
}

func TestPrefixLineKeepsTextOnOtherLines(t *testing.T) {
	out, caret := PrefixLine("a\nb", Pos{1, 0}, "- ")
	if out != "a\n- b" {
		t.Fatalf("got %q", out)
	}
	if caret.Line != 1 || caret.Col != 2 {
		t.Fatalf("caret = %+v", caret)
	}
}

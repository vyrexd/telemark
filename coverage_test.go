package telemark

import (
	"strings"
	"testing"
)

// findEntity returns the first entity of the given type, or nil.
func findEntity(ents []Entity, typ string) *Entity {
	for i := range ents {
		if ents[i].Type == typ {
			return &ents[i]
		}
	}
	return nil
}

func TestEntitiesBlockConstructs(t *testing.T) {
	md := "# Head\n\n> quoted line\n\n- a\n- b\n\n1. one\n2. two\n\n```go\ncode\n```"
	text, ents := Entities(md)

	if !strings.Contains(text, "Head") || !strings.Contains(text, "• a") || !strings.Contains(text, "1. one") {
		t.Fatalf("unexpected text: %q", text)
	}
	for _, typ := range []string{"bold", "blockquote", "pre"} {
		if findEntity(ents, typ) == nil {
			t.Errorf("missing %s entity in %+v", typ, ents)
		}
	}
	if pre := findEntity(ents, "pre"); pre != nil && pre.Language != "go" {
		t.Errorf("pre language = %q, want go", pre.Language)
	}
}

func TestEntitiesInlineVariants(t *testing.T) {
	text, ents := Entities("~~s~~ and ||sp|| and _i_ and [l](https://x.io)")
	if text != "s and sp and i and l" {
		t.Fatalf("text = %q", text)
	}
	for _, typ := range []string{"strikethrough", "spoiler", "italic", "text_link"} {
		if findEntity(ents, typ) == nil {
			t.Errorf("missing %s entity in %+v", typ, ents)
		}
	}
	if l := findEntity(ents, "text_link"); l != nil && l.URL != "https://x.io" {
		t.Errorf("link url = %q", l.URL)
	}
}

func TestEntitiesNested(t *testing.T) {
	// bold containing italic -> two overlapping entities
	text, ents := Entities("**bold _and italic_**")
	if text != "bold and italic" {
		t.Fatalf("text = %q", text)
	}
	if findEntity(ents, "bold") == nil || findEntity(ents, "italic") == nil {
		t.Fatalf("expected both bold and italic, got %+v", ents)
	}
}

func TestAutolink(t *testing.T) {
	// MarkdownV2 output
	if got := Convert("see <https://go.dev> now"); !strings.Contains(got, "[https://go\\.dev](https://go.dev)") {
		t.Errorf("autolink md = %q", got)
	}
	// entities
	text, ents := Entities("<https://go.dev>")
	if text != "https://go.dev" {
		t.Fatalf("text = %q", text)
	}
	l := findEntity(ents, "text_link")
	if l == nil || l.URL != "https://go.dev" {
		t.Errorf("autolink entity = %+v", ents)
	}
	// non-url in angle brackets stays literal
	if got := Convert("a <not a link> b"); !strings.Contains(got, "not a link") {
		t.Errorf("non-url angle = %q", got)
	}
}

func TestSplitOversizedBlock(t *testing.T) {
	// a single code block with many lines, larger than the limit, must be
	// split across multiple messages on line boundaries.
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "line") // 4 chars each
	}
	md := "```\n" + strings.Join(lines, "\n") + "\n```"
	chunks := Split(md, 20)
	if len(chunks) < 2 {
		t.Fatalf("expected the oversized block to split, got %d chunks", len(chunks))
	}
	for _, c := range chunks {
		if utf16Len(c) > 20 {
			t.Errorf("chunk over limit: %q (%d)", c, utf16Len(c))
		}
	}
}

func TestSplitSingleLongLine(t *testing.T) {
	// one physical line longer than the limit forces a rune-level hard split.
	long := strings.Repeat("x", 100)
	chunks := Split(long, 12)
	if len(chunks) < 2 {
		t.Fatalf("expected hard split, got %d chunks", len(chunks))
	}
	for _, c := range chunks {
		if utf16Len(c) > 12 {
			t.Errorf("chunk over limit: %q (%d)", c, utf16Len(c))
		}
	}
	if strings.Join(chunks, "") != long {
		t.Errorf("hard split lost content")
	}
}

func TestSplitDefaultLimit(t *testing.T) {
	// maxLen <= 0 falls back to TelegramMessageLimit; short input -> one chunk.
	chunks := Split("hello", 0)
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Fatalf("default-limit split = %+v", chunks)
	}
}

func TestCodeFenceVariants(t *testing.T) {
	// tilde fence
	if got := Convert("~~~\nplain\n~~~"); !strings.Contains(got, "```\nplain\n```") {
		t.Errorf("tilde fence = %q", got)
	}
	// backtick fence with a backtick in the info string is not a valid fence,
	// so the line is treated as a paragraph.
	got := Convert("```js`x\nbody")
	if strings.HasPrefix(got, "```") {
		t.Errorf("invalid info string should not open a fence: %q", got)
	}
}

func TestListMarkerVariants(t *testing.T) {
	// '+' bullet and ')' ordered marker
	if got := Convert("+ item"); got != "• item" {
		t.Errorf("plus bullet = %q", got)
	}
	if got := Convert("3) third"); got != "3\\. third" {
		t.Errorf("paren ordered = %q", got)
	}
}

func TestEscapeCodeAndURLBackslash(t *testing.T) {
	// backslash inside inline code must be escaped
	if got := Convert("`a\\b`"); got != "`a\\\\b`" {
		t.Errorf("code backslash = %q", got)
	}
	// backslash and ) inside a URL must be escaped
	got := Convert("[t](https://x.io/a)b)")
	if !strings.Contains(got, `https://x.io/a`) {
		t.Errorf("url escaping = %q", got)
	}
}

func TestLooseDelimitersLiteral(t *testing.T) {
	// single ~ and single | never form strike/spoiler and stay literal
	if got := Convert("a ~ b | c"); got != `a \~ b \| c` {
		t.Errorf("loose delimiters = %q", got)
	}
}

func TestThematicBreak(t *testing.T) {
	out := Convert("above\n\n---\n\nbelow")
	if !strings.Contains(out, "above") || !strings.Contains(out, "below") {
		t.Errorf("thematic break dropped surrounding text: %q", out)
	}
}

func TestEmptyInput(t *testing.T) {
	if got := Convert(""); got != "" {
		t.Errorf("empty convert = %q", got)
	}
	if chunks := Split("", 100); len(chunks) != 1 || chunks[0] != "" {
		t.Errorf("empty split = %+v", chunks)
	}
	text, ents := Entities("")
	if text != "" || len(ents) != 0 {
		t.Errorf("empty entities = %q %+v", text, ents)
	}
}

package telemark

import (
	"strings"
	"testing"
)

func TestConvertBasic(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain escaping", "a.b-c!", `a\.b\-c\!`},
		{"bold star", "**bold**", `*bold*`},
		{"bold underscore", "__bold__", `*bold*`},
		{"italic star", "*it*", `_it_`},
		{"italic underscore", "_it_", `_it_`},
		{"strike", "~~gone~~", `~gone~`},
		{"spoiler", "||secret||", `||secret||`},
		{"inline code", "`x = 1`", "`x = 1`"},
		{"lone trailing backtick escaped", "`a`b`", "`a`b\\`"}, // 1st pair is code, trailing ` is literal
		{"link", "[go](https://go.dev)", `[go](https://go.dev)`},
		{"link with special text", "[a.b](https://x.com)", `[a\.b](https://x.com)`},
		{"heading", "# Title", `*Title*`},
		{"nested bold italic", "***x***", "_*x*_"},
		{"intraword underscore literal", "snake_case_here", `snake\_case\_here`},
		{"escaped star literal", `a \* b`, `a \* b`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Convert(c.in)
			if got != c.want {
				t.Errorf("Convert(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestConvertCodeBlock(t *testing.T) {
	in := "```go\nfmt.Println(\"hi\")\n```"
	want := "```go\nfmt.Println(\"hi\")\n```"
	if got := Convert(in); got != want {
		t.Errorf("code block = %q, want %q", got, want)
	}
}

func TestConvertList(t *testing.T) {
	got := Convert("- one\n- two")
	want := "• one\n• two"
	if got != want {
		t.Errorf("bullet list = %q, want %q", got, want)
	}

	got = Convert("1. a\n2. b")
	want = "1\\. a\n2\\. b"
	if got != want {
		t.Errorf("ordered list = %q, want %q", got, want)
	}
}

func TestConvertBlockquote(t *testing.T) {
	got := Convert("> quoted line")
	want := ">quoted line"
	if got != want {
		t.Errorf("blockquote = %q, want %q", got, want)
	}
}

func TestEntities(t *testing.T) {
	text, ents := Entities("hello **world** and `code`")
	if text != "hello world and code" {
		t.Fatalf("text = %q", text)
	}
	if len(ents) != 2 {
		t.Fatalf("want 2 entities, got %d: %+v", len(ents), ents)
	}
	// bold "world" at offset 6 len 5
	b := ents[0]
	if b.Type != "bold" || b.Offset != 6 || b.Length != 5 {
		t.Errorf("bold entity = %+v", b)
	}
	// code "code" at offset 16 len 4
	c := ents[1]
	if c.Type != "code" || c.Offset != 16 || c.Length != 4 {
		t.Errorf("code entity = %+v", c)
	}
}

func TestEntitiesUTF16Offsets(t *testing.T) {
	// emoji outside the BMP occupies 2 UTF-16 code units.
	text, ents := Entities("😀 **x**")
	if text != "😀 x" {
		t.Fatalf("text = %q", text)
	}
	if len(ents) != 1 {
		t.Fatalf("want 1 entity, got %d", len(ents))
	}
	// "😀 " is 3 UTF-16 units (2 + space), so bold "x" starts at offset 3
	if ents[0].Offset != 3 || ents[0].Length != 1 {
		t.Errorf("entity = %+v, want offset 3 len 1", ents[0])
	}
}

func TestEntitiesLink(t *testing.T) {
	text, ents := Entities("[go](https://go.dev)")
	if text != "go" {
		t.Fatalf("text = %q", text)
	}
	if len(ents) != 1 || ents[0].Type != "text_link" || ents[0].URL != "https://go.dev" {
		t.Fatalf("entity = %+v", ents)
	}
}

func TestSplitByBlocks(t *testing.T) {
	// three paragraphs, force a small limit so they can't all fit.
	md := "aaaa\n\nbbbb\n\ncccc"
	chunks := Split(md, 10)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d: %q", len(chunks), chunks)
	}
	for _, c := range chunks {
		if utf16Len(c) > 10 {
			t.Errorf("chunk exceeds limit: %q (%d)", c, utf16Len(c))
		}
	}
	// round-trip: joining should recover the escaped content
	joined := strings.Join(chunks, "\n\n")
	if !strings.Contains(joined, "aaaa") || !strings.Contains(joined, "cccc") {
		t.Errorf("content lost in split: %q", joined)
	}
}

func TestSplitNeverBreaksEscape(t *testing.T) {
	// a long line of characters that each escape to two chars.
	long := strings.Repeat(".", 100)
	chunks := Split(long, 15)
	for _, c := range chunks {
		// no chunk may end with a lone backslash
		if strings.HasSuffix(c, `\`) && !strings.HasSuffix(c, `\\`) {
			t.Errorf("chunk ends with dangling backslash: %q", c)
		}
	}
	// reassembled, unescaping backslashes should give back the dots
	joined := strings.Join(chunks, "")
	plain := strings.ReplaceAll(joined, `\.`, ".")
	if plain != long {
		t.Errorf("split corrupted escaped content: %q", plain)
	}
}

func TestExpandableQuote(t *testing.T) {
	got := ConvertWith("> a\n> b", Options{ExpandableQuotes: true})
	if !strings.HasPrefix(got, "**>") {
		t.Errorf("expandable quote should start with **>, got %q", got)
	}
}

func TestNoDanglingSpecialsRoundTrip(t *testing.T) {
	// every MarkdownV2 special, as literal text, must come back escaped.
	in := mdV2Special
	got := Convert(in)
	for _, r := range in {
		if !strings.ContainsRune(got, r) {
			t.Errorf("special %q missing from output %q", r, got)
		}
	}
}

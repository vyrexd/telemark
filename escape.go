package telemark

import (
	"strings"
	"unicode/utf16"
)

// mdV2Special is the set of characters that must be backslash-escaped in
// ordinary MarkdownV2 text, per the Telegram Bot API documentation.
const mdV2Special = "_*[]()~`>#+-=|{}.!\\"

// escapeText escapes every MarkdownV2 special character in s so it renders as
// literal text.
func escapeText(s string) string {
	var b strings.Builder
	b.Grow(len(s) + len(s)/8)
	for _, r := range s {
		if r < 128 && strings.IndexByte(mdV2Special, byte(r)) >= 0 {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// escapeCode escapes the characters that are special inside `inline` and
// ```pre``` entities: only the backtick and the backslash.
func escapeCode(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		if r == '`' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// escapeURL escapes the characters that are special inside the parenthesised
// destination of an inline link: `)` and `\`.
func escapeURL(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		if r == ')' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// utf16Len returns the length of s measured in UTF-16 code units, which is the
// unit Telegram uses for MessageEntity offsets and lengths.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if utf16.IsSurrogate(r) {
			// invalid rune, counts as replacement char (1 unit)
			n++
			continue
		}
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

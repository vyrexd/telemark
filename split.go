package telemark

import (
	"strings"
)

// splitBlocks renders each top-level block independently and greedily packs the
// rendered strings into chunks no longer than maxLen (measured in UTF-16 code
// units). Splitting at block boundaries guarantees no formatting token is ever
// cut in half. A single block that is itself too large is split line-by-line as
// a best-effort fallback.
func splitBlocks(blocks []*node, opts Options, maxLen int) []string {
	const sep = "\n\n"
	sepLen := utf16Len(sep)

	var chunks []string
	var cur strings.Builder
	curLen := 0

	flush := func() {
		if cur.Len() > 0 {
			chunks = append(chunks, cur.String())
			cur.Reset()
			curLen = 0
		}
	}

	appendPiece := func(s string) {
		l := utf16Len(s)
		if curLen == 0 {
			cur.WriteString(s)
			curLen = l
			return
		}
		if curLen+sepLen+l <= maxLen {
			cur.WriteString(sep)
			cur.WriteString(s)
			curLen += sepLen + l
			return
		}
		flush()
		cur.WriteString(s)
		curLen = l
	}

	for _, blk := range blocks {
		rendered := renderBlockMD(blk, opts)
		if utf16Len(rendered) <= maxLen {
			appendPiece(rendered)
			continue
		}
		// oversized block: flush and split it on its own
		flush()
		chunks = append(chunks, splitOversized(rendered, maxLen)...)
	}
	flush()

	if len(chunks) == 0 {
		return []string{""}
	}
	return chunks
}

// splitOversized breaks a single rendered block that exceeds maxLen. It prefers
// line boundaries, then falls back to a rune-safe hard split that never cuts
// between a backslash and the character it escapes.
func splitOversized(s string, maxLen int) []string {
	var out []string
	var cur strings.Builder
	curLen := 0

	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
			curLen = 0
		}
	}

	for _, line := range strings.Split(s, "\n") {
		ll := utf16Len(line)
		if ll > maxLen {
			flush()
			out = append(out, hardSplit(line, maxLen)...)
			continue
		}
		add := ll
		if curLen > 0 {
			add++ // for the '\n'
		}
		if curLen+add > maxLen {
			flush()
			cur.WriteString(line)
			curLen = ll
			continue
		}
		if curLen > 0 {
			cur.WriteByte('\n')
		}
		cur.WriteString(line)
		curLen += add
	}
	flush()
	return out
}

// hardSplit cuts s into <=maxLen chunks along rune boundaries without ever
// separating a backslash escape from its target character.
func hardSplit(s string, maxLen int) []string {
	var out []string
	var cur strings.Builder
	curLen := 0

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		w := 1
		if r > 0xFFFF {
			w = 2
		}
		// keep a backslash together with the char it escapes
		pair := 0
		if r == '\\' && i+1 < len(runes) {
			nr := runes[i+1]
			pair = 1
			if nr > 0xFFFF {
				pair = 2
			}
		}
		if curLen+w+pair > maxLen && curLen > 0 {
			out = append(out, cur.String())
			cur.Reset()
			curLen = 0
		}
		cur.WriteRune(r)
		curLen += w
		if pair > 0 {
			i++
			cur.WriteRune(runes[i])
			curLen += pair
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

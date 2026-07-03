package telemark

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// parseInline turns a run of inline Markdown source into a slice of nodes.
//
// Processing order mirrors CommonMark: code spans and links bind tighter than
// emphasis, so they are resolved first into opaque atoms, then emphasis
// delimiters are matched around them with a delimiter stack.
func parseInline(s string) []*node {
	items := tokenizeInline(s)
	return processEmphasis(items)
}

// itemKind distinguishes the three things the emphasis pass operates on.
type itemKind int

const (
	itemText  itemKind = iota // literal text run (no delimiters, no specials)
	itemDelim                 // a run of identical emphasis delimiter chars
	itemNode                  // an already-resolved node (code span, link, break)
)

type inlineItem struct {
	kind itemKind
	text string // itemText
	n    *node  // itemNode

	// itemDelim fields
	dchar    byte
	dcount   int
	canOpen  bool
	canClose bool
}

// tokenizeInline scans s and resolves code spans and links, splitting the
// remaining text into literal runs and delimiter runs.
func tokenizeInline(s string) []*inlineItem {
	var items []*inlineItem
	var buf strings.Builder

	flush := func() {
		if buf.Len() > 0 {
			items = append(items, &inlineItem{kind: itemText, text: buf.String()})
			buf.Reset()
		}
	}

	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '`':
			if node, next, ok := scanCodeSpan(s, i); ok {
				flush()
				items = append(items, &inlineItem{kind: itemNode, n: node})
				i = next
				continue
			}
			buf.WriteByte(c)
			i++

		case c == '!' && i+1 < len(s) && s[i+1] == '[':
			// image: degrade to a link to its destination using the alt text.
			if node, next, ok := scanLink(s, i+1); ok {
				flush()
				items = append(items, &inlineItem{kind: itemNode, n: node})
				i = next
				continue
			}
			buf.WriteByte(c)
			i++

		case c == '[':
			if node, next, ok := scanLink(s, i); ok {
				flush()
				items = append(items, &inlineItem{kind: itemNode, n: node})
				i = next
				continue
			}
			buf.WriteByte(c)
			i++

		case c == '<':
			if node, next, ok := scanAutolink(s, i); ok {
				flush()
				items = append(items, &inlineItem{kind: itemNode, n: node})
				i = next
				continue
			}
			buf.WriteByte(c)
			i++

		case c == '*' || c == '_' || c == '~' || c == '|':
			// read a maximal run of the same delimiter char
			j := i
			for j < len(s) && s[j] == c {
				j++
			}
			before := lastRune(s[:i])
			after := firstRune(s[j:])
			it := &inlineItem{kind: itemDelim, dchar: c, dcount: j - i}
			it.canOpen, it.canClose = flanking(c, before, after)
			flush()
			items = append(items, it)
			i = j

		case c == '\\' && i+1 < len(s):
			// backslash escape: the next char is taken literally
			buf.WriteByte(s[i+1])
			i += 2

		default:
			buf.WriteByte(c)
			i++
		}
	}
	flush()
	return items
}

// scanCodeSpan parses a backtick code span starting at s[i] == '`'.
func scanCodeSpan(s string, i int) (*node, int, bool) {
	n := 0
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	open := i + n
	// find a closing run of exactly n backticks
	for k := open; k < len(s); k++ {
		if s[k] != '`' {
			continue
		}
		m := 0
		for k+m < len(s) && s[k+m] == '`' {
			m++
		}
		if m == n {
			content := s[open:k]
			// per CommonMark, strip a single leading & trailing space when the
			// content is not all spaces.
			if len(content) >= 2 && content[0] == ' ' && content[len(content)-1] == ' ' &&
				strings.TrimSpace(content) != "" {
				content = content[1 : len(content)-1]
			}
			return &node{typ: nodeCode, literal: content}, k + m, true
		}
		k += m - 1
	}
	return nil, 0, false
}

// scanLink parses [text](url) starting at s[i] == '['.
func scanLink(s string, i int) (*node, int, bool) {
	// find matching ] respecting one level of nested brackets
	depth := 0
	end := -1
	for k := i; k < len(s); k++ {
		switch s[k] {
		case '\\':
			k++
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = k
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 || end+1 >= len(s) || s[end+1] != '(' {
		return nil, 0, false
	}
	text := s[i+1 : end]
	// parse destination up to the matching ')'
	dstart := end + 2
	dend := -1
	for k := dstart; k < len(s); k++ {
		if s[k] == '\\' {
			k++
			continue
		}
		if s[k] == ')' {
			dend = k
			break
		}
	}
	if dend < 0 {
		return nil, 0, false
	}
	dest := strings.TrimSpace(s[dstart:dend])
	// drop an optional "title" part: url "title"
	if sp := strings.IndexByte(dest, ' '); sp >= 0 {
		dest = dest[:sp]
	}
	dest = strings.Trim(dest, "<>")
	n := &node{typ: nodeLink, url: dest, children: parseInline(text)}
	return n, dend + 1, true
}

// scanAutolink parses <https://...> starting at s[i] == '<'.
func scanAutolink(s string, i int) (*node, int, bool) {
	end := strings.IndexByte(s[i:], '>')
	if end < 0 {
		return nil, 0, false
	}
	end += i
	inner := s[i+1 : end]
	if !strings.HasPrefix(inner, "http://") && !strings.HasPrefix(inner, "https://") &&
		!strings.HasPrefix(inner, "tg://") {
		return nil, 0, false
	}
	n := &node{typ: nodeLink, url: inner, children: []*node{{typ: nodeText, literal: inner}}}
	return n, end + 1, true
}

// flanking computes whether a delimiter run can open and/or close emphasis,
// following CommonMark's left/right-flanking rules (with the intraword
// restriction for '_').
func flanking(c byte, before, after rune) (canOpen, canClose bool) {
	beforeWS := before == 0 || unicode.IsSpace(before)
	afterWS := after == 0 || unicode.IsSpace(after)
	beforePunct := isPunct(before)
	afterPunct := isPunct(after)

	leftFlank := !afterWS && (!afterPunct || beforeWS || beforePunct)
	rightFlank := !beforeWS && (!beforePunct || afterWS || afterPunct)

	switch c {
	case '_':
		canOpen = leftFlank && (!rightFlank || beforePunct)
		canClose = rightFlank && (!leftFlank || afterPunct)
	default:
		canOpen = leftFlank
		canClose = rightFlank
	}
	return
}

func isPunct(r rune) bool {
	if r == 0 {
		return false
	}
	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}

func lastRune(s string) rune {
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

func firstRune(s string) rune {
	if s == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r
}

// processEmphasis repeatedly matches the earliest closable delimiter with the
// nearest compatible opener, wrapping the items between them into a formatting
// node, until no matches remain. Remaining delimiters degrade to literal text.
func processEmphasis(items []*inlineItem) []*node {
	for i := 0; i < len(items); i++ {
		cl := items[i]
		if cl.kind != itemDelim || !cl.canClose {
			continue
		}
		for j := i - 1; j >= 0; j-- {
			op := items[j]
			if op.kind != itemDelim || !op.canOpen || op.dchar != cl.dchar {
				continue
			}
			k, typ, ok := matchDelims(cl.dchar, op.dcount, cl.dcount)
			if !ok {
				continue
			}
			inner := processEmphasis(cloneRange(items[j+1 : i]))
			wrap := &node{typ: typ, children: inner}

			op.dcount -= k
			cl.dcount -= k

			rebuilt := make([]*inlineItem, 0, len(items))
			rebuilt = append(rebuilt, items[:j]...)
			if op.dcount > 0 {
				rebuilt = append(rebuilt, op)
			}
			rebuilt = append(rebuilt, &inlineItem{kind: itemNode, n: wrap})
			if cl.dcount > 0 {
				rebuilt = append(rebuilt, cl)
			}
			rebuilt = append(rebuilt, items[i+1:]...)
			return processEmphasis(rebuilt)
		}
	}
	return finalizeItems(items)
}

// matchDelims decides how many delimiter chars to consume and which node type
// to emit for a given opener/closer pair.
func matchDelims(c byte, openCount, closeCount int) (consume int, typ nodeType, ok bool) {
	switch c {
	case '*', '_':
		if openCount >= 2 && closeCount >= 2 {
			return 2, nodeBold, true
		}
		return 1, nodeItalic, true
	case '~':
		if openCount >= 2 && closeCount >= 2 {
			return 2, nodeStrike, true
		}
		return 0, 0, false
	case '|':
		if openCount >= 2 && closeCount >= 2 {
			return 2, nodeSpoiler, true
		}
		return 0, 0, false
	}
	return 0, 0, false
}

func cloneRange(items []*inlineItem) []*inlineItem {
	out := make([]*inlineItem, len(items))
	copy(out, items)
	return out
}

// finalizeItems converts any leftover items into text/formatting nodes, turning
// unmatched delimiter runs back into literal characters.
func finalizeItems(items []*inlineItem) []*node {
	var out []*node
	for _, it := range items {
		switch it.kind {
		case itemText:
			out = append(out, &node{typ: nodeText, literal: it.text})
		case itemNode:
			out = append(out, it.n)
		case itemDelim:
			if it.dcount > 0 {
				out = append(out, &node{typ: nodeText, literal: strings.Repeat(string(it.dchar), it.dcount)})
			}
		}
	}
	return out
}

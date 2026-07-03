package telemark

import (
	"strings"
	"unicode/utf8"
)

// parseDocument parses a full Markdown document into a slice of block-level
// nodes.
func parseDocument(src string) []*node {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	lines := strings.Split(src, "\n")
	return parseBlocks(lines)
}

func parseBlocks(lines []string) []*node {
	var blocks []*node
	i := 0
	for i < len(lines) {
		line := lines[i]

		// blank line
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}

		// fenced code block
		if fence, lang, ok := codeFence(line); ok {
			var body []string
			i++
			for i < len(lines) {
				if f, _, ok := codeFence(lines[i]); ok && f == fence {
					i++
					break
				}
				body = append(body, lines[i])
				i++
			}
			blocks = append(blocks, &node{
				typ:     nodeCodeBlock,
				lang:    lang,
				literal: strings.Join(body, "\n"),
			})
			continue
		}

		// ATX heading
		if level, text, ok := atxHeading(line); ok {
			blocks = append(blocks, &node{
				typ:      nodeHeading,
				level:    level,
				children: parseInline(text),
			})
			i++
			continue
		}

		// horizontal rule
		if isThematicBreak(line) {
			blocks = append(blocks, &node{typ: nodeText, literal: "———"})
			i++
			continue
		}

		// blockquote
		if isBlockquote(line) {
			var quoted []string
			for i < len(lines) && isBlockquote(lines[i]) {
				quoted = append(quoted, stripQuote(lines[i]))
				i++
			}
			blocks = append(blocks, &node{
				typ:      nodeBlockquote,
				children: parseBlocks(quoted),
			})
			continue
		}

		// GFM table -> rendered as an aligned monospace block, since Telegram
		// MarkdownV2 has no table construct.
		if tbl, next, ok := tryTable(lines, i); ok {
			blocks = append(blocks, tbl)
			i = next
			continue
		}

		// list
		if _, _, _, ok := listMarker(line); ok {
			list, next := parseList(lines, i)
			blocks = append(blocks, list)
			i = next
			continue
		}

		// paragraph: consume consecutive non-blank lines that don't start a
		// different block.
		var para []string
		for i < len(lines) {
			l := lines[i]
			if strings.TrimSpace(l) == "" {
				break
			}
			if _, _, ok := codeFence(l); ok {
				break
			}
			if _, _, ok := atxHeading(l); ok {
				break
			}
			if isBlockquote(l) {
				break
			}
			if _, _, _, ok := listMarker(l); ok {
				break
			}
			if _, _, ok := tryTable(lines, i); ok {
				break
			}
			para = append(para, l)
			i++
		}
		blocks = append(blocks, &node{
			typ:      nodeParagraph,
			children: parseInline(strings.Join(para, "\n")),
		})
	}
	return blocks
}

func parseList(lines []string, start int) (*node, int) {
	_, ordered, startNum, _ := listMarker(lines[start])
	list := &node{typ: nodeList, ordered: ordered, start: startNum}

	i := start
	for i < len(lines) {
		marker, ord, _, ok := listMarker(lines[i])
		if !ok || ord != ordered {
			break
		}
		// item text is the remainder of the marker line plus subsequent
		// indented continuation lines.
		content := lines[i][len(marker):]
		item := []string{strings.TrimLeft(content, " ")}
		i++
		for i < len(lines) {
			if strings.TrimSpace(lines[i]) == "" {
				break
			}
			if _, _, _, ok := listMarker(lines[i]); ok {
				break
			}
			if strings.HasPrefix(lines[i], "  ") || strings.HasPrefix(lines[i], "\t") {
				item = append(item, strings.TrimLeft(lines[i], " \t"))
				i++
				continue
			}
			break
		}
		list.add(&node{
			typ:      nodeListItem,
			children: parseInline(strings.Join(item, "\n")),
		})
	}
	return list, i
}

// tryTable detects a GFM pipe table starting at lines[start] (a header row
// immediately followed by a delimiter row) and renders it as an aligned
// monospace block wrapped in a nodeCodeBlock. Telegram has no table support, so
// a fixed-width pre block is the closest faithful representation.
func tryTable(lines []string, start int) (*node, int, bool) {
	if start+1 >= len(lines) {
		return nil, 0, false
	}
	if !strings.Contains(lines[start], "|") || !isTableDelimiterRow(lines[start+1]) {
		return nil, 0, false
	}

	header := splitTableRow(lines[start])
	if len(header) == 0 {
		return nil, 0, false
	}

	rows := [][]string{header}
	i := start + 2
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" || !strings.Contains(lines[i], "|") {
			break
		}
		rows = append(rows, splitTableRow(lines[i]))
		i++
	}

	cols := len(header)
	widths := make([]int, cols)
	for _, r := range rows {
		for c := 0; c < cols; c++ {
			w := 0
			if c < len(r) {
				w = utf8.RuneCountInString(r[c])
			}
			if w > widths[c] {
				widths[c] = w
			}
		}
	}

	var b strings.Builder
	writeRow := func(cells []string) {
		for c := 0; c < cols; c++ {
			if c > 0 {
				b.WriteString(" | ")
			}
			cell := ""
			if c < len(cells) {
				cell = cells[c]
			}
			b.WriteString(padRight(cell, widths[c]))
		}
		b.WriteByte('\n')
	}

	writeRow(header)
	for c := 0; c < cols; c++ {
		if c > 0 {
			b.WriteString("-+-")
		}
		b.WriteString(strings.Repeat("-", widths[c]))
	}
	b.WriteByte('\n')
	for _, r := range rows[1:] {
		writeRow(r)
	}

	return &node{typ: nodeCodeBlock, literal: strings.TrimRight(b.String(), "\n")}, i, true
}

// isTableDelimiterRow reports whether line is a table delimiter row such as
// "| --- | :--: |".
func isTableDelimiterRow(line string) bool {
	s := strings.TrimSpace(line)
	if !strings.Contains(s, "|") || !strings.Contains(s, "-") {
		return false
	}
	for _, r := range s {
		if r != '|' && r != '-' && r != ':' && r != ' ' {
			return false
		}
	}
	return true
}

// splitTableRow splits a "| a | b |" row into trimmed cell strings, treating a
// backslash-escaped "\|" as literal pipe content rather than a column border.
func splitTableRow(line string) []string {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")

	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			if s[i+1] == '|' {
				cur.WriteByte('|') // unescape \| into a literal pipe
			} else {
				cur.WriteByte('\\')
				cur.WriteByte(s[i+1])
			}
			i++
			continue
		}
		if c == '|' {
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	if last := strings.TrimSpace(cur.String()); last != "" || len(cells) == 0 {
		cells = append(cells, last)
	}
	return cells
}

func padRight(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// codeFence reports whether line opens/closes a fenced code block, returning
// the fence string ("```" or "~~~") and the info string (language).
func codeFence(line string) (fence, lang string, ok bool) {
	t := strings.TrimLeft(line, " ")
	switch {
	case strings.HasPrefix(t, "```"):
		fence = "```"
	case strings.HasPrefix(t, "~~~"):
		fence = "~~~"
	default:
		return "", "", false
	}
	info := strings.TrimSpace(t[len(fence):])
	// an info string may not contain the fence char for backticks
	if fence == "```" && strings.Contains(info, "`") {
		return "", "", false
	}
	return fence, info, true
}

func atxHeading(line string) (level int, text string, ok bool) {
	t := strings.TrimLeft(line, " ")
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0, "", false
	}
	if n < len(t) && t[n] != ' ' {
		return 0, "", false
	}
	text = strings.TrimSpace(t[n:])
	text = strings.TrimRight(text, "#")
	text = strings.TrimSpace(text)
	return n, text, true
}

func isThematicBreak(line string) bool {
	t := strings.TrimSpace(line)
	if len(t) < 3 {
		return false
	}
	c := t[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] != c && t[i] != ' ' {
			return false
		}
	}
	return true
}

func isBlockquote(line string) bool {
	t := strings.TrimLeft(line, " ")
	return strings.HasPrefix(t, ">")
}

func stripQuote(line string) string {
	t := strings.TrimLeft(line, " ")
	t = strings.TrimPrefix(t, ">")
	return strings.TrimPrefix(t, " ")
}

// listMarker reports whether line begins a list item. It returns the full
// marker prefix (including trailing space), whether the list is ordered, and
// the starting number for ordered lists.
func listMarker(line string) (marker string, ordered bool, start int, ok bool) {
	i := 0
	for i < len(line) && (line[i] == ' ') {
		i++
	}
	if i >= len(line) {
		return "", false, 0, false
	}
	// bullet: - + *
	if c := line[i]; c == '-' || c == '+' || c == '*' {
		if i+1 < len(line) && line[i+1] == ' ' {
			return line[:i+2], false, 0, true
		}
		return "", false, 0, false
	}
	// ordered: digits followed by . or )
	j := i
	for j < len(line) && line[j] >= '0' && line[j] <= '9' {
		j++
	}
	if j == i || j >= len(line) {
		return "", false, 0, false
	}
	if (line[j] == '.' || line[j] == ')') && j+1 < len(line) && line[j+1] == ' ' {
		num := 0
		for _, d := range line[i:j] {
			num = num*10 + int(d-'0')
		}
		return line[:j+2], true, num, true
	}
	return "", false, 0, false
}

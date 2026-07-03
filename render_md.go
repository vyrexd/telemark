package telemark

import (
	"fmt"
	"strings"
)

// renderMarkdownV2 renders the block list into a single MarkdownV2 string.
func renderMarkdownV2(blocks []*node, opts Options) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		parts = append(parts, renderBlockMD(b, opts))
	}
	return strings.Join(parts, "\n\n")
}

func renderBlockMD(n *node, opts Options) string {
	switch n.typ {
	case nodeParagraph:
		return renderInlineMD(n.children)

	case nodeHeading:
		return "*" + renderInlineMD(n.children) + "*"

	case nodeCodeBlock:
		lang := ""
		if n.lang != "" {
			lang = escapeCode(n.lang)
		}
		return "```" + lang + "\n" + escapeCode(n.literal) + "\n```"

	case nodeBlockquote:
		inner := renderMarkdownV2(n.children, opts)
		var b strings.Builder
		lines := strings.Split(inner, "\n")
		prefix := ">"
		if opts.ExpandableQuotes {
			prefix = "**>"
		}
		for i, line := range lines {
			if i > 0 {
				b.WriteByte('\n')
			}
			// only the first line of an expandable quote takes the ** marker
			if opts.ExpandableQuotes && i == 0 {
				b.WriteString(prefix)
			} else {
				b.WriteString(">")
			}
			b.WriteString(line)
		}
		return b.String()

	case nodeList:
		var b strings.Builder
		num := n.start
		for i, item := range n.children {
			if i > 0 {
				b.WriteByte('\n')
			}
			if n.ordered {
				fmt.Fprintf(&b, "%d\\. ", num)
				num++
			} else {
				b.WriteString("• ")
			}
			b.WriteString(renderInlineMD(item.children))
		}
		return b.String()

	default:
		// stray inline node used as a block (e.g. thematic break placeholder)
		return renderInlineMD([]*node{n})
	}
}

func wrap(b *strings.Builder, open, inner, close string) {
	b.WriteString(open)
	b.WriteString(inner)
	b.WriteString(close)
}

func renderInlineMD(nodes []*node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.typ {
		case nodeText:
			b.WriteString(escapeText(n.literal))
		case nodeLineBreak:
			b.WriteByte('\n')
		case nodeBold:
			wrap(&b, "*", renderInlineMD(n.children), "*")
		case nodeItalic:
			wrap(&b, "_", renderInlineMD(n.children), "_")
		case nodeUnderline:
			wrap(&b, "__", renderInlineMD(n.children), "__")
		case nodeStrike:
			wrap(&b, "~", renderInlineMD(n.children), "~")
		case nodeSpoiler:
			wrap(&b, "||", renderInlineMD(n.children), "||")
		case nodeCode:
			wrap(&b, "`", escapeCode(n.literal), "`")
		case nodeLink:
			b.WriteByte('[')
			b.WriteString(renderInlineMD(n.children))
			b.WriteString("](")
			b.WriteString(escapeURL(n.url))
			b.WriteByte(')')
		}
	}
	return b.String()
}

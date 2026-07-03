package telemark

import (
	"fmt"
	"strings"
)

// entBuilder accumulates plain text together with the MessageEntities that
// describe its formatting. Offsets and lengths are tracked in UTF-16 code
// units, as required by the Telegram Bot API.
type entBuilder struct {
	sb  strings.Builder
	pos int // current offset in UTF-16 code units
	ent []Entity
}

func (b *entBuilder) write(s string) {
	b.sb.WriteString(s)
	b.pos += utf16Len(s)
}

// span records an entity covering everything written by fn.
func (b *entBuilder) span(typ string, fn func(), set func(*Entity)) {
	start := b.pos
	fn()
	length := b.pos - start
	if length <= 0 {
		return
	}
	e := Entity{Type: typ, Offset: start, Length: length}
	if set != nil {
		set(&e)
	}
	b.ent = append(b.ent, e)
}

// renderEntities renders the block list into plain text plus entities.
func renderEntities(blocks []*node) (string, []Entity) {
	b := &entBuilder{}
	for i, blk := range blocks {
		if i > 0 {
			b.write("\n\n")
		}
		renderBlockEnt(b, blk)
	}
	return b.sb.String(), b.ent
}

func renderBlockEnt(b *entBuilder, n *node) {
	switch n.typ {
	case nodeParagraph:
		renderInlineEnt(b, n.children)

	case nodeHeading:
		b.span("bold", func() { renderInlineEnt(b, n.children) }, nil)

	case nodeCodeBlock:
		b.span("pre", func() { b.write(n.literal) }, func(e *Entity) {
			e.Language = n.lang
		})

	case nodeBlockquote:
		b.span("blockquote", func() {
			for i, c := range n.children {
				if i > 0 {
					b.write("\n")
				}
				renderBlockEnt(b, c)
			}
		}, nil)

	case nodeList:
		num := n.start
		for i, item := range n.children {
			if i > 0 {
				b.write("\n")
			}
			if n.ordered {
				b.write(fmt.Sprintf("%d. ", num))
				num++
			} else {
				b.write("• ")
			}
			renderInlineEnt(b, item.children)
		}

	default:
		renderInlineEnt(b, []*node{n})
	}
}

func renderInlineEnt(b *entBuilder, nodes []*node) {
	for _, n := range nodes {
		switch n.typ {
		case nodeText:
			b.write(n.literal)
		case nodeLineBreak:
			b.write("\n")
		case nodeBold:
			b.span("bold", func() { renderInlineEnt(b, n.children) }, nil)
		case nodeItalic:
			b.span("italic", func() { renderInlineEnt(b, n.children) }, nil)
		case nodeUnderline:
			b.span("underline", func() { renderInlineEnt(b, n.children) }, nil)
		case nodeStrike:
			b.span("strikethrough", func() { renderInlineEnt(b, n.children) }, nil)
		case nodeSpoiler:
			b.span("spoiler", func() { renderInlineEnt(b, n.children) }, nil)
		case nodeCode:
			b.span("code", func() { b.write(n.literal) }, nil)
		case nodeLink:
			b.span("text_link", func() { renderInlineEnt(b, n.children) }, func(e *Entity) {
				e.URL = n.url
			})
		}
	}
}

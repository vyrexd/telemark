package telemark

// nodeType enumerates the kinds of nodes produced by the parser.
type nodeType int

const (
	nodeText nodeType = iota
	nodeLineBreak

	// inline formatting
	nodeBold
	nodeItalic
	nodeUnderline
	nodeStrike
	nodeSpoiler
	nodeCode // inline code span
	nodeLink

	// block level
	nodeParagraph
	nodeHeading
	nodeBlockquote
	nodeCodeBlock // fenced code block
	nodeList
	nodeListItem
)

// node is a single element of the parsed document tree.
//
// The tree is intentionally small: it only models the constructs that map onto
// Telegram's MarkdownV2 / MessageEntity feature set. Anything Markdown supports
// that Telegram does not (tables, nested emphasis quirks, images, etc.) is
// degraded to the closest supported representation while parsing/rendering.
type node struct {
	typ      nodeType
	literal  string // text / code / code-block content
	url      string // link destination
	lang     string // fenced code block language (info string)
	level    int    // heading level (1..6)
	ordered  bool   // list: ordered vs bullet
	start    int    // ordered list: number of the first item
	children []*node
}

func (n *node) add(child *node) { n.children = append(n.children, child) }

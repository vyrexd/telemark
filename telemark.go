// Package telemark converts standard (CommonMark / GitHub-flavoured) Markdown
// into the flavour of Markdown that the Telegram Bot API understands, known as
// MarkdownV2.
//
// It offers three outputs:
//
//   - Convert       — a ready-to-send MarkdownV2 string (parse_mode=MarkdownV2).
//   - Split         — the same, chopped into messages that respect Telegram's
//     4096-character limit without ever breaking formatting.
//   - Entities      — plain text plus a slice of MessageEntity descriptors, the
//     entity-based alternative to parse_mode.
//
// The package has zero external dependencies.
package telemark

// TelegramMessageLimit is the maximum length, in UTF-16 code units, of a single
// Telegram text message.
const TelegramMessageLimit = 4096

// Options tunes conversion behaviour. The zero value is a sensible default.
type Options struct {
	// ExpandableQuotes renders blockquotes as Telegram's expandable
	// (collapsible) quotes using the "**>" marker.
	ExpandableQuotes bool
}

// Entity describes a formatted range of a message, mirroring Telegram's
// MessageEntity object. Offset and Length are measured in UTF-16 code units.
//
// Type is one of: "bold", "italic", "underline", "strikethrough", "spoiler",
// "code", "pre", "text_link", "blockquote".
type Entity struct {
	Type     string `json:"type"`
	Offset   int    `json:"offset"`
	Length   int    `json:"length"`
	URL      string `json:"url,omitempty"`      // for "text_link"
	Language string `json:"language,omitempty"` // for "pre"
}

// Convert turns Markdown source into a single MarkdownV2 string suitable for
// sending with parse_mode="MarkdownV2".
func Convert(markdown string) string {
	return ConvertWith(markdown, Options{})
}

// ConvertWith is Convert with explicit options.
func ConvertWith(markdown string, opts Options) string {
	return renderMarkdownV2(parseDocument(markdown), opts)
}

// Split converts Markdown and splits the result into chunks no longer than
// maxLen UTF-16 code units, breaking only at block boundaries so that no
// formatting token is ever split across messages. If maxLen <= 0,
// TelegramMessageLimit is used.
func Split(markdown string, maxLen int) []string {
	return SplitWith(markdown, maxLen, Options{})
}

// SplitWith is Split with explicit options.
func SplitWith(markdown string, maxLen int, opts Options) []string {
	if maxLen <= 0 {
		maxLen = TelegramMessageLimit
	}
	return splitBlocks(parseDocument(markdown), opts, maxLen)
}

// Entities converts Markdown into plain text plus the MessageEntities that
// describe its formatting. This is the entity-based alternative to
// parse_mode="MarkdownV2": pass text as the message and entities in the
// "entities" field of sendMessage.
func Entities(markdown string) (text string, entities []Entity) {
	return renderEntities(parseDocument(markdown))
}

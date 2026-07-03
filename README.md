# telemark

[![CI](https://github.com/vyrexd/telemark/actions/workflows/ci.yml/badge.svg)](https://github.com/vyrexd/telemark/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/vyrexd/telemark.svg)](https://pkg.go.dev/github.com/vyrexd/telemark)
[![Go Report Card](https://goreportcard.com/badge/github.com/vyrexd/telemark)](https://goreportcard.com/report/github.com/vyrexd/telemark)

Convert standard **Markdown** (CommonMark / GitHub-flavoured) into the flavour
Telegram bots actually understand — **MarkdownV2** — in pure Go, with **zero
dependencies**.

Telegram's `MarkdownV2` is *not* regular Markdown: every one of
`` _ * [ ] ( ) ~ ` > # + - = | { } . ! `` must be backslash-escaped unless it is
part of a formatting token, or the Bot API rejects your message with
`can't parse entities`. `telemark` does that escaping correctly, maps standard
Markdown constructs onto Telegram's, and never leaves a dangling delimiter.

## Install

```bash
go get github.com/vyrexd/telemark
```

## Usage

### 1. A ready-to-send MarkdownV2 string

```go
text := telemark.Convert("# Hello\n\nThis is **bold**, _italic_ and `code` with a cost of $5.50.")
// send with parse_mode = "MarkdownV2"
```

Output:

```
*Hello*

This is *bold*, _italic_ and `code` with a cost of $5\.50\.
```

### 2. Auto-split long messages (Telegram's 4096-char limit)

Splitting happens at block boundaries, so a formatting token is **never** cut in
half across two messages.

```go
for _, msg := range telemark.Split(longMarkdown, telemark.TelegramMessageLimit) {
    bot.Send(chatID, msg) // each msg is valid MarkdownV2 on its own
}
```

### 3. Entities instead of parse_mode

The entity API returns plain text plus `MessageEntity` descriptors (offsets in
UTF-16 code units, exactly as Telegram expects). This avoids escaping entirely
and is the most robust option for complex text.

```go
text, entities := telemark.Entities("hello **world** and `code`")
// text     == "hello world and code"
// entities == [{bold 6 5} {code 16 4}]
// send with the "entities" field instead of parse_mode
```

## Supported Markdown → Telegram mapping

| Markdown                     | Telegram MarkdownV2 | Entity type     |
| ---------------------------- | ------------------- | --------------- |
| `**bold**`, `__bold__`       | `*bold*`            | `bold`          |
| `*italic*`, `_italic_`       | `_italic_`          | `italic`        |
| `~~strike~~`                 | `~strike~`          | `strikethrough` |
| `\|\|spoiler\|\|`            | `\|\|spoiler\|\|`   | `spoiler`       |
| `` `code` ``                 | `` `code` ``        | `code`          |
| ` ```lang ... ``` `          | fenced block        | `pre`           |
| `[text](url)`, `<url>`       | `[text](url)`       | `text_link`     |
| `# Heading`                  | `*Heading*` (bold)  | `bold`          |
| `> quote`                    | `> quote`           | `blockquote`    |
| `- item` / `1. item`         | `• item` / `1. item`| —               |

Intraword underscores (`snake_case`) are treated as literal text, not italics —
a common source of broken Telegram messages.

## Options

```go
telemark.ConvertWith(md, telemark.Options{ExpandableQuotes: true}) // collapsible quotes
```

## Notes & limitations

- Input is treated as standard Markdown, so strikethrough is `~~x~~` and spoiler
  is `||x||`. The output uses Telegram's single-`~` strikethrough.
- Images (`![alt](url)`) degrade to a text link, since MarkdownV2 has no inline
  images.
- Tables and deeply nested lists are not part of MarkdownV2 and are flattened.

## License

MIT — see [LICENSE](LICENSE).

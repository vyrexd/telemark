// Command bot is a live demo of the telemark library. It long-polls the
// Telegram Bot API and, for every text message it receives from anyone,
// treats that text as Markdown source and replies with it rendered as
// Telegram MarkdownV2 (via telemark.Convert). No chat id needs to be
// configured — each reply goes back to whoever sent the message.
//
// Usage:
//
//	export BOT_TOKEN=123456:ABC...   # from @BotFather (or put it in .env)
//	go run ./examples/bot
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/vyrexd/telemark"
)

// welcomeMD is plain Markdown source; it is rendered through the library like
// any other message, so it doubles as a self-test of the converter.
const welcomeMD = "# telemark demo\n\nSend me any Markdown and I'll reply with it " +
	"rendered as Telegram MarkdownV2.\n\nCommands:\n\n" +
	"- /demo — full showcase (code blocks, tables, quotes)\n" +
	"- /readme — this project's README, converted and auto-split"

// demoMD showcases every supported construct. The fenced code block and table
// are embedded here as literal source, so the demo does not depend on the
// triple backticks surviving a copy-paste from a rendered README.
const demoMD = "# telemark showcase\n\n" +
	"Regular text with $5.50, a_b_c and 2+2=4 — all escaped safely.\n\n" +
	"Inline `code`, **bold**, _italic_, ~~strike~~, ||spoiler||.\n\n" +
	"```go\n" +
	"func main() {\n" +
	"    x := arr[0]\n" +
	"    fmt.Println(\"hi\\n\")\n" +
	"}\n" +
	"```\n\n" +
	"| Feature | Status  |\n" +
	"| ------- | ------- |\n" +
	"| tables  | aligned |\n" +
	"| code    | mono    |\n\n" +
	"> A quote with _emphasis_ and a [link](https://go.dev)"

func main() {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("set BOT_TOKEN (env or .env)")
	}
	api := "https://api.telegram.org/bot" + token + "/"

	if me, ok := getMe(api); ok {
		log.Printf("started as @%s — message the bot to test", me)
	} else {
		log.Fatal("invalid BOT_TOKEN: getMe failed")
	}

	var offset int64
	for {
		updates, err := getUpdates(api, offset)
		if err != nil {
			log.Printf("getUpdates: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message == nil || u.Message.Text == "" {
				continue
			}
			handle(api, u.Message)
		}
	}
}

func handle(api string, m *message) {
	// /readme streams the project's real README.md through the converter,
	// split into 4096-char messages — an end-to-end test with genuine fences.
	if m.Text == "/readme" {
		sendReadme(api, m.Chat.ID)
		return
	}

	src := m.Text
	switch src {
	case "/start":
		src = welcomeMD
	case "/demo":
		src = demoMD
	}
	reply := telemark.Convert(src)

	ok, desc := sendMarkdownV2(api, m.Chat.ID, reply)
	if ok {
		log.Printf("chat %d: rendered %q -> OK", m.Chat.ID, src)
		return
	}
	// If Telegram rejects our output, that's a real bug — surface it and fall
	// back to the entities API so the user still gets a formatted message.
	log.Printf("chat %d: MarkdownV2 REJECTED: %s | payload=%q", m.Chat.ID, desc, reply)

	text, ents := telemark.Entities(src)
	if ok, desc := sendEntities(api, m.Chat.ID, text, ents); !ok {
		log.Printf("chat %d: entities also rejected: %s", m.Chat.ID, desc)
	} else {
		log.Printf("chat %d: entities fallback OK", m.Chat.ID)
	}
}

// sendReadme reads README.md (path overridable via README_PATH), converts it,
// and sends it as a sequence of MarkdownV2 messages within Telegram's limit.
func sendReadme(api string, chat int64) {
	path := os.Getenv("README_PATH")
	if path == "" {
		path = "README.md"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		sendMarkdownV2(api, chat, "could not read "+telemark.Convert(path)+": "+telemark.Convert(err.Error()))
		return
	}
	chunks := telemark.Split(string(raw), telemark.TelegramMessageLimit)
	for i, c := range chunks {
		ok, desc := sendMarkdownV2(api, chat, c)
		if ok {
			log.Printf("chat %d: readme part %d/%d OK", chat, i+1, len(chunks))
		} else {
			log.Printf("chat %d: readme part %d/%d REJECTED: %s", chat, i+1, len(chunks), desc)
		}
		time.Sleep(400 * time.Millisecond) // stay under rate limits
	}
}

// --- Telegram API plumbing ---

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *message `json:"message"`
}

type message struct {
	Text string `json:"text"`
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}

type apiResp struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

func getMe(api string) (string, bool) {
	r, ok := call(api+"getMe", nil)
	if !ok {
		return "", false
	}
	var res struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(r.Result, &res); err != nil {
		return "", false
	}
	return res.Username, true
}

func getUpdates(api string, offset int64) ([]update, error) {
	r, ok := call(api+"getUpdates", url.Values{
		"offset":          {strconv.FormatInt(offset, 10)},
		"timeout":         {"30"},
		"allowed_updates": {`["message"]`},
	})
	if !ok {
		return nil, fmt.Errorf("%s", r.Description)
	}
	var ups []update
	if err := json.Unmarshal(r.Result, &ups); err != nil {
		return nil, err
	}
	return ups, nil
}

func sendMarkdownV2(api string, chat int64, text string) (bool, string) {
	r, _ := call(api+"sendMessage", url.Values{
		"chat_id":    {strconv.FormatInt(chat, 10)},
		"text":       {text},
		"parse_mode": {"MarkdownV2"},
	})
	return r.OK, r.Description
}

func sendEntities(api string, chat int64, text string, ents []telemark.Entity) (bool, string) {
	entJSON, _ := json.Marshal(ents)
	r, _ := call(api+"sendMessage", url.Values{
		"chat_id":  {strconv.FormatInt(chat, 10)},
		"text":     {text},
		"entities": {string(entJSON)},
	})
	return r.OK, r.Description
}

func call(endpoint string, form url.Values) (apiResp, bool) {
	var resp *http.Response
	var err error
	if form != nil {
		resp, err = http.PostForm(endpoint, form)
	} else {
		resp, err = http.Get(endpoint)
	}
	if err != nil {
		return apiResp{Description: err.Error()}, false
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var r apiResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return apiResp{Description: string(raw)}, false
	}
	return r, r.OK
}
